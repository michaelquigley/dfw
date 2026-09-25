# native PDF fixture

A small product-owned HTTP UI exercising dfw's PDF capability. Linux only; install system Chromium 131+ first. No browser is bundled. Run from the dfw repository:

```sh
go run ./examples/dfw-example-pdf
```

Use only disposable output files. The fixture demonstrates native interaction and an atomic output replacement, not a real product's protected-source or expected-file policy. It has no markdown editor or sidecar. Its default `dfw-proof.pdf` is a suggestion: the chooser's accepted filename is used unchanged.

## acceptance checks

1. Click **save as PDF** and cancel the chooser. The page reports cancelled and creates no output. Repeat and accept a path: the PDF has selectable text, a clickable link, a brown image, repeating table headings, all rows, a final paragraph, and page counters.
2. Export to an existing disposable file. The chooser asks before replacing it; declining preserves the old contents. Accepting replaces it only after rendering completes.
3. With `report.pdf` already present, enter `report` instead. The output is the extensionless `report`, not `report.pdf`; the existing PDF stays untouched. If `report` exists too, the native overwrite prompt covers that file. A deliberately different suffix is likewise preserved.
4. Click **try broken image**. After choosing a disposable existing output, the missing image fails rendering and the output stays unchanged.
5. Run with `--delay 5s`. After accepting a destination, click **cancel export** while resources are loading. The request stops, the prior output survives, and the next export works. If cancellation races publication, the page deliberately tells you to check the destination.
6. With the slow fixture, close the window while the chooser is open and again while rendering. The process must exit rather than wait for the resource delay, with no remaining owned Chromium process or `dfw-pdf-*` profile. The modal chooser may require using its own cancel before the window manager permits closing the parent; verify the desktop's actual behavior.
7. Run with `--chromium /not/a/browser`. The window still opens and the chooser still works; rendering reports an unavailable executable without replacing an existing output.

The close-veto callback is not configured here. A PDF-enabled window still closes through dfw's termination path so the chooser is disposed before the parent is destroyed; the separate close-request fixture covers save/discard-style veto behavior.

## automated output gate

No visible chooser is driven automatically. The headless Chromium tests run separately:

```sh
DFW_PDF_TEST_FONT=/path/to/a/font.ttf make test-pdf
```

The gate requires `pdfinfo`, `pdffonts`, and `pdftotext` (typically the `poppler-utils` package), as well as Chromium. Its test outputs use temporary directories and are cleaned up. The normal `make test` gate does not require those tools or a display.

## status

The headless renderer, its policy failures, and cancellation have been exercised with system Chromium `153.0.8010.36 snap`. Michael ran the native fixture and slow-export acceptance checks and reported them working on 2026-09-25. This is the human native-window check, separate from the automated controller tests.
