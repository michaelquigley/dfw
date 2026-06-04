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
}

// TrayMenuItem describes a static daemon tray menu item.
type TrayMenuItem struct {
	Label    string
	Tooltip  string
	OnClick  func()
	Disabled bool
}
