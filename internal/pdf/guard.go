package pdf

import (
	"context"
	"sync"
)

// guard services Fetch events independently of command replies. a paused
// forbidden request is never continued; cancellation tears down its browser.
type guard struct {
	pipe    *pipe
	allowed map[string]bool
	target  string
	session string
	cancel  context.CancelCauseFunc
	mu      sync.Mutex
	loaded  map[string]bool
	changed chan struct{}
	done    chan struct{}
}

func newGuard(p *pipe, allowed map[string]bool, target, session string, cancel context.CancelCauseFunc) *guard {
	return &guard{pipe: p, allowed: allowed, target: target, session: session, cancel: cancel, loaded: make(map[string]bool), changed: make(chan struct{}, 1), done: make(chan struct{})}
}

func (g *guard) run(ctx context.Context) {
	defer close(g.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-g.pipe.done:
			g.cancel(g.pipe.failure())
			return
		case event := <-g.pipe.events:
			if err := g.handle(ctx, event); err != nil {
				g.cancel(err)
				return
			}
		}
	}
}

func (g *guard) handle(ctx context.Context, event map[string]any) error {
	method, params := text(event, "method"), object(event, "params")
	switch method {
	case "Target.targetCreated":
		info := object(params, "targetInfo")
		// Chromium 153 reports its own browser_ui target even in headless
		// mode. it is not a document, popup, frame, or worker.
		if text(info, "type") == "browser_ui" {
			return nil
		}
		if text(info, "targetId") != g.target {
			return Failure(Policy, "unexpected browser target during PDF rendering")
		}
	case "Target.attachedToTarget":
		if text(object(params, "targetInfo"), "targetId") != g.target {
			return Failure(Policy, "child browser targets are not allowed during PDF rendering")
		}
	case "Browser.downloadWillBegin", "Page.downloadWillBegin":
		return Failure(Policy, "downloads are not allowed during PDF rendering")
	case "Inspector.targetCrashed", "Target.targetCrashed":
		return Failure(RenderFailed, "PDF renderer crashed")
	}
	if text(event, "sessionId") != g.session {
		return nil
	}
	switch method {
	case "Page.frameAttached":
		return Failure(Policy, "frames are not allowed during PDF rendering")
	case "Fetch.requestPaused":
		request := object(params, "request")
		if text(request, "method") != "GET" || !g.allowed[text(request, "url")] {
			return Failure(Policy, "document requested a resource outside its PDF manifest")
		}
		if status, exists := params["responseStatusCode"]; exists {
			n, ok := status.(float64)
			if !ok || n < 200 || n >= 300 {
				return Failure(Load, "PDF resource failed or redirected")
			}
		}
		if text(params, "responseErrorReason") != "" {
			return Failure(Load, "PDF resource failed to load")
		}
		_, err := g.pipe.call(ctx, "Fetch.continueRequest", map[string]any{"requestId": text(params, "requestId")}, g.session)
		return err
	case "Network.loadingFailed":
		return Failure(Load, "PDF resource failed to load")
	case "Page.lifecycleEvent":
		if text(params, "name") == "load" {
			g.mu.Lock()
			g.loaded[text(params, "loaderId")] = true
			g.mu.Unlock()
			select {
			case g.changed <- struct{}{}:
			default:
			}
		}
	}
	return nil
}

func (g *guard) waitLoad(ctx context.Context, loader string) error {
	for {
		g.mu.Lock()
		ready := g.loaded[loader]
		g.mu.Unlock()
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-g.changed:
		}
	}
}
