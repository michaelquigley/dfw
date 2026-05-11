package cmd

import (
	"image"
	"io/fs"

	"github.com/michaelquigley/dfw"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/server"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/watcher"
	"github.com/spf13/cobra"
)

// NewRunCommand returns the single-process dfw.Run command.
func NewRunCommand(assets fs.FS) *cobra.Command {
	var devTools bool

	run := &cobra.Command{
		Use:   "run [path]",
		Short: "Run the watcher in a single window",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := applyDevTools(devTools); err != nil {
				return err
			}

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

			return dfw.Run(dfw.App{
				AppID:       appID,
				Title:       appTitle,
				InitialSize: image.Pt(1100, 760),
				IconPNG:     icon,
				Listen:      server.Listen(assets, watch),
			})
		},
	}
	run.Flags().BoolVar(&devTools, "devtools", false, "enable webview developer tools")
	return run
}
