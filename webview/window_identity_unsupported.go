//go:build !linux

package webview

import "unsafe"

func prepareNativeWindowIdentity(_ string) {}

func applyNativeWindowIdentity(_ unsafe.Pointer, _ string) {}
