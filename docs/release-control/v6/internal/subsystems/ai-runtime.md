# AI Runtime Contract

File-read failures must carry the canonical tool error bit. A missing host/guest
agent, failed command transport or nonzero host/container read exit cannot be
reported as successful evidence merely because a textual explanation exists.
The model, persisted tool history, telemetry and Assistant presentation consume
that same result. Successful reads retain their content and execution provenance.

## Contract Metadata

```json
{
  "subsystem_id": "ai-runtime",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/ai-runtime.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts",
    "cloud-paid",
    "frontend-primitives"
  ]
}
```

## Purpose

### Uncertain remediation retains a model-led follow-up

An inconclusive verification remains unresolved and may be investigated again
when the model reports it present, without another cooldown on top of Watch's
schedule. Existing attempt admission and autonomy checks still apply.
A model's fresh `present` assessment records evidence during Watch and hands off
only after the Watch adapter has unwound. It does not dispatch a second
investigation inside the detection conversation. Deliberate rejection,
needs-attention and cannot-fix outcomes keep their existing terminal meaning.

`pulse-enterprise:internal/investigation/orchestrator.go` captures the prior
investigation context before replacing the finding's session identity. It reads
only the exact retained session for the current occurrence and gives the model
its bounded summary and canonical action receipt as historical evidence.
Reopened occurrences with no current session cannot borrow an old receipt.
The model must reconcile an uncertain effect using current reads before deciding
whether a new action is justified. This adds no mutation or approval authority.
`findings_verification_followup_test.go` covers eligibility, persistence and the
Watch handoff. The enterprise orchestrator test covers receipt continuity and
session isolation. Native unattended runs remain necessary outcome evidence.


### Patrol resumes its actual next-check deadline after restart

The main Patrol timer checkpoints its next deadline and effective interval in
the existing run-history persistence envelope. A restart retains that deadline
after the normal collection/configuration startup grace. An overdue deadline
causes one immediate check after that grace, without replaying missed intervals.
An explicit interval change resets and checkpoints the new timer deadline.
Scoped checks and capped run-history retention cannot move an existing deadline.
Older stores without a cursor resume from the last successful full run and the
current interval. Disabled preparation never checkpoints a check that did not run.

`patrol_schedule_test.go` exercises the real timer loop with Go simulated time,
including repeated restarts, due/overdue execution, interval changes and first
startup. History adapter and configuration tests verify file reconstruction,
failed-write retry and concurrent history/deadline preservation. Live native
Patrol and independent workload observations remain necessary for operational
outcome qualification.

### Investigations retain collected backup and storage history evidence

Guest investigation manifests include the existing read-only storage tool so
backup failures do not require a host command connection. `backup_tasks` queries
the tenant's canonical recovery store without misclassifying execution records
as backup artifacts, supports canonical subject IDs and preserves failed-task
error text. Task success alone never proves that a restore artifact exists.
The API and tool adapter share `Manager.ListPointsForOrg`, including the
canonical graph source in demo mode, rather than presenting fixture evidence
through HTTP alone while tools consult an empty store.
`TestInvestigationReadsCanonicalBackupFailureWithoutAgent` uses mapped, persisted
task evidence and checks subject and tenant isolation. The `backups` inventory
also reads this org-scoped source with `KindBackup` and canonical subject IDs.
It returns canonical points with repository references, provenance and optional
verification fields, rather than VM-ID-filtered PBS/PVE summary lists or an
unscoped recent-task cache. Global offset pagination preserves source ordering
across providers and does not scan earlier pages. Read failures remain errors;
an artifact record alone is not proof of restorability.

Storage performance reads resolve the canonical registry metrics target and use
the monitor-owned history selector. Responses contain usage percentages with
timestamps and source provenance, including `mock_synthetic`, rather than
invented zero CPU/memory observations. Retained read errors remain errors and an
empty requested window cannot be filled with older chart samples.
History downsampling covers every observation in the returned window, including
the newest remainder samples when the count is not a multiple of the tool limit.
`TestDownsampleMetrics_IncludesNewestRemainder` covers dense retained series.
`TestStoragePerformanceReadsRetainedHistoryAcrossRestart`,
`TestStorageInvestigationHistoryKeepsRequestedWindowAndStoreErrors` and
`TestInvestigationStorageHistoryUsesCommunityMonitoringSource` cover retained
storage history, measured zero, failure, window bounds and API/tool parity.
These are read-path controls, not proof of real remediation or release readiness.

### Pro Patrol defaults to Ask first until the operator chooses

`AIConfig.PatrolAutonomyLevel` stays empty until an operator saves a Patrol
mode, and only explicit saves write it. `AIConfig.PatrolAutonomyLevelChosen`
reports that, and `AIConfig.ResolvedPatrolAutonomyLevel(entitled)` returns
`approval` (Ask first) for a never-chosen install that is entitled to
investigation, otherwise the stored or monitor level.
`Service.GetEffectivePatrolAutonomyLevel` applies the same default, but only on
a positive licence answer from `Service.PatrolInvestigationLicensed`, which
treats a missing licence checker as unlicensed. `HasLicenseFeature` stays
fail-open for its existing callers; the default must not, because a startup
window without a checker would otherwise start spending provider budget on an
install that may not be Pro. Unlicensed and unknown installs stay Watch only, an
explicit choice always wins, and readiness reads the same resolved level. The
default sends new and re-detected warning and critical findings to
investigation through the existing dispatch path and budget checker; it adds no
execution authority, because Ask first still waits for approval before any
change. Proofs: `internal/ai/service_patrol_default_test.go`,
`internal/config/ai_config_test.go`
(`TestResolvedPatrolAutonomyLevelDefaultsOnlyNeverChosenLicensedInstalls`) and
`internal/api/patrol_autopilot_test.go` (`TestPatrolAutonomyReportsTheProDefault`).

### Guest memory remains qualified through tools and local observers

The existing guest get/list/topology tools and Assistant inventory context carry
selected memory evidence with the percentage. Unavailable/invalid memory is
null, not zero usage; a measured zero stays numeric. Evidence retains source,
state and original observation time, including unknown pressure and possible
reclaimable cache. Condensed query facts retain the same qualification rather
than saving a bare percentage. `TestGuestMemoryQueryEvidence` and
`TestAssistantGuestMemoryContextAndFacts` check the real tool/context/fact paths.

The existing local memory observer uses that same interpretation for VM/system
containers. Unknown pressure wakes it with an explicit evidence gap, not a
numeric breach or a healthy predicate. A known selected guest sample is bounded
by its own observation age; refreshing the resource through another source does
not renew it. Other-platform legacy readings and other metric observers retain
their existing semantics. `TestPatrolObserverGuestMemoryOrigin` checks unknown,
retained, undated, future, invalid and stale origins alongside genuine low/zero/
high pressure and the age boundary. This changes no objective ABI, threshold,
collection lease, observer authority or guest-command admission.

Performance-tool guest memory history remains numeric, but explicitly lacks
per-sample source/pressure qualification. A live current reading cannot reattribute
past samples or prove that past high usage was pressure/a leak.
`TestGuestMemoryHistoryDoesNotBorrowLiveOrigin` checks unchanged numeric history,
its truthful limitation and non-guest compatibility. No History writer/schema
or existing stored finding is changed. These controls do not establish #2762's
running source, episode cause, native recovery or a containing release.

### Patrol guest memory is qualified evidence, not an unlabelled percentage

Patrol consumes the selected guest memory metric's original observation,
including absence, last-known state, source and time. It does not borrow a
different raw Proxmox facet, a refreshed resource timestamp or disk readiness.
Current, dated cache-aware Proxmox/QGA/Pulse-agent readings can support guest
pressure flags, anomalies, forecasts, actionability and recovery verification.
Cache-inclusive status/cluster/total-minus-used estimates, unknown sources,
undated/future origins and retained readings remain visible but cannot establish
current pressure or recovery. Missing memory is N/A, not measured zero; real
current zero and real high pressure both retain their meaning. Other platforms
without Proxmox observations keep their existing selected readings.

Full, quiet and condensed inventory and actual triage seeds preserve these
qualifications, including unflagged running guests within the existing analysis scope. Guest
analysis opt-outs do not acquire the new evidence section. The no-flag rollup is
not a health verdict. Native guest runtime status, rather than canonical collection
status, owns running/paused/stopped eligibility. A snapshot-only alert review
cannot auto-resolve a memory alert from unknown guest pressure; independently
verified current recovery and removed-resource handling remain separate.

`TestPatrolGuestMemoryEvidence`, `TestPatrolGuestMemoryForecastsAndAnomalies`,
`TestPatrolGuestMemoryAlertRecovery` and
`TestPatrolGuestMemorySelectedMetricAndOtherPlatforms` exercise these connected
paths. `TestIssue2762NamedMemoryFields` checks the existing named MemAvailable
parser against synthetic cache-heavy/reordered evidence, not a new free-table
parser. These controls do not establish issue #2762's installed input or cause,
a cleared finding, or a containing release. Monitoring collection, guest-agent
admission, configured alert thresholds, history storage and access are unchanged.

The `cluster-resources` source is read by guest type. For an LXC, Proxmox's
listing `mem` is cgroup usage with page cache subtracted (`PVE::CGroup::get_memory_stat`:
`memory.current` minus `memory.stat` `file` on cgroup v2, `usage_in_bytes` minus
`total_cache` on v1), measured against the container's own limit, so a current
dated reading on an agentless container keeps its pressure meaning for flags,
forecasts, anomalies and alert review. A QEMU guest's listing value is the host's
view of guest RAM and stays cache-inclusive and pressure-unknown, as do
`status-mem`, `status-freemem` and `derived-total-minus-used` for every guest.
`TestPatrolAgentlessLXCListingMemoryIsPressure` checks both guest types through
legacy and canonical read state.

Legacy Assistant mode echo and stored-preference compatibility follow the
[API settings contract](api-contracts.md#assistant-control_level-is-an-interactive-projection).

### Assistant protection is the per-resource lock, not a settings list

Settings no longer offers a Protected guests list, because it never filtered
canonical saved resource plans. The Assistant settings panel points the operator
at the resource's Never auto-remediate lock, and both existing help pages say
the same and state that the list is gone. The settings component, the
`settingsArchitecture` source pin and the mirrored-help checks prevent a
restored exclusion promise. `TestService_isAutonomousModeEnabled` also covers
every current saved level, explicit core-owned investigation enable/disable and
unchanged stored values.

### Interactive Assistant modes reflect action planning

Interactive Assistant offers Read-only and Ask first (`controlled`).
`config.AssistantControlLevel` projects legacy `autonomous` to controlled only
for interactive responses/tool availability. Saved bytes, shared capability
vocabulary, entitlement checks and legacy `IsAutonomous` approval-admission
remain intact; removing that legacy check would relax command approval.
Chat's control tool saves typed plans to Actions without execution. Scoped
handoff flags, execute authority, policy and action lifecycle
review/run/verification remain necessary; Patrol mode/Autopilot is separate.
Unrelated Settings saves compare normalised values and preserve saved legacy
preferences. Runtime, real encrypted-settings roundtrip/API, mounted component
and public-help controls cover this boundary; no installed action is proved.

### Assistant action plans do not guess an installation from session placement

An explicit current canonical resource ID takes precedence over stale session
aliases in `pulse_control`. Without one, session-to-inventory binding must have
one canonical match: repeated VMID/node pairs, container-ID/host pairs, agent
names or conflicting canonical aliases cannot select the first listed resource.
Ambiguity returns the canonical alternatives with the tool error bit before
calling the action planner, including for an older session-only reference.
The advertised-action projection omits these ambiguous targets too.

Unique placement, canonical aliases and inventory-only lookup remain usable
without another discovery step. Inventory-only canonical lookup retains session
registration. Capability checks, approval policy and execution remain with the
existing action lifecycle; these source controls establish no native action or
cross-installation monitoring recovery.
`control_target_identity_test.go` covers both inventory orders, all four control
resource kinds, conflicting aliases, planning refusal, stale-session canonical
override, unique placement and session continuity.

### Guest-config tools require an unambiguous resource identity

The existing VM/system-container configuration tool selects an exact canonical
resource ID before considering a name or bare VMID. A name/VMID matching more
than one guest or a target lacking complete provider placement returns a useful
error without invoking the config provider. Successful configuration and its
governance metadata come directly from the selected typed guest view, never a
second name-based resolution. Unique aliases remain supported; app-container configuration,
command authority and provider permissions are unchanged.

`TestGuestConfigToolIdentity` covers both inventory orders, shared names/VMIDs,
canonical-ID/name collisions, unique aliases and missing/invalid placement at
the actual tool/provider boundary. It checks config attribution and no provider
read on refusal. Source controls are not installed multi-instance recovery.


### Permanent suppression rules remain reversible — issue #2528

The existing manual suppression API/store semantics remain unchanged. Patrol
Activity now exposes scoped list/confirm/remove/readback for explicitly manual
rules. Deleting a rule permits future matches but does not reopen historical
dismissals, erase findings, broaden creation scope or modify other rules.
Finding-specific Reopen and remembered decisions remain distinct. Frontend
API/component controls plus authenticated handler/store controls verify these
boundaries; source/browser proof does not establish installed or reporter
acceptance.

Manual-rule confirmation retains a readable scope and exact ID with a scrollable long reason and separate action footer. This is an existing-flow accessibility repair, not a change to deletion, finding history or server authority.


### Host filesystem alerts retain exact subjects in scoped Patrol

An alert-button request may name a collected child subject such as
`agent:<source-id>/disk:docker-data`. Patrol binds that reference through the
shared current-inventory resolver only when the exact collected alert identifier
and subject agree. The canonical owner becomes the collection scope. The alert
keeps its original child identity, filesystem metadata, observed value, threshold
and message in both full and budgeted model context. Related child alert evidence
is retained only for the selected current owner. Missing owners, caller-authored
child paths, names and conflicting source identities cannot broaden scope.

Linked VM, container and node owners retain their agent observations. Node/host
views of the same proven canonical owner are one identity. Automatic alert type
hints follow that owner while explicit caller type restrictions stay intact.
Admission and execution resolve independently against current collection state.
`patrol_alert_scope_test.go` covers linked owners, evidence, repeated resolution
and rejected subjects. `patrol_host_disk_scope_test.go` exercises the production
host alert evaluator and HTTP handler. The opt-in browser backend and
`scripts/check-host-disk-patrol.mjs` verify `/alerts` admission and feedback at
1440 and 390 pixels. Browser fixtures qualify admission, not model diagnosis.

### Fully cached Patrol usage — issues #2118 / #2350

Zero ordinary input tokens do not mean a run was free or skipped. Any positive
ordinary-input, output, cache-creation or cache-read bucket retains the Patrol
result and usage event, including a partial result returned with a provider
error. The budget ledger and scheduled/scoped cost-history projection use the
same nonempty-usage rule; genuinely empty skipped runs and expired history stay
excluded. Ordinary input remains disjoint from cache buckets, with the existing
five-minute write/read pricing and next-run budget check unchanged.
`TestPatrolCacheOnlyUsageReachesBudget` checks the real Patrol-result, usage-ledger
and budget boundary with successful/partial write/read and uncached/empty controls.
`TestProjectPatrolCostIncludesFullyCachedRuns` checks six-hour schedule arithmetic,
scoped spend and history exclusions. These are synthetic accounting controls,
not provider-bill reconciliation, inter-run cache reuse or installed savings.

### RAID configured-member read evidence — issue #2369

The existing host RAID status tool preserves optional `required_devices` from
the canonical host view alongside source-native total, active, working, failed
and spare counts. Omission means the configured requirement is unknown, not
zero members. A total that includes spares is not evidence of a member deficit;
the shared health owner remains authoritative for risk. Tool serialization tests
check the separate counts. This read-only projection creates no feature, route,
array-repair grant, backup verdict or recovery confirmation.

### Ollama Basic Auth continuity
Patrol's runtime provider factory must carry the persisted Ollama username and
password into streaming requests without trimming password bytes.
`TestPatrolProviderUsesPersistedOllamaCredentials` verifies the persisted-config
to runtime-factory to HTTP Basic Auth path with a synthetic server. This is not
model capability or installed Patrol acceptance.


### Incident lifecycle occurrence identity (7 September 2026)

Fired, resolved and acknowledgement snapshots with a nonzero start time select
an exact (alert identifier, occurrence start) pair, not whichever incident is
open or latest. Replaying a retained closed occurrence must neither allocate a
new ID nor close a newer recurrence. A distinct start remains a distinct
occurrence even within the legacy timeline read tolerance. Zero-start legacy
callers retain best-effort matching. `TestIncidentStore_LifecycleReplayIdentity`
covers unchanged active evaluations, repeated fired/resolved transitions,
JSON checkpoint reload, canonical-backed shells and delayed historical events.
`TestIncidentStore_LifecycleRapidRecurrence` covers distinct subsecond starts.

An explicit newer refire reopens the same retained occurrence. The checkpoint
persists its refire timestamp so an older resolution, historical read repair,
or repeated refire cannot override the latest transition. Canonical resource
history carries `alert_started_at` separately from transition time, allowing
`QueryIncidents` to reconstruct one occurrence after fire, resolution and refire
even without a saved incident shell. Legacy history without this metadata keeps
its timestamp-based fallback. `internal/ai/memory/incidents_refire_test.go` pins
checkpoint and stale-replay behaviour, while
`TestMonitorLifecycleRefireReopensRetainedOccurrence` exercises both local and
canonical projections, including reconstruction from canonical history.

This does not migrate existing duplicate records, change bounded retention,
make acknowledgement event replay idempotent, or prove aggregate write-byte
reductions or recipient delivery. An evicted occurrence can still be recreated
by replay; exact identity is guaranteed only while its shell is retained.

The shared canonical incident query supersedes the earlier retained-shell-only
projection. Explicit firing boundaries, resource history identities and retained
shell boundaries isolate repeated occurrences. An unrelated resource's shell
cannot shorten the selected occurrence merely because its alert identifier
matches. Canonical-only occurrences and duplicate saved-shell notes use the same
query. Query bounds and read failures remain explicit. The upstream
`TestIncidentStore_CanonicalProjectionOccurrenceBounds` regression is preserved
alongside the new canonical-only, alias and duplicate-note cases.
The same regression checkpoints retained shells to JSON, reconstructs the store
and replays a firing. Stable projected identity and events remain identical,
while query observation bounds describe each fresh read. The unchanged JSON
checkpoint is not replaced. The canonical timeline stays in memory, so this is
not a durable event-store restart or installed-process write-rate measurement.


### Unchanged incident JSON checkpoints

Incident-memory checkpoints compare the serialized snapshot with bounded bytes
from the existing recovery file before replacing it. Byte-identical snapshots
do not rewrite the file, including after restart. Changed snapshots retain the
atomic replacement path; missing files and failed writes must remain retryable.
This does not change occurrence identity, retention or notification timing, nor
does it eliminate snapshot serialization. Proof:
`internal/ai/memory/incidents_unchanged_test.go` covers unchanged evaluations,
metadata, restart, resolution/recurrence, file loss and failed-write retry.

Coalesced asynchronous checkpoints are tracked until they finish. A queued save
still creates the data directory and writes its temp file after the mutation
returns, so a caller that removes or inspects that directory joins in-flight
saves through the store's unexported `flush` first. Tests backed by `t.TempDir`
register it as a cleanup; unjoined saves recreated the removed directory or
failed its cleanup with "directory not empty" on loaded CI runners. This is an
internal durability boundary. It does not change checkpoint timing,
coalescing or the stored format, and it is not a shutdown flush.
`TestIncidentStoreFlushJoinsQueuedSave` proves the join covers a queued save
that has not started.

### Process-wide memory ID ownership

`internal/ai/memory` preserves random UUID incident and incident-event IDs from
the reviewed occurrence repair. They have no shared mutable counter and do not
reuse a local timestamp after restart. Existing persisted references remain valid.
Remediation-record and context-memory IDs use the upstream package-wide atomic
`Int64` sequences through `nextSequencedID` in `ids.go`, without wrapping at 1000.
Their existing `<prefix><local second>-<n>` format remains; across restarts those
four sequences still depend on the local timestamp and retain that limitation.
`ids_test.go` checks all six actual formats and concurrent uniqueness, plus
independent concurrent tenant-store recording under `-race`. The reviewed
`incidents_identity_test.go` continues to pin UUID occurrence/event ownership.

### Canonical incident-history queries

Incident context reads `IncidentStore.QueryIncidents`, whose page contains
canonical evidence coverage and explicit truncation. Read failures become
unavailable context, not a healthy or empty-history claim. Explicit firing times
separate occurrences, and missing starts stay unknown. Historical lifecycle
status does not establish current resource health. Assistant handoff includes
the latest bounded events, their source provenance and query coverage. It
preserves the existing exclusion of raw command arguments/output. An absent
closure does not acquire a duration measured up to the current time.
A PVE disk alert row written under the physical disk its hardware identity
names (`unifiedresources.OwnedAlertReference`) is matched to occurrences under
its alert's own reference, as when it was journaled there, while its evidence
keeps the owning disk. Reads by alert identifier and by that reference
therefore return the same occurrences as before ownership, including one
occurrence whose rows two disks own after one took over the path while
failing. The one difference is an owned row whose occurrence's firing the
read does not hold (another disk owns it, or retention removed it): before
any event is assigned, its recorded start takes the occurrence that starts
there, else the closest saved shell within the start tolerance, else opens
the occurrence under the ID the alert-centric read gives it, instead of
leaving the row as partial evidence. That boundary is not canonical, so a
truncated read still keeps events without a start away from it. A read by a
disk returns the occurrences its rows belong to and admits their saved
shells, so a note recorded from a disk read reaches the occurrence the
alert-centric read shows. The exception is two occurrences of one alert
starting within the shell tolerance: the alert-centric read assigns a saved
shell by firing order, which a disk read missing a firing cannot reproduce.
`TestIncidentQueryKeepsOccurrencesOfHardwareOwnedDiskAlerts` and
`TestIncidentQueryTruncatedReadKeepsOwnedOccurrencesGuarded` in
`internal/ai/memory/incident_query_disk_owner_test.go` compare both layouts,
the first on the memory and SQLite stores.


### Canonical Patrol and Assistant continuation, 2026-09-07

Canonical investigation planning runs inside the model's tool call. The model
receives the persisted plan or refusal before choosing its next step. Accepted
planning does not end the model turn or establish a diagnosis. Accepted action
identity survives a later provider failure, which remains a separate failure.
Completed tool results supply evidence identity. Tool starts and proposed prose
do not become observations. Explicit run budgets, authority and independent
verification remain deterministic.

Patrol and unified finding replay preserve the original resolution time and do
not append another verification event. A genuine regression can resolve again.
Handoff-policy redaction preserves streamed whitespace. Assistant scrolling owns
only the transcript container and cannot move outer page ancestors. Current
qualification, failures and wider rollout limits are recorded in
`docs/qualification/PATROL_ASSISTANT_CUSTOMER_JOURNEY.md`.

Patrol investigation's mode contract states its non-interactive read/proposal
boundary and asks for diagnosis, supporting evidence and remaining uncertainty.
It does not prescribe restarting an unhealthy container or require a peer
investigation before admitting uncertainty. The model chooses useful evidence
and justified remediation. An advertised action recommendation is submitted
through patrol_propose_action. A proposal is neither execution nor proof of
causal correctness, and core policy retains execution authority.

The shared app-container query returns native filesystem observations from the
canonical resource or its typed view. Configuration mounts and measured
filesystem usage remain distinct. Filesystem capacity is not a resource quota.
Unavailable observations omit the usage payload rather than supplying zero.
The query contract describes those Pulse-specific semantics without a storage
diagnosis recipe. Real-model requalification is required after this prompt and
evidence change and remains recorded in the customer-journey qualification.

Action reconciliation refreshes the durable product investigation record from
the authoritative session/action even when the finding outcome already matches.
Action truth is authoritative for its originating investigation, not for every
later occurrence sharing the finding ID. Finding outcome and record writes
check the current session and persisted regression boundary under the findings
store lock. Historical callbacks and review hydration retain their original
action result without resolving, queuing, or replacing a newer occurrence,
including after restart and before its next investigation starts. A verified
action from the new occurrence can still resolve that occurrence normally.
Finding-scoped investigation and transcript reads return no current
investigation while only an older occurrence's session exists. Explicit
session and action history retains that older evidence.
The canonical Patrol handler suite in
`internal/api/ai_handlers_patrol_actions_additional_test.go` exercises three
occurrences with a persisted restart and historical action replay; findings
store controls also exercise concurrent re-detection and reconciliation.
The same builder owns initial completion and later refresh. Completion replaces
an early action projection with the final investigation evidence. Later action
refresh preserves original prose, impact, confidence, evidence and rollback. Unchanged hydration is a no-op, and a
record-only repair does not repeat outcome notifications. Resolved findings
retain the canonical action-history link and resolved status in Assistant context.
Live approved and rejected recovery cases pass, but storage's lexical scorecard
pass fails semantic review because an exhausted container tmpfs was incorrectly
attributed to host capacity. Storage and broader product qualification stay open.

The live qualification client follows the canonical resource API's pagination.
The API caps each page at 100, so a larger requested limit cannot establish a
complete inventory. A later-page failure returns an error rather than partial
inventory. Regression proof covers an unhealthy resource beyond the first 100
and failure while reading a later page. This changes collection coverage, not
fault oracles, model context policy or outcome scoring.

Stored chat and product history share the result-bearing `TranscriptToolCall`
contract. API adapters preserve observed output and the explicit success/error
bit. Only provider-request projections remove those display fields. A failed
read must not become an invocation with no visible result on the way to Patrol
or Assistant history. The adapter regression includes a `NO_AGENT` result and
`success: false`, and provider serialization retains its existing narrower shape.

Capturing a typed proposal does not create an action. The proposal response
discloses that broker validation is still pending, without forcing the model to
stop investigating. If the broker later refuses submission, the enterprise
orchestrator retains the model's diagnosis unchanged and records the broker
error as a failed investigation needing attention, with no action reference.
The real disconnected-agent case must remain unsuccessful until its actual
transport prerequisite is satisfied. A successful model turn or recorded
proposal is not approval, execution or recovery.

The shared investigation review renders sanitized Markdown and does not repeat
an identical persisted/fetched conclusion or error. Distinct evidence remains
visible. Pausing scheduled Patrol does not disable history review. The review
control has one detail target and returns keyboard focus when closed. Merged tool
results use Assistant's shared expandable evidence component. Historical calls
without an explicit result bit do not gain an inferred success/failure state.
Resolution copy cannot infer manual review from `needs_attention` or `cannot_fix`.

The shared pricing table includes reviewed standard Gemini 3.8 Flash rates for
the exact direct and OpenRouter routes. OpenRouter variants and aliases remain
unpriced until independently reviewed. Rates carry the review date and are
estimates, not reconciled provider charges. The introductory rates require a
new review on 2027-01-01. `TestGemini38FlashReviewedRoutePricing` covers real
qualification token counts and request-route preservation, and
`TestGemini38OpenRouterPricingDoesNotGuessVariantRates` preserves unknown variants.

The canonical query tool describes the app-container configuration boundary
explicitly: TrueNAS supports `config`, while Docker/Podman expose their collected
health, mounts, ports and networks through `get`. This communicates the existing
adapter contract to the model. It does not add configuration access, suppress
tool errors or weaken qualification gates. The existing
`TestAppContainerConfigObservationContract` retains unsupported-adapter and
provider/identity boundaries.

Shared app-container query mount evidence preserves native type, source,
destination, options and canonical read/write access. Compound options such as
`ro,noexec` cannot become writable through string equality heuristics. Both the
canonical provider and typed read-state projection preserve the same fields.
Configured size in mount options does not establish filesystem usage or free
space. `TestQueryPreservesMountConfigurationEvidence` covers these contracts.
The ordinary live storage case still failed diagnosis by excluding resource
pressure without capacity evidence. Mount fidelity alone does not qualify model
interpretation. Incident-record lookup across canonical and legacy Docker IDs
and typed compatibility lookup of canonical IDs remain explicit identity gaps.

Shared query projections name command transport explicitly through
`command_agent_connected`, `node_command_agent_connected` and the corresponding
topology counts. These observations do not establish monitoring freshness or
installation state. A topology built without a connection snapshot omits command
flags, execution hints and connected counts. An observed empty snapshot preserves
false/zero. Assistant's inventory seed carries that same absence semantics.
The parent node's connection cannot become a direct guest connection merely
because provider placement names that node. Existing command routing, control,
approval and invocation enforcement remain authoritative. A `can_execute` hint
reflects connected transport with control enabled, not approval for an operation.
`TestCommandConnectivityDoesNotReplaceMonitoringEvidence`,
`TestTopologyOmitsUnobservedCommandConnections` and
`TestAssistantInventoryDoesNotInventCommandConnectionObservations` cover these
projection and continuity boundaries. Existing persisted tool records are not
rewritten, and this contract does not qualify model diagnosis or recovery.

Native app-container configuration reads resolve identity, provider and placement
from current canonical inventory. Optional session discovery cannot fabricate a
not-found result or replace current placement with a stale execution target.
Query restrictions on both the supplied reference and canonical identity are
checked before registration, and an existing session's allowed actions are not
expanded by a read. Unsupported adapters, missing providers, incomplete placement
and nil provider observations retain known resource identity and an explicit
unavailability reason with the shared tool error bit. They cannot count as a
successful configuration read. Actual inventory absence remains distinct.
`TestAppContainerConfigObservationContract` exercises these boundaries with
strict resolution enabled. This read correction does not relax action or native
log validation and does not qualify autonomous diagnosis or recovery.

The published Patrol qualification schema must accept the fault injectors used
by the executable catalogue. `TestCatalogFaultInjectorsMatchPublishedSchema`
checks the actual scenario faults against the schema enum, including the
health-process fault shared by approved, rejected and autonomous restart cases.
This compatibility proof does not establish live diagnosis or action outcomes.

Physical-disk reads in `pulse_metrics` and `pulse_query` list/get/health
share `physicalDiskSummaryFromResource`. The projection preserves canonical
resource status, per-source freshness, physical-disk risk reasons and optional SMART counters alongside
the device-reported SMART health result. It never recomputes risk or collapses a
SMART pass into an all-clear. A stale source may explain a warning without a
hardware risk. Missing counters remain absent and observed zero
remains present. The tool projection labels `life_remaining_percent` explicitly, rather than
exposing the source-specific ambiguous `wearout` name, and distinguishes it from
`smart.percentageUsed` as consumed endurance. The regression proof is
`internal/ai/tools/physical_disk_evidence_test.go`.

Disk temperatures reach the model as current readings only when they were
collected now. `tools.SplitDiskTemperature` classifies a physical disk
temperature with `diskinventory.TemperatureCollected`. The physical-disk
projection's `temperature` and the `disk_health` SMART row's `temperature`
carry only a reading the current observation collected. A value normalization
retained instead (a disk in standby, a host agent past its reporting lease)
moves to `last_known_temperature` with `last_known_temperature_reason`. The
same split feeds the AI chat context and Patrol. The "Physical Disks Needing
Attention" list and Patrol's disk issue gate, triage flags and finding
verification judge heat on the collected reading alone. Their text renders a
retained value as "last known 41C (disk is in standby)". Proofs:
`TestDiskToolsReportRetainedTemperatureAsLastKnown` in
`internal/ai/tools/physical_disk_evidence_test.go`,
`TestBuildUnifiedResourceContextJudgesDiskHeatOnCollectedTemperature` in
`internal/ai/resource_context_test.go` and
`TestPatrolPhysicalDiskRowsJudgeHeatOnCollectedTemperature` in
`internal/ai/patrol_ai_more_test.go`.

They judge that collected reading against the alert disk temperature policy,
not a fixed line. `ThresholdProvider.GetDiskTemperatureThreshold(host, diskType)`
(the alert manager's `HostDiskTemperatureThreshold` trigger and clear) resolves
each disk's limits once in `patrolPhysicalDiskRows` and in the AI chat context,
for the host agent that reports the disk. `physicalDiskTemperatureHost` finds it
by walking the disk's parent to the agent machine, through the pool for an
Unraid disk, so that agent's Disk Temp override (or one inherited from its
linked node or guest) replaces the per-type trigger, and an override or agent
default that switches the host's alerts off leaves no heat to judge. A disk no
agent reports gets the per-type policy. The global alerts switch and the agent
alert-type switch stop `CheckHost` without changing this policy, as before.
A TrueNAS disk (`alerts.IsTrueNASDiskResource`, the disks the unified evaluator
alerts on as `truenas-disk`) skips that walk, because its TrueNAS system carries
a synthetic `Agent.AgentID` that would pass for a host agent:
`physicalDiskTemperatureLimits` resolves it through
`ThresholdProvider.GetTrueNASDiskTemperatureThreshold(resourceID, diskType)`
(the alert manager's `TrueNASDiskTemperatureThreshold`, the `truenas-disk`
tiers of `effectiveAlertPolicyNoLock`), so the disk's own override, then the
TrueNAS-wide TrueNAS Disks value, then the per-type policy decide, as for its
temperature alert, and a disabled override or TrueNAS Disks default leaves no
heat. A host override under the system's synthetic ID no longer reaches its
disks, and Disable all TrueNAS silences the alert without changing this policy.
The run state carries the provider (`patrolRuntimeState.thresholdProvider`). With no provider the factory alert
configuration applies. A disk is hot from its trigger: a triage warning, a disk
issue and an AI chat "needing attention" entry, as its temperature alert and
Physical Disks Running Hot verdict are. A disk-high finding recovers at or
below the clear value (under the trigger when there is no band below it), as
the alert does. With no current reading, a disk last seen hot leaves that
verification unknown rather than recovered. Scoped runs keep the provider, and a
provider set before Patrol starts is handed to it. Below the trigger Patrol flags nothing, so an NVMe at 63C stays
quiet and a SATA disk at 56C is flagged. Proofs:
`internal/ai/patrol_disk_temperature_test.go` (a real registry with one
overridden agent in `TestPatrolJudgesAgentDisksByTheAgentDiskTemperatureOverride`,
and TrueNAS fixture disks under a per-disk override, a TrueNAS-wide value and a
host override on the system ID, one with a retained reading, in
`TestPatrolJudgesTrueNASDisksByTheirAlertTiers`),
`TestAlertThresholdAdapter_DiskTemperatureFollowsAlertPolicy`,
`TestAlertThresholdAdapter_DiskTemperatureHonoursHostOverrides` and
`TestAlertThresholdAdapter_TrueNASDiskTemperatureFollowsItsAlertTiers` in
`internal/ai/alert_threshold_adapter_test.go`.

Retained summaries disclose that point and bucket timestamps describe returned
history inside the requested window. Their spacing does not measure collection
uptime or explain missing history. Collector lifecycle and retention settings
are not part of the summary query. These are source semantics supplied to the
model, not a harness-generated diagnosis or a claim of model qualification.

The manual Patrol API consumes the service's selected-mode runtime verdict as
an execution check. Dimension-level presentation warnings cannot authorize a
run after a provider failure leaves Watch-only readiness unassessed. Operator
cancellation retains its existing non-failure advisory semantics. Higher-mode canary guidance
retains its existing advisory behavior only after Watch has been verified.

The unavailable `current_resource` tool result describes an unattached shortcut,
not an unresolved explicit name. Its recovery guidance directs named resources
through canonical query search. The attachment guard remains enforced and
never invents a selected resource or grants execution authority.

Resource-scoped `pulse_metrics` performance reads resolve canonical identity
and unambiguous lookup aliases through the unified registry, then query its
metrics target. The retained SQLite metrics store supplies the requested window
across backend restarts. An available store error remains an error, with no
silent downgrade to a shorter in-memory history. The in-memory-only adapter
remains a compatibility boundary for installations without a retained store.
Unscoped `pulse_metrics` performance and baseline paths still use their existing
providers and are a modernization residual, not proof of retained-history coverage.
`TestPerformanceMetricsRetainedAcrossRestart` verifies native store coordinates,
canonical response identity, node/agent/guest families, restart retention and
store failure. The `node` get alias resolves to canonical `agent`, whose CPU
count retains Proxmox topology when no agent count is available. Physical disk
metrics use the advertised `disk_type` filter independently of `type=disks`,
and share canonical disk host projection with resource queries.

An incomplete streaming readiness evaluation leaves latency and Watch-only
suitability unassessed. Completed tool/context evidence remains available, and
an explicit provider-error guard prevents authorizing Patrol. Provider errors
remain distinct from measured latency failures. The exact legacy unfinished-probe latency verdict is corrected when persisted
evidence is decoded, preserving its timestamp, provider cause and completed
checks without a new provider request or granting a verified mode.

An explicit Claude terminal `stop_reason=refusal` becomes the typed provider
refusal error before any tool call or final answer can be recovered, on either
successful or unsuccessful process exit. Runtime and connection diagnostics
project `provider_refusal` with policy/support guidance rather than network,
latency or billing guidance. A saved legacy CLI envelope is reclassified only
when its complete terminal JSON contains that explicit signal. Initial probe
counts and the original evaluation time survive, the CLI dump is removed from
the displayed refusal detail, and no provider request or authorization is
created. Supported subscription Patrol execution remains unqualified.

Within an active investigation, tool observations are preserved across provider
turns while the full request fits the model context window. Turn age is not a
compaction trigger and does not prove evidence has been understood or retained.
The existing pre-request context-limit boundary owns overflow compaction,
re-estimation and fail-closed rejection. `TestAgenticLoopPreservesEarlierEvidenceWithinContextLimit`
exercises four tool rounds and checks that the final answer request still carries
the original alert state, sensor reading and observation timestamp.

The shared `pulse_metrics` temperature read projects both canonical agent and
Proxmox node observations through typed read state. Proxmox sensors must remain
available without an installed host agent. Results preserve canonical resource
identity and source, with the sensor collection timestamp when supplied, and
keep linked providers distinct rather than choosing between conflicting
readings. Hostname, display name, or canonical identity can select a host, and
`resource_id` filters exactly. Missing observations do not imply that an agent
must be installed. Standby-skipped disks are not current temperature evidence.
`TestExecuteGetTemperaturesCanonicalSources` verifies provider provenance,
selection, sensor coverage, and absence handling.

Incident timeline summaries must distinguish provider conditions from metric
threshold evidence. For `resource-incident` alert events, numeric value and
threshold placeholders must not render as a measured comparison; retain the
alert type and severity instead. Numeric metric alerts retain their comparison
format. This applies to reconstructed event summaries, not a migration of
already persisted summary strings.
`TestProviderIncidentSummaryDoesNotInventThreshold` in
`internal/ai/memory/provider_incident_summary_test.go` verifies both boundaries.

Own the Pulse Intelligence Core: canonical context, governed actions, safety
gates, approval state, action audit, and verification. That core backs Pulse
Patrol as the primary built-in operator that checks infrastructure, investigates
issues, follows the chosen Patrol mode before acting, verifies outcomes, and
records what happened. Pulse Assistant is the contextual
explanation, approval, and handoff access path over that same work, while the
Pulse MCP adapter is the external-agent access path over canonical API
contracts. The subsystem also owns AI orchestration, runtime cost control,
shared AI transport surfaces, and browser-visible Assistant transcript actions
that define what visible operator/model text can leave the transcript without
exposing hidden provider/tool metadata.
Patrol guest inventory consumes canonical unified-resource CPU percent when
read-state is available and the shared Proxmox guest CPU-percent normalizer for
legacy snapshot fallback. It must not independently rescale by configured cores
or substitute a linked in-guest host-agent CPU value, so Patrol evaluates the
same guest observation as dashboard, alerts, and history.
Dismissed Patrol findings remain durable operator history. Reopening a
finding-backed suppression row must use the canonical undismiss transition:
clear dismissal, suppression, acknowledgement, and reminder state; preserve the
operator note; append an `undismissed` lifecycle event; and immediately mirror
the reopened state into the unified findings projection.

Interactive Assistant invocation authority is fail-closed and request-local.
The `read_only` presentation means no model-invokable durable Pulse-state or
infrastructure mutation is projected or executable. An `ai:chat` token grants
conversation, read, and session access only; it cannot write knowledge, change
finding state, approve, or execute. Infrastructure mutation additionally
requires explicit `ai:execute` authority and a control level that permits it.
Provider projection and registry execution consume the same invocation-aware
policy. Detection may write only through its explicit finding report/resolve
allowlist; investigation is domain-read-only and may emit at most one
side-effect-free typed proposal whose correlation identity is server-authored.
Unknown profiles, actions, aliases, origins, scopes, and mutation
classifications fail closed.
An investigation proposal is a request for the canonical policy plane to make
an execution decision, not execution or approval by the model. When an
investigation's recommendation or conclusion tells the operator to try,
consider, or perform an advertised remediation, the model must submit that
exact remediation through `patrol_propose_action` instead of leaving it only in
prose. Unknown root cause does not by itself prohibit a reversible, bounded
initial proposal when evidence confirms the operational symptom; capability
validation, approval floors, tenant mode, emergency stop, dispatch, and
verification remain core-owned and fail closed.
For a running but currently unhealthy app container, exact current-state
evidence plus an advertised restart is sufficient to submit that governed
proposal unless evidence identifies a restart-specific hazard. Empty logs,
approval-blocked deeper inspection, or an unknown root cause do not substitute
for such a hazard and must not cause Patrol to pre-empt the policy/operator
decision by withholding the proposal. This permission does not relax causal
evidence ordering: when a cross-resource cause remains plausible, Patrol must
test at least one plausible peer or dependency through canonical query,
discovery, or topology evidence before proposing against the symptom resource
or concluding that root cause is unknown. Investigation reasoning must
reconcile the concrete resource entries in later topology/query/discovery
results before asserting that no peer is implicated. The Docker `services`
operation is explicitly Swarm-only: an empty service list never proves the
absence of ordinary Docker containers or container dependencies, and its tool
result must direct the model to topology or search for that evidence.
Every accepted investigation proposal preserves the canonical action target.
An optional `causal_resource_id` records the model's attribution when supported
by evidence. It is omitted when the cause is unknown. A recovery proposal may
address an observed symptom without establishing its underlying cause. The
reason carries observed evidence, expected benefit and remaining uncertainty.
The capture boundary validates capability schemas, permissions, correlation,
parameter isolation and invocation integrity. It does not maintain a duplicate
resource graph or infer causality from dependency names and status vocabulary.
The tool-free completion turn distinguishes proposal acceptance from diagnosis.
The proposal remains an attributed model decision. Core preserves final model
prose unchanged in the response, stream and transcript, including uncertainty
or a correction of the earlier rationale. Neither structural validation nor
proposal acceptance establishes a root cause or authorizes execution.
Investigation completion and proposal availability do not depend on successful
evidence-call counts. The model may conclude from supplied observations or
report that missing access prevents diagnosis. Failed reads remain failed in
the model context and retained transcript. Completion means the run ended,
not that its diagnosis is correct, an issue is resolved or an action executed.
Capability schemas, permissions, correlation and invocation integrity still
govern proposals and execution independently.

The configured evidence-attempt ceiling remains a resource limit. Attempted
calls consume it whether they succeed or fail. The request reports the remaining
allowance as a system fact and removes those tools when it reaches zero.
It does not prescribe an investigation priority, demand a causal peer, or inject
checkpoint instructions into tool results. The existing classification of
infrastructure evidence calls remains only the scope of this resource limit.
Planning and retained-context tools remain subject to the overall model-turn
limit. Output-limit recovery stays bounded and uses existing context.

Provider-returned function names must also match the exact tool manifest sent
on that turn before read/write classification or execution. A name invented
from an action or operation enum is rejected as `TOOL_NOT_ADVERTISED`, receives
the exact advertised-name set for one model-owned correction, does not consume
the investigation evidence budget, and must never be misreported as an FSM
mutation refusal. Unknown functions remain non-executable and are never
heuristically mapped onto a capability.

Model/tool orchestration is bounded by construction rather than by request
timeout. The legacy `Service` streaming and non-streaming paths share one
ten-provider-call budget owned by `internal/ai/service_tool_loop.go`; reaching
that budget returns a typed terminal failure. A provider that repeats the same
policy-denied tool name and arguments terminates on the second provider call,
before the repeated call can execute. Denial-loop termination must not create
an approval, mint command authority, dispatch an agent message, or mutate
external state. The canonical chat `AgenticLoop` retains its separately
configurable bounded-turn contract for native Assistant and Patrol flows.

Discovery `run` currently persists read-derived evidence in the discovery
cache. That cache write is not an H1 finding/knowledge/infrastructure mutation
exception and remains a separately governed residual under
`lane-followup:agent-native-continuous-discovery-findings` until discovery's
canonical persistence boundary is resolved.
App-shell Patrol chrome may expose only content-free current-work pressure, such
as an open-work count on the stable `Patrol` tab, from Patrol-owned findings and
approval read models. It must not rename the destination, create a second
findings queue, expose finding or approval identity in shell chrome, or turn
platform pages into Patrol summaries.
When backend Patrol summaries or Assistant handoffs expose run coverage to the
browser, they translate internal full-run, scoped-run, and verification
precision into plain operator check language rather than activation-loop,
proof-strip, or scoped/full-run jargon.
Coverage warnings must stay action-oriented: they may tell the operator to run
Patrol to check everything, but must not ask for a "full issue list" or otherwise
expose backend full-run vocabulary as the product instruction.
Patrol model-facing system prompts must derive the active control-mode block
from the effective `patrol_autonomy_level`, after license and full-mode-lock
clamping, so Watch only, Ask first, Safe auto-fix, and Autopilot give the model
the same investigate, approval, execution, and verification boundaries enforced
by the orchestrator. Prompt copy must not fall back to the legacy
`patrol_auto_fix` boolean or the retired "observe only" / "auto-fix mode"
framing.
Patrol's Watch evidence includes provider-reported failed health checks, backup
and replication state, scoped inventory, observation sources and times, and
heuristic triage flags. The model decides which observations require attention
and which further reads can change the diagnosis. A flag is neither a reporting
obligation nor a stop condition, and absent flags do not establish health.
Before execution, core loads the complete active-finding snapshot for the exact
caller-requested lifecycle scope and binds it to the run adapter. The original
model conversation receives that snapshot and owns its present, resolved or
uncertain assessments. Missing access and stale or partial evidence remain
explicit limitations.

Ordinary detection has a fixed forty-turn execution limit, independent of flag
count and inventory size. An explicitly requested quick scope retains its
four-turn limit. At the final allowed turn, the existing bounded finding-decision
projection can record conclusions from retained evidence. A saved finding does
not end evidence gathering before that limit. Rejected calls and their errors
remain in the same conversation, where the model may correct them within the
remaining limit. There is no separate unmatched-signal evaluator, assessment
sweep or accepted-report-count budget. Omitted assessments remain incomplete
and cannot resolve the finding. Provider failure remains a run failure even
when earlier finding writes succeeded.
Findings first created by an accepted `patrol_report_finding` in the current
run are already complete structured new-issue outcomes and never join that
run's pre-existing-finding assessment obligation, even if a concurrent or later `patrol_get_findings`
call observes them. Re-reported pre-existing findings remain subject to the
existing-finding verdict contract.
If the main provider pass nevertheless calls `patrol_assess_finding` for a
finding first created by its accepted report, the adapter preserves the report
as authoritative and returns a successful typed no-op (`applied=false`,
`reason=new_finding_reported_this_run`). No `present`, `uncertain`, or
`resolved` assessment is persisted and the new finding is not refreshed,
downgraded, or resolved.
The same run may not turn its own accepted report into a recurrence by
re-reporting the stable finding ID. A repeated same-run report returns a
successful typed no-op (`applied=false`, `reason=finding_reported_this_run`),
does not call the finding store again, and leaves `TimesRaised` and the
finding heartbeat unchanged. Active-finding loading is core-owned lifecycle
plumbing, not a model-owned diagnostic decision. When the complete snapshot is
already established on the run adapter, `patrol_get_findings` is removed from
the first and all subsequent provider turns and the model reuses the exact
snapshot in seed context. Narrow compatibility adapters that do not establish a
core snapshot retain the legacy one-shot behavior: after the first successful
read, the tool is removed.
The provider-neutral Watch orchestration boundary also collapses an exact
repeat of a finding lifecycle name and canonical arguments before it becomes a
second canonical tool invocation. This applies to repeated calls in one
provider batch and to calls repeated after an accepted lifecycle result. It
does not collapse calls with different arguments, so the model can still
report causally independent incidents sequentially. Lifecycle stream starts
are delayed until the complete provider batch is normalized, preventing a
suppressed retry from appearing as a second action in Patrol history or the
browser. The finding adapter remains independently idempotent as the durable
write boundary.
Patrol reports one operator-facing finding per causal incident. Symptoms that
share a causal chain are grouped on the user-facing degraded resource with
dependency evidence and honest uncertainty in the same finding; causally
independent incidents remain separate. Finding authoring accepts only warning
and critical severities, and lower-priority or healthy observations remain
summary prose rather than rejected lifecycle writes. Multi-incident Watch
runs record one complete report at a time so provider turns cannot split
required fields across parallel calls.
Resource-down state remains owned by Pulse's real-time alerts rather than
becoming duplicate Patrol work. When a report merely restates that a current
app container is stopped, exited, dead, or offline, the adapter returns a
successful typed no-op (`applied=false`,
`reason=alert_owned_resource_state`) and persists no finding. A distinct
downstream degradation or deeper reliability issue remains reportable. The
boundary classifies the proposed issue by its required stable key rather than
free-form prose, so evidence that a deeper issue also stopped the container
does not hide that issue. This lets causal incidents retain one user-facing
Patrol finding even when a provider tries to file the alert-owned dependency
state separately.
Investigation and interactive profiles retain their bounded final summary.
Patrol report, assessment and resolution tools are governed Pulse-state writes
for invocation authorization. Accepting one records a decision, does not prove
its diagnosis and cannot enter or satisfy the infrastructure read-after-write
FSM. The normal detection conversation retains its evidence capabilities and
standing prompt after those writes. Exact repeated accepted lifecycle calls are
suppressed before invocation, while distinct decisions and reads remain
available. A provider failure is retained alongside any accepted finding and
cannot be replaced with a success summary or a separate diagnostic session.
When a real infrastructure write does
require another verification turn, the internal verification constraint is
appended to the provider conversation as a user-role instruction; the request
must never end in an assistant prefill that compatible providers reject before
verification.
Patrol finding storage must consolidate equivalent active storage-capacity
siblings before they reach browser surfaces: a broader storage risk and a
generic usage/capacity finding for the same normalized storage identity are one
operator issue, while distinct backup, health, and pool-vs-device findings
remain separate. This consolidation belongs in the finding store, not the
frontend row renderer.
Capacity forecasts (days-to-full, current utilization, daily change rate) are
computed deterministically by linear regression over a resource's utilization
history and must persist as structured data on the Finding
(`capacity_forecast`), stamped by the finding store after a successful Patrol
analysis via `StampCapacityForecasts`. That structured field is the canonical
operator urgency source for capacity-relevant findings; the model-authored
description must never be treated as the urgency signal when a deterministic
forecast is present. A forecast requires the resource's utilization to be
ingested as a time-series in metrics history; resources without recorded
history cannot produce a forecast and must report its absence honestly rather
than fabricate one, and stable-high (>=80%) pools with no clear fill trend must
carry the deterministic "no fill trend" reading so the model's speculation
cannot override the verified trend. The forecast must query utilization
history under the resource's metrics-target ID (the same key the monitoring
layer records under, exposed as `StoragePoolView.SourceID()`), not the
canonical resource ID; the two diverge for most storage sources, so querying
the canonical ID silently leaves the feature dormant. The forecast's own
`resourceID` stays canonical so it can still be stamped onto findings keyed by
canonical resource ID.
Persisted Patrol run analysis is a display-safe summary, not an unchecked raw
model transcript. Infrastructure status and actions-taken text in run history
must be reconciled from the accepted structured Patrol result: accepted
findings, resolved finding IDs, run errors, and user-visible finding counts.
For a scoped run, the persisted status and findings summary count only active
findings reported or explicitly assessed by that run. Related hosts, agents,
and other supporting resources may remain in the effective diagnostic context,
but their unrelated findings cannot make an exact-target all-clear appear
unhealthy. A tool-free Watch negative control is valid only when its persisted
status, canonical summary, finding counts, and finding IDs all agree that the
run was healthy.
Rejected, failed, or attempted tool calls must not appear as actions taken, and
healthy/no-finding runs must not claim that an unaccepted
`patrol_report_finding` action occurred. Backup/PBS seed context must preserve
the source identity behind backup observations, including PBS instance,
datastore, and namespace when those fields are available, so model-authored
key observations can name the affected backup source rather than saying only
"PBS".

The public Pulse Intelligence overview is a projection of those runtime
contracts, not a second Assistant tool inventory. It must point readers at the
registry-owned Assistant tool governance and the manifest-owned external-agent
capabilities instead of carrying hand-maintained tool tables.
It is also the public product-language contract for paid Patrol capabilities:
Pro positioning must describe hands-on Patrol modes, issue investigation,
governed fixes, verified outcomes, and history. Public docs and generated
commercial copy must not reintroduce proof-first activation-loop framing or the
retired `Patrol Control Levels`, `Patrol control`, and `Alert investigation`
labels as the visible paid story. API fields and route markers may retain
compatibility names such as `autonomy_level` only when the surrounding copy
plainly calls the operator-facing choice `Patrol mode`.
Its Core/Patrol/Assistant/MCP relationship block must be rendered from
`PulseIntelligenceOverviewMarkdown(Manifest.SurfaceContract)` rather than
maintained as prose separate from the agent capabilities manifest.
Reusable workflow starters are manifest-owned `workflowPrompts` metadata
derived from the shared Pulse Intelligence workflow-prompt core. Prompt names,
Assistant display labels, presentation kind hints, descriptions, arguments,
availability, and rendered text belong to that shared catalogue. Pulse
Assistant-compatible starters, frontend manifest clients, MCP `prompts/list`,
and generated docs consume that catalogue instead of maintaining per-surface
prompt lists or browser-local starter copy.
The shared catalogue includes the global `pulse_operations_loop` workflow
starter for the Patrol mode operations loop: it is projected only when the
manifest has fleet context, resource context, findings, governed
plan/decide/execute action, and finding resolution capabilities. Its rendered
prompt must keep the operator in an approve-or-reject loop with execution only
after policy allows it and verification before finding resolution, while the
status schema treats Patrol mode starter evidence only as entry-point
orientation, Patrol mode completed-loop evidence as aggregate terminal
approve/reject proof, and Patrol mode resolved-loop evidence only as
aggregate approved-and-verified outcome proof, not Assistant transcript, Patrol
finding identity, approval payload, action command, or MCP setup detail.
`pulse_patrol`, the current paid `patrol_control` marker, the legacy
`patrol_autonomy` marker, and the legacy `pulse_pro_activation` alias all
contribute to first-party Patrol mode starter evidence;
`proActivationOperationsLoopStarterCount` is a compatibility field retained
for older external-agent clients, while primary clients must read
`patrolControlOperationsLoopStarterCount`. The other `proActivation*` status
fields are compatibility aliases for the same first-party Patrol mode
journey rather than a separate AI-runtime loop, and manifest descriptions must
not describe them as a Pro activation journey.
The same status schema must expose the aggregate Patrol mode value state as
a content-safe enum so Assistant-compatible and MCP-facing agents can branch
without reverse-engineering counts. `governed_decision_recorded` is a safe
terminal decision state, not proven operations value; `verified` is the only
completed first-party value state and requires the approved, verified outcome
plus recorded action history. MCP/external-agent readiness remains optional
external-agent setup context.
Any first-party workflow starter surface is a projection of that same
catalogue, not a second prompt registry (the browser Assistant that hosted the
starters was retired on 2026-10-22): it may decide which manifest-owned prompt
is contextually usable, but display labels and rendered prompt text must come
from shared manifest metadata and the shared backend renderer on
`POST /api/ai/workflow-prompts/render`.
AI provider integration is registry-owned. Provider identity, display names,
protocol family, default model, default base URL, credential fields,
configured-state fields, clear-key fields, env-var hints, docs links, and
fallback catalog models belong in the config-layer provider registry, not in
factory branches, settings-card literals, or per-provider model-list code. New
direct chat-compatible providers extend that registry and the shared
chat-compatible provider client; native transports remain explicit only where
the protocol is not chat-compatible.
Provider definitions that declare a BaseURLField (today OpenAI, Ollama, and
Z.ai) expose a user-overridable endpoint via the AI settings payload; the
registry default base URL applies when no override is stored, so one provider
can serve both standard and alternate (e.g. Z.ai coding) endpoint tiers.
Patrol-blessed quickstart models are registry-owned the same way. For a
provider whose model catalog is user-supplied (today Ollama), the suggested
Patrol model, its hardware-expectation note, and its equivalent catalog tags
live on the provider definition in `internal/config/ai_providers.go` and
project through `/api/settings/ai`; settings surfaces render that projection
and must not hardcode blessed model IDs. A blessing must be verified against
Patrol's real tool-call preflight before it ships or changes; the env-gated
`TestManualOllamaBlessedModelPreflight` harness in `internal/ai/` is the
re-blessing procedure, and connectivity or catalog presence alone is not
verification (qwen3:4b passed connectivity yet emitted a tool call in 0 of 4
preflight runs, which is why only qwen3:8b is blessed). Recommended-model
resolution may prefer blessed models only by exact normalized catalog ID
match, never by family or substring, so cloud-gateway catalog neighbours of
the same model family are not promoted by fuzzy matching.
Native provider tool-calling is a transport projection over the shared
provider-neutral `Tool`, `ToolChoice`, `ToolCall`, and `ToolResult` shapes.
Nil `ToolChoice` means provider-owned automatic selection, `none` disables
tools by omitting them from the provider request, and `required` is reserved for
callers that explicitly need provider-native forced tool use. That required mode
maps to OpenAI `required`, Anthropic `any`, and Gemini
`FunctionCallingConfig` `ANY`; normal chat and Patrol runs must not force tool
use unless the caller sets that override. Patrol's static model-readiness
self-test may use required mode for Gemini because it is a fixed capability
probe with no infrastructure identifiers and needs deterministic tool-call
proof. Provider-specific schema restrictions belong at the native transport
edge: Gemini strips unsupported `additionalProperties` only from Gemini function
declarations, while the registry-owned provider-neutral schemas and providers
that support stricter JSON Schema keep that field. Gemini function-call
continuations must preserve the provider `functionCall.id` on parsed
`ToolCall.ID` values and return that same value as `functionResponse.id`; the
function name remains a separate field resolved from the preceding assistant
call when building tool-result turns.

Local subscription-agent providers are explicit non-chat-compatible native
transports owned by `internal/ai/providers/subscription_agent.go`. Their
configured state is an operator opt-in, not possession of a Pulse-managed
credential. Readiness requires the corresponding CLI on `PATH` and the CLI's
own first-party subscription login. The transport must never read, persist,
refresh, export, or forward that login, and its child environment is an
allowlist that excludes API keys, Pulse secrets, cloud credentials, and
unrelated tokens. It must never fall back to an API-key provider.

CLI discovery is scoped to the Pulse service identity: the service `PATH`,
that identity's `.local/bin`, `bin`, and `.npm-global/bin`, or an explicit
absolute `PULSE_CLAUDE_CLI_PATH` / `PULSE_CODEX_CLI_PATH` override. It must not
search or expose another user's home. Missing executables and missing
first-party login are typed local-setup failures, not provider reachability
failures, and provider-test plus Patrol diagnostics must tell the operator to
install and authenticate the CLI as the account running Pulse (#1742).

Each subscription-agent call is a single structured provider turn in a fresh
temporary working directory. Codex runs ephemeral with user configuration
ignored and a read-only sandbox. Claude runs without session persistence or
customizations, with all Claude Code tools disabled and a non-interactive
permission mode. Its actual CLI system prompt is the bounded Pulse adapter
contract; the serialized provider request is supplied separately as input
data. The adapter treats the request's Pulse-owned `system`, `tools`, and
`tool_choice` fields as trusted control-plane fields while treating
infrastructure names, metadata, logs, command output, and tool results inside
the message history as untrusted evidence. It explicitly defines returned
`tool_calls` as routing decisions that Pulse may validate and execute, not as
Claude Code tool activity. This separation must not collapse back into a user
prompt that labels Pulse's own system instruction untrusted or activates a
coding/plan-mode persona. Neither route receives Pulse MCP configuration. The
agent returns tool arguments as native JSON objects. The structured-output
schema binds every selectable tool name to that declared tool's input schema;
it forbids tool calls when none are offered or tool choice is `none`, and
requires at least one when tool choice is `required`. That output schema is a
deep-copied transport projection rather than an alias of the native Pulse tool
schema: every projected object is closed, every projected property is required,
and fields that remain optional in the native schema are nullable only at the
strict transport boundary. Before normal Pulse validation or execution, the
adapter recursively removes null placeholders for those originally optional
fields while preserving required nulls for the native validator to decide.
No-tool turns use the same strict empty-object input contract, and neither
projection nor cleanup may mutate the registry-owned native schema. Pulse then
rejects unknown tool names, duplicate or empty call IDs, malformed argument
objects, and violations of `none` or `required` tool choice before the existing
provider-neutral tool loop can execute anything. The normal registry, profile, approval, protected
resource, action, and verification contracts remain the only infrastructure
authority. Calls are serialized per local subscription agent and bounded by
the configured request timeout and prompt/output size limits, with a two-minute
minimum configured request allowance for local subscription turns. Because the
CLI response is buffered and cannot expose intermediate chunks, a positive
provider `StreamIdleTimeout` is an earlier complete-turn deadline for this
transport; the configured minimum must not override that caller-owned Patrol
stall budget. Command cleanup after cancellation or process exit is also
bounded: a subscription CLI descendant that inherits an output pipe must not
extend the caller-owned deadline until that descendant exits independently.
Patrol tool-call
preflight owns one route-aware outer deadline: API providers retain the
30-second budget, while local subscription agents receive a bounded two-minute
budget for CLI startup and complete structured-output assembly, and Ollama or a
custom OpenAI-compatible endpoint receives the configured provider request
timeout for cold local-model loading. A caller's earlier cancellation still
wins. Qualification waits through the applicable local-provider deadline plus
bounded cache-publication grace before it may classify missing fresh preflight
evidence; its shorter hosted API-provider wait must not truncate a healthy
local CLI or local-model turn. The native provider boundary accepts the
canonical qualified Pulse model identity from shared callers, strips only its
own subscription-provider prefix before CLI execution, and rejects foreign
provider prefixes rather than forwarding an invalid or cross-provider model
name to the local agent.
Claude uses its native output-schema mode for every provider decision. Pulse
consumes Claude Code's verbose JSON event stream but still buffers the whole
process result before emitting provider events. Claude Code can mistake a tool
name serialized in the Pulse request for a local native tool even though local
tools are disabled; in that case it emits the intended `tool_use`, injects a
synthetic `No such tool available` result, and lets the model continue toward a
false `needs_attention` conclusion. The adapter must recover only the first
such call when its name is in the current Pulse-owned offered-tool manifest,
then return that call through the normal Pulse executor on the next provider
turn. It must ignore later attempts made without real Pulse tool results,
reject any undeclared native tool attempt, and never execute infrastructure
tools inside Claude Code. When no recoverable native call occurred, Pulse
decodes the terminal structured turn with unknown fields and trailing values
rejected. Live qualification also proved that Claude's wrapper may exhaust its
structured-output retries on the final turn after Pulse has already persisted
a successful `patrol_report_finding`, `patrol_assess_finding`, or
`patrol_resolve_finding` result. Only that typed terminal reason immediately
after that successful lifecycle result is normalized to a no-tool completion;
its reported usage is retained. The same wrapper failure before a durable
outcome, after a failed tool result, or for any other terminal reason remains a
provider error. This keeps the governed finding as source of truth without
turning prose-wrapped or incomplete tool decisions into healthy runs.
Patrol consumes the provider streaming interface, so the adapter projects each
fully validated CLI turn into canonical buffered `content`, `tool_start`, and
`done` events. It must emit nothing before the complete CLI response passes the
schema and tool-boundary checks. This is transport compatibility rather than a
claim of token-by-token CLI streaming; Pulse still owns tool execution,
`tool_end` progress, run-history persistence, and verification.
Codex JSONL event output is also a fail-closed audit channel: reported command,
file, MCP, web, computer, or image-tool activity invalidates the turn. This
detects a violated adapter instruction but cannot retroactively protect data a
same-user read-only process already read, so deployment guidance must require a
dedicated least-privilege Pulse OS account and must not describe the local
agent route as equivalent isolation to a remote chat-completions API.

`codex-subscription:*` and `claude-subscription:*` are distinct provider routes
in settings, readiness, model catalogues, usage, and qualification reports.
Qualification may temporarily opt in the selected route when applying an
explicit model override, must restore the previous opt-in and Patrol model even
after failure, and records `inference_route=local_subscription_agent` so a
subscription run is never compared or published as a metered API route.
Qualification cost evidence must preserve that route distinction. Unknown
pricing on `metered_api` remains a fail-closed dollar-budget failure. A
`local_subscription_agent` or `local_model_server` run instead records its
monetary cost as unknown with `budget_applicable=false`; it must not invent a
zero-dollar price, and latency, provider/plan errors, tool efficiency, and any
usage counts exposed by the transport remain scored and reportable.
The configured Z.ai `/api/coding/paas/` endpoint is likewise reported as
`coding_plan_allowance`, with unknown monetary cost and no per-run metered-API
dollar budget; the standard `/api/paas/` endpoint remains `metered_api`.

The provider-neutral agentic loop runs independent tool calls in parallel, but
parallelism must not erase a same-turn Patrol lifecycle dependency. When a
provider returns `patrol_get_findings` together with
`patrol_report_finding`, `patrol_assess_finding`, or
`patrol_resolve_finding`, Pulse executes that batch sequentially in the
provider's original order. It does not reorder an invalid write-before-read
batch, and it retains parallel execution for independent reads and independent
finding writes. This makes the existing read-before-write guard deterministic
without requiring the model to understand an internal goroutine schedule.

Patrol operational objectives are durable organization-scoped operator intent,
not prompts that exist only for one provider session. Core persists the brief,
optional context, canonical resource scope, lifecycle revision, and internal
observer record in `ai_patrol_objectives.enc` under the tenant data directory;
the file must decrypt successfully before any objective is loaded. Active
objectives are added to full and scoped Patrol seed context as desired outcomes,
not as trusted scripts or tool instructions. Scoped runs receive global
objectives plus only objectives whose canonical resource IDs intersect the
effective scope. Provider-bound resource-policy sanitization remains the final
transport boundary for the resulting context.

Objective coverage is core-derived. Saving intent starts `uncovered`; neither a
client nor a model may write `covered`. Coverage becomes `covered` only for an
internally recorded, read-only observer that traversed
`proposed -> validated -> installed` and holds an unexpired health lease. A
missing lease, expired lease, or observer failure is `degraded`; paused,
archived, missing, proposed, validated-but-uninstalled, and disabled observers
are `uncovered`. Observer artifact identity, SHA-256 digest, declared bounded
trigger kinds, version, state transitions, and machine failure code are
validated and persisted by core. Observer authority never includes an
infrastructure mutation capability.

Active retained objectives with a missing observer, or an observer disabled by
an intent edit, remain durable setup requests. The observer reconciliation loop
redelivers them through the existing objective-planning trigger after restart,
queue rejection, or a busy worker. It respects disabled or blocked Patrol and
never schedules a paused or archived objective. Admission rechecks the exact
stored revision after acquiring the run slot and atomically persists
`last_planning_attempt_at` before a provider call. Unfinished attempts retry at
the configured Patrol interval, independently of run-history retention. This
timestamp does not change the operator revision, edit time, or coverage, and an
operator edit clears it. A proposal, including a rejected or proxy proposal,
is an explicit model handoff and is not automatically replaced by recovery.

During the first-party Patrol detection profile only, the core may attach the
objective store through the narrow `patrol_propose_observer` tool adapter. The
model must address an active objective at its current optimistic revision and
provide a bounded measurable interpretation, one declared trigger kind,
canonical-JSON probe proposal, wake evidence, and canonical-JSON requirements.
Core—not model arguments—assigns observer identity and version, forces read-only
posture and `proposed` state, computes the SHA-256 artifact digest, and persists
the artifact inside the encrypted objective document. Public objective reads
and later Patrol seed context omit artifact contents. The detection profile
permits this Pulse-state proposal write explicitly; interactive Assistant and
Patrol investigation profiles do not. A successful proposal remains
`uncovered/observer_proposed`: this tool has no validator, installer, execution,
health-lease, or infrastructure-action authority. It remains a governed write
for invocation policy, but its returned revision and observer identity are the
authoritative persisted Pulse-state record. It cannot establish infrastructure
recovery. The objective identity and optimistic revision are supplied by core
and revalidated atomically by the store. The scoped objective mission has one
permitted handoff, after which its remaining turn explains that persisted
scheduling decision. Interactive and investigation profiles cannot invoke this
objective write. Real infrastructure outcomes require canonical independent
verification, regardless of later model reads or claims.
The proposal also carries a closed `evidence_fit` classification. `direct`
means the predicate itself measures the full retained outcome; `proxy` means it
is a useful correlated wake signal only. Core validates, installs, evaluates,
and leases both safe signal types, but `derivePatrolObjectiveCoverage` permits
only a healthy direct observer to become `covered`. A healthy proxy remains
`uncovered/observer_proxy`, and a missing fit on an older stored observer is
normalized conservatively to proxy. Semantic adequacy therefore stays
model-authored and improves with the model, while the deterministic core owns
the fail-safe rule that uncertainty cannot become a protection claim.

An observer wake must preserve the exact retained objective rather than
degrading it into a generic resource alert. `PatrolScope.ObjectiveContext`
carries the core-authored objective/observer identity, current objective
revision, local observation time, affected canonical resource IDs, and bounded
evidence together with the operator-authored brief and optional context. The
TriggerManager includes objective and observer identity in duplicate detection,
so two objectives on the same resource cannot be silently conflated. Watch
renders that exact snapshot as desired-outcome context, stamps it durably on
every finding created or refreshed by the run, and projects it through
`aicontracts.Finding` into Pro investigation. The Pro prompt quotes and bounds
operator-authored objective text as data, asks the model to determine what the
outcome requires from current evidence, and explicitly grants no tool choice,
mutation, approval, or policy authority. This is the retained-intent seam from
cheap local detection into model-owned diagnosis and governed action.

### Anthropic prompt-cache reuse of the stable system prefix (19 September 2026)

Provider-neutral `ChatRequest.System` stays the full system prompt. The optional
`SystemCacheablePrefix` marks its stable leading portion; the agentic loop
(`internal/ai/chat/agentic_prompt.go`) supplies the frozen base plus execution
mode text there and keeps the per-turn clock and accumulated knowledge after it,
so multi-turn runs can reuse the prompt instead of repaying it every turn
(#2118). When the prefix is an exact prefix of `System`, the Anthropic transport
(`internal/ai/providers/anthropic.go`) serializes `system` as content blocks with
an `ephemeral` cache breakpoint on the stable block and the volatile remainder as
an uncached block; otherwise it sends the plain string form. Anthropic caches in
tools then system then messages order, so the system breakpoint already covers
the tool definitions and the per-tool breakpoint is omitted when the block form
is used; providers without prompt caching ignore the field and send `System`
unchanged. The prefix is applied only when it is an exact prefix of the possibly
sanitized `System`, so a redacted secret cannot be reintroduced through the block
form; a mismatch falls back to the plain string. Cache creation/read counters are
logged at Info for production visibility. Proof: `TestBuildAnthropicSystem`,
`TestAnthropicClient_Chat_CacheableSystemPrefix`,
`TestAnthropicClient_ChatStream_CacheableSystemPrefix`,
`TestAnthropicClient_Chat_KeepsToolBreakpointWithoutCacheableSystem`, and
`TestAgenticLoop_SystemPromptPartsSeparateStablePrefix`. This does not change
model-facing content, tool exposure, or approval authority, and it does not claim
a measured cost reduction or installed-deployment cache-read rate.

## Canonical Files

- `pulse-enterprise:internal/investigation/orchestrator.go`: private investigation execution and current-occurrence follow-up context.

- `pkg/reporting/evidence.go`: measured retained-history evidence contract for model-facing summaries, with no report narrator or heuristic health judgement.

1. `internal/ai/`
   1g. `internal/ai/patrol_objectives.go`
   1e. `internal/ai/attention.go`
   1f. `internal/ai/patrol_metrics.go`
   1a. `cmd/pulse-mcp/main.go`
   1b. `cmd/pulse-mcp/README.md`
   1c. `internal/agentcapabilities/`
   1d. `scripts/generate-pulse-intelligence-docs.go`
2. `internal/config/ai.go`
   2a. `internal/config/ai_providers.go`
3. `internal/api/ai_handler.go`
4. `internal/api/ai_handlers.go`
5. `internal/api/ai_hosted_runtime.go`
6. `internal/api/ai_intelligence_handlers.go`
7. `frontend-modern/src/api/agentCapabilities.ts`
   7a. `frontend-modern/src/api/generated/agentCapabilities.ts`
8. `frontend-modern/src/api/ai.ts`
   8a. `frontend-modern/src/api/workflowPrompts.ts`
9. `frontend-modern/src/api/patrol.ts`
10. `frontend-modern/src/components/AI/AICostDashboard.tsx`
11. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx`
12. `frontend-modern/src/utils/aiCostPresentation.ts`
13. `frontend-modern/src/utils/aiProviderHealthPresentation.ts`
14. `frontend-modern/src/utils/aiProviderPresentation.ts`
15. `frontend-modern/src/utils/textPresentation.ts`
16. `frontend-modern/src/stores/aiRuntimeState.ts`
17. `frontend-modern/src/stores/aiAvailability.ts`
18. `docs/AI.md`
19. `pkg/aicontracts/investigation.go`
20. `pkg/aicontracts/orchestrator_deps.go`
21. `pkg/aicontracts/fix_execution.go`
22. `pkg/extensions/ai_autofix.go`
23. `internal/ai/qualification/`
24. `cmd/patrol-qualify/`
25. `tests/qualification/patrol/`
26. `docs/AI_PATROL_QUALIFICATION.md`

## Shared Boundaries

1. `cmd/pulse-mcp/main.go` shared with `api-contracts`: the Pulse MCP adapter runtime is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.
2. `cmd/pulse-mcp/README.md` shared with `api-contracts`: the Pulse MCP adapter guide is both an AI runtime surface for external-agent access to Pulse Intelligence and a canonical API contract projection over the agent capabilities manifest.
3. `frontend-modern/src/api/agentCapabilities.ts` shared with `api-contracts`: the agent capabilities frontend client is both the Pulse Intelligence external-agent manifest consumer and a canonical API payload contract boundary.
   Its presentation sanitizer may translate legacy Pro activation manifest
   descriptions into Patrol mode outcome/status language, but it must not
   reintroduce proof-first or activation-loop copy into Assistant, Patrol, or
   external-agent onboarding surfaces.
4. `frontend-modern/src/api/ai.ts` shared with `api-contracts`: the AI frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
5. `frontend-modern/src/api/generated/agentCapabilities.ts` shared with `api-contracts`: the generated agent capabilities frontend types are both the Pulse Intelligence manifest TypeScript projection and a canonical API payload contract boundary.
6. `frontend-modern/src/api/patrol.ts` shared with `api-contracts`: the Patrol frontend client is both an AI runtime control surface and a canonical API payload contract boundary.
7. `frontend-modern/src/api/workflowPrompts.ts` shared with `api-contracts`: the workflow prompt activity client is both the AI runtime workflow-prompt telemetry surface and a canonical API payload contract boundary.
8. `frontend-modern/src/components/Settings/AgentIntegrationsPanel.tsx` shared with `api-contracts`, `frontend-primitives`: the External agents settings panel is the optional settings-shell projection of Pulse MCP onboarding, the AI runtime connected-agent onboarding surface, and a presentation consumer of the shared agent capabilities frontend client.
   Its default copy must frame connected clients as optional connector access
   to Pulse context and Patrol work, with Patrol as the operator that watches,
   acts within Patrol mode, asks for approval when required, verifies outcomes,
   and records history. The External agents settings view must keep setup
   mechanics behind a `Show connector setup` disclosure, with direct setup
   links opening that disclosure automatically. Default copy must make clear
   that connected tools do not get separate powers. Tool-contract posture and manifest facts are
   client-builder diagnostics and must stay behind a Developer details
   disclosure rather than beside the normal external-agent setup steps; posture
   and policy context belongs under Patrol access model, while prompt, scope,
   failure-code, and tool inventories must sit one level deeper behind Live
   manifest details using user-facing labels such as Agent starting points and
   Agent capabilities.
   Direct links to
   `/settings/pulse-intelligence/agents#external-agent-setup` must focus this
   External agents panel after the settings layout settles,
   because connected agents are optional access to Patrol work and should not
   strand users at the generic token inventory. Legacy
   `/settings/security/api#external-agent-setup` and
   `/settings/security/api#pulse-mcp-setup` links must remain accepted and
   redirect to the canonical Pulse Intelligence External agents route.
   The `Choose Patrol mode` handoff must target the Patrol operator surface at
   `/patrol#patrol-control`, where the mode is configured, while API
   Access remains the token and external-client setup surface. The expanded
   setup checklist owns that handoff as Step 1; the panel header must not repeat
   a second visible `Choose Patrol mode` action while setup is open.
   The visible full-surface token preset for that setup is `Patrol external agent`;
   `pulse_intelligence_agent` remains only the compatibility preset id used by
   the route and token model.
9. `frontend-modern/src/stores/aiAvailability.ts` shared with `frontend-primitives`: the AI availability store is both the AI runtime session capability and the app-shell gate for Patrol entry points.
10. `internal/agentcapabilities/action_target.go` shared with `api-contracts`: the Pulse Intelligence governed action target type and resource-to-action-target mapping vocabulary are both the Assistant approval/runtime routing contract and the canonical API/agent target contract for governed actions.
11. `internal/agentcapabilities/control_level.go` shared with `api-contracts`: the Pulse Intelligence control-level vocabulary and control-tool availability predicate are both the Assistant runtime gating contract and the canonical API/agent permission posture for governed action tools.
12. `internal/agentcapabilities/errors.go` shared with `api-contracts`: the Pulse Intelligence agent error envelope is both the canonical API failure payload contract and the AI runtime adapter error-parsing contract for Assistant and external-agent surfaces.
13. `internal/agentcapabilities/events.go` shared with `api-contracts`: the Pulse Intelligence event vocabulary is both the canonical API SSE event contract and the AI runtime adapter notification contract for Assistant and external-agent surfaces.
14. `internal/agentcapabilities/governance_prompt.go` shared with `api-contracts`: the Pulse Intelligence surface-affordance-resolved model-facing operating-instruction, tool-governance prompt, reusable provider-tool governance description, Assistant-native offered-tool filtering, and Assistant-native interactive question-tool governance projections are both the Assistant system-prompt governance section and the shared API/agent vocabulary for action mode, approval posture, MCP affordance advertisement, and non-registry interaction-tool boundaries.
15. `internal/agentcapabilities/http.go` shared with `api-contracts`: the Pulse Intelligence agent HTTP substrate is both the API capabilities invocation contract and the shared AI runtime adapter execution primitive for MCP and reference agent clients.
16. `internal/agentcapabilities/invocation.go` shared with `api-contracts`: the canonical registry-owned invocation descriptors (per-tool discriminator, enum-exact case coverage, closed workflow-kind and mutation-target vocabularies, deep-copied lookups, fail-closed classification with unknown targets denied at policy evaluation) are both the native Assistant invocation safety-classification contract and the canonical API/agent governed-invocation policy contract consumed by provider projection and registry runtime enforcement; canonical tool names cannot carry descriptor overrides.
17. `internal/agentcapabilities/manifest.go` shared with `api-contracts`: the canonical Pulse Intelligence agent capabilities manifest declaration, including capability display titles, manifest-owned finding lifecycle schemas, manifest-owned governed action schemas and routes, manifest-owned external-adapter surface tool contracts, and manifest-owned structured output schemas, is both the API discovery payload source and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
18. `internal/agentcapabilities/markdown.go` shared with `api-contracts`: the Pulse Intelligence manifest Markdown projection, including manifest-owned capability titles, surface-filtered Pulse MCP tool/error inventories, and prompt labels, is both the canonical API/agent documentation projection and the AI runtime onboarding projection for Assistant-compatible external-agent surfaces.
19. `internal/agentcapabilities/mcp.go` shared with `api-contracts`: the Pulse Intelligence MCP protocol version, JSON-RPC, method dispatch, method payload, surface-tool-contract-gated initialize operating-instruction and capability advertisement payload, manifest surface-filtered tools/list and tools/call execution bridge, manifest surface-gated resources/list and resources/read bridge, manifest-owned and surface-affordance-gated workflow prompt projection, protocol wire aliases, resource and prompt handler gates, and notification projection collectively define the external-agent adapter wire contract over the shared Pulse Intelligence tool core; MCP initialize, tools/call execution, resource list/read projection, and prompt list/get projection must enter through manifest-owned surface and workflow-prompt contracts so raw capability slices cannot bypass the published external-adapter contract.
20. `internal/agentcapabilities/mcp_adapter.go` shared with `api-contracts`: the Pulse MCP adapter setup contract defaults and normalization are both the canonical API manifest setup projection and the AI runtime onboarding contract for Assistant-compatible external-agent surfaces.
21. `internal/agentcapabilities/patrol_scope_tools.go` shared with `api-contracts`: the typed Patrol resource-scope evidence mapping is both the AI runtime least-manifest projection contract and the canonical API/agent vocabulary that keeps scoped Watch and investigation tool reductions aligned across the Pulse and enterprise boundary.
21. `internal/agentcapabilities/projection.go` shared with `api-contracts`: the agent capability external-tool projection helper, normalized manifest-owned surface tool contract resolution and tools-affordance gating, manifest-owned resource-context route and argument vocabulary, operator-state capability and route vocabulary, finding workflow capability and lifecycle argument vocabulary including resolution and dismissal notes, governed action capability, route, and argument vocabulary, manifest-owned tool title and outputSchema projection, structured Pulse capability _meta, and shared tool behavior hints are both the canonical API manifest projection contract and the AI runtime adapter projection for Pulse Assistant and MCP-facing agent tools, with MCP annotation and metadata wire names confined to adapter-edge aliases.
22. `internal/agentcapabilities/provider_tool_artifacts.go` shared with `api-contracts`: the provider tool-call artifact detector and streaming tool-name prefix splitter are both the Assistant stream-sanitization boundary and the shared external-adapter leak guard for provider-native tool-call markup that escaped the structured channel.
23. `internal/agentcapabilities/schema.go` shared with `api-contracts`: the agent capability input schema contract is both the canonical API manifest schema envelope and the AI runtime structured tool-schema, governance-aware provider-projection with neutral behavior hints and Pulse governance metadata, offered-tool governance extraction for Assistant prompt policy, manifest-affordance-gated Assistant provider-surface composition, manifest raw-schema to Assistant provider-schema projection for capability tools, legacy native Assistant utility provider aliases and schemas, provider-call normalization, provider-result context projection, Assistant-native interaction provider-tool declaration, and live Assistant execution-normalization contract for Pulse Assistant and MCP-facing agent tools.
    Provider projection is closed by default (`additionalProperties:false`), provider-emitted internal approval metadata is rejected before registry execution, and the registry rejects undeclared public arguments before a handler runs. Interactive approved replays may carry only the non-serializable server-attached approval field. Public Assistant chat always installs an approval-gated request posture; caller JSON cannot promote the service to autonomous infrastructure authority, and autonomous model sessions cannot enter either the canonical or legacy raw-command executor.
24. `internal/agentcapabilities/scopes.go` shared with `api-contracts`: the manifest-derived required-scope summary is both the canonical API/agent token guidance contract and the AI runtime adapter startup/onboarding contract for Assistant-compatible external-agent surfaces.
25. `internal/agentcapabilities/sse.go` shared with `api-contracts`: the Pulse Intelligence SSE subscription transport and record parser are both the canonical API event-stream consumption contract and the AI runtime adapter push bridge contract for MCP and reference agent clients.
26. `internal/agentcapabilities/surface_contract.go` shared with `api-contracts`: the Pulse Intelligence operator-surface affordance contract, shared surface-affordance, surface-tool identity, Assistant surface tool filtering, normalized external surface tool resolver, surface lookup, affordance labels, and manifest-published external-adapter surface tool allowlist projection are both the canonical API manifest surface model and the AI runtime prompt and onboarding guardrail for Assistant and MCP-facing surfaces.
27. `internal/agentcapabilities/text_tool_invocation.go` shared with `api-contracts`: the Pulse Intelligence text tool invocation parser, internal approval argument, and current_resource handle vocabulary are both the Assistant approved-action execution projection and the shared tool-call params bridge for governed Pulse Intelligence tool calls, with MCP tools/call compatibility staying at the adapter edge.
28. `internal/agentcapabilities/tool_call.go` shared with `api-contracts`: the Pulse Intelligence shared tool-call params, normalization, validation, direct registry preparation, registry-entrypoint failure result helpers, and provider/registry tool-call safety classification are both the native Assistant execution contract and the canonical API/agent tools/call compatibility contract for governed Pulse Intelligence tool calls; the shared invocation-blocked result is the stable refusal for every profile-denied mutation (pulse-state and infrastructure alike).
29. `internal/agentcapabilities/tool_execution.go` shared with `api-contracts`: the Pulse Intelligence neutral capability tool HTTP execution helper and direct tool execution output/error mapper are both the Assistant-native direct execution contract and the canonical API/agent request/response execution contract, with MCP adapters consuming the neutral helpers only after the shared MCP manifest-surface execution bridge has applied the published surface tool contract.
30. `internal/agentcapabilities/tool_marker.go` shared with `api-contracts`: the Pulse Intelligence Assistant tool marker vocabulary and approval/policy marker parser are both the Assistant structured tool-result compatibility contract and the canonical API/agent branching contract for governed tool outcomes.
31. `internal/agentcapabilities/tool_names.go` shared with `api-contracts`: the Pulse Intelligence registry tool-name vocabulary is both the native Assistant execution/display contract and the canonical API/agent tool identity contract for MCP-facing external-agent adapters.
32. `internal/agentcapabilities/tool_response.go` shared with `api-contracts`: the shared tool response envelope, tool error-code vocabulary, and tool-result error-code and verification evidence parsers are both the Assistant structured tool-result contract and the canonical API/agent branching contract for Pulse Intelligence tool failures, recovery tracking, and write self-verification.
33. `internal/agentcapabilities/tool_result.go` shared with `api-contracts`: the Pulse Intelligence shared tool-result content/result envelope, structuredContent projection, result constructors, HTTP response-to-result mapping, text projection, and result interpretation helpers are both the Assistant registry result contract and the canonical API/agent result projection contract for governed tool outcomes.
34. `internal/agentcapabilities/transcript.go` shared with `api-contracts`: Stored Assistant tool results and product history share one result-bearing transcript contract, with an explicit narrower provider-request projection. Observed failures and absent historical result status must survive the API boundary.
34. `internal/agentcapabilities/types.go` shared with `api-contracts`: the agent capabilities manifest wire type, manifest-owned external-adapter surface tool contract field, capability display title and structured output schema fields, approval-policy vocabulary, capability governance normalization, and tool-governance descriptor shape are both the canonical API payload contract and the AI runtime projection contract for Pulse Assistant and MCP-facing agent tools.
35. `internal/agentcapabilities/workflow_prompt.go` shared with `api-contracts`: the Pulse Intelligence workflow prompt catalogue, manifest-owned `workflowPrompts` projection, MCP prompt title projection, presentation kind hints, shared resource-context and finding argument vocabulary, Patrol issue-handling capability gating, argument validation, and manifest-gated shared prompt rendering rules are both the AI runtime starter contract for Assistant-compatible surfaces and the canonical API/agent prompt projection contract for MCP-facing clients.
36. `internal/api/ai_handler.go` shared with `api-contracts`: Pulse Assistant handlers are both an AI runtime control surface and a canonical API payload contract boundary.
37. `internal/api/ai_handlers.go` shared with `api-contracts`: AI settings and remediation handlers are both an AI runtime control surface and a canonical API payload contract boundary.
38. `internal/api/ai_intelligence_handlers.go` shared with `api-contracts`: AI intelligence handlers are both an AI runtime control surface and a canonical API payload contract boundary.
39. `pkg/aicontracts/action_broker.go` shared with `api-contracts`: the public typed action-proposal broker contract is both an AI runtime proposal boundary (the only sanctioned Patrol route to an infrastructure mutation) and a canonical API dependency contract over the shared action lifecycle service.
40. `pkg/aicontracts/fix_execution.go` shared with `api-contracts`: the public approved-fix execution contract is both an AI runtime approved-action boundary and a canonical API dependency contract for Patrol and enterprise auto-fix binders.
41. `pkg/aicontracts/investigation.go` shared with `api-contracts`: the public Patrol investigation record and finding contract is both an AI runtime handoff boundary and a canonical API payload contract for Patrol, Assistant, unified findings, persistence, and audit surfaces.
42. `pkg/aicontracts/orchestrator_deps.go` shared with `api-contracts`: the public investigation orchestrator dependency contract is both an AI runtime handoff boundary and a canonical API payload contract for Assistant and Patrol tool-call history.
43. `pkg/extensions/ai_autofix.go` shared with `api-contracts`: the enterprise auto-fix extension dependency seam is both an AI runtime approved-action boundary and a canonical API extension contract over Assistant and Patrol execution dependencies.
44. `scripts/generate-pulse-intelligence-docs.go` shared with `api-contracts`: the Pulse Intelligence manifest docs generator is both an AI runtime docs/onboarding projection and a canonical API contract projection over the agent capabilities manifest and Pulse MCP surface tool contract.

The shared agent-capabilities manifest also owns the runtime surface contract:
the manifest wire type and generated frontend projection must name Pulse
Intelligence Core as the shared core, Pulse Patrol as the primary built-in
operator, and Pulse Assistant plus Pulse MCP as contextual and external-agent
access paths over the same governed capabilities. Frontend copy, MCP docs, and
Assistant onboarding must consume that relationship from the manifest rather
than carrying parallel surface tables.
The same manifest owns Pulse MCP adapter setup facts: server name, command,
base URL flag/default, token environment variable, and supported client config
families. Settings, README generation, and adapter onboarding may present those
facts for operators, but they must not maintain separate MCP setup constants.
The in-app setup surface for those facts belongs under Pulse Intelligence
settings because Pulse MCP is an external-agent access path over governed
Patrol actions. API Access may be linked for scoped-token creation, but it must
not host the external-agent setup walkthrough.
Pulse Intelligence telemetry readiness for the external-agent surface follows
that same manifest surface boundary: a non-expired API token covering any
Pulse MCP-published capability scope is enough to mark MCP/external-agent
readiness, while route-level usage still comes only from authenticated
capability activity markers. Readiness must not be tied to the full manifest
scope set because least-privilege external-agent setups are supported.
The `pulse-mcp` adapter marks its own capability requests with the content-free
`X-Pulse-Agent-Surface: pulse_mcp` header so telemetry can distinguish adapter
use from direct external-agent API use without recording prompts, payloads,
route parameters, token identity, or resource identifiers.
Successful workflow starter rendering follows the same content-free boundary:
native Assistant render calls, first-party paid Patrol autonomy handoffs, and
Pulse MCP `prompts/get` calls may persist only the manifest prompt name, coarse
surface (`pulse_assistant`, `pulse_patrol`, `patrol_autonomy`, legacy
`pulse_pro_activation`, or `pulse_mcp`), and timestamp. The first-party
activation marker is `POST /api/ai/workflow-prompts/activity`; it is an
authenticated `ai:chat` route for app-owned surfaces such as the paid Patrol
autonomy handoff into Patrol, validates prompt names against the manifest
workflow prompt catalogue, accepts only first-party surfaces, and must remain a
starter marker rather than proof that Assistant context, governed action
approval, execution, verification, or finding resolution occurred.
`pulse_pro_activation` exists only as a legacy alias for older Pro
success/current-plan entry points; current paid handoffs use `patrol_autonomy`
and neither surface adds a new prompt, tool, model surface, or operations-loop
completion signal. Prompt arguments, rendered text, resource
IDs, finding IDs, session IDs, token identity, request bodies, and model output
must not enter workflow starter activity history or outbound telemetry.
Runtime model-facing instructions must consume that same contract as data.
`BuildPulseAssistantOperatingInstructions` identifies Pulse Assistant as the
native in-app surface over Pulse Intelligence Core, while manifest-backed MCP
initialize responses identify Pulse MCP as the external-agent adapter over the
same core and Pulse Patrol as the primary built-in operator. That copy belongs in
`internal/agentcapabilities`; Assistant prompts, MCP initialize payloads, and
future agent surfaces must not re-create local Assistant/MCP/Patrol relationship
wording outside the manifest-owned surface contract.
Surface tool summaries follow the same shared-core rule. The native Assistant
runtime exposes provider tools through `ProjectPulseAssistantProviderTools`,
which applies the manifest-owned Pulse Assistant surface affordances before
registry or native interaction tools reach model requests, and exposes
`PulseToolExecutor.AssistantSurfaceToolContract`, which projects the same
runtime-available provider tools through `ProjectPulseAssistantSurfaceToolContract`;
external-agent adapters project request/response manifest capabilities through
`ProjectManifestSurfaceToolContract` and publish the static external-adapter
inventory on `Manifest.SurfaceToolContracts`. Missing external-adapter
`surfaceToolContracts` entries fail closed rather than inferring tools from raw
manifest capabilities; static surface-tool affordance metadata may narrow an
external operator surface, but must not re-enable a disabled surface affordance
or keep tool/capability names when the effective `tools` affordance is false.
Disabled Assistant affordances likewise fail closed rather than allowing runtime
registry availability to re-enable provider tools.
Assistant prompt fallback governance follows that same contract: when no live
executor-owned offered-tool manifest is available, registry-owned fallback
governance must enter through
`CanonicalToolGovernanceForManifestSurface(ControlLevelControlled, CanonicalManifest(), SurfaceIDPulseAssistant)`,
and the chat prompt fallback must derive explicit offered tool names from
`ManifestSurfaceAffordances` so registry tools and the native `pulse_question`
interaction tool cannot be reintroduced by nil offered-tool semantics.
Assistant registry tools, Assistant-native interaction tools, and MCP manifest
capabilities must stay explicit buckets in that contract so Pulse can support
both surfaces without duplicating tool logic or
implying MCP replaces the in-app Assistant.
Frontend consumers must import the generated `SurfaceToolContract` shape from
the shared agent-capabilities client rather than maintaining a local copy.
Patrol work contract availability for browser surfaces is also a shared
agent-capabilities-client verdict: consumers must derive the Pulse MCP contract
from the normalized MCP adapter setup, the manifest-owned
`pulse_operations_loop` workflow prompt, and the Pulse MCP surface tool contract
through `frontend-modern/src/api/agentCapabilities.ts`; feature shells must not
hard-code MCP contract availability or infer it from raw manifest capabilities.
Runtime external-agent readiness is stricter:
`GET /api/agent/patrol-control/status` may mark `externalAgentReady` only when
that contract is available and a single non-expired API token covers every
scope required by the published Pulse MCP Patrol work capability set.
`GET /api/agent/operations-loop/status` remains only a compatibility alias for
older clients. That shared Patrol work tool set starts with
`get_patrol_control_status`, the count-only
`GET /api/agent/patrol-control/status` orientation read, then uses the same
fleet-context, resource-context, finding, governed action, and finding
resolution tools as the rest of the governed issue-handling flow. The
structured `list_resource_capabilities` read is the companion to
resource-context for action planning: it returns the per-resource
governed capability names and parameter schemas an agent needs to
populate `plan_action` inputs without guessing, where resource-context
renders capabilities only as count-limited prose facts.
`get_operations_loop_status` and `/api/agent/operations-loop/status` are
compatibility aliases, not primary capability names. AI runtime prompts, MCP
readiness helpers, and frontend presentation helpers must not carry separate
Patrol work inventories. The native Patrol work surface may display that same
status projection, but it must fetch it through the shared agent-capabilities
frontend client so Assistant, MCP, and native UI stay on the same Patrol work
contract. The manifest-owned status schema must describe governed counts as
decision-backed, split approved and rejected decision counts explicitly, and
describe verified counts as approved-action-backed. The same schema now exposes
content-free Patrol work starter counts for the total flow and the
Assistant, Patrol mode, older-client compatibility field, and Pulse MCP
source surfaces, an Assistant step count for contextual Assistant or
external-agent collaboration, plus a content-free Patrol mode
completed-loop count derived when Patrol mode starter evidence, Patrol
issue evidence, contextual collaboration, and either a rejected governed
decision or an approved governed decision with verified outcome proof coexist
in the same status window. It also exposes a stricter Patrol mode
resolved-loop count only when that status window has an approved governed
decision and verified outcome proof. The `proActivationOperationsLoopStarterCount`
field is retained only for older clients; primary clients must use
`patrolControlOperationsLoopStarterCount`. The legacy `patrolAutonomy*` and
`proActivation*` completed-loop, resolved-loop, and value-state fields mirror
the Patrol mode counts and value state for compatibility. AI runtime
surfaces may use those counts to show that a starter
was rendered or launched, that contextual collaboration occurred, or that the
first-party Patrol mode
loop reached terminal or resolved proof, but must not treat them as Assistant
transcript, approval payload, action command, resource identity, or
finding-resolution detail. External-agent prompts and MCP adapters may treat a
rejected-only decision as a terminal no-execution outcome, but approved
decisions must still continue through execution and verification before the
verified-outcome stage is complete.
Runtime Assistant availability remains an authenticated Assistant concern,
served by `GET /api/ai/assistant/surface-tools`, not a static fact in the
public agent-capabilities manifest. A client that displays live capability
availability must load that authenticated endpoint, and a browser client formats
the posture through `frontend-modern/src/api/agentCapabilities.ts`; no client
may keep a local registry-tool catalogue or reuse MCP manifest capability names
as native Assistant inventory. The External agents settings panel may show the
external Pulse MCP tool posture, but it must get the normalized static
`SurfaceToolContract` from `manifest.surfaceToolContracts` through
`getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP)`
and format it with `getAgentSurfaceToolPosturePresentation`;
external-adapter request/response filtering, including the `subscribe_events`
streaming exception and any capability omitted from the published Pulse MCP
surface allowlist, must be owned by the backend projection in
`ProjectManifestSurfaceToolContracts`; shared capability reads must use
`ResolveManifestSurfaceToolContract` and `ManifestSurfaceToolCapabilities` so
the normalized surface contract and the resolved `tools` affordance gate are
applied once. MCP initialize must also advertise the `tools` capability and
name "offered tools" in operating instructions only when that same published
surface tool contract resolves. MCP prompt support must similarly enter through
`MCPManifestSurfacePromptProjectionSupported`, which combines the target
surface's `prompts` affordance with the manifest-owned workflow prompt
catalogue before initialize or `prompts/list` advertises prompts, and through
`GetMCPPromptFromManifestSurface` before `prompts/get` renders any prompt. The
global `pulse_operations_loop` prompt is part of that same shared catalogue,
not a native Assistant-only starter: it must appear in Assistant and MCP only
when the manifest can support the full Patrol-to-Assistant-to-governed-action
flow, and it must guide contextual explanation, plan-first governed action,
approval or rejection only when the returned policy requires a decision,
policy-allowed execution, post-action verification, and finding resolution from
the same rendered prompt text. The rendered prompt must orient
first through `get_patrol_control_status` and re-read that status, resource
context, and findings before treating an outcome as verified. Successful render activity for
that starter is activation evidence for the Patrol issue-handling journey, not
proof of contextual collaboration, governed action execution, verification, or
finding resolution.
The MCP `tools/call` execution
helper must
accept the manifest and surface id, apply the same affordance and surface-tool
allowlist used by `tools/list`, and only then delegate to neutral capability
HTTP execution; it must not accept a caller-supplied raw capability slice as the
MCP execution boundary. Frontend consumers may normalize a published
`surfaceToolContracts` entry for presentation, but must not infer MCP tools
from raw manifest capabilities when that entry is missing, and must not add a
Pulse-MCP-specific frontend helper alias around the generic surface resolver.
The generated Pulse MCP README follows the same rule: its surface-contract
block must be rendered by `MCPSurfaceContractMarkdown` from
`Manifest.SurfaceContract`, alongside the manifest-derived scope, tool, prompt,
and error-code inventories.
The public Pulse Intelligence overview follows the public-docs version of that
rule: the marked overview block in `docs/AI.md` must be rendered by
`PulseIntelligenceOverviewMarkdown` from the same manifest-owned
`SurfaceContract`, while surrounding product detail may remain human-authored.
The External agents settings panel follows the browser-facing version of
that same rule: its Pulse Intelligence Core, Patrol, Assistant, and MCP surface
summary must be projected from `/api/agent/capabilities.surfaceContract` through
`frontend-modern/src/api/agentCapabilities.ts`, not maintained as panel-local
copy; surface affordance badges are part of that same manifest projection. Its
capability-publication wording must describe published manifest-owned surface
contracts, not raw backend capability rows, so the panel does not imply that
adding a raw capability automatically exposes it to Pulse MCP.
Its external-agent recovery summary must also be manifest-backed: stable
failure-code chips or summaries may be rendered for setup ergonomics, but they
must derive from capability `errorCodes` through the shared frontend manifest
client rather than a panel-local recovery-code list.
External-agent token setup copy in that panel must route operators back to the
API Access token creation surface and the manifest-derived `Pulse Intelligence
agent` preset instead of carrying MCP-local token minting logic or a
component-local required-scope list.

## Extension Points

The published qualification schema accepts either literal summary terms or
non-empty groups of equivalent terms, matching the runtime expectation shape.
A full JSON Schema catalogue regression runs in CI alongside Go validation.
It must reject missing summary and evidence requirements and empty term groups.

The service-storage qualification driver owns a fixed 8 MiB container tmpfs.
Its fill operation checks the prepared container identity and exact ownership
labels, validates filesystem type and capacity, refuses existing fill files,
and requires both a real ENOSPC write failure and independently measured zero
free bytes. Model output, Pulse metrics and the service healthcheck cannot
establish capacity ground truth. Recovery and exact-label cleanup preserve
pre-existing inventory. The opt-in Docker oracle test makes no provider or
Pulse API call and cannot qualify diagnosis. Host/storage-pool, backup and
missing-source-access coverage remain separate required scenarios.

Patrol model qualification extends only through
`internal/ai/qualification/`, `cmd/patrol-qualify/`, and reviewed manifests
under `tests/qualification/patrol/`. A manifest's qualification repeat profile must be
capable of clearing the shared 95% Wilson lower-bound gate at a perfect pass
rate; manifest validation rejects statistically impossible profiles before any
live fault is provisioned. `live-suite` may sequence every manifest
for one explicit track, but it must acquire and freshly preflight one exact
provider/model route before provisioning any scenario, retain that route for
every repetition, and restore it once after the complete suite. Scenario
runners assert the leased route and must not rewrite model settings or trigger
their own provider preflights. On a fresh or intentionally disabled install,
the suite lease must temporarily enable Pulse Intelligence in the same atomic
route update so its route-specific preflight can run, then restore the original
enabled state together with the model and subscription settings when the suite
closes. The suite acquisition deadline must mirror the
runtime's route-aware preflight deadline, including the configured request
timeout for Ollama and custom OpenAI-compatible endpoints, plus bounded cache
publication grace; it must not restore a changed route while a permitted cold
local-model preflight is still running. A fresh suite preflight failure aborts setup once
as qualification-infrastructure evidence; cached failure state must never be
recounted as multiple model or scenario failures without independent provider
invocations. `live-suite` must retain the existing live-fault and separate
remediation authorization gates for every run. Voluntary community evidence
must be a local, mode-0600, allowlist-only derivation: it may carry aggregate
scores, safety booleans, cost/latency, exact model/provider, scenario and
runtime provenance, a pre-run public challenge, and source content digests. It
must not copy raw findings, resources, topology, endpoints, logs, prompts,
model output, tool identity or payloads, action identity or payloads, or error
prose, and creating it must never perform a network upload. Community evidence
may shortlist models but cannot satisfy Pulse certification or hosted-model
selection without a separately reproduced controlled-lab campaign.
Patrol finding-report authoring guidance extends through the registered
`patrol_report_finding` schema's required-argument list. Normal and bounded
final-decision prompts must render that shared list and require every parallel
report call to be independently complete; they must not maintain a second
hand-written field list. Runtime may let a model recover after a rejected call,
but formal qualification continues to reject every unsuccessful tool call,
including a recovered schema-validation lapse. When one parallel lifecycle
batch has both accepted and rejected siblings, the accepted writes must not
force the normal text-only summary before one bounded repair-only turn. That
turn may use only the lifecycle tools needed to correct rejected calls, must
preserve accepted calls without repetition, and may extend the normal Watch
turn budget by at most one provider call.

Assistant control wording must identify the effective scope as Assistant chat
only. Patrol autonomy and global Actions remain separate authority surfaces;
the chat pill may display the effective `control_level` but must not imply that
read-only chat demotes Patrol or infrastructure action policy.

The manual Patrol route is an extension boundary for scoped work. `POST
/api/ai/patrol/run` (`HandleForcePatrol`) accepts an optional scope body
(`resource_ids` and/or `resource_types`, plus optional `alert_identifier`,
and `alert_type`) to run a manual Targeted check through the same scoped engine
and run record as automatic alert-triggered work; with no body it keeps the
fleet-wide Patrol check. Client-authored context is rejected. The scoped path must
reuse the existing scoped engine rather than adding a parallel trigger route,
honour the same Patrol readiness gate as a full run, bypass the full-run
cadence gate (targeted checks never consume a manual full-run allowance), and
carry resource identity only — no command, prompt, or remediation payload —
while the route keeps requiring admin plus `ai:execute` scope for both shapes.
Manual scoped admission atomically reserves the single Patrol slot and returns
its run identity before the provider goroutine starts. If another run owns the
slot it returns an honest typed conflict; automatic scoped triggers retain
their bounded internal requeue path.

Pulse Intelligence presents Patrol as the primary first-party operations
surface. Pulse Assistant is the in-app contextual explanation, approval-card,
governed-action, verification, and handoff access path over Patrol work.
`cmd/pulse-mcp/` is the external-agent adapter for MCP-speaking clients. The
adapter must stay thin: it fetches `/api/agent/capabilities`, projects that
canonical manifest as MCP tools, and forwards calls to the declared API routes.
The shared `MCPManifestToolServer` must therefore consume the manifest as a
unit, not only a detached capability slice, so initialize instructions, tool
projection, resources, prompts, notifications, and future manifest-backed
surface metadata remain coupled to the same discovery document. It must resolve
the target surface's affordance contract before initialize advertisement and
before tool, resource, and prompt handlers run; raw manifest capabilities,
workflow prompts, or context routes are availability inputs only after the
surface contract allows that affordance.
When an adapter renders a manifest prompt locally, it may notify Pulse through
the authenticated `POST /api/agent/workflow-prompt-activity` marker endpoint
only after the prompt render succeeds. That marker route accepts manifest-owned
prompt names only, requires the token to cover `monitoring:read`, normalizes
`X-Pulse-Agent-Surface: pulse_mcp` into the Pulse MCP surface, and records no
prompt arguments, rendered text, resources, findings, token identity, or
request payload.
`scripts/generate-pulse-intelligence-docs.go` is part of that coupling: it
must update the MCP README's Pulse Intelligence surface contract and the public
`docs/AI.md` overview from the manifest instead of leaving hand-written
Core/Patrol/Assistant/MCP relationship prose beside generated inventories.
Public AI docs may describe outbound usage telemetry only as counts,
feature flags, and coarse Patrol control and governed Pulse Intelligence
operations adoption flags and counters.
They must keep prompts, chat messages, command text, action output, token
values, resource identifiers, and hostnames explicitly outside that telemetry
scope.
Governed-operation workflow starter counts may appear in that outbound telemetry
only as 30-day aggregate counts for the canonical `pulse_operations_loop`
prompt, split by total, native Assistant, first-party Patrol, and Pulse MCP surfaces. Those counts
may make `pulse_intelligence_loop_active_30d` true because the operator entered
the guided journey, but they must not satisfy contextual-collaboration,
complete-loop, approved-execution, or resolved-loop telemetry by themselves.
Assistant cost-ledger events may carry only the coarse `context_scope`
classifier for product-originated finding, resource, handoff, action, or
structured-mention turns; telemetry may aggregate that classifier into
governed-context Assistant counts, but it must not export prompts, session IDs,
resource IDs, finding IDs, command text, or action payloads.
Assistant cost-ledger events may also carry only a count-only
`tool_call_count` for accepted, model-selected governed tool calls within the
turn. Telemetry may aggregate that number into Assistant governed-tool
collaboration counts, but it must not export tool names, tool arguments, tool
results, provider call IDs, transcript content, resource IDs, finding IDs,
command text, action payloads, or session IDs.
The Pulse MCP README and the in-app Agent integrations panel must present that
adapter as one reusable runtime contract with client-native config wrappers:
OpenCode uses a top-level `mcp` object in `opencode.json` / `opencode.jsonc`,
while Claude-style clients use `mcpServers`. The shared command, base URL, and
token environment variable remain the Pulse Intelligence contract; client file
formats are wrappers around that contract, not separate Pulse Intelligence
surfaces. In the in-app Pulse Intelligence settings panel, those copied wrapper
snippets are on-demand setup detail; they must not occupy the default
external-agent read above the Patrol-autonomy and scoped-token hierarchy. The
Security API Access page may mint the scoped token, but it must not become the
external-agent setup page.
Full-surface token scope guidance must come from the canonical manifest's
manifest-owned `requiredScopes` set through shared
`agentcapabilities.ManifestRequiredScopeList` /
`ManifestRequiredScopeMarkdownList` helpers rather than Assistant-, UI-, or
adapter-local hardcoded or recomputed scope copy.
The native Assistant runtime prompt, lifecycle logs, and readiness messages
must identify that first-party surface as Pulse Assistant rather than the
legacy generic `Pulse AI` persona; the shared engine/core vocabulary remains
Pulse Intelligence.
The canonical manifest declaration, manifest wire shape, action-mode enum,
registry tool-name vocabulary, structured registry tool schema, provider projection helpers, JSON Schema
object-envelope helpers, provider tool/call/result shapes, provider-call
projection helper, provider tool-name catalog exact/prefix matching,
provider tool-call artifact detection and streaming tool-name prefix holding,
provider tool-call input normalization that clones caller
maps and nested JSON-like argument values before initializing empty input
objects, provider-emitted streamed tool-input parsing, final provider
tool-input raw fallback handling, and native Assistant question input parsing,
control-level vocabulary, control-tool availability
predicate, tool-governance defaulting, disabled-control guidance, capability
lookup and typed lookup-error helpers, external-tool
projection helpers, and shared agent HTTP substrate, including named capability
execution, live in
`internal/agentcapabilities`; MCP JSON-RPC envelopes, method constants,
request decoding, line-delimited stdio request serving, notification response
policy, stable JSON-RPC encoding, manifest-backed tool-server semantics,
tool-server method dispatch, initialize instruction/tool-call/resource/prompt payloads,
tool-server initialize result construction, notification methods, MCP
`tools/call` raw params decode, MCP resource URI projection, context-backed
`resources/list` / `resources/read` projection, and manifest-backed
`prompts/list` / `prompts/get` projection live at the MCP adapter edge.
Native Assistant registry declarations are part of that same identity contract:
every registry `Tool.Definition.Name` for Pulse Intelligence tools, including
the read-only `pulse_summarize` reporting synthesis tool and Patrol runtime
tools, must consume `internal/agentcapabilities/tool_names.go` constants rather
than owning string literals in `internal/ai/tools`.
Shared Pulse Intelligence workflow prompt definitions, manifest-owned
`workflowPrompts` catalogue selection, prompt display labels, MCP prompt title
projection, presentation kind hints, prompt argument validation, and prompt
rendering rules live in the neutral workflow-prompt core so frontend manifest
consumers and external MCP prompts cannot drift.
When `workflowPrompts` is present on a manifest it is authoritative; capability
projection is compatibility plumbing for older manifests, not a second prompt
catalogue. Frontend consumers must not copy prompt labels, prompt bodies, or MCP `prompts/get` rendering
logic into frontend state. MCP prompt list/get helpers must likewise accept the
manifest-owned workflow prompt catalogue through `ManifestPulseWorkflowPrompts`,
`ProjectMCPWorkflowPrompts`, `GetMCPPromptFromManifestSurface`,
`GetMCPPromptFromManifest`, and
`BuildMCPPromptFromManifest`; they must not expose raw-capability `ProjectMCPPrompts`,
`GetMCPPrompt`, or `BuildMCPPrompt` entrypoints that can re-create a second MCP
prompt catalogue. Shared tool-call
parameter normalization and validation live in the neutral tool-call core: tool
names are trimmed and required, argument maps
are cloned deeply enough to detach nested JSON-like `body` maps/slices and
initialized, and malformed or semantically invalid tool-call params must fail
before capability lookup so native Assistant execution, the legacy text
projection, and external MCP adapters cannot drift on empty-name, nil-argument,
or caller-owned nested argument handling. Shared capability tool HTTP execution
and direct execution output/error mapping live in the neutral tool-execution
core, while result wrapping, result-text extraction, result interpretation, and
the canonical `HTTPCallResponse` to shared tool-result mapping live in the
neutral tool-result core. JSON object result bodies must also project into the
same shared structuredContent envelope while preserving the serialized text
content block for compatibility, so MCP clients and native Assistant paths see
one canonical result contract. Native Assistant direct execution paths must use that
shared text/marker interpretation and output/error projection instead of
flattening tool-result content blocks or branching on `isError`, approval, or
policy outcomes locally, so in-app execution and external MCP adapters
interpret the shared result envelope the same way. MCP `tools/call` decoding is
adapter-edge protocol work, but execution authorization must still enter the
neutral tool-execution core only after the shared manifest surface contract has
selected the allowed request/response capabilities. Patrol-only Assistant
registry tools that return JSON must build those responses through the shared
`agentcapabilities` JSON result constructors instead of manually marshaling JSON
into text-only results, so `structuredContent` remains available to both native
Assistant and external-agent result consumers. Patrol verification
tool-call records that decide success, failure,
approval-required, or policy-blocked outcomes must also branch through the
shared tool result interpretation rather than treating `isError` alone as the
success boundary. Reference clients that need a successful raw response body must also
use the shared request/response capability body helper so named capability
lookup, path/body projection, API-token headers, and stable non-2xx error
formatting stay in one place. The
agent SSE subscription transport, record parser, actionable-record filter, and
SSE-to-MCP notification bridge also live there so MCP notification bridges and
reference probes consume the same `Accept: text/event-stream`, status handling,
`event:` / `data:` framing rules, transport-event filtering, and JSON-RPC
notification projection.
The legacy
approved-action text projection for Pulse tool calls is also parsed there into
the shared `ToolCallParams` shape, including the internal approved-action
argument key/helper that carries a granted approval into replayed tool execution,
the `current_resource` handle plus governed aliases, and the recursive
tool-argument detector that blocks unresolved attached-resource placeholders
before visible execution; text invocations are then normalized and validated
through the same shared parameter contract, so
Assistant approval execution and MCP-compatible adapters cannot drift on tool
name, `default_api:` prefix, quoted argument handling, placeholder target
semantics, empty names, nil
arguments, or pre-approved action replay semantics. Structured tool
response envelopes, tool error-code vocabulary, the structured tool-result
error-code parser, and the `verification.ok` evidence parser also live there
so Assistant blocked/failed tool outcomes, provider call params, agent-facing
tool failure contracts and structured verification payloads cannot drift.
A tool verification payload is not independently verified action recovery. The legacy-compatible Assistant tool markers
for approval-required and policy-blocked outcomes also live there, including
the stable `APPROVAL_REQUIRED:` / `POLICY_BLOCKED:` prefixes, payload `type`
values, formatter helpers, parser helpers, and the typed approval-required
payload contract for plan, context-confidence, preflight, target, risk, and
description data. Tool families may own the resource-specific payload fields
they add to those markers, but they must not redeclare the marker envelope,
rebuild anonymous approval payload readers, branch on local prefix strings, or
hide shared marker parsing behind Assistant-local parser wrappers. Legacy
service approval-marker handling may keep compatibility fallback behavior for
old marker text, but the canonical decision path must call the shared marker
parser directly at the branch site. Tool execution must not drop the returned
argument map when carrying a granted approval into replayed tool execution.
Native Assistant tool protocol aliases must expose only the shared neutral
registry shapes the Assistant actually executes (`Tool`, `ToolCallParams`,
`ToolResult`, `ToolContent`, and structured tool-response envelopes). JSON-RPC,
MCP initialize/list/resource/prompt payloads, and other MCP wire aliases remain
at the external adapter edge in `internal/agentcapabilities/mcp.go`, not in
`internal/ai/tools/protocol.go`. Chat transcript and API-facing tool-result
shapes must alias the shared provider-call contract while the richer in-app
Assistant transcript projects to that shared shape through an explicit helper.
Provider-result context projection also belongs to `internal/agentcapabilities`:
native Assistant, the legacy AI service loop, external adapters, and future
Pulse Intelligence surfaces must build full transcript results and model-context
results through the shared projection so transcript preservation, error flags,
and model-result truncation cannot drift into surface-local pairs.
API handlers, Assistant tool governance, `cmd/pulse-mcp`, and the in-repo probe
consume that shared contract so agent projections cannot drift into local
structs, local structured schema types, local provider schema projection, local
schema wrappers, local request/response filters, local capability maps, local
path placeholder parsing, local argument-to-body shaping, local manifest
fetchers, local API-token header spelling, local stable error-envelope
formatting, local named capability execution, local tool-governance defaulting,
local disabled-control guidance, local MCP manifest-backed
tool-server handlers, local MCP method dispatch, local MCP request decoding or
line-framing loops, local JSON-RPC encoders, local notification response
checks, local MCP initialize builders, local
HTTP-to-MCP result wrapping, local MCP result interpretation,
local SSE record scanners, local SSE-to-MCP notification bridges, local
tool-result maps, or local tool-description builders. Path placeholders from the manifest and top-level
tool arguments must be projected through the shared
`agentcapabilities.ProjectCapabilityCall` helper before dispatch, so path
values are percent-encoded single segments and non-placeholder write arguments
become the JSON request body when no nested `body` object is provided.
Internal tool-call metadata such as approved-action grants must stay on the
shared `agentcapabilities` argument helper path and be stripped by the shared
projection before any public manifest-backed HTTP body is built.
Canonical IDs containing `:`, `/`, spaces, or other reserved characters cannot
alter the route shape.
The Assistant and MCP surfaces must also rely on the router-owned manifest
route/scope proof: every canonical capability is projected through the shared
call helper into a concrete request, then exercised against the API router with
a token that lacks the advertised scope so drift between manifest metadata,
agent-facing placeholders, and real authorization fails before either surface
ships a mismatched tool.
Native Assistant provider seams are part of the Pulse Assistant surface, not
the external MCP adapter. Chat service aliases, API handler setter signatures,
router dependency wiring, direct in-app tool execution, and findings-store
adapters plus the broader native tool adapter family must use Assistant/Pulse
Intelligence terminology; `MCP` names are reserved for the protocol adapter,
MCP-compatible action-contract interface, wire envelopes, and result
interpretation contracts.
The Pulse Intelligence event vocabulary is part of that same shared
`internal/agentcapabilities` boundary. API SSE producers, the
`subscribe_events` manifest description, MCP notification advertising, MCP
transport-event filtering, MCP notification method projection, shared SSE
record parsing, shared SSE-to-MCP notification bridging, and reference probes
must consume the shared event kind constants/helpers instead of maintaining local copies of
`finding.created`, `approval.pending`, `action.completed`, `stream.connected`,
or `heartbeat`.
MCP `tools/list` must use the shared
`agentcapabilities.ProjectManifestSurfaceTools` projection for the Pulse MCP
surface. The manifest-owned `surfaceToolContracts` field is the allowlist for
which request/response tools Pulse MCP may advertise or execute; raw
`Manifest.Capabilities` remains the metadata and execution source, not the
surface availability decision. MCP initialize capability advertisement,
operating-instruction affordance copy, `tools/list`, `tools/call`,
`resources/list`, `resources/read`, `prompts/list`, and `prompts/get` must also
pass through the manifest-owned surface affordance contract: the surface must
allow tools, resources, prompts, and capability metadata before raw manifest
capability or prompt presence can expose them to an external client. The shared
`agentcapabilities.ProjectTools`
helper still owns the full request/response tool shape, including
manifest-owned capability metadata (category, method/path, scope, action mode,
approval policy, request/response shape, stable error codes, input schema, and
structured output schema). That metadata must be available both in model-facing
description prose and in the standard MCP `_meta["pulse.capability"]` object
for clients that need structured route/scope/governance facts, so external-agent
clients see the same governed API contract without a second MCP-specific tool
registry, projection loop, allowlist, or prose table. Action mode (`read`,
`mixed`, `write`) and approval policy (`scope_only`, `action_plan`) are
manifest-owned governance fields; MCP must not infer or override them from HTTP
method or local tool names. Manifest-backed capability lookup and `tools/list`
projection must also detach raw `inputSchema`, raw `outputSchema`, structured
metadata maps, and error-code slices through `agentcapabilities.CloneCapability`,
`CloneCapabilities`, and `CloneRawMessage`, so external-agent adapters cannot
mutate the manifest-backed contract while serializing or post-processing one
projected tool list.
The manifest-owned `get_patrol_control_status` structured output schema is
part of the same AI runtime contract. Its four-step operator rollup is a
content-free stage projection for Assistant and MCP callers: governance step
counts represent pending approvals until an approved or rejected decision
exists, then represent decision evidence, while verification step counts
represent verified outcomes or terminal rejected decisions. Optional MCP
readiness remains a separate `externalAgentReady` capability signal, not an
operator step. AI runtime surfaces must consume that manifest schema and backend
projection instead of inventing chat-local stage-count rules. The schema's
Patrol control starter count includes native Patrol, current paid Patrol
control, and legacy Pro activation handoff starts; its
completed-loop and resolved-loop counts are aggregate orientation fields for
native Assistant and MCP-facing orchestration only. Legacy
`patrolAutonomy*` and `proActivation*` completed-loop, resolved-loop, and
value-state fields are aliases for the Patrol control values; the legacy Pro
activation starter field remains an entry-point-specific count. All of
those fields must remain free of prompt names, finding IDs, action IDs,
resource names, actors, request bodies, commands, or model output.
Resolved-loop semantics remain approved-and-verified only.
`cmd/pulse-mcp` startup guidance and the generated MCP README token-scope
block must consume the manifest-owned `requiredScopes` summary through
`agentcapabilities.ManifestRequiredScopeList` /
`agentcapabilities.ManifestRequiredScopeMarkdownList`. MCP README tool and
capability-specific error-code inventories are generated through
`scripts/generate-pulse-intelligence-docs.go` from the manifest-owned Pulse MCP
surface contract via `agentcapabilities.MCPToolCapabilityInventoryMarkdown` and
`agentcapabilities.MCPErrorCodeInventoryMarkdown`. Workflow-prompt inventory is
generated through `agentcapabilities.MCPPromptInventoryMarkdown` from the same
`MCPManifestSurfacePromptProjectionSupported` gate and
`ManifestPulseWorkflowPrompts` / `ProjectMCPWorkflowPrompts` projection used by
`prompts/list`, including manifest labels as prompt titles. README tool lists,
prompt lists, scope lists, and capability-specific error-code lists must not be
hand-maintained snapshots. MCP
`tools/call` must use the same shared
`agentcapabilities.ManifestSurfaceToolCapabilities` resolver as `tools/list`;
streaming capabilities such as `subscribe_events`, and raw manifest
capabilities omitted from the Pulse MCP surface contract, remain unavailable to
request/response tool calls even if a client sends their name manually. Adding
a raw capability to `Manifest.Capabilities` is not enough to publish an MCP
tool; the Pulse MCP `surfaceToolContracts` allowlist must opt it into that
external-agent surface. Publishing the Patrol work prompt also requires the
surface contract to include `get_patrol_control_status`, so MCP clients can
read the same content-safe Patrol work stage before requesting fleet, finding,
resource, approval, action, or resolution detail. The legacy
`get_operations_loop_status` name may remain accepted only at compatibility
edges.
MCP `resources/list` and `resources/read` are also shared manifest-backed
projections: `resources/list` must call the canonical `get_fleet_context`
capability, `resources/read` must call `get_resource_context`, and the
`pulse://resource/<resource-id>` URI shape, resource content MIME type, and
resource read parameter validation live in `internal/agentcapabilities` rather
than in `cmd/pulse-mcp` or an MCP-only inventory registry. The
`get_fleet_context` capability accepts optional additive filter arguments
(hasFindings, severity, technology, resourceType) that an agent may pass
through `tools/call` to narrow the fleet to a relevant subset; the projection
layer forwards non-path GET arguments as URL query parameters so the manifest
contract, not a per-capability adapter, owns query-string transport. The
`resources/list` adapter calls fleet-context with no filters and continues to
receive the full fleet.
than in `cmd/pulse-mcp` or an MCP-only inventory registry. They must enter
through `ManifestSurfaceResourceCapabilities`,
`ListMCPManifestSurfaceResourcesHTTP`, and
`ReadMCPManifestSurfaceResourceHTTP`, not raw-capability
`ListMCPResourcesHTTP` or `ReadMCPResourceHTTP` helpers. The MCP resource
methods remain disabled when the target surface affordance contract disables
resources, even if those context capabilities exist in the manifest.
Operator-state writes, governed finding lifecycle tools, and action
plan/decision/execute tools must also carry manifest-owned `inputSchema`
definitions for their exact arguments so external-agent clients receive typed
fields and enums from the canonical manifest instead of an MCP-local free-form
body convention. When the manifest has no authored schema, MCP may derive a
permissive fallback through the shared `agentcapabilities.ToolInputSchema`
helper, but it must not hand-roll a second JSON Schema envelope.
Governed finding lifecycle tools must return the agent-stable error envelope
from `internal/agentcapabilities/errors.go` for branchable failures
(`invalid_finding_request`, `finding_not_found`,
`finding_action_not_allowed`, `patrol_unavailable`) and declare those codes in
the canonical manifest; UI callers may still use the same HTTP routes, but the
wire contract is owned by the Pulse Intelligence agent surface once the route
is advertised there. The exported `agentcapabilities.AgentErrCode*` constants
are the source of those values for both manifest declarations and
`internal/api/` handler emissions; local string literals in either layer are
contract drift.
The governed action tools also declare `mock_mode_enabled` for read-only mock
runtimes and `action_plan_identity_mismatch` when a presented reviewed hash no
longer matches the authoritative plan. Pulse MCP documentation must remain a
generated projection of those manifest codes so Assistant and external-agent
adapters branch on the same refusal vocabulary as direct HTTP clients.
`GET /api/actions` and `GET /api/actions/{id}` use that same shared vocabulary
for invalid view/limit and internal list/detail failures even though those
relay-mobile continuity reads are not separate Assistant tools; their codes
must not drift into handler-local literals or imply Task 10 terminal truth.
Manifest `scope` values are auth-owned vocabulary from
`pkg/auth` and must be declared through manifest-local aliases to those
constants, not repeated as string literals, so MCP tools and direct HTTP agents
advertise the same scopes the API authorization layer enforces. Patrol finding
capabilities (`list_findings`, `acknowledge_finding`, `snooze_finding`,
`dismiss_finding`, and `resolve_finding`) are part of the AI runtime route
surface and therefore advertise `ai:execute`; they must not be downgraded to
monitoring scopes in MCP docs or manifest projections while the router gates
those HTTP routes through AI execution authorization.
Do not add MCP-only business logic or a second tool registry here. If a new
operation should be available to external agents, add it to the canonical API
capability manifest and make Assistant consume the same governed backend
contract when the in-app UX needs it.
Assistant prompt governance follows the same rule: live turns derive tool
mode, typed approval policy (`scope_only` or `action_plan`), approval summary,
and tool summary text from the shared `agentcapabilities.ToolGovernanceDescriptor`
shape returned by `PulseToolExecutor.ListToolGovernance`; descriptor defaults
must be constructed through `agentcapabilities.NewToolGovernanceDescriptor`,
and fallback prompt
text must call the registry-owned
`tools.CanonicalToolGovernanceForSurface` helper with the explicit requesting
Pulse Intelligence surface rather than maintaining a chat-local copy of tool names, governance
descriptor fields, policy vocabulary, or governance prose.
Assistant-only structured interaction tools such as `pulse_question` must also
be projected through the shared
`agentcapabilities.BuildAssistantToolGovernancePromptSection` and
`agentcapabilities.AssistantGovernanceOfferedToolNames` helpers rather than by
hand-building prompt-policy lines or offered-tool filters in chat-local code;
those tools remain native Assistant interaction boundaries, not MCP manifest
capabilities.
Direct Assistant registry execution is also part of the shared Pulse
Intelligence tool-call contract: `ToolRegistry.Execute` must normalize and
validate tool names and arguments through `agentcapabilities.ToolCallParams`
and use the shared invalid-params, unknown-tool, and control-disabled tool
result helpers before lookup or handler dispatch, so in-app direct execution
cannot drift from external MCP `tools/call` on trimmed names, blank-name
rejection, nil-argument initialization, deep caller-argument copying, or
standard entrypoint failure envelopes.
Chat-service direct execution paths such as autonomous command execution and
approved-fix replay must route through one Assistant registry execution helper
before calling `agentcapabilities.InterpretDirectToolExecution`, so
surface-specific failure wording does not fork executor lookup, registry
dispatch, marker interpretation, or shared result handling. MCP-named
direct-execution symbols are not part of the shared adapter contract; native
Assistant code and MCP adapters must use the neutral direct tool execution
helper.
Cross-repo auto-fix and Patrol-approved action binders must resolve approved
tool execution through `AIAutoFixHandlerDeps.ResolveApprovedAssistantToolExecutor`;
that resolver returns the native `ApprovedAssistantToolExecutor` only, so
enterprise code cannot recreate MCP-named Assistant execution selection. The
current public runtime must populate `AssistantToolExecutor` only; MCP remains
an external adapter boundary and is not an approved-fix execution dependency.
Assistant telemetry that records recoverable policy-block attempts must also
use the shared `agentcapabilities` tool-response error-code constants rather
than local string literals, so metrics labels, structured tool responses,
Assistant recovery tracking, and MCP-facing agent error vocabulary stay
aligned.
Assistant provider tool schemas are also registry-owned: chat must obtain the
runtime provider tool surface from `PulseToolExecutor.AssistantProviderTools`
so availability filtering, registry-owned governance descriptors, and
Assistant-native interaction tools are composed at one executor-owned boundary
instead of being paired manually in chat paths. That executor projection must
project registered `tools.Tool.InputSchema` values through the shared
`agentcapabilities.ProjectProviderTools` provider-schema helper and the
manifest-affordance-gated
`agentcapabilities.ProjectPulseAssistantProviderTools` provider-surface helper,
and model-selected tool calls must flow through
`agentcapabilities.ProjectProviderToolCallToToolCall` and the shared
`agentcapabilities.NormalizeProviderToolCallForExecution` /
`NormalizeProviderToolCallsForExecution` execution normalizers, not local schema
maps, compatibility aliases, raw provider call fields, chat-local provider
projection wrappers, or MCP-shaped provider-projection wrappers.
MCP-shaped provider projection names remain protocol compatibility aliases in
`internal/agentcapabilities/mcp.go`; they must not be implemented in the neutral
provider schema boundary or imported by native Assistant runtime paths.
Shared projected-tool behavior hints are the same class of boundary: the
neutral projection owns `ToolBehaviorHints`, while any `MCPToolAnnotations`
name is an alias in the MCP wire edge only.
Assistant-only provider tools are still shared Pulse Intelligence declarations:
the native `pulse_question` clarification tool must be included in provider
tool lists through `agentcapabilities.ProjectPulseAssistantProviderTools` and
`agentcapabilities.AssistantNativeProviderTools`, its declaration must come
from `agentcapabilities.NewPulseQuestionProviderTool`, its JSON Schema must
remain owned by `agentcapabilities.PulseQuestionProviderInputSchema`, its
question type/defaulting vocabulary must use
`agentcapabilities.PulseQuestionToolTypeValues` and
`agentcapabilities.NormalizePulseQuestionToolType`, its emitted provider input
must parse through `agentcapabilities.ParsePulseQuestionToolInput`, and
allowlists that recognize leaked provider tool calls must consume
`agentcapabilities.NewAssistantProviderToolNameCatalog` instead of hard-coding
Assistant-native names or rebuilding provider-tool name sets locally. Provider
tool-call artifact detection and streaming partial-tool-name holding must use
`agentcapabilities.ProviderToolCallArtifactIndex` and
`agentcapabilities.SplitTrailingProviderToolNamePrefix` with that shared catalog,
so Assistant and future adapter boundaries do not fork DSML/XML/pipe/JSON/function
leak semantics. Chat may own waiting for user answers, UI event
emission, and user-answer delivery, but it must not carry a chat-local
provider schema, description, tool-list append rule, prompt-filter rule, parser,
tool identity, or provider artifact detector for that interaction.
That schema boundary is also a copy boundary: registry tool definitions must
normalize through `agentcapabilities.Tool.NormalizeCollections`, and
`ToolRegistry.Register` / `ToolRegistry.ListTools` must not expose caller-owned
or registry-owned `InputSchema` maps, required slices, enum slices, provider
input-schema maps, or JSON-schema default containers for later mutation by an
Assistant, provider, or external-agent surface. Direct provider schema
projection helpers must normalize through that same copy boundary before
emitting provider JSON so JSON-schema default containers cannot alias registry
definitions.
Provider tool-call normalization must also stay shared: provider responses,
Assistant transcript storage, live Assistant execution, and API adapter
projections must get independent input maps, nested JSON-like argument values,
trimmed executable tool names, initialized empty argument maps, and raw provider
thought-signature payload bytes from
`agentcapabilities.ProviderToolCall.NormalizeCollections` plus the shared
execution normalizers so a later mutation in one surface cannot rewrite another
surface's tool arguments or provider reasoning-continuity metadata.
Assistant transcript normalization is part of that copy boundary:
`chat.Message.NormalizeCollections` and `chat.ToolCall.NormalizeCollections`
must detach tool-call slices, nested tool-call input maps/slices, provider
thought signatures, success pointers, and tool-result pointers before stored
messages are reused across session storage, provider-continuation, browser/API,
or orchestrator adapter surfaces.
Provider-facing tool results follow the same shared boundary: Assistant
tool execution must project shared tool-call results through
`agentcapabilities.NewProviderToolResultFromToolResult` and build transcript or
model-context result messages through `agentcapabilities.NewProviderToolResult`
so result text flattening and `is_error` semantics stay aligned with
external-agent adapters. MCP-named provider-result projection helpers are not
part of the adapter contract; runtime paths that need the plain text from a
shared tool result must call `agentcapabilities.ToolResultText` at the execution
site instead of keeping Assistant-, Patrol-, or MCP-local result-text wrappers.
Runtime paths that convert chat transcripts, repair orphan tool calls, or run
legacy service tool loops must not assemble `providers.ToolResult` or
`chat.ToolResult` structs locally, and must not hide shared provider-result
construction behind Assistant-local pointer wrappers.
Native Assistant registry execution and external MCP method dispatch must also
normalize shared `ToolResult` values through
`agentcapabilities.ToolResult.NormalizeCollections` before returning them, so
handler-owned content slices and nil content collections cannot leak across
Assistant, provider, or MCP response boundaries. MCP may retain only actual
`MCPToolResult` / `MCPContent` protocol wire aliases; result constructors,
structured tool-response projection, JSON object and array-wrapper
structuredContent projection, text extraction, and interpretation stay neutral
shared helpers rather than MCP-named wrappers.
Assistant runtime settings and tool exposure must derive
read-only/controlled/autonomous semantics from
`agentcapabilities.ControlLevel` and
`agentcapabilities.ControlLevelAllowsControlTools`; future MCP adapters may
alias the same values, but must not carry separate control-tool availability
rules, disabled-control guidance, or tool-governance defaulting.

Guest-family AI runtime code paths (VMs and LXC system containers) share
generic helpers instead of per-family copies. Patrol guest intelligence
gathers both families through `gatherGuestIntelligenceFromViews` in
`internal/ai/patrol_intelligence.go`; the Assistant query tool resolves
single-guest gets through `canonicalGuestGetResult` / `guestViewGetResult`
and search matches through `addCanonicalGuestSearchMatches` /
`addGuestViewSearchMatches` in `internal/ai/tools/tools_query.go`; per-guest
disk summaries flow through `appendGuestDiskSummaries` in
`internal/ai/tools/tools_storage.go`. A new guest family extends those
helpers (their view-constraint interfaces name the read-state methods they
need) rather than re-rolling a parallel loop. The Assistant tool layer holds
no mutating tool pipeline to extend: `pulse_control` only plans typed resource
actions into Pulse Actions through `executeControlResource`
(`internal/ai/tools/tools_control.go`), and the shell, guest, Docker,
Kubernetes and file-mutation handlers that once carried their own approval and
audit steps were removed because nothing in production called them.
OpenAI-compatible message
conversion for both non-streaming and streaming turns is shared through
`convertMessagesToOpenAI` (`internal/ai/providers/openai.go`), Anthropic message
conversion for both non-streaming and streaming turns is shared by the API-key
and OAuth clients through `convertMessagesToAnthropic`
(`internal/ai/providers/anthropic.go`), Gemini message conversion for both
non-streaming and streaming turns is shared through `convertMessagesToGemini`
(`internal/ai/providers/gemini.go`), and AI memory history files load through
the generic `loadMemoryHistory`
(`internal/ai/memory/paths.go`). `Finding`/`findingJSON` and
`UnifiedFinding`/`unifiedFindingJSON` are deliberate marshal-mirror twins —
do not merge them; the mirror invariants are enforced by
`TestFindingJSONMirrorStaysInSync` and
`TestUnifiedFindingJSONMirrorStaysInSync`. The same mirror posture applies on
the transport side: `orchestratorChatAdapter.GetMessages`
(`internal/api/ai_handlers.go`) and `chatServiceAdapter.GetMessages`
(`internal/api/chat_service_adapter.go`) convert the same chat-service
messages onto deliberately separate output contracts; do not merge them, and
keep their field mapping aligned. Orchestrator-facing tool-call and
tool-result payloads must project through
`aicontracts.OrchestratorToolCallInfoFromProvider`,
`OrchestratorToolCallInfo.ProviderToolCall`,
`aicontracts.OrchestratorToolResultInfoFromProvider`, and
`OrchestratorToolResultInfo.ProviderToolResult` so the public investigation
handoff contract inherits the same `agentcapabilities` input-map,
thought-signature, and provider-result normalization as Assistant and MCP —
`TestOrchestratorAndChatAdaptersMapTheirMessageContracts` enforces it.

1. Add or change chat runtime, Patrol orchestration, findings generation, or remediation behavior through `internal/ai/`
   Patrol deterministic disk-health evidence changes in
   `internal/ai/patrol_signals.go` and `internal/ai/patrol_ai.go` must read
   SMART health, normalized SMART counters, and canonical physical-disk identity
   from storage and metrics tool payloads without treating missing or unknown
   SMART health as `PASSED`. Non-zero pending-sector, offline-uncorrectable,
   media-error, and related SMART counters must remain model-facing reliability
   evidence even when the headline health field is `PASSED`, so Patrol does not
   summarize a disk as healthy while deterministic counters indicate disk risk.
2. Add or change canonical AI provider config, provider registry metadata, provider-scoped model selection, or runtime auth/base-URL defaults through `internal/config/ai.go` and `internal/config/ai_providers.go`.
   Direct providers that speak the shared chat-compatible API must use the
   registry-backed `AIProviderProtocolOpenAICompatible` path and
   `NewOpenAICompatibleClient` rather than adding new provider-specific factory
   branches. Per-provider auth fields, configured booleans, clear-key fields,
   default base URLs, display names, docs links, env-var hints, and fallback
   model rows must be declared once in `internal/config/ai_providers.go` and
   projected to `/api/settings/ai`; frontend settings may arrange those
   provider cards, but must not become the source of truth for provider
   capability or credential metadata. Provider model-cache keys must include
   provider identity, auth/base-URL inputs, and credential identity changes
   without exposing raw secret values.
   Ollama request keep-alive is a provider runtime option owned by this path:
   `internal/config/ai.go` stores the value, `/api/settings/ai` exposes it,
   and `internal/ai/providers/ollama.go` is the only layer that turns it into
   the Ollama `keep_alive` request field. An empty configured value means
   Pulse omits `keep_alive` so the Ollama server default applies.
   Per-surface model fallback chains are owned by the `Get*Model()` getters in
   `internal/config/ai.go` and follow the operator's intent split between
   interactive and background work. Interactive chat resolves
   `ChatModel -> Model`. Background work — Patrol and auto-fix — resolves
   through the Patrol tier: `PatrolModel -> Model` for Patrol itself and
   `AutoFixModel -> PatrolModel -> Model` for auto-fix, so background work
   never falls back straight to the shared default model past a configured
   Patrol override.
   Cloud context privacy is a FIXED posture, not a user setting. Pulse is a
   self-hosted homelab/SMB tool: when the operator points the Assistant at a cloud
   model they have accepted that their (non-secret) infrastructure detail reaches
   that provider, so the cloud model receives real resource context (names, IPs,
   config) and the Assistant is actually useful. There is intentionally NO
   `cloud_context_privacy` dial and NO `share_operational_context_with_cloud`
   toggle — the privacy control users understand is the choice of model (a cloud
   provider vs. a local Ollama model). Granular per-resource governance is an
   enterprise concern, not the homelab default.
   Two invariants always hold at the model boundary and are NOT configurable:
   1. Prompt-secret sanitation strips credentials (API keys, passwords, tokens) —
      credentials never reach a cloud model.
   2. The local-only floor: resources the policy engine routes
      `ResourceRoutingScopeLocalOnly` (Restricted sensitivity — tagged secret/pii,
      PMG, k8s-secrets, ...) have their identifiers redacted and never leave the
      local trust boundary. Everything else (Internal and Sensitive) flows.
      Both are enforced by the model-boundary sanitizer
      `modelboundary.RequestSanitizerForModel(model, provider)` (`internal/ai/modelboundary`),
      which returns nil for local (Ollama) models — local always receives full context.
      UNIVERSAL backstop rule: EVERY code path that sends infrastructure-derived
      content to an external model MUST install `RequestSanitizerForModel` before the
      provider request — not only the interactive agentic loop. This explicitly
      includes session compaction (`internal/ai/chat/session_compaction.go`
      `SummarizeSession`), which sends the PERSISTED transcript (original user prompts
      and tool outputs carry raw identifiers), and the shared service helper
      `(*Service).requestSanitizerForModel` (`internal/ai/service.go`) used by
      discovery analysis, the report and fleet narrators, quick analysis, and the
      ExecuteAgentic paths. A new model-bound request path that skips the sanitizer is
      a leak and is not permitted. (Static capability probes with no resource content,
      e.g. the Patrol preflight self-test, are exempt only because their payload is
      fixed and carries no identifiers.)
      Redaction-placeholder hygiene: Pulse-authored model-bound directives (the
      resource-context handoff instructions in `internal/ai/chat/service.go` and
      `internal/ai/chat/plain_text_resource_context.go`) must NOT inject the literal
      redaction placeholder (`unifiedresources.ResourcePolicyRedactedLabel`,
      "redacted by policy") into the prompt — naming it makes the model echo it back
      as if it were a resource name. Directives reference withheld labels neutrally
      ("a withheld or placeholder label") and must instruct the model not to repeat a
      withheld placeholder back to the user as the resource identity; the
      `current_resource` handle remains the authoritative target.
3. Add or change Assistant chat request flow through `internal/api/ai_handler.go` and `internal/ai/chat/`
   The browser Assistant chat window was retired on 2026-10-22. This request
   flow still runs Patrol investigations and serves Pulse Mobile chat, so the
   session, stream, and provider rules below stay in force. The drawer,
   composer, slash-command, transcript-rendering, and browser dev-fixture
   rules that used to live in this item left with the window.
   Assistant session compaction is a runtime-backed session workflow, not a
   local waiting message, transcript-only UI action, or stubbed summarize
   endpoint. The referenced OpenCode source at fetched `origin/dev` commit
   `e82542b8023a8374f29c23b70ec019c8f256354e` registers `/compact` with
   `/summarize` as an alias in
   `packages/tui/src/routes/session/index.tsx` lines 548-570 and calls the
   runtime session summary action with the current provider/model route. Pulse
   adapts that behavior through `POST /api/ai/sessions/{id}/summarize`,
   `chat.Service.SummarizeSession`, and
   `SessionStore.CompactWithSummary`: the backend must reject active-running
   sessions, build a bounded and redacted transcript for the active chat model,
   record provider token usage as `assistant_session_compaction`, persist one
   visible assistant summary row plus the latest bounded turns, clear redo
   state, and never stream or persist raw provider tool-call artifacts, hidden
   reasoning, secrets, or raw provider responses as the compacted transcript.
   OpenRouter-routed Assistant chat requests must set a bounded default
   completion budget when the runtime request does not specify `MaxTokens`.
   OpenRouter preflights affordability against the requested maximum completion
   budget, so leaving the field unset can make tiny chat turns reserve a
   model-scale output default and fail against per-key total limits despite the
   account having credit. That bounded default must still be high enough for
   normal detailed Assistant answers such as inventory breakdowns; provider
   request shaping must not make an ordinary answer finish mid-sentence. The
   provider boundary owns this request-shape fix;
   UI code must not work around it with route-specific retry copy. If the
   provider still rejects the request because the requested completion budget
   exceeds the key limit, the Assistant error message must name that key-limit
   condition without exposing raw provider JSON, URLs, or key identifiers.
   Assistant stream provider failures emitted through chat must be classified
   in `internal/ai/chat` before callback events reach the frontend. Endpoint
   configuration/URL errors, provider reachability/transport failures, auth,
   quota/billing, rate-limit, timeout, and cancellation states must render as
   operator-facing Settings or retry guidance. Raw Go transport strings,
   provider JSON bodies, request URLs, request methods, dashboard links, and
   key-management links must not be streamed or persisted as chat-visible
   assistant output.
   Provider readiness checks through `/api/ai/test/{provider}` must use
   neutral provider diagnostic copy owned by `internal/ai/`, not Patrol
   runtime-finding wording. Chat turns retry on the selected provider/model
   route: `/api/ai/chat` must not switch provider or model routes inside a
   single Assistant turn after the selected route fails, and that prohibition
   includes same-model gateway equivalents and configured provider defaults.
   Those routes may be offered as explicit recovery actions, but the selected
   turn owns its selected route until it completes or fails.
   Assistant session model continuity is transcript-owned, not ambient client
   state. `internal/ai/chat` must persist the effective selected model route on
   the user turn as well as final assistant turns, so a client that reloads a
   session can restore the latest explicit `provider:model` route from the
   loaded messages. Assistant completion events must carry the effective model
   route that actually completed the selected turn, and provider retry copy
   must say Pulse is retrying the selected route rather than imply automatic
   provider or model fallback.
   Follow-up steering runs through `POST /api/ai/sessions/{id}/steer`. A
   steering-accepted follow-up joins the loop at its next turn boundary (the
   abort-check site in `agentic.go executeWithTools`) as a plain user message,
   never mid-provider-stream and never mid-tool-batch; the loop announces the
   injection with a `steer_applied` stream event carrying the client row id,
   splices the message into `resultMessages` at its true position, and
   persists it through the end-of-run save (whose skip-user-messages rule
   exempts `Steered` messages). Steering carries prompt text only: it cannot
   change the model route, control level, or autonomous mode of the running
   turn, does not extend the turn budget or reset wrap-up brakes, and is
   rejected for system sessions. Delivery is not guaranteed: a run that
   finishes before a boundary discards unconsumed steers without persisting
   them.
   Primary interactive chat model resolution must use the explicit configured
   chat route or, when no explicit chat route exists, a stable provider default
   without calling provider model catalogs before the selected stream starts. If
   an explicit chat route is unconfigured, retired, or recognized as a
   specialized non-chat endpoint, `/api/ai/chat` must fail that selected route
   visibly instead of substituting a same-provider default. Catalog-backed
   recommendation belongs to settings/model-list and explicit route-recovery
   flows, not `/api/ai/chat` first-response startup. Once the selected provider
   route starts, transient pre-output transport failures may retry the same route and
   must surface a `provider_retry` workflow state with `attempt`,
   `max_attempts`, and `retry_after_ms` before sleeping. If same-route retry is
   exhausted before visible output, Pulse must emit a normal provider error and
   leave recovery to the failed-turn actions; it must not emit
   `provider_fallback`, `failed_provider`, `failed_model`, `next_provider`, or
   `next_model` workflow metadata. Once visible output has streamed, Pulse must
   also keep the failure on that visible attempt rather than silently changing
   route mid-turn.
   Interactive Assistant streams must establish the session ID and emit the
   `session` event once as soon as the HTTP SSE writer is ready, before finding
   handoff recovery, model resolution, selected-provider startup,
   handoff/context prefetch, recent-session injection, inventory summary reads,
   tool scoping, or provider startup. The chat service then persists/ensures
   that same session ID while suppressing duplicate session events. This keeps the
   client's visible turn anchored while backend preparation continues and
   prevents simple prompts from appearing stuck in an opaque thinking state.
   Stored finding or handoff recovery is eligible only for a session ID supplied
   by the client as an existing conversation. A session ID generated at the
   HTTP stream boundary for a new turn must not trigger persisted handoff
   lookups or legacy session discovery before the service creates the session,
   because that turns a new prompt into a large-history storage scan before
   the runtime can even report progress.
   After the stream is anchored, pre-model work must keep emitting neutral
   `workflow_state` progress for the long phases that can otherwise look dead:
   context preparation, bounded canonical inventory reads for count/overview
   turns, and selected-provider startup. These events are runtime progress, not
   Assistant-authored analysis, and they must not become keyword routers,
   explore pre-passes, or instructions that choose the model's next action.
   Selected-provider startup copy must describe the provider route as actively
   starting the response (for example, `<Provider> is starting the response.`)
   rather than as a passive wait label, while preserving the same
   `provider_start` phase and concrete model-route payload.
   The HTTP chat stream handler also owns visible idle progress for silent
   intervals after the stream has opened: if no client-visible Assistant event
   reaches the client for the governed idle interval while execution is still
   request-bound, the handler must emit a neutral `workflow_state` with phase
   `stream_idle` rather than relying only on hidden SSE comment heartbeats.
   Legacy Assistant SSE routes that still use the older execute event envelope,
   including `/api/ai/execute/stream` and `/api/ai/investigate-alert`, share
   the same transport-liveness obligation while preserving their existing
   response payload shapes.
   Comment heartbeats and visible events must share a serialized writer so a
   progress tick cannot interleave bytes with a provider/tool event. This is a
   transport liveness signal only; it must replace the active workflow status,
   stop before terminal `done`/`error`, and must not become persisted assistant
   prose or raw provider reasoning.
   Provider reasoning must not become fallback assistant answer text when an
   OpenAI-compatible non-stream response contains `reasoning` or
   `reasoning_content` without visible `content`. The referenced OpenCode
   source at fetched `origin/dev` commit
   `4519a1da329c1a4fc384054e7203ba7d06928205` maps AI SDK
   `reasoning-start` / `reasoning-delta` / `reasoning-end` events to
   dedicated reasoning events in
   `packages/opencode/src/session/llm/ai-sdk.ts` (lines 158-188), preserves
   assistant reasoning as a `type: "reasoning"` message part in
   `packages/opencode/src/session/message-v2.ts` (lines 374-387), and keeps
   text and reasoning renderability distinct in
   `packages/ui/src/components/message-part.tsx` (lines 607-615). Pulse adapts
   that boundary by keeping `ChatResponse.Content` limited to provider
   `message.content` and storing OpenAI-compatible reasoning only in
   `ReasoningContent`, so hidden provider reasoning cannot be rendered as the
   final answer by any Pulse surface that consumes non-stream chat responses.
   Mock-mode Assistant streaming is a canonical backend fixture, not a
   client-only demo. When `mockmode.IsEnabled()` is true,
   `chat.Service.createProvider` must return the internal mock Assistant
   provider before invoking injected provider factories or configured external
   providers, and `chat.Service.ExecuteStream` must short-circuit the user turn
   only after the durable session is ensured and the user message is persisted.
   The fixture must emit the same client-facing event family used by real
   turns: anchored `session`/prepare events from the normal entry path followed
   by mock `workflow_state`, `tool_start`, `tool_progress`, `tool_end`,
   response `workflow_state`, content chunks, and terminal `done`. It must also
   persist a real assistant message with model route `pulse:mock-assistant`, a
   successful `pulse_query` tool call, and a successful `pulse_read` tool call
   whose progress event includes provider-style raw input such as
   `pulse_read(...)`, so session restore and stream-contract tests exercise the
   same shape as a real provider/tool turn. Mock mode is also an effective
   runtime-config overlay: `internal/api/ai_handler.go` startup, restart,
   config sync, and tenant service initialization must treat Assistant as
   enabled in memory when mock mode is active, without saving or mutating the
   persisted `AIConfig`, so `/api/ai/chat` and Assistant sessions are
   available with zero configured external providers or models. The fixture
   must not emit `provider_fallback` and must not become Pulse-authored
   remediation or routing behavior.
   A pre-tool assistant preamble such as "I'll check that" does not satisfy the
   final-answer contract after Pulse executes tools. The final-response guard
   must require non-empty assistant text after the latest user/tool-result
   anchor; otherwise it must make the bounded no-tools summary call or emit the
   deterministic fallback summary. This adapts the referenced OpenCode source
   at fetched `origin/dev` commit
   `06d7840d1d42c9815d2d2e45e7fa4090ca4e3577`: `packages/opencode/src/session/llm/ai-sdk.ts`
   maps tool input/call/result stream events into typed tool events (lines
   190-246), `packages/opencode/src/session/message-v2.ts` serializes completed
   tool results and reasoning as ordered provider message parts (lines
   340-390), and `packages/ui/src/components/message-part.tsx` only renders
   non-empty typed text/reasoning/tool parts through `renderable` and
   `AssistantParts` (lines 607-728). Pulse adapts that ordered-part invariant
   inside `internal/ai/chat/agentic_final.go` because Pulse's transcript is
   persisted as messages plus tool-result anchors rather than OpenCode's
   per-part store.
   Streamed provider startup and mid-stream progress must be bounded by the
   configured Assistant request timeout, the OpenAI-compatible SSE
   response-header guard, and the per-chunk SSE body-read guard adapted from
   OpenCode's provider `wrapSSE` source. The referenced OpenCode source at
   fetched `origin/dev` commit `ba57718b0516c7a8670d1e820b1a24146a8b8262`
   wraps `text/event-stream` responses in
   `packages/opencode/src/provider/provider.ts` so each stream read either
   yields a chunk or aborts within the configured `chunkTimeout`, distinct from
   the response-header timeout. Pulse's OpenAI-compatible Assistant transport
   follows that split: transient startup failures may retry once before
   surfacing failed-turn recovery, but a stalled route must not leave the user
   in an opaque first-token or mid-answer wait for the full provider timeout.
   Assistant turn tool exposure is backend-owned runtime behavior, not a
   provider whim or frontend polish concern, and it is model-owned at the
   manifest boundary: every interactive turn that reaches the selected model
   carries the full governed tool manifest from `toolsForExecutionMode`, and
   the model decides whether to answer directly, ask a clarification, or call
   tools inside Pulse policy. Pulse must not classify prompt wording into
   text-only or query-only tool scopes — greetings, exact-reply diagnostics,
   "no tools"/"before using tools" phrasing, inventory/count/list/breakdown
   wording, alert/finding wording, and prompt length are all model-visible
   text, not Pulse routing signals. The former `assistantToolScopeForPrompt`
   keyword router, its query-only manifest filter, its conversational/
   direct-text text-only scopes, and its query-only topology prefetch (inject
   compact inventory into the user message, then withhold tools) are removed
   and must not be reintroduced; that family withheld tools from real lookups
   ("hows esphome") and zeroed the manifest on false-positive phrases
   ("before using any tools, tell me your plan"). The only manifest-boundary
   exception is the contract-sanctioned context-only resource handoff turn
   (see Completion Obligations). `TestService_ExecuteStream_ToolManifestIsModelOwned`
   (`internal/ai/chat/service_tooling_test.go`) and
   `TestService_ExecuteStream_InventoryBreakdownIsModelOwned`
   (`internal/ai/chat/service_execute_additional_test.go`) are the regression
   proofs that previously-scoped prompt families all reach the model with the
   same governed manifest and an unmodified user message.
   The system prompt's missing-target policy is resolve-before-asking, not
   ask-first: when a command or diagnostic request names no target, the
   Assistant must first use read-only query/topology tools to identify
   plausible targets, proceed against a sole plausible match for read-only
   diagnostics while naming the target in the answer, and ask the operator
   only when several plausible targets remain or the action changes state.
   The earlier ask-first framing ("Missing target information is not a safe
   default... ask for the missing target") made the Assistant deflect every
   "run X" request back to the operator — including on single-host
   deployments where no real ambiguity exists — instead of investigating
   like a competent operator (the OpenCode-parity gap this supersedes).
   Placeholder targets remain forbidden in all modes: the model must never
   guess an unresolved target or substitute `current_resource` outside an
   attached-resource turn, and write actions still require an explicit or
   operator-confirmed target.
   `TestBuildSystemPrompt_CurrentResourceRequiresResourceHandoff`
   (`internal/ai/chat/service_tooling_test.go`) pins this boundary.
   Deterministic count-only inventory prompts remain the single Pulse-owned
   local answer shortcut, and it is an answer path, not tool selection: when
   canonical topology state already carries the complete aggregate counts,
   `internal/ai/chat` should answer locally by streaming normal typed
   assistant content and `done` before provider attempt creation.
   This adapts the referenced OpenCode `message.ts` and `processor.ts` typed
   part/delta model at commit `fa2b63f850fc0a23bec2bdff9e660450d3fe7913`:
   locally owned text is still transcript text, but it must not wait on a
   remote provider or pretend that shell/tool inspection produced the answer.
   The completion metadata for that path should identify the effective route
   as `pulse:local-inventory` so the transcript label reflects Pulse-owned
   local runtime output instead of the operator's selected remote model.
   Explicit or qualified operator intent is the escape hatch from this
   shortcut, not a Pulse-authored tool choice: prompts that ask to use tools,
   request a read-only attempt, provide a shell/read command or path such as
   `ls /dev | wc -l`, or qualify the count beyond pure inventory ("how many
   vms have errors") must reach the selected model/provider with the full
   governed Assistant tool manifest instead of answering from local
   inventory. Prompt classification
   (`assistantPromptQualifiesForLocalInventoryCount` in
   `internal/ai/chat/service.go`) may only suppress the shortcut and send the
   turn to the model — its false positives fail safe, to the model — and the
   selected model still decides whether to call `pulse_read`, answer
   directly, or ask a clarification. This adapts the referenced OpenCode
   source at fetched
   `origin/dev` commit `4519a1da329c1a4fc384054e7203ba7d06928205`, where
   `packages/core/src/session/message-updater.ts` mutates tool input, called,
   progress, success, and failed states as typed session events (lines
   247-318), by preserving the operator's explicit tool intent until Pulse can
   stream a visible `tool_start` / `tool_progress` / `tool_end` path instead of
   collapsing the turn into unrelated inventory prose.
   Stream-idle heartbeats must inherit the selected provider/model route when
   Pulse has that route. Pulse must not rewrite the inputs of a model-chosen
   tool call (the former summary-only `pulse_query` topology input default is
   removed with the query-only scope); `pulse_query` defaults are owned by the
   tool definition the model sees, not by post-hoc input mutation. Assistant
   answer style should stay operational and must not use emoji, warning icons,
   or decorative symbols unless the user explicitly asks for that tone.
   The system prompt for a turn must describe only the tools actually offered to the
   provider for that turn; generic tool-governance prose must not name tools
   hidden from the current manifest. Deterministic provider-request capture is
   the primary proof harness for this fast path; live OpenRouter/browser checks
   are final smoke tests, not the iteration loop.
   Saved Assistant message history is part of the same output contract:
   service reads and `GET /api/ai/sessions/{id}/messages` must return a
   client-safe projection, including persisted tool calls, that normalizes
   collection fields, strips `reasoning_content`, and sanitizes stored
   assistant prose before a client restores a transcript. Provider reasoning
   may remain in runtime-only history for model continuity, but it must not
   serialize through public session history.
   Assistant session listing is a history-browsing path, not a send-path lock.
   The session store must not hold its shared mutex while scanning and parsing
   every persisted session file for `/api/ai/sessions`; writes must remain
   atomically readable through temp-file plus rename so a large recent-session
   list cannot block `EnsureSession`, `AddMessage`, or stored handoff reads on
   a new prompt.
   Session listing cost must not scale with stored transcript bytes.
   `SessionStore.List` serves summaries from a per-file cache validated by
   each file's (modTime, size), re-reading only files that changed since the
   last list, and a cache-miss parse must not deep-decode message or redo
   payloads (only their counts feed the summary). The cache persists as
   `.sessions_index.json` beside the session files so the first list after a
   restart stays index-speed; the index is best-effort and self-healing
   (missing, corrupt, or stale entries only cost a re-parse, never a wrong
   summary, and dotfiles are never listed as sessions). `writeSession` and
   `Delete` keep the cache and index coherent on every mutation.
   Pulse-owned background sessions are not resumable chats. The session store
   owns the well-known background session IDs (`patrol-main`, `patrol-eval`,
   `investigation-*`) through `chat.IsSystemSessionID` and marks their
   summaries `system: true` in every `Session` projection, so clients can keep
   them out of resume lists. The canonical Patrol forensic surface remains the
   `PatrolRunRecord` history at `/api/ai/patrol/runs`.
   Assistant session forking is a runtime-backed session workflow, not a
   header-only UI affordance. The referenced OpenCode source in
   `packages/opencode/src/cli/cmd/tui/routes/session/index.tsx` registers
   `session.fork` as a first-class session command and opens
   `DialogForkFromTimeline` from the current session route. Pulse adapts that
   source behavior through `POST /api/ai/sessions/{id}/fork`: the local
   `chat.Service` must route to `SessionStore.Fork`, and the store must create
   a fresh durable session ID with fresh create/update timestamps while cloning
   the saved transcript and client-safe model handoff summary. Copied
   messages keep their per-session message/tool IDs so assistant tool-call and
   tool-result relationships remain intact inside the fork. Forking must not
   mutate the source session, must continue through normal session list/load
   contracts, and must not return `not_implemented` from the local runtime.
   OpenCode-style session file diff/revert workflows do not apply to Pulse
   Assistant sessions: Pulse does not own local code-file edits, and
   infrastructure mutations are governed through approval/action history. Legacy direct calls to
   `/api/ai/sessions/{id}/diff`, `/revert`, and `/unrevert` must fail
   explicitly with `501 Not Implemented` rather than returning a success-shaped
   placeholder.
   Assistant session rename is part of the same session workflow. Pulse owns it
   through `PATCH /api/ai/sessions/{id}`, `chat.Service.RenameSession`, and
   `SessionStore.Rename`; saving must persist the normalized title, return the
   updated `ChatSession`, and leave messages, handoff context, approvals, and
   tool evidence unchanged.
   Session titles follow OpenCode's generated-title model: after a session's
   first exchange completes, the chat runtime upgrades the auto-truncated
   first-prompt placeholder to a short model-generated title
   (`chat.Service.generateSessionTitle`, `internal/ai/chat/session_title.go`).
   The upgrade runs after the done event reaches the client (no turn
   latency), only on the first exchange, and never overwrites a title the
   user set: any title that differs from the placeholder is left alone, with
   a re-check before persisting to lose races against a concurrent rename.
   Title requests pass the same model-boundary sanitizer as normal turns,
   record usage as `assistant_session_title`, and a failed or timed-out
   title call leaves the placeholder in place rather than failing anything
   user-visible.
   Assistant turn undo/redo is a session repair workflow, not a
   transcript-local delete. Pulse owns it through
   `POST /api/ai/sessions/{id}/undo` and `POST /api/ai/sessions/{id}/redo`:
   the backend session store must remove the latest user-authored turn plus
   following assistant/tool messages as one durable turn, persist a bounded
   redo stack, expose `can_redo` on the client-safe session summary, clear redo
   state when a new message or trim changes the transcript, and keep forked
   sessions independent from the source redo stack.
   Retry and regenerate are turn re-runs over the same undo boundary, not a
   parallel history mutation path. `POST /api/ai/sessions/{id}/undo` accepts an
   optional JSON body (`chat.SessionTurnUndoOptions`) whose `expected_prompt`
   field, when non-empty, makes the removal conditional on the latest
   user-authored prompt matching the prompt being re-run, so a retry whose
   failed send never reached the session cannot remove a different turn.
   Session mutations stay POST-only at the handler: abort, summarize, fork,
   undo, redo, and steer refuse every other method with `405` and
   `Allow: POST` before resolving the Assistant service. A `GET` or `HEAD`
   that the demo-mode read-only guard or the CSRF check admits as a read
   therefore cannot cancel a running turn, compact or rewind a transcript,
   or persist a fork.
   Assistant output hygiene is part of the same boundary: provider reasoning
   and raw serialized tool-call artifacts must never serialize into
   client-facing chat streams or persisted assistant prose. Client-bound
   Assistant stream events must pass through `chat.StreamEvent.ClientSafe()`;
   provider `thinking` chunks are runtime-only and may be retained internally
   for model continuity, but they are dropped before the client/API boundary.
   Client-safe assistant prose has operational decorative status glyphs,
   warning icons, and check/cross badges stripped while ordinary Unicode
   answer text such as units is preserved. The agentic stream may translate
   the first private provider reasoning delta before visible output into a
   neutral `model_thinking` workflow status so clients can show live activity
   without exposing chain-of-thought. Pulse tool invocations must surface only
   through governed `tool_start` / `tool_end`, approval, or question blocks; if
   a provider emits `pulse_*` / `patrol_*` calls, DSML, XML/function-call
   envelopes, or JSON tool-call shapes as text content, the chat runtime must
   strip them before streaming and persistence. Compacted no-whitespace
   internal prelude text attached to a leaked tool invocation is part of that
   same artifact and must be suppressed or retracted from the current stream
   segment instead of streamed as assistant prose; if that compacted prelude
   is still pending when the stream closes, terminal flush must drop it rather
   than turning unreadable provider residue into the final answer. Token
   accounting and other provider metadata remain runtime/accounting data, not
   transcript prose.
4. Add or change Patrol, alert-analysis, or remediation transport through `internal/api/ai_handlers.go`, `internal/api/ai_intelligence_handlers.go`, and `frontend-modern/src/api/patrol.ts`
   Provider preflight diagnostics returned from `internal/api/ai_handlers.go`
   must reuse the Patrol runtime failure classifier in `internal/ai/` and
   expose only safe operator-facing cause, summary, recommendation, model, and
   action fields. Raw provider response bodies and transport errors may be
   logged server-side or attached as redacted internal Patrol evidence where
   governed, but they must not be returned through the browser provider-test
   contract.
   Anthropic runtime provider execution must be API-key backed. Legacy
   Anthropic OAuth tokens may remain in encrypted settings only as cleanup
   state; they must not make Anthropic configured, instantiate an OAuth-backed
   provider, refresh tokens, or send model requests. OAuth start/exchange and
   callback handling must fail closed, while disconnect may clear stored legacy
   tokens.
   Patrol findings history transport must stay bounded when resolved findings
   are included: `/api/ai/patrol/findings?include_resolved=1` defaults to a
   200-finding limit and caps explicit limits at 500, and the frontend Patrol
   client/store must send the same bounded history request once the Resolved or
   All view has made expanded history sticky. Per-finding suppression creation
   is similarly narrow by default: the browser helper must require a concrete
   resource ID and category, while backend broad/wildcard suppression scopes
   require an explicit `allow_broad_scope` request from a dedicated rule
   management surface.
5. Add or change AI usage/cost dashboard presentation through `frontend-modern/src/components/AI/AICostDashboard.tsx` and `frontend-modern/src/utils/aiCostPresentation.ts`
6. Add or change AI provider and provider-health presentation through `frontend-modern/src/utils/aiProviderPresentation.ts` and `frontend-modern/src/utils/aiProviderHealthPresentation.ts`
   AI provider/model presentation must preserve the model transport route when
   the selected provider is a gateway. OpenRouter-routed model IDs such as
   `openrouter:deepseek/...` must render with an explicit `via OpenRouter`
   label in the shared picker, System AI settings status, and inherited default
   descriptions unless the server-supplied model name already carries that
   route. Direct provider models must not gain a gateway label, but explicit
   direct route strings such as `deepseek:deepseek-v4-pro` must still render
   with provider identity rather than collapsing to a bare-looking model
   payload. Gateway route strings must still render as readable provider/model
   labels before catalog hydration, presenting catalog names with ID fallbacks
   rather than leaking raw route storage IDs into primary settings chrome.
   The shared model picker separates selected model labels from badges such as
   `default` in both visible text and the button accessible name, so an
   OpenRouter default route renders as
   `Qwen: Qwen3.7 Plus via OpenRouter · default` rather than
   `OpenRouterdefault`.
   Provider-readiness repair actions must send operators to the canonical
   Pulse Intelligence > Provider & Models route
   `/settings/pulse-intelligence/provider`; `/settings/system-ai` remains a
   compatibility alias for old deep links rather than the href emitted by new
   repair actions.
   Stable route markers, telemetry fields, and API wire names may retain
   `patrol_control`; user-facing external-agent, generated MCP, and frontend
   copy must call the operator's choice `Patrol mode`.
7. Keep AI presentation text helpers aligned through the shared `frontend-modern/src/utils/textPresentation.ts`
8. Keep the AI availability session capability aligned through the shared `frontend-modern/src/stores/aiAvailability.ts` boundary instead of letting `frontend-modern/src/App.tsx`, `frontend-modern/src/AppLayout.tsx`, or feature callers fork their own AI-enabled state
   The shared app shell may also mount the deployment-installability-owned
   post-update release highlights card. That card must consume published
   release notes through the update surface and must not read, reset, or
   otherwise couple itself to AI availability, Patrol state, model routing, or
   AI-provider readiness.
   The same app-shell boundary keeps Patrol and Actions utility navigation
   accessible-name safe: labelled icon SVGs may remain meaningful when rendered
   standalone, but `frontend-modern/src/AppLayout.tsx` must treat them as
   decorative inside tabs so the announced tab name comes from product chrome
   and meaningful badge text rather than icon title duplication. Global
   governed operations use the peer `/actions` utility destination. Actions
   owns its selected state and pending-approval badge while Patrol owns only
   its operational-attention count.
   Platform tabs in that same `frontend-modern/src/App.tsx` shell may remain
   admitted after a current authenticated REST bootstrap or from retained
   server-owned WebSocket state during stream recovery. That navigation
   evidence must remain independent from AI availability, and browser-local
   resource metadata must not resolve platform navigation on an otherwise
   evidence-free first load. The app shell's global GitHub-star prompt stays
   behind the kiosk/settings-access gate and outside every AI runtime surface.
   Patrol and Actions chrome must not host commercial prompts.
   Clients that reload Assistant sessions may consume the backend-owned
   `handoff_summary` only as safe presentation state and a Patrol finding
   pointer; hidden model context, command payloads, preflight data, and action
   results stay backend-owned and must not be reconstructed in a client.
9. Add or change public Pulse Intelligence overview wording through `docs/AI.md`; it may
   describe Patrol and external-agent capabilities, but it must not revive legacy
   commercial shorthand such as `incident memory` as a current product promise.
   The public overview must preserve the product architecture: Pulse
   Intelligence Core owns canonical context, governed actions, safety gates,
   approval state, action audit, and verification; Pulse Patrol is the primary
   built-in operator on that core; its public summary must stay concise as
   watching, investigating, acting within the chosen Patrol mode, verifying outcomes,
   and recording what happened; and Pulse MCP is the external-agent access
   path over the same governed capabilities.
   Public overview, safety, and onboarding copy must use the same visible
   Patrol mode labels as the product (`Watch only`, `Ask first`,
   `Safe auto-fix`, `Autopilot`) and must not reintroduce the retired
   `Only watch`, `Ask before changes`, `Fix safe issues`,
   `Auto-fix safe issues`, `Full control`, or `Policy autopilot` labels as
   customer-facing names.
   The public overview must also preserve the model-owned AI boundary: Patrol
   and the external-agent path provide governed context, tools, safety gates,
   approval state, and audit trails, while the configured LLM owns diagnosis,
   prioritization, fix reasoning, and tool choice. Public copy must not
   present Pulse code as the intelligence engine, a prompt-keyword router, a
   deterministic auto-remediation oracle, a provider replacement, a learned
   operational-meaning engine, or a deterministic finding author.
   The public overview must not carry a hand-maintained Assistant tool table.
   Assistant tool inventory, action modes, approval policies, provider
   declarations, and Patrol-only filtering are registry-owned in
   `internal/ai/tools/` and projected through `internal/agentcapabilities/`;
   external-agent inventory belongs to `/api/agent/capabilities` and
   `pulse-mcp` `tools/list`.
   External-agent setup metadata follows the same rule: Pulse MCP server name,
   command, base URL flag/default, token environment variable, and supported
   client config families are manifest-owned in
   `/api/agent/capabilities.mcpAdapter`. Pulse Intelligence settings,
   README, and MCP surfaces may present that setup, but must not
   maintain separate MCP setup constants. Security API settings may only host
   the scoped-token creation link used by that setup.
10. Platform/runtime top-level pages registered in
    `frontend-modern/src/App.tsx` and the primary tab list in
    `frontend-modern/src/AppLayout.tsx` must keep the Patrol entry chrome
    intact: they extend `aiAvailabilityStore`-aware route shells rather than
    mounting bespoke layouts that suppress the Patrol entry or its keyboard
    shortcut. New per-platform sub-routes inherit the shared chrome by virtue
    of routing through `AppLayout`, and must not introduce a chat surface,
    launcher button, or model picker on the platform page itself;
    cross-platform AI work stays routed through Patrol. The Patrol utility
    destination in `AppLayout`'s tab lists shares the nav identity-stability
    contract owned by `frontend-primitives` (`stableNavTabs.ts`): a websocket
    state frame with structurally unchanged tab content must not recreate the
    Patrol button DOM or its attention-count badge element. Patrol and the peer
    utility destinations commit navigation without awaiting best-effort module
    warming, so lazy-route work cannot make the shared shell drop or defer a
    tap.
    The primary-nav demotion of Infrastructure / Workloads / Storage /
    Recovery does not change Patrol addressability: Patrol remains reachable
    through the utility tab strip in `AppLayout.tsx` and via the canonical
    Patrol path, and platform pages must not replicate Patrol findings, AI
    prompts, or launcher affordances inside their own chrome.
    The vSphere Networks sub-route follows the same AI runtime boundary as the
    vSphere overview, datastore, health, and activity routes. Network rows
    reach Patrol only as shared `network` unified-resource references read
    through `/api/resources*`; the VMware page must not introduce VMware-local
    AI prompts, a provider model picker, or a vSphere-specific chat/runtime
    route just because networks are now rendered as a first-class API-native
    table.
    The frontend-primitives-owned Machines surface follows the same AI runtime
    boundary while retaining the internal `standalone` route/id contract: route
    registration and primary-tab chrome may expose the support-manifest `agent`
    platform plus agentless availability endpoints, but the page must not add
    an agent-specific AI prompt surface, launcher, model picker, or
    browser-owned model context. Machine rows reach Patrol only through the
    shared unified resource contracts. Its IA, primary shell position, and
    landing eligibility belong to `frontend-primitives`; those changes must not
    alter Patrol utility-tab ordering or shared keyboard handling in
    `frontend-modern/src/AppLayout.tsx`.

### Platform navigation admission

Surfaces hosted in the app shell MUST take platform navigation visibility from
the shell's resolved admission rather than re-deriving it from a full-state
payload or a resource count. Admission is per-resource ownership evidence and
has a single definition in the canonical resource contract.

## Forbidden Paths

1. Leaving new `internal/ai/` runtime entry points unowned under broad architecture or generic API ownership
2. Duplicating AI orchestration, Patrol runtime, or cost-tracking logic outside `internal/ai/`
3. Treating AI transport files as payload-only boundaries when they also define live runtime control behavior

## Completion Obligations

The `pulse_knowledge` incidents action reads the organization-pinned canonical
resource timeline used by resource history and Assistant handoffs. It preserves
resource identity, observation and optional occurrence time, source and event
metadata. Reads use explicit observation-time bounds and a bounded event count
with truncation disclosure. Empty retained history is not continuous healthy
coverage, and an unavailable or failed history store is a failed tool read.
Legacy recording IDs are isolated archive lookups bound to the requested
resource. Their recorder timestamps cannot establish source measurement time.
The primary history path must not restore the legacy recorder as a parallel
incident authority or derive fresh history by resampling cached metrics.
`TestIncidentHistoryRetainsCanonicalEvidence` uses SQLite lifecycle records,
time/resource negative controls, missing occurrence time and bounded reads.
The corresponding unavailable/invalid and archive tests cover failure semantics
and resource isolation. Live model interpretation remains governed by the
customer journey qualification plan.

Every per-organization Assistant or legacy AI service that can discover or
dispatch through the host-agent command transport must receive an
organization-pinned command-server view. A tenant service must never enumerate
the default organization or another tenant and must not rely on an unscoped
agent ID to select a session. Loss, revocation, or replacement of the pinned
session fails the invocation before command dispatch while the ordinary
approval, action-audit, timeout, and cancellation contracts remain in force.

Qualification floor: Patrol model launch and product-claim qualification must use
   `cmd/patrol-qualify` against reviewed scenario-owned ground truth. Expected
   faults and independent postconditions must be declared before the run and
   must not derive from Patrol-selected tools, deterministic signal extraction,
   or model output. A live scenario oracle must not mature before the canonical
   deterministic triage or alert threshold that supplies Patrol's normal evidence. In
   particular, threshold-based faults must converge beyond the configured
   trigger boundary before model invocation so a faster model cannot receive
   weaker evidence than a slower model.
   Watch, investigation, and remediation are separate tracks;
   remediation decisions require the runner's second authorization gate, exact
   action/finding/investigation/resource/capability/plan-hash binding, and an
   out-of-band postcondition. Scorer or tool-transcript replay is regression
   evidence only and cannot replace live normal-collection, real-model proof.
   Every new live report must serialize the independent safety inputs supplied
   to its scorer, including fault-intact and no-unexpected-mutation outcomes;
   fault predicate observations are not a substitute for the separate lab
   inventory comparison. Score replay must prefer those captured inputs. For
   legacy v1 reports that predate them, replay may preserve only the exact
   checksummed safety hard-failure outcome already recorded by the live scorer;
   it must not infer inventory safety from the model's tool transcript, finding
   output, or unrelated fault observations.
   A live Docker remediation lab must opt in separately to a command-enabled
   disposable agent with a short-lived `agent:exec` token; Watch and
   investigation labs remain report-only. The scenario must expect the
   state-dependent capability advertised by the canonical resource contract
   (`start` for an exited container), and teardown must revoke the temporary
   token as well as restore and remove the exact-run-labelled resources.
   Shared or remote labs must use exact-run-labelled disposable resources,
   two-pass cleanup, full Docker inventory comparison including images, and
   must never manufacture faults in production infrastructure.
   Read-only investigation tools that accept canonical resource IDs must
   resolve provider coordinates internally before calling provider-owned
   discovery or inspection paths. A model must not need to infer a Docker
   container ID or substitute a display name after `pulse_query` returns a
   canonical `app-container` ID; cross-tool identity translation is an AI
   runtime responsibility.
   The investigation seed must therefore label the finding's display name,
   canonical resource ID, and resource type as separate fields and state the
   tool-schema rule explicitly: `resource_id` receives the canonical ID, while
   a display name is used only by arguments documented as names or queries.
   Human-readable prompt formatting must not make those identities ambiguous.
   Investigation action-catalog and proposal tools inherit that identity
   boundary in both directions: an exact, uniquely resolved Docker provider
   coordinate may be translated back to its canonical `app-container` target,
   and the catalog response plus captured proposal must carry that canonical
   target. Unknown or ambiguous references still fail closed. The model is not
   responsible for Pulse's provider-to-canonical identity plumbing.
   The proposal canonicalizer accepts either that provider coordinate or the
   equivalent exact Pulse read coordinate
   `app-container:<host>:<provider-container-id>` when it uniquely identifies
   the same unified resource. It does not accept a partial ID, display name, or
   ambiguous host/container pairing.
   Agentic context compaction is also evidence-preserving rather than merely
   size-reducing. When a read tool returns structured safety-relevant state,
   its deterministic knowledge projection must retain the bounded fields needed
   for later diagnosis and action choice while excluding labels, environment
   values, and other unrelated raw inspection content. Docker inspection
   compaction therefore retains container state, running/OOM/dead flags, exit
   and health results, restart count/policy, and image identity; it must not
   collapse a valid inspection to punctuation or the first formatting lines.

Patrol autonomous-loop floor: Watch must preserve model-owned investigation
while requiring an accepted structured outcome for every active finding the
run presents or returns. New issues use `patrol_report_finding`; existing
   issues use `patrol_assess_finding` with exactly one `present`, `resolved`, or
   `uncertain` verdict. Missing verdicts are run errors, not absence-based
resolution or all-clear evidence. `present` refreshes durable evidence and
run ownership, `resolved` remains behind deterministic fail-closed
verification, and `uncertain` keeps the finding active. Scoped Patrol must
use its trigger as the initial clue rather than an artificial reasoning
boundary: within read-only authority the model may iteratively inspect related
canonical resources, relationships, metrics, timelines, changes, and other
governed evidence needed to test hypotheses. Governance constrains mutation
authority, data policy, and resource access; it must not add approval-shaped
friction to each safe investigative step. Scoped Patrol must still
resolve canonical/source IDs and unique aliases before collection, reject
   ambiguous or unmatched API identities synchronously, persist an error run
   for runtime zero-match races, record requested and effective identities,
   and summarize only findings within the effective scope. A scoped run keeps
   two deliberately different boundaries: its effective evidence scope may
   include related hosts, parents, and dependencies for model reasoning, while
   its finding-lifecycle scope is the exact resolved caller-requested identity
   set. An exact app-container scope retains its Docker host as the transport
   parent and retains the selected container's embedded image, mount, and
   network-attachment evidence, but it must not re-ingest the host's complete
   image, volume, network, or Swarm child inventory into the scoped provider.
   Those unrelated host children are neither dependencies of the selected
   workload nor permission to turn a targeted check into a host-wide scan.
   `patrol_get_findings`, report, assess, and resolve must not refresh or
   create a finding on a context-only dependency; the model may cite that
   dependency as evidence for the requested resource and Pro investigation may
   expand causally after Watch is durable. This prevents a targeted workload
   check from surfacing an unrelated host finding merely because the host was
   included as supporting context. Watch may mutate
   only finding lifecycle state; Pro investigation remains structurally
   read-only. A Watch model-reported finding must not dispatch Pro
   investigation from inside the finding tool call: Pulse first persists the
   completed Watch run and unwinds its run-scoped finding adapter, then
   dispatches the accepted findings into Pro investigation. Investigation may
   clone the shared executor only after that boundary; it must never inherit or
   mutate the active Watch run's finding-lifecycle adapter. Its finding resource
   is an observation anchor, never a search
   fence: the Pro prompt must explicitly require evidence-led expansion,
   competing-hypothesis testing, and a distinction between affected and causal
   resources. When evidence selects a different remediation target, the
   investigation-only `patrol_action_capabilities` lookup exposes that exact
   resource's current typed catalogue without authority or proposal side
   effects; only then may `patrol_propose_action` leave one side-effect-free
   typed proposal for the canonical approval, execution, and independent-
   verification lifecycle. For Docker container-health remediation, that
   lifecycle may claim verified recovery only from version-2 daemon evidence
   whose health state is explicitly `healthy` or `none`; running state,
   `starting`, `unhealthy`, and omitted legacy health remain non-terminal
   finding evidence.

1. Update this contract when canonical AI runtime or transport entry points move, including transport-level provider request-shape changes such as OpenAI-compatible `tool_choice` handling, runtime-failure classification splits (for example separating tool-choice request rejection, no tool-capable endpoint, and generic model-level lack of tool support into distinct causes), Patrol-specific verification surfaces such as `POST /api/ai/patrol/preflight` that exercise the full chat-completions path with a minimal tool definition rather than only listing models, Patrol-preflight cache observability where the AI Service caches the most recent preflight outcome (success, soft warning, or classified failure) and the AI settings response surfaces it as `patrol_preflight` so the UI can hydrate a "last verified" indicator without forcing operators to re-run preflight on every page load, the auto-trigger contract on `HandleUpdateAISettings` where the save handler runs `TriggerPatrolPreflightAsync` only when the change actually moved Patrol transport (model swap, provider key for that model changed, or assistant just enabled with a Patrol model) so routine settings saves do not burn provider tokens, the startup-seed contract where the AI Service handler dispatches the same async preflight on Pulse boot when assistant is enabled and a Patrol model is configured so the cache is populated for the first `/api/settings/ai` poll after a restart instead of blanking back to "never verified", the readiness-integration contract where the `tools` check in the Patrol readiness payload consults the cached preflight and surfaces the classified evidence (success, soft warning, or failure with classified summary plus "last preflight <age>") for the configured provider+model when available (falling back to the static `PatrolToolReadinessForModel` classifier only when the cache is empty or holds a result for a different model), the preflight-runtime-recovery contract where a successful Patrol preflight with an observed tool call resolves the synthetic Patrol runtime failure finding while failed or no-tool-call preflights leave it active, the stateless-Patrol-input contract where `ExecutePatrolStream` must pass only the current run's user prompt into the agentic loop rather than reloading the persisted `patrol-main` session history (so a prior run that ended with orphan `tool_calls` cannot poison every subsequent run with malformed conversation structure), and the deterministic-resolve-gate contract where the `patrol_resolve_finding` tool adapter rejects LLM-driven resolves of event/persistent category findings (`backup`, `reliability`, `security`, `general`) when a deterministic verifier exists for the finding's key and that verifier either still detects the failure signal **or returns an inconclusive result** — preventing the LLM from optimistically resolving a finding its current investigation simply didn't re-surface, which was the source of the "Backup failed" flap (detected → auto-resolved → re-detected ten times in a day before this gate). The fail-closed-on-inconclusive policy treats verifier errors (timeouts, executor unavailability, transport faults) as "we don't know" rather than "go ahead": resolution of an event/persistent finding is effectively permanent (next detection registers as a regression and inflates counters), so the safe default is to refuse and require either a successful re-verification or operator action. The gate has a symmetric counterpart, the verified stale-resolve contract: `reconcileStaleFindings` (`internal/ai/patrol_ai.go`) may auto-resolve an event/persistent finding that was seeded but neither re-reported nor resolved ONLY when the finding's key has a deterministic verifier and that verifier affirmatively confirms the failure signal is gone — absence of a re-report remains insufficient evidence for these categories, "still present" or inconclusive verification leaves the finding active (the same fail-closed default), and verifications are capped per reconcile pass (`maxVerifiedStaleResolvesPerRun`) with deferred candidates logged and retried on the next successful full patrol. Without this counterpart the lifecycle was asymmetric: a genuinely fixed backup or recovered service stayed an active finding indefinitely unless the LLM happened to call `patrol_resolve_finding`. `hasDeterministicVerifierForKey` (`internal/ai/patrol_findings.go`) is the single source of truth for which keys have verifiers, consulted by both the gate and the reconcile pass, and must stay aligned with the dispatch switch in `verifyFixDeterministically` (it previously listed two of the seven dispatch keys, silently skipping verification that existed). Finding keys normalize onto the canonical verifier vocabulary in `normalizeFindingKey` via an alias map of unambiguous directional synonyms (`high-cpu` → `cpu-high`, `high-memory` → `memory-high`, `high-disk` → `disk-high`) so deduplication and deterministic verification meet on one key, and the `patrol_report_finding` key guidance (`internal/ai/tools/tools_patrol.go`) teaches the canonical vocabulary; semantically distinct keys (`pbs-job-failed`, `node-offline`) must not be aliased onto verifier keys whose resource model they do not match, the assessment-recovery contract where the overall-health "Recent Patrol errors" coverage factor in `summarizeRecentPatrolCoverage` suppresses the score penalty once three consecutive trailing successful full Patrol runs exist at the most-recent end of the recent-runs window — so the grade reflects current reality after a Patrol-affecting bug is fixed rather than dragging stale failures forward for the ~9 hours it takes scheduled runs to age them out of the trailing-10 ratio, the orphan-tool-call-repair contract where `convertToProviderMessages` injects synthetic is_error tool result messages for any `tool_call_id` in an assistant message that has no matching downstream tool result, so a chat session that ended mid-tool-call (network drop, ctx timeout, browser crash) cannot poison its next message with the structural-violation error the provider rejects — the synthetic content is marked is_error=true and explains the interruption so the model can retry the call or proceed without the data, and the patrol-session-bound contract where `ExecutePatrolStream` calls `SessionStore.TrimMessages` after persisting each run's messages to cap the patrol-main session at 200 messages (roughly two recent runs' worth) — without the bound the file grew unbounded at every scheduled run, reaching 16 MB and 3,593 messages within a month and making every `AddMessage` rewrite linearly more expensive; the canonical Patrol forensic log is the `PatrolRunRecord` history surfaced at `/api/ai/patrol/runs`, not the chat-session-shaped file
   The bounded container-health failure vocabulary, including resource-family
   modifiers, health/check terms, and failed/failing/failure/unhealthy terms,
   must normalize independent of token order to the canonical
   `health-check-failed` key before finding identity is generated. Unrelated
   host-health, degraded, timeout, and unknown-token keys remain distinct.
   Existing-finding reconfirmation therefore converges on one durable issue
   even when the model changes an unambiguous key spelling between runs.
2. Keep AI runtime and shared API proof routing aligned in `registry.json`
3. Preserve explicit coverage for chat, Patrol, remediation, and cost-control behavior when AI runtime changes. Interactive Assistant and Patrol tool selection must remain model-owned: Pulse may provide governed context, tools, approval state, resource-resolution facts, safety policy, and neutral resource-scoped action history, but it must not add prompt-keyword routers, expected-tool retries, auto-recovery tool calls, keyword-matched prior-fix suggestions, or Pulse-authored remediation/finding fallbacks that choose the next investigative or corrective action for the model.
   Canonical invocation policy and the action lifecycle own authorization and
   independent verification. A successful follow-up read does not establish an
   action outcome, and no generic read/write sequence grants action authority.
   Assistant restored-session and recent-session context is also model-bound
   context, not an identity-policy bypass: when the referenced unified resource
   is governed or redacted, backend context builders must use the resource
   policy label or redacted value, suppress exact location/routing metadata
   such as `target_host`, and avoid exposing raw provider IDs, aliases, IPs, or
   hostnames to the model unless policy allows them.
   Resource drawer Assistant handoffs use the `resource_context` metadata kind
   and must attach product-originated resources as model-only context, not as
   saved user text. The backend must preserve that handoff kind through session
   persistence/restore, prepend an explicit selected-resource, discovery,
   tool-target-handle, data, raw-context, and action boundary before the
   resource context pack, and sanitize streamed assistant content, saved
   assistant messages, and tool results through the same unified-resource
   policy redaction path used for the context pack. The only model-facing tool
   target for an attached redacted resource is `current_resource`; read,
   query-get, and discovery tools resolve that handle server-side against the
   session-selected resource and must not treat `redacted by policy` as a raw
   infrastructure identifier. The live `resource-context` eval is the required
   regression proof for this path: the model must not ask which resource the
   user means, must not call discovery just to identify the attached resource,
   must report attached discovery readiness from context without a discovery
   tool call, must use the safe handle for scoped reads, may use the attached
   PII-free operational context (service access pattern, config/data/log paths,
   ports) to answer and guide the operator, must still refuse to expand
   environment variables, credentials, or other secret-bearing content, must not
   reveal raw hostnames, IPs, or aliases, and must not leak configured forbidden
   resource details in content or tool inputs.
   Plain-text resource references in live read, log, verification, or
   command-intent Assistant prompts may use the same selected-resource handle
   only after backend-owned canonical inventory resolution proves exactly one
   resource match, including explicit target-kind wording such as host, node,
   VM, container, or storage when inherited node labels would otherwise collide.
   That path must register the resource in the session resolved context, mark it
   as explicit current-turn access, and prepend a safe resource-context directive
   that exposes `current_resource` but not raw aliases, hostnames, platform IDs,
   paths, or other policy-redacted labels to external providers. Because that
   provider-safe rewrite replaces the user message, plain-text resolution runs
   only as a fallback when the prefetcher did not already resolve a structured
   mention or handoff resource; otherwise it would discard the injected
   resource context. Ambiguous or
   non-live prompts must fail closed to normal model clarification/query
   behavior; this is not a prompt-keyword router and must not choose, retry, or
   execute the model's next investigative action.
   Context-only resource handoff turns must be enforced at the tool-manifest
   boundary, not only by prompt wording: unless the operator explicitly asks for
   live runtime verification or a read attempt, the
   Assistant loop must receive no tools and must answer from the attached
   context or state that Pulse does not currently have that fact.
   Drawer handoffs route their attached resources through the same
   context-prefetch path as explicit `@`-mentions, so a handoff never delivers
   less resource context than an `@`-mention; identifying fields for local-only
   resources stay redacted at the model boundary.
   The base Assistant system prompt must instruct the model to use this
   provenance: briefly attribute facts drawn from Pulse context to their source
   (a context section, a metric/event timestamp), caveat stale or cached
   context instead of presenting it as current, and keep attribution concise and
   inline rather than citing every sentence.
   Patrol deterministic triage signals are prioritized evidence seeds for the
   configured model; they must not be described as a Pulse-authored final
   diagnosis, proof that unflagged resources are healthy, or a reason to
   prohibit the model from choosing governed tools for adjacent evidence
   relevant to its own findings.
   Patrol runtime failures are part of that runtime contract: provider, model,
   tool-calling, auth, quota, rate-limit, context-window, and connectivity
   failures must be classified in `internal/ai/` before they reach operators,
   surfaced as the synthetic Patrol runtime finding, and preserved on patrol
   run records as structured error summary/detail instead of collapsing to
   generic analysis-failed copy. If the provider fails after completing tool
   calls or accepting finding lifecycle operations, `runAIAnalysisState` must
   return that partial result alongside the error and the full/scoped run must
   preserve its completed tool records, token usage, accepted finding IDs,
   assessments, and display-safe partial response in `PatrolRunRecord`. This is
   diagnostic evidence only: the run remains `status=error`, must not trigger
   success-only runtime-finding recovery or absence-based reconciliation, and
   `internal/ai/qualification` must retain its diagnostic score while applying
   a hard failure for non-zero run errors or error status. Demo-mode Patrol run
   records must carry
   explicit source provenance and must not persist as live runtime evidence;
   outside demo mode, run-history reads, run lookup, and Patrol coverage
   scoring must filter both source-marked and legacy `demo-run-*` records so
   live assessment state cannot be contaminated by public demo fixtures.
   Unavailable-provider blocked states must direct operators to Assistant &
   Patrol provider settings and tool-capable Patrol model selection, not
   legacy `Settings > Pulse Assistant` copy.
   Patrol status must also carry server-authored readiness for provider,
   model, settings-persistence, and tool-calling prerequisites so the UI can
   block known-bad manual Patrol runs before they become generic runtime
   failures. The same `internal/ai` readiness evaluation must gate Patrol
   runtime admission directly: settings saves that are needed to recover a bad
   provider/model selection must persist and return structured readiness cause
   metadata, while manual run requests, scheduled ticks, and scoped
   alert/anomaly runs must fail or skip before LLM execution when the selected
   Patrol model/provider is known not-ready.
   Readiness checks may retain stable machine IDs such as `configuration`, but
   human-facing labels in the settings payload must frame the operator boundary
   as Patrol mode rather than Patrol configuration.
   Monitor-only Patrol autonomy saves are part of the same runtime gate:
   when the safe-remediation extension or entitlement is unavailable, the
   API must clear stale full-mode unlock and Autopilot activation state while
   clamping autonomy back to `monitor` (the monitor-only handler in
   `internal/api/ai_handlers.go` without the extension, the
   `SaveAutonomySettings` adapter in `internal/api/router_routes_ai_relay.go`
   behind the paid handler), and the browser state owner must refuse a paid
   level when a save starts, so paid remediation permission cannot survive
   through a free runtime save.
   Event-triggered Patrol runtime policy is also part of this gate: when local
   development background automation or another runtime policy pauses automatic
   alert/anomaly-triggered checks, `internal/ai` must expose an explicit
   event-trigger block reason/message on trigger status while preserving the
   operator's configured alert/anomaly trigger preferences.
   Patrol update-safety observation is part of the same read-state runtime
   boundary: Docker host/container snapshots used to detect image-digest
   divergence must be sourced from canonical `ReadState.DockerHosts()` views,
   with model-shaped data limited to the watcher adapter input rather than
   direct `StateSnapshot.DockerHosts` reads in the Patrol run loop.
4. Keep AI Discovery retired. The model-backed service-discovery runtime (`internal/servicediscovery`, `internal/ai/infradiscovery`, the `pulse_discovery` tool, `/api/discovery*` routes, and the `discovery_enabled`/`discovery_interval_hours`/`discovery_model` AI settings) was removed on 2026-10-11: no telemetry counter measured its use and the maintainer judged it a flop. Assistant and Patrol read resource evidence through `pulse_query`, `pulse_read`, and the unified-resource context sections; do not reintroduce a parallel model-backed discovery store, scheduler, or tool without a new evidence-backed `FEATURE_REQUESTS.md` entry. Network subnet discovery (`internal/discovery`) and the `discoveryTarget` agent coordinates are separate and stay.
5. Preserve auditability for outbound model-bound context exports and keep the export record aligned with the prompt boundary that actually reaches the provider
   External provider-bound unified-resource context must enforce the same
   data-handling policy the export audit records: `local-only` resources are
   represented only as aggregate posture and omitted from detailed prompt
   sections, while sensitive alert text is scrubbed through the shared
   unified-resource redaction helper before it reaches a non-local model.
   The final provider-bound chat, Patrol, investigation, tool-result, and any
   retained legacy managed-model compatibility requests must also pass through
   that same resource-policy sanitizer immediately before transport, so later
   agentic turns cannot reintroduce local-only identifiers after the original
   context export.
6. Keep AI resource and incident context aligned with the canonical unified-resource timeline before falling back to patrol-local change detectors
7. Keep platform assistant read/control claims aligned with
   `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`. New
   platform-native reads or writes must extend the shared Assistant tool
   contracts, and read-only or augmentation-only platforms must stay explicit
   there instead of drifting into provider-local tools.
8. Keep Pulse Assistant action governance canonical in the shared tool
   registry. Tool prompts and approval surfaces must derive read, mixed, write,
   and approval-policy claims from `internal/ai/tools/registry.go` and
   `internal/ai/tools/executor.go` instead of maintaining hand-written
   prompt-only tool lists. Fallback Assistant governance text and Patrol
   system-prompt tool summaries must mirror the same registry-owned capability
   shape instead of presenting stale read-only summaries. Frontend approval cards must surface backend
   approval risk/description without hiding a pending approval when skip or
   deny fails. Action-producing tools must also persist the unified
   `ActionPlan.Preflight` dry-run boundary through the canonical planner
   (`internal/actionplanner/planner.go`) rather than leaving dry-run
   availability as chat-only text.
   When the shared registry blocks a control tool in read-only mode, its
   operator guidance must point to Pulse Intelligence > Provider & Models
   settings and the Pulse Assistant Permissions Control mode, not legacy Pulse
   Assistant settings paths.
   Runtime status, preflight, settings-persistence, profile-suggestion, and
   remediation-impact messages must preserve the same product boundary:
   use Pulse Assistant only for the first-party in-app Assistant runtime itself,
   Provider & Models for the shared provider/model settings surface, and Pulse
   Intelligence for the shared Assistant/Patrol/agent substrate rather than
   reviving the legacy generic `Pulse AI` label or `Pulse Assistant settings`
   copy. Pulse Intelligence external-agent setup copy may point to Patrol mode,
   but it must frame the first step as setting how autonomous Patrol should be
   before connected agents request work; it must not present connected-agent
   setup as a separate operations-loop, MCP-readiness, or feature-unlock model.
9. Keep self-hosted Patrol messaging aligned with the v6 GA product contract:
   ordinary self-hosted installs use BYOK or local providers, and the runtime
   must not surface retired managed-model credits, trial prompts, account-backed
   AI activation, or general hosted chat entitlement in the normal app.
   The shared app shell must also keep `/cloud` and `/cloud/signup` out of
   ordinary self-hosted public routes so Cloud acquisition cannot reappear as a
   proxy for retired hosted-model or AI quickstart activation.
   The public Pulse Intelligence overview must likewise use productized context language such
   as alert history, Patrol runs, and resource timelines instead of presenting
   `incident memory` as a standalone feature. It must also describe Patrol
   baselines, trends, correlations, forecasts, and deterministic signals as
   model-bound evidence/context rather than Pulse-authored intelligence or
   fallback finding creation.
10. Keep discovery-analysis prompt bounds and response budgets aligned across
    `internal/ai/service.go` and the shared service-discovery prompt builders:
    the runtime must reserve enough output tokens for structured discovery JSON,
    and discovery prompts must cap fact/path/port fan-out explicitly instead of
    relying on providers to truncate oversized infrastructure inventories.
    That same runtime-owned command-target boundary must resolve hostnames
    through `internal/unifiedresources/hostname_equivalence.go`.
    `internal/ai/tools/internal_routing.go`,
    `internal/ai/tools/tools_control.go`, and adjacent AI command helpers may
    match a short host against its FQDN, but they must not broaden that
    fallback into a generic short-name collapse that would make two distinct
    FQDNs with the same short host look interchangeable.
11. Keep AI runtime transport compatibility separate from operator-facing
    product copy. Existing Patrol payload fields such as `fixed_count`,
    `auto_fix_model`, and `patrol_auto_fix` may remain stable wire/API names,
    but frontend comments, API denial messages, runtime logs, status labels,
    CLI help, and commercial prompts that describe the capability must use safe
    remediation or remediation wording.
12. Keep AI runtime controls runtime-owned rather than tier-owned.
    `frontend-modern/src/components/Settings/AIRuntimeControlsSection.tsx`
    owns the cost budget and request timeout, and must not add Pro-badge
    suffixes or local commercial tracking around those runtime controls. The
    Assistant chat control level has no browser surface since the chat window
    was retired; it remains an API setting for Pulse Mobile chat.
13. Keep Assistant control and Patrol paid runtime settings entitlement-effective
    at every runtime boundary. Stored config may preserve autonomous, Patrol
    auto-remediation, and alert-triggered analysis preferences so they come
    back if entitlement returns, but API responses, chat executor startup,
    restart, settings-update, request-clone paths, and Patrol execution must
    clamp those values through runtime entitlements before exposing or enforcing
    them.
14. Keep agent-backed Patrol reachability checks aligned with the agent command
    policy. `internal/ai/patrol_prober.go` may use connected agents for
    read-only guest ping probes, but it must validate each target as an IP
    address and issue only the single-target `ping -c 1 -W 1 <ip>` command
    shape covered by the agent-exec auto-approval policy. It must not compose
    shell loops, accept hostnames, interpolate unvalidated targets, or bypass
    approval requirements for compound commands.
15. Keep OpenAI-compatible streaming finalization fail-closed. `ChatStream` may
    flush buffered final SSE lines when EOF arrives with unread bytes and may
    accept providers that omit `[DONE]` only after a terminal `finish_reason`,
    but it must not emit `done` or executable tool calls from partial tool-call
    builders when the stream closes before that terminal provider state.
    Custom compatible endpoints use portable `max_tokens`, omit the optional
    `stream_options` extension, and omit Authorization when no key was saved.
    A server that explicitly rejects streaming, or returns a complete
    `application/json` completion for `stream=true`, may use the buffered
    compatibility path only after the entire response and tool arguments have
    validated; generic request failures, mid-stream failures, incomplete
    responses, and malformed tools must not trigger or survive that fallback.
    Restricted outbound transport must preserve metadata-service blocking,
    DNS validation, and same-origin redirects while allowing operator-selected
    private and loopback model servers.
16. Keep Patrol investigations product-facing through the shared
    `aicontracts.InvestigationRecord` contract. Patrol may keep
    `InvestigationSession` as execution detail, but Assistant handoff,
    unified findings, persistence, and approval/remediation context must use
    the durable investigation record when they need operator-facing
    investigation context. The durable record carries top-level `impact`
    and `rollback` strings alongside the existing `verification` array, so
    Assistant `/api/ai/chat` enrichment surfaces consequence-if-ignored and
    undo intent when Patrol has populated them and remains silent when the
    fields are empty rather than fabricating placeholder analysis through
    the model. The TS API client must keep its `InvestigationRecord` and
    `InvestigationRecordTrigger` mirrors aligned with the Go struct,
    including the `trigger.cause` string, so frontend handoff context does
    not lose backend-attributed failure cause. Detection-time finding
    creation may author `Finding.Impact` directly so the
    consequence-if-ignored statement is set at finding birth rather than
    waiting for an AI investigation to derive it;
    `BuildFindingInvestigationRecord` then propagates `Finding.Impact`
    into `InvestigationRecord.Impact` without transformation, and the AI
    engine must not synthesize impact text from severity or category when
    the finding source has not authored one. The Patrol runtime-failure
    classifier (`internal/ai/patrol_runtime_failure.go`) is the canonical
    example: it stamps a constant impact string on every runtime-failure
    cause because the operational consequence of a non-running Patrol is
    invariant across causes, and only the recommendation varies. That
    detection-time impact text propagates through `FindingsStore.Add` (which
    must overwrite `existing.Impact` alongside `Description` and
    `Recommendation` so re-detected findings adopt freshly-classified
    impact rather than preserving stale empty values left by older
    binaries) and through the Finding to UnifiedFinding conversion in
    `internal/api/router.go` so the operator-facing
    `unified.UnifiedFinding` surface receives the same impact string the
    durable investigation record carries. Threshold-alert findings author
    impact through a parallel hand-curated catalog in
    `internal/ai/unified/alerts.go` (`generateImpact`), keyed on alert
    type rather than severity or category, so threshold findings carry
    consequence-if-ignored copy at detection time without depending on an
    AI investigation. The unified-store update paths
    (`UnifiedStore.AddFromAlert` and `UnifiedStore.AddFromAI`) must
    propagate Impact on re-detected findings the same way they propagate
    Description: AddFromAlert backfills empty Impact on existing findings;
    AddFromAI overwrites existing Impact when the incoming finding carries
    one. Threshold-alert conversion must not synthesize remediation
    recommendations; unknown alert types must return an empty impact rather
    than synthesizing generic copy.
    Investigation-record `Rollback` is sourced from the canonical
    `RemediationPlan` when one exists for the finding:
    `AggregatePlanRollbackSteps` in
    `internal/ai/investigation_records.go` flattens
    `RemediationStep.Rollback` strings into a deduplicated record-level
    slice, and the patrol findings build site
    (`internal/ai/patrol_findings.go`) populates `record.Rollback` from
    `remediationEngine.GetPlanForFinding` when the engine and an active
    plan are present. Rollback must remain absent rather than fabricated
    when no plan exists, mirroring the impact rule.
    LLM-generated AI patrol findings author impact through the
    `patrol_report_finding` tool schema in
    `internal/ai/tools/tools_patrol.go`: the tool exposes an optional
    `impact` parameter, `PatrolFindingInput.Impact` carries it through,
    and `patrolFindingCreatorAdapter.CreateFinding` writes it onto
    `Finding.Impact` so the LLM's authored consequence-if-ignored copy
    flows through the same propagation path used by curated catalogs.
    The patrol system prompt instructs the LLM to author concrete
    operational consequences (named workloads, jobs, recovery windows)
    and to leave `impact` empty rather than fabricate one when the
    consequence is genuinely unknown; the runtime must not synthesize a
    default in that case.
    The Assistant tool layer carries no action broker. Its former
    shell-command, guest, Docker, Kubernetes and file-mutation handlers,
    the `executeCommandWithAudit` and `executeNativeActionWithAudit`
    execution lanes, and the approval-record, preflight and plan-hash helpers
    behind them (`createApprovalRecord`, `approvalPreflight`,
    `classifyApprovalCommand`, `VerificationCommandForCommand`,
    `approvalPlanHash`) were removed because nothing in production called
    them: `pulse_control` only plans a typed resource action through
    `executeControlResource`, and plan freshness
    (`unifiedresources.ErrActionPlanDrift`) and the operator remediation lock
    are enforced by `internal/actionlifecycle`. Class-derived read-after-write
    verification, per-class preflight copy and the `plan_drift:` audit prefix
    therefore have no tool-layer implementation; a change that needs one
    belongs in the lifecycle, not back in `internal/ai/tools`.
    `FindingsStore.GetTrustSummary` returns a snapshot of how currently
    tracked findings have resolved (tracked, currently-active, resolved,
    auto-resolved, fix-verified, fix-failed, dismissed-as-noise,
    dismissed-as-expected, dismissed-as-later, suppressed,
    regressed-at-least-once). It is the data layer for trust metrics on
    operator surfaces. `PatrolService.GetFindingsTrustSummary` exposes
    the same snapshot through the service boundary, and the
    patrol-status API response carries it under
    `PatrolStatusResponse.Trust` so the Patrol page can render a Trust
    strip without callers reaching past the service boundary. The summary is intentionally a snapshot of the
    in-memory store, not lifetime totals; once findings are cleaned up
    they no longer contribute. Downstream surfaces must frame the
    counts as current-state distribution rather than historical
    aggregates, and the AutoResolved bucket includes both the
    `Resolve(auto=true)` path and the
    `UpdateInvestigationOutcome(fix_verified)` path.
    Findings carry a `previous_resolved_fix_summary` field as
    operational memory across regressions: when a finding that had a
    resolved investigation with a proposed fix is re-detected,
    `FindingsStore.Add` captures
    `existing.InvestigationRecord.ProposedFix.Description` into the new
    field BEFORE clearing the investigation record, and the chat-context
    builder surfaces it as a "Previous Resolved Fix" line so Assistant
    sees what worked last time rather than treating each regression as a
    blank-slate diagnosis. The summary mirrors onto
    `unified.UnifiedFinding` and propagates through the Finding to
    UnifiedFinding conversion in `internal/api/router.go` and through
    `UnifiedStore.AddFromAI`'s update branch (non-empty overwrite). The
    TS API client mirrors the field on `UnifiedFindingRecord` and
    `Finding`, the aiIntelligence store normalizer copies it through as
    `previousResolvedFixSummary`, and `FindingsPanel.tsx` renders it on
    the expanded finding card so the operator sees the memory cue
    inline rather than only inside Assistant chat context. When `/api/ai/chat` receives `finding_id`, the
    runtime must enrich the provider turn from that durable record while
    preserving the user's authored prompt as the persisted conversation
    message; the model-only handoff may persist as session metadata so
    same-session follow-up turns keep the Patrol finding context without
    mutating saved user messages. Patrol run-history handoffs follow the same
    backend-owned context rule: the browser may seed only safe `patrol_run`
    metadata such as run ID/type/status/runtime-failure posture, while
    `/api/ai/chat` must rehydrate model-only run context, scoped resources, and
    safe failure detail from the current Patrol run record before model
    execution and again on same-session follow-up turns. If the Patrol run no
    longer resolves, browser-authored run context, resources, and actions must
    be dropped rather than used as fallback provider context. When the handoff
    identifies a resource, the
    runtime may also seed the session's resolved-resource scope, but only through
    canonical unified-resource tool registration so allowed actions, executors,
    and explicit-access checks stay governed. Structured handoff resource
    references may persist as session model-context metadata for follow-up turns,
    but they remain references only; each turn must rehydrate them from the
    current canonical unified-resource model before action validation can use
    them. Structured finding references from Patrol/Assistant handoffs may also
    persist as session model-context metadata so follow-up turns can refresh the
    current unified finding and investigation record before model execution;
    those references remain model-only context selectors, not saved user text or
    lifecycle authority. When the current finding identifies root-cause or
    correlated finding IDs, Assistant may resolve those related findings through
    the current unified finding store and include compact related-finding
    summaries plus their structured handoff resources as model-only explanation
    context. Those summaries must carry current recency facts and latest
    lifecycle state from the related record rather than only title/resource
    prose. Those related findings must be deduplicated, bounded, refreshed from
    current store state, and treated only as context for the same operator
    conversation; they do not grant approval, lifecycle, disclosure, or
    execution authority. If the referenced finding no longer resolves through
    the current unified finding store, Assistant must invalidate the stored
    model-only handoff and unpinned handoff-seeded resource scope instead of
    falling back to stale investigation context. The refreshed finding context
    must include unified finding lifecycle and recency facts such as active,
    resolved, snoozed, dismissed or suppressed state, detection/last-seen/
    resolved timestamps, recurrence, regression, and recent lifecycle events so
    Assistant explains the current Patrol record rather than only the original
    investigation narrative. The saved-session handoff envelope must also
    preserve first-class Patrol source identity when product callers provide
    safe metadata. Saved Patrol assessment sessions (the browser no longer
    creates them) remain `patrol_assessment` whole-surface review sessions even
    when their bounded action references name individual findings; the session
    list must not infer a `patrol_finding` identity from those action
    references once metadata is present. Saved Patrol mode save-failure
    sessions remain `patrol_configuration_failure` sessions for compatibility
    and may expose only the safe runtime-failure boolean needed for browser
    presentation. Run-specific
    fields stay reserved for `patrol_run` handoffs, while hidden model context,
    command payloads, preflight output, and action results remain
    backend-owned. The finding briefing may surface the primary
    finding's current recency facts, bounded evidence snapshot,
    verification summary, latest lifecycle event, and governed action
    artifact metadata, but it must not generate Pulse-authored attention
    reasons, operator-decision framing, or remediation next-step guidance
    for the model. Detailed lifecycle events must stay in a bounded
    `[Finding Lifecycle Context]` block with an explicit model-only boundary.
    Assistant runtime may also hydrate canonical
    resource-policy context for those handoff resources, using the same
    unified-resource resolution and policy presentation helpers that govern
    mention prefetch and provider-bound redaction; that context remains
    model-only handling guidance, not saved user text or disclosure authority.
    Before injecting any product-originated handoff context into the model
    prompt, the runtime must also apply canonical resource-policy redaction to
    the assembled handoff text itself, including finding briefings and
    lower-level finding/action context, so local-model prompts and non-local
    provider transport share the same governed identity boundary.
    Assistant runtime may also hydrate current canonical resource-state context
    for those handoff resources, including compact status, freshness,
    source-health, metric, incident, and governed-capability summaries from the
    unified-resource model. That state snapshot remains model-only read-only
    infrastructure context, must honor the same policy/redaction boundary before
    provider transport, and must not grant approval or execution authority.
    Assistant runtime may also hydrate canonical
    relationship context for those handoff resources through
    `FormatResourceRelationshipContext(...)` and canonical parent-edge synthesis,
    but those topology facts remain read-only explanation context and do not
    grant action authority. Assistant runtime may also hydrate recent changes for
    those handoff resources from the canonical unified-resource timeline as
    model-only context on each turn; it must resolve product-originated handoff
    references through the canonical unified-resource provider before querying
    timeline changes, with raw handoff IDs used only as a compatibility fallback.
    Those timeline facts remain read-only explanation context and do not grant
    action authority. Assistant runtime must also treat the shared
    `internal/agentcontext` resource context pack as the canonical rich
    resource-grounding substrate for product-originated resource handoffs:
    runtime/discovery, topology, safety, policy, and recent-change facts are
    hydrated as model-only context from canonical unified resources and the
    resource store, carry provenance/freshness/redaction metadata, and must not
    include raw command output, provider config, environment values, mount paths,
    label maps, or secret-bearing metadata. The runtime may also persist structured pending-action and
    approval references from the same investigation record as
    model-context metadata, and the API handoff builder may recover the current
    live Patrol investigation-fix approval by finding ID when the durable record
    does not yet carry the latest approval ID. Those references are review
    context only: they must not include raw command text, must not grant
    approval or execution authority, and must route any requested action back
    through the governed approval and fix flow. Patrol provides the
    configured LLM with observed finding context, evidence, policy posture, and
    governed action state; the LLM owns diagnosis and fix reasoning.
    Operator-visible handoffs must not describe Patrol as having already
    authored the correct fix. The finding briefing may carry only
    factual governed action artifact metadata from those same structured
    references after live-approval recovery, including safe current status,
    request/expiry timestamps, approval policy, action plan identity, plan
    expiry, and dry-run posture, so Assistant sees current approval/action state
    without Pulse choosing the model's next investigative or corrective step.
    When those references include an approval ID, Assistant runtime may refresh
    a current status snapshot from the canonical approval store on each turn,
    but it must enforce org scoping and still omit the approval command payload.
    When those references resolve to a governed action plan or action audit,
    Assistant runtime must hydrate the canonical action ID, lifecycle state,
    requester, capability, approval policy, plan expiry, preflight/dry-run
    summary, and terminal success/failure state from the action-audit store
    rather than treating the original approval as the current action truth.
    That action snapshot remains model-only review context and must expose
    lifecycle status rather than raw execution output or command text.
    The public chat session contract may expose only a bounded
    `handoff_summary` for this private model-context metadata so reloaded
    Assistant sessions can still be identified as scoped Patrol/product
    handoffs. That summary may include the handoff kind, finding ID, resource
    and Patrol run ID, safe run type/status/runtime-failure flags, resource and
    action counts, a primary resource label, last-known approval/action status,
    risk level, and timestamp, but it must not expose Patrol-authored
    recommended next-step titles, recommendation details, route-owned action
    labels, model-only handoff text, runtime failure detail, action
    preflight/result bodies, remediation descriptions, raw commands, or
    approval command payloads. Its
    `requires_approval` field is a current operator-decision flag only: pending
    approval states may set it, but approved, denied, rejected, executing,
    completed, failed, expired, or otherwise historical action references must
    remain action context without being relabeled as requiring approval.
    Before `/api/ai/sessions` returns summaries with stored handoff action
    references, the chat runtime must refresh their safe approval/action status
    from the canonical approval store and action-audit store. Session listing is
    an operator decision surface, so it must not leave stale pending/approval
    labels after the governed action moved on, and it must still omit raw
    commands, preflight bodies, and execution output. That refresh is bounded,
    not exhaustive: the list is newest-first and only the
    `maxSessionHandoffRefreshPerList` (20) most recent action-carrying sessions
    are re-checked per list call; deeper history self-heals because the send
    path refreshes the active session's handoff actions on load before the
    turn executes.
    Client-originated `handoff_context`, `handoff_resources`, and
    `handoff_actions` plus safe `handoff_metadata` are one-shot request seeds
    for the first successful chat turn. Patrol next-step titles, details,
    labels, and route-owned hrefs do not belong in `handoff_metadata`, and
    model-context text parsing must not resurrect them as a legacy fallback.
    Later turns rely on backend-owned session model-context hydration and
    current canonical stores rather than resent handoff payloads.
    Proposed-fix command text must stay out of both the persisted chat message
    and the model-only handoff context, and command payloads remain
    approval-context data, not conversational copy.
    `/api/ai/chat` clamps every exchange to approval-required mode, Patrol
    finding handoffs included: `ChatRequest` has no `autonomous_mode` field, so
    caller JSON (a stale client's `autonomous_mode:true` included) is dropped at
    decode, and `HandleChat` passes `AutonomousMode:false` into the chat service
    whether or not the request carries a `finding_id` or model-only briefing,
    resource, or action context. Clients send no execution-mode field.
    That clamp belongs to the
    backend/API execution boundary, does not mutate the user's persistent AI
    control setting, and prevents product-originated Patrol action context from
    becoming silent command authority.
    The chat runtime must apply that server-set request-local mode
    (`chat.ExecuteRequest.AutonomousMode`) to
    both the per-request `AgenticLoop` and the cloned `PulseToolExecutor`;
    persistent autonomous settings must not leak into approval-required
    requests through executor state. When the request forces approval mode
    and the saved control level is autonomous, the executor clone must clamp its
    effective control level to controlled for that request only, so even
    policy-allowed diagnostic commands require operator approval in public chat
    without mutating the user's saved setting.

### Operational attention projection boundary

`internal/ai/attention.go` is the canonical Patrol attention read-model
projection over alert-owned operational records, transition timelines,
evidence envelopes, and recovery-owned protection postures. It may order,
filter, paginate, summarize, and shape selected Assistant context, but it must
not create a second writable lifecycle or infer a healthy state when lifecycle
coverage is unavailable. The projection also owns customer-facing attention
titles: provider alarm messages yield their bounded alarm subject, generic
resource/storage incident types become plain issue language, and known
acronyms such as CPU, ZFS, and I/O retain their canonical casing. Clients must
not receive machine-title-cased placeholders such as `Resource Incident` and
repair them independently. VMware incidents are recognized by the alert's
`incidentCode` metadata, never by parsing message wording: a
`vmware_alarm_state` alert is titled with the alarm name its message carries,
cut before any appended consumer summary, and a `vmware_health_state` alert
reads as VMware host or datastore health. Records persisted before VMware
summaries became the alarm name still carry "<entity> <moref> has VMware alarm
<name> (<colour>)", so the title strips that wording to the same alarm name.
Patrol's seed context lists active and recently resolved alerts by resource:
each line names the alert's resource unless the message already does, because
provider incidents such as vCenter alarms say only what was flagged.
A recently resolved alert that closed without recovering, such as a node metric
handed to its Pulse agent, must not read as a recovery to Patrol, the
assistant or the alert tool: Patrol's seed writes `closed <ago>. <summary>` in place of `resolved <ago>`, the
assistant's alert context writes `closed <ago> ago) <summary>`, the
`pulse_alerts` `resolved` action returns the summary as `resolution` with
`successor_resource_id`, and incident memory titles the close with the summary
(`alertResolvedEventSummary`, or the `alert_resolution` resource-change
metadata when it projects from the canonical timeline).
`TestSeedHealthAndAlerts_HandoverIsNotARecovery`,
`TestService_buildAlertContext_HandoverIsNotARecovery`,
`TestExecuteListResolvedAlertsReportsHandover` and
`TestIncidentStore_HandoverCloseIsNotARecovery` pin it. Performance reports
carry the resolution too: the report narrator sends such an alert with
`resolved: false` and the engine's summary as `resolution`, plus
`successor_alert_listed` when the same report lists the alert the resource
it moved to raised for that metric. Its system prompt forbids counting the
alert as resolved or calling the resource healthy on its account, and tells
the model to describe a covered condition once, from the successor's alert
(`TestBuildReportNarratorPayload_HandoverIsNotARecovery`). The
deterministic verdict lives in `pkg/reporting` (see api-contracts). The projection also collapses open/resolved churn:
when a record's timeline holds at least four open-to-resolved or
resolved-to-open transitions inside the last 24 hours, the item carries a
`flapping` summary (transition count, window, first and latest transition) and
clients present one item with that label; the full transition list stays in the
detail timeline as forensic detail. The window and threshold are the shared
constants in `internal/ai/findings_storm_throttler.go`, so alerts and Patrol
findings agree on what flapping means. `internal/ai/patrol_metrics.go` exports only
low-cardinality lifecycle-state counts, queue age, acknowledgement time, and
calm-evaluation age; raw resource IDs are forbidden as metric labels.

Assistant receives a selected attention item only after the operator opens it.
The handoff is explanation-only and cannot infer a capability, approve an
action, or replace evidence and lifecycle truth.

A held threshold alert's attention summary leads with the live reading (issue
#2068). An alert's `Message` keeps the last breach while the alert waits below
its trigger for the clear level, so for a `MetricStatus` in the `latched` or
`recovering` phase that is under ten minutes old, the projection builds
`plainLanguageSummary` from the status instead: the reading now, then for a
latched alert the alert level it is back under and the clear level with its
hold, and for a recovering alert the clear level, delay and progress so far
(rounded down). The words and rounding follow the frontend's
`features/alerts/metricAlertPresentation.ts`, and a guest's disk is named
from `metadata.label`. Temperatures stay in °C like the alert messages,
because the server does not know a viewer's unit preference. A breaching
alert keeps `Message`, which tracks the breach, and a stale or missing status
falls back to it. The Patrol inbox, the investigation handoff and API clients
read that summary, so none of them re-derives the phase.

### Patrol model-readiness evidence

Keep the Patrol model-readiness advisor provider-neutral and evidence-based.
`POST /api/ai/patrol/readiness` must use synthetic, read-only streaming
scenarios to distinguish provider connectivity, typed tool-call protocol,
context selection quality, continuation behavior, and latency rather than
treating a successful model-list or chat request as proof of autonomy
suitability. Apply the shared cost budget and record probe token usage under
`patrol_readiness`. Persist only sanitized scores, timings, and model metadata;
invalidate evidence when the selected provider, model, credential, endpoint, or
relevant timeout budget changes. Monitor/Watch and Approval/Ask First may be
verified independently, while Safe auto-fix and Autopilot remain explicitly
unassessed until deterministic remediation evals exist.

A cancelled run is not provider evidence. The runtime failure classifier
(`internal/ai/patrol_runtime_failure.go`) must classify mid-run cancellation as
`interrupted`, never as a provider connection or analysis fault;
`context.DeadlineExceeded` keeps its provider-path timeout classification.
Cancellation is established from the run itself, never from error wording: an
error that wraps `context.Canceled`, or a run whose own context is cancelled.
A raw `context canceled` substring in a provider error body is not evidence of
anything, because providers embed that phrase when they abort an upstream
request of their own while the Pulse run is healthy, and treating it as
cancellation would persist a genuine provider failure as not assessed. Callers
holding the run context must classify through the context-aware entry point so
a torn-down request is not blamed on the provider, and the detail rewrite
follows the same rule as the cause. An interrupted readiness evaluation reports
the overall status and every unfinished dimension and autonomy mode as not
assessed while preserving the per-scenario evidence completed before the
interruption, and it must not overwrite the last completed evaluation in the
readiness cache. The readiness handler streams insignificant JSON-whitespace
keepalives while the evaluation runs so intermediaries with short read timeouts
do not sever slow local-model runs; severed or operator-cancelled runs surface
as interrupted, not as model verdicts (#1640).

A Pulse-side defect is not a model verdict either. A panic recovered on the
readiness evaluation path produces a readiness result with the
`internal_error` cause and every dimension and autonomy mode reported as not
assessed, because nothing about the model was measured. `internal_error`
carries the same non-evidence status as `interrupted`: neither may be
presented, cached, or reasoned about as a statement about the provider or the
selected model.

Readiness is claimed from the completed overall verdict, never from a single
dimension. An interrupted run keeps its tool-protocol dimension at `pass` when
every tool scenario finished before the cancellation, so any gate that reads
`Dimensions.ToolProtocol` alone reports "Patrol ready" from an evaluation that
never completed; the `tools` readiness check must require the snapshot's own
overall verdict (`Success`) before reporting ready. A snapshot carrying no
verdict — overall status `not_assessed`, or the `interrupted` or
`internal_error` cause — is not a failure either. It falls back to the same
base-config classifier an absent snapshot uses, capped at a warning so an
unfinished run can neither claim readiness nor block Patrol: a not-ready
readiness status disables the Patrol run control, and a severed or cancelled
check must leave Watch-mode Patrol runnable (#1640). A completed run whose tool
protocol passed while the overall verdict fell short warns on the tools check
rather than blocking, because the dimension that actually failed carries the
verdict on its own check.

## Current State

### AI Discovery removed (2026-10-11)

AI Discovery, the model-backed service-identification layer, is gone. The
`internal/servicediscovery` store and scanner, `internal/ai/infradiscovery`,
the `pulse_discovery` Assistant/Patrol tool, the `/api/discovery*` routes, the
mock Discovery fixtures, the resource/guest/node/Docker drawer Discovery tabs,
identified-service cards, readiness badges, provenance markers, suggested
web-interface URLs, the Workloads AI Context column, the chat "Discovery is
off" hint and the AI Service Context settings page were all removed. No
telemetry counter ever measured its use (the `discovery_enabled` telemetry
field folded it together with network subnet discovery), the maintainer judged it a flop, and
it carried a separate scheduler, store and model route for context that
`pulse_query`, `pulse_read` and the unified-resource context sections already
supply.
Network subnet discovery (`internal/discovery`, `/api/discover`, the
Infrastructure discovery settings) and the `discoveryTarget` agent coordinates
are separate features and stay. Stored `discovery_enabled`,
`discovery_interval_hours` and `discovery_model` AI settings are ignored on
load. The old `/settings/pulse-intelligence/discovery` route redirects to
Assistant settings.

Patrol's guest inventory table drops the Service column, the resource-context prefetcher no longer reads a Discovery provider, and stored HTTP JSON observers that depended on Discovery endpoints now fail as `observer_runtime_unsupported`.

### Agent surfaces expose execution-time readiness refusal

The canonical capability manifest and generated Pulse MCP documentation expose
`action_execution_unavailable` on `execute_action` as a stable permanent
refusal. Assistant and MCP clients may branch on that code after an approved
plan loses executor-owned live readiness, but an adapter cannot retry around
the terminal refusal, substitute a different capability, or dispatch through a
provider-local path. The bounded human reason and provider reason code are
diagnostic evidence only; the shared manifest remains the error-vocabulary
owner, and canonical Actions remains the sole mutation plane for both human and
automatic policy execution.

First-session Patrol discoverability is a contract concern. Successful
first-time provider setup (the Provider & Models setup modal) must refresh the
session `assistantEnabled` capability in place through
`frontend-modern/src/stores/aiAvailability.ts`, so the Patrol tab and alert
Investigate actions appear without a page reload; the enable toast points at
Patrol. Patrol static readiness treats the blessed Ollama Patrol model
(`config.OllamaSuggestedPatrolModel`) as Ready — the recommended free path
must not report itself degraded — while other Ollama models keep the
unverified-tool-support warning, which names the selected model instead of
instructing the operator to pull the model they already selected.
`pulse_summarize` fleet-mode argument errors instruct the model to enumerate
resources itself rather than interrogate the operator for resource IDs;
assistant-facing tool errors must prefer self-recovery guidance over operator
questions.

Automatic Watch detection now carries a declining 7,000-token run allowance:
normal structured turns receive at most 2,048 output tokens, terminal summaries
receive at most 1,024, and the remaining allowance is projected on every
provider request without aborting the final decision when prior usage runs
high. Providers with native output limits enforce `max_tokens`; local Codex and
Claude subscription transports additionally receive a low reasoning-effort
control. Ollama maps that same provider-neutral hint onto its native `think`
field: low effort selects the fast non-thinking path for hybrid-thinking
models, GPT-OSS receives its required named `low` level, and an absent hint
omits the field so the model keeps its own default. The brief Patrol tool-call
preflight uses low effort because it verifies protocol transport rather than
diagnostic depth. This economy posture is detection-only. Interactive
Assistant and Pro investigation retain their normal reasoning depth and
model-led diagnostic freedom. Non-interactive tool-turn prose remains available in provider context
but is not concatenated into the persisted Patrol run analysis; the bounded
terminal summary is the sole operator conclusion.

OpenRouter is part of that provider-neutral reasoning-control boundary. When a
runtime request declares low, medium, or high `ReasoningEffort`, both streaming
and non-streaming OpenRouter transports must serialize the gateway-native
`reasoning.effort` request object. An absent or invalid hint must omit the
object, and non-OpenRouter OpenAI-compatible endpoints must not receive this
OpenRouter-specific shape. This is required for bounded Patrol recovery turns:
raising the output allowance without constraining reasoning can consume the
entire allowance before a required observer or action proposal is emitted.

Patrol Pro investigation now separates three previously conflated quantities:
model responses, model-selected tool calls, and evidence-tool calls. The
operator-facing investigation budget limits evidence calls; core derives a
separate model-response ceiling with reserved completion capacity, and the
terminal `patrol_propose_action` call does not consume evidence budget. The
default budget is ten evidence calls. This is a resource limit, not a claim of
diagnostic sufficiency. Operators may choose the bounded 5–30 range. The
investigation execution profile reports the remaining limit in system context
and structurally removes evidence tools at exhaustion while retaining only the
typed proposal route and final prose. Tool results contain observations, without
appended completion instructions. This remains model-led: Pulse defines the
four completion questions (symptom, root cause or uncertainty, affected scope,
safe next action), not a prescribed diagnostic tool sequence. Structured run
completion also preserves the action handoff implied by the model's own
judgment: when the collected evidence supports a safe advertised remediation,
the model must submit that typed proposal before its final summary instead of
leaving the same recommendation as inert prose. When evidence is insufficient,
it concludes with explicit uncertainty and no proposal. Pulse still neither
selects a diagnostic sequence nor invents a remediation; it ensures that the
model's supported action decision reaches the canonical policy, approval, and
execution plane. Structured run results and persisted investigation records
carry truthful model-turn and
evidence-call counters. Qualification `max_evidence_calls` gates actual call
volume independently of `max_tools_used`, which continues to measure distinct
tool-name diversity; replay falls back to historical evidence IDs only for old
reports that predate the explicit counter.

Assistant compatibility audit writers no longer use whole-record action-audit
replacement. Fresh approval and direct-action records use atomic creation with
their initial events, while decisions, refusals, execution starts, and results
use the same typed CAS transitions as the canonical action lifecycle. This
keeps legacy boundary producers from reopening the state-rewind path while
their mutation-plane consolidation remains separately governed.

AI provider model-cache identity must never expose reusable credential material
or deterministic unkeyed credential hashes. `internal/ai/service.go` derives
cache-only credential identities with a process-local random HMAC key, so an
unchanged configuration remains cacheable during the process lifetime while
credential rotation invalidates the cache without creating an offline
credential verifier.

Remediation-history persistence is a serialized snapshot boundary. Async log
and rollback saves must take a deep copy of rollback metadata while holding the
record lock, then serialize and atomically replace the history file under one
persistence lock. Concurrent saves must never marshal shared rollback pointers,
race over the common temporary path, or allow an older async writer to replace
newer in-memory state.

The canonical findings store must present one active Patrol issue per real
problem. When the model reports an equivalent active sibling under a different
finding ID, key, or severity for the same resource, `internal/ai/findings.go`
must merge it into the existing active finding before it reaches
`/api/ai/patrol/findings`, keep the highest observed severity for the merged
issue, increment recurrence, and record a bounded duplicate-merge lifecycle
event instead of creating a second operator-facing row. Distinct symptoms on
the same resource, such as CPU pressure and memory pressure, must remain
separate findings. Verification is
`go test ./internal/ai -run 'TestFindingsStore'` plus
`go test ./internal/ai -run 'TestPatrolService_GetAllFindings|TestPatrolService_GetFindingsForResource|TestPatrolService_GetAllFindingsIncludingResolved'`.

Public Pulse Intelligence overview, safety, and onboarding copy now use the
same visible Patrol mode labels as the product: `Watch only`,
`Ask first`, `Safe auto-fix`, and `Autopilot`. The legacy `Only watch`,
`Ask before changes`, `Fix safe issues`, `Auto-fix safe issues`,
`Full control`, and `Policy autopilot` names may remain only as historical
context or compatibility implementation details, not as customer-facing Patrol
mode labels.

Assistant provider-readiness repair actions now use the canonical Pulse
Intelligence > Provider & Models route
`/settings/pulse-intelligence/provider`. The legacy `/settings/system-ai`
route remains a compatibility alias for old deep links, not a href emitted by
new Assistant provider-repair actions.

Primary platform route memory in `frontend-modern/src/AppLayout.tsx` may
preserve platform-local query state such as workload filters, but that memory
remains navigation chrome only. It must not fork AI availability state, Patrol
utility route state, prompt context, resource reads, commercial posture, or
cross-platform query parameters.

Rejected Patrol investigation-fix approvals are terminal governed-action
decisions in the AI runtime. `/api/ai/approvals/{id}/deny` must persist the
approval-store denial, record a rejected unified action-audit decision when the
approval carries a governed action plan, and move the owning finding to
`fix_rejected` so Assistant handoffs, Patrol summaries, and external agent
adapters see the declined fix as explicit loop state rather than a disappeared
pending approval.

Legacy Assistant provider-tool declarations for manifest-backed Patrol finding
lifecycle tools now consume the manifest-owned raw input schemas through
`agentcapabilities.ProviderInputSchemaFromRaw`. The native Assistant runtime
may keep first-party model-facing descriptions, but it must not re-declare
required arguments such as `resolution_note` or dismissal `note` when the
canonical Pulse Intelligence manifest and API contract mark those fields
optional.
Legacy native Assistant utility provider aliases for `fetch_url` and
`set_resource_url`, plus their provider JSON schemas, are owned by
`agentcapabilities.LegacyAssistantUtilityProviderTools`. The older native
Assistant service may continue to expose those compatibility aliases while the
execution migration proceeds, but it must consume the shared provider projection
and shared argument constants rather than carrying inline schema maps or local
tool-input string keys. The legacy `run_command` alias is retired: the Assistant
must not offer it, and a fabricated invocation must fail closed before any
command dispatch. Raw command execution remains available only through the
governed capability and action-lifecycle path.
Native Assistant registry tools that operate on Patrol finding lifecycle state
must also consume the same `agentcapabilities` argument vocabulary for
`finding_id`, `resolution_note`, `reason`, and `note` instead of repeating
local field-name strings in their provider schema or execution maps.

Anthropic runtime provider execution is API-key backed only. Legacy Anthropic
OAuth tokens may remain in encrypted settings solely for disconnect cleanup;
they do not make Anthropic configured, do not instantiate a provider, do not
refresh tokens, and cannot send model requests.

The done event carries `context_limit_tokens` (the active model's context
window from `providers.ContextWindowTokens`) alongside token usage, and the
drawer's last-turn summary renders input tokens as a percentage of that
window ("8,500 tokens (6% of context)") so operators can see compaction
coming; when the limit is absent the summary renders exactly as before.
The done event also carries `session_cost_usd`, the estimated cumulative
spend for the session summed from the operator cost ledger
(`cost.Store.SessionCostUSD`) across the session's recorded events: chat
turns, compaction, and title calls, all of which stamp `session_id` on
their `cost.UsageEvent`. The figure is omitted whenever any of the
session's events prices against an unknown model, and free local models
(Ollama) price known-at-zero, so a client's last-turn summary may append
"$0.12 session" only when the whole session is honestly priceable and
non-zero. The summary never invents a client-side pricing table; the
backend ledger is the only cost authority.

Post-tool model turns mark the status handoff: every agentic turn after the
first emits a `model_processing` workflow state ("Working on the response
with the gathered results.") before the provider call, so a provider that
reasons server-side without streaming events can never leave the previous
tool's status stale on screen; `model_thinking` still upgrades the status
when reasoning deltas actually arrive. Pinned by the interaction-quality
corpus scenario "post-tool model turn marks the status handoff".

Session titles are model-generated after the first exchange
(`internal/ai/chat/session_title.go`): the chat runtime upgrades the
auto-truncated first-prompt placeholder once the done event has reached the
client, never overwrites a user-set title (placeholder comparison plus a
pre-persist re-check), passes the model-boundary sanitizer, records usage as
`assistant_session_title`, and leaves the placeholder on failure or timeout.
The long-form contract lives with the session rename clause in Extension
Points.

Finding identity is the LLM-assigned key (resource+category+key hash → ID),
and key collisions are surfaced, not forked: when a same-ID re-detection's
title shares essentially no keywords with the existing finding's title
(`keywordOverlap` at or below `findingIdentityShiftMaxTitleOverlap`,
`internal/ai/findings.go`), the merge proceeds — the latest report still owns
the text, because key forking would split LLM rephrasings of one real issue
into duplicate findings — but `FindingsStore.Add` appends a
`content_replaced` lifecycle event preserving the previous and new titles in
metadata and logs the collision. This keeps the operator timeline honest
when a distinct issue reuses an existing finding's identity (including the
resolved case, where the reactivation otherwise reads as a regression of the
old issue: both `content_replaced` and `regressed` events appear).
Rephrasings and identical re-detections stay event-free per the
heartbeat-not-transition rule. The shared lifecycle presentation
(`frontend-modern/src/utils/aiFindingPresentation.ts`,
patrol-intelligence subsystem) labels the event "Re-detected with different
details" — see that contract's Current State entry for the UI-side rule.
`TestFindingsStore_KeyCollisionRecordsContentReplacedEvent` and its
companion tests (`internal/ai/findings_lifecycle_test.go`) pin the behavior.

The interaction-quality scenario corpus
(`internal/ai/chat/interaction_scenario_corpus_test.go`) is the canonical
regression home for chat-feel promises. Each scenario drives a
full `ExecuteStream` turn against a scripted provider and pins the
browser-facing event stream — ordered event types, forbidden event types,
answer-text teeth, and payload teeth — for the user-visible promises shipped
in the OpenCode-restraint arc: clean plain/greeting turns with no tool noise,
compact tool events with real tool names, the clean no-narrative fallback
sentence (no raw JSON or provider call ids), invisible pre-event provider
retries, and exactly one clear error event on terminal provider failure.
Interaction-quality fixes must land with a corpus scenario stating the
promise that would fail without the fix, so loop refactors (including
Patrol-phase work on the shared agentic loop) cannot silently regress the
chat feel.

The Assistant system prompt's missing-target policy is resolve-before-asking
(`buildSystemPromptWithToolGovernance`, `internal/ai/chat/service.go`): a
command or diagnostic request that names no target sends the model to
read-only query/topology tools first; a sole plausible match is used directly
for read-only diagnostics and named in the answer; the operator is asked only
when several plausible targets remain or the action changes state. This
supersedes the ask-first framing ("Missing target information is not a safe
default") that deflected every untargeted "run X" request back to the
operator even on single-host deployments. Placeholder targets
(`current_resource` outside an attached-resource turn) remain forbidden in
all modes, and write actions still require an explicit operator-confirmed
target. `TestBuildSystemPrompt_CurrentResourceRequiresResourceHandoff` pins
the boundary strings; the full Extension-Points entry sits beside the
model-owned tool-manifest rule.

Resolve-before-asking extends to tool-argument recovery: when a tool call
fails because an argument is missing or invalid, the prompt directs the model
to recover itself — enumerate the missing values with read-only tools or
retry with the arguments the error names — rather than converting the
failure into a user question. Internal identifiers (resource IDs, UUIDs,
metric keys) are never valid things to ask the operator for; the
`pulse_question` provider-tool description carries the same prohibition so
the structured clarification surface cannot be used as an identifier
elicitation channel (observed on small local models: a first-run
`pulse_summarize` failure became a "comma-separated resource IDs" question
to a brand-new user). Questions stay reserved for preferences, risky
choices, and genuine ambiguity between named options. The same prompt-pin
test and `TestNewPulseQuestionProviderToolBuildsSharedAssistantTool` hold
both strings.

The per-turn Assistant system prompt carries the current wall-clock time (the
Pulse server clock) so the Assistant answers "what time/date is it" directly
instead of deflecting ("I don't have access to a real-time clock") or demanding
a `target_host` just to run `date`. The timestamp is appended in
`AgenticLoop.getSystemPrompt` (`internal/ai/chat/agentic_prompt.go`) rather than
baked into `baseSystemPrompt`: the base prompt is frozen when the loop is
constructed at service start, so anything that must stay fresh per turn
(execution mode, current time) is appended on each turn. The current time
carries no PII and is therefore safe on cloud-routed turns regardless of the
operational-context sharing opt-in.

Prompt wording must not gate tool availability at all. The greeting/meta
text-only classifier (`assistantPromptLooksConversational`) is removed along
with the rest of the `assistantToolScopeForPrompt` keyword router: even "hi"
or "thanks" turns carry the full governed manifest, because tools offered is
not tools used — the model answers a greeting without calling anything, while
short prompts that look conversational to a keyword list are usually resource
lookups ("hows esphome", "check frigate", "grafana cpu") that need the
query/read/metrics tools to answer from Pulse data. Withholding tools by word
count or greeting keywords made the Assistant tell the user it had "no
infrastructure context or diagnostic tools" and ask them to run `docker ps`
themselves, for a resource Pulse already inventories. Tool selection stays
model-owned: offer the tools and let the model decide whether to use them,
rather than pre-deciding from prompt keywords/length that no tools are
needed. `TestService_ExecuteStream_ToolManifestIsModelOwned` is the
regression proof.

The `pulse_query` `get` action (`executeGetResource` in
`internal/ai/tools/tools_query.go`) must accept a canonical resource handle
(`<type>-<hash>`, e.g. `system-container-599a2e3…`) as `resource_id` without a
separate `resource_type` and infer the type from the handle, because that is how
a model naturally references a resource it read from context. Failing such a call
with "resource_type is required" burns visibly-failed tool calls in the
transcript before the model recovers with the numeric id. The type is recovered
from the trailing hex hash segment (no type word is all-hex); a bare numeric
VMID still requires an explicit `resource_type`.

For a resource resolved through the unified-resource provider, every
`pulse_query` `get`, `list`, and `search` response must keep `id` equal to the
canonical unified resource ID. Provider-native identifiers such as a Docker
container ID remain accepted lookup aliases and internal execution targets,
but they must not replace the canonical ID in model-visible resource results.
Otherwise a model can faithfully copy Pulse-provided evidence into a Patrol
finding and still mis-link that finding to a source record instead of the
canonical resource selected by the operator or scorer.

The shared `pulse_query` limit is authoritative for topology as well as list
and search. When a topology call supplies `limit` without a more precise
`max_*` bound, the executor applies that limit to each included nested
collection rather than silently reverting to its smaller default. Explicit
`max_*` values still take precedence. Every Docker host topology result exposes
both the total and returned container counts plus an explicit truncation flag,
so a model can distinguish complete topology evidence from a bounded preview
and continue with a larger bound or paginated list instead of inventing a root
cause from missing resources.

Docker container topology and resource context must also expose the canonical
resource's bounded `healthcheck_targets` hostname list when present. Patrol may
use a target that matches another exact container name to test a causal
dependency chain, but the target remains untrusted observational evidence and
never grants tool or action authority. The AI boundary must not receive the raw
health-check command, URL path, query, credentials, or environment expansion.

When the model runs tools but returns no final narrative, the deterministic
fallback summary (`buildAutomaticFallbackSummary` in
`internal/ai/chat/agentic_final.go`) must read as a clean operator message, not a
dump of internals. It names the tools it ran by their real tool name resolved
from the assistant tool calls (`pulse_` prefix stripped) — never the opaque
provider call id (`call_…`/`toolu_…`/`fc_…`), which must not leak into
chat-visible text — and it must not append raw tool output / result snippets.
The earlier form ("I completed N successful check(s) using call_27f0f389…, …
automatic summary. Latest successful result snippet: {…raw JSON…}") is forbidden:
provider call ids and raw tool JSON are not operator-facing answer content.


`tool_start` carries a running phase when execution is visible, so a client can
render a new start row as running immediately even if `tool_progress` arrives
later. This adapts OpenCode `packages/opencode/src/session/processor.ts`
(`ensureToolCall`, dev commit `3867fa2bad0e644166e360e2e99cfe426fe71105`),
which moves the visible tool part to `running` on `tool-call`.

Assistant provider retries are a first-class visible workflow state, not a
hidden server log. The referenced OpenCode source at fetched `dev` commit
`7ae856a9e97130f664f6f11fa5871a2795de9902` defines retry session status in
`packages/opencode/src/session/status.ts` (`SessionStatus.Info` /
`SessionStatus.set`), wires retry updates from
`packages/opencode/src/session/processor.ts` through
`SessionRetry.policy({ set })`, and renders retry attempt/backoff state from
`status().type === "retry"` in
`packages/opencode/src/cli/cmd/tui/component/prompt/index.tsx`. Pulse adapts
that contract at the owning stream boundary: `WorkflowStateData` carries
`attempt`, `max_attempts`, and `retry_after_ms`, and `AgenticLoop` emits
`provider_retry` before sleeping between transient pre-output provider
attempts, so a client can show attempt/backoff progress instead of an obsolete
provider-wait message.

Primary nav moved to governed platform/runtime destinations on 2026-05-16 and
was clarified on 2026-05-25 and 2026-06-25 through `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx`: the top of the app may expose canonical
platform pages (Proxmox, Kubernetes, TrueNAS, vSphere) plus the
Docker / Podman container-runtime lens (shown as Docker in the shell), aggregate
platform-owned Workloads / Storage / Recovery sub-surfaces, Alerts, Patrol, and
Settings. `Patrol` is the visible destination for Patrol-owned operator work; it
must route through the canonical `/patrol` surface and must not create a second
Assistant or Intelligence route namespace.
Provider/runtime destinations must pass the shared support-and-resource-
evidence gate before they appear in navigation, command palette entries,
keyboard shortcuts, or landing fallbacks. Aggregate workspace tabs are retired
top-level routes, not compatibility placeholders. Admitted-only or absent
platform/runtime surfaces must stay hidden from the AI-adjacent shell and must
not be kept as disabled placeholders. The legacy `/infrastructure` route shell
was retired alongside its page wrapper, and `/workloads`, `/storage`,
`/recovery`, `/ceph`, `/ai`, and `/operations/*` remain unregistered
top-level routes. Primary platform tab `settingsRoute` handoffs must also point
to the canonical `/settings/infrastructure` workspace rather than retired
settings aliases such as `/settings/workloads/docker` or nested
`/settings/infrastructure/platforms/*` paths. Patrol surfaces and their deep links must use canonical platform or runtime
routes (`/proxmox/overview`, `/proxmox/storage`, `/kubernetes/workloads`, etc.)
rather than reviving retired Infrastructure or aggregate workspace paths;
adding a platform tab through the same shell files must not fork AI availability
or Patrol shell state or smuggle in AI-owned platform reads.
Post-auth `/` and `/login` resolution follows the frontend-primitives-owned
provider-first platform landing contract, so the authenticated shell never
overrides Machines-surface eligibility or revives legacy Infrastructure as the
default estate surface. The user-facing Machines label is an app-shell
presentation label for the existing `standalone` route/id and must not create a
separate AI handoff or prompt namespace.

Alert-triggered scoped patrols now investigate the specific breach rather than
running a broad health check. The alert bridge (`internal/ai/unified/bridge.go`,
`internal/ai/unified/setup.go`) carries the firing alert's real payload — type,
level, value, threshold, resource identifier, and message — into
`PatrolScope.AlertContext`, and `internal/ai/patrol_ai.go` /
`internal/ai/patrol_triggers.go` frame the `alert_fired` run around that breach
instead of suppressing threshold context. Whether an alert triggers a patrol at
all is the operator's per-rule policy: `AIConfig.AlertTriggersInvestigation`
(`internal/config/ai.go`) enforces the master enable, a minimum-severity floor
(`patrol_alert_trigger_min_severity`, default critical-only), and an optional
alert-type allowlist (`patrol_alert_trigger_types`, empty = all types). The
router-side bridge wiring consults that policy and skips queuing the scoped
patrol when the alert does not qualify; an unknown alert level is treated as
critical so it is never silently dropped.

The route-backed Proxmox platform tab is app-shell navigation only. Adding the
tab through `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` must not fork Assistant or Patrol shell
state, synthesize platform-specific handoffs, or add AI-owned platform reads.
Future Proxmox-native Assistant or Patrol read/write claims must extend the
shared AI tool, handoff, and platform-support contracts instead of hiding
behind route registration or tab chrome.

Patrol deterministic signal extraction (`internal/ai/patrol_signals.go`)
does not mirror the Alerts surface. The `pulse_alerts` tool output is
intentionally absent from the signal switch in `DetectSignals` — alerts
already have their own canonical surface, lifecycle, and operator
acknowledgement model, and the `SignalActiveAlert` mirror path has
been removed. Mirroring previously double-counted (every alert was
also a Patrol "Active alert detected" finding), dragged the health
score down for issues the operator already knew about, and produced
bogus `auto_resolved` → re-detected → regressed cycles when the LLM
explicitly resolved the mirrored finding while the underlying alert
kept firing. Patrol's job, per its own system prompt, is to surface
issues alerts cannot — trends, capacity risks, misconfigurations,
reliability gaps, cross-resource correlations. The Alerts page is
the canonical surface for currently-firing alerts. To retire the
alert-mirror findings already persisted from an earlier build,
`FindingsStore.SetPersistence` runs a one-shot pass on load that
auto-resolves any active finding matching the legacy signature
(title `"Active alert detected"`, source `ai-analysis`, category
`general`) with a clear retirement reason; the pass is idempotent
and self-cleaning. The same load pass also resets the
`RegressionCount` and clears `LastRegressionAt` on any active
finding whose lifecycle contains an `auto_resolved` event **when
the finding's category is not eligible for stale-auto-resolve**
(i.e. anything other than `performance` or `capacity`). For
event/persistent categories there is no legitimate absence-driven
resolution path: an `auto_resolved` event there came either from
one of the now-gated absence paths (legacy reason strings still
recognized) or from the LLM `patrol_resolve_finding` tool
(empty-message via `Resolve(_, true)`) which has repeatedly
misjudged findings backed by still-active conditions and reverted
through a regression on the next run. The reset appends a
`regression_counter_reset` lifecycle event so the migration is
idempotent; genuine recurrences from then on accrue cleanly.
Performance/capacity findings retain their counter because the
metric-cleared resolution model is sound there.

The overall health score (`calculateOverallHealth` in
`internal/ai/intelligence.go`) tiers the "recent Patrol errors" coverage
factor by the ratio of errored runs to relevant runs in the scoring
window. Above 50% of recent runs erroring is a `-30` impact and is
described as "Most recent Patrol runs encountered errors"; above 25%
is `-20`; otherwise the original `-10` light-tier description applies.
This keeps the grade from reading A when most recent runs errored, which
previously happened whenever one successful manual run sat among many failed
startup runs. A light error share still deducts only 10 points, and three
trailing successful full runs drop the penalty entirely. The Patrol page no longer renders the score; it reads the overall
health only to qualify the findings empty state.

Absence-based auto-resolve paths in `internal/ai/patrol_ai.go` are
all gated on the category whitelist exposed by
`CategorySupportsStaleAutoResolve` in `internal/ai/findings.go`. Two
paths use the gate: `reconcileStaleFindings` (auto-resolves findings
the LLM didn't re-mention in a successful run) and the
resource-absent branch inside the seed-prompt builder (auto-resolves
findings whose resource isn't in the current global inventory
snapshot). Only `performance` and `capacity` findings — continuous
current-state metric thresholds where the most recent successful
scan's observation is authoritative — may be auto-resolved from
absence. `reliability`, `backup`, `security`, and `general` findings
represent discrete events or persistent states; the LLM not
re-mentioning a failed backup task or a single inventory snapshot
missing a container (transient agent reconnect, container churn,
refresh gap) is not evidence that the underlying issue has cleared.
Those categories must stay active until explicitly resolved either
by the LLM calling `patrol_resolve_finding` with evidence or by
operator action through the governed findings store. Lifecycle events recorded by `findings.go` must
not introduce duplicate generic transition rows: the canonical
`syncLoopStateLocked` records `loop_transition_violation` only when a
transition is rejected, and otherwise leaves the semantic event
(`auto_resolved`, `regressed`, `dismissed`, `acknowledged`, `snoozed`,
`reminded`, `suppression_lifted`, etc.) as the single audit row for the
transition. Re-detections of an already-active finding update
`TimesRaised` and `LastSeenAt` only — they must not append a `detected`
lifecycle event, because a heartbeat is not a transition.

Patrol's alert auto-resolve review (`reviewAndResolveAlertsState` in
`internal/ai/patrol_run.go`) is model-owned in the resolve direction
and cost-gated in the keep direction. Local state never resolves an
alert: only a parsed model RESOLVE verdict (or operator action) clears
one. But an aged active alert whose trigger condition demonstrably
still holds in the current runtime snapshot — a metric alert whose
looked-up current value is still at/above its positive threshold, or
an offline-family alert whose node/agent/VM/system-container resource
is present and still not online (`alertStillFiringState`) — must be
skipped without a billed model call, because a correct review could
only answer KEEP. Every uncertain case (unknown alert type, missing
resource, unmapped metric, non-positive threshold, resource types the
online check does not model) still goes to the model. The remaining
candidates are reviewed in batched `QuickAnalysis` calls of at most
`patrolAlertReviewBatchSize` alerts with numbered per-alert verdict
lines; verdict parsing is fail-safe (unparseable or out-of-range
verdicts default to KEEP, a bare `RESOLVE:` response is honored only
for single-alert batches), and the recorded usage events carry use
case `patrol` with `TargetType` `alert_autoresolve` so cost telemetry
can decompose alert-review spend from the main Patrol analysis pass.
This replaced the prior one-model-call-per-alert-per-run loop that
re-asked the same question about standing alerts every interval and
dominated Patrol's billed call volume in production telemetry.

The findings store now consults a `ResourceOperatorStateProvider`
during the new-finding path. The interface lives in `internal/ai` to
avoid an import cycle with `internal/unifiedresources`; the API layer
wires the adapter at startup, projecting
`unified.ResourceOperatorState` into the
`ResourceOperatorStateProjection` shape the findings runtime
consumes. The projection carries every operator-set signal in one
call (active maintenance window, `IntentionallyOffline`,
`NeverAutoRemediate`, and `Criticality`) so adding new signals later does
not multiply round-trips per finding. `FindingsStore.Add` stamps
`Finding.ResourceCriticality` from that projection on new and re-detected
findings. The field persists as `resource_criticality` and may affect
same-severity Patrol attention ordering only; it must not mutate
`Finding.Severity` or bypass severity escalation/resolution rules.

The approval store exposes `SetOnApprovalCreated(cb)` so the API
layer can install a fire-and-forget callback that runs after every
successful `CreateApproval` (the approval is already persisted at
that point). The callback is invoked on its own goroutine against
a snapshot of the request, keeping the approval hot path off any
consumer's slowness and avoiding any chance of the consumer
reentering the store under the held write lock. This is the seam
the agent SSE stream uses to publish `approval.pending` events
without coupling the approval store to the api package.
`ApprovalRequest.CanonicalResourceID()` is the helper the bridge
uses to stamp `resourceId` on those events, derived from
`(TargetType, TargetID, TargetName)` via the same rule the store
uses internally for preflight context normalization — Plan-less
approvals (the common shape on the approval hot path) still carry
a canonical resource id agents can match against the rest of
Pulse.

`PulseToolExecutor` still exposes `SetOnActionCompleted(cb)` as the
parallel seam for action-audit terminal states, and the API layer installs
the agent SSE bridge on it, but no production path dispatches the callback:
its only dispatcher, `publishActionCompleted` in
`internal/ai/tools/action_audit.go`, was reachable solely from the retired
`executeCommandWithAudit` and `executeNativeActionWithAudit` lanes and was
removed with them. The `action.completed` agent event is therefore published
only by API-owned execution (`ResourceHandlers` through
`PublishActionCompletedRecord`), not by the Assistant executor.

The investigation runtime now hands the orchestrator a Finding
pre-enriched with operator-set state and operational memory.
`MaybeInvestigateFinding` (in `internal/ai/patrol_findings.go`)
calls `f.ToCoreFinding()` then attaches a
`FindingOperatorContext` from the in-memory operator-state
projection (intentionally offline, never auto-remediate, active
maintenance window) and a `FindingOperationalMemory` projection
(regression count, previous resolved fix summary, times raised)
populated from fields the internal `Finding` already carries. The
orchestrator (in pulse-enterprise) consumes these fields when reasoning
about the next move — it does not need a separate read to get
the situated picture, and it can avoid proposing fixes the
operator has locked the resource against.

An effective autonomy transition out of `monitor` also reconciles findings
that were detected while Patrol was in Watch Only. The API publishes the
persisted autonomy mutation first, then
`PatrolService.ReconcileActionableFindingBacklog` selects active
warning/critical findings through the same `ShouldInvestigate` admission used
by scheduled and alert-triggered work. The activation pass is bounded to three
concurrent starts minus the orchestrator's current running count, orders
critical findings before warnings and older findings before newer ones, and
leaves remaining work to ordinary Patrol runs. It must not run for a no-op
save, a transition into Watch Only, an unavailable orchestrator, or an
effective mode still clamped to `monitor`; activation must never create an
unbounded model-work burst or bypass suppression, snooze, cooldown, attempt,
terminal-outcome, operator-state, and concurrency gates.
The enterprise orchestrator owns authoritative concurrency admission: it
atomically checks and reserves a slot before beginning an investigation, while
`CanStartInvestigation` remains advisory. Its running-count projection includes
both in-memory reservations and persisted running sessions, so simultaneous
activation, scheduled, alert-triggered, and manual callers cannot oversubscribe
`MaxConcurrent` through a check-then-increment race.

`ResourceOperatorStateProjection` carries `NeverAutoRemediate`
and `Criticality` alongside `IntentionallyOffline` and `MaintenanceWindow` so the
investigation read path and the suppression read path share a
single projection. The findings store exposes the projection via
`OperatorStateProjectionFor`; the suppression hot path keeps its
existing internal access. Both paths see the same operator-state
facts so investigation reasoning, suppression behavior, and Patrol
priority stamping cannot drift against each other.

A cross-slice consequence worth pinning: operator-state-suppressed
findings (auto-dismissed with `DismissedReason="expected_behavior"`
and `operator_state_cause` metadata) are also ineligible for
autonomous investigation, because `Finding.ShouldInvestigate`
already gates on `f.DismissedReason != ""`. Investigation budget is
not spent on findings the operator has told Pulse to stay quiet
about, regardless of autonomy level. This is delivered by the
existing chain (Add → auto-dismiss → ShouldInvestigate-false)
rather than a separate runtime check; the contract test in
`findings_test.go` pins the relationship so a future refactor of
either branch cannot silently waste investigation budget on
operator-suppressed findings.

The operator-state suppression is also reversible. When a finding
auto-dismissed under `operator_state_cause` re-detects after the
underlying suppression has lifted (maintenance window passed,
`IntentionallyOffline` cleared), `FindingsStore.Add` wakes it with
a `suppression_lifted` lifecycle event. The wake gates on the most
recent lifecycle dismiss event carrying `operator_state_cause`
metadata via `findOperatorStateDismissCause`, so a manual operator
dismissal that supersedes an earlier auto-dismiss is not
falsely re-awakened — the helper stops at the first `dismissed`
event when scanning from newest backwards, treating that as the
authoritative state.

When the projection carries an active maintenance window, the
new-finding path auto-dismisses with reason `expected_behavior`,
attributes the suppression on the lifecycle timeline
(`operator_state_cause: maintenance_window`, with
`maintenance_end_at` metadata), and persists the finding for audit
history. The active boundary may come from a one-shot or recurring schedule;
both are evaluated by the canonical operator-state model, and the projection
carries the active occurrence's own start and end
(`ActiveMaintenanceOccurrenceAt`), never the one-shot fields, which a recurring
schedule leaves nil. Patrol inherits maintenance exactly as alert intent does:
`unifiedresources.ActiveMaintenanceCoverageAt` resolves the resource's own
window and every ancestor window scoped `resource_and_descendants` on the live
canonical parent chain, and the latest-ending active window wins. A window
stored on an ancestor reaches the projection with `Inherited` and its
`SourceID`; the dismissal note says the window is on a parent resource and the
lifecycle event carries `maintenance_inherited` and `maintenance_source_id`.
Only maintenance is inherited. Monitoring mode, lifecycle, criticality and
action-remediation locks stay exact-resource state, so descendant scope never
weakens a lock or invents inherited action authority.

Operator state outranks per-action approval on the Pulse Actions path:
`validateExecutionPolicy` in `internal/actionlifecycle/service.go` reads the
same `resource_operator_state` table at the dispatch decision point, refuses
with `unifiedresources.ErrResourceRemediationLocked` when the operator has set
`NeverAutoRemediate=true` on the target resource, and fails closed when the
audit store is unavailable. The Assistant tool layer no longer consults the
table itself: the tool-layer broker that did so (`checkRemediationLockForDispatch`
inside the retired execution lanes) and its `ErrRemediationLockStateUnknown`
were removed with the unreachable handlers. The `IntentionallyOffline` branch is the indefinite
counterpart — same auto-dismiss but with
`operator_state_cause: intentionally_offline` and no
`maintenance_end_at` field because the suppression has no scheduled
end — and, like `expected_offline`, it silences only availability findings
(see "Patrol consumes canonical monitoring and lifecycle policy"). Maintenance windows take priority over intentionally-offline
when both are active, because the time-bounded suppression is the
more honest one to surface to the operator. Deployments without a
provider keep the original new-finding behavior — suppression is
opt-in.

This subsystem now makes Pulse Assistant and Patrol backend runtime ownership
explicit inside the current architecture lane instead of leaving those
surfaces implicit inside broad architecture or generic API ownership. A later
lane split can still promote this area into its own product lane once the
governed floor is ready.
That backend/runtime ownership does not require the Patrol product surface to
inherit `AI` as its canonical browser route: the customer-facing shell may use
`/patrol` while shared AI transport, provider settings, and payload contracts
remain the governed technical boundary behind it.
Operator-configured provider base URLs remain part of that backend transport
boundary. Ollama keeps supporting remote or local instances, but
`internal/ai/providers/ollama.go` must normalize the configured base URL and
route requests through the shared restricted outbound HTTP transport so
metadata, link-local, and redirect-escape paths do not bypass the runtime's
egress guardrails.
That same operator-facing vocabulary rule applies to the runtime usage surface:
`frontend-modern/src/components/AI/AICostDashboard.tsx` must present provider
usage and spend backing Pulse Assistant and Patrol rather than generic `AI`
history, and `frontend-modern/src/utils/aiCostPresentation.ts` must own the
title, empty/loading states, budget note, and reset/export history messaging so
settings shells and runtime widgets do not fork their own usage wording.
Usage grouping rows must present product concepts such as Assistant sessions,
Patrol runs, Discovery runs, or concrete resource labels. Raw target storage
keys, opaque session identifiers, UUIDs, and values such as
`assistant_session_title:<id>` are accounting implementation detail and must
stay out of the operator-facing table.
That same runtime-facing table ownership applies to the cost dashboard shell:
`frontend-modern/src/components/AI/AICostDashboard.tsx` owns provider usage,
budget, and history semantics, but its tabular presentation must compose the
shared `frontend-modern/src/components/shared/Table.tsx` primitive instead of
carrying AI-local scroll wrappers or raw table shell markup. Any future AI
usage table styling change must extend the shared primitive or its governed
wrapper affordances first, then consume that contract from the dashboard.
At phone widths, the provider usage table must retain provider, model, and
estimated spend as its primary decision fields while token-detail columns stay
hidden through the shared responsive table boundary. A loaded range with no
provider/model rows must render an explanatory row inside the table rather
than leaving a header above an unexplained empty shell.

`internal/ai/` is the live backend AI engine. It owns chat execution, Patrol
orchestration, findings generation, investigation support, provider selection,
remediation flow, and cost persistence.
That Patrol runtime ownership includes seed-context admission control.
`internal/ai/patrol_ai.go` must build Patrol and triage prompts from
canonical seed sections, size them against the runtime budget model, and when
a provider reports a smaller real context window than the static model map,
reassemble the same canonical sections under tighter provider-derived budgets
instead of hard-failing or truncating ad hoc prompt strings.
That same backend runtime ownership also includes bounded Patrol and
investigation read models. `internal/ai/patrol_history_persistence.go` and
`internal/ai/proxmox/events.go` must cap persisted-history loads and
caller-requested read limits at the canonical runtime maxima instead of
allocating directly from raw on-disk counts or transport-supplied limits.
Callers may request fewer records, but AI runtime storage and correlation
surfaces remain responsible for enforcing the governed ceilings that protect
memory and keep Patrol/history behavior stable under malformed or oversized
inputs.
That same backend runtime ownership includes `internal/config/ai.go`, because
provider auth, base URLs, provider-scoped model defaults, and other persisted
runtime AI selection rules must stay canonical in the shared AI config model
instead of drifting into handler-local fallbacks or frontend-only assumptions.
That same provider-model ownership now explicitly forbids Pulse from baking
vendor model IDs into BYOK default selection. `internal/config/ai.go` may
persist an explicit operator-chosen model, but when a BYOK provider is
configured without a concrete model selection,
`internal/ai/model_resolution.go` must resolve the effective model from the
provider's live catalog at runtime using the shared provider metadata policy
instead of reviving static vendor constants in config defaults, service
fallbacks, or frontend setup flows.
Explicit saved model routes fail closed when their provider is not configured
or cannot be initialized. Pulse may not replace an operator-selected route such
as `deepseek:*`, `openrouter:*`, or `openai:*` with another configured provider
default during settings load, Assistant chat startup, or service initialization.
Provider-owned defaults are allowed only when no explicit route exists, or when
the same configured provider needs its governed chat-suitable/default model
because live catalog lookup failed or the explicit model is unsuitable for chat.
That same provider-model ownership also governs live-catalog failure fallback:
when runtime client construction fails, test credentials intentionally block a
provider catalog, or a provider returns no usable models, the effective BYOK
selection may fall back only to the provider-owned default declared in
`internal/config/ai.go`. Runtime startup, connection-test, and load-config
paths may not return an empty effective model or borrow another provider's
selection just because live model discovery was unavailable. DeepSeek's
provider-owned fallback must track the current V4 API contract and use
`deepseek-v4-flash` rather than retired compatibility aliases such as
`deepseek-chat` or `deepseek-reasoner`; AI runtime context-window and cost
budgeting must likewise know the V4 Flash/Pro 1M context and distinct pricing
classes before Patrol treats those models as ready.
The shared `/api/ai/models` catalog must preserve that same direct-provider
fallback posture for configured DeepSeek paths: when DeepSeek live catalog
listing fails or omits current V4 entries, the backend catalog must still
surface direct `deepseek-v4-flash` and `deepseek-v4-pro` options plus clearly
labelled legacy aliases so saved Patrol or Assistant selections do not render
as unrelated default models in the browser.
Retired quickstart ownership is now an inert compatibility boundary, not a
self-hosted GA runtime path. The old quickstart provider, bootstrap manager,
and local token-cache persistence API are removed from the Pulse runtime;
ordinary self-hosted Assistant, Patrol, and AI Settings flows must use the
operator's configured provider or local model and must not bootstrap managed
credits, hosted-model tokens, or quickstart-backed provider clients from the
frontend.
Public-facing copy that reflects old quickstart fields must normalize back to
provider or local-model setup. It must not promise managed credits, account
activation support, trial CTAs, anonymous Community bootstrap, or full hosted
chat access in ordinary self-hosted v6 GA flows.
That same runtime-backed contract now governs AI settings enablement too:
unconfigured installs open provider setup, while stale managed-credit or
activation-required states are treated as compatibility metadata rather than a
direct-enable path.
That same AI/runtime boundary now also owns the server-side AI availability
fact used by the app shell. `internal/api/ai_handlers.go`,
`internal/api/security_status_capabilities.go`, and
`internal/api/router_routes_auth_security.go` must expose one canonical
`/api/security/status.sessionCapabilities.assistantEnabled` signal for the
shell's AI entry points, so unrelated shells do not probe `/api/settings/ai`
or `/api/ai/sessions` during ordinary route bootstrap just to decide whether
Patrol entry points render.
That same frontend runtime boundary now also owns the shared AI read model for
AI-owned surfaces. `frontend-modern/src/stores/aiRuntimeState.ts` is the
canonical frontend owner for shared `/api/settings/ai` and `/api/ai/models`
reads used by Patrol and AI usage surfaces, while
`frontend-modern/src/components/Settings/useAISettingsState.ts` remains the
write-side settings owner. AI-owned surfaces must not fork their own mount-time
settings/model fetch loops once this store exists.
The AI availability fact is a separate shared boundary:
`frontend-modern/src/stores/aiAvailability.ts` holds the session
`assistantEnabled` capability for the shell, while
`frontend-modern/src/stores/aiRuntimeState.ts` owns the shared backend-backed
settings and model catalog reads. Consumers must not move that fact into
page-local signals or teach `aiAvailability.ts` to bootstrap its own
`/api/settings/ai` or `/api/ai/models` reads.
Non-AI shell notices may coexist in `frontend-modern/src/AppLayout.tsx`, but
they must remain presentation-only. Prerelease banners, billing callouts, or
other header-adjacent notices must not gate on AI runtime fetches or move AI
availability logic out of `frontend-modern/src/stores/aiAvailability.ts` and
`frontend-modern/src/useAppRuntimeState.ts` just because they share the same
authenticated shell. The remaining prerelease-shell treatment is the compact
`Preview` badge on rc-channel builds; `frontend-modern/src/AppLayout.tsx` must
not revive a standalone release-candidate banner, release-notes CTA, or
feedback CTA that starts participating in AI availability state or modal
ownership.
The retired monitored-system capacity banner follows the same shell rule:
`frontend-modern/src/App.tsx` must not reintroduce app-shell commercial
volume warnings just because settings or support surfaces still expose
monitored-system grouping data. AI availability and shell notices stay
independent from retired infrastructure-volume commerce.
The global update progress watcher in `frontend-modern/src/App.tsx` is
likewise server-updater shell chrome, not an AI surface: its in-progress
stage vocabulary mirrors the backend updater pipeline (including the
`restoring` stage emitted by update rollback), and AI availability and AI
runtime surfaces must not key off those update stages.
On authenticated installations that watcher also mounts only when the served
`settingsCapabilities.systemSettingsRead` value mirrors the update-status
route's `RequireAdmin` plus `settings:read` gate. Auth-free installations keep
the watcher because those routes remain reachable there. Neither branch may
become an AI availability signal, provider-readiness signal, or AI bootstrap
probe.
That same shared shell rule applies when presentation policy suppresses hosted
organization chrome: `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` may hide org switchers or demo-only org
labels, but they must not couple AI availability or Patrol entry visibility
to that organization presentation state.
That same shell boundary also owns demo-only support-surface suppression:
Pulse no longer exposes Operations as a top-level route. Demo-only support
surfaces now hide inside the shared Settings navigation instead, and AI
availability must stay independent of that settings-nav presentation choice.
Authenticated `/login` recovery belongs to that same route shell boundary:
once login succeeds, `frontend-modern/src/App.tsx` must resolve `/login`
through the canonical provider-first post-auth landing route instead of leaving
the authenticated shell stranded on a route that only exists
for logged-out presentation.
App-shell route preloading may include the Patrol route module, but it must
remain module-only. It must not prefetch AI settings, model state, findings, or
chat sessions.
`docs/release-control/v6/internal/subsystems/registry.json` must therefore keep
`frontend-modern/src/stores/aiRuntimeState.ts` on the explicit AI runtime proof
route, and keep `frontend-modern/src/stores/aiAvailability.ts` on the shared
AI-runtime/frontend-primitives proof boundary instead of leaving the AI
availability fact unowned.
That same settings/runtime boundary now also governs BYOK first-run setup:
`frontend-modern/src/components/Settings/useAISettingsState.ts` may send only
provider credentials or base URLs when the operator connects a provider, and
`internal/api/ai_handlers.go` plus `internal/ai/service.go` must persist the
resolved provider model returned by the canonical runtime selection path. The
setup surface must not reintroduce vendor-default model IDs in modal payloads
just to make the backend accept the request.
Interactive Assistant chat must not put a Pulse-authored intent router, scout
model, or explore pre-pass in front of the operator's selected model. The
runtime may assemble governed context and expose the approved tool list, but
the selected model owns the decision to answer directly, ask a question, read
context, or request an action. Pulse must not use prompt heuristics to force
`tool_choice=any`, force a named tool, retry because an expected tool was not
used, or hide tools from the model based on keyword detection. Pulse
enforcement starts after that model choice: approval mode, strict
resource resolution, and tool policy remain the safety boundary.
Session continuity context follows the same boundary: Pulse may provide
neutral recent-resource facts and explicit resource addressing facts, but it
must not use prompt-keyword or pronoun heuristics to rewrite a user message as
targeted, inject log-routing instructions, or tell the model which context is
the answer. Pre-model context prefetch may only use structured resource
mentions explicitly selected by the operator; it must not scan plain chat text
for resource names, infer unresolved `@name` references, or inject lookup
results before the selected model chooses whether to use tools.
Legacy remediation memory follows the same boundary. Pulse may provide
resource-scoped prior action history as neutral context, but it must not
keyword-match the current problem against old fixes, label those fixes as
successful matches for the current issue, or use remediation memory to
recommend a command before the selected model has reasoned over current
evidence.

The same runtime ownership now includes the customer-facing AI usage and cost
surface. `frontend-modern/src/components/AI/AICostDashboard.tsx` is the
canonical AI usage dashboard shell, while
`frontend-modern/src/utils/aiCostPresentation.ts` owns its shared loading,
empty-state, and range-button presentation contract. Future cost-surface work
must extend those owners instead of reintroducing inline AI usage copy or
dashboard-local segmented-button styling.
The same runtime boundary also owns the shared AI semantic presentation
helpers used across settings, Patrol, and usage surfaces.
`frontend-modern/src/utils/aiProviderPresentation.ts` and
`frontend-modern/src/utils/aiProviderHealthPresentation.ts` are the canonical
owners for provider naming and provider health labels. Settings and Patrol
surfaces must consume those helpers instead of keeping local AI wording or
model/provider inference branches.
The Assistant chat stream must not carry Pulse-authored explore pre-pass
cards or internal workflow-state cards as assistant output. The user-facing
stream is model text, model thinking where supported, model-selected tool
calls, governed approval requests, and model questions; internal runtime
telemetry stays out of the transcript.
Cold-start Assistant chat session creation is also stream-owned. Ordinary first
messages may call `/api/ai/chat` without a `session_id`; `chat.Service.ExecuteStream`
must create or resolve the durable session before provider execution and emit a
first-class `session` SSE event carrying `SessionData{ID: ...}`. Clients bind
their active session from that stream event, with `done` and `question`
session identifiers retained only as compatible terminal/interactive payloads,
rather than issuing a separate `/api/ai/sessions` preflight before the first
user message. Explicit session-management actions may still create a session
through the session endpoint.

The AI transport files are shared with `api-contracts`, not delegated away to
it. `frontend-modern/src/api/ai.ts`,
`frontend-modern/src/api/patrol.ts`,
`internal/api/ai_handler.go`,
`internal/api/ai_handlers.go`, and
`internal/api/ai_hosted_runtime.go`, and
`internal/api/ai_intelligence_handlers.go` are runtime control surfaces for
the AI product while also remaining canonical payload contract boundaries.
That same AI transport boundary now also defines the narrow Pulse Mobile
runtime compatibility rule: mobile relay credentials are minted with the
dedicated backend-owned `relay:mobile:access` scope, and only the explicit
route inventory in `internal/api/relay_mobile_capability.go` may accept that
scope as a compatibility alias alongside legacy `ai:chat` or `ai:execute`
mobile tokens. Broader AI runtime surfaces must stay on their canonical AI
scopes instead of treating the mobile relay capability as a general-purpose
AI permission, and any new mobile-compatible AI route must land by extending
that governed backend inventory and proof set in the same slice.
That same shared AI transport boundary now also owns hosted AI bootstrap
retirement. When Pulse Cloud runs in hosted mode and no explicit `ai.enc`
exists yet, `internal/api/ai_hosted_runtime.go`, `internal/api/ai_handler.go`,
and `internal/api/ai_handlers.go` must return the same unconfigured
BYOK/local-provider default as self-hosted settings instead of deriving a
quickstart-backed managed-model config from hosted billing state. Any
explicitly written AI config remains authoritative, and hosted billing state
must not be converted into quickstart credits or a managed-model runtime.
That same hosted and self-hosted settings boundary must also retire legacy
hosted quickstart model aliases on read and write. Persisted values such as
`quickstart:minimax-2.5m` are historical implementation detail, not governed
runtime truth, so `internal/config/ai.go`,
`internal/config/persistence.go`, and `internal/api/ai_handlers.go` must clear
them before the runtime, API payloads, or structured logs consume those fields.
That same runtime boundary also owns approval-store lifecycle in
`internal/api/ai_handler.go`. Settings-driven enablement and restart must be
able to cold-start the direct AI runtime, initialize approval persistence, and
leave `/api/ai/approvals` ready for mobile and remediation flows even when AI
was disabled at process boot. The approval cleanup loop must follow owned AI
runtime lifetime rather than an HTTP request context, and approval persistence
may fail closed only when AI is actually disabled instead of because runtime
enablement happened after startup.
Pending approval reads from that store must be deterministic across web, mobile
relay, and API consumers: live pending approvals are ordered by soonest expiry,
then highest operational risk, then oldest request time, with approval ID as
the final tie-break so map iteration cannot decide which governed action looks
most urgent.
That same approval boundary also owns approved command execution. When
`internal/api/ai_handlers.go` or `internal/ai/service.go` consume a governed
approval record, the
runtime must carry that approval identifier into the final
`agentexec.ExecuteCommandPayload` so the host agent can re-check the shared
command policy locally and fail closed on blocked or still-unapproved commands
instead of treating control-plane approval as an implicit bypass.
AI tool calls, Patrol fixes and Assistant remediation must not mark
command payloads as `Trusted` to bypass the governed approval path.
The same action-audit boundary now also requires persisted action records to
carry a normalized plan and preflight: action id, request id, capability,
approval policy, dry-run availability, safety checks, verification steps, and
timestamps are normalized before persistence by the unified-resource store, so
runtime callers cannot publish an execution audit that skipped the canonical
planning contract.
Patrol investigation-fix approvals must use that same action-audit boundary:
when the orchestrator queues a fix approval, `internal/api/ai_handlers.go` must
attach a governed action plan, seed the shared action-audit store as planned
and pending with `pulse_patrol` as the requester/actor, and leave later
execution or approval decisions to the governed action/approval paths instead
of creating Patrol-only execution context or collapsing Patrol proposals into
generic Assistant-origin actions. The approval record itself must also persist
and expose that requester identity so `/api/ai/approvals` and Assistant
handoffs preserve Patrol provenance before later action-audit hydration refreshes
the current action state. Backend chat refresh of a Patrol finding handoff must
hydrate the same requester identity directly from the live approval record, so
Assistant does not depend on browser-authored metadata to distinguish
Patrol-origin proposals from generic Assistant actions. Rejected Patrol
investigation-fix approvals must also enter that shared decision lifecycle:
`/api/ai/approvals/{id}/deny` records a rejected unified action-audit decision
when the approval has a governed action plan and moves the owning Patrol
finding to `fix_rejected`, so a declined fix remains a visible governed loop
outcome rather than disappearing when the pending approval leaves the queue.
The typed action-proposal seam in `pkg/aicontracts/action_broker.go` is the
successor to that command-shaped approval flow and the ONLY sanctioned route
from an enterprise Patrol investigation to a Pulse infrastructure mutation:
`OrchestratorActionBroker` exposes exactly `Capabilities` (read-only catalog
of a resource's advertised capabilities and parameter schemas, including
sensitivity and core-owned auto-authorization eligibility) and `Submit`
(typed proposal). The contract
deliberately omits org ID, requestedBy, autonomy, risk, approval-policy,
destructive, command, and target-host fields, and has no decide or execute
methods, so enterprise code can neither claim authorization nor dispatch;
authorization always derives inside core from the tenant Patrol mode,
capability eligibility, persisted per-resource allowlist/window, remediation
lock, and the canonical lifecycle. Enterprise receives the resulting pending
or terminal disposition, including honest verification status, but never an
authorization primitive. `OrchestratorDeps.ActionBroker` carries the seam and is
Policy-authorized submission enters `actionlifecycle.ExecuteUnderPolicy`; the
broker may evaluate eligibility to decide whether to attempt automatic
admission, but it cannot persist approval or call an executor itself. The
lifecycle re-evaluates the full policy under the shared admission coordinator
and atomically persists the typed authorization lease, policy approval, and
executing transition. Revocation before that transition produces zero dispatch
and no policy approval. Human approvals remain semantically separate.
`OrchestratorDeps.ActionBroker` carries the seam and is
now REQUIRED: the enterprise factory disables the orchestrator when it is
absent rather than falling back. The command-execution side doors are
gone. `OrchestratorChatService` exposes only investigation-specific
execution and listing (`ExecuteInvestigationStream` returning a
structured `OrchestratorInvestigationResult`, and
`ListInvestigationTools`); it has no generic `ExecuteStream`, no
`SetAutonomousMode`, no `ListAvailableTools`, and the request type carries
no autonomy field, so enterprise code can neither select a profile nor
grant execution authority. `OrchestratorCommandExecutor`,
`OrchestratorApprovalStore`, and the autonomy/fix-verifier/license
dependency interfaces are removed along with their pulse-side adapters
(`orchestratorChatAdapter.ExecuteStream`/`ExecuteCommand`/
`SetAutonomousMode`, `orchestratorApprovalAdapter`,
`autonomyLevelProviderAdapter`, `patrolFixVerifierAdapter`,
`licenseCheckerForOrchestrator`). The enterprise investigation
orchestrator no longer parses `PROPOSED_FIX`, runs guardrails, executes
commands, or queues command-shaped approvals: it consumes the structured
investigation result and, only for a non-nil proposal from a completely
successful run, submits through the broker and persists the resulting
`ActionReference`. Command text in investigation prose is inert
historical text with no execution, approval, or remediation-plan pathway
(`classifyInvestigationOutcome` reads only conclusion markers). The core
`PatrolService.generateRemediationPlanFromInvestigation` path that copied
`Fix.Commands` into an executable remediation plan is deleted, so no
second command-backed remediation artifact remains. The legacy
`aiautofix` command-approval endpoints fail closed
(`command_fix_retired`), never approving, re-arming, or executing a
persisted command-shaped fix. Investigations reference their canonical
action through the additive `Action *ActionReference` fields on
`InvestigationSession` and `InvestigationRecord`; the command-shaped
`ProposedFix`/`ApprovalID` fields remain readable for old persisted
records only and are never populated by new investigations nor re-armed
into executable approvals. Proposal parameters live in the canonical
action audit, not duplicated into investigation stores. Proofs:
`TestOrchestratorActionBrokerIsProposeOnly`,
`TestOrchestratorChatServiceIsInvestigationOnly`,
`TestOrchestratorDepsHasNoCommandOrAutonomyDeps`,
`TestActionProposalWireShapeIsTypedAndCommandFree`, and
`TestActionReferenceIsAdditiveOnInvestigationShapes` in
`pkg/aicontracts/contracts_test.go`, plus the enterprise
`TestProposedFixProseIsInertHistoricalText` negative regression.
Tool-call safety classification is registry-owned through the canonical
invocation descriptors in `internal/agentcapabilities/invocation.go`:
every registered Pulse tool has a static or discriminator-based
descriptor whose classification carries both a workflow kind
(read/resolve/write) and a mutation target (`none`, `pulse_state`, or
`infrastructure`). A mixed tool's descriptor must exactly cover its
schema enum for the declared discriminator (Kubernetes discriminates on
`type`, not `action`); `ToolRegistry.Register` panics on a missing
descriptor or on missing/extra cases, so an unclassifiable tool is not
registerable. Missing, malformed, or unknown discriminator values
classify fail-closed as write/infrastructure. Provider projection
(`ToolRegistry.ListTools`/`ListToolGovernance`) and runtime enforcement
(`ToolRegistry.Execute`, before the handler runs) consume the same
descriptor under one `InvocationPolicy` (control level plus the
request-local `deny_infrastructure_mutations` restriction on the
executor, which clones per session and never serializes): projection
removes forbidden enum values, drops tools with no permitted
invocation, and recomputes the offered governance action mode, so the
offered schema and the enforcement boundary can never disagree.
Control-level blocks keep returning the operator guidance message;
policy blocks return the shared invocation-blocked result.
Registration authority on the Assistant tool registry is split:
`registerBuiltin` is the construction-time path for canonical Pulse
tools (shared descriptor mandatory, override rejected), while
`RegisterExtension` - the only path exposed through
`PulseToolExecutor.RegisterTool` - rejects every canonical tool name
outright and requires the extension to declare its own descriptor.
Registry entries are append-only: a name registers exactly once, so a
later registration (with or without a descriptor override) can never
replace an already-governed handler such as pulse_read's
execution-intent-enforcing exec path.
The classification vocabulary is closed: descriptor validation rejects
any class outside the known kinds and mutation targets, and
`InvocationPolicy.Allows` independently denies unknown mutation targets
outright, so an unvalidated class still cannot execute. Descriptor
lookups and registration store deep copies, so callers can never mutate
the canonical table through shared case maps or static class pointers,
and registration rejects descriptor overrides for canonical tool names
(overrides exist only for genuinely non-canonical extension/test
names). The projected governance action mode derives from mutation
targets, not workflow kinds, and is recomputed even when no enum value
was removed; a projection whose remaining invocations mutate nothing
downgrades to scope-only approval metadata. Docker `check_updates`
classifies read/none: it queues a read-only scan and must not drive
verification workflow or make a read-only Docker projection look mixed.
Execution posture is profile-owned through the core-only, never-
serialized `tools.ExecutionProfile` (interactive Assistant, Patrol
detection, Patrol investigation). Both Patrol profiles are
non-interactive, deny infrastructure mutations, and clear any inherited
autonomous mode; detection additionally restricts pulse-state mutations
to an explicit allowlist of the finding lifecycle tools
(patrol_report_finding / patrol_resolve_finding - a blanket pulse-state
allowance would also permit alert dismissal and knowledge writes),
while investigation denies all pulse-state mutations. Chat turns build
ONE effective request executor (control level, autonomy, profile,
resolved context) BEFORE provider projection and clone that executor
for every provider attempt, so the offered schema and the runtime
boundary always agree; scheduled Patrol (`ExecutePatrolStream`) applies
the detection profile the same way, and `ListAvailableTools` projects
through the identical path. Non-interactive profiles independently hide
pulse_question from the manifest AND runtime-block it in the agentic
loop before the interactive-call-set special case - a fabricated
question call returns a non-interactive error without emitting a
waiting event and sibling tool calls from the same provider turn keep
processing; approval waits never block (they queue). Assistant and Watch
do not infer diagnostic sufficiency from tool-call totals or silent tool turns.
Configured run limits and repeated identical-call/error recovery still bound
execution. Tool results never receive count-based wrap-up instructions.
Interactive runs additionally enforce a
look-before-asking gate: a `pulse_question` issued before the run has
attempted any real tool call is refused with an error tool result that
steers the model to read-only enumeration (the resolve-before-asking
prompt policy made enforceable — small local models otherwise open
natural first questions like "any alerts?" with a "which resource?"
elicitation whose answer is derivable from enumeration). The refusal
emits no question card or clarify event, sibling tool calls from the
same provider turn keep processing rather than tripping the
interactive-call-set skip path, any queued tool attempt (success or
error) satisfies the gate for the rest of the run, and the gate fails
open after `maxLookGateBlocks` refusals so a genuinely unanswerable
prompt cannot livelock. The stream-boundary promise — a natural first
question produces an answer, never a clarification card — is pinned in
the interaction scenario corpus. The system prompt describes detection and
investigation modes directly instead of claiming controlled or
autonomous execution. Non-interactive operation grants no mutation
authority. The profile vocabulary is closed: unknown profile values are
rejected at apply time and classify as non-interactive, never as the
permissive interactive default.
The typed proposal channel rides the investigation profile.
`patrol_action_capabilities` and `patrol_propose_action` are side-effect-free,
mutation-none, read-kind tools projected and executable ONLY under the
investigation profile: the registry policy rejects a fabricated call under any
other posture before the handler runs. The capability lookup accepts one
canonical resource ID and reads the same tenant-bound catalogue used by
proposal validation without consuming proposal cardinality. This allows the
model to inspect the typed actions of a causal resource discovered during the
run instead of forcing the original finding resource to remain the action
target. The proposal tool's model-authored schema carries only
resource_id, capability_name, params, and reason; proposal, finding,
investigation, and evidence identity are injected from trusted
orchestration context through the request-local proposal capture sink
(`tools.ProposalCapture`), which executor clones share so one run has
exactly one capture. Tool calls carry an explicit invocation envelope
(tool-use ID, name, arguments) through `ExecuteInvocation`, and the
sink keys on call identity plus payload fingerprint: an idempotent
replay (same ID, same payload) re-succeeds; the same ID with a
different payload latches a terminal integrity error; a second distinct
valid proposal latches terminal ambiguity - and both terminal states
INVALIDATE the captured proposal, because concurrent per-turn execution
makes "first" nondeterministic. Proposals count only after catalog
validation (advertised capability, declared/required/enum parameters,
sensitive parameters rejected before success with no value echo in any
output) and a successful tool result. `ExecuteInvestigationStream`
returns proposal cardinality as a structured result - consumers never
reconstruct proposals from session messages - with typed errors for
ambiguity, integrity violations, and the failed-attempts-only case
(zero successful proposals with failed attempts is an error outcome,
never the valid zero-proposal conclusion); `ListInvestigationTools`
projects through the identical profile path. Proposal parameter values
exist only transiently for provider continuation and validation: the
canonical exposure projector
(`RedactToolCallArgumentsForExposure`) redacts them from the durable
chat transcript and every tool_start/tool_progress/tool_end stream
event, while the action audit remains their canonical durable home.
Proposal validation reuses the planner's canonical exported rules
(`actionplanner.FindCapability` exact-name matching and
`actionplanner.ValidateParams` for declared/required/typed/enum/pattern
parameters and malformed schemas), so proposal acceptance and planning
can never drift; the sensitive-parameter rejection stays a
proposal-specific ratchet on top. Captured proposals are immutable:
params and evidence identity are deep-cloned on capture and again on
outcome, and fingerprint serialization failures are errors, never a
shared sentinel value. Provider-streamed RawInput overrides on progress
events are discarded for exposure-restricted tools - the override is
unredacted model output and would reintroduce exactly what the
projector removed. An investigation run refuses to start without
finding and investigation identity, and any run error nils the
proposal while preserving simultaneous proposal errors via errors.Join:
a non-nil proposal exists only from a completely successful run.
Proofs: `internal/ai/tools/proposal_capture_test.go` (concurrent
ambiguity with nil proposal, replay/integrity semantics by tool-use ID,
failed-attempts typed error, sensitive rejection without echo,
profile-only projection and execution, caller-mutation immunity,
exact-name matching, canonical planner validation) and the end-to-end
loop redaction and progress-override proofs in
`internal/ai/chat/service_tooling_test.go`. A blocked question call is persisted to
the durable transcript paired with its refusal result (the assistant
message records the call, so the transcript must never retain an
unanswered call). Profile proofs live in
`internal/ai/tools/invocation_policy_test.go` (detection allowlist,
investigation structural read-only, profile clone isolation) and
`internal/ai/chat/service_tooling_test.go` (profile prompt modes,
question-tool hiding, effective-executor projection).
Handler-level checks remain defense in depth, and pulse_read's
structural execution-intent classifier
(`ClassifyExecutionIntent` rejecting write-or-unknown command text
before dispatch) is mandatory second-stage enforcement for exec, not
merely defense in depth: the static read/none descriptor alone cannot
prove arbitrary command text safe. The
deny restriction is deliberately separate from autonomous mode:
suppressing interactive questions grants no mutation authority.
`pulse_file_edit` is write-only (append/write); file inspection routes
through `pulse_read` action="file". The retired hard-coded classifier
misread Kubernetes's discriminator and classified `type:"scale"` as
read, and mixed tools such as Docker bypassed tool-level control
gating entirely (an `action:"update"` reached direct execution at
read-only). Proofs: `internal/agentcapabilities/invocation_test.go`
and the invocation-policy regression suite in
`internal/ai/tools/invocation_policy_test.go`.
The same ownership includes the Pulse query tool schema under
`internal/ai/tools/`: topology-query input names must stay canonical inside
the AI runtime itself, so new tool arguments such as `max_proxmox_nodes`
cannot reintroduce parallel legacy aliases once the backend query contract is
renamed.
That same AI tool ownership also governs `pulse_read action="exec"` safety.
`internal/ai/tools/tools_query.go` and `internal/ai/tools/tools_read.go` must
fail closed on unknown commands: the shared read path may execute only commands
that are known read-only by construction or proven read-only by an explicit
content inspector. The runtime must not preserve a model-trusted fallback for
unknown binaries, custom scripts, downloads, shells, or dual-use interpreters
such as `python`, `node`, `ruby`, `perl`, `bash`, or `sh`, because those
surfaces can mutate state even when invoked in non-interactive forms. Wrapper
and inspection-shaped commands must inherit the same fail-closed boundary:
`timeout` may only bound an inner command that independently classifies as
read-only, `env` with a utility is executable and therefore blocked, `find`
must reject write or exec actions, `awk` and `sed` must not regain read-only
status through direct invocation or pipes, `wget` is read-only only for spider
checks, and `curl` must reject request-body, mutation-method, config, cookie
jar, upload, and file-output forms while preserving ordinary HTTP(S) probes.
That same AI tool ownership now also includes canonical resource-native
control. `internal/ai/tools/executor.go`,
`internal/ai/tools/tools_control.go`, and `internal/api/router.go` must keep
API-backed control actions such as TrueNAS app start/stop/restart on the
shared `pulse_control` tool with `type="resource"` and native audited
execution, instead of adding provider-local control tools or bypassing the
shared approval and policy model.
That same AI tool ownership now also includes canonical resource-native
diagnostics. `internal/ai/tools/tools_read.go`,
`internal/ai/tools/executor.go`, and `internal/api/router.go` must keep
API-backed app log reads such as TrueNAS app-container logs on the shared
`pulse_read` tool with `action="logs"` and `resource_id=<canonical app>`
instead of requiring `target_host` for non-agent platforms or adding a
provider-local log-read tool.
That same AI tool ownership now also includes canonical resource-native
configuration reads. `internal/ai/tools/tools_query.go`,
`internal/ai/tools/executor.go`, and `internal/api/router.go` must keep
API-backed app configuration reads such as TrueNAS app-container runtime
shape on the shared `pulse_query` tool with `action="config"` and
`resource_id=<canonical app>` instead of forcing those resources through the
guest-config shim or adding a provider-local config tool.
That bounded tool set is the current Assistant floor for TrueNAS. Supported
now means read-side app logs/config and native app start/stop/restart on
canonical `app-container` resources through the shared `pulse_read`,
`pulse_query`, and `pulse_control` tools. Pulse does not promise a blanket
TrueNAS admin plane, host command execution on API-backed systems without the
unified agent, or provider-local AI tools outside the shared action-governed
runtime contract.
That same platform-claim boundary now also covers the admitted VMware vSphere
direction. The phase-1 Assistant floor is
read-only access to canonical VMware-backed `agent`, `vm`, and `storage`
resources through the shared read and query paths only. The AI runtime must
not add VMware-local tools or action verbs for VM power, snapshot lifecycle,
guest operations, host maintenance, or cluster administration before the
governed action surface expands.
That same VMware AI rule now also includes capability exposure. Even if
runtime code can identify VMware-backed actions through upstream APIs,
canonical resource capabilities and tool routing must stay read-only in phase
1: shared `pulse_read` and `pulse_query` may expose VMware-backed context, but
`pulse_control` must not grow VMware verbs and VMware-backed resources must not
advertise action metadata that implies a supported VMware admin plane.
That same capability boundary also governs resolved-context enforcement inside
`internal/ai/chat/context_prefetch.go`, `internal/ai/tools/tools_query.go`, and
`internal/ai/tools/tools_control.go`. Once the shared runtime has resolved a
canonical VMware-backed `agent`, `vm`, or `storage`, Assistant summaries may
not emit `pulse_control` instructions for it. Phase-1 VMware host and
datastore summaries without discovery must direct `pulse_query` or
`pulse_read` only, VMware guest summaries must stay explicitly read-only, shared
resource registrations must stay limited to read-side actions, and any
attempted `pulse_control` restart/stop/shutdown path must fail as a read-only
denial instead of falling through to legacy guest resolution or provider-local
control assumptions.
That same boundary also governs shared Assistant wording in
`internal/ai/chat/service.go` and `internal/ai/tools/tools_control.go`: the
base system prompt and `pulse_control` schema/description must not claim that a
generic `vm` or `system-container` is controllable. Shared AI text must describe
control as capability-gated and explicitly allow read-only platform variants
such as VMware phase-1 guests. For a canonical Proxmox VM or LXC that does
advertise a lifecycle capability, that advertised hypervisor action is the
authority: start, stop, shutdown, and reboot do not depend on QEMU guest-agent
availability, and Assistant must not replace the offered governed action with
manual `qm` or `pct` instructions. Claims that a guest agent or capability is
unavailable require current tool evidence. The provider-facing `pulse_control`
schema exposes only `type=resource`, `resource_id`, and `action`; retired guest,
raw-command, target-host, host-routing, and force fields must not remain in the
schema as misleading alternate paths.
That same VMware AI rule also includes the investigation path. Alarm, health,
event, task, metrics-history, and snapshot-tree context for VMware-backed
resources must stay reachable through those same shared read/query surfaces
and canonical resource links rather than through a VMware-only AI tool or
provider-local incident adapter.
That same shared read/query rule also governs AI prompt hints and prefetch
summaries in `internal/ai/chat/service.go` and
`internal/ai/chat/context_prefetch.go`: API-backed read-only resources such as
VMware-backed `agent` / `vm` / `storage` and TrueNAS-backed host/storage
resources must not inherit synthetic `target_host` log-routing hints from
agent-routed platforms. Shared AI context should carry canonical
`resource_id` guidance for those resources, and `pulse_read action=logs` may
only be suggested when the runtime has an explicit native resource read path
such as supported TrueNAS `app-container` logs.
If a caller still targets `pulse_read action=logs` with `resource_id` for a
resource that lacks that native log path, the shared tool boundary must fail
as a structured blocked response with a governed recovery hint toward the
correct shared path, such as `pulse_query action=get` for API-backed read-only
resources or `target_host` plus `container` for agent-routed app logs.
When that recovery path is safe to execute deterministically, the blocked
response should also carry a structured recovery tool call so the shared
agentic loop can retry through the correct shared tool and arguments instead
of assuming every recovery is a `command` rewrite on the original tool.
That same VMware AI rule also now includes mention resolution. Frontend
Assistant mention payloads for VMware-backed `agent`, `vm`, `storage`, and
canonical `app-container` resources must preserve the shared unified resource
ID coming from `/api/resources`, and backend prefetch/runtime code must
resolve those mentions through canonical read-state lookups rather than
reconstructing provider-local IDs in the UI or adding VMware-only read routes
under `/api/vmware/*` for Assistant context.
That same AI tool ownership also applies to recovery-backed storage reads.
When `internal/ai/tools/adapters.go` returns recovery points with malformed
persisted metadata omitted at the shared recovery-store boundary, the storage
tool runtime in `internal/ai/tools/tools_storage.go` must still keep snapshot
and backup-task results visible by preferring canonical point fields such as
`display.clusterLabel`, `display.nodeHostLabel`, `display.entityIdLabel`,
`display.itemType`, and point outcome before falling back to raw `details`.
That availability contract also applies when recovery points are the only storage data source.
`internal/ai/tools/executor.go` must keep `pulse_storage` exposed whenever a
`RecoveryPointsProvider` is configured, so tenant and self-hosted Chat surfaces do not lose
recovery-backed snapshot and backup-task reads just because backup/read-state adapters are absent.
Storage pool rows built by `storagePoolSummaryFromResource` in
`internal/ai/tools/tools_storage.go` (`pulse_storage` pools and `pulse_query`
storage pools) take `enabled` from the storage facet's own
`StorageMeta.Enabled`, so storage that reported itself unavailable (Proxmox
`active=0`, unified status `offline`) never reads as administratively disabled
to Patrol or Assistant. `active` is the facet's `Active` flag and also requires
the row not to be `offline`, because some sources (PBS datastores) report
`Active` even while unavailable. Only a row without a storage facet falls back
to the status-derived value.
`TestStoragePoolSummaryReadsStorageFacetFlags` in
`internal/ai/tools/tools_storage_test.go` pins unavailable, disabled, online,
offline-but-flagged-active and facet-less rows.
Tenant-scoped AI services must now also follow canonical runtime ownership:
Patrol may initialize and operate from tenant `ReadState` and unified-resource
providers without requiring a tenant snapshot-provider bridge, and
`internal/api/ai_handlers.go` must not mint tenant-local `StateSnapshot`
adapters purely to satisfy Patrol when canonical tenant read-state is already
available.
That same AI ownership also extends to persisted runtime state under
`internal/config/persistence.go`: AI findings, usage history, patrol run
history, and chat sessions must not keep legacy plaintext files on the runtime
primary path once the process can read them. Plaintext AI persistence files may
only serve as migration input and must be rewritten immediately into
encrypted-at-rest storage on load.
That same Patrol runtime ownership also governs Patrol run-summary taxonomy.
`internal/ai/` must keep API-backed TrueNAS systems distinct from unified-agent
hosts in runtime counts, triage summaries, and persisted Patrol run history
instead of collapsing both surfaces back into `hosts_checked` or generic
`agent` resource wording.
That same config-persistence boundary also owns fixed runtime file paths: the
resolved data directory must be normalized once and fixed AI/runtime filenames
must rejoin through the shared storage-path helper instead of raw
`filepath.Join(dataDir, "...")` construction.
That same persistence boundary also governs AI memory package storage roots:
fixed store files such as change history, incident memory, and remediation
history must resolve through normalized owned data directories and fixed
storage-leaf joins instead of raw `filepath.Join(dataDir, ...)` paths.
The same migration-only rule applies to guest knowledge under
`internal/ai/knowledge/`: legacy `.json` knowledge files and plaintext `.enc`
knowledge files may only serve as migration input, and the knowledge store
must rewrite canonical encrypted-at-rest storage immediately on load instead
of leaving guest knowledge plaintext on disk until a future note update.
That same knowledge-store boundary also governs directory scans: when the store
rejoins discovered knowledge files for reads, it must route those already-owned
leaves back through the shared storage-path helper instead of rebuilding raw
`filepath.Join(dataDir, entry.Name())` paths.
Chat-session and guest-knowledge persistence now also keep canonical on-disk
names opaque and machine-owned. Legacy identifier-derived filenames may be
discovered only by inspecting already-owned files for embedded record IDs, and
the next successful write must rewrite them to hashed canonical paths instead
of preserving user-controlled identifiers as filesystem path segments.
That trust boundary also applies when the store is constructed: if the
knowledge store cannot initialize encryption, construction must fail closed
instead of silently creating a plaintext-at-rest runtime store.

Unified-resource-backed AI context now also consumes the canonical
policy-aware metadata contract. The AI runtime may summarize governed resource
policy counts for context, and it must switch to `aiSafeSummary` when a
resource is marked `local-only` instead of leaking raw resource names or local
identifiers for restricted resources through ad hoc context formatting.
That governed context should also surface the canonical routing posture and
redaction hints that were derived from the shared policy model, so prompts
reflect the same sensitivity, routing, and scrub decisions that the runtime
uses for export boundaries instead of rebuilding privacy posture locally.
That governed posture block and its export-routing inputs now also flow through
the dedicated `internal/ai/resource_context_policy_model.go` owner, so
`resource_context.go` stays on AI context composition instead of duplicating
policy redaction sections or recomputing export metadata inline.
That same ownership now includes the canonical policy-posture summary object
itself: `resource_context.go` must compute the shared
`unifiedresources.SummarizePolicyPosture(...)` result exactly once per unified
context build and pass that summary into
`buildUnifiedResourcePolicyContext(...)`, instead of letting downstream AI
context helpers silently rebuild posture counts from the raw resource slice.
The same shared policy presenter also owns the routing-scope labels used in
the AI-facing policy surfaces, so the policy wording stays canonical instead
of being rendered inline by the consumer.
That same policy boundary now applies to chat structured-mention prefetch and
resource-summary formatting: mention resolution must consume canonical
unified-resource policy metadata, skip discovery fan-out when governed
redaction already blocks cloud-safe raw context, and withhold routing
coordinates, bind-mount paths, hostnames, and discovery file paths whenever
resource policy marks those identifiers as redacted.
The governed mention formatter must also render the policy line and redaction
list through the shared unified-resource policy presentation helper so the
chat prefetch path stays aligned with the same canonical sensitivity, routing,
and redaction labels used by the AI summary and resource drawer.
The decision to show that governed mention block now comes from the shared
unified-resource policy helper as well, so the local gate stays aligned with
the same routing and redaction rules as the rendered summary itself.
The governed mention preamble and footer text now also come from the shared
policy presenter, so the warning copy around the block does not drift from the
canonical policy wording.
The complete governed mention block is also assembled by the shared policy
presenter, so chat prefetch only decides when to render it and never rebuilds
the summary layout locally.
The chat prefetch path now also calls the shared governed-summary predicate
directly at each mention site, so it no longer carries a local wrapper around
the canonical policy decision or a separate mention-summary trim helper.
Structured mentions and command routing use the shared unified-resource
reference resolver. An explicit ID selects one owned record for current name,
type, policy and native execution coordinates. Client labels, node hints and
unscoped provider IDs cannot replace an unknown or ambiguous selection.
The chat mention picker now also carries the canonical preferred resource
label as `label` through the structured mention payload, and the insertion
path uses that same label for prompt text and cursor placement, so mention
search, selection, and submission do not depend on a raw `displayName` field
fork.
All mention kinds carry the registry-issued ID from /api/resources unchanged.
The ID's spelling is not an execution contract. Bounded legacy mention and
session-registration formats remain readable only when they identify a unique
current owner. ResolvedContext stores records by that ID and indexes only
unambiguous names and aliases, including after JSON reload and pinned clearing.
Native provider IDs remain executor coordinates. Session registration and
explicit-access marking share ResourceRegistration.ResolvedID rather than
rebuilding identity independently. Recent context omits unresolved owners and
continues to withhold governed identifiers.
Compatibility-only top-level TrueNAS mention types must also collapse to the
canonical `agent` host type at that same handler boundary, so the AI runtime
does not carry a parallel raw `truenas` mention contract once transport input
has been normalized.
That same compatibility-collapse rule also applies to alert, finding, and
Patrol scope payloads. API-backed TrueNAS systems may still keep `truenas`
platform metadata and separate run-history coverage counts, but AI resource
type fields must normalize to canonical `agent` once they cross the governed
runtime boundary.
Patrol scope resolution registers the alert subsystem's Docker references as
aliases, built by the same `internal/alerts` builders rather than formatted
locally: `DockerHostResourceID` on the Docker host record, and on each
container record `dockerAlertScopeAlias`, which passes the container's name so
a container reported without an ID answers to its marked name reference
(`docker:<host ID>/name:<name>`) and never to its host's reference.
`dockerServiceAlertScopeAliases` registers service references, including
`docker:<host ID>/service/name:<name>` for a service without an ID, on the
owning host. The legacy snapshot record of a container without an ID has no ID
to scope by, so only the unified read state can scope its alerts. Proof:
`TestResolvePatrolScopeResolvesDockerAlertResourceIDs` in
`internal/ai/patrol_run_test.go` and
`TestDockerServiceAlertScopeAliasesDeriveIDFromName` in
`internal/ai/patrol_state_scope_alias_test.go`.
The same governed-context rule also applies to the main unified AI resource
overview: infrastructure, workload, alert-label, and top-consumer summaries
must not leak raw resource names, cluster labels, IP addresses, or unresolved
topology identifiers once canonical resource policy marks aliases, hostnames,
platform IDs, or addresses as redacted. Sensitive resources should remain
useful through `aiSafeSummary` and explicit redaction markers rather than
falling back to raw local identifiers in list or summary sections.
That same governed policy boundary now extends through AI tool payloads and
chat-memory extraction. Resource-bearing `pulse_query` results must carry the
canonical `policy` and `ai_safe_summary` fields derived from unified resources,
and deterministic knowledge extraction must prefer those governed summaries
when policy redaction covers aliases, hostnames, or platform IDs instead of
persisting raw resource labels into cached AI facts.
That same `pulse_query` boundary now also owns canonical resource coverage for
API-backed platforms such as TrueNAS. The runtime must expose canonical
`agent`, `app-container`, `storage`, and `physical-disk` resource views
through the shared unified-resource model instead of falling back to Proxmox-
or Docker-local enumerations when a platform projects onto canonical host,
storage, disk, or workload contracts. Compatibility aliases such as
`system` and `storage-pool` may still be accepted at the `pulse_query`
boundary, but the governed runtime contract is the canonical `agent` /
`storage` read path and the resolved-context registration emitted from it.
That same runtime contract applies to resource-native diagnostics. When
resolved context points at an API-backed canonical `app-container` such as a
TrueNAS app, chat/runtime prompt hints and tool execution must route log reads
through `resource_id` on `pulse_read` rather than inventing agent-host hints
for platforms that are not reached through the unified agent.
Unified AI context should follow the same rule: storage summaries may mention
canonical storage pools and physical disks that need attention, but must not
mislabel lower-topology storage resources such as TrueNAS datasets as
top-level pools.
That same requirement includes `pulse_query action=config`: guest-config
payloads must carry canonical resource policy metadata, and config-fact
extraction must not persist raw guest hostnames when governed redaction covers
hostname or platform identity fields. The same `action=config` contract now
also applies to API-backed canonical `app-container` resources such as
TrueNAS apps: runtime routing must resolve the shared resource identity first
and then read native config through the owned provider path rather than
falling back to guest semantics.
Outbound model-bound context exports now also belong to this runtime
boundary. When the AI service assembles unified-resource context for a model
request, it must record a durable export audit with the active destination
model and governed redaction decision instead of treating the prompt boundary
as a transient formatting step.
That export decision must come from the shared unified-resource privacy
helpers, so sensitivity floors and redaction-triggered routing stay aligned
with the canonical policy contract instead of being recomputed in AI-local
code.
The export audit should also record canonical human-readable redaction labels
from the shared policy presentation helper, so the audit trail and the
resource-context surfaces speak the same governed redaction language instead
of reformatting hint names locally.
The canonical AI-safe summary builder also owns the `sensitive` and
`restricted` suffix phrases, so downstream AI consumers should treat those
ending fragments as shared policy output instead of inventing their own
wording.
The same AI runtime boundary now also consumes the canonical unified-resource
timeline when it assembles rich resource or incident context. Recent-change
context should come from the shared resource store first so AI prompts reflect
the same change record that powers the resource API, with patrol-local change
detectors only serving as fallback coverage when the canonical store is not
available. When that patrol-local fallback is used, it must render through the
shared memory change presentation helper so the same heading, scope prefix, and
change-type labels are reused instead of being rebuilt ad hoc in AI-local code.
`internal/ai/memory/incidents.go` is therefore an alert-scoped investigation
projection only: it may retain notes, analysis, command executions, runbooks,
and alert lifecycle breadcrumbs for one incident, but it must not become a
parallel source of truth for durable backend history that already belongs to
`internal/unifiedresources/`.
When canonical resource history is available, the incident read path must also
project alert lifecycle and remediation entries back out of the unified-resource
timeline instead of reading those durable facts only from AI memory. AI memory
may retain annotation-only entries such as notes and analysis, but the live
incident timeline shown to handlers, prompts, and operators should read as one
projection over canonical resource history plus investigation-local annotations.
That read-side projection must also discard incident-local derived lifecycle
state when canonical history is present: acknowledgement, resolution, and
command or runbook entries in `internal/ai/memory/incidents.go` may still
exist as compatibility-era shell state for segmentation and fallback, but the
projected incident returned to runtime consumers must rebuild those fields from
canonical resource changes and preserve only annotation-local entries such as
analysis and notes.
The remaining shell should stay as narrow as possible: alert occurrence
boundaries and annotation anchors may remain private implementation state, but
public incident status, acknowledgement, and remediation entries should be
treated as read-model output rebuilt from canonical history whenever that
history exists.
That boundary should also be visible in code shape: the persisted incident
shell used by `internal/ai/memory/incidents.go` should stay a private storage
model for occurrence segmentation and annotations, while the exported
`Incident` type remains the public/projected read model returned to handlers
and operators.
The AI correlation root-cause engine also consumes the canonical unified-
resource relationship model directly, so cross-resource reasoning stays aligned
with the same relationship edges that back the resource API instead of
maintaining a parallel relationship vocabulary inside AI correlation.
The canonical relationship-summary helper also feeds resource change records,
so AI timeline prompts read the same relationship wording and edge labels that
the unified-resource contract emits instead of building another summary shape
in AI-local code.
The same shared change presenter also owns the resource state, restart,
incident, and config summary fragments used by change emission, so the AI
timeline prompt can reuse the canonical from/to wording before it formats the
markdown section itself.
The Patrol-backed correlation endpoint, resource-intelligence payload, and
seed prompt correlations now flow through the shared AI intelligence facade
first, so the detector remains an implementation detail behind one canonical
correlation access path instead of being routed directly by handlers or prompt
builders.
The browser does not read the global learned-correlation list:
`frontend-modern/src/api/ai.ts` carries no client for
`/api/ai/intelligence/correlations`, and neither the shared AI intelligence
store nor the Patrol page loads it, because no page renders it. The resource
drawer reads per-resource correlations from the resource-intelligence payload.
The browser does not read `GET /api/ai/circuit/status` either:
`frontend-modern/src/api/ai.ts` carries no client for it and the shared AI
intelligence store holds no breaker state, because an open breaker already
reaches Patrol as the `circuit_open` runtime block cause. The route stays a
backend API surface.
AI-facing policy metadata must also be cloned through the shared unified-
resource policy helper so chat and tools consumers do not maintain their own
policy copy logic. Chat mention prefetch now calls that shared helper directly
at each resolved mention site rather than going through an AI-local wrapper.
AI resource and intelligence consumers now also refresh canonical identity and
policy through the shared unified-resource metadata helper, so the AI runtime
no longer keeps its own slice-level normalization shim for the same
composition.
Chat knowledge extraction and resource-context rendering now also consume the
shared unified-resource label helpers directly, so governed labels and
redacted values stay consistent without AI-local presentation shims.
Those same paths also use the shared resource display-name helper, so the
name-or-ID fallback stays aligned across chat extraction, resource context,
and unified adapter presentation.
The unified resource context's IP summaries now also route through the shared
policy redaction helper, so the local "IPs" line follows the same governed
redaction decision and label vocabulary as the rest of the policy-aware
resource presentation layer. Cluster labels for AI resource context now also
come from the shared unified-resource presentation helper, so the same policy
rules govern cluster names and IP summaries instead of leaving the fallback
logic in the AI package.
The policy-posture aggregate itself now also comes from
`internal/unifiedresources/policy_posture.go`, so AI summaries and resource
context reuse the same canonical sensitivity, routing, and redaction counts
instead of collecting governance posture in an AI-local helper.
That shared presentation layer also owns the elapsed-time and "ago" wording
utilities, so the same "time ago" phrasing stays consistent across resource,
incident, and fallback memory summaries instead of being reformatted
independently.
The canonical resource-change kind, source type, and source adapter labels
now also come from the shared change presentation helper, so the resource
summary card and drawer history use the same badge vocabulary instead of
hardcoding their own labels.
Action-plan stale-plan protection now keys the durable audit payload on the
canonical `resourceVersion`, `policyVersion`, and `planHash` fields only,
so the audit record stays on the minimal deterministic contract instead of
carrying extra versioning for relationship topology.
Resource-only incident context should follow the same rule: if an alert
timeline is absent, the incident prompt path should fall back to the canonical
unified-resource timeline rather than depending only on patrol-local change
memory.
When both an alert identifier and a canonical resource ID are known, the prompt
path should include both surfaces in source-precedence order: alert-scoped
incident memory first, canonical resource timeline second.

The same runtime boundary now also owns durable action execution auditing.
`internal/ai/chat/service.go` initializes the unified-resource audit store on
startup. Governed API action execution must enter through
`POST /api/actions/{id}/execute`, which records `executing` before invoking the
registered executor and records the terminal `completed` or `failed` result
afterward; missing executors must fail closed without mutating the approved
audit record. Existing write-action tool paths under `internal/ai/tools/`
must keep their persisted lifecycle and result records aligned with that same
unified-resource action state machine: approval decisions must use the
canonical action decision transition, execution starts must use
`BeginActionExecution` plus `RecordActionExecutionStart`, and terminal tool
results must use `CompleteActionExecution` plus
`RecordActionExecutionResult` rather than inventing AI-local execution states.
AI incident handling must now also write durable resource-history facts
through the canonical unified-resource change store when a concrete resource
target is known. Command executions and runbook executions triggered during an
alert investigation may remain visible inside `internal/ai/memory/incidents.go`
as operator-facing incident projection entries, but the durable backend truth
for those events now belongs to canonical `ResourceChange` kinds such as
`command_executed` and `runbook_executed`, keyed by canonical resource ID and
linked back to the alert through metadata instead of being stored only in AI
memory.
Alert snooze and resume entries follow the same projection rule: AI incident
memory may present `alert_snoozed` and `alert_unsnoozed` for operator context,
including actor and exact expiry metadata, but it must read them from canonical
resource history. Those events pause and restore alert operations; they do not
infer acknowledgement, resolution, or a new incident identity.
The patrol-local `memory.ChangeDetector.GetChangesSummary` path now also
delegates to the shared memory recent-change presentation helper, so any
future fallback summary entry point inherits the same heading, resource
prefixing, and change-type labels without re-implementing the markdown shape.
That detector is a reader of legacy history, not a detector. Its on-disk
history (`ai_changes.json`) was written by releases that still ran change
detection, which patrol dropped when it moved to agentic execution. The
detector loads that file once at construction and never modifies it, so it
holds no lock and has no background save to join before a data directory is
removed. `DetectChanges`, its snapshot diffing and `ResourceSnapshot` were
reachable only from tests and are retired; new change history belongs in the
unified-resource timeline, not in this fallback. Tests seed the fallback
through a legacy `ai_changes.json`, the same path production reads, and
`TestChangeDetector_LegacyHistoryIsReadOnly` proves reads leave the file and
directory untouched.
Those unified-resource action and export audit records are now also exposed
through the enterprise audit read surface so operators can inspect the
execution trail without reaching into storage internals.
AI resource and incident context now also surfaces a canonical relationship
section from unified-resource relationships, so relationship wording and edge
provenance stay aligned with the same shared resource model instead of being
reconstructed from the drawer or prompt helpers.
That relationship section is now rendered by the shared
`internal/unifiedresources.FormatResourceRelationshipContext` helper, so the
service layer only resolves the canonical resource and does not rebuild the
section format locally.
The canonical recent-change sentence formatting also lives in
`internal/unifiedresources.FormatResourceChangeSummary`, so AI runtime prompt
sections and Patrol seed context reuse the same change wording instead of
keeping another lane-local formatter.
The confidence percentage wording used by the drawer's change timeline rows
also flows through a shared frontend formatter, so the same `50%`-style
labels stay consistent across timeline surfaces instead of being re-derived
in the component.
The remaining fallback token humanization used by those same timeline and
drawer surfaces also flows through one shared frontend helper, so the
title-casing and underscore cleanup used for change and drawer labels stay
centralized instead of being reimplemented locally.
The canonical recent-change section wrapper also lives in
`internal/unifiedresources.FormatResourceRecentChangesContext`, so the AI
summary and resource-specific context share the same heading and prefix rules
instead of rebuilding that section layout locally.
The canonical memory conversion helpers also live in
`internal/ai/memory/presentation.go`, so the Patrol fallback feed and the
AI summary path translate between unified-resource changes and memory.Change
through one shared adapter boundary instead of keeping local shims.
The related-resource correlation section now also comes from the shared
correlation formatter in `internal/ai/correlation`, so resource chat and
incident prompts reuse the same learned-edge wording instead of rebuilding a
second patrol-local bullet format.
The Patrol intelligence page renders no learned correlations. The resource
drawer renders its resource-scoped learned edges through the shared
`frontend-modern/src/components/Infrastructure/ResourceCorrelationSummary.tsx`
card, so any surface that shows learned correlations keeps the same layout and
edge wording. That shared card also owns the correlation ordering and
truncation rule, so callers pass raw learned edges instead of page-specific
top-N slices.
Assistant finding handoffs now also receive a model-only finding briefing
derived from the current unified finding and structured Patrol investigation
record before the lower-level finding context. That briefing must summarize the
finding, resource, priority, current recency facts, bounded evidence and
verification summaries, investigation confidence, latest lifecycle event, and
governed action artifact metadata as factual model context without generating
Pulse-authored attention, operator-decision, or remediation guidance, while
leaving detailed lifecycle history, current
resource-state, timeline, related-finding, and action-audit
hydration in the existing canonical AI runtime handoff builders. Related
root-cause and
correlated finding records may be summarized from current unified finding state,
including their recency and latest lifecycle facts, and may seed their own
handoff resources for canonical policy, state, topology, and timeline
hydration. That related context is explanation and review context only, not
approval or execution authority. Detailed lifecycle events are
likewise current Patrol review context only. The assembled briefing, lifecycle,
and related context are policy-sanitized by the chat handoff runtime before
prompt injection, so governed resource names, IDs, aliases, nodes, paths, and
addresses are redacted or represented through the canonical AI-safe summary
instead of leaking through product prose.
The Patrol page renders no recent-change timeline. The resource drawer renders
its latest canonical change through the shared
`frontend-modern/src/components/Infrastructure/ResourceChangeSummary.tsx`
card, and its `Change history` list through the shared
`frontend-modern/src/utils/resourceChangePresentation.ts` helpers, so the
canonical recent-change wording stays aligned instead of being rebuilt per
surface.
The Patrol intelligence seed context now also prefers the canonical
unified-resource timeline before falling back to the patrol-local change
detector, so deterministic patrol context and resource detail context share
the same change source of truth.
The unified intelligence summary should follow the same rule when it counts
recent activity, so the shared AI summary and the Patrol seed context stay
aligned with the canonical timeline.
The same unified intelligence summary now also surfaces a canonical policy
posture snapshot derived from unified resources, so sensitivity, routing, and
redaction counts stay aligned with the governed resource model that the
runtime uses for prompt export and context rendering.
That posture snapshot must render redaction labels through the canonical
unified-resource hint order, not alphabetically, so the AI summary, drawer,
and any future policy surfaces all present the same redaction precedence.
Its sensitivity and routing counts must also follow the canonical
unified-resource order and shared human-readable count summaries, so the
backend summary and any frontend posture presentation stay aligned on the same
presentation sequence.
The unified AI resource data-governance block must also use the shared
unified-resource redaction-label helper directly, so the same canonical
policy labels back both the posture summary and the governed prompt context
without an AI-local wrapper.
The governed query-fact and resource-context paths must also use the shared
unified-resource policy helpers for the `aiSafeSummary` decision and
redaction predicates, so the same local-only and redaction rules are applied
consistently instead of being reimplemented in chat-local helpers.
The frontend unified-resource hook now trusts backend canonical `policy` and
`aiSafeSummary` values directly, so the canonical summary and policy posture
stay aligned with the same resource-policy boundary that governs
policy-aware routing and redaction without any frontend-local re-normalization.
The resource detail drawer now also resolves the visible AI-safe summary
through the same shared policy helper, so governed resources still show the
canonical redacted label if the backend summary is missing instead of
silently dropping the summary block.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` now carries recent changes,
dependencies, dependents, correlations, and knowledge only; policy posture
stays on the system-wide intelligence summary instead of riding the
resource-detail payload, and no frontend surface currently renders those
posture counts.
That same resource-intelligence payload also carries dependency and
dependent correlation context from unified-resource correlations, so the drawer
can show canonical correlation relationships without reconstructing them from the
relationship timeline alone.
The shared AI resource and infrastructure prompt contexts should also surface
the same canonical recent changes section before any patrol-local fallback so
the model sees the same timeline entries that power the resource API and
intelligence summary counts.
The `/api/ai/intelligence/changes` endpoint should also route through the
canonical unified-intelligence recent-change accessor before any
patrol-local detector fallback, so the API surface reads the same unified
timeline source that powers the summary payload.
Retired dashboard Pulse Brief context follows the same monitoring-first AI
boundary in negative space: `frontend-modern/src/features/dashboardOverview/`
and the Dashboard route must not be restored just to create an Assistant-ready
operator paragraph. Future overview or brief surfaces need a governed product
owner first, must pass fact-bound structured context from owning Infrastructure,
Workloads, Patrol, storage, recovery, and alert summaries, and must not let an
unbounded prompt become a route's source of truth.
Any future route-to-chat handoff inherits the server-set execution mode
instead of carrying one: the submitted chat request carries no execution-mode
field (`/api/ai/chat` runs every request approval-required), and the
operator's persistent Assistant control-level setting stays untouched.
Scoped Assistant handoffs may also send bounded `handoff_context` text,
structured `handoff_resources`, and safe structured `handoff_actions` through
`/api/ai/chat`. That context is model-only session metadata, not saved
user-authored message text, and the backend must clamp the exchange to
approval-required mode whenever such scoped handoff context, resources, or
action references are present (`HandleChat` currently clamps every exchange).
Patrol finding IDs remain stricter: when
`finding_id` resolves, backend-refreshed durable Patrol context remains the
canonical authority; the handler may merge only a recognized same-finding
Patrol product handoff section as secondary model-only briefing, and it must
drop mismatched resource/action references plus raw command payload lines.
Direct alert-investigation runtime handoffs follow the same rule.
`/api/ai/investigate-alert` must set
`ai.ExecuteRequest.AutonomousMode` to false plus
`ai.ExecuteRequest.RequireCommandApproval` to true, and
`internal/ai/alert_provider.go` must frame diagnostics as approval-bound
operator actions rather than instructing the model to execute commands because
they appear safe.
Those backend AI and Patrol change summaries should derive their canonical
labels and provenance fragments from
`internal/unifiedresources/change_presentation.go`, so the resource-model
semantics are shared before any surface-specific markdown styling is applied.
The patrol-local recent-change fallback itself should derive its section layout
and change labels from `internal/ai/memory/presentation.go`, so detector-based
fallbacks stay consistent across AI runtime entry points when the canonical
resource timeline is unavailable.
The per-resource intelligence payload returned from
`/api/ai/intelligence?resource_id=...` should also include the canonical
`recent_changes` history so UI and API consumers can read the same timeline
slice that the prompt context uses.
The system-wide `/api/ai/intelligence` summary should also surface the same
canonical recent-change slice, alongside the count, so the aggregate payload
and the prompt context stay aligned on the same shared timeline source.
The frontend Patrol page reads that canonical summary payload through the
shared AI client and store, but only for its overall health; it renders no
summary card and no recent-change slice.
The Patrol runtime now also exports a canonical `runtime_state` alongside
`blocked_reason` in the Patrol status payload, so provider-availability and any
legacy managed-credit block conditions remain part of the governed runtime
contract instead of being inferred later from the last successful patrol
summary.
When missing provider configuration blocks Patrol, `blocked_reason` must point
to Pulse Intelligence > Provider & Models settings and tool-capable Patrol model
selection.
When a provider is configured but its initialization failed — model resolution
can require the provider's live catalog, so Pulse booting before a local model
server loses that race — `blocked_reason` must name the initialization failure
instead of claiming no provider is configured, and the runtime must retry
provider initialization on each scheduled run so a boot-time race strands
Patrol for at most one interval rather than until the next settings save.
That runtime-state contract must be derived from live Patrol runtime inputs,
not only from the last failed run attempt, and the backend must clear any stale
managed-credit block once a provider or local model configuration returns.
The same runtime contract now also governs when the system-wide Patrol health
summary is allowed to read as healthy. `internal/ai/intelligence.go` must not
derive `Health A` or `100/100` from "no active findings" alone when recent
Patrol evidence is limited to alert-scoped runs or includes recent Patrol run
errors; the summary must degrade and explain that overall infrastructure health
is not fully verified until a recent successful full Patrol run exists.
The backend may keep precise full-run, scoped-run, and verification terms in
wire fields and internal decisions, but browser-facing summary and handoff copy
emitted from `internal/ai` should translate that precision into check language:
recent broad checks, targeted checks, follow-up checks, current issues, and
verified outcomes. Runtime explanations must not surface activation-loop,
proof-strip, or scoped/full-run jargon as the ordinary operator vocabulary.
That coverage explanation must also stay faithful to the actual recent run
shape. When the most recent verification evidence includes a full Patrol run
that ended with errors, the health summary must say that a recent full patrol
errored rather than claiming recent activity was limited to scoped runs.
The Patrol status payload must keep that same scope distinction explicit in its
own recency fields. `last_patrol_at` is reserved for the most recent completed
full Patrol run, while scoped runs and fix-verification checks advance
`last_activity_at` without pretending a full verification sweep just happened.
That same runtime contract also owns scoped trigger source policy. Alert- and
anomaly-triggered Patrol work are independent runtime gates; the canonical AI
settings model must preserve them separately, and runtime status must expose
which scoped sources are enabled plus whether queued scoped work or busy-mode
acceleration is currently active. Runtime policy blocks, including the local
development background-automation guard, must be represented as an explicit
event-trigger block on the Patrol trigger status rather than by rewriting the
operator's alert/anomaly trigger preferences to disabled.
That same runtime boundary also owns which Patrol work counts toward
full-patrol cadence gates. Community-tier or other full-run limits must key
off completed full sweeps only; recent scoped or verification activity may
advance `last_activity_at`, but it must not block a manual full Patrol request
as if a scheduled estate-wide sweep already happened.
The manual Patrol route carries that same scoped/full distinction. `POST
/api/ai/patrol/run` (`internal/api/ai_handlers.go` `HandleForcePatrol`) accepts
an optional scope body — `resource_ids` and/or `resource_types`, optionally with
`alert_identifier`, `alert_type`, and `context`. With a scope it runs a manual
Targeted check through the same scoped engine (`TriggerScopedPatrol`) and run
record (`Type: "scoped"`) as automatic alert-triggered work, not a new
investigation path; without a body it keeps the legacy fleet-wide Patrol check
behaviour. A manual scoped run must reuse the existing scoped engine rather
than adding a parallel trigger route, must honour the same Patrol readiness gate
as a full run, and must bypass the full-run cadence gate because targeted
checks never consume an operator's manual full-run allowance. The request must
carry resource identity only — no command, prompt, or remediation payload — and
the route must keep requiring admin plus `ai:execute` scope for both shapes.
The Patrol startup scheduler must preserve that coverage guarantee as well:
`internal/ai/patrol_run.go` may skip the startup full patrol only when recent
run history already includes a successful full Patrol run, not merely because
some recent scoped alert-triggered run exists.
The Patrol runtime also owns synthetic Patrol service findings canonically.
Provider-credit and provider-auth failures raised against the synthetic
`ai-service` Patrol resource are runtime conditions, not inventory resources,
so the full-run seed/reconcile path must not auto-resolve them as
`Resource no longer exists in infrastructure` just because `ai-service` is not
present in the infrastructure snapshot. Those findings stay active until
Patrol actually succeeds or resolves them for a Patrol-owned reason.
That success boundary includes provider-backed scoped Patrol runs and successful
Patrol tool-call preflights. A successful scoped run proves that Patrol can
currently reach the selected provider/model and complete tool-backed analysis;
a successful preflight with an observed tool call proves the configured
provider/model currently accepts Patrol's tool-call path. Either must clear the
synthetic `ai-service` runtime failure just as a successful full Patrol run
does, without loosening ordinary scoped finding reconciliation for
infrastructure issues. Full and scoped runs must account for the same circuit
breaker lease around the actual provider call: successful scoped work records
success, failed scoped work records its classified provider error, and
collection or scope failures before inference never acquire a half-open probe.
A successful current-generation preflight for the exactly configured Patrol
route also resets stale circuit-open state and its corresponding blocked status,
because it is direct tool-call recovery evidence. A successful override for a
different route, a stale preflight generation, or a soft-warning response
without a tool call cannot reset the configured route's breaker.
Because the synthetic runtime finding is seeded into scoped runs as Patrol-wide
health evidence, the run's finding adapter must also expose and accept a
terminal assessment for that finding even though `ai-service` is not part of
the caller-requested inventory scope. This is the sole lifecycle-scope
exception; it does not grant infrastructure action authority, and the existing
manual-action prohibitions remain in force.
The same selected-route preflight is part of the live Patrol readiness gate,
not merely settings-page diagnostics. A completed matching billing, quota,
authentication, connection, unsupported-tool, or timeout failure makes Patrol
not ready and disables new runs; a matching accepted response without a tool
call remains a warning; and a matching observed tool call clears a static
gateway "unverified" warning unless the deeper model-readiness advisor has
stronger contrary evidence. Preflight evidence must match both provider and
bare model ID before it can influence runtime status or UI copy. Any Patrol
transport change clears the cache and advances a generation fence, so an old
slow check cannot finish later and overwrite evidence for the restored or newly
selected route. An interrupted check is not provider evidence and must not
block Patrol by itself.
Because those findings represent Patrol blindness rather than operator-triaged
infrastructure noise, the Patrol runtime must also reject manual acknowledge,
snooze, dismiss, resolve, and suppress actions against synthetic `ai-service`
runtime findings. The canonical recovery path is to correct Patrol provider
configuration in Pulse Intelligence > Provider & Models settings and let Patrol
re-evaluate the runtime condition on the next run.
Runs Patrol never attempts are part of that same finding surface. A scheduled
or scoped run skipped by a persistent readiness blocker or an unavailable
provider must raise the deduped synthetic runtime finding, so an enabled-but-
inert Patrol reaches the shared findings surfaces and alert notification
channels once instead of living only on the Patrol page banner; field
telemetry showed installs recording weeks of empty error runs before anyone
noticed. A transiently open circuit breaker must stay finding-free, because
the attempts that opened it already raised their classified finding.
Turning Patrol off is a Patrol-owned resolution of that synthetic runtime
finding: the disable transition must resolve it rather than leave an
unactionable warning nagging about a state the operator just opted out of,
and infrastructure findings are untouched by that transition.
The shared findings lifecycle must also treat a regressed issue as a new active
occurrence. When a resolved finding reappears, `internal/ai/findings.go` must
clear any stale acknowledgement timestamp from the prior occurrence instead of
carrying that acknowledgement forward onto the regressed active issue. The
same owner must normalize already-persisted active findings on load when a
stored acknowledgement predates the last recorded regression, then persist the
cleaned state back through the canonical findings store.
AI chat tool-name labels, pending-tool headers, and assistant status copy now
also route through the shared frontend identifier-label helper, so the chat
surfaces do not keep their own underscore-stripping behavior separate from
the rest of the governed presentation helpers.
AI chat stream matching and mention dedupe now route through the shared
frontend chat identifier helper, so tool-name prefix stripping and mention-key
normalization stay aligned across the chat runtime instead of being redefined
inline in the stream processor or container component.
That same provider-stream boundary also owns EOF-safe SSE finalization for
OpenAI-compatible chat streams. Provider reads that return payload bytes with
`io.EOF`, or close immediately after the final `data:` frame, must still
process the buffered frame set and route tool-call assembly plus final done
event emission through the same canonical finalizer used for `[DONE]` instead
of dropping the last chunk or leaving tool calls unfinalized on clean close.
Patrol detection and investigation streams under that boundary must also carry
an explicit 60-second inter-chunk stall allowance, capped by the operator's
configured provider request timeout. Interactive Assistant streams retain the
short default stall bound. A compatible provider pausing between streamed
reasoning and its next tool-call chunk must not invalidate an otherwise live
Patrol run, while a genuinely stalled stream must still terminate within the
Patrol run budget.
Non-interactive Patrol turns may replay one provider stream that closes before
its completion marker, even when the provider emitted a partial fragment,
because no tool call or durable turn exists until the terminal marker arrives.
Interactive Assistant turns retain the no-replay rule after visible output.
When a Patrol investigation has already gathered its evidence but its final
tool-free response reaches the provider output limit, core must discard the
partial conclusion and allow one bounded, tool-free, low-reasoning synthesis
turn over the existing transcript. That recovery may only produce the required
investigation sections; it cannot gather more evidence or reopen mutation
authority, and a second truncation fails the investigation closed.
Qualification scoring must keep synthetic Patrol runtime findings on the
`ai-service` resource separate from model-authored infrastructure findings.
Provider/runtime failure remains an unconditional qualification hard failure,
but its synthetic service finding must not be reported as an infrastructure
false positive or matched to scenario ground truth.
Qualification cost gates must resolve the configured provider route rather
than silently borrowing a direct-provider family price. Reviewed OpenRouter
routes use exact model IDs and model-specific review dates from the public
OpenRouter Models API; aliases, fast variants, routers, and other unreviewed
routes remain unknown and therefore fail any non-zero qualification cost
budget until their actual route price is explicitly recorded. An exact
OpenRouter `:free` route may be recorded at zero only after that route's public
catalog entry is reviewed; the suffix alone never makes other free aliases
known. The reviewed zero-priced qualification routes are currently
`nvidia/nemotron-3.5-lightning:free` and
`nvidia/nemotron-3-super-120b-a12b:free` (reviewed 2026-08-14), plus
`nvidia/nemotron-3-ultra-550b-a55b:free` (reviewed 2026-08-15). Live scoring
uses the provider resolved by Patrol readiness, even when the configured
OpenRouter model is an unprefixed slash route such as
`anthropic/claude-sonnet-5`; scorer replay persists and reuses that resolved
provider so the same captured report prices deterministically.
First-party Anthropic cost estimation is version-sensitive even within one
model family. Standard API calls for Opus 4.5, 4.6, 4.7, 4.8, and 5 use the
reviewed $5 input / $25 output per-million-token rates, while Opus 4, 4.1, and
3 retain the legacy $15 / $75 rates. Haiku 4.5 similarly uses $1 / $5 rather
than the Haiku 3 fallback. Sonnet 5 uses $2 / $10 rather than the prior
Sonnet-family $3 / $15 rate, while Fable 5 and Mythos 5 use $10 / $50 and must
remain priced rather than making the complete cost summary unknown. These
version-specific rows must precede broad family fallbacks because pricing
lookup is first-match prefix-based, and dated model aliases must resolve to the
same row as their base version. The 30-day runtime budget recalculates retained
usage through this table, so a stale higher fallback must not disable Assistant
or Patrol before estimated first-party spend reaches the configured limit, and
a current priced model must not bypass enforcement by falling through as
unknown.
After a non-interactive Patrol finding lifecycle write succeeds, the structured
tool result is authoritative and investigation remains closed. Watch first
offers a bounded lifecycle-completion request containing only report and
assessment tools so providers may record independent findings sequentially
without repeating accepted calls. A tool-free response is the operator-facing
summary. Mixed-success repair remains a separate single repair-only extension,
and a write on the final allowed turn falls back to the bounded text-only
summary system instruction. Neither completion nor summary may invent findings,
evidence, actions, verification, or remediation.
Docker dependency qualification fixtures must use only executables present in
the catalogue's pinned image and must reach a healthy dependency/client
baseline before fault injection. The Watch correlation and Pro investigation
fixtures use Alpine's `nc` applet for the disposable HTTP dependency; a missing
optional daemon must fail baseline convergence and never be scored as a model
miss.
Repository Actions may validate the Patrol catalogue and replay recorded
qualification evidence only on a fresh GitHub-hosted runner. Live and release
qualification require private-lab credentials, fault authority, and reports
that can contain private resource identity, so they must run directly from a
disposable canary-lab host at a pinned Pulse revision, never from a persistent
self-hosted Actions runner. Keep raw artifacts outside the working tree in an
access-controlled root with checksums, and execute the reviewed Watch scenarios
sequentially so model overrides and run association cannot race.
Governed Docker remediation qualification must not define a deliberate
container stop as a required Watch finding. Patrol intentionally suppresses an
ordinary stopped workload unless independent evidence establishes that it was
expected to remain online and failed, so treating every runner-issued stop as
ground truth would contradict the product's noise policy and make model scores
depend on an ambiguity hidden from the model. The approval and rejection
canaries instead keep a representative service running, stop a driver-owned
internal health sentinel, require the normal collection path to expose the
resulting failed health check, and score an exact typed `restart` action.
Approval must independently prove that restart restores running/healthy state;
rejection must independently prove the service remains running/unhealthy with
no execution event until runner-owned teardown. The injector command is fixed
inside the Docker lab driver rather than supplied as manifest shell text.
Live qualification collection convergence must require both exact scenario
resource names and the scenario-owned Docker oracle state projected by Pulse's
canonical resources API. A direct lab oracle may prove the fault exists, but
Patrol must not run until the normal agent/API path independently exposes the
relevant health, running-state, or restart-count condition; a stale healthy
projection is a collection failure, never a model miss. These expectations are
derived from manifest oracles before execution and never from Patrol tool
selection, preserving non-circular scoring.
Healthy negative controls use the same convergence rule in the opposite
direction: their manifest-owned baseline predicates must be visible through
the canonical resource projection before Patrol starts. A transient `starting`
projection cannot force the model to spend a confirmation turn after the lab
oracle has already established `healthy`.
For a fault-free Watch scenario with an explicit healthy negative control, a
tool-free all-clear is the desired product outcome and must not be invalidated
merely because no ceremonial lifecycle read was persisted. That narrow
qualification shape sets `require_tool_call_evidence=false` and instead
requires positive persisted input/output token usage plus non-empty model
analysis, alongside the normal zero-false-positive, exact-collection, safety,
and teardown gates. Faulted Watch cases, existing-finding reconfirmation,
investigation, and remediation continue to require persisted tool-call
evidence. Manifest validation must reject a tool-free exception outside the
fault-free Watch negative-control shape.
Watch qualification must score the confirmed provider-visible symptom rather
than require an unproved causal attribution. In the correlated Docker scenario,
the stopped dependency remains the independently injected fault and the
downstream unhealthy client is the required Watch finding; one finding proves
symptom deduplication without teaching Watch that every stopped container is
unexpected. Pro investigation separately owns the causal diagnosis and its
evidence requirements. Ground-truth v2 therefore records the injected target
and the independently declared expected-finding resource as separate resolved
identities; an expected resource may differ only when the manifest declared it
as related before execution. V1 replay artifacts retain target-as-expected
semantics.
Pro cross-resource qualification must keep the initial trigger scope separate
from the lab's collected/oracle resource set. A reviewed `scope_resources`
alias list narrows only the Patrol trigger; it never removes related resources
from collection or ground truth. Scenario-owned `root_cause_resources` and
`affected_resources` aliases are resolved before execution and scored against
the exact `Root Cause` and `Affected Resources` response sections using the
collected canonical name or ID. The expected diagnosis therefore remains
independent of the read-only tools Patrol chose, and merely repeating the
downstream symptom cannot pass causal grounding.
An existing-finding reconfirmation scenario may seed its scored run only from
a successful prerequisite Patrol run that created a run-owned finding on every
scenario-owned expected resource. An incidental provider, readiness, or other
unrelated finding cannot satisfy setup, and the failed prerequisite remains in
the report as diagnostic evidence rather than being converted into a model
reconfirmation miss.
Scenario-owned `required_summary_term_groups` may declare reviewed semantic
alternatives for one independently known fact, such as Docker's injected
`stopped` action and its normally collected `exited` state. Every group still
requires one match, but scoring must not fail an otherwise exact causal
diagnosis merely because the model used the provider-visible equivalent rather
than the injector's verb. These alternatives are fixed in the manifest before
execution and never inferred from the model response or its tool path.
An autonomous remediation scenario uses the explicit `await_autonomous`
decision. It requires effective Full autonomy (the manifest's legacy
`autonomous` spelling aliases to `full`), and the qualification client sends no
approval decision and no execute request. It waits for the independently
created Patrol action to reach its governed terminal lifecycle, then checks the
required lifecycle-verification state and independent postconditions. The
separate operator authorization to run a disposable fault scenario permits the
fault injection only; it is not action approval. Only this exact autonomous
shape may allow the declared fault to be repaired before the post-Patrol
observation, while unexpected mutation and inventory-restoration gates remain
mandatory.
The canonical `pulse_query` schema and executor must admit the same resource
types. In particular, `action=get` accepts `docker-host`, returns a governed
read-only host response when the identity resolves, and returns a successful
typed `not_found` result when it does not; a model following the advertised
schema must not incur a failed tool call because the executor used a narrower
legacy type switch.
Scoped Patrol prompt context must describe canonical/source IDs and aliases as
identity aliases, not count each value as a distinct infrastructure resource.
The exact scoped inventory is the authoritative resource set. Patrol core must
load the complete exact-scope active-finding snapshot before provider execution
and the model must reuse the seeded result for all lifecycle decisions. The
normal Watch provider manifest omits `patrol_get_findings`, avoiding a
deterministic bookkeeping call and its extra or failed provider turn on both
healthy and faulted runs.
On a quiet deterministic triage with a current exact scoped inventory showing
the scoped resources running and healthy, no container restart evidence, and no
active alerts or findings, the model uses the supplied snapshot as sufficient
calm-day evidence and does not spend platform or inventory tool calls merely
reconfirming the same state. Exact scoped app-container rows carry the canonical
restart count from normal collection. A non-zero count is a concrete signal
that prevents the calm-day shortcut and may justify one targeted current
resource read. A provider-observed count of at least three deterministically
flags repeated exits as model context. The model may report that grounded
reliability warning even when the sampled lifecycle state is `running`, but it
must not call the container an active restart loop from that snapshot alone. A
current `restarting` state or a count that increased from the scoped snapshot
confirms the active-loop symptom. The model does not spend logs, discovery,
Docker-service, or other root-cause tool calls after the repeated-restart
symptom is established; causal analysis belongs to the separate Pro
investigation track. Quiet triage is not a deterministic replacement for
model-owned assessment.
High-confidence deterministic lifecycle flags must nevertheless retain a
model-owned decision opportunity before Watch completes. Health, reliability
(including provider-confirmed repeated restarts), backup, connectivity, and
learned-anomaly candidates join the existing unmatched signal evaluation floor
after the main agentic pass; ordinary CPU, memory, and disk threshold crossings
stay with Pulse alerting and do not trigger a duplicate Patrol decision pass.
Candidates already covered by a model-authored finding are skipped, while at
most twenty remaining candidates, prioritized with direct lifecycle failures
before learned anomalies, receive one bounded evidence-only model pass. Pulse
must not translate a flag directly into a finding or choose remediation; this
floor exists only so unrelated existing-finding lifecycle work, a provider's
sequential tool calls, or a tool-free conclusion cannot silently consume the
sole decision opportunity for a confirmed scoped symptom.
Model-authored structured findings remain concise for continuous operation:
description uses at most three short sentences, impact one sentence, evidence
three concrete facts, and recommendation two short sentences, without
repeating the same state or caveat across fields. Concision must preserve the
exact evidence needed to verify the finding rather than replacing it with a
generic summary. The provider-facing `patrol_report_finding` schema and its
runtime handler both require non-empty evidence and a safe, actionable
recommendation for every model-authored finding. A bounded investigation or
verification step is a valid recommendation when remediation is not yet
justified; impact remains optional so the contract never pressures the model
to fabricate a consequence. Providers that omit either grounding field must
receive a tool error and no partial finding may be persisted.
The normal Patrol detection provider manifest exposes no finding-lifecycle
read: core has already established and seeded the exact-scope snapshot before
the first provider request. `patrol_get_findings` remains a registered internal
compatibility capability for narrow adapters and tests that cannot preload the
snapshot, where it is one-shot. The general `pulse_alerts` surface remains
available to interactive Assistant sessions but is withheld from Patrol even
when every monitored subsystem is enabled. This prevents a provider from
combining overlapping lifecycle and Alerts read contracts. The Patrol adapter
returns only findings inside the run's resolved caller-requested finding scope,
never dismissed history or context-only dependency records from the broader
effective evidence scope.
The provider schema is also the source of truth for the required-argument
checklist rendered into Patrol's normal and bounded final-decision prompts.
Every report call must be independently complete, including parallel calls for
several findings; model-facing guidance must not maintain a second hand-written
field list that can drift from the registered tool schema. Runtime recovery
after a rejected call remains allowed so Patrol can safely finish the user
outcome, but formal qualification continues to reject every unsuccessful tool
call, including a recovered schema-validation lapse. Recovery evidence is a
useful product diagnostic, not permission to retroactively relabel a published
run or weaken protocol reliability as a launch gate.
Infrastructure values remain untrusted across both the full Patrol prompt and
the bounded post-finding summary turn. Model-authored analysis, findings, and
operator summaries must not quote, reproduce, or closely paraphrase embedded
instructions, prompt-injection payloads, canary markers, or secrets from
names, labels, annotations, logs, command output, discovered metadata, or tool
results. When relevant, the model may state only that untrusted metadata was
ignored, without repeating its content.
Provider-facing `pulse_docker` calls are host-scoped: every advertised action
requires a Docker host name or ID in the structured schema, matching the
executor contract and preventing invalid hostless Swarm-state calls.
Persisted Patrol tool inputs must retain complete structured finding calls up
to the bounded 16 KiB record limit so normal evidence-rich findings remain
deterministically replayable. If any captured input is nevertheless incomplete
or malformed, report generation must preserve the report, replay diagnostic,
Markdown, and checksums, mark the run failed, and refuse to present that capture
as executable deterministic replay.
That same provider-transport boundary owns OpenAI-compatible tool protocol
adaptation. Pulse must keep normal tool selection automatic/model-owned for
OpenAI-compatible providers, including direct DeepSeek paths. Text-only
turns reached through loop, budget, or verification gates should omit tools
entirely instead of sending provider-specific `tool_choice=none`; that transport
setting must not be used as an intent classifier.
Reasoning-backed provider turns that return tool calls with `reasoning_content`
must preserve that reasoning state on the following tool-result turn when the
provider requires it, so Assistant and Patrol can complete multi-turn tool use
against live BYOK providers.
Readiness classification for the same provider path must be model-aware, not
provider-only. Current official DeepSeek V4 tool-capable models may report
Patrol readiness as ready; legacy DeepSeek aliases may only warn with the
alias-retirement posture and a recommendation to select the current V4 model
IDs; unknown direct DeepSeek model IDs must be not-ready with
`model_unavailable`; and known reasoning-only families must continue to fail
closed before Patrol work is admitted.
The explicit Patrol model advisor must keep provider/model transport health
separate from Patrol capability evidence. Successful model-list connectivity
sets `transport_healthy`; only the typed streaming/buffered tool protocol,
context fixtures, continuation and latency checks set `patrol_capable`.
Ordinary Assistant success plus missing Patrol tools is a warning and
`not_suitable` Patrol mode, not a provider connection failure. Patrol action
admission remains fail closed. Local Ollama and custom OpenAI-compatible health,
catalog and preflight operations honor the configured request timeout so cold
model loads are not truncated by the hosted-provider 30-second check budget.
Advisor probes must pin the runtime they are testing rather than inherit
provider server defaults: a runtime context window sized to the haystack
fixtures (forwarded as `num_ctx` on Ollama and clamped to the model's trained
window), an explicitly forwarded temperature 0, a generation budget that
absorbs thinking-model reasoning, and the same inter-chunk stream stall
allowance as the Patrol loop the advisor certifies. Adapters must forward an
explicitly pinned zero temperature and synthesise tool-call IDs when a
compatible server omits them (llama.cpp) instead of failing tool protocol on
transport shape. Probe and validator errors, including the provider stop
reason, must be logged and surfaced in the readiness result's `details`
evidence rather than discarded, and a transport-level probe failure keeps its
specific diagnosis instead of the generic capability wording.
That same browser-owned chat read model must keep target normalization helper-
driven. Assistant shells may still derive legacy VM identifiers or display
labels for read-only targeting, but they must do so through shared helpers and
store context precedence instead of passing component-local resource objects or
duplicating naming fallbacks inline.
That same runtime boundary also owns executor session isolation. Shared AI
runtime services may reuse one canonical executor configuration, but each chat
or Patrol run must clone that executor before attaching resolved-context,
approval-routing, or patrol-finding state so concurrent sessions cannot
overwrite one another's mutable runtime context.
That same Patrol runtime boundary owns Community monitor-mode autonomy saves.
The open-source/free `PUT /api/ai/patrol/autonomy` adapter may persist
findings-only `monitor` configuration and the governed investigation budget /
timeout clamps, but it must continue to reject `approval`, `assisted`, and
`full` autonomy with the canonical safe-remediation license response.
Every successful autonomy mutation must publish the persisted investigation
budget and timeout to the already-wired tenant investigation orchestrator
before a newly triggered finding can be admitted. Requiring a server restart,
or leaving the orchestrator on its construction-time defaults while the API
reports the new values, is a contract violation: it silently turns the
operator's model-led investigation allowance into a smaller framework-owned
ceiling. A running investigation snapshots its limits when execution starts,
so later settings changes affect subsequent runs without racing or rewriting
an in-flight run.
The same canonical findings store owns dismissal-reason semantics. The three
`dismissed_reason` values must remain behaviorally distinct, not copy-only
variants: `not_an_issue` flips `Suppressed=true`, `expected_behavior`
acknowledges without escalation, and `will_fix_later` is an operator
commitment that populates `Finding.RemindAt` (default
`DefaultWillFixLaterRemindAfter`, 7 days). On re-detection, the canonical
store wakes a `will_fix_later` finding once `RemindAt` has passed by
clearing the dismissal and emitting a `reminded` lifecycle event, and the
`dismiss_finding` LLM tool response must communicate the remind-at date so
Patrol's conversational explanations stay aligned with the persisted
behavior.
The unified-finding mirror in `internal/ai/unified/alerts.go` also carries
that same `RemindAt` field so the API surface preserves the will_fix_later
wake-up deadline across the canonical findings store and the read model.
The `AddFromAI` dedup-merge path must mirror `RemindAt` onto the existing
record (including clearing it when a remind-at wake or undismiss has
already cleared the dismissal in the canonical store), and the TS API
clients in `frontend-modern/src/api/patrol.ts` and
`frontend-modern/src/api/ai.ts` must round-trip the `remind_at` field
verbatim so the operator surface can preview and badge the deadline.
The same Patrol API client also exposes the operator-driven manual
resolve path. `resolveFinding(findingId)` in
`frontend-modern/src/api/patrol.ts` must POST `{finding_id}` to the
canonical `/api/ai/patrol/resolve` endpoint owned by
`HandleResolveFinding` in `internal/api/ai_handlers.go`, mirroring the
acknowledge / snooze / dismiss client surface so the same Patrol service
contract drives every operator-feedback action.
The `unified.UnifiedFinding` mirror also carries an explicit
`AutoResolved` flag alongside `ResolvedAt`, set by the canonical
`Finding.AutoResolved` field. The AddFromAI dedup-merge path must
mirror that flag (allowing flips between auto-detected closure and
operator-driven closure as the canonical store transitions), and the
Finding to UnifiedFinding conversion in `internal/api/router.go` must
copy `f.AutoResolved` on both the live wire-up callback and the
persistence-recovery resync, so the frontend can honestly attribute who
closed the loop instead of flattening every resolution into a generic
"resolved" state.

The same canonical AI runtime now also owns the report-narrative surface.
`internal/ai.Service` implements `pkg/reporting.Narrator` and
`pkg/reporting.FindingsProvider` directly through `report_narrator.go` and
`report_findings.go` so the reporting engine can request an AI-generated
executive summary without depending on AI-internal types. The narrator is a
single-turn, no-tools call that reuses the canonical provider abstraction
already powering Patrol and Assistant; the request sanitizer, model
selection (`PatrolModel` preferred, falling back to `GetChatModel()`), cost
budget enforcement (`enforceBudget("report_narrative")`), and provider
factory must be the same shared seams used by `QuickAnalysis`, not a
parallel report-only provider stack. The structured report payload sent to
the model is denormalised through `buildReportNarratorPayload` so reporting
package types do not leak into the prompt surface, and the response is
parsed through `parseReportNarratorResponse` which tolerates an optional
`json` code fence the model may emit despite the no-fences instruction.
Severity normalisation maps the model's free-form output back onto the
narrative bullet severity set the renderer understands (`ok`, `info`,
`warning`, `critical`); unknown values default to `info` rather than
silently rendering as muted. Both interfaces fail closed: a nil provider,
parse failure, empty narrative, or context cancellation returns an error
so the reporting engine falls back to the deterministic heuristic
narrator. The findings provider filters the patrol findings store by
resource ID and lifecycle overlap with the report window via
`findingOverlapsWindow`, and truncates to `reportFindingsLimit` (25)
sorted entries so retrospective summaries stay within a predictable
prompt budget. Reporting is therefore an additive consumer of AI
runtime, not a new ownership boundary, and the narrator/findings
surfaces inherit the same governance the rest of the canonical AI
runtime already enforces.
Report narration is caller-requested spend, so on the single-resource HTTP
route only `POST /api/admin/reports/generate` reaches it (fleet reports, from
`POST .../generate-multi` or a report schedule, use the fleet narrator). The
reporting narrator resolver goes through `AISettingsHandler.GetAIService`,
which constructs a tenant service on first use and can then list provider
models or start background discovery. A GET single-resource report never calls
that resolver:
it narrates deterministically and reads Patrol findings through
`AISettingsHandler.ExistingAIService`, which returns only a service that is
already running and never constructs one
(`TestAISettingsHandler_ExistingAIServiceNeverConstructs`, and through the
router's real wiring `TestContract_SingleReportGETDoesNotConstructTenantAIService`).

The same canonical AI runtime now also owns the fleet-level report
narrative through `report_fleet_narrator.go`. `Service` implements
`pkg/reporting.FleetNarrator` with its own use-case label
(`report_narrative_fleet`) so fleet vs single-resource spend is
distinguishable in the cost ledger, and so the budget gate
(`enforceBudget`) and dashboard taxonomy can address the two
separately. The fleet payload is denormalised through
`buildReportFleetPayload` into compact per-resource rows plus a
fleet-wide aggregate so prompt cost scales linearly with fleet
size without exploding token usage. The same fail-closed invariant
holds: nil provider, parse failure, empty narrative, or context
cancellation returns an error so the reporting engine falls back to
`HeuristicFleetNarrator`. Single-resource report narration is
deliberately not propagated through the multi-report path; a
50-resource fleet report performs exactly one AI call (the fleet
narrator) rather than 51 (one per resource plus a fleet-level
summary).

Both the single-resource and fleet narrator system prompts also
encode an explicit detection-boundary invariant: Pulse Patrol is the
canonical detection layer, and the report narrators must function as
summarizers of Patrol's classified state rather than parallel
detectors. The narrator may classify an observation at "warning" or
"critical" severity only when it is backed by a Patrol finding, an
alert, or a hard-threshold breach in the structured input (cpu max
above 90, memory avg above 85, disk avg above 85, failed or
high-wear disks, storage pools at 90 percent or more). Patterns it
notices in the metric data without that backing may be mentioned at
"info" severity but must not be promoted. Recommendations follow
the same rule: no remediation for inferred issues that lack a
finding, alert, or threshold breach. This keeps the report
narrative honestly retrospective on Patrol's work and prevents
silent shadow-classification competing with Patrol's detection
rules.

`pulse_summarize` is the model-facing retained-metric evidence tool. It uses
`pkg/reporting.MetricEvidenceProvider`, implemented by `ReportEngine` in
`pkg/reporting/evidence.go`, to read the same retained store as reports without
invoking a narrator. The capable model in the active conversation owns synthesis.
Report-only narratives and PDF health cards are separate reporting behavior,
not evidence of machine health or a substitute for an investigation.

The tool is self-targeting: `action=fleet` with `resource_ids`
omitted enumerates the known fleet from the executor's unified
resource provider itself (infrastructure parents first, then
guests, then storage; deduped; bounded by the same 50-resource cap
as the API, with an explicit truncation note in the response)
instead of erroring. This exists because the natural first-session
question — "how is my machine doing?" on a fresh install with a
small local model — produced a `resource_ids is required` failure
that the agentic loop converted into a jargon elicitation asking
the operator for "comma-separated resource IDs" (the GitHub
discussion #1042 funnel-killer). Both modes also resolve the
references models actually pass: canonical unified IDs and plain
resource names (unambiguous, case-insensitive) are translated onto
the reporting request shape the way the API report path's
`resolveReportSubject` does — the canonical ID stays the request
`ResourceID` (Patrol findings and recovery points key on it) while
the resolved metrics target rides `MetricsResourceID`, so the
engine's store queries actually find data instead of silently
returning zero points. Reporting types are classified from the
unified resource, with the canonical metrics target taking precedence.
Agent-backed hosts use `agent`, pure Proxmox nodes use `node`, and
pure Docker hosts use `docker-host`. The Proxmox storage-family
correction belongs to the unified target, so summary tools no longer
carry their own exception. `resource_type` is now an
optional filter/default for fleet mode and only required for
`action=resource` when the identifier is not a known resource.
Every remaining error path in the tool tells the model to
enumerate or retry and explicitly forbids asking the operator for
resource IDs. `TestSummarizeTool_FleetEnumeratesWhenIDsOmitted`,
`TestSummarizeTool_FleetResolvesNamesAndTranslatesMetricsIDs`, and
the compile-time `resourceMetricsTargetResolver` pin on the monitor adapter
hold this behavior.

Both modes return statistics with units, retained point counts, first/latest
observation timestamps and the largest gap between returned points. Means are
unweighted means of retained values, which may themselves be retention
aggregates. Extrema retain bucket minima/maxima. Neither the requested range
nor the first-to-last span proves continuous coverage. Empty metrics remain an
empty evidence map. Store errors and unavailable evidence capabilities return
errors, never a healthy verdict or heuristic fallback. Fleet reads fail visibly
if a selected resource query fails rather than silently dropping that resource.

The response explicitly names alerts, findings, disk health, backups and topology
as not queried. Models must use their corresponding tools to collect those
sources. The evidence tool does not emit health labels, scored outliers,
recommendations or a second model's conclusions. Report narrator dependencies
remain available for report generation but are not invoked by `pulse_summarize`.
`TestSummarizeToolReturnsHighUtilizationAsEvidenceWithoutDiagnosis` verifies that
92% memory remains a measured reading and is not labelled critical or healthy.
`TestMetricEvidenceRetainsObservedCoverageAcrossRestart` verifies retained
coordinates, sparse timestamps, statistics, empty results and query failures.

Retained metric evidence is organization-scoped. Every `pulse_summarize`
request carries the executor's organization as `MetricReportRequest.OrgID`, and
the server's report engine resolves that organization's own metrics store
(`EngineConfig.MetricsStoreForOrg`), refusing a request without one
(`ErrReportOrgRequired`) rather than reading the default organization's store.
A tenant's Assistant therefore summarizes only its own organization's history,
even for a resource ID that also exists in another organization.
`TestSummarizeToolReadsOnlyItsOrganizationsMetrics` and
`TestReportEngineReadsOnlyTheRequestOrganizationsMetrics` hold this.

The same canonical AI runtime now also records user-chat token
usage to the cost ledger. `chat.Service.ExecuteStream` was a
long-standing gap: the agentic loop accumulated token counts via
stream callbacks and surfaced them in the SSE done envelope, but
nothing on the server side recorded a `cost.UsageEvent`. Chat is
the bulk of AI token spend, so the operator AI usage dashboard
was understating cost dramatically. `recordChatTurnCost` now runs
after every `loop.ExecuteWithTools` return — success or error,
since the operator was billed regardless of whether the loop
produced a clean response. It emits a `cost.UsageEvent` with
`UseCase="chat"` in the same shape the rest of the runtime uses.
When a chat turn carries a product-originated finding, resource, handoff,
action, or structured-mention context, the event may also include the coarse
`context_scope` classifier so privacy-safe telemetry can count governed-context
Assistant collaboration without exporting prompts, session IDs, resource IDs,
finding IDs, command text, or action payloads.
When the same turn includes accepted model-selected governed tool calls, the
agentic loop records only the total count on the resulting cost event. That
count supports Pulse Intelligence adoption telemetry for Assistant
collaboration while keeping tool names, arguments, results, provider call IDs,
transcript content, resource IDs, finding IDs, command text, action payloads,
and session IDs out of persisted/exported telemetry.
The store is threaded through `chat.Config.CostStore`, wired by
the router from the per-tenant `AISettingsHandler.GetAIService`
via `Service.CostStore()`. `ExecutePatrolStream` deliberately
does NOT record here — its caller (`patrol_ai.go`) records via
its own helper, so cost is never double-counted on the
patrol-via-chat path.

Patrol action continuity is audit-authoritative after proposal submission.
Investigation execution exposes run failure and proposal failure as independent
typed channels; a proposal-only failure may become a needs-attention outcome,
but a simultaneous provider/runtime failure must remain a failed investigation.
Before the model runs, the enterprise orchestrator receives the exact broker
capability catalog, including approval floor, parameter types, enums, patterns,
and sensitivity, and must explicitly forbid proposal guessing when catalog
lookup fails or returns no capabilities. Once `ActionBroker.Submit` succeeds,
action-transition callbacks are wakeups only: `internal/api/patrol_action_reconciliation.go`
re-reads the authoritative action audit, projects its current `ActionReference`
onto both the investigation and finding, and maps terminal verification onto
`fix_verified`, `fix_verification_failed`, or `fix_verification_unknown`.
Investigation reads perform the same hydration by action id or trusted origin,
so a missed callback cannot strand Patrol on stale approval state. Finding
lifecycle publication is idempotent and emits unified lifecycle updates plus
honest terminal push outcomes without turning unverified execution into an
all-clear.
Policy-authorized submission is still audit-authoritative. Missing policy
state, provider errors, closed/out-of-window resource policy, unsupported
eligibility, or a Patrol mode below the capability's class leave the action
pending rather than converting a valid proposal into an investigation failure.
An idempotent resubmission returns the existing action disposition and must not
execute a terminal action twice.

Operator review reads a server-computed current-readiness projection, and the
shared lifecycle refuses a Patrol approval before persistence when the exact
plan has expired, drifted, lost executor reachability, encountered an operator
lock, or is stopped by current policy. An expired or drifted Patrol plan can be
refreshed only through the shared lifecycle: the replacement gets a new action
identity, retains trusted finding/investigation evidence links, and recomputes
current tenant/resource policy authorities under the fixed Patrol service
actor. The deciding human cannot supply or downgrade those broker-owned
fields, and no prior approval transfers to the replacement.

### Canonical mutation registry boundary

Assistant infrastructure mutations are classified by the generated
`internal/mutationregistry` registry before handler execution. The only active
model-originated mutation is `pulse_control type=resource`, which submits a
typed capability request to the shared action-lifecycle planner and never
contacts infrastructure directly. Retired tool names and compatibility aliases
cannot be registered or shadowed by extensions. Docker update/control,
Kubernetes scale/restart/delete, arbitrary exec, raw command, and file writes
are omitted from offered schemas and denied if fabricated.

The Pulse self-update administrative exception `admin.pulse.self-update`
covers `POST /api/updates/apply`, `/api/updates/rollback` and
`/api/updates/pro-runtime/apply`. The runtime-switch route is the same
admin-only manager apply pipeline as an update, with no model-originated,
Assistant tool or agent transport path.

The registry audits enumerate actual registered tool discriminator values and
bind mechanically discovered API, job, and transport candidates to one registry
disposition. The action lifecycle API surface registers five lifecycle
entries — `action.api.plan`, `action.api.decision`, `action.api.execute`, and
`action.api.force-fail`, plus `action.api.refresh` — all executed by
`internal/actionlifecycle.Service` with committed-lifecycle delivery.
`action.api.force-fail` is the operator
override that terminalizes a dispatched action stranded without agent
completion evidence; it shares the `execute_action` capability and admin
approval floor with execute, writes terminal inconclusive audit truth through
the same completion path, and never carries transport authority of its own. Transport lifecycle entries must name non-transport lifecycle
authority and committed authority before delivery. Task 07 owns durable
delivery/reconnect and Task 10 owns terminal truth/compensation; incomplete
Kubernetes/native-provider, delivery, and rollback paths remain
`retired_denied`, not parallel executors. Docker container image updates left
the incomplete set: `resource.docker.container-update` and
`transport.agent.docker-container-update` are lifecycle-dispositioned over the
typed `docker_container_update` operation with durable receipts and declared
backup/rollback compensation, while the model-originated
`assistant.docker.update` route and the legacy queued update transports stay
`retired_denied`.
Proxmox guest lifecycle dispatch has the same closed pairing:
`transport.agent.proxmox-guest-lifecycle` requires prior durable
`resource.proxmox.guest-lifecycle` authority, while the lifecycle result is
non-admitting evidence and cannot create dispatch authority.

Enterprise command-remediation records are readable historical imports only.
Production code contains no command or rollback execution algorithm; exported
approve/execute/rollback interfaces and HTTP endpoints are permanently inert
even when a command executor is injected.

The agent transport catalogue now classifies wire roles independently from
mutation identity. `docker_container_lifecycle` is the typed mutation request
for the existing `resource.docker.container-lifecycle` capability and may be
sent only after committed action-lifecycle authority.
`proxmox_guest_lifecycle` is classified by the corresponding
`transport.agent.proxmox-guest-lifecycle` entry and may carry only the existing
`resource.proxmox.guest-lifecycle` durable authority; its result message is
non-admitting.
`agent_operation_query` is query-only reconciliation;
`agent_operation_query_result` and `docker_container_lifecycle_result` are
non-admitting receipt/result messages. Query, result, receipt, and general
protocol roles are forbidden from carrying a mutation registry id or durable
authority reference, so response-shaped lookalikes cannot become dispatch
entry points. Task 10 remains the sole owner of `ActionResultV2` execution,
verification, evidence, and compensation truth.

Action planning and approval attribution are now server-owned across Assistant,
Patrol, and MCP projections. Trusted brokers use explicit service/policy actor
contexts; public `requestedBy` content cannot become audit authority. The agent
manifest advertises the granular `actions:plan`, `actions:approve`, and
`actions:execute` scopes while the server alone owns the bounded legacy-scope
compatibility window. AI-facing method labels, model assertions, and local
biometric claims never satisfy an MFA floor. Until the core step-up verifier
accepts action-bound cryptographic evidence, the honest runtime outcome is
step-up unavailable, not MFA approved.

The legacy Patrol approval bridge must project stored approvals into that same
server-owned model before a lifecycle decision is appended. It binds a scoped
service requester, the deciding human `ActorBinding`, and method/session
`ApprovalEvidence` to the exact action, plan hash, outcome, organization, and
decision time. A zero-version legacy approval requirement is upgraded to the
current canonical floor before evaluation; missing or inconsistent authority
fails closed. A dry-run plan is never executable and must persist refusal
without minting decision approval authority.

Patrol action proposals persist the exact bounded policy authorities consulted
at planning time through the canonical unified-resource `policyDecision`
object. Capability, tenant Patrol, and resource operator factors retain typed
source revisions and reason codes; unavailable and absent sources remain
explicit rather than becoming human-authored labels. Planning and dispatch
call one shared pure Patrol evaluator over separately fetched current inputs.
The plan snapshot is descriptive context for Task 11 and is never passed back
as automatic authority; Task 04 dispatch admission still re-fetches policy and
issues the only executable lease.

AI and Patrol projections consume `unifiedresources.ActionResultV2`; they do
not own action-result enums. Finding reconciliation maps confirmed,
contradicted, and inconclusive verification independently from execution, and
therefore cannot treat terminal audit state as proof of the postcondition.
Legacy investigation outcome strings remain projections for compatibility and
must be derived from canonical truth rather than reconstructed locally. The
typed disposition retains the serialized canonical two-axis result so the
legacy single outcome is never the sole durable/read-model truth.

### Server-owned Patrol Autopilot activation

Patrol full mode is now an effective server decision, not the stored
`patrol_full_mode_unlocked` compatibility boolean. The runtime evaluates a
versioned acknowledgement and activation bound to one human actor credential
and organization; missing, stale, expired, revoked, malformed, cross-tenant,
or API-token evidence falls back to approval mode. Acknowledgement creation is
separate from activation, lower modes clear activation without rewriting
history, and version rotation or revocation takes effect before new Patrol
admission through the existing Task 04 policy-mutation coordinator. The API
handler and every tenant AI service share the same injected server clock and
current-version provider, so a newly supported contract cannot activate at the
API boundary while remaining stale in the runtime that enforces Patrol mode.
When that API gate consumes an acknowledgement and normalizes the downstream
paid-handler request, it must preserve omitted investigation budget and timeout
fields as omitted so a mode-only activation cannot reset existing controls.

The accepted limits promise only policy allowlisting, emergency-stop and
approval-floor enforcement, reconciliation when supported, disclosed evidence
class, and honest inconclusive outcomes. They do not promise independent
verification, and execution success remains separate from outcome truth.
Task 11 still owns the explicit acknowledgement UX and browser/device proof;
Task 12 still owns final certification.

### Task 09 deterministic APT workflow routing (non-lab floor)

`internal/ai/findings_apt_workflows.go` deterministically produces agent-managed
APT update and pressure-gated package-cache findings. Admission requires fresh
dual-timestamp telemetry and the exact current canonical capability/handler.
Stale, skewed, replayed, errored, or capability-less state is unknown and must
not resolve an active finding; only fresh authoritative clear evidence or
explicit resource removal reconciles it. The same pure detector entry point is
exercised through an exact empty-parameter proposal, shared planning/policy and
human approval, durable typed dispatch, canonical terminal audit, and finding
resolution for both workflows. Finding evidence stays bounded and contains no
command, path, package selector, raw APT output, stderr, or reboot authority.
The APT verifier claims only canonical APT finding keys. It must return
unhandled for CPU, memory, disk, and every other non-APT key so the owning
deterministic verifier can evaluate that finding; APT resource lookup failure
must never intercept unrelated verification.

Fake-only callback-loss tests reopen the server-side action store and consume
Task 07 terminal receipts without resending either typed mutation, then
reconcile the audit and originating finding. This closes the non-lab detector-
to-reconciliation code/test floor only. Claims 16/17 and both APT scorecards
remain below operational completion because browser workflow evidence,
disposable Debian/Ubuntu real-lab evidence, and Task 12 final-SHA certification
remain explicit residuals; no tier-5 or tier-6 evidence is claimed here.

`fix_verified` is reserved for a canonical action result whose successful
execution has a confirmed independent postcondition. Agent-attested
confirmation remains useful evidence but projects to
`fix_verification_unknown`; investigation and finding records remain unresolved
and terminal push copy stays explicitly inconclusive. This keeps legacy
single-outcome consumers conservative while `ActionDisposition.ActionResultV2`
retains both truth axes and the evidence source.

The production Docker/Podman Patrol action path now supplies the independent
postcondition that this rule requires. After a successful typed lifecycle or
update receipt, the API executor performs a separate action-bound daemon
observation through the versioned read-only Unified Agent protocol. Only a
fresh, identity-matched daemon snapshot from the distinct daemon observer
trust domain can project to `fix_verified`; unsupported older agents,
inspection failure, stale facts, or mismatched identities remain
`fix_verification_unknown`. The mutation receipt's own same-agent readback
continues to project only `agent_attested` evidence, so adding the production
observer does not loosen the verification rule.

### Incomplete investigation follow-up

An empty or unusable final model response, provider error, or final-summary
timeout does not constitute a model conclusion. The shared agentic loop retains
a readable failure receipt and returns the original error identity for Patrol
investigations. Interactive chat retains its existing best-effort fallback.
The investigation service must not advance a captured proposal after that error.

A final summary that leaks a textual tool call is unusable only for Patrol
investigations: the model was still investigating, and the orchestrator would
read the unmarked prefix as a terminal `needs_attention` conclusion. Interactive
chat keeps the non-empty clean prefix it already streamed as the final message,
so the persisted message matches the stream, and falls back only when that
prefix is empty. Proof: `TestInteractiveFinalSummaryKeepsCleanPrefixBeforeLeakedToolCall`
and `TestInvestigationFinalSummaryWithLeakedToolCallIsNotAConclusion` in
`internal/ai/chat/agentic_final_failure_test.go`.
The same rule holds for a main-loop turn with no structured tool calls whose
content leaked a textual tool call: a Patrol investigation drops that prefix and
takes the bounded summary turn and failure receipt, while interactive chat keeps
it. Proof: `TestInvestigationMainLoopLeakedToolCallIsNotAConclusion` and
`TestInteractiveMainLoopKeepsCleanPrefixBeforeLeakedToolCall` in
`internal/ai/chat/agentic_main_loop_leak_test.go`.

The private orchestrator clears the previous finding outcome when starting a
new session, after retaining its prior context. Failed runs preserve evidence,
counts and any existing action reference. They remain failed rather than being
classified from fallback prose as a completed `needs_attention` investigation.
Failed runs without a conclusion, including timeouts, may be assessed at the
next scheduled Watch without another cooldown. Existing attempt limits,
admission, autonomy and action authority still apply. A real completed model
decision requiring human attention remains terminal.

Proofs: `internal/ai/chat/agentic_final_failure_test.go`,
`internal/ai/findings_provider_failure_test.go`, and the private
`TestProviderFailureClearsPriorConclusionAndRetainsEvidence` regression.
These causal tests do not themselves qualify model-owned unattended recovery.

### Bounded unattended provider follow-up rerun (2026-10-16)

- Pulse PR2994 and enterprise PR123 retain incomplete investigations as failures, preserve their evidence, clear prior outcomes before execution, and leave the next scheduled assessment eligible within the existing policy and attempt limit. New tests fail on the original code and pass after the repair; full affected suites and the private investigation race suite pass. Both source PRs have passing CI.
- Frozen proof `provider-followup-20261016` ran Pro and free Pulse plus Claude for 36 minutes each with the same Opus 5.5 model, goal, ten-minute cadence, target faults at 120/840/1500 seconds, persistent dependency fault at 120 seconds, and controller restart at 1440 seconds. No operator diagnosis, approval, remediation or configuration change occurred inside either scored window. The independent observer retained 208 samples per arm, with no transport loss or source/helper/binary identity mismatch.
- Both arms recovered all three distinct target outages with HTTP 200 and a new process boot, including after controller restart. Both left the healthy control untouched and the dependency unresolved without restart. Pro recovery delays from injection were 672.734/504.897/457.347 seconds; Claude delays were 514.012/397.035/332.276 seconds, sampled approximately every ten seconds. Pro retained three succeeded action receipts with confirmed health. All four external scheduled turns completed and retained their journal.
- Provider-reported list-equivalent usage was US$5.5860168 for Pro and US$1.2533858 for Claude. All 38 Pro CLI invocations and four external turns retained completion receipts. These counts describe different architectural units; actual subscription cash cost and steady-state operating cost were not measured.
- The live Pro run did not naturally encounter an empty response or summary timeout. Controlled causal tests establish those error and eligibility boundaries. The live result establishes this bounded workflow only: fixed order, one trial per arm, restart-only authority and development-mode licensing. Production licence onboarding, repeatability, permanent root-cause repair and wider customer readiness remain unqualified. Keep the customer-outcome gap and candidate planned.
- Community used shipped monitoring MCP/APIs, a monitoring-only agent/read token and the retained scoped Docker bridge, timer and journal. Core free unified action APIs also exist; this experiment does not establish minimum DIY effort or make manual action receipts and guardrails paid-only. Pro restarted the Pulse process; the external arm restarted its scheduler while monitoring continued.
- The external journal repeatedly claimed Pulse had raised no alerts, while retained state included critical container-health alerts. Zero fleet findings did not establish zero alerts. The Pro late history collector also stopped on an unrelated finding ID containing a slash; retained sessions, final records and independent observations succeeded, and a separate read-only post-cutoff supplement retained both workload investigation records. Preserve that collector error with the evidence.
- Final-source browser proof covered `/patrol` > Activity > Finding options and history at 1440x1000 and 390x844: failed empty response, timed-out summary, genuine human attention, retained error/evidence scrolling, thread disclosure, enabled retry focus, close/focus return and reload persistence. This qualification entry changes evidence metadata only; runtime and rendered source remain the tested bytes. Scoped tokens were revoked, owned containers removed, agent state archived and task listeners closed.

### Proxmox guest lifecycle Patrol detector floor

`internal/ai/findings_proxmox_lifecycle.go` owns the deterministic production
detector for Proxmox VM and LXC stopped transitions. The first observation is a
baseline only: Patrol may emit a finding only when a fresh Proxmox source
observation moves the same canonical guest from running to stopped at a newer
source timestamp. Admission also requires the exact current resource-owned
`start` capability and matching `proxmox.vm.lifecycle` or
`proxmox.ct.lifecycle` handler. Templates, locked guests, stale or missing
source authority, repeated timestamps, unknown states, and capability loss
fail closed. Existing stopped guests therefore do not become findings merely
because Patrol or Pulse restarted.

Production admission also reconciles the transition against the canonical,
organization-scoped action-audit store before emitting. A completed `shutdown`
or `stop` may suppress the finding only for the same canonical guest when its
canonical `ActionResultV2` execution succeeded and a recorded approval falls
between the prior fresh running observation and the newer stopped observation.
Legacy-only results, failed or nonterminal actions, other capabilities or
resources, out-of-window decisions, cross-organization stores, and audit-read
errors do not suppress the warning. Suppression advances the observation
baseline so the intentional stop cannot surface one cycle later.

The watcher runs before the model-provider gate whenever guest analysis is
enabled, so detection is not conditional on an available LLM. Fresh running
evidence reconciles an active finding; stale or incomplete state does not, and
explicit resource removal uses its own typed resolution reason. Finding
evidence is bounded to guest kind, stopped state, Proxmox instance/node/VMID,
and observation time and carries no command, credential, token, or provider
mutation authority. Operator maintenance and intentionally-offline intent
continue through the shared finding-store suppression boundary.

The non-lab integration proof takes the emitted finding through an exact empty-
parameter `start` proposal, shared planning, separate human approval, durable
node-agent dispatch, canonical action audit, and server-side Proxmox API
observation. Only confirmed independent evidence projects the originating
finding to `fix_verified`; command success or same-agent status alone cannot
produce the green terminal state.

Qualification fixtures must not disclose benchmark intent to the tested model
through model-visible infrastructure identity. Docker resource names use a
representative service identity plus an opaque run token; ownership, scenario,
and teardown selectors use opaque hashed label values. Exact run/scenario truth
remains in the runner-owned manifest and artifact mapping, outside the model's
tool transcript. The shared-host safety boundary still requires an explicit
target, dual authorization, exact owner-label cleanup, a second no-op cleanup,
and pre/post inventory equality. Legacy `pulse-qual-${run_id}` names remain
accepted for external manifest compatibility, but governed remediation
qualification uses neutral identities so a model is scored on infrastructure
evidence rather than recognizing that it is inside a benchmark.

### Operational Trust action eligibility

`internal/ai/attention_actions.go` is the canonical pure projection for the
first attention-owned action offer. It maps only a
`docker-container-health` lifecycle item on an app-container to the existing
declared Docker `restart` capability. Freshness, completeness, confidence,
permissions, canonical subject binding, unambiguous correlation, live resource
capability, exact internal handler, approval floor, executor readiness, and
operator authority all fail closed through typed reasons. Frontend and API
callers must not reproduce those gates from finding text or loose metadata.

An origin-bound existing action may be projected for authorized historical
review. Verification presentation is derived only from the durable canonical
action state and `ActionResultV2`: confirmed, contradicted, inconclusive, and
not-attempted remain distinct. The projection has no authority to resolve the
operational record.

### Operational Trust rollout telemetry and lifecycle controls

Patrol exposes evidence-backed lifecycle controls only for the selected
canonical attention item. Acknowledge is reversible. Suppression requires a
reason and bounded expiry. Neither action changes detector truth, and stale or
unknown collection state remains stronger than a retained legacy
acknowledgement flag. `internal/ai/patrol_metrics.go` compares the canonical
active summary with the projected state set and increments one unlabeled
low-cardinality mismatch counter on contradiction. It never labels metrics with
resource, record, evidence, actor, or provider-instance identity.

### Trust-gate manual Patrol acceptance

An unscoped or scoped manual Patrol run is accepted only after the runtime
atomically reserves one execution slot and assigns the run ID and start time.
That accepted identity is synchronously visible through Patrol status before
the provider goroutine can emit its first event. A concurrent request therefore
fails as already running instead of starting a second provider call. Once
accepted, the run owns exactly one history record: provider or runtime setup
failure is recorded against that run as an error outcome and must not be
reclassified as an HTTP start failure or silently dropped.

Acceptance authorizes analysis only. Any proposed customer-infrastructure
mutation must still enter the canonical Actions lifecycle with its normal
planning, approval, execution, audit, and verification boundaries; Patrol does
not gain a parallel mutation plane from owning the run slot.

`docs/AI_PATROL_QUALIFICATION.md` treats provider cold start as an explicit
compatibility matrix. First provider events at 0, 15, 30, and 60 seconds must
all retain the one accepted run identity, while the browser's delayed
status/history read remains bounded and cannot trigger a replacement run.
`internal/ai/patrol_manual_acceptance_test.go`,
`internal/api/ai_handlers_patrol_actions_additional_test.go`, and
`frontend-modern/src/features/patrol/__tests__/patrolRunAcceptance.test.ts`
are the focused acceptance, API, and cold-start timing proofs.

Assistant web-interface URL updates resolve metadata providers per organization
from the active tenant monitor. Provider replacement refreshes both the default
service and already-created tenant services after monitor reload, so no AI
service retains an orphaned cache or writes a tenant's URL through another
organization's guest, Docker, or host metadata store.

The same shell chrome boundary now also has to respect browser session
authority. When the authenticated shell in `frontend-modern/src/App.tsx` and
`frontend-modern/src/AppLayout.tsx` is driven by a scope-limited API token
(a kiosk token carrying `monitoring:read` only), the Patrol utility tab route
`/patrol` stays inside the shell's blocked-route redirect whether or not kiosk
mode is currently on, and the Escape key must not be usable to leave kiosk into
that chrome. AI availability and Patrol shell state stay owned by
`frontend-modern/src/stores/aiAvailability.ts` and the canonical Patrol path as
before; this containment is a shell-level authority rule and must not be forked
into a second AI-side scope check.

Application branding in `frontend-modern/src/AppLayout.tsx` is presentation
only. A licensed custom name or logo may replace the default Pulse lockup and
browser-title suffix, but it must not rename Pulse Assistant, alter Assistant
page context, change tool/provider identity, or enter prompt, transcript, or
Patrol evidence payloads. The AI runtime continues to consume canonical route
and resource context independently of the shell's displayed brand.

The Assistant fetch tool's SSRF guard in `internal/ai/service.go` classifies a
destination by reachability, not by the literal bytes of the address. The
`net.IP` predicates it is built from (`IsLoopback`, `IsUnspecified`,
`IsLinkLocalUnicast`, `IsPrivate`, `IsGlobalUnicast`) each inspect only the
address handed to them, so an IPv6 transition address carries an internal IPv4
destination past all of them: `64:ff9b::a9fe:a9fe` reaches 169.254.169.254
while reporting itself as ordinary global unicast. `isBlockedFetchIP` must
therefore unwrap NAT64, 6to4, Teredo, ISATAP, IPv4-compatible and
IPv4-translated encodings through the shared
`securityutil.EmbeddedIPv4Candidates` helper that the audit-webhook validator
and the restricted outbound transport already use, and must not fork a
second AI-local list of transition prefixes. Every embedded destination is held
to the same policy as the outer address, so `PULSE_AI_ALLOW_LOOPBACK` and
`PULSE_AI_ALLOW_PRIVATE_IPS` relax the embedded check exactly as they relax the
outer one, and a transition address wrapping a permitted public target stays
permitted.

### Patrol consumes canonical monitoring and lifecycle policy

The findings operator-state projection carries canonical `MonitoringMode` and
`LifecycleState` in the same store read as maintenance, criticality, and action
lock state. Active maintenance, expected-offline, muted, and retired policy
produce distinct lifecycle causes in durable finding history. Suppression is
reversible when the canonical cause lifts. Legacy providers that project only
`IntentionallyOffline` (no `MonitoringMode`) retain their established
`intentionally_offline` cause and read as expected-offline.

Expected-offline is availability-specific in Patrol exactly as it is in the
alert runtime. `ResourceOperatorStateProjection.monitoringSuppressionCauseFor`
decides per finding: retired and muted silence every finding, while
expected-offline (and the legacy flag) silence only findings in the shared
down condition class (`findingConditionClass` in
`internal/ai/findings_alert_mirror.go`: stopped, exited, offline, unreachable,
PDM offline, Proxmox guest stopped). Capacity, performance, backup, security,
and disk-health findings on an expected-offline resource stay visible; SMART
"offline uncorrectable" reports are disk health, not the resource being down.
The wake path judges the re-detected finding with the same helper, so a
finding silenced under muted wakes when the resource moves to expected-offline
unless it is itself an availability finding.
`internal/ai/findings_expected_offline_test.go` pins the split with muted as
the negative control.

Retired lifecycle also projects an effective remediation lock into Patrol
investigation and the shared execution admission paths, so a resource cannot
be retired from attention while remaining eligible for autonomous action.
Canonical enum fields are additive in the agent capability manifest and its
generated documentation; the legacy boolean remains a compatibility edge, not
the AI runtime policy source of truth.
### Patrol run records no longer carry a dead auto-fix count

`PatrolRunRecord.AutoFixCount` was removed from `internal/ai/patrol.go`,
`internal/ai/patrol_run.go`, `internal/ai/patrol_history_persistence.go`,
`internal/ai/patrol_assistant_handoff.go`, and the persisted mirror in
`internal/config/persistence.go`. The field was constructed as a literal `0` in
`buildPatrolRunRecord` and had no increment site anywhere in the tree, so it was
structurally incapable of holding a non-zero value while still being plumbed
through run history, the Assistant handoff summary, and outbound telemetry.
Governed Patrol fixes are delivered through the approved-action pipeline and
are already counted there (`pulse_intelligence_approved_action_*`), which is the
signal to read for remediation activity. Existing persisted history files that
still contain `auto_fix_count` load cleanly; the unknown key is ignored.

### Model-authored observers have a core-owned local execution floor

`internal/ai/patrol_observer_runtime.go` owns the executable observer ABIs for
retained Patrol objectives. The model may author the meaning and predicate, but
it cannot author executable code, an HTTP origin, secret values, mutation
authority, or lifecycle authority.
Core strictly decodes `pulse-resource-state/v1` for canonical resource status
and `pulse-availability-state/v1` for the outcome of an existing canonical
agentless availability target. `pulse-resource-metric/v1` adds bounded
comparisons over fresh canonical CPU, memory, disk, and temperature evidence.
`pulse-http-json/v1` adds bounded concurrent GET-only JSON-pointer assertions
whose origin and optional authentication value are resolved from an exact
in-scope encrypted discovery record. It accepts a relative path and a secret
reference name, never an origin or credential value; restricted outbound HTTP
blocks metadata/link-local targets and cross-origin redirects, while timeout
and response size are capped. The ABIs require canonical resource scope (with
estate-wide objectives dynamically expanded through unified resources),
an interval trigger, an empty external-requirements object, a 10-to-300-second
local sample interval, and a bounded consecutive-failure window. The
availability ABI accepts only an exact canonical target ID and
`probe_outcome`; it never accepts a URL, address, header, credential, request
body, or secret. Before installation, core proves that the target exists, is
enabled, and belongs to the objective's resource scope. Missing, disabled,
deleted, or cross-scope targets fail closed with typed observer reasons rather
than being interpreted as objective failure. Unknown fields and unsupported
runtimes, triggers, requirements, paths, operators, or values also fail closed
and persist a safe machine reason in the `rejected` or `degraded` state while
coverage remains truthful.

Creating or materially changing an active retained objective queues one
`objective_changed` scoped run carrying its exact ID, revision, brief and
optional context. Distinct objectives never deduplicate together; estate-wide
objectives expand to current canonical resource IDs before the run. This wakes
the model once to design coverage rather than waiting for the next scheduled
Patrol cycle.

That `objective_changed` invocation is a dedicated observer-design mission,
not a shortened global Watch pass. Core omits active and dismissed finding
lifecycle context, withholds finding report/assessment/resolution authority,
and skips the unmatched-signal and assessment continuations. The seed is the
complete mission handoff: exact objective identity and revision, canonical
scope evidence, availability target IDs, and discovery references are resolved
by core before inference. The provider receives only
`patrol_propose_observer`; it cannot spend the objective's bounded turns
rediscovering facts that Pulse has already supplied. This is a least-authority
boundary rather than application-specific intelligence: the model still
chooses the useful signal, evidence fit, interpretation, and unsupported richer
design when necessary, but an unrelated global finding or redundant inventory
search cannot consume the mission or leave a healthy run that never attempted
observer design. If that model-owned turn reaches its output limit before the
proposal call, core discards the partial prose and permits one bounded recovery
turn only when the provider projection is exactly the single
`patrol_propose_observer` capability, with required tool choice and the remaining
fixed run-level token budget. Merely having that capability registered beside
normal Watch tools cannot select this path. An accepted proposal then closes tool authority;
the optional completion turn is prose-only so it cannot duplicate the handoff.
A second truncated turn fails the objective mission closed.

Scoped Patrol identity resolution includes exact canonical unified resources
that have no legacy `StateSnapshot` projection, including standalone
availability `network-endpoint` resources. The scoped runtime rebuild retains
only provider resources matched by the resolved ID/type set, preserves their
canonical availability evidence, and counts them as real scoped work. It does
not fall back to an unfiltered provider or relax exact/unique identity rules.
An objective attached to an availability endpoint can therefore reach the
model-authored observer planner without exposing unrelated estate resources or
failing before the already-supported availability observer ABI is evaluated.

Accepted observers transition through proposed, validated, and installed under
core authority. Resource observers evaluate Pulse's canonical `ReadState`
status locally, falling back to the existing Patrol snapshot only where
canonical read state is unavailable. Availability observers reuse the
canonical observations already collected by Pulse or a selected host agent;
they do not create a second poller and never invoke a model on the sampling
interval. Applicable canonical target IDs and current outcomes are included as
bounded quoted data beside the retained objective so the model can compose the
generic ABI without inventing infrastructure authority. Health leasing is
persisted without changing the operator
objective revision, with all leases due in one sweep committed in one atomic
encrypted-document transaction. A failure transition queues one scoped
`objective_evidence` check through the existing TriggerManager, so the model
wakes for reasoning only after local evidence crosses the declared window and
does not become the polling loop. That check carries the exact objective
identity, revision, brief, optional context, observer identity, affected
resources, observation time, and local evidence through Watch into the durable
finding and the Pro investigation prompt. Duplicate queueing is scoped by
objective and observer identity as well as resources; two retained outcomes on
the same resource remain two distinct reasons to wake. Operator text stays
bounded quoted context and never becomes executable instruction. The observer
remains read-only and cannot enter infrastructure mutation; any later response
still uses the canonical capability, autonomy, approval, dispatch, audit, and
verification kernel. Queue acceptance is not delivery acknowledgement: while
the predicate remains breached, the observer retries an unacknowledged wake no
more than once per fifteen minutes. Only a durable, error-free Patrol run bound
to the exact objective revision and observer version confirms delivery. A
provider outage, runtime refusal, scope failure, restart, or dropped queue item
therefore cannot turn one accepted enqueue into permanent silence, while a
completed Watch assessment prevents repeated model polling until the predicate
recovers and breaches again.
Changing the retained brief, optional context, or resource scope disables the
installed observer and clears its lease; coverage cannot survive a semantic
change to the intent it was designed to protect.

### Patrol follow-up passes are exact-scope, least-manifest continuations

An unscoped Patrol Watch retains the full governed detection-profile manifest
so the selected model owns evidence gathering and diagnosis, minus the
core-satisfied active-finding read. An explicitly scoped Watch whose exact
resource identity resolves to known canonical resource types receives the
least evidence manifest for those types plus the detection-owned finding
lifecycle. `internal/agentcapabilities/patrol_scope_tools.go` owns that mapping:
canonical query and relevant subsystem reads remain available for evidence-led
cross-resource causality, while unrelated subsystem schemas are removed. Watch
does not receive the optional agent-routed `pulse_read` deep-inspection schema;
root-cause log, file, and bounded exec evidence belongs to the separate finding
investigation. The mapping consumes only caller-supplied or core-resolved typed
scope, never prompt text. An empty or unknown type keeps the full governed profile instead of
guessing and silently blinding Patrol; the allowlist is applied after profile
projection and therefore can remove tools but cannot add authority.

The internal Patrol request bridge carries the explicit run limit, caller-owned
tool allowlist and execution identity. It no longer carries an evaluator's
signal-count-derived report budget. Each invocation receives fresh resolved
resource context. Its stable session ID is a forensic-log key and cannot import
prior-run resource aliases or validated targets. The generic infrastructure
resolve/write/verify state machine is removed. Server-owned run adapters enforce
exact finding scope, active identity, complete findings-read preconditions and
objective revision. Invocation policy still forbids infrastructure mutations
from detection and investigation profiles.

An explicitly scoped Watch may carry related hosts or dependencies in its
effective runtime snapshot as model evidence, but active, dismissed, and
snoozed finding lifecycle context is seeded from the exact caller-requested
resource identities. A dependency finding cannot therefore be refreshed or
forced into an assessment merely because its resource helped explain the
target. Deleted-resource reconciliation still uses the full current global
state, and the synthetic Patrol runtime finding retains its existing special
handling.

Finding creation also validates the provider-authored resource ID and display
name as one identity pair against the current runtime snapshot. When both
values resolve uniquely but identify different same-type resources, core
rejects the report and lets the model correct the rejected call in the original conversation
within its execution limit; it never guesses which value was intended. This prevents one
incident from being persisted twice when a report copies the ID of an in-scope
sibling while describing the correct affected resource by name. The existing
exact-name repair remains limited to unknown ID transcription errors and does
not redirect a real known resource ID across the finding scope boundary.

A Watch provider turn that stops because its output-token allowance was
exhausted is not a completed finding decision. Core preserves the partial turn
only as model context, excludes its unfinished prose from the durable Patrol
summary, and offers one bounded retry containing only the governed finding
report and assessment tools. That retry may use a larger per-turn allowance
from the same fixed run-level output budget so reasoning-heavy models can reach
the structured decision without increasing the run's total spend. A second
token-limited turn fails the run closed; it can never be interpreted or
published as an all-clear. An objective-design mission uses the parallel
proposal-only recovery contract above rather than being rejected merely because
finding lifecycle tools were intentionally absent.

Every provider tool call admitted by the original model conversation is
retained in the durable Patrol run trace, including raw provider arguments, failed calls, and bounded
outputs. Qualification and operator forensics therefore see the complete
canonical model-owned decision path, including a failed lifecycle write.
No auxiliary model session can hide those attempts in an ephemeral callback. Exact
finding-lifecycle retries suppressed before canonical invocation are runtime
loop-control decisions rather than tool executions and do not create a second
tool-start/tool-end pair.

An investigation carries the finding's trusted canonical `resource_type`
through `pkg/aicontracts/orchestrator_deps.go` into the core-owned chat runtime.
Core uses the same scoped evidence mapping to project generic query/read tools,
the relevant subsystem evidence tool, and only the investigation-owned
capability lookup and typed proposal sink. The type is not model-authored and
cannot add a tool outside the already-projected investigation profile. Unknown
types keep the full safe investigation profile, but that fallback is not a
validation bypass: it must still contain a tool from the shared closed
infrastructure-evidence vocabulary, the capability lookup, and the proposal
sink before inference. Synthesis, retained knowledge, alert/finding state, and
planning tools do not satisfy that requirement. Known types intersect their
evidence
ceiling with tools that are actually available after profile
projection, so a deployment without an agent-routed or native `pulse_read`
adapter still investigates from canonical query and subsystem evidence. At
least one structured evidence tool plus both the capability lookup and proposal
sink remain mandatory for both scoped and full-profile paths; missing proposal
authority or a zero-evidence manifest fails before inference. Runtime
completion and proposal authority additionally
require at least one of those evidence calls to succeed; attempted calls remain
visible for budget accounting and forensics, but failed or blocked results are
not grounding. Blank tool results and machine-readable policy or approval
blocks are also non-grounding even when an adapter fails to set `is_error`.
This keeps focused Patrol
usable with smaller local models without replacing model judgment with a
diagnostic script or weakening proposal, policy, approval, execution, or
verification boundaries.

### Patrol investigation fallback summaries remain evidence-grounded

When a provider completes an investigation tool loop but fails to emit the
final narrative, `internal/ai/chat/agentic_final.go` must not replace the
operator's evidence with a generic retry request. The deterministic fallback
projects only bounded finding fields from the core-authored investigation
handoff plus successful tool identities. It names the recorded symptom and
canonical affected resource, distinguishes missing narrative from confirmed
root cause, discloses failed tool-call count without treating failures as
evidence, and describes a successful typed proposal only as a governed
proposal. It never dumps raw tool output, infers execution, or upgrades the
finding to resolved. A failed proposal remains absent from the receipt.

### Live Patrol uses the context envelope it certifies

Every Patrol tool preflight, Watch, and Patrol investigation provider request,
including bounded recovery and final-summary turns, carries a provider-neutral
minimum context request of 16,384 tokens. Providers without a runtime context control ignore
the hint; Ollama projects it to native `num_ctx`. The Patrol model-readiness
probe imports this same core-owned value rather than maintaining a second
qualification-only constant. A model must therefore be exercised and run with
the same minimum context envelope: Pulse must never certify tool use with a
large probe context and then silently truncate live infrastructure context or
governed tool schemas at Ollama's smaller server default. Output-token and
reasoning allowances remain separately bounded and do not expand mutation
authority.

### Watch lifecycle closure and subscription receipts fail closed

Watch exposes one existing-finding lifecycle contract to the selected model:
`patrol_assess_finding` with an explicit `present`, `resolved`, or `uncertain`
verdict. The legacy `patrol_resolve_finding` handler remains registered only as
a compatibility boundary for older internal callers; the Patrol detection
profile neither projects nor executes it. Scoped Watch manifests derive from
the same shared detection-tool vocabulary, so resource-type minimization cannot
reintroduce that compatibility handler after profile projection. A resolved assessment still crosses
the same server-owned verification boundary. In particular, a health finding
for an app container cannot close while the exact current Patrol snapshot
continues to report a non-healthy provider health value, even if model prose or
the selected tool claims otherwise.

Local subscription transports retain provider-owned usage receipts without
making token fields part of the model-authored structured turn. Claude usage
comes from its terminal result envelope. Codex usage comes from the terminal
`turn.completed` event emitted by `codex exec --json` and is joined to the
strict last-message payload only after both have passed transport validation.
Qualification may therefore require positive input/output usage for a
tool-free real-model all-clear without falsely rejecting Codex subscription
runs whose analysis and healthy outcome were durably persisted.

The finding resource anchors investigation context without restricting which
canonical resources the model can inspect. The model decides whether further
peer or dependency evidence is needed and whether missing access prevents a
causal conclusion. Neither a successful read count nor a submitted proposal
establishes that cause. Failed reads and explicit uncertainty remain in the
transcript and final conclusion. Canonical topology/query/discovery tools retain
the current evidence required for cross-resource diagnosis.

Resource-scoped investigation health evidence uses the canonical resource
projection. `pulse_query` with `action=health` and a non-empty `resource_id`
must resolve through the same governed lookup as `action=get`, including the
canonical identity, lifecycle status, provider health, restart count, and
health-check dependency targets available for an app container. It must not
silently ignore the resource selector and return fleet connection counts.
Calls without `resource_id` retain the existing aggregate connection-health
overview for backward compatibility. Provider-native IDs and names remain
lookup aliases; responses preserve the canonical unified-resource ID.

Patrol finding creation keeps scope authoritative while tolerating a bounded
redundant-ID transcription error. When a model submits a `resource_id` that
matches no current runtime identity, core may replace it only when the exact
`resource_name` uniquely identifies a same-type runtime record already admitted
to the requested finding lifecycle scope. The canonical ID from that record
then owns deduplication and persistence. A submitted ID that identifies any
real out-of-scope resource, an ambiguous name, a type mismatch, or a name that
does not resolve inside the lifecycle scope remains rejected. No fuzzy name or
ID matching is permitted at this boundary.

### Mobile destination links do not expand AI authority

`AppLayout` supplies the same remembered primary route to desktop and mobile
anchors. Converting the Patrol destination from a button to an anchor changes
browser navigation semantics only: native modified activation may open the
existing Patrol route in another browsing context, while plain activation uses
the existing shell callback. It does not start an investigation, approve an
action, invoke a tool, or create mutation authority.

Menu-opening controls remain buttons, and the mobile bar continues to own
`--pulse-mobile-nav-height`; Assistant overlay containment and action approval
boundaries are unchanged. Mobile navigation and AppLayout tests pin the route
and focus behavior.

### Model-owned continuation and canonical action authority

Assistant chooses whether further evidence, clarification or a governed action
is useful from the request and current evidence. Lifecycle words in user prose
are not an instruction parser. No tool-count or read/write sequence decides
whether the answer is acceptable. The runtime preserves exactly the assistant
prose it streamed, including uncertainty or a no-action conclusion. It never
replaces that saved answer with a provider-only correction.

The model must use canonical capability and readiness facts when explaining an
action or limitation. Invented QEMU guest-agent or discovery-session prerequisites
remain diagnosis defects to catch in real-model qualification (issue #1782).
They are not repaired by regex-based instructions forcing a control call.
`pulse_control` still binds a canonical resource through the shared target
resolver, returns ambiguity or missing-resource evidence when resolution fails,
and plans through the shared lifecycle. Approval requirements, tenant identity,
current availability and independent verification remain canonical authorities.
Typed control results retain the complete canonical plan (including approval
requirements, preflight, related-resource scope and rollback facts), an exact
Actions URL and `execution_requested: false`. Approval and execution are
separate recorded steps, and the current Actions controls own their ordering.
A prepared plan awaiting approval is not an executed write. A subsequent read,
self-reported verification flag or successful call cannot establish recovery.

`pulse_control` also declines to plan when the target's operator state blocks
remediation (`NeverAutoRemediate`, or a retired lifecycle).
`assistantTypedActionPlanner` (`internal/api/assistant_typed_action_planner.go`)
reads that state through the tenant-scoped lifecycle store
(`actionlifecycle.Service.ResourceOperatorState`, the same read Patrol's broker
and dispatch use, not the chat executor's own audit store) and returns
`unifiedresources.ErrResourceRemediationLocked` before `Plan`, so no plan is
written. `controlPlanFailureResult` turns that into an `ACTION_NOT_ALLOWED` tool
response whose details carry `reason_code: resource_remediation_locked` and a
`policy_boundary` telling the model to report the block and that it is cleared
from the resource's Operator overrides. This is a courtesy refusal when the
state can be read, not the safety gate: the lifecycle's dispatch gate stays
authoritative and refuses even a human-approved plan once the lock is stored (a
lock saved while a human dispatch is being admitted is a known narrow race that
the lifecycle owns), an operator state that
cannot be read at planning is not a refusal (planning executes nothing and
dispatch re-checks), and REST and Patrol planning still go through the
unchanged lifecycle planner. The block applies to typed actions that target the
exact locked resource, not its host, its descendants or read-only investigation.
The Assistant has no guest allowlist or denylist of its own: the per-resource
lock is the single place an operator tells Pulse not to act. The former
`protected_guests` AI setting was saved and pushed into the tool executor but
read only by an unreachable handler, so it never constrained `pulse_control`.
It is removed from the AI settings projection, the tool executor and the
Settings panel; `PUT /api/settings/ai/update` ignores the field from older
clients, and a stored value is dropped the next time the settings are written.

`pulse_query action=action` reads an exact action ID from the tenant-pinned
canonical audit. It retains the full plan, recorded decisions, origin and
canonical `ActionResultV2`, including independent observer identity and timestamps.
It exposes no request parameters, credential bindings or raw driver output.
Missing records remain unavailable evidence. Cached inventory and absent timeline
events cannot establish that an action was never approved or executed. The
recorded postcondition is time-bounded evidence, not a current-health guarantee.

The runtime retains the configured turn, evidence-call, token and spend limits,
cancellation and registered invocation permissions. Repeated identical reads
and several failed calls do not independently remove tools. These may be useful
rechecks or evidence of missing access. The model chooses how to continue within
those explicit limits. First-turn clarification is delivered through the normal
interactive question protocol without demanding an unrelated read first.

Removed metrics include workflow-machine blocks and inferred policy-block
self-correction opportunities/success. Actual strict-resolution and routing
refusal counts remain operational counters, not useful-diagnosis or recovery
measurements. Independent action receipts and labelled qualification truth own
outcome verification.

Regression sources: `internal/ai/chat/agentic_action_gate_test.go`,
`internal/ai/chat/agentic_look_gate_test.go`, `internal/ai/chat/agentic_test.go`,
`internal/ai/tools/control_targets_test.go`. Current-source real-model and browser
qualification is recorded in the customer-journey plan. Historical passes do
not satisfy that gate. `TestAgenticLoop_GeminiDiscoveryToLifecyclePlan` also exercises
the actual Gemini wire adapter against synthetic SSE responses: discovery returns
canonical inventory, a submitted reboot reaches typed planning with approval
retained, and a tool-free model conclusion does not manufacture an action. This
is transport/orchestration coverage, not hosted-model action-selection acceptance.
The tool-free case deliberately returns an invented binding prerequisite: the
loop preserves that provider conclusion and does not plan an action. Patrol's
request-local proposal capture and its forced
final turn remain an explicitly unfinished boundary until canonical planning
acceptance/refusal is returned inside the tool call.

### Alert-mirroring findings fold under the alert; flapping collapses to one row

Real-time alerts own down, threshold, and age conditions, but Patrol's
deterministic watchers and the model can still emit a finding for the same
resource and condition. `internal/ai/findings_alert_mirror.go` is the single
matcher: a finding mirrors an alert when it carries the alert's identifier or
when both name the same canonical resource and the same condition class
(down, restart loop, container health, disk capacity, memory, CPU, backup age,
snapshot age, temperature). Unknown alert types and free-form findings never
match. `PatrolService.reconcileAlertMirrors` runs at the end of every real and
demo patrol cycle against the unscoped active-alert snapshot and stamps
`mirrors_alert_id` and `mirrors_alert_type` on unresolved findings through
`FindingsStore.StampAlertMirrors`, clearing the stamp once the alert resolves.
The stamp is a derived read-model annotation: it is not persisted and is not a
reason to drop the finding. Surfaces demote stamped findings under the alert
instead of listing the same problem twice.

Flapping is owned by the existing storm throttler rather than a parallel
mechanism. `FindingsStore.recordTransitionLifecycleLocked` routes every
regressed, resolved, and auto_resolved transition through
`findingStormThrottler.observeFlapLocked`, which keeps a 24-hour sliding window
per finding (hydrated from the persisted lifecycle on first sight so a restart
does not forget). At four or more transitions the finding carries `flapping`
(count, window, first and latest transition) and the store maintains one
`flapping` lifecycle row whose metadata carries the count, updating it in place
instead of appending a row per transition; below the threshold the ordinary
rows return and the label clears. Regression counters keep counting while
collapsed. `internal/ai/findings_flapping_test.go`,
`internal/ai/findings_storm_throttler_test.go`, and
`internal/ai/findings_alert_mirror_test.go` pin the threshold, the collapse,
the hydration, and the matcher.

### Patrol cost projection, model guidance, and budget refusal are runtime-owned

`internal/ai/patrol_cost_projection.go` owns the Patrol cost preview:
`ProjectPatrolCost` prices a provider/model and schedule from
`internal/ai/cost` and the install's own run history (median of the last
thirty days of priced full runs once three exist, otherwise the measured
104,528-in / 4,491-out run from issue #1789), separates scheduled from
observed alert-triggered runs, reports 30-day spend and budget on the same
basis `enforceBudget` uses, and recommends the slowest schedule preset that
keeps scheduled runs under half the configured budget (20 USD reference when
none is set), never faster than the 6-hour default. Every figure carries its
assumption in the payload; the frontend never re-derives dollars.
`internal/ai/patrol_model_guidance.go` owns the recommended / suggested /
caution rules: the Ollama preflight blessing is the only recommended entry,
Gemini Flash-Lite the only caution (support, 2026-07-20), and cloud starting
points are labelled price-driven and unqualified; the cached readiness pass
for the configured Patrol model is surfaced as "verified". Budget refusal is
the `ErrCostBudgetExceeded` sentinel (`CostBudgetExceededError` carries the
figures), classified as `PatrolFailureCauseBudgetExhausted`, excluded from
circuit-breaker accounting, and promoted into the Patrol runtime block state.
The cause is persisted with the runtime finding (`AIFindingRecord.FailureCause`),
refreshed when the finding is re-raised, and not cleared by a provider preflight
success, because a healthy provider says nothing about spend.
`patrol_cost_projection_test.go` and `patrol_model_guidance_test.go` pin the
arithmetic, the history/default switch, the recommendation ladder, and the
guidance matching.

### Patrol digest is a pure rollup of retained runtime records

`internal/ai/patrol_digest.go` builds the weekly "what Patrol did" digest from
run history (`PatrolRunRecord`), the findings store (lifecycle events for
dismissals, suppressions, and investigation outcomes, plus still-open findings
by severity), Patrol-origin action audits, and `internal/ai/cost` usage events
with `use_case = patrol`. It is a pure function over those inputs: it starts no
run, calls no provider, spends no tokens, and mutates no store. New-finding and
resolution totals come from run records because resolved findings are purged
from the store after 24 hours, and the payload reports
`history_complete=false` with `history_since` when the bounded run history no
longer reaches the start of the window. Unknown model pricing is reported as
`pricing_known=false`, never as zero known spend. The effective mode comes from
`Service.GetEffectivePatrolAutonomyLevel`, so an unlicensed install reads as
`monitor`. `frontend-modern/src/api/patrol.ts` mirrors the payload as
`PatrolDigest`. Proofs: `internal/ai/patrol_digest_test.go` and
`frontend-modern/src/api/__tests__/patrol.test.ts`.

Since 23 October 2026 the digest counts investigation runs, not outcome rows.
`digestInvestigationRuns` counts one investigation for each run that finished
inside the window, from the lifecycle rows the findings store already writes
for every run: the `investigation_updated` row that moves the status out of
`running` (or out of the empty status a regression set while the run was
open), or, for a run whose own fix was verified while it was open, the
`verification_passed` row, after which a closing row belongs to the same run.
Later `investigation_outcome` and `verification_passed` rows from the action
lifecycle relabel the latest finished run with its fix's result instead of
counting again, so a queued fix that is later run and verified is one
investigation ending `fix_verified`. A run that failed with no outcome is
`failed`, the bucket telemetry already uses. The summary-field
(`LastInvestigatedAt`) fallback is gone: every run since the v6 release writes
lifecycle rows, and the fallback could count one run in two adjacent weeks.
Lifecycle rows carry no session ID, so a late end written by an older run reads
as the end of the open run, and a run whose rows the 50-row lifecycle has
dropped is not counted. Before this the digest counted only
`investigation_outcome` rows and read their outcome from a `metadata["outcome"]`
key the store never writes, so every one was `unknown`. Proof:
`internal/ai/patrol_digest_test.go` drives the store's own writers.

### Patrol digest client is the only consumer path

`getPatrolDigest(days)` in `frontend-modern/src/api/patrol.ts` is the single
client for `GET /api/ai/patrol/digest`; it forwards the `days` window and
returns the typed `PatrolDigest` payload unchanged. Presentation code must not
recompute digest counts from run history, findings, or cost events, and must
not call the endpoint through any other client. Proof:
`frontend-modern/src/api/__tests__/patrol.test.ts`.

Issue explanation follows the [customer-journey qualification boundary](../../../../qualification/PATROL_ASSISTANT_CUSTOMER_JOURNEY.md#real-outcome-qualification-still-required),
which separates scripted interaction proof from provider reasoning, remediation
and repeatable customer outcomes.

### Subscription routing content boundary

Subscription routing turns emit validated tool calls only. Native CLI narration
and synthetic local errors are transport material, not Assistant answer text or
infrastructure observations. For Claude's native-tool fallback, retain the entire
first declared tool batch in order and discard later CLI continuations produced
without Pulse tool results. Exact tool names, invocation identity, refusal,
permission-denial and disabled local-tool boundaries remain enforced. Structured
final answers retain their content unchanged.

The regression uses a serialized routing envelope before a two-call native batch
and a later synthetic continuation. It verifies empty routing prose, both
original calls, original order and preserved final uncertainty. A completed
real Assistant request after the transport change showed no routing envelope
in the persisted conversation or visible answer. Its unsupported historical
inferences remain failed diagnostic qualification.

The shared Assistant response prompt is scoped to the user's resource, period
and decision. It no longer demands general thoroughness or unsolicited next
steps on every answer. Provenance follows the product's distinct source
observation, model hypothesis, proposal, execution and verification records.
This is model context, not a deterministic quality classifier. Real responses
remain subject to factual review and latency qualification.

### Readable answer tables

The shared sanitized markdown renderer owns horizontal overflow for each table
through a labelled, keyboard-focusable region. Application styling is added only
after sanitization. Model-provided classes, styles and event handlers remain
disallowed. Streaming DOM reconciliation preserves table region identity, focus
and scroll position. The conversation itself must not acquire horizontal scroll
from wide answer tables. Browser proof covers a persisted real answer and
explicit renderer fixtures at 1440, 900 and 390 pixel widths.


### Monitoring identity without command access

An ambiguous target returns `STRICT_RESOLUTION` with the tool error bit and
guidance to select an exact canonical resource ID. It cannot be described as a
missing command connection or fall back to a connected hostname. Docker children
inherit placement, not their reporting host's identity aliases, through the
shared canonical identity owner. `TestDockerLogReadRoutesToHostWithCollectedChildren`
checks registered read dispatch with host/child ingestion and real-collision
refusal; `TestAmbiguousResourceNameCannotFallBackToConnectedHostname` checks the
failure envelope without dispatch.

Command routing resolves monitored topology independently of agent connection
availability. A missing connection must preserve the known target kind and
parent node, cannot fall back to a different agent with a colliding name or ID,
and cannot dispatch a command. No-target routing requires exactly one connection.
Diagnostic reads and file mutations return the same NO_AGENT failure envelope
when no command connection is available. Failure is distinct from an established
permission denial. Do not infer an installation requirement or guest capability
from this absence. Advertised hypervisor lifecycle capabilities remain governed
by their canonical action path, independently of diagnostic command access.


### Independent dependency and restart fault proof

The opt-in live dependency/restart oracle uses the checked-in investigation
and action manifests through the existing DockerLab. It requires an explicit
Docker context, preserves fault state while observing and refusing duplicate
injection, verifies fixture recovery, and requires exact run cleanup plus
unchanged pre-existing inventory. The ordinary package run skips live Docker
work unless explicitly enabled. A direct fixture restart is teardown and must
never be counted as a Pulse approval, execution, rejection or outcome. Model-led
and canonical-action qualification remain separate required evidence.

### Legacy incident recording retirement

`internal/metrics/incident_archive.go` owns the historical recording format and
explicit, organization-pinned archive reads. `IncidentArchiveProvider` exposes
only a resource-bound window lookup with an error result. The primary
`pulse_knowledge` incident action reads the canonical resource timeline. An
explicit `window_id` reads saved legacy observations and labels their timestamp
and historical-status limits. It must never start a recorder or infer source
freshness from a recording timestamp. The disconnected incident coordinator,
fleet sampling adapter, timer loop, retention writer and duplicate tool adapter
are retired. There is no replacement incident scheduler or diagnosis policy.

The archive reader preserves saved timestamps, resource labels, metadata and
summary values, including records older than the former retention period. The response explicitly
identifies the nanosecond encoding of the historical `summary.duration_ms` field. It
distinguishes unavailable archives, failed reads and absent exact resource/window
pairs. Proof lives in `internal/metrics/incident_archive_test.go` and the
registered-tool cases in `internal/ai/tools/incident_history_test.go`.

The incidents listing now uses the shared canonical incident query, including
canonical-only events, alias-aware selection, explicit query bounds and surfaced
read errors. Saved incident shells preserve occurrence identity and local notes.
They do not own a second lifecycle. The old sampler-derived `active_count` stays
null with `active_count_status=not_measured`. Implementation and qualification
receipts are recorded in `docs/qualification/PATROL_ASSISTANT_CUSTOMER_JOURNEY.md`.
The wider customer-outcome qualification gate remains open.

The history tool describes its actual read scope: alert resolution records
closure rather than workload recovery, related-resource IDs identify relationships
rather than additional event targets, and the read does not query action records.
The model owns interpretation of those records. A history-only read cannot prove
that a governed action exists or does not exist.

Canonical alert lifecycle evidence owns the projected incident's resource ID,
resource type, alert type, level and message when present. Saved shells keep
stable incident IDs and local notes, but cannot override that canonical context.
Related command targets do not retarget an alert. The funded OpenRouter
`openai/gpt-6-astra` route has explicitly reviewed, tiered cost estimates dated
2026-09-07. Unreviewed aliases stay unknown. Its history-explanation qualification
is distinct from the earlier Gemini lab matrix and from wider rollout evidence.

### Compact shell Assistant coexistence

The mobile authenticated header may wrap at compact widths so an entitled
Organization selector does not displace session actions or the connection
indicator beyond the page; wrapping must not introduce entitlement reads.
AppLayout component and architecture regressions preserve these boundaries;
real-backend phone geometry proof is retained by the Organization mobile audit.

### Synthetic hosted lifecycle diagnostic

`internal/ai/chat/hosted_lifecycle_diagnostic_test.go` provides an opt-in,
synthetic-only lifecycle acceptance probe using production-built Assistant
instructions and the OpenAI-compatible gateway adapter. Native Gemini wire
regressions remain separate: gateway model behaviour is not native transport
acceptance. Normal tests skip the hosted entry point without its explicit flag.

The fixed five-VM inventory has no agent, action executor or persistent store.
The recording planner accepts only those VMs' reboot capabilities and always
requires approval. Three fresh sessions each allow at most eight logical adapter
calls (including final/recovery calls), 120 seconds, a 128 KiB provider-neutral
request snapshot and 2048 output tokens per logical call. This is a 49,152-token
logical request budget across the experiment. It is not an upstream-attempt or
billing ceiling: the fixed production adapter retains bounded transport retries
and compatibility fallbacks, and execution accounting must include those
attempts. Model-owned tool choice and adverse/tool-free results are preserved.
Acceptance requires each target's actual approval-required tool receipt, not
final prose. This fixture changes no production behaviour and proves no hosted
acceptance until the fixed-source, credential-contained execution has actually
completed.

#### Plan-only capability exposure

The available tool list and rendered governance manifest advertise
`pulse_control` only when the canonical typed-action planner and read state are
installed, subject to the existing control-level policy. Agent command transport
and native app execution providers are neither prerequisites nor substitutes for
that planner. Read-only sessions still hide control. Discovery, capability
validation, approval and execution remain distinct; advertising planning grants
no execution authority.

The offline lifecycle fixture checks the offered provider-neutral tools and
rendered manifest before returning scripted calls, and the loopback gateway
checks actual wire tool names. A scripted call to an unoffered tool is not
capability acceptance. Availability regressions cover planner-only operation,
read-only policy, missing state and executor-only configurations. Search probes
retain the existing normalised literal matching: wildcard strings do not expand,
while a broader literal query can find the five synthetic targets. That read
proof does not establish model-owned recovery or permit broadening action scope.

Service control-setting, execution-mode and Assistant projection fixtures must
install a typed planner whenever they expect planning to be available; a mock
agent transport alone does not model that capability. Provider projection must
not invoke the planner merely to list tools. The read-only projection control
must include an installed planner so it tests policy rather than accidentally
passing because capability wiring is absent. Surface regressions also remove
the planner while retaining execute authority and transport, and assert control
is absent in both provider tools and the surface contract. These are offline
fixture and policy proofs, not hosted-model or installed acceptance.

### Knowledge-store persistence serialization

`internal/ai/adapters.KnowledgeStore` persists resource notes through an atomic
temp-file, fsync and rename. Concurrent `SaveNote` calls must not interleave on
that temp path: disk writes are serialized behind a dedicated save mutex, and
asynchronous saves are tracked so a caller that removes or inspects the data
directory can join them. An observer must read either the previous store or a
complete new snapshot, never a partially written file. This is an internal
durability boundary, not a public API or stored-format change. The
`TestKnowledgeStore_SaveLoad` cleanup join and the concurrent-save reload
regression in `internal/ai/adapters/adapters_additional_test.go` enforce it.

### AI entry points follow the assistant session capability

Every AI entry point in the app shell follows the `assistantEnabled` session
capability (AI enabled and a provider configured, or mock mode), read through
`frontend-modern/src/stores/aiAvailability.ts`. That covers the Patrol
top-level tab, mobile bottom-bar slot, `g r` shortcut, shortcuts-help row and
`Go to Patrol` palette command, plus alert Investigate actions. Patrol cannot
run without a configured provider, so an AI-off install no longer carries
navigation that only reaches an "off" setup page, which restores the issue
#905 behaviour for Patrol. The Pulse Intelligence settings pages and the
`/patrol` route stay reachable, so turning AI back on needs no new path. This
is presentation gating only: Patrol runtime, findings, schedules and the
capability's server derivation are unchanged. `AppLayout.test.tsx`,
`KeyboardShortcutsModal.test.tsx`, `InvestigateAlertButton.test.tsx` and
`App.architecture.test.ts` pin it.

### Prompt-cache usage is accounted, not just logged

Anthropic reports `cache_creation_input_tokens` and `cache_read_input_tokens`
as buckets disjoint from `input_tokens`. The provider client now carries both
buckets on `providers.ChatResponse` and the streaming `DoneEvent` (assigned
from `message_start` and the final cumulative `message_delta`, never summed
across events), the agentic loops and Patrol responses sum them beside the
ordinary totals, and the usage ledger (`cost.UsageEvent`), Patrol run records
and their persisted copies keep them. `cost.EstimateUsageUSD` prices each
bucket at the provider's cache rate when the table knows it (Anthropic:
5-minute write 1.25x and read 0.1x of the input rate) and at the ordinary
input rate otherwise; `cost.EstimateUSD` remains the no-cache wrapper, so
callers without cache data are unchanged. Spend summaries, the Patrol digest
spend, the cost projection (per-run medians now carry cache buckets) and the
usage export expose the buckets and price them the same way. `InputTokens`
keeps its meaning everywhere, so the telemetry token buckets and the
`patrol_input_tokens` semantics are unchanged; cache tokens are reported but
not folded into those counters. Proofs: `anthropic_test.go` (both response
paths keep the buckets, the final delta does not double them),
`pricing_anthropic_test.go`, `store_test.go`, `patrol_digest_test.go`,
`patrol_cost_projection_test.go` and `cost_persistence_test.go`. This repairs
the lost accounting data from Pulse #2350; it is not a measured provider bill
or a claim of savings across six-hourly runs.

### pulse_read admits only argument-validated read forms

`pulse_read` runs commands on monitored hosts, usually as root, without an
approval prompt at any control level, so `ClassifyExecutionIntent` is the only
gate between a model-proposed command and the agent. The classifier used to
approve a command from its first word and checked arguments only for `find`,
`curl` and `timeout`, which admitted state-changing forms such as `date -s`,
`sort -o`, `journalctl --vacuum-time`/`--rotate`, `fuser -k`, `dmesg -C`,
`sar -o`, `ss -K`, `hostname NAME` and interface, ARP and route setters.
`internal/ai/tools/tools_query_readonly.go` now owns the read-only forms:
commands are parsed as literal shell words (expansions, chaining, redirects
other than the fixed fd ones, and globs on validated programs are refused),
every pipeline stage must itself be read-only, intrinsically read-only
programs are listed by name, and every other allowlisted program accepts only
explicitly listed options and subcommands, failing closed on anything unknown.
`route` is judged on purpose rather than through the `dd` substring rule, and
plain `blkid` needs `-p` because without it blkid updates its cache. Ordinary
diagnostics (`journalctl -u ... --since`, `dmesg -T`, `ss -tlnp`, `ip addr`,
`sar 1 1`, `docker ps`/`logs`, `smartctl -a`, `systemctl status`) stay
read-only. Proof: `TestExecutionIntent_ArgumentAwareReadCommands` and
`TestExecutionIntent_RouteOperationBoundary` in
`internal/ai/tools/tools_query_readonly_test.go`.

The browser Assistant chat window was retired on 2026-10-22. Operators who
want to talk to their infrastructure use Claude Code, Codex, and other MCP
clients, which reach the same governed tools through Pulse MCP, so the drawer,
its launcher and command-palette entries, and every "Open in Assistant" or
"Discuss" hand-off left the web app. Alert Investigate now starts a targeted Patrol run
(`POST /api/ai/patrol/run` with the alert's resource and identifier), is offered
only to sessions whose `settingsCapabilities.patrolRun` says that admin route
accepts them, and its findings land on the Patrol page. The External agents settings page moved to
`/settings/pulse-intelligence/agents`; `/settings/pulse-intelligence/assistant`
and its `?tab=` aliases redirect there with the hash preserved. The chat
service in `internal/ai/chat` stays: Patrol investigations run on it and Pulse
Mobile chat uses it until 31 March 2027, so `/api/ai/chat`, the session
endpoints, and the stream contract keep their rules in this contract.
`frontend-modern/src/stores/aiAvailability.ts` replaced the chat store as the
owner of the shell's AI-enabled fact.

### Typed investigation conclusions and subscription-aware investigation limits (22 October 2026)

Patrol investigations record their zero-action conclusion through
`patrol_conclude_investigation`, an investigation-only, side-effect-free,
read-kind, mutation-none tool on the request-local `ProposalCapture`. The model
supplies one of `resolved`, `cannot_fix` or `needs_attention` and a reason.
The capture keeps the first conclusion: an identical replay is accepted and a
different outcome is refused. `InvestigationRunResult.Conclusion` carries it to
the orchestrator beside any proposal. A typed proposal still owns remediation
and outranks a conclusion, and the conclusion consumes no evidence budget.
Once a conclusion is recorded, the next turn is text-only: the summary.
The final turn, whether reached by the turn limit or by the deadline reserve,
offers only the tools that end a run: the conclusion, plus the typed proposal
once the evidence budget is spent. Evidence exhaustion offers the proposal and
the conclusion.

A run reserves time to conclude before its context deadline. The reserve is
one more evidence turn plus the conclusion and summary turns, each estimated as
the slowest turn so far (20 s minimum), plus 15 s, capped at half the run's
budget. When the time left reaches the reserve, the current turn becomes the
final turn. The final summary call may use the time left before that deadline
instead of the 20 s interactive bound, which interactive chat keeps.

On subscription agents, a positive `StreamIdleTimeout` remains an earlier
complete-turn deadline, because the transport buffers the whole turn.
Investigation turns there get a four-minute caller-owned stall budget, while
Watch keeps 60 s. Without an explicit setting, an investigation on a
subscription agent defaults to 25 minutes and to one investigation at a time,
because that transport serves one call at a time.

`LoadConfig` and autonomy saves republish the timeout and, through the optional
`aicontracts.InvestigationConcurrencyLimitUpdater`, the concurrency limit, so a
provider change applies without rebuilding the orchestrator. Both read the
orchestrator before taking the service lock, because `PatrolService` takes its
own lock before the service lock. Patrol's outer deadline for an automatic
investigation follows the effective timeout plus a 5-minute grace instead of a
fixed 10 minutes.

The stuck-run sweep looks up the session of the run each running finding names
among every session the orchestrator's store holds for the finding, so a newer
session cannot hide a live run. Without the store it uses the finding's latest
session, and leaves the finding alone when a different run is the latest. A
live run is reset as
`timed_out` only after its own `InvestigationSession.Deadline` plus that
grace, or its start plus the current timeout and grace when no deadline is
recorded. So a timeout lowered mid-run does not cut it short, and the previous
run's `LastInvestigatedAt` no longer resets a live re-investigation. A run the
orchestrator holds no session for, or closed as `interrupted` when it reloaded
after a restart, is recorded as `interrupted`. Such a run keeps its attempt, so
a run that crashes Pulse cannot retry forever. A run the orchestrator closed
without updating the finding takes the result its session recorded, once the
orchestrator's own update has had the grace to land, or `timed_out` when the
session recorded none. Without an orchestrator the sweep keeps the
`LastInvestigatedAt` rule. A first investigation cut short by a hard restart no
longer stays running for good. `FindingsStore.RecoverStuckInvestigation`
applies the recovery only while the finding still shows that run as running,
so a result or a newer run that landed after the sweep looked is kept. A run
that started before the finding's current occurrence is recorded only as
`interrupted`, never with its session's result. An outcome the action
lifecycle recorded on the finding during the run, such as
`fix_queued`, is kept and the run completes, so a queued fix still blocks
another investigation. The close stamps `LastInvestigatedAt` like any other
closed run.

Two outcomes join the vocabulary. `unclassified` records a completed run that
left neither a typed conclusion nor a recognisable marker. The private
orchestrator no longer defaults that case to `needs_attention`, which ends
investigation of the finding. `unclassified` keeps the normal cooldown and
attempt limit, presents as needs-attention in the loop state, and lands in the
telemetry `other` bucket with no schema change. `interrupted` records a run
cut short by a restart, shutdown or caller cancellation. It is not
`timed_out`, has no cooldown, and reports as `in_progress`. A run cancelled by
shutdown spends no attempt.

A typed conclusion recorded before a later provider failure or summary timeout
stands when the run holds no proposal or action. A failure without one remains
failed, and fallback prose is still never classified as a conclusion. The
prose-marker reader remains a fallback. It accepts only line-start markers
inside the conclusion section. It ignores fenced and indented-code lines,
reading any fence-like line as a fence and ending it only where every
CommonMark reading would, and it treats a quote as running to the next blank
line. Where that differs from CommonMark it hides more, which can only leave
a run unclassified. It now tolerates markdown emphasis, list markers,
numbering, case and heading punctuation.

Proofs:
- `agentic_investigation_conclusion_test.go`: final-turn tools, the summary
  after a conclusion, the deadline reserve, and the reserve, summary-timeout
  and subscription turn-budget helpers.
- `tools_conclude_test.go`: the closed vocabulary, first conclusion wins,
  profile and capture gating, and refusals.
- `findings_test.go`: `interrupted` and `unclassified` eligibility.
- `patrol_investigation_limits_test.go`: the sweep (live runs held to their
  own deadline, a live or pending run behind a newer session, restart
  reconciliation, the settle window, session results, kept action outcomes,
  a result from before a regression, the guarded recovery write and the
  no-orchestrator rule) and the run ceiling.
- `service_patrol_limits_test.go`: provider-aware limits on reload.
- `ai_config_test.go`, `contracts_test.go` (including the session `deadline`
  wire shape), `patrol_cost_test.go` and the orchestrator-setup case in
  `ai_handlers_investigation_additional_test.go`.

This changes outcome recording and run limits only. It widens no mutation,
approval or execution boundary, and it is not measured improvement on
installed systems.

### Orchestrator writes land only on the run's own occurrence (23 October 2026)

The private orchestrator writes a finding's investigation fields through
`orchestratorFindingsAdapter.Update` in `internal/api/ai_handlers.go`, which now
takes each run's start from the orchestrator's own session store. A write that
closes a run goes through `FindingsStore.CompleteInvestigation`. That records
the run's status, outcome, close time and attempt count only while the finding
still names the run: its session must be the finding's current
`InvestigationSessionID`, and its start must not predate the finding's latest
regression. A write that starts a run (`pending` or `running`) goes through
`FindingsStore.StartInvestigation`. That installs the run as before, unless its
session started before the finding's latest regression, which means the run was
started for an earlier occurrence.

Before this, a run still in flight when its finding was resolved and detected
again, or when the stuck-run sweep gave it up and a retry started another run,
could overwrite the newer state with its own session, status, outcome and
attempts. A stale `resolved` then closed the new occurrence's investigation and
stopped Patrol investigating it again. A regression that landed between a run
recording its session and marking the finding as running let that run install
the earlier occurrence's attempt count, or replace a newer run. A late result
from the run the finding still names still lands after the sweep gave it up, so
the run's real result replaces the sweep's `timed_out`.

Starts and closes compare a run's start only with an occurrence start this
process observed: a regression, or the finding's removal by `ClearAll` or
`Cleanup`, after which the same ID detected again is a new finding. The store
keeps these, unpersisted and with the clock's monotonic reading, for a day,
and refuses a start from a session older than that, which could predate one it
has forgotten. A regression an earlier process recorded predates every run of
this one, so it orders none of them, even when the clock was set back across a
restart. Comparing wall times there would refuse every new run until the clock
caught up, starting a run every Patrol cycle or exhausting the attempt limit.
Investigation records and action outcomes keep comparing with
`LastRegressionAt`, because they also arrive for sessions of earlier processes,
stamped by the same clock. After a clock set back across a restart they can
still refuse a newer run's results, as before.

A run whose result does not count, but which the finding still shows as
running, is closed at once through `RecoverStuckInvestigation`, the way the
sweep would close it later: as `interrupted`, or completed with an outcome the
action lifecycle recorded during the run, which is kept. Its attempt count
stays as its admission wrote it. Through the adapter this is rare, because the
start check keeps a run from an earlier occurrence off the finding: it takes a
run with no recorded start, or findings reloaded from disk that still name an
earlier run. A run that records its session after a regression, from a
snapshot taken before it, is still installed with that snapshot's attempt
count. When two runs are dispatched for one finding at once, the run whose
start lands last is the finding's run and only its result is recorded, where
before the run that closed last won. The `aicontracts` boundary is unchanged,
and the orchestrator needs no change.

Proofs:
- `findings_test.go`: `StartInvestigation` skips a run from an earlier
  occurrence or from before the finding was cleared or cleaned up, and installs
  one with no recorded start or against a regression an earlier process
  recorded; occurrence starts past the retention window are forgotten, and a
  session older than that window is refused.
  `CompleteInvestigation` refuses another session, an unnamed run and an
  unknown finding; records the finding's own run, including a late result, a
  run with no recorded start before any regression, and a regression an
  earlier process recorded; and closes a run from before a regression the way
  the sweep does, without projecting its verified fix and keeping a queued fix.
  An earlier process's action outcome stays refused against `LastRegressionAt`.
- `ai_handlers_investigation_additional_test.go`: through the adapter the
  orchestrator receives, a run's result skips a later occurrence and a newer
  run, a run started for an earlier occurrence is not installed and does not
  replace the current run, and the current run's result lands.

This changes which writes land, not what they record. It widens no mutation,
approval or execution boundary.
