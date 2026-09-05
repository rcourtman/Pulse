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

## Application refresh boundary

```sh
pulse-heavy-run -- node scripts/check-backup-app-refresh.mjs
pulse-heavy-run -- env PULSE_BROWSER_WIDTH=390 node scripts/check-backup-app-refresh.mjs
```

This separate runner loads the actual application entry point, authentication
bootstrap, router, scoped/paginated resource queries, WebSocket store and PBS
resource drawer. Authentication and all API/socket data are synthetic; no live
Pulse server or credentials are used. The browser blocks off-origin HTTP traffic.
It supplies 240 workloads and a hybrid PBS resource with a metrics-history target.
Three replacement `rawData` frames per view change rendered names, rather than
merely waiting on a timer with unchanged content.

Assertions cover:
- Coverage: fewer rendered rows than the inventory (windowing enabled), non-zero
  scroll, expanded restore evidence, focused expansion button and route retained.
- By date: changed workload name, non-zero scroll, focused navigation link and route
  retained. This view has no per-artifact expansion toggle.
- PBS: real History panel remains visible and selected, with its tab focused and
  route/scroll retained while the PBS name changes. Its baseline scroll is zero;
  this checks drawer state, not a non-zero drawer-scroll regression.
- No uncaught page errors or error-level browser console messages.

Printed samples record scroll offsets and active controls. The fixed 500ms waits
allow initial layout/scroll synchronisation; each refresh waits for changed text.
The backup artifact HTTP payload itself is static after initial loading: this is
resource-provider/socket replacement qualification, not backup API refetch proof.
Narrow-width interaction is keyboard-driven, not touch or Brave qualification.
The test does not cover Settings Manage editing, Authentication strategy selection,
expansion padding, arbitrary inventory churn/reordering, or a deployed release.
Passing is not grounds to close the mixed report #1869.
