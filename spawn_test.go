package dfw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCmd(t *testing.T) {
	cmd := buildCmdWithEnv(
		[]string{"EXISTING=1", "DFW_DAEMON_ADDR=old"},
		"dfw-example-watch",
		[]string{"window", "--devtools"},
		"127.0.0.1:53291",
	)

	assert.Equal(t, "dfw-example-watch", cmd.Path)
	assert.Equal(t, []string{"dfw-example-watch", "window", "--devtools"}, cmd.Args)
	assert.Equal(t, []string{"EXISTING=1", "DFW_DAEMON_ADDR=127.0.0.1:53291"}, cmd.Env)
}

func TestBuildCmdCopiesArgs(t *testing.T) {
	args := []string{"window"}
	cmd := buildCmdWithEnv(nil, "dfw-example-watch", args, "127.0.0.1:53291")

	args[0] = "daemon"

	assert.Equal(t, []string{"dfw-example-watch", "window"}, cmd.Args)
}
