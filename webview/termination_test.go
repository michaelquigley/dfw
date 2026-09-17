package webview

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDispatcher captures scheduled UI-thread work so a test chooses when, and
// whether, each function runs.
type fakeDispatcher struct {
	mu         sync.Mutex
	scheduled  []func()
	terminated int
}

func (d *fakeDispatcher) dispatch(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.scheduled = append(d.scheduled, f)
}

func (d *fakeDispatcher) terminate() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.terminated++
}

func (d *fakeDispatcher) scheduledCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.scheduled)
}

func (d *fakeDispatcher) terminatedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.terminated
}

// runAll executes every captured function, including any scheduled while
// running, and returns how many ran.
func (d *fakeDispatcher) runAll() int {
	ran := 0
	for {
		d.mu.Lock()
		if len(d.scheduled) == 0 {
			d.mu.Unlock()
			return ran
		}
		f := d.scheduled[0]
		d.scheduled = d.scheduled[1:]
		d.mu.Unlock()
		f()
		ran++
	}
}

func newBoundCoordinator() (*terminationCoordinator, *fakeDispatcher) {
	d := &fakeDispatcher{}
	c := newTerminationCoordinator()
	c.bind(d.dispatch, d.terminate)
	return c, d
}

// readyThroughDispatch mirrors desktopWebView.Run: readiness is scheduled on
// the UI thread and declared when that callback executes.
func readyThroughDispatch(c *terminationCoordinator, d *fakeDispatcher) {
	d.dispatch(c.ready)
	d.runAll()
}

func TestTerminationRequestBeforeBindStaysLatched(t *testing.T) {
	c := newTerminationCoordinator()
	c.request()
	assert.True(t, c.latched())

	d := &fakeDispatcher{}
	c.bind(d.dispatch, d.terminate)
	assert.True(t, c.latched(), "the latch survives binding so Run skips the native loop")
	assert.Zero(t, d.scheduledCount())
	assert.Zero(t, d.terminatedCount())

	// Run does not enter the loop; end is the only remaining transition.
	c.end()
	c.request()
	assert.Zero(t, d.scheduledCount())
	assert.Zero(t, d.terminatedCount())
}

func TestTerminationRequestAfterBindBeforeReadyTerminatesFromReadyCallback(t *testing.T) {
	c, d := newBoundCoordinator()
	assert.False(t, c.latched())

	d.dispatch(c.ready)
	c.request()
	c.request()
	assert.Equal(t, 1, d.scheduledCount(), "no premature work beyond the readiness bootstrap")
	assert.Zero(t, d.terminatedCount())

	require.Equal(t, 1, d.runAll())
	assert.Equal(t, 1, d.terminatedCount())
	assert.Zero(t, d.scheduledCount())
}

func TestTerminationRequestWhileReadyDispatchesOnce(t *testing.T) {
	c, d := newBoundCoordinator()
	readyThroughDispatch(c, d)
	assert.Zero(t, d.terminatedCount())

	c.request()
	c.request()
	assert.Equal(t, 1, d.scheduledCount())
	assert.Zero(t, d.terminatedCount(), "termination waits for the UI thread")

	require.Equal(t, 1, d.runAll())
	assert.Equal(t, 1, d.terminatedCount())

	c.request()
	assert.Zero(t, d.scheduledCount())
	d.runAll()
	assert.Equal(t, 1, d.terminatedCount())
}

func TestTerminationRequestRacingEndNeverTerminatesAfterEnd(t *testing.T) {
	for i := 0; i < 200; i++ {
		c, d := newBoundCoordinator()
		readyThroughDispatch(c, d)

		var barrier sync.WaitGroup
		barrier.Add(2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			barrier.Done()
			barrier.Wait()
			c.request()
		}()
		go func() {
			defer wg.Done()
			barrier.Done()
			barrier.Wait()
			c.end()
		}()
		wg.Wait()

		// the request either owned accepted work (scheduled before end) or was
		// ignored; captured work that runs after end must not terminate.
		assert.LessOrEqual(t, d.scheduledCount(), 1)
		d.runAll()
		assert.Zero(t, d.terminatedCount())
	}
}

func TestTerminationAcceptedWorkRunsBeforeEnd(t *testing.T) {
	c, d := newBoundCoordinator()
	readyThroughDispatch(c, d)
	c.request()
	require.Equal(t, 1, d.scheduledCount())
	require.Equal(t, 1, d.runAll())
	assert.Equal(t, 1, d.terminatedCount())
	c.end()
	c.request()
	assert.Zero(t, d.scheduledCount())
}

func TestTerminationRequestAfterEndIsIgnored(t *testing.T) {
	t.Run("after run end", func(t *testing.T) {
		c, d := newBoundCoordinator()
		readyThroughDispatch(c, d)
		c.end()
		c.request()
		assert.Zero(t, d.scheduledCount())
		assert.Zero(t, d.terminatedCount())
		assert.False(t, c.latched())
	})
	t.Run("destroy repeats end", func(t *testing.T) {
		c, d := newBoundCoordinator()
		readyThroughDispatch(c, d)
		c.end()
		c.end()
		c.request()
		assert.Zero(t, d.scheduledCount())
		assert.Zero(t, d.terminatedCount())
	})
	t.Run("ready after end stays ended", func(t *testing.T) {
		c, d := newBoundCoordinator()
		c.request()
		c.end()
		c.ready()
		assert.Zero(t, d.terminatedCount())
	})
}

func TestTerminationConcurrentRequestsTerminateAtMostOnce(t *testing.T) {
	c, d := newBoundCoordinator()
	readyThroughDispatch(c, d)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c.request()
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, 1, d.scheduledCount())
	d.runAll()
	assert.Equal(t, 1, d.terminatedCount())
}

func TestTerminationCloseRequestCloseFlowsThroughCoordinator(t *testing.T) {
	c, d := newBoundCoordinator()
	readyThroughDispatch(c, d)

	delivered := make(chan *CloseRequest, 1)
	controller := newCloseController(func(request *CloseRequest) { delivered <- request }, c.request)
	require.True(t, controller.requestClose())
	request := <-delivered
	request.Close()

	assert.Equal(t, 1, d.scheduledCount())
	d.runAll()
	assert.Equal(t, 1, d.terminatedCount())
	assert.False(t, controller.requestClose())
}
