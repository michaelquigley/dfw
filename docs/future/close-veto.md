# Close veto

Today a window-manager close is final. The native window is destroyed, the blocking webview loop returns, dfw saves the window state, and the process leaves; application code only observes the result. That is enough for applications whose state is already durable, but not for a document window holding work the writer has not chosen to save or discard.

Close veto gives the application one narrow capability: receive a close request before the native window is destroyed, hold that request while it makes its own decision, then either close the window or keep it open. dfw owns the native interception and the window lifecycle. The product owns every question above that layer... whether work is dirty, how its page is asked, what saving or discarding means, and what the interface looks like while the decision is pending.

## Public contract

`App` and `WindowApp` both gain the same optional callback:

```go
type App struct {
	// existing fields...
	OnCloseRequest func(*CloseRequest)
}

type WindowApp struct {
	// existing fields...
	OnCloseRequest func(*CloseRequest)
}
```

The request is a concrete, dfw-created capability with two resolutions:

```go
type CloseRequest struct {
	// unexported lifecycle state
}

func (*CloseRequest) Close()
func (*CloseRequest) KeepOpen()
```

There is no explicit `NotYet` result. A newly delivered request is pending, and returning from the callback without resolving it leaves it pending. `Close` and `KeepOpen` are safe to call from any goroutine. The first resolution wins; repeated calls and calls against a request whose window has already ended do nothing.

A nil callback preserves the current behavior exactly. The native close proceeds, the webview loop returns, window state is saved, and the window is destroyed.

## One pending request

The first native close request is suppressed before the callback is dispatched. dfw establishes the pending state first, then invokes `OnCloseRequest` asynchronously in its own goroutine so product work never holds the native UI thread.

Only one request may be pending. Further native close requests while it remains unresolved are consumed and dropped: they do not create another request, invoke the callback again, queue work for later, or imply stronger consent to close. There is no timeout and a repeated click is never a forced close. The request remains pending until the product resolves it; operating-system process controls are the hard escape from an application that cannot answer.

`KeepOpen` clears the pending state and rearms the native close path. A later close creates a new request and invokes the callback again. `Close` grants consent to end the window lifecycle and terminates the blocking webview loop without passing through the callback a second time.

Every delivered request has its own identity. If request A resolves `KeepOpen` and a later native close creates request B, a delayed call on A cannot resolve B. This identity is internal to dfw; products do not compare or transport it unless they create an application-level identifier for their own protocol.

## Lifecycle boundaries

The veto governs native close requests from the window manager: the title-bar close control, the platform's close shortcut, and equivalent native messages. It does not govern dfw's own termination paths. A supervised HTTP server failure still terminates a `Run` window immediately, returns the serve error, and leaves any outstanding request unable to act. Cleanup after the webview loop returns also bypasses the veto.

The existing close sequence stays intact. A vetoed close does not return from the webview loop and therefore does not save window state. Once a close is allowed, the loop returns, `SaveWindowState` observes the final bounds and zoom, and deferred destruction runs as it does today. Native interception is detached before that destruction so cleanup cannot appear as a new close request.

`WindowApp` receives the same capability as `App` because both own the same kind of native window. dfw does not add transport to make daemon-mode decisions. A `WindowApp` product that needs another process or its page to participate owns that round trip, just as a `Run` product owns the in-process HTTP or event-stream path it uses.

## Platform behavior

Linux and Windows implement the same contract through their existing native integration layers. Linux handles GTK's `delete-event`, suppressing it while a request is pending. Windows handles `WM_CLOSE` through the window-procedure integration already used for bounds tracking, consuming it until the request grants close.

Close veto is opt-in but not best-effort. If `OnCloseRequest` is configured on a platform without an implementation, or if dfw cannot install the native interceptor on a supported platform, window setup fails rather than running without the promised interception. Silently falling back to a hard close would turn a product's data-safety decision into platform-dependent behavior. A nil callback needs no interceptor and remains supported wherever the ordinary webview runs.

## Scenarios

### Immediate close

An application knows it has nothing to preserve. Its callback calls `request.Close()` immediately. The native event was still intercepted, but the asynchronously delivered answer ends the window through the ordinary dfw shutdown path.

### Deferred decision

A document application receives the request, publishes a message to its page over its own channel, and returns. The native window stays open. When the page finishes asking the writer, the product calls `Close` after save or discard succeeds, or `KeepOpen` when the writer stays.

gloss is the motivating case. Its page can use the close request as the occasion to send its ordinary editor state to the session owner before offering save, discard, or stay. The exact drain, prompt, and discard behavior belong to gloss; dfw supplies only the pending native request and its eventual resolution.

### Repeated native close

The application has a pending request and is waiting for its page. The writer clicks the native close control again. dfw consumes the second event and does nothing else. The original request remains the only request, with the same eventual `Close` or `KeepOpen` resolution.

### Keep open, then close again

The writer chooses to stay, so the product calls `KeepOpen`. A later native close is a new attempt and produces a new request. A delayed method call retained from the first attempt cannot affect the second.

### Internal termination while pending

The application is considering a close when its supervised server fails. dfw terminates the webview, returns the server error, saves the final window state, and destroys the window. A later resolution of the abandoned request is a no-op.

### Unsupported platform

An application configures `OnCloseRequest` where no native interceptor exists. The entry point returns an explicit error rather than opening a window whose close behavior contradicts the configured contract.

## Seam census

**Model / transport — separate.** `CloseRequest` models one native lifecycle decision and carries no HTTP, event-stream, JavaScript, or daemon protocol. Products transport the existence and outcome of that decision through their own application boundary. Revisit only if multiple consumers independently prove that a common transport belongs in dfw rather than in their servers.

**Contract circumvention — native user close goes through the interceptor; internal termination deliberately does not.** A configured callback must cover the platform's ordinary native close path, while server failure and dfw cleanup retain their existing authority to end the loop. Native destruction during cleanup must not re-enter the application callback.

**Error by tier — fail at window bootstrap when the promised native contract cannot be installed; make lifecycle races harmless at runtime.** Unsupported platforms and interceptor installation failures fail window setup rather than leaving an unprotected window running. Duplicate native requests are dropped, and stale or repeated resolutions are no-ops rather than runtime errors.

## Deferred (and Why)

**macOS interception.** macOS remains outside dfw's exercised platform set. Its close delegate belongs with the broader native integration when that platform becomes supported, not as an untested branch carried by this feature.

**A general window controller.** `CloseRequest.Close` is sufficient to complete the decision that created it. A public handle for arbitrary programmatic window control would be a larger lifecycle API with no demonstrated need here.

**Built-in page or daemon transport.** dfw remains a window around a product-owned HTTP service, not a JavaScript bridge or IPC framework. Close veto creates no exception to that boundary.

**Timeouts and repeated-close escalation.** Neither is an escape. Both can convert delay or an accidental repeated click into lost work. A pending request resolves only through the product; operating-system process controls remain available when the product itself is stuck.

**Product save, discard, and prompt semantics.** The motivating products differ in what they hold and how they make it durable. dfw does not define dirty state, force a flush, render a dialog, or decide what discard means.
