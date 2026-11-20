//go:build darwin

package webview

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nativeMainMenu and nativeUIDelegate are what newNativeApp left behind,
// read in TestMain: AppKit wants the main thread, and only TestMain's
// goroutine holds it (webview_go locks it in init).
var (
	// the menu bar before newNativeApp, a host's own, installed by the test;
	// and the main menu's identity before newNativeApp, during, and after
	// Close.
	menuBeforeApp string
	menuIDBefore  uintptr
	menuIDDuring  uintptr
	menuIDAfter   uintptr

	nativeMainMenu          map[string][]string
	nativeMainMenuText      string
	nativeUIDelegate        string
	nativeWindowStyle       string
	nativeQuitCloseRequests int

	// the same, read again after the controller's Close, which reaches the
	// hooks through the references it kept, not the window handle, as after an
	// ordinary close.
	detachedMainMenu          string
	detachedUIDelegate        string
	detachedQuitHandlerWindow bool
	detachedQuitCloseRequests int
	detachedQuitFallback      bool
	// the quit handler once releaseNativeApp has run.
	releasedQuitFallback bool

	// the process's activation policy before prepareNativeApp, after it, and
	// after releaseNativeApp.
	policyBefore   int
	policyPrepared int
	policyReleased int

	// a chooser pending across the detach: how many times a second request
	// was answered at once, and how many times WebKit was answered in all.
	secondChooserAnswers  int
	pendingChooserAnswers int
	// a chooser asked for from a web view with no window.
	windowlessChooserAnswers int

	// the window's own live flag: after Activate, after EndActivation, and
	// after Activate on a closed controller.
	liveAfterActivate      bool
	liveAfterEndActivation bool
	liveAfterClose         bool

	// a quit event once the window's loop has ended, before Close.
	endedQuitCloseRequests int
)

func TestMain(m *testing.M) {
	window := newNativeHiddenWindow()
	// an accessory, as a process started from an LSUIElement bundle is. the
	// test binary's own policy, unbundled, is prohibited, which AppKit may
	// refuse to return to.
	setNativeActivationPolicy(nativeAccessoryPolicy)
	policyBefore = nativeActivationPolicy()
	prepareNativeApp()
	policyPrepared = nativeActivationPolicy()
	installNativeTestMenu()
	menuBeforeApp = describeNativeMainMenu()
	menuIDBefore = nativeMainMenuID()
	app := newNativeApp(window)
	nativeMainMenuText = describeNativeMainMenu()
	menuIDDuring = nativeMainMenuID()
	nativeMainMenu = parseNativeMenu(nativeMainMenuText)
	nativeUIDelegate = describeNativeUIDelegate(window)
	nativeWindowStyle = describeNativeWindowStyle(window)
	nativeQuitCloseRequests = nativeQuitEventCloseRequests(window)
	windowlessChooserAnswers = windowlessNativeTestOpenPanel()
	armNativeTestOpenPanel(window)
	secondChooserAnswers = secondNativeTestOpenPanel()
	// the main queue never runs here, so the focus work Activate schedules
	// never fires; only the flag it checks is read.
	app.Activate()
	liveAfterActivate = nativeAppLive(app)
	app.EndActivation()
	liveAfterEndActivation = nativeAppLive(app)
	// the loop has ended, Close has not run: a quit event is ignored.
	endedQuitCloseRequests = nativeQuitEventCloseRequests(window)
	app.Close()
	app.Activate()
	liveAfterClose = nativeAppLive(app)
	detachedMainMenu = describeNativeMainMenu()
	menuIDAfter = nativeMainMenuID()
	detachedUIDelegate = describeNativeUIDelegate(window)
	detachedQuitHandlerWindow = nativeQuitHandlerHasWindow()
	detachedQuitFallback = nativeQuitHandlerFallsBack()
	// during teardown a quit event is ignored, so it can still be sent here.
	detachedQuitCloseRequests = nativeQuitEventCloseRequests(window)
	releaseNativeApp()
	// a quit event now would terminate the test binary, as AppKit's handler
	// does; only the handler's state is read.
	releasedQuitFallback = nativeQuitHandlerFallsBack()
	policyReleased = nativeActivationPolicy()
	pendingChooserAnswers = finishNativeTestOpenPanel()
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

// newNativeApp drains its own autorelease pool before it returns, so a
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

// Destroy closes the controller, which undoes what newNativeApp wired to the
// window before the binding destroys it: the quit handler lets go of the
// window, the menu bar dfw replaced is back, and the web view no longer has
// dfw's UI delegate.
func TestNativeDetachLeavesTheHooksInert(t *testing.T) {
	assert.False(t, detachedQuitHandlerWindow, "the quit handler still answers for the window")
	assert.False(t, detachedQuitFallback, "the quit handler would terminate during teardown")
	assert.Equal(t, 0, detachedQuitCloseRequests, "a quit event during teardown")
	assert.Equal(t, "nil", detachedUIDelegate)
	assert.NotEqual(t, menuIDBefore, menuIDDuring, "newNativeApp installed no menu bar")
	assert.Contains(t, menuBeforeApp, "Host: Marker=", "the host's menu bar before newNativeApp")
	// the host's own menu bar is the one back, and none of dfw's menus are.
	assert.Equal(t, menuIDBefore, menuIDAfter, "the menu bar after Close")
	assert.Contains(t, detachedMainMenu, "Host: Marker=")
	assert.NotContains(t, detachedMainMenu, "Edit: ")
}

// a chooser still open at teardown is answered by the detach, and its sheet's
// result arriving afterwards finds nothing left to answer: WebKit's completion
// runs exactly once.
func TestNativeDetachAnswersAPendingChooserOnce(t *testing.T) {
	assert.Equal(t, 1, pendingChooserAnswers)
}

// a second chooser asked for while one is open is answered with no files at
// once, rather than displacing the first.
func TestNativeSecondChooserIsAnsweredAtOnce(t *testing.T) {
	assert.Equal(t, 1, secondChooserAnswers)
}

// a window's focus work checks its own live flag: set by Activate, cleared by
// EndActivation as the run loop returns, and never set again once the
// controller is closed.
func TestNativeAppActivationIsTiedToItsWindow(t *testing.T) {
	assert.True(t, liveAfterActivate, "after Activate")
	assert.False(t, liveAfterEndActivation, "after EndActivation")
	assert.False(t, liveAfterClose, "Activate after Close")
}

// once teardown has finished, the quit handler stands in for AppKit's and ends
// the process on a quit event.
func TestNativeReleaseFallsBackToTerminate(t *testing.T) {
	assert.True(t, releasedQuitFallback)
}

// the window process takes the regular policy for its window, and gives back
// the one it replaced once the window is gone.
func TestNativeReleaseRestoresTheActivationPolicy(t *testing.T) {
	assert.Equal(t, nativeRegularPolicy, policyPrepared, "after prepareNativeApp")
	assert.Equal(t, policyBefore, policyReleased, "after releaseNativeApp")
}

// a chooser asked for once the window has gone is answered with no files at
// once, never with a standalone modal panel.
func TestNativeWindowlessChooserIsAnsweredAtOnce(t *testing.T) {
	assert.Equal(t, 1, windowlessChooserAnswers)
}

// once the window's loop has ended, before Close takes the hooks down, a quit
// event no longer reaches the window.
func TestNativeQuitAfterTheLoopEndsIsIgnored(t *testing.T) {
	assert.Equal(t, 0, endedQuitCloseRequests)
}
