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

History, incident and resource HTTP responses are mocked; the history route's
WebSocket is suppressed. Authentication uses the local backend, but this is not
an exact release-pair, live WebSocket convergence, real incident-store, or delivered
notification check. Keep any failed application run as evidence, not a component
fixture success in its place.

Known harness limitation (8 September 2026): the first local run reached the real
history row and settled initial incident details, then failed the native-hidden
preflight: the original page stayed `visible` for ten seconds after another
Playwright page was activated. The source-built frontend/backend succeeded and
the backend was stopped by cleanup. Do not remove the visibility assertion or
replace it with a synthetic DOM event to claim foreground coverage. The native
browser-control prerequisite remains unresolved; the Refresh portion of this
application scenario has **not** been executed successfully. The raw single-CDP
component checks above do not clear this application failure.
