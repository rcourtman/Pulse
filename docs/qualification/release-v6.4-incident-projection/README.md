# Retained incident occurrence projection backport

Named regression on supplied release/v6.4 base
601d061dadcba23410b0b251bcb3736ab4fffe5a: canonical timeline reads absorb
later recurrences, making an older resolved incident appear open. Release
Train rules 2/4 permit this corrective backport of Core
a41c60597b3f1282803bedba7b2cba16f04b6357. It is distinct from the
already-present shell identity repair. Runtime delta is identical to Core;
the monitoring test additionally imports testify/require on this line.

Baseline with new tests:
- CanonicalProjectionOccurrenceBounds fails at both 2m and 500ms.
- Dispatcher initially failed compilation (missing test-only require import);
  after adding that import it fails expected resolved / actual open at both
  timings. No runtime change was present for either baseline.

Focused repaired verification command:
go test -race ./internal/ai/memory ./internal/monitoring -run '^TestIncident|^TestMonitorLifecycleReplayPreservesOccurrenceTimelines$' -count=20

Logs are retained beside the lane outcome in
/var/lib/pulse-maintainer/queue/staging/20260907T221516Z-release-line/
(baseline.log, dispatcher-baseline.log, repaired.log); the coordinator may
archive this directory. These tests use in-memory canonical stores.

Scope: retained-shell reads are bounded by exact start and next retained
start for the same alert/resource; historical acknowledgement stays separate.
No shell-less/evicted-history repair, duplicate migration, write-byte reduction,
installed restart or recipient-delivery acceptance is asserted. Issue #1966
remains unresolved. No new feature or persistence optimisation is included.

This is source backport evidence, not selected-snapshot qualification. Normal
protected review and steward version/candidate disposition remain required;
do not substitute this head for an immutable selected release. Preserve prior
failed latency qualification and excluded crash evidence. No release approval.

Result: PASS, memory 7.117s and monitoring 4.444s, race-enabled count=20.
