package dfw

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

const serverShutdownTimeout = 5 * time.Second

// Run starts a single-window application. This process owns the HTTP server,
// any background work, and the window. It returns when the window is closed or
// on fatal error.
func Run(app App) (err error) {
	if app.Listen == nil {
		return errors.New("dfw: app listen function is required")
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

	shuttingDown := atomic.Bool{}
	serveErr := startHTTPServer(server, listener)

	var observedServeErr error
	serveDone := make(chan struct{})
	defer func() {
		shuttingDown.Store(true)
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

	window, err := newConfiguredWebView(webviewConfig{
		AppID:       app.AppID,
		Title:       app.Title,
		InitialSize: app.InitialSize,
		IconPNG:     app.IconPNG,
		Debug:       DevToolsEnabled(),
	})
	if err != nil {
		go observeServeResult(serveErr, &observedServeErr, serveDone, nil, &shuttingDown)
		return err
	}
	defer window.Destroy()

	go observeServeResult(serveErr, &observedServeErr, serveDone, window, &shuttingDown)

	window.Navigate("http://" + listener.Addr().String())
	window.Run()
	window.SaveWindowState()
	return nil
}

func startHTTPServer(server *http.Server, listener net.Listener) <-chan error {
	done := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		done <- err
	}()
	return done
}

func observeServeResult(serveErr <-chan error, observed *error, done chan<- struct{}, window *desktopWebView, shuttingDown *atomic.Bool) {
	err := <-serveErr
	if err != nil {
		*observed = err
		if window != nil && !shuttingDown.Load() {
			window.Terminate()
		}
	}
	close(done)
}

func shutdownHTTPServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	err := server.Shutdown(ctx)
	cancel()
	if err != nil {
		_ = server.Close()
		return fmt.Errorf("dfw: shut down http server: %w", err)
	}
	return nil
}
