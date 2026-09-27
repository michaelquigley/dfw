//go:build linux && dfw_jsc_probe

package webview

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// each scenario needs a fresh process: JSC initialization and signal ownership
// cannot safely be reset between tests. no GTK window or desktop input is used.
func TestNativeJavaScriptSignals(t *testing.T) {
	if mode := os.Getenv("DFW_JSC_PROBE_CHILD"); mode != "" {
		runtime.LockOSThread()
		runJSCSignalProbe(t, mode)
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"default", "automatic", "explicit", "explicit-empty", "disabled", "occupied", "blocked", "initialized"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, self, "-test.run=^TestNativeJavaScriptSignals$", "-test.v")
			for _, entry := range os.Environ() {
				if strings.HasPrefix(entry, jscSignalEnv+"=") || strings.HasPrefix(entry, disableJSCSignalEnv+"=") || strings.HasPrefix(entry, "DFW_JSC_PROBE_CHILD=") {
					continue
				}
				command.Env = append(command.Env, entry)
			}
			command.Env = append(command.Env, "DFW_JSC_PROBE_CHILD="+mode)
			out, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("probe '%s': %v\n%s", mode, err, out)
			}
			if mode != "default" && mode != "initialized" && strings.Contains(string(out), "Overriding existing handler") {
				t.Fatalf("unexpected signal collision:\n%s", out)
			}
			if strings.Contains(string(out), "invalid option") {
				t.Fatalf("setup injected an invalid JSC option:\n%s", out)
			}
			t.Log(strings.TrimSpace(string(out)))
		})
	}
}

func runJSCSignalProbe(t *testing.T, mode string) {
	t.Helper()
	if !jscProbeAvailable() {
		t.Fatal("native probe needs WebKit exporting JSConfigureSignalForGC; normal startup does not")
	}
	free := jscProbeFreeSignal()
	if free == 0 {
		t.Fatal("native probe needs an initially unused real-time signal")
	}
	usr1 := jscProbeUSR1()
	before := jscProbeHandler(usr1)
	switch mode {
	case "default":
		if jscProbeGC() != 20000 {
			t.Fatal("JSC allocation/GC/release failed")
		}
		if before == jscProbeHandler(usr1) {
			t.Fatal("baseline did not reproduce the Go/JSC collision")
		}
		fmt.Printf("baseline replaces Go signal %d\n", usr1)
		return
	case "explicit", "explicit-empty":
		value := "37"
		if mode == "explicit-empty" {
			value = ""
		}
		if err := os.Setenv(jscSignalEnv, value); err != nil {
			t.Fatal(err)
		}
		prepareNativeJavaScriptSignals()
		if processJSCSignal.signal != 0 || jscProbeHandler(usr1) != before || jscProbeHandler(free) != 0 {
			t.Fatal("explicit configuration changed")
		}
		if actual, exists := os.LookupEnv(jscSignalEnv); !exists || actual != value {
			t.Fatal("explicit environment changed")
		}
		return
	case "disabled":
		if err := os.Setenv(disableJSCSignalEnv, "1"); err != nil {
			t.Fatal(err)
		}
		prepareNativeJavaScriptSignals()
		if processJSCSignal.signal != 0 || jscProbeHandler(usr1) != before || jscProbeHandler(free) != 0 {
			t.Fatal("opt-out was ignored")
		}
		return
	case "occupied", "blocked":
		if !jscProbeOccupy(mode == "blocked") {
			t.Fatal("could not reserve fixture signals")
		}
		reserved := jscProbeHandler(free)
		prepareNativeJavaScriptSignals()
		if processJSCSignal.signal != 0 || jscProbeHandler(usr1) != before || jscProbeHandler(free) != reserved {
			t.Fatal("claimed an owned or blocked signal")
		}
		return
	case "initialized":
		if jscProbeGC() != 20000 {
			t.Fatal("early JSC initialization failed")
		}
		installed := jscProbeHandler(usr1)
		prepareNativeJavaScriptSignals()
		if processJSCSignal.signal != 0 || jscProbeHandler(usr1) != installed || jscProbeHandler(free) != 0 {
			t.Fatal("reconfigured an initialized runtime")
		}
		return
	case "automatic":
		prepareNativeJavaScriptSignals()
		chosen := processJSCSignal.signal
		if chosen != free {
			t.Fatalf("selected=%d, expected unused signal=%d", chosen, free)
		}
		for range 3 {
			if jscProbeGC() != 20000 {
				t.Fatal("JSC allocation/GC/release failed")
			}
		}
		if jscProbeHandler(usr1) != before {
			t.Fatal("Go SIGUSR1 handler was replaced")
		}
		if jscProbeHandler(chosen) == 0 {
			t.Fatal("JSC did not install its selected signal")
		}
		prepareNativeJavaScriptSignals()
		if processJSCSignal.signal != chosen {
			t.Fatal("later window reconfigured the signal")
		}
		if _, exists := os.LookupEnv(jscSignalEnv); exists {
			t.Fatal("setup polluted JSC's option environment")
		}
		fmt.Printf("JSC signal %d; Go SIGUSR1 preserved; GC and release passed\n", chosen)
	default:
		t.Fatalf("unknown probe '%s'", mode)
	}
}
