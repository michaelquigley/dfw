package webview

import (
	"errors"
	"image"
	"strings"

	webview "centrifuge.hectabit.org/HectaBit/webview_go"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/internal/core"
)

type webviewConfig struct {
	AppID          string
	Title          string
	InitialSize    image.Point
	IconPNG        []byte
	Debug          bool
	EnableZoom     bool
	OnCloseRequest func(*CloseRequest)
	PDF            *PDFExporter

	// termination is an optional coordinator created before the window so
	// early termination requests stay latched. when nil the window owns one.
	termination *terminationCoordinator
}

type desktopWebView struct {
	w                webview.WebView
	appID            string
	boundsTracker    nativeWindowBoundsTracker
	zoom             nativeZoomController
	savedZoom        *int
	termination      *terminationCoordinator
	closeController  *closeController
	closeInterceptor nativeCloseInterceptor
	pdf              *PDFExporter
}

func newConfiguredDesktopWebView(config webviewConfig) (*desktopWebView, error) {
	appID := strings.TrimSpace(config.AppID)
	prepareNativeWindowIdentity(appID)

	w := webview.New(config.Debug)
	if w == nil {
		return nil, errors.New("dfw: create webview")
	}
	applyNativeWindowIdentity(w.Window(), appID)

	termination := config.termination
	if termination == nil {
		termination = newTerminationCoordinator()
		termination.onEnding = config.PDF.cancelWindow
	}
	termination.bind(w.Dispatch, w.Terminate)
	config.PDF.bind(termination.enqueue, nativePDFDialogFactory(w.Window()))

	window := &desktopWebView{
		w:             w,
		appID:         appID,
		boundsTracker: newNativeWindowBoundsTracker(w.Window()),
		termination:   termination,
		pdf:           config.PDF,
	}
	if config.Title != "" {
		window.SetTitle(config.Title)
	}

	state, hasState := core.LoadWindowState(config.AppID)
	window.savedZoom = state.ZoomPercent
	if config.EnableZoom {
		window.zoom = newNativeZoom(w.Window(), core.ChooseInitialWindowZoom(state, hasState))
	}
	size := core.ChooseInitialWindowSize(config.InitialSize, state, hasState)
	if size.X > 0 && size.Y > 0 {
		window.SetSize(size)
	}
	if x, y, ok := core.ChooseInitialWindowLocation(state, hasState); ok {
		applyNativeWindowLocation(w.Window(), x, y)
	}
	if err := window.SetIcon(config.IconPNG); err != nil {
		window.Destroy()
		return nil, err
	}
	if config.OnCloseRequest != nil || config.PDF.Supported() {
		// an opted-in PDF window closes through termination so its native
		// chooser is disposed before the parent window is destroyed.
		requestClose := func() bool { termination.request(); return true }
		if config.OnCloseRequest != nil {
			window.closeController = newCloseController(config.OnCloseRequest, termination.request)
			requestClose = window.closeController.requestClose
		}
		interceptor, err := newNativeCloseInterceptor(w.Window(), requestClose)
		if err != nil {
			window.Destroy()
			return nil, err
		}
		window.closeInterceptor = interceptor
	}

	return window, nil
}

// Destroy ends the window lifecycle, detaches close interception ahead of the
// bounds and zoom controllers, and only then destroys the underlying webview,
// so native teardown never reaches application code as a close request.
func (w *desktopWebView) Destroy() {
	w.endLifecycle()
	if w.closeInterceptor != nil {
		w.closeInterceptor.Close()
		w.closeInterceptor = nil
	}
	if w.zoom != nil {
		w.zoom.Close()
		w.zoom = nil
	}
	if w.boundsTracker != nil {
		w.boundsTracker.Close()
		w.boundsTracker = nil
	}
	w.w.Destroy()
}

func (w *desktopWebView) Navigate(url string) {
	w.w.Navigate(url)
}

// Run enters the native event loop unless termination was requested before
// the window was ready, and ends the lifecycle as soon as the loop returns so
// a late close resolution or server failure cannot touch a stopping window.
func (w *desktopWebView) Run() {
	if w.termination.latched() {
		w.endLifecycle()
		return
	}
	// readiness is declared from the UI thread itself; a request that raced
	// with startup is consumed there rather than being lost.
	w.w.Dispatch(func() {
		w.termination.ready()
		w.pdf.markReady()
	})
	w.w.Run()
	w.endLifecycle()
}

func (w *desktopWebView) endLifecycle() {
	if w.closeController != nil {
		w.closeController.end()
	}
	w.termination.end()
	w.pdf.endUI()
}

func (w *desktopWebView) SaveWindowState() {
	if strings.TrimSpace(w.appID) == "" || w.boundsTracker == nil {
		return
	}

	bounds, ok := w.boundsTracker.Bounds()
	if !ok {
		return
	}

	state := core.WindowStateFromBounds(bounds)
	state.ZoomPercent = w.savedZoom
	if w.zoom != nil {
		percent := w.zoom.Percent()
		state.ZoomPercent = &percent
	}
	if _, err := core.WriteWindowState(w.appID, state); err != nil {
		dl.Errorf("dfw: write window state: %v", err)
	}
}

func (w *desktopWebView) SetIcon(iconPNG []byte) error {
	return applyWindowIcon(w.w.Window(), iconPNG)
}

func (w *desktopWebView) SetSize(size image.Point) {
	if applyNativeWindowSize(w.w.Window(), size) {
		return
	}
	w.w.SetSize(size.X, size.Y, webview.HintNone)
}

func (w *desktopWebView) SetTitle(title string) {
	w.w.SetTitle(title)
}

// Terminate requests termination from any goroutine. the request is
// marshaled to the UI thread through the termination coordinator; the pinned
// Windows backend stops its loop with PostQuitMessage, which must run there.
func (w *desktopWebView) Terminate() {
	w.termination.request()
}
