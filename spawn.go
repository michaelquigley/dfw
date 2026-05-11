package dfw

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SpawnSelf returns a SpawnWindow function that launches the current binary
// with args and DFW_DAEMON_ADDR set in the child environment.
func SpawnSelf(args ...string) func(daemonAddr string) error {
	spawnArgs := cloneArgs(args)
	return func(daemonAddr string) error {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		return startSpawn(binary, spawnArgs, daemonAddr)
	}
}

// Spawn returns a SpawnWindow function that launches binary with args and
// DFW_DAEMON_ADDR set in the child environment.
func Spawn(binary string, args ...string) func(daemonAddr string) error {
	spawnArgs := cloneArgs(args)
	return func(daemonAddr string) error {
		return startSpawn(binary, spawnArgs, daemonAddr)
	}
}

func startSpawn(binary string, args []string, daemonAddr string) error {
	cmd := buildCmd(binary, args, daemonAddr)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func buildCmd(binary string, args []string, daemonAddr string) *exec.Cmd {
	return buildCmdWithEnv(os.Environ(), binary, args, daemonAddr)
}

func buildCmdWithEnv(env []string, binary string, args []string, daemonAddr string) *exec.Cmd {
	cmd := exec.Command(binary, cloneArgs(args)...)
	cmd.Env = withDaemonAddrEnv(env, daemonAddr)
	return cmd
}

func withDaemonAddrEnv(env []string, daemonAddr string) []string {
	next := make([]string, 0, len(env)+1)
	for _, value := range env {
		if envKey(value) == daemonAddrEnv {
			continue
		}
		next = append(next, value)
	}
	return append(next, daemonAddrEnv+"="+daemonAddr)
}

func envKey(value string) string {
	key, _, ok := strings.Cut(value, "=")
	if !ok {
		return value
	}
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func cloneArgs(args []string) []string {
	return append([]string(nil), args...)
}
