//go:build !linux

package webview

import (
	"fmt"
	"runtime"
	"unsafe"
)

// newNativeCloseInterceptor fails on platforms without close interception so
// a configured OnCloseRequest never degrades to the ordinary hard close.
func newNativeCloseInterceptor(_ unsafe.Pointer, _ func() bool) (nativeCloseInterceptor, error) {
	return nil, fmt.Errorf("dfw: close request interception is not supported on '%s'", runtime.GOOS)
}
