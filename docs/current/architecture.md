# Architecture

`dfw` wraps a Go HTTP server in a native desktop webview window. It does not ship a UI framework, a JavaScript-to-Go bridge, a request proxy, or any kind of distribution tooling. The product owns its HTTP API and its web UI; `dfw` only owns the process, window, and tray lifecycle around them.

The library exposes three entry points across two consumable subpackages. Each is a single function call from the product's `main`; the function blocks until the user-facing lifecycle completes.

## Package Layout

`dfw` is split so a product can consume the tray/daemon portion without pulling the webview's native dependency:

- **`github.com/michaelquigley/dfw/tray`** — the tray-resident daemon (`tray.Daemon`, plus `tray.SpawnSelf` / `tray.Spawn`). Depends on `fyne.io/systray` only.
- **`github.com/michaelquigley/dfw/webview`** — the desktop window entry points (`webview.Run`, `webview.Window`, and `webview.DevToolsEnabled`). Depends on the CGO webview binding (WebKitGTK / WebView2).
- **`github.com/michaelquigley/dfw/internal/pdf`** — the private Chromium process, bounded CDP pipe, resource checks, and PDF byte stream. Used by webview's opt-in PDF capability; no native window or product data model, and no tray dependency.
- **`github.com/michaelquigley/dfw/internal/core`** — shared HTTP-server supervision, daemon discovery, icon decoding, and window-state persistence. Pure Go, no CGO; imported by both subpackages, not by consumers.

Because Go resolves dependencies per package, the `tray` package's transitive imports never include the webview binding. A consumer that imports only `dfw/tray` therefore builds **without** WebKitGTK/WebView2 — the binding is never compiled, and Go's module-graph pruning keeps it out of the consumer's module graph entirely. The two heavy native dependencies are fully isolated: the tray build never pulls the webview binding, and the webview build never pulls `systray`.

## The Three Entry Points

### `webview.Run(App)`

One process. The HTTP server and the webview window run together, navigation points at the listener's loopback address, and the function returns when the user closes the window. Use this for a single-window desktop application where the server and window live and die together.

### `tray.Daemon(DaemonApp)`

A tray-resident process. Owns the HTTP server and any background work, writes a `daemon.json` runtime file so window clients can discover its address, and shows a system tray icon with a menu. The function returns when the daemon is shut down via the tray menu (or on fatal error). The daemon itself does not open a window — it relies on a separate `Window` process for that.

### `webview.Window(WindowApp)`

A webview-only process that connects to a running daemon. Resolves the daemon address from `DFW_DAEMON_ADDR` (falling back to the `daemon.json` runtime file), opens a webview, and navigates to it. Returns when the user closes the window. Multiple `Window` processes can connect to the same daemon concurrently.

`App.EnableZoom` and `WindowApp.EnableZoom` optionally enable native page zoom on Linux. The configured window owns its keyboard handler and releases it with the window. Its zoom preference uses the existing window-state persistence; no product routes or frontend bindings are involved. See [runtime zoom controls](runtime.md#page-zoom).

`App.OnCloseRequest` and `WindowApp.OnCloseRequest` optionally receive the window manager's close request before the native window is destroyed. dfw suppresses the native close, delivers a `*CloseRequest` on its own goroutine, and keeps the window open until the product calls `Close` or `KeepOpen`. Both entry points get the same capability because both own the same kind of native window. The decision usually lives in the product's page, and the product carries it there over its own HTTP API: `CloseRequest` is a Go value with two methods, not a transport. See [close requests](runtime.md#close-requests).

`App.PDF` and `WindowApp.PDF` bind an optional `PDFExporter` to that same window lifetime. It presents a native save chooser and renders a caller-prepared loopback document through an isolated system-Chromium process. Products still own the HTTP API, immutable document resources, destination protection, and final file write; `WindowContext()` carries cancellation through product work after rendering. See [PDF export](pdf.md). A PDF-enabled window without an `OnCloseRequest` callback also routes native close through termination so its chooser is disposed before the parent is destroyed.

## Process Topology

```mermaid
flowchart LR
    subgraph "Run mode"
        R[Run process]
        R -->|loopback HTTP| RW[webview]
        R --- RS[HTTP server]
    end

    subgraph "Daemon + Window mode"
        D[Daemon process]
        D --- DS[HTTP server]
        D --- DT[system tray]
        D -->|writes| RF[daemon.json]
        DT -.spawns.-> W1
        W1[Window process] -->|loopback HTTP| D
        W2[Window process] -->|loopback HTTP| D
        RF -.read by.-> W1
        RF -.read by.-> W2
    end
```

Window processes can be spawned by the daemon (typically via `tray.SpawnSelf`, which re-executes the daemon's own binary with the appropriate subcommand and the `DFW_DAEMON_ADDR` environment variable populated), or launched independently by the user — in which case the runtime file provides discovery.

## The HTTP Server Is The System Boundary

The product implements `app.Listen`, which returns a configured (but not yet started) `*http.Server` and an open `net.Listener`. From there, `dfw` owns the lifecycle: it calls `server.Serve(listener)` in a supervisor goroutine, shuts the server down cleanly on exit, and propagates serve failures so the window terminates rather than orphans.

This boundary is deliberate. The product owns routing, middleware, embedded static asset serving, websockets, authentication, and any other HTTP concern. `dfw` does not introspect the routes; it does not proxy requests; it does not even know whether the server is serving HTML, JSON, or both. The webview points at `http://<listener.Addr()>` and the product takes it from there.

The shape lets a product:

- Embed its compiled web bundle via `//go:embed` and serve it from a static handler.
- Add API routes under any path scheme it wants.
- Stream events to the UI via websockets or server-sent events.
- Run any background goroutines it needs alongside the server — `Listen` can start them before returning, since `dfw` doesn't take over the goroutine until `Serve()` runs.

The product remains a normal Go HTTP service; `dfw` just gives it a window.

## Non-goals

`dfw` v1 deliberately does not provide:

- A JavaScript-to-Go bridge. The web UI talks to the product's HTTP API, not to Go through a postMessage channel.
- General-purpose native menus or dialogs. The opt-in PDF destination chooser is the one file-dialog exception, earned by gloss's document-export workflow; other file pickers, modals, and context menus remain product concerns.
- Request proxying. The webview navigates to the HTTP server directly.
- Single-instance enforcement. Products that need this implement it themselves (lock files, port detection, named mutex on Windows, etc.).
- Multi-window in a single process. `Run` is one window; multi-window scenarios use `Daemon` + multiple `Window` processes.
- Transport for lifecycle decisions. A close request is delivered as a Go value; the product's page learns about it, and answers it, through the product's own API.
- Packaging or distribution tooling. Code signing, installer generation, auto-update — all out of scope.

These choices keep the library narrow enough that a product can adopt it without taking on a framework. If something on this list becomes essential to a real product, it earns inclusion through that pressure.

## Related

- [building.md](building.md) — toolchains, platform support, and how to compile against `dfw`.
- [runtime.md](runtime.md) — what's on disk, what's in the environment, and the tray menu shape.
- [example.md](example.md) — a walkthrough of `dfw-example-watch`, the reference product.
