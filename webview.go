package dfw

import (
	"errors"
	"image"
	"strings"

	webview "centrifuge.hectabit.org/HectaBit/webview_go"

	"github.com/michaelquigley/df/dl"
)

type webviewConfig struct {
	AppID       string
	Title       string
	InitialSize image.Point
	IconPNG     []byte
	Debug       bool
}

type desktopWebView struct {
	w             webview.WebView
	appID         string
	boundsTracker nativeWindowBoundsTracker
}

func newConfiguredWebView(config webviewConfig) (*desktopWebView, error) {
	appID := strings.TrimSpace(config.AppID)
	prepareNativeWindowIdentity(appID)

	w := webview.New(config.Debug)
	if w == nil {
		return nil, errors.New("dfw: create webview")
	}
	applyNativeWindowIdentity(w.Window(), appID)

	window := &desktopWebView{
		w:             w,
		appID:         appID,
		boundsTracker: newNativeWindowBoundsTracker(w.Window()),
	}
	if config.Title != "" {
		window.SetTitle(config.Title)
	}

	state, hasState := loadWindowState(config.AppID)
	size := chooseInitialWindowSize(config.InitialSize, state, hasState)
	if size.X > 0 && size.Y > 0 {
		window.SetSize(size)
	}
	if x, y, ok := chooseInitialWindowLocation(state, hasState); ok {
		applyNativeWindowLocation(w.Window(), x, y)
	}
	if err := window.SetIcon(config.IconPNG); err != nil {
		window.Destroy()
		return nil, err
	}

	return window, nil
}

func (w *desktopWebView) Destroy() {
	if w.boundsTracker != nil {
		w.boundsTracker.Close()
		w.boundsTracker = nil
	}
	w.w.Destroy()
}

func (w *desktopWebView) Navigate(url string) {
	w.w.Navigate(url)
}

func (w *desktopWebView) Run() {
	w.w.Run()
}

func (w *desktopWebView) SaveWindowState() {
	if strings.TrimSpace(w.appID) == "" || w.boundsTracker == nil {
		return
	}

	bounds, ok := w.boundsTracker.Bounds()
	if !ok {
		return
	}

	if _, err := writeWindowState(w.appID, windowStateFromBounds(bounds)); err != nil {
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

func (w *desktopWebView) Terminate() {
	w.w.Terminate()
}
