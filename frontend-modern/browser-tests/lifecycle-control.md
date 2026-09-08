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

## Headed window control — 8 September 2026

The opt-in headed diagnostic uses an owned display (never a user's desktop):

```sh
pulse-heavy-run -- xvfb-run -a -s '-screen 0 1280x800x24 -nolisten tcp' node scripts/check-browser-single-session-control.mjs --headed-window
```

[Playwright's headed Linux guidance](https://playwright.dev/docs/ci#running-headed)
identifies Xvfb as the display prerequisite. This is not proof of window-manager
activation. This mode launches real headed Chromium with one raw page session,
without enabling focus emulation. It requests minimisation of the target's owned
window before freeze, restoration after resume, then tab activation. It records
all intervention commands and samples every 250ms for a preselected ten-second
foreground bound. Freeze/resume must be ordered with identical timer counts;
subsequent timer progress is required at foreground, rather than after the old
250ms hidden-page delay. A visible event after resume and visible/focused final
state are independently required. Default/headless checks retain their old bound.

One run on Chrome 141.0.7390.37 (revision as above), Playwright 1.56.1,
Node v24.20.0, linux x64, Xvfb/xserver-common `2:21.1.12-1ubuntu1.6`, display
`:99`, exited 1 at `Must observe visible transition after resume`:

- Baseline visible/focused at tick 5.
- Acknowledged minimise request: still visible/focused at tick 10, no event.
- Freeze/resume ordered at tick 10; post-resume hidden/unfocused at tick 10.
- Acknowledged restore and tab activation: hidden/focused throughout the bounded
  observation; timers eventually reached tick 20, no visible event.

Thus the short timer bound is no longer the immediate failed assertion, but
headed launch alone did not establish visibility restoration. No window manager
was started; openbox and xdotool were unavailable. The unchanged minimise sample
is adverse evidence against this window mechanism, not proof that the browser
actually minimised. Do not repeat this bare-Xvfb arrangement for favourable samples.
A subsequent experiment needs verified native window/tab state transitions,
not merely a larger timeout or successful CDP acknowledgements. No Pulse fixture,
installed convergence, destination receipt or release qualification was exercised.
Complete output is retained in maintainer run `20260908T065015Z-web-product`,
`headed-window-control.log`. Earlier failed controls remain relevant.

## Native tab-switch control — 8 September 2026

Run the same owned Xvfb command with `--headed-tab` instead of
`--headed-window`. This does not request minimisation or require a window
manager: it creates a second owned blank tab and activates it. Before freezing,
it requires observed hidden → visible/focused → hidden states, each bounded to
ten seconds with 250ms samples. It then uses the existing 2100ms host-side frozen
wait, resumes while backgrounded, and activates the original tab. Ordered
freeze/resume with unchanged ticks, a subsequent visible event, final focus and
timer progress remain required. No synthetic visibility events or forced focus
are used; no Playwright page session is attached.

One run exited 0 on the same Chrome revision, Playwright and Node versions above,
owned display :99. Preflight recorded hidden at tick 5, visible/focused at tick
11, then hidden at tick 12. Freeze and resume both recorded tick 12.
Post-resume remained hidden/unfocused at tick 12; tab activation produced a
visible event and visible/focused state at tick 18. Complete runtime identities,
intervention commands/responses and samples are retained in maintainer run
`20260908T070012Z-web-product/headed-tab-control.log`.

This establishes the positive control for the **tab-switch** mechanism, not
window minimisation. Earlier adverse results and the default mode's unreliable
250ms hidden-timer sample remain unchanged. No Pulse fixture or installed-pair
acceptance ran. The next source-level convergence fixture must retain raw
single-session ownership, the genuine background preflight, and the suspension
probe rather than attach a Playwright page and assume equivalent behaviour.

## Suspended incident component convergence — 8 September 2026

```sh
pulse-heavy-run -- xvfb-run -a -s '-screen 0 1280x800x24 -nolisten tcp' node scripts/check-browser-single-session-control.mjs --incident-convergence
```

This opt-in mode retains the raw single-session headed-tab preflight and probe,
then mounts the production incident hook and panel through a loopback Vite
fixture. No Playwright page is attached. It initiates two synthetic HTTP reads
before freezing, sends the newer response followed by the obsolete response from
the host while frozen, waits 2100ms, resumes and activates the original tab.
Both native foreground recovery and ordered suspension must pass before checking
convergence (ten-second bound): latest incident retained, obsolete incident
absent, loading/error cleared and no notification error. Fixture button clicks
are scripted setup, not a claim of native pointer/keyboard interaction.

The first run used identical pending GET URLs and exited 1 waiting for the
second request, before freeze. Only one request reached the fixture. Same-URL
request handling was a possible confounder, not an established browser diagnosis
or product failure. The revised fixture gives reads distinct query identifiers
and disables fetch caching. One revised run exited 0: hidden → visible/focused →
hidden preflight; freeze/resume both at tick 11; visible/focused at tick 17;
latest incident rendered, obsolete absent, loading/error false, notifications 0.

Both outputs are retained under maintainer run `20260908T072007Z-web-product`:
`incident-convergence.log` (adverse) and
`incident-convergence-distinct-reads.log` (passing). Runtime was Chrome
141.0.7390.37, revision `9f043f63b0e5b728c8d09f3e3ddfc1681a4bd58e`,
Playwright 1.56.1, Node v24.20.0 linux x64, owned Xvfb display :99.
The existing ownership runner now imports the unchanged shared fixture string.

This is **component-source evidence**, not full application convergence:
authentication, application WebSocket refresh, installed restart, selected public/
private release pair, native mobile lifecycle, and destination receipt are not
exercised. The fixture replaces AlertsAPI's incident read with synthetic HTTP;
it does not establish that the application initiates a refresh on foreground,
nor guarantee network callback execution order from host response order. Earlier
failed controls remain adverse evidence about their respective mechanisms.
