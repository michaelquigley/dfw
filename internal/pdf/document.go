// Package pdf renders a caller-prepared, local web document through a private
// Chromium process. it has no native window or product filesystem policy.
package pdf

import (
	"fmt"
	"net/url"
	"strconv"
)

// Code identifies an export failure without leaking document URLs or browser output.
type Code string

const (
	Unsupported   Code = "unsupported"
	NotReady      Code = "not_ready"
	Closed        Code = "closed"
	Busy          Code = "busy"
	Unavailable   Code = "browser_unavailable"
	Incompatible  Code = "browser_incompatible"
	Load          Code = "load"
	Policy        Code = "resource_policy"
	RenderFailed  Code = "render"
	ChooserFailed Code = "chooser"
)

// Error is a classified, safe-to-display error. cancellation remains a context error.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return "dfw: pdf: " + e.Message }

// Failure constructs an error using fixed, non-document diagnostic text.
func Failure(code Code, message string) error { return &Error{Code: code, Message: message} }

// Document names a printable HTML document and every resource it may request.
type Document struct {
	URL       string
	Resources []string
}

func (d Document) allowlist() (map[string]bool, error) {
	main, err := localURL(d.URL)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{d.URL: true}
	for _, raw := range d.Resources {
		u, err := localURL(raw)
		if err != nil {
			return nil, err
		}
		if u.Host != main.Host {
			return nil, Failure(Policy, "resources must share the document's loopback origin")
		}
		allowed[raw] = true
	}
	return allowed, nil
}

func localURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, Failure(Policy, "expected an absolute loopback HTTP URL without credentials or a fragment")
	}
	if u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return nil, Failure(Policy, "document resources must use a literal loopback address")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, Failure(Policy, "document resources need an explicit valid port")
	}
	return u, nil
}

func text(m map[string]any, key string) string           { v, _ := m[key].(string); return v }
func object(m map[string]any, key string) map[string]any { v, _ := m[key].(map[string]any); return v }
func number(m map[string]any, key string) float64        { v, _ := m[key].(float64); return v }

func browserVersion(product string) error {
	var major int
	if _, err := fmt.Sscanf(product, "Chrome/%d.", &major); err != nil {
		if _, err = fmt.Sscanf(product, "HeadlessChrome/%d.", &major); err != nil {
			return Failure(Incompatible, "could not identify the Chromium version")
		}
	}
	if major < 131 {
		return Failure(Incompatible, "Chromium 131 or newer is required")
	}
	return nil
}
