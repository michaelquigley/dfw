//go:build !darwin

package webview

import "unsafe"

// the application-level setup is macOS's alone: GTK and Win32 windows get
// their keyboard handling, focus and menus without it.

func prepareNativeApp() {}

func applyNativeApp(_ unsafe.Pointer) {}

func detachNativeApp() {}

func activateNativeApp(_ unsafe.Pointer) {}

func endNativeActivation() {}
