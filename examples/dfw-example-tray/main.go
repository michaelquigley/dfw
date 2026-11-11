// dfw-example-tray is a deliberately small fixture for a tray daemon that
// takes part in its own lifetime. it imports only dfw/tray, so it builds
// without the webview's native dependency. a "busy" state, toggled from the
// menu, changes the icon, the tooltip, and a status item through tray.Control;
// Quit is declined through DaemonApp.OnQuit while busy; and the daemon can be
// ended by a countdown item or by POST /api/stop, neither of which starts at
// Quit. the manual sequence in README.md is the proof.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/michaelquigley/df/dd"
	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/tray"
)

const (
	appID             = "com.quigley.dfw.example.tray"
	appTitle          = "dfw Example Tray"
	readHeaderTimeout = 5 * time.Second
	shutdownSeconds   = 3
)

// indexes into TrayItems, for tray.Control.Item.
const (
	statusItem = iota
	busyItem
	shutdownItem
)

type daemonState struct {
	control  *tray.Control
	idleIcon []byte
	busyIcon []byte

	mu       sync.Mutex
	busy     bool
	declined int
}

type statusResponse struct {
	Busy     bool
	Declined int
}

func main() {
	if err := run(); err != nil {
		dl.Error(err)
		os.Exit(1)
	}
}

func run() error {
	idleIcon, err := iconPNG(color.NRGBA{R: 22, G: 28, B: 36, A: 255}, color.NRGBA{R: 74, G: 199, B: 185, A: 255})
	if err != nil {
		return err
	}
	busyIcon, err := iconPNG(color.NRGBA{R: 235, G: 94, B: 94, A: 255}, color.NRGBA{R: 22, G: 28, B: 36, A: 255})
	if err != nil {
		return err
	}

	state := &daemonState{control: tray.NewControl(), idleIcon: idleIcon, busyIcon: busyIcon}
	return tray.Daemon(tray.DaemonApp{
		AppID:   appID,
		Title:   appTitle,
		IconPNG: idleIcon,
		Listen:  state.listen,
		TrayItems: []tray.TrayMenuItem{
			statusItem: {Label: "Idle", Disabled: true},
			busyItem:   {Label: "Start work", OnClick: state.toggleBusy},
			shutdownItem: {
				Label:   fmt.Sprintf("Shut down in %d s", shutdownSeconds),
				OnClick: func() { go state.shutDownLater() },
			},
		},
		OnQuit:  state.onQuit,
		Control: state.control,
	})
}

// toggleBusy flips the busy state and shows it in the icon, the tooltip, and
// the status and toggle items.
func (s *daemonState) toggleBusy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = !s.busy

	icon, tooltip, status, toggle := s.idleIcon, appTitle, "Idle", "Start work"
	if s.busy {
		icon, tooltip, status, toggle = s.busyIcon, appTitle+": busy", "Busy", "Finish work"
	}
	if err := s.control.SetIcon(icon); err != nil {
		dl.Errorf("tray icon: %v", err)
	}
	s.control.SetTooltip(tooltip)
	s.control.Item(statusItem).SetLabel(status)
	s.control.Item(busyItem).SetLabel(toggle)
}

// onQuit declines Quit while busy, the way a product would route it through a
// "shut down when idle" gate, and allows it otherwise.
func (s *daemonState) onQuit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.busy {
		return true
	}
	s.declined++
	s.control.Item(statusItem).SetLabel(fmt.Sprintf("Busy: Quit declined (%d)", s.declined))
	return false
}

// shutDownLater ends the daemon without going through Quit: the item disables
// itself, counts down in its own label, then calls Stop.
func (s *daemonState) shutDownLater() {
	item := s.control.Item(shutdownItem)
	item.Disable()
	for remaining := shutdownSeconds; remaining > 0; remaining-- {
		item.SetLabel(fmt.Sprintf("Shutting down in %d s", remaining))
		time.Sleep(time.Second)
	}
	s.control.Stop()
}

func (s *daemonState) status() statusResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return statusResponse{Busy: s.busy, Declined: s.declined}
}

func (s *daemonState) listen() (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	dl.Infof("listening on 'http://%s'", listener.Addr())

	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/stop", s.handleStop)
	return &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderTimeout}, listener, nil
}

func (s *daemonState) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := dd.UnbindJSON(s.status())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// handleStop is a shutdown accepted from the product's own API: it ends the
// daemon through tray.Control.Stop, which does not consult OnQuit.
func (s *daemonState) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.control.Stop()
	w.WriteHeader(http.StatusNoContent)
}

// iconPNG draws a 32x32 icon: a background with a centered square.
func iconPNG(background, square color.NRGBA) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(9, 9, 23, 23), &image.Uniform{C: square}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
