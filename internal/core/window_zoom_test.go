package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWindowZoomSurvivesCloseAndReopen(t *testing.T) {
	withUserConfigDir(t, t.TempDir())
	percent := 110
	_, err := WriteWindowState("com.example.zoom", WindowState{Width: 1024, Height: 768, ZoomPercent: &percent})
	require.NoError(t, err)
	state, ok, err := readWindowState("com.example.zoom")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 110, ChooseInitialWindowZoom(state, ok))
}

func TestWindowZoomWithoutUsablePreference(t *testing.T) {
	require.Equal(t, 100, ChooseInitialWindowZoom(WindowState{Width: 1024, Height: 768}, true))
	for _, percent := range []int{0, -10, 105, 210} {
		require.Equal(t, 100, ChooseInitialWindowZoom(WindowState{ZoomPercent: &percent}, true))
	}
}
