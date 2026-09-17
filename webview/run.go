package webview

import (
	"github.com/michaelquigley/dfw/internal/core"
)

// Run starts a single-window application. This process owns the HTTP server,
// any background work, and the window. it returns when the window is closed or
// on fatal error.
func Run(app App) (err error) {
	server, listener, err := core.ResolveListen("run", app.Listen)
	if err != nil {
		return err
	}

	// the coordinator exists before the server starts, so a serve failure that
	// arrives before the window is constructed stays latched instead of being
	// dropped.
	termination := newTerminationCoordinator()
	supervisor := core.SuperviseServe(server, listener, termination.request)
	defer func() {
		shutdownErr := supervisor.Shutdown()
		if err == nil {
			err = shutdownErr
		}
	}()

	window, err := newConfiguredDesktopWebView(webviewConfig{
		AppID:          app.AppID,
		Title:          app.Title,
		InitialSize:    app.InitialSize,
		IconPNG:        app.IconPNG,
		Debug:          DevToolsEnabled(),
		EnableZoom:     app.EnableZoom,
		OnCloseRequest: app.OnCloseRequest,
		termination:    termination,
	})
	if err != nil {
		termination.end()
		return err
	}
	defer window.Destroy()

	window.Navigate("http://" + listener.Addr().String())
	window.Run()
	window.SaveWindowState()
	return nil
}
