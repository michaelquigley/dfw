package webview

import "sync"

// CloseRequest is one native close request delivered to OnCloseRequest. the
// request is pending until it is resolved with Close or KeepOpen. both methods
// are safe from any goroutine, tolerate a nil receiver, and become no-ops after
// the first resolution, after a later request supersedes this one, or after the
// window lifecycle ends.
type CloseRequest struct {
	controller *closeController
	generation uint64
}

// Close grants the close. the window's blocking run loop terminates and the
// entry point returns through its ordinary save-and-destroy path.
func (r *CloseRequest) Close() {
	if r == nil || r.controller == nil {
		return
	}
	r.controller.close(r.generation)
}

// KeepOpen declines the close and rearms native close interception. a later
// native close request creates a new CloseRequest.
func (r *CloseRequest) KeepOpen() {
	if r == nil || r.controller == nil {
		return
	}
	r.controller.keepOpen(r.generation)
}

// closeController owns the single-flight close request state shared by the
// native adapters. the adapters call requestClose from the native close path
// and consume the native event when it returns true.
type closeController struct {
	mu         sync.Mutex
	callback   func(*CloseRequest)
	terminate  func()
	generation uint64
	pending    uint64
	closing    bool
	ended      bool
}

func newCloseController(callback func(*CloseRequest), terminate func()) *closeController {
	return &closeController{callback: callback, terminate: terminate}
}

// requestClose is the native-facing entry point. it reports whether the native
// close event must be consumed. when idle it allocates a new generation and
// dispatches the callback asynchronously; while a request is pending it
// consumes the event without dispatching or queuing anything; once the
// controller is closing or ended it lets the native close continue.
func (c *closeController) requestClose() bool {
	c.mu.Lock()
	if c.ended || c.closing {
		c.mu.Unlock()
		return false
	}
	if c.pending != 0 {
		c.mu.Unlock()
		return true
	}
	c.generation++
	c.pending = c.generation
	request := &CloseRequest{controller: c, generation: c.generation}
	callback := c.callback
	c.mu.Unlock()

	go callback(request)
	return true
}

func (c *closeController) keepOpen(generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended || c.closing || c.pending != generation {
		return
	}
	c.pending = 0
}

func (c *closeController) close(generation uint64) {
	c.mu.Lock()
	if c.ended || c.closing || c.pending != generation {
		c.mu.Unlock()
		return
	}
	c.closing = true
	c.pending = 0
	terminate := c.terminate
	c.mu.Unlock()

	// terminate runs outside the mutex so re-entry from the termination path
	// cannot deadlock against this controller.
	if terminate != nil {
		terminate()
	}
}

// end expires any pending request and makes every retained CloseRequest inert.
// it is idempotent.
func (c *closeController) end() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = true
	c.pending = 0
}

// nativeCloseInterceptor is the platform hook that routes the native close
// path through a closeController. Close detaches the hook; it must run before
// the native window is destroyed.
type nativeCloseInterceptor interface {
	Close()
}
