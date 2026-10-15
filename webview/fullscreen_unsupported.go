//go:build !linux

package webview

import "unsafe"

func newNativeFullscreen(window unsafe.Pointer, initial bool) nativeFullscreenController { return nil }
