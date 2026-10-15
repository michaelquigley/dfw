---
title: windows fullscreen
state: inbox
created: 2026-10-15
tags: [enhancement]
log:
  - stamp: 2026-10-15
    note: fullscreen landed on Linux only; see docs/journal/2026-10-15.md
---

Bring `EnableFullscreen`, `Fullscreen`, and the F11 toggle to Windows, which has them as a documented no-op since they landed on Linux. Extend the existing window procedure that tracks bounds rather than subclassing again: F11 swaps the window between its normal style and a borderless monitor-sized one, and the state follows `WM_SIZE` rather than the last request. `SaveWindowState` already keeps the windowed bounds while fullscreen, so the Windows controller reports its state through the same `nativeFullscreenController`, and `fullscreen_unsupported.go` narrows to the platforms with neither.
