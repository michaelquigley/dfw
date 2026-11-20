// dfw-example-macos is the manual fixture for a tray daemon and its window on
// macOS, run from an application bundle the way a product ships one: the
// daemon lives in the menu bar (LSUIElement), and "Open Window" starts the
// window process from the same binary. its page holds what a window has to
// get right there: the keyboard (focus, Edit menu keys), links that open a
// new window, and the file chooser. what the page sees, it reports to the
// daemon's log. the manual sequence in README.md is the proof. it builds and
// runs elsewhere too, but checks nothing there that other fixtures do not.
package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/tray"
	"github.com/michaelquigley/dfw/webview"
)

const (
	appID             = "com.quigley.dfw.example.macos"
	appTitle          = "dfw Example macOS"
	readHeaderTimeout = 5 * time.Second
)

//go:embed page.html
var page []byte

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "window" {
		err = webview.Window(webview.WindowApp{
			AppID:       appID,
			Title:       appTitle,
			InitialSize: image.Pt(900, 720),
		})
	} else {
		err = daemon()
	}
	if err != nil {
		dl.Error(err)
		os.Exit(1)
	}
}

func daemon() error {
	icon, err := iconPNG()
	if err != nil {
		return err
	}
	spawn := tray.SpawnSelf("window")
	return tray.Daemon(tray.DaemonApp{
		AppID:       appID,
		Title:       appTitle,
		IconPNG:     icon,
		Listen:      listen,
		SpawnWindow: spawn,
		// opening the bundle again, from Finder or open, opens a window as
		// Open Window does.
		OnReopen: func(daemonAddr string) {
			if err := spawn(daemonAddr); err != nil {
				dl.Errorf("opening a window on reopen: %v", err)
			}
		},
	})
}

func listen() (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	dl.Infof("listening on 'http://%s'", listener.Addr())

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("/api/report", handleReport)
	return &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderTimeout}, listener, nil
}

// handleReport logs one line the page saw: a key with a modifier, a copy or
// a paste, a link click, a file chooser's result.
func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dl.Infof("page: '%s'", r.URL.Query().Get("m"))
	w.WriteHeader(http.StatusNoContent)
}

// iconPNG draws a 32x32 icon: a dark rounded field with a teal square, which
// reads at the menu bar's size in light and dark appearance.
func iconPNG() ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, image.Rect(2, 2, 30, 30), &image.Uniform{C: color.NRGBA{R: 22, G: 28, B: 36, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(10, 10, 22, 22), &image.Uniform{C: color.NRGBA{R: 74, G: 199, B: 185, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
