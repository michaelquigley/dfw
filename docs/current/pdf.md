# PDF export

`webview.App.PDF` and `webview.WindowApp.PDF` optionally bind a `*webview.PDFExporter` to one window. Linux provides a native save chooser and a headless system-Chromium renderer. The visible window still uses WebKitGTK. This is a Go capability, not a JavaScript bridge, an HTTP route, a markdown renderer, or a file-writing service.

## ownership

Create the handle with `webview.NewPDFExporter(webview.PDFOptions{Executable: ""})` before calling the window entry point. Construction neither starts a browser nor checks that one is installed. Retain the handle in the product's own request handlers and pass it as `PDF` to exactly one `Run` or `Window`. A handle cannot be rebound; rejecting a second use does not cancel its original window.

`Supported()` reports the platform adapter, not readiness or the presence of Chromium. `Choose` and `Render` return promptly before the loop is ready and after the window has ended. They are blocking Go calls for background goroutines, not for native UI callbacks. Only one operation is admitted at a time; another receives `PDFBusy`. The product owns any larger lock spanning choose, render, and publication. Nil opts out without changing existing window behavior. Outside Linux, support is false and operations return `PDFUnsupported`; setting the optional field does not itself prevent the window from opening.

`WindowContext()` is stable for the handle's entire lifetime. An accepted close or internal termination cancels it synchronously before native termination or graceful server shutdown, even when no chooser or render is running. Startup failure and native-loop exit cancel it too. A pending or declined close request does not. A product must bind its complete export job to both its request context and this window context, and check both directly before beginning publication; returning PDF bytes does not end the window's responsibility to cancel later product work. Once publication has begun, its outcome is the product's to report honestly.

## choosing and rendering

`Choose(ctx, PDFDestinationRequest{SuggestedName: "report.pdf", InitialDirectory: "/absolute/directory"})` returns `PDFDestination{Path: ...}` or `PDFDestination{Cancelled: true}`. An empty initial directory lets GTK choose its default. The suggested name must be a basename. The transient modal chooser provides PDF filtering and its ordinary overwrite confirmation. It accepts the exact filename the writer chose: removing or changing `.pdf` is honored, with no suffix appended later and no extra confirmation dialog. Context cancellation is an error identifiable by `errors.Is`; a user pressing the chooser's cancel button is a normal cancelled choice. Pending native cleanup may briefly keep the operation slot busy after a caller cancels.

`Render(ctx, PDFDocument{URL: documentURL, Resources: resourceURLs})` returns a complete PDF byte slice. The caller supplies already-rendered HTML, CSS, fonts, and images, chooses a destination, protects any source files, and publishes the returned bytes. Rendering never writes the selected path, invokes a print dialog, or alters the main webview. This split lets a product validate a chosen destination before starting the browser.

URLs must be absolute HTTP URLs using `127.0.0.1` or `::1` and an explicit port, without userinfo or a fragment. Every resource shares the document's origin. The document and resource list form an exact allowlist, not access to every route on that origin. The product must retain immutable resources until rendering returns and protect their delivery with its own local-server policy. File URLs, remote pages, downloads, extra document targets, redirects, failed resources, and requests outside the manifest fail the render. Authored scripts are disabled; dfw's fixed font/image readiness code runs in an isolated world. SVG completeness and other content validation remain product responsibilities.

Serve a complete standalone document with an explicit page style. Use an empty favicon (`<link rel="icon" href="data:,">`) to prevent Chromium's automatic `/favicon.ico` request from falling outside the manifest. CSS `@page` controls layout; fallback paper is Letter portrait, background printing is enabled, and browser-generated headers and footers are disabled. All declared font faces must load successfully and every image must decode before printing. This is not a screenshot or a fixed wait after navigation. CSS-generated and other document-specific completeness requirements still belong to the caller.

## system browser and limits

Empty `PDFOptions.Executable` searches `chromium`, `chromium-browser`, then `/snap/bin/chromium`. An explicit executable path/name is authoritative; it is not shell syntax and has no fallback after failure. Chromium 131 or newer is required for CSS page-margin boxes. No browser is bundled, downloaded, updated, made the default, or attached to the user's browsing session. A missing browser affects export only.

Each render starts a separate headless process with a fresh mode-0700 `dfw-pdf-*` directory under the user's home. This non-hidden location works with Snap confinement; the host's `/tmp` is not assumed to be the browser's `/tmp`. The sandbox stays enabled. Extensions and component background extensions are disabled, as are first-run prompts and background networking. Control uses private inherited pipes, not a debugging TCP port. PDF bytes return through that pipe, so Chromium does not need filesystem access to the chosen destination.

The render deadline is 60 seconds across launch, navigation, readiness, and printing, excluding time in the chooser. Incoming protocol messages are limited to 8 MiB and PDF output to 128 MiB, read in 256 KiB chunks. Browser stderr is bounded to 16 KiB and is not exposed in returned errors because it may contain credential-bearing URLs. Protocol errors likewise do not repeat browser-supplied text. `*PDFError` carries a `PDFErrorCode`; context cancellation/deadlines remain recognizable through `errors.Is`.

Every normal result path closes streams/pipes, stops and reaps the owned browser, and removes its profile. Shutdown allows two seconds for normal exit, then terminates the process group and escalates to kill after another two seconds. Native teardown cancels outstanding work and disposes the chooser before destroying the parent. A hard kill of the application can leave a private profile behind; dfw does not sweep directories that may belong to another running export.

## verification

Ordinary Go tests cover controller lifetime, cancellation, queued callbacks, stale completion, single-window binding, pipe framing, foreign CDP keys, malformed/oversized input, process failure, shutdown escalation, and cleanup with fake native boundaries and real child-process fixtures. They need no display, Chromium installation, or poppler tools.

The opt-in `make test-pdf` gate exercises the installed system Chromium and checks PDF text, embedded fonts, links, page counters, table headers, delayed resources, blocked requests, redirect failures, invalid fonts/images, authored-script suppression, and cancellation. Set `DFW_PDF_TEST_FONT` to a readable TrueType file; optionally set `DFW_PDF_EXECUTABLE`. Missing prerequisites fail this gate explicitly. See [building](building.md#pdf-checks).

The automated native renderer checks passed with Chromium `153.0.8010.36 snap` on Linux. Michael exercised the [native fixture](../../examples/dfw-example-pdf/README.md), including the chooser and slow-export checks, and reported it working on 2026-09-25. Keep using disposable output paths for that acceptance sequence.
