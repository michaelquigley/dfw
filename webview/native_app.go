package webview

// nativeAppController is the application-level setup a window needs from its
// platform beyond the window itself: on macOS the menus, the quit event, the
// UI delegate and the focus as it opens. it belongs to the window that applied
// it, and Destroy closes it before the binding destroys the window. platforms
// that need no such setup have none (nil).
type nativeAppController interface {
	// Activate brings the window forward as its run loop starts.
	Activate()
	// EndActivation makes focus work still pending from Activate do nothing,
	// as the run loop returns.
	EndActivation()
	// Close detaches what the setup wired to the window.
	Close()
}
