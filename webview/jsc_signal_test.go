package webview

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestJSCSignalSetupHonorsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"default", nil, true},
		{"explicit", map[string]string{jscSignalEnv: "37"}, false},
		{"explicit-empty", map[string]string{jscSignalEnv: ""}, false},
		{"disabled", map[string]string{disableJSCSignalEnv: "1"}, false},
		{"disabled-true", map[string]string{disableJSCSignalEnv: " TRUE "}, false},
		{"enabled-zero", map[string]string{disableJSCSignalEnv: "0"}, true},
		{"enabled-false", map[string]string{disableJSCSignalEnv: "false"}, true},
		{"enabled-no", map[string]string{disableJSCSignalEnv: "no"}, true},
		{"enabled-off", map[string]string{disableJSCSignalEnv: "off"}, true},
		{"enabled-empty", map[string]string{disableJSCSignalEnv: ""}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var setup jscSignalSetup
			calls := 0
			lookup := func(key string) (string, bool) { value, ok := tc.env[key]; return value, ok }
			configure := func() int { calls++; return 99 }
			got := setup.prepare(lookup, configure)
			want := 0
			if tc.want {
				want = 99
			}
			if got != want || ((calls == 1) != tc.want) {
				t.Fatalf("selected=%d calls=%d", got, calls)
			}
			// the process decision is stable even if later code changes its env.
			if again := setup.prepare(func(string) (string, bool) { return "", false }, configure); again != got {
				t.Fatal("process decision changed")
			}
			if calls > 1 {
				t.Fatal("native setup repeated")
			}
		})
	}
}

func TestJSCSignalSetupOnceAcrossConcurrentCallers(t *testing.T) {
	var setup jscSignalSetup
	var calls atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := setup.prepare(func(string) (string, bool) { return "", false }, func() int { calls.Add(1); return 99 })
			if got != 99 {
				t.Errorf("selected=%d", got)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("native setup called %d times", calls.Load())
	}
}

func TestJSCSignalUnavailablePreservesStartup(t *testing.T) {
	var setup jscSignalSetup
	if got := setup.prepare(func(string) (string, bool) { return "", false }, func() int { return 0 }); got != 0 {
		t.Fatal("unavailable setup acquired a signal")
	}
}
