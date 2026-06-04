package core

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

var minimumRestoredWindowSize = image.Pt(320, 240)

// WindowState is the persisted size and (optional) location of a window.
type WindowState struct {
	Width  int
	Height int
	X      *int
	Y      *int
}

// LoadWindowState reads the persisted window state for appID, returning false
// when no usable state exists.
func LoadWindowState(appID string) (WindowState, bool) {
	if strings.TrimSpace(appID) == "" {
		return WindowState{}, false
	}

	state, ok, err := readWindowState(appID)
	if err != nil {
		dl.Errorf("dfw: read window state: %v", err)
		return WindowState{}, false
	}
	return state, ok
}

func readWindowState(appID string) (WindowState, bool, error) {
	path, err := windowStatePath(appID)
	if err != nil {
		return WindowState{}, false, err
	}

	state := WindowState{}
	if err := dd.BindJSONFile(&state, path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WindowState{}, false, nil
		}
		return WindowState{}, false, fmt.Errorf("dfw: bind window state: %w", err)
	}
	if !state.validSize() {
		return WindowState{}, false, nil
	}
	return state, true, nil
}

// WriteWindowState persists state for appID and returns the file path. A state
// with a non-positive size is ignored and returns an empty path.
func WriteWindowState(appID string, state WindowState) (string, error) {
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

// ChooseInitialWindowSize returns the size to open a window at, preferring a
// restorable persisted size over the initial size.
func ChooseInitialWindowSize(initial image.Point, state WindowState, hasState bool) image.Point {
	if hasState && state.restorableSize(initial) {
		return image.Pt(state.Width, state.Height)
	}
	return initial
}

// ChooseInitialWindowLocation returns the persisted window location, if any.
func ChooseInitialWindowLocation(state WindowState, hasState bool) (int, int, bool) {
	if !hasState || !state.hasLocation() {
		return 0, 0, false
	}
	return *state.X, *state.Y, true
}

// WindowStateFromBounds converts a native bounds snapshot into a WindowState.
func WindowStateFromBounds(bounds WindowBounds) WindowState {
	state := WindowState{
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

func (s WindowState) validSize() bool {
	return s.Width > 0 && s.Height > 0
}

func (s WindowState) restorableSize(initial image.Point) bool {
	if !s.validSize() {
		return false
	}

	minimum := minimumRestoredWindowSize
	if initial.X > 0 && initial.X < minimum.X {
		minimum.X = initial.X
	}
	if initial.Y > 0 && initial.Y < minimum.Y {
		minimum.Y = initial.Y
	}

	return s.Width >= minimum.X && s.Height >= minimum.Y
}

func (s WindowState) hasLocation() bool {
	return s.X != nil && s.Y != nil
}
