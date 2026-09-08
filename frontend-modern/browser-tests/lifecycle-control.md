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

## Single-owner control — 8 September 2026

`pulse-heavy-run -- node scripts/check-browser-single-session-control.mjs`
launches the installed Chromium executable selected by Playwright, but uses one
raw CDP session per owned blank target. It does not attach a Playwright page
session, modify dependencies, or load Pulse. The preselected comparison is forced
focus (negative control) versus no forced focus (positive control), with fresh
targets. Both must meet their assertions for exit 0. Protocol calls have bounded
timeouts; the temporary profile and owned browser are cleaned up.

Fresh [Playwright 1.56.1 source](https://raw.githubusercontent.com/microsoft/playwright/v1.56.1/packages/playwright-core/src/server/chromium/crPage.ts)
shows focus emulation enabled on its main-frame session. This suggested a
competing-session confounder in the earlier test, not another reason to repeat
false/true-to-false commands on the separate diagnostic session.
[Chrome lifecycle guidance](https://developer.chrome.com/docs/web-platform/page-lifecycle-api)
describes timer suspension in frozen pages. These sources informed the control;
only observed timer and event behaviour determines success.

Observed on Chrome 141.0.7390.37, revision
`9f043f63b0e5b728c8d09f3e3ddfc1681a4bd58e`, Playwright 1.56.1,
Node v24.20.0, linux x64:

- Forced focus: ticks 5 → 52, visible/focused, no lifecycle events.
- No forced focus: ticks 5 → 6; visibilitychange to hidden, freeze at tick 5,
  resume at tick 5, then timer progress. All commands acknowledged; exit 0.

The resumed target remained hidden/unfocused. This proves freeze/resume, **not**
foreground return, visibility restoration, incident convergence, or an installed
release. Preserve the earlier failed experiment: this uses a different protocol
ownership arrangement, not a passing rerun of that intervention. A Pulse browser
scenario must retain this ownership arrangement and its timer/event probe; adding
a Playwright page attachment may reintroduce the confounder. Before testing
foreground convergence, separately observe return to visible. Installed acceptance
still requires the authorised synthetic installation and exact byte identities
listed in `alert-recovery-acceptance.md`; neither is supplied by this control.

## Foreground activation control — 8 September 2026

`pulse-heavy-run -- node scripts/check-browser-single-session-control.mjs --foreground`
adds `Page.bringToFront` after the separate post-resume snapshot, then takes a
foreground snapshot after 250ms. Default invocation retains the original
freeze/resume-only behaviour. The optional mode requires visible **and** focused
state plus timer progress; it does not force focus emulation back on to obtain a
passing result. No Playwright page is attached.

Fresh [CDP documentation](https://chromedevtools.github.io/devtools-protocol/tot/Page/#method-bringToFront)
describes tab activation separately from lifecycle state. The command's success
is not evidence of visibility restoration. On the same Chrome revision and
runtime listed above, both the initial implementation and the final optional-mode
run exited 1:

- Forced-focus negative: ticks 5 → 52 → 57; visible/focused throughout, no events.
- Unforced positive: freeze and resume at tick 5, hidden/unfocused after resume;
  after activation, **hidden/focused**, without a visible visibilitychange event.
- The first run stayed at tick 5 through the foreground sample; the optional-mode
  run reached tick 6 only at that final sample. Both failed the earlier
  post-resume timer-progress assertion before reaching the foreground assertion.

The 250ms post-resume sample is therefore not a dependable timer-progress bound
for this hidden target. Do not interpret that failed assertion as missing resume:
ordered freeze/resume events were recorded. Independently, visibility restoration
failed in both observed foreground samples. Neither threshold was relaxed and
neither failure is cleared by the earlier successful freeze-only control.

No Pulse fixture was exercised: the prerequisite foreground control is still
unestablished. Before applying this arrangement to application convergence,
choose and verify an actual visibility transition (potentially a headed browser
with owned window activation), retaining the suspension probe and all failed
observations. Repeating this activation unchanged, synthetically dispatching a
visibility event, or enabling forced focus would not resolve the evidence gap.
This diagnostic is not a release gate and proves nothing about installed
incident state or destination receipt.
