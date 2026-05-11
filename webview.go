package dfw

import (
	"errors"
	"image"

	webview "centrifuge.hectabit.org/HectaBit/webview_go"
)

type webviewConfig struct {
	Title       string
	InitialSize image.Point
	IconPNG     []byte
	Debug       bool
}

type desktopWebView struct {
	w webview.WebView
}

func newConfiguredWebView(config webviewConfig) (*desktopWebView, error) {
	w := webview.New(config.Debug)
	if w == nil {
		return nil, errors.New("dfw: create webview")
	}

	window := &desktopWebView{w: w}
	if config.Title != "" {
		window.SetTitle(config.Title)
	}
	if config.InitialSize.X > 0 && config.InitialSize.Y > 0 {
		window.SetSize(config.InitialSize)
	}
	if err := window.SetIcon(config.IconPNG); err != nil {
		window.Destroy()
		return nil, err
	}

	return window, nil
}

func (w *desktopWebView) Destroy() {
	w.w.Destroy()
}

func (w *desktopWebView) Navigate(url string) {
	w.w.Navigate(url)
}

func (w *desktopWebView) Run() {
	w.w.Run()
}

func (w *desktopWebView) SetIcon(iconPNG []byte) error {
	return applyWindowIcon(w.w.Window(), iconPNG)
}

func (w *desktopWebView) SetSize(size image.Point) {
	w.w.SetSize(size.X, size.Y, webview.HintNone)
}

func (w *desktopWebView) SetTitle(title string) {
	w.w.SetTitle(title)
}

func (w *desktopWebView) Terminate() {
	w.w.Terminate()
}
