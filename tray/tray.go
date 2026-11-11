package tray

import (
	"fmt"

	"fyne.io/systray"
	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/internal/core"
)

type trayConfig struct {
	Title       string
	IconPNG     []byte
	DaemonAddr  string
	SpawnWindow func(daemonAddr string) error
	TrayItems   []TrayMenuItem
	OnQuit      func() bool
	Control     *Control
	Stop        <-chan struct{}
}

func runTray(config trayConfig) error {
	ready := make(chan error, 1)
	exited := make(chan struct{})

	// every path that quits the tray goes through quit, which ends the
	// control first so no change reaches systray after it starts shutting
	// down.
	quit := func() {
		config.Control.end()
		systray.Quit()
	}

	systray.Run(func() {
		if err := configureTray(config, quit); err != nil {
			ready <- err
			quit()
			return
		}
		ready <- nil
		if config.Stop != nil {
			go func() {
				select {
				case <-config.Stop:
					quit()
				case <-exited:
				}
			}()
		}
	}, func() {
		config.Control.end()
		close(exited)
	})

	select {
	case err := <-ready:
		return err
	default:
		return nil
	}
}

func configureTray(config trayConfig, quit func()) error {
	if config.Title != "" {
		systray.SetTitle(config.Title)
		systray.SetTooltip(config.Title)
	}
	if len(config.IconPNG) > 0 {
		icon, err := trayIconBytes(config.IconPNG)
		if err != nil {
			return err
		}
		systray.SetIcon(icon)
	}

	if config.SpawnWindow != nil {
		openWindow := systray.AddMenuItem("Open Window", "Open Window")
		go func() {
			for range openWindow.ClickedCh {
				if err := config.SpawnWindow(config.DaemonAddr); err != nil {
					dl.Errorf("dfw: SpawnWindow: %v", err)
				}
			}
		}()
	}

	menuItems := make([]*systray.MenuItem, 0, len(config.TrayItems))
	for _, item := range config.TrayItems {
		menuItem := systray.AddMenuItem(item.Label, item.Tooltip)
		menuItems = append(menuItems, menuItem)
		if item.Disabled {
			menuItem.Disable()
		}
		if item.OnClick != nil {
			onClick := item.OnClick
			go func() {
				for range menuItem.ClickedCh {
					onClick()
				}
			}()
		}
	}

	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit")
	go runQuitItem(quitItem.ClickedCh, config.OnQuit, quit)

	config.Control.bind(systrayBackend{items: menuItems}, len(menuItems))
	return nil
}

// runQuitItem serves the Quit item. a click arriving while onQuit runs finds
// no receiver and is dropped by systray, so a slow onQuit is never re-entered.
func runQuitItem(clicked <-chan struct{}, onQuit func() bool, quit func()) {
	for range clicked {
		if onQuit == nil || onQuit() {
			quit()
			return
		}
	}
}

// systrayBackend applies Control changes to the live systray. Control
// serializes the calls; systray's MenuItem setters are not safe concurrently.
type systrayBackend struct {
	items []*systray.MenuItem
}

func (b systrayBackend) SetIcon(icon []byte) {
	systray.SetIcon(icon)
}

func (b systrayBackend) SetTooltip(tooltip string) {
	systray.SetTooltip(tooltip)
}

func (b systrayBackend) SetItemLabel(index int, label string) {
	b.items[index].SetTitle(label)
}

func (b systrayBackend) SetItemEnabled(index int, enabled bool) {
	if enabled {
		b.items[index].Enable()
	} else {
		b.items[index].Disable()
	}
}

func validateTrayIconPNG(iconPNG []byte) error {
	if len(iconPNG) == 0 {
		return nil
	}
	if _, err := core.DecodeIconPNG(iconPNG); err != nil {
		return fmt.Errorf("dfw: tray icon: %w", err)
	}
	return nil
}
