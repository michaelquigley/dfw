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

// resolveListen invokes the caller-provided listen function and validates its
// results. The name argument identifies the entry point ("run", "daemon") so
// the returned errors point at the source.
func resolveListen(name string, listen func() (*http.Server, net.Listener, error)) (*http.Server, net.Listener, error) {
	if listen == nil {
		return nil, nil, fmt.Errorf("dfw: %s: listen function is required", name)
	}
	server, listener, err := listen()
	if err != nil {
		return nil, nil, err
	}
	if server == nil {
		if listener != nil {
			_ = listener.Close()
		}
		return nil, nil, fmt.Errorf("dfw: %s: listen returned nil server", name)
	}
	if listener == nil {
		return nil, nil, fmt.Errorf("dfw: %s: listen returned nil listener", name)
	}
	return server, listener, nil
}

// serveSupervisor runs an HTTP server in the background and surfaces an
// unexpected failure through an onFailure callback. The supervisor must be
// shut down via Shutdown before the surrounding entry point returns.
type serveSupervisor struct {
	server       *http.Server
	done         chan struct{}
	observedErr  error
	shuttingDown atomic.Bool
}

// superviseServe starts server.Serve(listener) in a goroutine. If the server
// exits with an error before Shutdown is called, observedErr is recorded and
// onFailure (if non-nil) is invoked. http.ErrServerClosed is treated as a
// clean exit and is not surfaced to onFailure.
func superviseServe(server *http.Server, listener net.Listener, onFailure func()) *serveSupervisor {
	s := &serveSupervisor{
		server: server,
		done:   make(chan struct{}),
	}
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		s.observedErr = err
		if err != nil && !s.shuttingDown.Load() && onFailure != nil {
			onFailure()
		}
		close(s.done)
	}()
	return s
}

// Shutdown gracefully stops the server and waits for the serve goroutine to
// exit. The observed serve error takes precedence over the shutdown error;
// both nil means a clean stop.
func (s *serveSupervisor) Shutdown() error {
	s.shuttingDown.Store(true)
	shutdownErr := shutdownHTTPServer(s.server)
	<-s.done
	if s.observedErr != nil {
		return s.observedErr
	}
	return shutdownErr
}

func shutdownHTTPServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("dfw: shut down http server: %w", err)
	}
	return nil
}
