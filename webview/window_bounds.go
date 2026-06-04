package webview

import (
	"unsafe"

	"github.com/michaelquigley/dfw/internal/core"
)

type nativeWindowBoundsTracker interface {
	Bounds() (core.WindowBounds, bool)
	Close()
}

type noopWindowBoundsTracker struct{}

func (noopWindowBoundsTracker) Bounds() (core.WindowBounds, bool) {
	return core.WindowBounds{}, false
}

func (noopWindowBoundsTracker) Close() {}

func validNativeWindow(window unsafe.Pointer) bool {
	return window != nil && uintptr(window) != 0
}
