---
title: macos javascript dialogs
state: inbox
created: 2026-11-20
tags: [defect]
source: github:michaelquigley/dfw#1
---

`alert`, `confirm` and `prompt` do nothing in a macOS window: WKWebView shows them only through its UI delegate, and `DFWUIDelegate` in `webview/app_darwin.go` answers neither `runJavaScriptAlertPanelWithMessage:`, `runJavaScriptConfirmPanelWithMessage:` nor `runJavaScriptTextInputPanelWithPrompt:`. WebKitGTK and WebView2 show their own dialogs by default. Implement the three as sheets on the window, calling each completion handler exactly once (including when the window closes with a sheet open), and add a case to `examples/dfw-example-macos`'s page.
