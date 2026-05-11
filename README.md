# dfw

`dfw` is a small Go library for wrapping HTTP servers with embedded web UIs
into native desktop applications. It is an add-on to
[`github.com/michaelquigley/df`](https://github.com/michaelquigley/df) and uses
the `dd` package for runtime data binding.

The v1 API is intentionally narrow:

- `dfw.Run` starts a single process containing the HTTP server and one webview
  window.
- `dfw.Daemon` starts a tray-resident HTTP daemon.
- `dfw.Window` starts a separate window process that connects to a daemon.

Products own their HTTP API, web UI, background work, and distribution. `dfw`
does not provide a UI framework, JavaScript-to-Go bridge, request proxy, native
file dialogs, native menu bar API, single-instance enforcement, or packaging
tooling.

## Status

Initial implementation. MVP platform support is Windows and Linux. macOS is
deferred.

The design lives in [`docs/future/initial-spec.md`](docs/future/initial-spec.md)
and the staged work order lives in
[`docs/future/initial-work-order.md`](docs/future/initial-work-order.md).

## Building

For the library itself:

```sh
go build ./...
go test ./...
```

Stage 2 and later use a WebKitGTK 4.1-compatible Go webview binding, so desktop
builds need the native webview toolchain:

- Linux: GTK 3 plus WebKitGTK 4.1 development files. On Fedora this is
  `gtk3-devel webkit2gtk4.1-devel`; on Debian/Ubuntu this is typically
  `libgtk-3-dev libwebkit2gtk-4.1-dev`. Tray support uses the DBus
  StatusNotifier/AppIndicator protocol, so the desktop environment needs a
  compatible tray host.
- Windows: WebView2 runtime, usually already present on Windows 10/11, plus a
  CGO-capable Windows toolchain.

The `examples/dfw-example-watch` binary embeds its React bundle, so a
repository-wide `go build ./...` requires that bundle first:

```sh
cd examples/dfw-example-watch/web
pnpm install
pnpm build
```

See `examples/dfw-example-watch/README.md` for the full example build sequence.
