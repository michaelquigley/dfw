//go:build windows

package webview

import (
	"errors"
	"unsafe"
)

// windowsCloseInterceptor attaches close interception to the bounds tracker's
// existing window procedure. the tracker remains the owner of restoring the
// original procedure; the interceptor only sets and clears the hook.
type windowsCloseInterceptor struct {
	tracker *windowsWindowBoundsTracker
}

func newNativeCloseInterceptor(window unsafe.Pointer, request func() bool) (nativeCloseInterceptor, error) {
	if !validNativeWindow(window) {
		return nil, errors.New("dfw: install close interceptor on 'windows': no native window")
	}
	if request == nil {
		return nil, errors.New("dfw: install close interceptor on 'windows': no request handler")
	}

	value, ok := windowsTrackers.Load(uintptr(window))
	if !ok {
		return nil, errors.New("dfw: install close interceptor on 'windows': no window procedure for HWND")
	}
	tracker := value.(*windowsWindowBoundsTracker)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.oldProc == 0 || tracker.closed || tracker.destroyed {
		return nil, errors.New("dfw: install close interceptor on 'windows': window procedure is not subclassed")
	}
	tracker.closeRequest = request
	return &windowsCloseInterceptor{tracker: tracker}, nil
}

// Close clears the hook so later WM_CLOSE traffic forwards to the original
// window procedure. it does not restore that procedure; the tracker does.
func (i *windowsCloseInterceptor) Close() {
	if i == nil || i.tracker == nil {
		return
	}
	i.tracker.mu.Lock()
	i.tracker.closeRequest = nil
	i.tracker.mu.Unlock()
	i.tracker = nil
}
