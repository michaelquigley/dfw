package dfw

import "unsafe"

type windowBounds struct {
	Width       int
	Height      int
	X           int
	Y           int
	HasLocation bool
}

type nativeWindowBoundsTracker interface {
	Bounds() (windowBounds, bool)
	Close()
}

type noopWindowBoundsTracker struct{}

func (noopWindowBoundsTracker) Bounds() (windowBounds, bool) {
	return windowBounds{}, false
}

func (noopWindowBoundsTracker) Close() {}

func validNativeWindow(window unsafe.Pointer) bool {
	return window != nil && uintptr(window) != 0
}
