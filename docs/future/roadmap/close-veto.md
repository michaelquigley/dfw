---
title: close veto
state: researching
created: 2026-09-16
tags: [feature]
milestone: v0.1.x
---

Let an application answer the window manager's close request instead of being told about it afterwards. Today `webview.Run` returns when the window closes, `SaveWindowState` runs, and the process exits; there is no point at which the application can say "not yet" or "not at all". `webview.App` carries the identifier, title, size, icon, zoom flag, and listen function, and nothing else observes the window's lifetime.

The need comes from documents. A product whose window owns unsaved work cannot ask the writer whether to save, discard, or stay, because the close has already happened by the time any of its code runs. gloss is the immediate case: closing its window ends the process that owns the piece and the editing session with it. Its unsaved work survives through a recovery envelope offered at the next open, which is a floor rather than an answer; the writer is never asked, and keystrokes newer than the last acknowledgment are outside it.

## Shape

The decision usually lives in the web content, not in Go, so a plain synchronous predicate is not enough on its own. Three pieces, roughly:

- A close-request callback on `webview.App` and `webview.WindowApp`, called before the window is destroyed.
- An answer that may be "close", "keep open", or "not yet", so a product can ask its own page over its own channel and decide afterwards.
- A way to close the window programmatically once the product is satisfied. `Terminate` exists on the internal window type and is already used when the supervised server stops; a deferred answer needs something equivalent in the application's hands.

## Considerations

- Linux blocks a close by returning true from the GTK `delete-event` handler. The underlying webview library does not expose that today, so this is native work of the kind `window_bounds_linux.go` and `zoom_linux.go` already do. Other platforms can follow the precedent of position tracking and zoom: the callback simply never fires where it is unsupported, and the window closes as it does now.
- A window that cannot be closed is worse than one that closes too easily. A veto needs an escape: a second close request that forces, a timeout on an unanswered ask, or both. An application that hangs must not strand its own window.
- `SaveWindowState` runs after the blocking `Run()` returns, so a vetoed close must leave that path untouched and save only on the close that actually happens.
- A product that owns its server can already close its own window by shutting the server down, which the supervisor turns into a terminate. That covers an in-application quit. It does not cover the window manager's own close button, which is the whole of this card.

## Why

Any product that holds unsaved state in a window needs to be asked before it loses it. Without this, every dfw application either saves whatever it has without being told to, or accepts that the close button is a hard exit and builds a recovery path behind it.
