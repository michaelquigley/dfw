# dfw-example-tray

`dfw-example-tray` is the manual fixture for a tray daemon that takes part in its own lifetime: `DaemonApp.OnQuit` and `tray.Control`. It is deliberately small: one Go file, no page, and it imports only `dfw/tray`, so it builds without the webview's native dependency.

The daemon has a busy state, toggled from its menu. Busy shows in the tray icon (red rather than dark), in the tooltip, and in a disabled status item at the top of the menu. While busy, `Quit` is declined through `OnQuit` and the status item counts the declines; while idle, `Quit` quits. Two routes end the daemon without going through `Quit`: the `Shut down in 3 s` item, which disables itself and counts down in its own label before calling `Control.Stop`, and `POST /api/stop`, the shape of a shutdown accepted from a product's page. `GET /api/status` reports the busy state and the decline count.

The listen address is logged at startup and written to `daemon.json` under the `com.quigley.dfw.example.tray` config directory.

## Build and run

```sh
cd examples/dfw-example-tray
go build .
./dfw-example-tray
```

## Sequence

1. Start the example. The tray shows the dark icon; the menu reads `Idle`, `Start work`, `Shut down in 3 s`, `Quit`.
2. Choose `Start work`. The icon turns red, the tooltip ends in `busy`, the status item reads `Busy`, and the toggle reads `Finish work`.
3. Choose `Quit` twice. The daemon keeps running, the status item reads `Busy: Quit declined (2)`, and `GET /api/status` reports two declines.
4. Choose `Finish work`, then `Quit`. The icon and labels return to idle, and `Quit` ends the daemon; `daemon.json` is removed.
5. Start it again, choose `Start work`, and `curl -X POST http://<address>/api/stop`. The daemon ends even though it is busy: `Control.Stop` does not consult `OnQuit`.
6. Start it again and choose `Shut down in 3 s`. The item greys out and counts down, then the daemon ends.

Every run exits 0 and removes `daemon.json`. Steps 1–6 were run on Linux (GNOME on Wayland) on 2026-11-11 by sending dbusmenu `clicked` events to the tray over D-Bus with `gdbus`, not by hand; Windows has not been exercised.
