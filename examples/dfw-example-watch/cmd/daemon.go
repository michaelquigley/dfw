package cmd

import (
	"io/fs"

	"github.com/michaelquigley/dfw/examples/dfw-example-watch/server"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/watcher"
	"github.com/michaelquigley/dfw/tray"
	"github.com/spf13/cobra"
)

// NewDaemonCommand returns the tray-resident tray.Daemon command.
func NewDaemonCommand(assets fs.FS) *cobra.Command {
	daemon := &cobra.Command{
		Use:   "daemon [path]",
		Short: "Run the watcher as a tray daemon",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := watchPathArg(args)
			if err != nil {
				return err
			}

			watch, err := watcher.New(root)
			if err != nil {
				return err
			}
			defer watch.Close()

			icon, err := appIconPNG()
			if err != nil {
				return err
			}

			return tray.Daemon(tray.DaemonApp{
				AppID:       appID,
				Title:       appTitle,
				IconPNG:     icon,
				Listen:      server.Listen(assets, watch),
				SpawnWindow: tray.SpawnSelf("window"),
				TrayItems: []tray.TrayMenuItem{
					{
						Label:    "Watching " + watch.DisplayRoot(),
						Tooltip:  watch.Root(),
						Disabled: true,
					},
				},
			})
		},
	}
	return daemon
}
