package cmd

import (
	"github.com/michaelquigley/dfw/webview"
	"github.com/spf13/cobra"
)

// NewWindowCommand returns the daemon-connected webview.Window command.
func NewWindowCommand() *cobra.Command {
	var devTools bool

	window := &cobra.Command{
		Use:   "window",
		Short: "Open a window connected to a running daemon",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := applyDevTools(devTools); err != nil {
				return err
			}

			app, err := windowApp()
			if err != nil {
				return err
			}
			return webview.Window(app)
		},
	}
	window.Flags().BoolVar(&devTools, "devtools", false, "enable webview developer tools")
	return window
}
