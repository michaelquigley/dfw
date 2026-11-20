# dfw-example-macos

`dfw-example-macos` is the manual fixture for a tray daemon and its window on macOS, run from an application bundle the way a product ships one. The daemon lives in the menu bar, with `LSUIElement` set, and `Open Window` starts the window process from the same binary through `tray.SpawnSelf("window")`. Its page holds what a window has to get right on macOS:

- **the keyboard**: the window takes the focus when it opens, and the Edit menu's keys work.
- **links that ask for a new window**: `target="_blank"` and `window.open`.
- **the file chooser**: files and folders.
- **reopening**: opening the bundle again while the daemon runs opens a window, through `DaemonApp.OnReopen`.

The page reports what it sees, a key pressed with a modifier, a copy or a paste, a link click, a chooser's result, to the daemon's log. The behavior under test is in [Runtime › macOS windows](../../docs/current/runtime.md#macos-windows).

`bundle.sh` builds the binary and packs it into `dfw-example-macos.app` beside itself. The binary is the bundle's executable, and the bundle is signed ad hoc. Both outputs are ignored by git.

## Build and run

```sh
cd examples/dfw-example-macos
sh bundle.sh
open --stdout "$PWD/fixture.log" --stderr "$PWD/fixture.log" dfw-example-macos.app
```

The daemon's address is logged at startup and written to `daemon.json` under `~/Library/Application Support/com.quigley.dfw.example.macos/runtime/`.

## Sequence

1. Start the bundle. The menu bar shows the icon (a teal square on dark) with no text beside it; there is no Dock icon. The menu reads `Open Window`, `Quit`.
2. Click a Finder window, so another application has the focus. Choose `Open Window`. The window comes to the front with a Dock icon and the menu bar, and the log reads `page: 'loaded'`. Press Cmd+K at once, without clicking: the log shows the key, so the keyboard is in the page, not in Finder. No menu binds Cmd+K, so only the page can have logged it.
3. Click the first box and press Cmd+A and Cmd+C. Click the second, then press Cmd+V and Cmd+Z. The text is copied, pasted and undone; the log shows the copy and the paste.
4. Click the `target="_blank"` link, then the `window.open` button. Each opens `example.com` in the default browser; the window stays on its page.
5. Use each file chooser. A sheet opens on the window: files for the first, folders for the second. Choosing returns the count to the page and to the log.
6. Press Cmd+W. The window closes, and the Dock icon goes with it; the menu bar icon stays. Open the window again and press Cmd+Q: the same. Open it once more and choose Quit from its Dock icon's menu: the same.
7. With no window open, open the bundle again: double-click it, or `open dfw-example-macos.app`. No second daemon starts; `OnReopen` opens a window, in front.
8. Choose `Quit` in the menu bar. The daemon ends and `daemon.json` is removed.

Steps 1 to 6 were run by hand on macOS 26.6 (arm64, Apple silicon) on 2026-11-20, with the menu bar on an external display; step 2 used Cmd+A then, before the check moved to a chord no menu binds; in step 5 the sheets opened, and no choice was made. Step 2's focus was also checked without a click, by starting `Contents/MacOS/dfw-example-macos window` with `DFW_DAEMON_ADDR` set while Finder was frontmost, and reading the frontmost application with `lsappinfo front`. Step 7 was run the same day with `open`, Finder frontmost: one daemon, a window process, and the window frontmost. Step 8 was not run against this fixture; the same Quit ended the spike it came from. Step 6's Dock Quit was not run by hand: the quit Apple Event the Dock sends was sent to a bare window binary with `NSRunningApplication terminate`, on 2026-11-20, and `Window` returned.
