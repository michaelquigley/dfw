package webview

type nativeZoomController interface {
	Percent() int
	Close()
}
