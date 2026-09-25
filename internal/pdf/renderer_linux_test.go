//go:build linux

package pdf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// native prerequisites fail explicitly when opted in; ordinary tests need no browser.
func nativePrerequisites(t *testing.T) string {
	t.Helper()
	if os.Getenv("DFW_PDF_NATIVE") != "1" {
		t.Skip("set DFW_PDF_NATIVE=1 for the real Chromium gate")
	}
	for _, tool := range []string{"pdfinfo", "pdffonts", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("install '%s' for the native PDF gate", tool)
		}
	}
	font := os.Getenv("DFW_PDF_TEST_FONT")
	if font == "" {
		t.Fatal("set DFW_PDF_TEST_FONT to a readable TrueType font")
	}
	if _, err := executable(os.Getenv("DFW_PDF_EXECUTABLE")); err != nil {
		t.Fatal(err)
	}
	return font
}

func TestNativeRender(t *testing.T) {
	font, err := os.ReadFile(nativePrerequisites(t))
	if err != nil {
		t.Fatal(err)
	}
	var fontReads, imageReads atomic.Int32
	rows := strings.Repeat("<tr><td>row marker</td><td>a complete row of prose for pagination</td></tr>", 90)
	html := `<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"><style>
@font-face{font-family:proof;src:url('/font.ttf')}
@page{size:letter;margin:18mm;@bottom-center{content:"page " counter(page) " of " counter(pages)}}
body{font:12pt/1.5 proof}td,th{padding:6pt;border:1px solid #999}table{width:100%;border-collapse:collapse}thead{display:table-header-group}tr{break-inside:avoid}
</style></head><body><h1>native PDF proof</h1><p>selectable café</p><a href="https://example.com/pdf-proof">external link</a><img src="/image.svg"><table><thead><tr><th>repeated heading</th><th>description</th></tr></thead><tbody>` + rows + `</tbody></table><p>final paragraph marker</p></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
		case "/font.ttf":
			fontReads.Add(1)
			time.Sleep(150 * time.Millisecond)
			w.Header().Set("Content-Type", "font/ttf")
			_, _ = w.Write(font)
		case "/image.svg":
			imageReads.Add(1)
			time.Sleep(200 * time.Millisecond)
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="30"><rect width="80" height="30" fill="brown"/></svg>`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	data, err := Render(context.Background(), os.Getenv("DFW_PDF_EXECUTABLE"), Document{URL: srv.URL + "/", Resources: []string{srv.URL + "/font.ttf", srv.URL + "/image.svg"}})
	if err != nil {
		t.Fatal(err)
	}
	if fontReads.Load() == 0 || imageReads.Load() == 0 {
		t.Fatal("resources were not loaded")
	}
	path := filepath.Join(t.TempDir(), "proof.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	text := runTool(t, "pdftotext", "-layout", path, "-")
	if strings.Count(text, "row marker") != 90 || !strings.Contains(text, "final paragraph marker") || !strings.Contains(text, "café") {
		t.Fatalf("missing PDF content: %s", text)
	}
	if strings.Count(text, "repeated heading") < 2 || !strings.Contains(text, "page 1 of ") {
		t.Fatalf("missing pagination: %s", text)
	}
	if !strings.Contains(runTool(t, "pdfinfo", "-url", path), "https://example.com/pdf-proof") {
		t.Fatal("missing clickable link")
	}
	if !strings.Contains(runTool(t, "pdffonts", path), "yes") {
		t.Fatal("fonts not embedded")
	}
}

func runTool(t *testing.T, tool string, args ...string) string {
	t.Helper()
	out, err := exec.Command(tool, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v: %s", tool, err, out)
	}
	return string(out)
}

func TestNativePolicyAndFailure(t *testing.T) {
	nativePrerequisites(t)
	var forbidden atomic.Int32
	hostile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		t.Error("forbidden resource was contacted")
	}))
	defer hostile.Close()
	for _, tc := range []struct {
		name, html string
		status     int
		code       Code
	}{
		{"outside-manifest", `<img src="` + hostile.URL + `/secret">`, 200, Policy},
		{"redirect", ``, 302, Load},
		{"decode", `<img src="/bad">`, 200, Load},
		{"font", `<style>@font-face{font-family:bad;src:url('/bad')}body{font-family:bad}</style><p>text</p>`, 200, Load},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/bad" {
					w.Header().Set("Content-Type", "application/octet-stream")
					_, _ = w.Write([]byte("not an image or font"))
					return
				}
				if tc.status == 302 {
					w.Header().Set("Location", hostile.URL+"/redirect")
				}
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`<link rel="icon" href="data:,">` + tc.html))
			}))
			defer srv.Close()
			data, err := Render(context.Background(), os.Getenv("DFW_PDF_EXECUTABLE"), Document{URL: srv.URL + "/", Resources: []string{srv.URL + "/bad"}})
			var classified *Error
			if len(data) != 0 || !errors.As(err, &classified) || classified.Code != tc.code {
				t.Fatalf("got %d bytes, %v", len(data), err)
			}
		})
	}
	if forbidden.Load() != 0 {
		t.Fatal("resource policy allowed network access")
	}
}

func TestNativeScriptsAndCancellation(t *testing.T) {
	nativePrerequisites(t)
	loaded := make(chan struct{}, 1)
	var scriptRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/executed" {
			scriptRequests.Add(1)
			t.Error("authored script executed")
			return
		}
		if r.URL.Path == "/slow" {
			loaded <- struct{}{}
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<link rel="icon" href="data:,"><script>fetch('/executed')</script><p>inert script</p>`))
	}))
	defer srv.Close()
	_, err := Render(context.Background(), os.Getenv("DFW_PDF_EXECUTABLE"), Document{URL: srv.URL + "/", Resources: []string{srv.URL + "/executed"}})
	if err != nil {
		t.Fatal(err)
	}
	if scriptRequests.Load() != 0 {
		t.Fatal("script ran")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Render(ctx, os.Getenv("DFW_PDF_EXECUTABLE"), Document{URL: srv.URL + "/slow"})
		done <- err
	}()
	select {
	case <-loaded:
		cancel()
	case <-time.After(20 * time.Second):
		t.Fatal("browser never requested document")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("renderer did not cancel")
	}
}
