package core

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaelquigley/df/dd"
)

// DaemonAddrEnv is the environment variable a window process reads to discover
// its daemon's HTTP address.
const (
	DaemonAddrEnv = "DFW_DAEMON_ADDR"
	runtimeDir    = "runtime"
	daemonJSON    = "daemon.json"
)

var (
	errEmptyAppID           = errors.New("dfw: app id is required")
	errDaemonAddressMissing = errors.New("dfw: daemon address not found")

	userConfigDir = os.UserConfigDir
)

// DaemonRuntime is the metadata a daemon writes for window discovery.
type DaemonRuntime struct {
	PID     int
	Address string
}

// ResolveDaemonAddr resolves a daemon's address from DFW_DAEMON_ADDR or the
// AppID-derived runtime file.
func ResolveDaemonAddr(appID string) (string, error) {
	if addr, ok := os.LookupEnv(DaemonAddrEnv); ok && addr != "" {
		if err := validateDaemonAddr(addr); err != nil {
			return "", fmt.Errorf("%w: %s: %v", errDaemonAddressMissing, DaemonAddrEnv, err)
		}
		return addr, nil
	}

	runtime, err := readDaemonRuntime(appID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errDaemonAddressMissing, err)
	}
	if runtime.Address == "" {
		return "", fmt.Errorf("%w: runtime file has empty address", errDaemonAddressMissing)
	}
	if err := validateDaemonAddr(runtime.Address); err != nil {
		return "", fmt.Errorf("%w: runtime file: %v", errDaemonAddressMissing, err)
	}
	return runtime.Address, nil
}

func validateDaemonAddr(addr string) error {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return err
	}
	return nil
}

// WriteDaemonRuntime writes the daemon runtime file for appID and returns its
// path.
func WriteDaemonRuntime(appID string, runtime DaemonRuntime) (string, error) {
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

func readDaemonRuntime(appID string) (DaemonRuntime, error) {
	path, err := daemonRuntimePath(appID)
	if err != nil {
		return DaemonRuntime{}, err
	}

	runtime := DaemonRuntime{}
	if err := dd.BindJSONFile(&runtime, path); err != nil {
		return DaemonRuntime{}, fmt.Errorf("dfw: bind daemon runtime: %w", err)
	}

	return runtime, nil
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
