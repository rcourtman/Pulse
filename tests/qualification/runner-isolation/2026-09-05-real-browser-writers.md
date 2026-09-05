# Real browser writer interruption — 5 September 2026

Base: 07404979e2e65238b7f5a89290c855675571b923. Qualification-only;
no runtime behaviour, permissions or user-visible surface changed.

## Judgment and fresh evidence

The accepted supervisor needed real-browser proof rather than another synthetic
process fixture. Fresh reads of https://playwright.dev/docs/auth confirm stored
browser state can impersonate accounts. Fresh community discovery and a visible
thread read at
https://www.reddit.com/r/Proxmox/comments/1t1h834/deploying_pulse_monitoring_agents_as_root_on/
surface operator questions about root-agent deployment. That is trust context,
not evidence of a sharing defect, new demand, or a complete issue-triage audit.
No new bet or feature was introduced.

## Reproduction

Install the locked integration dependencies if absent:
`npm ci --prefix tests/integration --ignore-scripts --no-audit --no-fund`.
With the locked Playwright Chromium installed, run:

```
pulse-heavy-run -- python3 tests/integration/scripts/owned-run-browser.test.py
```

One test method, three real Chromium subcases (TERM, INT, HUP), passed in
3.255 seconds. Each waits for live browser PID and synthetic cookie storage,
signals the actual supervisor, then requires completed late screenshot, storage,
trace, nonempty saved video and summary writes before browser close. Supervisor
exit must match the signal. Browser PID and owned cookie/report/results roots
must be absent; a later check rejects recreation and an unowned sentinel stays
unchanged. Temporary synthetic artefacts are removed, never committed.

The first run failed all three subcases at the nonempty-video assertion: a
connected browser requires explicitly saving its video to the client results
path. Added `video.saveAs` after context close; the passing rerun retains the
nonempty-video assertion. No product fix is inferred from that harness repair.

## Scope limits and next proof

This launches real Chromium through the actual Python supervisor, with browser
signal handlers disabled so delayed client shutdown exercises late writers.
The report summary is a fixture, not the Playwright Test HTML reporter. The
page is offline and cookies synthetic; no Pulse server, Docker stack or customer
account is used. It does not replace full shell/Playwright Test interruption or
actual container inventory proof, and does not qualify supervisor SIGKILL or
host failure. Existing process fixtures cover escalation and detached writers.

Delayed outgoing admission, failed/superseded incoming admission, reconnect
recovery and fresh 390x844 table-clipping inspection remain unqualified. The
accepted identity/RBAC happy paths were not repeated. No release readiness,
visual-polish or quarantined-tier promotion claim.

Focused regression checks also passed: `python3
 tests/integration/scripts/owned-run.test.py` (3 methods) and `node --test` on
`run-tests-images.test.mjs`, `run-tests-cleanup.test.mjs` and
`run-tests-interruption.test.mjs` (19 tests). `git diff --check` passed.
