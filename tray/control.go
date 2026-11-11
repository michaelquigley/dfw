package tray

import (
	"errors"
	"sync"
)

// Control changes a running daemon's tray and ends the daemon. create one
// with NewControl, keep it wherever the application decides what the tray
// should show, and pass it to Daemon in DaemonApp.Control.
//
// every method is safe from any goroutine and tolerates a nil receiver.
// changes made before Daemon has built its tray are recorded and applied when
// the tray appears, so a change is never lost to startup ordering; the latest
// value of each setting wins. once Daemon is returning, every method does
// nothing. a Control serves one Daemon call; passing it to a second one makes
// that call return an error.
type Control struct {
	mu        sync.Mutex
	phase     controlPhase
	backend   trayBackend
	itemCount int
	stop      func()
	stopped   bool

	icon    []byte
	tooltip *string
	items   map[int]*itemOverride
}

type controlPhase int

const (
	// controlIdle: not yet given to Daemon. changes and Stop are recorded.
	controlIdle controlPhase = iota
	// controlClaimed: Daemon is starting; the tray is not built yet. changes
	// are recorded, Stop reaches the daemon.
	controlClaimed
	// controlLive: the tray is built. changes are applied as they arrive.
	controlLive
	// controlEnded: the daemon is quitting or has returned. nothing applies.
	controlEnded
)

type itemOverride struct {
	label   *string
	enabled *bool
}

// trayBackend applies tray changes. the systray implementation lives in
// tray.go; tests substitute a recording one. Control serializes every call.
type trayBackend interface {
	SetIcon(icon []byte)
	SetTooltip(tooltip string)
	SetItemLabel(index int, label string)
	SetItemEnabled(index int, enabled bool)
}

var errControlReused = errors.New("dfw: tray control already used by a Daemon call")

// NewControl returns a Control ready to be passed to Daemon.
func NewControl() *Control {
	return &Control{}
}

// SetIcon replaces the tray icon with PNG bytes. it returns an error, and
// changes nothing, when png is empty or does not decode.
func (c *Control) SetIcon(png []byte) error {
	if c == nil {
		return nil
	}
	if len(png) == 0 {
		return errors.New("dfw: tray icon: empty PNG")
	}
	icon, err := trayIconBytes(png)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.phase {
	case controlLive:
		c.backend.SetIcon(icon)
	case controlIdle, controlClaimed:
		c.icon = icon
	}
	return nil
}

// SetTooltip replaces the tray tooltip. the tray title set from
// DaemonApp.Title is unchanged.
func (c *Control) SetTooltip(tooltip string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.phase {
	case controlLive:
		c.backend.SetTooltip(tooltip)
	case controlIdle, controlClaimed:
		c.tooltip = &tooltip
	}
}

// Item returns the handle for DaemonApp.TrayItems[index]. an index that does
// not name an item yields a handle whose methods do nothing.
func (c *Control) Item(index int) *Item {
	if c == nil {
		return nil
	}
	return &Item{control: c, index: index}
}

// Stop ends the daemon as choosing Quit does when OnQuit is absent or allows
// it: the tray closes and Daemon returns through its ordinary shutdown,
// stopping the HTTP server and removing the runtime file. OnQuit is not
// consulted; the application is the one deciding. Stop called before Daemon
// makes Daemon stop as soon as its tray is up. it is idempotent, and does
// nothing once the daemon is ending.
func (c *Control) Stop() {
	if c == nil {
		return
	}
	c.mu.Lock()
	switch c.phase {
	case controlIdle:
		c.stopped = true
		c.mu.Unlock()
	case controlClaimed, controlLive:
		stop := c.stop
		c.mu.Unlock()
		// stop runs outside the mutex: it ends this control on its way to
		// quitting the tray.
		stop()
	default:
		c.mu.Unlock()
	}
}

// Item changes one product item in a running daemon's tray menu. its methods
// share the Control's guarantees: safe from any goroutine, nil-tolerant,
// recorded before the tray exists, inert once the daemon is ending.
type Item struct {
	control *Control
	index   int
}

// SetLabel replaces the item's label.
func (i *Item) SetLabel(label string) {
	if i == nil || i.control == nil {
		return
	}
	i.control.updateItem(i.index, func(o *itemOverride) { o.label = &label }, func(b trayBackend) {
		b.SetItemLabel(i.index, label)
	})
}

// Enable makes the item clickable.
func (i *Item) Enable() {
	i.setEnabled(true)
}

// Disable greys the item out; clicks on it are not delivered.
func (i *Item) Disable() {
	i.setEnabled(false)
}

func (i *Item) setEnabled(enabled bool) {
	if i == nil || i.control == nil {
		return
	}
	i.control.updateItem(i.index, func(o *itemOverride) { o.enabled = &enabled }, func(b trayBackend) {
		b.SetItemEnabled(i.index, enabled)
	})
}

func (c *Control) updateItem(index int, record func(*itemOverride), apply func(trayBackend)) {
	if index < 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.phase {
	case controlLive:
		if index < c.itemCount {
			apply(c.backend)
		}
	case controlIdle, controlClaimed:
		if c.items == nil {
			c.items = make(map[int]*itemOverride)
		}
		override := c.items[index]
		if override == nil {
			override = &itemOverride{}
			c.items[index] = override
		}
		record(override)
	}
}

// claim attaches the control to a starting Daemon. stop must be idempotent; a
// Stop recorded before the claim is delivered to it now.
func (c *Control) claim(stop func()) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.phase != controlIdle {
		c.mu.Unlock()
		return errControlReused
	}
	c.phase = controlClaimed
	c.stop = stop
	stopped := c.stopped
	c.mu.Unlock()

	if stopped {
		stop()
	}
	return nil
}

// bind connects the built tray and applies every recorded change. itemCount is
// len(DaemonApp.TrayItems); recorded changes to other indexes are dropped.
func (c *Control) bind(backend trayBackend, itemCount int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase != controlClaimed {
		return
	}
	c.phase = controlLive
	c.backend = backend
	c.itemCount = itemCount

	if c.icon != nil {
		backend.SetIcon(c.icon)
	}
	if c.tooltip != nil {
		backend.SetTooltip(*c.tooltip)
	}
	for index, override := range c.items {
		if index >= itemCount {
			continue
		}
		if override.label != nil {
			backend.SetItemLabel(index, *override.label)
		}
		if override.enabled != nil {
			backend.SetItemEnabled(index, *override.enabled)
		}
	}
	c.icon, c.tooltip, c.items = nil, nil, nil
}

// end makes the control inert. it must run before the tray is told to quit:
// on Linux, systray panics on an icon change after its D-Bus connection
// closes. it waits for any change in flight and is idempotent.
func (c *Control) end() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.phase = controlEnded
	c.backend = nil
	c.icon, c.tooltip, c.items = nil, nil, nil
}
