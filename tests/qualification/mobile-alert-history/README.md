# Mobile alert history browser qualification

From the repository root, after `npm ci` and `npm --prefix frontend-modern ci`:

```sh
pulse-heavy-run -- node tests/qualification/mobile-alert-history/run.mjs
```

Requires Playwright's Chromium and WebKit browsers and their system libraries.
The runner starts a loopback-only Vite server on an ephemeral port and closes
it and both browsers on completion. No backend, credentials or live data are used.

The fixture renders the production mobile history list and dialog with synthetic
history and incident events. At 390×844 it checks both Timeline and Resource
investigations, Tab containment, Escape dismissal, focus return to the surviving
trigger or list when the row disappears, and page overflow. A computed-style
assertion guards against accidentally testing without production Tailwind CSS.

This is component-browser evidence, not whole-application, physical-device,
screen-reader, persistence or notification-delivery qualification. Fixture
callbacks do not implement note saving, filtering or network operations.
The fallback case should fail if `return action ?? list` in
`AlertHistoryMobileList` is changed to `return action ?? null`.
