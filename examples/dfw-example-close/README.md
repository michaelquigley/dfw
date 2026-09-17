# dfw-example-close

`dfw-example-close` is the manual fixture for the native close request lifecycle. It is deliberately small: one Go file, an inline page, and no frontend toolchain. It exists to exercise `OnCloseRequest` against a real window. Linux is supported now; Windows follows in the next stage of the close-veto work, and until then a configured callback fails window setup there rather than opening an unprotected window.

The page polls `/api/status` and shows whether a close request is `idle` or `pending` and how many requests have been delivered. While a request is pending it shows two buttons, `Keep open` and `Close`, whose POST handlers resolve the request. Nothing resolves automatically and there is no timeout.

This is application-owned transport on purpose. The example carries the request over its own loopback HTTP API; `dfw` adds no transport, page bridge, or save/discard semantics.

## Build and run

```sh
cd examples/dfw-example-close
go build .
./dfw-example-close
```

## Sequence

1. Start the example and use the native title-bar close control. The window remains open, the page shows one `pending` request, and the delivered count becomes one.
2. Use the native close control several more times while pending. The count remains one.
3. Choose `Keep open`. The page returns to `idle` and the window remains.
4. Use the native close control again. A second `pending` request appears and the delivered count becomes two.
5. Choose `Close`. The entry point returns through the ordinary save-and-destroy path and the process exits.

The delivered count proves repeated native closes are dropped rather than queued; the two buttons prove rearming and accepted close through the real native window.
