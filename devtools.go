package dfw

import (
	"os"
	"strings"
)

const devToolsEnv = "DFW_DEVTOOLS"

// DevToolsEnabled reports whether the webview developer tools should be
// enabled from the DFW_DEVTOOLS environment variable.
func DevToolsEnabled() bool {
	value, ok := os.LookupEnv(devToolsEnv)
	if !ok {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
