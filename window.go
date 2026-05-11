package dfw

// Window opens a single webview window pointing at a remote HTTP server. It
// discovers the server address via DFW_DAEMON_ADDR or the AppID-derived
// runtime file.
func Window(app WindowApp) error {
	daemonAddr, err := resolveDaemonAddr(app.AppID)
	if err != nil {
		return err
	}

	window, err := newConfiguredWebView(webviewConfig{
		Title:       app.Title,
		InitialSize: app.InitialSize,
		IconPNG:     app.IconPNG,
		Debug:       DevToolsEnabled(),
	})
	if err != nil {
		return err
	}
	defer window.Destroy()

	window.Navigate("http://" + daemonAddr)
	window.Run()
	return nil
}
