# Known RC Issue Closure For GA PBS Health Authority Record

- Date: `2026-07-24`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1465`
- Result: `fixed-main-proof`

## Context

Issue `#1465` showed two configured PBS systems as Active/Fleet OK in Settings
while the Dashboard rendered both as errors and the saved connection test
failed. The original `v6.0.0-rc.4` cause included deferred poll accounting that
captured a nil error before the poll ran. Commits `bf6261adc` and `bd7d196c1`
corrected that capture and are ancestors of `v6.1.1`.

Release-tag replay proved that correction was necessary but incomplete.
`v6.1.1` records an authentication failure in the scheduler ledger, yet the
authentication branch returns before updating the Dashboard PBS model.
Separately, the connections aggregator treats a new timeout as Active whenever
an earlier success exists and the circuit breaker has not opened. The same
current error is already defined as outstanding state and is cleared on the
next success, so the Active projection contradicts both the Dashboard and the
ledger.

## Disposition

PBS polling now finalizes one dynamic poll error and projects that result into
the scheduler ledger, staleness tracking, poll metrics, connection-health map,
Dashboard PBS model, and legacy PBS alert evaluation. Authentication, timeout,
cancellation, initialization, and panic failures publish offline/error
consistently. Optional node, datastore, namespace, and job collection failures
remain partial data evidence and do not convert proven connectivity into a
connection failure.

Client creation and retry recreation no longer claim connectivity before a
poll. Invalid initialization records and publishes a failed outcome; successful
construction remains pending until the first completed poll.

The connections API now treats any current `LastError` as unreachable, or
unauthorized for authentication errors, even when `LastSuccess` is preserved
for freshness context. Settings Fleet governance and connection alerts consume
that same connection row. Diagnostics exposes the canonical monitored state
and the immediate support probe separately, and the Settings diagnostics row
names both when they disagree. Saved Test Connection remains an isolated
credential/network probe and does not mutate runtime health.

## Scenario Proof

The managed dual-PBS HTTP fixture and API projection tests cover:

- two independently keyed PBS systems sharing the same hostname;
- initial success followed by authentication rejection and network timeout;
- preservation of last-success freshness alongside the current failure;
- a successful version probe with failed datastore inventory as connected
  partial data;
- credential rotation, URL replacement, monitor restart, and recovery;
- invalid URL initialization and first-poll pending behavior;
- consistent Settings/API/Fleet, Dashboard, alert-snapshot, and diagnostics
  projections;
- a direct saved-connection test using the current stored token without
  changing scheduler or connection health.

## Proof

- `v6.1.1`: `TestMonitor_PollPBSInstance_AuthFailure` passes, proving the RC5
  deferred-accounting correction is present.
- `v6.1.1` with the new regressions applied: the authentication case leaves
  the Dashboard model online, and a post-success timeout plus the same-hostname
  second PBS row both remain Active. These expected failures reproduce the
  unresolved contradiction on the release tag.
- `go test ./internal/monitoring -count=1`
- `go test ./internal/api -count=1`
- `go test -race ./internal/monitoring -run
  'TestPollPBSInstancesKeepsAllHealthProjectionsOnOneOutcome|TestPollPBSInstanceKeepsPartialDataSeparateFromConnectivity|TestInitPBSClientsDoesNotTreatClientConstructionAsConnectivity|TestMonitor_PollPBSInstance_AuthFailure'
  -count=1`
- `go test -race ./internal/api -run
  'TestDeriveConnectionState_CurrentFailureOverridesRecentSuccess|TestBuildConnectionsKeepsDistinctPBSSystemsWithSameHostname|TestPBSDiagnosticsKeepsCanonicalPollStateSeparateFromLiveProbe|TestHandleTestNodePBSIsADirectProbeWithoutChangingRuntimeHealth'
  -count=1`
- focused Settings diagnostics, connections-ledger, infrastructure-source, and
  architecture Vitest suites: 169 tests passed
- frontend TypeScript, ESLint, Prettier, and Settings diagnostics-boundary
  checks
- subsystem registry, contract, status, control-plane, staged-shape, and
  canonical-completion audits

## Outcome

The RC5 deferred-accounting correction is present in `v6.1.1`, but the complete
cross-surface behavior in this record is not. The remaining source-authority
fix and regressions are on `main` for a future v6 release; no backport or
publication date is claimed. Issue `#1465` remains open for reporter
confirmation after retesting a release that contains this change.
