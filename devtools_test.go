package dfw

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevToolsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "one", value: "1", want: true},
		{name: "true", value: "true", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "zero", value: "0", want: false},
		{name: "false", value: "false", want: false},
		{name: "empty", value: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(devToolsEnv, tt.value)
			assert.Equal(t, tt.want, DevToolsEnabled())
		})
	}
}

func TestDevToolsEnabledUnset(t *testing.T) {
	oldValue, hadValue := os.LookupEnv(devToolsEnv)
	require.NoError(t, os.Unsetenv(devToolsEnv))
	t.Cleanup(func() {
		if hadValue {
			require.NoError(t, os.Setenv(devToolsEnv, oldValue))
			return
		}
		require.NoError(t, os.Unsetenv(devToolsEnv))
	})

	assert.False(t, DevToolsEnabled())
}
