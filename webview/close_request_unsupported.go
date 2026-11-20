//go:build !linux && !windows

package webview

import (
	"fmt"
	"runtime"
	"unsafe"
)

// checkNativeCloseInterception fails on platforms without close interception,
// before any window exists, so a configured OnCloseRequest never opens a
// window that closes without asking, not even for a moment.
func checkNativeCloseInterception() error {
	return fmt.Errorf("dfw: close request interception is not supported on '%s'", runtime.GOOS)
}

// newNativeCloseInterceptor fails the same way, behind the check above, so a
// configured OnCloseRequest never degrades to the ordinary hard close.
func newNativeCloseInterceptor(_ unsafe.Pointer, _ func() bool) (nativeCloseInterceptor, error) {
	return nil, checkNativeCloseInterception()
}
