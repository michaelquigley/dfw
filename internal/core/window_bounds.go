package core

// WindowBounds is a snapshot of a native window's size and, when the platform
// supports it, its location. It is produced by the platform-specific bounds
// trackers in the webview package and consumed by WindowStateFromBounds.
type WindowBounds struct {
	Width       int
	Height      int
	X           int
	Y           int
	HasLocation bool
}
