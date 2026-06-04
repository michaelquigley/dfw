package webview

import (
	"sync/atomic"

	"github.com/michaelquigley/dfw/internal/core"
)

// Run starts a single-window application. This process owns the HTTP server,
// any background work, and the window. It returns when the window is closed or
// on fatal error.
func Run(app App) (err error) {
	server, listener, err := core.ResolveListen("run", app.Listen)
	if err != nil {
		return err
	}

	var windowPtr atomic.Pointer[desktopWebView]
	supervisor := core.SuperviseServe(server, listener, func() {
		if w := windowPtr.Load(); w != nil {
			w.Terminate()
		}
	})
	defer func() {
		shutdownErr := supervisor.Shutdown()
		if err == nil {
			err = shutdownErr
		}
	}()

	window, err := newConfiguredWebView(webviewConfig{
		AppID:       app.AppID,
		Title:       app.Title,
		InitialSize: app.InitialSize,
		IconPNG:     app.IconPNG,
		Debug:       DevToolsEnabled(),
	})
	if err != nil {
		return err
	}
	defer window.Destroy()
	windowPtr.Store(window)

	window.Navigate("http://" + listener.Addr().String())
	window.Run()
	window.SaveWindowState()
	return nil
}
