//go:build !darwin

package webview

import "unsafe"

// the application-level setup is macOS's alone: GTK and Win32 windows get
// their keyboard handling, focus and menus without it.

func prepareNativeApp() {}

func releaseNativeApp() {}

func newNativeApp(_ unsafe.Pointer) nativeAppController { return nil }
