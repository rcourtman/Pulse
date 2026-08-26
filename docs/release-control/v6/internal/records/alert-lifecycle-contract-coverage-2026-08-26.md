# Alert lifecycle transitions lack a deterministic contract harness

Recorded: 2026-08-26
Origin: alerting audit (2026-08-26) finding 2 and docs/ALERT_ENGINE_EVOLUTION.md
Phase 1.

## Gap

The alert manager's transition semantics (hysteresis trigger/clear,
sustained-for delays, severity derivation, confirmation counts) are
implemented imperatively across per-family check paths and asserted only
by per-unit tests. There is no deterministic, engine-independent statement
of the lifecycle contract, so regressions in the frozen transition core
keep shipping (#1682, #1683, #1553, #1693, #1724) and cross-family
semantic drift is invisible.

## Resolution

Phase 1 of docs/ALERT_ENGINE_EVOLUTION.md: a pure transition reducer
(`internal/alerts/reducer`) that characterizes the manager's semantics
family by family, plus a parity harness (`reducer_parity_test.go` in
`internal/alerts`) that drives both engines through identical observation
sequences and fails on any divergence. The reducer must characterize, not
improve: divergences are reducer bugs unless investigation proves a
manager defect, which is then fixed in the manager first.

First slice (2026-08-26): the metric-threshold family. The harness
immediately caught a real manager defect: `checkMetric` hysteresis
resolution removed alerts by the legacy `<resourceID>-<metric>` ID while
canonical-identity alerts are stored under the canonical state key and the
legacy ID carries no alias, so the removal silently no-oped — a resolved
notification went out while the alert stayed active and re-resolved every
poll (guest per-disk usage alerts were the remaining production caller;
user-visible as the stale-alert class, e.g. #1580). Fixed at the caller
with a focused regression test before the parity slice landed.

Second slice (2026-08-26): the confirmation (discrete-state) family —
N-consecutive-match activation, single-observation recovery at the
evaluator layer, spec-carried severity — characterized against
`evaluateCanonicalLifecycleAlert`. The harness again surfaced a real
defect: the confirmation maps persist only counts, so
`lifecyclePreviousState` reconstructed pending runs dated at the current
observation and activation stamped StartTime at the final confirming poll,
understating outage start by the whole confirmation window (the manager
had already fixed this once for unified incidents via
`unifiedIncidentFirstSeen` but not for the generic lifecycle path). Fixed
with `lifecycleFirstMatched`, mirroring that precedent, before the parity
slice landed.

Third slice (2026-08-26): the recovery-confirmation gate — the poll-driven
offline composition where offline polls reset the recovery counter and run
the evaluator while healthy polls skip the evaluator and run
`clearResourceOfflineAlert` (N consecutive healthy polls to resolve,
default 3, storage 2; pending still clears on one healthy poll). The
harness caught a defect in the slice-2 fix itself: callers that reset
confirmation counts directly (bypassing the evaluator path) left a stale
`lifecycleFirstMatched` entry that backdated the next run's alert to the
previous run's first observation. Fixed by re-stamping whenever an
observation starts a new run.

Fourth slice (2026-08-26): re-fire-within-retention start restoration —
a resolve followed by a re-activation inside the recently-resolved
retention (5 minutes) restores the original occurrence's StartTime and is
treated as the same occurrence (EventRefired in the reducer; the manager
reactivates without a new history entry); outside the window the re-fire
is a fresh occurrence. First slice where parity passed with no manager
defect found. Known un-characterized nuance, deliberately deferred: the
manager's retention check mixes wall-clock now against evidence-stamped
resolve times, which the reducer normalizes to ObservedAt throughout —
that difference disappears at cutover.

Fifth slice (2026-08-26): the acknowledge lifecycle — ack marks a firing
incident and its canonical record, survives per-tick alert rebuilds
(preserveAlertState's existing branch) and short resolve/re-fire cycles
(the restore branch), is removed by unacknowledge, and expires once the
inactive record passes the one-hour cleanup TTL. The manager's restore
has no age check of its own — expiry is cleanup's pruning — so the
reducer draws the hour deterministically at restore time (AckRetention),
and the parity expiry scenario exercises the manager's real Cleanup pass.
Applies to both families (checkMetric shares preserveAlertState). Second
consecutive slice with no manager defect found.

Sixth slice (2026-08-26): the intent gate — operator suppression
(expected offline, muted, retired, active maintenance windows) and
explicit grace policies hold ACTIVATION only, while confirmations keep
counting; release activates with the run's first active observation as
the alert start; an already-firing incident is never suppressed by
intent. The manager accrues grace on monotonic process runtime while the
reducer measures the signal clock — coincident under continuous
operation, recorded as a nuance that disappears at cutover. Third
consecutive parity slice with no manager defect; the operator scenarios'
StartTime parity additionally exercises the lifecycleFirstMatched fix in
composition. Deferred: the backup-offline deferral sub-policy
(MaxDeferralSeconds / PostGraceSeconds).

Capstone (2026-08-26): the shadow-mode runtime feed. The reducer now runs
continuously inside the live manager against the same production
observations — the canonical lifecycle path (connectivity, powered-state,
discrete-state kinds), the poll-driven recovery paths, manual
acknowledge/unacknowledge/clear, with persisted-alert seeding at enable —
and every state disagreement is counted (Manager.ShadowDivergences) and
recorded in the alert event log as a shadow_divergence event
(rate-limited per key; the reducer resyncs to the manager after each
report so one divergence yields one event, including divergences from
unhooked mutations). Enabled by the monitoring bootstrap next to the
event log. The divergence rate is the go/no-go evidence for each Phase 2
family cutover.

Remaining: the backup-offline deferral sub-policy (the shadow feed maps
its hold to operator suppression, which is behavior-equivalent while
uncharacterized), and Phase 2 itself.
