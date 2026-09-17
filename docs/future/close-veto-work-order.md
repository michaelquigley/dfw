# Close veto — work order

This work order translates [the close-veto spec](close-veto.md) into the existing `webview` package. The spec owns the behavior: one asynchronously delivered request, one pending request at a time, no timeout or repeated-close escalation, and explicit `Close` or `KeepOpen` resolution. This document owns where that behavior lands and how each stage proves it.

## Where the spec meets the code

`App` and `WindowApp` are configuration values copied into a private `webviewConfig`, then consumed by `newConfiguredDesktopWebView`. Both entry points already converge there, so the callback needs no duplicated lifecycle implementation: the two public fields feed one private window controller.

The blocking sequence is also already in the right shape. `Run` and `Window` call `window.Run()`, then `SaveWindowState()`, with `Destroy()` deferred. A veto only has to keep the native event loop running. Resolving `Close` terminates that loop; state capture and destruction stay exactly where they are. A request must be expired as soon as the loop returns, before window-state persistence and native teardown, so a late product answer cannot touch a window on its way out.

Linux has independent GTK signal handlers for bounds and zoom, so close interception can be another native controller with its own lifetime. The pinned webview binding connects only to `destroy`; it does not consume GTK's `delete-event`. A dfw handler can therefore return `TRUE` while the Go request is pending and `FALSE` once close has been granted.

Windows already subclasses the HWND procedure in `window_bounds_windows.go`, and that procedure already sees `WM_CLOSE`. Do not install a second subclass for close veto: two chained dfw procedures would make restoration order part of correctness. Extend the existing tracker to hold the optional close handler, consume `WM_CLOSE` when that handler says the request is pending, and otherwise forward to the original procedure.

One pinned-dependency detail is load-bearing. The Go binding says `Terminate` is safe from a background thread, but its Windows backend implements termination with `PostQuitMessage`, which must run on the UI thread whose message loop is being stopped. The new callback is deliberately dispatched in a goroutine, and the existing server supervisor also terminates from a goroutine. `desktopWebView.Terminate` must therefore marshal the underlying call through `WebView.Dispatch`; it must also stop accepting termination work once `Run` has returned.

The current supervisor callback has another race: it drops a server failure when the failure arrives before `Run` publishes the constructed window. Replace that atomic-pointer handoff with a termination coordinator created before `SuperviseServe`. A request made before the window is bound or before its UI loop is ready remains latched. Once construction succeeds, a request already latched prevents entry into the native run loop; a request racing with startup is consumed by the first UI-thread-ready callback and terminates there. Only after that callback has established readiness may later requests use ordinary `Dispatch`. This makes early supervisor failure sticky, fixes the internal termination path, and makes `CloseRequest.Close` honest on Windows.

## Public API

Add this field to both public application types in `webview/app.go`:

```go
// OnCloseRequest receives a native close request before the window is
// destroyed. The callback runs asynchronously; resolve the request with
// Close or KeepOpen. A nil callback preserves the native close behavior.
OnCloseRequest func(*CloseRequest)
```

`CloseRequest` is an exported concrete type in a new `webview/close_request.go`. Its fields remain private. `Close()` and `KeepOpen()` return nothing, tolerate a nil or zero-value receiver, are safe from any goroutine, and become no-ops after the first resolution, after a later request supersedes their generation, or after the window lifecycle ends.

Do not add a decision enum, callback return value, timeout, context, general window handle, or capability query. Unsupported opt-in is reported as an entry-point error; absence of the callback is the compatibility path.

## Shared request controller

The platform-neutral controller in `close_request.go` owns all behavioral state. Keep the native adapters mechanical.

The controller holds a mutex, the callback, a monotonically increasing generation, the currently pending generation, an ended/closing state, and an injected terminate function. Its native-facing request method returns whether the native event must be consumed.

When idle, a native request allocates the next generation, records it as pending, creates the public `CloseRequest`, launches the callback in a goroutine, and returns `true`. When another native request arrives with a generation pending, it returns `true` without allocating, launching, or queuing anything. When the controller is closing or ended, it returns `false`, allowing a native close already backed by consent to continue.

`KeepOpen` clears the pending generation only when the handle's generation still matches, returning the controller to idle. `Close` makes the controller closing, clears the pending generation, unlocks, and then invokes the injected terminate function; calling external code outside the mutex prevents re-entrancy from becoming a deadlock. Ending the controller expires the pending generation and makes every retained handle inert. The generation check is mandatory: request A must not be able to close request B after A has already kept the window open.

Tests should drive this controller directly, without a native window. Cover:

- the native request returns promptly while a deliberately blocked callback runs elsewhere;
- a callback is dispatched exactly once for a pending request, with repeated native requests consumed and dropped;
- returning from the callback without resolving leaves the request pending;
- `KeepOpen` rearms the controller and the next native request receives a different handle;
- a stale handle from the previous generation cannot close or keep open the new request;
- `Close` calls terminate once and later native requests are allowed rather than redispatched;
- the first of repeated or competing resolutions wins;
- ending the lifecycle expires a pending request and makes late resolution harmless.

Use channels and bounded test waits only to coordinate goroutines; do not add production timers to make the tests convenient. Run these tests under the race detector as part of the stage verification.

## Termination coordinator tests

Keep the termination coordinator independent of the native binding by injecting its dispatch and terminate operations. Tests use a fake dispatcher that captures scheduled functions and lets the test choose when each function executes; they must not depend on timing, a real window, or a platform message loop.

Cover the full coordinator lifecycle:

- a request before the window is bound remains latched and prevents native `Run` from being entered;
- a request after bind but before UI readiness schedules no premature native work and is terminated exactly once by the ready callback;
- a request while ready dispatches once and terminates on execution of the captured callback;
- a request racing with run-end invalidation either owns already accepted work or is ignored, and no captured callback calls the native terminate operation after end;
- requests after `Run` ends or `Destroy` begins neither dispatch nor terminate;
- repeated and concurrent requests still result in at most one native termination.

Also race the close controller's lifecycle end against both `Close` and `KeepOpen`, using barriers to exercise both lock orderings and `go test -race` to check the surrounding state. These tests complement the controller's sequential late-resolution cases: they enforce the boundary that keeps retained requests and supervisor callbacks away from a stopped or destroyed webview.

## Desktop lifecycle integration

Extend `webviewConfig` with the callback and pass it through from both `Run` and `Window`. Add the shared controller and the installed native interceptor to `desktopWebView`.

When the callback is non-nil, `newConfiguredDesktopWebView` creates the shared controller and installs the platform interceptor against `w.Window()`. An installation error fails construction and cleans up every controller already created. When the callback is nil, no close controller or native handler is installed.

Create the supervisor termination coordinator before starting `SuperviseServe`, and give its request function directly to the supervisor callback. Construction binds the resulting `desktopWebView` to the coordinator without declaring its dispatcher ready. If termination was requested before that bind, `Run` observes the latch and returns without entering the native loop. To close the remaining startup race, `Run` arranges a bootstrap callback on the UI thread and does not declare dispatch readiness until that callback executes; the callback immediately performs any termination that arrived after the pre-run check. Requests made after readiness marshal termination through the ordinary dispatch path. Construction failure ends the coordinator, and the deferred supervisor shutdown remains responsible for reporting the server error.

`desktopWebView.Run` marks the close controller and termination coordinator ended immediately after the underlying `Run` returns. `Destroy` repeats that end operation idempotently, detaches the close interceptor before the bounds/zoom controllers, and only then destroys the underlying webview. This order prevents GTK cleanup or `WM_CLOSE`/destroy traffic from reaching application code.

Change `desktopWebView.Terminate` so background callers request termination through that coordinator. While the UI loop is ready, the coordinator schedules the underlying `w.Terminate()` through `w.Dispatch(...)`; before readiness it latches the request, and after end it ignores it. Guard the ready-to-ended transition so a server failure racing after `Run` has returned does not enqueue work against a stopped or destroyed webview. Both `CloseRequest.Close` and the existing `Run` supervisor use this same coordinator; neither gains a direct native shortcut.

## Native adapters

Add a small internal interface, owned by the `webview` package, whose only lifecycle operation is `Close()`. Its installer accepts the native window and the shared controller's native-request function, returning an error when the contract cannot be installed.

### Linux

Implement the GTK adapter in new `close_request_linux.go`, `close_request_linux.c`, and `close_request_linux.h` files rather than adding more C to `window_bounds_linux.go`. The Go file exports the callback into C and therefore must keep its cgo preamble declaration-only; the C file owns the signal callback and allocation.

Use `runtime/cgo.Handle` to carry the Go request function through GTK's `gpointer` without storing a Go pointer in C. The C controller records the `GtkWindow`, the `delete-event` handler id, and a `destroy` handler that invalidates the window pointer. Its delete handler calls Go and returns `TRUE` when the event is consumed, `FALSE` when the shared controller is already closing or ended. Installation treats allocation failure or a zero signal id as an error and unwinds any partial handler. Teardown disconnects the GTK handlers before deleting the cgo handle.

The Linux adapter must remain independent of the bounds and zoom controllers. `desktopWebView.Destroy` provides the only ordering between them.

### Windows

Add the Windows installer in `close_request_windows.go`, but reuse the `windowsWindowBoundsTracker` and its single installed window procedure. The installer looks up the tracker for the HWND, verifies that subclassing succeeded (`oldProc` is nonzero), and attaches the request function under the tracker's mutex. Failure to find a working tracker is a close-interceptor installation error, even though bounds tracking itself currently degrades quietly.

On `WM_CLOSE`, capture the final bounds as today, copy the optional request function while holding the mutex, then call it without the mutex. A `true` result consumes the message by returning zero. A `false` result continues to the original window procedure, which performs the native close. `WM_DESTROY` and `WM_NCDESTROY` retain their current capture and cleanup behavior. The native close interceptor's `Close` clears only the optional request function; the bounds tracker remains the owner of restoring the original window procedure.

### Unsupported platforms

Add `close_request_unsupported.go` with the `!linux && !windows` build constraint. A non-nil callback reaches an installer that returns an error naming the unsupported platform. Do not provide a no-op interceptor: configured close protection must never degrade to the ordinary hard close.

## Critical files

| File | Change |
| --- | --- |
| `webview/app.go` | Add the documented `OnCloseRequest` field to `App` and `WindowApp`. |
| `webview/run.go` | Pass the callback into the shared window configuration and replace the lossy atomic window pointer with the sticky supervisor-termination coordinator. |
| `webview/window.go` | Pass the callback into the shared window configuration. |
| `webview/webview.go` | Own the controller/interceptor, construction cleanup, run-end invalidation, teardown order, and UI-thread termination dispatch. |
| `webview/close_request.go` | Define `CloseRequest`, the single-flight generation state, and the native interceptor interface. |
| `webview/close_request_test.go` | Prove asynchronous dispatch, coalescing, resolution, generation isolation, end-of-life behavior, and concurrency safety. |
| `webview/termination.go` | Implement the sticky bind/readiness/end coordinator over injected dispatch and terminate operations. |
| `webview/termination_test.go` | Deterministically prove startup handoff, live dispatch, and run-end/destruction races without a native window. |
| `webview/close_request_linux.go` / `.c` / `.h` | Bridge GTK `delete-event` to the shared Go controller with a cgo handle. |
| `webview/close_request_windows.go` | Attach close interception to the existing HWND tracker without another subclass. |
| `webview/window_bounds_windows.go` | Consult the optional handler on `WM_CLOSE` while preserving bounds capture and original-procedure forwarding. |
| `webview/close_request_unsupported.go` | Fail configured close protection outside Linux and Windows. |
| `examples/dfw-example-close/main.go` | Provide the cross-platform manual fixture for pending, dropped, kept-open, and accepted close requests. |
| `examples/dfw-example-close/README.md` | State the Linux/Windows exercise and the expected observations. |
| `docs/current/architecture.md` | Describe the callback on both entry points and keep product transport above the HTTP boundary. |
| `docs/current/runtime.md` | Document pending-request behavior, repeated-close dropping, internal termination, and platform support. |
| `docs/current/example.md` | Point from the topology-oriented watch example to the focused close-lifecycle fixture. |
| `README.md` | Add the close-request capability and point to the close example without turning the README into the full contract. |

Do not add close endpoints or UI to `dfw-example-watch`. Keep that reference product about the three process topologies it already demonstrates. Add a separate, deliberately small `examples/dfw-example-close` application whose only purpose is exercising this native lifecycle on Linux and Windows.

The example is a single Go program plus a short README, with no frontend toolchain or generated assets. It serves inline HTML from its `Run` listener and holds the current `*webview.CloseRequest` behind a mutex. The callback records the handle and increments a delivered-request count. The page polls a small status endpoint showing `idle` or `pending` and the count, and exposes two buttons while a request is pending: `Keep open` and `Close`. Their POST handlers remove the current handle from the example state, then resolve it outside the mutex.

The fixture must not resolve automatically or use a timeout. Its manual sequence is the proof:

1. Start the example and use the native title-bar close control. The window remains open, the page shows one pending request, and the delivered count becomes one.
2. Use the native close control several more times while pending. The count remains one.
3. Choose `Keep open`. The page returns to idle and the window remains.
4. Use the native close control again. A second pending request appears and the delivered count becomes two.
5. Choose `Close`. The entry point returns through the ordinary save-and-destroy path and the process exits.

This is application-owned HTTP transport on purpose: it demonstrates how a product can carry the request without adding transport to dfw. Keep it generic and mechanical; it is not an unsaved-document prompt and should not acquire save, discard, recovery, or gloss concepts.

## Stages

### Stage 1 — shared contract and Linux path

Implement `CloseRequest` and its fully tested single-flight controller. Add the public fields and private configuration flow, replace the supervisor's atomic-pointer handoff with the sticky termination coordinator, integrate controller lifetime and dispatched termination into `desktopWebView`, and add the native interface, Linux GTK implementation, and temporary unsupported implementation for every non-Linux platform. Add `dfw-example-close` with the status page and explicit resolution controls. At the end of the stage, nil callbacks behave byte-for-byte as before, an early supervisor failure cannot be lost, Linux opt-in works end to end, and other platforms fail the opt-in explicitly rather than pretending to protect the window.

Verify with `gofmt`, focused package tests, `go test -race ./webview -count=1`, and the full `make test` gate. The focused tests must include coordinator termination before readiness, while running, concurrent with run-end invalidation, and after end or destruction, plus close-controller lifecycle races against `Close` and `KeepOpen`. Run `dfw-example-close` on Linux and complete the README sequence; the request count proves repeated native closes are dropped, while the two resolution buttons prove rearming and accepted close through the real GTK window. Generation isolation remains a unit-test responsibility rather than a control added to the example. Run Terminus over the stage and bring it to `clean` before handoff.

### Stage 2 — Windows parity, documentation, and closeout

Add the Windows adapter by extending the existing bounds tracker/window procedure, narrow the unsupported build constraint to platforms other than Linux and Windows, and complete the current documentation and README. The close example remains unchanged: the same source and README sequence are the cross-platform fixture.

Run the shared tests and full gate again on Linux. Build and run `dfw-example-close` against a real Windows WebView2 window and complete the same README sequence used on Linux. Separately prove that supervisor-driven termination still exits from its background goroutine; the deterministic coordinator suite remains the proof for early failure and concurrent `Run` return, while the Windows exercise proves the real backend dispatch. A configured callback must fail on an unsupported build; a nil callback must retain ordinary close behavior everywhere the webview already builds. Run Terminus over the complete implementation and bring it to `clean`.

## Closeout

Once both stages are accepted, synthesize the shipped behavior into `docs/current/architecture.md`, `docs/current/runtime.md`, `docs/current/example.md`, and the README as named above. Repair the runtime document's existing claim that `dfw-example-watch` exercises every feature: the watch example remains the topology reference, while `dfw-example-close` is the native close-lifecycle fixture. No changelog exists in this repository, so this work does not create one as a side effect.

The roadmap card moves to `evaluating` for product adoption and soak. Replace its log pointer to this spec with a pointer into the current documentation before removing `docs/future/close-veto.md` and this work order; the realized intent documents then live in git history. Any unverified native-platform check is recorded as an accepted residual rather than silently described as proven.
