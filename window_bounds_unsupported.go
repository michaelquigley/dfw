//go:build !linux && !windows

package dfw

import "unsafe"

func newNativeWindowBoundsTracker(_ unsafe.Pointer) nativeWindowBoundsTracker {
	return noopWindowBoundsTracker{}
}

func applyNativeWindowLocation(_ unsafe.Pointer, _ int, _ int) bool {
	return false
}
