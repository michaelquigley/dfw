package dfw

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/df/dl"
)

const windowStateJSON = "window_state.json"

type windowState struct {
	Width  int
	Height int
	X      *int
	Y      *int
}

func loadWindowState(appID string) (windowState, bool) {
	if strings.TrimSpace(appID) == "" {
		return windowState{}, false
	}

	state, ok, err := readWindowState(appID)
	if err != nil {
		dl.Errorf("dfw: read window state: %v", err)
		return windowState{}, false
	}
	return state, ok
}

func readWindowState(appID string) (windowState, bool, error) {
	path, err := windowStatePath(appID)
	if err != nil {
		return windowState{}, false, err
	}

	state := windowState{}
	if err := dd.BindJSONFile(&state, path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return windowState{}, false, nil
		}
		return windowState{}, false, fmt.Errorf("dfw: bind window state: %w", err)
	}
	if !state.validSize() {
		return windowState{}, false, nil
	}
	return state, true, nil
}

func writeWindowState(appID string, state windowState) (string, error) {
	if !state.validSize() {
		return "", nil
	}

	path, err := windowStatePath(appID)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("dfw: create runtime directory: %w", err)
	}

	if err := dd.UnbindJSONFile(state, path); err != nil {
		return "", fmt.Errorf("dfw: unbind window state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("dfw: set window state permissions: %w", err)
	}

	return path, nil
}

func windowStatePath(appID string) (string, error) {
	base, err := userConfigPath(appID)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, runtimeDir, windowStateJSON), nil
}

func chooseInitialWindowSize(initial image.Point, state windowState, hasState bool) image.Point {
	if hasState && state.validSize() {
		return image.Pt(state.Width, state.Height)
	}
	return initial
}

func chooseInitialWindowLocation(state windowState, hasState bool) (int, int, bool) {
	if !hasState || !state.hasLocation() {
		return 0, 0, false
	}
	return *state.X, *state.Y, true
}

func windowStateFromBounds(bounds windowBounds) windowState {
	state := windowState{
		Width:  bounds.Width,
		Height: bounds.Height,
	}
	if bounds.HasLocation {
		x := bounds.X
		y := bounds.Y
		state.X = &x
		state.Y = &y
	}
	return state
}

func (s windowState) validSize() bool {
	return s.Width > 0 && s.Height > 0
}

func (s windowState) hasLocation() bool {
	return s.X != nil && s.Y != nil
}
