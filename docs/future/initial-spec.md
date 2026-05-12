# dfw: Desktop Application Framework for Go

**Repository**: `github.com/michaelquigley/dfw`
**License**: Apache 2.0
**Status**: Specification — v1 (initial release)

## Overview

`dfw` is a small Go library for wrapping HTTP servers with embedded web UIs into native desktop applications. It is a sibling to `dfx` (imgui-based desktop UI) in the same toolkit family — both extend the `df` core with desktop application capabilities, differing in their UI rendering technology.

`dfw` provides the minimal infrastructure to:

- Wrap a Go HTTP server in a native window (single-window mode)
- Run an HTTP server as a tray-resident daemon with separate window clients (daemon mode)
- Open windows that connect to a remote (typically localhost) HTTP server

It does **not** provide a UI framework, a JS↔Go bridge, or build/distribution tooling. The contract between the window and the server is the product's HTTP API surface (typically OpenAPI-backed); `dfw` does not route, proxy, or transform requests.

As a `df` add-on, `dfw` uses companion `github.com/michaelquigley/df` packages for shared infrastructure: `dd` for all data binding/unbinding and `dl` for logging when logging is needed.

## Goals

- Cross-platform: macOS, Windows, Linux
- Minimal API surface (three entry points)
- Two operational modes (single-window; tray daemon + windows)
- Apache 2.0 licensed, with permissively-licensed dependencies
- Single-binary builds preserved where the product wants them
- Reusable across multiple products
- No build or distribution opinions; products own their packaging

## Non-Goals (v1)

Explicit non-goals, by design:

- **Native menu bar API.** The selected webview binding's menu support is thin; building proper native menus per platform would push the design toward Wails.
- **Native file dialogs.** Same reasoning. Products call platform-specific libraries directly when needed.
- **Multi-window in a single process.** Native event loops are single-threaded on every OS; the selected webview binding reflects this. Multiple windows are achieved via multiple `Window` processes.
- **JS↔Go bridge.** The contract is the product's HTTP API.
- **HTTP forwarding or proxy between processes.** Windows' webviews connect directly to the daemon's HTTP server.
- **Single-instance enforcement.** Products implement this themselves if they need it.
- **Distribution helpers** (`.app` bundling, `.exe` packaging, AppImage). Per-product concern.
- **Auto-update, code signing infrastructure, login-item integration.** Future work.

## Architecture

### Operational Modes

`dfw` supports two operational modes. Products choose based on their needs.

**Single-window mode** (`dfw.Run`): one process owns the HTTP server, any background jobs, and the window. Closing the window terminates the process. Suitable for apps with no background work that should persist beyond the window.

**Daemon mode** (`dfw.Daemon` + `dfw.Window`): one daemon process owns the HTTP server and background jobs and exposes a tray icon; one or more separate window processes connect to the daemon's server via webview. Suitable for apps with background work or multi-window needs.

### Process Model

Single-window mode:

```
┌────────────────────────────────────┐
│  product binary (dfw.Run)          │
│  ├─ http.Server on 127.0.0.1:0     │
│  ├─ background work (in-process)   │
│  └─ webview window (main thread)   │
└────────────────────────────────────┘
```

Daemon mode:

```
┌────────────────────────────────────┐
│  daemon binary (dfw.Daemon)        │
│  ├─ http.Server on 127.0.0.1:0     │
│  ├─ background work (in-process)   │
│  ├─ tray icon + menu               │
│  └─ writes runtime/daemon.json     │
└─────────────┬──────────────────────┘
              │ HTTP (localhost)
       ┌──────┼──────┐
       ↓      ↓      ↓
   window  window  window      ← each a separate process (dfw.Window)
```

### Discovery

In daemon mode, the daemon writes a runtime file describing how to connect to it:

```
{user_config_dir}/{AppID}/runtime/daemon.json
```

Contents:

```json
{
  "pid": 12345,
  "address": "127.0.0.1:53291"
}
```

`{user_config_dir}` is resolved via `os.UserConfigDir()`. `AppID` is the reverse-DNS identifier (e.g. `com.quigley.flo`).

Window processes discover the daemon by:

1. Checking the `DFW_DAEMON_ADDR` environment variable. If set, use it directly.
2. Falling back to reading the runtime file at the AppID-derived path.
3. Returning an error if neither is available.

The daemon writes the file on startup and removes it on clean shutdown. `dfw` does **not** detect stale files or already-running daemons; products that need single-instance enforcement implement it themselves. A runtime-file address is not liveness-checked: if the daemon died ungracefully (panic, signal, crash) the file remains, the next `Window` reads it, navigates to that address, and the webview surfaces the connection failure normally. Products may probe the daemon themselves before opening a window if a cleaner UX is needed.

Version handshakes between daemon and window are out of scope for v1. An in-place upgrade across daemon and window binaries will surface as whatever the HTTP API surface does — a route mismatch, a 4xx, or a working connection if the surface is unchanged. Products that need stricter coordination implement it on top of their own API.

### Lifecycle

**Single-window mode (`dfw.Run`)**:

1. Call `app.Listen()` to obtain server and listener.
2. Start serving in a goroutine.
3. Create webview at the persisted window bounds when available, otherwise `InitialSize`; navigate to the listener's address.
4. Block on the webview event loop until the window closes.
5. Shut down the HTTP server gracefully.
6. Return.

**Daemon mode (`dfw.Daemon`)**:

1. Call `app.Listen()` to obtain server and listener.
2. Start serving in a goroutine.
3. Write runtime file with PID + address.
4. Initialize tray icon. Menu order: "Open Window" (only if `SpawnWindow != nil`), then any `TrayItems` in declared order, then a separator and "Quit" last.
5. Block on the tray event loop until "Quit" is selected.
6. Remove the runtime file.
7. Shut down the HTTP server gracefully.
8. Return.

When the daemon shuts down, any open window processes will lose their HTTP connection. The window's React UI is responsible for surfacing this state to the user; `dfw` does not coordinate window shutdown from the daemon side.

**Window mode (`dfw.Window`)**:

1. Resolve the daemon address (env var, then runtime file).
2. Create webview at the persisted window bounds when available, otherwise `InitialSize`; navigate to the resolved address.
3. Block on the webview event loop until the window closes.
4. Return.

## Public API

### Types

```go
package dfw

type App struct {
    AppID       string                                       // reverse DNS, e.g. "com.quigley.flo"
    Title       string                                       // display name
    InitialSize image.Point                                  // window size at startup
    IconPNG     []byte                                       // window/app icon as PNG bytes
    Listen      func() (*http.Server, net.Listener, error)
}

type DaemonApp struct {
    AppID       string
    Title       string                                       // displayed as tray tooltip
    IconPNG     []byte                                       // used for tray icon
    Listen      func() (*http.Server, net.Listener, error)
    SpawnWindow func(daemonAddr string) error                // called when "Open Window" tray item is clicked; if nil, item is omitted
    TrayItems   []TrayMenuItem                               // inserted between "Open Window" and "Quit"
}

type WindowApp struct {
    AppID       string
    Title       string
    InitialSize image.Point
    IconPNG     []byte
}

type TrayMenuItem struct {
    Label    string
    Tooltip  string
    OnClick  func()
    Disabled bool                                            // read once at menu build time
}
```

### Listen contract

`Listen` is the single seam between the product and `dfw` for the HTTP server.

- Returns an **unstarted** `*http.Server` (handlers wired, not yet serving) and an **open** `net.Listener`. The typical pattern is `net.Listen("tcp", "127.0.0.1:0")` to let the OS pick an ephemeral port.
- The listener's `Addr()` must be navigable as `"http://" + listener.Addr().String()`. v1 supports TCP listeners only; unix sockets and other transports are out of scope.
- After `Listen` returns successfully, `dfw` owns `server.Serve(listener)`, `server.Shutdown(ctx)`, and the listener's `Close` (`Shutdown` handles the close). Products **must not** call `Serve`, `Shutdown`, or `Close` themselves.
- Any error returned from `Serve` other than `http.ErrServerClosed` before `dfw` initiates shutdown is fatal: `Run` / `Daemon` capture it and return it as the entry-point's error.
- `Listen` itself returning an error is also fatal and surfaces directly through the entry point.
- **Cleanup invariant.** Once `Serve` has been started, every return path from `Run` / `Daemon` must shut the server down and wait for the Serve goroutine to exit before returning — even if a later setup step (webview/tray initialization, icon decoding, `daemon.json` write, etc.) fails. In daemon mode, once `daemon.json` has been written successfully, every non-crash return path must best-effort remove it. `defer` is the idiomatic way to express both.

### Entry points

```go
// Run starts a single-window application. This process owns the HTTP server,
// any background work, and the window. Returns when the window is closed or
// on fatal error.
func Run(app App) error

// Daemon starts a tray-resident daemon. This process owns the HTTP server
// and any background work, exposes a tray icon, and writes a runtime file
// for window discovery. Returns when the daemon is shut down via the tray
// menu or on fatal error.
func Daemon(app DaemonApp) error

// Window opens a single webview window pointing at a remote HTTP server.
// Discovers the server address via the DFW_DAEMON_ADDR environment variable
// or by reading the runtime file at the AppID-derived path. Returns when
// the window is closed or on fatal error.
func Window(app WindowApp) error
```

### Spawn helpers

For products that want to spawn window processes from the daemon's "Open Window" tray item:

```go
// SpawnSelf returns a SpawnWindow function that launches the current binary
// with the given subcommand args and DFW_DAEMON_ADDR set in the environment.
func SpawnSelf(args ...string) func(daemonAddr string) error

// Spawn returns a SpawnWindow function that launches the named binary with
// the given args and DFW_DAEMON_ADDR set in the environment.
func Spawn(binary string, args ...string) func(daemonAddr string) error
```

Typical use:

```go
// Single-binary product with subcommand routing:
DaemonApp{
    SpawnWindow: dfw.SpawnSelf("window"),
}

// Multi-binary product (e.g. flo pattern):
DaemonApp{
    SpawnWindow: dfw.Spawn("flo-window"),
}
```

**Error handling on tray click.** When the user clicks "Open Window", `dfw` calls `SpawnWindow(resolvedAddr)`. If it returns an error, `dfw` logs the error through `github.com/michaelquigley/df/dl` and continues — the daemon keeps running, the tray loop keeps running. Spawn errors are non-fatal to `Daemon`. The common failure mode is a deployment problem (wrong binary name, wrong argv) rather than a state problem in the daemon, so tearing down the tray would be hostile. Products that need visible failure reporting wrap their own `SpawnWindow` and surface the error through their UI, HTTP API, or log.

### DevTools

A `--devtools` command-line flag (or `DFW_DEVTOOLS=1` environment variable) enables the webview's developer tools inspector. Applies to `Run` and `Window` modes. `dfw` exposes a small helper for products that use cobra or `flag`:

```go
// DevToolsEnabled returns true if the dev tools inspector should be enabled,
// based on the DFW_DEVTOOLS env var. Products may also expose a CLI flag
// that sets the env var or otherwise toggles this.
func DevToolsEnabled() bool
```

### Window state

Window state persistence is automatic for `Run` and `Window` when `AppID` is set. `dfw` stores the latest known window bounds at:

```
{user_config_dir}/{AppID}/runtime/window_state.json
```

The file stores `width` and `height` everywhere. It stores `x` and `y` only when the platform can report useful screen coordinates. Windows supports size and location. Linux supports size everywhere; location is best-effort and currently X11-only because Wayland does not expose reliable application-controlled top-level window coordinates.

If the file is missing, malformed, or contains an invalid size, `dfw` falls back to `InitialSize`. Window-state read/write failures are non-fatal and are logged through `dl`.

## Internal Structure

```
dfw/
├── LICENSE                     # Apache 2.0
├── NOTICE                      # Dependency attributions
├── README.md
├── go.mod
├── go.sum
├── doc.go                      # Package overview
├── app.go                      # App, DaemonApp, WindowApp, TrayMenuItem types
├── run.go                      # Run() implementation
├── daemon.go                   # Daemon() implementation
├── window.go                   # Window() implementation
├── spawn.go                    # Spawn, SpawnSelf helpers
├── discovery.go                # Runtime file read/write, env var resolution
├── devtools.go                 # DevToolsEnabled and friends
├── webview.go                  # selected webview binding wrapper
├── tray.go                     # fyne.io/systray wrapper
├── icon.go                     # Cross-platform icon helpers
├── icon_darwin.go              # macOS-specific
├── icon_linux.go               # Linux-specific
├── icon_windows.go             # Windows-specific
└── examples/
    └── dfw-example-watch/
        └── ...
```

Platform-specific code uses build tags (`//go:build darwin` etc.) where native API calls differ. Cross-platform code lives at the package root.

## Platform Support

> **MVP scope.** This spec describes the full v1 target across macOS, Windows, and Linux. The accompanying work order at `docs/future/initial-work-order.md` narrows the initial implementation to Windows + Linux; macOS is deferred. Anything in this section labelled "macOS" is informational for the broader target, not a v1 commitment.

### Build requirements

| Platform | Toolchain          | System packages                                          |
|----------|--------------------|----------------------------------------------------------|
| macOS    | Xcode CLT (clang)  | none beyond OS                                           |
| Windows  | MinGW-w64 or MSVC  | none beyond OS                                           |
| Linux    | gcc + pkg-config   | `libwebkit2gtk-4.1-dev`, `libgtk-3-dev` (Debian/Ubuntu)  |

### Runtime requirements

| Platform | Webview engine     | Notes                                                  |
|----------|--------------------|--------------------------------------------------------|
| macOS    | WKWebView (system) | No install required                                    |
| Windows  | WebView2 (system)  | Ships with Windows 11; evergreen bootstrapper on Win10 |
| Linux    | WebKitGTK 4.1      | Install via `libwebkit2gtk-4.1-0`                      |

### Windows subsystem

Binaries that use `dfw.Run` or `dfw.Window` should be built with the GUI subsystem to avoid a background console window:

```
go build -ldflags "-H windowsgui" ./cmd/<binary>
```

Binaries that use `dfw.Daemon` (no window in this process) can choose either subsystem. CONSOLE is more useful for diagnostic output but shows a console window when launched from a shortcut.

**Single-binary subcommand layouts.** Products that ship one binary covering multiple `dfw` entry points via subcommands (like `dfw-example-watch run`/`daemon`/`window`) cannot pick a subsystem per subcommand — the subsystem is a property of the linked Windows binary, not a runtime code path. The two viable choices are:

1. Build the single binary with `-H windowsgui`. No console flashes for `run`/`window`. `daemon` has no usable stdout/stderr; diagnostics go to a log file. Recommended for shipping builds.
2. Build the single binary with the console subsystem. `daemon` prints normally, but `run` and `window` flash a console window when the webview opens. Useful at dev time when you want `daemon` stdout in your terminal.

### Cross-compilation

CGO requires a target-platform toolchain. Pure-Go cross-compilation does not work for `dfw`-based binaries. Each platform must be built on a runner with the appropriate toolchain. CI matrix should include macOS, Windows, and Linux runners.

## Demonstration: `dfw-example-watch`

A filesystem watcher demonstrating both operational modes from a single binary via subcommands.

**Location**: `github.com/michaelquigley/dfw/examples/dfw-example-watch`

**Subcommands**:

- `dfw-example-watch run <path>` — single-window mode (`dfw.Run`). Watches the path, displays recent events in a window. Process exits when window closes.
- `dfw-example-watch daemon <path>` — tray daemon mode (`dfw.Daemon`). Watches the path. Tray icon and tooltip are static (the product name); dynamic state — including the running event count — is exposed via the HTTP API and shown in spawned windows, not in the tray. "Open Window" tray menu item spawns a `dfw-example-watch window` child process via `dfw.SpawnSelf("window")`.
- `dfw-example-watch window` — window client (`dfw.Window`). Discovers the daemon via `DFW_DAEMON_ADDR` env var or runtime file. Opens a window showing the event timeline.

**What the example demonstrates**:

- Both `dfw.Run` and `dfw.Daemon` + `dfw.Window` patterns from one source tree
- Spawning window child processes from a daemon
- A simple OpenAPI-shaped server with embedded React UI
- Background work (`fsnotify` on the watched path) that persists across window lifecycles in daemon mode
- A product choosing the single-binary-with-subcommands layout (alternative to multi-binary)

## Build and Distribution

`dfw` v1 does not include distribution helpers. Each product owns its own packaging.

Recommended patterns (documented in product repos, not provided by `dfw`):

- **macOS**: shell script that creates `.app` bundle with `Info.plist` and `AppIcon.icns`; `gon` or `notarytool` for signing/notarization when needed
- **Windows**: `goversioninfo` or `rsrc` to embed icon and manifest into the `.exe`; signtool for signing when needed
- **Linux**: raw binary + `.desktop` file, optionally wrapped in AppImage

These patterns may move into a `dfw/dist` subpackage in a future version if demand justifies it.

## Testing

### Unit tests

Non-UI logic — discovery, runtime file format, AppID path resolution, spawn helpers — covered by standard Go tests. CI runs these on every platform.

### Integration tests

Smoke tests for window and tray creation are gated by build tag `dfwtest` and skipped by default. They require a display (X server / Wayland / macOS GUI / Windows GUI) and run on platform-specific CI runners with displays available.

### Manual smoke tests (per release)

Run `dfw-example-watch` in all three modes on all three platforms. Verify:

- Window appears with correct title and icon
- Tray icon appears with correct tooltip
- Tray menu items work ("Open Window" spawns a window; "Quit" exits cleanly)
- Daemon → window discovery succeeds via both env var and runtime file
- Devtools open when `--devtools` is set
- Clean shutdown removes runtime file

## License and Attribution

`dfw` is licensed under Apache License 2.0.

Dependencies and their licenses:

| Dependency           | License      | Linkage                           |
|----------------------|--------------|-----------------------------------|
| github.com/michaelquigley/df | MIT | Static (Go module)                |
| centrifuge.hectabit.org/HectaBit/webview_go | MIT | Static (Go module) |
| fyne.io/systray      | Apache-2.0   | Static (Go module)                |
| WebKitGTK (Linux)    | LGPL 2.1+    | Dynamic via system shared library |
| WebView2 (Windows)   | Microsoft    | Dynamic via system runtime        |
| WKWebView (macOS)    | Apple        | System framework                  |

The `NOTICE` file forwards attribution per Apache 2.0 §4(d) for `fyne.io/systray`.

WebKitGTK is used via dynamic linking against the system-installed shared library — the permitted "work that uses the library" pattern under LGPL. `dfw` does not include WebKitGTK source.

## Deferred / Future Work

Items explicitly out of scope for v1, captured for future consideration:

- **Distribution helpers**: per-platform packaging in a `dfw/dist` subpackage
- **Auto-update**: integration with `go-update` or similar; possible self-hosted depot pattern
- **Code signing helpers**: macOS notarization, Windows signtool workflows
- **Login-item integration**: per-platform auto-start at login
- **Native menu bar API**: would push toward Wails for parity; reconsider if requirements demand it
- **Native file dialog API**: same reasoning
- **CLI ↔ daemon HTTP forwarding**: `dfw` doesn't proxy; products implement if needed
- **Single-instance enforcement helpers**: products implement themselves
- **Additional window state**: maximized/fullscreen state, per-window-kind identities, and stricter multi-monitor restore policy are deferred. v1 persists one shared size/location record per `AppID`.
- **Dynamic tray menu state (checked, disabled, label, tooltip)**: v1 reads `TrayMenuItem` fields once when the tray menu is built; `dfw` does not watch or rebuild the menu in response to product state changes. When a real use case appears, this gets designed as an explicit API (e.g. a `Tray.SetItem(idx, TrayMenuItem)` rebuild call or per-item handles) with cross-platform behavior pinned for both Windows and Linux. The omitted `TrayMenuItem.Checked *bool` from earlier drafts belongs here.
- **Crash reporting**
- **Accessibility-specific work** (OS webviews handle the common case)
- **i18n** (product concern)

## Decisions Made

This section captures decisions whose rationale should outlive the discussion that produced them.

- **`webview_go`-style binding over Wails**: minimal abstraction, preserves vanilla `go build`, sufficient for the consume-the-HTTP-boundary pattern. The initial implementation uses a WebKitGTK 4.1-compatible Go binding because Fedora 43 no longer provides the older WebKitGTK 4.0 pkg-config target. Wails remains a graduation path if native menus, dialogs, or in-process multi-window become hard requirements.
- **`fyne.io/systray` over `getlantern/systray`**: cleaner API ergonomics, active maintenance lineage. Either would work; the choice is reversible.
- **Reverse-DNS `AppID`**: matches macOS Bundle ID convention, avoids collisions in user config directories across vendors.
- **Three entry points instead of a unified `Run(opts)` with a mode enum**: clearer at the call site; each function takes only the fields relevant to its mode.
- **Daemon shutdown handled at OS level**: simplest model; products coordinate via their own React UI state if richer disconnect handling is needed.
- **Demo at `examples/dfw-example-watch` with subcommands**: demonstrates that products can choose their binary layout (single-binary subcommand-driven, multi-binary, etc.) without `dfw` caring.
- **Multi-window via multi-process**: native event loop constraints make in-process multi-window infeasible without a heavier framework; multi-process is aligned with how the daemon mode already works.
- **Apache 2.0 license**: permissive, compatible with all chosen dependencies, suitable for sharing across the toolkit family and potentially externally.
