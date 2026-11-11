package tray

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingBackend struct {
	mu    sync.Mutex
	calls []string
}

func (b *recordingBackend) record(format string, args ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, fmt.Sprintf(format, args...))
}

func (b *recordingBackend) SetIcon(icon []byte)       { b.record("icon %d", len(icon)) }
func (b *recordingBackend) SetTooltip(tooltip string) { b.record("tooltip %s", tooltip) }
func (b *recordingBackend) SetItemLabel(index int, label string) {
	b.record("label %d %s", index, label)
}
func (b *recordingBackend) SetItemEnabled(index int, enabled bool) {
	b.record("enabled %d %t", index, enabled)
}

func (b *recordingBackend) snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4))))
	return buf.Bytes()
}

func countingStop() (func(), *atomic.Int32) {
	var count atomic.Int32
	return func() { count.Add(1) }, &count
}

func TestControlRecordsChangesUntilBound(t *testing.T) {
	control := NewControl()
	icon := testPNG(t)

	require.NoError(t, control.SetIcon(icon))
	control.SetTooltip("first")
	control.SetTooltip("busy")
	control.Item(0).SetLabel("Working")
	control.Item(0).Disable()
	control.Item(1).Enable()
	control.Item(5).SetLabel("out of range")
	control.Item(-1).SetLabel("negative")

	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	control.Item(1).SetLabel("claimed")

	backend := &recordingBackend{}
	control.bind(backend, 2)

	calls := backend.snapshot()
	assert.ElementsMatch(t, []string{
		fmt.Sprintf("icon %d", len(icon)),
		"tooltip busy",
		"label 0 Working",
		"enabled 0 false",
		"label 1 claimed",
		"enabled 1 true",
	}, calls)
	assert.Equal(t, fmt.Sprintf("icon %d", len(icon)), calls[0])
	assert.Equal(t, "tooltip busy", calls[1])
}

func TestControlAppliesChangesWhileLive(t *testing.T) {
	control := NewControl()
	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	backend := &recordingBackend{}
	control.bind(backend, 1)

	control.SetTooltip("idle")
	control.Item(0).SetLabel("Ready")
	control.Item(0).Enable()
	control.Item(0).Disable()
	control.Item(1).SetLabel("ignored")
	control.Item(-1).Disable()

	assert.Equal(t, []string{
		"tooltip idle",
		"label 0 Ready",
		"enabled 0 true",
		"enabled 0 false",
	}, backend.snapshot())
}

func TestControlInertAfterEnd(t *testing.T) {
	control := NewControl()
	stop, stops := countingStop()
	require.NoError(t, control.claim(stop))
	backend := &recordingBackend{}
	control.bind(backend, 1)
	control.end()
	control.end()

	require.NoError(t, control.SetIcon(testPNG(t)))
	control.SetTooltip("late")
	control.Item(0).SetLabel("late")
	control.Item(0).Enable()
	control.Stop()

	assert.Empty(t, backend.snapshot())
	assert.Equal(t, int32(0), stops.Load())
}

func TestControlEndBeforeBindDropsTheBind(t *testing.T) {
	control := NewControl()
	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	control.SetTooltip("recorded")
	control.end()

	backend := &recordingBackend{}
	control.bind(backend, 1)
	control.SetTooltip("late")

	assert.Empty(t, backend.snapshot())
}

func TestControlStop(t *testing.T) {
	t.Run("before claim is delivered at claim", func(t *testing.T) {
		control := NewControl()
		control.Stop()
		control.Stop()

		stop, stops := countingStop()
		require.NoError(t, control.claim(stop))
		assert.Equal(t, int32(1), stops.Load())
	})

	t.Run("while claimed reaches the daemon", func(t *testing.T) {
		control := NewControl()
		stop, stops := countingStop()
		require.NoError(t, control.claim(stop))
		control.Stop()
		assert.Equal(t, int32(1), stops.Load())
	})

	t.Run("while live reaches the daemon", func(t *testing.T) {
		control := NewControl()
		stop, stops := countingStop()
		require.NoError(t, control.claim(stop))
		control.bind(&recordingBackend{}, 0)
		control.Stop()
		assert.Equal(t, int32(1), stops.Load())
	})

	t.Run("may end the control re-entrantly", func(t *testing.T) {
		control := NewControl()
		require.NoError(t, control.claim(func() { control.end() }))
		backend := &recordingBackend{}
		control.bind(backend, 1)
		control.Stop()
		control.SetTooltip("late")
		assert.Empty(t, backend.snapshot())
	})
}

func TestControlServesOneDaemon(t *testing.T) {
	control := NewControl()
	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	assert.ErrorIs(t, control.claim(stop), errControlReused)

	control.end()
	assert.ErrorIs(t, control.claim(stop), errControlReused)
}

func TestDaemonRejectsReusedControlBeforeListening(t *testing.T) {
	control := NewControl()
	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	control.end()

	listened := false
	err := Daemon(DaemonApp{
		AppID:   "com.quigley.dfw.test.reused",
		Control: control,
		Listen: func() (*http.Server, net.Listener, error) {
			listened = true
			return nil, nil, errors.New("listen should not be reached")
		},
	})
	assert.ErrorIs(t, err, errControlReused)
	assert.False(t, listened)
}

func TestControlSetIconRejectsBadPNG(t *testing.T) {
	control := NewControl()
	assert.Error(t, control.SetIcon(nil))
	assert.Error(t, control.SetIcon([]byte("not a png")))

	stop, _ := countingStop()
	require.NoError(t, control.claim(stop))
	backend := &recordingBackend{}
	control.bind(backend, 0)
	assert.Error(t, control.SetIcon([]byte("not a png")))
	assert.Empty(t, backend.snapshot())
}

func TestControlNilReceiver(t *testing.T) {
	var control *Control
	assert.NoError(t, control.SetIcon(nil))
	control.SetTooltip("x")
	control.Stop()
	assert.Nil(t, control.Item(0))
	control.Item(0).SetLabel("x")
	control.Item(0).Enable()
	control.Item(0).Disable()
	assert.NoError(t, control.claim(func() {}))
	control.bind(&recordingBackend{}, 0)
	control.end()

	var item *Item
	item.SetLabel("x")
	item.Enable()
	item.Disable()
}

// run with -race: changes and Stop from many goroutines against bind and end.
func TestControlConcurrentUse(t *testing.T) {
	control := NewControl()
	icon := testPNG(t)
	var stops atomic.Int32
	require.NoError(t, control.claim(func() { stops.Add(1) }))
	backend := &recordingBackend{}

	var wg sync.WaitGroup
	for n := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_ = control.SetIcon(icon)
				control.SetTooltip(fmt.Sprint(n))
				control.Item(n % 2).SetLabel(fmt.Sprint(n))
				control.Item(n % 2).Disable()
			}
			control.Stop()
		}()
	}
	control.bind(backend, 2)
	wg.Wait()
	control.end()

	assert.Positive(t, stops.Load())
}

func TestRunQuitItem(t *testing.T) {
	t.Run("nil OnQuit quits on the first click", func(t *testing.T) {
		clicked := make(chan struct{}, 2)
		clicked <- struct{}{}
		clicked <- struct{}{}
		quits := 0
		runQuitItem(clicked, nil, func() { quits++ })
		assert.Equal(t, 1, quits)
		assert.Len(t, clicked, 1)
	})

	t.Run("false keeps running until true", func(t *testing.T) {
		clicked := make(chan struct{}, 3)
		for range 3 {
			clicked <- struct{}{}
		}
		answers := []bool{false, false, true}
		asked, quits := 0, 0
		runQuitItem(clicked, func() bool {
			answer := answers[asked]
			asked++
			return answer
		}, func() { quits++ })
		assert.Equal(t, 3, asked)
		assert.Equal(t, 1, quits)
	})

	t.Run("false never quits", func(t *testing.T) {
		clicked := make(chan struct{}, 2)
		clicked <- struct{}{}
		clicked <- struct{}{}
		close(clicked)
		quits := 0
		runQuitItem(clicked, func() bool { return false }, func() { quits++ })
		assert.Equal(t, 0, quits)
	})
}
