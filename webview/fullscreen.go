package webview

type nativeFullscreenController interface {
	Fullscreen() bool
	Close()
}

// fullscreenEnabled reports whether the window binds F11. it is opt-in, so a
// window with neither option keeps F11 for its page.
func (c webviewConfig) fullscreenEnabled() bool {
	return c.EnableFullscreen || c.Fullscreen
}
