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

Remaining for later slices: offline/confirmation families, ack/snooze
lifecycle, intent-policy interaction, and a shadow-mode runtime feed.
