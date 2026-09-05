# Owned browser writers — 5 September 2026

Base: ddafcf53303d0637a6b138bb5920e4f8abc32a06 (main-based lane).
Scope: qualification tooling only; no user-visible or permission change.

Fresh external evidence: https://playwright.dev/docs/auth warns that stored
browser state can impersonate test accounts. Wider discovery on September 5
again found security/alerting concerns in
https://www.reddit.com/r/Proxmox/comments/1vnztw1/enterprise_observability_solutions/;
search excerpts only this turn, not a new demand count or full-thread triage.
Neither source is demand for a sharing extension. The concrete local defect
was shared cookie storage and shell-only interruption cleanup.

The Linux shell runner now enters a Python child-subreaper supervisor. It
allocates the invocation identity and private cookie directory, overrides
inherited cookie paths, adopts orphaned writers and reaps them before deleting
owned state. TERM/INT/HUP remove reports/results only after writers exit.
Ordinary completion removes cookies but retains reports for inspection.
Unresponsive writers receive KILL after five seconds; failure to reap within
fifteen seconds retains artifacts and fails rather than claiming cleanup.

Validation (focused fixtures, not Docker or browser qualification):

- `python3 tests/integration/scripts/owned-run.test.py`: 3 passing test methods,
  including success/nonzero exits, three catchable signals, a concurrently
  running neighbour, detached late-writing grandchildren and TERM-resistant
  writer escalation. Detached writer PID no longer exists after completion.
- `node --test tests/integration/scripts/run-tests-images.test.mjs tests/integration/scripts/run-tests-cleanup.test.mjs tests/integration/scripts/run-tests-interruption.test.mjs`:
  19 passed. Three new shell-entrypoint tests observe a final writer marker,
  absent owned roots, preserved unowned cookie and a compose-down invocation.
  Docker/npx are fixtures: this does not prove actual container teardown.
- Sharing-only managed-local-backend fixture: 1 passed.
- Bash syntax and `git diff --check` passed.

Limits: Linux/Python 3 and /proc are now explicit requirements for this isolated
shell runner; unsupported platforms fail closed. Direct npm/setup/helper runs
retain their existing weaker guarantees. Supervisor SIGKILL, host failure and
unreapable processes require manual cleanup. No full suite/build/browser run
was made and no diagnostic tier was promoted.

Next qualification remains delayed outgoing admission, failed/superseded
incoming admission and reconnect recovery. Inspect fresh 390x844 screenshots
before changing or claiming polished table layout. The accepted identity/RBAC
repair and happy-path matrix were not repeated. Real-browser interruption is
still required before claiming browser-process cleanup beyond these fixtures.

## Bounded preflight repair

The first candidate omitted the same-commit deployment-installability contract
and a registry-recognised orchestration proof. Added the actual supervisor
requirements, lifecycle and failure limits to the contract's Purpose section.
The recognised managed-local-backend test now executes cookie-path selection
and the existing behavioural supervisor/shell fixtures. No exemption is used.

Initial focused rerun: 22 passed, 4 discovery tests failed because this isolated
checkout lacked Playwright dependencies. Installed the locked dependencies
with `npm ci --ignore-scripts --no-audit --no-fund`; all 26 tests then passed,
including diagnostic discovery and tier refusal (no browser launched).
Canonical completion and browser-receipt guards passed/skipped as appropriate;
contract, control-plane, status and registry audits returned zero. No new
browser receipt was fabricated for this tooling-only change.
