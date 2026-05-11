package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/michaelquigley/df/dl"
	"github.com/michaelquigley/dfw/examples/dfw-example-watch/cmd"
	"github.com/spf13/cobra"
)

//go:embed all:web/dist
var embeddedWeb embed.FS

func main() {
	assets, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		dl.Error(fmt.Errorf("load embedded web bundle: %w", err))
		os.Exit(1)
	}

	root := &cobra.Command{
		Use:           "dfw-example-watch",
		Short:         "Watch a directory with dfw",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		cmd.NewRunCommand(assets),
		cmd.NewDaemonCommand(assets),
		cmd.NewWindowCommand(),
	)

	if err := root.Execute(); err != nil {
		dl.Error(err)
		os.Exit(1)
	}
}
