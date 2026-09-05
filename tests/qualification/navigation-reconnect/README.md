# Navigation during cold-stream reconnect (#1899)

## Reproduction and scope — 5 September 2026

Source-built v6.4.1 (`db7e26deac2a77dd5eff1dfb5bf2f1546683d5d6`) and
integrated main (`193ead50fd9559b92ee242f34b8e9718fd7a3e3e`) both lost all six
platform destinations after a real connected browser websocket was closed with
1013 during startup. HTTP health remained available, Docker inventory remained
visible and System controls survived. Socket recovery alone did not restore the
destinations in either baseline run. This reproduces a path consistent with
https://github.com/rcourtman/Pulse/issues/1899, not the reporter's LXC installation.

Chromium 141.0.7390.37, fresh contexts, 100% zoom, 1440×900 and 1100×900;
real isolated Go backend, embedded production frontend, synthetic Proxmox and
Docker inventory. Below xl the desktop navigation is intentionally hidden even
before interruption; its destinations disappeared from the DOM in the failing
baseline. This is not a new responsive-navigation requirement.

Alert REST recovery can populate `activeAlerts` before the first resource frame.
The previous runtime-resolution predicate treated that as authoritative resource
state, replacing the admission facet with an empty estate. The repair tracks
receipt of a resource snapshot independently: preserve this knowledge during
transport reconnect, clear it on organisation URL changes, and continue treating
an explicit empty resource snapshot as authoritative. No navigation redesign.

## Repeat locally

Install locked dependencies in the repository root, `frontend-modern` and
`tests/integration`, then run from the repository root:

```sh
pulse-heavy-run -- env \
  PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 \
  PULSE_MOCK_MODE=true PULSE_E2E_NAVIGATION_RECOVERY=1 \
  PULSE_E2E_LOCAL_BACKEND_PORT=18765 \
  npm --prefix tests/integration test -- \
  tests/96-navigation-socket-recovery.spec.ts --project=chromium
```

Install the pinned Playwright browser first if absent. The opt-in runner builds,
starts and stops an isolated backend; it does not use a shared application.
Screenshots and browser/viewport metadata are attached to the Playwright report.
Counts are excluded from navigation equality because alerts can change normally.

The repaired main run passes both widths, including desktop navigation into
active incidents and an enabled acknowledgement control while disconnected;
recovery requires no page reload. This is access/control availability evidence,
not acknowledgement persistence or notification delivery qualification.
102 focused runtime, websocket and architecture tests pass. Frontend typecheck
remains blocked by two errors in untouched `TrueNASAlertsTable.test.tsx`
(lines 83 and 119: `healthy` is not a `ResourceStatus`).

## Remaining boundaries

No published artifact, candidate branch, reporter browser, reverse proxy,
admission-HTTP-failure injection, mobile menu interaction or long outage soak
was qualified here. A release-line backport needs reproduction and repair
verification on that exact candidate, followed by its restarted soak. Test the
admission-failure path separately rather than assuming this repairs it. This
opt-in spec does not become an automatically executed release gate by existing.

## Bounded preflight repair

Matching alert, cloud-paid, performance and unified-resource contracts now
record the snapshot/admission boundary, with an additional architecture guard.
Fresh repair validation: 87 focused tests pass; three real-backend Chromium
checks pass at 1440, 1100 and 390×900. Desktop and narrow reconnect screenshots
were inspected. At 390px the Docker inventory and bottom navigation remain
visible; this does not exercise the More menu or mobile platform switching.
The matching browser receipt records the exact runtime source hashes.

## Admission HTTP failure and mobile interaction — 5 September 2026

The expanded experiment on integrated main
`deac5e7750a79052170ba4396b700a3d6b855e3f` found a second failure:
after a real socket reconnect, returning HTTP 503 specifically for
`/api/resources?page=1&limit=1` removed the platform destinations at 1440
and 1100px. The populated inventory was not sufficient to establish canonical
resource-snapshot receipt. System destinations and incident controls remained.
This is a reproduced source-build path consistent with #1899, not confirmation
of the reporter's installation or a newly qualified v6.4.1 artifact.

Baseline: four checks passed, two failed. All three no-HTTP-failure checks
passed; the 390px failure-injection check also passed. Do not describe this as
an unconditional defect at every width: resource snapshot arrival affects it.
The new focused retention test independently failed before the repair.

The repair retains the last valid admission facet on request failure. No new
fetches, retries or navigation surface. Successful responses still replace it,
including an all-false facet. First-load failures remain unresolved, and an
organisation switch still clears outgoing admission before the new request;
focused tests cover that request failing as well.

The same opt-in command above now runs six checks: all three widths with and
without admission HTTP failure. At 390px it opens and dismisses More, follows
Settings from More, switches to Proxmox and Docker, and opens active incidents
with an enabled acknowledgement control before, during and after interruption.
Failure injection must observe a completed 503 response, not merely install a
route handler. The test checks document identity through recovery without reload.

This supersedes the earlier admission-failure/mobile-interaction exclusions,
not the published-artifact, candidate-branch, reporter-browser, reverse-proxy or
long-outage exclusions. Release qualification and soak remain separate.

Repair validation: all six browser checks pass (Chromium 141.0.7390.37,
100% zoom, 1440/1100/390×900), each injected case recording one failed admission
request. Desktop and narrow repaired screenshots were inspected. 107 focused
runtime, architecture and websocket tests pass. The browser receipt binds
the repaired runtime source bytes; baseline artifacts are retained locally in
`tmp/navigation-admission-baseline/`, repaired artifacts in
`tests/integration/test-results/` and `tests/integration/playwright-report/`.
These local artifacts are not release qualification or automatic CI admission.

## Superseded organisation admission — responsive qualification

The isolated main-derived base `58413ebdf6af81018bce15ff4023ca9824c0cdae`
allows an outstanding outgoing organisation admission response to restore its
platform destinations after switching organisations. This is a separate
request-ordering defect, not evidence that #1899 involves organisation switching
or that resource data crosses tenant boundaries.

The repair permits only the latest admission request to update navigation.
A newer failed request still supersedes an older successful response; a newer
successful all-false response remains authoritative. There is no API, entitlement,
navigation design, retry or polling change.

Repeat the bounded full-app experiment after locked dependency installation in
`frontend-modern` and `tests/integration`:

```sh
pulse-heavy-run -- bash -c '
  for width in 390 1440; do
    for mode in failure success reverse; do
      PULSE_PROOF_WIDTH=$width PULSE_PROOF_MODE=$mode \
        node scripts/check-navigation-admission-race.mjs || exit
    done
  done
'
```

This uses local Vite, fresh Chromium contexts, synthetic HTTP responses for two
organisations and a cold synthetic socket. It invokes the production reconnect
subscriber and real organisation selector, not a replacement application store.
It is deliberately a frontend ordering experiment, not real-backend tenancy or
production-build qualification.

Six repaired cases pass at 390×900 and 1440×900 in Chromium 141.0.7390.37
at default zoom: newer admission fails, newer admission succeeds with no
platforms, and the outgoing response completes before the pending newer success.
All cases retain system navigation and restore platform navigation on a subsequent
successful switch without document reload. Narrow cases open More, dismiss with
Escape, reopen and follow Settings, then switch to Docker after recovery.
The synthetic inventory is empty; this proves route/control access, not loaded
resource rendering or alert acknowledgement.

Per-case screenshots, request sequences and exact runtime SHA-256 receipts are
written to `tmp/navigation-admission-race/repaired-<width>-<mode>/`.
Run the failure mode on the base source with `PULSE_PROOF_PHASE=baseline`
to retain separate baseline artifacts. Desktop and narrow baseline failure occurs
after the outgoing response restores platform navigation. Three focused ordering
regressions also discriminate the base; repaired runtime/architecture checks total
59 passes, and frontend typecheck passes.

This completes the missing narrow synthetic-browser coverage from the withdrawn
ordering proposal. Exact-candidate backend operation, reporter browser/proxy
behaviour, resource isolation, external alert delivery and release soak remain
unqualified. No released fix or issue closure is implied.
