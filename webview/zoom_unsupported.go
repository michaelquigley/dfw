//go:build !linux

package webview

import "unsafe"

func newNativeZoom(window unsafe.Pointer, percent int) nativeZoomController { return nil }
