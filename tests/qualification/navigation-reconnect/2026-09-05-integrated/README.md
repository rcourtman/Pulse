# Integrated navigation recovery check — 5 September 2026

## Result

At source `c8931787adb7bc3152f060a829ebe2a1aa2a2b9c`, all eight existing
`96-navigation-socket-recovery.spec.ts` Chromium cases passed (132 seconds,
starting 20:43:55 UTC). No application or test changes were made for this run.
The backend was built locally with embedded production frontend assets and
synthetic inventory; it was stopped by the runner afterwards.

Local executable SHA-256:
`a5aaf32fdba70744d1dfa4e0749f080dd8960dcd19d381ab6ec2fdc286d36b27`.
This identifies this source-built test executable, not a published artifact.
Locked dependencies were installed in root, frontend-modern and tests/integration;
the runner used Playwright 1.56.1 and the Chromium project.

Widths 1440 and 1100 (height 900), and 390 and 320 (height 844), each passed
with and without injected admission HTTP failure. The checks exercise socket
closure with 1013, retained navigation/inventory, recovery without document
reload, and incident-control access at desktop/mobile widths. Narrow cases
also exercise platform switching and More navigation. Docker table keyboard
access and clipped-text assertions require the separate `PULSE_E2E_TABLE_ACCESS=1`
opt-in. The retained JUnit records eight successful cases but does not record
that flag, so it cannot independently establish those conditional assertions
ran. Admission-failure cases require an observed failed request.

`junit.xml` is the unmodified runner result. Two inspected screenshots retain
representative desktop incident access during reconnect and 320px inventory.
The narrow screenshot alone is not proof of connection state; the test asserts
that state separately. Small update cells still wrap heavily at 320px, despite
the earlier reported no-clipped-text result. That conditional result is not
independently established by this retained JUnit. No readability redesign is implied.

Additional focused checks: 53 tests passed across websocket-resilience and
websocket-unified; `TestAlertCharacterizationGetActiveAlertsExportsCanonicalIdentity`
passed with `-count=1` in internal/alerts. No full repository suite was run.

## Repeat

From repository root after installing locked dependencies:

```sh
pulse-heavy-run -- env \
  PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 \
  PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 \
  PULSE_E2E_TABLE_ACCESS=1 \
  PULSE_E2E_LOCAL_BACKEND_PORT=18765 \
  npm --prefix tests/integration test -- \
  tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

## Decision and limits

Fresh retrieval of the [community comparison thread](https://www.reddit.com/r/Proxmox/comments/1lblkk8/anyone_else_switch_to_pulse_from_netdata_or_any/)
on 5 September reinforced protecting quick-glance, low-overhead monitoring.
This is historical, anecdotal feedback, not a new independent demand count or
benchmark. The latest integrated source warranted checking the existing repairs
together rather than creating another surface or repeating a withdrawn repair.

This result narrows the integration regression uncertainty. It does **not**
qualify acknowledgement persistence, off-host delivery, restart history,
malformed alert snapshots, a reporter installation, or a release candidate.
The REST recovery code still skips entries without usable IDs. The server's
normal read model exports typed alerts and canonical identity has focused test
coverage, but that is not proof every malformed-response path is impossible.
Do not count that previously withdrawn repair as delivered. Revisit it with
browser fault injection, genuine empty-response clearing, and the required
same-content performance/contracts qualification if pursued.

Next useful evidence is exact-candidate recovery and installed alert receipt,
not another equivalent source-only navigation run without changed inputs.
No release readiness or backport eligibility judgment is made here.

## Receipt audit — 5 September 2026

The repeat command above now explicitly enables table-access assertions. This
is a documentation correction, not a fresh browser run or a claim that the
original run omitted the flag. Retain the invocation flags and the
`narrow-table-access` attachment on the next qualification run.

The recovery assertions in this spec establish reconnection, retained navigation
and unchanged document identity. They do not compare a changed incident or
resource value before and after the interruption. The managed-runtime recovery
spec likewise checks connection status and HTTP health, not changed incident
content. A future data-recovery qualification should change an identified
incident during disconnection and verify its rendered state after reconnect,
including a genuine empty active-alert response. Existing store-level snapshot
tests are narrower evidence; no missing-data browser regression is claimed here.
