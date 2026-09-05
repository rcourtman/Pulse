# Changed incident recovery — 5 September 2026

## Purpose and evidence boundary

The previous integrated run only established unchanged navigation and socket
recovery. The admission continuation explicitly asked for changed-data and empty
active-response evidence. Fresh reading of the [community comparison thread](https://www.reddit.com/r/Proxmox/comments/1lblkk8/anyone_else_switch_to_pulse_from_netdata_or_any/)
on 5 September still supports protecting the quick-glance overview, including
phone use; one user separates this from historical monitoring and alerting in
CheckMK. These are historical self-reports, not new demand counts or current
performance measurements. No new product surface or ledger bet is introduced.

The test uses an owned, source-built core backend with synthetic inventory and
production embedded frontend assets. It never fulfils active-alert REST with a
fixture, injects a store event, or replaces the application's WebSocket store.
Real sockets close with 1013; browser active-alert requests are aborted while
blocked, preventing periodic REST polling from hiding missing recovery. A
separate authenticated API client changes an identified incident on the backend and
verifies the changed server value while the rendered value remains old.

After reconnect, the browser must receive a successful active-alert response,
render the same incident as acknowledged, and retain document time origin.
A second interruption pauses detection and bulk-clears backend incidents; the
backend and browser must both return `[]`, and the old incident cards must go.
Original alert configuration is restored in `finally`. This is deliberate
administrative clearing, **not** a claim that lost telemetry or disabling
alerts represents monitored-resource recovery. No notification delivery is exercised.

## Results and test development

Final full matrix: **10 passed, no retries**, 2.4 minutes starting
2026-09-05T21:41:31Z. Chromium 141.0.7390.37. Eight navigation cases cover
1440/1100/390/320px, each with/without admission HTTP failure; table-access opt-in
ran at 390/320px. Two additional cases cover changed acknowledgement and genuine
empty-response clearing at 1440/320px (height 900). A preceding cold-runtime
focused invocation passed both changed-data cases in 30 seconds.

`final/` retains unmodified JUnit, extracted report timings, both
`narrow-table-access` JSON attachments, environment attachments, actual identified
selection/acknowledged/empty response receipts, and four screenshots. `focused/`
retains the preceding two-case pass. Inspected desktop acknowledged and phone
cleared screenshots from the focused run show the changed service card and
zero active/acknowledged counts with an **Alerting is paused** state respectively.
This is intentionally not an “all infrastructure healthy” claim.

Earlier test-development failures are retained rather than counted as product
regressions:

| Receipt | Local draft | Result and diagnosis |
| --- | --- | --- |
| `initial/` | `d82f55cb23` | Eight navigation cases passed; two new cases received acknowledged REST but failed to find the card. Initial grouping hypothesis was incomplete. |
| `standalone/` | `2a3dcc3378` | Eight passed; standalone host also moved outside the DOM window after acknowledgement. Second case inherited the acknowledged fixture. Source inspection established acknowledged-last sorting and list windowing. |
| `cold-fixture/` | `e934385ddc` | Two filtered cases failed before mutation: offline-host incident had not appeared in the fresh runtime. |
| `named-service/` | `75aa1b6b0a` | Two filtered cases failed before mutation: service names differ between generated estates. |

The final test selects a current `docker-service-health` incident by canonical
ID, resets acknowledgement if needed and scrolls the windowed list to its last
group after acknowledgement. No UI hook or artificial renderer setting is used.
Test-development commits were consolidated locally into `1abaea81c3`; no remote
history changed. Passing runs executed draft `0cccef0dd4`, whose exact test blob
`685a028a71c8e3a13a881c4ae66d7988bfa08ec9` is unchanged in the consolidated commit.
Application source was unchanged throughout.

In response attachments, `target` is the selection-time snapshot, before the
optional reset. It is already acknowledged in the final 320px case because the
previous case's acknowledgement can survive fixture regeneration. The test then
uses the real unacknowledge endpoint and requires its rendered Acknowledge button
before disconnecting and acknowledging again. Do not read `target.acknowledged`
as the post-reset baseline. The retained `acknowledgedSnapshot` and
`emptySnapshot` are actual browser recovery responses.

## Invocation

Locked dependencies installed with `npm ci --no-audit --no-fund` in repository
root, frontend-modern and tests/integration. The two full development attempts and final full matrix used exactly:

```sh
pulse-heavy-run -- env \
  PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 \
  PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 \
  PULSE_E2E_TABLE_ACCESS=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 \
  npm --prefix tests/integration test -- \
  tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

The three filtered development attempts and successful focused run appended
`--grep 'changed backend'` to that command. All builds/browser runs were
serialized through `pulse-heavy-run`; the final rerun reused the same binary.

Application source: `adeecd4079185cf9358415e2b7a55fc371f408b7` (the supplied
integrated main snapshot, not the newer remote head in the admission packet).
Only tests differ at the tested `0cccef0dd4` and consolidated `1abaea81c3`. Local executable SHA-256:
`891fc3827d1e6522f21011ef62052b73f29f6407f498131e7e0cbcf454f7c677`.
Built with Go 1.26.8 on Linux amd64, embedded Vite production assets and locked
Playwright 1.56.1. This binary has no embedded VCS build fields; the source
snapshot and content hash, not the displayed development version, bind this run.

## Limits and next trigger

No application repair, released-candidate qualification, reporter retest,
acknowledgement persistence across process restart, history durability, installed
alert receipt or off-host delivery is claimed. REST and stream are both restored,
so this does not establish which path wins their race. Malformed snapshots are
not covered. Global detection is paused only inside the owned test runtime.

Prefer changed integration/candidate or installed evidence next, not another
unchanged synthetic matrix. Release judgment remains independently held on
publication continuity and candidate issue disposition; these checks do not
clear those boundaries. No timed revisit is justified by elapsed time alone.

## Mechanical admission repair

The first admission rejected eight trailing spaces inside failed-run JUnit
CDATA. Failed-run `junit.xml` files in `initial/`, `standalone/`,
`cold-fixture/` and `named-service/` now have trailing line whitespace removed.
Their original bytes are preserved in adjacent deterministic `junit.xml.gz`
archives, with original and normalised SHA-256 values in
`receipt-normalisation.json`. Decompression was checked byte-for-byte. The
passing `focused/` and `final/` JUnit files remain unmodified. No test result,
application source or test code changed; browser checks were not rerun for this
receipt-only repair. Evidence commits were consolidated locally so intermediate
commits no longer introduce the rejected whitespace.
