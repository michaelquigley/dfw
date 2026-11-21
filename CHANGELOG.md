# CHANGELOG

## Unreleased

## v0.1.7

FEATURE: Windows work on macOS, from a bare binary or from an application bundle. A window process takes the regular activation policy, so it has a Dock icon and the menu bar even when started from an `LSUIElement` menu bar daemon. It carries the standard application, Edit and Window menus, so Cut, Copy, Paste, Select All and Undo reach the page, and Quit, from the menu, the Dock, a logout or a script, closes the window rather than ending the process early. It comes to the front with the keyboard in the page when it opens, and takes the front once more if the menu it was opened from hands the activation back. Links with `target="_blank"` and `window.open` open `http`, `https` and `mailto` addresses in the default browser. `examples/dfw-example-macos` is the native fixture, with a `bundle.sh` that packs it as a menu bar application.

FIX: The file chooser opens on macOS. webview_go's UI delegate was released as soon as it was set, so `<input type="file">` did nothing; dfw now sets and holds its own, which opens a sheet on the window and also answers the new-window links above.

FEATURE: `DaemonApp.OnReopen` answers a running tray daemon being opened again on macOS, from Finder, the Dock, Spotlight, Launchpad or `open`. macOS starts no second process then and reports the reopen to the running one, where nothing visible happened before. It receives the daemon's address, as `SpawnWindow` does, so the usual answer opens a window. Other platforms never call it.

FIX: A macOS window that no `InitialSize` or saved state gives a size has its close and minimize buttons. It had neither, so Quit, Close and Minimize did nothing to it.

FIX: A window configured with `OnCloseRequest` on a platform without close interception fails before any window opens. It used to show a window first, then fail.

CHANGE: On macOS, when the tray has an icon, the title is only the tooltip. The menu bar drew it as text beside the icon, which widened the status item and hid it sooner beside a notch. A tray with no icon still shows its title.

## v0.1.6

FEATURE: Tray daemons can take part in their own lifetime and change their tray while they run. `DaemonApp.OnQuit` is asked when `Quit` is chosen and can decline it. A `tray.Control` from `tray.NewControl()`, passed in `DaemonApp.Control`, replaces the icon and tooltip, relabels, enables, and disables `TrayItems`, and ends the daemon through the same path as `Quit` with `Stop`. Calls are safe from any goroutine, and calls made before the tray is built are applied when it appears. Daemons that set neither field behave as before.

## v0.1.5

FEATURE: Applications can opt into fullscreen on Linux with `EnableFullscreen` on `webview.App` or `webview.WindowApp`, which binds F11 to toggle it; `Fullscreen` opens the window fullscreen and binds F11 too. The state follows the window manager's reports, and a window closed fullscreen saves its windowed size and location rather than the screen's.

## v0.1.4

FIX: Linux window startup can select an unused real-time signal for JavaScriptCore before initialization instead of letting its GC handler replace Go's SIGUSR1 handler. The optional private API is resolved dynamically, explicit settings and existing signal use are respected, and unsupported/already-initialized cases retain their previous behavior. `DFW_DISABLE_JSC_SIGNAL_SETUP` opts out for applications with their own signal policy.

## v0.1.3

FEATURE: Linux windows can opt into `PDFExporter` for a native destination chooser and PDF rendering through separately installed system Chromium. The capability preserves the chooser's accepted filename, returns PDF bytes for product-owned publication, and exposes window-lifetime cancellation through `WindowContext`; it uses a private profile and control pipe with the browser sandbox enabled.

## v0.1.2

CHANGE: GitHub Actions now builds the example frontend and installs dfw's Linux native dependencies before running the full race-enabled Go suite with coverage and a separate `golangci-lint` job on every push and pull request.

FEATURE: Applications can intercept native window-close requests on Linux and Windows by setting `OnCloseRequest` on `webview.App` or `webview.WindowApp`. Each one-shot `CloseRequest` remains pending until the product calls `Close` or `KeepOpen`; repeated native closes are dropped, while transport and save/discard policy stay product-owned.

FIX: A supervised server failure that arrives before the webview's UI loop is ready now remains latched and terminates through the UI thread instead of being dropped or targeting a stopped window.

## v0.1.1

FEATURE: Applications can opt into remembered native page zoom on Linux with `EnableZoom`; standard control-key and keypad shortcuts adjust the complete WebKit page, and the selected percentage persists with window state.

## v0.1.0

First tagged release.

FEATURE: `webview.Run`, `webview.Window`, and `tray.Daemon` provide single-process window, daemon-attached window, and tray-resident application topologies around a product-owned HTTP server.

CHANGE: Public APIs are separated into `webview` and `tray` packages so tray-only consumers do not compile or carry the native webview dependency.

FEATURE: Native Linux and Windows integrations apply application icons, remember window bounds, supervise HTTP-server lifetime, and let independent window processes discover a tray-resident daemon.
