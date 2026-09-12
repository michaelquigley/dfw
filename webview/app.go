package webview

import (
	"image"
	"net"
	"net/http"
)

// App describes a single-window desktop app.
type App struct {
	// AppID is a reverse-DNS identifier, for example "com.quigley.flo".
	AppID string
	// Title is the window title.
	Title string
	// InitialSize is the window size at startup.
	InitialSize image.Point
	// IconPNG is the window/app icon as PNG bytes.
	IconPNG []byte
	// EnableZoom enables remembered page zoom and standard keyboard shortcuts on Linux.
	EnableZoom bool
	// Listen returns an unstarted server and open listener for dfw to own.
	Listen func() (*http.Server, net.Listener, error)
}

// WindowApp describes a separate window process that connects to a daemon.
type WindowApp struct {
	// AppID is a reverse-DNS identifier, for example "com.quigley.flo".
	AppID string
	// Title is the window title.
	Title string
	// InitialSize is the window size at startup.
	InitialSize image.Point
	// IconPNG is the window/app icon as PNG bytes.
	IconPNG []byte
	// EnableZoom enables remembered page zoom and standard keyboard shortcuts on Linux.
	EnableZoom bool
}
