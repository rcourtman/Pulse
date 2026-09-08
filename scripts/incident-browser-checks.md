# Incident browser checks

These mount the production incident hook and panel with synthetic loopback HTTP.
They do **not** mount the full application, authenticate, connect its WebSocket,
exercise a real history-table row, or qualify installed delivery.

Install the root and `frontend-modern` lockfile dependencies and the matching
Playwright Chromium. Run headed checks on an owned X display (for example Xvfb):

```sh
pulse-heavy-run -- xvfb-run -a node scripts/check-browser-single-session-control.mjs --incident-user-refresh
pulse-heavy-run -- xvfb-run -a node scripts/check-browser-single-session-control.mjs --incident-convergence
```

`--incident-user-refresh` completes an initial read using the fixture's Open row
button, backgrounds the tab, explicitly freezes it through CDP, resumes it, and
activates the original tab. It verifies freeze/resume timer suspension separately
from native foreground visibility, then invokes the enabled production Refresh
button and checks that changed incident data replaces the cached result without
an error. It also checks that no incident request occurred merely on resume
within the observation interval. The DOM button invocation is not pointer or
keyboard accessibility coverage. Automatic foreground freshness is not implied.

`--incident-convergence` instead starts two overlapping reads using an unrestricted
fixture button and sends latest then obsolete HTTP responses while frozen. It
checks latest-request ownership after resume, not callback ordering or ordinary
production-button interaction. Keep these two evidence claims separate.

Both commands log browser/runtime versions, lifecycle observations, commands and
final state. Retain failed logs alongside successes; a successful component check
must not be described as installed or full-application acceptance.

## Full application row / native return

The existing VMware history journey has an opt-in headed Chromium extension:

```sh
pulse-heavy-run -- xvfb-run -a env PULSE_E2E_USE_LOCAL_BACKEND=1 \
  PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 PULSE_E2E_INCIDENT_FOREGROUND=1 \
  npm --prefix tests/integration test -- \
  tests/36-vmware-alert-history-resource-incidents.spec.ts \
  --project=chromium --headed --workers=1
```

Install the `tests/integration` lockfile dependencies as well. The managed runner
builds and owns a local backend and embedded frontend, authenticates using the
existing helper, and cleans up the backend. Do not target a shared installation.
The extension selects the real history-table Resource button, settles the first
incident response, observes native hidden/visible transitions with focus emulation
disabled, then clicks the enabled production Refresh button. It requires exactly
two incident reads and changed rendered details. This does not freeze the page.

Alert configuration, active alerts, history, incident and resource HTTP responses
are mocked; the history route's WebSocket is suppressed. In particular, the fixture
supplies `enabled: true` and `activationState: "active"`; it does not save settings
or prove that saved intent survives reload or restart. Authentication uses the
local backend, but this is not an exact release-pair, live WebSocket convergence,
real incident-store, activation, or delivered notification check. Keep any failed application run as evidence, not a component
fixture success in its place.

Retained adverse run (8 September 2026, c98c0565a7): the first local run reached the real
history row and settled initial incident details, then failed the native-hidden
preflight: the original page stayed `visible` for ten seconds after another
Playwright page was activated. The source-built frontend/backend succeeded and
the backend was stopped by cleanup. Do not remove the visibility assertion or
replace it with a synthetic DOM event to claim foreground coverage. That revision did not successfully execute Refresh after return. The raw
single-CDP component checks above do not clear this application failure.

The additive harness uses `tests/integration/tests/native-visibility.ts` only
when the opt-in is set. Playwright 1.56.1 enables focus emulation on its owning
CDP session (`crPage.js`); disabling it on a secondary session leaves the first
override active. A loopback-only CDP proxy forwards commands and replies, changing
only `Emulation.setFocusEmulationEnabled({enabled:true})` to `false` on that
owning session. It checks that this command was encountered. It does not inject
DOM visibility, fabricate events, freeze the page or change assertion results.
An owned Chromium/profile and proxy are cleaned up afterwards. Native tab
activation and visibility polling still have to pass before Refresh is clicked.
This version-sensitive harness must fail rather than infer backgrounding if
Chromium or Playwright changes. Ordinary runs use the unmodified base fixtures.

For the ordinary-journey control, repeat the command above with
`PULSE_E2E_INCIDENT_FOREGROUND` unset. Retain both exact-revision results;
a prior success is not evidence for a later untested revision.

## Saved alert intent (unmocked local backend)

```sh
pulse-heavy-run -- env PULSE_E2E_USE_LOCAL_BACKEND=1 PULSE_MOCK_MODE=false \
  PULSE_E2E_ALERT_CONFIG_PERSISTENCE=1 PULSE_E2E_SKIP_PLAYWRIGHT_INSTALL=1 \
  npm --prefix tests/integration test -- tests/97-alert-config-persistence.spec.ts \
  --project=chromium --workers=1 --retries=0
```

This opt-in test requires the managed, disposable backend. It activates that
instance through the real configuration API, changes Recovery notifications
through the production Schedule UI, verifies the staged value has not reached
the server, reloads to prove the unsaved edit is discarded, then edits again
and clicks Save Changes. It checks the real PUT result, subsequent
GETs and rendered control after both page reload and managed backend restart
with preserved data. It does not intercept HTTP or WebSocket responses and
must not be run against a shared installation. The attachment contains only
expected non-secret settings and evidence boundaries, not full configuration.

This proves saved recovery intent only when the test passes. API activation
setup is not UI activation coverage, and no resource or destination is seeded.
It does not establish alert-engine firing, retry exhaustion, delivered recovery,
recurrence or off-LAN receipt. Those require separate evidence; notification
package loopback tests are not a substitute for installed exact-pair acceptance.
