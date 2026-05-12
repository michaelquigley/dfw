# Building

`dfw` is a CGO-using library. The Go side is unremarkable — `go build` and
`go test` work as normal — but the webview binding pulls in a native
toolchain on each platform.

## Library

The library itself compiles with:

```sh
go build ./...
go test ./...
```

No additional setup beyond the platform prerequisites below.

## Linux

The webview is WebKitGTK 4.1 inside a GTK 3 window. The tray uses the
DBus StatusNotifier / AppIndicator protocol, so the desktop session needs
a tray host that speaks one of those (GNOME Shell with the AppIndicator
extension, KDE Plasma's stock tray, XFCE's `xfce4-statusnotifier-plugin`,
etc. — most modern Linux desktops include one).

Fedora:

```sh
sudo dnf install gtk3-devel webkit2gtk4.1-devel
```

Debian / Ubuntu:

```sh
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

A CGO-capable toolchain (typically `gcc` and `pkg-config`) is also
required and is usually already present.

## Windows

The webview is WebView2 (Edge Chromium), which is installed by default on
Windows 10 and 11. The Go side needs:

- A CGO-capable toolchain. The standard recommendation is `mingw-w64`
  (`x86_64-w64-mingw32-gcc`) installed via MSYS2 or chocolatey.
- The WebView2 SDK headers come bundled with the webview binding — no
  separate install needed.

For a release build that does not open a console window alongside the
webview, link with the GUI subsystem:

```sh
go build -ldflags "-H windowsgui" .
```

This is a binary-level choice. For a single binary with subcommands (as in
the example app), all subcommands share the same subsystem — `daemon` and
`window` get the GUI subsystem too, which is usually what you want.

## macOS

Deferred for v1. The library compiles on macOS (the `_unsupported` build
tags fill in noop implementations for window-bounds tracking, icon
application, and tray icon conversion), but window position is not
persisted and the webview integration has not been exercised. Treat macOS
as not yet supported until that changes.

## Building the Example

`examples/dfw-example-watch` is a single binary that embeds a React
bundle via `//go:embed`. The bundle directory (`web/dist`) is
intentionally gitignored, so a fresh checkout needs to build the bundle
before `go build ./...` will succeed at the repo root.

```sh
cd examples/dfw-example-watch/web
pnpm install
pnpm build
```

The npm equivalent works too:

```sh
cd examples/dfw-example-watch/web
npm install
npm run build
```

Then build the Go binary from the example directory or from the repo
root:

```sh
cd examples/dfw-example-watch
go build .
```

### pnpm and `esbuild`

The example's `package.json` explicitly allows `esbuild`'s install script
for pnpm. Vite uses esbuild's native binary at runtime, and pnpm's
build-script approval mode blocks unknown install scripts by default — so
that allowlist entry must not be removed.

## Related

- [architecture.md](architecture.md) — what the library does and why it
  needs a webview.
- [example.md](example.md) — the example app's full build sequence and
  layout.
