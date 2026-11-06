# Notification cooldown and recovery continuity, 5 November 2026

This is source repair evidence for coverage gap
`notification-occurrence-delivery-state-continuity`, owned by notifications and
alerts in L6. It does not close L6, qualify a release, establish installed-provider
acceptance or resolve a reported notification outage.

## Reproduced boundary

The baseline was `c85a480c40bb7dd920503cbdade15a3db43d3168`. A persistent queue
and owned loopback HTTP receiver reproduced two separate defects:

- After one accepted critical firing and graceful restart, restored monitoring
  sent the same occurrence again inside its 30-minute cooldown. Both an in-process
  monitor replacement and separate native processes observed two HTTP deliveries.
- Aging only a successful receipt to 31 days and invoking real queue cleanup
  removed it while the occurrence remained active. Subsequent recovery reached
  no receiver. The one-day control retained the receipt and delivered recovery.

The earlier evidence record is
[Alert persistence reliability](alert-persistence-reliability-2026-11-05.md).
Retained baseline output excerpts are incomplete logs, not a claim that an entire
baseline suite passed. The regression sources below retain the connected
producer/queue/HTTP boundaries so the behavior can be checked against source.

The basic cooldown/recovery fix in PR #3948 already precedes the current
integration baseline `cf9354876e0482a8edc8386557e17e06a99c3aec`. Its connected
retention and restart tests remain in this candidate. The broader repair extends
that landed schema and preserves its nanosecond completion and resolved evidence.
History lifecycle authority uses the canonical `Store.Healthy()` seam from PR #3974.

## Ownership and resulting behavior

Successful receipts retain exact occurrence identity, destination generation,
highest delivered severity and nondecreasing completion evidence. Ordinary
callbacks read SQLite delivery authority after restart. A severity
increase, new occurrence or newly configured recipient remains deliverable.
Scheduled escalation retains its existing cadence. A legacy receipt's severity
is unknown, permitting its first known severity rather than inventing history.

The queue's nullable generation field is independent of immutable provider
configuration bytes. Migration fixes already-admitted legacy recovery to generation
zero. Firing binds before its first provider attempt. Quiet-hours partitions copy
the field, and recovery grouping includes it in its merge identity. An old recovery
cannot consume a later generation's receipt.

Explicit recipient rotation, removal or disable retires the old generation.
An exact revert does not resurrect it, even when an old request completes late.
Already-admitted snapshots retain their original receiver under existing delivery
policy. Already-admitted recovery pins its generation until delivery, policy
cancellation, terminal retention or operator disposition. Same-start refire cancels
delayed recovery before dispatch. Positive resolution remains until a newer
successful firing replaces it. Global pause does not retire recipient identity.

Active and unavailable current-state authority retain receipts. A complete current
inventory can settle an absent occurrence only with an aged explicit receipt resolution,
even if historical health remains degraded. Positive resolution
supplies a 30-day settlement window and observed history resolution persists
before that history can compact. Healthy compacted history can use a retained
explicit receipt resolution only without a later activation. Unknown legacy
obligations are exceptional holds until positive lifecycle evidence or explicit
destination retirement. Missing, unreadable and shutting-down data never establishes
inactivity. Cleanup shares lifecycle lookup per occurrence and builds recovery pins
once per pass, then deletes with completion/generation/disposition revision checks.

Canonical monitor construction preloads the complete saved destination inventory
before creating live callback/API owners or workers. Read failure holds notification delivery and fences destination API reads,
writes and tests before persistence while monitoring and alert detection continue.
Validated full backup import can restore saved settings while held, returning a
saved-but-restart-required response when live reload has not already reconstructed
a complete notification owner. Independent sections apply normally, so later
settings edits preserve the imported state. Reconstruction after restart applies the
restored notification inventory. Notification storage preparation and reconciliation errors also hold delivery
without stopping detection; deferred bootstrap never falls back to volatile delivery. Live retirement completes before
publishing a new configuration; failure keeps the previous live state and HTTP
handlers restore the previous saved state. Unchanged webhook imports preserve
generation and changed/removed receivers retire atomically; Apprise also reloads.

Local generation/receipt bookkeeping errors keep work pending on a 30-second
cadence, without consuming provider attempts or inventing a provider outcome audit.
Unsettled storage reports unavailable delivery health. The existing successful
provider audit is the durable acknowledgement. After it commits, pending work
retries local receipt/status settlement without resending, and retains that audit
past ordinary retention. Accepted scalar occurrence facts commit atomically with the successful audit,
independently of mutable queued configuration or alert JSON. Indexed exact facts
carry resolution and recovery disposition, and durable audit order prevents late
settlement from overwriting newer firing authority. Accepted-sending repair
excludes workers that still own a live attempt. A crash before acknowledgement commits can
repeat a send. If acknowledgement storage itself fails, ordinary bounded provider
retries apply. No exactly-once provider guarantee is made.

The four-column receipt primary key is a forward database migration. Older
binaries cannot use their previous receipt upsert against it. Downgrade requires
restoring the prior database backup rather than only replacing the executable.

## Regression boundaries

- `internal/notifications/delivery_continuity_test.go`: real HTTP and separate
  native restart phases, severity/new-occurrence/destination controls, schema and
  immutable-byte migration, recovery generation grouping, monotonic completions,
  future-clock normalization, cache-miss shutdown and abrupt exit after actual
  provider acknowledgement but before receipt commit, plus process death after durable audit/facts commit
  and before receipt projection without another provider call.
- `internal/notifications/delivery_retention_continuity_test.go`: aged receipts,
  active/unavailable/resolved/compacted lifecycle, recovery pins and corrupt pending
  data, live/offline rotation and revert, same-URL recipient/account changes,
  legacy and current in-flight generations, retirement/receipt/generation SQL
  faults, mixed-destination checkpoint and same-start delayed-recovery cancellation.
- `internal/alerts/notification_lifecycle_test.go`: exact live/public occurrence
  identity, healthy history, refire, explicit imported state, malformed snapshots,
  incomplete authority and closed/shutting-down owner.
- `internal/notifications/delivery_settlement_test.go`: acknowledged firing and
  recovery projection faults, same-generation durable ordering, final-status
  failure, subsecond lifecycle order, malformed destination isolation, reserved
  metadata collision and unchanged legacy anonymous inventory restoration.
- `internal/monitoring/notification_continuity_integration_test.go`: monitoring
  callbacks, persisted active state, native process restart and actual HTTP inside
  cooldown, plus corrupt saved inventory delivery hold, continued alert detection, write
  fencing and corrected recovery replay.
- Notification HTTP controls cover failure/rollback/retry acknowledgement, and
  configuration import retains its saved-but-incomplete-live-reload response.
- The shared frontend warning card and destination editing integration tests
  preserve accepted mutations and drafts through unavailable follow-up reads,
  offering review/refresh until degraded health can be verified again.

## Callback boundary outside this repair

The accepted receipt and already-pending recovery floor does not establish a
durable detector-to-notifier outbox. Source inspection of alerts
`safeCallResolvedAlertCallback` and monitoring callback adapters shows asynchronous
work can still be outside notifier admission when shutdown begins. Closing that
boundary requires a shared callback admission/drain protocol across owners,
including independent detector/finding lifecycle ownership, reentrant callback
ordering, delayed resolution after same-start refire,
and hold-period firings and resolutions (including noncritical incidents still active
after restart). Ambiguous legacy receiver holds can likewise leave active warnings
without a recovery unless a new firing establishes the current receiver. Producers
without matching alerts lifecycle authority (for example Patrol findings) remain
admissible for fresh firings; their receipt obligations remain unknown until positive
resolution or explicit receiver retirement. This class is unbounded for
Patrol: receipts accumulate with historical findings per destination, regressing
main's 30-day age expiry. Hourly per-occurrence active scans/history walks also grow
without bound. Producer tags/owner identity and owner-aware expiry are required
follow-up work. Fresh unstamped alerts-manager firings can likewise replay after
resolution missed notifier admission at shutdown; the shared producer follow-up
must restore safe stale rejection without dropping independent findings. New notification
callbacks are not queued while saved inventory is unreadable. It must admit recovery in the same lease
as resolution cancellation, or re-admit retained resolved, unconsumed receipts
without a pending/terminal recovery at startup under resolve and quiet-hours policy.
Operator-dismissed recoveries must never be resurrected. It is recorded separately as discovered
coverage gap `notification-callback-shutdown-admission`; no installed loss or
executable reproduction of that separate boundary is claimed here.

A no-backup audited discard/replacement flow for unreadable saved inventory is
separately discovered as `notification-unreadable-inventory-disposition`. Current
recovery is validated full backup import with live reconstruction or a required
restart, or filesystem repair and restart. A read error never
implicitly authorises deletion or replacement of existing destinations.

## Verification

The 6 November CodeQL review traced both `go/weak-sensitive-data-hashing` alerts
to decrypted configuration-backup content flowing into webhook identity fields.
The import passphrase is consumed by PBKDF2 and AES-GCM; it does not enter either
receiver digest. The SHA-256 sinks identify receiver content for generation and
receipt joins, rather than storing or verifying a password. The two specific
alerts therefore require a false-positive disposition, without disabling the rule.
Review also found one receipt-projection error log using the internal receiver key.
It now uses the canonical public delivery-log destination ID; the real SQLite
projection-fault regression checks that correlation remains available while the
internal digest and credential stay absent. Acknowledged pending queue records
retain their existing plaintext transport snapshots inside private queue storage
until settlement; the digests do not replace credential encryption or qualify
installed storage security.
The logging-only follow-up passed the confidentiality, receiver-fingerprint and
receipt-commit-fault controls under the race detector (10.460 seconds). It changes
no delivery, receiver identity or migration behavior; the protected checks still
requalify the updated commit.

The native grouped-warning control collects real `CheckHost` callback snapshots,
persists and drains the producer, then admits those snapshots through public
notifier methods before actual SIGTERM. This keeps producer journal writes out
of the four-second grouping window. The restart controls positively require two
checkpointed pending rows with the same original persisted deadline; the reload
control checks continuity, payloads and timing. The separate monitor constructor
control retains a 600-second pending warning across reconstruction. These controls
do not close the shared producer-admission/drain gap.

A retained SIGQUIT stack caught cold-start waiting in `syscall.Fsync` during
idempotent legacy alert-history import. Native ready/reloaded phases therefore
allow 30 seconds; command phases retain 15 seconds and each child retains its
45-second total timeout. Provider receive, durable-completion and persisted
no-early-send assertions remain independent. This is not startup-performance or
physical-storage qualification.

Validation ran in the owned Linux checkout through the normal shared worker gate.
The final integration baseline is `1e552302e74336ac7547906fcb284933bfcbd5a5`.
The following source checks passed:

- Complete notifications suite: 295.684 seconds; native grouped-process race
  control: 38.272 seconds, including restart after the persisted deadline.
- Complete monitoring suite: 772.136 seconds, including the corrected saved-policy
  and buffered-warning reconstruction controls. Alert and monitoring race controls
  also passed; the complete alerts suite passed on the preceding source snapshot.
- Earlier containing alerting API, configuration API and config suites passed.
  Current masked-destination publication and delivery controls passed on the
  integration baseline (12.350 seconds); the exact-head protected API check
  remains a separate source-landing requirement.
- The relevant frontend suite passed all 114 files and 2,437 tests. Type checking,
  production frontend build and backend build passed.
- Actual Chromium views at 1440×1000 and 390×844 passed on Notifications and
  Overview. Inspected states include saving, real SQLite publication failure,
  retained draft and previous receiver, keyboard retry, persisted reload, webhook
  recovery warning, storage-only unavailability and mixed retained failures.
  Unreadable inventory preserved its bytes and refused settings reads/writes;
  validated full backup import reconstructed the live owner with HTTP 200, and
  another process restart retained healthy notification storage. The receipt
  `frontend-modern/browser-verification/40c63114b5a00aa6.json` binds all six
  changed frontend runtime files to those inspected views.

The unchanged-main writer capture is from
`cf9354876e0482a8edc8386557e17e06a99c3aec`; its fixture SHA-256 is
`e99770f17822cd710585db29f40d8a6dcd52774bbd0557747c797a36cc75099f`.
It preserves actual resolved-second precision, nanosecond completion evidence and
immutable queued configuration before candidate migration. Exact tested source
snapshots and full output were retained separately. The final notifications
snapshot archive SHA-256 is
`b6ee20e5a538face500b06cad8c186eff908131dc444cf3410955e9ab2cfc64e`;
the containing monitoring snapshot is
`d18c4ba1e5e132deb79dec98645689e22f6cb277c51f2cfd19a428c50d277bae`.
Their production Go files are identical; the later native fixture correction is
confined to test setup. Read-only review was repeated after material corrections
and left no outstanding material finding within this accepted-receipt and
already-admitted-recovery floor.

This closes coverage gap `notification-occurrence-delivery-state-continuity`.
The two separately discovered gaps above remain active. L6 remains partial with
bounded residual completion; lane scores, readiness assertions and release gates
are unchanged. Protected source landing still requires the required checks for the
exact commit. This record does not predeclare their result or release qualification.
