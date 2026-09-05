# Backup polling browser regression

From the repository root, after `npm ci` at the root and in `frontend-modern`:

```sh
pulse-heavy-run -- node scripts/check-backup-browser-polling.mjs
pulse-heavy-run -- env PULSE_BROWSER_WIDTH=390 node scripts/check-backup-browser-polling.mjs
```

Requires Playwright Chromium (`npx playwright install chromium`). The runner
starts and closes a loopback-only Vite server on port 5198 and a headless browser;
it neither needs nor contacts a live Pulse deployment. API responses are synthetic.

The fixture mounts the production backup table, router and CSS in a native
scroll container, with 40 workloads replaced by HTTP polling once per second.
It expands a mid-table workload and focuses its toggle, then verifies three
changed snapshot names render while focus, restore evidence, scroll offset and
the coverage route survive. Non-zero initial scroll prevents a vacuous pass.

This covers #1869's refresh stability at the coverage-table boundary. It does
**not** qualify the full application resource provider/WebSocket path, PBS drawer,
By date view, virtualised large tables, Settings editing, or Brave. A passing
result is not grounds to close #1869 or claim the reported top-of-page jump fixed.
