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
