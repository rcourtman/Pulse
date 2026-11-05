# Alert persistence and delivery continuity evidence

Recorded 5 November 2026. These are unresolved reliability findings. The
durable-ID projection replay repair does not repair these storage or delivery
obligations, and this record does not close a lane or qualify a release.

## Sources and boundaries

The history fault experiment used source
`8c5dfe161870090d7ce5ef9101e49c11d1a70e01` with the ID-ordered replay candidate.
The notification experiment used
`f1f2480e869f39e776e5bd93611ad89e3ec6a534`. Both used isolated temporary data.
The retained source files below are documentation artifacts. They are excluded
from ordinary Go test discovery and introduce no runtime fault switch.

History faults use SQLite `PRAGMA query_only` on the real serialized writer
connection. They are driver write faults, not physical disk-full or crash-kill
qualification. The manager-level experiment counts delivery callbacks. It does
not establish successful provider delivery. The notification experiments use a
real loopback HTTP receiver and reopened persistent queues.

An independent Linux repeat at integrated source
`ea43f1a05ad690ddcf6f7a1ec3db05e8946005e3`, as ordinary UID 1000 with
Go 1.26.10, reproduced all six history diagnostics and both notification
findings. The 40-event loss, 500 buffer drops and healthy reopen with missing
facts remained. The notification continuity assertions failed on their actual
delivery counts; the one-day receipt control passed. These are retained
negative experiments, separate from passing source qualification.

## Event-history loss survives while its evidence disappears

The store and producer experiments observed:

- A failed background transaction lost 40 accepted events. Later successful
  writes did not restore them. `Flush` kept failing in process, but reopening
  made it succeed while those events were still absent.
- A full diagnostic buffer dropped 500 events while `Flush` returned success.
- A failed synchronous durable append poisoned later background flushes even
  when every background event had been written. Import failures used different
  accounting.
- A store-level append after close was counted but not written. A production
  producer race reaching that state was not established.
- A real system-alert producer under a write fault marked history degraded in
  process. A later JSON-history import recovered a final occurrence snapshot,
  while the exact fired transition and notification decision remained absent.
- After JSON history had already been imported and retired, a whole occurrence
  could disappear across restart. Its producer delivery callback ran, but later
  boots had no history for it and reported healthy event history.

The current counter is therefore neither a complete loss inventory nor a
durable health record. Resetting it after a successful unrelated write, taking
a fresh counter snapshot for each flush, or treating reopen as recovery would
conceal missing facts.

The alerts owner needs a durable account of unsettled history obligations.
The design must decide whether failed lifecycle facts are journalled for a
later commit with their original occurrence times, how unrecoverable gaps are
reported, when gaps expire or are acknowledged, and how history-health
telemetry reflects them. A recovery journal must preserve fail-open alert
evaluation and cannot rely on writing synchronously to the database that has
just failed. ID-ordered projection replay can consume facts committed late,
but it cannot reconstruct a fact that was never retained.

## Notification state is not continuous across restart and cleanup

Two independent delivery experiments observed:

- An already delivered, active, unacknowledged critical occurrence was delivered
  again after a graceful reopen inside its 30-minute cooldown. The duplicate
  also reproduced across two separate native subprocesses. The firing receipt
  was durable, but the reusable cooldown map started empty. Recovery and a
  genuinely new occurrence remained deliverable controls.
- Backdating only a firing receipt to 31 days, then running the real cleanup,
  deleted that receipt while the occurrence was still active. After reopen,
  the destination received no recovery. A one-day-old receipt control retained
  the receipt and delivered recovery. This simulates age, not 31 days of
  elapsed runtime.

Cooldown should be a cache over occurrence-bound delivery evidence, including
the highest delivered severity and monotonic completion time. A cache-miss
read must use the notification manager's existing admission lease so shutdown
cannot close the queue under it. Legacy receipts have no severity history, so
migration must preserve delivery of genuinely new severity information.

Retention additionally needs an explicit destination-retirement policy.
Keeping receipts for every active occurrence is not bounded by current
destination count because URL, recipient and target rotations create distinct
identities. Deleting by firing age alone loses owed recoveries. A missing,
unreadable or shutting-down lifecycle source is not proof that an occurrence
ended. Cleanup must preserve uncertain obligations rather than silently
falling back to age-only deletion. Pending recovery jobs must retain their
receipts. Whether removal of a destination retires its old obligations, and
what an exact later configuration revert should do, remains a product
decision.

## Retained executable experiments

All files are in
`alert-persistence-reliability-2026-11-05-proofs/` beside this record.

| File | SHA-256 |
| --- | --- |
| `writer_fault_overlay.go.txt` | `fdd009ffeb8b34368087dbe8c479596b46df506f255e51cb78547a908f9656f3` |
| `eventlog_loss_proofs_test.go.txt` | `5b7904dc6ec24dee310a0db0d664e731ab551d734e51ee009f77671ca8cc94e6` |
| `lifecycle_loss_proofs_test.go.txt` | `9f630f3f0c7cad475242fd198d394c87fb6c8cf272c8a187aabfa9338fa24d17` |
| `restart_cooldown_proof_test.go.txt` | `3b874fc166fa53f5d84d7dcb5a8ab5409f358b24c26eaa78748fe4eb4be829b7` |
| `receipt_retention_proof_test.go.txt` | `9e26e2386f9803f831cbd5c4da21b17bba5fc6abe5d851de7d7687a50abfcc48` |

Run only in an owned source checkout and through the normal Linux validation
gate. Map the text files into these virtual paths with Go's `-overlay` option:

| Artifact | Virtual path |
| --- | --- |
| `writer_fault_overlay.go.txt` | `internal/alerts/eventlog/zz_audit_fault_overlay.go` |
| `eventlog_loss_proofs_test.go.txt` | `internal/alerts/eventlog/zz_audit_loss_proofs_test.go` |
| `lifecycle_loss_proofs_test.go.txt` | `internal/alerts/zz_audit_lifecycle_loss_proofs_test.go` |
| `restart_cooldown_proof_test.go.txt` | `internal/monitoring/zz_restart_cooldown_proof_test.go` |
| `receipt_retention_proof_test.go.txt` | `internal/notifications/zz_receipt_retention_proof_test.go` |

The first three are diagnostic experiments. Their completion is not a repaired
verdict. In particular, P5 and P6 log authority, retained events and telemetry
across reopen for inspection. The notification proofs assert the desired
continuity guarantee and fail on the observed defective source. Preserve their
negative result and successful control cases separately from ordinary source
qualification. None establishes signal-driven shutdown, crash recovery,
installed-service behavior, a real notification provider or production-fleet
recovery.
