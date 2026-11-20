---
title: macos window position
state: inbox
created: 2026-11-20
tags: [enhancement]
log:
  - stamp: 2026-11-20
    note: macOS windows came into use for mega; see docs/journal/2026-11-20.md
---

Persist a window's size and location on macOS as Linux and Windows do. `window_bounds_unsupported.go` still covers darwin, so a macOS window opens at `InitialSize`, or a size from a state file it did not write, wherever AppKit puts it, every time. Track the `NSWindow` frame through `NSWindowDidMoveNotification` and `NSWindowDidResizeNotification` into the existing `nativeWindowBoundsTracker`, convert AppKit's bottom-left origin to the top-left one `core.WindowState` stores, and apply a saved location only when it still falls on a connected screen. The README's platform table and `building.md` say "not persisted" until then, and `runtime.md`'s macOS Quit bullet says position is not saved and points at this card.
