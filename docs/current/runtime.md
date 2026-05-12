# Runtime

What `dfw` reads, writes, and observes at runtime. Everything in this
document is library behavior; products do not configure it directly, but
the on-disk and environment surface is visible to the user and to other
processes on the same machine.

All paths below resolve against `os.UserConfigDir()`:

- Linux: `$XDG_CONFIG_HOME` or `~/.config`
- Windows: `%AppData%` (typically `C:\Users\<user>\AppData\Roaming`)
- macOS: `~/Library/Application Support`

The per-app subdirectory is named after `App.AppID` (or `DaemonApp.AppID`
/ `WindowApp.AppID`) — typically a reverse-DNS string such as
`com.quigley.flo`.

## Window State

Persisted to `{user_config_dir}/{AppID}/runtime/window_state.json`,
0600 mode, JSON-encoded.

Schema:

```json
{
  "width": 1024,
  "height": 720,
  "x": 120,
  "y": 80
}
```

- `width` and `height` are always written. They are read back unless the
  saved values are below the minimum restored size (`320 × 240` by
  default, but no larger than the product's `InitialSize` if that is
  smaller). Invalid or absent values fall back to `App.InitialSize`.
- `x` and `y` are written only when the platform supports window
  position tracking (currently Linux and Windows). On unsupported
  platforms the keys are absent and the next launch starts at the
  window manager's default position.

The file is written once when the window closes (`SaveWindowState`
called from `Run` / `Window` after the webview's blocking `Run()`
returns). It is read once when the next window is created. There is no
intermediate write during normal operation, so a crash mid-session
forfeits the in-flight size and position changes.

## Daemon Discovery

`dfw.Daemon` writes
`{user_config_dir}/{AppID}/runtime/daemon.json` on startup and removes
it on clean shutdown. 0600 mode, JSON-encoded.

Schema:

```json
{
  "pid": 12345,
  "address": "127.0.0.1:53291"
}
```

- `pid` is the daemon process's PID. It is informational — `dfw` does
  not use it for liveness checks. Products can read it if they want to
  detect stale runtime files.
- `address` is the listener address as a `host:port` string.

There is no single-instance enforcement: two daemons started with the
same `AppID` will both write to the same path, and the second one wins.
Products that need single-instance behavior implement it themselves.

### `DFW_DAEMON_ADDR`

`dfw.Window` resolves the daemon address via the
`DFW_DAEMON_ADDR` environment variable first; if unset or empty, it
falls back to reading `daemon.json` for the same `AppID`. In both
cases the resolved address is validated with `net.SplitHostPort`
before being returned — a malformed value surfaces immediately rather
than later at navigation time.

`dfw.SpawnSelf` (and the lower-level `dfw.Spawn`) sets this env var
when the daemon spawns a window child, so the spawned window does not
need to read the runtime file. The runtime file is the discovery
mechanism for windows launched independently of the daemon (e.g., a
user typing `mybinary window` at a shell).

## DevTools

The webview's developer tools are enabled when the process starts with
the `DFW_DEVTOOLS` environment variable set to any value other than the
"falsy" set: empty string, `0`, `false`, `no`, `off`. Comparison is
case-insensitive and whitespace-trimmed. Any other value — `1`, `true`,
`yes`, or anything unrecognized — enables devtools.

`dfw.DevToolsEnabled()` exposes the same check for products that want to
gate their own debug behavior on it (verbose logging, extra HTTP
endpoints, etc.).

Products typically expose this via a `--devtools` flag that sets
`DFW_DEVTOOLS=1` for the current process before calling `dfw.Run` /
`dfw.Window`. When a daemon spawns a window child through
`dfw.SpawnSelf`, the env var is inherited.

## Tray

`dfw.Daemon` shows a system tray entry built at startup. The menu has
three regions, in this order:

1. **`Open Window`** — added only when `DaemonApp.SpawnWindow` is
   non-nil. Clicking it calls `SpawnWindow(daemonAddr)`; the typical
   implementation is `dfw.SpawnSelf("window")`. Errors are logged via
   `df/dl` and do not tear down the daemon.
2. **Product items** — each `TrayMenuItem` from `DaemonApp.TrayItems`
   becomes a menu entry with the supplied `Label`, `Tooltip`, and
   `OnClick`. `Disabled: true` items are added but greyed out.
3. **Separator + `Quit`** — clicking `Quit` triggers the systray
   shutdown, which returns control to `dfw.Daemon` and lets the
   library's deferred cleanup run.

The menu is built once at startup. Items cannot be added, removed, or
relabeled at runtime; products that need dynamic menus would either
fork the tray code or wait for that capability to be added.

The tray tooltip is set from `DaemonApp.Title` when non-empty.

## Icons

`App.IconPNG`, `DaemonApp.IconPNG`, and `WindowApp.IconPNG` all take raw
PNG bytes. Products supply the icon however they like — embedded via
`//go:embed`, read from a file, generated procedurally — and `dfw`
converts to the platform-native form:

- **Linux window icon** — decoded into a `GdkPixbuf` and applied to the
  GTK window.
- **Windows window icon** — decoded, repacked as a DIB, and attached to
  the HWND.
- **Linux tray icon** — PNG bytes passed through to `systray` (which
  hands them to the StatusNotifier protocol).
- **Windows tray icon** — wrapped in an ICO container before being
  handed to `systray`.

No client-side resizing happens. Supply an icon at a reasonable display
size (32×32 is a good default for tray icons; window icons are commonly
larger).

On GNOME, the running-app panel/dock icon is matched through desktop identity,
not just the per-window pixbuf. Linux windows use `AppID` for the process/window
class and themed icon name; products should install a desktop entry and hicolor
icon whose basename matches the same `AppID`.

## Failure Behavior

- If the HTTP server's `Serve()` returns an error after startup, the
  serve supervisor records the error and signals the front end to exit:
  for `Run`, it calls `window.Terminate()`; for `Daemon`, it closes the
  tray-stop channel. Either way the entry-point function returns with
  the recorded error.
- If `Run` or `Daemon` exit cleanly, `Serve()` is shut down via
  `server.Shutdown(ctx)` with a 5-second timeout, falling back to
  `server.Close()` on timeout.
- `daemon.json` is removed on clean shutdown only. A daemon killed
  with `SIGKILL` leaves a stale file; subsequent `dfw.Window` launches
  read the stale address and fail at the HTTP layer when they cannot
  connect. The PID in the file is informational and can be used to
  detect this case if the product cares.

## Related

- [architecture.md](architecture.md) — the three entry points and the
  HTTP boundary that frames everything here.
- [example.md](example.md) — `dfw-example-watch` exercises every
  feature on this page.
