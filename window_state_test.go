package dfw

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowStateRoundTrip(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	x := 120
	y := 80
	expectedPath := filepath.Join(configDir, "com.example.app", "runtime", "window_state.json")

	path, err := writeWindowState("com.example.app", windowState{
		Width:  1024,
		Height: 768,
		X:      &x,
		Y:      &y,
	})
	require.NoError(t, err)
	assert.Equal(t, expectedPath, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	raw := map[string]any{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, map[string]any{
		"width":  float64(1024),
		"height": float64(768),
		"x":      float64(120),
		"y":      float64(80),
	}, raw)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	state, ok, err := readWindowState("com.example.app")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 1024, state.Width)
	assert.Equal(t, 768, state.Height)
	require.NotNil(t, state.X)
	require.NotNil(t, state.Y)
	assert.Equal(t, 120, *state.X)
	assert.Equal(t, 80, *state.Y)
}

func TestWindowStateOmitsMissingLocation(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	path, err := writeWindowState("com.example.app", windowState{
		Width:  1024,
		Height: 768,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	raw := map[string]any{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.NotContains(t, raw, "x")
	assert.NotContains(t, raw, "y")
}

func TestWindowStateMissingFileFallsBack(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	state, ok, err := readWindowState("com.example.app")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, windowState{}, state)
}

func TestWindowStateMalformedFileFallsBack(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	path, err := windowStatePath("com.example.app")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))

	state, ok := loadWindowState("com.example.app")
	assert.False(t, ok)
	assert.Equal(t, windowState{}, state)
}

func TestWindowStateInvalidSizeFallsBack(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	path, err := windowStatePath("com.example.app")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"width":0,"height":600}`), 0o600))

	state, ok, err := readWindowState("com.example.app")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, windowState{}, state)
}

func TestWindowStatePathShape(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	path, err := windowStatePath("com.example.app")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, "com.example.app", "runtime", "window_state.json"), path)
}

func TestChooseInitialWindowSize(t *testing.T) {
	initial := image.Pt(800, 600)

	assert.Equal(t, initial, chooseInitialWindowSize(initial, windowState{}, false))
	assert.Equal(t, initial, chooseInitialWindowSize(initial, windowState{Width: -1, Height: 700}, true))
	assert.Equal(t, image.Pt(1200, 900), chooseInitialWindowSize(initial, windowState{Width: 1200, Height: 900}, true))
}

func TestChooseInitialWindowLocation(t *testing.T) {
	x := 20
	y := 40

	_, _, ok := chooseInitialWindowLocation(windowState{}, false)
	assert.False(t, ok)

	_, _, ok = chooseInitialWindowLocation(windowState{Width: 800, Height: 600, X: &x}, true)
	assert.False(t, ok)

	actualX, actualY, ok := chooseInitialWindowLocation(windowState{Width: 800, Height: 600, X: &x, Y: &y}, true)
	require.True(t, ok)
	assert.Equal(t, 20, actualX)
	assert.Equal(t, 40, actualY)
}

func TestWindowStateFromBounds(t *testing.T) {
	state := windowStateFromBounds(windowBounds{
		Width:       900,
		Height:      700,
		X:           11,
		Y:           22,
		HasLocation: true,
	})

	assert.Equal(t, 900, state.Width)
	assert.Equal(t, 700, state.Height)
	require.NotNil(t, state.X)
	require.NotNil(t, state.Y)
	assert.Equal(t, 11, *state.X)
	assert.Equal(t, 22, *state.Y)
}
