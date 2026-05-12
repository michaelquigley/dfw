//go:build windows

package dfw

import (
	"sync"
	"syscall"
	"unsafe"
)

const (
	wmMove             = 0x0003
	wmSize             = 0x0005
	wmClose            = 0x0010
	wmDestroy          = 0x0002
	wmNCDestroy        = 0x0082
	wmWindowPosChanged = 0x0047

	monitorDefaultToNearest = 2

	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

var (
	gwlpWndProc = ^uintptr(3)

	boundsWindowProc = syscall.NewCallback(windowBoundsWndProc)
	windowsTrackers  sync.Map

	procCallWindowProcW    = user32.NewProc("CallWindowProcW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procGetDpiForWindow    = user32.NewProc("GetDpiForWindow")
	procGetMonitorInfoW    = user32.NewProc("GetMonitorInfoW")
	procGetWindowRect      = user32.NewProc("GetWindowRect")
	procMonitorFromRect    = user32.NewProc("MonitorFromRect")
	procSetWindowLongPtrW  = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPosBounds = user32.NewProc("SetWindowPos")
)

type windowsWindowBoundsTracker struct {
	hwnd    uintptr
	oldProc uintptr

	mu        sync.Mutex
	bounds    windowBounds
	ok        bool
	destroyed bool
	closed    bool
}

type windowsRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type windowsMonitorInfo struct {
	Size    uint32
	Monitor windowsRect
	Work    windowsRect
	Flags   uint32
}

func newNativeWindowBoundsTracker(window unsafe.Pointer) nativeWindowBoundsTracker {
	if !validNativeWindow(window) {
		return noopWindowBoundsTracker{}
	}

	tracker := &windowsWindowBoundsTracker{hwnd: uintptr(window)}
	tracker.capture()

	windowsTrackers.Store(tracker.hwnd, tracker)
	oldProc, _, _ := procSetWindowLongPtrW.Call(tracker.hwnd, gwlpWndProc, boundsWindowProc)
	if oldProc == 0 {
		windowsTrackers.Delete(tracker.hwnd)
		return tracker
	}

	tracker.mu.Lock()
	tracker.oldProc = oldProc
	tracker.mu.Unlock()
	return tracker
}

func (t *windowsWindowBoundsTracker) Bounds() (windowBounds, bool) {
	if t == nil {
		return windowBounds{}, false
	}

	t.mu.Lock()
	destroyed := t.destroyed
	t.mu.Unlock()

	if !destroyed {
		t.capture()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bounds, t.ok
}

func (t *windowsWindowBoundsTracker) Close() {
	if t == nil {
		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	hwnd := t.hwnd
	oldProc := t.oldProc
	destroyed := t.destroyed
	t.mu.Unlock()

	if !destroyed {
		t.capture()
	}
	windowsTrackers.Delete(hwnd)
	if !destroyed && hwnd != 0 && oldProc != 0 {
		procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, oldProc)
	}
}

func (t *windowsWindowBoundsTracker) capture() {
	if t == nil || t.hwnd == 0 {
		return
	}

	var client windowsRect
	ok, _, _ := procGetClientRect.Call(t.hwnd, uintptr(unsafe.Pointer(&client)))
	if ok == 0 {
		return
	}

	width := int(client.Right - client.Left)
	height := int(client.Bottom - client.Top)
	if width <= 0 || height <= 0 {
		return
	}

	dpi := windowDPI(t.hwnd)
	bounds := windowBounds{
		Width:  scaleToDefaultDPI(width, dpi),
		Height: scaleToDefaultDPI(height, dpi),
	}

	var rect windowsRect
	if ok, _, _ := procGetWindowRect.Call(t.hwnd, uintptr(unsafe.Pointer(&rect))); ok != 0 {
		bounds.X = int(rect.Left)
		bounds.Y = int(rect.Top)
		bounds.HasLocation = true
	}

	t.mu.Lock()
	t.bounds = bounds
	t.ok = true
	t.mu.Unlock()
}

func (t *windowsWindowBoundsTracker) markDestroyed() {
	t.mu.Lock()
	t.destroyed = true
	t.mu.Unlock()
}

func windowBoundsWndProc(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr) uintptr {
	var oldProc uintptr
	if value, ok := windowsTrackers.Load(hwnd); ok {
		tracker := value.(*windowsWindowBoundsTracker)
		tracker.mu.Lock()
		oldProc = tracker.oldProc
		tracker.mu.Unlock()

		switch msg {
		case wmMove, wmSize, wmWindowPosChanged, wmClose, wmDestroy, wmNCDestroy:
			tracker.capture()
		}
		if msg == wmDestroy || msg == wmNCDestroy {
			tracker.markDestroyed()
		}
		if msg == wmNCDestroy {
			windowsTrackers.Delete(hwnd)
		}
	}

	if oldProc != 0 {
		ret, _, _ := procCallWindowProcW.Call(oldProc, hwnd, uintptr(msg), wparam, lparam)
		return ret
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
}

func applyNativeWindowLocation(window unsafe.Pointer, x int, y int) bool {
	if !validNativeWindow(window) {
		return false
	}

	hwnd := uintptr(window)
	x, y = clampWindowLocation(hwnd, x, y)
	ok, _, _ := procSetWindowPosBounds.Call(
		hwnd,
		0,
		uintptr(x),
		uintptr(y),
		0,
		0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
	return ok != 0
}

func clampWindowLocation(hwnd uintptr, x int, y int) (int, int) {
	var rect windowsRect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return x, y
	}

	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)
	if width <= 0 || height <= 0 {
		return x, y
	}

	desired := windowsRect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x + width),
		Bottom: int32(y + height),
	}
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&desired)), monitorDefaultToNearest)
	if monitor == 0 {
		return x, y
	}

	info := windowsMonitorInfo{Size: uint32(unsafe.Sizeof(windowsMonitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return x, y
	}

	minX := int(info.Work.Left)
	minY := int(info.Work.Top)
	maxX := int(info.Work.Right) - width
	maxY := int(info.Work.Bottom) - height
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	return clampInt(x, minX, maxX), clampInt(y, minY, maxY)
}

func windowDPI(hwnd uintptr) int {
	if err := procGetDpiForWindow.Find(); err == nil {
		dpi, _, _ := procGetDpiForWindow.Call(hwnd)
		if dpi > 0 {
			return int(dpi)
		}
	}
	return 96
}

func scaleToDefaultDPI(value int, dpi int) int {
	if dpi <= 0 {
		return value
	}
	return (value*96 + dpi/2) / dpi
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
