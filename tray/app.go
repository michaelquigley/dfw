package tray

import (
	"net"
	"net/http"
)

// DaemonApp describes a tray-resident daemon app.
type DaemonApp struct {
	// AppID is a reverse-DNS identifier, for example "com.quigley.flo".
	AppID string
	// Title is displayed as the tray tooltip.
	Title string
	// IconPNG is used for the tray icon.
	IconPNG []byte
	// Listen returns an unstarted server and open listener for dfw to own.
	Listen func() (*http.Server, net.Listener, error)
	// SpawnWindow is called when the "Open Window" tray item is clicked.
	SpawnWindow func(daemonAddr string) error
	// TrayItems are inserted between "Open Window" and "Quit".
	TrayItems []TrayMenuItem
	// OnQuit is called when "Quit" is chosen. returning true quits as an
	// absent OnQuit does; returning false keeps the daemon running, leaving the
	// application to confirm or to shut down gracefully and end the daemon
	// with Control.Stop. it runs on the tray's own goroutine and should return
	// promptly; "Quit" chosen again while it runs is ignored. a nil OnQuit
	// preserves the unconditional Quit.
	OnQuit func() bool
	// Control optionally lets the application change the tray icon, tooltip,
	// and TrayItems while the daemon runs, and end the daemon itself. see
	// NewControl.
	Control *Control
}

// TrayMenuItem describes a daemon tray menu item as it starts. use
// Control.Item with the item's index in TrayItems to change it later.
type TrayMenuItem struct {
	Label    string
	Tooltip  string
	OnClick  func()
	Disabled bool
}
