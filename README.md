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

Once `examples/dfw-example-watch` exists, a repository-wide `go build ./...`
will require its React bundle first:

```sh
cd examples/dfw-example-watch/web
pnpm install
pnpm build
```

See `examples/dfw-example-watch/README.md` for the full example build sequence
after that example lands.
