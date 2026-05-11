package dfw

import (
	"errors"
	"os"
)

// Daemon starts a tray-resident daemon. This process owns the HTTP server and
// any background work, exposes a tray icon, and writes a runtime file for
// window discovery. It returns when the daemon is shut down via the tray menu
// or on fatal error.
func Daemon(app DaemonApp) (err error) {
	if app.Listen == nil {
		return errors.New("dfw: daemon listen function is required")
	}

	server, listener, err := app.Listen()
	if err != nil {
		return err
	}
	if server == nil {
		if listener != nil {
			_ = listener.Close()
		}
		return errors.New("dfw: listen returned nil server")
	}
	if listener == nil {
		return errors.New("dfw: listen returned nil listener")
	}

	serveErr := startHTTPServer(server, listener)
	trayStop := make(chan struct{})

	var observedServeErr error
	serveDone := make(chan struct{})
	go func() {
		if serveErr := <-serveErr; serveErr != nil {
			observedServeErr = serveErr
			close(trayStop)
		}
		close(serveDone)
	}()

	defer func() {
		shutdownErr := shutdownHTTPServer(server)
		<-serveDone
		if err != nil {
			return
		}
		if observedServeErr != nil {
			err = observedServeErr
			return
		}
		err = shutdownErr
	}()

	runtimePath, err := writeDaemonRuntime(app.AppID, daemonRuntime{
		PID:     os.Getpid(),
		Address: listener.Addr().String(),
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(runtimePath)
	}()

	return runTray(trayConfig{
		Title:       app.Title,
		IconPNG:     app.IconPNG,
		DaemonAddr:  listener.Addr().String(),
		SpawnWindow: app.SpawnWindow,
		TrayItems:   app.TrayItems,
		Stop:        trayStop,
	})
}
