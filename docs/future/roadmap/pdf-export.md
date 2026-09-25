---
title: pdf export
state: building
created: 2026-09-24
tags: [feature]
---

add an optional Go-facing PDF capability to both window entry points: a native save chooser and system-Chromium rendering of caller-prepared HTML into PDF bytes. keep markdown, HTTP routes, and final file publication in the consuming product. Linux first, Windows later; keep the browser sandbox enabled and use a private profile and control pipe, not the user's browsing session or a debugging port.

## background

gloss is the first consumer. the shared [spec](../../../../gloss/docs/future/print-to-pdf.md) and [work order](../../../../gloss/docs/future/print-to-pdf-work-order.md) live in the sibling gloss repository; do not create a second copy here. the Chromium proof and why native WebKit printing was insufficient are recorded in gloss's [journal](../../../../gloss/docs/journal/2026-09-24.md).
