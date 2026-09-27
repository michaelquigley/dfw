package webview

import (
	"os"
	"strings"
	"sync"
)

const (
	jscSignalEnv        = "JSC_SIGNAL_FOR_GC"
	disableJSCSignalEnv = "DFW_DISABLE_JSC_SIGNAL_SETUP"
)

// jscSignalSetup is process-wide: JavaScriptCore fixes its signal at initial
// initialization, not at each window's creation or destruction.
type jscSignalSetup struct {
	once   sync.Once
	signal int
}

func (s *jscSignalSetup) prepare(lookup func(string) (string, bool), configure func() int) int {
	s.once.Do(func() {
		// even an empty explicit JSC setting belongs to the embedding product.
		if _, set := lookup(jscSignalEnv); set {
			return
		}
		if value, set := lookup(disableJSCSignalEnv); set {
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "", "0", "false", "no", "off":
			default:
				return
			}
		}
		s.signal = configure()
	})
	return s.signal
}

var processJSCSignal jscSignalSetup

func prepareNativeJavaScriptSignals() {
	processJSCSignal.prepare(os.LookupEnv, configureNativeJSCSignal)
}
