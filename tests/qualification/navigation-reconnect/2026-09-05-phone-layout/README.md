# Docker phone update-cell correction — 5 September 2026

Mainline base: `5beeaf4cbe32603bf4d8dcb07069957a659aa1a4`.

## Decision and scope

The preceding access receipt demonstrates clipped update badges at 390×844,
with neither wheel nor keyboard horizontal input recovering them. Viewed its
retained screenshot again. Fresh external reading of W3C's Reflow guidance
(https://www.w3.org/WAI/WCAG22/Understanding/reflow.html, September 5) confirms
that a table exception does not justify losing cell information. This is not
a claim of WCAG conformance or new operator demand.

Read the current pulse-pro FEATURE_REQUESTS.md from the supplied team's
20260905T170527Z-pro-customer checkout. The shipped resizable-columns entry
explicitly keeps phones responsive; it does not authorise extending desktop
manual scrolling to phones. No new surface or named product bet is introduced.

The existing Docker update cell now reduces phone padding and wraps badge/action
content rather than clipping it. Labels, accessible names, handlers and review
requirements are unchanged. Detail headings wrap long identities instead of
ellipsising them (shared drawer, not only Docker). No overflow-container or
touch-handler change; the Android page-owned scrolling rule remains intact.

## Validation

- Focused Vitest: DockerNativeTables and ResourceDetailDrawer.docker-container:
  38 tests passed.
- Initial real-backend Chromium run: 390px reconnect and new text-bounds checks
  passed. Screenshot inspection nevertheless found avoidable mid-word breaks;
  cell padding was then reduced and a single-line Current-label assertion added.
- Final run: all six Chromium cases passed (390, 1100, 1440px; normal and
  failed admission). Managed backend PID 2415019 stopped afterwards. Browser assertions inspect text ranges in all
  rendered Docker update cells, assert clip/non-scrollable wrapper geometry,
  preserve Enter expansion, and verify a non-ellipsised, fitting detail heading.
  The populated synthetic fixture must include a visible Current status.

Reproduce from repository root (locked packages and pinned Chromium installed):

```sh
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 PULSE_E2E_TABLE_ACCESS=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 npm --prefix tests/integration test -- tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

No full repository suite, physical Android touch, screen-reader, installed
release or every asynchronous action-state qualification. Full names remain
available by expanding the existing row; overview names remain truncated.
Review and integration are still required; nothing was published or deployed.

Final screenshots were viewed: Current and Update fit without mid-word breaks
at 390px, and notifications-worker wraps fully in the expanded heading.
Screenshots contain only synthetic local inventory. The final run rebuilt the
embedded frontend and core backend through pulse-heavy-run. Wider cases protect
navigation recovery; phone text-bound assertions run in the 390px normal-admission
case. The fixture does not guarantee every update error/progress state.
