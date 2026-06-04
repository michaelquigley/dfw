// Package dfw wraps Go HTTP servers with native desktop webview windows and a
// tray-resident daemon.
//
// The package is intentionally small. Products own their HTTP API, embedded
// web UI, background work, and distribution. dfw provides only the process and
// window/tray lifecycle needed to run that HTTP UI as a desktop app.
//
// The functionality is split across subpackages so a consumer can take only
// what it needs:
//
//   - github.com/michaelquigley/dfw/tray — the tray-resident daemon
//     (tray.Daemon). Depends on fyne.io/systray; it does NOT pull the webview
//     binding, so a tray-only consumer builds without WebKitGTK/WebView2.
//   - github.com/michaelquigley/dfw/webview — the desktop window entry points
//     (webview.Run, webview.Window). Depends on the CGO webview binding.
//   - github.com/michaelquigley/dfw/internal/core — shared HTTP-server
//     supervision, daemon discovery, icon decoding, and window-state
//     persistence used by both. Pure Go, no CGO.
//
// Runtime data files use github.com/michaelquigley/df/dd for binding and
// unbinding.
package dfw
