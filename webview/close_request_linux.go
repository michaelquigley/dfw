//go:build linux

package webview

/*
#cgo pkg-config: gtk+-3.0

#include "close_request_linux.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"unsafe"
)

// this file exports a Go callback into C, so its preamble stays
// declaration-only; close_request_linux.c owns the signal handlers.

type linuxCloseInterceptor struct {
	interceptor *C.dfw_close_interceptor
	handle      cgo.Handle
}

func newNativeCloseInterceptor(window unsafe.Pointer, request func() bool) (nativeCloseInterceptor, error) {
	if !validNativeWindow(window) {
		return nil, errors.New("dfw: install close interceptor: no native window")
	}
	if request == nil {
		return nil, errors.New("dfw: install close interceptor: no request handler")
	}

	handle := cgo.NewHandle(request)
	interceptor := C.dfw_close_interceptor_install((*C.GtkWindow)(window), C.uintptr_t(handle))
	if interceptor == nil {
		handle.Delete()
		return nil, errors.New("dfw: install close interceptor: connect GTK delete-event")
	}
	return &linuxCloseInterceptor{interceptor: interceptor, handle: handle}, nil
}

// Close disconnects the GTK handlers before deleting the cgo handle, so no
// late signal can reach a freed handle.
func (i *linuxCloseInterceptor) Close() {
	if i == nil || i.interceptor == nil {
		return
	}
	C.dfw_close_interceptor_free(i.interceptor)
	i.interceptor = nil
	i.handle.Delete()
}

//export dfwLinuxCloseRequested
func dfwLinuxCloseRequested(handle C.uintptr_t) C.int {
	request, ok := cgo.Handle(handle).Value().(func() bool)
	if !ok || request == nil {
		return 0
	}
	if request() {
		return 1
	}
	return 0
}
