package dfw

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaelquigley/df/dd"
)

const (
	daemonAddrEnv = "DFW_DAEMON_ADDR"
	runtimeDir    = "runtime"
	daemonJSON    = "daemon.json"
)

var (
	errEmptyAppID           = errors.New("dfw: app id is required")
	errDaemonAddressMissing = errors.New("dfw: daemon address not found")

	userConfigDir = os.UserConfigDir
)

type daemonRuntime struct {
	PID     int
	Address string
}

func resolveDaemonAddr(appID string) (string, error) {
	if addr, ok := os.LookupEnv(daemonAddrEnv); ok && addr != "" {
		return addr, nil
	}

	runtime, err := readDaemonRuntime(appID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errDaemonAddressMissing, err)
	}
	if runtime.Address == "" {
		return "", fmt.Errorf("%w: runtime file has empty address", errDaemonAddressMissing)
	}
	return runtime.Address, nil
}

func writeDaemonRuntime(appID string, runtime daemonRuntime) (string, error) {
	path, err := daemonRuntimePath(appID)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("dfw: create runtime directory: %w", err)
	}

	if err := dd.UnbindJSONFile(runtime, path); err != nil {
		return "", fmt.Errorf("dfw: unbind daemon runtime: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("dfw: set daemon runtime permissions: %w", err)
	}

	return path, nil
}

func readDaemonRuntime(appID string) (daemonRuntime, error) {
	path, err := daemonRuntimePath(appID)
	if err != nil {
		return daemonRuntime{}, err
	}

	runtime := daemonRuntime{}
	if err := dd.BindJSONFile(&runtime, path); err != nil {
		return daemonRuntime{}, fmt.Errorf("dfw: bind daemon runtime: %w", err)
	}

	return runtime, nil
}

func removeDaemonRuntime(appID string) error {
	path, err := daemonRuntimePath(appID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("dfw: remove daemon runtime: %w", err)
	}
	return nil
}

func daemonRuntimePath(appID string) (string, error) {
	base, err := userConfigPath(appID)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, runtimeDir, daemonJSON), nil
}

func userConfigPath(appID string) (string, error) {
	if strings.TrimSpace(appID) == "" {
		return "", errEmptyAppID
	}

	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("dfw: resolve user config directory: %w", err)
	}

	return filepath.Join(dir, appID), nil
}
