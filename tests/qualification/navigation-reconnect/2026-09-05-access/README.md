# Access and interruption qualification — 5 September 2026

Base: `76fd854da1b42402ad742258297ff31461f1bb39`, plus accompanying test-only
changes. Chromium 141.0.7390.37; local synthetic inventory. No product change.

## Access diagnosis

Fresh external reference: https://www.w3.org/WAI/WCAG22/Understanding/reflow.html
(accessed September 5). W3C permits two-dimensional data-table scrolling but
not indiscriminate loss of information. This challenges treating a clipped
screenshot as either an automatic failure or a mobile pass. It supports access
diagnosis, not demand for a new surface or a conformance assertion.

At 390×844, Docker's wrapper measures 362px client/scroll width, uses
`overflow-x: clip`, and remains at scrollLeft 0 after horizontal wheel input
and ArrowRight from an existing detail-toggle button. The last update cell is
48.44px wide; its badge is visibly clipped in both viewed fresh wheel screenshots.
This is not off-screen content recoverable by horizontal scrolling. Source
`frontend-modern/src/index.css` applies `overflow: clip` to phone platform
wrappers below 40rem. Do not simply restore nested scrolling: that rule addresses
Android vertical gesture ownership, which needs regression protection.

The first-row accessibility snapshot retains the full update action despite
badge clipping. A second run selects truncated `notifications-worker`: its
accessibility snapshot retains the full name, Enter sets expanded=true, and
rendered detail text includes the full hostname. The viewed detail screenshot
still truncates the drawer heading. This proves accessible naming and keyboard
expansion, not screen-reader/device testing or sighted access to every long value;
scrolling to the identity field was not exercised. Pointer input here means
desktop Chromium horizontal wheel, not Android touch.

Text receipts are human-readable reporter extracts, not JSON. Cell geometry
samples the first container row even in the long-name run; accessibleRow and
detailText describe the selected long-name row. The opt-in diagnostic does not
assert that clipping is acceptable.

## Real full-stack interruption

`run-tests.sh multi-tenant` source-built mock and e2e_runtime Docker images and
started isolated Compose. The watcher waited for an owned cookie file and live
Chromium descendants, recorded project container inventory, then sent SIGTERM
to the supervisor. `interruption.json`: exit 143; server/mock running and seed
exited before the signal; no owned containers, volumes, sampled descendant PIDs
or auth/report/result directories afterwards. No cookie contents retained.

An initial watcher did not recognise headless_shell; that run completed normally
(six passes, one expected skip), so it was not interruption proof. The corrected
run supplies the receipt. The reproduction watcher also explicitly fails if
criteria are unmet. SIGINT/SIGHUP have existing stub coverage only. SIGKILL/host
failure, every possible new descendant and preservation of unrelated stacks
were not qualified. No shared runtime was interrupted or deployed.

## Reproduction and checks

From repository root, install locked frontend-modern and tests/integration
packages and pinned Chromium:

```sh
pulse-heavy-run -- python3 tests/qualification/navigation-reconnect/2026-09-05-access/interrupt.py
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 PULSE_E2E_TABLE_ACCESS=1 PULSE_E2E_LOCAL_BACKEND_PORT=18765 npm --prefix tests/integration test -- tests/96-navigation-socket-recovery.spec.ts --project=chromium --grep '390px.*false'
pulse-heavy-run -- bash -c 'set -e; for width in 390 1440; do for mode in failure success reverse superseded; do PULSE_PROOF_WIDTH=$width PULSE_PROOF_MODE=$mode node scripts/check-navigation-admission-race.mjs; done; done'
node --test tests/integration/scripts/run-tests-interruption.test.mjs tests/integration/scripts/run-tests-cleanup.test.mjs
```

Both narrow backend runs passed and stopped; both recorded backend PIDs were
absent afterwards. Twelve focused shell/cleanup tests passed. No full repository
suite. Admission results are retained separately. Multiple-switch toast
locators initially detached; the harness now waits for normal expiry instead
of clicking a changing list. This is a harness correction, not a product fix.

## Next maintenance

Preserve supersession and real interruption coverage. Investigate fitting phone
update actions into allocated cells while retaining accessible naming, keyboard
access and Android page-owned vertical scrolling. Check demand-ledger scope
before product work; this pass introduced no bet or user-visible surface.
Installed alert delivery, release acceptance and physical-device accessibility
remain separate dependencies. Full-stack SIGTERM proof is no longer missing;
no Docker blocker is established.
