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
