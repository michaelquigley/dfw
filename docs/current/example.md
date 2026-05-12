# Example: dfw-example-watch

`dfw-example-watch` is the reference product for `dfw`. It watches a
directory for filesystem changes, exposes that activity over an HTTP
API, and presents the result through an embedded React UI. The same
binary supports all three `dfw` entry points as cobra subcommands, so
the example is also the canonical demonstration of how `Run`, `Daemon`,
and `Window` compose with a real product.

If you want to see how the pieces fit, read this file with the example
source open: `examples/dfw-example-watch/`.

## Layout

```
examples/dfw-example-watch/
├── main.go             # cobra root, embeds web/dist via //go:embed
├── cmd/
│   ├── common.go       # appID, icon generator, shared helpers
│   ├── run.go          # `run` subcommand -> dfw.Run
│   ├── daemon.go       # `daemon` subcommand -> dfw.Daemon
│   └── window.go       # `window` subcommand -> dfw.Window
├── server/
│   └── server.go       # HTTP handlers + dfw Listen callback
├── watcher/
│   └── watcher.go      # fsnotify recursive watcher
└── web/                # Vite + React UI (web/dist is gitignored)
```

A single binary; three subcommands. The web bundle is embedded at
compile time, so the example ships without external assets — but the
bundle has to exist before the Go build will succeed. See
[building.md](building.md) for the full build sequence.

## The Watcher

`watcher.New(root)` (`watcher/watcher.go:51`) opens an `fsnotify`
watcher rooted at `root`, walks the tree to add every existing
directory, and starts a goroutine that drains events into an in-memory
ring buffer (default capacity 256).

Public surface:

- `Root()` / `DisplayRoot()` — absolute path and short label for UI use.
- `Snapshot()` — current summary (`Root`, `StartedAt`, `EventCount`,
  `BufferCapacity`).
- `Events()` — copy of the current event buffer.
- `Close()` — stops the watcher; safe to call multiple times.

New subdirectories created inside `root` are picked up automatically:
when a `Create` event lands and the target is a directory, the watcher
walks it and adds every nested directory to the backend.

## The Server

`server.Listen(assets, watch)` (`server/server.go:44`) returns a
function with the signature `dfw` expects for `App.Listen` /
`DaemonApp.Listen`:

```go
func() (*http.Server, net.Listener, error)
```

The implementation binds to `127.0.0.1:0` (an OS-assigned port on
loopback only), wires three routes, and returns the unstarted server
plus the open listener. `dfw` takes ownership from there.

Routes:

- `GET /api/status` — JSON snapshot of the watcher (root, start time,
  event count, buffer capacity).
- `GET /api/events` — JSON array of the current event buffer.
- `GET /` and everything else — static asset fallback served from the
  embedded `web/dist`. Unknown paths fall back to `index.html` so the
  React router can handle client-side routes.

JSON is encoded via `github.com/michaelquigley/df/dd` for consistency
with the rest of the `df` ecosystem.

## The Subcommands

### `run`

```sh
dfw-example-watch run [path]
```

Calls `dfw.Run` with the watcher and server in the same process. If
`[path]` is omitted, the current working directory is watched.
`--devtools` sets `DFW_DEVTOOLS=1` before `dfw.Run` opens the webview.

See `cmd/run.go`. The full wiring is six lines of business logic:
parse the path → start the watcher → produce the icon → call
`dfw.Run`.

### `daemon`

```sh
dfw-example-watch daemon [path]
```

Calls `dfw.Daemon` and stays resident in the tray. The tray menu is:

- `Open Window` — `dfw.SpawnSelf("window")`, which re-executes the
  daemon's own binary with the `window` subcommand and
  `DFW_DAEMON_ADDR` populated.
- `Watching <basename>` — a disabled item showing the watched
  directory, with the absolute path in the tooltip.
- `Quit` — shut down the daemon.

The daemon does not open a window itself. Use `Open Window` (or run
the `window` subcommand manually) to attach.

### `window`

```sh
dfw-example-watch window
```

Calls `dfw.Window`, which resolves the daemon address via
`DFW_DAEMON_ADDR` or the runtime file (see [runtime.md](runtime.md))
and opens a webview against it. `--devtools` toggles devtools the same
way `run` does. When the daemon spawns a window via `SpawnSelf`, the
spawned process inherits `DFW_DEVTOOLS` if it was set on the daemon —
but the daemon does not set it from `--devtools` itself, so devtools
on a spawned window means setting the env var before launching the
daemon.

## Icons

`cmd/common.go` generates the icon procedurally at first use: an NRGBA buffer,
dark background, three colored rectangles (teal bar plus orange and red
squares), encoded to PNG. The 32×32 version is cached behind a `sync.Once` for
window and tray use; the Linux desktop installer writes 32×32 and 128×128
hicolor icon files. Real products would `//go:embed` artist-supplied PNGs
instead.

The same PNG bytes are used for the window icon and the tray icon. On
Linux the tray icon is passed through directly; on Windows `dfw` wraps
it in an ICO container before handing it to `systray`. The product
does not need to know about either transformation.

## Talking To The API

The React UI under `web/` is a Vite project that fetches from
`/api/status` and `/api/events` on the same origin. Because the
webview navigates to the embedded server (`http://127.0.0.1:<port>/`),
the relative-URL fetch lands at the correct handler without any
cross-origin or proxy configuration.

This is the intended pattern for products using `dfw`: the web UI
talks to the product's own HTTP API. There is no JavaScript-to-Go
bridge; if the UI needs to trigger something on the Go side, it calls
an HTTP endpoint.

## Build Sequence

Full build:

```sh
# build the React bundle (required before go build can embed it)
cd examples/dfw-example-watch/web
pnpm install
pnpm build

# build the Go binary
cd ..
go build .
```

See [building.md](building.md) for platform-specific prerequisites and
the pnpm/esbuild note.

## Related

- [architecture.md](architecture.md) — the three entry points the
  example exercises.
- [runtime.md](runtime.md) — every on-disk and environment-variable
  behavior the example triggers in practice.
- [building.md](building.md) — toolchain requirements.
