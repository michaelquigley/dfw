//go:build darwin

package webview

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nativeMainMenu and nativeUIDelegate are what applyNativeApp left behind,
// read in TestMain: AppKit wants the main thread, and only TestMain's
// goroutine holds it (webview_go locks it in init).
var (
	nativeMainMenu          map[string][]string
	nativeUIDelegate        string
	nativeWindowStyle       string
	nativeQuitCloseRequests int

	// the same, read again after detachNativeApp, which is given no window
	// handle, as after an ordinary close.
	detachedMainMenu          map[string][]string
	detachedUIDelegate        string
	detachedQuitCloseRequests int
)

func TestMain(m *testing.M) {
	window := newNativeHiddenWindow()
	applyNativeApp(window)
	nativeMainMenu = parseNativeMenu(describeNativeMainMenu())
	nativeUIDelegate = describeNativeUIDelegate(window)
	nativeWindowStyle = describeNativeWindowStyle(window)
	nativeQuitCloseRequests = nativeQuitEventCloseRequests(window)
	detachNativeApp()
	detachedMainMenu = parseNativeMenu(describeNativeMainMenu())
	detachedUIDelegate = describeNativeUIDelegate(window)
	detachedQuitCloseRequests = nativeQuitEventCloseRequests(window)
	os.Exit(m.Run())
}

// the hidden window starts titled only, as webview_go's does until SetSize.
func TestNativeWindowIsClosableWithoutASize(t *testing.T) {
	assert.Equal(t, "closable=1 miniaturizable=1", nativeWindowStyle)
}

// the Dock's Quit and a logout send the quit Apple Event. NSApplication's
// own handler would call terminate: and end the process inside the run loop;
// dfw's asks the window to close, once.
func TestNativeQuitEventRequestsWindowClose(t *testing.T) {
	assert.Equal(t, 1, nativeQuitCloseRequests)
}

// applyNativeApp drains its own autorelease pool before it returns, so a
// delegate held only by the web view's weak reference, as webview_go's is,
// would read nil here.
func TestNativeUIDelegateOutlivesTheSetup(t *testing.T) {
	assert.Equal(t, "DFWUIDelegate openPanel=1 newWindow=1", nativeUIDelegate)
}

func parseNativeMenu(description string) map[string][]string {
	menus := map[string][]string{}
	for _, menu := range strings.Split(description, "|") {
		title, items, _ := strings.Cut(menu, ": ")
		menus[title] = strings.Split(items, "; ")
	}
	return menus
}

func TestNativeMainMenuHasEditKeys(t *testing.T) {
	edit := nativeMainMenu["Edit"]
	for _, item := range []string{
		"Undo=cmd+z/undo:>responder",
		"Redo=shift+cmd+z/redo:>responder",
		"Cut=cmd+x/cut:>responder",
		"Copy=cmd+c/copy:>responder",
		"Paste=cmd+v/paste:>responder",
		"Paste and Match Style=opt+shift+cmd+v/pasteAsPlainText:>responder",
		"Select All=cmd+a/selectAll:>responder",
	} {
		assert.Contains(t, edit, item)
	}
}

// nativeAppMenu is the application menu, the one that is neither Edit nor
// Window, and its Quit item.
func nativeAppMenu(t *testing.T, menus map[string][]string) (app []string, quit string) {
	for title, items := range menus {
		if title != "Edit" && title != "Window" {
			app = items
		}
	}
	require.NotEmpty(t, app, "no application menu in %v", menus)
	for _, item := range app {
		if strings.HasPrefix(item, "Quit ") {
			quit = item
		}
	}
	return app, quit
}

func TestNativeMainMenuQuitTargetsWindowClose(t *testing.T) {
	app, quit := nativeAppMenu(t, nativeMainMenu)
	// Quit is the window's own close, never NSApp terminate:, which would end
	// the process before Window returns and saves its state.
	assert.True(t, strings.HasSuffix(quit, "=cmd+q/performClose:>NSWindow"), "quit item: %q", quit)
	assert.Contains(t, app, "Hide Others=opt+cmd+h/hideOtherApplications:>responder")
}

// Destroy detaches what applyNativeApp wired to the window before the binding
// destroys it: a quit event and the menu's Quit no longer reach the window,
// and the web view no longer has dfw's UI delegate.
func TestNativeDetachLeavesTheHooksInert(t *testing.T) {
	assert.Equal(t, 0, detachedQuitCloseRequests, "a quit event after detach")
	assert.Equal(t, "nil", detachedUIDelegate)
	_, quit := nativeAppMenu(t, detachedMainMenu)
	require.NotEmpty(t, quit, "no Quit item in %v", detachedMainMenu)
	assert.NotContains(t, quit, "performClose:", "quit item: %q", quit)
	assert.False(t, strings.HasSuffix(quit, ">NSWindow"), "quit item: %q", quit)
}

func TestNativeMainMenuHasWindowKeys(t *testing.T) {
	window := nativeMainMenu["Window"]
	assert.Contains(t, window, "Minimize=cmd+m/performMiniaturize:>responder")
	assert.Contains(t, window, "Close=cmd+w/performClose:>responder")
}
