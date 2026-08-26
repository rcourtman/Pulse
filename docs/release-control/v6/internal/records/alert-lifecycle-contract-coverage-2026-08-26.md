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

Remaining for later slices: re-fire-within-retention start restoration,
ack/snooze lifecycle, intent-policy interaction, and a shadow-mode
runtime feed.
