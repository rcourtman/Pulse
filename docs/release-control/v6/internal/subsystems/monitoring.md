# Monitoring Contract

## Contract Metadata

```json
{
  "subsystem_id": "monitoring",
  "lane": "L13",
  "contract_file": "docs/release-control/v6/internal/subsystems/monitoring.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "unified-resources"
  ]
}
```

## Purpose

### PBS live-state slots belong to selected groups, not cache or response order

The existing 5,000-row per-connection window reserves slots in descending
last-backup group order within each enumerated datastore/namespace. Cache hits
and refreshes consume the same budget; a cache hit cannot reset outstanding
reservations. Selected groups publish in that order, each with its newest real
snapshots, regardless of worker response order. Older cached rows cannot crowd
out a newer refresh, and unselected older groups are not fetched to fill a
failed or denied selected group's slots. Configured datastore/namespace traversal,
the 100-snapshot group cap and five-reader HTTP ceiling remain unchanged.

A group crossing the live-state boundary, or outgrowing its listed count during
the snapshot read, makes collection partial rather than authorising deletion of
unseen durable history. A clipped payload expires that exact group's cache so a
still-stale count cannot certify completeness on the next poll. Failed refreshes
retain their original age; access rejection withdraws raw rows without renewing
them. Complete ordinary recovery resumes cache reuse, and authoritative empty
history still reconciles deletion.

`pbs_live_selection_test.go` joins real HTTP reads at the actual cap to raw/cache
publication, guest backup age, recovery mapping, stored history and protection,
including mixed reuse/refresh, transient and denied reads and ordinary recovery.
Its complete emitted frames feed the production Backups surface through
`scripts/check-pbs-live-selection-browser.cjs` at desktop and narrow viewports.
These are source/consumer proofs, not installed PBS, restore, guest thaw or
filesystem-liveness acceptance. No limit, permission, API field, guest operation
or native-recovery prerequisite is added.


### Running scheduled backups cannot qualify unidentified archive output

An active vzdump parent without a positive guest ID cannot identify every file
it may be writing. Its log is bounded and may be unreadable, deferred behind the
existing 25-log budget, or a valid prefix ending after an earlier guest. The same
connection's archive pass retains its exact previous inventory and age until a
live read establishes that the parent has ended. Positive parent/guest task
observations and independent guest snapshots continue. A later history verdict
cannot retroactively qualify the earlier live writer scope. Individual-guest
active tasks retain ordinary partial-file classification; terminal parents and
complete-empty reads resume ordinary archive collection without requiring a
successful log or broader permissions.

This deliberately delays newly completed sibling archives until the scheduled
parent ends, rather than guessing an incomplete file's guest or reporting a false
restore point. Other connections and direct PBS collection are unchanged. No
new field, request, log budget, guest operation or QGA admission is introduced.

`pve_backup_unscoped_writer_test.go` joins both real HTTP adapters to the ordinary
scan, exact archive retention, Last Backup, recovery mapping, bounded log reads,
positive sibling evidence and ordinary scoped/finished/empty recovery. Its emitted
frames feed the production Backups component at desktop and narrow viewports via
`scripts/check-pve-unscoped-backup-writer.cjs`. These are synthetic source/consumer
proofs, not installed, restore, thaw or filesystem-liveness acceptance.


### PVE archive refresh requires the same scan's live task read

A vzdump file is listed under its final name while still being written. If the
live task inventory fails, retained tasks and successful history backfill cannot
rule out a new writer. The ordinary scan therefore keeps its previous archive
inventory, PBS visibility confirmations and protection collection age instead
of enumerating/reconciling new archives or advancing Last Backup. Independent
guest snapshot polling continues with its existing budget. A later successful
live task read resumes ordinary partial-file classification, finished archives
and complete-empty removal. Other connections and direct PBS polling are unchanged.

A successful task response alone is insufficient: without confirmed `Sys.Audit`
at each online node it may list only the caller's own tasks. The scoped inventory
keeps those positive rows in the existing bounded live view, including ordinary
guest-log expansion, but the archive pass retains its previous inventory and age.
It does not request broader grants or discard observed tasks on a failed permission
confirmation. When the recent 200-task page fills, a separate bounded `source=active`
read must finish below its cap before absence can qualify archive output. Active
rows precede and supersede overlapping recent rows so an older writer survives the
live projection's 200-parent cap. A failed, malformed, foreign-node or full active
page cannot qualify the pass; a short, audited active census lets busy nodes
progress without unbounded history scanning. Ordinary confirmed reads recover
Running, Finished and complete-empty states. These reads are not an atomic
task/storage snapshot or a guarantee of multi-guest log attribution.

`pve_backup_task_barrier_test.go` exercises both legacy/scoped adapters through
the complete scan, unchanged archives, backup ages, repeated failure, running,
finished and empty recovery. Its emitted snapshots feed the production Backups
component using `scripts/check-pve-backup-task-barrier.cjs` at desktop and narrow
viewports. These are closed source/consumer fixtures, not guest, thaw, filesystem
liveness, restore, installed or reporter recovery proof. No provider mutation,
new guest command, broader permission or new API field is introduced.

`TestPVEArchiveScanRequiresConfirmedTaskVisibility` additionally joins both real
HTTP adapters, own-only and bounded/failed active views, archive preservation,
Last Backup, recovery mapping, positive task caps and independent snapshots.
The companion SDK controls cover permission ambiguity, overlap and an older
writer outside the newest-200 window; the observed-parent control keeps log
expansion useful without full audit visibility. No installed recovery follows
from these fixtures.

### Bounded backup-log reads make progress past unfinished jobs

The existing 25-log per-poll budget serves never/least-recently attempted
multi-guest jobs before retrying others. Failed, empty, ambiguous or running
logs cannot monopolise every cycle and hide later guests forever in a stable
backlog. Request order follows private attempt metadata; published tasks retain
provider order. Failed/deferred reads retain their original guest evidence and
observation time. Offline scopes, immutable qualified results, connection/runtime
withdrawal and cache pruning retain their existing boundaries. Attempt metadata
is not guest evidence, report completeness or a restore artifact.

`pve_backup_log_budget_test.go` covers sustained unfinished logs, reordered
inventories, child cancellation, scope/retention and ordinary HTTP backlog
recovery through state/cache and reporting reducers. The provider request cap,
API shape and QGA/guest operations are unchanged.

### Per-guest backup success requires matching section markers

A multi-guest vzdump finish must match exactly one active start for that guest.
Missing, repeated, reversed or overlapping markers cannot invent guest success,
erase an explicit failure, or seal an immutable log cache. Affected results use
existing Unknown/detail fields; well-formed siblings retain their own outcomes.
An ordinary corrected read replaces the uncertain result and may qualify the
finished-job cache. Failed reads retain the original observation time. Reports
keep ambiguous attribution partial and do not advance that guest's last-success.
No task result creates a restore artifact or proves guest/filesystem recovery.

`pve_backup_log_attribution_test.go` exercises marker ordering, failure precedence,
HTTP polling, cache/read failure, corrected reads and reporting consumers. This
changes no provider request budget, QGA admission, guest operation or API shape.

### Backup inventory readers do not clone unrelated estate state

PVE archive collection reads running tasks and held archives together through
the existing detached primary PVE-backup projection. PBS cache construction
uses the existing primary PBS-backup projection, including its detached files
and verification fields. Neither metadata-only read copies resource inventory
or consumes the separately cached backup aggregate. Full-state reads needed for
canonical fallback rebuilding and guest backup-time synchronisation remain.

`TestBackupReadProjectionDoesNotScaleWithUnrelatedEstate` holds canonical guests
and backup inputs fixed while unrelated estate metadata grows. The companion
cache/collector controls pin independent connections/namespaces, primary fields,
newest-first limits, detached mutation, unread-node archive retention and running
task/partial-file correlation. Existing HTTP/lifecycle tests retain cancellation,
replacement and ordinary recovery coverage. Fixed old/new benchmarks measure
source allocation cost, not installed CPU/RSS relief or frozen-guest recovery.
Request scope, budgets, publication fences, evidence decisions and served fields
are unchanged.

### Guest snapshot input reads avoid unrelated payload copies

Snapshot polling retains typed canonical VM/container views for identity, name,
tags, template and container-type lookups instead of materialising complete guest
models with unused disks, interfaces and metrics. Prior snapshot inventory comes
from the existing detached primary PVE-backup projection, not a second whole-estate
snapshot. Canonical/mock selection and fallback rebuilding remain unchanged.
Request targets/order, snapshot fields, tag suppression, names, budgets, worker
bounds, unread-evidence retention and alert/recovery/publication fences retain
their existing meaning. No API, resource field or rendered state changes.

`TestGuestSnapshotProjectionDoesNotCopyUnconsumedPayload` varies only unrelated
Kubernetes metadata or unused guest disk/interface data on fixed canonical inputs;
`TestGuestSnapshotProjectionPreservesAlertIdentityAndScope` checks ordinary
requests, exact published fields, templates, independent connections and canonical
alert names/tag exclusions. Fixed-sample collector benchmarks measure source
allocation cost, not installed RSS, incident cause or native guest recovery.

### Guest snapshot polling bounds workers, not just requests

Snapshot inventory uses at most eight reusable workers per existing connection
scan, with no worker for an empty fleet. Waiting guests do not each retain a
goroutine stack. The existing VM/LXC selection, template exclusions, request
limit and bounded scan budget remain unchanged. Cancellation stops queued reads
and still waits for active readers to actually return before releasing the scan.
Completed per-guest observations remain usable; failed and undispatched guests
retain prior inventory rather than becoming successful emptiness. Existing size
uncertainty, alert/recovery semantics and connection/runtime publication fences
remain in force. No resource field or rendered state is added or reinterpreted.

`guest_snapshot_workers_test.go` counts actual collector goroutine frames under
gated mixed-fleet reads, checks scoped inventory and actual-return cancellation,
and measures fixed-sample allocation cost. These are source resource/evidence
proofs, not installed RSS, incident diagnosis or frozen-guest recovery.

### Storage content is shared evidence, not a backup-only list

The validated Proxmox content reader returns every content kind to its existing
consumers. Only `backup` entries may create storage-backup records or successful
backup evidence. Container root usage accepts live `rootdir`/`images` volumes,
never snapshot (`@`) or archive rows. Whole-response integrity, connection scope,
ordinary read budgets and guest-operation exclusions are unchanged.

Snapshot size is a complete sum, not a lower bound from whichever disk responded.
Each node stages totals until all eligible storage reads succeed; inactive/unread
storage makes that node's size unknown without blocking independently readable
nodes. Missing/zero or overflowing matching volume sizes cannot establish a
smaller total. Retained snapshot inventory does not renew an old size. Existing
alert evidence keeps the last known breach, identity, acknowledgement and original
size-observation time until a complete smaller reading or authoritative snapshot
removal establishes recovery. No new route or settings surface is added.

`TestStorageContentRetainsAllValidatedKinds` exercises single and cluster clients;
`TestSnapshotStorageTotalsRequireCompleteNode` covers incomplete totals and peer
independence. `TestStorageContentConsumersKeepSnapshotAndLiveRootEvidenceSeparate`
joins real HTTP, the ordinary container/archive/snapshot collectors, collected
state, recovery points and alert hold/recovery. Desktop/narrow browser proof uses
these actual projections in existing Backups, workload and alert components.
These fixtures do not establish appliance acceptance or frozen-guest recovery.


### Root PBS datastore discovery owns backup replacement authority

The foreground datastore-list result travels into the same connection/runtime-owned
background scan as its namespace inputs. Admission checks the runtime epoch captured
before discovery, even when replacement reuses an uncancelled parent context. A transient or malformed root discovery
publishes unavailable protection evidence while retaining exact raw backups, cache
read times and stored recovery points; it probes no cached datastore path. An
access-rejected discovery withdraws only that connection's raw rows and reusable
cache, retaining historical points with denied/unavailable evidence. Failed discovery
does not re-observe those points or acquire reconciliation authority.

A complete empty datastore array is an authoritative inventory and can clear that
connection's raw/cache/recovery scope. Datastore/backups/global scope-off and cancelled
foreground polls are not successful emptiness. The existing cadence, actual-return
occupancy, publication fence, state limits, exclusions and guest/backup operation
policy remain effective. Before filtering or detail requests, root list admission
requires a complete bounded HTTP 200 array with unique nonblank datastore identities;
missing/null, conflicting, truncated or unreadable replies are not empty evidence.

`TestPBSDatastoreInventoryAdmission` covers client admission and closure;
`TestPBSRootDatastoreDiscoveryRegression` joins ordinary HTTP polling, raw/cache,
durable points and protection through unavailable/denied/empty/malformed discovery,
removed-store and independent-connection controls, scope-off, cancellation and
ordinary recovery. The existing Backups consumer is checked at desktop/narrow widths.
These are source/consumer proofs, not installed backup/restore, independent guest
thaw, filesystem writes/liveness or prior-active restoration.

### PBS backup scans belong to their registered connection and runtime

Background PBS inventory admits the registered client and current runtime
atomically with cadence. Runtime replacement, client replacement and failed-node
retirement cancel the old reader and fence its state, cache, recovery/alert and
broadcast admission. A cancelled scan retains occupancy until actual return;
settings resets cannot admit overlapping scans, and late return cannot postpone
the next ordinary scan. Network reads stay outside the publication lock.

Previously accepted rows can remain last known. A new connection cannot reuse
the previous connection's snapshot-cache freshness even if group counts/times
match. The same connection and independent siblings keep ordinary cache reuse.
PBS namespace/time/size/verification admission, existing request budgets and
partial-read preservation are unchanged; no backup or guest command is added.

`TestPBSBackupLifecycleHTTP` exercises the ordinary foreground/background path
with real synthetic group, snapshot and writer HTTP through runtime/client
withdrawal, failed-node removal and ordinary resumption. The
`TestPBSBackupLifecycle*` ownership controls pin actual-return occupancy,
concurrent admission, publication ordering, ended-runtime rejection and scoped
cache replacement. The existing By date/Coverage surface consumes emitted rows
at desktop/narrow widths. These proofs do not establish native backup/restore,
independent guest thaw, filesystem liveness or installed recovery.

### PVE task pages require unambiguous execution identity

Every task row needs a nonblank opaque UPID and task type, with no repeated
UPID within a page. A missing/null/blank identity or duplicate makes the whole
page unavailable before filtering, live replacement or History reconciliation.
It cannot silently drop a failed guest task, publish an OK prefix or establish
complete History. Earlier valid History pages and independently readable nodes
remain available; malformed live reads retain their earlier observations and
original observation times. Ordinary complete reads can recover normally.

Scheduled parents may still have an empty guest ID; running tasks may omit their
result/end time. Omitted redundant node fields, explicit other task kinds,
additive metadata, genuine empty arrays and cross-page History deduplication
retain their existing meaning. No UPID format constraint, guest/backup command,
request budget or privilege is added.

`TestTaskListsRequireUnambiguousIdentity` covers all three HTTP task-list consumers;
`TestTaskIdentityPreservesSupportedOptionalFields` and
`TestBackupHistoryIdentityGapKeepsEarlierPagesAndPeer` pin compatibility and scope.
`TestPVEBackupTaskIdentityGapCannotEraseFailure` joins ordinary direct/cluster
polling to the served backup projection. Desktop/narrow Coverage proof consumes
those actual emitted values, not invented verdicts. Persistence is unavailable
in the fixture: these are evidence-integrity checks, not durable report storage,
installed backup/restore, independent thaw or filesystem liveness.

### PVE job logs retain their numbered text sequence

The bounded task-log read starts at line zero and admits only a contiguous
one-based sequence with present, non-null text. Missing, duplicate, reordered
or gapped line identities and missing/null text are unavailable observations,
not discarded log lines that can leave a false per-guest success. Empty logs,
blank text and additive metadata remain valid; the existing body/line bounds,
credential scope and no-guest-command behaviour are unchanged.

An unreadable finished log cannot finalise the per-guest cache or renew the
last successful running observation. An ordinary complete reread can publish
the actual failure, then cache that immutable verdict; a later successful job
can recover normally. `TestTaskLogRequiresNumberedTextSequence` and
`TestPVELogSequenceRejectsFalseCompletionAndResumes` join wire admission,
polling, cache and served snapshot. The existing Coverage view consumes those
snapshots at desktop/narrow widths. This is truthful task evidence, not native
backup completion, guest thaw, filesystem liveness or installed recovery.

### Infrastructure plaintext destination confidentiality (16 October 2026)

PVE/PBS/PMG's shared HTTP client does not send explicit plaintext infrastructure
requests through inherited environment proxies, even for public-style hosts.
The configured endpoint receives the existing token, ticket/CSRF or JSON/form
password request directly. HTTPS CONNECT/authenticated proxying, `NO_PROXY`,
local-target bypass and all CA/pin/explicit-opt-out verification stay unchanged.
Public HTTP through an ambient forward proxy is deliberately no longer a route;
use a direct path or verified HTTPS. This does not encrypt an HTTP connection.

The shared factory's `TestInfrastructureProxyRouting` and all three client
packages' `TestClientPlaintextProxyRouting` exercise actual HTTP, login/read/write
and CONNECT/TLS bytes over listener-free pipes with isolated environment caches.
Client tests seed password mode on the constructed transport before real login;
they do not claim a new installed password-constructor acceptance. Host/base path,
request encodings, ordinary node/version results and ticket/CSRF semantics remain.
No resource model, monitoring health verdict, saved trust or frontend field/state
changes. Guest/backup guards and separately owned native recovery remain required.

### PBS job and writer replies require complete observations

PBS running-writer, bounded task-history and job-configuration reads consume the
complete bounded HTTP 200 body and close it before admitting evidence. Missing
or null data, non-object rows, identity-less list entries, duplicate/case-variant
consumed keys, provider errors, trailing JSON/bytes, invalid UTF-8 and read/close
failures are not successful empty observations. Complete empty lists and an
explicit empty GC configuration object remain valid; supported field aliases,
additive metadata, bounded pagination and HTTP permission/404 handling remain.
Malformed body contents do not enter errors or served partial-read evidence.
Unobserved optional PBS job end/next-run timestamps are omitted on the wire,
not serialized as year-0001 dates. The existing drawer can therefore show the
actual observation time. Known timestamps, age/state and JSON round-trip meaning
are pinned by `TestPBSJobHealthFreshnessJSONDoesNotInventRunTimes`.

An unreadable enabled job family emits existing unknown/partial-error evidence,
not an empty ledger or an invented permission denial. An unreadable live-writer
view leaves an incomplete snapshot's write visibility unknown: it cannot clear
the conservative guest backup-running flag, advance completed-backup time, or
claim that a writer stopped. Complete empty writer reads can still clear that
flag, and ordinary readable recovery restores direct job/task evidence.

`TestPBSJobReadersRequireCompleteResponse` and
`TestPBSRejectedJobListingsRemainPartialEvidence` cover all four readers and
all enabled families. `TestPBSJobResponseGapReachesResources` joins ordinary
polling to served Resources through failure, missing/null/trailing replies,
confirmed empty, explicit success and failure again, preserving independent
verification and connectivity. `TestPBSWriterResponseGapCannotClearGuestActivity`
joins snapshot/cache polling, write correlation, guest backup times and served
Resources while a completed peer remains unchanged. Consuming proof uses those
emitted observations in the existing PBS and workload drawers at desktop and
narrow widths. These synthetic read-only checks establish neither native
backup/restore, guest thaw/liveness nor installed recovery.

### PBS job health requires explicit successful completion

PBS job-health evidence grants `healthy` only for a trimmed, case-insensitive
`OK` or `success` state. Arbitrary status text (including `token`, `not OK` or
`unsuccessful`) must not acquire a Healthy badge through substring matching.
An exact `running` state remains unknown completion, not a successful last run.
Explicit failure/error text outranks warning and overdue scheduling evidence;
warning text remains warning even when it mentions OK/success. Partial reads
remain unknown. Raw status, source, confidence, identity, timestamps and freshness
are preserved separately; job failure does not turn working PBS connectivity off.

`TestPBSJobHealthRequiresExplicitSuccess` covers classification and precedence.
`TestPBSPollJobHealthStatusProjection` joins read-only synthetic PBS HTTP, the
ordinary poll, published state and canonical Resources evidence through failed,
warning, running, unknown, recovered and failed-again observations, with an
independent successful verification job. Consuming proof must use these emitted
observations in the existing PBS service drawer at desktop and narrow widths.
This is truthful source/surface evidence, not native backup/restore, thaw,
filesystem writes/liveness or installed recovery of the reported guests.

### Disabled PVE backup stores retain known evidence

A listed backup-capable store which is disabled (or not confirmed enabled)
is not queried and cannot clear its known archives or PBS attribution. The
same local/shared preservation scopes apply as for a failed content read;
a complete readable shared view remains authoritative regardless of node
order. Held rows keep their original archive identity, timestamps and size.
Skipped content makes provider history partial/unavailable, not complete
empty history and not a permission refusal. Ordinary re-enable refreshes the
known copy; successful empty content or removal from a complete storage
configuration can still withdraw it. No disabled store is probed.

`TestPVEDisabledBackupStorageRetainsKnownEvidence` joins real synthetic HTTP,
raw state, persisted recovery points and provider protection projection for
local/shared/sole stores, both enable-field forms and node orders. Consuming
proof uses these actual observations in the existing date/Coverage views.
`TestPVEDisabledPBSStorageRetainsAttribution` protects saved PBS associations.
This does not establish archive availability, backup/restore execution or
native guest thaw, filesystem liveness or installed recovery.

### PVE archive removal requires complete storage inventories

Node/all-storage lists and storage-content lists become authoritative only
when the whole bounded HTTP 200 body has completed and closed successfully.
Missing/null inventories, conflicting envelopes or field spellings, duplicate
identities, malformed rows and invalid UTF-8 are unavailable observations, not
empty inventories or credential refusals. Content filtering follows validation
of every row. Genuine empty arrays, measured zeroes, sparse optional fields and
unique additive provider metadata remain supported; existing request timeouts,
authentication, TLS, failover and guest-command admission are unchanged.

The backup poll's existing per-node/per-storage failure paths keep an unreadable
local archive's identity and original backup time while readable siblings can
refresh or empty. An ordinary complete read replaces the held copy, and a
complete empty read can remove it. `TestPVEStorageInventoryAdmissionPreservesUnreadableCopies`
joins real read-only HTTP, polling, state and recovery mapping in both node
orders with an independent-connection control. Consuming proof uses those
emitted observations in the existing By date and Coverage tables. No backup,
guest probe, thaw, restore or installed recovery is established by this proof.

### PBS group cache follows the actual latest snapshot

A successful group listing may move `last-backup` backwards as well as forwards
without changing `backup-count` (for example, a prune plus an older import).
Only an equal latest time permits snapshot-cache reuse before the existing TTL.
Either direction of change causes an ordinary snapshot read, so a removed
newest snapshot cannot keep its timestamp or verification in raw inventory,
stored recovery points or protection posture. Unchanged sibling groups retain
their own cache age and bounded read count. Verification-only changes with an
unchanged group still use the existing TTL; no extra probe or backup is added.

`TestPBSGroupLatestChangeRefreshesBeforeCacheTTL` joins real PBS HTTP,
publication/cache, persisted recovery points and protection posture for VM/CT,
root/nested namespaces and both time directions, plus unchanged reuse, actual
removal and ordinary recovery. `TestPBSGroupLatestChangePreservesFailedRefreshBoundaries`
retains transient history without renewing its cache, withdraws access-rejected
raw rows, and never treats a failed refresh as current protection. Consuming
proof exercises the emitted observations through existing Backups date and
Coverage views. This does not establish native backup, restore or guest recovery.

### PBS inventory failures retain only the unreadable namespace

A transient or invalid group-list response preserves the exact datastore and
normalised namespace's known raw backup rows, even when a sibling succeeds.
Parent and child namespaces are independent scopes. Held rows keep their
original identity, backup time and snapshot-read cache age; they do not establish
complete provider history or current protection. Readable scopes still refresh
or authoritatively empty, and access-rejected scopes still withdraw their raw
rows. A different transient failure cannot reintroduce rejected raw rows through
a datastore/global fallback. Historical recovery points remain governed by the
existing incomplete-provider reconciliation rule, not by raw row presence.

`TestPBSNamespaceFailuresRespectReadScope` joins real PBS HTTP collection,
matching cache publication, stored recovery points and protection projections
through sibling success, malformed/503 reads, rejection, ordinary recovery,
mixed whole-provider failure and scoped empty reads, in both namespace orders.
Root normalisation, a same-ID parent/child, another datastore and another
connection stay distinct. Desktop/narrow consuming proof uses the emitted
projections through the existing Backups table, filters and Coverage view.
This does not start a backup, change access checks, verify restore readiness,
or establish native guest thaw, writes/liveness or installed recovery.

### Local PVE archive identity and partial-read scope

Volume IDs identify one archive only inside its storage location. PVE's
`shared` flag permits cross-node deduplication; otherwise IDs and failure
preservation include the queried storage node. `StorageBackup.storageNode`
retains that local host independently of the guest's current `node`, including
after migration. An empty successful local-store inventory removes only its
own rows; an unreadable or offline peer keeps its prior archives without
renewing their dates. A successful shared view still replaces that shared
inventory regardless of node order. PBS attribution confirmations retain their
existing separate scope. `TestPVEBackupStorageScopeFailureAndRecovery` exercises
real read-only HTTP collection, migration, persisted recovery points and
connection isolation. This is not native backup, guest thaw or restore proof.

### Snapshot inventories distinguish unavailable from empty

PVE QEMU/LXC snapshot reads authorise removal only after a complete bounded
HTTP 200 response with one unambiguous array and unique named rows. Missing/null
inventories, conflicting error envelopes, duplicate/case-overwritten fields,
invalid names/types, broken/truncated bodies and trailing values are collection
failures, not empty success or a reason to rotate credentials. Additive metadata,
optional snapshot times and PVE's filtered `current` position remain supported.
Only endpoint scope supplies the guest's VMID. Invalid response contents do not
appear in the new validation diagnostic.

Every failed guest snapshot read, including an unavailable LXC endpoint, retains
that guest's existing inventory. Error text containing `596` or `not available`
cannot clear it. Independent guests still refresh; successful explicit empty
lists remove only their scoped snapshots, recovery points and corresponding
alerts. Other connections and backup classes remain outside that reconciliation.
No QGA command, trust policy, guest action or notification routing is changed.

`TestSnapshotInventoryResponseAdmission`, `TestSnapshotInventoryReadRecovery`
and `TestSnapshotInventoryFailuresPreserveServedRecovery` cover real HTTP client
admission, ordinary collection, original snapshot/alert identity, recovery-store
reconciliation and served state through failure, usable recovery and scoped
empty-list replacement. Consuming desktop/narrow proof exercises those actual
projections through the production Backups and Alerts surfaces. This controlled
proof is not native thaw, backup completion, every-filesystem writes/liveness,
prior-active restoration or installed/fresh-install recovery.

### Guest metadata safety follows the VM across node migration

Guest metadata, OS-command suppression, refresh/backoff and whole-fetch ownership
are keyed by exact configured connection and cluster-wide VMID, not the current
node coordinate. Migration and a move back cannot repeat a known-unsupported OS
command, reset its failure counter, bypass an outstanding metadata fetch or
renew retained observations. Eligible network/version refresh still works while
the OS command remains suppressed. The node used on the wire and in the served
resource remains the current node; node-bound disk/memory observations and their
origins are not transferred by this metadata key.

Independent VMIDs and connection labels (including delimiters) remain isolated.
Explicit guest invalidation, connection retirement, late-result exclusion,
idle cleanup and every lower-level lock/uncertainty/TLS fence remain effective.
This in-memory state does not survive process restart or establish native guest
thaw, backup success, writes/liveness or recovery of any reported guest.

`TestGuestMetadataMigrationKeepsSuppression` learns suppression through real
HTTP clients and both ordinary guest collectors, then checks migration, eligible
refresh, return migration, cleanup and explicit invalidation. The existing
whole-HTTP-fetch reservation test also rejects a migrated duplicate, including
after its scheduling hold expires. Consuming desktop/narrow browser proof uses
the actual final producer projections through the production Workloads adapter,
row and drawer; it is controlled surface proof, not native migration acceptance.
The existing Windows bootstrap identity control uses distinct connection/VMID
identities for Linux and unknown-OS peers. A same-VMID node move instead preserves
the observed Windows metadata and sends no optional Linux read; the destination
gets independent status memory, not a copy of the source node's memory cache.
Its existing expired-OS/reinstalled-Linux and measured-zero controls still apply.

### Filesystem diagnostics exclude reply payloads

The Proxmox filesystem reader logs response byte count and node/VM attribution,
not the raw QGA reply. Additive or rejected guest metadata must not be copied
into persistent logs, the live log stream or support-bundle logs by that reader,
even at Debug. Existing decoding, measured zero, partial-inventory errors and
shared guest-command admission remain unchanged. Other diagnostic events may
still contain guest names and paths: this is not whole-log sanitisation.
Unrecognized object results retain only their byte count and node/VM attribution,
never their raw contents, without changing the existing empty-result outcome.
`TestGuestFilesystemResponseDiagnosticsNoPayload` covers array/object, large
additive, incomplete, empty, null and small/large unrecognized object responses
through the real HTTP client.

Malformed filesystem arrays are decoded one row at a time, without reserving
accepted-filesystem output for every rejected row. Non-object rows retain their
existing rejection without constructing a per-filesystem object decoder.
`TestGuestFilesystemNonObjectRowsRemainIncomplete` pins null/scalar/list rows,
accepted measured-zero peers and unchanged completed-command admission.
A completed reply with
malformed rows emits one payload-free Warning with total rows, rejected rows,
the first rejected zero-based index and node/VM attribution, not one event per
rejection. Valid peers retain order, disk postprocessing and measured zero;
the existing counted incomplete-inventory error remains authoritative. Empty,
null and object compatibility, the 8 MiB body bound, both operation-lock checks,
single-attempt admission and uncertainty cooldowns are unchanged. This bounds
this reader's rejection warning volume, not all log events or total parsing
cost; it does not establish native guest/backup or installed recovery.
`TestGuestFilesystemMalformedReplyCost` joins 0/1/32/4096 rejections, Warn/Debug
logs, peers at both ends, command counts and ordinary recovery through HTTP.
`BenchmarkGuestFilesystemReplyCost` retains comparable reader allocations for
small valid and rejection-heavy replies, without a timing acceptance threshold.
The existing System Logs panel consumes an actual producer warning via SSE at
desktop/narrow widths; Pause/Resume/Clear remain display-only controls.

### Backups can reread healthy inventory without leaving the view

The existing Backups surface discloses that its restore points reflect the last
inventory read, not a live check. Refresh is available in readable, empty and
failed states. It rereads the existing PVE/PBS collected-snapshot APIs and current
protection posture; it starts no backup or guest command and adds no recurring
inventory poll. No API, trust, policy or native recovery authority changes.

The existing independent read owners settle each source. While either inventory
or a foreground posture read is pending, another whole-view refresh is disabled
and rejected by the handler. Retained readable observations stay visible pending
replacement; a failed source withdraws its old points, while successful empty
inventory is authoritative replacement. Existing per-source retry, organisation
switch cancellation, query ownership and resource health remain independent.
Filters, route, logical row expansion and the refresh control are not remounted
to perform a read. This does not establish appliance collection freshness,
backup completion, thaw or filesystem writes.

The existing all-failed error action uses the same pending predicate, including
an initial posture read that has not settled yet. It must not appear enabled
while its handler would reject the click. Other platform error actions retain
their existing behaviour when no disabled state is supplied.

`ProxmoxBackupsTable.test.tsx` covers readable/empty/failed starts, replacement,
per-owner pending settlement and disabled duplicate activation. Consuming desktop
and narrow browser proof must drive actual synthetic GET responses through the
production reader and tables, preserving view state and inspecting current,
pending, changed, failed and successful-empty pixels.

### Node-fallback History preserves missing filesystem evidence

When the cluster guest endpoint is unavailable, ordinary node fallback uses the
same VM disk-History admission as the efficient collector. Retained usage and
allocation-only tuples with a disk-status reason remain display continuity,
not new samples. Complete current readings, including measured zero and genuine
volume removal, still append History and clear the corresponding disk alerts.
Independent CPU/memory/counter recording, poll ownership, original disk age,
unknown metrics and QGA/backup safety fences are unchanged. Existing History is
not rewritten or deleted.

`TestVMFilesystemIncompletePollingPreservesHistoryAndBreach/fallback` drives the
actual client and owned fallback dispatch through complete, partial, expired and
complete-zero recovery, checking persisted/in-memory History, canonical/served
metrics, original observation times and filesystem breach continuity. This is
controlled source/surface evidence, not native backup completion, guest thaw,
every-filesystem writes/liveness, fresh-install or installed recovery.

### Efficient guest inventory distinguishes deletion from node grace

A completed cluster guest inventory with no guests on a currently online node
is authoritative removal, including the last VM/container and template-only
results. Effective Online alone is not evidence of a current node sighting:
the efficient collector also carries the raw `/nodes` generation. Only nodes
missing/offline there, still Online under the existing node grace policy and
absent from the guest inventory retain prior guests. The existing fallback
when no node data is available remains conservative; expired/never-online nodes
retain their previous removal policy.

Grace retention uses complete Proxmox-owned tuples, not reduced or linked-agent
read-state enrichment. Values and original observation times remain unchanged
and contribute no new History samples. A cluster-wide guest ID positively
observed at new coordinates or as a template cannot be resurrected at an old
node by grace retention. Other connections with the same node/VMID are not
changed. Normal recovery on the same live connection advances observations;
no cache, extra request, guest command, safety interval, TLS or schema changes.

`efficient_guest_inventory_lifecycle_test.go` exercises real node/resource HTTP
reads and ordinary owned publication for accepted emptiness, offline/omitted
grace, expiry, unknown node data, templates, moves and subsequent recovery.
Consuming desktop/narrow Workloads table/drawer verification uses actual final
producer projections. These controlled source/surface checks are not native
thaw, filesystem writes/liveness, restart/fresh-install or installed recovery.


### PVE task evidence requires completed collection

PVE node/backup task lists and task logs consume the complete bounded HTTP
response before publishing rows. Explicit empty arrays remain valid; missing,
null, invalid UTF-8, error-bearing or ambiguous envelopes/consumed fields do
not become successful empty or completed task observations. Additive metadata,
running tasks without status/endtime, filtering and offline-node scope remain.
If any online node fails, the instance task read fails atomically; the monitor
retains its prior tasks and original observation times rather than dropping
running/failed peers. This does not claim the retained tasks are still current.

A task log at the existing line cap is incomplete evidence, not an immutable
finished-job log. Failed/cancelled/skipped log reads cannot renew a non-final
cached guest observation. The cache itself stores the original successful read
time, not merely the published copy; a later complete ordinary read can populate the
cache, and completed immutable logs remain reusable. Independent guest status,
zero/presence counters, operation-lock checks and every QGA safety fence are
unchanged. Source/consumer proof establishes no backup success, guest thaw,
writes/liveness, native recovery or published availability.

`task_response_test.go`, `pve_backup_task_completion_test.go`,
`TestIncompleteBackupTasksLeaveIndependentGuestCountersUsable` and
`TestPVELogRetryDoesNotRenewRetainedEvidence` cover real HTTP failures,
complete/empty recovery, retained producer timestamps, cache poisoning and the
independent status boundary.

### Conflicting saved endpoint pins cannot select certificate trust

For equivalent Proxmox endpoint URL spellings, all nonempty saved member pins
must agree under the TLS verifier's existing case/colon normalization. Exact
spelling and map iteration can no longer select one of contradictory pins.
Resolution returns a fixed, credential-free trust-conflict error before a TLS
probe, password login, token read, first-use capture or polling-client creation.
The member stays unreachable to Pulse; an independently trusted sibling can
continue ordinary monitoring. No saved pin is removed or overwritten, and the
error cannot become an empty pin/CA or opt-out fallback. Agreeing pin formats,
nonempty invalid pins, primary-pin fallback, declared-member first-use and the
existing scheme/port/base-path/IPv4-mapped trust boundaries remain unchanged.

`TestClusterConflictingSavedPinsNeverAuthenticate` covers startup, recovery and
lazy selection in token/JSON/form modes, three URL spellings and both VerifySSL
settings with real TLS sockets. It checks zero TLS/login/read work for conflicts,
no installed client or trust mutation, sibling use, and accepted monitoring
following a normal reload with reconciled fixture pins. Existing established-pin,
first-use, cancellation, ownership and guest-admission controls still apply.
`TestClusterSavedPinResolutionRejectsConflicts` and
`TestClusterFirstUseCaptureDoesNotReconcileConflictingPins` retain the conflict
through normalized authorities, invalid nonempty pins and a capture race.
`TestClusterConflictingPinsEndpointProjection` uses the production endpoint
builder, real clients and monitor projection in all three auth modes: native
Online, identity and saved trust are not changed by failed Pulse reachability.
The producer also records contradictory exact duplicate saved rows before
its map/deduplication can overwrite them, and carries those endpoint conflicts
through initialisation, missing-client reconnect and cluster refresh. The
cluster owns a copy of that conflict set; aliases cannot escape it, and a
later repeated row cannot clear it. Normal configuration reload with agreeing
pins is required to resume that endpoint. A non-conflicting sibling is not
refused. The public constructor's optional conflict inputs preserve existing
callers and do not introduce a new trust mode. Producer tests cover both
routes, pin formatting, repeated contradictions and configuration ownership.
The projection cases include exact duplicates as well as URL aliases.
A credential-free agreement check also holds automatic topology refresh while
saved trust is contradictory: discovery cannot collapse the rows, move an
alias or substitute a pin to clear that refusal. Current trusted sibling
clients keep monitoring; topology changes wait for independently reconciled
configuration. `TestClusterPinConflictsSurviveMonitorLifecycle` exercises the
actual initialisation, missing-client retry and refresh/rebuild paths in all
three auth modes and both layouts, including normal refresh after reconciliation.
Saved-view consolidation also refuses a view with internal pin contradictions,
so map indexing cannot hide an earlier contradictory row before monitoring
starts. `TestConsolidatePVEInstancesRetainsInternallyConflictingPins` covers
both merge orders, repeated rows, same-node/address evidence and agreeing formats.
Internal agreement uses the TLS verifier's case/colon comparison even when
legacy identity formatting would erase or simplify a malformed nonempty pin;
colon-only, whitespace and prefixed pins cannot hide a contradiction.
The endpoint-ownership routing control uses a five-second fixture client and
a ten-second enclosing caller lifetime, rather than a one-second latency
criterion. It records elapsed time and both targets' request counts; unchanged
pin, admission and routing assertions remain required. Runtime timeouts and
separate timeout/cancellation controls are unchanged. This is not a diagnosis
or dismissal of the retained earlier one-second timeout.
Lock-evidence fixture cases use independent base-path namespaces while each
case's clients share the same guard key. A recycled loopback port cannot inherit
an earlier synthetic guest's cooldown. `TestGuestAgentLockEvidenceFixtureIsolation`
shares one listener and retains an earlier uncertainty fence unchanged while an
independent fixture completes its lock/command checks. No runtime fence is
cleared, expired or weakened; native guest recovery is not established.
HTTP-status fixtures share this same test-only endpoint namespace helper.
Independent cases keep distinct base-path identities even when a loopback
listener is reused; every diagnostic and cluster alias within a case retains
the same identity and VM-wide cooldown. `TestGuestAgentHTTPFailureFixtureIsolation`
uses one listener for two independent uncertain commands, then verifies both
unchanged fences still reject all follow-up guest reads without wire work.
The old bare-endpoint fixture behaviour fails that control. This is a separately
claimed test-stabilization correction; no runtime guard, cooldown or provider
completion rule is changed.
This establishes source/consumer behaviour, not native recovery or attribution
of a guest, PBS or certificate report.


### Cluster recovery requires success of the operation actually probed

Startup and unhealthy-member recovery read `/nodes`; lazy construction uses the
same probe. A failed nodes or login request cannot count as successful recovery
because its response text mentions a guest agent, QMP or `guest-get-*`. The old
per-guest substring exception is removed from these non-guest operations.
Failed selection retains its actual sanitised reason and observation time, and
installs no polling client. Successful probe plus accepted full-timeout session
is still required. The existing temporary-rate-limit path may admit an ordinary
request, but does not report the failed probe as passed or clear a known failure.

The single-endpoint last-resort path remains available with its cached client;
selecting it no longer marks that endpoint healthy before a request succeeds.
A later accepted ordinary read still clears the failure. Actual per-guest
errors remain request-level errors, without marking an otherwise working node
unhealthy. Certificate pins, first-use trust, credential encodings, request and
recovery cadence, rate-limit backoff and guest/backup fences are unchanged.

`TestClusterNodeProbeRequiresSuccess` covers startup/recovery/selection in token,
JSON password and form password modes, three guest-like failure bodies at the
nodes or login stage, no extra login/full client and distinct later recovery.
`TestClusterSingleEndpointFallbackKeepsFailedHealth` checks cached selection,
failed ordinary read and later success. `TestClusterNodeProbeRateLimitKeepsOrdinaryRead`
preserves genuine rate-limit handling. `TestClusterNodeProbeEndpointProjection`
carries the actual failed and later recovered member through the production
monitor projection while independent native Online, saved trust and identity
remain unchanged. These are source/consumer controls, not attribution or
installed acceptance for a reported PBS/QGA/backup incident.


### Node inventory admission consumes the complete response

The PVE `/nodes` reader consumes and closes the entire existing 8 MiB-bounded
body before returning inventory or allowing startup/recovery/lazy probes to
succeed. A valid JSON prefix with a missing transport tail, extra JSON values,
oversized suffix, malformed UTF-8, missing/null data, conflicting errors or
ambiguous/case-overwritten fields is not an authoritative inventory. Each row
must have a canonical nonempty, unique node name; null/unnamed/duplicate rows
and ambiguous supported node fields reject the whole response rather than
publish partial or invented empty inventory. Validation errors contain no body
material. Unique provider metadata and explicit empty arrays remain accepted,
including scoped tokens with no visible nodes. Existing node fields and offline
status are preserved without changing provider permissions.

`node_inventory_response_test.go` exercises actual HTTP reads and all supported
fields, the explicit-empty controls, byte/transport bounds and no replay.
`cluster_node_inventory_test.go` checks pinned HTTPS in token, JSON-password and
form-password modes at startup, recovery and lazy selection: failed success
installs no client and never advances to a full session; distinct complete
recovery remains usable. The cached fallback keeps its earlier failure until a
complete ordinary read succeeds. `TestClusterNodeInventoryEndpointProjection`
drives missing/duplicate/incomplete responses and subsequent ordinary recovery
through the production monitor projection while native Online, identity and
saved pins remain intact. Recovery cadence, auth encodings, TLS admission,
rate-limit policy, other API readers and guest/backup fences are unchanged.
These are source and consuming-surface proofs, not attribution or installed
acceptance for any reported PBS, guest restart or workload-safety incident.


### First-use cluster trust includes password construction

Declared, previously untrusted Proxmox members use the existing credential-free
TLS capture and pinned retry whether the inherited primary pin fails during
password login or during the token-authenticated nodes probe. Startup, ordinary
unhealthy recovery and lazy first selection share this path. Capture still
requires typed local pin-mismatch evidence and cannot replace a primary, saved
member or concurrent winner pin; it never sends an HTTP request or credentials.
The pinned retry is not repeated to obtain a nodes count. Failed or cancelled
authentication installs no polling session; a separately accepted full-timeout
client is required before recovery becomes healthy. Refusal retains the
established runtime pin for a distinct later ordinary recovery. Saved trust,
CA/redirect policy, login encodings, cadence and guest/backup fences do not change.

`TestClusterFirstUseSessionAdmission` covers JSON password, existing form
fallback and token modes in all three phases, then ordinary member reads.
The adjacent refusal, established-pin and cancellation controls exercise actual
TLS sockets. Concurrent lazy selection retains serialized construction while
health/trust inspection stays usable during capture. `TestClusterFirstUsePasswordEndpointProjection` carries first-use
probe/full-session refusal and later public-read recovery into the monitor's
cluster-member projection without changing native Online state, identity or
saved fingerprints. This is source/consumer qualification, not installed
cluster recovery or attribution of an existing user report.


### Proxmox password success admits a complete session, not just HTTP 200

PVE and PBS consume the entire existing byte-bounded login body before
publishing ticket, CSRF token and the normal two-hour expiry together. The
shared `internal/securityutil/proxmox_session.go` parser requires canonical,
unambiguous envelope/session keys, complete UTF-8 JSON, a nonempty cookie-safe
ticket and a nonempty header-safe CSRF token. Unknown unique metadata such as
username and capabilities remains supported. Partial/null, duplicate or
case-conflicting credentials, extra JSON values, incomplete reads and oversized
tails fail without replacing the retained tuple or advancing PVE generation.
Validation errors contain no response credentials. A malformed success is not
a provider credential refusal and does not trigger JSON-to-form fallback.

`session_response_admission_test.go` in both client packages exercises these
controls and eight joined expired readers, zero subsequent API/form requests
on invalid success, retained state and distinct later ordinary recovery with
coherent cookie/CSRF pairs. Cluster recovery additionally checks empty and
partial success for the retained polling client, with and without a previous
client. `TestPBSInvalidSessionKeepsFirstPollBoundary` exercises the real pinned
constructor, normal startup retry, poll executor and production broadcast:
invalid success installs no client, valid construction is not healthy telemetry,
and only the accepted first poll publishes recovered node/datastore readings.

TLS trust, credentials, auth encodings and 400/415 fallback, API-token paths,
renewal ownership, retries/cadence, guest/backup fences and History are unchanged.
The parser establishes response integrity, not cryptographic ticket validation
or native/installed acceptance. It neither attributes nor resolves a reported
PBS, Windows guest, restart or workload-safety incident.


The invalid-QGA-success matrix uses a distinct synthetic base path for each
case/reader, so OS reuse of a closed ephemeral listener cannot borrow a previous
fixture's live cooldown. `TestGuestAgentInvalidSuccessFixtureIsolation` shares
one listener deliberately and checks two independent first admissions and both
retained second-client fences. No production key, alias, TTL or guard state is
changed or cleared to qualify these tests.

### Proxmox recovery sessions honour the owning caller

PVE/PBS context-aware constructors bind initial password login, including the
existing JSON/form fallback, to the initiating context. Cluster selection,
first-use pinned retry, synchronous construction and unhealthy-member recovery
carry that context through both the short probe and the retained polling
session. The six-second background recovery budget also bounds construction;
ending a caller is neither a credential refusal nor successful recovery.
Cancelled selection/recovery preserves prior errors and does not install a
partial or nil client. Startup retry exits without publishing a new health
outcome or advancing another provider after its monitor stops. Discarded probe
transports close their idle connections.

Initial PVE login runs synchronously on that construction context: an unpublished
client has no other readers to preserve. This is distinct from renewal on a
retained PVE client, whose bounded shared login survives its starter leaving.
Restoring constructor cancellation must not abort live renewal waiters, replay
their failed login or lose a completed renewal. The JSON and 400/415 form paths
use the same lifetime distinction. `TestClientSessionRenewalStarterCancellation`
checks both encodings alongside the constructor and cluster caller-budget
controls below; it preserves the existing detached-renewal timeout and
ticket/CSRF publication tests.

PVE response consumption retains the same owner boundary: cancellation or the
login budget ending takes precedence over an empty, refused or unreadable reply.
An ended exchange cannot try another encoding or publish a new ticket/CSRF/expiry
and generation, even if complete valid bytes arrive as it ends. This checks the
bounded shared login context, not the departed reader's context, so live renewal
waiters remain independent. `TestClientCancelledAuthResponseOwnsErrorAndSession`
cancels deterministically during JSON/form response reads, checks body closure
and retained session/generation for five response classes, then exercises one
distinct later login and an ordinary authenticated node read. This complements,
and does not replace, the pinned-HTTPS constructor/deadline/renewal controls.

The compatibility constructors retain their background-context behaviour.
Accepted clients retain their configured full timeout, and subsequent ordinary
reads use their own contexts, not the ended construction context. TLS pins,
CA verification, redirects, first-use admission, auth encodings, retry cadence,
provider permissions, guest/backup fences and History are unchanged.

`TestClientConstructionHonoursContext` in both client packages covers ended
password/token construction, in-flight JSON/form cancellation/deadlines,
independent later reads and wrong-pin refusal. Cluster caller-budget controls
cover selection/recovery at first login, probe and full login, then trusted
ordinary recovery. Startup cancellation covers PVE, PBS and multi-endpoint
cluster construction. The connected PBS first-poll control emits the actual
broadcast for desktop/phone consumption: cancellation cannot turn an offline
server healthy, and accepted construction alone still cannot do so. These are
secret-free local fixtures, not installed recovery, native guest safety or an
attribution of any reporter's failure.


### Cluster recovery requires a usable polling session

A short endpoint probe cannot establish successful recovery until the client
retained for ordinary polling has been constructed successfully. A refused or
malformed password login for that second client preserves the existing client
(or its absence), records the failure and does not count or publish recovery.
An endpoint must never become healthy with a nil polling client. A later
ordinary eligible recovery can install an accepted full-timeout session and
clear its own error; usable siblings continue to serve polling throughout.

`TestClusterRecoveryRequiresUsableSession` covers pinned HTTPS 401/403,
malformed success and JSON-to-form refusal, with and without a retained client,
then accepted recovery and ordinary reads. `TestClusterRecoverySessionEndpointProjection`
drives public cluster reads and background recovery through the monitor's
per-endpoint projection: native Online state, identity and saved pins survive,
while PulseReachable and PulseError reflect the actual polling session. The
existing recovery interval, authentication encodings, TLS trust/first-use
policy, failover order and guest-command safety state are unchanged. These
synthetic controls establish neither installed trust recovery nor a reported
incident's cause.

### Rejected infrastructure URLs stay out of diagnostics

PVE/PBS/PMG constructors validate a host before logging it. URL syntax errors omit
`url.Parse`'s copy of the supplied input; ordinary validation reasons remain.
Cluster logs, health snapshots, recovery summaries and aggregate errors use
display-only labels:
a valid base URL or `[invalid endpoint]`. Health snapshots preserve each entry
even when rejected endpoints share that label, and are built only for enabled
log events. These labels never supply routing,
alias membership, saved configuration or TLS-pin identity. HTTP/HTTPS defaults,
base paths, authentication, CA/pin/opt-out choices and guest fences are unchanged.

`TestClientRejectedHostDiagnosticsContainNoCredentials` and
`TestClusterRejectedEndpointDiagnosticsContainNoCredentials` exercise actual
constructors, single/multiple endpoint routing and recovery with synthetic
userinfo/query/fragment and malformed URLs. Accepted-host and
`TestInfrastructureURLValidationPolicyUnchanged` controls preserve ordinary
routing and diagnostics. Initial add/test request logs and saved-provider client
initialisation (including the cluster endpoint list and refusal logs) use the
same display-only labels; filtering never changes the supplied configuration.
`TestNodeRequestEndpointDiagnosticsExcludeRejectedInput` exercises the ingress
handlers before credentials are supplied, and
`TestMonitorClientInitEndpointDiagnostics` exercises all three real
constructors through saved-provider initialisation. Parseable saved input keeps
its existing configuration normalisation; malformed syntax stays refused.
Controls retain routing, client registration and the existing health outcomes,
including PBS awaiting its first connectivity poll. The saved-PMG refusal test preserves
the exact generic HTTP response while excluding raw parser input from its
internal error. This bounds these diagnostics, not arbitrary provider error
text, other log paths, previously saved logs, native trust recovery or an
installed service.

### PBS password renewal publishes one complete shared session

Concurrent reads of an expired PBS password session share one in-flight
authentication attempt. Ticket, CSRF token and expiry are published together;
each ordinary request uses one complete tuple. Only renewal is shared: the
subsequent API reads remain independent, and a cancelled waiter cannot cancel
the owner. Caller cancellation and deadlines stay context errors, not credential
refusals, including token reads that never attempt a login. Joined callers receive the owner's failure without reissuing it;
a later ordinary call may try again. JSON authentication and the existing
400/415 form fallback retain their semantics. Tokens do not acquire session
authority, and API 401/403 replies remain terminal without adding a replay.

`pkg/pbs/auth_session_concurrency_test.go` exercises pinned local HTTPS expiry,
encoding fallback/refusal, malformed responses, independent reads, cookie/CSRF
pairing, owner/waiter cancellation and ordinary later recovery. Its accepted
node/datastore responses also consume through the canonical PBS projection.
TLS pins/CA/opt-out and redirect policy, credentials, ticket lifetime, provider
permissions, wire shapes, polling schedules, guest/backup fences and stored
History are unchanged. These synthetic controls do not establish an installed
connection, native recovery or the cause of a reported PBS/guest failure.

### Connection retirement preserves unrelated guest safety state

Retiring a consolidated PVE connection cancels and clears only its own guest
metadata lifecycle. Connection labels may contain `|`; matching a label prefix
cannot establish ownership. The metadata key's two fixed node/VMID suffixes
are read from the right, while diagnostic memory snapshots use their recorded
`Instance`. A separate `site|other` connection keeps its outstanding owner,
OS-command suppression, original observations and refresh/backoff deadlines
when `site` is retired. The retired connection's cancelled fetch still owns
its key until completion, and shared transport uncertainty is never cleared.

`TestGuestMetadataRetirementOwnershipContract` covers exact cleanup, nested and
Unicode labels, diagnostic origins and in-flight HTTP bodies at all three
metadata stages. `TestGuestMetadataRetirementPollingKeepsSuppression` exercises
both clients and ordinary collectors: no reissued suppressed OS command, no
unrelated identity loss, and continued independent filesystem/CPU/memory reads.
Wire formats, TTLs, guest-command/backup-lock/TLS fences and stored History are
unchanged. These are synthetic source controls, not native thaw/restart recovery,
a reporter's failure cause or installed/release acceptance.

### Rate refusals cannot permanently disable guest OS discovery

An observed HTTP 429 from guest OS-info preserves the existing OS-command
failure count and suppression state, like access refusal or stopped QGA. It
neither exercised the OS command nor supplies a fresh OS observation. Ordinary
metadata backoff remains in force; a subsequent admitted poll can learn the OS
without a restart or clearing command-safety state. Only the typed wire status
admits this distinction: provider text quoting 429 is not a rate refusal.

`TestGuestMetadataRateLimitRecoveryContract` covers both HTTP client types and
ordinary collectors with cold/stale metadata, OS-like 429 bodies, unchanged
backoff and accepted filesystem/memory readings, subsequent Windows discovery
before Linux-only memory admission, and genuine repeated-failure suppression
with a quoted 429. Existing unsupported-command, backup-lock and uncertain-reply
fences are unchanged. Its fixtures use local sockets and synthetic credentials,
not native guest recovery, History storage or a reported failure's cause.

### Storage investigation history shares the chart source boundary

`ReadStorageMetricHistory` and `GetStorageMetricsForChart` use the same monitor
selector for memory, retained and generated storage history. Analytical reads
retain the requested window and propagate retained-store errors. Chart-only
lookback remains presentation behavior and cannot supply older observations as
current investigation history. Generated demo samples carry `mock_synthetic`;
they are never evidence of a real storage growth rate. The requested-window and
store-error regression is `internal/monitoring/storage_history_test.go`.

### Cluster endpoint membership belongs to its client

`NewClusterClient` snapshots the supplied endpoint slice before registering
shared guest-command aliases or starting health and trust checks. After the
constructor returns, reusing caller configuration storage cannot replace live
failover targets, declare a new first-use TLS member, or route a guest read
outside its registered serial channel. Routing, health recovery and trust
admission use that same client-owned snapshot. Order, duplicate entries and
nil/empty inputs retain their previous semantics; actual configuration changes
still construct a replacement client rather than editing an existing client
through a borrowed slice.

`TestClusterClientEndpointOwnershipContract` exercises pinned ordinary reads,
typed TLS mismatch/first-use refusal, all six guest-read paths while a response
body is in flight, independent-guest and completed-command resumption, input
shape and concurrent caller reuse. Its fixtures use only synthetic credentials
and local sockets. Endpoint URLs, saved pins, CA/opt-out and first-use policy,
alias equivalence, backup locks, uncertainty fences and non-replay are unchanged.
This is source-bound admission safety, not native workload thaw, Windows restart
recovery, the cause of a reported guest failure, or installed/release acceptance.

### Repeatable resource wiring publishes a complete binding once

Router and tenant initialization applies saved notification policy before the
first resource publication, then attaches the store and supplemental provider
batch together. Repeating identical wiring does not rebuild the estate; the
single setters retain their explicit-refresh behaviour. Explicit nil provider
withdrawals remain in the router's initializer registry so existing and future
tenants cannot retain a removed source. An empty saved private-CIDR allowlist
revokes previous grants; absent/unreadable settings and the automatic public-URL
fallback retain their existing semantics.

A refresh captures its store/provider generation before collecting records.
Delayed old collectors are refused before publication; full store writes are
ordered after collection, without holding monitor or provider locks across
callbacks. Changed bindings advance the existing alert epoch and join admitted
old checks without clearing active alerts. Dependency setters, like a mock-mode
switch, must not change bindings from an admitted alert callback. Consume-once
source changes are drained only after an admitted publication, outside the
publication lock. The read path keeps its existing non-blocking/freshness gate.

Native TrueNAS/VMware providers and the demo adapter declare their mode. One
mode read filters the pair before records, ownership or changes are collected,
including the interval before the existing post-switch hook replaces them.
Opposite-mode data cannot become a live/demo mixture; tenant fixture exclusions
remain. Runtime-check shutdown, cancellation, token/tenant boundaries, resource
wire shapes, thresholds, retention and guest/TLS safety fences are unchanged.

`resource_wiring_test.go` and `router_resource_wiring_test.go` cover pass counts,
first-alert policy, allowlist revocation, withdrawals, delayed collection and
publication, re-entry, admitted-check joins and switch-hook gaps. These are
bounded synthetic component controls, not whole-source, native startup/RSS,
notification delivery or installed/reporter acceptance.


### Guest filesystem guidance honours its caller's logger

Missing-filesystem guidance uses the existing caller context logger, falling
back to the ordinary global logger when none is set. Message, severity, scope,
provider-text exclusions and backup precautions are unchanged. The connected
`TestGuestFilesystemFailureGuidanceUsesObservedEvidence` captures its own
context-tagged output without replacing a shared logger; the scheduler-health
fixture stops the monitor it owns. This repairs the observed shared-logger race
with an otherwise leaked alert-save worker, not a guest or backup recovery.

### Measured zero guest availability stays authoritative

Status/current meminfo distinguishes an explicit numeric `available: 0` from
an omitted or null field. The measured zero stays selected, including through
JSON copies, rather than being replaced by free/cache components or another
memory source. Positive programmatic samples and legacy payloads without
availability retain their existing behaviour; invalid numeric fields remain
errors. The internal presence bit never becomes a wire field.

`TestGuestStatusMemoryAvailabilityContract` covers selection and both ordinary
collectors through direct/cluster HTTP clients, canonical views, History and
actual threshold evaluation: measured exhaustion raises a breach, missing
availability cannot clear it or add History, and fresh healthy evidence permits
recovery. `TestVMStatusMemoryAvailabilityPresence` preserves the wire distinction.
The sampled capacity, original observation time, guest command/backup-lock
fences, QGA parser and configured thresholds are unchanged. This is synthetic
source evidence, not reporter #2762 cause, native recovery or availability.

### Guest memory readings keep their own capacity

The QEMU memory selector pairs guest availability with the total from the
same status meminfo or QGA `/proc/meminfo` sample, and linked in-guest agent
usage with that agent's total. A configured maximum is not guest usage:
ballooning and kernel reservations can make those capacities differ.
Existing availability-only payloads retain their configured-capacity fallback.
Reported totals larger than a known VM maximum are not accepted as guest
evidence, including a hotplug transition before configured status catches up.
The existing balloon/free-memory fallback also keeps the denominator it used
to derive usage, without being upgraded to cache-aware pressure evidence.
The diagnostic snapshot records `guestAgentMemTotal` alongside the already
retained configured maximum and memory fields; it adds no read or public log.

Retention keeps the selected total, used, free/cache and original observation
as one tuple. Configured-capacity changes, including a resize to the old guest
total, cannot borrow that tuple. The existing source age bounds, backup lock,
filesystem-first ordering and shared command fences are unchanged; a held or
expired sample cannot produce new History, breach or recovery evidence.
`TestGuestMemorySampleCapacityContract` and
`TestGuestMemorySampleCapacityCanonicalProjection` check selectors, coherent
cache binding and both ordinary collectors with direct/cluster HTTP clients,
canonical views, History and actual threshold decisions. Healthy ballooned,
full-pressure/zero-available, legacy, resize and age controls remain distinct.
These are secret-free source fixtures, not native balloon or freeze recovery,
reporter #2762 cause/resolution, or release acceptance.

### Contradictory guest meminfo cannot manufacture healthy pressure

The QGA `/proc/meminfo` parser rejects physical counters above their sample's
MemTotal, integer-overflowing cache estimates and derived availability above
that total. It does not clamp contradictory evidence into a trusted zero-used
sample. Shmem is subtracted before checking the derived bound; valid older-kernel
estimates, exact-total/zero-used readings and explicit zero MemAvailable retain
their semantics. Swap capacity is not physical RAM, and explicit MemAvailable
still takes precedence over the optional cache estimate.

A completed but contradictory file-read remains an ordinary failed observation,
not a QGA transport pause. Existing negative-cache timing and independent
provider/linked-agent selection remain unchanged. A cold guest can retain an
independent provider tuple; an existing successful guest reading may instead
remain bounded last-known evidence with its original observation time. Neither
provider usage nor held guest data becomes current cache-aware guest pressure,
and neither can clear an incident established by an earlier qualified sample. Normal valid polling can restore guest authority and
clear it. The original backup-lock, single-attempt, uncertainty, observation-age
and History rules remain required; no command or recovery route is added.

Retained snapshots carry their explicitly annotated original source through
subsequent failed and negative-cache polls. The `previous-snapshot` wrapper
cannot discard a qualified origin after one poll and thereby clear a held
incident using an independent provider fallback. This continuity remains
last-known, uses the unchanged two-minute original-observation window and
capacity fence, and adds no History points or guest-pressure authority. A
wrapper with unknown/low-trust provenance, a missing/future/expired original,
or a capacity change remains ineligible; independent current qualified memory
still wins. `TestGuestMemoryRetainedOriginSurvivesRepeatedLowTrustPolls` covers
those controls and 125 synthetic poll transitions across the original deadline.

`pkg/proxmox/meminfo_integrity_test.go` covers contradictory physical components,
overflow and the supported zero/shared-memory/modern-available boundaries.
`TestGuestMeminfoIntegrityPollingContract` exercises both ordinary collectors
with actual single/cluster HTTP clients for every physical-counter and
estimate-refusal branch, canonical views, negative caching, backup-lock
admission and the production alert decision. Rejected, negative-cached and
locked polls must leave both percentage and byte History point sets exactly
unchanged, including original times. Valid recovery and genuine zero must add
only their own points. Each synthetic poll has a bounded caller context without
changing production timeouts. These are
synthetic source controls, not #2762 attribution, native freeze/restart recovery,
resolution of #2439/#2619/#2822 or released availability.

### Existing memory origins cannot be renewed by AI consumers

AI query/context/fact consumers now retain the selected guest observation and
explicit availability. The existing local VM/system-container memory observer
judges pressure only from a qualified selected sample and applies its age bound
to that original observation, not a fresh resource LastSeen from another source.
Unknown pressure is an evidence-gap wake, not a measured threshold breach or
recovery. `TestPatrolObserverGuestMemoryOrigin` and
`TestGuestMemoryEvidenceUsesSelectedMetricOnly` cover these consumption boundaries.
Collectors, guest commands, native leases, shared safety pauses, History storage
and configured thresholds remain unchanged; this is not native #2762 recovery.

### Guest memory evidence survives typed consumers

The monitor's existing guest memory observation remains source-owned when
Patrol reads it through VM/system-container views. Selected canonical memory,
not a retained raw Proxmox facet, supplies provenance; absent memory is distinct
from measured zero. `TestGuestViewMemoryObservationKeepsSelectedOrigin` checks
that read boundary and `TestPatrolGuestMemoryEvidence` checks the consumer's
qualifications through ordinary snapshot ingestion. This changes no memory
collector/parser, guest command, TTL, shared safety pause, alert threshold or
History writer. Synthetic named-field checks and qualified summaries are not
native acceptance of #2762's reported input or guest recovery.

### Cluster recovery preserves established certificate pins

Startup and unhealthy-endpoint recovery must not replace the configured primary
fingerprint or a saved/previously captured member fingerprint after a TLS
mismatch. No authenticated retry or unverified certificate capture is admitted
for those known endpoints; their failure remains visible until the trusted
certificate returns or the operator independently verifies and saves new trust.
Equivalent URL spellings preserve the same pin without merging distinct scheme,
port or base-path authorities.

First-use trust remains available for a declared, previously untrusted member
that fails the primary fingerprint check. Only the typed local TLS verifier
failure admits this bootstrap, not a provider message quoting a mismatch.
Capture itself sends no API credential. The subsequent API request verifies the
captured pin. A competing capture cannot replace a pin established while its
probe was in flight, and cancelled work cannot initiate or commit new trust.
Runtime captures belong to the client, not the caller's saved fingerprint map.
This does not add a certificate-management surface or persist new runtime trust.

The credential-free first-use TLS probe shares the polling caller's context:
cancellation or deadline ends a stalled handshake and releases its socket without
waiting for the peer. The existing five-second capture ceiling remains; it never
extends the caller's budget. Cancellation is checked again under the trust mutex
before a runtime pin is committed. A later ordinary caller may still establish
the declared member's first-use pin and make the existing pin-verified API read.

`TestClusterClientFingerprintTrustContract` checks changed and restored real TLS
leaves through startup/recovery, token containment, known/equivalent pins,
provider-text/undeclared/cancelled exclusions, concurrent first-use capture and
ordinary first-use authenticated acceptance. The existing #1664 new-member
control remains. These are secret-free loopback source proofs, not live cluster
or installed acceptance, exploitation evidence or native guest recovery.

`TestClusterClientFirstUseCaptureCancellation` holds an actual first-use TLS
handshake behind a local barrier. Cancellation and deadline must return before
that barrier is released, without a new pin or authenticated request; a fresh
caller subsequently establishes the owned leaf and makes one pinned API read.

### Known Windows guests do not receive Linux-only memory reads

An accepted guest OS reply identifying Windows suppresses the optional
`/proc/meminfo` fallback for that instance/node/VM, including after an earlier
failure cache expires. The classification has its own original OS observation
time, bounded by the existing ten-minute guest evidence age; other useful
network/version replies and retained display strings cannot renew it. Missing,
future and expired origins do not suppress the existing fallback. A completed
empty OS reply invalidates the classification; a subsequent Linux reply permits
ordinary memory polling, including measured zero availability. An accepted OS
outcome survives a later version-command deferral without publishing partial
identity or renewing general QGA availability evidence.

This avoids an inapplicable command that can start a shared uncertainty pause.
It does not invent a negative memory observation or clear an existing pause.
Independent status/balloon/linked-agent memory, filesystem-first ordering,
unknown-OS compatibility and all command/backup-lock admission remain intact.
The first unknown-OS poll can still attempt the existing Linux fallback; this
requires completing ordinary metadata collection first. Both detailed-status
collectors now collect filesystems, then metadata, then optional Linux memory.
On an ordinary cold start or expired metadata refresh, the real OS reply can
therefore establish the existing Windows exclusion before an inappropriate
Linux read creates a shared pause. No extra probe, persistent OS hint or
configuration/display-name inference is added. If metadata itself is uncertain,
the same channel guard still prevents every later command; independent current
status/balloon/linked-agent memory is not turned into guest meminfo evidence.

`TestGuestWindowsMeminfoPollingContract` covers the real direct/cluster clients
and both collectors after an actual OS read: repeated NTFS usage through zero,
canonical/public read state, in-memory and persistent History, live CPU/memory,
unchanged original caches, backup-lock deferral and unlocked resumption. Unit
controls pin exact OS-name boundaries, original age/identity, partial-fetch
evidence, Linux recovery and cache/source compatibility. These are synthetic
source controls, not native QEMU-only Windows recovery, thaw, cross-process or
restart containment, reporter resolution or release acceptance.

`TestGuestWindowsRestartPollingContract` starts empty monitors through both real
clients and both collectors, ages actual OS receipts for the expiry control,
and runs two distinct fresh test processes against the same loopback endpoint.
It checks both NTFS volumes, healthy zero recovery, OS/canonical/public state,
memory source and in-memory/persistent History. The Linux command's fixture
response remains an unexplained HTTP 500: recovery comes from not sending it,
not from a completion exemption. Lock, in-flight/shared-client uncertainty,
original ages, Linux/unknown and replacement-identity controls remain covered.
The ordinary supported loopback process restart is source acceptance, not
native Pro/PVE restart recovery, every-filesystem writes/liveness, independent
thaw, prior-active restoration or a released fix. Existing OS-info safety
suppression and metadata backoff are not reset to force recovery.

### QGA fallback eligibility keeps its original observation age

Only an actual guest-agent reply, a usable filesystem reading or useful guest
metadata, can establish fallback query eligibility. The `status/current` `agent`
flag means the agent is enabled in the VM config, not that the in-guest service
answers, so a successful VM-status read does not establish or renew it. A later
poll that preserves optional IP/OS/version strings does not renew that evidence.
Internal `GuestAgentEvidence` crosses model, canonical and previous-state
snapshots, including an explicit missing origin. Legacy direct snapshots import
their receipt once, while already annotated disk-only evidence uses its recorded
filesystem origin, never a linked-agent origin or renewed VM receipt. Expired,
missing and future origins cannot admit a fallback.
The existing ten-minute boundary, authoritative unavailable status, metadata
backoff and every-command backup-lock/shared uncertainty fences remain intact.
Cached display identity remains available, but cannot upgrade a failed/empty
refresh to current availability: a fallback needs a usable filesystem or useful
metadata observed in that cycle to claim availability. Linked Pulse-agent disks
remain independent.
`TestGuestAgentAdmissionEvidenceContract` checks the original-age boundaries,
repeated canonical replacement and both cluster/node collectors through a
cancelled detail cycle, unavailable status, missing status and ordinary recovery
with measured-zero disk usage. Its `config-flag-cannot-renew` case polls with the
real integer `agent: 1` flag: an enabled agent that never replied admits no
fallback command once status fails, while one that answered keeps its fallback.
This is synthetic source proof, not native thaw,
QEMU-only Windows acceptance or reporter resolution.


### QEMU disk fallbacks retain their original filesystem source time

A successful QGA filesystem read owns the disk observation's source and original
receipt time. Internal model, canonical facet and previous-state copies preserve
that evidence. Repeated polling, cached OS/version identity and an optional
metadata failure cannot renew the existing ten-minute carry-forward window.
Recent disk-only readings remain eligible without optional metadata; expired,
future, missing or non-QGA origins do not. A legacy direct reading may use its
receipt once; an already unavailable/retained legacy value with unknown origin
cannot borrow the latest poll time. A new ordinary successful read restores the
observation, including measured zero. Linked Pulse-agent disks retain their own
source and precedence, not a fabricated QGA origin. Guest-command admission,
backup safety, History and alert missing-observation rules remain unchanged.

`TestGuestDiskObservationContract` covers original/legacy age boundaries,
repeated canonical replacement, disk-only continuity, real-client deferral and
expiry in cluster/node collectors, cancelled detail, current zero recovery,
unchanged prior History and alert continuity. These are synthetic source checks,
not native thaw, QEMU-only Windows acceptance or completion of #2439/#2619.

### Automatic Proxmox agent attribution preserves distinct FQDNs

Hostname-only attribution uses the common full-hostname equivalence rule:
case, surrounding whitespace and a DNS root dot are normalised; a short name
may match an FQDN, but different full domains never identify the same machine.
A bare name matching multiple provider nodes/guests stays unlinked. Explicit
endpoint and unique provider-network hints retain their existing precedence.
The connection FQDN and Pulse display name are not guest-hostname evidence.

`TestFindLinkedProxmoxEntityPreservesDistinctGuestFQDNs` covers node, VM and LXC
selection, a sole unrelated domain, ambiguous short names and legacy short/full
pairs. `TestCrossInstallationIdentitySurvivesStandaloneAddition` adds FQDN
connections with overlapping native node names/VMIDs and distinct guest domains,
checking original node health, scoped backups, Docker inventory and agent link
continuity. These are source controls, not the established reporter cause or
native three-surface recovery for #2681.


### PMG saved collection scope and incomplete observations

The PMG poller snapshots the selected connection's saved scope once per poll.
Mail statistics, mail-count trends and spam-score reads follow `MonitorMailStats`;
each discovered node's queue read follows `MonitorQueues`; both quarantine
category reads follow `MonitorQuarantine`; domain reads follow `MonitorDomainStats`.
Disabled datasets make no requests and supply no current observation. Version,
cluster discovery and configuration-backup reads remain independent of these
four switches; pausing the connection prevents every subsequent ordinary poll.
An edit or pause does not revoke an already running poll's snapshot.

A successful version read still establishes online connection health, not
complete collection. Downstream read failures are retained as failed poll
evidence, with missing fields instead of synthetic zero readings. Quarantine
totals require both category reads. Collector controls in `monitor_pmg_test.go`
cover sixteen scopes, next-poll edits/pause/resume, in-flight snapshots, each
read failure, recovery, legacy defaults and cancelled/unknown connections.
These synthetic controls are not native PMG or installed privacy acceptance.

The PMG client rejects omitted/null object payloads for version, mail, queue
and quarantine instead of materialising them as zero structs. Explicit
successful zero objects remain observations. `TestPMGCollectionMissingObjectData`
and `TestPMGCollectionObservedZeroData` in `pkg/pmg/client_test.go` exercise the
real request/decoder boundary using an in-memory HTTP transport, with no listener.

### PMG 9 cluster ids and backup node identity — issue #2960

PMG 9 returns the cluster status `cid` as a string. `pmg.ClusterStatusEntry`
no longer decodes the field, which Pulse never read, so no `cid` format can
fail the cluster read and hide the node list. This supersedes the older typed
integer decoder; node name, type, IP and fingerprint remain unchanged.
The containing poller's existing completeness boundary stays strict: failed,
empty or unnamed cluster enumeration cannot authorise node-specific reads.
A connection display label is never substituted for a discovered node hostname.
Failed enumeration retains the existing failed-collection and backup-preservation
semantics rather than fabricating a successful empty inventory.
`TestClusterStatusIgnoresUnusedCID`, `TestClientClusterStatusIgnoresClusterIDFormat`
and the `TestPollPMGInstance*` issue tests preserve the upstream string-id repair
and the containing no-guessed-node boundary. `TestPMGDiscoveredNodeRouting`
also accepts unused textual ids through the full poller;
`TestPMGNodeDiscoveryFailurePreservesAcceptedBackups` uses a malformed consumed
node name, not an ignored id, to exercise decoder-failure preservation. These are source regressions,
not native appliance recovery or release availability.

### PMG backups keep last-known entries for unread nodes

`UpdatePMGBackups` replaces a connection's whole backup list, so only a node
whose backup list was read in this poll is authoritative for it. A node that
was not read keeps its previous backups: every node after a failed cluster
read, a node whose backup read failed, and a node a cancelled poll never
reached. This is the same per-scope carry that `preserveFailedStorageBackups`
applies to failed PVE storages. A node that answered with no backups, or that
a successful cluster read no longer lists, still clears its entries. The
failed read is still recorded as a poll failure; the stricter containing
no-guessed-node boundary above remains unchanged. `TestPMGBackupsSurviveFailedClusterRead`,
`TestPMGBackupsSurviveEveryNodeBackupReadFailing`,
`TestPMGBackupsKeepFailedNodeOnPartialRead` and
`TestPMGBackupsClearAfterCompleteEnumeration` in `monitor_pmg_test.go` cover
the carry and the complete-enumeration clear, including a successfully read
empty node beside an unread node. Discovery validation and unread-node carry
use the same captured cluster-read error; later read errors cannot change
which nodes were enumerated. The containing update publishes the combined
per-node inventory, not an all-or-nothing replacement after any failed read.
These are synthetic poller controls, not native PMG acceptance.

### Guest configuration reads preserve installation and node identity

Resolving omitted placement for the existing guest-config read honours every
supplied instance/node constraint and requires one distinct current placement.
A bare VMID or node/VMID shared by installations or missing placement returns an error without reading a provider. Current canonical
inventory, including an empty inventory, takes precedence over stale state.
Explicit complete placement still reads that exact provider and preserves its
original error. This read neither dispatches QGA nor changes operation locks,
backup policy, guest-command admission or permissions.

`TestGuestConfigPlacement*` in `monitor_additional_test.go` verifies VM/LXC
placement constraints, both inventory orders, zero-provider ambiguous/missing
reads, canonical versus stale state, direct/provider-failure preservation and
concurrent snapshot publication. These are synthetic config-read controls, not
native recovery or a diagnosis of #2681.


### Automatic system-mount exclusions respect directory boundaries

The shared filesystem filter excludes a named system directory and its
descendants, not independent local mounts with the same text prefix. Volumes
such as `/snapshots`, `/runtime` and `/var/lib/docker-data` remain eligible for
capacity, current usage and ordinary alert evaluation without an explicit
include override. Genuine system directories, virtual/read-only/remote types
and container layers remain excluded. Explicit operator wildcard/exact excludes
and include precedence are unchanged; guest commands and their safety admission
are untouched.

`mount_boundary_test.go` in `pkg/fsfilters` and `internal/hostmetrics` cover the
directory boundary, real pre-usage collector admission and skip/override
controls. `filesystem_mount_boundary_test.go` carries local mount counters
through QGA aggregation and both report/state/canonical-read paths, and host
usage alert onset/recovery. These synthetic controls do not establish native
guest polling, installed agent recovery or completion of a reported incident.

### QEMU Windows capacity is owned by the volume, not the physical drive

The guest filesystem aggregate uses the reported Windows volume GUID when
present, or a normalised drive/mount path without one. QGA `disk.dev` names
the backing `PhysicalDriveN`, so equal-sized partitions on that drive must not
be treated as one filesystem. Multiple mount paths for one GUID still count
capacity once; each admitted mount row and its original device metadata remain
visible. Non-Windows device/subvolume deduplication, byte-validation and the
signed aggregate-overflow fence are unchanged. These temporary capacity keys
are not hardware identity, guest identity or History keys.

`guest_filesystem_identity_test.go` checks separate equal-size volumes, alias
mounts, missing GUIDs, Linux bind/device peers and overflow. Its synthetic
QGA-only polling lifecycle is intended to check complete capacity, volume removal
and explicit zero through the VM builder, canonical read state and both
in-memory/persistent History. Executing that lifecycle is separate from the
non-listening capacity checks: fixture presence or compilation alone does not
establish polling/History acceptance, native Windows/PVE recovery or a diagnosis
of issue #2619. The optional-read lifecycle fixture also carries the same
captured mock-mode scope through poll publication and broadcast after the
mode-fence API change, rather than recapturing an epoch for stale state. Guest
commands, operation locks, shared admission, retries, timeout and backup policy
are unchanged.

### Sustained poll failures preserve the capped retry fence

Scheduler circuit-breaker backoff saturates before duration arithmetic can
wrap. Every failure episode retains the existing threshold, exponential
sequence up to the configured cap and single half-open probe window; prolonged
failure cannot turn that cap into an immediate retry or a past retry timestamp.
A successful poll still resets the failure count, interval and outstanding
error, and one connection's failures do not block another connection.

`TestCircuitBreaker_BackoffNeverOverflows` compares 130 failure transitions
against an independent unbounded-integer oracle, including odd and maximum
representable duration caps, exact retry boundaries and success/new-episode
recovery. `TestMonitorPersistentFailureBackoffAndRecovery` carries persistent
failure through actual scheduler result/admission and health entry points,
including unrelated-instance progress and cleared error state. These are
in-memory source controls, not native guest recovery, appliance request repair
or installed CPU/network relief. No database or provider operation is involved.

### Active-alert state projection carries the live metric status — issue #2068

`activeAlertsSnapshot` copies the alert manager's volatile `MetricStatus`
into `models.Alert.metricStatus`, so the websocket and `/api/state` alert
lists carry the reading being evaluated now for a threshold alert held below
its trigger, while `value` and `message` keep the last breach. The projection
adds no copy of its own: `GetActiveAlerts` already returns deep clones.

The optional status field `lastBreachAt` carries the held breach's original
`LastSeen` through that same projection, without publishing the poll-varying
`LastSeen` of other lifecycle alerts. `observedAt` still dates the current
comparison. `TestSyncAlertsToStateCarriesLiveMetricStatusOfHeldAlert` checks
the concrete frontend JSON as well as the in-memory held reading, with an
independent breach date and omitted legacy `lastSeen`.

### Fresh broadcast projection owns one resource/target capture

Canonical live, fallback and standalone-host continuity broadcast reads prefer
the adapter's detached bulk resource/metrics-target capture. After host
coalescing selects presentation identity, missing history targets are attached
from that same capture, including authoritative absence. Stores without this
optional capability retain the previous point-resolver fallback. Only the owned
outer coalesced slice is decorated in place; nested source payloads stay read-only.

Every read still observes complete current resource data and derives live alert
health, metadata URL edits/clears and connected/ignored infrastructure. No
timestamp-only cache, telemetry omission, policy change or new wire field is
introduced. All existing catalogs, discovery/history coordinates, original
observation times, LastSeen, identity and tenant/revocation boundaries remain.

`broadcast_metrics_snapshot_test.go` compares the complete served JSON with an
independent earlier projection across the metadata-heavy nine-provider demo,
typed-view and same-time dirty cases; it also covers captured target replacement
and continuity/metadata/live-alert boundaries. `monitor_host_agents_test.go`
retains its one-materialization and live-change regression. Benchmarks include
the exact parent's complete conversion and final JSON encoding, with cold,
list-only, typed-clean and dirty inputs; they do not establish installed CPU/RSS
relief, native identity refresh throughput or completion of #2199.

Complete-content comparisons use one health-evaluation instant through a
private clock seam. Production still samples real UTC at the same post-read
boundary. The minute-boundary control requires age labels to advance without
renewing LastSeen; comparisons retain every health field instead of stripping
the reproduced `1m`/`2m` clock difference from JSON.

### Report identity lookups do not rebuild unrelated Kubernetes metadata

The producer's two temporary host/Docker-host admission registries use a
source-locked detached lookup snapshot. It omits only twenty explicitly named
Kubernetes metadata collections before cloning; clusters, nodes, pods and
controller workloads, every non-Kubernetes source and all other snapshot
fields remain. The ordinary registry resolves identity and freshness on each
lookup, including the post-per-host-lock read; there is no generation/time
cache, ID-hint shortcut or changed report watermark/credential policy.

These temporary registries are never publication or alert inventories.
Accepted-report refresh, normal GetSnapshot/GetState, canonical reads and
broadcasts still ingest every collection. Exact source-record point reads
clone only the selected native host, after the existing canonical Docker ID
resolution. Missing native records allocate nothing. Host clone ownership also
includes mutable agent modules, update status/times and config fingerprints.

`agent_lookup_test.go` compares every field of both canonical host views with
the previous whole-snapshot registry, exercises current replacement/removal,
and carries serialized host/Docker reports through complete native and
canonical publication. Allocation controls pin independence from unrelated
metadata/peer-host payloads; paired unchanged-reference benchmarks measure
this lookup, not fleet CPU/RSS relief or all of #2199's remaining mechanisms.

The metadata allocation control measures the detached lookup-snapshot clone
in an isolated invocation of the same test binary, retaining the three samples
and baseline-plus-ten limit. The full-snapshot clone is a positive control for
copying the same metadata. Complete lookup snapshots and every admitted
resource field must match between zero and 1,000 unrelated metadata objects;
only the independent registry construction timestamp is excluded. This keeps
fresh-registry allocation variation and other tests' asynchronous allocations
from masquerading as a metadata-scaling regression. Whole host-view,
replacement/removal and complete accepted-report publication oracles remain.


### Guest memory observations are source-owned

QEMU and LXC memory carry an optional `observation` with `state` (`current`,
`last-known`, `unavailable`), canonical `source`, and original RFC3339
`observedAt` when known. Normal TTL cache hits keep the original read time.
Repeated QGA/backup deferrals are last-known, never renewed by a poll or
diagnostic RetrievedAt; expired/missing usage and powered-off guests expose
unavailability without inventing a read time. Legacy unannotated values make
no freshness assertion. Independently selected PVE status/listing or linked
Pulse-agent memory remains distinct from disk or QGA collection state. Agent
receipt time belongs to its source status, not a guest row refreshed by PVE.

Annotation preserves numeric selection, capacity, retention, guest identity,
agent admission, alert thresholds and existing History recording. It grants
no live-check authority or proof of thaw. Existing legacy state, raw Proxmox
facets and selected canonical memory metrics preserve the discriminator;
selected metric provenance must not be borrowed from another merged facet.
`TestGuestMemoryObservationContract` exercises poll/cache/deferral/expiry/
resumption through unified and served conversions; wire and source-selection
controls cover omission, cloning and independent live agent data. These are
synthetic source proofs, not native QGA/HAOS acceptance or delivered relief.

Mock-mode guests carry the same provenance so the UI qualifies mock readings
as it qualifies live ones: running VMs read `guest-agent-meminfo` (Linux only),
`agent` or the cache-inclusive `status-mem` with no cache split, running
containers read `cluster-resources` or `agent`, and stopped or scenario-stopped
guests are `unavailable` from `powered-off`. A current mock observation's
`observedAt` moves with each refresh, static-metrics mode included, and a
retained one keeps its own state, source and time
(`TestGeneratedGuestMemoryProvenanceMatchesPowerStateAndOS`,
`TestMockGuestMemoryObservationFollowsScenarioPowerState`,
`TestFixtureRefreshMovesCurrentGuestMemoryObservationTime`). This is fixture
realism, not a change to live source selection.

The per-node LXC collector that runs when `cluster/resources` is unavailable
(`monitor_polling_containers.go`) annotates through the same
`guestMemoryObservation` builder as the cluster path, passing the selected
source, the listing's `ObservedAt`, the previous snapshot and the linked agent.
A running container therefore reads `current` from `cluster-resources` (or
`agent`) at the listing time and a stopped one `unavailable` from
`powered-off`, rather than an unannotated value the UI qualifies as freshness
unknown for every container on that node
(`TestPerNodeLXCMemoryObservationMatchesClusterPath`). The fallback's numeric
selection, stabilisation and alerting are unchanged.

Low-trust memory stabilisation uses the original memory observation time for
its existing two-minute reconnect and ten-minute healthy-guest windows, not
the diagnostic poll's `RetrievedAt`. Repeated carried-forward polls cannot
extend either window. Missing/future original times and explicit unavailable
or incomplete observations cannot supply fallback usage. A legacy direct
reading can still use its own receipt time; a legacy `previous-snapshot`
without original provenance cannot acquire another lease from a new poll.
Expiry leaves the independently selected current source in place (including
unavailability when that source has no usage); it neither queues QGA commands
nor changes cache/admission, current PVE/agent selection, alert thresholds or
History's rejection of retained observations.
`TestGuestMemoryCarryForwardExpiryContract` covers the original-age boundaries,
repeated polling, real-client lock-unverified deferral, canonical served values,
in-memory History, independent PVE readings and ordinary QGA read recovery.
The existing cluster/node preservation tests use an original source timestamp;
these fixtures do not establish native thaw, HAOS/Windows readings or shipment.

### Unavailable backup-age evaluation is not silent recovery

PVE, PBS and mock backup checks share one failure-preserving evaluator. The
existing two-second complete rollup-read budget includes connection waiting;
it is not extended or bypassed. A failed or partial read does not invoke backup
age evaluation with an empty result, clear existing age alerts or assert backup
health. A fixed system warning makes the skipped evaluation visible in the
existing alert list and served state, with no raw database/provider/path error.
A complete subsequent evaluation clears that warning; normal cancellation or a
missing manager does not stand in for read recovery. Deliberately disabled checks
clear their irrelevant warning without reading recovery history.

Overlapping global checks coalesce instead of queueing. The single evaluation
owner orders failure/recovery publication so an older success cannot erase a
newer failure. `backup_alert_evaluation_test.go` exercises actual persisted
rollups, stale/fresh backups, read failure/reopening, served-state visibility,
repeated failures, missing dependencies, cancellation/deadlines and concurrent
coalescing. `TestBackupAlertEvaluationCallersShareFailureVisibility` binds all
three callers to that path. These are synthetic source controls, not native
backup thaw, Raspberry Pi relief or published availability.


### Platform connection discovery follows tenant storage identity

TrueNAS and VMware pollers enumerate validated, sorted persistence directory
keys, including `default`, independently of `org.json` metadata. A missing,
malformed or foreign metadata ID cannot prevent a saved connection from
acquiring a provider, redirect its config read, or block authoritative
disable/removal in an unrelated tenant. Configs, credentials, runtime statuses
and discovered records remain keyed by that storage identity and connection
ID. Existing path validation, disabled-connection pruning, encrypted config
loading and poll intervals are unchanged. A directory-list failure still
returns no partial inventory; a connection-config read failure does not gain
an old-credential fallback. Metadata-dependent account and authentication
consumers retain strict organization loading: storage discovery is not
authorization, membership or a substitute for request tenant resolution.

The storage-key, TrueNAS and VMware discovery tests pin malformed/absent IDs,
metadata redirection and unrelated corruption, plus disable/removal without
additional polling. API controls use the actual saved Test, periodic poller
and redacted list response: a probe cannot manufacture runtime evidence, and
ordinary polling supplies transport, attempt/success and observed inventory.
These synthetic source controls reproduce a family matching #2382's second
operator response, not that operator's cause, the original downgrade-resolved
regression, native appliance acceptance or release availability.

### Metrics database override stays on the default organization

`PULSE_METRICS_DB_PATH` names one SQLite file, so it applies only to the
default organization's monitor. `MultiTenantMonitor.GetMonitor` clears
`MetricsDBPath` on each non-default tenant's detached config copy after
setting its `DataPath`, and `monitoring.New` then opens that organization's
store at `<org data dir>/metrics.db`. Before this, the deep copy carried the
override into every tenant and all organizations wrote to and read from one
metrics database, so per-org charts, reports and Pulse Assistant summaries
could return another organization's series. Keeping tenant stores inside the
org directory also means `DeleteOrganization` removes that org's metrics with
the rest of its data. The trade-off is that tmpfs or RAM-disk placement
through the override covers the default org only; non-default org metrics
stay on the data volume. The default org's override path, rollup interval and
retention handling are unchanged. Regression coverage:
`internal/monitoring/multi_tenant_monitor_metrics_path_test.go` sets the
override, provisions two orgs, and proves each opens its own `metrics.db`
and cannot read series written by the default org or by the other org.
`TestMetricsStoreRuntimeOverridesStayOnMonitorBoundary` also requires the
clear to sit between the tenant `DataPath` assignment and `New`.

### Lightweight detached History reads

Guest, node, disk and storage History readers allocate one exact-sized snapshot
per populated metric series, rather than repeatedly growing temporary arrays.
Empty series remain non-nil arrays for the existing JSON representation. The
copy preserves the exclusive time cutoff, original values, timestamps, order
and duplicates, including non-chronological input; it never aliases mutable
retained History. Single/batch chart and metric-window consumers retain the
same observations. Retention, sampling, source authority and SQL are unchanged.
`metrics_history_read_test.go` covers every public series, caller mutation,
cutoff/empty/unordered semantics and connected poll-to-chart/window readers.
Allocation controls and paired same-executable in-memory benchmarks measure
copy cost, not native CPU/RSS relief or attribution of an existing report.

### Used-memory history retention

Periodic in-memory History cleanup applies the existing retention window and
backing-array release policy to raw `memoryused` bytes as well as `memory`
percentages, for guests and nodes. A live byte series alone keeps its resource
entry; an expired byte series cannot remain pinned by ongoing CPU or other
observations. Missing current memory does not refresh its last-known samples.
The existing poll recorder, chart readers and metric-window provider retain
in-window byte values and their original timestamps through cleanup, then
accept fresh memory on resumption. SQL retention, sampling limits, memory
source selection and wire formats are unchanged. The History cleanup tests
cover every supported series, byte-array reclamation and connected readers;
these controls do not attribute a reporter's native process-memory growth.

### Mock chart history keeps one timeline per resource

Mock chart history thins every series of a resource onto one wall-clock grid
(`sampleMetricSeriesOnMockChartGrid` in `internal/monitoring/mock_chart_history.go`,
called by `downsampleMetricSeriesForMockChart`). It keeps the first reading in
each epoch-aligned bucket and the newest reading, each at its own timestamp, so
which readings survive depends only on when they were taken, never on their values.
The poller writes all of a resource's metrics at the same times, and the drawer
History panels break a line where another series in the panel has a reading this
one lacks; per-series LTTB picked value-dependent timestamps, so mock guests and
nodes drew as dots with "Missing observations" on the public demo. Live poller
history, the store query path, chart point targets and mock values are unchanged.
`TestMockChartHistorySharesTimestampsAcrossMetrics` in `monitor_metrics_slo_test.go`
pins identical cpu/memory/disk/network/disk-I/O timestamps for a mock VM, container
and node, on synthetic and seeded history, at 1h, 4h, 24h and 7d.

### Podman interval CPU selection — issue #2447

Every Podman one-shot response uses consecutive cumulative CPU-time nanoseconds
over elapsed wall-clock nanoseconds. The compatibility `cpu` percentage is not
bound to that interval and cannot replace it, whether positive, zero, missing or
invalid; measured idle remains zero. First samples, counter resets and container
start changes establish a baseline instead of presenting lifetime CPU as current
usage. A new lifetime also rebaselines across a backward clock; duplicate or older
reads in the same lifetime cannot move that baseline. Missing timestamps cannot
invent an interval. The collector keeps the 100%-per-core scale (#1391), and the
server converts exactly once to host-capacity percent for History and alerts.
Docker's system-counter calculation, memory, report shape and permissions stay
unchanged. `Container CPU selected for report` logs the final selected raw value,
not a preliminary calculation or a claim of successful delivery.

`TestCollectContainerPodmanCPUIntervals` covers decoding, idle, restarts, backward
clocks, duplicates and Docker isolation. `TestPodmanCPUCollectorHistoryAndAlerts`
in `internal/dockeragent/cpu_roundtrip_test.go` follows deliberately conflicting
synthetic percentages through real compressed HTTPS report delivery, scoped
authentication, ingestion, memory and persistent HTTP History, and alert identity
and units on 8- and 4-CPU hosts. Genuine high usage still breaches its configured
threshold; no clipping or smoothing masks spikes. These source controls do not
establish the reporter's native payload, installed cause or containing release.

### Filesystem collection precedes optional guest reads — discussion #2538

Filesystem byte observations do not default absent, null, negative or
fractional usage to a healthy zero. Positive capacity requires an explicit
integral used-byte count within capacity; counters and the deduplicated guest
aggregate must fit the signed disk model. Numeric strings, integral decimal
and exponent forms, privileged-total fallback, genuine zero/full readings and
Windows mount/device metadata retain their existing support. Exact integer
bytes do not round through a JSON float, including object-style replies.
Ambiguous duplicate/case-conflicting byte fields are not usage evidence.

Malformed array rows remain isolated from usable peers. If no usable reading
survives, the existing unavailable/previous-reading path remains in force.
An unrepresentable aggregate is unavailable, not a wrapped or silently partial
total; invalid counters are not labelled as special mounts. These completed
replies do not alter command admission, locks, retries, cooldown or backup
behaviour. `guest_filesystem_bytes_test.go` in both Proxmox and monitoring
covers exact counters, wire/object/peer admission, signed arithmetic and an
ordinary later poll with an explicit healthy zero. This establishes source
reading integrity, not native QEMU-only recovery or independent thaw.

Both the detailed-status and recent-evidence/no-status QEMU builders collect
filesystem usage before optional Linux `/proc/meminfo` and guest metadata.
Windows/Android guests may answer filesystem queries without supporting every
optional command. A completed, verified filesystem response can therefore reach
the read projection and History even if a subsequent optional command starts a
shared pause. Memory and metadata helpers return admission deferral separately
from observations: a successful disk must not hide later uncertainty.

No completion-error exemption, operation-lock requirement, timeout, retry,
failover or process-local coordination rule changes. An unknown optional reply
still blocks every later command, including newly constructed clients and cold
collectors in the same process. Subsequent paused polls retain old values without
adding disk/memory History or renewing original memory evidence. Explicit
unsupported replies retain their existing terminal/error and negative-cache
semantics. Independent current provider counters remain usable. This does not
establish the reporter's original failed command, supported native filesystem
response, process-restart safety, thaw or installed/released repair.

`TestGuestAgentOptionalReadOrderingContract` drives actual single/cluster clients,
normal and recent-evidence/no-status builders, unified JSON readback and memory,
persistent and chart History for NTFS/ext4 success, terminal unsupported memory/OS
and uncertain memory/network/OS/version replies across repeated polls and cold
collectors. Existing backup, lock, transport and observation contracts remain
required; all responses in this fixture are synthetic.

### Guest-agent coordination around backups — issue #2439

A completed OS-info outcome keeps its suppression semantics even when
the later agent-version read is deferred. Unsupported OS-info still disables
that optional command; ordinary completed command failures still accumulate
towards the existing three-failure limit, and a supported reply resets that count.
Actual HTTP 401/403 or exact command-bound stopped-agent rejections did not
exercise OS-info and therefore do not accumulate towards permanent suppression.
They preserve prior command failures and normal refresh/backoff timing; they
neither renew retained identity nor establish a Windows observation. Once ordinary
access/agent availability returns, an eligible accepted OS reply can recover
the identity and cross-platform memory classification without a Pulse restart.
Quoted status/diagnosis text does not grant this distinction. Unsupported,
repeated completed-failure and uncertain-command safety suppression is retained.
Only these safety fields are retained on version deferral: partial network/OS
metadata is not published, old identity is not made fresh, and the shared pause
and configured retry backoff remain in force. A network or OS-info deferral
does not establish another completed OS-info failure.

Metadata-cache cleanup judges inactivity separately from the last accepted
useful observation. A guest that is still being polled keeps its OS-info failure
count and unsupported-command suppression even when network/version replies
are empty, fail, or are deferred. Cache access is process-local bookkeeping:
it cannot renew useful metadata or OS evidence, make retained Windows identity
current, satisfy fallback eligibility, change backoff, or admit a guest command.
The janitor never evicts an in-flight fetch. Idle cache/access entries still
expire after ten minutes; legacy entries use their original useful timestamp.
Explicit unavailable-agent invalidation and instance retirement remove access
bookkeeping while retaining outstanding fetch ownership.

`TestGuestMetadataCleanupSuppressionContract` exercises both ordinary collectors
with single/cluster HTTP clients, cold/retained caches, unsupported and repeated
completed failures, a janitor pass between polls, and network/version recovery
without repeating the suppressed OS request. The idle/invalidation/concurrent
cleanup contracts in `monitor_host_agents_test.go` verify bounded idle eviction,
in-flight ownership, cross-instance isolation and unchanged observation ages.
These are synthetic source controls, not native guest/backup recovery or proof
of the cause or resolution of #2439, #2619 or #2822.

`guest_metadata_suppression_test.go` covers cold and retained caches, threshold
accumulation, supported reset and earlier deferrals. Its real HTTP fixture
drives cluster and node poll entry points through a lock appearing before the
version request, then ordinary disk/network/version resumption without another
suppressed OS-info request and with fresh disk History. It changes no native
guest, backup policy, lock admission, cooldown, retry or single-attempt rule.
These source controls do not establish native recovery or a reported cause.

`TestGuestMetadataRejectionRecoveryContract` verifies typed rejections with cold
and retained caches, prior genuine failures, unchanged backoff and fresh OS
receipt requirements. Both ordinary collectors and single/cluster HTTP clients
recover Windows OS, individual NTFS volumes and disk History after a stopped-agent
fixture, without dispatching Linux-only memory reads on the recovered Windows
guest. Existing suppression, backup-lock and shared uncertainty contracts remain
required. These synthetic source controls do not establish native restart/thaw,
the cause of #2822/#2619, or installed/released recovery.

The process-local admission table remains capped at 4,096 endpoint/VMID
entries. Expired entries replaced by a new admission consume no extra slot;
when new aliases would exceed the bound, expired idle entries are reclaimed
before returning `agent-capacity`, even before the periodic cleanup deadline.
In-flight commands and unexpired uncertainty cooldowns are never evicted,
shortened or renewed to make space. An alias group is reserved atomically or
not at all; full protected capacity still defers before any HTTP request.

`pkg/proxmox/guest_agent_capacity_test.go` covers near-limit expired alias
replacement, immediate expiry reclamation through all six HTTP readers,
backup-lock rejection and ordinary resumption, plus fully protected capacity
and incomplete group admission. These are synthetic source controls, not
native recovery or a diagnosis of a reported shared pause. Existing lock,
no-replay, cooldown, endpoint/VMID isolation and restart limitations remain.


Filesystem-query diagnostics preserve this safety boundary. Missing, empty,
unsupported or refused readings do not prescribe installing, activating or
restarting QGA, changing backup settings, re-running setup or broadening a
shared Proxmox role. Operator logs identify the existing guest and fixed reason,
lead to guest-local disk checks, and keep existing guest-agent/backup settings.
An actual HTTP 401/403 leads only to checking the existing credential's access
to that VM. A stopped-agent reason requires exact command-bound rejection
evidence, not an HTTP 500 or a status/diagnosis quoted in provider text.
Unsupported commands remain errors without claiming that QGA is stopped.
Untyped local timeout/deadline errors keep their conservative timeout reason;
this diagnostic correction does not turn local uncertainty into recovery.
Deferrals retain their fixed reasons and do not gain new diagnostic commands.
`TestGuestFilesystemFailureGuidanceUsesObservedEvidence` crosses the real
client/collector/log boundary and preserves unavailable disk sentinels and
single-attempt uncertainty. The error-reason controls preserve original error
text/causes and terminal-rejection admission. These synthetic diagnostics are
not independent thaw, every-filesystem writes or native backup recovery.

All PVE QEMU guest-agent reads, including disk diagnostics and legacy meminfo
callers, share fail-fast per-guest admission within this Pulse process. Clients
for the same endpoint and configured cluster aliases coordinate across methods;
VMIDs remain serial through node migration. Other VMs and independent endpoints
remain independent. No guest payload or credentials enter coordination state.
Coordination keys treat DNS hostname case, numeric/default HTTP(S) ports,
equivalent IPv6 spellings and unzoned IPv4-mapped spellings of the same numeric
IPv4 address as the same channel. The mapped-address equivalence applies only
to guest admission, not saved certificate-pin identity or first-use trust.
Compatible IPv6, NAT64, DNS aliases and zoned addresses are not unmapped.
Alias registration
uses that same key, including when a standalone request is already in flight
or two configured alias groups are joined. Scheme, non-default port, base-path
case, IPv6 zone and VMID still separate independent sources. No DNS lookup or
unconfigured cluster inference is performed. Request URLs, credential scope,
TLS/proxy policy and wire Host headers are not rewritten or shared.
`pkg/proxmox/guest_agent_endpoint_test.go` binds these keys to all six actual
HTTP readers: a streamed in-flight body and an uncertain completed transport
cannot queue follow-up commands through another spelling. Other VMs, independent
origins and known-completed resumption retain normal admission. These source
controls do not establish native QGA completion, thaw or restart protection;
the existing process-local one-minute uncertainty pause is unchanged.
`pkg/proxmox/guest_agent_ipv4mapped_test.go` additionally crosses a real IPv4
loopback listener through both numeric address forms, in both directions,
without a custom dialer. All six read methods share in-flight and uncertainty
refusals; fresh cluster clients and migrated node names retain the same VMID
fence. Other VMs and ordinary non-QGA status remain readable, and refusals do
not renew the original uncertainty deadline. The canonical cluster API tests
keep distinct saved pins and the existing declared-member/no-replacement policy.
Before each command and before returning its fully received bounded payload,
Pulse reads the operation lock from VM config. Any nonempty lock (not just
backup), missing/malformed config or an unverifiable lock defers the command.
Only an absent `lock` or an exactly empty string establishes lock clearance.
A nonempty whitespace-only value, including escaped and Unicode whitespace,
is malformed evidence and returns `lock-unverified`, not an invented unlocked
state or a named backup lock. It cannot dispatch a guest command or publish an
already completed command's payload. A later complete, genuinely unlocked
config resumes ordinary reads without clearing a transport-uncertainty fence.
The lock-evidence matrix covers all six readers at both admission boundaries;
the collector projection control retains known disk evidence without renewing
it, leaves independent status memory usable, and resumes the same guest normally.
These synthetic controls do not establish native thaw or installed recovery.
Rooted and unrooted DNS names retain distinct admission and saved-trust keys:
resolver search suffixes can give them different destinations. No DNS lookup
or automatic alias inference is added to solve lock admission.
Known locks in node inventory, cluster resources or detailed status suppress
polling enrichment even when status fails and recent agent evidence exists.

The VM config body must finish within the existing response-size and request
bounds and contain exactly one complete JSON value. An unlocked JSON prefix
does not establish lock clearance: a short Content-Length body, stalled suffix,
extra JSON/garbage or chunked body exceeding 8 MiB yields `lock-unverified` both
before dispatch and after a completed guest command. Post-command verification
holds per-guest admission until body completion; its failure discards the
payload. Complete responses at the limit, including trailing whitespace, remain
valid. Ordinary PVE CPU/I/O observations remain independent of this deferral.

Lock verification also requires a direct HTTP 200 from the configured command
endpoint and one unambiguous `data` configuration object. It never follows a
redirect, including a same-origin redirect to another configuration path.
Duplicate fields (including escaped duplicates), differently cased `data` or
`lock` fields, competing error/failure envelopes and non-string operation locks
yield `lock-unverified`; a later unlocked value cannot overwrite earlier backup
evidence. Ordinary `GetVMConfig` remains compatible with its existing decoding
and redirects. The lock check shares the configured pooled transport, TLS,
proxy and timeout policy and retains config-read password-session recovery;
the guest command itself is still single-attempt. This narrows the authority
of lock evidence, not the ordinary API read contract. Failed post-command lock
verification discards the guest payload without inventing a transport cooldown;
subsequent same-target unlocked verification permits normal resumption.
`TestGuestAgentLockEvidenceRejectsAmbiguity` and
`TestGuestAgentLockEvidenceRejectsRedirects` exercise all six readers before and
after dispatch, all five redirect statuses, same/cross endpoint redirects,
ordinary config compatibility and fresh-client resumption. The connected
`TestGuestAgentBackupLockEvidenceContract` covers warm-cache continuity, live
CPU, unrenewed observations and History, then normal fresh poll resumption.

Admission is held through the complete response, not just headers. A transport
failure, timeout, wrong-command-ID response or incomplete body defers every
command for that guest for a minute; callers neither immediately retry nor
replay through another cluster endpoint. Coordination retains at most 4,096
active/cooling endpoint-guest entries and expires cooldowns. Explicit API
permission refusals and unsupported commands remain errors, not successes.

A successful guest reply also needs direct HTTP 200 and exactly one complete
UTF-8 JSON envelope with one canonical `data` dictionary. Malformed, missing,
null/scalar data, extra values, duplicate/escaped-duplicate envelope or data
fields, competing error envelopes, or differently cased `data`, `result`,
`content` or `truncated` fields cannot establish a completed reply. The check
runs while admission is held, before method decoding or postflight lock reads;
failure retains `agent-completion-unverified` and the existing one-minute
VM-wide uncertainty pause. Other methods, fresh clients and configured cluster
aliases cannot immediately follow it with QGA work. Unexpected 2xx statuses
(including accepted/no-content/partial responses) have the same boundary.

Valid empty dictionaries, null/empty results, object-style rows, Unicode and
trailing whitespace retain existing method semantics, including partial row
admission and known truncated-file rejection. Ordinary API reads, permissions,
budgets, trust and backup settings are unchanged. The six-reader wire controls
pin no immediate redispatch, independent live counters, alias/VM/source isolation
and fresh admission after cooldown. `TestGuestAgentSuccessEnvelopeMonitoringContract`
connects malformed, duplicate and prefix-only replies to real polling builders:
original metadata/memory timestamps and guest History are not renewed, labelled
last-known values remain and current CPU advances. These checks establish reply
consistency, not native payload-to-command provenance, thaw, cross-process
coordination or restart safety. The native recovery precaution below remains.

The same pre-release check validates the types of present canonical reply
fields: `result` is an object, array or null, `content` a string or null, and
`truncated` a boolean or null. Primitive results and wrongly typed file fields
cannot become empty filesystem/network inventories or release admission before
a file decoder reports its error. They retain `agent-completion-unverified` and
the original one-minute fence; blocked follow-ups do not renew that deadline.
Absent/null fields, compatible object/array rows, unknown metadata and completed
but unusable memory content keep their existing semantics. A measurement
rejection alone does not pause ordinary OS discovery.

`TestGuestAgentTypedSuccessDefersEveryRead` checks sixteen impossible field
shapes through all six real HTTP readers, including no postflight/follow-up
request and unaffected live counters.
`TestGuestAgentTypedSuccessSharesUnrenewedFenceAndResumes` checks fresh clients,
cluster aliases, VM/source isolation, unchanged expiry and valid resumption.
`TestGuestAgentTypedSuccessPreservesFieldAndMeasurementSemantics` keeps empty,
null, partial-row, escaped-text and truncated-measurement compatibility.
`TestGuestAgentTypedReplyMonitoringContract` connects all three field kinds to
the real poll builder, broadcast resource and History, with untouched
healthy-read cache and observation origins: original memory/disk remain
last-known with their QGA provenance, metadata is not renewed, CPU advances,
and guest metrics are not renewed. This is synthetic source/consuming-surface proof, not attribution of
the original freeze or native thaw/restart recovery.

The success boundary also checks compatibility with the command sent, before
postflight lock reads or release of shared admission. Present, non-null fields
exclusive to another supported command (file content, OS identity, agent command
catalogue, filesystem capacity or network interfaces), an explicit error payload,
and list results for the two dictionary-valued information commands retain
`agent-completion-unverified`. A recognizable network row must not become a
current filesystem named `eth0`, nor a file/agent-info reply an empty current
disk inventory. The original one-minute uncertainty fence, alias coordination,
no-replay transport, credentials and independent live counters are unchanged.
Empty/null replies, shared `name`/`version` fields, unknown metadata, supported
object/array inventories, partial rows and completed unusable file measurements
keep their existing semantics. This is contradiction detection, not proof of
payload-to-command correlation: replies without discriminating fields remain
indistinguishable, and native thaw/restart recovery remains unverified.

`TestGuestAgentForeignReplyDefersEveryReader` covers cross-command/error replies
through all six HTTP readers, with no postflight/follow-up guest request and
independent PVE/VM controls. Alias/fresh-client/unchanged-expiry/resumption and
compatible empty/partial-data tests complete the client boundary.
`TestGuestAgentCommandReplyMonitoringContract` connects three foreign reply
families through the actual polling builder, state, broadcast and History:
original disk/memory remain last-known, metadata and measurement times are not
renewed, and CPU advances. Source and browser fixture proof do not establish a
native command-ID collision, incident cause, guest liveness or successful thaw.

The completed-error boundary uses the actual HTTP status retained by the request
layer, not an `API error` phrase in provider or proxy text. The supported
explicit request/access/command refusals (HTTP 400, 401, 403, 404, 405, 422 and
429) remain errors, never successful observations. Other client-error statuses,
including conflict/early-request responses and non-standard client-disconnect
statuses, cannot establish command completion merely by belonging to HTTP 4xx.
HTTP 408 and unexplained server/proxy failures (including PVE 595) retain the same per-VM
uncertainty cooldown, even when their bodies finish: command completion is still
unverified. Another method, diagnostic client or configured cluster alias cannot
immediately send a command. Only complete, exact command-bound terminal QGA
rejections under HTTP 500 (unsupported/missing command, explicit stopped agent,
and OS-info's known missing os-release) retain ordinary error handling. Ambiguous
or conflicting envelopes, duplicate keys, other VM/command identities, extra
values and gateway responses quoting a rejection do not gain that exemption.
Existing authentication hints, error text, ordinary API session/retry policy and
endpoint health remain unchanged. Unknown server failures now defer enrichment
for a minute rather than inviting a sequence of diagnostic reads; this is a
safety trade-off, not evidence of a freeze cause or native recovery.
`TestGuestAgentHTTPFailureDefersEveryRead` exercises all six guest readers,
independent status counters and cross-client cooldown; cluster and semantic
controls pin no failover, definitive refusals and conservative error provenance.
The connected transport/History lifecycle and counter-receipt controls also
cover complete server/gateway failures without refreshing old guest evidence.
`TestGuestAgentUnrecognisedHTTPStatusDefersEveryRead` and its alias/resumption
control cover fourteen unrecognised 4xx statuses across all six readers, fixed
wire-status preservation, no follow-up command/config work, independent VM and
endpoint admission, normal post-cooldown admission and unchanged endpoint health.
`TestGuestAgentUnrecognisedHTTPStatusContract` connects conflict, early-request
and client-disconnect responses to polling, last-known disk/memory continuity,
unrenewed metadata/memory caches and History while live CPU advances. These
synthetic checks are not a diagnosis of a frozen guest or native recovery.

Guest commands also prevent transport-level replay: each uses a fresh single-use
HTTP/1 connection, with no pooled-connection or HTTP/2-stream retries and no
redirect following. The client preserves its configured TLS verification or
fingerprint, proxy, DNS dialer, authentication and request bounds. An opaque
transport or custom TLS dialer defers without dispatch rather than substituting
a different trust policy. A fully received redirect discards its body and
starts the same per-VM uncertainty cooldown; its destination is never contacted.
Ordinary config/resource reads retain pooling, HTTP/2, redirects and session
recovery, except that authoritative guest lock checks refuse redirects as above.
This deliberately trades one new connection per guest command for
single-attempt safety; it does not reserve the QGA channel or establish thaw.
Wire controls drop a reply only after receiving a command (including when a
backup lock then appears), and exercise every read and redirect code. TLS
controls retain CA/fingerprint trust, reject untrusted peers and verify command
HTTP/1 isolation alongside ordinary HTTP/2. The connected monitoring controls
retain old cache timestamps and History while live CPU continues through lost
reply/redirect deferral. `TestGuestAgentTransportDeferralPreservesLiveCounterReceipts`
also pins independent current CPU/I/O, explicit zero/null/missing counter presence
and receipt timestamps without clearing guest-command uncertainty.

Deferrals retain the original metadata/memory cache timestamps and supported-OS
failure evidence. Locked guests retain labelled last-known disks/identity and
previous-snapshot memory, or unavailable memory without trustworthy evidence.
Current non-QGA memory, CPU and I/O may still be observed. History never writes
retained disk values or deferred cached memory as newly observed telemetry;
normal unlocked polling resumes and replaces them with fresh data.

Repeated QGA deferrals preserve the complete last-known memory value, including
truly-free and reclaimable cache, using the original successful memory-read
cache as authority. The ordinary 60-second read TTL schedules refresh; retained
evidence remains bounded by the original cache's ten-minute cleanup age.
Deferral polls never renew that timestamp. Missing, failed, future-dated or
expired observations, a different instance/node/VM, changed capacity, or a
previous value inconsistent with the original read cannot support retention.
Current non-QGA memory remains authoritative when available. The retained
value stays `previous-snapshot` and does not produce fresh memory History.
`TestGuestAgentBackupMonitoringContract` includes eight consecutive locked polls
after ordinary memory-cache expiry, checks complete memory and unchanged cache
timestamps/History, and then exercises fresh unlocked resumption.
`TestDeferredVMGuestMemoryRequiresOriginalEvidence` covers the provenance,
age, identity and availability boundaries, including measured zero usage.
These are synthetic runtime proofs; they do not qualify the native freeze race.

`TestGuestAgentBackupMonitoringContract` exercises healthy → status lock →
resource lock with failed status → node fallback → unlocked resumption through
the real client/builders and History. The client controls cover every read,
unknown lock state, diagnostic overlap, aliases/source/VM isolation, in-flight
lock changes, incomplete bodies, cooldown and absence of failover replay.
`TestGuestAgentConfigResponseMustBeComplete` covers every guest read before and
after dispatch and subsequent healthy resumption; the stalled-prefix and
post-config-body controls pin completion and cross-client admission.
`TestUnverifiedVMConfigPreservesStatusCounterObservations` and the monitoring
lifecycle pin live counter presence and rejection of cached disk/memory History
refresh during an incomplete lock read, not just a decoder error.
These reproduce missing Pulse safety controls, **not** the reported native
PVE/QGA command-ID collision or successful backup thaw. HTTP lock checks cannot
atomically reserve PVE's serial channel against a backup beginning afterwards,
cancel work already running in pvedaemon, detect a guest left frozen after its
lock clears, or coordinate separate Pulse processes. Native freeze/thaw acceptance
on a containing build remains required. Until then, stopping Pulse before a
freeze-enabled backup and restarting only after verified thaw remains the safe
workaround, with monitoring and alerts unavailable during that window.

The [bounded backup observation tools](../../../../../tests/qualification/guest-agent-backup/README.md)
provide an independent guest filesystem writer (file/directory fsync and
synthetic readback over non-QGA stdout) and a fail-closed checker for native
overlap, truthful deferred readings/History and post-task writes/resumption.
An OK task without fresh writes on every covered filesystem cannot pass those
record checks. Resumed memory/disk History also needs successful native
`file-read`/`get-fsinfo` dispatches newly started after task completion and its
clock uncertainty, and completed by the resumed readback within the recorded
clock allowance. Old in-flight results, metadata-only reads, fresh timestamps
alone and later polling cannot substitute for those reads. The checker tests
record consistency, not independent payload-to-command provenance.
Local tests are not native freeze/thaw evidence; supplied
platform/artifact provenance and mount completeness still need independent
review. The tools add no runtime interface, recovery action or release gate.


Metadata enrichment owns one fetch per existing `(instance, node, VMID)` cache
key until the whole fetch returns, including slot waiting and early deferral.
The configured hold remains retry timing, not proof that an in-flight fetch
completed. A cold cache cannot override another owner's reservation or an early
failure's backoff. A denied reservation returns only previously retained
identity (or none), without creating fresh evidence, a transport deferral or a
new retry deadline. Other keys remain independent; completion releases only
in-flight ownership and preserves normal refresh/backoff scheduling. Deliberate
cache invalidation on agent unavailability also discards its obsolete scheduling
deadline, so a later available poll can begin a fresh metadata lifecycle; it
never discards an outstanding fetch owner or the shared Proxmox command fence.

Invalidation and PVE instance retirement also cancel that owner's context and
mark its lifecycle obsolete until the fetch actually returns. A late network,
OS or version reply cannot recreate identity, OS-command suppression, useful/OS
evidence or refresh/backoff deadlines after either operation. Publication and
invalidation share one lock order, and fallback metadata is re-read within the
reserved lifecycle rather than copied from the pre-admission cache snapshot.
Slot waiting is cancelled without releasing another fetch's slot. Cancellation
is not proof that a consumed QGA command stopped: the existing shared transport
uncertainty fence still applies to new polling and independent diagnostics.
There are no persistent generation tombstones; the invalidated owner is removed
only on completion. Other instance/VM keys and ordinary new-lifecycle discovery
remain independent.

`guest_metadata_lifecycle_test.go` covers late synthetic replies at every
enrichment stage, cold/retained caches, explicit invalidation/retirement, late
OS-suppression/version-deferral publication, slot waiting and independent
new-lifecycle recovery. Direct and cluster HTTP clients cover cancellation of
consumed incomplete bodies, no further commands, preserved diagnostic/caller
cooldown and independent VM discovery, without shortening or replaying that
fence. These are source lifecycle/safety controls, not native thaw, restart or
every-volume/History recovery evidence.

Guest memory fallback has its own per-instance/node/VM read owner, protected
by the same lock as its short-lived cache. An overlapping refresh defers rather
than dispatching a second read. Explicit metadata invalidation, known agent
unavailability (even while display metadata is fresh), an observed non-running
VM and PVE connection retirement cancel an outstanding memory context and
discard both successful memory samples and negative backoff. A new start or
re-added connection must obtain its own sample. Retirement matches the complete
instance component, not a connection-name prefix. Ordinary backup-lock and
transport deferral retain the original successful sample and its age.

The obsolete read still owns its key until it actually returns. Its late
success, empty result or error cannot recreate a sample/backoff or escape as a
new observation. Cancelled callers likewise do not create a negative guest
sample. Publication and invalidation are serialized; completion releases the
owner with no persistent generation tombstones. None of these operations
releases, shortens or retries the Proxmox transport's consumed-command fence.
Independent VMs, PVE/status memory and linked Pulse-agent evidence remain usable.

`guest_memory_lifecycle_test.go` covers successful/negative cache invalidation,
late replies, overlapping reads, cancelled admission and ordinary recovery.
`guest_memory_lifecycle_http_test.go` covers consumed-body cancellation through
direct/cluster clients, preserved diagnostic uncertainty and independent VMs;
both ordinary collectors publish new restart samples through canonical JSON
and memory History. `TestRetiredQGAMemoryCannotAlterIndependentAgentEvidence`
checks connection-scoped removal and unchanged independent agent reports and
linked memory selection. These are synthetic source lifecycle controls, not
native OS/disks/History recovery, backup thaw or reporter #2762 attribution.

`guest_metadata_reservation_test.go` exercises real HTTP enrichment with cold
and retained caches, active and expired scheduling holds, early unverified-lock
and cancelled-slot exits, independent VMs and ordinary resumption. The existing
shared Proxmox busy/cooldown fence, lock checks and no-replay policy are unchanged.
These source controls establish neither a reported freeze cause nor native
QEMU-only recovery.

The monitor's random source is shared by concurrent guest metadata refresh
jitter and ordinary poll retry backoff. Both accesses, including lazy backoff
initialisation, hold the same dedicated random-source mutex. The limiter mutex
alone cannot protect the source: other guest keys and poll retries also use it.
Keep seeded distributions, configured intervals, half-open jitter bounds and
the existing no-jitter behaviour when metadata scheduling has no source.
`monitor_random_test.go` checks source ownership deterministically, mixed
concurrent seeded/lazy access under the race detector, and real HTTP enrichment
for independent guests with bounded refresh and unchanged command counts.

### Exact TrueNAS subscription termination — issue #2396

JSON-RPC stream readers and the subscription-acknowledgement wait recognise
`notify_unsubscribed` only for the exact requested collection, including its
arguments. An unrelated interval or event source cannot reject the current
subscription. A matching rejection ends the operation promptly, discards the
stream session and is not retried or downgraded to REST. A subsequent ordinary
read may authenticate a fresh session within the existing operation budget.
The notification uses middleware errno fields, not a JSON-RPC response error;
only its numeric errno and a fixed message survive, never provider reason,
trace, extra fields or private collection arguments. Malformed termination
cannot leave a stream reusable. A clean end without telemetry remains
unavailable, not a zero sample; a clean log end returns the bounded collected
tail and retains the socket without cancelling an already-ended subscription.

`TestJSONRPCSubscriptionTerminationRejectsWithoutWaitingOrRetry` covers all
three readers before/after acknowledgement, cleanup, confidentiality and fresh
read recovery. The adjacent termination controls pin exact-collection
isolation, malformed events, absent samples and clean log completion.
`TestTrueNASPollerSubscriptionRejectionKeepsPollingAndRecovers` exercises
three connected protocol/poller snapshots: usable inventory and poll-ledger
progress during rejected nonempty Apps enrichment, native alert disappearance,
then ordinary stats recovery with stable identity. These are source controls
using the reporter's source-derived envelope, not a retained wire capture,
native appliance/incident acceptance or release availability.

### RAID count provenance across ingestion — issue #2369

Agent ingestion, host snapshots and canonical read-state projection retain
`requiredDevices` separately from source-native `totalDevices` and spare counts.
Health compares active members with a known configured requirement, including
explicit zero-active deficits; a spare cannot fill a missing active role.
Collector fallback preserves an explicit zero active count under a known
requirement rather than substituting members from another probe.
Legacy reports retain conservative total-based detection except for an exact
clean/active, zero-failure tuple where total equals working and active plus
spares. That corroborated mdadm tuple is healthy; arbitrary subtraction from
mdstat totals is forbidden. Missing or unreconciled evidence does not gain
that exception. Explicit degradation/failures remain critical and scrub/resync
never hide a member deficit. `TestRAIDRequiredMembersReportAndReadState` pins
the wire/ingestion/snapshot/readback and canonical alert/risk paths; storage
health tests pin legacy compatibility and reconstruction/maintenance boundaries.

### Single-pass live broadcast projection — issue #2199

A frontend broadcast lists the continuity-aware read store once, coalesces host
presentation once, and decorates its owned outer resource slice with current
persisted URLs and freshly evaluated health. Generic conversion callers still
coalesce their own input; an already-coalesced broadcast uses the prepared
projection converter. Conversion sorts final frontend rows beside precomputed
keys instead of retaining an estate-sized array of conversion inputs. This
must preserve canonical identity, parent/action targets, catalogs, ordering,
ignore surfaces and source isolation. No timestamp/generation cache is added:
metrics, tags, alert state, metadata clears and time-sensitive health continue
to be evaluated on each requested broadcast.

`TestBroadcastProjectionMatchesPreviousPipeline` compares full JSON with the
pre-repair conversion/sort/coalesce pipeline on split hosts and mixed workloads.
`TestBroadcastProjectionListsRegistryOnceAndKeepsLiveChanges` pins one registry
list and verifies same-freshness metric/tag changes, live alerts, ignored
hosts and immutable prior projections. The 1,000-resource broadcast benchmark
includes real registry cloning, projection, encoding, per-client deltas and
queues with zero/one/four viewers, not persistence, transport or installed CPU.

### Synthetic machine identity survives restart

Linked mock hosts and Docker hosts derive their machine ID from their stable
fixture host ID. Docker agent IDs are stable as well.
Changing metric samples or rebuilding the graph must not give the same fixture
machine a new canonical agent ID, orphaning saved operator policy. The existing
`TestFixtureIdentityStableAcrossBoots` compares machine IDs per host and the
unified canonical agent IDs, in addition to source IDs and names. This is a
fixture guarantee. Production identity matching must still reject a conflicting
nonempty machine ID rather than transfer policy to a different real machine.

### Canonical registry publication during accepted ingest — issue #2199

The monitor adapter builds a replacement resource generation away from readers.
The previous generation remains readable while change records are classified
and persisted; classification releases registry read locks before any backing
store write can block. Writer ordering, journal classification, active-alert
copying and publication of the replacement generation remain intact. A repeat
of an unchanged snapshot must not append duplicate change records.
`TestMonitorAdapterPopulateFromSnapshotDoesNotRepeatChangeJournal` and
`TestMonitorAdapterSerializesSupplementalMutationAfterSnapshotPublication`
cover the publish and blocked-persistence boundaries. Synthetic performance
improvement alone does not establish field CPU relief.

### One registry snapshot per store refresh pass — issue #2199

Each store refresh pass (`updateResourceStore`, its read-path twin, and so
every accepted agent report) ran five metric syncs and the alert sync, and each
cloned the whole registry through `GetAll`. A pass now wraps the store in
`resourceSnapshotStore`, which clones once and forwards metrics-target
resolution, so every consumer of the pass reads one generation. Stores without
metrics-target resolution pass through unwrapped, keeping the syncs' capability
checks unchanged. `TestAgentReportRefreshClonesRegistryOnce` requires one clone
per agent-report refresh and `TestResourceSnapshotStoreClonesOncePerPass` pins
the wrapper. With node-indexed guest parents, a synthetic agent-report refresh
at 2,080 resources moved from 188 to 82 ms; the pass still covers the whole
estate, and report-driven passes run at most once per 2-second window (see
the agent-lifecycle contract).

### Linked Pulse agent memory for Proxmox LXC — issue #2148 (22 September 2026)

### Linked Pulse agent memory for Proxmox LXC — issue #2148 (22 September 2026)

Both LXC memory paths (the efficient `cluster/resources` builder and the
per-node fallback) may substitute the linked Pulse agent's own memory sample for
a Proxmox fallback listing value. The substitution applies only when the current
memory source is a fallback (`cluster-resources` or `unavailable`), the guest is
`running`, the linked agent sample has known usage, and the agent total matches
the container's provisioned limit with used not exceeding it. A stopped
container, a non-fallback source, a missing linked agent, and a total mismatch
are left untouched; preferred or derived guest evidence is never replaced. The
provisioned platform total is retained, and the previous-guest agent map is
threaded through both builders.

`TestIssue2148LXCPrefersLinkedAgentMemoryOverClusterResources` and
`TestIssue2148LXCRejectsAgentMemoryWithMismatchedTotal` pin the substitution and
the rejection boundary; `TestCorrelatedContainerMemoryNextPoll` pins the
next-poll merged-row path. This is synthetic runtime proof, not a live Proxmox
installation or browser receipt.

### Preserve explicit agent filesystem selection at ingestion

Both `ApplyHostReport` and `ApplyDockerReport` honour a disk's optional
`explicitlyIncluded` marker when applying automatic filesystem filtering.
Selected tmpfs mounts must not be discarded a second time after the agent
has admitted them. Unmarked virtual/system filesystems remain filtered and
ordinary physical filesystem handling is unchanged. Selection evidence does
not confer command authority or imply successful observation beyond the
reported metrics.

`TestAgentReportsPreserveExplicitDiskIncludes` in
`internal/monitoring/monitor_host_agents_test.go` exercises JSON-decoded disks
through both ingestion paths: two selected equal-capacity tmpfs mounts survive,
unselected /run is rejected, and root remains. Agent-side exclusions still
precede marker production. Legacy reports without the marker retain the
existing default policy; matched agent and server support is required.
This proves ingestion behaviour, not reporter installation or release delivery.

### TrueNAS RPC operation budgets and poll isolation

The configured client timeout (30 seconds by default) bounds each serialized
RPC operation, including its session-lock wait, any inline authentication,
method exchange or subscription, and the permitted read retry/backoff. They
share one budget; initial transport negotiation before a method is separately
bounded by the same timeout. A shorter caller deadline remains effective. A cancelled
waiter neither dispatches a request nor alters the current owner's socket or
transport status. RPC/stream readers receive the bounded context, and a
timed-out socket is discarded before a subsequent operation authenticates a
fresh session. Modern appliances never downgrade to REST on timeout, and an
action timeout after dispatch retains its unknown outcome without replay.
Keepalive still belongs to the session, not any completed operation's context.

Optional live-telemetry timeouts do not prevent otherwise usable inventory;
unobserved metrics remain unavailable. A required-method timeout returns a
truthful failure, preserves the prior successful observation/cached identity,
and lets the shared poll cycle continue to other connections. Socket deadline
errors are classified as timeout, not credential or generic connection failures.
The next normal poll can recover; completion-based failure backoff is unchanged.
This is an operation budget, not a new total-snapshot deadline or parallel poller.
The app-log idle window still yields a bounded tail; an earlier caller/operation
deadline is an error rather than a successful empty log response.

`TestJSONRPCConfiguredTimeout*`,
`TestJSONRPCWaitingDeadlineDoesNotInterruptSessionOwner`,
`TestJSONRPCPreCancelledCallDoesNotDispatch` and
`TestJSONRPCSnapshotContinuesAfterTelemetryTimeout` in
`internal/truenas/transport_test.go` cover silent handshake/authentication,
read/retry budgets, all three stream readers, queued cancellation, caller
deadlines, recovery and action no-replay.
`TestTrueNASPollerUnresponsiveRPCDoesNotFreezeOtherConnections` and
`TestClassifyTrueNASError` in `internal/monitoring/truenas_poller_test.go`
cover actual TLS/RPC clients through two successful/failed/recovered poll cycles,
cached identities, truthful health and unchanged backoff. These are controlled
runtime proofs, not native SCALE acceptance or a diagnosis/resolution of #2382.

### TrueNAS persistent-session liveness and successful poll cadence

Authenticated JSON-RPC WebSocket sessions send transport-only PING controls
every 25 seconds with a five-second write deadline. The sender belongs to the
session, not its opening request context, and disposal stops and joins it.
A failed control write closes the socket so existing transport handling owns
retry; actions retain their no-replay boundary. Existing serialized RPC and
stream readers consume pongs. Neither ping nor pong updates inventory freshness
or appliance health, and sending keepalives is not proactive pong-timeout detection.

Successful TrueNAS refreshes target start-to-start cadence, bounded below by
completion plus min(five seconds, configured interval), preventing back-to-back
load on slow appliances. Failed refreshes retain a full completion-based retry
interval. Manual connection probes use a separate client and do not alter
scheduling or poll evidence: success must not clear a runtime error or advance
last-attempt, last-success, observed collection or inventory readiness. Failure
remains in the probe response, not a fabricated runtime attempt. Observed
last-attempt and last-success timestamps remain runtime completion timestamps; resource
freshness thresholds must not be extended to hide genuinely stale devices.

`TestAuthenticatedRPCSurvivesIdleTransportTimeout` and
`TestRPCSessionKeepaliveConcurrentCallsAndShutdown` in
`internal/truenas/transport_test.go` verify an authenticated synthetic idle-timeout
server, opening-context cancellation, concurrent controls/RPCs and sender disposal.
`TestTrueNASSuccessfulPollCadenceIncludesBoundedIdleGap` in
`internal/monitoring/truenas_poller_test.go` verifies due boundaries, short/slow
cycles, completion timestamps and unchanged failure backoff. These are synthetic
runtime proofs, not native firmware timeout or reporter-resolution evidence.
`TestTrueNASSavedProbePreservesRuntimeEvidence` in `internal/api/contract_test.go`
checks the actual saved probe and list handlers against never-polled and
previously observed-but-failing runtime states. A synthetic successful login
with a required pool-method failure is not attribution of the native #2382
regression; the failing appliance/session method remains field evidence.

### TrueNAS app CPU JSON numbers preserve readings and sessions — issue #2400

`app.stats` CPU usage is a percentage-valued JSON number, not an integer
counter. Fractional values and idle `0.0` must decode without rejecting the
whole event; legacy integer values remain supported. Preserve the emitted
percentage without truncation or an additional ×100 conversion. Memory,
network and block-I/O integer fields retain their existing units and decoding.

Successful subscribe/read/unsubscribe must reuse the authenticated session
across GetApps and provider polls. A fresh event advances its collected-at
observation and the connection-scoped canonical CPU/memory/network readings
and History; rereading the same observation must not renew History. Malformed
CPU types and out-of-range JSON numbers still reject the complete event,
discard its unusable session and return no partial sample, without retrying a
non-transport decoding error. Existing permission/termination/deadline,
transport retry, cleanup, authentication, identity and tenant boundaries stay.
No session lifetime, keepalive, poll interval or freshness threshold changes.

`TestJSONRPCAppStatsNumbersReuseSession` and
`TestJSONRPCAppStatsRejectsMalformedCPUNumbers` in
`internal/truenas/transport_test.go` exercise actual GetApps subscriptions,
reused login, numeric compatibility and malformed-event disposal/recovery.
`TestTrueNASFractionalAppStatsPollAndHistory` in
`internal/monitoring/monitor_polling_test.go` connects TLS/RPC, thirteen-app
inventory, repeated actual provider polls, canonical resources and all four
CPU/memory/network History series with stable connection/tenant identity. Its
first poll also crosses the real persisted/chart History read path.
These short local polls are source regressions, not native SCALE/Traefik
sixty-second session acceptance, installed restart or containing publication.

### TrueNAS empty app inventories and session-disposal evidence

An empty or null `app.query`/legacy `/app` result remains a successful empty
app inventory. It must not request `app.stats`: there is no workload to enrich,
and a stopped Apps service must not consume the operation budget or dispose of
an otherwise usable monitoring session just to enrich no rows. Nonempty app
inventories retain best-effort stats enrichment. This removes a needless
subscription path reported in #2396; it does not establish the exact
`notify_unsubscribed` envelope or resolve rejection with nonempty inventories.
Unrecognised stream notifications remain bounded by the existing operation
timeout, not classified as a known subscription's rejection without evidence.

The one permitted stream transport retry uses the same disposal rules as the
initial exchange. Malformed events, empty subscription IDs and failed
unsubscribe cleanup discard the second session and report disconnected status;
normal app-log idle completion returns the collected tail and discards its
timed-out socket rather than surfacing an internal consumed-session sentinel.
No extra retry, action replay or REST downgrade is introduced.

Each authenticated socket's first local disposal records an INFO event with
`component=truenas_rpc`, `action=close_session`, a fixed `reason`, authenticated
`session_age`, and optional request-method/error-category classifications. Causes
distinguish client closure, stream completion/error, transport error and failed
keepalive writes. Error categories distinguish timeout, cancellation, method,
transport and protocol errors. No error text, params, subscription ID, endpoint,
username or credential is logged. Keepalive failure may precede a subsequent
poll's disposal, so that first cause must be preserved and logged only once.
These are local close-path observations, not appliance/proxy diagnoses or
evidence that #2400's minute-by-minute reauthentication has been resolved. There
is still no fixed session-age or per-poll closure policy.

Verification in `internal/truenas/transport_test.go`:
`TestJSONRPCEmptyAppsDoNotSubscribeToStats`,
`TestLegacyRESTEmptyAppsDoNotNegotiateStats`, the snapshot inventory contract,
`TestJSONRPCStreamRetryDiscardsUnusableSecondSession`,
`TestJSONRPCStreamRetryReturnsCompletedLogTail`, `TestRPCSessionCloseDetails`,
`TestRPCSessionCloseLogIsBoundedAndSecretFree` and
`TestRPCSessionCloseLogKeepsFirstKeepaliveCause`. These controlled runtime tests
preserve inventory, retry limits, fresh-session recovery and log confidentiality;
native SCALE behaviour, published delivery and field acceptance remain separate.


### TrueNAS reporting summaries preserve raw History — issue #2346

Every reporting time-range query requests `aggregate: true`, the TrueNAS
default, through the shared `reportingRangeQuery`: JSON-RPC host/disk-temperature
History and short temperature reads, plus legacy REST host History/live telemetry.
SCALE 25.04.2.6 rejects `aggregate: false` because its response schema requires
the min/mean/max fields omitted by that option. These summaries accompany the
raw `data` series; History still parses only raw samples, with the same
start/end window, timestamps, units and default duration. There is no new
retry, error suppression, transport selection or graph-isolation policy.

`TestReportingHistorySupportsSCALE2504Aggregations` models the supplied
ReportingGetDataResult/EINVAL failure and preserves 3,600 raw samples through
system and disk History. `TestReportingAggregationsPreserveRPCFailures` keeps
validation, access and missing-method errors visible. The REST default-summary
regression and existing CORE graph-isolation/live-memory tests retain legacy
telemetry and canonical CPU/memory History projections. Synthetic transports
are not native appliance or containing-release acceptance.

**TrueNAS legacy-REST memory telemetry — issue #2077 (20 September 2026)**

A connection proven to run a recognized legacy CORE/FreeNAS release has no
`reporting.realtime` JSON-RPC subscription, so live system telemetry and
host-chart history are reconstructed from the REST reporting API
(`reporting.get_data`, served as a POST endpoint keyed by the middleware
schema names `graphs` and `reporting_query`). The reporting `memory` and `arcsize` graphs supply the
available and ARC readings that the JSON-RPC realtime subscription otherwise
provides. When no available-memory reading can be established, the provider
must not derive usage from an absent available value: the system memory metric is
omitted and agent memory is projected as usage-unavailable with its known total,
so missing data never reports a CORE appliance as 100% used. This is a behavioral
correctness repair with no public API or schema delta.
`TestRESTSystemTelemetryReadsReportingMemory`,
`TestRESTSystemMetricHistoryUsesReporting` and
`TestTrueNASMemoryWithoutAvailableIsNotReportedAsUsed` in
`internal/truenas/client_test.go` cover the REST reporting fallback, the REST
history query and the unavailable-usage guard. These are synthetic transport
and projection proofs, not native CORE appliance acceptance or reporter
confirmation.



### CORE REST reporting envelope — issues #2964 and #2077

Legacy REST `POST /reporting/get_data` sends exactly `graphs` and
`reporting_query`. WebSocket's positional second argument is not a REST key
named `query`: CORE 13.0-U6.8 rejects that unknown attribute before collecting
any graph. Both short live reads and full History use this same envelope and
unchanged aggregate/start/end range. The graph catalogue, per-device selection,
optional-graph isolation, transport gate, TLS, authentication and zero/unknown
semantics are unchanged; no guessed-key or transport retry is added.

The request-validating CORE and REST aggregation fixtures bind the corrected
schema name. `TestCORE13RESTReportingQueryResourceJourney` in
`internal/api/resourceapi/truenas_core_reporting_test.go` exercises a pinned-TLS
CORE bridge, real client/provider, resource-list HTTP response and native History
through complete, rejected, partial, observed-zero and recovered readings. It
rejects the former key with the reported HTTP 400 before the normal path, retains
inventory/capacity on telemetry failure and does not invent sibling metrics.
These are synthetic protocol/consumption controls, not appliance acceptance,
reporter confirmation, a data-pool capacity repair or release availability.

### TrueNAS partial samples and nonblocking telemetry diagnostics — issue #2077

Live system identity and both telemetry parsers carry explicit per-metric
availability. A reporting timestamp or realtime interval is not evidence that
CPU, memory or any sibling I/O series was measured. Each canonical metric is
omitted unless its own reading exists; observed zeros remain valid, including
zero free memory. Capacity-only snapshots keep hardware capacity and mark
usage unavailable. Older static snapshots retain their compatibility projection;
no live client uses that fallback. Null/malformed single-series objects never
substitute a timestamp or another arbitrary numeric field for the missing value.
Supported generic `value`, `y` and `temperature` row aliases remain supported.

Optional telemetry failure must not discard inventory or degrade a successful
inventory poll. Existing connection diagnostics expose optional
`observed.telemetry` with six availability booleans (`cpu`, `memory`, `netIn`,
`netOut`, `diskRead`, `diskWrite`), a fixed `errorCategory` and, for HTTP failures,
`httpStatus`. No raw response body, endpoint, key, hostname or arbitrary provider
error text is copied into this projection. Error categories record the observed
collection result, not its cause. A later refresh replaces availability and
clears obsolete failures; snapshot and connection-summary copies cannot mutate
shared state. This is an additive diagnostic field on the existing read surface,
not a separate resource, route, telemetry store or alert policy.

`TestRESTReportingObservationPresence`,
`TestRESTSnapshotRetainsSanitizedTelemetryFailure`, `TestRealtimeObservationPresence`
and `TestSystemTelemetryFailureCategoriesAreBounded` in
`internal/truenas/client_test.go` pin repeated full snapshots, canonical rows,
native History/zero semantics and bounded failure/recovery controls.
`TestTrueNASPartialReportingPipeline` in
`internal/monitoring/truenas_poller_test.go` exercises the HTTP client, poller,
registry, memory metadata, shared metrics writer and in-memory/persisted chart
readbacks across partial, missing and zero-valued cycles. These synthetic proofs
are not the reporter's native response schema, installed CORE acceptance or a
claim that #2077's missing graph journey is resolved.


**Legacy reporting failure isolation and memory components — issue #2077**

Live telemetry and history retain the legacy REST graph/query contract and the
single successful-batch fast path. A batch rejected with HTTP 400, 422 or 500
falls back to one request per selected graph. The native catalogue extension
below bounds the expanded selection and its split loop.
A failed optional graph must not discard successful CPU, memory or ARC graphs.
Authentication, rate-limit, missing-endpoint, service-unavailable, decoding,
transport and cancellation errors are not graph fallback triggers; these also
stop an in-progress split. If every graph fails, the original batch error is
returned. Missing memory readings remain unavailable, never fabricated usage.
FreeBSD `active` pages are not a total-used-memory alias. Without an explicit
used series, history derives usage from the known capacity and reported free
memory, subtracting separately reported ARC as the live projection does.

The legacy REST graph request also matches the request shape the native TrueNAS
13 GUI itself sends (#2077): a graph that is not scoped to a device omits the
`identifier` key instead of sending an explicit null. The reporter's CORE
13.0-U6.1 WebSocket capture shows `reporting.get_data` graphs as
`{"name":"cpu"}` (also memory, arcsize, load, swap, cputemp), with an
`identifier` present only for parameterized graphs such as
`{"name":"arcresult","identifier":"demand_data"}`. Sending `identifier: null`
is not the native contract and is a plausible whole-batch rejection that would
discard otherwise usable CPU and memory telemetry.

`TestRESTReportingGraphFailurePreservesSnapshotTelemetry`,
`TestRESTReportingGraphFailureBoundaries`,
`TestRESTReportingPartialAndMalformedResponses`,
`TestReportingActiveMemoryIsNotTotalUsed` and
`TestRESTReportingRequestMatchesNativeGraphShape` in
`internal/truenas/client_test.go` exercise request-validating synthetic REST
responses, repeated full snapshots, CPU/free/ARC projections, matching history,
partial data, bounded errors, cancellation and the native identifier-less graph
request shape. They do not establish the native CORE response schema, row
timestamps/units, appliance acceptance or the reporter's exact failure. Native
response evidence remains required before claiming that this repair resolves the
reported telemetry journey.

### CORE native reporting rows and device graphs — issue #2077

`internal/truenas/core_reporting.go` owns the native reporting envelope alongside
`client.go`, not a second transport or metrics store. The reporter's five incoming
CORE 13.0-U6.1 results now establish value-only rows with one element per legend,
external `start`/`end`/`step`, FreeBSD memory-class labels, disk-octet labels,
interface RX/TX/overlap and per-core `cputemp` readings. They do not establish that
Pulse's REST bridge, installed browser or complete appliance journey works.

Value-only rows derive time from `start + row-index * step`, with positive step,
valid ordered bounds, exact legend width and overflow-safe end checks. Explicit
row timestamps and supported object aliases retain compatibility. A metric is
never a timestamp, a null is never zero and a window mean is never a replacement
for a missing current row. History retains measured buckets; native current
readings accept only the last two steps before the requested end, preserving
sample time rather than stamping an old value as newly measured. This allowance
never exceeds the live query window, even with a coarse returned step. The five-state
CORE CPU vector is normalized by its sum (RRD state rates need not sum to 100).
All-zero CPU or complete memory-class vectors are empty buckets, not 100% usage;
explicit zero usage, all-idle CPU, zero-free RAM with used pages and zero I/O
remain measured zeros. Free memory comes from `memory-free_value`, not active
pages or the sum of memory classes. ARC subtraction uses the matching free-RAM
bucket when free RAM is present; independently reported ARC remains available
without claiming measured memory usage. Pre-cancelled collection stops before
reading the catalogue or posting a reporting query. Reporting I/O values remain rates in the existing bytes/s contract;
step supplies time, not a second rate division; `overlap` is not extra traffic.

Legacy REST reads `/reporting/graphs` once per live/History collection and sends
its disk/interface identifiers, omitting identifiers on global CPU, memory,
ARC and CPU-temperature graphs. Only missing/method-unsupported catalogue
endpoints retain the older unscoped compatibility request. All other catalogue
failures remain failures; no authentication, TLS, rate-limit, transport, decode
or cancellation retry is added. Selection is deduplicated and capped at 256
entries, refusing oversize rather than publishing a truncated total. Successful
reporting remains one batch; graph-only 400/422/500 failures may split once per
selected graph. Responses are bound to the selection: an omitted device stays
absent. Device rates sum once per identifier only at simultaneous observed
buckets of every selected member, never by carrying a stale member forward or
assuming a missing member was idle. Other graphs remain independently usable.

Native CPU temperature maps into the existing canonical host temperature and
`temperature` History key, using the same package-versus-core outlier safeguard
as current readings. Disk-temperature History uses the same envelope decoder.
The canonical writer also records each positive finite TrueNAS host temperature
in local and persistent `temperature` History, under the existing source/dedup
gates. A full local CPU window must not make the existing Thermals panel depend
on native fallback or lose its observed temperature. Missing temperature is not
written as zero, and no additional sensor writes are added to other sources.
No frontend route, resource identity, alert suppression policy, credential scope
or recognised-legacy transport boundary changes.

Verification: `internal/truenas/core_reporting_test.go` and its anonymised
`testdata/core13_reporting.json` cover five source-bound native excerpts,
row/timing/alias/zero/null/empty/finite controls, device totals, stale buckets,
ARC alignment, current and canonical History projections, request validation,
catalogue limits and failure boundaries. Unique overflow is rejected rather than
truncated, while duplicate identifiers consume only one selection entry. The
excerpt windows are deliberately
shortened controls, not complete native windows. Extended
`TestTrueNASPartialReportingPipeline` in
`internal/monitoring/truenas_poller_test.go` crosses pinned-certificate HTTP,
poller, canonical registry, writer, in-memory/persistent chart readbacks and
native fallback, including temperature and the sufficiently covered local
chart fast path. Existing CORE isolation, recognised
transport/auth/TLS and modern SCALE aggregation tests remain required. Source
proofs do not substitute for containing-release qualification or installed CORE
live/collapsed/expanded/History acceptance.



**Correlated VM memory fallback — issue #1962 (7 September 2026)**

The next Proxmox guest poll indexes live agent memory embedded in VM read views,
not only standalone host views. Keys retain the instance/node/VMID scope. This
restores the existing host-agent fallback when correlation removes the standalone
host row; it does not change preferred QEMU memory sources or platform metric
merge priority. Agent-source freshness and unavailable/invalid memory fail closed.
`TestCorrelatedGuestMemoryNextPoll` exercises real manual registry merging,
next-poll resolution, VM construction, card read-state projection, diagnostic
snapshots and actual memory-threshold evaluation, plus stale/offline/unavailable
samples and other-instance lookup. It is synthetic runtime proof, not a live
FreeBSD installation, automatic identity-match or browser receipt.

### Incident-history reconciliation boundary

Active-alert timeline reconciliation uses the shared error-bearing incident
query. If canonical history is unavailable, reconciliation stops for that alert
and records the read failure. It must not treat a failed read as proof that a
firing event is absent. Historical source messages own alert condition wording.
Projection does not manufacture comparison direction from numeric placeholders.


Container filesystem observations are native, resource-scoped reads. The shared
`pkg/agents/filesystem` contract keeps measurement time, mountpoint, source,
filesystem type and optional usage together. Unavailable reads have an error
and no usage. Capacity/free/available counters describe the filesystem visible
there, not a container quota, image layer or host-wide capacity diagnosis.
Finite inode inventory is optional independently of byte capacity.

The Docker collector requires a local Unix runtime endpoint, an exact full
container ID, a matching process cgroup and a stable inspected PID/start time.
Linux reads through the held process-root descriptor using confined openat2
resolution and fstatfs, without executing container programs. A remote daemon,
unattested or inaccessible namespace, unsupported kernel or changed process
produces unavailable evidence. This path does not grant ptrace, root, helper
or command privileges. Ordinary root-owned containers may therefore remain
unavailable to an unprivileged collector even when its Docker socket is usable.

One native probe per container may remain in flight, with a bounded wait and
an observer-wide limit. Timeout does not start replacement syscalls or reuse
late measurements. Report ingestion and snapshot conversion clone all nested
usage fields. A later failed or absent observation replaces prior capacity.
Proof surfaces are the filesystemprobe native/timeout tests, shared report
tests, container collection boundary tests, ingestion regression and the
opt-in owned tmpfs collector fixture. Installed-agent and model qualification
are tracked separately in PATROL_ASSISTANT_CUSTOMER_JOURNEY.md.

The same observer reads Proxmox LXC guests for the host agent and its typed
helper (#2511). Its socket-free producer must not label the first 128 guests as
a complete inventory without inspecting later configs. A 129th running guest or
an unknown guest after the limit leaves collection unavailable/degraded through
the existing bounded fallback; a stopped later guest does not invalidate 128
established guests. This preserves the consumer's distinction between complete
inventory replacement and explicit partial omissions. The producer boundary is
covered by `TestSocketFreeProxmoxLXCDiscoveryCannotTruncateCompleteInventory`;
partial-wire, omitted-reading and complete-empty cache tests remain authoritative.
Wide cgroup trees do not by themselves disqualify a verified init: the producer
inspects `ns/init.scope` before unrelated workload siblings using at most 64
cgroup process reads and 4,096 candidate PIDs. Exhausted, unreadable, cancelled
or identity-invalid searches remain unknown/degraded; no omitted guest becomes
healthy empty telemetry. The wide-tree helper collection and search-boundary
controls in `internal/hostagent/privilege_helper_client_test.go` preserve measured
capacity and exact identity without IPC commands. They are not native recovery.
An `lxc` request names the decimal VMID; the pinned process
must sit in that guest's exact `lxc/<vmid>` cgroup and be its init (`NSpid` is
exactly `<pid> 1`, so the PID 1 of a namespace nested inside the guest, such
as Docker in LXC, cannot stand in for it), checked before and after the read.
With `MountsOnly`, a configured mountpoint sharing its parent's device reports
`ErrNotMounted` instead of the parent's counters. Docker and Podman identity
rules are unchanged. Confined resolution prefers openat2 with `RESOLVE_IN_ROOT`;
when the kernel or a systemd seccomp filter answers ENOSYS (the typed helper's
`RestrictSUIDSGID=true` blocks openat2 because its flags cannot be inspected),
the clean path is walked component by component with `O_NOFOLLOW` and any
symbolic link is refused, so neither path resolves outside the guest root.
Docker and Podman reads take the same fallback, so a collector under such a
filter now gets confined counters instead of unavailable evidence.
`TestComponentWalkRefusesSymlinksAndStaysBeneathRoot`,
`TestMountsOnlyDetectsUnmountedPathsWithinRoot`,
`TestNonInitProcessCannotSupplyProxmoxGuestCapacity` and
`TestProxmoxLXCIdentityIsExact` pin these rules; the Linux tests also passed
as root on a disposable PVE 9.2.18 node, where the helper-sandboxed collector
returned readings matching `pct df` through the component walk.

Docker mount collection preserves both native `Mounts` records and entries
reported only in `HostConfig.Tmpfs`. Existing reported destinations remain
authoritative. Additional tmpfs destinations are ordered deterministically,
retain their options and read/write setting, and use the existing mount report
shape. Configured tmpfs size is configuration, not measured used/free space.
`TestCollectContainerPreservesTmpfsMounts` reproduces a live tmpfs-only inspect
shape and covers mixed mounts, read-only options, overlap and absent host config.

Docker collection records read and write counter presence independently,
including explicit zero, in optional report fields. Older reports without those
fields establish only positive counters. Container reports propagate this
presence to the shared rate tracker. An omitted block-I/O payload and the first counter sample
produce no rate history. Unchanged observed counters produce measured zero,
including after an omitted report. Container writable/root layer sizes never
produce capacity usage history. The ingestion regression is
`internal/monitoring/docker_metric_presence_test.go`. This changes measurement
projection only and grants no agent lifecycle authority.
The shared resource-to-browser conversion preserves each optional I/O rate
independently. Missing directions are omitted from JSON, while measured zero
remains numeric zero. No aggregate presence flag may fabricate its sibling
direction. `TestResourceDiskIOWirePreservesAbsentDirection` pins that wire path.

### PBS datastore alert evaluation belongs to the live poll

After publishing freshly polled PBS datastore storage rows, the poller invokes
the existing storage alert evaluator with capacity-trend evidence. It must not
depend on mock ticks or unified metric evaluation: that path does not evaluate
PBS storage thresholds. Existing storage policy, aliases, suppression and
connectivity semantics remain alerts-owned; this wiring adds no new policy.

Absent or partial capacity counters must not resolve an existing usage
incident. Explicit empty-store counters (positive total, zero used, free equal
to total) may recover it. Repeated polls and subsequent unified alert sync must
preserve the usage incident identity, including recurrence with alternate PBS
counter names. Independent backup-posture incidents remain separate.

Verification: `TestPBSPolledCapacityRequiresObservedRecovery` in
`internal/monitoring/monitor_pbs_coverage_test.go` crosses synthetic HTTP
decoding, live polling and unified sync. It checks an 80% threshold at 85%,
missing counter retention, confirmed-empty recovery and 86% recurrence with an
explicit one-point minimum delta. This is local capacity-path evidence, not
installed recipient receipt, restart qualification or exhaustive connectivity
validation.

### Host-local addresses are not PVE identity

Automatic host/PVE network matching excludes non-global-unicast addresses,
lo/docker* interfaces and Docker-generated `br-` plus 12-hex-character bridge
names from both reported-host and provider-node address inventories. A Docker
bridge address seen on only one PVE node can still exist on an unrelated NAS;
provider-only owner counts cannot make it machine identity. Management bridges
such as vmbr0 and br-mgmt, unnamed legacy interfaces, private IPv4 and IPv6/ULA
remain eligible. Explicit unicast report-IP hints remain eligible independently
of inferred interface evidence. No blanket private-subnet exclusion is
permitted.

Verification in `internal/monitoring/monitor_host_agents_test.go`:
`TestFindLinkedProxmoxEntityWithHints_RejectsHostLocalNetworkIdentity` covers
seven previously false associations; `TestAgentLinkNetworkIdentityFiltering`
pins both-side exclusions and positive management controls;
`TestApplyHostReportDoesNotLinkUnrelatedDockerBridge` requires repeated
ingestion to leave both link directions absent. Existing endpoint and
ambiguous-name tests remain required. These checks prevent a reproduced
synthetic misassociation, not all private-address collisions or unknown
custom bridge names, and do not prove the cause of #1930.

### Host/node associations retain durable operator intent

Monitoring owns the provenance used to distinguish an operator selection from
an automatic association. The host continuity journal records `manual`,
`unlinked` or `automatic`; an absent value is legacy/unknown, not evidence that
the link was automatic. This internal provenance does not add a public API or
frontend field.

The existing link/unlink APIs hold the host lifecycle write lock and persist
the complete affected intent transaction under the state lock before publishing
either direction of the association. Missing storage or a failed journal write
must return an error without changing visible links or the journal's in-memory
entries. Replacing a node's owner persists the displaced owner's explicit unlink
too, including an owner which has not yet reconnected after restart. Link writes
preserve identity, report watermarks, credential and removal evidence.

A manual selection remains pinned to the selected provider ID through unmatched
reports, provider refresh and restart before another report. A replacement
provider ID must not inherit that selection by name: intent remains dormant
until the original ID returns or the operator explicitly selects another ID.
The persisted reservation prevents another host's automatic match taking that
node before its owner reconnects. Explicit unlink survives restart and suppresses
automatic node reassociation even when the next report supplies matching evidence.

Automatic name matching uses the provider's node name, never the display name
merged from an already-linked agent. Known automatic associations may be
re-evaluated and cleared; provenance-bearing host updates remove obsolete
reverse links without clearing another agent's link. Unmarked legacy forward
links are retained during report ingestion, and unmarked one-way reverse links
must survive host updates because SMART fallback also consumes them, except
that a link the operator split in the resource store (unlink or report-merge
in the resources API) is cleared on both sides whatever its provenance. Legacy
provider reconciliation otherwise keeps its existing behaviour. This deliberately
does not claim to repair every persisted v6.4.1 association or resolve #1930.

Verification:
- `internal/monitoring/host_manual_link_regression_test.go` covers immediate
  restart, explicit unlink with positive matching evidence, provider-ID
  replacement/return, failed writes, automatic versus unknown provenance,
  dormant reservations and concurrent reports/provider refresh.
- `internal/config/host_continuity_test.go` pins journal preservation and
  replacement of a dormant owner.
- `internal/models/state_additional_test.go` pins transaction rollback,
  obsolete reverse-link cleanup and retention of unknown legacy reverse links.
- `TestMaybePollPhysicalDisksAsync_AgentFallbackWhenDiskQueryFails` in
  `internal/monitoring/monitor_pve_disk_fallback_test.go` remains required:
  association cleanup must not remove the one-way link used by SMART fallback.

### One primary CPU temperature rule for node and agent history

`models.PrimaryCPUTemperatureCelsius` (`internal/models/temperature_primary.go`)
is the only rule that names a machine's single CPU temperature: the hottest
usable reading among the package (Intel Package id, AMD Tctl/Tdie), the
producer's CPU maximum and every core. `models.HostSensorPrimaryCPUTemperatureCelsius`
applies it to an agent's flat sensor map (the hottest `cpu*` key). Node history
(`monitor_pve_storage.go`), agent history (`monitor_agents.go`), the mock seed
and live ticks (`mock_metrics_history.go`) and the CPU low/record tracking
(`monitor_polling_node_helpers.go`, `mergeTemperatureData` in
`host_agent_temps.go`) all call it. The package-first agent copy and the
CPUMax-first node copy are gone: on an agent-fed node `CPUMax` is built from
cores alone and can sit below the package, and elsewhere a core can run above
the package, so the two copies split an agent-linked node's drawer headline
from the line beneath it. For the keys an agent reports (`cpu_package`,
`cpu_core_N`) the map form equals the rule applied to the node temperature
`convertHostSensorsToTemperature` builds from the same map. A raised record now
also carries its `MaxRecorded` time. TrueNAS keeps its own package-versus-core
outlier safeguard (`maxTrueNASSystemTemperature`, #2122) because that quirk is
specific to its producer.

Mock mode follows the same shape. A node and its linked agent carry one CPU
reading, sampled from the agent's series (`mock.CPUTemperatureSeries`), which
is also the series that seeds both histories, under the keys a real agent
reports; the package carries the sample and cores sit a stable amount below,
so the rule returns the sample exactly. Agent hosts no longer carry a second
package reading under a `cpu.package` key. Proof: `monitor_host_agents_test.go`
(`TestApplyHostReportChartsThePrimaryCPUTemperatureEverySurfaceShows`),
`monitor_additional_test.go`, `host_agent_temps_test.go`,
`mock_metrics_history_test.go`
(`TestSeedMockMetricsHistory_LinkedNodeTemperatureContinuesAgentSeries`) and
`internal/mock/generator_test.go`
(`TestFixtureNodeAndLinkedAgentShareOnePrimaryCPUTemperature`); each fails
against the previous rules.

### Physical-disk observation cadence

Physical disk inventory has an independent collector schedule. The PVE poller
carries its default five-minute or configured interval with each disk record,
while keeping the last successful observation timestamp on retained records.
Quick temperature refreshes and typed read-state conversions must preserve that
cadence rather than falling back to the general node polling interval. It feeds
the canonical source-freshness contract, not a model-generated diagnosis.
Proof: the async fallback poll in `monitor_pve_disk_fallback_test.go`, the canonical
roundtrip in `issue1595_collection_trust_test.go`, and registry cadence/recovery
coverage. Live `/api/resources` and final Assistant browser proof are also required.

PBS node-status collection must reject HTTP-success responses whose `data`
is omitted or null (including a null response envelope). Absent status is
unavailable telemetry, not measured zero usage: the poller retains independently
established connectivity, marks node metrics unavailable, and must not resolve
an active metric incident from that response. Populated objects must include
non-null CPU, memory used and memory total measurements, with non-negative
usage and a positive memory denominator. Missing fields must not become zero
usage. Since the poller has a single node-metrics availability flag, incomplete
CPU or memory makes the node metrics unavailable together; unrelated status
fields are not required. Explicit zero CPU and memory usage remain valid.

The upstream PBS API schema at
https://pbs.proxmox.com/docs/api-viewer/apidoc.js (read 2026-09-05) declares CPU
and memory used/total as numeric measurements. This validation is limited to
measurements consumed by node alerting, not exhaustive schema enforcement.

Verification: `TestClient_GetNodeStatus_MissingData` in
`pkg/pbs/client_http_test.go` covers the absent envelopes.
`TestPBSPartialMetricsWebhookLifecycle` in
`internal/monitoring/monitor_pbs_webhook_test.go` exercises repeated null status
and missing memory-used measurements through the real poller, alert manager,
notification queue and local webhook. `TestClient_GetNodeStatus_MetricCompleteness`
pins omitted/null fields, invalid denominators, negative usage, genuine zero,
and tolerance for unrelated extra fields.
It requires online-but-unavailable projection, unchanged incident identity,
no false recovery history or delivery, then one identity-preserving recovery
after valid low-memory samples. These are local synthetic checks, not
installed-artifact or off-host destination qualification.

Direct PBS backup polling classifies typed API and authentication failures by
the response status exposed by `pbs.HTTPStatus`, including wrapped errors.
A 5xx gateway or server response remains transient even when its body quotes
an upstream “API error 403” or “API error 404”; that text must not erase the
last known backup inventory. Genuine 4xx responses remain terminal under the
existing cache policy. The legacy untyped-error fallback is unchanged.
Retaining cached inventory does not establish a successful poll or fresh
backup evidence.

Verification: `TestPollPBSBackups_PreservesCacheOnTransientDatastoreError` and
`TestPollPBSBackups_DropsStaleCacheOnTerminalDatastoreError` in
`internal/monitoring/monitor_backups_readstate_test.go` exercise actual HTTP
fixtures for 500, 502 quoting 403, 503 quoting 404, and genuine 401/403/404.
These tests prove cache retention/removal, not installed PBS wake, service
restart, or notification receipt.

Docker alert lifecycle events pass through the shared resource history identity
writer. A full Docker source reference must reach the same canonical container
history as inventory changes, including recovery after inventory removal and
restart. Existing alert lifecycle event IDs remain unchanged so replay cannot
duplicate retained events. Same-name containers and abbreviated IDs must not
join another container's history. The real alert-manager callback path is
covered by `TestDockerAlertTimelineUsesCanonicalHistoryIdentity` in
`internal/monitoring/monitor_alert_handling_test.go`.

Proxmox node, guest and storage alert lifecycle events carry source-native IDs
and pass through the same writer, which resolves them to the canonical
resource. Resource facets, resource intelligence and Assistant resource context
therefore include their alert history, including events emitted after the
resource left inventory. Event IDs still hash the alert's source reference, so
replay stays idempotent. The real alert-manager callback path is covered by
`TestProxmoxAlertTimelineUsesCanonicalHistoryIdentity` in
`internal/monitoring/monitor_alert_handling_test.go`.

Docker host and Swarm service alert lifecycle events, and sub-resource alert
events (ZFS pools and devices, host filesystems, disks, RAID arrays, sensors
and the Unraid array), pass through the same writer, which binds them to the
Docker host, Swarm service, storage or host named in the unified-resources
history identity clause. Container names and shortened container IDs still
bind nothing, and neither does the marked name a container or Swarm service
reported without an ID alerts under, so its events join neither its host's
history nor that of a service whose ID it is named like. A producer
that changes one of these reference shapes must change
`dockerHostHistoryReference` or `historySubResourceOwner` with it.
`TestOwnerAlertTimelinesUseCanonicalHistoryIdentity` in
`internal/monitoring/monitor_alert_handling_test.go` pins the Docker host,
Swarm service, ID-less container and service, ZFS pool and device, and host
filesystem shapes against the real alert manager.

PVE disk health and wearout lifecycle events pass through the same writer but
never bind their path reference: each row goes to the physical disk its
recorded serial or WWN names, or stays under the reference, under the
unified-resources history identity clause. When a reboot reorders devices
before the registry catches up, a row that records a serial or WWN goes to the
disk the stale registry places at the path only if that disk reports the same
identity (the unified-resources clause lists where inventory merges or keys
disks differently); a row without either names only an identity-less disk at
its path.
`TestProxmoxDiskAlertTimelineFollowsRecordedHardware` in
`internal/monitoring/monitor_alert_handling_test.go` pins that path, and the
reference's incidents for the Alerts history Resource action, against the real
alert manager.

TrueNAS native alert projection preserves the trimmed, uppercase provider level in ResourceIncident.NativeSeverity. INFO and NOTICE retain the same canonical monitor risk; consumers must not lose their distinct actionability when projecting provider evidence. Native CRITICAL, ALERT, and EMERGENCY all project to canonical critical severity; EMERGENCY must not be discarded as unknown or make a still-active condition appear recovered. WARNING remains warning, and INFO and NOTICE remain informational at this projection boundary.

Verification: `TestIncidentProjectionPreservesNativeSeverity` in `internal/truenas/provider_pool_health_contract_test.go` covers all seven native levels and case/whitespace normalization. `TestTrueNASNativeSeverityDispatch` in `internal/alerts/truenas_native_dispatch_test.go` verifies downstream INFO suppression, NOTICE preservation, notification severity, duplicate-poll retention, and confirmed recovery callback identity. The TrueNAS lifecycle tests in `internal/alerts/unified_incidents_test.go` require repeated EMERGENCY evidence to interrupt recovery confirmation. These are fixture-based projection and manager checks, not appliance ingestion or external notification-provider receipt proof.

Unraid host ingestion and canonical read-state reconstruction preserve the optional array disk count, distinguishing explicit zero from unknown. Storage assessment suppresses only the no-parity warning for an explicit zero-disk array; unknown counts retain the prior warning and disabled, invalid, or missing member evidence remains effective.

Direct PBS backup polling correlates manifestless snapshots with current
writer tasks before publishing guest backup-running state. The client queries
running `backup` and `syncjob` task families separately with bounded pagination;
an HTTP, permission, decode, or page-cap failure leaves task visibility unknown
and preserves the conservative running interpretation. A complete empty result
marks the incomplete artifact observed but inactive. Backup workers match their
datastore and `vm|ct/id` subject when that shape is available; a running sync
job remains an instance-wide writer because it may touch multiple groups.

Own polling, typed collection, runtime state assembly, and canonical monitoring
truth for live infrastructure data.
Monitoring supplies the live unified-resource snapshot used by the alerts-owned
versioned identity migration. The migration must be planned without mutating
the active configuration, persist successfully before the alert manager adopts
it, leave ambiguous identities untouched, and reject schema versions newer than
the running binary. Monitoring owns this orchestration only; alert identity,
override semantics, and the schema version remain alerts authority.
Agent Doctor interprets Proxmox capability profiles according to the monitored
product. Only PVE host profiles require a link to a PVE node; an explicit PBS
profile, or an auto/missing-type host that matches a configured PBS instance by
normalized hostname or reported address, must not receive the PVE-only
`proxmox_profile_unlinked` warning.
Agent Doctor also projects privilege and command authority without conflating
them. The process user, local `monitoring-only`/`command-capable`/`legacy`
ceiling, and server-derived credential execution scope are distinct evidence.
A monitoring-only runtime with `agent:exec` is an over-scope warning; a
command-capable runtime whose monitoring credential cannot re-enable commands
is a scope-mismatch warning; an actively command-enabled runtime without
`agent:exec` remains critical. These facts do not change collection truth or
make a least-privilege service unhealthy by themselves.
For a credential explicitly marked `monitoring-collector`, Doctor also compares
the complete scope set against the canonical role allowlist. Any wildcard,
operator, action, management, execution, or unknown scope is surfaced as the
critical `agent_credential_scope_excess` reason with only the unexpected scope
names as evidence; a current heartbeat never hides that server-side authority
violation.
Action-runner posture is a server-side join, not collector-reported monitoring
truth. The collector may report only that its typed local helper is configured;
Agent Doctor separately overlays the current tenant's host-bound runner-token
inventory and admitted command-session inventory. A runner is connected only
when the canonical agent ID matches an admitted `action-runner` session with
the closed `typed_actions.v1` capability. Credential presence, expiry,
connection, version, and protocol facts remain distinct, and none may be
inferred from collector health or helper configuration.
Configured helper posture and current SMART/Proxmox/container-inventory helper
operation health are also distinct. Monitoring preserves the collector-authored
`typed-privilege-helper` module status and derives the stable warning
`agent_privilege_helper_degraded` when it is not running. The reason states
that affected privileged telemetry was omitted without local privilege
widening, while its error evidence passes through the existing bounded
diagnostic redaction. A later success clears only the matching operation on the
collector; monitoring must not infer recovery from an unrelated healthy helper
operation. A container-inventory helper failure stops that summary report and
remains visible through the same module until a complete helper inventory (or
an applicable collector-owned rootless runtime) succeeds; monitoring must not
infer recovery from the Docker module merely remaining connected. A Docker-only
collector's explicit incomplete status report updates liveness and helper
health while retaining its last complete inventory, including across the
degraded interval; only a complete report may replace that inventory.
When the safe collector uses direct rootless Docker or Podman monitoring, every
collection revalidates the exact socket path, ownership, mode, and fresh daemon
rootless attestation immediately after `Info` and before any other daemon API.
A boundary change or permission revocation is runtime unavailability: no
post-`Info` inventory call is allowed, reconnect must repeat full admission,
and repeated failure may recover only through a complete typed-helper summary.
The safe collector cannot execute report-response update commands or autonomous
cleanup/update work in either direct or helper monitoring mode.
The collector emits no Proxmox degradation for an absent inventory on an
ordinary Linux host unless Proxmox mode or local `pct` discovery establishes
that the operation applies.
If any running-container filesystem query fails, the helper returns a stable
typed operation failure and the collector omits that Proxmox snapshot; partial
inventory must not be reported as a healthy collection.
Monitor construction also applies the persisted alert schedule's normalized
initial-delivery target to the tenant notification manager. This is runtime
wiring only: monitoring does not choose destinations or own notification
policy, and live API saves must apply the same setting without requiring a
monitor restart.
Monitor construction also applies the persisted grouping enabled flag, window,
and node/guest keys as one notification-manager policy, so restart behavior is
identical to a live alert-configuration save.
Monitor construction installs external alert callbacks before publishing the
monitor to API handlers. `Monitor.Start` adds runtime WebSocket context and
lifecycle replay, but an immediately reconnecting agent must not create a
canonical warning while notification delivery is still unwired.
Monitor construction enables the alerts-owned persistent event log once for
each tenant alert manager. This is bootstrap wiring only: monitoring does not
own event types, retention, query semantics, or lifecycle/notification truth,
and an event-store startup or append failure must not interrupt polling, state
assembly, alert evaluation, or notification delivery.
Monitoring also owns the distinction between Proxmox VM power state and QEMU
guest-agent reachability: fresh or never-healthy VMs with an enabled but
unavailable guest agent stay `not-running`, while only VMs with recent healthy
guest-agent evidence may become `expected-unreachable`.
Monitoring owns source freshness cadence for Proxmox, PBS, and PMG resources:
the stale threshold is derived from the cadence the monitor actually polls at
(the configured polling interval, or the adaptive scheduler's maximum interval
when adaptive polling is enabled) with a minimum floor, so API-facing resource
status must not degrade merely because a healthy source is between normal poll
cycles.
Each unified view carries the thresholds its listed registry generation was
judged by, read with the listing from the same generation: the resource
store's configured ones, or none (the defaults) for a view built from mock
fixtures or a bare state snapshot.
`UnifiedResourceSnapshotWithStaleThresholds` hands them, with the seed, to
consumers that rebuild a registry from the monitor's resources (the resources
API reads each tenant monitor's), and the websocket broadcast coalesces split
host rows with them, so a coalesced row prefers readings by the same
freshness rule and thresholds as the view's own merges. Regression coverage:
`TestBroadcastCoalescedHostJudgesFreshnessByConfiguredThresholds` and
`TestUnifiedViewCarriesItsGenerationsStaleThresholds` in
`internal/monitoring/monitor_host_agents_test.go`.
PBS and PMG configured instances also have one monitoring-owned runtime
resource identity constructor. Poll publication, connection status, setup and
auto-registration checks, canonical alias resolution, and alert-policy bridges
must all use that constructor rather than rebuilding `pbs-<name>` or
`pmg-<name>` independently.
Proxmox guest enumeration is a generation boundary. VM and LXC collection and
enrichment must finish before one `State.UpdateGuestsForInstance` publication,
so readers never observe a VM-only or LXC-only intermediate snapshot. A failed
online cluster member retains only that member's last coherent guests and their
source-native `{instance}:{node}:{vmid}` IDs; a successful empty member
enumeration is authoritative and removes genuinely deleted guests. Collection
failure remains visible through source freshness/error state and must not be
converted into an authoritative empty inventory.
Large Proxmox generations separate authoritative inventory from optional deep
detail. The efficient poller must bound VM/LXC enrichment below the whole-cycle
deadline, rotate the first enriched row across cycles, and still publish every
non-template row returned by `cluster/resources` when that detail budget is
exhausted. VM and LXC work share the bounded slots concurrently so one guest
kind cannot permanently starve the other. Replication, storage, backup, and
snapshot enrichment owns independent runtime-scoped budgets; exhaustion or
cancellation of those optional tails must not turn a successfully enumerated
PVE API connection into `unreachable`.
Running LXC filesystem detail may be supplemented by a node-local Unified
Agent report when the reporting host is securely linked to exactly one current
PVE node. Monitoring admits only bounded, normalized VMID/name/disk rows,
keys them by PVE instance, node, and VMID, and requires the subsequent API poll
to match the exact container name and running state. Server receipt time owns
the cadence-derived cache lease. Missing, expired, renamed, stopped, or
migrated observations fall back to the Proxmox API disk view; a fresh accepted
rootfs reading also replaces the row's primary disk summary before alerts and
history are evaluated.
The Proxmox API disk view must itself enumerate every configured mount point.
Stock PVE reports no per-mount LXC usage through the status API, so mounts
known only from the container config (`rootfs`/`mpX` keys with
`mp=`/`mountpoint=` targets and `size=` capacity) surface as disk rows carrying
the configured capacity and the negative unknown-usage sentinel instead of
being dropped (#1477, restoring the v5.1.32 behavior on the v6 line).
Config-only rows merge by mountpoint identity and must never displace an
existing live-usage entry such as the aggregate-seeded rootfs row; admitted
node-local `pct df` agent rows replace the config-derived view wholesale.
Consumers must treat negative usage as unknown, never as a measured zero — the
alert engine already skips such rows in both aggregate and per-disk
evaluation, and mock mode must keep at least one running container fixture in
this exact shape so frontend surfaces keep exercising it.
Host-agent report liveness is server-observed, not agent-clock-observed:
`ApplyHostReport` must stamp `Host.LastSeen`, agent-sourced Ceph cluster
freshness, and host-agent cluster sensor freshness from Pulse receipt time, so
a reporting machine with a slow or fast local clock cannot be ingested as stale
or keep offline/recovery alerts flapping while reports are still arriving.
Report ordering is a separate, source-authored contract. Current agents publish
a process-unique stream plus monotonic sequence, and monitoring must serialize
each host's complete accept-and-apply transition, reject duplicate/older or
retired-stream reports without replacing state or writing metrics/history/
alerts, and still advance receipt-time liveness for every authenticated
arrival. Transport activity must not extend `Host.LastSeen`, the accepted
telemetry lease, or alert recovery when the payload itself is rejected. The
accepted ordering watermark, reporting interval, accepted receipt time,
transport receipt time, and observation time are durable host continuity so a
server restart cannot let a delayed buffered report resurrect an operation
that a newer report already stopped.
Docker/Podman module reports follow the same source-stream ordering rule. Their
accepted stream, sequence, observation time, transport receipt time, and
retired streams are persisted separately from operator-managed Docker host
metadata before telemetry state is replaced, so a server restart cannot let a
buffered complete inventory overwrite a newer helper-degraded status report.
Legacy reports without a sequence retain timestamp-based reconnect-burst
protection, but a clock correction after a normal report interval must be
admitted rather than freezing telemetry indefinitely.
Host telemetry also has a reporting lease derived from the agent cadence.
Monitoring must not clear a genuinely active Unraid parity operation or Linux
RAID rebuild during an ordinary polling gap. Once the lease expires, it must
clear transient operation/progress fields from host and canonical resource
projections while retaining static topology/health evidence that remains in
the live last-known host snapshot. After a server restart, durable continuity
must still expire persisted operation alerts on the same accepted-telemetry
lease; missing telemetry then remains visible through the separate confirmed
connectivity lifecycle.
PBS backup snapshot refresh is a bounded monitoring hot path: group-level
snapshot fetches must run through the fixed worker pool in
`internal/monitoring/monitor_backups.go`, reuse cached snapshots on per-group
fetch failures, and must not allocate one goroutine or buffered result slot per
backup group in large PBS datastores. Live PBS backup state is intentionally
bounded: groups are processed newest-first, per-group snapshots are capped to
the newest bounded set of real fetched snapshots, and the per-instance PBS
backup list must not grow without an explicit monitoring-owned limit. Bounding
must never synthesize placeholder backup entries from group metadata: a
placeholder drops verification, size, file, and per-snapshot time data, which
users read as broken discovery and failed verification. PBS backup group cache metadata
must be pruned to the retained group set after a completed poll, while
preserving cache metadata only for groups still observed or intentionally reused
after transient datastore failures. Recovery-point ingestion started by backup
polling must be serialized and coalesced so slow store writes cannot retain one
full backup point batch per poll cycle. Complete authoritative enumerations
coalesce only within the same provider, ID-prefix, and instance scope; distinct
source scopes and non-reconciling event batches remain FIFO so bounding memory
does not discard independent recovery facts.
PBS datastore exclusions are applied to the cheap datastore-name listing
before any per-store RRD, status, garbage-collection, namespace, group, or
backup request. Exact, prefix, suffix, and contains patterns use the canonical
case-insensitive datastore matcher. An excluded datastore must be absent from
the monitoring snapshot, storage projection, and backup enumeration, and the
version-compatibility fallback must use the same pre-detail filter rather than
reintroducing requests for excluded removable or intermittently offline
stores.
PBS snapshot-to-guest attribution for VMIDs that exist on more than one PVE
location is evidence-driven, never guessed. When the direct PBS connection is
authoritative and pbs-type storage contents are dropped from the PVE backup
list, storage backup polling must still harvest each listed snapshot as a
per-connection guest confirmation carrying the storage it was listed from
(storage, type, VMID, backup time). A storage listing is evidence that the
connection can see a snapshot, not that it authored it: a shared owner token,
a synced datastore, or an offsite copy all surface another cluster's
snapshots. A confirmation may therefore attribute a collision VMID only from
a storage view that never lists a snapshot another connection also lists; a
view that overlaps another connection's has demonstrated it sees snapshots it
did not author, so nothing it lists attributes anything and it can never
override other evidence. Guest backup-time sync weighs those confirmations
alongside a submission-source mapping (owner token, datastore, PBS instance —
scoped to the PBS instance, strongest first) learned from the same poll's
attributable snapshots, and where both speak they must agree. A source
component that was never positively attributed stops
resolution rather than deferring to weaker components, and the source mapping
stays inconclusive for a PBS instance while any PVE connection owning a
candidate guest there has had no snapshot attributed to it at all — an
unobserved connection may be submitting through the very same source.
Snapshots that remain unattributable stay dropped for colliding guests. The
confirmation evidence is monitoring-internal state: cleared when a PVE
instance is retired or its storage poll returns no pbs-type content, carried
forward per storage when that storage's content query fails so a partial poll
failure cannot evict attribution, and never serialized into state payloads or
snapshots.
Removed host-agent reconnect blocks are identity-scoped: matching may use the
canonical host ID or token-qualified machine/hostname continuity, but must never
block a distinct live host by hostname alone.
Proxmox cluster node identity is connection scoped and immutable after first
assignment. Configuration owns one retained identity ledger per PVE instance,
correlating current endpoint membership by the stored identity first, numeric
Proxmox node ID second, then unambiguous native-name or address evidence.
Native rename, re-IP, temporary absence, confirmed removal and later
reappearance must not regenerate an established identity or lose its optional
display-name override. Ambiguous case-folded names or address matches fail
closed instead of transferring identity. Monitoring publishes the configured
override when present and otherwise the current native node name; it also
retains current and prior native names for diagnostics, history lookup, and
search. Presentation must never alter polling addresses, credentials,
fingerprints, external URLs, action routing, source-native guest IDs, or
same-name-cluster provider scoping.
Node state aggregation is cluster-identity-scoped beyond presentation: a
hostname match alone must never bind a node to a host agent, and neither a
shared linked-agent identity nor an endpoint merge alias may fold two node
slots into one, when the two sides carry contradicting non-empty cluster
names. Cluster names shared across different connection instances are equally
ambiguous: such views stay split unless matching TOFU-captured TLS
fingerprints, propagated from the instance's endpoint records onto each node,
prove both views reach the same machine — then the legitimate
same-cluster-added-twice duplicate still folds into one slot even without
config-level endpoint overlap. Contradicting or unknown fingerprints keep the
fail-safe split. Agent binding follows the same doctrine: an endpoint address
match is not machine identity across sites that reuse RFC1918 ranges, so
every hostname- or IP-based agent match is rejected when the candidate
agent's linked nodes live in a different named cluster or carry a different
TLS fingerprint, and a hostname-based match is also rejected when the node's
endpoint IP is absent from the candidate agent's reported IPs. Weak-evidence
folds across connection instances — a bare-hostname endpoint alias or a
shared linked-agent identity — additionally require positive same-machine
proof (matching non-empty cluster identity or matching TLS fingerprints)
whenever cluster identity is in play on either side, because node names
repeat across sites and host agents key on /etc/machine-id, which cloned
template deployments reuse across unrelated machines. An unclassified node
(empty cluster name) must never displace or fold into an established named
cluster slot on such evidence alone. Two views that are both unclassified
still dedup freely, and address-based endpoint aliases keep folding on the
contradiction checks alone, so standalone and not-yet-classified endpoint
views of the same machine still fold together. PVE polling must run cluster
membership detection before the cycle's node-state commit so a newly added
connection's nodes carry their cluster identity from the first state write
whenever detection succeeds, instead of transiting the aggregation layer
unclassified.
Docker and Podman container CPU collection preserves the runtime-native raw
per-core CPU percent, but monitoring-owned history and alert threshold
evaluation use host-capacity-normalized CPU percent when host CPU capacity is
known. Raw runtime CPU remains alert/resource metadata, not the canonical
threshold value.
Docker and Podman container OOM state is runtime-authored evidence, not an
exit-code inference. Current agents must publish Docker inspect's `OOMKilled`
boolean for every inspected container, including explicit `false`; monitoring
must preserve that nullable boolean through report ingest, internal/frontend
models, and unified resources. An absent value means an older or reduced-fidelity
report did not provide the evidence and must remain distinguishable from both
confirmed OOM and confirmed non-OOM state. Exit code 137 proves only SIGKILL and
must not be promoted into OOM truth by monitoring.
Docker host identity collapse must be surfaced, not silently absorbed. Docker
report ingest keys host identity on the agent-reported machine ID in unified
mode, so cloned VMs that still share `/etc/machine-id` fold into one
`models.DockerHost` whose reports alternately overwrite each other (#1584).
Ingest must watch each resolved host identity for identity-field revisits: a
reported hostname (or machine ID) that switches away and returns to a value
already observed inside the monitoring-owned flap window proves two machines
share the identity, while a one-time hostname rename never revisits and must
not be flagged. An active conflict is published as
`models.DockerHost.IdentityConflict` carrying the flapping values so
downstream surfaces can warn, and it must clear on its own once only one
machine keeps reporting for the window. Monitoring must not auto-split the
collapsed identity: the machine ID is the identity key, and the remedy
(regenerating the clone's machine-id) belongs to the operator.
Host-agent identity collapse follows the same doctrine. Host report ingest
keys agent identity on the machine-derived agent ID, so template deployments
that still share `/etc/machine-id` fold two physical machines into one
`models.Host` whose reports alternately overwrite each other (hostname, report
IP, and interfaces flapping between sites), which also poisons node-agent
linking. Ingest must watch each resolved host identity for identity-field
revisits: a reported hostname or report IP that switches away and returns to a
value already observed inside the monitoring-owned flap window proves two
machines share the identity, while a one-time hostname rename never revisits
and must not be flagged. The report IP is tracked alongside the hostname
because template fleets often reuse hostnames across sites, leaving the
address as the only field that betrays the clone. An active conflict is
published as `models.Host.IdentityConflict` carrying the flapping values so
downstream surfaces can warn, and it must clear on its own once only one
machine keeps reporting for the window. Monitoring must not auto-split the
collapsed identity here either; the remedy belongs to the operator.
Unified Agent module projection is report-authored and additive. When one
agent sends host and Docker reports for the same machine identity, monitoring
must refresh one canonical machine carrying both facets so the Hosts and
Docker typed views expose the same canonical ID. A deliberately Docker-only
agent sends no host report and must remain absent from the Hosts view rather
than gaining a synthetic host facet from Docker telemetry.
Proxmox read-state rehydration is the inverse boundary: canonical
unified-resource CPU metrics are 0..100 percentages, while legacy
`models.Node.CPU`, `models.VM.CPU`, and `models.Container.CPU` remain Proxmox
0..1 ratio fields. Monitoring-owned read-state conversion must divide canonical
Proxmox node, VM, and LXC CPU percentages before handing them back to legacy
snapshot/current-row paths.
Proxmox guest live state, alerts, and history share one guest CPU-percent
normalizer. Efficient cluster polling and traditional per-node polling must
write VM/LXC history under the Proxmox guest ID in that same 0..100 unit, with
no core-count division and no in-guest host-agent substitution. A linked host
agent may supplement metrics the platform does not provide, but guest CPU and
its `vm` / `system-container` history target remain platform-owned so dashboard,
details, API state, alerts, and history cannot select different authorities.
Proxmox guest disk and network throughput has one cumulative-counter sampling
contract. `diskread`, `diskwrite`, `netin`, and `netout` are cumulative bytes;
the canonical rate is `(current counter - previous counter) / elapsed
observation seconds`, in bytes per second, with no 1024 divisor. Elapsed time
comes from the receipt time stamped immediately after the relevant Proxmox API
response is decoded, not from later guest-agent, filesystem, or metadata
enrichment. Each counter keeps an independent adjacent-sample baseline:
explicitly unchanged counters produce a valid zero, missing/null fields produce
unknown, out-of-order samples produce unknown without moving the baseline, and
a counter decrease caused by restart, reconnect, migration epoch change, or
wrap rebases that counter and produces a valid zero for the reset interval. A
source-uptime rollback rebases the complete counter epoch and leaves the first
post-restart rate unknown, including when a busy guest already surpassed its
pre-restart counter value before the next poll.
LXC rows may merge `cluster/resources` with the independently sampled
`/nodes/{node}/lxc/{vmid}/status/current` response. When both describe the same
uptime epoch, a lower status/current disk counter is lagging evidence and must
not overwrite a higher cumulative counter already observed from the cluster
row. Only an uptime rollback proves a restart and permits the lower value to
start a new epoch. This merge rule applies before rate calculation so endpoint
ordering cannot fabricate a reset, zero interval, or negative disk rate.
First-sample and missing-field unknowns remain internal validity state; the
legacy API/websocket guest number fields stay numeric, while history, unified
metrics, and alerts omit the unknown observation instead of manufacturing
zero. The rate-tracker identity is `(configured PVE instance, guest kind,
VMID)`: it survives node migration, separates QEMU from LXC, and prevents
duplicate configured cluster identities from sharing a concurrent baseline.
Idle and partial samples still refresh tracker liveness.
Proxmox row liveness uses the same cadence-derived threshold as source
freshness (`max(2 * poll cadence, 60s)`, the cadence being the configured poll
interval or, under adaptive polling, the scheduler's maximum interval). Node
offline grace and guest preservation must not expire between healthy 60- or
90-second polls, or between adaptive polls, and must not use a separate fixed
60-second timer.
Tenant monitor enumeration is monitoring-owned runtime topology, not a
reporting source of truth. `MultiTenantMonitor.ListOrganizationIDs` may expose
persisted organization IDs to API-owned background workers, but it must not
initialize monitors, start pollers, or reinterpret tenant IDs as monitored
resource health.
Organization deletion waits up to `tenantDeletionShutdownTimeout` (30 seconds)
for the tenant monitoring loop to exit before it refuses with
`tenant_shutdown_incomplete` and keeps the data. Process shutdown and
`RemoveTenant` keep the two-second `tenantMonitorShutdownTimeout`. Deletion
gates the data on that wait, so the budget must cover a loop that is still
starting up or finishing a tick. The observed case is mock mode:
`Monitor.Start` runs `startMockMetricsSampler` synchronously before its select
loop, and `prepareMockMetricsHistory` seeds seven days of fixture history
without a context, so a just-used organization's loop took 8-10 seconds under CI
load to see its cancelled context and the two-second budget turned an ordinary
delete into a refusal. A loop that never exits still refuses.
Proxmox physical-disk polling is also a continuity boundary. A failed or
permission-denied `disks/list` call must remain an error so the monitor can use
linked host-agent inventory or retain same-instance, same-node prior evidence;
it must never become a successful empty inventory that removes valid boot or
data disks. Between PVE disk polls, readback into `State.PhysicalDisks` accepts
only a disk with an actual Proxmox source observation. A linked Agent-only
SMART disk may inherit the PVE instance for presentation, but must not be
written back as PVE inventory; doing so adds and removes a false source and
change-journal rows when the PVE inventory is empty. Agent-only disks remain
visible through their Agent source, and an explicit failed-query Agent fallback
still supplies PVE inventory. `TestPhysicalDiskSkippedPollDoesNotPromoteAgentOnlySMARTToPVEInventory`
checks repeated empty-inventory/skipped-poll cycles and journal row counts;
its serial-bearing fixture yields a tags-only row if the source guard is
removed, matching the reported history shape without claiming the reporter's
topology or installed write rate. With the guard, all three cycles add no rows.
`TestPhysicalDiskSkippedPollPreservesSourceIdentity` checks genuine PVE disk
continuity. These fixtures do not establish the reporter's installed cause.
SMART enrichment matches serial, WWN, device path, and controller
member topology uniquely and fail-closed. Serial and WWN are interchangeable
hardware-identity carriers across reporters: comparison may case-fold and
remove only `naa.`, `eui.`, `wwn-`, and `0x` framing, but must reject
placeholders and must not truncate values, because sibling RAID volumes can
share a shortened WWN prefix. Enrichment preserves explicit failure over a
later coarse healthy value and lets explicit SMART endurance replace
contradictory Proxmox wearout. Missing
permission, ambiguous identity, standby, and absent SMART fields remain
neutral rather than borrowing telemetry from another disk.
The device-path match is the last resort, and a path names a slot, not a
disk: the row there may describe the slot's previous occupant, because the
agent has not reported since a swap or went silent and keeps its last rows.
`mergeHostAgentSMARTIntoDisks` refuses such a row when its identity
contradicts the Proxmox disk's (`diskinventory.HardwareIdentityConflict`):
nothing agrees across serial and WWN, and a field both producers report like
for like disagrees. WWNs are compared for every disk. Serials are compared
only where Proxmox's serial is the drive's own, as smartctl's is: a Proxmox
disk typed `nvme` against an NVMe row, and one presented with SCSI vendor
`ATA` (libata, or a SAS HBA's SCSI-ATA translation, where udev's `ata_id`
reads IDENTIFY) and not typed `usb` or `sas` against an ATA row. Any other
Proxmox serial may be a SAS address or SCSI designator (#1595) or a USB
bridge's own serial, so it counts toward agreement only; a USB bridge that
does expose the drive's serial is excluded too unless it reports vendor `ATA`.
Reporter spellings are not disagreement: serials compare on letters and digits
only (udev writes whitespace as underscores), a T10 vendor designator for an
ATA drive (`ATA`, the model, then the serial) is no evidence against the serial
it ends with, and a WWN that begins with the other is no evidence (udev's `ID_WWN` keeps only
the first 64 bits of a 128-bit NAA 6 identifier). The refusal comes after the
unique-path check, so a second row on the path still makes it ambiguous. The
WWN match itself compares through `HardwareIdentityMatch`, so the agent's
`5-c50-a1b2c3d4` spelling finds the Proxmox `0x5000c500a1b2c3d4` disk. The
registry's SAS path join applies the same WWN refusal (unified-resources).
Proxmox's disk inventory (`PVE::Diskmanage`) writes the literal `unknown` for
a serial or WWN udev cannot read; the poller records it as empty, so the
serial's collection state reads `missing` and the agent's serial or WWN can
fill it, where the placeholder used to read as a collected serial. Retained
evidence carries a previous serial only when it is a usable identity, so a
record from before this change cannot restore the placeholder. Identity keys
already ignored the placeholder, so the empty value keys the same; a disk whose
serial the linked agent now fills keys its Proxmox SMART samples on that serial
instead of its WWN or path, as the agent's own samples already were. Before the
guard, the retained row of a swapped-out disk filled or promoted the old serial
onto the replacement, which keyed it to the old disk's canonical resource and
history, and lent it the old disk's temperature and failed health
(`TestHostAgentSMARTRowForSwappedOutDiskDoesNotLendItsIdentity`,
`TestHostAgentSMARTMergeRefusesPathMatchAcrossContradictingIdentity`,
`TestRegistrySASPathJoinRefusesContradictingWWN`,
`TestProxmoxUnknownDiskIdentityIsRecordedAsUnreported`,
`TestHardwareIdentityConflict`). Without comparable evidence there is nothing
to act on: a row without a usable serial or WWN still matches its path, and a
replacement whose serial is not comparable (SAS, SCSI, USB) still takes a
stale row's identity when either record lacks a WWN. The registry's seeded
source mapping, which a restart reuses by the slot-shaped Proxmox source ID,
does not apply this guard yet.
Retaining evidence a disk poll could not collect follows the same identity
rule. `previousPhysicalDiskEvidence` looks up the earlier record by serial, then
by WWN, each compared through `diskinventory.HardwareIdentityMatch` against
both fields of the earlier record. `HardwareIdentityMatch`, which the
registry's linked-disk join also uses, may additionally re-pad a WWN field
written in the host agent's unpadded NAA 5 `naa-oui-id` spelling
(`5-c50-a1b2c3d4`, from smartctl's WWN fields) to the 16-digit form udev and
Proxmox report; serials are never re-padded. The registry's merged disk view
keeps the agent's WWN spelling, so without it one disk seen by both reporters
would read as two. The Proxmox source ID and the device token
name a slot, not a disk, so a slot match is refused when both records carry a
serial or WWN and no value is shared between them. Missing identity is not
conflicting identity: Proxmox's literal `unknown` and other placeholders count
as unreported, and a record whose serial arrives empty keeps the slot's last
known serial, still marked `missing`. Before this rule, a replacement in the
same slot took the previous occupant's retained temperature, controller, pool
and I/O, and a replacement record with an empty serial also took the old
serial, which keyed it to the old disk's canonical resource and SMART metric
series, so every later poll carried the borrowed serial forward
(`TestPhysicalDiskReplacementInSameSlotDoesNotInheritPreviousSerial`,
`TestPreviousPhysicalDiskEvidenceRejectsSlotMatchAcrossConflictingIdentity`).
A replacement that reports no usable serial or WWN cannot be told apart from
the same disk and still inherits. The linked agent's SMART merge, which runs
first, refuses a contradicting device-path row on its own terms (above).
Negative percentage-used counters remain unknown; values above 100 clamp to
exhausted before deriving remaining life, so invalid or over-limit controller
data cannot wrap into a fabricated healthy value.
QEMU's default disk serials are placeholders too. A virtual disk configured
without a serial reports one built from its drive ID or a per-VM counter, the
same on every VM built the same way: a SCSI disk's udev `ID_SERIAL_SHORT`,
which Proxmox's `disks/list` passes through as the serial, is `drive-scsi0`
under Proxmox, `drive-scsi0-0-0-0` under libvirt, and QEMU's automatic drive
ID for a `-drive` given none (`scsi0-hd0` for `if=scsi`, `none0` for
`if=none`), and an ATA disk's serial is `QM` plus a five-digit counter.
`diskinventory.IsUsableHardwareID` rejects exactly those shapes (`drive-scsi`
plus digits with at most three `-digits` groups, `scsi<n>-hd<n>`, `none<n>`,
and `QM` plus five digits), so a real serial that only shares a prefix, such
as `DRIVE-ASSET123` or `QM0001`, stays identity. It also
strips `naa.`, `eui.`, `wwn-` and `0x` framing before its all-zero and all-F
test, so `0x0000000000000000` is a placeholder as well. Before this rule every
node of a nested Proxmox cluster minted one `machine:drive-scsi0` canonical
disk and one SMART series for its first disk, and all but one node lost that
disk from inventory (`TestNestedProxmoxDefaultQEMUSerialsStayPerNode`,
`TestIsUsableHardwareIDRejectsQEMUDefaultDiskSerials`). The TrueNAS provider
applies the same rule: a disk takes its serial as canonical identity only when
the serial is usable, and a disk without a usable serial is keyed on its
source ID by native TrueNAS disk history and by the unified SMART writer for
disks without a native writer, which is the key the registry's metrics target
resolves for it. QEMU serials used to be accepted, so those disks shared one
key. A serial that was already rejected, such as `UNKNOWN`, had the opposite
problem: native history used TrueNAS's own disk identifier and the writer used
the canonical resource ID, so neither reached the chart
(`TestTrueNASPlaceholderDiskSerialsStayPerApplianceAndShareOneHistoryKey`,
`TestProviderPlaceholderDiskSerialsKeyNeitherIdentityNorHistory`). Upgrade
effect: a disk reporting one of these values changes canonical ID and SMART
metric key once. Samples recorded under the old key are not carried over,
because that key mixed every disk that shared the value. Path-shaped alert
references (Proxmox disk health and wearout, host-agent SMART) are unaffected;
anything keyed on the old canonical ID or metric key starts again under the
new one.
Proxmox cluster API polling has one configured connection authority: the
operator-saved `PVEInstance.Host` and its single credential set. Auto-discovered
member/corosync addresses remain ordered failover candidates and direct
reachability evidence; they are not per-node API connections or credentials and
must not randomly displace a healthy configured authority. When the authority
is healthy, recovery checks for unreachable members run bounded and
asynchronously so snapshots, storage content, replication, and other API-only
data do not wait on cluster-private addresses. When no endpoint is healthy,
recovery remains synchronous so a reachable member can restore service.
Periodic cluster discovery refreshes changed member addresses and rebuilds the
failover client. Pulse reachability evidence survives that reconciliation only
when the member's effective dial URL is unchanged; a network move resets the
old result until the new target is checked. Infrastructure Settings presents
one cluster-level API source while retaining member addresses and their
node-local Agent evidence.
Proxmox cluster membership is not the `/nodes` telemetry slice. A quorate,
complete `/cluster/status` response is absence-authoritative; members present
there but missing from `/nodes` remain in `models.State` with their stable
identity and last-known linkage while live CPU/uptime is cleared and
connection state is offline or stale. Failed, incomplete, non-quorate, or
cluster-identity-mismatched membership reads retain the last-known
node/endpoint union, break any pending absence sequence, and never advance
deletion. A member absent from a healthy
authoritative membership read is retired only after two consecutive
confirmations; the first omission remains in durable endpoint configuration so
a monitor restart resets the confirmation window rather than converting
uncertainty into removal. A newly reported member is admitted immediately.
Cluster display names are not global identity: config consolidation requires
overlapping endpoint authority, and different provider instances with the same
cluster/member names stay distinct in node and storage identity.
Endpoint address overlap is not sufficient identity either: sites that reuse
RFC1918 ranges can present colliding member IPs for different machines, so
TOFU-captured TLS fingerprints veto consolidation whenever the instance
authorities, a same-named endpoint, or a same-addressed endpoint carry
contradicting non-empty fingerprints. The fail-safe direction is fixed: a
certificate rotation may leave a genuinely duplicated cluster as two views,
but two distinct clusters must never be silently folded into one. The same
per-endpoint fingerprint evidence propagates onto monitored nodes so node
state aggregation applies the identical doctrine one layer down.


Storage risk assessment owns the wearout evidence boundary for every consumer.
`storagehealth.WearoutReported` is the single authority for whether a wearout
reading is evidence: `-1` is unreported, a positive value is always evidence,
and `0` is evidence only from a non-rotational device. Callers must not
recompute that boundary inline. Read-state projection of physical disks must
also preserve the unreported sentinel rather than collapsing an absent facet
onto the struct zero value, because `0` is a real reading meaning no endurance
remains.

Storage health assessment accepts an explicit SMART policy from alerting for
agent-only host disks while retaining the factory-policy entry points for all
other consumers. The policy controls failed-health, sector, media-error,
remaining-life, spare, and reallocated-sector classification; a zero threshold
disables only that rule. It must not alter collection truth, device identity,
the wearout evidence predicate, or the separate temperature classification.
This separation keeps provider physical-disk risk deterministic while allowing
the alert subsystem to apply resolved per-host settings without forking SMART
parsing or risk reason codes.

Discovery suppression for configured connections is a fail-closed obligation,
not a best-effort optimisation. Every configured PVE, PBS and PMG host is
resolved into the discovery IP blocklist so the scanner never fingerprints a
connection Pulse already holds credentials for. Those fingerprint probes are
unauthenticated by construction and land in the operator's own server logs as
authentication failures, so a host that escapes suppression is visible damage
on someone else's system rather than a missed optimisation. Resolution must
cover every IPv4 address a configured host answers with, not the first, because
the scanner reaches whichever address it reaches. A host that cannot be resolved
yields no entry and must say so in the log rather than passing silently, and it
must never prevent the remaining configured hosts from being suppressed.

External availability-probe freshness is evaluated against the effective
target cadence with a five-minute minimum grace. The server-authored receipt
time is authoritative for reporting freshness; the agent-authored check time
remains observation metadata and must not create or suppress a disconnect when
clocks differ. Missing, stale, or wrong-agent results are indeterminate
monitoring evidence: their `network-endpoint` resources degrade to warning
without manufacturing a target reachability incident. Monitoring aggregates
stale targets by current agent, updates the single canonical probe alert
lifecycle, and treats a fresh result from that same assignment as recovery.
Assignment trackers are removed with their targets and reset when agent identity
changes.

Storage capacity forecasting consumes monitoring-owned percentage history
through `internal/monitoring/storage_capacity_forecast.go`. The bridge combines
the durable SQLite series with the in-memory tail and current observation, then
caches the alert-owned trend result for a bounded interval. Durable history is
required in the read path so restart cannot erase a previously earned
confidence floor. Canonical storage resources must resolve their metrics target
before trend evaluation, keeping TrueNAS and vSphere forecast identity aligned
with the series written by `syncUnifiedStorageMetrics`; Proxmox and Ceph retain
their source-native storage history IDs. Monitoring supplies evidence only and
must not choose forecast horizons, severity, hysteresis, notification routing,
or lifecycle identity.

Rolling alert evaluation uses the same monitoring-owned canonical metric
identity and durable history as charts. `internal/monitoring/metric_window_provider.go`
resolves the unified resource metrics target, reads the fresh in-memory tail,
and falls back to the SQLite metrics store when that tail lacks the requested
coverage. Persistent fallbacks are briefly cached to bound restart-time query
load, merged by timestamp, and returned as observations only. Each request must
snapshot the active `MetricsHistory` under the monitor lock and use that same
history generation for its in-memory read and persistent-cache access; mock
history seeding may replace the active generation concurrently, but a request
must never lock one generation's cache mutex and unlock another's. When the durable
series and fresh in-memory tail contain the same timestamp, the in-memory value
is authoritative so an older persisted or rolled-up value cannot replace the
latest observation. Alert policy owns averaging, readiness, hysteresis, and
lifecycle decisions. Missing target, query failure, shallow history, or gapped
history must remain unknown at the alerts boundary rather than being replaced
by a synthetic healthy value.

Monitoring ingest keeps mock mode hermetic. The unified read path already
substitutes the mock snapshot wholesale, so anything that runs after that
substitution has to be suppressed explicitly rather than assumed hidden. Server
side agent report application discards real reports while mock mode is on, and
`recentStandaloneHostContinuityEntries` returns nothing, so persisted continuity
cannot reappear through the standalone host projection, the host online/offline
sweep, or availability probe display names. There is no real-polling exception
on this path: agent ingest is not gated on `PULSE_MOCK_KEEP_REAL_POLLING`, and
the read state is mock-substituted either way, so injecting real hosts would
only graft them onto fixture data.

Mock alert evaluation preserves the live Docker connectivity boundary. An
explicitly offline Docker fixture routes through `HandleDockerHostOffline`, not
the fresh-report `CheckDockerHost` path. Its last container states are unknown
supporting inventory rather than a new batch of independent exits, so the
confirmed host incident clears child alerts instead of producing one alert per
container.

Mock Docker hosts also keep the live removal boundary. Nested Docker-in-LXC
hosts are named after their guest's VMID, so a runtime mock config change can
drop one from the estate; `evaluateMockDockerHosts` then routes it through
`HandleDockerHostRemoved`, as a deleted live host is. `pruneStaleDockerAlerts`
clears the same alerts, but only on a state read or subscribed broadcast, so an
unwatched stack would otherwise keep them active for `/api/alerts/active`,
Patrol and escalations. The Docker pass shares the host-agent pass's lock and
mock-mode check, and leaving mock mode removes the fixture Docker hosts too, so
a pass in flight cannot leave Docker alerts behind in live mode.
`TestMockDockerHostLeavingFixtureUsesRemovalLifecycle`,
`TestMockDockerPassAfterLeavingMockModeRaisesNothing`,
`TestLeavingMockModeRemovesFixtureDockerHostAlerts`,
`TestMockFixturePassFromBeforeAnEstateRebuildIsRejected` and
`TestMockFixturePassPausedAcrossDisableAndReenableIsRejected` pin this.

Mock alert evaluation covers host agents the same way. Live agents are
evaluated by `CheckHost` as each report lands. Mock mode discards reports, and
the `evaluateHostAgents` health sweep reads monitor state, which never holds
fixture agents, so `checkMockAlerts` evaluates every fixture agent itself
through `evaluateMockHostAgents`. Without that pass no agent CPU, memory, disk,
temperature or offline alert could open against mock data, while the same
reading on a real agent would alert. The pass keeps the live lifecycle
boundaries:

- An offline fixture goes through `HandleHostOfflineWithCorrelation`, as a
  lapsed live report does, because `CheckHost` treats its input as a fresh
  report and would mark the host online.
- Agents are evaluated before nodes, so a node with an online linked agent
  hands its CPU, memory and disk alerts to the agent on the first tick, as
  node-link deduplication does in production.
- A runtime mock config change rebuilds the estate. An agent that leaves it
  goes through `HandleHostRemoved`, as a deleted live agent does, so its alerts
  and node link do not outlive it.
- Leaving mock mode routes the fixture agents through `HandleHostRemoved`.
  `ClearActiveAlerts` drops their alerts but not their node links, and a
  leftover link would keep suppressing CPU, memory and disk alerts on any node
  that later carries the linked fixture node's ID. `SetMockMode` ends the
  mock-mode epoch before forgetting fixture hosts, so evaluations holding an
  older epoch cannot recreate alerts after the clear. Departures refused by
  the epoch fence remain tracked for the switch to remove.
- Ticks start passes concurrently, so a pass whose snapshot predates an
  estate rebuild can reach the lock after a newer one. The agent and Docker
  steps also carry the fixture's structural revision and reject revisions
  older than the last applied or disabled one. Both checks apply: the epoch
  stops mode-crossing evaluations, while the revision stops an older estate
  replacing newer host ownership within one epoch. Guest, node, storage, PBS,
  PMG and physical-disk evaluations use the epoch fence, not host revision
  tracking.

Fixture agents must carry readings a real agent could report. Mock Kubernetes
pods share 0.7 single-pod memory footprints per node, which keeps a node's pods
at or below 58% of allocatable memory in steady state, and a 60% per-node
ceiling clips the transient while rescheduled pods' readings settle. A dense
node and its linked agent therefore no longer read a constant 100% and raise a
critical alert no real cluster would. Whether a demo agent alerts at default thresholds depends on
the readings each boot draws. On 2026-10-06, private stacks with the public
demo's estate (8 nodes, 2 standalone agents and 3 Kubernetes node agents)
opened none on one boot and one (a standalone agent at 91% memory) on another.
With the agent memory trigger lowered to 60%, exactly the agents above it
alerted and their linked Proxmox nodes raised none, and shrinking the estate
through the mock settings API cleared the departed agents' alerts on the next
pass. Those are measurements, not test guarantees. The tests pin the
boundaries:
`TestCheckMockAlertsEvaluatesHostAgentsBeforeLinkedNodes`,
`TestCheckMockHostAlertsUsesHostLifecycleForOfflineFixtures`,
`TestLeavingMockModeReleasesFixtureAgentNodeLinks` and
`TestMockHostAgentPassAfterLeavingMockModeRegistersNothing` in
`internal/monitoring/monitor_mock_alerts_test.go`,
`TestMockHostAgentLeavingFixtureUsesRemovalLifecycle` in
`internal/monitoring/monitor_host_agent_removal_lifecycle_test.go`, and
`TestMockKubernetesDenseNodesStayBelowAllocatableMemory`,
`TestMockKubernetesRescheduledPodsDoNotSaturateReceivingNode` and
`TestMockKubernetesLoneHeavyPodStaysUnderNodeCeiling` in
`internal/mock/generator_test.go`.

Mock alert evaluation covers physical disks as well. Live Proxmox disks are
evaluated by the physical disk poller in `maybePollPhysicalDisksAsync`, which
only polls configured Proxmox instances, so fixture disks had no evaluation
path. The fixture deliberately keeps a FAILED health cohort and worn SSDs
stable across restarts, so the estate showed failing and "Replace Now" disks
with no `disk-health` or `disk-wearout` alert. `checkMockAlerts` now evaluates
fixture disks through `checkMockPhysicalDiskAlerts`. Both paths share the
per-disk rule in `checkPhysicalDiskAlerts` (a device matched by the linked
agent's `--disk-exclude` patterns is evaluated as healthy, otherwise
`CheckDiskHealth` judges the disk), and the mock pass applies the poller's
boundary around it:

- Only disks on online nodes are evaluated, as the poller only evaluates nodes
  it reached. A disk on an offline node keeps whatever alert it had. The mock
  pass treats every disk on an online fixture node as collected; the poller
  also skips a node whose disk query and SMART fallback both failed. Nodes are
  matched by instance and name, as the poller only pairs a disk with its own
  instance's nodes.
- Excluding a disk resolves the alerts it already raised, because the excluded
  device is evaluated as healthy rather than skipped.
- The poller's wait for host-agent links to settle after a restart (#1674) is
  not applied: fixture links and exclusions are complete from the first pass.
- A disk whose node name leaves the estate loses its alerts through
  `CleanupAlertsForNodes`, which the same pass already runs. That cleanup
  matches node names only, so it relies on fixture node names being unique
  across instances, which they are.

`checkPhysicalDiskAlerts` also hands each disk's temperature to
`CheckProxmoxDiskTemperature`, so a Proxmox disk that only the node's sensors
report alerts under the disk temperature policy, as its Running Hot verdict
already says it should. It passes the reading only when this poll collected it
(`collectedPhysicalDiskTemperature`, `diskinventory.TemperatureCollected`), so
a retained last-known value holds the alert rather than judging it, and it
passes `AgentSMARTReported`, which `mergeHostAgentSMARTIntoDisks` sets on a
disk a still-reporting linked agent lists in its SMART report (disks built
from that report when the Proxmox query fails go through the same merge). A
disk the agent lists is the agent's: `CheckHost` raises its temperature
alert, and the PVE disk check closes its own as moved to the agent, so no
disk alerts twice. Once the agent's lease lapses (`ExpireHostTelemetry` marks
the host offline) its retained rows still enrich the disk, but ownership
returns to the PVE check, which judges the node's own current readings; an
agent newly linked to a disk with an open PVE alert takes it over on the next
disk poll. An excluded device closes its temperature
alert with the health and wearout ones. Fixture disks carry no agent SMART
merge, so on a mock estate every hot disk on an online node alerts.
`TestMergeHostAgentSMARTIntoDisksMarksDisksTheAgentReports` and
`TestCheckPhysicalDiskAlertsRaisesProxmoxDiskTemperatureAlerts` in
`internal/monitoring/physical_disk_roundtrip_test.go` pin the marker,
the collected reading, agent ownership and exclusion.

A disk the operator split from the agent's disk (report-merge on the merged
disk) is not paired with the agent's row. `mergeHostAgentSMARTIntoDisks` asks
the resource store, through `diskAgentSplitStore`
(`unifiedresources.MonitorAdapter.ProxmoxDiskAgentSMARTSplit`), once per disk
it matches a row to, on the full poll and on the skipped polls between, and a
split disk takes nothing from the row: no attributes, I/O, health, identity,
type or temperature, and no `AgentSMARTReported`. It takes no agent-supplied
node-sensor temperature either: the node poll takes the node's SMART list from
the linked agent when the agent's report carries a usable temperature
(`mergeTemperatureData`; `hostSuppliesNodeSMARTTemperatures` mirrors that
rule), and its legacy NVMe list is matched by order, so
`dropNodeSensorTemperature` removes a reading `mergeNVMeTempsIntoDisks` took
from either, leaving the PVE disk check no such reading to judge beside the
agent's own alert, while a reading from the SSH collector's list stays (the
rule needs the agent still reporting). The
split disk is still evaluated by `checkPhysicalDiskAlerts` whenever its node
is polled. It is marked
`AgentSMARTSplit` (internal poll evidence, never serialized), so the poll's
retention of unavailable evidence restores nothing into it. The decider is
asked about the observation under its own ID, as the registry holds it. The
Proxmox-query-failure fallback omits the rows the operator split from the disk
in their slot, so while the query fails that disk's Proxmox record is not
listed (a node every row of which is split keeps its last records, and is not
evaluated by `checkPhysicalDiskAlerts`, as any unqueried node). A store that holds no
decisions, or none for the pair, leaves the merge as it was. The decision
comes from the registry generation the monitor last built, so a split applies
from the first full disk poll that collects the node from Proxmox and matches
the agent's row after the rebuild that loaded it; the skipped polls before then
mark the record and drop its node-sensor temperature but leave what an earlier
poll merged into it.
`TestMergeHostAgentSMARTIntoDisksLeavesDisksTheOperatorSplitAlone`,
`TestHostAgentSMARTFallbackSkipsRowsTheOperatorSplit`,
`TestFailedDiskQueryListsOnlyTheRowsTheOperatorDidNotSplit`,
`TestMergeHostAgentSMARTIntoDisksKeepsSSHNodeSensorReadingOfSplitDisk`,
`TestDropNodeSensorTemperatureRemovesOnlyAgentSuppliedReadings`,
`TestMergeNVMeTempsLabelsReadingsWithTheNodeSensorSources`,
`TestHostSuppliesNodeSMARTTemperaturesFollowsUsableReadings` and
`TestOperatorSplitKeepsAgentSMARTOffTheProxmoxDisk` in
`internal/monitoring/physical_disk_roundtrip_test.go` pin the merge, the
fallback and the poller end to end, and `TestProxmoxDiskAlertsRunOnMergedDiskState`
in `canonical_guardrails_test.go` pins the signature and call.

Switching mock mode fences the alert evaluations that read mode-dependent data
(`mockModeFence`, `internal/monitoring/mock_mode_fence.go`). `GetState`, the
fixture graph, the unified read view, the recovery rollups and the connection
ledger all change source when the mode flips. A pass already holding one side's
data used to reach the alert manager after `SetMockMode` cleared it, and its
fixture alerts then stayed in live mode, with nothing left to evaluate or
resolve them, until the 24-hour stale sweep. Each of these paths takes a scope
before it reads anything mode-dependent and sends its alert-manager calls
through it:

- `checkMockAlerts`, for every loop including the host-agent and disk steps
  and the mock alert snapshot cache;
- `checkBackupAlerts`, shared by the PVE, PBS and mock polls, and
  `pollGuestSnapshots`, whose guest inventory is the fixture read view in mock
  mode;
- `checkConnectionAlerts`, whose ledger lists fixture connections in mock mode;
- `pruneStaleDockerAlerts`, which removes Docker alerts for hosts missing from
  the read view;
- the resource-store refresh. `updateResourceStore` and
  `updateResourceStoreForRead` take the scope of the state they rebuild from
  (`currentStateWithScope` for `GetState`), and the unified alert sync
  (`syncUnifiedResourceAlertsToState`) runs its policy reconcile, identity and
  availability-link migrations, per-resource metrics and incident sync under
  it.

`SetMockMode` flips the mode, ends the epoch, waits for the alert-manager calls
already admitted under it and only then clears, so nothing these paths read
before the flip lands after the clear, in either direction. Switches are
serialized. The wait covers the calls already running, not whole passes, but it
has no fixed wall-clock bound: backup evaluation and unified incident sync are
estate-wide calls, and an admitted call can include metric-window reads and
migration persistence. Unified metrics are evaluated one resource per call, and
reads, capacity-trend queries and registry rebuilds stay outside the wait.
Admission never blocks, so a call made inside an admitted one cannot deadlock a
waiting switch; `SetMockMode` itself must not be called from inside an
evaluation.

A registry rebuild publishes shared state, so a rebuild holding the mode the
monitor left can finish after a newer refresh published and replace the
registry that refresh reads back. A rebuild whose epoch has ended does not
start, and one already running is tracked: a refresh evaluates the registry it
read back only if no ended-epoch rebuild overlapped it, and otherwise leaves the
alerts to the next refresh. Even without a late rebuild, the registry still
holds the fixture estate right after leaving mock mode, and may hold it until an
undisturbed rebuild in the new epoch has replaced it
(`mockModeFence.registryCurrent`); in mock mode the read paths use the fixture
view instead. The live readers that act on that inventory (Docker pruning, the
backup-age guest lookup, the guest inventory of the backup and snapshot
pollers, and the pollers' previous-node and previous-guest carry, which writes
back into live state) read it through `currentModeReadState`, which serves a
view of current monitor state meanwhile. That view omits provider-owned
supplemental resources and persisted manual agent links, so a carry in the
window can fall back to platform guest memory for one poll, and it copies the
estate, so only callers outside per-resource loops use it. Every other live
registry reader (`GetUnifiedReadStateOrSnapshot` callers such as metrics
targets and system usage, and the broadcast view) may still see the fixture
estate until the next rebuild, as before, and metric samples synced from a late
rebuild stay in metric history.

The fixture sources behind the router's mock supplemental adapters
(`mock.SupplementalRecords`, `mock.SupplementalChanges`) serve nothing while
mock mode is off. The router swaps those adapters for the real TrueNAS and
vSphere pollers only after `SetMockMode` returns, one source at a time with a
refresh after each, and the adapters used to fall back to default fixtures, so
a single switch to live mode raised fixture vSphere incidents. Two concurrent
mock-mode requests can still leave the router's provider set disagreeing with
the mode (fixture adapters serving nothing in live mode, or real pollers
feeding mock mode); neither raises fixture alerts in live mode.

Report admission and the live pollers outside the paths above are not fenced:
a live report or poll already past its mock-mode check when mock mode is
switched on can still evaluate after that clear, and its alerts can persist
until mock mode is left.

The mode is process-wide while each monitor keeps its own fence, alert manager,
fixture agents and state, and the server runs one monitor per organization, so
`SetMockMode` switches every running monitor, whichever tenant's monitor it is
called on (the mock-mode API calls the requesting tenant's, the demo-fixture
licence sync the default organization's). `Start` joins a process-wide set of
running monitors under `mockModeSwitchMu`, and leaves it when its loop returns.
A switch flips the mode once, then ends each monitor's epoch, waits for its
admitted calls, clears its alerts, forgets its fixture agents when leaving mock
mode and resets its state, and only then restarts the mode-dependent runtime of
each monitor in the set. `Start` sets its runtime context before it joins and
chooses the mock metrics sampler or discovery while it joins, under the same
lock, so no switch lands between reading the mode and starting the runtime for
it, and a switch that reaches a monitor not yet joined leaves the runtime to
its `Start`. A monitor that missed switches between `New` and `Start` first
leaves what it holds: `New` records the mode and the number of switches made so
far, and `Start` compares both, so even a round trip that brought the flag back
drops the fixture alerts a read path raised meanwhile, as well as the live
alerts `New` restored outside mock mode. A monitor that is not running is
switched only when the switch is made through it; a call for the mode the flag
already holds still moves any reached monitor whose alerts belong to the other
mode, and leaves the rest alone.

Release builds reach several tenant monitors in mock mode only on an instance
running `DemoMode` with the `demo_fixtures` entitlement (release
`ValidateEnablement` refuses mock fixtures otherwise) and with more than one
organization's monitor running; the licence sync switches only for the
default organization.

The proofs park a real evaluation at a fixed point, switch modes and release
it. `TestLeavingMockModeRefusesTheRestOfAnInFlightPass` holds a mock pass at its
first storage capacity read, `TestLeavingMockModeDiscardsAnInFlightFrontendRefresh`
holds a frontend read in its supplemental-records supply,
`TestLeavingMockModeDiscardsAnInFlightBackupEvaluation` holds a backup
evaluation at its guest lookup, and
`TestLeavingMockModeDiscardsAnInFlightConnectionCheck` holds the ledger read.
Against the unfenced code they left about 40 fixture storage, disk and Docker
alerts, 5 vSphere incidents and 137 backup-age alerts in live mode, and counted
a mock-mode observation toward a live connection's three-observation
confirmation. `TestLeavingMockModeWaitsForAnInFlightEvaluation` holds an
evaluation in its CPU window read, outside the alert manager's lock, and fails
if the switch returns first; `TestLiveRefreshRejectsCollectedRecordsFromMockMode`
holds collected fixture records across a mode switch and checks that a live
refresh publishes and evaluates live provider records without fixture leakage.
`TestDockerPruneIgnoresTheRegistryMockModeLeft` raises a live Docker alert
before any live rebuild and fails if pruning reads the fixture registry, and
`TestPreviousStateCarryIgnoresTheRegistryMockModeLeft` fails if the
previous-state carry returns fixture nodes or guests. All eight are in
`internal/monitoring/monitor_mock_alerts_test.go`;
`TestSupplementalFixturesServeNothingOutsideMockMode` in
`internal/mock/platform_fixtures_test.go` pins the fixture sources;
`TestMockModeFence*` in
`internal/monitoring/mock_mode_fence_test.go` pin the fence itself, and
`TestStoreRefreshTakesItsMockModeScopeBeforeReadingState` in
`internal/monitoring/canonical_guardrails_test.go` rejects a call that reads
state in an argument ahead of `m.mockModeFence.begin()` and pins the
end-epoch-then-clear order in `endMockModeEpoch`, which `SetMockMode` runs for
every monitor it reaches.

`TestLeavingMockModeClearsEveryRunningTenantMonitor` and
`TestEnteringMockModeClearsEveryRunningTenantMonitor` start the default and one
other tenant monitor through `MultiTenantMonitor`, wait until both have joined,
raise alerts in both, switch through one of them and check both. Against a
switch that reached only its own monitor, the default monitor kept its fixture
alerts (69 to 104 across runs) and all 69 fixture agents after leaving mock
mode through the other tenant, and that tenant kept its live alert after mock
mode was entered through the default monitor. `TestMonitorStartedAfterAMockModeSwitchLeavesTheModeItWasBuiltIn`
fails if `Start` joins without leaving the mode its monitor was built in, and
`TestSetMockModeAlignsAMonitorWhoseModeTheFlagAlreadyHolds` fails if a call for
the flag's current mode returns without moving such a monitor, and
`TestMockModeSwitchLeavesAnUnjoinedMonitorsRuntimeToItsStart` fails if a switch
starts the runtime of a monitor whose `Start` has not joined, and
`TestMonitorStartedAfterAMockModeRoundTripDropsWhatItRaisedMeanwhile` fails if
`Start` compares only the mode. All six are in
`internal/monitoring/monitor_mock_alerts_test.go`.

The disk evaluation runs on the mock alert tick, not the poller's disk
interval (five minutes by default, `PhysicalDiskPollingMinutes` per instance).
`CheckDiskHealth` logs at error level whenever it evaluates a FAILED disk
(models with a known firmware bug skip the health check) and at warn level for
a worn SSD, so an evaluated fixture disk in either state logs on every tick. On
2026-10-06, private stacks with the public demo's estate (8 nodes) carried 12
Proxmox disks, none FAILED. Baseline against patched, the only new disk alert
was one warning-level `disk-wearout` (a 7% SSD on `pve5`), whose resource ID is
one of the disk resource's canonical aliases, and neither stack had a
`disk-health` alert. `CheckDiskHealth` raises only those two alert types, so
the other differences between the two boots (ZFS and Docker service alerts) are
not from the disk pass; those fixtures draw random state each boot. The default
test estate raises five `disk-health` and one `disk-wearout` alert. The tests
pin the boundary: `TestCheckMockAlertsEvaluatesPhysicalDisks` and
`TestCheckMockPhysicalDiskAlertsFollowsPollerBoundary` in
`internal/monitoring/monitor_mock_alerts_test.go`, with
`TestProxmoxDiskAlertsRunOnMergedDiskState` in
`internal/monitoring/canonical_guardrails_test.go` keeping the poller on the
shared helper.

Host and container-runtime disk collection supports an explicit include list
for filesystems hidden by Pulse's automatic virtual/container filtering. The
include list is bounded to that automatic filter; explicit disk exclusions
still win and disk-I/O filtering retains its existing exclusion semantics.

Unified host removal is authoritative for monitoring state owned by that
agent, including Docker/Podman runtime reports and active alerts keyed to the
removed host. Pulse must republish the remaining unified read state after the
cleanup so readers cannot retain orphaned runtime or alert projections.

## Canonical Files

1. `internal/monitoring/monitor.go`
2. `internal/monitoring/poll_providers.go`
3. `internal/monitoring/monitor_discovery_helpers.go`
4. `internal/monitoring/monitor_polling_node.go`
5. `internal/monitoring/monitor_pve.go`
6. `internal/monitoring/monitor_pve_storage.go`
7. `internal/monitoring/node_disk_sources.go`
8. `internal/monitoring/metrics.go`
9. `internal/monitoring/metrics_history.go`
10. `internal/unifiedresources/read_state.go`
11. `internal/unifiedresources/monitor_adapter.go`
12. `internal/unifiedresources/views.go`
13. `internal/monitoring/connected_infrastructure.go`
14. `internal/monitoring/reload.go`
15. `docker-entrypoint.sh`
16. `internal/monitoring/truenas_poller.go`
17. `internal/monitoring/vmware_poller.go`
18. `internal/monitoring/monitored_system_usage.go`
19. `internal/dockeragent/swarm.go`
20. `internal/dockeragent/collect.go`
21. `pkg/proxmox/ceph.go`
21a. `pkg/proxmox/cluster_client.go`
21b. `pkg/proxmox/client.go`
21c. `pkg/proxmox/io_counters.go`
22. `pkg/proxmox/zfs.go`
22a. `pkg/pbs/client.go`
23. `internal/monitoring/guest_memory_sources.go`
23a. `internal/monitoring/guest_memory_agent.go`
23b. `internal/monitoring/guest_memory_lifecycle.go`
24. `internal/monitoring/guest_memory_stability.go`
25. `internal/monitoring/monitor_polling_vm.go`
26. `internal/monitoring/monitor_pve_guest_builders.go`
27. `internal/monitoring/monitor_pve_guest_poll.go`
28. `internal/monitoring/guest_disk_stability.go`
29. `internal/monitoring/mock_metrics_history.go`
30. `internal/monitoring/mock_chart_history.go`
31. `internal/monitoring/availability_poller.go`
31a. `internal/availabilityprobe/probe.go`
31b. `internal/config/availability.go`
31c. `pkg/tlsutil/certificate.go`
32. `internal/monitoring/scheduler.go`
33. `internal/monitoring/docker_detection.go`
34. `internal/monitoring/monitor_polling_containers.go`
34a. `internal/monitoring/monitor_agent_lxc_filesystems.go`
35. `internal/mock/fixture_graph.go`
35a. `internal/mock/action_fixtures.go`
35b. `internal/mock/availability_fixtures.go`
35c. `internal/mock/recovery_points.go`
35d. `internal/mock/integration.go`
35e. `internal/mock/alert_incidents.go`
35f. `internal/mock/alert_history.go`
36. `internal/dockeragent/docker_client.go`
37. `pkg/agents/docker/report.go`
38. `internal/models/models.go`
38a. `internal/models/proxmox_guest_state.go`
38b. `internal/models/metrics_types.go`
39. `internal/models/models_frontend.go`
40. `internal/models/converters.go`
41. `internal/models/deepcopy.go`
42. `internal/mock/generator.go`
43. `internal/mock/demo_scenarios.go`
44. `internal/kubernetesagent/agent.go`
45. `pkg/agents/kubernetes/report.go`
46. `internal/monitoring/temperature.go`
47. `internal/truenas/client.go`
47a. `internal/truenas/transport.go`
47b. `internal/truenas/pool_capacity.go`
48. `internal/truenas/disk_health.go`
49. `internal/truenas/provider.go`
50. `internal/models/ceph_cluster_identity.go`
51. `internal/truenas/types.go`
52. `internal/monitoring/monitor_alert_sync.go`
52a. `internal/monitoring/storage_capacity_forecast.go`
53. `internal/monitoring/platform_poller_shared.go`
54. `internal/monitoring/monitor_backups.go`
55. `internal/monitoring/resource_stale_thresholds.go`
56. `internal/monitoring/recovery_ingest.go`
56a. `internal/monitoring/pbs_protection_observation.go`
56b. `internal/monitoring/pve_protection_observation.go`
57. `internal/monitoring/multi_tenant_monitor.go`
58. `internal/monitoring/proxmox_action_observer.go`
59. `internal/monitoring/agent_fleet_doctor.go`
60. `internal/config/host_continuity.go`
61. `internal/monitoring/docker_metadata_migration.go`
62. `internal/monitoring/kubernetes_metadata_migration.go`
62a. `internal/monitoring/monitor_xcpng.go`
63. `internal/monitoring/metadata_stores.go`
64. `internal/monitoring/system_alerts.go`
65. `internal/monitoring/deadman.go`
63a. `internal/config/docker_metadata.go`
63b. `internal/config/guest_metadata.go`

## Shared Boundaries

Helper-backed Proxmox inventories carry explicit complete/partial status and omitted VMIDs. Before cache mutation the monitor validates this shared contract. A complete inventory replaces only the exact linked-node/agent cache; a partial inventory removes that agent's explicitly omitted guests without renewing their data. Legacy reports retain legacy row admission and expiry.

PBS polling owns the internal `PBSInstance.NodeMetricsUnavailable` discriminator:
each poll starts unavailable and only a successful non-nil node-status result
clears it. A denied or failed node-status endpoint does not invalidate successful
connectivity or independently accessible datastore inventory. In-process state
copies preserve this evidence; JSON deliberately does not carry it. The
zero-value compatibility default is not persisted availability evidence.
Only that successful node-status observation writes CPU and memory percentages
to the PBS service's `agent:<PBS source ID>` history target. Missing or invalid
node measurements append no sample, rather than a false zero; measured zero is
valid. The service series is separate from a correlated Pulse Agent's host
series and contains no inferred disk, network, or I/O measurements.
Proof: `internal/models/metrics_types_test.go` and
`internal/monitoring/monitor_pbs_coverage_test.go`; the latter exercises real
HTTP polling through normal alert-manager publication, not destination delivery.
`TestPollPBSNodeStatusRecordsServiceHistory` verifies volatile and persisted
service-history reads plus denial/recovery without fabricated samples.


1. `internal/config/host_continuity.go` shared with `agent-lifecycle`: the durable host identity, report-order watermark, and removal tombstone journal is jointly owned by agent lifecycle admission and monitoring report continuity.
2. `internal/kubernetesagent/agent.go` shared with `agent-lifecycle`: the Kubernetes native agent runtime is both a monitoring inventory source and an agent lifecycle Pulse control-plane transport client.
3. `internal/mock/fixture_graph.go` shared with `performance-and-scalability`: the canonical mock fixture graph is both monitoring-owned runtime data and a protected large-estate demo transport hot path.
4. `internal/mock/generator.go` shared with `performance-and-scalability`: mock metric generation is both monitoring-owned runtime data and a protected large-estate demo update hot path.
5. `internal/mock/integration.go` shared with `performance-and-scalability`: the mock runtime scheduler is both monitoring-owned sampling infrastructure and a protected large-estate demo cadence boundary.
6. `internal/models/models.go` shared with `agent-lifecycle`: removed host-agent identity aliases and tombstone state are both agent lifecycle authority and monitoring runtime report state.
7. `internal/monitoring/monitor.go` shared with `agent-lifecycle`: monitor construction owns both monitoring runtime initialization and fail-closed agent lifecycle journal hydration before report admission.
8. `internal/monitoring/monitor_agents.go` shared with `agent-lifecycle`: server-side Unified Agent report, removal, token binding, tombstone expiry, and re-enrollment semantics are jointly owned by agent lifecycle authority and monitoring ingest.
9. `internal/proxmoxidentity/backup_identity.go` shared with `alerts`, `storage-recovery`: Proxmox PBS backup subject identity is a shared runtime boundary for monitoring backup freshness, backup-age alert attribution, and recovery-point guest mapping.
10. `pkg/agents/host/report.go` shared with `agent-lifecycle`: the Unified Agent host report is both an agent lifecycle authored-state contract and a monitoring ingest contract for host maintenance posture.

## Extension Points

1. Add pollers/providers and discovery-provider coordination through `internal/monitoring/poll_providers.go` and `internal/monitoring/monitor_discovery_helpers.go`
   The PVE/PBS/PMG providers share one wiring layer inside
   `internal/monitoring/poll_providers.go`: instance listing, instance
   description, and connection-status publication go through the generic
   `sortedClientNames` / `describeProviderInstances` /
   `providerConnectionStatuses` helpers, and the prefixed PBS/PMG providers
   are built by `newPrefixedPollProvider` from a `prefixedPollProviderSpec`.
   New scheduler-backed platform providers extend those helpers instead of
   re-rolling per-platform copies of the same loops.
   Source freshness thresholds for PVE/PBS/PMG resource ingestion are derived
   through `internal/monitoring/resource_stale_thresholds.go` from the active
   poll interval and passed into the unified-resource adapter. New pollers or
   config paths that change source cadence must update that derivation instead
   of hard-coding stale windows inside registry or API code.
   Non-default tenant monitors poll against a detached config copy (#1619), so
   a system-settings save reaches them only as PBS and PMG runtime polling
   overrides. Under fixed-cadence scheduling, `Monitor.resourceStaleThresholds`
   reads those overrides through the same clamped setting readers as the
   scheduler, so a saved interval moves the monitor's resource freshness and
   polling together. A PVE interval save reloads every monitor from saved
   config instead. `ResourceStaleThresholdsForConfig` stays the config-only
   derivation for callers without a live monitor, such as adapter
   construction. Regression coverage:
   `TestMonitorResourceStaleThresholdsFollowRuntimePollingOverrides` in
   `internal/monitoring/canonical_guardrails_test.go`.
   An adaptive scheduler (`ADAPTIVE_POLLING_ENABLED` or `system.json`; off by
   default, no UI) selects its own intervals and never reads the per-platform
   intervals or those overrides. A healthy instance's staleness score is near
   zero after each success, so its cadence stretches toward the scheduler's
   maximum interval (5 minutes by default; a healthy instance polls a few
   minutes apart). Selected intervals never exceed it; a poll can still run
   later than its slot while workers are busy; the factor of two gives
   headroom for that. Under adaptive scheduling
   `Monitor.resourcePollIntervals` therefore derives PVE, PBS and PMG
   freshness from `AdaptiveScheduler.MaxInterval()`, and
   `ResourceStaleThresholdsForConfig` derives it from the
   `AdaptivePolling*Interval` bounds normalized by `normalizedSchedulerConfig`,
   the normalization the scheduler itself applies. Deriving it from the
   per-platform intervals (1 minute for PBS and PMG, 10 seconds for PVE) left
   healthy rows stale for much of every cycle. The Proxmox threshold is also
   the window the node grace helpers (`determineNodeIDAndStatus`,
   `preserveOrExpireNodes`) hold a node online after it was last seen online,
   and the node temperature carry window derives from the same cadence with
   its five-minute floor. With the defaults that hold is 10 minutes, observed
   at the next poll after it lapses: a longer detection delay than fixed
   polling, which is the price of polling a healthy fleet every few minutes.
   A valid `/cluster/status` membership report that a member is offline is
   authoritative (`reconcilePVENodeInventory`) and still applies immediately.
   Regression coverage:
   `TestMonitorResourceFreshnessFollowsAdaptiveCadence`,
   `TestResourceStaleThresholdsForConfigFollowAdaptiveBounds` and
   `TestAdaptiveNodeOfflineGraceSpansTheAdaptiveCycle` in
   `internal/monitoring/adaptive_resource_freshness_test.go`.
   Periodic out-of-scheduler platform pollers (TrueNAS, VMware) share their
   lifecycle and config-resolution scaffold through
   `internal/monitoring/platform_poller_shared.go`: `startPollerLoop` owns
   the double-start guard, stopped-channel handshake, and sync+poll cadence,
   and `loadActiveInstanceConfigs` owns the "enabled instances keyed by
   trimmed connection ID with defaults applied" active-connection policy. A
   new platform poller of this family must reuse both rather than copying
   the TrueNAS/VMware loop, and traditional PVE guest polling records
    per-guest series through the canonical `recordGuestMetric` helper in
   `internal/monitoring/monitor_pve_guest_helpers.go` instead of inline
   metric writes. Proxmox guest memory history stores both the canonical
   guest-relative `memory` percentage and raw `memoryused` bytes so API
   consumers can apply alternate capacity denominators without reconstructing
   bytes from a mutable guest allocation. Mock mode owes the same pair on
   every chart window. Seeded mock history covers a bounded window, so ranges
   beyond it fall through to the synthetic generator in
   `internal/monitoring/mock_chart_history.go`, and that generator must derive
   `memoryused` from the sampled `memory` percentage and the fixture memory
   capacity rather than omitting the series. Capacity resolves through the
   fixture registries synced in `internal/mock/metric_personas.go`, never a
   per-call fixture graph clone on the chart path. A percentage series without
   its byte companion silently empties the host-capacity memory column instead
   of degrading it. The same obligation is general. Seeded coverage in
   `internal/monitoring/mock_metrics_history.go` and the synthetic generator
   must carry the same series set per resource kind, so a chart window never
   decides whether a series exists. Docker hosts seed disk and network I/O for
   that reason, matching what the agent reports on a real host. A series
   present on one range and absent on another reads as a broken column, not as
   missing history.
   Discovery config and configured-host IP resolution must stay off the
   monitor lock. `internal/monitoring/monitor_discovery_helpers.go` exposes
   the canonical `discoveryConfigSnapshot()` that discovery providers consume,
   and both it and `getConfiguredHostIPs()` may take a brief `m.mu.RLock` only
   to deep-copy config before releasing it; configured Proxmox/PBS/PMG hostname
   resolution runs through the package-local `lookupConfiguredHostIP` seam
   outside the lock, so slow or blocked DNS cannot stall monitor writers or
   discovery subnet probes. The discovery `IPBlocklist` is the deduplicated
   merge of the operator-configured blocklist and the resolved configured-host
   IPs through `mergeDiscoveryIPBlocklist`, never one silently replacing the
   other, and `Start` / `StartDiscoveryService` must read the snapshot through
   that single helper instead of re-inlining the lock-and-clone path.
2. Add metrics capture or history-retention behavior through `internal/monitoring/metrics.go` and `internal/monitoring/metrics_history.go`
3. Add typed read access through `internal/unifiedresources/views.go`
4. Add unified supplemental ingest through `internal/monitoring/poll_providers.go`
5. Add or change container startup ownership/bootstrap behavior for hosted or managed Pulse runtime mounts through `docker-entrypoint.sh`
6. Add or change Docker Swarm manager service, task, node, secret, or config runtime collection through `internal/dockeragent/swarm.go`
   Swarm node inventory is manager-sourced through the documented nodes API
   when available and falls back to the local `system/info` Swarm node
   metadata when a worker or non-manager runtime cannot list cluster nodes.
   Manager-side list failures are warnings, not host-report failures.
   Swarm secret and config inventory is metadata-only: the collector may
   preserve object id, name, labels, driver/template metadata, and timestamps,
   but it must never copy or serialize secret/config payload bytes from the
   Docker API.
7. Add or change Docker or Podman container stats compatibility and runtime metric semantics through `internal/dockeragent/collect.go`
   Docker / Podman collection now owns native runtime inventory as well as
   container metrics. It may collect image summaries, volume summaries,
   network summaries, Swarm services, Swarm tasks, Swarm nodes, Swarm secrets,
   Swarm configs, and daemon storage-usage buckets from the documented runtime
   API, then publish those
   records through the Docker / Podman module report for unified-resource
   ingestion.
   Swarm service records must preserve documented service update status
   (`UpdateStatus.State`, message, and completion time when reported) so the
   container runtime surface can distinguish stable services from active or
   failed rollouts without inventing frontend-only state.
   Failures in image, volume, network, node, secret, config, or storage-usage
   collection are best-effort warnings and must not make the whole host report
   fail when container/runtime health data is otherwise usable. Podman libpod
   pods remain outside this collector until a libpod-native collector owns that
   API shape.
   Docker's native CPU convention reports 100% per CPU core. Agent reports and
   compatibility APIs may keep that raw value, but canonical monitoring history
   and Docker container CPU alerts must pass through the shared normalized
   capacity helper so an 80% threshold means 80% of the reporting host capacity,
   not 0.8 of one core on a multi-core host.
   Container OOM evidence must come from the inspected runtime state. The report
   wire field is nullable for compatibility with older agents, but a current
   collector must set it to the exact Docker inspect boolean even when false;
   report ingest and model conversion must clone and preserve the pointer so
   concurrent state replacement cannot alter previously accepted evidence.
   Container health-check dependency targets must likewise come from the
   inspected runtime configuration, but monitoring accepts only the agent's
   bounded, normalized URL hostname projection. Ingest must preserve an
   independent copy through `models.DockerContainer`; raw health-check command,
   path, query, credential, and environment text is outside the report model.
8. Add or change Proxmox Ceph compatibility payload decoding through `pkg/proxmox/ceph.go`
9. Add or change Proxmox ZFS compatibility payload decoding and vdev-role normalization through `pkg/proxmox/zfs.go`
10. Add or change mock chart synthesis, seeded history continuity, or mock-owned
   chart fallbacks through `internal/monitoring/mock_metrics_history.go` and
   `internal/monitoring/mock_chart_history.go`
11. Honor the per-instance `Disabled` flag on PVE/PBS/PMG at poller client
   init, reconnect, and per-node iteration so disabled connections do not
   drive API calls, scheduler health, or surface ingest. Zero-value
   `Disabled=false` must remain the migration-safe default for existing
   `nodes.json` content; the poller must never create a client or mark an
   instance reachable when `Disabled` is true.
   Source-specific backup snapshot accessors in `internal/monitoring/monitor.go`
   are monitor-state read surfaces, not recovery-store projections. PVE backup
   consumers read `PVEBackupsSnapshot()`, while PBS artifact consumers read
   `PBSBackupsSnapshot()` so PBS size, protection, verification, file, owner,
   datastore, and namespace facts remain the live PBS poller result carried on
   `models.PBSBackup`.
   Proxmox PVE backup and guest snapshot polling in
   `internal/monitoring/monitor_backups.go` must consume the canonical
   `unifiedresources.ReadState` shape for guest, storage, and recovery mapping.
   If a store-backed read-state has not yet been refreshed for the PVE instance
   whose current monitor state already contains guests, backup/snapshot polling
   refreshes the canonical resource store from the current state and continues
   through the read-state interface. Explicit selections use the optional
   `ReferenceReadState` capability on the current `MonitorAdapter` generation,
   preserving registry identity across rename or node movement instead of
   binding a same-name guest from another provider. It must not fall back to direct legacy
   guest slices as the primary source of truth. Clustered PVE snapshot polling
   must therefore see guests collected earlier in the same cycle before calling
   the Proxmox guest snapshot APIs.
   Guest lookups handed from backup/snapshot polling to alert evaluation must
   preserve the canonical instance, node, VMID, type, live display name, and
   live guest tags from read-state. Monitoring must key snapshot lookups with
   the shared alert guest identity and must not downgrade the handoff to a
   name-only map, because ignored-name prefixes, `pulse-no-alerts`, ignored
   tags, and required-tag filtering are alerts-owned policies that require the
   same guest context as ordinary threshold evaluation.
   PBS backup snapshot refreshes in that same file must stay bounded by the
   package worker-pool constant and stream requests through workers instead of
   creating one goroutine per backup group; per-group API failures may reuse
   cached snapshots, but the polling loop must keep memory proportional to the
   worker count rather than datastore cardinality. Per-group cache timestamps
   must be removed when successful group discovery no longer retains that
   group, and recovery-store ingestion must be a bounded latest-batch pipeline
   rather than an untracked goroutine per poll. Latest-batch replacement applies
   only to complete enumerations with the same provider, ID-prefix, and instance
   scope; distinct scopes and event batches must remain independently queued.
12. Add or change agentless availability monitoring only through the
   poll-provider path. `internal/monitoring/availability_poller.go` owns ICMP,
   TCP, and HTTP probes, provider health, scheduler task construction, and
   supplemental unified-resource records for saved availability targets.
   Legacy HTTP and HTTPS targets without an explicit response contract start
   with `HEAD` and retry once with `GET` only when the endpoint explicitly
   reports that `HEAD` is unsupported (`405 Method Not Allowed` or `501 Not
   Implemented`). Explicit application contracts instead execute their bounded
   `HEAD`, `GET`, or `POST` request exactly once, distinguish a received HTTP
   response from an application assertion failure, and inspect no more than
   64 KiB of response content. Other server-error responses remain failed
   probes and must not be converted into healthy application evidence.
   Failed endpoint probes are observed runtime state for that target; they
   must publish provider health and incidents without dead-lettering the
   scheduler task itself.
13. Add or change broadcast resource projection through
   `internal/monitoring/monitor.go` and monitoring guardrails together.
   `/api/state` and websocket broadcasts must coalesce transient split host
   resources before serialization so a single Proxmox node with a reporting
   host agent remains one hybrid top-level system across rebuild ticks.
   High-frequency monitor ticker, mock-mode, and alert-resolution broadcast
   signals are current-state invalidations. They must call the WebSocket hub's
   lazy current-state broadcast path and let the hub resolve tenant-aware
   frontend state after coalescing; monitor callsites must not build or retain
   full frontend-state snapshots for supersedable broadcast signals.
   Mock mode is the narrow exception to the no-subscriber fast path:
   `GetState()` owns lazy fixture alert-snapshot initialization, so the ticker
   must preserve that maintenance call before its subscriber early-exit while
   production monitors continue to skip the snapshot build.
   The mock update loop must advance the 50-node demo through ten node-scoped
   metric cohorts on the shared two-second sampler. It must preserve unchanged
   resource timestamps between cohorts, cover every node within twenty seconds,
   and refresh provider-backed fixtures only once per full rotation so one demo
   tick cannot manufacture an estate-wide WebSocket delta.
   Mock replication timing follows that same refresh instead of staying at its
   startup stamp: each job keeps a stable phase inside its schedule window and
   is rewritten only when that phase passes, so a healthy job always last
   synced less than one interval ago with its next sync in the future, a
   failing job's last good sync falls behind by its failure count, and a
   long-running demo never reports healthy jobs as overdue. A mock job never
   targets its own source node, and an estate without a second node has none.
   The mock estate carries shared storage in the shape the poller produces,
   not the per-node shape the PVE API returns: a shared PBS storage is one
   cluster-scoped row listing its reporting nodes (issue 1049), never one row
   per node with a different usage figure on each.
   Demo Docker container names are unique per host, as Docker itself enforces:
   a container profile reused for a second round on one host takes a numbered
   suffix instead of producing two rows with one name.
   Mock metrics history must also stay bounded independently of estate size:
   eager multi-day PVE guest history is limited to a deterministic sample spread
   across the estate, while every omitted guest continues to receive the same
   canonical timeline through deterministic on-demand chart synthesis. Dashboard
   chart prewarming is limited per workload family and must never rebuild an
   estate-sized guest chart cache on each mock sampler tick. The eager-history
   limit must be applied inside the seed preparation boundary itself, before
   either the active tenant history or reusable seed template allocates series,
   so a caller cannot accidentally cache the complete fixture graph. The
   reusable template exists only for the bounded tenant-startup window and must
   then be released; a single-tenant runtime may not pin a duplicate history for
   the lifetime of the process.
   `POLL_TASK_WORKERS` is a process-wide scheduled-task concurrency ceiling,
   not a per-monitor pool size. Each monitor may own one queue dispatcher, but
   all dispatchers must acquire the shared bounded limiter before executing a
   task so tenant creation and monitor reload cannot multiply the configured
   value into independent worker herds.
14. Add or change Proxmox-side LXC Docker detection or inventory through
   `internal/monitoring/docker_detection.go`,
   `internal/monitoring/monitor_pve_guest_poll.go`,
   `internal/monitoring/monitor_polling_containers.go`, and monitoring
   guardrails together. Socket detection may only annotate LXC guests after
   explicit server opt-in. Both the efficient cluster/resources guest poll and
   the traditional per-node container polling path must run
   `CollectProxmoxGuestDockerInventory` after Docker presence detection and
   before updating container state, so the Docker runtime lens does not depend
   on which Proxmox polling path is active. LXC Docker inventory may only emit
   Docker / Podman
   module-compatible reports into `ApplyDockerReport`, must skip guests with a
   linked online guest-local host agent, and must keep the command set to
   minimal read-only Docker summary and aggregate stats collection. The socket
   probe must run its yes/no marker inside the target LXC through `pct exec`;
   host-side `pct` / `lxc-attach` failures are probe errors and must not be
   converted into cached `HasDocker=false` results. Negative Docker detections
   may be rechecked on a short cadence so command enrollment, daemon startup,
   or transient Proxmox access failures do not hide later inventory. Negative
   detections from before the current Docker checker configuration must be
   rechecked after monitor/router startup, so explicit guest Docker inventory
   can repopulate immediately after backend restarts instead of waiting for the
   normal negative-cache cadence.
15. Add or change mock-mode Discovery context through the canonical mock
    fixture graph. Mock Discovery records must be derived from the same authored
    state graph as mock nodes, guests, Docker hosts, containers, and Kubernetes
    workloads, then exposed through API-owned Discovery handlers. Monitoring
    must not create a second frontend-only fixture path for service versions,
    config paths, bind mounts, ports, or suggested URLs.
    Mock Docker runtime inventory must use the same authored Docker host graph
    for images, volumes, networks, engine storage-usage buckets, Swarm
    services, tasks, nodes, secrets, and configs so platform pages and browser
    proof exercise the live report/resource contracts rather than a
    frontend-only demo inventory.
    Mock Kubernetes clusters must likewise keep distinct display names,
    contexts, and server hints when the fixture graph authors multiple
    clusters, so platform Overview rows read as real cluster identities instead
    of duplicated placeholder labels. Recovery-point fixture assertions should
    verify readable `cluster/namespace/object` identity, not rely on one
    hard-coded cluster name.
16. Add or change TrueNAS supplemental inventory only through the native
    TrueNAS provider path and unified-resource projection. TrueNAS apps are
    API-owned application records: `app.query` is the live inventory source on
    the negotiated current transport, with legacy REST allowed only for a
    connection proven to run a release that predates the versioned API. The
    provider may preserve Docker-compatible runtime metadata for shared
    container tooling, but it must also publish the native app identity, state,
    version, update availability, workload containers, ports, images, volumes,
    networks, and stat collection metadata through `TrueNASData.App` on the
    canonical `app-container` resource.
    Monitoring must not rebuild a second Docker-only TrueNAS app inventory or
    make the Docker fallback the source of truth for the TrueNAS platform page.
    TrueNAS child source identities are appliance-local and must be scoped under
    the owning system source key before unified-resource ingest, so common pool,
    dataset, app, VM, share, and disk names from different appliances remain
    distinct. Mock fixture metrics and seeded/live history must use the same
    scoped source keys as the TrueNAS provider metrics targets.
    The owning system source key itself is connection-scoped: the poller
    constructs live providers through `NewLiveProviderForConnection` so
    `systemSourceID` keys the system (and every child scoped under it) by the
    configured connection ID, never by the snapshot-reported hostname, and the
    system's ingest identity carries no machine key (DMI serials are shared by
    DR clones and can be vendor placeholders). Two appliances that report the
    same hostname must remain distinct resources (#1573, #1575). The hostname
    arm of `systemSourceID` exists only for fixture snapshots, which carry no
    connection; mock-mode identities stay hostname-scoped through that arm.
    Regression coverage: `TestTrueNASPollerKeysSystemsByConnection` in
    `internal/monitoring/truenas_poller_test.go` and
    `TestRegistryIngestRecordsKeepsSameHostnameSystemsDistinct` in
    `internal/truenas/contract_test.go`. The canonical-ID migration semantics
    for rows minted under the retired hostname-keyed derivation live in the
    unified-resources contract (record-declared succession, item 27).
    Every live TrueNAS client owns one explicit, immutable transport decision.
    SCALE 25.04 and later must use JSON-RPC 2.0 over a TLS WebSocket at
    `/api/current`; authentication, authorization, TLS, protocol, or method
    failures on that endpoint are authoritative and must never wake the
    deprecated REST bridge. Only an unsupported-endpoint WebSocket handshake
    may trigger a REST `/system/info` version probe, and REST may then be
    selected only for recognized SCALE releases before 25.04 or TrueNAS
    CORE/FreeNAS. The legacy probe recognizes the `TrueNAS-12.` and
    `TrueNAS-13` prefixes, versions carrying an explicit `CORE` marker, and
    the `FreeNAS-` prefix; this includes CORE 12 maintenance releases such as
    `TrueNAS-12.0-U5` without admitting an unknown numeric family. Unknown or
    current versions fail closed. The decision and persistent socket belong
    to one configured client, so reconnects or legacy negotiation for one
    appliance cannot alter another appliance. Regression coverage for the
    exact CORE 12 and CORE 13 HTTPS redirect/version paths lives in
    `internal/truenas/transport_test.go`.
    On a connection already configured as HTTPS, an `/api/current` handshake
    that redirects to the same appliance's HTTPS web UI is unsupported-endpoint
    evidence, not successful JSON-RPC negotiation. It may open only the bounded
    REST version probe described above: recognized CORE/legacy releases select
    REST, while current SCALE and unknown releases still fail closed. This
    exception does not treat authentication, TLS, protocol, or method failures
    as downgrade signals.
    A plaintext `ws://` handshake answered with a redirect is not an
    unsupported endpoint and must never wake the REST bridge. When the
    redirect target is an https URL on the same host — TrueNAS's HTTP to
    HTTPS redirect, or an equivalent proxy — the dial retries once over TLS
    with the entry's verification settings, and the upgraded `wss://`
    endpoint becomes the client's endpoint for its remaining lifetime
    (#1631). The upgrade is scheme-only and monotonic: cross-host redirects,
    downgrades to plaintext, and relative targets fail closed with the
    redirect target named in the error, so a redirect can move a connection
    to TLS but never to another appliance or back to plaintext. Regression
    coverage: the `Issue1631` handshake-redirect tests in
    `internal/truenas/transport_test.go`.
    Current API-key authentication uses `auth.login_ex` with the key owner's
    username and `API_KEY_PLAIN`; password authentication uses
    `PASSWORD_PLAIN`. Username-less stored API keys may use the deprecated
    login method only as an upgrade bridge, with explicit remediation when a
    release removes that method. Read calls may reconnect with bounded backoff
    and replay once after a transport failure. Mutating app calls must never
    replay after dispatch because their outcome is ambiguous. Event reads must
    retain the `core.subscribe` ID, call `core.unsubscribe` before reusing the
    session, and discard a socket after a terminal stream read deadline.
    Connection summaries expose only secret-free transport mode, endpoint,
    TLS, authentication mechanism, appliance version, reconnect count, and
    last-error timing diagnostics.
    TrueNAS storage and alert inventory follow the negotiated transport:
    pools use `pool.query`, datasets use `pool.dataset.query`, disks use
    `disk.query`, and alerts use `alert.list`. Inventory readers must only consume fields the
    API actually serves on every supported TrueNAS line (CORE 13 REST-only
    included): `pool.dataset.query` carries no `mounted` field, so a listed
    dataset counts as mounted unless `locked` or an explicit value says
    otherwise; `disk.query` carries no health/status field and its
    `extra.pools` join cannot cross the REST bridge, so per-disk pool
    membership and ZFS member state derive from the vdev topology that
    `pool.query` attaches unconditionally; and `disk.temperatures` is a
    parameterized method the legacy REST bridge only serves as POST, while
    current releases use native JSON-RPC reporting without per-method REST
    fallback. Missing disk telemetry is reported as
    unknown, never as a failure signal. Regression coverage:
    `internal/truenas/client_api_shapes_test.go`. Unhealthy pool state
    from `pool.query` must emit a provider-native `zfs_pool_state` incident on
    the canonical pool resource when `alert.list` does not already provide a
    warning or critical pool alert for that same pool, so pool degradation does
    not depend on the TrueNAS appliance's own email or alert-delivery setup.
    Boot-pool inventory follows the separate `boot.get_state` contract because
    `pool.query` is not a reliable boot-topology source across supported CORE
    and SCALE releases. The client must merge that state into the
    connection-local pool list, preserve the boot-pool role, and derive
    path-only leaf devices without matching pool or disk identities across
    configured appliances.
    Read-only dataset health must retain replication intent from
    `replication.query`: `SET` and `REQUIRE` target roots may normalize
    receive-side read-only datasets and descendants only after the poller maps a
    local/PULL task or uniquely matches a remote PUSH target host to one
    configured connection. Missing or ambiguous remote identity fails closed,
    `IGNORE` never normalizes read-only state, and locked, unmounted, pool
    failure, disk failure, or unavailable state remains fault-bearing.
    TrueNAS VMs and network shares follow the same provider-owned inventory
    boundary: `vm.query` data publishes native `TrueNASData.VM` on canonical
    `vm` resources, while SMB/NFS share data from `sharing.smb.query` and
    `sharing.nfs.query` publishes native `TrueNASData.Share` on canonical
    `network-share` resources parented to the owning dataset or pool when the
    API/path supplies that evidence. TrueNAS protection inventory follows the
    same native-query rule: current connections prefer
    `zfs.resource.snapshot.query`, with `pool.snapshot.query` allowed only as a
    same-transport method-name compatibility path; version-gated legacy
    connections use REST. Replication tasks use `replication.query` on current
    connections.
    TrueNAS system services are also native appliance inventory: `service.query`
    is the preferred source for service name, boot enablement, runtime state,
    and process IDs. Pulse must publish that data through `TrueNASData.Services`
    on the owning top-level TrueNAS system resource instead of inventing a
    generic service resource type or rendering services as Docker/container
    rows.
17. Add or change provider supplemental platform activity through the
    provider-owned supplemental-change path and the canonical mock fixture
    graph together. vSphere task/event activity must be authored by the VMware
    provider or VMware mock fixture graph as `activity` resource changes with
    `platform_event` provenance, then recorded by monitoring's supplemental
    resource-change bridge. Monitoring must not create a frontend-only VMware
    activity fixture or bypass the unified resource-change store.
18. Add or change Kubernetes native API inventory through
    `internal/kubernetesagent/agent.go` and
    `internal/monitoring/kubernetes_agents.go`. The Kubernetes agent may read
    Namespaces, Services, ReplicaSets, StatefulSets, DaemonSets, Jobs,
    CronJobs, Ingresses, EndpointSlices, NetworkPolicies, PersistentVolumes,
    PersistentVolumeClaims, StorageClasses, ConfigMaps, Secrets, ServiceAccounts,
    Roles, ClusterRoles, RoleBindings, ClusterRoleBindings, ResourceQuotas,
    LimitRanges, PodDisruptionBudgets, HorizontalPodAutoscalers, and Events as
    bounded best-effort inventory. RBAC inventory (Roles, ClusterRoles,
    RoleBindings, ClusterRoleBindings) reports summary counts only — rule
    counts, subject counts, subject Kinds, and ClusterRole aggregation labels
    — so Pulse stays a "what permissions exist where" surface, not an RBAC
    enumeration tool. Full PolicyRule contents and individual subject names
    (User / Group / ServiceAccount) remain outside the report contract.
    ConfigMap and Secret payload values must not be collected for inventory.
    Current agents must prefer the Kubernetes metadata-only API path for
    ConfigMap and Secret inventory and mark those rows as metadata-only; older
    agent reports may still carry key names, but Secret values remain outside
    the report contract. Mock/demo Kubernetes ConfigMap and Secret inventory
    must mirror the current metadata-only trust boundary rather than seeding
    payload key names. Mock/demo Kubernetes inventory must also seed
    representative Service rows with ClusterIP, external IP, ServicePort,
    targetPort, nodePort, and selector metadata, plus Ingress, EndpointSlice,
    storage-class, persistent-volume, and persistent-volume-claim rows. They
    must also seed StatefulSet, DaemonSet, Job, and CronJob controller rows with
    their API-native target, current, ready/succeeded, availability, exception,
    service-name, schedule, and timing fields so the native services,
    networking, storage, and workload-controller tabs exercise the same
    report/resource contract as live agents. Deployment inventory must preserve
    Kubernetes object metadata creation time and `status.observedGeneration`
    from the agent report through monitoring models so the frontend can show
    API-native age and generation evidence instead of reconstructing those
    fields locally. Monitoring must preserve those objects as native cluster
    inventory instead of flattening them into pods, deployments, or generic
    networking, storage, configuration, or controller rows.
    Agent Fleet Doctor diagnostics must derive from the current monitoring
    `StateSnapshot`, agent-profile assignments, and any persisted legacy
    profile deployment acknowledgements only. Absence of that legacy
    acknowledgement is not failure evidence: current managed-config
    convergence is owned by the desired-versus-applied fingerprint projected
    through `/api/connections`. A persisted acknowledgement may still explain
    explicit failed, pending, or version-drift state.
    `internal/monitoring/agent_fleet_doctor.go` may explain liveness, version
    drift, identity splits, expected telemetry gaps, and evidenced profile
    drift, but it must remain read-only and must not become a separate
    collector, repair executor, or replacement for the canonical
    `/api/connections` fleet projection.
19. Add or change unified-resource alert synchronization through
    `internal/monitoring/monitor_alert_sync.go` and the alerts subsystem
    contract together. Monitoring may pass the current unified-resource snapshot
    into the alert manager, but threshold selection, override identity, active
    alert state, and notification delivery remain alerts-owned. The monitoring
    sync bridge must not introduce per-platform evaluator branches.
20. Add or change PBS API transport, optional node identity collection, or PBS
    HTTP retry classification through `pkg/pbs/client.go`. HTTP status decisions
    must use the concrete client error status rather than rendered error or body
    text. PBS exposes the compatibility `/nodes` node-name endpoint only to a
    direct `root@pam` session: API tokens, including tokens owned by root, and
    non-superuser password sessions must return the typed unavailable-for-auth
    result without issuing a request. Callers must treat that result as absent
    optional identity evidence without repetitive failure logging, not as a
    reason to request broader credentials.
    For an eligible direct-root session, 401, 403, 429, 5xx, decoding,
    cancellation, timeout, and network failures remain transient and retry on
    the next polling call. Concurrent callers must share one in-flight `/nodes`
    request so immediate retry does not create a request storm.
21. Add or change system-alert evaluation through
    `internal/monitoring/system_alerts.go`. Monitoring holds both the
    notification manager and the alert manager, so it is where a condition
    about Pulse itself becomes an alert. The founding case is notification
    delivery: a destination that has stopped delivering cannot announce itself
    through a notification, so the alert list and navigation badge are the only
    escalation path left. The verdict must come from
    `notifications.ClassifyQueueHealth` rather than a local rule, and the
    result must be raised through `alerts.RaiseSystemAlert` so identity and
    idempotence stay owned by the alerts subsystem. Evaluation runs on the poll
    ticker and must stay throttled well below the polling cadence, because
    reading queue health costs a database query. Monitor construction also
    registers the notification queue's committed health-transition callback;
    that path bypasses the timer throttle so terminal failures appear and
    operator retry/dismissal clears the warning immediately. The callback may
    run only after the queue has released its database lock, and monitoring
    must still derive the result from the canonical notification verdict rather
    than trusting the transition that triggered the refresh. New Monitor struct fields
    added for this must fit the existing field alignment column: a longer name
    makes gofmt re-pad the whole block and breaks the canonical guardrail tests
    that pin those declarations verbatim.

## Forbidden Paths

1. New consumer logic built directly on `Monitor.GetState()`
2. New runtime truth living only in `models.StateSnapshot`
3. Snapshot-backed helper paths used where `ReadState` should be authoritative

## Completion Obligations

Discovery belongs to the monitor that constructs it. Both initial startup and
runtime enablement pass that monitor's org into the service constructor; every
scan callback retains it. `internal/discovery/service_event_tenant_test.go`
checks started, found-server, progress, update and completion callbacks for
named and default organizations. The custom event envelope stays unchanged.

`internal/monitoring/monitor_host_agents_test.go` and `pkg/agents/host/proxmox_lxc_test.go` must prove JSON wire-report ingestion through `ApplyHostReport`, partial/complete-empty recovery, contradictory IDs, unchanged node/agent/name/TTL protection, and no invalid-list-to-healthy-empty conversion. Omitted telemetry is never zero. Source proofs are distinct from native PVE and released availability.

1. Update this contract when monitoring truth ownership changes
2. Tighten guardrails when `GetState()`-centric paths are removed
3. Keep network-discovery, host-agent ingest, guest-memory trust, metrics-history, storage-risk, Docker/Podman container collection, Docker report/model payloads, Proxmox Ceph and ZFS compatibility, Docker Swarm collection, mock runtime fixtures, and container bootstrap proof routes explicit in `registry.json`
4. Update related read-state or monitor tests when new collector paths land
5. Keep platform ingestion semantics aligned with
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`: hybrid is a
   declared ingestion mode on an admitted first-class platform, not a license
   to create new platform ids from secondary pollers or optional agent
   augmentation paths.
6. Preserve Proxmox storage backing-pool truth through the canonical storage
   poller path. `pkg/proxmox.Storage`, `internal/monitoring/monitor_polling_storage.go`,
   and the attached ZFS health model must carry the provider-reported `pool`
   field through to runtime storage snapshots and use it before name/path
   heuristics when matching ZFS pool health on multi-storage hosts.
   Inherently shared or remote-backed storage types (NFS, CIFS, PBS, RBD, and
   peers classified by `isInherentlySharedStorageType`) must never be matched
   to a node-local ZFS pool, including by the single-pool fallback: a
   node-local pool backs only local-capable storage types, and attaching it
   more broadly raises one duplicate ZFS device alert per shared storage when
   a device degrades (#1731).
   That same Proxmox compatibility boundary also owns top-level ZFS vdev-role
   normalization. Provider payload buckets such as `special`, `log`, `cache`,
   and `spares` may omit a concrete health state; `pkg/proxmox/zfs.go` must
   treat those blank-state grouping rows as role metadata rather than projecting
   operator-visible `UNKNOWN` failures unless the bucket or one of its children
   carries an actual degraded state or error count.
7. Keep Proxmox-side LXC Docker inventory privacy bounded. The monitoring path
   may collect Docker host/runtime summary, container ID/name/image/state/status,
   ports, and aggregate `docker stats` counters, but it must not run
   `docker inspect` or collect guest environment values, mount sources, files,
   container commands, or process details.
8. Keep TrueNAS app inventory native to the TrueNAS API projection. The
   monitoring/provider boundary may expose Docker-compatible fields for
   cross-runtime tooling, but platform-page app rows, source identity, and
   update posture must be carried by the TrueNAS app facet published into
   unified resources.
9. Keep TrueNAS network-share inventory native to the TrueNAS API projection.
   SMB/NFS shares must enter unified resources as `network-share` records with
   the TrueNAS share facet, not as generic storage rows or Docker/container
   compatibility records.
10. Keep the monitor-adapter rebuild lifecycle persisting canonical identity
    pins. `internal/unifiedresources/monitor_adapter.go` calls
    `PersistIdentityPins` after snapshot rebuilds and supplemental-record
    ingestion so canonical host IDs stay stable across restarts (see the
    unified-resources contract's durable identity-pin obligation). Rebuild
    paths added to the adapter must keep that persistence step; ephemeral
    snapshot-bridge adapters stay read-only. The rebuild, the live
    supplemental refresh and the read-state overlay ingest records with the
    adapter's configured stale thresholds, because record ingest joins
    operator links and the freshness gate of every metric merge, a link's or
    a source's into an existing row, reads them (unified-resources contract,
    "Operator links reach record-ingested resources"). Regression
    coverage:
    `TestMonitorAdapterJoinsLinkedRecordsWithConfiguredStaleThresholds` and
    `TestMonitorAdapterSourceMergesUseConfiguredStaleThresholds` in
    `internal/unifiedresources/monitor_adapter_read_state_test.go` and
    `TestManualLinkToSupplementalGuestHoldsWithAndWithoutContinuity` in
    `internal/monitoring/issue1913_host_continuity_test.go`. The rebuild
    holds the operator-link pass back until the snapshot and every record
    source except availability are ingested, then folds each chain of links
    once over the assembled estate and ingests availability after
    (`deferManualLinks`, `applyDeferredManualLinks`; unified-resources
    contract, "Links apply as chains"): a pass after each source folded
    members as their sources arrived, and a later source's member could not
    outrank one already folded, so the surviving identity depended on which
    source reported it. The live supplemental refresh and the read-state
    overlay still run a pass per call over the rows they find. Regression
    coverage:
    `TestMonitorRebuildJudgesLinkChainsOverTheAssembledEstate` in
    `internal/unifiedresources/monitor_adapter_read_state_test.go`.

11. The TrueNAS provider projects pools with `Storage.Topology` fixed to
    `pool` and the ZFS data vdev layout in `Storage.VDevLayout`. The
    layout summary (`poolVDevLayout`) returns an empty string when the
    native report carries no data vdevs so callers fall back instead of
    inventing a layout. Publishing the layout as the topology drops every
    pool out of the TrueNAS page, and package fixtures carry no vdevs, so
    layout-bearing pools must be exercised explicitly. Regression
    coverage: `TestPoolTopologyStaysDiscriminatorAcrossVDevLayouts` in
    `internal/truenas/provider_pool_health_contract_test.go`.

12. Guest metric history records only guests the current poll actually
    observed. A guest preserved while its node sits in the grace period
    carries the `LastSeen` of the cycle that saw it, so every metric
    recording loop gates on `guestObservedInCycle` before writing a
    sample. Recording a carried-forward guest fabricates a reading: the
    projection has no counters, so it writes CPU, disk and network zeroes
    for a guest Pulse cannot currently see and the history shows a
    collapse to zero rather than a gap. The guard fails open on an absent
    `LastSeen`, because losing a real sample is the worse error and the
    harder one to notice. Regression coverage:
    `TestRecordGuestMetricsSkipsGracePeriodGuestsButKeepsObservedOnes` and
    `TestGuestObservedInCycleFailsOpenWithoutEvidence` in
    `internal/monitoring/monitor_additional_test.go`, and
    `TestGracePeriodGuestContributesNoMemorySample` in
    `internal/monitoring/memory_trust_characterization_test.go`.

13. Multi-guest vzdump job runs must surface per-guest task status. A
    scheduled backup job executes under one UPID whose VMID slot is empty,
    so the task listing alone cannot say which guests it covered and the
    guest-centric coverage surfaces would show status only for guests
    backed up individually. `pollBackupTasks` therefore expands each
    job-run task by fetching its task log (`GetTaskLog` on the PVE client,
    with cluster failover) and parsing the per-guest markers into
    synthetic `BackupTask` entries whose IDs embed the parent UPID. The
    task listing queries `source=all` so running jobs are visible: a guest
    an in-progress job is currently backing up carries a `running`
    synthetic task, which is valid backup-intent evidence for
    `resolveBackupIntentContext`. Finished job logs are immutable and are
    fetched at most once per instance|UPID, with a per-cycle fetch cap so
    a historical backlog cannot stall the shared backup poll budget.
    Regression coverage: `TestParseVzdumpJobLogFinishedJob`,
    `TestPollBackupTasksSynthesizesJobGuestTasks`, and
    `TestPollBackupTasksRunningJobRefetchesAndSuppressesAlerts` in
    `internal/monitoring/monitor_backup_job_tasks_test.go`,
    `TestResolveBackupIntentContextAcceptsSynthesizedJobGuestTask` in
    `internal/monitoring/monitor_alert_intent_test.go`, and
    `TestClusterClient_GetTaskLog` in
    `pkg/proxmox/cluster_client_api_test.go`.

    A guest-scoped `ERROR:` line without the explicit `Backup of VM ...
    failed` verdict must not become clean success on `Finished Backup`.
    Keep that guest running until completion; then publish `warning` with
    a fixed logged-error summary, not the private log text. This does not
    establish a failed archive or guest recovery. An explicit guest failure
    retains precedence; job-wide/quoted errors and clean sibling guests
    remain independent. The next clean job replaces the warning normally.
    `TestVzdumpGuestWarningsDoNotBecomeSuccess`,
    `TestVzdumpGuestWarningScopeAndFailurePrecedence`, and
    `TestPVEGuestLogErrorSurvivesPollingCacheAndReports` in
    `internal/monitoring/pve_backup_guest_warning_test.go` cover parsing,
    real read-only HTTP polling, immutable cache, live inventory and report
    counts/last-success without operating a guest or backup.

14. A TrueNAS app container reported as `EXITED` is a completed workload and
    must raise no incident. TrueNAS classifies container exits before Pulse
    sees them: a normal exit code becomes `EXITED`, an abnormal one becomes
    `CRASHED`, and any `CRASHED` container promotes the app itself to
    `CRASHED`. Every SCALE catalog app ships one-shot init containers from
    ixSystems' base images (`permissions`, `postgres_upgrade`,
    `pgvecto_upgrade`) that exit cleanly and stay exited for the life of the
    app, so treating `EXITED` as failure raises a standing critical per
    installed app that can never clear. `incidentsFromAppState` therefore
    raises `truenas_app_container_failed` only for `CRASHED` containers,
    where the per-container incident exists to name the failing service
    behind the app-level `truenas_app_crashed`. The rendered container state
    is likewise a collapse of every workload using the precedence TrueNAS
    applies (`crashed` > `created` > `starting` > `running` > `exited`), not
    the first entry `app.query` happens to return, so an init container
    sorting first cannot make a running app read as exited. Regression
    coverage: `TestRunningTrueNASAppWithCompletedOneShotContainersRaisesNoIncident`,
    `TestCrashedTrueNASAppContainerStillRaisesIncident` and
    `TestStoppedTrueNASAppContainerStateStaysExited` in
    `internal/truenas/provider_oneshot_containers_test.go`.
15. Keep PBS client HTTP error status structural and the node-name authentication
    boundary explicit. Package proof must cover zero `/nodes` I/O for API-token
    and non-superuser sessions, direct-root 401/403, 429/5xx, and network retry,
    response bodies containing permission-like text, recovery, caching, and
    concurrent single-flight behavior under the race detector. Monitoring proof
    must cover a healthy API-token poll without a `/nodes` request.
16. Keep large-cluster Proxmox polling bounded without weakening inventory or
    reachability truth. `cluster/resources` remains authoritative for the full
    VM/LXC generation when optional detail runs out of budget, optional tail
    collectors use runtime-scoped contexts, and only a failed core provider read
    may make the PVE connection unreachable. Regression coverage lives in
    `internal/monitoring/proxmox_large_cluster_poll_budget_test.go`,
    `internal/monitoring/monitor_shutdown_additional_test.go`, and
    `internal/monitoring/monitor_backups_dir_storage_test.go`.


## Current State

### LXC runtime observations own running-only work

Both the cluster/resources and per-node LXC collectors apply the nonempty
runtime state from their already-fetched complete `status/current` before
selecting memory, optional interfaces, Docker admission, alerts and History.
An accepted newer non-running status cannot be overwritten by an older running
listing or linked-agent memory. Absent or rejected detail leaves the independent
listing fallback intact; no second status request or guest command is added.

Non-running LXC CPU, memory and I/O follow the existing off-state policy, with
memory unavailable from `powered-off`. Observed platform storage capacity and
usage remain available: stopping a container does not empty its filesystems.
The alert engine still suppresses metric alerts and History still admits only
running guest observations. Config/OS/autostart metadata remain available and
an ordinary subsequent running poll selects its own current readings.

`TestLXCStoppedDetailedStatusTransition` in `monitor_pve_guest_lxc_test.go`
crosses both real clients and both collectors through cold/repeated stops,
ordinary running/resume, an explicitly stopped listing, a rejected short-body
stopped reply and another non-running state. It checks work/request counts,
linked-memory rejection, storage preservation, canonical/broadcast state and
in-memory History. These synthetic controls are not native shutdown, thaw,
filesystem-write/liveness or installed recovery acceptance.

### Detailed guest status is a complete observation

QEMU and LXC `status/current` reads require the entire bounded HTTP 200 body to
finish and close successfully before assigning `ObservedAt`. A valid JSON prefix
on a short, stalled or oversized body cannot supply current memory, counters,
metadata, lock or agent evidence. One complete, unambiguous `data` object with a
nonempty canonical `status` is required; duplicate/case-variant consumed fields,
competing errors and missing/null status are rejected without copying response
contents into diagnostics. Sparse stopped replies, unknown future status strings,
unique additive metadata, optional metrics and measured zero/presence semantics
remain compatible. Nested metric decoding and normal read authentication/failover
are unchanged; this does not alter the QGA single-attempt or lock boundaries.

A rejected container snapshot is explicitly prefetched absence for both counters
and enrichment. Enrichment does not issue a second status request in that poll;
direct enrichment without a prefetched snapshot still owns its ordinary read.
Independent listing data remains usable. Rejected VM status cannot renew prior
guest memory or turn it into a new History point; a later complete ordinary reply
recovers, including a real zero-availability observation.

`pkg/proxmox/guest_status_response_test.go` covers both client paths and guest
kinds, complete-body/deadline/close/HTTP boundaries, ambiguous envelopes, sparse
and additive compatibility and resumption. `guest_status_observation_test.go`
crosses the real clients and both QEMU collectors through canonical state,
broadcast projection and in-memory History, and exercises ordinary LXC counter
and metadata recovery without a duplicate read. These are synthetic source
controls, not installed recovery, native thaw, backup safety or a reported cause.


### AI Discovery removed (2026-10-11)

The [AI runtime retirement boundary](ai-runtime.md#ai-discovery-removed-2026-10-11)
owns the removed service-identification scheduler and suggestion helpers.
Network subnet discovery remains monitoring-owned. Availability probes activate
through the canonical saved-target configuration path.

### Interface-associated guest address selection

LXC status, configuration and existing runtime-interface fallback collection,
and QEMU guest-agent network metadata, select display addresses by their named
interface rather than globally sorting flattened IP text. Name-ordered guest
interfaces without common local-container/overlay names lead; remaining scalar
addresses keep unknown association, and secondary interfaces remain useful as
fallbacks. All collected addresses and interface/traffic metadata are retained.
Within an interface valid IPs sort numerically, IPv4 before IPv6; unparsed
metadata retains a deterministic fallback. This is a display hint, not evidence
of physical NICs, default routes or reachability. No new guest query, permission,
poll cadence, cache/backoff or machine-identity behaviour is introduced.

`TestIssue2757ContainerAddressSelection` covers status, stopped/config-only,
DHCP fallback, IPv6, duplicates, numeric sorting, unknown associations and
secondary-only/management-bridge paths through canonical listing and broadcast.
`TestIssue2757VMAddressSelectionAndCache` exercises the actual metadata fetch
and warm cache with reversed input order, preserving interfaces and ownership.
These source fixtures are not native #2757 recovery or released availability.

### TrueNAS physical-disk History — issue #2519

The actual all-metric drawer request also reaches this reader when stored I/O
exists but thermal coverage does not. API single/all-metric supplementation
keeps independent stored metrics, issued-window filtering, existing aggregate
bounds and store-error behaviour; complete local coverage makes no native
call. The connected authenticated route includes ordinary store-backed controls.

The existing disk drawer's `smart_temp` chart consumes the same tenant-scoped
native temperature-history provider as physical-disk sparklines when local
history is shallow. Sufficient local coverage and unrelated SMART/I/O metrics
make no new native call. Native Celsius values retain their original sample
times; downsampling and existing canonical metrics-target resolution remain.
Neither chart pads a missing or single sample into a fabricated flat history.

A successful `reporting.get_data` response with missing disk series permits
one `reporting.netdata_get_data` query for those identifiers alone. Both reads
share the original start/end window, `aggregate:true`, authenticated session
and operation deadline. Failure of the first method does not select another
method, transport or credential. Failure of the second is retained alongside
independently successful series; missing disks do not become observed zero.
Only requested, valid timestamped samples inside the requested window are
admitted, never aggregation summaries, future values or unrequested identities.

Providers map native disk names through the current inventory to canonical
metric IDs, detach returned series and reject ambiguous aliases. In-flight
inventory replacement cannot attach an old disk's series to a new disk at its
former device name. Pollers isolate tenants, reject cross-connection ambiguous
metric IDs and recheck provider ownership after reads so removed/replaced
connections cannot serve stale in-flight output. No host-temperature summary,
CPU substitution, permission widening, store schema or retention change is
introduced. Native appliance and containing-release acceptance remain separate
from synthetic client/provider/authenticated-route controls.

`disk_history_netdata_test.go`, `disk_history_identity_test.go`,
`truenas_disk_history_scope_test.go`, `disk_drawer_native_history_test.go` and
`truenas_disk_history_integration_test.go` cover method distinction, partial
failure, empty/invalid/windowed samples, canonical identity, revocation,
existing reader behaviour and the real authenticated/scoped Pulse route.

V2 partial inventories preserve usable filesystem readings while leaving typed-helper health degraded. Global inventory failure still yields no inventory. No host-network exception, guest mutation, database/retention change or temperature substitution accompanies this repair.

### Mock-mode unified view applies operator links

The mock branch of currentUnifiedStateView serves the fixture graph to the
broadcast, /api/state, GetUnifiedReadStateOrSnapshot and UnifiedResourceSnapshot.
The graph delegates resource assembly to the live
MonitorAdapter.PopulateSnapshotAndSupplemental generation boundary. Snapshot
and supplemental records share source ordering, operator-link application,
availability projection and final freshness policy. Fixtures still generate
synthetic provider inputs rather than exercising provider collection.

The storeless graph receives the links loaded by the monitor's current resource
store generation through resourceStoreManualLinks and
mock.UnifiedResourceSnapshotWithLinks. Its next rebuild loads new links. Mock
broadcast reads normally trigger that refresh after readPathRegistryFreshness.
Storeless generations retain explicit links without writing fixture identity
pins, successions or change history.

The cached data view compares fixture freshness and each link's pair, primary
and creation time through sameManualLinks. The structure view compares the
fixture structure revision and the same link fields. Changing a link or the
oldest link in a chain rebuilds the view without a fixture tick. No-link readers
use the shared memoized view. Linked readers build their monitor's own view when
fixture data or links change. Concurrent cache misses can each build a view.

Publication under mockUnifiedViewMu preserves freshness only for identical
fixture freshness and links. Every different view receives a later value than
all previous publications, so a slow build cannot reuse a newer view's value.
The resources API keys its registry cache on that published freshness. Metrics
targets are captured with resources through the adapter's indexed view lookup.
The fence behind currentModeReadState remains a plain, unlinked projection
until the new epoch's rebuild applies the links.

See unified-resources, "Mock-mode unified view applies operator links", for
chain, availability and fixture-exclusion behavior. Proof remains
TestMockUnifiedViewAppliesOperatorManualLinks and
TestMockUnifiedViewFollowsALinkCreationTimeChange in
internal/monitoring/monitor_host_agents_test.go, the fixture boundary and
freshness tests in internal/mock/platform_fixtures_test.go, and
TestMockUnifiedStateViewUsesCanonicalMockFixtureGraph in
internal/monitoring/canonical_guardrails_test.go.

### Mock-mode metrics-target lookups ride the fixture structure revision

`Monitor.MetricsTargetForResource` is the lookup behind every windowed alert
metric: an alert pass resolves a target once per resource and metric
(`metricWindowPoints`). In mock mode it answers from the `metricsTargets` the
cached mock view captured when it was built (`mockMetricsTarget` in
`internal/monitoring/metrics_target_resolution.go`), and that view serves
lookups for as long as `mock.FixtureStructureRevision` and the operator link
list it was built with are unchanged. A metric tick advances
`mock.FixtureDataVersion` but moves neither the revision nor any input of a
target (`BuildMetricsTarget` reads a resource's type, its source mappings and
the technology, Docker container ID and disk identity of its facets, with the
resource's own `MetricsTarget` as fallback), so a warm cache no longer sends
the next lookup off to build an estate. Mock mode being toggled, the estate
being reconfigured or a link change rebuilds the view on the next lookup; as
before, two lookups that miss together can each build one, and a cold or
invalidated cache builds even for an ID the estate does not list. Once the
view is current an unknown ID resolves to nothing and falls to the raw
resource store. The view stays keyed on the data version for the consumers
that read what a tick moves (broadcast, `/api/state`,
`GetUnifiedReadStateOrSnapshot`), which still rebuild it on a data-version
miss.

Callers that read the estate's identity and topology only share the
structure-keyed view instead (`currentStructureUnifiedStateView`, with
`GetUnifiedStructureReadState` and `currentModeStructureReadState` over it in
`internal/monitoring/unified_structure_view.go`): the Docker-alert prune
(`pruneStaleDockerAlerts`, host IDs), the host-agent evaluation
(`evaluateHostAgents`, which runs on every monitor tick and lists nodes through
`structureNodes`, for their IDs and links, only when an offline host names a
linked node), the chart cache prewarm (metric IDs) and the demo patrol's
operator-state resolver (`patrolResourceOperatorStateProvider`, canonical ID
resolution). Such a caller can be handed a view several ticks old, so it must
not read status, metric values, timestamps, sensors or anything a link merges by
freshness, and a new consumer that does belongs on the data-version view. The
mock storage chart history (`resolveMockStorageTotal`) is one of those: a linked
pool takes its capacity from whichever source was seen last, which moves
without a structure change, so it totals each pool whose chart is not cached
from the data-version view, and `GetStorageMetricsForChartBatch` reads that view
once for all the pools of a request instead of once per pool
(`TestMockStorageChartReadsTheEstateOnlyForAnUncachedChart`). The connection-degraded alert feed
(`buildAlertConnectionSnapshotsWithRuntimeSources`) reads neither the hosts nor
the PBS instances: it keeps the platform rows and drops every agent row, and a
PBS row's reported node name only becomes a host alias that the alert snapshot
does not carry, so it asks the aggregator for platform rows only
(`aggregatorRuntimeSources.platformRowsOnly`).

The revision is read before the data version when a view is cached, and the
fixture advances its data version before its revision
(`advanceFixtureStructure` in `internal/mock/integration.go`). A reader that
sees a new revision therefore also sees the new data version and cannot cache
the previous estate's memoized snapshot under it. The interleaving cannot be
forced from outside, so `TestFixtureStructureRevisionOrderingContract` pins the
order of the writes and of the reads in source.

Keyed on the data version, a lookup after a tick built the view (about 0.5 s
of CPU for the default 1,776-resource estate on an idle core), and a build
slower than the 2-second tick published an already-stale view, so the next
lookup built again. Startup runs the first alert pass before the listener
opens, so on a starved host it never finished: on 2026-10-08 the startup
watchdog's dump on pulse-dev parked the main goroutine in
`evaluateMetricWindow` -> `MetricsTargetForResource` -> `currentUnifiedStateView`
and `/api/health` timed out. The same binary pinned to one core behind four
busy loops missed the view cache on 675 of 929 requests at a 16-second average
build and had not served by 606 s; with this change it served at 148 s (13 s
on the core alone). This was not a regression: builds from 1a094646d7 and
d2ac0c5112 stall identically under equal contention and both start in 14 to
16 seconds at normal load. The remaining time on a starved host was the other
consumers of the shared view, which rebuilt it whenever they found it stale (on
the starved run, 42 view requests in a start of 194 s were all misses: the
Docker-alert prune 18, the connection snapshots 13, the host-agent evaluation 8,
the chart prewarm 2 and the metrics-target lookup 1), and the monitor's repeated full resource-store
passes at startup. The consumers above now ride the structure view or skip the
read, and the passes are one (see "Store and provider wiring publishes the
estate once, and only when it changed"). The loops that still reach the
data-version view once per item are the per-node polling lookups of a monitor
that keeps real polling in mock mode (`PULSE_MOCK_KEEP_REAL_POLLING`):
`linkedHostForNode`, `getHostAgentTemperatureForNode` and
`carriedTemperatureOutlivesAgentLease`. They read a host's status, disks,
sensors and report lease, which a tick moves, so the structure view cannot
answer them; hoisting the read out of the per-node loop would need the read
state threaded through the node poller, and they run in the polling goroutines
after startup, not before the listener opens.

Measured on pulse-dev (single runs on a shared host at load 50 to 90, the
process pinned to one core with GOMAXPROCS=1, probe builds counting estate
builds): before the listener answered, a start of the previous build made 19
store passes and 25 estate view builds; with the wiring change alone it made one
pass and two builds (the first metrics-target lookup and the Docker-alert
prune); with the structure view as well it makes one pass and one build. With no
client connected, a mock-mode process kept building the estate about once every
nine seconds (33 builds in the 299-second life of a run with the wiring change
alone) and used 12.3 seconds of CPU over a 180-second window; with the
structure view it built the estate twice in all, both at startup, and used 3.3
seconds over the same window. The previous build used 9.0 seconds over that
window in a separate run, so read the 12.3 against 3.3 as the comparison, not as
exact percentages.

`TestMockMetricsTargetLookupsDoNotRebuildTheViewAfterFixtureTicks` and
`TestMockMetricsTargetForUnknownIDDoesNotRebuildTheViewAfterFixtureTicks` fail on
the data-version key; `TestMockMetricsTargetFollowsAStructuralFixtureChange`,
`TestMockMetricsTargetFollowsAnOperatorLink`,
`TestMockMetricsTargetRebuildsWhenTheLinkListMoves` and
`TestMockMetricsTargetMatchesTheRegistryResolution` pin that the reused view is
replaced when the structure or the links change and answers as the registry
does. All six are in `internal/monitoring/monitor_host_agents_test.go`, with
their helpers in `internal/monitoring/metrics_target_resolution_test.go`.
`TestMetricTicksNeverChangeTheFixtureStructure` in
`internal/mock/platform_fixtures_test.go` pins the assumption itself: after each
tick of two cohort rotations the revision, the resource set and every
resolved target are unchanged.

### Saved quiet-hours policy before queue activation

`Monitor.New` binds `alerts.Manager.QuietHoursNotificationPolicy` to the
notification manager after saved policy/destinations load and before
`StartQueueProcessing`. The notification owner revalidates due stored work and
owns all partitioning, delivery and cancellation; monitoring neither duplicates
the schedule nor changes destination selection or operational lifecycle.
`TestNewRevalidatesPersistedQuietHoursBeforeDelivery` in
`internal/monitoring/monitor_notification_startup_test.go` reconstructs the
actual monitor twice with due persisted work under a full-day saved schedule,
checking pending state and no local delivery/audit. This is source bootstrap
proof, not installed restart or provider acceptance.

TrueNAS REST alert arguments may be object, scalar, array, null or absent.
Non-object arguments must not abort snapshot collection or discard alerts.
Only typed object fields supply disk identity and SMART counters; neither scalar
arguments nor formatted messages establish those measurements. Alert identity,
severity, dismissal and timestamp remain intact. Malformed JSON and HTTP
failures still fail collection rather than falsely reporting recovery.
`TestRESTAlertArgsSnapshot` covers two snapshot cycles and mixed SMART alerts
through an in-memory HTTP transport with REST already selected; it does not
qualify negotiation or a physical TrueNAS appliance.


### Install snapshots aggregate alert quality across isolated tenant managers

`ReloadableMonitor.AggregateInstallSnapshotCounts` loads each provisioned
organization through its own monitor, reads one privacy-bounded alert-quality
snapshot from that tenant's alert manager, and adds only aggregate integers to
the install snapshot. Organization IDs and alert identities are never copied
into telemetry state. Existing fired, acknowledged, and resolved totals now
come from the same canonical fold as the v14 quality fields, keeping legacy
consumers compatible while preventing two calculations from drifting.

### Proxmox and agent disk observations share full hardware identity

PVE may publish a RAID array volume's full NAA value as a bare serial while
smartctl publishes the same value as an `naa.`-prefixed WWN. Monitoring's
cross-source join compares those serial/WWN carriers after framing-only
normalization, preserving full-length equality so a controller's sibling
volumes cannot collapse through a common truncated udev WWN. Placeholder
identifiers remain non-evidence. `pkg/diskinventory/identity_test.go`
(`TestHardwareIdentityMatch`) and `internal/unifiedresources/registry_test.go`
(`TestIssue1720ArrayVolumeMergesBareSerialWithPrefixedWWN`) are the focused
proofs.

### Proxmox node network inventory is secondary and continuity-safe

Online PVE node polls read `/nodes/{node}/network` through an optional client
capability after the primary status read. Standalone and cluster clients both
support it; clusters retain normal endpoint failover. Interface inventory never
turns a healthy node poll into a failure: transient errors and offline cycles
retain last-known data, while an authoritative empty response clears it. CIDR
is preferred over a duplicate bare IPv4 value, IPv6 is preserved, configured
bridges remain visible, and output order is stable by interface name.
`internal/monitoring/monitor_pve_cluster_refresh_test.go`,
`internal/monitoring/node_memory_sources_test.go`, and
`internal/models/metrics_types_test.go` pin mapping, failure continuity, and
the runtime report shape.

### Poll task concurrency remains bounded across tenants

Without an override, each monitor retains the established client-derived
worker clamp of one through ten. With `POLL_TASK_WORKERS`, monitor queues use
one dispatcher apiece and share a single process limiter capped at 128 active
tasks. A large tenant count therefore adds only one blocked dispatcher per
monitor rather than another full override-sized goroutine pool, while a single
large tenant can still fill the configured I/O concurrency budget.

### Configured fixed poll intervals hold on both scheduler paths

An instance that carries a user-configured cadence (availability targets via
`FixedInstanceInterval`) polls at exactly that cadence whether or not the
adaptive scheduler is enabled. With adaptive polling disabled, planning
passes run on the main poll tick and re-upsert every instance task; they
preserve an already-queued pending slot instead of stamping it due-now, and
tighten it only when a freshly shortened interval justifies an earlier run.
Without this, a sixty-second availability target polls at the ten-second
tick cadence whenever adaptive polling is off, which is the default.

### Base poll cadence is read through the monitor, not its config copy

`Monitor.BasePollInterval` (`internal/monitoring/monitor_runtime_settings.go`)
is the exported reader for the clamped base cadence the monitor polls PVE,
PBS and PMG at. For the built-in poll providers it returns the same values as
the scheduler's `baseIntervalForInstanceType`, including PBS and PMG runtime
polling overrides, but skips the poll-provider lookup, so it never takes
`m.mu`. A replacement provider's own `BaseInterval`, a provider's fixed
instance interval, and a task already queued at its previous interval can
differ from it until the next planning pass. A monitor without config reports
zero where the scheduler falls back to ten seconds.
Non-default tenant monitors poll against a detached config copy (#1619), so a
saved PBS or PMG interval reaches them only as one of those overrides, and
`GetConfig` keeps the old value until that monitor is rebuilt (a restart, or a
full reload such as a PVE interval save). Code outside the monitor that
judges poll freshness, starting with the connections list, must read the cadence
through `BasePollInterval`, not from the config. It reports the same base under
adaptive scheduling, where it is only a floor: the adaptive scheduler plans
each instance's interval independently of the per-platform intervals, and
`PlannedPollInterval` reports that plan. The fixed-cadence branch of
`Monitor.resourcePollIntervals` reads the same accessor. Regression coverage:
`TestMonitorBasePollIntervalReportsSchedulerBaseCadence` in
`internal/monitoring/canonical_guardrails_test.go`.

### Host report admission does not wedge on stale removal blocks

Host-agent report admission consults removal blocks across the durable
continuity store, the legacy in-memory map, and persisted monitor state.
Clearing a block on re-enroll honors whichever store still holds it (the
agent-lifecycle contract carries the matching clause), so admission cannot
wedge into permanent 400s for a host whose block predates the durable
store.

### Large Proxmox generations preserve reachability under bounded enrichment

The efficient PVE poll reserves a fixed tail of the scheduler deadline and
spends at most sixty seconds on per-guest VM/LXC detail. Work begins at a
rotating offset and both guest kinds share the worker pool concurrently. When
the budget closes, builders retain the live `cluster/resources` rows under a
canceled detail context, then publish one complete coherent generation.
Replication runs asynchronously with its own ten-second runtime-scoped budget;
storage and backup polls schedule against the monitor lifecycle rather than the
completed cycle.
The core poll therefore records success after an authoritative inventory even
when those optional collectors time out independently.

### Local libvirt guests are bounded host-agent observations

Authenticated Linux host reports may carry a read-only libvirt inventory for
at most 128 domains. Server ingest discards source-authored IDs, validates and
deduplicates names, derives stable domain IDs, clamps vCPU and memory values,
and computes CPU and cumulative I/O rates only from two accepted samples.
Missing inventory means collection failed and preserves the last successful
sample with its original collection time; a present empty inventory is an
authoritative removal. Domain metrics use
`<host-id>:libvirt:<domain-id>` consistently for history, metrics storage, and
unified-resource lookup. No libvirt lifecycle or configuration operation is
part of the monitoring contract.

### XCP-ng xe inventory is a bounded pool observation

Authenticated host reports may carry one normalized XCP-ng pool inventory
with at most 1024 VMs. Server ingest validates UUIDs and names, clamps vCPU and
memory values, de-duplicates VMs, and preserves the last successful inventory
when a later local `xe` query fails. A present empty VM list is authoritative.
No XAPI action, console, migration, snapshot, or configuration operation is
part of this monitoring contract.

### Custom sensor evidence stays typed through host monitoring

Authenticated host reports now copy bounded `sensors.custom` entries into the
host model and the unified read state without folding their units or values
into temperature maps. Value pointers and collections are cloned at report,
model/frontend, and read-state boundaries; a stale last-good value keeps its
original observation time. Monitoring passes the typed status to the alert
subsystem but does not execute probes or recompute locally configured
thresholds. `TestApplyHostReportPreservesTypedSensorSummary` and
`TestHostSensorsFromReadStateViewPreservesTypedSensorData` pin accepted ingest
and read-state preservation.

### PBS health is one completed-poll outcome

Configured, active PVE/PBS connections whose initial client could not be
constructed remain eligible for startup recovery for the monitor's lifetime.
The existing 5/10/20/40/60-second backoff stays capped at sixty seconds; a
five-minute startup window must not permanently strand a missing client
outside the providers' ordinary polling lists. Cancellation interrupts the
wait, paused instances are excluded, and constructor retries preserve the
configured authority, authentication and TLS trust. This does not add a PMG
retry route or change existing clients' request/renewal policies.
`TestConnectionRetrySurvivesLongStartupOutage` advances the real timers with
Go's virtual clock through a six-minute outage and checks every attempt's
cadence and unchanged connection inputs. The cancellation and pause controls
pin lifecycle/exclusion behaviour. `TestPBSStartupRecoveryKeepsTrustAndFirstPollBoundary`
exercises the real pinned-TLS PBS constructor, ordinary scheduler task and
broadcast producer: a recreated client still has no successful poll evidence,
an accepted first poll restores measured host/datastore values, and a wrong
saved pin remains refused without password-bearing HTTP or pin replacement.

PBS client construction is transport setup, not connectivity evidence. Initial
client creation and retry recreation therefore remain pending until a poll
completes; client-construction failures publish a failed scheduler result
instead of leaving Settings pending while legacy state is disconnected.
`pollPBSInstance` finalizes its dynamic `pollErr` once and uses that outcome for
the scheduler ledger, staleness tracker, poll metrics, connection-health map,
dashboard `PBSInstance`, and legacy PBS alert evaluation. Authentication,
timeout, cancellation, and panic outcomes all publish `offline`/`error`, while
a later success clears the current error and publishes `online`/`healthy`.
Optional node, datastore, namespace, or job collection failures remain partial
data evidence and do not turn a successful version/datastore connectivity
probe into a connection failure. Once connectivity is proven, the poll also
captures the hostname the PBS node reports about itself (`GET /nodes`) on
`models.PBSInstance.NodeName` as machine-identity evidence for connected-system
grouping; node-name fetch failure is partial data like the other optional
collections, never a poll failure. `pkg/pbs/client.go` preserves HTTP status in
its concrete API error. Because PBS restricts the compatibility `/nodes`
endpoint to direct superuser sessions, API-token and non-superuser password
clients return typed unavailable-for-auth evidence without network I/O; Pulse
does not ask operators to broaden credentials for this optional grouping hint.
For an eligible direct `root@pam` session, 401, 403, 429, 5xx, malformed
responses, cancellation, timeout, and network errors retry on the next call
regardless of error-body wording. A successful node name remains cached for the
client lifetime, and concurrent callers join one in-flight request so a
transient response produces one bounded request per polling wave rather than
one request per caller. The `GetNodeName` tests in
`pkg/pbs/client_http_test.go` are the focused authentication-boundary, retry,
recovery, cache, and race proof; monitoring coverage also proves a token poll
never requests `/nodes`.

The captured name only helps connected-system grouping if it survives the trip
back out of the unified read state: `PBSData.NodeName` holds it on the PBS
resource, `PBSInstanceView.NodeName()` exposes it, and
`pbsInstanceFromReadStateView` copies it onto the `models.PBSInstance` that
`Monitor.PBSInstancesSnapshot()` returns and `/api/connections` reads. A
conversion that drops it leaves the PBS connection without its reported alias
and the host agent on that machine as a separate row, with no error anywhere.
`TestMonitorPBSInstancesSnapshotCarriesReportedNodeName` pins the snapshot
carrying the name, and
`TestConnectionsLedger_GroupsHostAgentWithPBSByReportedNodeName` in
`internal/api` pins it from the unified PBS resource to the grouped system.

### Host snapshots carry integration provenance; doctor copy is user-facing

`models.Host.IntegrationSource` mirrors the unified fabric's
`HostView.IntegrationSource()` discriminator on hosts produced by
`hostFromReadStateView` (the source behind `Monitor.HostsSnapshot()`); hosts
built from agent reports leave it empty by construction, so state-side host
records never claim integration provenance. Agent fleet doctor reason
messages are user-facing copy: stale detection now reads "No report has
arrived for 10m 2s. Pulse marks an agent stale after 5m without a report."
with durations humanized by `formatFleetDuration` ("45s", "5m", "10m 2s",
"1h 3m", "2d 4h") instead of Go's `5m0s` form, and the non-online status
reason quotes the reported status without leaking the internal
online/running/healthy vocabulary. The diagnostics subject set is unchanged:
`GetAgentFleetDiagnosticsForTarget` still derives subjects from the state
snapshot (real agents), which now matches what agent-only surfaces show once
integration-backed ledger rows are excluded.

A host produced by `hostFromReadStateView` describes the agent, so its
`Status` and `LastSeen` are the agent's own. A host agent linked to a Proxmox
node merges into one row whose `LastSeen` follows its freshest source and
whose status stays online while PVE polling continues, so the row's values
would present a silent agent as online and current. The projection reads the
agent source's sighting from `HostView.SourceStatus(SourceAgent)`: the host
carries that report time and reads `offline` once it is past the reporting
lease (`hostAgentReportCurrent`, the `hostAgentHealthWindow` that
`evaluateHostAgents` applies to the host in state). Rows without an agent
sighting, such as vSphere and TrueNAS integration hosts, keep the row's
values. `linkedHostForNode` therefore stops handing a silent agent's retained
filesystem summary and ZFS datasets to its node, whose disk falls back to the
Proxmox rootfs or `/nodes` reading before disk alerts and history read it, and
agent connections built from `HostsSnapshot()` (the Connections list and the
update-readiness agent-continuity check) age the agent from its last report
rather than the PVE poll time, applying their own heartbeat cutoff.
`TestSilentLinkedAgentStopsFeedingNodeDisk` drives the disk and projection
path through `evaluateHostAgents` and the registry-backed store refresh.

The metrics-history API's live fallback relies on the same projection: it
treats a host row reading `offline` as past its lease, and an agent linked to
a Proxmox node then answers from the node row. For node rows it takes a
projected `offline` or `unknown` as the lapse and otherwise reads the
registry's sightings (`NodeView.SourceStatus`), because a projected `warning`
is ambiguous: the stale pass shows an unreachable node that way, and so does a
reporting linked agent's storage risk. The projection does not always carry
the poller's verdict: a node merged with a silent linked agent projects
`online` while its Proxmox sighting is current, even after the poller reports
it offline. Docker host rows likewise lose the `offline` that
`evaluateDockerAgents` set, to the stale pass's `warning` or to a merged
agent's status, so that consumer also reads the Docker source's own sighting
from `DockerHostView.SourceStatus(SourceDocker)`.

Unified Agent host reports now make module readiness and updater/config
lifecycle evidence monitoring-owned observed state. Monitoring preserves the
last successful one-shot update transition across subsequent reports, forwards
the applied config fingerprint without config values, and records Host,
Docker/Podman, and Kubernetes module failures so API consumers can distinguish
an active process from an initialized monitoring source. The Kubernetes module
uses the shared agent TLS constructor for custom CA and leaf-fingerprint trust;
its Pulse transport must not regress to a Kubernetes-local insecure-only TLS
configuration.
The same host report carries bounded OS package-update posture: supported
manager, pending count, package/version identifiers, inspection time,
reboot-required state, and an operator-safe inspection error. Monitoring owns
normalization and deep-copying of that observed state, and stamps inventory
freshness from server receipt time so a skewed agent clock cannot keep update
authority fresh indefinitely; it does not infer
updates from kernel strings, refresh package indexes, install packages, or
convert the presence of an update into execution authority.
The host report also carries bounded package-cache cleanup posture: supported
provider, reclaimable byte count, fingerprint, inspection time, and an
operator-safe error. Monitoring stamps freshness from server receipt time and
normalizes that scalar evidence without receiving cache entry names or paths.
It does not infer cleanup eligibility, run cache scans server-side, or turn
reclaimable bytes into mutation authority.
The authenticated Unified Agent report also carries `OperationReceiptVersion`
as monitoring-owned runtime ingest metadata. Absence, zero, unknown, and future
versions are unsupported; monitoring must never infer support from an agent or
product version string. Each accepted report replaces the prior value, so an
agent replacement or compatible-to-legacy downgrade immediately removes the
receipt-protocol prerequisite for actionable update and cleanup capabilities.
The raw protocol integer stays internal to agent transport, monitoring ingest,
canonical capability construction, and dispatch readiness. Customer-facing
resource and frontend contracts expose only the derived capabilities. A
compatible report is necessary but never sufficient mutation authority: the
agent execution server's live, authenticated connection recheck remains
authoritative immediately before durable action admission and dispatch.

HTTP availability probes consume the shared explicitly unverified,
parseable-peer-certificate capture
boundary used by connection discovery, so support for operator self-signed
endpoints does not create independent skip-verification configurations.
Direct mock-node generation also clamps its allocation count to the canonical
fixture bound even when called below the normal configuration-normalization
entry point. The backing slice uses that fixed canonical capacity rather than a
request-derived capacity, while the normalized count continues to determine the
generated fixture length.

The monitoring-owned storage metrics runtime must preserve store-backed storage
chart continuity during resolver warm-up. `syncUnifiedStorageMetrics` must
prefer the resolver's canonical storage metrics target when it exists, but must
fall back to the storage resource id instead of dropping the resource when the
resolver has not yet produced a storage target. The fallback is a metrics
continuity path for canonical storage resources, not a second storage identity
or recovery-source model.

That same reloadable multi-tenant monitor boundary also owns instance-wide
notification settings fan-out. `ForEachMonitor` visits every live tenant
monitor so callers can propagate the webhook security allowlist and public
URL to each org's notification manager, and tenant monitors inherit those
persisted settings at creation through the router's monitor initializer, so
an org created after the settings were saved (or after a restart) observes
the same allowlist as the default org.

This subsystem now sits under the dedicated core monitoring runtime lane so
discovery, metrics-history correctness, and platform-specific runtime coverage
can be governed as first-class product work instead of staying diluted inside
architecture coherence.
Standalone host-agent identity continuity is part of that monitoring runtime
contract. `internal/monitoring/monitor_agents.go` must resolve short/FQDN
hostname aliases through the shared unified-resource equivalence rule when it
binds tokens, matches reports, and removes ignored agents, so reconnects and
reloads keep the same canonical host without weakening token uniqueness across
different machines.
Docker / Podman token binding in `internal/monitoring/monitor_agents.go` follows
the same single-agent product boundary: token uniqueness and conflict messages
are about Docker / Podman module reports from `pulse-agent`, not enrollment of a
separate Docker-specific agent product.
That same monitoring boundary now owns agentless availability targets as a
first-class provider, not as a settings-only helper. Saved availability targets
load from the config persistence boundary, schedule through
`InstanceTypeAvailability`, and publish `SourceAvailability`
`network-endpoint` supplemental records for unified-resource consumers. ICMP is
the default low-overhead check, while TCP and HTTP are canonical fallbacks for
devices or runtimes where ICMP is unavailable or the useful signal is a port or
web interface.
Completed scheduled availability probes also write source-owned categorical
history through the metrics-store writer lifecycle without encoding outcomes
as ordinary numeric metrics. The durable vocabulary is `reachable`,
`unreachable`, and `indeterminate`; uncovered time is derived as `unknown`
from the bounded validity window rather than backfilled from current status,
alerts, or stale-state synthesis. Local checks use the server-authored check
time. Accepted assigned-agent results use server receipt time for coverage,
retain the agent time only as evidence metadata, and carry a stable observation
ID so retries are idempotent. Execution-defining target edits increment a
server-authored configuration revision and history exposes the revision
boundary. Raw and minute/hour/day rollups share the existing entitlement and
retention lifecycle, are deleted with the target, and retain no target address,
agent identity, raw error, certificate detail, or customer identity. The
monitoring-read batch path is bounded to 200 targets and 120 presentation
buckets without a query per target.
HTTP/S application response contracts extend that same source-owned provider
without creating a second uptime product or result store. Current status and
unified-resource availability data carry the transport outcome separately from
the typed application outcome, HTTP status, and bounded failure code; the
overall availability outcome remains the alert/history state so a reachable
endpoint returning an incorrect application response can still fail the saved
service check. Request bodies, response bodies, Basic passwords, bearer tokens,
and operator header values never enter status, categorical history, incidents,
evidence envelopes, telemetry, logs, or Patrol context. Execution-defined
contract edits use the existing server-authored configuration revision so
history visibly breaks at the new proof definition instead of blending unlike
checks.
The explicit availability-target create path is the activation boundary for
saved availability probes.
HTTPS checks also author one canonical certificate observation from the same
probe execution. `internal/availabilityprobe` captures the presented leaf and
`pkg/tlsutil/certificate.go` derives subject, issuer, SANs, SHA-256 fingerprint,
validity bounds, hostname match, chain validity, and the bounded trust status.
Monitoring is enabled by default for HTTPS targets, may be explicitly disabled,
and uses a configurable expiry-warning window that defaults to 30 days. Expired,
not-yet-valid, and otherwise untrusted certificates produce critical provider
incidents. Certificates inside the warning window produce a warning incident.
A verified self-signed leaf is identified as `self-signed` and is exempt from
the untrusted-chain incident, while its identity and expiry remain visible.
These certificate incidents stay owned by the source `network-endpoint` and
enter the normal unified-incident alert pipeline rather than a second notifier.
Supplemental records carry the saved target's optional `LinkedResourceID`
forward into `AvailabilityData` so the unified-resource registry can correlate
and project the probe facet onto the referenced resource. Every saved target
continues to emit its own `network-endpoint` supplemental record regardless of
that correlation outcome; monitoring never substitutes a matched host or
service identity for the configured check. Monitoring does not perform the
correlation decision itself; it only forwards the link hint for the registry
to resolve.
The monitor sync cadence also keeps Docker container alert-override keys on
stable identity. Alongside `MigrateCanonicalOverrideKeys`,
`syncUnifiedResourceAlertsToState` runs
`alerts.MigrateDockerContainerOverrideKeys` over the unified resource
snapshot, re-homing container overrides stored under runtime container IDs or
v6 unified hash ids onto the durable `docker:{host}/{containerName}` key and
pruning ID-shaped orphans left by past container recreates (#1601); a changed
config persists through `SaveAlertConfig` before `UpdateConfig` republishes
it. Regression coverage:
`TestSyncUnifiedResourceAlertsMigratesDockerContainerOverrideKeys` in
`internal/monitoring/monitor_alert_override_migration_test.go`.
Monitoring does own keeping that stored link hint current across canonical-ID
eras. On the same cadence as the alert-override migration,
`migrateAvailabilityLinksToCanonicalIDs`
(`internal/monitoring/availability_link_migration.go`) re-homes a
`LinkedResourceID` that references a retired canonical resource ID (declared
via `SupersededCanonicalIDs` by exactly one live resource) or a node-scoped
Proxmox guest source triple whose instance+VMID matches exactly one live
guest (#1669), rewriting it to the resource's current canonical ID and
persisting through `SaveAvailabilityTargets`. Only provider-declared
persistence keys migrate — never address, hostname, or display-alias
coincidences — ambiguous claims fail closed, and a link that resolves to a
live canonical ID is never touched, so the explicit link stays authoritative
and fail-closed. Regression coverage: `TestMigrateAvailabilityLinkedResources`
in `internal/monitoring/monitor_alert_override_migration_test.go`. Every completed probe also authors an operational-trust
`EvidenceEnvelope` with provider `availability`, collector
`availability-poller`, the saved target as its provider reference, the exact
observation/ingest times, and a validity window of twice the effective polling
interval for local checks. Remote-probe evidence instead uses its canonical
server-receipt freshness window (three effective intervals with the five-minute
minimum), so resource evidence, Connections state, and the probe alert lifecycle
cannot disagree. Before the first completed probe, evidence is explicitly
partial and unknown with reason `availability_not_observed`; monitoring must
never encode that state as a confirmed failure or a healthy observation. The registry owns
binding the source envelope to the check resource and cloning a separately
bound envelope for any matched-resource facet projection after correlation.
Availability target kind is monitoring-owned runtime metadata, not a frontend
guess. Saved targets carry the bounded `targetKind` values `machine`, `service`,
and `device`; monitoring must preserve that value in probe status, supplemental
resource availability data, and availability tags. Missing legacy target kinds
default to `service`, and monitoring must not promote a ping, TCP, or HTTP probe
to a machine solely from address shape, protocol, name, or successful reachability.
Mock-mode availability targets must use that same provider vocabulary. The
mock fixture graph may author ping/TCP/HTTP endpoint examples, but monitoring
and API consumers must receive them through `SourceAvailability` supplemental
records and probe-status projections, not through a mock-only monitoring type.
Frontend monitoring consumers should treat those supplemental records as
day-to-day availability evidence. Settings owns saved target management, while
the frontend-primitives-owned Standalone surface may read the same
`network-endpoint` projection to show current reachability, latency, check age,
and failure state without creating another monitoring provider or top-level
availability route.
Availability execution is location-owned. Each saved target has a normalized,
deduplicated set of observation-location IDs: `pulse:local` for this Pulse
runtime and `agent:<agent-id>` for an eligible connected host agent. The legacy
single `probeAgentId` field remains an input compatibility boundary only; the
location set owns scheduling, configuration revision, assignment, status, and
resource projection. While `external_probe` is entitled, Pulse may schedule
the local path and every selected agent path for one logical target at the same
time. On entitlement lapse the effective set collapses to the local path
without a restart. An agent result is accepted only for its exact selected
location and current target revision; another agent cannot author that path.

Reachability outcome and the optional certificate observation still travel in
the same bounded report entry, and the server clones that observation before
status and resource projection so report buffers cannot alias live state.
Monitoring retains one current status per target and location, using server
receipt time for remote freshness while keeping the agent-authored observation
time as evidence metadata. A stale or disconnected agent path derives as
indeterminate at read time; slow or fast agent clocks cannot manufacture or
conceal a disconnect. A local path beside other locations that has not run
within the same window derives as indeterminate too ("no recent local check").
The target takes its freshness from its newest location, so otherwise a
failing remote report kept an old local pass counted as reachable and the
check read available with current evidence. For the same reason an available
check's evidence lasts only as long as its newest reachable path, not a
fresher path that failed. When the external-probe entitlement lapses, a
single remote result stands in for the local location until the local poller
runs, with the earlier of its agent check time and the server's receipt time
as its check time, so a fast agent clock cannot keep it current. An aggregate
over several locations never stands in: a target without a retained local
observation reads as never checked, and not connected, until the local poller
runs.
Aggregation is conservative: all reachable paths are
healthy, mixed reachable and failed/unknown paths are degraded, all current
paths unreachable are unavailable, and no reachable path with incomplete or
indeterminate coverage is unknown. Only aggregate unavailability advances the
target failure threshold or authors an outage incident. A path-local failure
therefore cannot claim that the service is universally down. Single-location
targets preserve their prior status and history semantics. Multi-location
targets write only their aggregate service conclusion to the logical target's
deletable categorical history; current per-location evidence remains a status
and resource projection until a separately governed location-history schema
can preserve deletion, retention, and bounded-query semantics end to end.
Mock-mode Discovery context follows the same fixture-graph rule. Demo service
details such as detected version, config/data/log paths, Docker bind mounts,
ports, and suggested web URLs may be authored in mock fixtures, but consumers
must receive them through the normal Discovery API contract rather than through
frontend-only demo data or a monitoring-only side channel.
Demo Discovery fixtures must cover the authored estate broadly enough that the
default drawer experience demonstrates meaningful service context instead of a
majority of unknown placeholder records. Non-HTTP services may publish a clear
no-web-interface diagnostic, but fixtures must still identify the service,
version, category, and useful operator paths through the normal Discovery API.
That same monitoring boundary also owns the escalation callback bridge into the
alerts delivery layer. Monitor-owned escalation handling may still publish
canonical escalation state to websocket consumers, but notification fan-out
must defer quiet-hours and resolved-notification suppression policy to the
alerts manager instead of bypassing that shared routing contract when monitor
plumbs escalations outward. Scheduled escalation delivery must use the
notification manager's explicit escalation path, not normal alert re-send
delivery, so delivery cooldown cannot suppress an escalation level that the
alert manager has already deemed due and the escalation channel target remains
the configured email/webhook/all target for that level.
That same monitoring owner now also governs monitored-system grouping readiness
for settings and support boundaries. A non-nil unified read-state is not
sufficient when provider-owned supplemental inventories such as TrueNAS or
VMware are still settling: monitoring must report the grouping view as
unavailable until every active connection in that provider has reached an
initial baseline and the canonical monitor store has rebuilt at or after that
provider watermark, otherwise previews and support ledgers can freeze against
a transient startup undercount.
That same monitoring boundary also owns the machine-readable unavailable-state
contract for monitored-system usage. `internal/monitoring/monitored_system_usage.go`
must emit canonical reason codes such as
`monitor_state_unavailable`, `supplemental_inventory_unsettled`, and
`supplemental_inventory_rebuild_pending` when usage cannot yet be resolved, so
settings and support surfaces can show verification or recovery state without
inventing their own readiness heuristics or falling back to a fake count.
That same continuity rule applies to canonical unified resource snapshots.
`internal/monitoring/monitor.go` must overlay recent standalone host-agent
continuity records onto `UnifiedResourceSnapshot()` and
`GetUnifiedReadStateOrSnapshot()` results, so first-login and post-restart
Infrastructure views retain the durable agent-backed systems Pulse already
knows about while live reports and supplemental providers catch up. A host
projected only from durable continuity has no current report in that monitor
generation and must therefore render offline while preserving its last-known
identity and removal action; persisted telemetry must never manufacture an
online sighting after restart.
That same monitoring owner also governs collector payload compatibility at the
shared boundary. Podman container stats must honor Podman's compat payload when
it exposes a direct CPU percentage and otherwise fall back to Podman's
wall-clock delta semantics rather than Docker's multi-core normalization, and
Proxmox Ceph status decoding must accept monitor totals from either
`monmap.num_mons` or the concrete monitor arrays and manager standby entries as
either bare names or structured objects so collector payload variations do not
break the canonical monitoring path. That same compatibility boundary also owns
legacy Unraid raw-status normalization at host-agent ingest: when older agents send
`rawStatus` without the newer normalized `status`, `internal/monitoring/monitor_agents.go`
must derive the canonical disk status before storage-risk assessment runs so
v5 aggregate counters do not override clearly healthy per-disk state during v6
compatibility operation.
Unraid long-running operation state uses the same authoritative transition
boundary. A non-empty active sync action may carry progress; cancellation,
completion, or idle state is represented by an empty action and normalized
zero progress even when an older collector or buffered payload retains a
terminal percentage. Accepted terminal reports must immediately remove
`unraid_sync_active` from host state, canonical storage resources, alerts, and
UI-facing risk projections. Older reports must never restore it after that
transition, while a mere loss of reports waits for the reporting lease and then
clears only the transient operation evidence alongside an explicit connectivity
signal.
The same monitoring compatibility boundary owns Unraid slot filtering and
operator health posture after host-agent ingest. Empty no-present Unraid slots
must be removed before storage-risk assessment so unassigned array capacity is
not reported as missing or disabled media. Unraid topology labels such as
`disk6` and `parity2` are not assignment evidence by themselves: a
`DISK_NP`/`DISK_NP_DSBL` member remains reportable as genuinely missing only
when device, model/serial identity, filesystem, or size evidence shows that a
disk was assigned. Unraid's transport-only `ata-_` identity and the `ata -`
model artifact emitted by older Pulse parsers are also placeholders, not disk
identity. The distinct provider status `DISK_NP_MISSING` is authoritative
assigned-member evidence and must never be removed merely because those
identity fields are empty or placeholders. After filtering, structured member statuses override stale
aggregate missing/disabled/invalid counters, and structured parity status
overrides the aggregate protected-parity count. This normalization belongs at
server ingest as well as agent collection so deployed older agents stop
creating false health alerts without waiting for an agent upgrade. The focused
compatibility proof lives in `internal/monitoring/monitor_host_agents_test.go`
and `internal/unraid/status_test.go`. An Unraid
array with assigned data disks but no configured parity is an attention/warning
posture with the machine-readable `unraid_no_parity` reason, while active parity
check/sync state remains a separate `unraid_sync_active` reason. Realtime resource
broadcasts must preserve canonical identity, discovery target, metrics target,
incident rollups, and raw `agent`/`storage` facet payloads so frontend
infrastructure surfaces can explain degraded/warning rows without falling back
to generic status labels. Storage platform-data payloads built by
`monitorStoragePlatformData` must carry the full `zfsPool` report (scan
activity and per-device states/errors/messages) alongside the flattened
`zfsPoolState`/error scalars whenever canonical `StorageMeta.ZFSPool` is
present, so `/api/state` and websocket consumers can render device-level ZFS
health in parity with the unified-resources read path. That realtime broadcast contract also owns source and
platform identity for storage resources: `internal/monitoring/monitor.go` must
carry the canonical `Resource.Sources` array onto `ResourceFrontend` and
`platformData.sources`, and must derive storage `platformType` from the owning
source/facet instead of treating the `storage` resource type as Proxmox by
default. Appliance presentation details such as Unraid array identity may remain
inside storage metadata, but agent-backed storage must stay canonical
`platformType=agent`. That same broadcast resource contract owns resolved
metrics targets. Monitoring must enrich broadcast/state `ResourceFrontend`
payloads from the active metrics-target read-state before serialization so
`/api/state` and websocket consumers use the same canonical metrics history IDs
as `/api/resources`; storage resources must not fall back to generated
`Resource.ID` values when the unified resource registry can resolve a
source-owned `storage` target.
That same broadcast projection owns aggregate resource disk I/O. When the
canonical unified-resource metrics include `diskRead` or `diskWrite`,
`internal/monitoring/monitor.go` must project those rates into
`ResourceFrontend.diskIO` through the shared resource converter, so
`/api/state` and websocket consumers read disk throughput from the same
freshness-gated resource metrics contract as CPU, memory, disk, and network.
That broadcast projection carries no per-resource alert copy:
`ResourceFrontend` has no `alerts` field and `ResourceConvertInput` no alert
input, because open alerts reach clients once, through the state-level
`activeAlerts` map, and nothing ever populated the embedded list.
That same broadcast projection owns client-payload static-metadata slimming
(governed gap `resource-payload-static-metadata`): identical resource
capability blobs are deduped into the state-level `capabilityCatalog` under
content-addressed ids and referenced per resource via `capabilitiesRef`
instead of being inlined on every row; resources whose derived policy posture
is the default (internal sensitivity, cloud-summary routing, no redactions)
omit `policy` and `aiSafeSummary` from the broadcast, which ingestion
synthesizes back, while non-default postures keep both inline; and broadcast
`canonicalIdentity.aliases` entries that duplicate `supersededIds` are
dropped, with the superseded ids still shipped for identity resolution.
Slimming edits only the per-broadcast copy refreshed by
`RefreshCanonicalMetadata`, never stored monitor state.
Unraid ingest must preserve the agent's native disk topology fields through the
monitoring model and read-state projection. `internal/monitoring/monitor_agents.go`
and `internal/monitoring/monitor.go` must carry model, transport, filesystem,
native capacity, used/free bytes, temperature, spin state, and read/write/error
counters without requiring a parallel SMART row. Monitoring may normalize legacy
statuses and filter empty slots, but it must not collapse assigned Unraid
array/cache members back to generic host disks or discard native fields before
unified resources builds storage and physical-disk resources.
Host-agent memory ingest carries the reclaimable page-cache split. The host
agent reports `cacheBytes` (gopsutil Available minus Free, with the ZFS ARC
adjustment recomputing free so used + cache + free still covers the total),
and `internal/monitoring/monitor_agents.go` maps it into
`models.Memory.Cache`, clamping inconsistent or older-agent reports so
used + cache never exceeds total. Mock fixtures author the same split for
generic hosts and node-linked host agents, and any mock drift updater must
hold the used + cache + free invariant as sampled usage changes.
VMware vSphere now also has a locked phase-1 ingestion boundary under this
lane. The admitted direction is vCenter-only in phase 1, and monitoring must
stay API-first through the
official vCenter Automation API plus the Virtual Infrastructure JSON API.
Direct ESXi remains out of phase 1 because the standalone host-agent hierarchy
is materially narrower than the vCenter inventory and the declared support
floor depends on vCenter-backed topology, shared datastore scope, alarm state,
and historical performance access. Any later direct-ESXi work must be admitted
explicitly instead of inheriting vCenter support by implication.
That same VMware monitoring boundary now also includes the canonical telemetry
rule. ESXi host metrics and history belong on the shared `agent` path, VM
metrics and history belong on the shared `vm` path, and datastore
capacity/accessibility history belongs on the shared `storage` path. VMware
network inventory belongs on the shared `network` resource path, but phase 1
does not claim VMware network metrics or history. VMware phase-1 work must not
create `vmware-host`, `vmware-vm`, `vmware-datastore`, or `vmware-network`
history stores just because the collection APIs differ from other platforms.
That same VMware monitoring boundary now also includes the source and identity
rule. Runtime collection may authenticate to `vCenter`, call multiple VMware
API families, and gather several object classes, but the emitted state must
still collapse onto one canonical VMware source classification and one
provider-scoped identity model for hosts, VMs, and datastores. Monitoring must
not leak `vcenter` versus `esxi` transport distinctions into downstream
resource identity or source filtering.
That same VMware monitoring boundary now also includes provider ownership. One
saved VMware connection should map to one provider owner and one canonical poll
health record, even if that provider keeps separate authenticated Automation
API and VI JSON clients internally. Connection edits that change host, auth,
TLS, or poll cadence must replace that live provider state instead of leaving
stale VMware sessions resident until restart.
That same provider-owned summary must also serve the shared settings runtime
surface. `internal/monitoring/vmware_poller.go` owns the per-connection poll
summary (`poll` plus `observed`), `POST /api/vmware/connections/{id}/test`
with no edit overlay must refresh that same summary owner, and
`/api/vmware/connections` list reads must consume the poller summary instead of
recomputing or shadowing it inside handler-local runtime state. Internal
sub-second test harness intervals must not leak `intervalSeconds: 0` onto that
operator-facing contract.
Manual saved-connection tests that prove the required VMware API floor but
encounter non-fatal optional signal or performance failures must record a
successful poll attempt together with `observed.degraded`, `issueCount`, and
the summarized upstream diagnostics. They must not erase the same
partial-success evidence that a live inventory poll would publish, while
authentication, TLS, network, and required API-family failures must continue
to increment canonical poll failure state.
That same monitoring boundary now also owns runtime mock rebind continuity for
API-backed supplemental providers. When `/api/system/mock-mode` flips on a
running server, the live TrueNAS and VMware provider bindings must swap to the
mock-backed supplemental records and refresh canonical read-state immediately
instead of waiting for a process restart before shared resource consumers can
see the platform inventory.
That same runtime boundary also owns authorization order for demo toggles.
`internal/monitoring/monitor.go` must not clear alerts, reset runtime state,
or restart discovery until the canonical mock runtime has accepted the
requested mode change; rejected release-demo fixture enables must fail before
any monitoring reset so the live preview does not blank itself on an
unauthorized toggle.
That same monitoring boundary now also owns atomic unified-metric persistence.
When unified resource sync projects agent, VM, app-container, or storage
metrics into persisted history, it must append in-memory history first and
flush the backing store through one `metrics.WriteBatchSync` batch per sync
sweep instead of per-metric async writes, so canonical chart history cannot
race itself into partial persisted windows.

### Unified storage rebuilds preserve source observation time

Canonical resource-store rebuilds are projections, not fresh storage polls.
When `internal/monitoring/monitor.go` syncs a non-native storage metric into
in-memory and persisted history, every metric in that projected observation
must use the selected metric source's `SourceStatus.LastSeen`. If that source
timestamp is unavailable, the resource `LastSeen` is the continuity fallback;
wall-clock sync time is used only when neither observation timestamp exists.
Repeated registry rebuilds of one unchanged PBS datastore observation must
therefore replace the same timestamped history point rather than create a new
raw point for each unrelated provider completion. This does not change the
native Proxmox storage writer exclusion or canonical storage target selection.

That same chart boundary now also owns long-range in-memory coverage
selection. `internal/monitoring/metrics_history.go` must expose guest and node
coverage spans for the requested metric families, and
`internal/monitoring/monitor_metrics.go` must prefer the in-memory history
when that span already covers the requested chart window before falling back
to SQLite, so long-range chart batches do not pay an unnecessary store round
trip just because the request is larger than the old fixed in-memory
threshold.
Agent and node CPU temperature are part of that shared metrics-history family.
`internal/monitoring/monitor_agents.go` must write the primary host-agent CPU
temperature into both in-memory guest history and the persisted `agent`
metrics-store stream, while `internal/monitoring/metrics_history.go` keeps
`temperature` alongside CPU, memory, disk, and I/O for guest and node history
reads. The PVE node polling path in
`internal/monitoring/monitor_pve_storage.go` must likewise append the selected
node CPU temperature to in-memory and persisted `node` history whenever a
positive collected temperature is available. Mock history must seed the same
metric so Proxmox node drawer thermals exercise the production contract instead
of relying on a frontend-only fallback.
Host-agent thermal pressure is not part of that Celsius history stream.
`internal/monitoring/monitor_agents.go`, `internal/monitoring/monitor.go`, and
the shared model conversion helpers must preserve `sensors.thermalState`
through ingest, read-state projection, and frontend conversion, while leaving
`agent.temperature` and `metric=temperature` unset unless a real Celsius value
exists.
Windows Storage-module reliability temperatures use the existing host-agent
physical-disk route rather than a provider-specific monitoring payload.
Authenticated ingest must preserve each validated `sensors.smart` device,
model, transport, capacity, temperature, and field-level
`windows-storage-reliability` provenance through the canonical host resource,
disk-temperature presentation, history, and alert boundaries. An unknown
health value remains unknown; monitoring must not convert the presence of a
temperature counter into SMART health evidence.
Host-agent GPU sensor summaries follow that same descriptive-host-telemetry
path. Monitoring must preserve typed GPU id, name, temperature, utilization,
and VRAM readings from agent reports through models, read-state projection, and
frontend conversion. When typed readings are present, authenticated host-report
ingest records bounded maximum-per-host `gpu`, `gpu_memory`, and
`gpu_temperature` samples in both in-memory guest history and the persisted
`agent` metrics-store stream. Utilization and VRAM pressure are percentages;
VRAM pressure is derived only from non-negative used bytes and a positive total,
and temperatures must be positive Celsius values no greater than 150. Missing
or invalid fields omit only their own series. The aggregate remains attached to
the existing agent resource and does not promote GPU workload or process
inventory into monitoring state.
Host-agent power sensor summaries follow the same descriptive-host-telemetry
path. Monitoring must preserve `sensors.powerWatts` readings from agent reports
through models, read-state projection, and frontend conversion without
promoting wattage into temperature history, resource lifecycle, storage health,
or alert metrics unless a separate governed contract adds that metric.
Host-agent custom metrics may originate from a locally configured executable or
an HTTP(S) REST endpoint, but remain agent-authored typed host metadata.
Monitoring must preserve group, subgroup, numeric/boolean/timestamp kind,
numeric threshold value, observation time, optional event time, status, bounded
error, stale state, and error-alert preference through ingest and frontend
projection. Boolean values use 1/0 and timestamp values use age in seconds so
the existing threshold and canonical alert lifecycle remains authoritative.
REST collection ownership stays in the agent: server configuration cannot
inject destinations, redirects are not followed, response bodies are bounded,
and an endpoint-supplied RFC3339 `observedAt` must trip the configured
`staleAfter` policy when old.
That same monitoring owner also owns canonical unified-resource publication on
`/api/state` and the websocket `state.resources` hydrate path. Monitoring must
publish those resources from the same canonical unified snapshot that
`/api/resources` seeds in mock and live mode, rather than projecting a second
raw store-only inventory for broadcast. Otherwise cold hydrate and later
registry-backed refreshes can swap the operator-visible infrastructure set
under one running session.
The same state-publication owner also carries Proxmox tag presentation. PVE
polling must fetch datacenter `tag-style` through `/cluster/options`, parse the
color map and `case-sensitive` flag per configured Proxmox instance, and merge
that into `models.State.PVETagStyles` before websocket/API publication. Clearing
a Proxmox color map must replace that instance's stored style with an empty
style and rebuild the legacy aggregate `PVETagColors`; stale colors from a
previous poll must not survive as if they still came from Proxmox.
That websocket publication boundary must also treat an absent hub as an absent
broadcast channel in both direct nil and typed-nil forms. Tenant-scoped
background monitors can start in headless test or maintenance runtimes before a
hub is wired, and state publication must no-op safely instead of dereferencing
a nil `*websocket.Hub` during ticker refresh.
That same headless runtime boundary applies to agent ingest itself. Once
`ApplyHostReport`, `ApplyDockerReport`, or `ApplyKubernetesReport` accepts and
finishes applying a report, monitoring must refresh the canonical monitor
adapter before returning so typed `ReadState`, `/api/resources`, and Patrol see
the accepted host, workload, and cluster truth even when no websocket clients
are connected. Websocket client presence and broadcast hydrate are delivery
concerns; they must not gate canonical agent-report publication.
That same Docker/Podman monitoring boundary now also owns Docker
authorization-plugin posture. `internal/dockeragent/collect.go` must project
`system.Info().Plugins.Authorization` into the canonical agent report,
`internal/monitoring/monitor_agents.go` must preserve that posture on the
shared Docker host model, and `internal/monitoring/docker_commands.go` must
refuse Docker daemon-mutating commands when authorization plugins are
configured until the upstream Moby authz-plugin advisory line has a fixed Go
module release.
Unified-resource Docker / Podman lifecycle capabilities consume that preserved
posture and must fail closed when it blocks mutation; monitoring remains the
runtime truth producer and must not grow a monitor-local start/stop/restart
transport around the governed action executor.
That same collector boundary also owns the maintained engine-client seam.
`internal/dockeragent/docker_client.go`, `internal/dockeragent/collect.go`,
and `internal/dockeragent/swarm.go` must keep Pulse's package-local
`dockerClient` interface as the compatibility layer while the underlying
implementation routes through maintained `github.com/moby/moby/api` and
`github.com/moby/moby/client` modules, so monitoring runtime collection does
not drift back onto the legacy `github.com/docker/docker` Go module line.
The Unified Agent may share that maintained, already-connected client through
a narrow typed lifecycle bridge, but collection remains read-only unless the
canonical governed command channel invokes the bridge. The bridge may inspect
one exact container and issue one allowlisted start, stop, or restart; it is not
a general monitoring mutation API, does not expose the daemon client to model
tools, and must not retry an ambiguous mutating request. Module absence or a
runtime mismatch fails closed while ordinary inventory collection continues
under its existing local configuration and privacy controls.
That same monitoring owner now also governs restart-safe standalone host
continuity for monitored-system grouping. `internal/monitoring/monitor_agents.go`
must persist recent host identity at report time, and
`internal/monitoring/monitored_system_usage.go` must project that continuity
back into the canonical read state through the unified-resources-owned overlay
path instead of rebuilding registry truth locally. A server restart or v6
upgrade must not briefly forget an already admitted standalone host and
misclassify its next report as a brand-new counted system.
Re-enrollment can leave older saved identities for a currently reporting
machine. Continuity hydration must use `ReadStateWithHostContinuity`, which
adds only canonically absent resources. An older enrollment must not replace
the live host ID, erase metrics, or turn its linked Proxmox node offline in
`HostsSnapshot`, `NodesSnapshot`, or the unified snapshot. This read does not
delete continuity records or change token binding. The regression in
`internal/monitoring/issue1913_host_continuity_test.go` exercises repeated reads
with a live successor, an older enrollment, and a genuinely absent host.

That same standalone-host continuity boundary also owns host snapshot and
connection-list continuity during monitor reloads. `internal/monitoring/monitor.go`
must apply the same continuity overlay when `HostsSnapshot()` resolves its
canonical read state, so settings and other host-list consumers do not blank
previously admitted Pulse Agent rows during a config-driven monitor swap while
fresh reports are still in flight.
Continuity lookups must stay cheap because every canonical read-state lookup
makes one. `readStateWithStandaloneHostContinuity` takes the live host list
once per call, and `GetLiveHostsSnapshot` copies only hosts through
`State.GetHosts` rather than deep-copying every guest through `GetSnapshot`;
agent reports and config fetches share that accessor. With one offline
standalone agent, a lookup at 2,080 synthetic resources moved from 136 ms to
74 microseconds. `TestStandaloneHostContinuityReadStateReusedAcrossLookups` and
`TestGetLiveHostsSnapshotCopiesOnlyHosts` pin both.
That same mock-runtime boundary also owns freshness while demos are running.
The mock update loop must keep provider-backed TrueNAS and VMware records plus
legacy PBS and PMG summaries on current `LastSeen` and health state each tick,
so long-lived infrastructure, workloads, storage, and recovery demos do not
decay into synthetic stale-state warnings while mock mode remains enabled.
That same Proxmox container monitoring boundary now also owns runtime counter
authority when the lower-fidelity container list or cluster-resources payload
and the current-status payload differ. `internal/monitoring/monitor_pve.go`,
`internal/monitoring/monitor_pve_guest_lxc.go`, and
`internal/monitoring/monitor_polling_containers.go` must merge the current
`GetContainerStatus` counters through one canonical `mergeContainerRuntimeCounters`
path before LXC rate calculation. A present status field is newer authority
even when it is zero or lower after a restart; an absent/null status field
retains the listing field and its presence state. The merge must retain the
status response receipt time and reuse the same prefetched status snapshot for
metadata enrichment instead of paying disconnected metric and metadata status
reads that can diverge.
That same Proxmox backup/snapshot boundary owns bounded concurrent guest
snapshot enumeration. `internal/monitoring/monitor_backups.go` must query VM
and LXC snapshot endpoints through one capped worker pool and preserve
previously-known snapshots only for guests that were not successfully polled in
the current cycle, so large or slow clusters do not starve later guests while
transient misses still avoid destructive state churn.
That same guest-monitoring boundary also owns linked host-agent precedence for
Proxmox VMs. When a VM has a live linked Pulse host agent, the canonical disk
inventory and aggregate disk summary must prefer that linked host-agent disk
collection over the narrower QEMU guest-agent filesystem list, so VM overview
surfaces keep the richer inside-guest storage truth instead of silently
regressing to mount-only visibility.
That same mock-runtime boundary also owns update cadence. Demo and preview
environments may slow the configured tick interval to reduce visual churn, but
that cadence must flow through the shared mock update loop and smoothing model
rather than through page-local polling suppression or demo-only frontend
special cases.
That same demo-owned mock boundary also owns chart continuity. Seeded mock
history and runtime mock sampling must be projections of the same canonical
metric timeline, so changing chart ranges feels like zooming one history
window instead of stitching a second live tail onto the end of seeded
sparklines. Monitoring must not let any mock-owned resource receive a
duplicate generic unified-resource writer that appends a divergent recent tail
after the canonical mock sampler has already seeded and extended that series.
That same sampler-owned boundary also owns seeding cost. Historical mock
seeding runs synchronously on monitor startup and in package proofs, so it
must stay deterministic and bounded by fixture size instead of carrying
per-resource pacing sleeps or other wall-clock throttles that can exhaust the
package-level `go test -race -timeout 10m` budget on hosted runners before
canonical mock history has even finished initializing.
The seed path must therefore include the canonical terminal `now` sample on
its tiered timeline and anchor seeded series to the canonical metric model at
that timestamp instead of to mutable state fields, so historical charts match
the exact runtime history that would have been recorded live.
Historical backfills must resolve resource identity, metric bounds, stable
seed, speed, and role once per series, then append that ordered series to
`MetricsHistory` under one series-level mutation. They must not repeat
normalization, hashing, role lookup, locking, retention scans, and capacity
checks independently for every point. Test harnesses must also bound mock seed
duration to the deepest history window they actually prove; Core E2E owns a
seven-day chart contract and must not make every parallel shard build the
production-preview 90-day timeline.
That means seeded history must sample the shared canonical mock runtime metric
function at every historical timestamp for every mock-owned resource class.
Monitoring must not approximate the past from snapshot/current values and then
switch to the canonical sampler only for recent live ticks, because that still
creates a visible seam even when the identities and timestamps are correct.
Seeded history and subsequent live mock writes must also record on the same
canonical chart-time grid. Monitoring must not seed on one wall-clock phase
and append live ticks on another `time.Now()` phase, because the canonical
sampler is dynamic enough that off-grid recent points still look like a
different tail.
Runtime mock tick writers must also sample the canonical metric model at the
recorded chart timestamp instead of copying mutable state fields directly,
because graph refresh cadence and state rounding can otherwise append a recent
tail that looks like a different generator even when the underlying mock
resource identity has not changed. Provider-backed fixture refresh paths must
derive their live host, workload, storage, and disk-history writes from that
same canonical sampler instead of replaying snapshot values. Native polling
lanes and unified sync must not append duplicate mock history once the
canonical mock sampler owns that resource class.
That same ownership rule applies by default whenever mock mode is enabled.
Real client initialization, native pollers, and async agent-origin metric
writers are support-only opt-ins, not the normal demo path, and they must not
append chart-history or persistent metric-store points onto mock-owned
timelines while the canonical mock sampler is active.
That same chart boundary also owns role-shaped realism. Seeded history,
synthetic summary fallbacks, and runtime mock writes must derive their bounds
and curve shape from the same canonical resource-role registry, so database,
cache, backup, web, and storage workloads keep believable long-range behavior
instead of switching from one generic seeded pattern to a different recent
runtime pattern.
Bounded plateau generation must preserve an exact constant when the lower and
upper metric bounds are equal; interpolation and noise must not introduce
floating-point drift into a synthetic flat series.
That same mock chart boundary also owns request-path efficiency. Demo chart
reads must reuse monitor-owned downsampled mock history for the current mock
sampler generation instead of regenerating or re-downsampling the same seeded
timeline on every endpoint hit. When seeded mock history is rebuilt or a live
mock tick advances, monitoring must invalidate that cache so preview charts
stay current without paying repeated per-request synthesis cost.
The mock sampler path must prewarm the default Workloads guest-chart
cache through `GetGuestMetricsForChartBatch`, using canonical `ReadState`
workload identities for VMs, system containers, Kubernetes pods, and app
containers so `/api/charts/workloads` does not rebuild every guest sparkline
on the first post-tick request. The retired aggregate
`/api/charts/storage-summary` trend is no longer prewarmed; its route,
`GetStorageSummaryCapacityTrend` and the mock aggregate cache went with it,
and sampler ticks must not regain an aggregate storage synthesis step
without a live consumer.
That same metrics-hot-path ownership also includes metric-type selection for
compact chart reads. When chart routes such as `/api/charts/infrastructure` or
`/api/charts/workloads` request only a subset of canonical chart series,
`internal/monitoring/monitor_metrics.go` must preserve that narrowed metric
set through the batch store fallback path instead of querying every metric type
for each resource and discarding most of the payload afterward.
That same mock-runtime owner now also owns demo-scenario curation.
`internal/mock/fixture_graph.go`, `internal/mock/platform_fixtures.go`, and
`internal/mock/demo_scenarios.go` may project an authored demo estate over
generic fixture synthesis, but that authored layer must stay graph-native and
runtime-stable so infrastructure, workloads, storage, and recovery all present
the same human-readable platform story instead of a lab of random names,
legacy `mock-cluster` labels, or surface-specific mock overrides.
The authored Proxmox recovery story must also be evidence-backed rather than
painted onto random backup dates. Each curated VM and system-container profile
declares a protection story, and `applyDemoBackupScenario` replaces generic
guest backup artifacts with deterministic PVE backup, PBS backup, snapshot,
and task histories anchored to that profile's `BackupAge`. The default graph
must include representative current protected workloads, stale and
newer-failure attention workloads, snapshot-only and task-only unprotected
workloads, and a workload with no linked evidence that remains unknown.
`LastBackup`, recovery-point inventory, canonical protection posture, summary
counts, and filters must agree. A successful task without an independently
enumerated backup artifact must never mint protected posture. The
`TestCuratedDemoProtectionStoriesProduceRepresentativePostures` proof in
`internal/mock/demo_scenarios_test.go` evaluates those fixtures through the
production posture engine.
Mock alert history and incident timelines belong to the same fixture authority.
`internal/mock/alert_incidents.go` enriches every displayed mock alert row with
one occurrence-qualified incident whose resource identity, acknowledgement
state, lifecycle status, and ordered events agree with that row. Historical
fixtures close with a resolution event, active fixtures remain open, and
representative investigation and runbook events exercise the richer timeline
surface. Alert-level reads, resource-level incident lists, and graph-lifetime
notes must all query or mutate that same fixture instance; a mock history row
must never expose a Timeline control backed by a missing incident.
`internal/mock/alert_history.go` is the canonical history fixture generator and
must mirror the live history read contract: rows are newest-first, closed rows
carry the same resolution timestamp as their incident in `lastSeen`, and live
alerts come from the active-alert snapshot rather than unrelated synthetic open
history rows. The current-day generator is bounded by the fixture observation
clock and local calendar boundary, including short or long daylight-saving
days, so neither a row nor its lifecycle events can occur in the future.
History duration, status, and Timeline therefore remain three projections of
one occurrence instead of contradicting each other in demo mode.
Mock fixture defaults in `internal/mock/generator.go` (the `DefaultConfig`
constant) are also part of that mock-runtime contract. The Proxmox default is
an intentionally large public-demo estate so platform-first pages exercise
multi-cluster navigation, table density, sorting, grouping, drawer behavior,
responsive layout, and the production workload-windowing threshold out of the
box: eight named six-node Proxmox clusters plus two standalone nodes, with 10
VMs and 8 LXCs per node (900 guests total), 5 Docker/Podman hosts with 14 containers
each, 4 standalone Pulse-managed hosts, and 3 Kubernetes clusters
(Production EU + Staging EU + Development EU; a fourth Edge / k3s
profile is curated in `demo_scenarios.go` and instantiates when
`K8sClusterCount` is bumped) with 5 nodes, 40 pods, 14 deployments
each, and curated native controller inventory so the Kubernetes
platform-page overview tab shows multiple clusters and the
nodes/pods/deployments/controller tabs exercise multi-cluster grouping.
Each Kubernetes cluster carries its own node-name prefix
(`prod-euw1-k8s-*` / `stage-euw1-k8s-*` / `dev-euw1-*`), a distinct
kubelet version, and exactly one degraded scenario — Production EU
runs a NotReady worker (`prod-euw1-k8s-03`), Staging EU runs the
payments-worker CrashLoopBackOff, Development EU runs the
cron-nightly-backfill ImagePullBackOff — so the demo tells three
distinct stories instead of three clones. Each cluster also seeds
curated RBAC inventory (per-namespace Roles + RoleBindings plus an
aggregated ClusterRole / ClusterRoleBinding for `pulse-demo-monitoring`)
so the Kubernetes platform-page Configuration tab exercises the same
RBAC summary-count contract live agents use. The
`TestKubernetesDemoClustersTellDistinctStories` test in
`internal/mock/demo_scenarios_test.go` guards that distribution.
Generic Kubernetes fixture synthesis must also preserve at least one ready,
schedulable node whenever a cluster has nodes. Randomized readiness and cordon
stories may degrade the remaining nodes, but they must not accidentally create
a total outage that erases running-pod metrics or makes the demo and its proof
nondeterministic; explicit curated outage scenarios remain the owner of
cluster-wide unavailability.
Bumps to those defaults must keep the
curated demo scenario's per-node hostname seasoning in
`demo_scenarios.go` aligned (today: pve1..pve30 distributed across Production
West, Production East, Core Services, Disaster Recovery, and Edge Sites, plus
two named standalone nodes, with regional labels, shared-fabric storage names,
and per-node fallback naming) so the
broadcast and snapshot views render the same human-readable estate
regardless of the configured fixture size. The
clustered storage graph must remain linear in node count: each node may report
the cluster's bounded primary and offsite PBS targets, while shared NFS, RBD,
and CephFS pools are emitted once per cluster rather than once per peer. Demo
backup history likewise keeps two representative recovery points per protected
guest so a large estate remains realistic without manufacturing an unbounded
payload. `TestDefaultDemoProxmoxEstateIsLargeMultiClusterAndBounded` guards the
estate shape and storage bound, while `demo_scenarios_benchmark_test.go`
records graph-build, sampler-update, and unified-snapshot costs at the default
density.
The
monitor-broadcast equivalence test
(`TestMonitorBuildBroadcastFrontendStateUsesCanonicalMockUnifiedResources`)
compares broadcast count against the canonical snapshot count within a
±5% tolerance to absorb the legitimate row drops from
`coalesceBroadcastResources` and `convertResourcesForBroadcast` under
larger fixture sizes; that tolerance does not loosen the rest of the
test's exact-name and exact-identity assertions.
That same chart boundary also owns storage-series identity. Monitoring and
`ReadState` consumers must address storage pool and physical-disk history
through the resolved unified-resource metrics target, so seeded history,
runtime writes, storage summary hover selection, and detail charts all extend
one series instead of splitting between canonical resource IDs and
source-native metric IDs.
Proxmox Ceph pools are part of that same storage-series contract. When Ceph DF
exposes pools, monitoring must project each pool through the shared
`models.CephPoolStorage` helper, write storage history under that pool storage
id, and evaluate alerts through `CheckStorage` so per-pool thresholds, active
alerts, and charts all use the same storage series identity.
Ceph cluster identity is FSID-owned across discovery sources. Proxmox API Ceph
reports are canonical when available, host-agent Ceph reports are the fallback
or supplemental source, and state reconciliation must collapse reports for the
same FSID into one cluster while preserving source aliases for existing pool
thresholds. Host-agent Ceph pool storage ids must not carry `agent:` as their
canonical identity; that prefix remains only an alert/threshold alias for
previously persisted overrides.
That same chart boundary also owns provider-backed workload bridging.
Workload-chart consumers may query VM and system-container history through the
resolved unified-resource metrics target, but the emitted series identity must
stay on the canonical workload row ID, so VMware-backed workloads participate
in summary hover and focus without leaking provider-native metric IDs into the
UI contract.
That same chart boundary also owns Kubernetes mock-history completeness.
Seeded mock history and live mock appends must project Kubernetes clusters,
nodes, pods, and deployments onto the same canonical unified-resource metrics
targets that the registry exposes, instead of seeding only pod timelines and
leaving cluster, node, or deployment charts blank on the demo path. When the
mock sampler records a Kubernetes series, it must write the canonical cluster,
node, pod, or deployment key directly and preserve the same identity across
seeded history, in-memory continuation, and metrics-store fallback reads.
That same summary owner also owns VMware partial-success classification.
Optional VI JSON or Automation enrichment reads that fail after base
host/VM/datastore inventory succeeds must not collapse the whole poll into a
runtime failure. The client should preserve the usable base snapshot, record
degraded enrichment issues on the snapshot, and let the poller publish those
as `observed.degraded` plus summarized issue metadata instead of clearing the
observed contribution or pretending the refresh was fully healthy.
That same VMware inventory floor also owns operator-visible uptime and guest
filesystem usage. `vmware.InventoryMetrics` carries `UptimeSeconds`,
`DiskUsedBytes`, `DiskTotalBytes`, and `DiskPercent` for hosts and VMs so the
canonical `Resource.Uptime` field and `ResourceMetrics.Disk` series populate
on vSphere-backed workloads — without these the workloads table renders
empty "0s" and blank disk cells for every vSphere row. Real collection uses
PerformanceManager `sys.uptime.latest` (host + VM) plus
`sys.osUptime.latest` for VMs (Tools-reported guest OS uptime; preferred
when present), and `GET /api/vcenter/vm/{vm}/guest/local-filesystem`
aggregated across mount points for disk usage. A 503 from that REST
endpoint (Tools not running) is recorded as a non-fatal `unavailable`
enrichment issue rather than failing the poll. Mock fixtures
(`internal/mock/platform_fixtures.go`) must synthesize the same fields per
powered-on VM and drop them for powered-off VMs so the demo estate
exercises the same workload-table contract as live vCenter would.
That same broadcast converter owns the canonical Resource.Uptime
fallback. `monitorUptime` walks platform-specific carve-outs
(`Agent.UptimeSeconds`, `Proxmox.Uptime`, `Docker.UptimeSeconds`,
`Kubernetes.UptimeSeconds`, `PBS.UptimeSeconds`, `PMG.UptimeSeconds`,
`TrueNAS.UptimeSeconds`) before falling back to `Resource.Uptime`. The
vSphere adapter populates only the canonical field for ESXi hosts and
VMs, so without that final fallback the websocket payload would silently
drop uptime for VMware-backed rows even though the REST contract carries
it. Carve-outs still take precedence so existing platforms keep their
prior behavior.
That same poller-owned partial-success model must also keep runtime
observability non-noisy. Repeated polls with the same degraded optional-read
issue classes should not emit a fresh warning every interval; monitoring
should log only when VMware optional enrichment first degrades, materially
changes, or recovers.
That provider ownership now has a concrete phase-1 runtime seam:
`internal/monitoring/vmware_poller.go` must keep VMware inventory on the
shared supplemental-ingest path, declare `SourceVMware` as its owned source,
and cache per-organization, per-connection provider records instead of
projecting VMware through `StateSnapshot`-local host or storage arrays.
`internal/api/router.go` may start and stop that poller as shared runtime
infrastructure, but monitoring still owns the provider lifecycle, source
ownership, and canonical record emission rules for VMware.
That same VMware monitoring boundary now also includes the proof rule for
history depth. `PerformanceManager.QueryPerfComposite` clearly supports
host-plus-child metric collection, but exact VM and datastore history fidelity
still requires live proof on the supported version floor. If that proof does
not hold on the shared history model, the support claim must narrow rather
than falling back to VMware-only history paths.
That same VMware monitoring boundary now also includes the incident-context
rule. VMware event and task reads may support investigation, but they must
feed the shared incident and canonical resource-history paths instead of a
parallel VMware event store or provider-only incident timeline.
That same VMware monitoring boundary also includes the topology-signal rule.
Signals collected from non-projected VMware topology objects such as clusters,
folders, or datacenters may inform investigation only when they can be
attached honestly to canonical `agent`, `vm`, `storage`, or `network`
resources; the collector must not solve that ambiguity by creating VMware-only
top-level incident targets.
That same monitoring boundary now also has a concrete detail-enrichment seam.
`internal/vmware/client.go`, `internal/vmware/client_topology.go`, and
`internal/vmware/provider.go` may use the official vCenter Automation API plus
VI JSON `name`, `parent`, `runtime`, `resourcePool`, `datastore`, `host`,
`vm`, `Network.host`, `Network.vm`, and datastore-summary paths to enrich canonical VMware-backed resources
with placement, guest identity, and storage consumer context. That
enrichment remains best-effort provider detail on the shared VMware source: it
must not create a second topology cache, a VMware-only placement store, or a
parallel guest-identity model outside the canonical `agent` / `vm` /
`storage` / `network` resource graph.

The monitor adapter now also acts as the canonical bridge from live registry
rebuilds and supplemental ingest into the unified-resource timeline. That means
monitoring no longer just materializes state snapshots for consumers; it also
emits durable `ResourceChange` history through the shared resource store so
live monitoring updates and historical inspection stay aligned.
That same ownership now includes the resource-history projection of canonical
alert-lifecycle facts. The alerts-owned SQLite event log is the lifecycle source
of truth; monitoring consumes its delivery-independent lifecycle seam and
materializes fired, acknowledged, unacknowledged, snoozed, unsnoozed, and
resolved breadcrumbs in the unified-resource change store. Snooze projections
carry their actor and exact expiry so timelines explain both who paused
operations and when automatic delivery and escalation will resume; neither
transition changes acknowledgement, resolution, or incident identity.
Projection IDs derive deterministically from
alert identity, canonical resource, transition kind, and occurrence time, so
restart repair and duplicate consumer delivery are idempotent. Notification
activation, quiet hours, grouping, throttling, and destination health may never
gate this projection. Incident timelines project those breadcrumbs for
operator flow, while the resource timeline remains the durable resource-scoped
index rather than a second alert lifecycle authority.
Monitoring must install the lifecycle consumer, replay durable lifecycle
events oldest first, and then reconcile restored active alerts that predate
the event store. Replay and reconciliation create only missing projections,
including a stable `pulse-system` timeline identity for system alerts whose
public alert payload intentionally has no monitored-resource link; neither path
may duplicate a resource change or invoke notification delivery.
The monitor-owned incident store wiring must therefore attach the canonical
resource timeline reader whenever the unified monitor adapter is present, so
operator alert timelines and AI incident context project those lifecycle events
from canonical history instead of reading a second monitoring-owned timeline.
Lifecycle projection replay is bounded by a durable projection watermark
(`alert-lifecycle-timelines-v1`, stored beside the event log) and runs as one
background catch-up pass scheduled at the canonical resource-store boundary —
never synchronously on router construction or health serving, because a large
un-projected backlog must delay projections, not startup. The watermark
advances, with periodic mid-pass checkpoints, only when a pass runs with both
the incident store and the canonical resource-change recorder attached; a
partial-surface pass repairs what it can without marking events applied.
Resetting the watermark to zero forces a full repair replay for rebuilt
projection stores, while wiping a projection store without resetting the
watermark leaves already-applied events to request-time read-repair only.

The registry proof map now treats provider discovery and metrics history as
their own governed runtime surfaces instead of leaving them folded into a
generic monitoring catch-all. Changes to provider wiring, discovery helpers,
or metrics history retention must stay attached to those explicit proof routes.
Monitor construction is the runtime handoff for metrics-store placement and
aggregation cadence: `internal/monitoring/monitor.go` may pass the resolved
data path, `PULSE_METRICS_DB_PATH`, and `PULSE_METRICS_ROLLUP_INTERVAL` through
to `pkg/metrics`, but the SQLite path normalization, rollup bounds, and write
amplification policy stay owned by the metrics store rather than by a
monitoring-local helper. The `PULSE_METRICS_DB_PATH` handoff reaches only the
default org's monitor; non-default tenants always resolve `metrics.db` from
their own data path.
Install-wide telemetry counts are also monitoring-owned now. Any telemetry or
reporting surface that claims installation totals must aggregate across the
provisioned tenant set through the reloadable multi-tenant monitor boundary,
not by reading `GetMonitor()`'s default-org compatibility shim.
Those install-wide counts are now the canonical aggregate adoption signal for
outbound usage telemetry: monitoring owns the source counts for agent hosts, Docker
and Kubernetes workloads, storage pools and physical disks, Ceph, network
shares, TrueNAS systems/VMs/apps, VMware hosts/VMs/datastores, availability
targets, and active alerts. Telemetry callers may consume those coarse totals,
but they must not bypass monitoring to read provider-local identifiers or
tenant-local resource names.
That install-wide boundary also owns privacy-bounded outcome aggregation for
telemetry schema v3. It may count alert history entries fired, acknowledged,
or resolved within the existing 30-day local history window and notification
attempt, successful-delivery, and terminal failed/dead-letter totals within the
notification queue's existing seven-day telemetry window, across the
provisioned tenant set. Attempts include retries; a recoverable failed attempt
must not also become a terminal failure. It must consume only content-free
totals from tenant-owned managers and must not export alert
IDs, resource IDs, actors, reasons, destinations, recipients, endpoints,
timestamps, error text, or message content. Notification queue state remains
delivery evidence rather than alert-lifecycle truth, and monitoring must not
infer alert resolution from delivery success or failure.
That same reloadable multi-tenant monitor boundary also owns wiring tenant
identity into per-org notification delivery. When a tenant monitor is
initialized for a non-default org, monitoring installs an org-backed tenant
identity resolver on that org's notification manager so webhook payloads can
stamp the org ID and current display name; the resolver reads the org record
lazily so display-name renames propagate without monitor restarts. The
default org keeps environment-provided identity and must not be overridden
here.

Consumer packages already use `ReadState`, but the monitoring core still has
dual truth between unified resources and `StateSnapshot`. This is the main
remaining architecture-coherence lane.
Alert arrays are the explicit freshness exception inside that remaining dual
truth. Monitoring APIs that still serve `StateSnapshot` must project
`ActiveAlerts` and `RecentlyResolved` from the live alert manager at read time
instead of trusting the cached snapshot fields, so externally served alert
counts and recently resolved incidents do not lag behind acknowledgement,
resolve, or clear operations between explicit sync points.
A recently resolved alert that closed without recovering keeps that on the
projection: `models.Alert.Resolution` (reason, successor resource ID and name,
and the alert engine's summary) is set only for such a close, `cloneAlert`
copies it into every snapshot, and `recordAlertTimelineChange` passes it into
the `alert_resolved` resource change so the incident timeline says the alert
moved. `TestAlertResolutionWireContract` in
`internal/models/metrics_types_test.go` and
`TestMonitor_HandleAlertLifecycle_HandoverCloseIsNotARecovery` in
`internal/monitoring/monitor_alert_handling_test.go` pin it.
The container entrypoint in `docker-entrypoint.sh` now also lives under this
boundary. Hosted or managed tenant bootstrap changes must preserve safe startup
when immutable read-only mounts are layered into `/etc/pulse`; the entrypoint
may not reintroduce ownership mutation against those read-only files during
container boot.
That same startup path must avoid recursive ownership mutation of image-owned
runtime directories such as `/app` and `/opt/pulse`; those paths are build-time
artifacts, and copy-up into per-container writable layers is a monitoring and
host-health regression, not a valid runtime repair.
That same monitoring boundary now also owns Docker Swarm runtime truth at the
collection seam. `internal/dockeragent/swarm.go` is the canonical manager-side
filter for live Swarm services and tasks, so monitoring consumers do not ingest
historical shutdown tasks as if they were still part of the active runtime.
Standalone Docker daemons report `Swarm.LocalNodeState=inactive`; that is not
Swarm capability evidence and must be normalized away before agent reports,
monitoring ingest, or unified-resource consumers can surface Swarm roles,
services, tasks, tabs, or alerts.

Storage export is now derived from canonical `ReadState.StoragePools()`
instead of `GetState().Storage`; `models.Storage` is treated as a boundary
artifact for that path.

Node export is now derived from canonical `ReadState.Nodes()` instead of
`GetState().Nodes`; `models.Node` is treated as a boundary artifact for that
path.

Host export is now derived from canonical `ReadState.Hosts()` instead of
`GetState().Hosts`; `models.Host` is treated as a boundary artifact for that
path.

Docker host export is now derived from canonical `ReadState.DockerHosts()`
instead of `GetState().DockerHosts`; `models.DockerHost` is treated as a
boundary artifact for that path.

VM and container export are now derived from canonical `ReadState.VMs()` and
`ReadState.Containers()` instead of `GetState().VMs`/`GetState().Containers`;
`models.VM` and `models.Container` are treated as boundary artifacts for those
paths.

PBS instance export is now derived from canonical `ReadState.PBSInstances()`
instead of `GetState().PBSInstances`; `models.PBSInstance` is treated as a
boundary artifact for that path.

Backup-alert guest lookup assembly now derives VM/container identity from
canonical `ReadState` workload views instead of from snapshot-owned guest
arrays, so backup alert resolution follows unified runtime truth when a live
resource registry exists.

Physical-disk refresh/merge logic now derives physical disks, nodes, and linked
host-agent context from canonical `ReadState` before applying NVMe temperature
and SMART merges, so skipped or background disk refresh no longer treats the
snapshot as internal truth for that path.
That same monitoring-owned disk merge path must also treat host-agent SMART
attributes as canonical fill data for the Proxmox disk view. When a linked
host agent reports SMART health, SMART identity, ZFS pool membership, or NVMe
`percentage_used` for a physical disk that Proxmox itself exposes without
trustworthy health, wearout, model, serial, WWN, type, size, or pool data, the
merge path in `internal/monitoring/monitor.go` must promote that missing data
into the canonical physical-disk model without overwriting provider truth. The
read-state sensor conversion must preserve SMART `SizeBytes` so subsequent
refreshes keep whole-disk capacity evidence available for Proxmox disk merges.
The Proxmox polling runtime in `internal/monitoring/monitor_pve.go` must
evaluate disk alerts only after that merged disk view exists, so
controller-backed disks do not lose health and endurance coverage between
collection and alerting.
Wide SAS inventories use the same trust boundary. The host collector must
fan out per-disk SMART reads with bounded concurrency so a single report
deadline cannot truncate a controller-sized suffix of the inventory, and it
must preserve the Linux controller plus HCTL (or controller-member target)
alongside each reading. When Proxmox exposes a SAS address as `serial` while
smartctl exposes the drive serial, the smartctl serial is the canonical
hardware identity; exact device-path correlation is permitted only inside an
already-linked host/node parent and must fail closed when topology is
ambiguous. Direct SATA, SAS, and NVMe device fallback IDs retain their legacy
shape, while multiple controller members behind one block path add their
controller target to the fallback identity. Per-member I/O must never inherit
an aggregate controller counter.
A linked node's agent with no SMART reading for a device, such as a member's
standby row, files that device's I/O under the Proxmox disk's metrics target
(`proxmoxPhysicalDiskMatchesForLinkedNode` reads `MetricResourceID`). For a
member merged with its Proxmox row that target is the member's own SMART key,
scoped to the member once (`PhysicalDiskMetaMetricID`), so its SMART and I/O
history share one series. The target used to carry the member topology twice,
and I/O already stored under that doubled key is left to age out rather than
migrated. Proof: `TestIdentitylessControllerMembersReadTheirWritersHistory`.
The same rules apply to SATA and NVMe inventory: direct-disk source IDs keep
their historical shape, controller-member IDs add their member target, and
cross-source correlation is scoped to the canonical parent node. A successful
retry may enrich an earlier smartctl attempt but must not erase earlier model,
serial, failure, or counter evidence. SMART temperature selection accepts only
plausible readings, prefers ATA attribute 194 over 190 when higher-level
temperature fields are invalid, and preserves reported zero counters as known
values while leaving omitted counters unknown. Plausibility is decided at the
full 64-bit width of the raw attribute, before any narrowing to the reported
`int` temperature. `int` is 32 bits wide on the 386 and arm agent builds, so a
raw value whose low 32 bits happen to land in the plausible band, such as
4294967316 truncating to 20, must be rejected as the out-of-range value it is
rather than published as a real reading.

Disk identity, temperature, I/O, controller association, and pool membership
also carry typed collection state from `pkg/diskinventory`: `available`,
transiently `unavailable`, provider/controller `unsupported`, or unexpectedly
`missing`. Normalization may retain the last known value when the current
observation is not available, but it must preserve the current state and
reason so API and UI consumers do not present retained evidence as freshly
collected. A host agent's SMART temperature and I/O counters follow its
reporting lease: once `State.ExpireHostTelemetry` expires the agent, they stay
as last-known values marked `unavailable` ("host agent stopped reporting"),
and the linked Proxmox disk merge and the registry let the agent's own later
state supersede the availability it supplied earlier
(`diskinventory.MergeReportedStatus`), so neither a skipped disk poll nor a
host that is never disk-polled again can carry it forward. Expiry is
compare-and-set on the report time the evaluation judged stale, so a report
accepted in between is never expired.
A legacy agent (from before collection provenance) withdraws its readings
without a source, so `hostAgentSMARTCollection` stamps its reading copied onto
the Proxmox disk through `diskinventory.LegacyHostAgentStatus`, the same rule
the unified-resources registry applies to match that withdrawal to the copy: a
silent legacy agent's retained temperature is not collected on the canonical
disk either, even when no disk poll refreshes the copy. Proof:
`TestSilentLegacyAgentCanonicalDiskFollowsItsWithdrawal` in
`internal/monitoring/physical_disk_roundtrip_test.go`.
Every SMART temperature history writer
records a temperature only when its current collection state is available (or
predates collection state), so a retained reading has to keep its non-available
state to stay out of history; a path that relabels a carried reading as
available, such as a node-temperature carry stamped `proxmox_node_smart`, is
not covered by this rule.
`diskinventory.TemperatureCollected` is the shared form of that test for
consumers outside the history writers: the metrics-history API serves a disk
`smart_temp` live point, and mock mode derives a synthetic disk temperature
series from the disk's reading or pads a stored one, only from a collected
temperature, so a retained standby or silent-agent reading never reappears as a
point at the current time.
The SMART history writers in `internal/monitoring/monitor.go` and
`monitor_agents.go` call that helper rather than a private copy of the rule,
and `GetPhysicalDiskTemperatureCharts` returns observed history samples without
padding a short series to now, even when the current reading was collected.
Retained readings never manufacture history. A live disk with no history keeps
its metadata without a fabricated series. Mock mode may use its seeded demo
history, which is separate from a disk reading. Proof:
`TestDiskTemperatureChartsRetainOnlyObservedSamples` in
`internal/monitoring/monitor_metrics_slo_test.go`.
Unified-resource physical-disk round trips must retain named
`StorageGroup` membership rather than degrading it to the generic `Used`
filesystem label.
That same host-agent temperature boundary must prefer a recent linked host-agent
payload over legacy SSH collection once the agent provides any usable CPU, NVMe,
GPU, or SMART temperature reading. `internal/monitoring/monitor_polling_node_helpers.go`
may invoke SSH only when no linked, recent, available host-agent temperature
exists or the agent payload has no usable positive reading. Identity-only or
zero-temperature SMART rows do not count as usable by themselves, but the
runtime must not keep probing legacy SSH solely to augment an otherwise healthy
agent temperature payload with SMART data.
A host agent's sensors feed the node the agent is linked to. Several Proxmox
connections can each have a node of the same name (one `px1` per site), and
`Host.LinkedNodeID` always holds the Proxmox source node ID, whether the link is
automatic, manual or restored from host continuity. The poller therefore hands
`getHostAgentTemperatureForNode` in `internal/monitoring/host_agent_temps.go` the
polled node itself, and the lookup works on that node's slot. A node the read
state holds under its own source ID is its own slot. The state folds the views
of one machine reached through two connections (a cluster added twice, or a
multi-homed host added by each address) into one node kept under one view's ID,
chosen by its merge preference, and an automatic agent link follows that node.
So a polled view the read state does not hold also takes the one same-named
node proven to be the same machine, by `models.NodeObservationsSameMachine` or,
for the agent linked to that node, `models.HostAgentBridgesNodeViews` (the agent
reports both endpoints and the views' TLS fingerprints do not contradict), both
in `internal/models/node_machine_identity.go`. These are pairwise forms of
evidence `UpdateNodesForInstance` folds on, and it shares the agent
corroboration rule with them;
`TestNodeObservationsSameMachineMatchesStateFold` and
`TestHostAgentBridgesNodeViewsMatchesStateFold` pin them against the state's
fold for the covered cases. Two or more such candidates are ambiguous and none
is taken, as the state declines an ambiguous alias. An agent linked to a slot ID
is the node's agent. A manual link stays pinned to the source ID the operator
chose, because the state never transfers operator intent to a replacement
provider identity, so while the state keeps that machine under the other view's
ID the node has no linked agent and no agent reading. Otherwise the hostname
fallback takes only an agent with no node, VM or container link whose hostname
is exactly the node name, and only when no Proxmox node outside the slot and no
second unlinked agent has that name. Cluster-sibling readings an agent collects over SSH arrive keyed by bare
node name, so the cluster sensor cache records the reporting agent. A reading
serves the node only when the node that agent is currently linked to belongs to
one of the slot's connections, or to a connection whose configured view of the
node carries a matching TLS fingerprint (the other connection of a cluster added
twice). The reading follows the agent's link the way the agent's own sensors do,
and a reading from an unlinked agent serves no node. Without these bounds, site
B's `px1` showed, alerted on and recorded history for site A's agent or
cluster-sibling sensors. `TestNodeTemperatureStaysWithinSameNamedNodesSite` in
`internal/monitoring/monitor_additional_test.go` pins the poller lookup,
including the folded-view cases, and
`TestApplyHostReportScopesClusterSensorsToReportersConnection` in
`internal/monitoring/monitor_host_agents_test.go` pins the agent ingest side.
A node temperature presented as available must describe the node now, because
node alert evaluation, node history writes, reporting, and the UI all read it as
a live measurement. A host agent linked to a Proxmox node merges into one host
row whose `LastSeen` every PVE poll keeps fresh, while the agent's last sensors
stay in state after it stops reporting, so the row's `LastSeen` cannot decide
whether those sensors are current. `internal/monitoring/host_agent_temps.go`
reads the agent's own sighting from the row's `SourceStatus(SourceAgent)`, feeds
the agent's sensors into the node only while that sighting is inside the
reporting lease that keeps the agent online (`hostAgentReportCurrent`, the same
`hostAgentHealthWindow` that `evaluateHostAgents` applies, so a slow-interval
agent's readings count exactly while the agent is shown online), and stamps the
reading with that report time. Past the lease the lookup falls back to the
cluster sensor cache, which keeps its own recency check. When every source
returns nothing, `internal/monitoring/monitor_polling_node_helpers.go` may carry
a previous reading, with its original `LastUpdate`, only inside the carry window
(twice the PVE polling cadence, which under adaptive polling is the
scheduler's maximum interval, never under five minutes); an older reading is
dropped rather than re-presented as current. A carried reading also keeps a
lapsed host agent's lease. Agent readings are stamped with the agent's report
time, so `carriedTemperatureOutlivesAgentLease` drops a carried reading stamped
no later than the last report of the node's matched agent (by link, then
hostname, whether or not that report had sensors) once that agent has lapsed.
Readings record no source, so the bound is conservative: an SSH, cluster-cache
or agent-plus-SSH merged reading (stamped with the agent's time) of that age is
dropped too, being already older than the lease, while a reading stamped after
that report keeps the carry window. A carried reading (`carriedNodeTemperature`)
keeps only the node's CPU and GPU temperatures: its SMART and NVMe rows would
also stamp the node's physical disks, whose freshness and standby state a
carried copy cannot follow (a disk that spun down would be stamped and recorded
with its pre-sleep value), so they are left out, and nothing is carried when
only they remain.
The poller reads its previous poll's nodes back from the unified read state
through `internal/monitoring/monitor_previous_state.go`. `nodeLastOnline` and
the temperature carry with its CPU low/record tracking match them by node ID,
which is the Proxmox source ID, so the projection must return
`NodeView.SourceID()`, never the unified resource ID, and carry the stored
`TemperatureDetails`. Nodes preserved through an instance outage
(`preserveOrExpireNodes`) or authoritative membership reconciliation are written
back into state from that projection, so it also restores the identity inputs
the registry derives a node's canonical ID and cross-view merges from: cluster
name, provider-scoped flag, native aliases, TLS fingerprint and linked agent ID,
the config-derived ones stamped the way `pollPVENode` stamps them. With the
unified ID a node re-keys on every failed poll, and without its cluster
identity a cluster member changes canonical ID for the length of an outage.
A preserved node keeps its
temperature only under the same carry rules and loses it when it goes
offline. Node memory carry-over after a failed status read uses the poller's own
validated `NodeMemorySnapshot`, not the read state, which holds the agent's
reading on a node merged with a host agent, and keeps used, cache and free
summing to the total.
The poller's node verdicts reach the read state as its own. A cluster member
`pollPVENode` reports offline on a live poll stays `offline` in
`NodesSnapshot`, `/api/resources` and every read-state consumer when its linked
host agent falls silent. A node `preserveOrExpireNodes` expires stays `offline`
once its Proxmox sighting goes stale, unless another source says otherwise: a
reporting agent or a current availability check keeps it `online`, and a quiet
agent whose last verdict was not offline makes it `warning`. The registry's
stale pass used to turn the first into `online` and the second into `warning`,
because it read a sighting that had delivered recently as online
(unified-resources, "Status follows each source's own verdict"). A node inside
the offline grace keeps its `online` verdict and reads `warning` once its
sighting goes stale. Node alerts were never affected, because `CheckNode` reads
the poller's own `models.Node`.
Legacy SSH temperature collection must also use the Pulse sensor-wrapper
contract before falling back to raw lm-sensors output. `internal/monitoring/temperature.go`
must request `/usr/local/sbin/pulse-sensors` when it exists, parse the wrapper
payload as `{sensors, smart}`, preserve backward compatibility with old forced
`sensors -j` keys, and expose SMART disk temperatures through the same
`models.Temperature.SMART` path used by the physical-disk merge.
When the payload arrives in the legacy raw `sensors -j` shape, the parser must
mark it via `models.Temperature.LegacySensorsFormat` and the host-agent merge
in `internal/monitoring/host_agent_temps.go` must preserve that marker, so the
frontend can surface a data-gated outdated-sensor-setup notice instead of
letting SATA/SAS disk temperatures silently stay blank on pre-rc.6 SSH key
setups.
That same Proxmox monitoring boundary also owns checked response parsing for
polymorphic numeric fields. Shared client parsers such as
`pkg/proxmox/replication.go` must use the package's checked integer conversion
helpers instead of direct casts, so malformed or oversized Proxmox values do
not overflow into monitoring state.

Backup polling and recovery guest identity assembly now derive workload node,
name, and type context from canonical `ReadState` instead of from
snapshot-owned VM/container arrays, so storage backup polling, guest snapshot
polling, timeout sizing, PBS recovery candidate assembly, and Proxmox recovery
ingest all follow unified runtime truth when a live resource registry exists.
That recovery identity adapter must carry the canonical unified-resource
`ResourceID` separately from the provider-native Proxmox `SourceID`. Recovery
mappers consume the canonical ID directly for subject linkage and retain the
source ID only for provider correlation and fallback derivation; a canonical
workload ID must never be passed through source-specific ID generation again.
That same monitoring-owned workload boundary now includes canonical app
workloads projected through unified resources, not only VM/LXC-style guests.
Consumers that need runtime workload truth must treat `ReadState.Workloads()`
as the cross-platform workload surface for VMs, system containers, docker
containers, and API-backed app containers such as TrueNAS apps instead of
assuming workload views stop at traditional guest types.
Typed unified-resource views also need to present canonical monitoring truth,
not raw ingest formatting. Linked topology accessors exposed through
`internal/unifiedresources/views.go` must trim outer whitespace before
returning linked agent, node, VM, or container IDs so downstream consumers do
not observe `" node-99 "` style drift when the canonical linkage is `node-99`.
Source-owned IDs exposed through those same typed views must also trim outer
whitespace before they reach monitoring consumers, so a docker host, VM, node,
or storage view cannot appear to carry a different source identity just
because the ingest payload wrapped the source ID in spaces.
That same monitoring-owned Docker ingest path must also preserve persisted
container metadata across routine container recreation. When
`ApplyDockerReport` observes the same canonical docker host reporting a new
runtime container ID under the same normalized container name, monitoring must
copy custom URL, description, tags, and notes metadata onto the new container
ID instead of dropping that operator state on ordinary container replacement.
If multiple prior containers normalize to the same name, the migration must
fail closed and skip the copy rather than guessing between ambiguous sources.
Name normalization for that contract must treat Docker's leading `/` prefix as
presentation noise rather than identity, so routine recreate flows keep
metadata continuity when one report spells the same container as `/app` and a
later report spells it as `app`.
Docker-managed app-container web-interface metadata must use the same
host-plus-normalized-container-name identity (`app-container:<host>:name:<name>`)
as the stable synchronization key. Monitoring must migrate current runtime-key
Docker metadata and legacy app-container guest metadata to that key when a
Docker report is ingested, then prefer that stable guest key when projecting
unified app-container custom URLs. Runtime container IDs remain action and
metric identities, not the persistent URL metadata identity.
When a container retains its runtime ID but changes normalized name, monitoring
must move stable guest and Docker metadata to the new name and remove the
obsolete name key after a successful or already-resolved destination. Rename
migration must snapshot all sources before writing so swaps do not exchange
URLs accidentally, and ambiguous normalized source or target names must fail
closed. A later unrelated container that reuses the old name must not inherit
the renamed container's URL.
Kubernetes pod, Deployment, and Service web-interface metadata uses
`k8s-workload:<cluster>:<kind>:<namespace>:<name>` as its stable logical
identity. Monitoring must migrate a current legacy unified-resource key, plus
the legacy `k8s:<cluster>:pod:<pod-uid>` key for pods, when that resource is
observed. Runtime UIDs remain discovery and metrics coordinates. Every scope
component is required so a URL cannot cross cluster, namespace, or kind
boundaries, and an existing empty stable record is an intentional clear that
must block legacy fallback.
Unified resource projection must also hydrate saved host-level web-interface
URLs from the tenant monitor's canonical metadata stores for standalone
agents, Proxmox nodes, Docker/Podman runtimes, PBS and PMG instances, and
Kubernetes clusters and nodes. Stable provider/source identities take
precedence over display names, runtime registry IDs, or host labels; canonical
and superseded identity aliases may be consulted for migration continuity but
must not broaden tenant scope. A non-empty operator metadata value overrides a
configured source URL; absent or cleared host metadata preserves or reveals a
configured source fallback. Stable workload metadata is authoritative even
when empty so an explicit workload clear cannot revive a stale runtime-key or
projected URL. Projection must copy the resource snapshot rather than mutating
the ingest input.
The monitor-owned guest, Docker, and host metadata stores are the live
in-memory authority for projection and migration. API, config export/import,
tenant usage, Assistant URL discovery, and reload paths must share those exact
tenant-scoped store instances rather than opening parallel caches over the
same files.
The same applies to proxmox topology coordinates exposed through typed views:
node, cluster, and instance accessors must return canonical trimmed values so
monitoring consumers do not fork topology grouping or labeling on `" pve-a "`
versus `pve-a`.
That same canonical guest runtime truth now also includes Proxmox pool
membership. The cluster-resource builders and traditional VM/LXC pollers must
carry `pool` through `models.VM` and `models.Container` so reporting and
inventory surfaces consume one canonical guest topology contract instead of
re-deriving pool membership from API-local queries.
Connected infrastructure and monitored-system projections now also use the
shared unified-resource display-name fallback, so the monitoring layer does
not rebuild its own canonical name-or-hostname selection for those surfaces.
Connected infrastructure now also consumes the shared top-level system
resolver from unified resources instead of maintaining an independent
machine/hostname grouping heuristic. Monitoring-owned inventory surfaces must
therefore stay aligned with the monitored-system ledger on one canonical
top-level system identity contract, and that contract must not count friendly
display names as identity.

Storage-backup preservation now also derives node-to-storage membership from
canonical `ReadState.StoragePools()` instead of from snapshot-owned storage
arrays, leaving only persisted backup/cache payloads in this path on direct
snapshot state.

Canonical monitoring guardrails now also fail if resource-array access is
reintroduced through `GetState().VMs`/`Containers`/`Nodes`/`Hosts`/`Storage`/
`DockerHosts`/`PBSInstances` helpers, and the subsystem registry now requires
explicit proof-policy coverage for all owned runtime files.
Memory-source classification now also routes through one canonical runtime
catalog and extracted node resolver under `internal/monitoring/`. Node, VM,
LXC, diagnostics, and
diagnostic-snapshot consumers must normalize aliases such as `avail-field`,
`meminfo-available`, `meminfo-derived`, `meminfo-total-minus-used`, and
`listing-mem` onto the governed canonical labels `available-field`,
`derived-free-buffers-cached`, `derived-total-minus-used`, and
`cluster-resources` before trust or fallback reporting is emitted.
That same catalog owns fallback-reason defaults for governed fallback sources,
so monitoring producers and downstream diagnostics must not fork fallback
classification or reason text through lane-local switch statements.
That same canonicalization boundary must also run when snapshots are recorded,
not only at source selection time: node and guest diagnostic snapshots must
normalize memory-source aliases and backfill default fallback reasons before
logging or persistence, so later diagnostics/reporting cannot diverge just
because one poll path still emitted a compatibility label.
That same guest-memory boundary also owns the low-trust Proxmox status-memory
selector. When cache-aware availability is unavailable, the shared selector in
`internal/monitoring/guest_memory_sources.go` must derive `status-freemem`
against the effective balloon total and prefer that fallback over `status-mem`
when Proxmox reports a saturated or materially inconsistent used figure, so
Windows and ballooned guests do not get pinned to false 100% usage samples.
That same guest-memory boundary also owns fallback order and cache scoping for
Proxmox VMs when `MemInfo` is absent. Guest RRD is not a memory evidence
source: recorded PVE 8 and PVE 9 guest `rrddata` responses (fixtures under
`pkg/proxmox/testdata/rrd/`) prove the cache-aware `memavailable`/`memused`
columns exist only in node RRD, so `GuestRRDPoint` parses only the recorded
guest columns (`time`, `maxmem`), `PVEClientInterface` exposes no guest RRD
lookups, and the VM memory resolver must not consult the guest RRD endpoint
(#1634). Monitoring must try guest-agent `/proc/meminfo` via the shared
Proxmox client, and only then linked host-agent memory. Guest-agent fallback
caches must key on `(instance, node, vmid)` instead of raw `node/vmid`, so
separate Proxmox instances cannot leak stale or foreign memory evidence into
each other just because they reuse the same node name and VMID.
Linux memory availability must never be inferred from `MemTotal-MemFree`.
Nodes and guests prefer a valid explicit `MemAvailable`/`available` field,
then a complete reclaimable-component estimate, then — for nodes, where the
columns actually exist — valid node RRD availability or used evidence. The
conservative old-kernel guest-agent estimate is
`MemFree + Buffers + Cached + SReclaimable - Shmem`; it is valid without swap
but not from truncated or total/free-only meminfo. A material `total-used`
gap may remain a lower-trust estimate only when it supplies independent
evidence that the reported used value already excludes cache. Invalid,
overflowed, non-finite, over-total, or conflicting candidates must be rejected
before the next source is considered; an explicitly present zero node RRD used
or zero available value remains a valid idle or full-pressure sample rather
than being mistaken for an absent field.
Node RRD fallback caches must key on `(instance, node)`, just as guest-agent
caches key on `(instance, node, vmid)`, so identically named nodes in
different Proxmox instances cannot exchange memory evidence.
Running LXC memory acknowledges the same Proxmox API reality: guest RRD
responses carry only cache-inclusive `mem`/`maxmem` columns, so the LXC
memory path performs no guest RRD lookup at all. A running container with a
non-zero cluster-resource listing value must report that cache-inclusive
value under the low-trust `cluster-resources` source rather than reporting
the guest unavailable; `unavailable` is reserved for running containers with
no listing evidence at all (#1634). The guest sources `rrd-memavailable` and
`rrd-memused` are node-only labels now; guest reliability scoring must not
treat them as trusted guest evidence.
Unified Linux and Docker agent ingest likewise must not repair a missing used
value from total minus free alone; it may use an explicit used/percentage or
complete free-plus-cache evidence. In every collector, known capacity with no
cache-aware usage is represented by `models.Memory.UsageUnavailable`, the
canonical memory source `unavailable`, and fallback reason
`cache-aware-memory-unavailable`. A recent trusted node or guest snapshot may
be carried across a transient reconnect under the existing bounded
`previous-snapshot` rule; otherwise the unknown state must remain honest.
Unknown memory samples must not append zeroes to in-memory or persistent
history, project a canonical unified-resource memory metric, start or clear a
threshold alert, or render as 0% in product surfaces. Existing active alerts
remain fail-safe until a later trusted sample crosses the clear threshold.
That same guest-memory boundary also owns stabilization when Proxmox falls
back to low-trust VM full-usage readings. The shared VM polling paths must use
the previous guest diagnostic snapshot, not the resource model, to decide when
one more `previous-snapshot` carry-forward is justified. A live guest-agent
signal is sufficient healthy evidence for that decision even before disk or
network enrichment finishes, and the preserved result must be recorded with an
explicit snapshot note so diagnostics can distinguish deliberate stabilization
from ordinary fallback.
Guest-disk continuity now follows the same canonical rule. The shared VM
polling paths must classify guest-agent disk failures consistently, surface the
resulting disk-status reason on the VM model, and only carry forward previous
disk usage when the last VM snapshot is still recent guest-agent truth rather
than an already carried-forward fallback. That keeps transient guest-agent or
status-call failures from regressing a VM back to misleading allocated-disk
data while still avoiding indefinite replay of stale disk summaries.
The previous VM that `previousVMFromView` builds from the unified read state
carries disk usage only when it is Proxmox's own reading. When a manually
linked Pulse agent's disk filled in because Proxmox had no guest filesystems
(`VMView.DiskFromLinkedAgent`), carrying it would relabel the agent's value as
a Proxmox `prev-` read, and with Proxmox outranking the agent on guests that
copy would then freeze over the agent's live disk.
`TestPreviousVMFromViewKeepsLinkedAgentDiskOutOfProxmoxCarry` pins both owners.
That compatibility boundary also applies to historical snapshot labels that may
still exist in tests, live in-memory state, or pre-canonical diagnostic paths:
legacy aliases such as `rrd-available`, `rrd-data`, `node-status-available`,
`calculated`, and `listing` must normalize onto the governed canonical labels
before snapshots are returned to diagnostics consumers, not only when new
snapshots are first recorded.
The same canonical identity rule now applies when removed host agents are
blocked from re-reporting. `ApplyHostReport` must resolve the final canonical
host identifier before it checks `removedHostAgents` or emits the
reconnect-blocking error, and removed-host records must carry machine and token
identity so the block is scoped to the retired host. Hostname equivalence may
only participate when it is qualified by the same token and compatible machine
identity; removing one stale duplicate must not poison a different live host
that shares the same hostname or raw machine identifier through a different
token.
Docker host re-identification now shares the same hostname-equivalence rule:
monitoring may treat `qnap` and `qnap.local` as the same host when the token
or machine identity already points at one canonical runtime, but it must not
invent broader short-name collapsing on its own or fork away from the
unified-resource monitored-system contract.
That same Docker host identity boundary also owns token-binding aliases after
a reconnect match. When `ApplyDockerReport` has already matched a report to an
existing canonical Docker host, the token uniqueness guard must accept that
host's stable source ID and previous agent ID as aliases for the current raw
agent ID so container recreation does not reject the same logical host after it
has been matched. This must not weaken the one-token-per-Docker-agent rule for
different hosts.
Node disk-source selection now also routes through one canonical resolver
under `internal/monitoring/`. When a Proxmox node has a linked Pulse host
agent, the node summary must prefer the linked host's canonical disk view over
Proxmox `rootfs` bytes because dataset-level `rootfs` can materially
under-report ZFS-backed node capacity and usage. Proxmox `rootfs` and `/nodes`
disk values remain fallback sources only when no linked host disk truth is
available. When the runtime must fall back beyond the linked host and `rootfs`
paths, it must treat the raw `/nodes` disk figure as low-confidence and prefer
the canonical local system storage owner instead of whichever mounted storage
is merely present or largest. On multi-storage Proxmox hosts, fallback
selection must rank `local-zfs`, `local-lvm`, `local`, and other non-shared
guest-root storages ahead of backup-only mounts, and storage-derived disk
metrics may override the `/nodes` figure only when that figure is the active
source or node disk truth is otherwise absent.
TrueNAS monitoring ownership now also includes provider rebind semantics in
`internal/monitoring/truenas_poller.go`. When a stored TrueNAS connection's
host, auth, TLS, or fingerprint settings change, the poller must replace the
live provider instance instead of keeping stale connection state in memory
until the process restarts.
That same monitoring boundary now also owns canonical per-connection poll
health and discovered-summary state for the settings platform-connections
surface. `internal/monitoring/truenas_poller.go` must honor each configured
TrueNAS connection's `pollIntervalSeconds`, keep the next poll schedule plus
last success/failure state in one canonical runtime owner, and project the most
recent discovered host/pool/dataset/app/VM/share/disk/recovery counts there instead of
recomputing settings health panel-by-panel. That same poller-owned summary must
also absorb manual saved-connection test results from the shared
`POST /api/truenas/connections/{id}/test` path, so row-level operator tests in
settings update the canonical last success / last error state instead of
stopping at disconnected toast notifications.
That same runtime owner also defines the feature-default contract for TrueNAS:
the API-backed integration is on by default, and `PULSE_ENABLE_TRUENAS` is an
explicit opt-out switch rather than a required bootstrap toggle.
That same TrueNAS monitoring boundary owns system identity compatibility for
`/system/info`. `internal/truenas/client.go` must tolerate provider-version
drift in non-identity display fields such as `buildtime`, including structured
date/value wrappers, and still preserve the canonical hostname, version,
machine ID, capacity, and poll-health path instead of failing connection tests
or background refreshes during JSON decoding.
### TrueNAS pool capacity is an observation, not a default zero

`internal/truenas/pool_capacity.go` owns validation of the existing pool-specific
byte sources. Legacy REST consumes its flat `size`, `allocated`, and
optional `free` fields. RPC retains its existing pool-state/property envelope;
the property envelope is also used by the separate boot-pool reader. Neither
that boot source nor a hypothetical nested REST shape establishes a supported
CORE data-pool capacity fallback. Missing native capacity remains unavailable;
datasets, quotas, physical disks, and leaf vdevs must not be summed to invent it.

The one supported CORE data-pool source is the pool's own vdev space accounting
(#2510). CORE 13 `pool.query` (REST `GET /pool`) returns neither flat bytes nor
zpool properties; its `pool_extend` attaches the libzfs vdev groups, and
py-libzfs gives every vdev a `stats` object whose `size`/`allocated` are
`vs_space`/`vs_alloc`. OpenZFS maintains those on top-level vdevs only, and
zpool's own `size`/`allocated` properties are the sum of the normal, special
and dedup classes. Pool capacity therefore falls back, last after flat fields
and properties, to the sum of the top-level `data`, `special` and `dedup` vdev
stats. Log, cache and spare vdevs and leaf children are never counted. Any
counted top-level vdev without exact valid stats, an empty data group, an
overflowing sum, or allocated above size leaves the pool unavailable rather
than reporting a partial total, and an explicitly null flat field never
triggers the fallback. REST topology is decoded number-preserving.

A usable reading requires exact non-negative integer total and allocated bytes,
with a positive total and allocated no greater than total. Optional free bytes
are derived from those same two pool bytes only when absent; an explicitly
reported free value must equal their difference. Null, malformed, fractional,
overflowing, negative, or contradictory fields invalidate the capacity reading,
not the pool's identity, health, topology, scan, or the rest of the snapshot.
Numeric strings are accepted without float conversion. RPC pool results use
number-preserving decoding, and flat/property readings never mix or use a real
zero as a fallback trigger. Transport, permission and invalid JSON-envelope
errors retain their existing failure behaviour; there is no extra method call.

The provider emits no pool usage metric for an unavailable or out-of-bounds
reading. A host aggregate is available only if every inventoried pool has valid
usage and its byte sum fits int64; a known boot pool is not a whole-host reading
when a data pool is unknown. Available per-pool and dataset readings remain
independent. Unknown polls must not append a zero or refresh an old observation
in either memory or persistent storage History. Existing historical readings
remain intact, and a later valid poll resumes at that poll's observation time.

`internal/truenas/pool_capacity_test.go` covers the TLS legacy snapshot through
provider/registry ingestion, real empty/full pools, absent/malformed fields,
exact RPC bytes, source separation and incomplete/overflowing aggregates.
`internal/truenas/core_pool_capacity_test.go` drives a CORE 13 `pool_extend`
row (mirror, special, log, cache and spare vdevs) and the CORE boot state
through the legacy client, snapshot, provider and registry, plus the
fail-closed topology cases.
`internal/monitoring/truenas_pool_capacity_test.go` connects TLS collection,
provider refresh, two same-named connection-scoped estates, canonical ingestion,
memory and persistent History over known/unknown/malformed/recovered polls.
Its deterministic polling clock and API bodies are declared fixtures, not native
CORE acceptance, a containing release or an alternative capacity source.

That same monitoring boundary now also owns live TrueNAS disk temperatures.
`internal/truenas/client.go` and `internal/truenas/provider.go` must ingest
legacy `disk.temperatures` from the REST API or `reporting.get_data` `disktemp`
from the current JSON-RPC transport, and project those readings into the
canonical physical-disk model and risk path
instead of leaving temperature telemetry agent-only or adding a TrueNAS-local
presentation shim.
That same monitoring boundary also owns SMART-backed TrueNAS disk risk
projection. When TrueNAS raises disk-local SMART alerts such as
`truenas_smart`, `internal/truenas/provider.go` must fold that incident truth
into the canonical physical-disk risk payload instead of leaving SMART failure
state trapped in incident/status-only decorations that storage consumers do
not read. Current TrueNAS drive-health releases expose failure counters through
`alert.list` without exposing equivalent raw attributes through the supported
disk API. Pulse must therefore retain a dismissed SMART alert only when its
native arguments or serial resolve to exactly one currently inventoried disk:
dismissal acknowledges notification state but does not erase monotonic hardware
evidence. Other dismissed alerts, and dismissed SMART alerts with missing or
ambiguous disk identity, remain suppressed. Native uncorrectable-error, failed
self-test, and low-spare-block classes map to critical canonical disk risk even
when TrueNAS labels the source alert as a warning. For the corresponding native
classes, typed `ue` and `sb` arguments from `alert.list` project onto canonical
media-error and available-spare SMART fields after exact disk resolution. The
provider must reject negative or out-of-range values, retain the worst value
when duplicate evidence is present, and never derive a counter from formatted
alert text. Available-spare parsing must validate the integer and its 0..100
domain before conversion to the canonical `int` field; oversized, fractional,
or wrapped malformed values are rejected rather than narrowed or truncated.
The same boundary owns TrueNAS `smart_status` normalization. `internal/truenas/client.go`
must parse REST and RPC SMART status separately from native disk state, and
`internal/truenas/disk_health.go` plus `internal/truenas/provider.go` must map
null, empty, missing, unknown, or unavailable SMART telemetry to canonical
`UNKNOWN` health with no replacement-required risk. Explicit SMART failure and
native failure states such as `FAULTED`, `FAILED`, `OFFLINE`, `REMOVED`, and
`UNAVAIL` must continue to produce canonical disk-health risk.
That same boundary owns boot-pool and replication-target storage posture.
`internal/truenas/client.go` must collect `boot.get_state` through the
connection's negotiated transport, use REST only on a version-gated legacy
connection, merge it only within the current configured connection, and use
its vdev leaves to enrich boot-disk pool membership and native ZFS state.
`internal/monitoring/truenas_poller.go` must correlate
`replication.query` intent across providers within the same organization using
local/PULL ownership or a unique configured/observed target-host match for
remote PUSH tasks. The resulting `SET`/`REQUIRE` receive-side read-only posture
is healthy, while ordinary read-only datasets remain warning and locked or
unmounted datasets remain offline. Correlation must fail closed when target
identity is absent or ambiguous, and common pool or dataset names on another
connection are never sufficient identity.
That same boundary now also owns recent aggregate TrueNAS disk temperature
history. `internal/truenas/client.go` must ingest `disk.temperature_agg`, and
`internal/truenas/provider.go` must project the returned min/avg/max readings
onto the shared `physicalDisk.temperatureAggregate` contract so disk-health
consumers can reuse one canonical metadata shape instead of inventing a
TrueNAS-only history payload.
That same boundary now also owns the canonical disk-history write path for
API-backed disks. `internal/monitoring/monitor.go` must sync non-native
physical-disk resources such as TrueNAS disks into the shared `disk`
metrics-store contract via the existing SMART-temperature writer, so physical
disk charts and disk-health consumers read one history path instead of a
TrueNAS-only temperature cache.
That same TrueNAS monitoring ownership also includes runtime mock continuity.
When `/api/system/mock-mode` changes on a live server, the TrueNAS supplemental
provider must rebind immediately and repopulate the canonical read state so
settings, infrastructure, storage, and other shared consumers see the same
mock-backed inventory without restart.
That same runtime mock ownership now also includes fixture authority. Mock
TrueNAS and VMware inventory plus mock metrics-history seeding must derive from
one shared platform fixture owner in `internal/mock/` so settings payloads,
supplemental ingest, unified read-state, and seeded charts cannot drift from
each other when the v6 runtime runs in mock mode.
That same fixture authority now also includes legacy snapshot-backed platforms.
`internal/monitoring/monitor.go` and
`internal/monitoring/mock_metrics_history.go` must treat
`internal/mock/fixture_graph.go`, `internal/mock/platform_fixtures.go`, and
`internal/mock/demo_scenarios.go` as the one canonical mock owner for legacy
Proxmox/Docker/Kubernetes/agent/PBS/PMG snapshot state plus provider-backed
TrueNAS and VMware fixtures. Monitoring must not rebuild mock provider context
from standalone defaults, consume partial legacy helper exports, or mix
snapshot state with separate provider fixtures when seeding read-state or
metrics history. The graph, its platform projections, and its curated demo
scenario layer are the canonical mock runtime API.
Availability mock fixtures belong to that same graph authority: UPS network
cards, MQTT meters, HTTP panels, and controller ping targets must be authored
once in `internal/mock/` and then projected into availability status, unified
resources, and connections payloads from that shared graph.
Governed action mock fixtures follow the same rule. Pending, approved,
executing, completed, rejected, and failed action examples are authored once in
`internal/mock/action_fixtures.go`, reference resources from the graph's
canonical unified-resource snapshot, and are projected by the read-only action
API without writing demo rows into the durable action-audit database.
That same boundary now also owns native disk-history fallback when Pulse's own
history is shallow. `internal/truenas/client.go`,
`internal/truenas/provider.go`, `internal/monitoring/truenas_poller.go`, and
`internal/monitoring/monitor_metrics.go` must route TrueNAS `disktemp`
reporting history through the shared physical-disk chart path, so canonical
disk charts can render real provider-backed history instead of flat padding
after restarts or immediately after onboarding.
That same monitoring boundary now also owns modern TrueNAS app workload
telemetry. `internal/truenas/client.go`, `internal/truenas/provider.go`, and
`internal/monitoring/monitor.go` must ingest `app.stats` through the official
`/api/current` JSON-RPC websocket transport, project those readings onto the
canonical `app-container` metrics contract, and sync them into the existing
guest metrics-history/store path. Pulse must not add a TrueNAS-only charts
lane for that telemetry.
That same monitoring boundary now also owns native TrueNAS VM inventory.
`internal/truenas/client.go` must ingest `vm.query` through the official
`/api/current` JSON-RPC websocket transport, `internal/truenas/provider.go`
must project those rows as canonical `vm` resources under the top-level
TrueNAS appliance, and frontend TrueNAS surfaces must read the typed
`TrueNASData.VM` facet instead of inventing a provider-local VM table contract.
Pulse must not treat TrueNAS VMs as Proxmox guests, Docker containers, or a
separate `truenas-vm` resource type.
That same monitoring boundary now also owns connected-infrastructure
projection for API-backed platforms. `internal/monitoring/connected_infrastructure.go`
must project TrueNAS into the canonical connected-infrastructure surface list,
carry TrueNAS hostname/version through the shared top-level system grouping,
and preserve platform-managed surfaces such as `proxmox`, `pbs`, `pmg`, and
`truenas` when host telemetry is ignored. Ignore/remove semantics on that
surface remain machine-scoped and may only strip the local `agent`, `docker`,
and `kubernetes` reporting surfaces from the grouped row. That same
connected-infrastructure payload now also owns guest-link continuity for host
agents: when an agent is running inside a VM or system container, monitoring
must preserve the canonical linked guest identity on both active and ignored
connected-infrastructure rows instead of forcing settings consumers to infer
guest-backed hosts from labels or hostnames.
path or treat API-backed app workloads as second-class compared with native
Docker reports.
That same boundary now also owns native host-history fallback for API-backed
TrueNAS systems. `internal/truenas/client.go`,
`internal/truenas/provider.go`, `internal/monitoring/truenas_poller.go`, and
`internal/monitoring/monitor_metrics.go` must route TrueNAS
`reporting.get_data` system history through the shared `agent` guest-chart
path, so canonical host charts can show real provider-backed CPU, memory,
network, and disk throughput history when Pulse's own local history is still
shallow. That same guest-chart boundary must treat windows beyond the
in-memory chart threshold as store-backed hot paths: batch helpers may merge
native/provider history afterward, but they must not spend the steady-state
latency budget on full in-memory pre-scans that can never satisfy long-range
coverage, and any caller-supplied metric filters must flow into the shared
batch store query instead of being trimmed only after retrieval.
That same monitoring boundary now also owns canonical TrueNAS app control
refresh semantics. `internal/truenas/provider.go` and
`internal/monitoring/truenas_poller.go` must execute native app start/stop
actions through the owned TrueNAS runtime and refresh cached records and
recovery ingest immediately afterward, so assistant-driven app control does
not rely on stale provider state or ad hoc config-local action paths.
That same monitoring boundary now also owns canonical TrueNAS app log reads.
`internal/truenas/client.go`, `internal/truenas/provider.go`, and
`internal/monitoring/truenas_poller.go` must read bounded app-container logs
through the owned `/api/current` JSON-RPC runtime and tenant-scoped poller
selection path, so assistant-driven diagnostics do not depend on the unified
agent or a parallel config-local read path.
That same monitoring boundary now also owns canonical TrueNAS app
configuration reads. `internal/truenas/provider.go` and
`internal/monitoring/truenas_poller.go` must serve API-backed app-container
runtime/config shape through the same tenant-scoped provider snapshot and app
selection path used for control and logs, so assistant config reads do not
fork into a separate ad hoc fetch path or stale config cache.
That same monitoring boundary now also owns API-backed TrueNAS system
telemetry for the top-level NAS host. `internal/truenas/client.go` must ingest
`reporting.realtime` through the official `/api/current` JSON-RPC websocket
transport, `internal/truenas/provider.go` must project those readings onto the
canonical host `AgentData` and shared `ResourceMetrics` contract, and
`internal/monitoring/monitor.go` must sync them into the existing `agent`
metrics-history/store path. Pulse must not add a TrueNAS-only top-level
system charts path or leave TrueNAS host telemetry outside the canonical host
history contract. Host profile fields on `AgentData`, such as `hostProfile`
for Unraid-compatible Pulse Agent hosts, are presentation identity only; typed
read access through `internal/unifiedresources/views.go` must keep
`Platform()` as the normalized runtime platform and expose the profile through
a separate host-profile accessor.
That same monitoring boundary now also owns API-backed TrueNAS CPU
temperature. `internal/truenas/client.go` must use the modern
`reporting.get_data` RPC surface to derive current `cputemp` readings in the
same RPC session as system telemetry, and `internal/truenas/provider.go` must
project those readings into the canonical host temperature and host-sensor
contract. Pulse must not treat TrueNAS CPU temperature as an agent-only
capability or invent a TrueNAS-local sensor payload.
Taken together, this is the current monitoring-owned TrueNAS floor: one stored
API connection can surface one canonical top-level system, shared host
telemetry/history, app-container workloads, native VM workloads, disk
health/history, native network shares, and per-connection poll health plus
observed contribution counts without requiring the unified agent. The same
poller/provider path also owns assistant-driven app start/stop, logs, and
config refresh for canonical app workloads. Pulse does not promise a separate
TrueNAS runtime model, broader NAS administration, or agent-required bootstrap
at this floor.
That same monitoring boundary now also owns VMware signal enrichment on the
canonical alert timeline. `internal/vmware/client_signals.go`,
`internal/vmware/provider.go`, and `internal/monitoring/monitor_alerts.go`
may collect VI JSON overall status, active alarms, recent tasks, and VM
snapshot counts, but they must project those reads onto shared canonical
resources plus shared alert/resource history metadata instead of persisting a
VMware-only signal cache, event log, or provider-specific incident timeline.
That same monitoring boundary now also owns VMware recent-task and recent-event
breadcrumbs on the shared canonical resource timeline. `internal/vmware/`
provider code plus `internal/monitoring/vmware_poller.go` and
`internal/monitoring/monitor.go` may emit read-only `activity` changes through
the shared supplemental-ingest path, but those entries must land in the same
canonical `resource_changes` store used by every other resource timeline read.
Pulse must not add a VMware-only task/event table, replay log, or provider
history reader just because the VI JSON event surfaces differ from alert and
metrics collection.
That same monitoring boundary now also owns VMware performance telemetry on
the shared chart/history paths. `internal/vmware/client_metrics.go` must use
the VI JSON `PerformanceManager` read surfaces to resolve current-support,
available counters, and current samples from the supported `vCenter` release
floor; `internal/vmware/provider.go` must project ESXi host readings onto
canonical `agent` `ResourceMetrics` and VM readings onto canonical `vm`
`ResourceMetrics`; and `internal/monitoring/monitor.go` must sync those
metrics into the existing shared `agent` and `vm` history stores. Pulse must
not add a VMware-only charts cache, host history model, or VM metrics store
just because vSphere performance collection uses a different API family from
inventory and alarm reads.
That same monitoring boundary now also owns Proxmox guest-agent continuity
when `/status` is transiently missing. Recent guest-agent evidence and the
shared guest metadata cache must keep VM network and identity metadata alive
long enough to survive short Proxmox status failures, while incomplete
guest-agent metadata stays on a short retry cadence instead of freezing
partial VM summary data for minutes.
The persisted guest metadata store must also remain the synchronization
boundary for last-known guest identity updates. Store reads and writes must
copy metadata, including slice fields, so asynchronous monitor persistence
cannot expose mutable store pointers to caller goroutines or race with
release-pipeline `-race` backend proofs.
When Proxmox reports saturated VM memory without `meminfo` or `freemem` but
the QEMU guest agent is queryable, the monitoring memory selector must prefer
the guest's own `/proc/meminfo` `MemAvailable` signal before lower-trust
Proxmox RRD or status fallbacks. Guest-agent filesystem payloads from Windows
volume GUID mounts remain part of the same canonical VM disk metric path and
must not be dropped just because system-reserved partitions share a physical
disk with usable volumes.
That same monitoring boundary now also owns physical-disk I/O history as a
first-class canonical metric stream. `internal/monitoring/monitor_agents.go`
must project host per-device I/O counters onto the same SMART-resolved disk
resource id that unified resources expose, `internal/monitoring/metrics_history.go`
must retain `disk`, `diskread`, `diskwrite`, and `smart_temp` on one shared
disk history model, and mock seeding plus live mock ticks in
`internal/monitoring/mock_metrics_history.go` must append to that same disk
timeline instead of creating a second drawer-only or mock-only disk history
path.
The SMART-resolved id is the serial the disk resource carries: when a SMART row
reports none, `HostSMARTDiskMetricID` takes the one the host's Unraid inventory
reports for that disk, as the unified-resources adapter does, unless the Unraid
row does not describe the disk (`unraidDiskDescribesSMARTRow`: a controller
member, or several SMART rows on the disk's device path), and
`hostDiskIOMetricResourceID` keys
a device with no non-standby SMART row by its Unraid serial
(`HostUnraidDeviceMetricID`, which refuses a device several Unraid rows name)
before the linked Proxmox node's disks and the `<host>:<device>` fallback.
Controller members sharing a device path keep distinct keys, so the I/O
writer's aggregate-device guard, which compares every row on the path by its
own identity (standby rows and legacy `sdc [sat]` labels included), files
their counter under no member.
Proof:
`TestAgentDiskHistoryFollowsTheSerialItsUnraidRowReports` and
`TestHostDiskIOMetricResourceIDFallbacks`.
The agent's `smart_temp` series holds the reading the disk resource shows,
not only the SMART row's own. `writeHostSMARTMetrics` writes
`HostSMARTDiskTemperature`, which falls back to the host's Unraid inventory
reading when the row has none, and `writeHostUnraidDiskTemperatures` writes
`HostUnraidDiskTemperature` under `HostUnraidDiskMetricID` for each Unraid row
whose key is not one of the host's SMART rows' keys, such as a `--disk-exclude`
member, so an Unraid reading never overwrites a SMART row's own at the same
timestamp.
Unraid rows sharing a key are one disk in the registry, which shows the latest
of their readings, so the last row with a reading decides that key's single
sample. `diskinventory.TemperatureCollected` keeps a spun-down disk's or an
expired host's leftover reading out of history.
Proof:
`TestAgentDiskChartsTheUnraidTemperatureItShows` and
`TestApplyHostReportChartsUnraidTemperatureOfDiskWithoutSMART`.
That same monitoring-owned disk-health boundary also includes shared storage
risk assessment in `internal/storagehealth/`. When providers or host agents
emit structured storage topology such as Unraid per-disk state, the shared
assessment layer must derive canonical risk and alert severity from that
richer disk topology instead of letting coarser aggregate counters override it
and flap the operator-facing storage alert surface.
That shared assessment never judges disk temperature. Heat is a metric owned
by the alert disk temperature policy (`alerts.Manager.DiskTemperatureThreshold`:
the per-type `diskTempByType` trigger, else the agent Disk Temp default), which
users tune per disk type. Disk risk is rebuilt by registries that cannot see
that configuration, so a heat rule here would contradict it: a flat 60/70C rule
once called a healthy 63C NVMe hot and a 56C SATA disk fine. No disk type,
temperature or collection state reaches `storagehealth.Sample`, the TrueNAS
provider and the Unraid inventory assessment included, and a hot disk keeps an
`online` status. Proof: `TestDiskAssessmentsNeverJudgeTemperature` in
`internal/storagehealth/risk_test.go` and
`TestRecordsLeaveDiskHeatToTheAlertPolicy` in
`internal/truenas/provider_test.go`.
That same monitoring-owned storage polling boundary also owns cluster-shared
Proxmox storage status coherence. `internal/monitoring/monitor_polling_storage.go`
must merge shared storage observations across nodes into one cluster-scoped
record whose canonical status remains `available` whenever any reporting node
still has the shared target active; node-local inactive copies may expand node
affinity, but they must not downgrade the cluster record into an offline
projection just because that node won the capacity sample.
That same monitoring-owned Proxmox backup boundary also owns the inventory
readiness signal used by backup orphan alerts. `internal/monitoring/` must
record when PVE VM and container inventory has successfully observed a given
instance and guest type, including template VMIDs that are intentionally
excluded from normal workload resources. Backup alert evaluation may then
receive that scoped signal from monitoring, but alert code must not infer PVE
orphan readiness from recovery rollups alone.
That same Proxmox backup boundary also owns permission-repair guidance for PVE
backup visibility failures. When storage content reads fail with authorization
errors, the monitoring warning must tell operators to grant `/storage`
`PVEDatastoreAdmin` to both the `pulse-monitor@pve` service user and the
configured privilege-separated token when that token id is known.
That same monitoring-owned host-agent ingest boundary now also owns
vendor-managed NAS RAID normalization. `internal/monitoring/monitor_agents.go`
must filter vendor-managed system arrays through the shared
`internal/storagehealth/` rules before host state sync so internal Synology
`md0/md1` and QNAP `md9/md13` volumes do not leak into canonical APIs,
resources, or alert inputs just because those hosts report Linux md arrays
alongside customer-managed storage pools.
That same monitoring runtime boundary also owns logger-safe reload behavior.
`internal/monitoring/reload.go` may refresh runtime config, but it must do so
through the no-logging-init config loader so an in-process monitoring reload
does not reinitialize the global logger while pollers, websocket writers, or
tests are still emitting logs. Runtime context access in the monitor-owned
pollers must likewise route through the monitor's synchronized accessor instead
of reading mutable shared fields directly from concurrent goroutines.
That same monitoring-owned PBS job-health boundary must keep backup task
evidence honest. PBS does not expose a canonical scheduled backup-job
configuration API, so PBS-side backup-family entries may only be labeled as
observed task-history evidence. Scheduled backup compliance for PVE workloads
belongs to a future PVE `/cluster/backup` source. PBS task-history reads must
therefore use a bounded filtered lookback over `/nodes/localhost/tasks` and
surface truncation or permission gaps explicitly instead of treating one recent
unfiltered sample as configured backup-job proof.

Task 09 preserves two APT telemetry clocks at host-agent ingest: `CheckedAt`
is agent-observed time and `ObservedAt` is server-received time. Monitoring
must not overwrite the former with the latter; replay and skew safety consumes
both timestamps downstream.

Monitoring now also exposes a bounded direct Proxmox guest observation for the
governed action verifier. `ObserveProxmoxGuest` resolves the configured
instance client under the monitor lock, reads VM or LXC status and uptime from
the Proxmox API, validates the requested guest identity, and stamps server
observation time. It must not satisfy this contract from cached resource state
or node-agent telemetry: the action layer depends on this read remaining in a
trust domain distinct from the node agent that executes `qm` / `pct`.

Canonical Proxmox VM/LXC typed views also expose the monitoring-owned source
delivery status and timestamp alongside guest power state. Downstream Patrol
transition detection consumes that status instead of inventing a fixed stale
window: a stopped guest can have fresh inventory, while a stale source cannot
authoritatively prove either a stopped transition or recovery.

### Agent fleet diagnostic derivation

Monitoring owns the read-only Agent Fleet Doctor derivation over current host,
Docker, Kubernetes, removed-agent, profile-assignment, and deployment state.
`internal/fleethealth/agent.go` supplies the shared agent connection identity,
heartbeat cutoff, and version-drift vocabulary used by both the monitoring
diagnostic and API connections ledger. Five expected reports must be missed,
with a five-minute minimum, before an agent becomes stale; missing timestamps
remain pending/never-reported rather than silently healthy. Version comparison
uses the canonical agent-update target independently from the running server
build version.

The diagnostic may derive bounded updater and module failure reasons, normalized
platform and network evidence, profile drift, and safe repair-handoff support.
It hashes raw machine IDs, filters malformed interface addresses, and redacts
unbounded error strings before returning evidence. Derivation must not mutate
monitor state, probe providers, enqueue commands, or turn a repair hint into
execution authority. Runtime-family normalization is shared with agent
lifecycle through `platformsupport.ResolveAgentRuntimePlatform`. Explicit
Windows, macOS, and FreeBSD families remain distinct; known unsupported OS
families and missing evidence fail closed; and unmatched non-empty values from
legacy agents resolve to Linux because those releases reported gopsutil
distribution identifiers instead of their compiled GOOS. This intentionally
avoids a duplicated distro allowlist, so long-tail distributions such as
Mageia receive the same Linux repair handoff as Ubuntu or Debian. Unknown
updater states remain explicit warnings, and unverified FreeBSD/pfSense
installer state still fails closed for upgrade-command support.
The `typed-privilege-helper` module is the explicit fail-closed specialization
of that generic module derivation: a non-running state emits
`agent_privilege_helper_degraded` and explains the omitted privileged
telemetry. Ordinary operation degradation remains a warning; a future terminal
`failed` state may retain critical severity under the same reason code. Neither
form creates a remote repair action or broadens collector authority.
Credential truth is derived from the current server token inventory, not the
last successful heartbeat alone. A live host whose reported `TokenID` no
longer resolves emits `agent_credential_missing`; a resolved but expired record
emits `agent_credential_expired`; both are critical and expose a bounded
authentication-repair handoff only when the runtime family is safely known.
A missing-credential verdict is additionally cross-checked against the
subject's own authentication evidence: when a row reporting the judged token
id shows it authenticated within the freshness window (three report
intervals, ten-minute floor), the diagnostic emits the warning
`agent_credential_registry_stale` naming the id and the last authentication
instead of the critical outage, and offers no authentication-repair handoff,
because a credential that authenticated moments before diagnosis is not
revoked — the server's token registry view is stale (#1730). Only same-token
rows vouch; a fresh sibling row on a different credential never rescues a
genuinely revoked one, and rows with stale or absent authentication evidence
keep the critical missing verdict.
An active record is not sufficient when the host reports Pulse command
execution enabled: if that record lacks `agent:exec`, the diagnostic emits the
critical `agent_exec_scope_missing` reason and exposes the same bounded
authentication-repair handoff. Command-channel disconnection alone is not
credential evidence and must not synthesize this reason. The token inventory
therefore retains the active record's normalized scopes for read-only
diagnosis rather than reducing active credentials to an existence set.
Monitoring also compares live host-agent generations without merging them: two
different host IDs with the same non-empty machine ID and equivalent hostname
each receive `duplicate_host_agent_installation` plus bounded peer evidence.
That reason disables generic upgrade and authentication handoffs, because the
server cannot infer which co-installed local service an operator's command
would mutate. Distinct rows are retained so Agent Doctor does not disguise two
installations as one healthy machine. Mock mode has no authoritative token
inventory for its synthetic hosts and therefore must not turn fixture token IDs
into missing-credential incidents.

The default-org monitor retains the canonical server configuration pointer
rather than a tenant-isolation copy, so tokens minted after startup become part
of that current inventory immediately and cannot produce a false
`agent_credential_missing` diagnosis. `MultiTenantMonitor` deep-copies only
non-default tenant configuration; those tenant copies remain isolated from the
primary runtime's mutable token state.

That ownership survives a full monitor reload. `ReloadableMonitor` loads and
validates replacement settings, stops the old monitors, and refreshes the
original configuration under `config.Mu` before creating the replacement
manager. It must not split the retained API/authentication configuration from
the default monitor. `TestReloadPreservesLiveAgentCredentialInventory` exercises
two reloads followed by token creation and revocation, including the diagnostic
for a recently used credential. It preserves the registry-stale warning for a
credential that is actually absent, while preventing it for a live token.

`internal/fleethealth/agent_test.go` and
`internal/monitoring/agent_fleet_doctor_test.go` are the focused runtime proofs.

Removed-agent rows keep command-scoping identity instead of losing it with the
live record: host and Docker removal capture the agent's last-known reported
platform onto the removed record (`RemovedHostAgent.Platform`,
`RemovedDockerHost.Platform`), and the fleet diagnostic resolves that retained
value through the same runtime-family normalization as live subjects, so
`/api/agents/diagnostics` reports Linux for retained legacy distro identifiers
but no platform for missing or explicitly unsupported evidence. Removed
Kubernetes clusters retain no platform because the cluster report never
carries one; downstream host-side cleanup handoffs must treat an empty platform
as "offer explicitly labeled commands for every family, never one guessed
executable". The retained field is additive and optional on the
serialized removed lists, so snapshots recorded before the field existed load
unchanged with an empty platform.

### Unified Agent destination delivery metrics

The local agent health listener exports
`pulse_agent_destination_configured{module,destination,role}` and
`pulse_agent_destination_delivery_up{module,destination,role}`. Role is bounded
to `primary` or `observer`; destination names come from validated configuration.
Observer delivery failure is visible but does not make the primary authority
unready or merge observer retry state into primary delivery health.
Host, Docker/Podman, and Kubernetes reporters each fan out the already-collected
snapshot without triggering a second collection. Their retry queues and latest
delivery gauges remain per destination, and Kubernetes observer transport uses
the same destination-scoped TLS and explicit plaintext policy as the host and
Docker reporters. Observer acknowledgements never change the canonical
monitoring configuration returned by the primary.

### Proxmox protection evidence collection

PVE backup-file enumeration follows the same explicit evidence boundary as
direct PBS collection. Every completed PVE cycle emits subject-linked recovery
points with provider scope and point evidence plus one typed observation for
the polled PVE instance. Full node and backup-storage enumeration records
complete history with sufficient permissions. Partial node or content success
records partial history, and total failure records unavailable history. A
partial or total authorization failure records partial or denied access
without deleting retained artifacts. The observation is persisted before
reconciliation, so cached backup files cannot continue presenting current
protection after collection becomes incomplete or unauthorized.

Direct PBS backup enumeration emits two separate storage/recovery inputs:
subject-linked recovery points and one typed provider observation for the
polled PBS instance. A complete poll records complete history with sufficient
permissions; a partially successful poll records partial history and the
appropriate partial or unknown permission posture; total transient failure
records unavailable history; total terminal authorization failure records
denied access. Retained backup points survive failed enumeration, but the new
provider observation immediately prevents those cached points from being
presented as current protection truth.

PBS mapping attaches provider scope and a typed evidence envelope to every
successfully enumerated recovery point. Identity correlation is confirmed only
for direct canonical identity and inferred only for an auditable unique
provider-scoped guest match. Monitoring persists the collection observation
before point reconciliation so completeness and permission failure cannot be
lost behind a successful cached-artifact path. Shared protection semantics stay
in `internal/recovery/`; Proxmox monitoring owns only this explicit evidence-quality
adapter.

### Alert-intent evidence adapters and UDP outcomes

Monitoring supplies read-only context to the alerts-owned intent resolver. The
operator-state adapter resolves source-native references to one canonical
unified-resource ID before reading durable operator intent. Lookup failure,
ambiguity, absence, or store error yields no suppression context; monitoring
does not synthesize maintenance state. The adapter may traverse the live
canonical parent chain for maintenance only. An ancestor contributes an active
occurrence only when its scope is `resource_and_descendants`; monitoring mode,
lifecycle state, and every other operator field remain exact-resource policy.
When active windows overlap, the adapter projects the occurrence with the
latest end together with its source id and inherited marker. That resolution
is `unifiedresources.ActiveMaintenanceCoverageAt`, the rule Patrol's
operator-state projection shares, so a host window scoped to descendants
silences its guests' alerts and Patrol findings alike.

Backup-aware offline intent consumes a PVE task only when VMID, instance, and
node match and the task is active. `pollBackupTasks` stamps server observation
time. Evidence older than five minutes, more than one minute in the future,
finished, terminal, or missing an observation time fails closed. This
short-lived alert context is separate from PBS protection evidence and from
recovery assurance; it cannot claim that a backup is restorable or authorize a
restore.

Availability probing owns three outcomes: reachable, unreachable, and
indeterminate. UDP response-required mode needs a request and treats timeout or
mismatch as unreachable. Open-or-filtered mode may return indeterminate after
the full response deadline. Indeterminate clears accumulated failure count,
projects warning evidence, and emits no availability incident; it never claims
reachability. `internal/monitoring/availability_udp_test.go`,
`internal/monitoring/monitor_alert_intent_test.go`, and the backup polling
assertion in `internal/monitoring/monitor_full_coverage_test.go` are the focused
proofs.

### Durable host-agent removal admission

Monitoring ingest shares server-side host lifecycle authority with the
agent-lifecycle subsystem. `Monitor.RemoveHostAgent` must persist a tombstone
in the host continuity journal before it revokes an unused token, removes the
live record, unlinks resources, clears connection health, or resolves alerts.
The journal transition is rollback-safe. Persistence failure leaves the live
host present; journal load failure prevents monitor construction; expiry
failure retains the block. Monitoring must never convert an unavailable
security journal into an empty in-memory removal map.

`Monitor.New` hydrates canonical ID plus report-host, agent, machine, hostname,
platform, and token aliases from every non-expired tombstone. Removed entries
are excluded from active standalone-host continuity, monitored-system
projection, and remote-config fallback. The 24-hour expiry is based on the
persisted `removedAt` timestamp and deletes durable state before clearing the
snapshot and cache.

`ApplyHostReport` and removal are ordered by the dedicated host lifecycle
read/write lock. Concurrent reports may proceed together, but deletion waits
for earlier reports and prevents later reports from resurrecting the host.
Accepted reports and completed removals both republish the canonical unified
resource store before returning. An immediate state read therefore observes
the removal even when the prior registry generation remains inside the
read-path freshness window.
Only a post-removal token and matching retained machine identity may transition
the tombstone back to active continuity. The canonical host ID survives report
ID or persisted-agent-ID alias changes across Linux/systemd, Docker unified
agents, and Windows MachineGuid identities. Token-plus-hostname disambiguation
continues to keep simultaneous cloned or duplicate machine IDs distinct.

After fresh-token re-enrollment, the old token stays attached to the host's
denied-token lineage even when that token is intentionally shared and remains
valid for another active agent. Monitoring rejects the detached credential
before token binding can manufacture a duplicate host. Manual operator
allowance is the only path that clears that lineage. Focused proofs are
`internal/monitoring/monitor_host_agent_removal_lifecycle_test.go`,
`internal/monitoring/monitor_host_agents_test.go`, and
`internal/api/host_agent_removal_lifecycle_integration_test.go`; the concurrency
proof must also pass under the Go race detector.

Fresh-install reconciliation also applies before a tombstone exists. A
Pulse-issued install token created after a same-machine,
same-normalized-hostname observation is explicit re-enrollment evidence:
monitoring preserves the established host ID, removes older duplicate
generations and token bindings, and keeps physical-disk resource identities
attached to that host. The retiring process may deliver one final in-flight
report after token creation, so eligibility extends through at most that
agent's own health window. One unambiguous candidate is required; differing
non-empty report IPs, an active identity conflict, multiple candidates, an
arbitrary API token, or reports beyond the overlap window preserve the
clone-safe fork. Accepted handoffs retire the old token binding so a late old
process cannot overwrite the replacement. `internal/monitoring/monitor_host_agents_test.go`
proves stable-ID reuse, overlap handoff, clone-safety vetoes, duplicate cleanup,
and the live-generation guard.

### Native pool-health collection and appliance isolation

CORE's legacy REST disk inventory treats `rotationrate` as optional provider
metadata, just like the RPC reader: integers and numeric strings retain their
classification, while null, absent, unknown and unrecognised values provide no
rotation evidence. The existing explicit HDD/SSD/NVMe type fallback remains;
an unknown QEMU/VTSCSI disk gains no SMART health, temperature, serial or SSD
type assertion. Required disk-field decoding, complete JSON inventory shape
and HTTP errors still fail collection. Both inventory and disk-temperature
identifier discovery use the tolerant shape.

`TestRESTDiskRotationRateSnapshot` and
`TestRESTDiskRotationRatePreservesErrorsAndTopology` in
`internal/truenas/client_api_shapes_test.go` bind complete snapshots, mixed
alert arguments, RPC parity, healthy neighbours and actual topology evidence.
`TestTrueNASPollerCOREUnknownRotationRate` in
`internal/monitoring/truenas_poller_test.go` binds saved-config ordinary polling,
canonical unknowns, failure without renewed evidence, recovery and disable.
These synthetic controls reproduce and repair #2466's disk failure; they do
not establish native appliance acceptance or a containing release.

TrueNAS monitoring preserves the complete native `pool.query` observation
needed by the shared storage-health contract: pool GUID and status detail,
structured scrub or resilver state, pool and vdev read/write/checksum counters,
mirror/RAIDZ/spare topology, path-only leaves, and explicit native
missing/unavailable members. `disk.query` absence alone is not missing-disk
evidence. Unknown fields remain unknown and may not be converted into a failed
device, a recovered pool, or a zero-error observation.

The poller keys system and child source identity by configured connection.
Appliances with matching hostnames, restored pool GUIDs, or matching pool names
remain separate through refresh, cache rebuild, restart, and registry ingest.
Replication-target readonly classification remains a separate native-evidence
step and cannot hide locked or unmounted dataset state.

TrueNAS pool and dataset storage facets carry `StorageMeta.Enabled` and
`Active` like every other storage producer, because the AI storage tools and
storage platform payloads read those flags directly. TrueNAS has no disabled
state for an imported pool or a listed dataset, so both are always `Enabled`. A
pool is `Active` unless its native state is FAULTED, OFFLINE, REMOVED or
UNAVAIL, and a dataset is `Active` only while it is mounted and unlocked.
`TestProviderRecordsPopulateStorageEnabledAndActive` in
`internal/truenas/provider_test.go` pins both.

Ceph monitoring may enter the provider-neutral pool-health envelope only from
the native cluster health state and native health-check map. It preserves check
codes, severity, and summaries in deterministic order. A cluster-level
`HEALTH_WARN` or `HEALTH_ERR` does not identify a failed OSD or disk unless the
provider supplies that more specific evidence.

`internal/truenas/client_api_shapes_test.go`,
`internal/monitoring/truenas_poller_test.go`, and
`internal/monitoring/ceph_test.go` are the focused collection and identity
proofs.

### Cluster-endpoint discovery policy stays off the resolver on repeat polls

The cluster-endpoint discovery-policy check (`clusterEndpointRuntimeURL` →
`clusterEndpointAllowedByDiscoveryPolicy` in
`internal/monitoring/monitor_cluster_helpers.go`) is a function of
configuration, not poll state, and must not generate per-poll DNS load. It
resolves hostname endpoints through the process-global cached resolver that
`pkg/tlsutil` dials with (`tlsutil.LookupHostCached`), never through a bare
`net.LookupIP`. That gives the policy and the connection one DNS view, and
because the resolver caches lookup failures as well as answers, repeat poll
cycles cost a cache hit rather than a query — roughly one query per endpoint
host per DNS cache refresh, whose interval operators set through
`DNS_CACHE_TIMEOUT`. There is deliberately no second, policy-level verdict
cache: it would buy nothing on top of the resolver cache, would make the
verdict trail the configuration, and would freeze a fail-open
resolution-failed verdict in place for the length of its window. The effective
default policy — the `NormalizeDiscoveryConfig`-injected link-local blocklist
`169.254.0.0/16` — is therefore enforced against resolved addresses too, so a
hostname endpoint pointed into the link-local range is rejected rather than
allowed through unresolved. Literal-IP endpoints are still evaluated without
any resolution.

SSH-based collectors in the same runtime follow the equivalent rule for
process spawning, and only escalate for work that actually ran. `knownhosts`
caches keyscan failures with doubling backoff instead of re-executing
`ssh-keyscan` every cycle, and reports a suppressed call as
`ErrKeyscanSuppressed` so callers can tell it apart from a refusal. The
temperature collector backs off per host after failed SSH collection instead of
re-running its two SSH probes every 10-second cycle, but leaves its backoff
untouched when the host key scan was suppressed (no ssh was executed) and holds
it at the floor when the collection deadline expired rather than compounding on
evidence about Pulse's own budget rather than the host. Both backoffs decay: a
failure whose retry deadline passed more than one window ago restarts at the
floor instead of resuming the ceiling. Neither backoff may be a trap the
operator cannot leave: `TemperatureCollector.ResetSSHFailures` clears both
maps, and it is triggered both from the per-cycle key change check (replacing
the temperature SSH key on disk) and from every system-settings save
(`Monitor.ResetSSHFailureBackoff`, pushed into each live tenant monitor by the
settings API), so repairing the key — or saving settings after repairing SSH
access any other way — is retried on the next cycle rather than after a window
that has compounded to fifteen minutes.
The shared socket dialler also tries the usable addresses in that one cached
answer rather than discarding all but the first. It preserves answer order,
filters explicit IPv4/IPv6 networks and deduplicates equivalent literals.
Serial socket attempts share a ten-second ceiling or the earlier caller
deadline; each attempt reserves a share for the remaining addresses.
Cancellation stops later attempts and closes a connection that arrives after
the caller ends. This is TCP/socket establishment, not HTTP replay, a change
to discovery admission, new DNS resolution or a relaxation of TLS/authority.
`pkg/tlsutil/dns_fallback_test.go` covers actual second-address sockets, retained
cache reuse, caller budgets, cancellation, family selection and pinned HTTPS.
The existing macOS private-address subprocess route is preserved; Linux socket
controls do not establish native macOS/VPN acceptance.

`internal/monitoring/issue1638_dns_cache_test.go` is the registered proof that
repeat polls stay on the DNS cache, that the link-local blocklist still rejects
hostname endpoints resolving into it, and that the SSH backoffs suppress,
decay, and reset as described.

### Datacenter storage node restriction bounds the per-node storage surface

Proxmox's per-node storage endpoint (`GET /nodes/{node}/storage`) is not a view
of what that node can use. It returns every storage in the datacenter
configuration, and reports the ones the node is excluded from with
`enabled:0`/`active:0` rather than omitting them. The canonical storage poller
must therefore treat the datacenter `nodes` restriction, not the per-node
enabled/active flags, as the authority on which node a storage belongs to.

In `internal/monitoring/monitor_polling_storage.go`, `pollStorageWithNodes`
already reads the datacenter configuration once through `GetAllStorage` before
fanning out per node. When a storage name is present in that configuration and
its `nodes` field parses to a non-empty set (`parseClusterStorageNodes`) that
does not contain the node currently being polled, the per-node row is dropped
and never becomes a `models.Storage`. Node-name matching is case-insensitive
and whitespace-tolerant, matching how node identity is compared elsewhere in
the poller.

The restriction is the only permitted reason to drop a row here. A storage with
no `nodes` restriction stays visible on every node it is reported from,
including when it is disabled everywhere — a datacenter-wide disabled storage
must still surface as a `disabled` entry rather than disappearing, because
disappearing would hide a real misconfiguration. Because the shared-storage
aggregation derives `Nodes`, `NodeIDs`, and `NodeCount` from the surviving
per-node rows, honouring the restriction at ingest is also what keeps a
restricted shared storage from claiming cluster members that cannot mount it.
The cluster-only synthesis path further down already derives its node list from
the same restriction, so both paths now agree.

`internal/monitoring/monitor_additional_test.go` is the registered proof
(`Issue1645*`) that a restricted storage is dropped on excluded nodes, that an
unrestricted storage still appears on every node, that a globally disabled
unrestricted storage still surfaces as disabled, and that the shared-storage
node list excludes non-member nodes.

### Linked host evidence enriches provider-owned ZFS pools

The host collector supplements mounted filesystem facts with a bounded,
read-only `zfs list` query for filesystems and zvols under already-discovered
pools. Authenticated report ingest validates that optional evidence and stores
it on the canonical host model. During Proxmox storage polling, a node's linked
host dataset evidence is copied onto the matching provider-owned ZFS pool;
provider health, scan, device, and error fields remain authoritative. When the
provider cannot return pool detail, monitoring may synthesize only a minimal
`UNKNOWN` pool so valid dataset evidence is still inspectable.
### Guest identity admission coalesces owned JSON snapshots

`persistGuestIdentity` calls `GuestMetadataStore.RememberIdentity`, which merges
only the last-known name/type under the store lock, preserves operator URLs,
descriptions, tags and notes, and never downgrades OCI. Admission is visible
before return. `SetAsync` deep-copies its full replacement at admission as well;
caller mutation and goroutine scheduling cannot reverse accepted update order.

One lazy store-owned writer snapshots the complete metadata map. Filesystem I/O
does not hold the memory lock, so changes admitted during that I/O are coalesced
into the next complete snapshot. A separate write mutex serializes it with
synchronous Set/Delete/ReplaceAll/UpdateAll and legacy migration; an older
snapshot cannot resurrect a deleted or migrated entry or overwrite an operator
edit. The existing tenant-local JSON shape, private file permissions and atomic
temporary-file/rename path stay unchanged. A failed write is reported and not
spin-retried; a later ordinary identity poll can retry the unpersisted state.
An accepted guest-key migration retains its operator fields under the new key
in memory even when persistence fails. Atomic persistence keeps the previous
successful disk snapshot, and the existing writer records and retries the
retained state on an ordinary update. A failed write cannot restore an obsolete
in-memory key after the guest's new coordinates have been admitted.

`WaitForPendingWrites` reports quiescence, not durable success. `Close` seals
mutation admission and observes the background writer and any admitted
synchronous write within the unchanged caller budget. Timeout or final
persistence failure remains an error. Monitor Stop logs that failure instead of
claiming a clean stop; tenant removal retains the runtime owner and lifecycle
guard, and organisation deletion refuses to remove its directory. An unfinished
monitoring loop likewise prevents deletion. Shutdown does not cancel an admitted
filesystem write or silently discard metadata to obtain a timely result. A later
explicit close may retry a completed failed snapshot after filesystem repair;
admission stays sealed, successful close is idempotent, and no close retries
itself or overlaps a still-running attempt.

`internal/config/guest_writer_control_test.go` exercises blocked snapshots at
25/100 updates and caller-copy ownership. `guest_metadata_writer_test.go` covers
ordered replacement, synchronous mutations, field-scoped identity, failure and
explicit recovery, bounded close with blocked synchronous/asynchronous I/O,
closed admission, real filesystem permissions and tenant separation. Existing
ten-second drain and cleanup controls remain unchanged. Monitoring shutdown
controls retain blocked writer/loop ownership; the real organisation-delete
handler returns its safe failure before offboarding or directory removal.
These are source controls, not an attribution of an earlier drain timeout,
native CPU/RSS relief or all-path guest-agent recovery.

### Monitoring projects canonical resource policy into alert evaluation

The monitoring-owned operator-intent adapter projects `monitoringMode` and
`lifecycleState` together with effective one-shot/recurring maintenance timing
and the legacy compatibility
boolean. It still resolves source-native references through canonical resource
identity before reading the store and fails open on missing, ambiguous, or
errored identity lookup. Monitoring does not reinterpret provider ownership or
invent lifecycle state; Alerts owns signal suppression and unified resources
owns persistence. `internal/monitoring/monitor_alert_intent_test.go` and the
alerts intent-policy proof pin this adapter boundary.

The monitor adapter asks the registry for only the canonical resource ID when
resolving an alert reference. Exact IDs, superseded IDs, source IDs and
canonical aliases retain their precedence and ambiguous aliases remain
unresolved. This avoids cloning a full resource for every policy lookup while
keeping alert intent tied to the same identity rules as resource reads.

Alert intent resolves references and ancestors through the read state Patrol
resolves through and the resources API seeds from: the published registry with
saved hosts that have not reported since a restart overlaid
(`operatorIntentIdentity`). A saved host exists only there, so against the
published registry alone its alerts (`agent:<host ID>`) read their literal
reference after a restart and missed intent set on the host, and, when an
operator link joins it to a live resource, intent set on the link primary that
the registry now resolves the saved host to. Alerts resolve under the alert
manager's lock, once per metric check, so the overlay is kept for the published
registry generation it was built from. Publication builds it before alert
evaluation, outside that lock; a lookup that races ahead of publication, or the
first one after leaving mock mode, builds it under the lock. A generation is
the registry the adapter publishes and when it last changed
(`MonitorAdapter.Generation`), since two publications can share a
`LastRebuiltAt`. A finished build is stored only if that generation is still
the published one and the mock-mode epoch it started in is current, checked and
stored inside the mock-mode fence, so a build that overlapped a mode switch is
discarded. A lookup made while mock mode is on drops the stored overlay; a
switch to mock and back with no lookup in between keeps it, since it still
describes the published generation. A saved-host change that arrives without a
new generation shows on the next one. Mock mode carries no saved hosts: lookups
then resolve against the published registry as before.
`monitor_alert_intent_test.go`
(`TestOperatorIntentIdentityKeepsOneOverlayPerGeneration`,
`TestOperatorIntentReachesSavedHostAlertsAfterRestart`) pins the reuse, a
generation sharing or preceding the last one's timestamp, the mock switch, a
rebuild made from inside an admitted fence call, and an unlinked saved host's
intent reaching its offline alert.
`internal/monitoring/issue1913_host_continuity_test.go`
(`TestManualLinkToSupplementalGuestHoldsWithAndWithoutContinuity`) pins a saved
agent's offline alert reading maintenance set on the guest it is linked into,
and `monitor_host_agent_removal_lifecycle_test.go`
(`TestHostAgentRemovalLifecycleHonorsSavedHostIntentAfterRestart`) pins the
restarted monitor's own host-offline check raising no alert for a saved host
the operator marked intentionally offline.

Alert restore precedes resource-store attachment during startup. Once the
adapter attaches the persisted operator-policy resolver, monitoring asks Alerts
to reconcile the restored set and refreshes shared alert state if it changed.
Resource publication repeats reconciliation before evaluation and shared-state
export, because native aliases may only become resolvable after the first
observation populates the canonical registry. Persisted agent policy applies
to those restored native-ID alerts even when the initial registry was empty.
Muted or retired resources cannot retain active alerts merely because policy
was unavailable during restore. The resolution survives a subsequent restart
and preserves unaffected alerts. This ordering is pinned by
`internal/monitoring/monitor_alert_restore_test.go`.

`internal/maintenancesentinel/` is monitoring-owned post-maintenance assurance.
Its bounded sweep derives every concrete one-shot or recurring occurrence that
ended in the seven-day lookback, de-duplicates on canonical resource plus exact
occurrence end, and writes one maintenance-verification report and timeline
record per occurrence. Restart therefore backfills recent missed recurrences
without mutable scheduler state or duplicate reports; ancient windows remain
out of scope. `internal/maintenancesentinel/sentinel_test.go` and
`verification_test.go` are the focused proof.

### Agent privilege profile is descriptive model state

Host reports may carry an agent-authored privilege profile (effective root,
service user, active smartctl/pct helpers). Ingest copies it verbatim into
`models.Host.AgentPrivilege` (trimming the user), state deep-copy isolates it
(`cloneHost`), the frontend host projection clones it, and the agent fleet
doctor surfaces it as the dedicated descriptive `privilege` field rather than
a health reason. A report without the block yields nil — the server never
invents a profile — and a non-root profile must never degrade agent health on
that evidence alone. Proofs:
`pkg/agents/host/report_test.go` (`TestAgentInfoPrivilegeStatusRoundTrip`),
`internal/monitoring/monitor_host_agents_test.go`
(`TestApplyHostReportCarriesAgentPrivilegeProfile`),
`internal/models/deepcopy_test.go` (`TestCloneHostIsolatesAgentPrivilege`),
`internal/monitoring/agent_fleet_doctor_test.go`
(`TestAgentFleetDiagnosticsSurfacesPrivilegeProfileWithoutDegradingHealth`).

### Guest Docker command dispatch is single-flight, context-honest, and node-breakered

The Proxmox guest Docker socket probe and inventory dispatcher in
`internal/monitoring/docker_detection.go` owns dispatch discipline, not just
result caching (minipc probe-storm incident, 2026-08-20):

- **Single-flight per guest.** A dispatched probe or inventory command takes
  an in-flight claim keyed by container ID (inventory under an
  `inventory:` prefix). Overlapping poll cycles skip claimed guests, so a
  slow `pct exec` can never be stacked with identical copies of itself. A
  completed command (success or genuine failure) releases the claim; an
  abandoned one (the wait ended with a context error, so the agent may
  still be executing) holds it for a 2-minute window.
- **No dispatch under a dead context.** `CheckContainersForDocker` and
  `CollectProxmoxGuestDockerInventory` bail out before dispatching when the
  enrichment context has already expired, preserving previous Docker
  status; the parallel prober also re-checks the context deterministically
  after semaphore acquisition.
- **Abandonment is not evidence.** Context-canceled/deadline results feed
  neither the per-guest failure backoff nor the node breaker — they say
  nothing about the guest or node.
- **Per-node circuit breaker.** Three consecutive completed command
  failures on one node suspend all guest Docker command dispatch to that
  node on the existing 1m→30m backoff schedule (a node-level stall such as
  NFS flapping fails every guest, including newly appearing ones); any
  completed success closes the breaker. Checker reconfiguration resets
  claims, streaks, and breakers.

Proofs in `internal/monitoring/monitor_docker_test.go`:
`TestCheckContainersForDocker_InFlightProbeNotReissued`,
`TestCheckContainersForDocker_ExpiredContextDoesNotDispatch`,
`TestCheckContainersForDocker_AbandonedProbeHoldsClaimWithoutFailure`,
`TestCheckContainersForDocker_NodeCircuitBreaker`,
`TestCollectProxmoxGuestDockerInventory_InFlightAndExpiredContext`.

### Docker storage inventory is decoupled from live telemetry

The Docker / Podman agent keeps container liveness and running-container stats
on the configured report cadence, but Docker's full verbose `DiskUsage`
(`system df`) walk is a separate 15-minute inventory. That scan traverses
container layers, images, volumes, and build cache and can saturate appliance
daemons such as Synology DSM when many stopped containers exist. It runs once
per refresh window with no immediate transient retry, preserves the last good
aggregate across a failed refresh, and suppresses a failed cold-start scan
until the next window instead of starting it again on every 30-second report
(#1729). Live image-list requests also leave Docker's optional `shared-size`
calculation disabled; image IDs, tags, and digests remain fresh each report,
while shared-layer bytes and container counts come from the cached storage
snapshot. `TestBuildReportSynologySizedInventoryBoundsStorageComputations`
qualifies two live cycles over the reported 47-container/4-running inventory
shape and pins one full storage walk with no additional shared-size request.
`TestCollectStorageUsageDecouplesFullDaemonScanFromLiveTelemetry` and
`TestCollectStorageUsageThrottlesInitialTransientFailureWithoutRetry` pin the
cadence, stale-result continuity, and no-retry boundary.

### External dead-man proves canonical loop liveness across restarts

`internal/monitoring/deadman.go` owns the monitoring side of the external
watchdog. A separate worker emits one success signal per minute, while a
15-second marker written only by the canonical `Monitor.Start` select loop
proves that the polling scheduler itself is still progressing. A stale marker
causes the worker to send the provider's `/fail` signal and raise a critical
system alert; the worker's own timer can never count as monitoring progress.
Every DNS answer is compared with all Pulse host interface addresses before
dialing, in addition to the loopback, unspecified, and link-local exclusions.
Local-interface enumeration failure rejects the signal rather than weakening
the different-host guarantee, so a hostname or LAN address that resolves back
to Pulse cannot masquerade as an external watchdog.

The runtime persists a privacy-minimized `alerts/deadman-state.json` record
through fsync, atomic replacement, and directory sync. It stores endpoint
fingerprint and timing only, never the ping URL. On startup, a gap of at least
two minutes for the same configured endpoint is reported in the first healthy
POST and recorded through the alerts-owned system lifecycle, distinguishing a
clean stop from an unexpected one. Stop and configuration changes cancel an
in-flight request before durable stop or replacement state is published, so a
revoked endpoint receives no trailing heartbeat and a removed destination
becomes disabled immediately. State corruption and write failure are visible
system conditions rather than silent loss of future outage evidence.

### Agent removal keeps credential truth continuous across restart

Host-agent, Docker-host, and Kubernetes-cluster removal may clean up a
dedicated API token only through the shared monitoring revocation boundary.
That boundary holds the global configuration lock across mutation and
persistence, snapshots the complete prior inventory, and restores both the
records and legacy primary-token projection if the reduced inventory cannot
commit. Resource tombstones remain authoritative when credential persistence
fails, but the token stays consistently active instead of disappearing only
from the live process and silently returning after restart. Success and
forced-write-failure coverage lives in
`internal/monitoring/monitor_host_agent_removal_lifecycle_test.go`.
Collector self-uninstall is stricter than operator removal: while holding the
host lifecycle write lock it verifies the exact live host/token binding,
persists the removal tombstone, and durably revokes a dedicated credential
before changing live resource state. Failure to load or write continuity, an
unavailable credential persister, or failure to persist the reduced token
inventory returns an error and retains the live host and retry credential; a
shared legacy token remains active only for its other live resources. The
production Router regression in
`internal/api/host_agent_removal_lifecycle_integration_test.go` forces both
continuity-journal and credential-inventory writes to fail, restarts the
server, retries with the exact bearer, and then proves removal plus old-secret
rejection survive a second restart.
Collector self-uninstall refuses a token that is still referenced by another
live host. That legacy shared authority must be rotated or separated before
the server can return teardown-authorizing success; preserving the bearer for
the other host is not equivalent to revoking the uninstalling collector's
credential.

### Escalation callbacks preserve exact routing intent

The monitoring callback resolves the configured escalation level and forwards
exact logical destination IDs to notification delivery when present. Legacy
levels continue to route by channel, including Apprise; this compatibility path
must not silently skip a supported destination. Monitoring broadcasts the
escalated alert after dispatch but does not reinterpret destination identity,
retry semantics, acknowledgement, or the critical-repeat cadence.

### External alert callbacks are ready before monitor publication

`monitoring.New` installs firing, resolution, AI, and escalation callback slots
before returning a monitor that API handlers can publish. The asynchronous
`Monitor.Start` loop adds lifecycle projection handling and rewires escalation
with its WebSocket hub, but it is not the first notification-delivery boundary.
Before those callbacks are installed, the constructor restores saved email,
Apprise, and webhook destinations, then activates the queue processor. A due
persisted notification cannot be consumed against an incomplete destination
list by the queue's five-second ticker during a slow startup. Activation wakes
the backlog only after the destination load stage completes; it does not
change alert admission, agent identity, or provider-delivery policy.
This closes the startup interval in which an immediately reconnecting agent
could create and persist an active warning while the outbound callback was
still nil. The constructor-time callbacks use the already loaded alert and
destination configuration; runtime rewiring replaces single callback slots and
does not duplicate delivery. `TestNewMonitorRoutesStartupCustomSensorWarningBeforeStart`
in `internal/monitoring/monitor_host_agents_test.go` proves a first custom-sensor
warning reaches the external path before `Start` runs.
`TestMonitorStartupRetainsQueuedAgentDiskAlertUntilDestinationRestored` in
`internal/monitoring/physical_disk_roundtrip_test.go` verifies that the real
constructor activates a persisted disk-alert job without policy-cancelling it.
The test uses a deliberately blocked local URL and does not claim provider
acceptance or installed delivery.

### Proxmox node unavailability is not credential evidence

The Proxmox client treats HTTP 595 from a node-scoped API path as a resource
availability failure. This is the pveproxy response when a cluster member is
offline or unreachable, so it remains available at debug level without
emitting the repeated authentication warning used for cluster-scoped 595,
401, and 403 responses. Returned errors retain their compatibility shape;
`pkg/proxmox/client_request_test.go` pins both sides of the log classification
(#1794).

### Monitoring supplies fail-open shared-system alert identity

Host-agent availability evaluation may attach alerts-owned shared-system
context only from monitoring's canonical reciprocal PVE node-agent link. The
host `linkedNodeId` and node `linkedAgentId` must name each other exactly, and
the matched node must supply one unambiguous non-empty PVE instance. Missing,
one-sided, or conflicting evidence returns no correlation. Monitoring does not
use hostnames, resource-path prefixes, or observation timing to manufacture a
relationship, and it does not merge alert lifecycle state.

This adapter is read-only and bounded to the host evaluation snapshot. Alerts
owns the correlation wire type and presentation semantics; monitoring owns the
identity evidence that permits the host signal to join that system.
`internal/monitoring/alert_correlation_test.go` pins reciprocal-link admission
and every fail-open case.

### Proxmox update polling preserves evidence quality

The 30-minute node package poll treats one successful response, including an
empty package list, as `checked` and records its completion time. A failed read
must remain an error through the cluster-client fallback boundary. Monitoring
maps it to a fixed privacy-safe reason and reports `unavailable` when no prior
success exists or `stale` with the last successful count and timestamp when a
cache entry exists. An offline node is `not_checked` with `node_offline`.

Failures never refresh the successful timestamp or convert permission and
reachability errors into zero. Cached successful observations remain checked
until their normal refresh boundary, and a failed refresh is retried without
discarding the cache. `node_pending_updates_evidence_test.go` and the cluster
client pending-update tests pin zero, permission, reachability, stale-cache,
and offline behavior.

### Summary-only container collection remains explicit through ingestion

Monitoring copies the Docker report's optional `collectionMode` into the
stored host model and frontend projection without upgrading its authority.
`typed-helper-summary` reports may carry the bounded container summaries and
ordinary host metrics, but missing stats, storage, images, networks, volumes,
Swarm, and update evidence remain absent rather than being reconstructed from
older or adjacent observations. Model, monitor, and unified-resource tests pin
the additive field through the ingestion path.

### Asymmetric bridge evidence regression coverage

`TestApplyHostReportBridgeIdentity` verifies two consecutive reports through
host ingestion and checks both host-to-node and reciprocal node-to-agent links.
Custom management bridges, private IPv4 and ULA addresses retain association
when names differ and the provider endpoint is DNS-based. A Docker or generated
bridge on either side alone must not supply inferred identity: testing only
symmetric bridges would miss removal of filtering from one inventory.
This proof does not establish safety for arbitrarily renamed Docker bridges
or repair persisted links, and is not reporter or installed-release validation.

### Reload handshake cancellation

`ReloadableMonitor.Reload` rejects calls before `Start` and returns the server
parent context error if shutdown cancels either request submission or the wait
for completion. The reply remains buffered so completion of an already accepted
reload does not block a watcher whose caller has left. Ordinary reload success
and configuration errors still propagate unchanged. This does not cancel an
in-flight `doReload`, bound its teardown time, or change monitor replacement and
router refresh ordering. `Stop` alone is not parent-context cancellation.

`internal/monitoring/reload_test.go` exercises cancellation before submission,
without a request consumer, and after acceptance, plus late completion and
success/error propagation using channel-only fixtures without starting monitors
or storage. This is shutdown handshake proof, not installed notification receipt
or full server reload acceptance.

### Stale escalation callback snapshots are not delivery authority

`handleAlertEscalated` obtains current occurrence/policy admission from the
alert manager before dispatch. `monitor_escalation_stale_test.go` invokes the
callback after disablement, inactive activation, recovery, recurrence,
and acknowledgement, requiring no queued notification. Existing routing,
quiet-hours and cooldown tests now use real active manager occurrences rather
than fabricated inactive alerts. This is callback-boundary proof, not a claim
that already admitted or provider-accepted deliveries can be recalled.

### Physical-disk provider identity round trip

Disk ingest and readback follow the [unified source-identity contract](unified-resources.md#physical-disk-provider-identity-round-trip),
including its source controls and qualification limits.

### Unified metric replay guard bounds registry-rebuild writes

Read-side registry rebuilds (/api/state, websocket hydrate) re-run the unified
metric sync as often as every two seconds. Every unified sync (agent, VM,
storage, and app-container) now timestamps its samples with the source
observation time (`unifiedResourceObservedAt`/`unifiedMetricObservedAt`), not the
rebuild wall clock. Before this change each rebuild re-issued the same rows as
SQLite UPDATEs and committed a fresh transaction for no new data, churning the
WAL and driving the disk-write amplification in #1966. Monitoring tracks the
last sample handed to the store per (resource type, resource id, metric type)
series and drops exact timestamp+value repeats before the batch is enqueued; a
corrected value at the same observation time is still written, so a real change
is never masked. The guard is bounded so a long-lived monitor with churning
resource IDs cannot pin memory. Proxmox and libvirt VMs have native history
writers and are excluded from the unified VM sync so the same series is not
written twice. Physical-disk SMART history follows the same rule: the unified
physical-disk sync anchors both the in-memory chart point and the persisted
SMART batch to the resource's source observation time and drops exact replays
through the same guard, so a TrueNAS/Unraid disk without a native SMART writer
does not re-commit its attributes on every registry rebuild. Focused proof lives
in
`internal/monitoring/monitor_host_agents_test.go`
(`TestDedupeUnifiedMetricWritesDropsExactReplays`,
`TestSyncUnifiedPhysicalDiskMetricsUsesSourceObservationTimeAcrossRegistryRebuilds`)
and `internal/monitoring/monitor_polling_test.go`
(`TestSyncUnifiedAppContainerMetricsUsesSourceObservationTimeAcrossRegistryRebuilds`,
`TestSyncUnifiedVMMetricsUsesSourceObservationTimeAcrossRegistryRebuilds`).

### Unified metric sync commits one batch per pass

The five unified syncs run serially and each previously waited for its own
SQLite commit, so one registry rebuild that carried new data for several
resource types opened one transaction per type and committed the same WAL
repeatedly. `syncAllUnifiedMetrics` now collects the surviving writes of the
agent, VM, storage and app-container syncs through a batch sink and commits them
in a single `WriteBatchBounded` transaction. Each sync still runs its own
per-series replay guard first, so the change only moves the transaction
boundary: the same series, values and source observation times are written.
Standalone callers and tests still pass no sink and keep the immediate,
read-your-writes write. The physical-disk SMART path keeps its own transaction
because it writes directly. Focused proof lives in
`internal/monitoring/monitor_polling_test.go`
(`TestSyncUnifiedStorageMetricsDefersWritesToBatchSink`).

### Docker image update comparison accepts every local RepoDigest

Docker records every RepoDigest an image is known by, and the digest for a
tag's manifest is not always the first entry. Update detection now compares the
whole local RepoDigest set against the registry's resolved and index digests,
so a current image whose manifest appears under a later RepoDigest is not
reported as outdated. `TestRegistryChecker_MultipleLocalRepoDigestsSuppressFalseUpdate`,
`TestAgent_getImageRepoDigests_MultipleDigestsForOneImage` and the multi-digest
`TestRegistryChecker_DigestsDiffer` cases pin the comparison. This is synthetic
registry-transport evidence, not reporter acceptance.

### A failed image inspect leaves the update status unknown

Image inspect shares the 15-second per-container collection budget with the
container inspect (sized when disk metrics are on), the one-shot stats call and
filesystem collection, so it can time out on a busy host. It previously
returned an empty local digest and no platform, and comparing that empty
digest reported `UpdateAvailable: false` with no error, which the alerts
runtime took as an affirmative clear of a pending update (#2353).
`localImageDigestsForUpdateCheck` now falls back to the last successful
inspect of the same image ID, kept by the registry checker and pruned to the
images of the collected containers; an image ID names immutable content. With
no earlier inspect the container carries no update status that cycle, which
the server treats as unknown. A locally built image (inspect succeeds, no
RepoDigests) keeps its existing status. `getImageRepoDigests` keeps its
signature for its other callers.
`TestCollectContainerKeepsUpdateStatusWhenImageInspectFails` pins both the
fallback and the unknown case, with the registry answer seeded offline; on the
previous code it reproduces the false no-update status.

### Empty live Docker inventory is not removal evidence

Agent inventory starts empty after a monitor restart/reload and grows as each
agent reports. Alert-state synchronisation must not interpret an absent host
as removed: router bootstrap could clear a restored update occurrence before
its first report, discarding the pending age that the alert manager restored.
`pruneStaleDockerAlerts` now uses membership-based removal only for the complete
mock fixture estate, retaining the existing mode fence. Explicit live removal
still owns `finalizeDockerHostRemoval` and clears that host's alerts, including
when it has not reported in this process. Ordinary inactivity retention remains.

`TestMonitor_PruneDockerAlerts_Extra` covers empty live inventory and explicit
removal. `TestDockerUpdateCollectorAdmissionPreservesOccurrence` joins the real
collector, compressed authenticated HTTPS report, admission/order checks,
restored 24/48-hour occurrence and retention/recovery. Its aged JSON checkpoint
and daemon/registry cache are synthetic; the legacy no-current-digest wire
control is labelled separately from collector output. It establishes neither
the reporter's installed cause nor a naturally elapsed day or release use.

### Forked host reports cannot bypass a base-identity removal block

Host report admission resolves identity before consulting removal blocks, so a
report that forked onto a derived `<base>-<hex>` identity could bypass a block
keyed on the base machine identity and be silently admitted. Monitoring now
consults the base identity as well when the presenting token had no prior
binding: a fresh install token clears the block and heals to the base identity,
and a token already bound to its own derived identity is left alone (#1753).
Focused proof lives in `internal/monitoring/monitor_host_agents_test.go`.

### Host-agent endurance evidence cannot raise a disk's remaining life

The physical-disk merge lets the linked host agent's SMART endurance counter
correct the Proxmox inventory, because the agent's `PercentageUsed` is more
precise than the provider's coarse `wearout`. It must not, however, make a disk
look healthier than Proxmox already reports. An intermittent agent reading
(for example an NVMe endurance log that briefly reports `PercentageUsed` 0)
previously raised the merged remaining life to 100, which resolved a genuine
low-life alert and let it re-fire on the next physical-disk poll with a fresh
resolved notification, repeating roughly every poll interval (#2112). The merge
now takes the agent value when the Proxmox value is unreported (`-1`) or when
the agent value is lower (more pessimistic); it never raises a reported
remaining-life value. `PercentageUsed` and the other merged SMART attributes are
still recorded. Focused proof lives in
`internal/monitoring/physical_disk_roundtrip_test.go`
(`TestMergeHostAgentSMARTIntoDisks_AgentWearoutDoesNotHideLowPVELife` and
`TestMergeHostAgentSMARTIntoDisks_AgentWearoutFillsUnreportedPVELife`). This is
synthetic merge evidence, not reporter acceptance or hardware confirmation.

### TrueNAS system temperature rejects a synthesised CPU aggregate

TrueNAS reports the `cputemp` series through a bare `cpu` legend entry, which
`canonicalSystemTemperatureKey` maps to the canonical `cpu_package` key. On
SCALE 25.10.7 running AMD hardware that aggregate has been observed at roughly
1.5x the hottest per-core reading (for example an 88C aggregate against a 59C
core max, with the cores matching `k10temp`/`Tctl`). Monitoring previously
trusted `cpu_package` unconditionally in `maxTrueNASSystemTemperature`, so the
false-high value reached the host temperature and fired a configured 80C
threshold alert at a real ~60C. The selector now compares `cpu_package` with
the hottest `cpu*` per-core entry and falls back to the per-core max only when
the package value exceeds it by more than 20%. A real package sensor stays
within a few degrees of the hottest core, so the margin retains genuine
package readings while discarding the synthesised aggregate. Package-only
payloads and per-core-only payloads keep their existing behaviour. Focused
proof lives in `internal/truenas/provider_test.go`
(`TestMaxTrueNASSystemTemperaturePrefersCoresOverSynthesisedAggregate`). This
is source-level proof, not reporter acceptance or appliance confirmation
(#2122).

### Recovery callbacks retain the resolved occurrence

`wireExternalAlertCallbacks` binds the occurrence-qualified recovery callback,
not the legacy ID-only callback. The monitor uses its immutable alert/start/end
snapshot for AI recovery and for exact notification cancellation, so a delayed
recovery cannot remove a newer firing admitted under the same reusable ID.
AI receives a separate clone. An ID-only websocket removal is omitted when the
manager already exposes that ID as active; the regular state stream continues
to own current alert truth. The legacy local ID adapter resolves a snapshot once
and fails closed when no resolved record exists; it is not the live delivery
binding. No browser payload, retention or notification routing contract changes.

`monitor_alert_handling_test.go` and
`monitor_notification_startup_test.go` covers a delayed ID-adapter cancellation
with queue restart/readback, and a real PBS breach/recovery/new breach held at
the callback boundary. Both require the replacement firing's exact ID/start to
reach a loopback HTTP destination with one sent row and one cancelled old row.
No clock, reducer map or configured cooldown is altered. This establishes local
source-connected delivery, not the outstanding natural flapping cooldown check,
a native appliance result or containing stable availability.

Recovery eligibility is destination-receipt-owned, not the cancellation helper's
pending-work count or the RAM cooldown marker. After partial delivery/restart,
a cancelled retry at one destination cannot suppress a recovery to another
that actually accepted the same occurrence. The connected
`TestMonitorDelayedPartialResolutionKeepsDestinationRecovery` retains a real
HTTP 200/503 split, reopens the persistent queue, and requires the old accepted
destination's recovery plus the new firing, with no recovery to the unannounced
destination. Quiet hours and disabled recovery controls still apply.

### Complete, unambiguous replication reads preserve the last useful observation

Both replication inventory and source-job status require the bounded HTTP body
through EOF and a successful close before publication. The complete envelope
must have one canonical `data` field, no error payload, valid UTF-8 and no
trailing JSON, duplicate or case-shadowed fields. Inventory requires unique,
nonempty job identities; a rejected inventory preserves the previous rows,
failure detail and original `LastPolled`, rather than replacing them with an
apparently successful empty prefix. Explicit complete empty arrays still remove
only that connection's jobs.

Rejected per-job status does not hide readable configuration or become a
successful sync. Object and array replies share validation and the existing
job/source binding; conflicting legacy identity, failure-count or outcome
aliases cannot conceal adverse evidence. Consistent native/legacy shapes,
string/numeric zero and unique additive metadata remain supported. Independent
jobs and connections retain their own observations, and a later ordinary read
can recover without a restart. Raw provider response bodies no longer enter
debug logs. Request scope, permissions, polling cadence and API shape are
unchanged.

`pkg/proxmox/replication_response_test.go` covers complete-body/close failures,
malformed/ambiguous envelopes and identities, privacy and compatible shapes.
`scripts/replication_read_acceptance.py` connects emitted client jobs to
`replication_read_acceptance_test.go` in monitoring and API, preserving the exact
five phases through ordinary conversion, scoped replacement and 25 handler
requests. Consuming browser verification uses those emitted HTTP payloads, not
hand-invented verdicts. Socket-free fixture transport is not wire/TLS or native
PVE acceptance, and this repair does not diagnose #2606's original interruption
or establish frozen-guest/backup recovery.

### Observed Proxmox replication sync outcome

Replication configuration is enriched only from the source node's per-job
status endpoint. A successful timestamp plus an explicitly valid zero failure
count and no failure/unknown-state evidence yields `lastSyncStatus: ok`;
error text, positive failures or an explicit failed state yields `error`,
including error-with-zero-failures. Missing, denied, malformed, never-synced,
invalid-count or unknown-state responses do not acquire success from defaults.
Job/source mismatches and ambiguous array identities are rejected before any
status fields merge. Single legacy rows without IDs retain endpoint binding;
a multi-row response requires exactly one matching job.

`replication_test.go` covers native field shapes, failure/unknown controls,
HTTP enrichment and identity. `monitor_full_coverage_test.go` carries the real
client through four normal fourteen-job/five-node polls into model/wire state,
with zero-count error, denied status, recovery and another-instance isolation.
The unchanged replication table consumes those explicit outcomes. These local
HTTP controls are not native restart recovery; issue #2606 remains separate.

### Next guest polls preserve correlated agent filesystem inventory

The previous-guest context carries disks and their original agent source time
from a manually correlated VM/container even when the standalone host row was
removed, and even if agent memory is unavailable. Standalone linked hosts use
the same source-owned eligibility; a current platform row cannot renew agent
disk evidence. The existing VM disk preference consumes this inventory without
issuing another guest-agent read. Empty replacements, expiry and withdrawal
remove eligibility; a new ordinary report restores it. Guest-read safety and
backup deferrals remain independent of usable Pulse-agent filesystem data.

`guest_linked_agent_disks_test.go` carries actual registry correlation, retained
links and automatic link hints through ordinary VM construction and canonical
projection. It covers disk-only evidence, source time, missing links, identical
VMIDs in separate instances, expiry/offline/empty/withdrawal/recovery and a
backup lock with zero filesystem calls. Sequential snapshot replacement pins
inventory removal/recovery without restarting the monitor. These are synthetic
source controls, not a native cause, thaw or installed recovery for #2616/#2619.

### Runtime-owned Proxmox replication observations — #2606

The ordinary PVE scheduler/provider/executor dispatches a bounded replication
read independently of `cluster/resources` availability, traditional guest
fallback and disabled guest-detail monitoring. Its ten-second context is
captured from the monitor runtime before dispatch, not from the short-lived
scheduled task. One read per registered instance/client coalesces overlapping
cycles; runtime replacement and retirement cancel/invalidate the claim. A
replacement client can recover on the next ordinary cycle without waiting for
an old read. Publication checks cancellation, the claim and the current client
under the same lock; a late obsolete completion cannot overwrite or clear a
newer result, including a successful empty inventory.

Only an explicitly decoded `data` array is a complete replication inventory.
Missing/null/malformed envelopes, cancellation (including after final status
body consumption), access/transport errors and unavailable endpoints preserve
the prior jobs and their original `LastPolled`. A complete successful empty
array removes only that instance's jobs. Individual uncanceled status-access
failures retain the existing unknown-outcome contract, not inferred success.
No wire fields, classification, permission contract or scheduler cadence change.

`monitor_pve_replication_lifecycle_test.go` exercises ordinary fallback/disabled
paths and canceled/unavailable versus complete empty reads; the owned-boundary
controls in `monitor_pve_replication_owner_test.go` cover coalescing, cycle/runtime
cancellation, replacement, retirement, late completion and ordinary recovery.
`pkg/proxmox/replication_lifecycle_test.go` binds envelope and cancellation
controls to the real HTTP client. Synthetic proofs do not establish #2606's
reported eleven-hour cause, native PVE recovery or published availability.

### Non-destructive PVE backup access guidance

Backup storage-list/content access warnings name the attempted endpoint and
point operators to the actual saved PVE connection, installed-version endpoint
permissions and both user/token scopes without disabling privilege separation.
Observed HTTP 401 is an authentication rejection; 403 is an access denial, not
proof of a particular missing role. Unknown errors and proxy bodies quoting
another status remain unconfirmed. The private HTTP error wrapper exposes only
its numeric wire status for this distinction, never its body or credentials.
Warnings and associated log guidance contain no unconditional ACL command,
assumed account, token ID, token secret or role prescription. Endpoint text is
bounded and control-quoted. Collection, inventory/error completeness, polling
cadence and setup permissions are unchanged.

`monitor_backup_poll_test.go` and `canonical_guardrails_test.go` retain the safe
saved-identity/manual-token contract. `pve_backup_warning_test.go` exercises
401/403/proxy errors through the real client and storage-list/content producer,
as well as unknown evidence and hostile endpoint text. The HTTP status helper
has wrapped-error/body-injection controls in `api_error_status_test.go`. These
are local source controls, not a native permission diagnosis or availability.

### Physical-disk chart metadata without invented History

`GetPhysicalDiskTemperatureCharts` preserves a currently collected physical
disk's identity, display name, node and instance even when its stored
temperature series is empty. A retained-only disk with no observed samples
does not introduce an empty chart entry. Neither current nor retained readings
create historical timestamps, duplicate a single stored point or pad a flat
series: stored sample values and times remain unchanged. This composes current
disk metadata with the sparse-history and retained-observation boundaries;
collection policy and native health claims are unchanged.

`TestDiskTemperatureChartsRetainOnlyObservedSamples` in
`internal/monitoring/monitor_metrics_slo_test.go` binds live-without-history
metadata, retained-without-history exclusion and original single-point
observations to the production chart path. These source controls do not
establish native temperature collection or installed History acceptance.

### Broadcast host coalescing honours operator splits

The websocket broadcast coalesced top-level host views with
`CoalescePresentationHostResources`, which applies no merge exclusions, while
the resources API's `ResourceRegistry.ListForPresentation` applies the
registry's. A host pair the operator split (unlink or report-merge) could
therefore be one broadcast row and two REST rows.
`Monitor.buildBroadcastFrontendStateFromSnapshotWithClock` now calls
`coalesceResourcesForPresentation`, which asks the view's read state for
`MonitorAdapter.CoalesceForPresentation` when its registry is store-backed
(the resource store's adapter, or a host-continuity overlay that loaded the
store's exclusions afresh), and otherwise the resource store's adapter: the
mock-mode view lists through a store-less registry that carries no operator
decisions. Both surfaces share `presentationExclusionFilter`. Polling, rebuild cadence,
alert evaluation and the broadcast payload shape are unchanged.
`TestBroadcastPresentationCoalesceHonoursMergeExclusions` compares broadcast
and REST row counts with and without the split, and
`TestBroadcastPresentationCoalescePrefersTheListingReadState` pins the
listing read state over an older store generation and the store's exclusions
for a mock-style view.

### Reconciled node-agent relink fold bookkeeping

Relink consumers preserve the [unified fold-bookkeeping contract](unified-resources.md#reconciled-node-agent-relink-fold-bookkeeping).

Mock-link lifecycle validation drives the owning ingest/rebuild boundary
after unlink and relink: read hydration intentionally reuses a registry
generation younger than two seconds. The decision takes effect in the next
loaded generation, not retroactively in the listing already served. The
fixture seed keeps default thresholds while the configured live adapter
retains its four-minute Proxmox threshold.

### Incomplete guest filesystem responses do not establish recovery

Filesystem record identity/filter fields (`name`, `type`, `mountpoint`, `disk`)
use the same exact canonical key rule as byte counters. Case-variant keys must
not exploit JSON's case-insensitive matching to overwrite a mount/device or
make a real volume appear special and silently removed. They reject that row
as incomplete regardless of field order or whether the canonical key is also
present. Exact JSON-escaped canonical keys and unrelated additive fields remain
compatible. `TestVMFilesystemMetadataRejectsNoncanonicalFields` and
`TestVMFilesystemMetadataPreservesCanonicalCompatibility` pin that boundary;
the connected completeness fixtures cover direct/cluster clients and both
collectors through held/expired evidence and genuine ordinary recovery.

`GetVMFSInfo` preserves valid peer rows from a malformed array but returns an
error with them, through both single and cluster clients. The surviving rows
cannot establish total guest capacity, usage, or removal of a rejected volume.
A completed response with invalid counters is not a command-completion failure:
serial admission, same-target lock checks and single-attempt guest requests
remain unchanged, and there is no failover/replay or new cooldown. Alternate
clients' contradictory or out-of-range counters enforce the same completeness
boundary before publishing a current aggregate.

Monitoring retains only the prior complete disk tuple, mount inventory and
original observation time within the existing age bound; otherwise disk usage
is unavailable. Partial responses do not create disk History, resolve genuine
filesystem alerts, or remove unknown volumes. Independent current CPU/memory
continues. A later complete ordinary inventory can still remove a volume and
publish an explicitly measured zero, append History and recover alerts.

`pkg/proxmox/guest_filesystem_completeness_test.go` and
`internal/monitoring/guest_filesystem_completeness_test.go` cover valid peers plus
malformed records, both client/collector paths, canonical and served state,
original age/expiry, History, breach continuity and complete removal/recovery.
These are synthetic source controls, not native thaw, the cause of #2619/#2439,
or installed/released acceptance.

### Node and agent links honour operator splits

An operator's split of a Proxmox node and its pulse-agent (resources API
unlink or report-merge) split the registry's rows but left the monitor's
own link (`Node.LinkedAgentID`, `Host.LinkedNodeID`) standing. What reads
that link kept treating the two machines as one: the physical disk poll
took the agent's SMART inventory as the node's disk fallback and applied the
agent's `--disk-exclude` patterns to the node's disks, `linkedHostForNode`
fed the node's disk sources, guests inherited the agent for discovery and
commands, deployment reported the node as already running an agent, service
discovery routed through it, and alert correlation tied the node to it.
`models.State` now holds such a link back on both sides through a
`models.NodeAgentSplitDecider` that `Monitor.SetResourceStore` installs from
the resource store's adapter. `resetStateLocked` installs it again when mock
mode replaces the state. Every link write applies it: `UpdateNodesForInstance`
before the merge key reads a node's link and again after the manual-intent
reconcile, `UpsertHost`, `LinkNodeToHostAgent`, and report ingest before it
applies the agent's LXC filesystems. A report that finds an agent's link
split, or ends its split manual intent, clears that agent's cached container
filesystem readings. A cached reading is also not shown for a node the
operator split from the agent that cached it
(`enrichContainerWithAgentLXCFilesystems`), whether or not a report cleared
it. A report uses the link `UpsertHost` kept when a split lands between
the report's own check and the state's store. The split's rules and its one record,
the resource store, are in the unified-resources contract. A registry
rebuild that cannot read the store's decisions carries the replaced
generation's, so the decider keeps reporting a split through a store outage
and the state does not link the pair again until the store has been read.
`Monitor.LinkHostAgent` first removes an older split of that pair from the
store, and records it again if the intent fails to persist, without replacing
a decision the operator recorded in between. A split recorded
after a manual link ends that manual intent on the agent's next report that
finds the split confirmed in the store. The alert manager's node coverage
for the agent changes on that agent's next checked report or offline
transition. Mock mode's fixture graph keeps its own links. Polling cadence,
link inference and the broadcast payload are unchanged. `TestOperatorSplitStopsMonitoringTreatingNodeAndAgentAsLinked`
checks each of those readers after a report-merge, against a control
exclusion that splits nothing, and `TestManualNodeLinkReplacesAnOperatorSplit`
covers the agents API's link after a split, a split after it, and a
resources API relink.

### Resource publication ownership correction (10 October 2026)

One refresh captures its store, binding generation, tenant and provider set once.
Snapshot-source ownership, records and consume-once changes use that selection;
changes are drained only after an admitted publication, into that pass's store.
Replaced or ended-epoch collections and skipped read refreshes leave changes
pending. An admitted old pass can finish its own history without consuming a
replacement provider's events. Poll-provider replacement also invalidates pending
collections. Provider callbacks run outside wiring, publication and adapter locks.
Read refreshes share ingest publication ordering, retain fresh-generation skips
and never queue behind a warm ingest; a cold read still builds its first registry.

Canonical unified snapshot reads also enter this same read-refresh boundary,
with a cheap fresh-store check before taking a state copy. Unattended inventory
and reporting consumers must not depend on a dashboard `/api/state` request to
publish polled VM data. Mock fixture projections and stores without read-refresh
support keep their existing read semantics. The refresh retains the existing
provider, tenant, wiring and mode fences and warm non-blocking publication gate.
`unified_inventory_read_refresh_test.go` covers headless VM publication and a
warm read contending with ingest publication.

`resource_publication_ownership_test.go` supplies deterministic channel barriers,
consume-once replacement-store controls, refused and skipped drains, read-provider
re-entry, and duplicate/loss assertions, alongside the original initialization,
tenant withdrawal, saved first-alert policy and joined-shutdown controls.

### Guest-cycle publication ownership (12 October 2026)

The production guest cycle stages diagnostic snapshots, alert evaluations,
backup-template eligibility and History until both VM/container collections are
ready. It admits publication only while its registered PVE client and cycle
remain current. Retirement, transport replacement and a new runtime cancel
outstanding guest cycles and order their state changes against any already
admitted publication; late inventories/details cannot recreate or erase the
replacement's guest readings. An interrupted inventory does not grant fallback.
One cycle per live binding owns collection until it completes, including when
its caller is cancelled. Another connection can still advance independently.
Before publishing accepted VM/container coordinates, that same owner migrates
their persisted guest metadata keys. Readers cannot observe a new node ahead
of its saved web URL, tags and notes. Rejected or replaced cycles do not migrate
metadata. Collector-local post-publication callbacks own diagnostics and alerts,
while metadata migration belongs to the complete accepted publication.

Deep-enrichment/caller-deadline expiry is distinct from explicit parent-cycle
cancellation: already collected authoritative inventory still publishes even
when local assembly outlasts the caller budget. No new read or fallback is
admitted after deadline expiry. An
accepted collection retains existing failed-node/grace rules, empty-inventory
handling and metric/source/age policy. Focused builder and collector entry points
retain their existing immediate effects. No guest operation lock, consumed-read
uncertainty, timeout, permission, TLS policy or automatic retry is relaxed.
Client replacement is not a guest reboot and does not erase accepted cache age
or OS-command safety state. Remote collection holds no publication lock; effects
run without the monitor state mutex held. The guest publication gate does not
claim ownership of node, storage, backup-job or native Docker-command collection.

`guest_poll_ownership_test.go`, `diagnostic_snapshots_test.go`,
`monitor_host_agents_test.go`, `proxmox_guest_snapshot_coherence_test.go` and
`proxmox_large_cluster_poll_budget_test.go` cover late replies on both enumeration
routes, cancellation/retirement/runtime/client changes, complete staged readings,
History and breach continuity, independent agents, single-flight/recovery,
retirement ordering and bounded-detail inventory. These are controlled source
proofs, not native restart, independent thaw, installed or release acceptance.

The canonical Docker-ingest source guard follows both guest enumeration routes
through `publishPVEGuests` to the canonical state update. Publication reconciliation
updates its former direct-call assertions without changing Docker ingestion or
the runtime ownership gate; the behavioral withdrawal controls remain required.

### Complete guest enumeration before publication (14 October 2026)

PVE `cluster/resources`, node `qemu` and node `lxc` enumeration consume and
close the whole existing 8 MiB-bounded HTTP response before admission. Only
HTTP 200 containing a complete UTF-8 JSON object with an explicit canonical
`data` array is authoritative inventory. Missing/null data, duplicate or
case-variant supported fields, conflicting errors, trailing values, incomplete
bodies, invalid rows and duplicate guest identities fail the whole enumeration;
no partial inventory or provider body is returned as a successful reading.
Cluster guest identities require a node and positive cluster-wide unique VMID.
Node lists require positive unique VMIDs; existing LXC string integers remain.
Scoped empty arrays, optional names/status/metrics, null counter presence,
templates, unique provider metadata and unfiltered non-guest resources remain
supported. Observation timestamps are assigned only to an admitted inventory.

A refused efficient enumeration can use the existing ordinary node fallback.
Failed node enumerations retain their earlier guest observations, History and
backup-template eligibility; a complete fallback can still remove deleted
guests and restore ordinary sampling. Complete cluster recovery returns to
its efficient route. This changes no TLS/pin, credentials, permissions,
polling cadence, guest-command/backup-lock checks, completion uncertainty,
publication ownership or grace-period rules.

`guest_inventory_response_test.go` covers complete-body and all-or-nothing
admission on all three real HTTP readers; `io_counters_test.go` covers
scoped-empty, extended fields, counter presence and receipt-time controls.
`diagnostic_snapshots_test.go` covers actual efficient/fallback collection,
owned publication, retained observation times, breach/History/template
continuity, independent-connection preservation and normal recovery/removal.
These controlled source proofs do not establish native thaw/restart recovery,
installed trust, report attribution or release availability.

Failed-node retention selects canonical guest membership but takes the unchanged
complete tuple from that connection's last accepted producer state. Enrichment's
partial previous-state projection is not an observation: it must not turn an
unavailable/retained memory reading into zero, lose CPU/disk usage or change the
original in-guest collection state. A node-scoped detached snapshot copies only
selected guests, not unrelated host disks or Kubernetes state. The existing
canonical/enrichment path remains the fallback when no accepted source tuple
exists; linked-agent evidence is never promoted to Proxmox ownership. The
same publication owner still admits or withdraws the eventual generation.
`proxmox_guest_observations_test.go` checks scope, original memory provenance,
CPU/disk/read time and nested detachment, alongside the end-to-end admission
controls above. The first complete admission proof exposed the former partial
retention and is retained as failed evidence, not weakened into acceptance.

### Permission-denial diagnostics retain scoped access (14 October 2026)

A PVE token's 403 error explains that privilege-separated access is the
intersection of user and token ACLs on the intended resource. It no longer
says to inspect only the user or includes the configured username/provider
body. The actual HTTP status, authentication-error wrapping and guest permission
classification remain; there is no retry, token rotation, access widening or
TLS change. Diagnostics distinguishes rejected authentication from permission
and other wire failures without promoting a denied resource to a reading.
`TestClientRequest_403TokenPermissionHint` and
`TestGetNodeStatusPermissionRefusalDoesNotInventMemory` cover the ordinary
HTTP boundary, unknown-not-zero memory and resumed CPU/memory with the same
credential. `TestComputeDiagnosticsAPIRejectionsPreserveCredentials` covers
pinned PVE/PBS fixture use and preserves the separate PBS scheduler state.
Native scoped-fleet recovery, QGA/thaw and installed trust remain separate.

### Complete PBS backup history reads (15 October 2026)

PBS group and snapshot listings consume and close their whole existing
8 MiB-bounded response before publishing rows. An authoritative read requires
HTTP 200, complete UTF-8 JSON, an explicit canonical `data` array and no
conflicting errors. Missing/null data, duplicate or case-variant supported
fields, trailing values, truncated/oversized bodies, failed closes and invalid
rows are errors, not empty inventories or completed snapshot reads. Groups
require unique type/ID, an explicit nonnegative count and a positive latest
time when nonempty. Snapshots require unique positive times and the requested
group identity; conflicting nested verification states cannot establish
verification. Additive metadata, genuine empty arrays, zero-count groups,
version-dependent verification and manifestless in-progress snapshots remain.

The existing collector retains historical rows/points and their original cache
read times on these failures. Its provider observation marks history partial
or unavailable, rather than renewing verified protection or authoritatively
deleting missing points. Ordinary complete reads resume refresh and protection;
complete empty arrays still remove deleted history only for their connection.
`TestPBSBackupInventoryAdmission` in `pkg/pbs/client_http_test.go` covers both
real client readers and complete-body boundaries;
`TestPBSBackupInventoryAdmissionPreservesHistoryAndResumes` covers ordinary
HTTP collection, cache age, stored points, protection and recovery/deletion.
These are controlled source proofs, not appliance backup/restore acceptance,
native guest thaw, installed recovery or release availability.

### PMG collection health and configured identity (15 October 2026)

The PMG poller carries `connectionId` from the exact saved config key (`pmg:`
plus that key), independently of its generated runtime ID, display and host.
A successful version request proves reachability only: any failed selected
read makes the resulting instance `degraded` and the provider health false.
Offline version failures remain offline. Complete ordinary collection restores
online/healthy; scope-off data remains absent and pause retains the observation.
No request, credential, TLS, backup replacement or scope rule changes.
`TestPMGCollectionFailureEvidence`, `TestPMGNodeDiscoveryFailurePreservesAcceptedBackups`
and `TestPMGConfiguredConnectionSnapshotWire` cover failure/recovery and snapshot
preservation. Consumer/HTTP and desktop/narrow evidence remains separate from
native appliance recovery and release availability.

The full PMG detail handler consumes the same canonical view reference as the slim resource. It preserves degraded collection health and optional configured attribution without an extra collection request; `TestContract_PMGConfiguredReferenceInFullDetail` covers both legacy absence and explicit reference.

### Future-dated backup evidence and alert evaluation

The shared PVE/PBS/mock backup-age evaluator retains an existing affected
condition when its only reported success is in the future. A usable success
for that subject can still be evaluated, and unrelated guests continue normally.
Unusable evidence is surfaced via the existing system-scoped backup-evaluation
warning with stable identity, deduplication and clock guidance; a successful
rollup read alone cannot clear it. Deliberate disablement still clears it.
`TestBackupFutureSuccessWarningThroughReadState` exercises persisted point
replacement, independent guest alerts, repeated evidence, served-state output
and genuine fresh recovery. This does not alter stored timestamps, rollup
aggregation, backup-server clocks or native guest/backup operations.

### Snapshot partial-evidence alert projection

`TestSnapshotPartialEvidenceServedState` connects the production snapshot
alert detector to `Monitor.GetState` across held age, current-size escalation,
future/missing/epoch times, missing size, independent recovery and deletion.
The served array preserves metric observation times/held flags and fixed
explanations, without changing snapshot collection or issuing guest/backup
commands. Its exact payloads feed desktop/narrow browser verification; source
and browser proof do not establish installed/native guest or restore recovery.

`TestSnapshotPartialEvidenceClockCorrectionServedState` restores a real alert
with old-clock dates through the ordinary temporary-profile JSON seam and
checks that a clock correction cannot establish recovery. It changes no system
clock and emits the actual served frame for additional consuming proof.

### Kubernetes inventory destination confidentiality (16 October 2026)

Kubernetes report delivery no longer diverts plaintext Pulse traffic through
an ambient HTTP proxy. The selected primary or explicitly admitted observer
remains the sole plaintext recipient of both authentication headers and the
compressed inventory. HTTPS proxy support, CA/pin verification, target admission,
report content, timestamps and independent destination buffers are unchanged;
this is a transport-only boundary, not a new field, health verdict or UI state.

`TestKubernetesControlPlaneProxy` in `internal/kubernetesagent/multi_target_test.go`
drives real HTTP reports and HTTPS CONNECT/TLS over in-memory connections,
including base paths, IPv4, IPv6, service DNS, an observer, custom CA trust,
matching/mismatched pins and an untrusted certificate. It compares the exact
decompressed report bytes and both token headers at the admitted recipient.
The fixture performs no DNS resolution, listener or network connection, and
does not establish native Kubernetes or installed fleet recovery.
### Backup history collection carries completeness

Each history page must bind every backup task's explicit node to the node
whose task endpoint was read, before appending rows or deduplicating UPIDs.
A foreign-node row makes that whole page `collection_failed`: none of its
prefix can replace live outcomes, enter report history or qualify complete
observation. Earlier admitted pages and independently readable nodes remain
available. An omitted redundant node field still inherits the request's node.
Correctly scoped later history can still update a live task's outcome; the
existing window, page/log budgets, authority and no-guest-operation boundaries
are unchanged. `TestBackupHistoryRejectsForeignNodePages` checks first/later
pages, duplicate-UPID contamination and peer preservation;
`TestPVEBackupHistoryForeignNodeCannotReplaceLiveFailure` joins direct/cluster
HTTP readers, ordinary polling and the served Coverage projection without
opening a database. This is evidence admission, not native backup/restore,
independent guest thaw, filesystem liveness or installed recovery.

`GetBackupTaskHistory` in `pkg/proxmox/backup_history.go` enumerates a bounded
ninety-day PVE task window with pagination and a fixed collection cutoff. Offline
nodes, failed reads, repeated/capped pages and unconfirmed `Sys.Audit` authority
remain explicit partial evidence. A successful HTTP response alone cannot prove
visibility of other users' backup jobs. The cluster client retains its canonical
transport failover wrapper. Older adapters can still supply live tasks but
cannot qualify report observation coverage.

`pollBackupTasks` persists mapped task evidence synchronously through the
organization's recovery manager before recording collection quality. The
continuity allowance derives from the configured backup interval/cycle cadence.
An unavailable manager or failed write cannot certify complete observation.
Multi-guest attribution requires a nontruncated, contiguous completed task log
with a terminal marker and parsed guests. Incomplete logs remain unqualified
and eligible for later collection. No backup artifact or restore assurance is
created by an execution outcome.

Proofs: `pkg/proxmox/backup_history_test.go` covers pagination, permissions,
partial reads and offline nodes. `internal/monitoring/backup_task_history_test.go`
covers durable failures surviving an empty poll and log qualification.

Backup collection starts with a bounded retained-history backfill, then uses a
durable collection checkpoint with a day of overlap. The oldest known pending
execution remains in range until completion, including after restart. Failed
reads do not advance the checkpoint. A capped backfill may advance a later
incremental read without backdating observation coverage. Another node's
failure cannot be hidden by a truncation reason. The live API/state keeps its
existing 200 provider tasks per node plus their guest details, while the recovery
store retains executions beyond that live window. Checkpoint, long-running-job,
failed-read, capped-backfill and bounded-live-payload regression tests exercise
these boundaries.

Recovery rollup and alert readers take a locked snapshot of the manager pointer,
matching the existing setter during live monitor dependency rebinding. The
`TestBackupHealthRecoveryRebindingCanOverlapActiveReaders` race proof exercises
rebinding while both consumers remain active.

### Live backup inventory and report backfill have separate admission

The unwindowed, bounded `GetBackupTasks` read remains the replacement boundary
for live backup tasks: every online node must return a complete valid response.
A failed live collection preserves the entire instance's rows, original
observation times and unseen job-log caches. A complete empty live inventory
may clear them. Report backfill may still retain independently observed task
executions while its collection quality is explicitly incomplete. An empty
incremental report window, missing audit confirmation or failed history page
cannot delete live tasks or certify complete report history.

Both task-page and audit-authority admission consume the complete bounded
HTTP body; a valid JSON prefix, null/duplicate envelope, provider error or
ambiguous privilege cannot qualify observation coverage. The existing live
200-task-per-node and shared 25-log-read budgets remain. Log finalisation still
requires contiguous numbered, present-text lines and a terminal marker; a
failed running-log reread retains its successful-read timestamp.

`TestBackupHistoryCollectionCannotReplaceLiveInventory` joins the real HTTP
readers, polling/cache, live snapshots and durable execution/coverage APIs
through node/body/history/authority failures, incremental emptiness, recovery
and authoritative live emptiness. `TestBackupHistoryRejectsIncompletePagesAndAuditEvidence`
checks page and permission admission. Existing log fixtures now include the
upstream terminal marker rather than weakening immutable-cache assertions.
Source and consuming desktop/narrow proof do not establish installed backup,
restore, independent guest thaw, writes/liveness or prior-active restoration.


### Bounded VMware whole-inventory refresh budget

`PULSE_VMWARE_POLL_TIMEOUT` is an optional positive Go duration for the whole
vCenter refresh. The default stays twenty seconds. Empty values use the default;
malformed, nonpositive and overflowing values warn without logging their raw
contents, then keep the default bound. This changes neither the independent
per-HTTP-request timeout nor a shorter caller lifetime. Polls stay serial and
cancellable; extending one connection's allowed time can delay its siblings.
Failure retains cached records, observed inventory and the last success time;
only an accepted refresh replaces them and clears its consecutive failures.

`TestVMwareInventoryRefreshBudgetContract` covers the parsing and private-value
warning boundary, then uses Go's virtual clock to
exercise a twenty-one-second production-provider refresh against the default
and a configured thirty-second budget, then a shorter parent deadline. It
checks cache/summary retention, ordinary successful replacement, serial
multi-connection dispatch and canonical resource projection for consuming
browser proof. No poll interval, API limit, TLS setting, provider permissions
or timeout threshold is waived. These synthetic controls are not vCenter 8.0.3
or installed recovery, and do not attribute #2070's HTTP 500 to refresh duration.

### PBS namespace discovery is inventory evidence (18 October 2026)

The ordinary foreground PBS poll carries each datastore's namespace-discovery
result into the same admitted background backup scan. A readable root cannot
certify complete history when discovery failed. Transient discovery failure
retains only that datastore's previously observed nested rows at their original
cache age; the root and other datastores retain independent replacement results.
Rejected discovery withdraws those unread raw rows instead. Neither outcome
probes cached namespace paths, renews their successful-read times, nor authorises
instance-wide recovery-point deletion. The provider observation is partial (or
unavailable without a readable scope), with unknown or partial/denied permission
evidence as appropriate. An ordinary complete discovery and inventory can again
confirm deletion and clear unused cache entries.

Namespace success consumes and closes the whole existing 8 MiB-bounded response,
requiring a canonical array and identifiable, unique namespace rows. Missing/null
data, conflicting errors, duplicate/case-variant supported keys, incomplete
bodies, trailing content and read/close failures are not empty inventories.
Supported `ns`/`path`/`name` spellings, explicit root, additive metadata and genuine
empty arrays remain accepted. Only an actual HTTP 404 preserves the older-PBS
root-only compatibility result; a different status mentioning 404 does not.

`TestPBSNamespaceInventoryAdmission` checks the real client boundary;
`TestPBSNamespaceDiscoveryFailureIsNotEmptyInventory` and
`TestPBSNamespaceDiscoveryAdmissionAndRecovery` join foreground discovery,
background admission, raw/cache scope, stored protection and ordinary recovery.
The controlled HTTP and consumer proofs do not establish incident cause,
installed recovery, backup/restore availability or independent guest thaw.


### CORE reporting reaches the existing chart consumer — #2077 / #2964

The monitor's unified-resource snapshot carries History targets resolved from
that same captured registry generation into the REST resource seed. The resource
service must not lose a native TrueNAS connection ID while rebuilding canonical
presentation rows, then make the drawer query the row's hashed ID instead.
Captured absence clears a retained target; no display-name inference or lookup
against a later live registry is allowed. Decorating a snapshot does not mutate
shared rows or the captured target map. Existing freshness, tenant/scope/token
checks, TLS verification and source precedence remain unchanged.

`TestUnifiedSnapshotPreservesCapturedHistoryTargets` verifies ownership and
known/unknown target preservation. `TestCORE13RESTReportingChartJourney` drives
the pinned-TLS CORE REST client, provider, native poller/monitor fallback and
actual authenticated resources/History routes for default-day and selected-hour
reads, complete/rejected/partial/measured-zero/recovered telemetry, free/ARC
memory and device I/O. `check-core-reporting-journey.cjs` serves those captured
JSON documents to the production resource decoder, TrueNAS systems table, inline
drawer and chart client at desktop and narrow widths. The fixture does not
require local History persistence or establish appliance, installed or released
recovery. Original scan/cleanup facts and stopped guest monitoring are separate.

### Persisted chart history uses the canonical value projection

Single-resource and fleet chart history reads in `monitor_metrics.go` use
`metrics.Store.QueryMetricValues` and `QueryMetricValuesBatch`. These return
fresh, detached `models.MetricPoint` slices, the same canonical point type
already aliased by monitoring. The shared retained reader projects timestamp
and value without transporting unused extrema or constructing an intermediate
full-point map and a second value-point copy. Existing metric filters, tier
precedence, chronological ordering, gap-fill lookback and Docker observation
provenance still apply through that same reader.

The chart boundary applies the existing LTTB target to its owned result maps.
It does not change chart sampling, point budgets, source precedence or payload
meaning. Persisted results remain detached across calls; this ownership does
not relax the separate detached-snapshot requirement for mutable in-memory
History or the mock resource timeline rule. Full retained readers continue to
return extrema for consumers that need them.

`TestStoreValueProjectionPreservesChartSampling` in
`internal/monitoring/monitor_metrics_slo_test.go` compares filtered and
unfiltered chart results against the full reader with 1,000-point series,
including LTTB selection and caller mutation. Metrics-store guards compare the
projections across mixed tiers and buckets, changed database snapshots and
Docker legacy/observed provenance; the query-plan guard covers both SQL forms.


### Non-destructive PBS monitoring-token collisions

`SetupMonitoringAccess` makes one token-creation attempt. A provider response
reporting an existing token returns `ErrMonitoringTokenExists` with its original
HTTP status; it never deletes the token, retries creation or changes that token's
ACL. An existing secret cannot be recovered by recreating its token. Explicit
`DeleteUserToken` remains available only to its separate intentional callers.
The ordinary new-token setup, monitoring-user ACL and token-ACL sequence remain.

`TestSetupMonitoringAccessCollisionKeepsExistingReader` exercises the real
request and version readers through 400, 409 and 500 collision responses,
preserving a saved-token reader before and afterwards without listeners.
`TestClient_SetupMonitoringAccess_PreservesExistingToken` replaces the former
rotation expectation. API/persistence and consuming-editor proof are separate;
these controls do not establish installed PBS or guest/backup recovery.
