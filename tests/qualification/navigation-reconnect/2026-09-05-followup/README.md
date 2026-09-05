# Browser follow-up — 5 September 2026

Qualified supplied main-based `0c3846d8bf56e41e03436ebad785665405597143`
with the test-only changes accompanying this receipt. No product code changed.
Chromium 141.0.7390.37; synthetic inventory and local authentication only.

## Results

- Admission race harness: six passes, failure/success/reverse at 1440×900 and
  390×844. Delayed outgoing organisation admission cannot restore destinations
  after switching to Acme, whether Acme returns 503, an empty admission facet,
  or completes after the outgoing response. Returning to Default restores
  navigation without a document reload. Narrow More/Settings and Docker
  platform selection work. JSON receipts retain the runtime source hash.
- Added a subsequent synthetic `websocket_reconnected` event assertion: the
  production subscriber requests the current organisation and navigation remains
  available. This is subscriber/request evidence, not an actual socket recovery.
- Separate real-backend socket qualification: all six checks passed (1.4m),
  widths 1440, 1100 and 390, each with and without admission HTTP 503 on
  reconnect. Real connected sockets are interrupted with 1013, HTTP remains
  available, navigation survives, mobile menus and incident controls remain
  accessible, and transport recovers without reload. The runner built embedded
  assets and the Go backend, then stopped its owned backend; PID absence checked.
- Focused `useAppRuntimeState.test.ts`: 28 tests passed. Diff check passed.

## Visual evidence

Both attached recovered screenshots were viewed at native 390×844. These contain
only the local backend's synthetic estate. Bottom navigation remains accessible.
Docker names are truncated and rightmost container update badges are visibly
cut off at the table edge. This remains a narrow-layout investigation item,
not a visual-polish pass. No assertion about horizontal scroll reachability,
full names via assistive technology or other platforms follows from these images.
The admission harness has empty inventory and cannot qualify table layout.

## Reproduction

Install locked dependencies in frontend-modern and tests/integration, with the
pinned Chromium available. From repository root:

```sh
pulse-heavy-run -- bash -c 'set -e; for width in 390 1440; do for mode in failure success reverse; do PULSE_PROOF_WIDTH=$width PULSE_PROOF_MODE=$mode node scripts/check-navigation-admission-race.mjs; done; done'
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 npm --prefix tests/integration test -- tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

## Judgment and limits

Fresh external read: https://community-scripts.org/scripts/pulse?id=pulse
on September 5 lists installed v6.4.1 and exposes upstream preview recovery and
mobile accessibility claims. This is distribution context, not independent
confirmation of those claims or demand for a new surface. Combined with the
unfinished admission frontier it favours verification over new UI scope.

No published image, release candidate, reporter installation, alert dispatch,
long outage soak or full shell/Playwright Test/container interruption was
qualified. Docker is available here; full-stack interruption is unfinished work,
not an established environment blocker. A third organisation switch that
supersedes an already incoming request is not covered by this two-organisation
matrix. Next useful work: diagnose narrow-table reachability and qualify that
additional race or full-stack cleanup separately. No new product bet or demand
ledger edit; no release-readiness upgrade.
