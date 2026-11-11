---
title: tray quit hook and live icon
state: inbox
created: 2026-11-11
tags: [feature]
---

Let a tray daemon take part in its own Quit and change its tray presence while it runs: a hook that can veto or redirect Quit, and calls to replace the icon and to enable, disable or relabel tray items after start.

## why

`tray.Daemon` adds an unconditional Quit and sets the icon and items once. That suits ranger, which holds no work in flight. metawoo mega runs coding agents as child processes: a stray Quit interrupts every one of them, where the product has a "shut down when idle" gate it would rather send the operator through. And the reason for mega to sit in the tray at all is to show, from the icon, that something needs the operator or is still working, which a fixed icon cannot do.

## shape

- An optional `OnQuit func() bool` on `DaemonApp`: called when Quit is chosen; returning false keeps the daemon running, so the application can open its own confirmation or start a graceful shutdown and stop the tray itself when ready. Absent, Quit behaves as today. The close-veto work for webview windows is the precedent.
- A handle returned to the application, or passed to a start callback, with `SetIcon(png []byte)`, `SetTooltip(string)`, and per-item `SetLabel`, `Enable` and `Disable`, safe to call from any goroutine. The underlying systray library already supports all of them.
- A way for the application to end the tray itself, for a shutdown that began somewhere other than the menu.

## background

Raised 2026-11-11 while planning `mega daemon` (`products/mega/docs/future/roadmap/dfw-daemon-mode.md`).

## decided for the first consumer (2026-11-11)

Michael chose to land this before building `mega daemon`. Mega will use the Quit hook to send a Quit on a busy workspace through its "shut down when idle" gate, the live icon to show when a conversation needs the operator, and the stop call to end the tray after a shutdown accepted from its page. It needs a tagged release to depend on.

