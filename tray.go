package dfw

import (
	"fmt"

	"fyne.io/systray"
	"github.com/michaelquigley/df/dl"
)

type trayConfig struct {
	Title       string
	IconPNG     []byte
	DaemonAddr  string
	SpawnWindow func(daemonAddr string) error
	TrayItems   []TrayMenuItem
	Stop        <-chan struct{}
}

func runTray(config trayConfig) error {
	ready := make(chan error, 1)
	exited := make(chan struct{})

	systray.Run(func() {
		if err := configureTray(config); err != nil {
			ready <- err
			systray.Quit()
			return
		}
		ready <- nil
		if config.Stop != nil {
			go func() {
				select {
				case <-config.Stop:
					systray.Quit()
				case <-exited:
				}
			}()
		}
	}, func() {
		close(exited)
	})

	select {
	case err := <-ready:
		return err
	default:
		return nil
	}
}

func configureTray(config trayConfig) error {
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

	for _, item := range config.TrayItems {
		menuItem := systray.AddMenuItem(item.Label, item.Tooltip)
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
	quit := systray.AddMenuItem("Quit", "Quit")
	go func() {
		for range quit.ClickedCh {
			systray.Quit()
			return
		}
	}()

	return nil
}

func validateTrayIconPNG(iconPNG []byte) error {
	if len(iconPNG) == 0 {
		return nil
	}
	if _, err := decodeIconPNG(iconPNG); err != nil {
		return fmt.Errorf("dfw: tray icon: %w", err)
	}
	return nil
}
