//go:build linux

package webview

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <pthread.h>
#include <signal.h>
#include <stdbool.h>
#include <stdlib.h>

// this private API is exported on supported WebKit builds, but not declared
// by their installed public headers. do not introduce a load-time dependency.
static void *dfw_jsc_signal_configuration(void) {
    return dlsym(RTLD_DEFAULT, "JSConfigureSignalForGC");
}

static int dfw_configure_jsc_signal(void *entry) {
    if (!entry || getenv("JSC_SIGNAL_FOR_GC") != NULL) return 0;
    bool (*configure)(int) = (bool (*)(int))entry;
    sigset_t blocked;
    if (pthread_sigmask(SIG_BLOCK, NULL, &blocked)) return 0;

    // libc's usable range excludes its private threading signals. leave every
    // installed/ignored or blocked signal alone, including Go's runtime hooks.
    for (int signal = SIGRTMIN; signal <= SIGRTMAX; signal++) {
        struct sigaction action;
        if (sigaction(signal, NULL, &action)) continue;
        if (action.sa_handler != SIG_DFL || (action.sa_flags & SA_SIGINFO)) continue;
        if (sigismember(&blocked, signal) != 0) continue;
        // refusal means initialization has already fixed the signal. trying
        // another number cannot repair that and must not mutate its handler.
        return configure(signal) ? signal : 0;
    }
    return 0;
}
*/
import "C"

import "unsafe"

func configureNativeJSCSignal() int {
	return configureJSCSignalEntry(C.dfw_jsc_signal_configuration())
}

func configureJSCSignalEntry(entry unsafe.Pointer) int {
	return int(C.dfw_configure_jsc_signal(entry))
}
