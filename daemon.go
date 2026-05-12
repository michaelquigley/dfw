package dfw

import (
	"os"
	"sync"
)

// Daemon starts a tray-resident daemon. This process owns the HTTP server and
// any background work, exposes a tray icon, and writes a runtime file for
// window discovery. It returns when the daemon is shut down via the tray menu
// or on fatal error.
func Daemon(app DaemonApp) (err error) {
	server, listener, err := resolveListen("daemon", app.Listen)
	if err != nil {
		return err
	}

	trayStop := make(chan struct{})
	var stopOnce sync.Once
	supervisor := superviseServe(server, listener, func() {
		stopOnce.Do(func() { close(trayStop) })
	})
	defer func() {
		shutdownErr := supervisor.Shutdown()
		if err == nil {
			err = shutdownErr
		}
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
