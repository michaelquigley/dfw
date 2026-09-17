package webview

import "sync"

type terminationState int

const (
	terminationUnbound terminationState = iota
	terminationBound
	terminationReady
	terminationEnded
)

// terminationCoordinator carries termination requests from background
// goroutines (the serve supervisor, CloseRequest.Close) to the webview's UI
// thread. a request is sticky: one made before the window is bound or before
// its UI loop is ready stays latched until the loop can act on it, and one made
// after the loop has ended is ignored. at most one native termination runs.
//
// the dispatch operation must be asynchronous; it is invoked while the
// coordinator's mutex is held so that the ready-to-ended transition cannot
// race with an enqueue against a stopped webview.
type terminationCoordinator struct {
	mu         sync.Mutex
	state      terminationState
	requested  bool
	terminated bool
	dispatch   func(func())
	terminate  func()
}

func newTerminationCoordinator() *terminationCoordinator {
	return &terminationCoordinator{}
}

// bind attaches the UI-thread dispatch and native terminate operations. it does
// not declare the dispatcher ready; ready does that from the UI thread.
func (c *terminationCoordinator) bind(dispatch func(func()), terminate func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != terminationUnbound {
		return
	}
	c.state = terminationBound
	c.dispatch = dispatch
	c.terminate = terminate
}

// request asks for termination. before readiness the request is latched; while
// ready it schedules one native termination on the UI thread; after end it is
// ignored. repeated requests never schedule more than one termination.
func (c *terminationCoordinator) request() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == terminationEnded || c.requested {
		return
	}
	c.requested = true
	if c.state != terminationReady {
		return
	}
	c.dispatch(c.performTerminate)
}

// latched reports whether a termination request is already waiting. Run
// consults it before entering the native loop.
func (c *terminationCoordinator) latched() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requested
}

// ready declares the UI-thread dispatcher usable. it must run on the UI thread
// and immediately performs any termination latched during startup.
func (c *terminationCoordinator) ready() {
	c.mu.Lock()
	if c.state != terminationBound {
		c.mu.Unlock()
		return
	}
	c.state = terminationReady
	requested := c.requested
	c.mu.Unlock()

	if requested {
		c.performTerminate()
	}
}

// end stops the coordinator. requests after end neither dispatch nor
// terminate, and dispatched work that runs after end does nothing.
func (c *terminationCoordinator) end() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = terminationEnded
}

// performTerminate runs on the UI thread and calls the native terminate
// operation at most once, and never after end.
func (c *terminationCoordinator) performTerminate() {
	c.mu.Lock()
	if c.state == terminationEnded || c.terminated {
		c.mu.Unlock()
		return
	}
	c.terminated = true
	terminate := c.terminate
	c.mu.Unlock()

	terminate()
}
