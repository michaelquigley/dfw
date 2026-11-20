package tray

import "sync"

// reopen holds the running daemon's OnReopen for the native reopen event,
// which knows no daemon. a process runs one tray, so one handler.
var reopen struct {
	mu      sync.Mutex
	handler func()
}

func setReopenHandler(handler func()) {
	reopen.mu.Lock()
	defer reopen.mu.Unlock()
	reopen.handler = handler
}

// reopenHandler binds the product's OnReopen to the daemon's address, which
// the native event does not carry.
func reopenHandler(onReopen func(daemonAddr string), daemonAddr string) func() {
	return func() { onReopen(daemonAddr) }
}

// reopened runs the handler, if any, on a goroutine of its own, so the
// native event loop that delivered the event never waits on the product.
func reopened() {
	reopen.mu.Lock()
	handler := reopen.handler
	reopen.mu.Unlock()
	if handler != nil {
		go handler()
	}
}
