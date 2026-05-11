package dfw

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonRuntimeRoundTrip(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	expectedPath := filepath.Join(configDir, "com.example.app", "runtime", "daemon.json")

	path, err := writeDaemonRuntime("com.example.app", daemonRuntime{
		PID:     12345,
		Address: "127.0.0.1:53291",
	})
	require.NoError(t, err)
	assert.Equal(t, expectedPath, path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	raw := map[string]any{}
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, map[string]any{
		"pid":     float64(12345),
		"address": "127.0.0.1:53291",
	}, raw)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	runtime, err := readDaemonRuntime("com.example.app")
	require.NoError(t, err)
	assert.Equal(t, daemonRuntime{
		PID:     12345,
		Address: "127.0.0.1:53291",
	}, runtime)
}

func TestResolveDaemonAddrEnvOverridesRuntimeFile(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)
	t.Setenv(daemonAddrEnv, "127.0.0.1:11111")

	_, err := writeDaemonRuntime("com.example.app", daemonRuntime{
		PID:     12345,
		Address: "127.0.0.1:22222",
	})
	require.NoError(t, err)

	addr, err := resolveDaemonAddr("com.example.app")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:11111", addr)
}

func TestResolveDaemonAddrMissing(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)
	t.Setenv(daemonAddrEnv, "")

	addr, err := resolveDaemonAddr("com.example.app")
	require.Error(t, err)
	assert.Empty(t, addr)
	assert.ErrorIs(t, err, errDaemonAddressMissing)
}

func TestDaemonRuntimePathShape(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	tests := []struct {
		name  string
		appID string
	}{
		{name: "reverse dns", appID: "com.quigley.flo"},
		{name: "nested vendor", appID: "io.example.product"},
		{name: "single segment", appID: "local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := daemonRuntimePath(tt.appID)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(configDir, tt.appID, "runtime", "daemon.json"), path)
		})
	}
}

func TestDaemonRuntimePathRequiresAppID(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	path, err := daemonRuntimePath(" ")
	require.Error(t, err)
	assert.Empty(t, path)
	assert.ErrorIs(t, err, errEmptyAppID)
}

func TestRemoveDaemonRuntimeIgnoresMissingFile(t *testing.T) {
	configDir := t.TempDir()
	withUserConfigDir(t, configDir)

	require.NoError(t, removeDaemonRuntime("com.example.app"))
}

func TestUserConfigDirError(t *testing.T) {
	expected := errors.New("boom")
	oldUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) {
		return "", expected
	}
	t.Cleanup(func() {
		userConfigDir = oldUserConfigDir
	})

	path, err := daemonRuntimePath("com.example.app")
	require.Error(t, err)
	assert.Empty(t, path)
	assert.ErrorIs(t, err, expected)
}

func withUserConfigDir(t *testing.T, dir string) {
	t.Helper()

	oldUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) {
		return dir, nil
	}
	t.Cleanup(func() {
		userConfigDir = oldUserConfigDir
	})

	t.Setenv(daemonAddrEnv, "")
}
