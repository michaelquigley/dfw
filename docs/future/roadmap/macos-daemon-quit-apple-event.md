---
title: macos daemon quit apple event
state: inbox
created: 2026-11-20
tags: [defect]
source: github:michaelquigley/dfw#1
log:
  - stamp: 2026-11-20
    note: window side fixed in PR #1, daemon side left open; see docs/journal/2026-11-20.md
---

A tray daemon on macOS that receives the quit Apple Event (a logout or restart, `quit app` from a script) ends inside AppKit's `terminate:`. systray's `applicationWillTerminate:` runs `onExit` and then the process exits, so `Daemon` never returns: `OnQuit` is not consulted, the server is not shut down, and `daemon.json` is left behind. Answer the event in the tray the way the window process does (`dfw_install_quit_handler` in `webview/app_darwin.go`) and route it into `runTray`'s `quit` wrapper, so the daemon ends through the same path as `Quit`. Decide whether `OnQuit` may decline a logout, since declining there cancels the logout. Remember that systray's own `Quit` stops the loop with `[NSApp stop:]` and never runs `onExit` on macOS, so cleanup belongs after `systray.Run` returns.
