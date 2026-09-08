# Browser suspension diagnostic

Run `npm ci --ignore-scripts --no-audit --no-fund` at repository root, then
`pulse-heavy-run -- node scripts/check-browser-lifecycle-control.mjs` using an
installed Playwright Chromium. This uses owned blank pages only, no Pulse server
or credentials. It is deliberately not wired into CI or release qualification.

The comparison is fresh-session focus false versus true then false, each in a
new context. Host-side waits avoid evaluating the frozen page. Success requires
ordered freeze/resume events with identical timer counts and subsequent timer
progress, not merely acknowledged protocol commands. Exit 1 means the
intervention did not establish the control; it does not mean Pulse is broken.

On 8 September 2026, Chromium 141.0.7390.37 / Playwright 1.56.1 / Node 24.20.0
returned successful commands for both cases, but each advanced from 5 to 52
ticks and remained visible/focused without lifecycle events. The intervention
failed. No Pulse application was loaded and no incident-convergence assertion
was exercised.

The hypothesis came from the version-matched Chromium
[emulation agent source](https://raw.githubusercontent.com/chromium/chromium/141.0.7390.37/third_party/blink/renderer/core/inspector/inspector_emulation_agent.cc):
the equality guard in `setFocusEmulationEnabled` can skip updating the page on
a fresh false request. Avoiding that guard did not establish suspension in
this experiment. Do not continue treating that guard as a sufficient diagnosis.
The next investigation must establish actual page visibility/lifecycle control
under the automation harness before attempting Pulse recovery acceptance.
