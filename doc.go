// Package dfw wraps Go HTTP servers with native desktop webview windows.
//
// The package is intentionally small. Products own their HTTP API, embedded
// web UI, background work, and distribution. dfw provides only the process
// and window/tray lifecycle needed to run that HTTP UI as a desktop app.
//
// Runtime data files use github.com/michaelquigley/df/dd for binding and
// unbinding.
package dfw
