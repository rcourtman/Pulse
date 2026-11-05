# Canonical event-history durability repair

Recorded 5 November 2026. This is the event-history follow-up to
[the retained reliability experiments](alert-persistence-reliability-2026-11-05.md).
Notification receipt continuity and agent update identity are separate work.
This record does not close L6 or qualify a release.

## Baseline

The first three retained proof artifacts were repeated on Linux with
Go 1.26.10 at exact source `c85a480c40bb7dd920503cbdade15a3db43d3168`,
through the shared native validation gate. The diagnostics reproduced 40
accepted background events missing after reopen, 500 admission drops out of
1,524 offered events while Flush succeeded, and returned synchronous failures
poisoning later flushes. The real system-alert producer's post-retirement
occurrence disappeared across later boots while history telemetry reported
healthy. Its callback ran; no successful provider delivery was established.
The baseline diagnostics pass by observing defects, not by proving repair.

Local retained baseline receipts are under
`/Volumes/Development/.caches/pulse-reliability-followups-20261105/history-baseline/`,
including `source-c85a480.sha` and `history-baseline-c85a480.stdout.txt`.
The earlier distinct `6d32995154de9ff51e34cdc1df0ec2cf7981cea5` repeat remains
separately labelled and is not the exact-main baseline.

## Runtime account

Every admitted lifecycle, imported history batch and distinct diagnostic
episode is encoded into an independently synced append journal before SQLite
is attempted. Journal files and metadata live in `event-history-journal/`
beside `events.db`, not inside the failed database transaction. Append performs
bounded journal I/O without waiting for the SQLite writer; storage refusal
never suppresses alert evaluation or callbacks. This does not promise zero
filesystem latency during admission.

Transactions commit event rows, active-state projection, durable journal
origins and the replay cursor together. Recovery visits the unsettled prefix
in admission order, preserving original occurrence times. Reading the durable
cursor before retry handles a commit whose acknowledgement or compaction was
interrupted. Journal compaction cannot duplicate events already pruned from
ordinary history. Durable sequence reservations prevent origin reuse after a
compacted journal reopens. Public-operation leases drain before clean sealing
and physical journal-lock release. Recovery never invokes notification callbacks.

Active-state degradation markers capture the journal admission boundary before
copying active state. Event rows retain their origin through compaction, so
startup can fold committed and still-pending facts over a marker that predates
a journalled transition, even while the writer remains faulted. If SQLite is unreadable, independent
pending facts still fold over the trusted recovery marker and authority remains
incomplete. A failed partial bootstrap keeps a conservative proven boundary in
subsequent checkpoints. A complete fold releases that floor even if storing the
projection still fails. Partial checkpoints are explicitly labelled and cannot
replace a complete SQLite active projection: that projection deliberately
outlives event-history retention. On recovery it supplies the active set and
controls; partial data can refresh observations only for matching proven active
occurrences. Full checkpoints have a separate durable completeness declaration,
so one newly replayed activation cannot attest an unknown baseline. Legacy
authority declarations migrate before new replay writes. Unreadable baselines
retain their fallback until all positive sources can be reconciled.
A failed first SQL checkpoint does not downgrade a known complete fallback;
that fallback and retained lifecycle facts can establish a full checkpoint on
recovery, even after an isolated activation has initialized the projection.
Before the first full checkpoint, recovery unions all readable positive projection
rows with the partial fallback, folding each from its own cursor. Projection
membership and current controls survive even after their event rows are pruned;
older retained controls cannot roll them back. Only the completed reconciliation
establishes a full checkpoint, including an empty first baseline. No demo-mode
reset or database deletion is required.
Partial markers use schema 2, so older schema-1 readers fail closed; complete
schema-1 markers remain supported. Same-occurrence replay preserves newer observation fields
and applies explicit lifecycle control changes. Replacement restore and explicit
clear reset the independent failure-checkpoint mirror too. A hash-bound capture
sidecar for the compatibility mirror, and a separate capture prefix in degraded
markers, preserve positive omission by cleanup even when an older replay floor
remains necessary. Partial folds cap capture evidence at the proven floor; they
never label missing, compacted SQLite facts as deliberate absence. Cleanup and
whole-state reset now journal an explicit `active_retired` fact, which removes
that active occurrence without inventing detector recovery or dispatching a
resolution callback. Guest re-keying records one `active_moved` fact that
atomically removes the prior identity and preserves the new occurrence without
firing/resolving callbacks. Suppressed re-keying retires the prior identity.
Delayed acknowledgement/snooze/escalation snapshots cannot
reactivate an occurrence after resolution or overwrite unrelated observation
fields in the active projection. Interrupted mirror/sidecar replacement never binds the
boundary to different snapshot bytes. An explicit matching resolution can remove that occurrence;
absence alone cannot. Durable-ID order does not infer ordering from event
timestamps or treat an occurrence clock as an admission cursor.

## Missing facts and bounds

Pending facts never expire automatically. Admission is capped at 16,384 events
and 32 MiB of encoded unsettled payload. Diagnostic admissions stop at 75%
of either bound, preserving 4,096 events and 8 MiB for lifecycle obligations. The live append log, one atomic compact
replacement and one preserved damaged log bound peak journal payload to
96 MiB, apart from small metadata and filesystem overhead. Later damaged payloads retain a digest, size and unknown-gap account without
overwriting the first forensic artifact. If that copy cannot be written, the
durable digest, size and unknown-gap account remain the evidence. Startup cleanup
failures retain the journal owner, and a valid pending file does not require a
payload-sized rewrite to open. If the target cursor cannot be read, the existing
byte-bounded physical frames remain retained even if the conservative pending
count exceeds the fresh admission cap. Crash-left replacement files are
reclaimed under exclusive journal ownership. Ordinary
committed-event retention remains 90 days and 256 MiB of lifecycle snapshots.
These committed-history retention policies are independent of pending-work
retention.

Capacity or validation refusals of fail-open emissions record exact offered-event loss counts when independent metadata
can be synced. An incomplete frame or checksum failure preserves the valid
prefix, preserves damaged bytes when storage permits and records an unknown gap. An unclean
exit conservatively records an unknown pre-journal tail even when every
retained frame can be recovered. If both journal and gap persistence fail,
only a previously synced open-session marker can support unknown-tail evidence
on reopen; no durable exact loss count is claimed. If even an initial session
marker cannot be persisted, no new durable evidence can be guaranteed. The
current owner remains unavailable and retries storage rather than claiming health. The journal shares SQLite's
filesystem. Destruction of that filesystem, externally removed state and a
device that violates sync semantics are outside this recovery guarantee.

`Flush` settles the admission prefix captured at entry and fails while
unacknowledged missing-fact evidence remains. Retrying the exact retained facts
can settle a transient write failure. An ambiguous append/fsync failure can be
repaired in-process from the successfully synced admitted prefix after unknown
evidence has been persisted. That repair never clears the gap. Import retries
reuse already-pending immutable identities instead of exhausting lifecycle reserve. Unrelated writes, a new counter baseline
and reopening cannot settle missing facts. `Healthy` and telemetry use current
obligations and persistent gaps, not the historical failed-transaction count.
Migration authority is retained separately from completeness.

Retained event reads use `Settle` and remain available after a recovered crash
or acknowledged/unacknowledged historical gap, and still return readable rows
when settlement is deferred by a current write fault. Their existing array response
carries `X-Pulse-History-Complete` and `X-Pulse-History-Gap-Revision`; reads never
clear missing-fact evidence.

Monitoring-read `GET /api/alerts/history/durability` exposes tenant-scoped
counts, gap revision, unknown-gap state and bounded acknowledgement metadata.
It hides filesystem error paths. Monitoring-write
`POST /api/alerts/history/durability/acknowledgement` requires the observed
revision and a reason of 1 to 1,024 bytes. It refuses stale revisions, pending
facts and current storage errors. Acknowledgement accepts missing facts and
retains its cumulative loss count, last time and reason. It neither claims
recovery nor deletes pending events. No automatic expiry clears gaps.

## Verification scope

Regression source includes real writer-connection `PRAGMA query_only` faults,
real SQLite trigger faults through the system-alert producer, actual descriptor
and unavailable-directory failures, reopen and native process-kill boundaries.
The process tests stop after journal sync and after database commit but before
journal compaction. They verify original times, one event and one active-state
transaction, and conservative unknown-tail health. Other regressions cover the
old buffer limit, bounded admission, corruption/quarantine, journal and gap
persistence failure together, retired JSON history, callback counts and scoped
HTTP admission, partial snapshots over compacted canonical facts, retirement
during unreadable-target runs, delayed controls after resolution, readable legacy
schemas during a read-only upgrade, pruned history with retained active rows,
partial observation/control separation, partial first baselines and legacy
checkpoint migration, guest identity migration across partial
recovery, invalid null snapshots, constrained startup writes, cleanup refusal, open-session metadata
retry, replacement/clear mirror reset, capture hash mismatch and cleanup absence
through unreadable-target restarts. Callback counts are not provider-delivery evidence.
First-checkpoint trigger faults also verify recovery from an initialized-only
projection. Unreadable first baselines recover empty and nonempty unions,
including pruned active occurrences and controls newer than retained events.

Candidate verification and protected source landing must bind these tests to
the final source. Physical power-cut, installed-service shutdown, actual
provider delivery, fleet recovery and release qualification remain separate
boundaries.


## Verified candidate evidence

The final candidate's 20 changed Go files match the retained SHA256 manifest
from `history-final-verification9-20261105`, based on source
`35f0a31074ed94d8803918e6c780a559b790a284`. Every reviewed Go context
file and all 20 changed Go files match that source byte-for-byte. Through the normal shared native
validation gate on Linux with Go 1.26.10, the focused fault/restart regressions,
full `go test ./internal/alerts/... ./internal/api/alerting`, targeted durability
races and Windows amd64 eventlog test-binary compilation all completed with
exit status 0. The Windows binary was compiled, not executed.

The complete command output and per-file hashes are retained locally under
`/Volumes/Development/.caches/pulse-reliability-followups-20261105/evidence/history-final-verification9/`.
Earlier failures and superseded snapshots remain separately labelled. The
nested-launcher attempts executed no tests and supply no qualification evidence.
This is source evidence for the named paths, not a release or production cut.
