package webview

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const closeTestWait = 5 * time.Second

// closeTestHarness drives a closeController without a native window.
type closeTestHarness struct {
	controller *closeController
	delivered  chan *CloseRequest
	terminated chan struct{}
	release    chan struct{}
}

func newCloseTestHarness(t *testing.T) *closeTestHarness {
	t.Helper()
	h := &closeTestHarness{
		delivered:  make(chan *CloseRequest, 16),
		terminated: make(chan struct{}, 16),
		release:    make(chan struct{}),
	}
	h.controller = newCloseController(func(request *CloseRequest) {
		h.delivered <- request
		<-h.release
	}, func() {
		h.terminated <- struct{}{}
	})
	return h
}

func (h *closeTestHarness) request(t *testing.T) *CloseRequest {
	t.Helper()
	require.True(t, h.controller.requestClose(), "native request must be consumed")
	select {
	case request := <-h.delivered:
		return request
	case <-time.After(closeTestWait):
		t.Fatal("callback was not dispatched")
		return nil
	}
}

func (h *closeTestHarness) assertNoDelivery(t *testing.T) {
	t.Helper()
	select {
	case <-h.delivered:
		t.Fatal("callback was dispatched unexpectedly")
	case <-time.After(50 * time.Millisecond):
	}
}

func (h *closeTestHarness) assertTerminated(t *testing.T, times int) {
	t.Helper()
	for i := 0; i < times; i++ {
		select {
		case <-h.terminated:
		case <-time.After(closeTestWait):
			t.Fatalf("terminate call %d did not happen", i+1)
		}
	}
	select {
	case <-h.terminated:
		t.Fatal("terminate was called more than expected")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCloseRequestNativeRequestReturnsWhileCallbackBlocks(t *testing.T) {
	h := newCloseTestHarness(t)
	defer close(h.release)

	done := make(chan bool, 1)
	go func() { done <- h.controller.requestClose() }()
	select {
	case consumed := <-done:
		assert.True(t, consumed)
	case <-time.After(closeTestWait):
		t.Fatal("native request blocked behind the callback")
	}
	h.request(t)
}

func TestCloseRequestRepeatedNativeRequestsAreDropped(t *testing.T) {
	h := newCloseTestHarness(t)
	defer close(h.release)

	first := h.request(t)
	require.NotNil(t, first)
	for i := 0; i < 3; i++ {
		assert.True(t, h.controller.requestClose(), "repeated request must still be consumed")
	}
	h.assertNoDelivery(t)
	h.assertTerminated(t, 0)
}

func TestCloseRequestUnresolvedCallbackLeavesRequestPending(t *testing.T) {
	h := newCloseTestHarness(t)
	close(h.release)

	h.request(t)
	// the callback has returned without resolving; the request is still pending.
	assert.True(t, h.controller.requestClose())
	h.assertNoDelivery(t)
}

func TestCloseRequestKeepOpenRearms(t *testing.T) {
	h := newCloseTestHarness(t)
	close(h.release)

	first := h.request(t)
	first.KeepOpen()
	second := h.request(t)
	assert.NotSame(t, first, second)
	assert.NotEqual(t, first.generation, second.generation)
	h.assertTerminated(t, 0)
}

func TestCloseRequestStaleHandleCannotResolveNewRequest(t *testing.T) {
	h := newCloseTestHarness(t)
	close(h.release)

	first := h.request(t)
	first.KeepOpen()
	second := h.request(t)

	first.Close()
	h.assertTerminated(t, 0)
	first.KeepOpen()
	// the second request is still pending: a repeated native request is dropped.
	assert.True(t, h.controller.requestClose())
	h.assertNoDelivery(t)

	second.Close()
	h.assertTerminated(t, 1)
}

func TestCloseRequestCloseTerminatesOnceAndAllowsLaterNativeClose(t *testing.T) {
	h := newCloseTestHarness(t)
	close(h.release)

	request := h.request(t)
	request.Close()
	h.assertTerminated(t, 1)

	assert.False(t, h.controller.requestClose(), "native close must proceed after consent")
	h.assertNoDelivery(t)
	request.Close()
	h.assertTerminated(t, 0)
}

func TestCloseRequestFirstResolutionWins(t *testing.T) {
	t.Run("close then keep open", func(t *testing.T) {
		h := newCloseTestHarness(t)
		close(h.release)
		request := h.request(t)
		request.Close()
		request.KeepOpen()
		h.assertTerminated(t, 1)
		assert.False(t, h.controller.requestClose())
	})
	t.Run("keep open then close", func(t *testing.T) {
		h := newCloseTestHarness(t)
		close(h.release)
		request := h.request(t)
		request.KeepOpen()
		request.Close()
		h.assertTerminated(t, 0)
		h.request(t)
	})
	t.Run("competing goroutines", func(t *testing.T) {
		h := newCloseTestHarness(t)
		close(h.release)
		request := h.request(t)

		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				if i%2 == 0 {
					request.Close()
				} else {
					request.KeepOpen()
				}
			}(i)
		}
		close(start)
		wg.Wait()

		h.controller.mu.Lock()
		closing := h.controller.closing
		pending := h.controller.pending
		h.controller.mu.Unlock()
		assert.Zero(t, pending)
		if closing {
			h.assertTerminated(t, 1)
		} else {
			h.assertTerminated(t, 0)
		}
	})
}

func TestCloseRequestEndExpiresPendingRequest(t *testing.T) {
	h := newCloseTestHarness(t)
	close(h.release)

	request := h.request(t)
	h.controller.end()

	request.Close()
	request.KeepOpen()
	h.assertTerminated(t, 0)
	assert.False(t, h.controller.requestClose(), "native close proceeds after the lifecycle ends")
	h.assertNoDelivery(t)
	h.controller.end()
}

func TestCloseRequestNilAndZeroValueAreInert(t *testing.T) {
	var nilRequest *CloseRequest
	nilRequest.Close()
	nilRequest.KeepOpen()

	var zero CloseRequest
	zero.Close()
	zero.KeepOpen()
}

func TestCloseRequestEndRacesResolution(t *testing.T) {
	for _, resolve := range []struct {
		name string
		fn   func(*CloseRequest)
	}{
		{name: "close", fn: (*CloseRequest).Close},
		{name: "keep open", fn: (*CloseRequest).KeepOpen},
	} {
		t.Run(resolve.name, func(t *testing.T) {
			for _, endFirst := range []bool{true, false} {
				h := newCloseTestHarness(t)
				close(h.release)
				request := h.request(t)

				var barrier sync.WaitGroup
				barrier.Add(2)
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					barrier.Done()
					barrier.Wait()
					if endFirst {
						h.controller.end()
					} else {
						resolve.fn(request)
					}
				}()
				go func() {
					defer wg.Done()
					barrier.Done()
					barrier.Wait()
					if endFirst {
						resolve.fn(request)
					} else {
						h.controller.end()
					}
				}()
				wg.Wait()

				// whichever ordering won, the controller is ended and no later
				// native request can reach the callback.
				assert.False(t, h.controller.requestClose())
				h.assertNoDelivery(t)
				select {
				case <-h.terminated:
				case <-time.After(50 * time.Millisecond):
				}
			}
		})
	}
}
