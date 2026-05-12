package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type desktopInstallPaths struct {
	desktopFile string
	icon32      string
	icon128     string
}

// NewInstallDesktopCommand returns the Linux desktop metadata installer.
func NewInstallDesktopCommand() *cobra.Command {
	installDesktop := &cobra.Command{
		Use:   "install-desktop",
		Short: "Install Linux desktop entry and icons",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve executable: %w", err)
			}
			executable, err = filepath.Abs(executable)
			if err != nil {
				return fmt.Errorf("resolve executable path: %w", err)
			}

			dataHome, err := desktopDataHome()
			if err != nil {
				return err
			}
			paths := desktopInstallPathsFor(dataHome)
			if err := installDesktopFiles(paths, executable); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(command.OutOrStdout(), "Installed desktop entry: %s\n", paths.desktopFile)
			_, _ = fmt.Fprintf(command.OutOrStdout(), "Installed icons: %s, %s\n", paths.icon32, paths.icon128)
			return nil
		},
	}
	return installDesktop
}

func desktopDataHome() (string, error) {
	return desktopDataHomeFromEnv(os.Getenv("XDG_DATA_HOME"), os.Getenv("HOME"))
}

func desktopDataHomeFromEnv(xdgDataHome, home string) (string, error) {
	if filepath.IsAbs(xdgDataHome) {
		return xdgDataHome, nil
	}
	if home == "" {
		return "", errors.New("HOME is required when XDG_DATA_HOME is unset or relative")
	}
	return filepath.Join(home, ".local", "share"), nil
}

func desktopInstallPathsFor(dataHome string) desktopInstallPaths {
	return desktopInstallPaths{
		desktopFile: filepath.Join(dataHome, "applications", appID+".desktop"),
		icon32:      filepath.Join(dataHome, "icons", "hicolor", "32x32", "apps", appID+".png"),
		icon128:     filepath.Join(dataHome, "icons", "hicolor", "128x128", "apps", appID+".png"),
	}
}

func installDesktopFiles(paths desktopInstallPaths, executable string) error {
	for _, dir := range []string{
		filepath.Dir(paths.desktopFile),
		filepath.Dir(paths.icon32),
		filepath.Dir(paths.icon128),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create desktop install directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(paths.desktopFile, []byte(desktopEntry(executable)), 0o644); err != nil {
		return fmt.Errorf("write desktop entry: %w", err)
	}
	if err := writeIcon(paths.icon32, 32); err != nil {
		return err
	}
	if err := writeIcon(paths.icon128, 128); err != nil {
		return err
	}
	return nil
}

func writeIcon(path string, size int) error {
	icon, err := generateAppIconPNG(size)
	if err != nil {
		return fmt.Errorf("generate %dpx desktop icon: %w", size, err)
	}
	if err := os.WriteFile(path, icon, 0o644); err != nil {
		return fmt.Errorf("write %dpx desktop icon: %w", size, err)
	}
	return nil
}

func desktopEntry(executable string) string {
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + appTitle,
		"Exec=" + desktopExecPath(executable) + " run",
		"Icon=" + appID,
		"StartupWMClass=" + appID,
		"StartupNotify=true",
		"Terminal=false",
		"Categories=Development;Utility;",
		"",
	}, "\n")
}

func desktopExecPath(path string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"`", "\\`",
		"$", "\\$",
		"%", "%%",
	)
	return `"` + replacer.Replace(path) + `"`
}
