//go:build !linux && !windows

package dfw

import (
	"image"
	"unsafe"
)

func newNativeWindowBoundsTracker(_ unsafe.Pointer) nativeWindowBoundsTracker {
	return noopWindowBoundsTracker{}
}

func applyNativeWindowSize(_ unsafe.Pointer, _ image.Point) bool {
	return false
}

func applyNativeWindowLocation(_ unsafe.Pointer, _ int, _ int) bool {
	return false
}
