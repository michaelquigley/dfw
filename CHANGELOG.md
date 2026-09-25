# CHANGELOG

## Unreleased

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
