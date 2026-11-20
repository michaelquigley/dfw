//go:build darwin

package webview

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#include <stdlib.h>
#include <string.h>
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

// the name a bundle gives itself, else the process name: what macOS shows
// as the application menu's title, so Hide and Quit say the same.
static NSString *dfw_app_name(void) {
	NSString *name = [[NSBundle mainBundle] objectForInfoDictionaryKey:@"CFBundleName"];
	if ([name length] == 0) {
		name = [[NSProcessInfo processInfo] processName];
	}
	return name;
}

static NSMenuItem *dfw_menu_item(NSMenu *menu, NSString *title, SEL action, NSString *key, NSEventModifierFlags mods) {
	NSMenuItem *item = [menu addItemWithTitle:title action:action keyEquivalent:key];
	if (mods != 0) {
		[item setKeyEquivalentModifierMask:mods];
	}
	return item;
}

// the standard menus a window needs on macOS. without an Edit menu, WebKit
// receives no Cut, Copy, Paste, Select All or Undo key equivalents. Quit
// closes window, the one this process owns, rather than terminating the
// application, so the run loop ends the way the close button ends it. the
// Quit item is handed back too, for the hooks to disarm at detach.
static NSMenu *dfw_main_menu(NSWindow *window, NSMenu **windowsMenu, NSMenuItem **quitItem) {
	NSString *name = dfw_app_name();
	NSEventModifierFlags cmd = NSEventModifierFlagCommand;
	NSMenu *bar = [[NSMenu alloc] init];

	NSMenuItem *appItem = [bar addItemWithTitle:@"" action:nil keyEquivalent:@""];
	NSMenu *app = [[NSMenu alloc] initWithTitle:name];
	dfw_menu_item(app, [@"About " stringByAppendingString:name], @selector(orderFrontStandardAboutPanel:), @"", 0);
	[app addItem:[NSMenuItem separatorItem]];
	dfw_menu_item(app, [@"Hide " stringByAppendingString:name], @selector(hide:), @"h", 0);
	dfw_menu_item(app, @"Hide Others", @selector(hideOtherApplications:), @"h", cmd | NSEventModifierFlagOption);
	dfw_menu_item(app, @"Show All", @selector(unhideAllApplications:), @"", 0);
	[app addItem:[NSMenuItem separatorItem]];
	NSMenuItem *quit = dfw_menu_item(app, [@"Quit " stringByAppendingString:name], @selector(performClose:), @"q", 0);
	[quit setTarget:window];
	*quitItem = quit;
	[appItem setSubmenu:app];

	NSMenuItem *editItem = [bar addItemWithTitle:@"" action:nil keyEquivalent:@""];
	NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Edit"];
	dfw_menu_item(edit, @"Undo", @selector(undo:), @"z", 0);
	dfw_menu_item(edit, @"Redo", @selector(redo:), @"z", cmd | NSEventModifierFlagShift);
	[edit addItem:[NSMenuItem separatorItem]];
	dfw_menu_item(edit, @"Cut", @selector(cut:), @"x", 0);
	dfw_menu_item(edit, @"Copy", @selector(copy:), @"c", 0);
	dfw_menu_item(edit, @"Paste", @selector(paste:), @"v", 0);
	dfw_menu_item(edit, @"Paste and Match Style", @selector(pasteAsPlainText:), @"v", cmd | NSEventModifierFlagShift | NSEventModifierFlagOption);
	dfw_menu_item(edit, @"Select All", @selector(selectAll:), @"a", 0);
	[editItem setSubmenu:edit];

	NSMenuItem *windowItem = [bar addItemWithTitle:@"" action:nil keyEquivalent:@""];
	NSMenu *windows = [[NSMenu alloc] initWithTitle:@"Window"];
	dfw_menu_item(windows, @"Minimize", @selector(performMiniaturize:), @"m", 0);
	dfw_menu_item(windows, @"Zoom", @selector(performZoom:), @"", 0);
	[windows addItem:[NSMenuItem separatorItem]];
	dfw_menu_item(windows, @"Close", @selector(performClose:), @"w", 0);
	[windowItem setSubmenu:windows];

	*windowsMenu = windows;
	return bar;
}

static WKWebView *dfw_find_web_view(NSView *view) {
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView *)view;
	}
	for (NSView *sub in [view subviews]) {
		WKWebView *found = dfw_find_web_view(sub);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

// DFWUIDelegate is the web view's UI delegate on macOS, in place of
// webview_go's. that one is autoreleased as it is set, and WKWebView holds
// its delegate weakly, so it is gone before the run loop turns: the file
// chooser never opens and new-window links are dropped. this one is held by
// the web view itself (dfw_own_ui_delegate) and answers both.
@interface DFWUIDelegate : NSObject <WKUIDelegate>
// WebKit's completion for the chooser that is open, and the chooser. WebKit
// requires the completion to be called exactly once, so it is kept here,
// where a detach can answer it, and not only in the sheet's handler.
@property (copy) void (^openPanelCompletion)(NSArray<NSURL *> *URLs);
@property (weak) NSOpenPanel *openPanel;
- (void)finishOpenPanel:(NSArray<NSURL *> *)URLs;
- (void)cancelOpenPanel;
@end

@implementation DFWUIDelegate

// <input type="file">: a sheet on the window. a webkitdirectory input
// chooses folders, any other chooses files.
- (void)webView:(WKWebView *)webView
	runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters
	initiatedByFrame:(WKFrameInfo *)frame
	completionHandler:(void (^)(NSArray<NSURL *> *URLs))completionHandler {
	// one chooser at a time: a second request is answered with no files at
	// once, rather than displacing the first, whose completion is still due.
	if ([self openPanelCompletion] != nil) {
		completionHandler(nil);
		return;
	}
	// a web view without a window is one whose window has closed: a late
	// request gets no files, never a standalone modal panel nothing could
	// cancel at teardown.
	NSWindow *window = [webView window];
	if (window == nil) {
		completionHandler(nil);
		return;
	}
	NSOpenPanel *panel = [NSOpenPanel openPanel];
	BOOL directories = [parameters allowsDirectories];
	[panel setCanChooseDirectories:directories];
	[panel setCanChooseFiles:!directories];
	[panel setAllowsMultipleSelection:[parameters allowsMultipleSelection]];
	[self setOpenPanelCompletion:completionHandler];
	[self setOpenPanel:panel];
	[panel beginSheetModalForWindow:window completionHandler:^(NSModalResponse response) {
		[self finishOpenPanel:(response == NSModalResponseOK ? [panel URLs] : nil)];
	}];
}

// answers WebKit once, for whichever comes first: the chooser's own result
// or a detach. the other finds nothing left to answer.
- (void)finishOpenPanel:(NSArray<NSURL *> *)URLs {
	void (^completion)(NSArray<NSURL *> *) = [self openPanelCompletion];
	[self setOpenPanelCompletion:nil];
	[self setOpenPanel:nil];
	if (completion != nil) {
		completion(URLs);
	}
}

// answers a chooser still open at teardown with no files, then takes its
// sheet down.
- (void)cancelOpenPanel {
	NSOpenPanel *panel = [self openPanel];
	[self finishOpenPanel:nil];
	NSWindow *parent = [panel sheetParent];
	if (parent != nil) {
		[parent endSheet:panel returnCode:NSModalResponseCancel];
	}
}

// a link with target="_blank", or window.open: an http, https or mailto
// address opens in the user's default handler. no second web view is made.
- (WKWebView *)webView:(WKWebView *)webView
	createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration
	forNavigationAction:(WKNavigationAction *)action
	windowFeatures:(WKWindowFeatures *)features {
	NSURL *url = [[action request] URL];
	NSString *scheme = [[url scheme] lowercaseString];
	if ([scheme isEqualToString:@"http"] || [scheme isEqualToString:@"https"] || [scheme isEqualToString:@"mailto"]) {
		[[NSWorkspace sharedWorkspace] openURL:url];
	}
	return nil;
}

@end

static char dfw_ui_delegate_key;

// sets dfw's delegate on the window's web view and returns that web view, or
// nil if the window holds none.
static WKWebView *dfw_own_ui_delegate(NSWindow *window) {
	WKWebView *web = dfw_find_web_view([window contentView]);
	if (web == nil) {
		return nil;
	}
	DFWUIDelegate *delegate = [[DFWUIDelegate alloc] init];
	objc_setAssociatedObject(web, &dfw_ui_delegate_key, delegate, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
	[web setUIDelegate:delegate];
	return web;
}

// the web view's delegate class, for the tests.
static char *dfw_describe_ui_delegate(void *window) {
	WKWebView *web = dfw_find_web_view([(__bridge NSWindow *)window contentView]);
	id delegate = [web UIDelegate];
	if (delegate == nil) {
		return strdup("nil");
	}
	NSString *s = [NSString stringWithFormat:@"%@ openPanel=%d newWindow=%d", NSStringFromClass([delegate class]),
		[delegate respondsToSelector:@selector(webView:runOpenPanelWithParameters:initiatedByFrame:completionHandler:)],
		[delegate respondsToSelector:@selector(webView:createWebViewWithConfiguration:forNavigationAction:windowFeatures:)]];
	return strdup([s UTF8String]);
}

// a bundle with LSUIElement, as a menu bar daemon has, makes every process
// it starts an accessory, and webview_go leaves a bundled application's
// activation policy alone. the window process is the visible application,
// so it takes the regular policy: a Dock icon and a menu bar. the policy is
// the process's, so the one it replaced is kept for dfw_release_app to put
// back once the window is gone.
static NSApplicationActivationPolicy dfw_previous_policy;
static BOOL dfw_policy_saved;

static void dfw_prepare_app(void) {
	NSApplication *app = [NSApplication sharedApplication];
	dfw_previous_policy = [app activationPolicy];
	dfw_policy_saved = YES;
	[app setActivationPolicy:NSApplicationActivationPolicyRegular];
}

// gives the process a menu bar of its own before dfw's, as a host might
// have, and reports the main menu's identity, for the tests. a regular
// application with no menu bar gets a bare one AppKit makes up for nil, so the
// test starts from a menu bar it owns.
static void dfw_install_test_menu(void) {
	NSMenu *bar = [[NSMenu alloc] init];
	NSMenuItem *item = [bar addItemWithTitle:@"" action:nil keyEquivalent:@""];
	NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Host"];
	[menu addItemWithTitle:@"Marker" action:nil keyEquivalent:@""];
	[item setSubmenu:menu];
	[NSApp setMainMenu:bar];
}

static uintptr_t dfw_main_menu_id(void) {
	return (uintptr_t)(__bridge void *)[NSApp mainMenu];
}

// the process's activation policy, read and set for the tests.
static int dfw_activation_policy(void) {
	return (int)[[NSApplication sharedApplication] activationPolicy];
}

static void dfw_set_activation_policy(int policy) {
	[[NSApplication sharedApplication] setActivationPolicy:(NSApplicationActivationPolicy)policy];
}

// plain ints, so cgo need not read AppKit's NS_ENUM.
static const int dfw_policy_regular = NSApplicationActivationPolicyRegular;
static const int dfw_policy_accessory = NSApplicationActivationPolicyAccessory;

// DFWQuitHandler answers the quit Apple Event, which the Dock's Quit, a
// logout or restart, and `quit app` from a script send. NSApplication's own
// handler calls terminate:, which exits inside the run loop before Window or
// Run returns, so the window state, the server's shutdown and every deferred
// cleanup are skipped. this one closes the window, as Quit in the menu does.
// it is a handler and not applicationShouldTerminate:, because cancelling
// there cancels a logout too, and a later reply would be due after the run
// loop that has to carry it has ended. it has three states: with a window it
// closes the window; detached, while the binding tears the window down and
// drains its events, it ignores the event, so a late quit cannot end the
// process before its cleanup; released, once teardown has finished, it does
// what AppKit's own handler does: terminate:. AppKit's handler object cannot
// be put back without private API, so this one stands in for it, and a
// process that outlives its window still quits.
@interface DFWQuitHandler : NSObject
@property (weak) NSWindow *window;
@property BOOL fallback;
- (void)handleQuit:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
@end

@implementation DFWQuitHandler
- (void)handleQuit:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
	NSWindow *window = [self window];
	if (window != nil) {
		[window performClose:nil];
		return;
	}
	if ([self fallback]) {
		[NSApp terminate:nil];
	}
}
@end

// the Apple Event manager does not hold its handlers.
static DFWQuitHandler *dfw_quit_handler;

// NSApplication registers its own handler in finishLaunching, which would
// replace this one. it has run by now: webview_go's constructor runs the
// application until applicationDidFinishLaunching:, and the run loop that
// Run enters later does not register it again.
static void dfw_install_quit_handler(NSWindow *window) {
	if (dfw_quit_handler == nil) {
		dfw_quit_handler = [[DFWQuitHandler alloc] init];
	}
	[dfw_quit_handler setWindow:window];
	[dfw_quit_handler setFallback:NO];
	[[NSAppleEventManager sharedAppleEventManager] setEventHandler:dfw_quit_handler
		andSelector:@selector(handleQuit:withReplyEvent:)
		forEventClass:kCoreEventClass
		andEventID:kAEQuitApplication];
}

// webview_go's window is titled only, and gets its close and minimize
// buttons from SetSize, which dfw calls only for a size it knows. a window
// without them ignores performClose: and performMiniaturize:, so Quit, Close,
// Minimize and a quit from the Dock or a logout would all do nothing.
static void dfw_make_closable(NSWindow *window) {
	[window setStyleMask:[window styleMask] | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable];
}

// DFWAppHooks is what dfw_apply_app wired to one window, owned by that
// window's controller (darwinApp) and released by dfw_detach_app. it holds
// the window, its web view and the menu's Quit item weakly: it keeps none of
// them alive, and reaches them without the window handle, which webview_go
// clears as the window closes. live says the window's run loop is running;
// its delayed focus work checks it. main thread only.
@interface DFWAppHooks : NSObject
@property (weak) NSWindow *window;
@property (weak) WKWebView *webView;
@property (weak) NSMenuItem *quitItem;
@property BOOL live;
// the menu bar dfw installed, and the one it replaced, put back at detach.
@property (weak) NSMenu *mainMenu;
@property (strong) NSMenu *previousMainMenu;
@property (strong) NSMenu *previousWindowsMenu;
@end

@implementation DFWAppHooks
@end

// applies the macOS setup to window and returns the hooks that own it, +1 for
// the caller. its own pool, drained before it returns: whatever must outlive
// the call is held by something other than autorelease.
static void *dfw_apply_app(void *window) {
	DFWAppHooks *hooks = [[DFWAppHooks alloc] init];
	@autoreleasepool {
		NSWindow *w = (__bridge NSWindow *)window;
		dfw_make_closable(w);
		NSMenu *windows = nil;
		NSMenuItem *quit = nil;
		NSMenu *bar = dfw_main_menu(w, &windows, &quit);
		[hooks setPreviousMainMenu:[NSApp mainMenu]];
		[hooks setPreviousWindowsMenu:[NSApp windowsMenu]];
		[hooks setMainMenu:bar];
		[NSApp setMainMenu:bar];
		[NSApp setWindowsMenu:windows];
		[hooks setWindow:w];
		[hooks setQuitItem:quit];
		[hooks setWebView:dfw_own_ui_delegate(w)];
		dfw_install_quit_handler(w);
	}
	return (__bridge_retained void *)hooks;
}

// brings window, and only window, to the front with the keyboard in its page.
// webview_go shows the window before the run loop starts, so a window that is
// no longer visible has been closed or minimized since, and is left alone.
static void dfw_front(NSWindow *window) {
	if (window == nil || ![window isVisible]) {
		return;
	}
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	// cooperative activation ([NSApp activate]) is refused here: nothing
	// yields to a process the daemon started. this one is honoured.
	[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
	[window makeKeyAndOrderFront:nil];
	WKWebView *web = dfw_find_web_view([window contentView]);
	if (web != nil) {
		[window makeFirstResponder:web];
	}
}

// brings the window to the front with the keyboard in the page, once the run
// loop turns. a window opened from a tray menu loses the activation to the
// application that was active before the menu, as the menu closes, so it is
// taken once more if it was lost within the first moment. the blocks hold the
// hooks weakly, so they keep nothing alive, and do nothing once the hooks are
// released or no longer live.
static void dfw_activate_app(void *handle) {
	DFWAppHooks *hooks = (__bridge DFWAppHooks *)handle;
	[hooks setLive:YES];
	__weak DFWAppHooks *owner = hooks;
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 200 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
		DFWAppHooks *h = owner;
		if ([h live]) {
			dfw_front([h window]);
		}
	});
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 800 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
		DFWAppHooks *h = owner;
		if ([h live] && ![NSApp isActive]) {
			dfw_front([h window]);
		}
	});
}

// called on the main thread as Run's loop returns. a window can outlive its
// loop still visible (a server failure ends the loop, Destroy comes after),
// so visibility alone does not say the window is still dfw's to bring forward.
static void dfw_end_activation(void *handle) {
	[(__bridge DFWAppHooks *)handle setLive:NO];
}

// called once the binding has destroyed the window and teardown has finished:
// a quit event from now on ends the process, as AppKit's own handler would.
// nothing in dfw services events after this, so it applies only to an event
// loop the product runs itself.
//
// the activation policy prepare replaced is put back too, so a menu bar
// process that outlives its window loses the Dock icon it gained for it.
// AppKit may refuse the switch back (an unbundled process returning to
// prohibited, say); nothing depends on it succeeding.
static void dfw_release_app(void) {
	[dfw_quit_handler setFallback:YES];
	if (dfw_policy_saved) {
		dfw_policy_saved = NO;
		[[NSApplication sharedApplication] setActivationPolicy:dfw_previous_policy];
	}
}

// the quit handler's state, for the tests: whether it still answers for a
// window, and whether it has fallen back to terminate:. a quit event cannot
// be sent once it has, because it would end the process.
static int dfw_quit_handler_has_window(void) {
	return [dfw_quit_handler window] != nil;
}

static int dfw_quit_handler_falls_back(void) {
	return [dfw_quit_handler fallback];
}

// whether the hooks are live, for the tests.
static int dfw_hooks_live(void *handle) {
	return [(__bridge DFWAppHooks *)handle live];
}

// undoes what dfw_apply_app wired to the hooks' window, before the binding
// destroys it, and releases the hooks: the quit Apple Event no longer reaches
// the window and ends the process as AppKit's would, the menu bar dfw
// replaced is back, pending focus work does nothing, a chooser still open is
// answered, and the web view loses dfw's UI delegate while it still exists.
static void dfw_detach_app(void *handle) {
	@autoreleasepool {
		DFWAppHooks *hooks = (__bridge_transfer DFWAppHooks *)handle;
		[hooks setLive:NO];
		// the handler is process-wide; it is cleared only while it still
		// answers for this window.
		if ([dfw_quit_handler window] == [hooks window]) {
			[dfw_quit_handler setWindow:nil];
		}
		NSMenuItem *quit = [hooks quitItem];
		[quit setTarget:nil];
		[quit setAction:nil];
		// the menu bar is put back only while it is still the one dfw installed.
		if ([NSApp mainMenu] == [hooks mainMenu]) {
			[NSApp setMainMenu:[hooks previousMainMenu]];
			[NSApp setWindowsMenu:[hooks previousWindowsMenu]];
		}
		WKWebView *web = [hooks webView];
		if (web != nil) {
			DFWUIDelegate *delegate = objc_getAssociatedObject(web, &dfw_ui_delegate_key);
			[delegate cancelOpenPanel];
			[web setUIDelegate:nil];
			objc_setAssociatedObject(web, &dfw_ui_delegate_key, nil, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
		}
	}
}

// the window's delegate, armed with a pending chooser as WebKit's request
// leaves it, and the times WebKit's completion ran, for the tests.
static DFWUIDelegate *dfw_test_delegate;
static int dfw_test_open_panel_answers;

static void dfw_arm_test_open_panel(void *window) {
	WKWebView *web = dfw_find_web_view([(__bridge NSWindow *)window contentView]);
	dfw_test_delegate = objc_getAssociatedObject(web, &dfw_ui_delegate_key);
	dfw_test_open_panel_answers = 0;
	[dfw_test_delegate setOpenPanelCompletion:^(NSArray<NSURL *> *URLs) {
		dfw_test_open_panel_answers++;
	}];
}

// asks the armed delegate for a second chooser while the first is pending, and
// reports how many times that second request was answered before returning.
static int dfw_second_test_open_panel(void) {
	__block int answered = 0;
	// the early answer reads none of these; variables keep clang's nonnull
	// warning for a literal nil out of the build.
	WKWebView *noView = nil;
	WKOpenPanelParameters *noParameters = nil;
	WKFrameInfo *noFrame = nil;
	[dfw_test_delegate webView:noView runOpenPanelWithParameters:noParameters initiatedByFrame:noFrame completionHandler:^(NSArray<NSURL *> *URLs) {
		answered++;
	}];
	return answered;
}

// asks a fresh delegate for a chooser from a web view with no window, as a
// late request after the window closed would, and reports how many times it
// was answered before returning.
static int dfw_windowless_test_open_panel(void) {
	__block int answered = 0;
	DFWUIDelegate *delegate = [[DFWUIDelegate alloc] init];
	WKWebView *web = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 100, 100)
		configuration:[[WKWebViewConfiguration alloc] init]];
	WKOpenPanelParameters *noParameters = nil;
	WKFrameInfo *noFrame = nil;
	[delegate webView:web runOpenPanelWithParameters:noParameters initiatedByFrame:noFrame completionHandler:^(NSArray<NSURL *> *URLs) {
		answered++;
	}];
	return answered;
}

// finishes the armed chooser once more, as its sheet's result arriving after a
// detach would, and reports how many times WebKit's completion ran in all.
static int dfw_finish_test_open_panel(void) {
	[dfw_test_delegate finishOpenPanel:nil];
	dfw_test_delegate = nil;
	return dfw_test_open_panel_answers;
}

// a window that is never shown, titled only and holding a web view as
// webview_go's is, for the tests to apply the menu and the delegate to.
static void *dfw_new_hidden_window(void) {
	[NSApplication sharedApplication];
	NSWindow *w = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 100, 100)
		styleMask:NSWindowStyleMaskTitled
		backing:NSBackingStoreBuffered
		defer:YES];
	[w setReleasedWhenClosed:NO];
	[w setContentView:[[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 100, 100)
		configuration:[[WKWebViewConfiguration alloc] init]]];
	return (__bridge_retained void *)w;
}

// the window's buttons, for the tests.
static char *dfw_describe_window_style(void *window) {
	NSWindowStyleMask mask = [(__bridge NSWindow *)window styleMask];
	NSString *s = [NSString stringWithFormat:@"closable=%d miniaturizable=%d",
		(mask & NSWindowStyleMaskClosable) != 0, (mask & NSWindowStyleMaskMiniaturizable) != 0];
	return strdup([s UTF8String]);
}

// records the window being asked to close, and keeps it open.
@interface DFWCloseRecorder : NSObject <NSWindowDelegate>
@property int asked;
@end

@implementation DFWCloseRecorder
- (BOOL)windowShouldClose:(NSWindow *)window {
	[self setAsked:[self asked] + 1];
	return NO;
}
@end

// sends this process the quit Apple Event, as the Dock's Quit does, and
// counts the times the window is asked to close, for the tests.
static int dfw_quit_event_close_requests(void *window) {
	NSWindow *w = (__bridge NSWindow *)window;
	DFWCloseRecorder *recorder = [[DFWCloseRecorder alloc] init];
	id<NSWindowDelegate> previous = [w delegate];
	[w setDelegate:recorder];
	NSAppleEventDescriptor *event = [NSAppleEventDescriptor appleEventWithEventClass:kCoreEventClass
		eventID:kAEQuitApplication
		targetDescriptor:[NSAppleEventDescriptor currentProcessDescriptor]
		returnID:kAutoGenerateReturnID
		transactionID:kAnyTransactionID];
	AppleEvent reply = {typeNull, NULL};
	// the reference constant goes unused, but may not be null.
	[[NSAppleEventManager sharedAppleEventManager] dispatchRawAppleEvent:[event aeDesc] withRawReply:&reply handlerRefCon:(__bridge void *)w];
	AEDisposeDesc(&reply);
	[w setDelegate:previous];
	return [recorder asked];
}

// describes the main menu as "Menu: Title=mods+key/action>target; ...|...",
// for the tests.
static char *dfw_describe_main_menu(void) {
	NSMutableArray *menus = [NSMutableArray array];
	for (NSMenuItem *top in [[NSApp mainMenu] itemArray]) {
		NSMenu *menu = [top submenu];
		NSMutableArray *items = [NSMutableArray array];
		for (NSMenuItem *item in [menu itemArray]) {
			if ([item isSeparatorItem]) {
				continue;
			}
			NSEventModifierFlags m = [item keyEquivalentModifierMask];
			NSString *mods = [NSString stringWithFormat:@"%@%@%@%@",
				(m & NSEventModifierFlagControl) ? @"ctrl+" : @"",
				(m & NSEventModifierFlagOption) ? @"opt+" : @"",
				(m & NSEventModifierFlagShift) ? @"shift+" : @"",
				(m & NSEventModifierFlagCommand) ? @"cmd+" : @""];
			NSString *target = [item target] != nil ? NSStringFromClass([[item target] class]) : @"responder";
			[items addObject:[NSString stringWithFormat:@"%@=%@%@/%@>%@", [item title], mods, [item keyEquivalent],
				NSStringFromSelector([item action]), target]];
		}
		[menus addObject:[NSString stringWithFormat:@"%@: %@", [menu title], [items componentsJoinedByString:@"; "]]];
	}
	return strdup([[menus componentsJoinedByString:@"|"] UTF8String]);
}
*/
import "C"

import "unsafe"

// prepareNativeApp runs before the webview exists, on the UI thread.
func prepareNativeApp() {
	C.dfw_prepare_app()
}

// releaseNativeApp runs on the UI thread once the binding has destroyed the
// window, or when no webview could be made: it winds down what
// prepareNativeApp and newNativeApp set up for the whole process.
func releaseNativeApp() {
	C.dfw_release_app()
}

// darwinApp owns the DFWAppHooks that dfw_apply_app returned for one window.
// its methods run on the UI thread.
type darwinApp struct {
	hooks unsafe.Pointer
}

// newNativeApp applies the macOS setup to window once the webview and its
// window exist, on the UI thread, and returns the controller that owns it.
func newNativeApp(window unsafe.Pointer) nativeAppController {
	if window == nil {
		return nil
	}
	return &darwinApp{hooks: C.dfw_apply_app(window)}
}

func (a *darwinApp) Activate() {
	if a.hooks != nil {
		C.dfw_activate_app(a.hooks)
	}
}

func (a *darwinApp) EndActivation() {
	if a.hooks != nil {
		C.dfw_end_activation(a.hooks)
	}
}

// Close detaches the setup and releases its hooks. it needs no window handle,
// which webview_go has cleared by then after an ordinary close. later calls,
// Activate included, do nothing.
func (a *darwinApp) Close() {
	if a.hooks == nil {
		return
	}
	C.dfw_detach_app(a.hooks)
	a.hooks = nil
}

// newNativeHiddenWindow, the describeNative helpers, the test chooser helpers
// and nativeQuitEventCloseRequests serve the tests, which cannot use cgo
// themselves.
func newNativeHiddenWindow() unsafe.Pointer {
	return C.dfw_new_hidden_window()
}

func describeNativeUIDelegate(window unsafe.Pointer) string {
	description := C.dfw_describe_ui_delegate(window)
	defer C.free(unsafe.Pointer(description))
	return C.GoString(description)
}

func describeNativeMainMenu() string {
	description := C.dfw_describe_main_menu()
	defer C.free(unsafe.Pointer(description))
	return C.GoString(description)
}

func describeNativeWindowStyle(window unsafe.Pointer) string {
	description := C.dfw_describe_window_style(window)
	defer C.free(unsafe.Pointer(description))
	return C.GoString(description)
}

func nativeQuitHandlerHasWindow() bool {
	return C.dfw_quit_handler_has_window() != 0
}

func nativeActivationPolicy() int {
	return int(C.dfw_activation_policy())
}

func installNativeTestMenu() {
	C.dfw_install_test_menu()
}

func nativeMainMenuID() uintptr {
	return uintptr(C.dfw_main_menu_id())
}

func setNativeActivationPolicy(policy int) {
	C.dfw_set_activation_policy(C.int(policy))
}

// NSApplicationActivationPolicyRegular and ...Accessory, for the tests.
var (
	nativeRegularPolicy   = int(C.dfw_policy_regular)
	nativeAccessoryPolicy = int(C.dfw_policy_accessory)
)

func nativeQuitHandlerFallsBack() bool {
	return C.dfw_quit_handler_falls_back() != 0
}

func nativeAppLive(app nativeAppController) bool {
	a, ok := app.(*darwinApp)
	return ok && a.hooks != nil && C.dfw_hooks_live(a.hooks) != 0
}

func armNativeTestOpenPanel(window unsafe.Pointer) {
	C.dfw_arm_test_open_panel(window)
}

func secondNativeTestOpenPanel() int {
	return int(C.dfw_second_test_open_panel())
}

func windowlessNativeTestOpenPanel() int {
	return int(C.dfw_windowless_test_open_panel())
}

func finishNativeTestOpenPanel() int {
	return int(C.dfw_finish_test_open_panel())
}

func nativeQuitEventCloseRequests(window unsafe.Pointer) int {
	return int(C.dfw_quit_event_close_requests(window))
}
