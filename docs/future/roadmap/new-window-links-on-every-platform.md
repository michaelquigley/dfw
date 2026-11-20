---
title: new window links on every platform
state: inbox
created: 2026-11-20
tags: [enhancement]
source: github:michaelquigley/dfw#1
log:
  - stamp: 2026-11-20
    note: macOS policy landed with PR #1; see docs/journal/2026-11-20.md
---

Give `target="_blank"` links and `window.open` one contract on every supported platform, the one macOS has: an `http`, `https` or `mailto` address opens in the user's default handler, any other scheme is dropped, and no second web view is ever created. Today only macOS does this (`DFWUIDelegate`'s `createWebViewWithConfiguration:` in `webview/app_darwin.go`). On Linux, webview_go connects no `create` handler, so WebKitGTK drops the link; on Windows, nothing handles WebView2's `NewWindowRequested`, so it opens its own unmanaged popup window. Answer `create` on the GTK web view and `NewWindowRequested` on WebView2 (mark it handled, then launch the URI), and decide the scheme allowlist once, in Go, tested on every platform, with the darwin delegate using the same rule. `docs/current/runtime.md` then moves new-window links out of "macOS windows" into a section of their own.
