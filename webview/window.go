package webview

import "github.com/michaelquigley/dfw/internal/core"

// Window opens a single webview window pointing at a remote HTTP server. It
// discovers the server address via DFW_DAEMON_ADDR or the AppID-derived
// runtime file.
func Window(app WindowApp) error {
	if err := app.PDF.claim(); err != nil {
		return err
	}
	defer app.PDF.cancelWindow()
	daemonAddr, err := core.ResolveDaemonAddr(app.AppID)
	if err != nil {
		return err
	}

	window, err := newConfiguredDesktopWebView(webviewConfig{
		AppID:          app.AppID,
		Title:          app.Title,
		InitialSize:    app.InitialSize,
		IconPNG:        app.IconPNG,
		Debug:          DevToolsEnabled(),
		EnableZoom:     app.EnableZoom,
		OnCloseRequest: app.OnCloseRequest,
		PDF:            app.PDF,
	})
	if err != nil {
		return err
	}
	defer window.Destroy()

	window.Navigate("http://" + daemonAddr)
	window.Run()
	window.SaveWindowState()
	return nil
}
