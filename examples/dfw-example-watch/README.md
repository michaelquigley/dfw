# dfw-example-watch

`dfw-example-watch` is a single-binary example for all three `dfw` entry
points:

- `run` starts a watcher, HTTP server, and window in one process.
- `daemon` starts a tray-resident watcher and HTTP server.
- `window` opens a separate window connected to a daemon.

## Build

Build the React bundle first. The Go binary embeds `web/dist`, and that
directory is intentionally gitignored.

```sh
cd examples/dfw-example-watch/web
pnpm install
pnpm build
```

The package metadata explicitly allows `esbuild`'s install script for pnpm.
Vite uses esbuild's native binary, so pnpm's build-script approval mode must
not block that dependency.

The npm equivalent is:

```sh
cd examples/dfw-example-watch/web
npm install
npm run build
```

Then build the Go example from the example directory:

```sh
cd examples/dfw-example-watch
go build .
```

From the repository root, `go build ./...` also works after the React bundle
exists.

## Run

Single-window mode:

```sh
./dfw-example-watch run /tmp/dfw-example-watch
```

Daemon mode:

```sh
./dfw-example-watch daemon /tmp/dfw-example-watch
```

Use the tray menu's `Open Window` item to spawn a window process, or open one
directly while the daemon is running:

```sh
./dfw-example-watch window
```

`--devtools` is available on `run` and `window`:

```sh
./dfw-example-watch run --devtools /tmp/dfw-example-watch
./dfw-example-watch window --devtools
```

## Windows Subsystem

This example is a single binary with subcommands, so the Windows subsystem is a
binary-level build choice:

```sh
go build -ldflags "-H windowsgui" .
```

Use that form for shipping builds when you do not want console windows for
`run` or `window`. For development, plain `go build .` keeps daemon diagnostics
attached to the console, but `run` and `window` may briefly show a console
window.
