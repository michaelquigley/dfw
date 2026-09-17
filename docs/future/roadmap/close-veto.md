---
title: close veto
state: evaluating
created: 2026-09-16
tags: [feature]
milestone: v0.1.x
log:
  - stamp: 2026-09-16
    note: spec drawn; realized 2026-09-17 and synthesized into docs/current/runtime.md#close-requests
  - stamp: 2026-09-17
    note: landed and exercised on Linux and Windows; evaluating for product adoption and soak
---

Let an application answer the window manager's close request instead of being told about it afterwards. Today `webview.Run` returns when the window closes, `SaveWindowState` runs, and the process exits; there is no point at which the application can say "not yet" or "not at all". `webview.App` carries the identifier, title, size, icon, zoom flag, and listen function, and nothing else observes the window's lifetime.

The need comes from documents. A product whose window owns unsaved work cannot ask the writer whether to save, discard, or stay, because the close has already happened by the time any of its code runs. gloss is the immediate case: closing its window ends the process that owns the piece and the editing session with it. Its unsaved work survives through a recovery envelope offered at the next open, which is a floor rather than an answer; the writer is never asked, and keystrokes newer than the last acknowledgment are outside it.

## Shape

The decision usually lives in the web content, not in Go, so a plain synchronous predicate is not enough on its own. Three pieces, roughly:

- A close-request callback on `webview.App` and `webview.WindowApp`, dispatched asynchronously after the native close has been suppressed.
- A one-shot request with `Close` and `KeepOpen` resolutions. Leaving it unresolved is "not yet", so a product can ask its own page over its own channel and decide afterwards.
- One pending request at a time. Native close requests arriving while it remains pending are dropped rather than queued or treated as stronger consent.

## Considerations

- Linux blocks a close by returning true from the GTK `delete-event` handler. Windows consumes `WM_CLOSE` through the window-procedure integration already used for bounds tracking. Configuring the callback on an unsupported platform, or failing to install the native interceptor, fails window setup rather than silently falling back to a hard close.
- A pending request has no timeout, and repeated native close requests do not force it. It remains pending until the product resolves it; operating-system process controls are the hard escape from a stuck application.
- `SaveWindowState` runs after the blocking `Run()` returns, so a vetoed close must leave that path untouched and save only on the close that actually happens.
- A product that owns its server can already close its own window by shutting the server down, which the supervisor turns into a terminate. That covers an in-application quit. It does not cover the window manager's own close button, which is the whole of this card.

## Why

Any product that holds unsaved state in a window needs to be asked before it loses it. Without this, every dfw application either saves whatever it has without being told to, or accepts that the close button is a hard exit and builds a recovery path behind it.
