# dfw — initial implementation plan

## Context

`dfw` is a new Go library at `github.com/michaelquigley/dfw` that wraps a Go HTTP server (with embedded web UI) in a native webview window, with an optional tray-resident daemon mode for background work and multi-window scenarios. It is a sibling to `df` (the toolkit core) and `dfx` (imgui-based desktop UI) — same family, different rendering technology.

The full design is captured in `docs/future/initial-spec.md`. This plan does not restate that design — it sequences the work into reviewable stages and pins decisions made during planning.

**Repo state today**: blank. Only `go.mod` (module path + `go 1.26.2`) and the spec. No prior webview or systray code anywhere in `/home/michael/Repos/q/products/` to inherit from — `dfw` will be the first library in the toolkit family to introduce these dependencies.

**Decisions pinned during planning** (override or refine the spec):

- **Platform scope for MVP: Windows + Linux only.** macOS is deferred. Drop `icon_darwin.go` and any `//go:build darwin` files from Stage 2; the spec's macOS rows (WKWebView, Xcode CLT, accessory mode for daemons) are out of scope until post-MVP.
- **Delivery: phased, human-reviewed.** Four stages below. Each stage compiles, has tests where applicable, and is reviewable cold. The user commits between stages; I do not commit.
- **Example app scope: full.** `examples/dfw-example-watch` ships with a real React UI (Vite), `fsnotify`-backed watcher, and all three subcommands wired end-to-end. This is Stage 4.
- **`df` companion packages are defaults.** `dfw` is an add-on to `github.com/michaelquigley/df`: use `dd` for all data binding/unbinding (no `json` struct tags just to shape runtime data; `dd` snake_cases by default) and use `dl` for all logging if logging is needed. When `AGENTS.md` is drafted, these conventions must be captured there. Runtime files should use `dd`'s file helpers; once the required `dd.Options` file-mode API is in the selected `df` version, express runtime-file permissions there instead of with a follow-up chmod.

## Stages

### Stage 1 — Foundations (no UI, no CGO)

Pure-Go scaffolding. Compiles and tests on every platform without a webview toolchain.

**New files:**
- `doc.go` — package overview comment
- `app.go` — `App`, `DaemonApp`, `WindowApp`, `TrayMenuItem` types per spec §Public API
- `discovery.go` — `daemon.json` read/write, `DFW_DAEMON_ADDR` resolution, `userConfigPath(AppID)` helper.
- `devtools.go` — `DevToolsEnabled()` reading `DFW_DEVTOOLS` env var
- `spawn.go` — `SpawnSelf(args ...string)` and `Spawn(binary string, args ...string)` returning `func(daemonAddr string) error`. Build an `exec.Cmd` with the binary + args, inject `DFW_DAEMON_ADDR=<addr>` into `cmd.Env` (alongside `os.Environ()`), call `cmd.Start()`, and on success spin a goroutine that calls `cmd.Wait()` and discards the result so the child is reaped when it eventually exits (otherwise long-running daemons accumulate zombie processes on Linux). The closure returns from `SpawnWindow` as soon as `Start` succeeds — it does not block on `Wait`. Children outlive the daemon because of OS-level process semantics under the chosen subsystem, not because the parent skipped `Wait`.
- `LICENSE` — Apache 2.0 boilerplate
- `NOTICE` — attribution stub including `github.com/michaelquigley/df` (updated as more deps are added in later stages)
- `README.md` — short overview pointing at the spec. Explicitly label platform support as "MVP: Windows + Linux; macOS deferred" so the README does not overclaim what the spec describes as the broader target. Include a one-line "Building" hint: `go build ./...` at the repo root will succeed for the library itself but will fail at the example's `//go:embed` until `pnpm install && pnpm build` has run in `examples/dfw-example-watch/web/`. Point at `examples/dfw-example-watch/README.md` for the full sequence. This saves reviewers and CI runs from having to discover the dependency from inside the example.

**Tests** (`*_test.go`, alongside implementation, using `stretchr/testify`):
- `discovery_test.go` — round-trip `daemon.json` write/read; env-var overrides file; missing file + missing env returns error; **AppID path shape**: a table test exercising the helper that derives `{user_config_dir}/{AppID}/runtime/daemon.json` for a range of AppIDs (using a temporary config-dir override or helper injection) so that the public path convention can't silently regress under helper refactors.
- `spawn_test.go` — `SpawnSelf` builds correct argv + env without launching anything heavy (use a no-op test binary or table-test the command construction by exposing an internal `buildCmd` helper).

**Verification:**
- `go build ./...` on Linux and Windows
- `go test ./...` on Linux and Windows
- No CGO required at this stage — confirms the pure-Go portion is portable

**Critical files to read first:** `docs/future/initial-spec.md` §Discovery, §Lifecycle, §Public API.

---

### Stage 2 — Single-window (`dfw.Run`)

First stage with CGO + webview. Lands the `Run` entry point and platform-specific icon plumbing for Windows + Linux only.

**New files:**
- `webview.go` — thin wrapper over the selected WebKitGTK 4.1-compatible Go webview binding (`centrifuge.hectabit.org/HectaBit/webview_go` for the initial Linux/Fedora 43 implementation). Centralizes `New(debug bool)`, `Navigate(url)`, `SetTitle`, `SetSize`, `Run()` (blocking), `Terminate()`, and devtools toggle. Keep the surface small so Stage 3 reuses it.
- `icon.go` — cross-platform icon decode helpers (PNG bytes → platform-native form where needed)
- `icon_linux.go` — `//go:build linux`
- `icon_windows.go` — `//go:build windows`
- `run.go` — `func Run(app App) error` implementing the lifecycle in spec §Lifecycle and the contract in spec §Listen contract (including the **cleanup invariant**: once `Serve` is started, every return path shuts the server down and waits for the goroutine — use `defer`):
  1. Call `app.Listen()` to get server + listener; propagate the error as-is on failure
  2. Start a goroutine running `server.Serve(listener)`; capture its return value on a buffered `srvErr` channel (`http.ErrServerClosed` is expected post-shutdown; any other value is a fatal error for `Run`)
  3. `defer` a cleanup block that calls `server.Shutdown(ctx)` with a small timeout and then `<-srvErr` to wait for the goroutine to exit. This runs whether step 4+ succeed or fail
  4. Create webview at persisted bounds when available, otherwise `app.InitialSize`, then `Navigate("http://" + listener.Addr().String())`. If webview construction or icon setup fails, return the error — the deferred cleanup handles the rest
  5. Block on `wv.Run()` (or surface a captured non-`ErrServerClosed` Serve error early if the goroutine fails before the user closes the window)
  6. Return the captured Serve error if non-nil and non-`ErrServerClosed`, else `nil` (the deferred cleanup has already shut the server down)

**Dependencies added:**
- `centrifuge.hectabit.org/HectaBit/webview_go` (MIT) — update `NOTICE`. This binding is API-compatible with `github.com/webview/webview_go` but uses `webkit2gtk-4.1`, which is the WebKitGTK version available on Fedora 43.
- Add a short developer-setup note to `README.md` calling out Linux requires GTK 3 + WebKitGTK 4.1 development files and Windows requires WebView2 runtime (usually present on Win10+/Win11).

**Manual verification:**
- Stand up a trivial `net/http` server returning a static HTML page; call `dfw.Run` with it. Window opens, displays the page, closes cleanly.
- Set `DFW_DEVTOOLS=1` and confirm devtools inspector becomes available.
- Confirm the window opens at `InitialSize` when no persisted window state exists.

**Critical files to read first:** Stage 1 files (especially `discovery.go`), the selected webview binding's `README.md` / godoc for current API surface — verify the close-event hook before depending on it.

---

### Stage 3 — Daemon + Window (`dfw.Daemon`, `dfw.Window`)

Multi-process flow. Tray daemon owns the HTTP server; window children connect via the runtime file or env var.

**New files:**
- `tray.go` — wrapper over `fyne.io/systray`. Sets tooltip from `Title`, sets icon from `IconPNG`, installs menu items in this order: "Open Window" (only when `SpawnWindow != nil`), then `TrayItems` from `DaemonApp` in declared order, then a separator and "Quit" last. All `TrayMenuItem` fields are read **once** when the menu is built — `dfw` does not watch product state or rebuild the menu after construction. Dynamic tray state is explicitly deferred (see spec §Deferred / Future Work). Manages the tray event loop. When the "Open Window" item is clicked, the handler calls `SpawnWindow(resolvedAddr)`; on a non-nil error, log through `github.com/michaelquigley/df/dl` (e.g. `dl.Errorf("dfw: SpawnWindow: %v", err)`) and continue — the daemon and tray loop are not torn down (see spec §Spawn helpers).
- `daemon.go` — `func Daemon(app DaemonApp) error` implementing spec §Lifecycle "Daemon mode" and the contract in spec §Listen contract (including the **cleanup invariant**: once `Serve` is started, every return path shuts the server down and waits for the goroutine; once `daemon.json` is written, every non-crash return path removes it — use two `defer`s):
  1. `app.Listen()` → server + listener; propagate the error as-is on failure
  2. Start a goroutine running `server.Serve(listener)`; capture its return value on a buffered `srvErr` channel (`http.ErrServerClosed` is expected post-shutdown; any other value short-circuits the tray loop and becomes the fatal error for `Daemon`)
  3. `defer` a cleanup block that calls `server.Shutdown(ctx)` and then `<-srvErr`
  4. Write `daemon.json` (PID, address). If the write succeeds, immediately `defer` a best-effort `os.Remove(daemonJSONPath)` so every later return path cleans up — including failures in tray init below
  5. Initialize tray (`tray.go`). If tray construction or icon setup fails, return the error — the two deferred cleanups handle the rest
  6. Block on the tray event loop until "Quit" (or surface a captured non-`ErrServerClosed` Serve error early if the goroutine fails)
  7. Return the captured Serve error if non-nil and non-`ErrServerClosed`, else `nil` (the deferred cleanups have already shut the server down and removed `daemon.json`)
- `window.go` — `func Window(app WindowApp) error` implementing spec §Lifecycle "Window mode":
  1. Resolve daemon address: `DFW_DAEMON_ADDR` env var first, then read `daemon.json` at the AppID-derived path; error if neither present
  2. Create webview at persisted bounds when available, otherwise `app.InitialSize`, then navigate to `"http://" + resolvedAddr`
  3. Block on the webview event loop until the window closes; return any captured error

**Dependencies added:**
- `fyne.io/systray` (Apache-2.0) — update `NOTICE` with the attribution per Apache 2.0 §4(d)

**Manual verification:**
- Build a tiny daemon that serves a static page. Run it; confirm tray icon + tooltip appear.
- Invoke the daemon's `SpawnWindow` (via the tray menu) — child process opens a window pointing at the daemon's address. Repeat to confirm multi-window.
- Kill the daemon while a window is open; confirm the window process stays alive and `dfw.Window` does not crash. A page already loaded into the webview will remain visible until something triggers another HTTP request — Stage 3 does not promise a visible load-failure, only that nothing crashes. Product-level connection-lost UI is verified in Stage 4 once the React example surfaces it.
- Set `DFW_DAEMON_ADDR=127.0.0.1:NNNN` and start a `Window` process directly — confirms env-var path resolution.
- Unset `DFW_DAEMON_ADDR` and start a `Window` process directly while the daemon is running — confirms the runtime-file discovery path actually works end-to-end (the spawned-window path always uses the env var, so this is the only place the file branch is exercised).
- Delete `daemon.json` while a daemon is still running; confirm the daemon does *not* try to recreate it (dfw writes once on startup, removes on clean shutdown only).

**Critical files to read first:** spec §Discovery, §Process Model, §Lifecycle. fyne.io/systray's godoc.

---

### Stage 4 — `dfw-example-watch`

Single binary with three subcommands demonstrating all of Stages 1–3 end-to-end.

**New files** under `examples/dfw-example-watch/`:
- `main.go` — root command (cobra) dispatching `run`, `daemon`, `window` subcommands
- `cmd/run.go`, `cmd/daemon.go`, `cmd/window.go` — subcommand entry points wiring `dfw.Run`, `dfw.Daemon`, `dfw.Window`
- `server/server.go` — small OpenAPI-shaped HTTP server (`/api/events`, `/api/status`) with an embedded `fs.FS` serving the built React bundle from the package binary
- `watcher/watcher.go` — `fsnotify` watcher producing event records to a ring buffer; both `run` and `daemon` modes feed from the same code path
- `web/` — Vite + React project; `web/dist/` is `//go:embed`ed by `server/server.go`. Keep the UI minimal: an event timeline pulled via fetch from `/api/events`. **Repository convention: `web/dist/` is gitignored.** A fresh `git clone` will not contain `web/dist`; `pnpm install && pnpm build` (or the npm equivalent) must run before `go build ./...` or the embed will fail with a clear "pattern matches no files" error. The example's README puts this step first. This avoids committing large generated artifacts and matches typical Vite+Go embed conventions.
- `examples/dfw-example-watch/README.md` — how to build the React bundle (`pnpm install && pnpm build` or `npm` equivalent), then `go build ./...`. On Windows, document the single-binary subsystem tradeoff explicitly: `go build -ldflags "-H windowsgui" .` for shipping (daemon diagnostics go to a log file) vs. plain `go build .` for dev (daemon prints to stdout, but `run`/`window` flash a console window). Do not present the spec's per-binary advice without this caveat — the example is single-binary.

**Manual verification (per spec §Testing — manual smoke tests):**
- `dfw-example-watch run /tmp/somewhere` — window opens, touching files in `/tmp/somewhere` shows events in real time.
- `dfw-example-watch daemon /tmp/somewhere` — tray icon and static tooltip (product name) appear. The running event count lives in the window and the HTTP API, not in the tray (dfw's tray surface is intentionally static; see spec §Demonstration).
- Tray "Open Window" → child `dfw-example-watch window` process spawns, connects, displays same timeline.
- Quit daemon via tray — clean shutdown, `daemon.json` removed.
- With a spawned window open, quit the daemon via tray (or SIGKILL it for the abrupt-shutdown variant). Confirm the React UI in the still-open window surfaces the disconnected / failed-load state (banner, toast, or whatever the example chooses) rather than appearing frozen. This is the product-level connection-lost check that Stage 3 deferred here — Stage 3 only verifies that `dfw.Window` itself doesn't crash.
- `--devtools` flag works for `run` and `window`.

---

## Cross-stage notes

- **CGO toolchain on the dev machine.** Linux requires GTK 3 plus WebKitGTK 4.1 development files. On Fedora 43, `pkg-config --exists gtk+-3.0 webkit2gtk-4.1` must pass before Stage 2 can be fully verified.
- **Windows GUI subsystem.** Per spec §Windows subsystem. Because `dfw-example-watch` is a single binary with subcommands (`run`/`daemon`/`window`), Windows cannot pick a subsystem per subcommand — the subsystem is a property of the linked binary. Stage 4's example README must document this as a binary-level choice, not as separate per-subcommand build steps: (1) `-ldflags "-H windowsgui"` for shipping builds — no console flashes for `run`/`window`, `daemon` diagnostics go to a log file; (2) console subsystem for dev-time builds — `daemon` prints to stdout normally, but `run`/`window` briefly flash a console. Do not bake either choice into `dfw` itself; the choice is the example's, and downstream products will make the same call for their own layouts.
- **No commits from me.** User reviews each stage's diff and commits it. I produce a working tree per stage and stop for review.
- **Testing posture.** Unit tests cross-platform throughout. The `dfwtest` build-tag integration tests mentioned in the spec are deferred — they need display-equipped CI runners which aren't set up yet. Stage 2/3 verification is manual smoke-testing on the dev box.
- **No macOS files.** Anywhere the spec calls out a `_darwin.go` companion, only Linux + Windows companions get written. No `//go:build darwin` build tags introduced.
