# 320px Docker and navigation qualification — 5 September 2026

Base: `6cd22b2e039db08f2c3e2ce715912db1f31ecd48` (main-derived).
Test/evidence change only; no application behaviour or demand ledger change.

## Judgment

The supplied checkout already includes the 390px phone-label correction
`537c7ae9a5`; the older shared assessment's clipped-label conclusion is stale.
Fresh W3C reflow guidance includes 320 CSS pixels and distinguishes data-table
exceptions from individual cell content:
https://www.w3.org/WAI/WCAG22/Understanding/reflow.html

A bounded fresh community search also found independent preference for Pulse's
monitoring simplicity, not evidence demanding another mobile web surface:
https://www.reddit.com/r/Proxmox/comments/1lblkk8/anyone_else_switch_to_pulse_from_netdata_or_any/
That older self-report is not current adoption or release acceptance evidence.
The useful action here is qualification, not new product scope.

## Actual evidence

- Locked frontend and integration dependencies installed after an initial
  browser attempt failed before tests because frontend dependencies were absent.
- Focused Vitest: App.architecture and DockerNativeTables, 64 tests passed.
- Source-built local backend and Chromium, 320x844, synthetic inventory:
  failed-admission reconnect case passed; normal-admission case reached the
  label checks and failed the inherited single-line Current assertion (two
  lines). The preceding all-rendered-update-cell horizontal text-range bounds
  check passed. This is not an eight-case pass.
- Viewed `wrapped-labels.png`: Current and Update wrap within their cells;
  small status/metric/identity columns remain truncated. No blanket layout or
  accessibility pass. This screenshot contains synthetic inventory only.
- Managed backend reported stopped after this run.
- Adjusted the test to retain the single-line assertion at 390px only; at 320px
  wrapping is permitted but the text-bound checks remain. Extended mobile
  navigation interactions and heading checks to 320px. This final test revision
  has NOT completed a browser run: the eight-case rerun remained queued behind
  other heavy work for over 13 minutes and was cancelled before starting.
- `git diff --check` passed. No full repository suite, installed artifact,
  physical Android, screen reader, 400% browser zoom, or asynchronous update
  state qualification. Heading proof at 320px remains pending because the
  initial run stopped before that assertion.

## Reproduce next

```sh
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 PULSE_E2E_TABLE_ACCESS=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 npm --prefix tests/integration test -- tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

Expect eight selected cases (normal/failed admission at 320, 390, 1100, 1440).
Do not replace browser-verification.json with a pass until actually completed.
Awkward 320px word wrapping remains a presentation limitation, not newly fixed
behaviour. Check current ledger scope before any product layout follow-up.

## Canonical integration follow-up

The coordinating maintainer ran the command above against merged canonical
commit `5be32dbaa7` on 5 September 2026 at 18:16 UTC. All eight Chromium cases
passed in 1.8 minutes, including normal and failed admission at 320px. The
managed local backend stopped cleanly. Visual inspection of the generated
320px `table-full-detail-heading.png` confirmed that the expanded heading was
horizontally present; the awkward narrow-table wrapping and truncation noted
above remain. This focused run is not a blanket accessibility, installed
artifact, or release-readiness result, and `browser-verification.json` remains
unchanged.
