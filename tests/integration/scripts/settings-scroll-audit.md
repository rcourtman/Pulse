# Settings scroll audit helper

The Settings overflow audit must scroll and measure `.app-scroll-shell`,
not the document. The root is outside the app's independently scrolling
`h-screen` layout; mobile WebKit also does not support Playwright mouse wheels.

The helper keeps the bounded incremental traversal and bottom tolerance used
by the audit. Horizontal width includes the shell's scrollable width, so content
overflow inside the shell cannot pass merely because the document fits.

Run from the repository root:

```sh
pulse-heavy-run -- node --test tests/integration/scripts/settings-scroll-audit.test.mjs
```

This uses synthetic markup matching the app's scroll ownership in Chromium and
WebKit at 320, 390 and 430 pixels. It proves the window-scroll negative control,
the final setting's visibility, and detection of deliberately oversized content.
It does not establish a real-backend Settings pass or touch-gesture behaviour.
