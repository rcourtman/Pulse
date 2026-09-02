# Patrol Weekly Digest

Status: building — endpoint and in-app "This week" card on `main` (PRs #1856
and #1860, 2026-09-02); the weekly email lands as a `patrol_digest` report
schedule kind. Demand ledger: `pulse-pro/FEATURE_REQUESTS.md`, "Patrol weekly digest
(what Patrol did for you)", a named bet under the Patrol operations loop.

## The job, in the customer's words

"Show me what I am paying for." A Pro customer turns Patrol on, it runs about
five times a day, and nothing in Pulse ever adds that up. They see individual
findings when they open the page and individual emails when a finding fires,
but never the week: how often Patrol looked, what it caught, what it fixed,
what it cost. The 2026-09-01 assessment found 33 of 175 Pro subscriptions past
due and 80 percent of paying installs never seeing the paid loop fire. A
customer who cannot see the work stops paying for it.

The least-expert plausible reader is a homelab operator who set Patrol up once
and opens Pulse when an alert email arrives. They do not know what an
"investigation", a "verdict", or an "evidence class" is. The digest must read
as a plain weekly account, and every number must lead to something they can
click or a decision they can make.

## What the digest reports

Everything below is computed from data Pulse already records. No new
telemetry fields; the readout task grades this bet from support and issue
mentions.

| Line | Source | What the reader does with it |
| --- | --- | --- |
| Runs: how many times Patrol checked, how many checks in total, how many resources it covered, when it last ran | `PatrolRunRecord` history (`internal/ai/patrol.go`, capped at 100 runs) | Confirms Patrol is alive. If runs are zero or failed, the card says so and points at Patrol setup. |
| Mode: monitor, approval, assisted, or full | `Service.GetEffectivePatrolAutonomyLevel` | Explains why actions were or were not taken. Monitor mode with fixable findings is the upgrade prompt, stated plainly. |
| New issues this week, and how many are still open by severity | run `new_findings` totals plus the findings store for still-tracked findings detected in the window | Open critical or warning issues are the thing to look at; the count links to the findings list. |
| Issues resolved and how many Patrol resolved on its own | run `resolved_findings` totals plus manual resolutions still in the findings store | This is the "it did something" line. |
| Issues you dismissed or muted | finding lifecycle events (`dismissed`, `suppressed`) in the window | Reminds the reader what they told Patrol to ignore. |
| Investigations and their outcomes | finding lifecycle `investigation_outcome` events in the window | Shows the paid loop firing; outcomes that need attention are named. |
| Actions proposed, approved, executed, verified, failed, and still waiting | canonical action audit records with `origin.surface = patrol` (`internal/unifiedresources`) | "Waiting" links straight to Actions and approvals. |
| Alerts Patrol looked into | alert-triggered runs (`alert_fired`, `alert_flapping`) and their distinct alert identifiers | Shows Patrol responding to the reader's own alerts, not just its schedule. |
| Estimated model spend, tokens, and calls | `internal/ai/cost` usage events with `use_case = patrol` | Answers "what does this cost me" without opening the AI cost dashboard. Marked as an estimate; unknown pricing is stated, never zeroed. |

Deliberately left out: per-run tool call traces, evidence classes, verdict
strings, model names, and anything the reader cannot act on. Those stay in run
history and the Actions audit, one click away.

## Honest limits of the data

- Run history keeps the last 100 runs. At five or six runs a day a week fits,
  but a busy install with event-triggered runs can overflow. The payload
  carries `window.history_complete` and `window.history_since`; the card says
  "since <date>" instead of "this week" when the window is cut short.
- Resolved findings are purged from the findings store 24 hours after
  resolution, and dismissed findings after 30 days. Resolution totals therefore
  come from run records, and severity is only reported for findings still
  tracked. Investigation outcomes are read from lifecycle events on tracked
  findings, so a finding that was investigated, fixed, and purged more than a
  day ago no longer contributes. Executed and verified actions do not have this
  gap: action audit records are durable.
- Spend is `cost.EstimateUSD` over recorded usage events. When the model has
  no known price the digest says pricing is unknown rather than reporting a
  smaller number.

## Surface

**In-app card first.** A "This week" card at the top of the Patrol page's
Activity tab, above Verified outcomes
(`frontend-modern/src/features/patrol/PatrolWeeklyDigestCard.tsx`, backed by
`GET /api/ai/patrol/digest?days=7`). It costs nothing to deliver, every paying
install can see it, and the Activity tab is already where "what happened"
questions are answered. The Inbox stays a decision surface and does not gain a
summary card.

**Weekly email second.** The past-due population is the population that has
stopped opening Pulse, so the email is the slice that reaches them. It is a
report schedule kind, `patrol_digest`, on the report schedule API
(`POST /api/admin/reports/schedules` with `"kind": "patrol_digest"`): the
existing scheduler, cadence, recipients, and Pro reporting entitlement are
reused, and the run renders the same digest as plain-language HTML and text
(`internal/api/patrol_digest_email.go`) through the tenant email config via
`SendEmailWithRetry`. Digest schedules are weekly and email-only; nothing is
written to disk. A run without an email destination fails with a message that
names the missing setting instead of silently doing nothing. Webhook and
Apprise channels are out of scope. The Settings > Reporting form gains a
"Report type" selector for the kind in a follow-up; its browser proof needs a
Pro-licensed instance, which the isolated verification stack does not have.

## API

`GET /api/ai/patrol/digest` — requires `ai:execute` scope like the other
Patrol read endpoints. Query `days` (1 to 30, default 7). Returns the window,
current mode, and the run, finding, investigation, action, alert, and spend
rollups described above. Empty history returns zero counts with
`window.history_complete = true`; a missing AI or Patrol service returns the
same zero shape so the card can render its "Patrol has not run" state.
