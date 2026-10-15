package webview

import (
	"testing"

	"github.com/michaelquigley/dfw/internal/core"
	"github.com/stretchr/testify/require"
)

type fakeBoundsTracker struct {
	bounds core.WindowBounds
	ok     bool
}

func (f fakeBoundsTracker) Bounds() (core.WindowBounds, bool) { return f.bounds, f.ok }
func (fakeBoundsTracker) Close()                              {}

type fakeZoom int

func (f fakeZoom) Percent() int { return int(f) }
func (fakeZoom) Close()         {}

type fakeFullscreen bool

func (f fakeFullscreen) Fullscreen() bool { return bool(f) }
func (fakeFullscreen) Close()             {}

// saveTestWindow is a window with the given controllers, opened windowed at
// 1024x768 at (10, 20), saving under a temporary config directory.
func saveTestWindow(t *testing.T, tracker fakeBoundsTracker, fullscreen nativeFullscreenController) *desktopWebView {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	x, y := 10, 20
	return &desktopWebView{
		appID:         "com.example.fullscreen",
		boundsTracker: tracker,
		zoom:          fakeZoom(120),
		fullscreen:    fullscreen,
		windowed:      core.WindowState{Width: 1024, Height: 768, X: &x, Y: &y},
	}
}

func savedState(t *testing.T) core.WindowState {
	t.Helper()
	state, ok := core.LoadWindowState("com.example.fullscreen")
	require.True(t, ok, "window state must be written")
	return state
}

var screenBounds = fakeBoundsTracker{bounds: core.WindowBounds{Width: 1920, Height: 1080, X: 0, Y: 0, HasLocation: true}, ok: true}

func TestSaveWindowStateWhileFullscreenKeepsWindowedBounds(t *testing.T) {
	saveTestWindow(t, screenBounds, fakeFullscreen(true)).SaveWindowState()
	state := savedState(t)
	require.Equal(t, 1024, state.Width)
	require.Equal(t, 768, state.Height)
	require.Equal(t, 10, *state.X)
	require.Equal(t, 20, *state.Y)
	require.Equal(t, 120, *state.ZoomPercent)
}

func TestSaveWindowStateWhileFullscreenWithoutBounds(t *testing.T) {
	saveTestWindow(t, fakeBoundsTracker{}, fakeFullscreen(true)).SaveWindowState()
	state := savedState(t)
	require.Equal(t, 1024, state.Width)
	require.Equal(t, 120, *state.ZoomPercent)
}

func TestSaveWindowStateWindowedSavesBounds(t *testing.T) {
	for name, fullscreen := range map[string]nativeFullscreenController{"windowed": fakeFullscreen(false), "no controller": nil} {
		t.Run(name, func(t *testing.T) {
			saveTestWindow(t, screenBounds, fullscreen).SaveWindowState()
			state := savedState(t)
			require.Equal(t, 1920, state.Width)
			require.Equal(t, 1080, state.Height)
			require.Equal(t, 0, *state.X)
			require.Equal(t, 120, *state.ZoomPercent)
		})
	}
}

func TestNativeFullscreenWithoutWindowIsInert(t *testing.T) {
	require.Nil(t, newNativeFullscreen(nil, true))
}

func TestFullscreenIsOptIn(t *testing.T) {
	require.False(t, webviewConfig{}.fullscreenEnabled(), "an unset window leaves F11 to its page")
	require.False(t, webviewConfig{EnableZoom: true}.fullscreenEnabled())
	require.True(t, webviewConfig{EnableFullscreen: true}.fullscreenEnabled())
	require.True(t, webviewConfig{Fullscreen: true}.fullscreenEnabled())
}
