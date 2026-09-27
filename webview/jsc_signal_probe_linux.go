//go:build linux && dfw_jsc_probe

package webview

/*
#cgo pkg-config: javascriptcoregtk-4.1
#include <JavaScriptCore/JavaScript.h>
#include <dlfcn.h>
#include <pthread.h>
#include <signal.h>
#include <stdint.h>

static int dfw_probe_jsc_available(void) {
    return dlsym(RTLD_DEFAULT, "JSConfigureSignalForGC") != NULL;
}
static int dfw_probe_usr1(void) { return SIGUSR1; }
static uintptr_t dfw_probe_handler(int signal) {
    struct sigaction action;
    if (sigaction(signal, NULL, &action)) return (uintptr_t)-1;
    return (uintptr_t)action.sa_handler;
}
static int dfw_probe_free_signal(void) {
    sigset_t blocked;
    if (pthread_sigmask(SIG_BLOCK, NULL, &blocked)) return 0;
    for (int signal = SIGRTMIN; signal <= SIGRTMAX; signal++) {
        struct sigaction action;
        if (!sigaction(signal, NULL, &action) && action.sa_handler == SIG_DFL
            && !(action.sa_flags & SA_SIGINFO) && sigismember(&blocked, signal) == 0)
            return signal;
    }
    return 0;
}
static int dfw_probe_occupy_signals(int block) {
    sigset_t blocked;
    sigemptyset(&blocked);
    for (int signal = SIGRTMIN; signal <= SIGRTMAX; signal++) {
        struct sigaction action;
        if (sigaction(signal, NULL, &action)) return 0;
        if (action.sa_handler != SIG_DFL || (action.sa_flags & SA_SIGINFO)) continue;
        if (block) sigaddset(&blocked, signal);
        else {
            struct sigaction ignored = {0};
            sigemptyset(&ignored.sa_mask);
            ignored.sa_handler = SIG_IGN;
            if (sigaction(signal, &ignored, NULL)) return 0;
        }
    }
    return !block || pthread_sigmask(SIG_BLOCK, &blocked, NULL) == 0;
}
static int dfw_probe_jsc_gc(void) {
    JSGlobalContextRef context = JSGlobalContextCreate(NULL);
    JSStringRef source = JSStringCreateWithUTF8CString("var kept=[]; for(var n=0;n<20000;n++) kept.push({number:n,text:'gc probe'}); kept.length");
    JSValueRef exception = NULL;
    JSValueRef value = JSEvaluateScript(context, source, NULL, NULL, 0, &exception);
    int count = exception ? -1 : (int)JSValueToNumber(context, value, NULL);
    JSStringRelease(source);
    JSGarbageCollect(context);
    JSGlobalContextRelease(context);
    return count;
}
*/
import "C"

func jscProbeAvailable() bool            { return C.dfw_probe_jsc_available() != 0 }
func jscProbeUSR1() int                  { return int(C.dfw_probe_usr1()) }
func jscProbeHandler(signal int) uintptr { return uintptr(C.dfw_probe_handler(C.int(signal))) }
func jscProbeFreeSignal() int            { return int(C.dfw_probe_free_signal()) }
func jscProbeOccupy(block bool) bool {
	mode := 0
	if block {
		mode = 1
	}
	return C.dfw_probe_occupy_signals(C.int(mode)) != 0
}
func jscProbeGC() int { return int(C.dfw_probe_jsc_gc()) }
