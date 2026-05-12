package cmd

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopDataHomeFromEnv(t *testing.T) {
	t.Run("uses absolute XDG_DATA_HOME", func(t *testing.T) {
		got, err := desktopDataHomeFromEnv("/tmp/dfw-data", "/home/example")
		require.NoError(t, err)
		require.Equal(t, "/tmp/dfw-data", got)
	})

	t.Run("falls back to home for empty XDG_DATA_HOME", func(t *testing.T) {
		got, err := desktopDataHomeFromEnv("", "/home/example")
		require.NoError(t, err)
		require.Equal(t, filepath.Join("/home/example", ".local", "share"), got)
	})

	t.Run("falls back to home for relative XDG_DATA_HOME", func(t *testing.T) {
		got, err := desktopDataHomeFromEnv("relative-data", "/home/example")
		require.NoError(t, err)
		require.Equal(t, filepath.Join("/home/example", ".local", "share"), got)
	})

	t.Run("requires home for fallback", func(t *testing.T) {
		_, err := desktopDataHomeFromEnv("", "")
		require.Error(t, err)
	})
}

func TestDesktopInstallPathsFor(t *testing.T) {
	paths := desktopInstallPathsFor("/tmp/dfw-data")

	require.Equal(t, filepath.Join("/tmp/dfw-data", "applications", appID+".desktop"), paths.desktopFile)
	require.Equal(t, filepath.Join("/tmp/dfw-data", "icons", "hicolor", "32x32", "apps", appID+".png"), paths.icon32)
	require.Equal(t, filepath.Join("/tmp/dfw-data", "icons", "hicolor", "128x128", "apps", appID+".png"), paths.icon128)
}

func TestDesktopEntry(t *testing.T) {
	entry := desktopEntry(`/opt/dfw example/dfw-example-watch`)

	require.Contains(t, entry, "[Desktop Entry]\n")
	require.Contains(t, entry, "Type=Application\n")
	require.Contains(t, entry, "Name=dfw Example Watch\n")
	require.Contains(t, entry, `Exec="/opt/dfw example/dfw-example-watch" run`+"\n")
	require.Contains(t, entry, "Icon="+appID+"\n")
	require.Contains(t, entry, "StartupWMClass="+appID+"\n")
	require.Contains(t, entry, "StartupNotify=true\n")
	require.Contains(t, entry, "Terminal=false\n")
	require.Contains(t, entry, "Categories=Development;Utility;\n")
}

func TestDesktopExecPathEscapesDesktopEntrySpecials(t *testing.T) {
	require.Equal(t, `"/opt/dfw path/dfw-example-watch"`, desktopExecPath(`/opt/dfw path/dfw-example-watch`))
	require.Equal(t, `"C:\\Program Files\\dfw \"watch\"\\app%%"`, desktopExecPath(`C:\Program Files\dfw "watch"\app%`))
	require.Equal(t, "\"/tmp/\\$HOME/\\`cmd\\`\"", desktopExecPath("/tmp/$HOME/`cmd`"))
}

func TestGenerateAppIconPNGSizes(t *testing.T) {
	for _, size := range []int{32, 128} {
		icon, err := generateAppIconPNG(size)
		require.NoError(t, err)

		config, err := png.DecodeConfig(bytes.NewReader(icon))
		require.NoError(t, err)
		require.Equal(t, size, config.Width)
		require.Equal(t, size, config.Height)
	}
}

func TestInstallDesktopFiles(t *testing.T) {
	dataHome := t.TempDir()
	paths := desktopInstallPathsFor(dataHome)

	err := installDesktopFiles(paths, "/opt/dfw-example-watch")
	require.NoError(t, err)

	entry, err := os.ReadFile(paths.desktopFile)
	require.NoError(t, err)
	require.Contains(t, string(entry), `Exec="/opt/dfw-example-watch" run`)

	assertPNGSize(t, paths.icon32, 32)
	assertPNGSize(t, paths.icon128, 128)
}

func assertPNGSize(t *testing.T, path string, size int) {
	t.Helper()

	icon, err := os.ReadFile(path)
	require.NoError(t, err)

	config, err := png.DecodeConfig(bytes.NewReader(icon))
	require.NoError(t, err)
	require.Equal(t, size, config.Width)
	require.Equal(t, size, config.Height)
}
