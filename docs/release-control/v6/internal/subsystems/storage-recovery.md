# Storage Recovery Contract

## Contract Metadata

```json
{
  "subsystem_id": "storage-recovery",
  "lane": "L15",
  "contract_file": "docs/release-control/v6/internal/subsystems/storage-recovery.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": [
    "api-contracts",
    "cloud-paid",
    "frontend-primitives",
    "unified-resources"
  ]
}
```

## Purpose

Own the storage and recovery product surfaces, recovery-point persistence and
querying, and the operator-facing storage health presentation layer while
keeping adjacent commercial reporting APIs out of storage/recovery product
state.

## Canonical Files

1. `internal/recovery/index.go`
2. `internal/recovery/manager/manager.go`
3. `internal/recovery/store/store.go`
4. `frontend-modern/src/components/Storage/Storage.tsx`
5. `frontend-modern/src/features/storageBackups/storageModelCore.ts`
6. `frontend-modern/src/utils/storageSources.ts`
7. `frontend-modern/src/hooks/useRecoveryPoints.ts`
8. `frontend-modern/src/routing/resourceLinks.ts`
9. `frontend-modern/src/types/recovery.ts`
10. `frontend-modern/src/utils/recoveryDatePresentation.ts`
11. `frontend-modern/src/utils/recoveryTimelinePresentation.ts`
12. `frontend-modern/src/utils/recoveryItemTypePresentation.ts`
13. `frontend-modern/src/utils/textPresentation.ts`
14. `frontend-modern/src/utils/storageSummaryCache.ts`
15. `frontend-modern/src/components/Storage/useStorageSummaryCharts.ts`
16. `frontend-modern/src/features/storageBackups/storageCapacityDeltaPresentation.ts`
17. `frontend-modern/src/features/proxmox/BackupActivityChart.tsx`
18. `frontend-modern/src/features/proxmox/proxmoxBackupActivityPresentation.ts`
19. `frontend-modern/src/features/proxmox/proxmoxBackupRecoveryModel.ts`
20. `frontend-modern/src/features/proxmox/ProxmoxBackupsCoverageStrip.tsx`
21. `frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx`
22. `frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx`
23. `frontend-modern/src/features/proxmox/ProxmoxPageSurface.tsx`
24. `frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx`
25. `frontend-modern/src/features/proxmox/proxmoxBackupsTableShared.tsx`
26. `frontend-modern/src/features/proxmox/proxmoxBackupSourcePresentation.ts`

## Shared Boundaries

1. `frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx` shared with `unified-resources`: Proxmox backup server table rows are both a storage/recovery backup-health surface and a unified-resource platform-table consumer boundary.
2. `frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx` shared with `unified-resources`: Proxmox recoverable workload table rows are both a storage/recovery coverage surface and a unified-resource platform-table consumer boundary.
3. `internal/api/setup_script_render.go` shared with `agent-lifecycle`, `api-contracts`: the generated Proxmox setup-script is a shared boundary across agent lifecycle (forced-command keys, install/uninstall edits), API contracts (rendered token shape and encoded rerun URL), and storage/recovery (backup visibility grants, Pulse-managed temperature SSH keys, and SMART disk-temperature collection).
4. `internal/proxmoxidentity/backup_identity.go` shared with `alerts`, `monitoring`: Proxmox PBS backup subject identity is a shared runtime boundary for monitoring backup freshness, backup-age alert attribution, and recovery-point guest mapping.

## Extension Points

Commercial v5-to-v6 migration posture may be carried by `internal/api/`
handlers that storage/recovery references for setup or support-adjacent flows,
but it remains API/cloud-paid state. Storage and recovery surfaces may point
operators at the v6 upgrade guide when license egress is blocked, but they must
not reinterpret `commercial_migration` as backup coverage, restore readiness,
or storage-health evidence, and must not mutate `first_failed_at`.

Mobile onboarding reads exposed through `internal/api/onboarding_handlers.go`
are storage/recovery-adjacent only as hosted recovery/support handoff
surfaces. Recovery code may consume the API-owned
`409 onboarding_not_ready` diagnostics, but it must not construct a partial
mobile pairing QR/deep-link payload from relay settings when relay
registration or the dedicated Pulse Mobile credential is incomplete.

Docker and Podman app-container CPU fields on unified resource metadata are
storage/recovery-adjacent only because `DockerData` is a shared resource
payload. Raw per-core CPU evidence and normalized capacity CPU must not be used
as backup coverage, restore readiness, storage-health, or recovery-point
signals.

Shared command-agent token binding in `internal/api/agent_exec_token_binding.go`
is API-owned adjacent infrastructure. Storage- and recovery-adjacent setup or
diagnostics flows may observe the API-owned `bound_agent_id`,
`bound_hostname`, and `bound_at` metadata, but they must not rebind generic
`agent:exec` tokens or treat a Proxmox install-command token as recoverable for
another host after the first command registration identity has been persisted.
Remote-config suppression of command enablement for generic or unbound
command-scope tokens is part of that same adjacent API boundary. Storage and
recovery surfaces may observe disconnected command readiness, but they must not
re-enable command config, reinterpret the suppression as backup or restore
state, or introduce a recovery-local command-token binding path.

Hosted tenant agent install commands in `internal/api/cloud_agent_install_command.go`
are adjacent API/lifecycle transport only. A provider-hosted MSP PVE/PBS install
token may allow agent reporting for the scoped tenant workspace, but it must not
grant backup visibility, recovery authority, or storage health privileges; those
remain governed by the setup-script and source-specific backup API boundaries
below.
Operations-loop status wiring in `internal/api/agent_resource_context.go` is
storage/recovery-adjacent only through the shared action-audit and verification
projection. Sibling handlers in `internal/api/` such as the AI settings handler
(`ai_handlers.go`) carry AI provider configuration (for example per-provider
base URL overrides) that is ai-runtime config-surface, not storage or recovery
state; a manual scoped Patrol check routed through this handler is
investigate-only as well (it may analyze storage resources but invokes no
backup, restore, SMART, or recovery operation, and carries resource identity
only). Starter counts, contextual Assistant/external-agent collaboration
counts, Patrol control completed-loop or resolved-loop outcome evidence,
`patrolControlValueState`, legacy `patrolAutonomy*` compatibility aliases, and
the operator-readable `progressLabel` on that
status are API-contract orientation evidence
for Assistant, Patrol control, the legacy Pro activation entry point, and MCP
entry points; they must not be treated as recovery coverage, backup
verification, storage-health proof, recovery-job state, or a reason to surface
verification stdout, command text, resource IDs, or backup identities. The legacy
`proActivationOperationsLoopStarterCount` field is entry-point-specific, while
the legacy completed/resolved/value `proActivation*` fields mirror the same
Patrol control outcome classifier as compatibility aliases and do not create a
second storage/recovery signal.
First-party workflow starter activity recorded through shared `internal/api/`
handlers, including Pro activation entry-point telemetry for the same
operations-loop prompt, is likewise API/privacy/commercial activation evidence
only; storage and recovery surfaces must not treat it as backup readiness,
restore capability, recovered-state proof, or storage-health verification.
Outbound usage telemetry projection in `internal/api/telemetry_pulse_intelligence.go`
is likewise API/privacy evidence only. Update funnel and governed-action
adoption counters may summarize content-free activity for product analytics,
but storage and recovery surfaces must not treat those counters as backup
coverage, restore capability, recovery-job proof, storage-health verification,
or evidence that a specific resource was protected or recovered.
Proxmox page stale-agent notices are adjacent frontend and agent-lifecycle
plumbing even though `ProxmoxPageSurface` is a storage/recovery canonical file.
Those notices may link an operator to scoped agent update commands for
agent-contributed node detail and command support, but they must not be
interpreted as backup visibility, recovery readiness, restore capability, or a
storage/recovery-owned command path. Suppressing those notices when the API has
no deployable agent update target is likewise lifecycle/frontend behavior and
does not change Proxmox backup or recovery evidence.
Proxmox overview Patrol coverage posture is adjacent Patrol context, not
storage/recovery evidence, and must not render on `ProxmoxPageSurface`.
Patrol coverage, schedule, finding, and approval state belongs on Patrol-owned
surfaces or explicit Patrol affordances; Proxmox overview must not treat it as
backup coverage, restore readiness, PBS verification/protection proof, or a
replacement for the Proxmox Backups tab and workload Backup column.
Proxmox backup inventory loading and load-failure chrome is likewise a
frontend-primitives dependency. `ProxmoxBackupsTable` owns the backup API
queries, recovery model, filters, coverage split, and backup-specific error
copy, but repeated loading and retry shells must compose
`PlatformTableLoadingState` and `PlatformErrorState`; storage/recovery must not
fork local table-card loading rows or refresh-button error empty states.
Proxmox backup source/state chips are also a frontend-primitives boundary:
`proxmoxBackupSourcePresentation.ts` owns backup-source labels and semantic
badge tones, `proxmoxBackupsTableShared.tsx` owns the backup table helper
composition, and visible source/state chips must render through `MetadataBadge`
instead of restoring local rounded-sm xs badge spans.
The Proxmox backup view selector follows the same ownership split:
storage/recovery owns the chronological-versus-coverage view values and backup
semantics, while the visible segmented selector shell must compose the
frontend-primitives `FilterButtonGroup`.
Tenant report branding settings are adjacent tenant-local configuration, not a
storage or recovery product state. `reportBranding` persisted in a tenant
runtime's `system.json` should be preserved by the existing tenant data
backup/restore path, but it must not create cross-client report storage,
restore scope, backup visibility, or recovery authority.
Scheduled report definitions and generated report files are likewise
reporting-owned tenant-local artifacts. The scheduler may save output under
the workspace data directory for operator retrieval and retention, but those
PDF/CSV files are not recovery points, backup artifacts, storage-health
evidence, restore manifests, or cross-client storage inventory. Storage and
recovery flows may preserve them as normal tenant data during backup/restore,
but must not interpret their presence as infrastructure protection.

Generated Proxmox setup-script, runtime host-agent setup, and installer
auto-registration changes that affect backup visibility permissions are
storage/recovery-adjacent: optional PVE `/storage` grants must remain effective
for privilege-separated tokens by assigning the same `PVEDatastoreAdmin` role to
both the service user and the concrete token id.
Generated PVE setup-script smoke checks must run after those concrete token
grants have been applied and must not skip or reorder the optional `/storage`
backup-visibility grant path. If the smoke check fails, the script may fall
back to manual completion, but the displayed token details must still describe
the same ACL state that would have been submitted to Pulse.
The generated Proxmox Audit/Repair path is storage/recovery-adjacent for the
same reason: when backup visibility was requested, repair must reapply the
optional `/storage` grant to the service user and to the current concrete token
id if that token still exists, without rotating credentials from repair mode.
PBS generated setup-script auto-registration is storage/recovery-adjacent
because the registration result determines whether PBS backup evidence can flow
without manual follow-up. The rendered script must post auto-registration to the
canonical Pulse base URL plus `/api/auto-register`; it must not derive the POST
target from the setup-script artifact download URL.

Proxmox platform backup surfaces may embed source-specific backup evidence, but
the source of truth must stay explicit. PBS source-detail tables on the Proxmox
Backups tab consume `/api/backups/pbs` and render PBS-authored size,
protection, verification, namespace, owner, and file facts from
`models.PBSBackup`. PVE snapshot, storage-archive, and task tables consume
`/api/backups/pve` and must keep columns source-aware: columns that PVE cannot
populate for the current data set are omitted rather than rendered as all-dash
placeholders. The Proxmox Backups tab owns two primary operator workflows: a
default chronological `By date` recoverable-artifact feed with daily activity
summaries, and a `By guest` workload coverage table for current live guest
posture. Raw PBS, snapshot, and backup-file facts remain source-visible inside
those aggregate workflows rather than equal-weight source browser tabs. Those
views may correlate unified-resource workload identity with PBS artifacts, PVE
backup archives, guest snapshots, and backup tasks, but they must keep each
artifact's source visible and must not flatten PBS verification/protection facts
into PVE archive or snapshot semantics. Workload coverage posture must be
derived from real recovery evidence and recent task outcomes, including
explicit uncovered and failed-latest-task states, rather than from source-detail
row counts alone. PBS server/datastore rows may display backup counts, but the
counts must come from the PBS backup API artifact identity, not from a
datastore-capacity approximation. The table owns which PBS artifact count is
meaningful for the backup-health row, while dense integer count presentation
belongs to `frontend-primitives`: backup count cells must compose
`PlatformTableNumberValue` with `formatPlatformTableIntegerValue` instead of
reintroducing local `toLocaleString()` formatting.
PBS server/datastore utilization follows the same split: storage/recovery owns
which datastore usage percentage is meaningful for backup-health risk, while
frontend-primitives owns one-decimal percent presentation through
`PlatformTablePercentValue` and `formatPlatformTablePercentValue` instead of
local rounded `%` strings.
PBS server uptime follows the same split: storage/recovery owns whether the
server uptime belongs on the backup-health row, while frontend-primitives owns
the one-unit uptime label through `formatPlatformTableUptimeValue` instead of
local `formatUptime` calls.
PBS server memory/datastore byte labels and recoverable-artifact size labels
follow the same split: storage/recovery owns whether memory, datastore usage,
or artifact size is meaningful backup/recovery evidence, while
frontend-primitives owns byte-unit presentation through
`formatPlatformTableBytesValue` instead of direct `formatBytes` imports in
backup table files.
Recoverable-artifact age labels follow the same split: storage/recovery owns
which artifact creation timestamp and age band describe recovery freshness,
while frontend-primitives owns compact relative-time rendering through
`PlatformTableRelativeTimeValue` instead of direct `formatRelativeTime` imports
in backup table files.
Those Proxmox backup workflows own recovery semantics, source-specific
empty-state copy, and retry/filter actions, but their table empty-state frame
must compose the frontend-primitives-owned `PlatformTableEmptyState` shell
instead of importing `EmptyState` directly or wrapping it in a backup-local
card.
The Proxmox overview workload table's Backup column is the primary
glance-level protection signal for current guests. It must remain visible,
dense, and backed by the canonical guest `LastBackup` synchronization from PVE
storage and PBS evidence; the Proxmox Backups tab is the drilldown for
coverage, restore-point, and source-detail evidence, not the first place users
must visit to learn whether a guest is backed up.
That overview Backup signal belongs to Proxmox VMs and LXCs. If the embedded
Workloads table demotes Docker-in-LXC `app-container` rows out of peer
membership, the backup surface must still receive the page model's Proxmox
guest inventory for VM/LXC coverage and must not reinterpret Docker runtime
membership as backup or recovery ownership.
Docker-in-LXC drawer links may add Docker runtime host route state through the
shared `resourceLinks.ts` module, but that query state is Docker-lens scope
only. Storage and recovery route builders/parsers must not reuse that Docker
host facet or treat it as backup-source, datastore, node, or recovery ownership.
The platform page embedding point may pass read-only Proxmox guest inventory
into that backup surface solely for workload identity correlation; the backup
surface must still source restore evidence from the PVE/PBS backup APIs rather
than treating the page shell as recovery state.
Shared router wiring may pass monitor-owned Proxmox/PBS/PMG source freshness
thresholds into unified-resource adapters so resource rows do not flap between
normal poll cycles, but storage/recovery consumers may use that only as
operational resource-status context. Backup freshness, restore coverage,
storage protection, and recovery authority remain sourced from the canonical
PVE/PBS backup and storage-health evidence.

Storage/recovery auth-adjacent changes may consume SSO-authenticated sessions,
but they must not reinterpret SAML or multi-provider SSO availability as a
storage/recovery entitlement; that provider-route and license truth belongs to
the shared API/security boundary.
Storage/recovery may consume authenticated chart and report routes through the
shared API boundary, but it must not own router-level auth configuration reads.
Those reads stay API/security-owned and must snapshot mutable auth credentials
under `config.Mu.RLock()` before storage-adjacent routes rely on the resulting
session or API-token decision.
Storage/recovery may also consume org-scoped session identity from the shared
API boundary, but durable user IDs remain the authorization principal. Contact
email may support display or legacy lookup only; storage and recovery surfaces
must not create their own email-keyed membership or entitlement interpretation.
Hosted direct handoff subjects that reach recovery-adjacent protected routes
must therefore already be stable non-email principals; a blank handoff `UserID`
must fail at the shared API boundary instead of being repaired from contact
email.
Hosted public magic-link sessions follow the same dependent-auth contract:
storage/recovery-adjacent routes may consume the resulting browser session, but
shared auth must reject blank owner/member principals rather than minting an
email-keyed session from delivery metadata.
Checkout webhook magic-link delivery is part of that same dependent-auth
boundary: storage/recovery may observe billing activation for a server-linked
org, but Stripe contact email must not become recovery or storage authority
unless the shared API organization resolver maps it to a stored owner/member
principal first.
Runtime org authorization on that shared API boundary must also stay strict:
storage/recovery-adjacent routes may consume accepted org access only after
`OwnerUserID` or member `UserID` matches the authenticated principal, not after
the session user string matches `OwnerEmail` or member `Email`.
The canonical actor vocabulary for those shared sessions is
`docs/release-control/v6/internal/IDENTITY_INVARIANTS.md`; recovery and storage
work may consume accepted org access, but must not mint or widen access from a
delivery email. For SSO-authenticated browser sessions, storage and recovery
consume the API-owned provider-scoped principal only, not raw SAML/OIDC
username, email, or display claims.
Storage/recovery remediation or restore-adjacent workflows may consume
`POST /api/actions/plan` only as the API-owned resource capability planning
contract. This subsystem must not create a storage-local approval policy,
stale-plan hash, blast-radius model, or execution protocol outside
`internal/api/actions.go` and `internal/actionplanner/planner.go`.
Storage/recovery surfaces may consume unified-resource `platformScopes` as
read-only platform membership context, but they must not reinterpret runtime
scope overlap as storage or recovery ownership. A Docker workload that also
belongs to an owning platform remains governed by the resource, policy, and
backup capability contracts exposed by unified resources and the shared API
boundary.
Successful action plans also remain API-owned audit facts before
storage/recovery surfaces consume them: approval-required plans must persist
as `pending_approval` with initial lifecycle evidence, and retry/idempotency
handling must not duplicate those lifecycle events. Storage/recovery
approval or rejection decisions must route through
`POST /api/actions/{id}/decision`, which records the API-owned audit decision
without executing the underlying capability. Any storage/recovery execution
handoff for the approved action must route through
`POST /api/actions/{id}/execute` so the API-owned action audit records
`executing` before dispatch and the terminal result afterward instead of
creating storage-local action transport. The API execution gate also owns
stale-plan protection: if the current resource or capability no longer hashes
to the stored plan, execution must fail as `action_plan_drift` with a failed
audit row and `plan_drift:` result before any provider restore/remediation
driver runs. Dry-run-only plans remain planning
evidence only; storage and recovery surfaces must not present them as
executable, dispatch them through provider-local restore/remediation paths, or
bypass the API fail-closed execution gate.
Storage/recovery consumers of agent-surface action failures must use the
shared `internal/agentcapabilities` error envelope and
`agentcapabilities.AgentErrCode*` vocabulary emitted by `internal/api/`; they
may explain `action_plan_drift`, missing resources, unavailable executors, or
approval-state failures, but must not mint storage-local error codes or branch
on handler-local string copies outside the canonical manifest vocabulary.
Docker / Podman start, stop, and restart actions are adjacent runtime actions,
not storage or recovery controls: storage/recovery consumers may render their
redacted action history as context, but must not treat container lifecycle
capabilities, `DockerData`, or agent command verification as backup freshness,
restore support, or a recovery-local execution path. When the API-owned Docker
lifecycle executor resolves a command agent by Docker source ID or canonical
host name and dispatches a trusted internal container command after action
approval, storage and recovery consumers may observe only the resulting action
audit and verification evidence; they must not reinterpret that trusted command
path as recovery authority or a storage-owned action transport.
Disconnected command-agent readiness for those lifecycle actions remains an
API/runtime fail-closed condition; storage/recovery consumers may observe that
an action is unavailable, but must not reinterpret it as recovery degradation
or attempt a recovery-local container command path. Typed resource
`actionReadiness` reasons are operator explanation only in this subsystem, not
backup freshness, restore support, or recovery action authority.
Storage and recovery surfaces may consume Discovery context from the shared
API boundary when it helps explain protected workloads or storage-adjacent
services, including mock-mode config/data/log path examples. That context is
read-only evidence: it must not become a storage/recovery-owned command path,
secret source, restore entitlement, or frontend-only fixture separate from the
canonical `/api/discovery` payload. For Proxmox VM and system-container
resources, a node-agent-backed workload `discoveryTarget` is shared API and
unified-resource evidence only; storage/recovery consumers may use it to reach
the canonical discovery/action path, but must not treat it as backup
visibility, restore authority, or storage-local command ownership.
The agent resource-context endpoint follows the same adjacent-evidence rule for
storage/recovery consumers: bounded context sections, provenance, redactions,
and recent action counts may help explain a workload or protected service, but
they must not create restore authority, backup visibility, storage-local
approval policy, or a bypass around the API-owned action and recovery
contracts.
Assistant session rename through `PATCH /api/ai/sessions/{id}` is also only
browser-safe history metadata for storage/recovery consumers. A renamed
conversation may make protected-item investigation easier to find, but title
text must not become backup coverage evidence, restore entitlement,
storage-owner identity, approval policy, or a provider-local recovery command
handoff.
Assistant session undo/redo through `POST /api/ai/sessions/{id}/undo` and
`POST /api/ai/sessions/{id}/redo` is likewise adjacent conversation repair
state only. A restored prompt or restored message count may help an operator
continue a protected-item investigation, but it must not become backup coverage
evidence, recovery freshness, restore entitlement, storage-owner identity,
approval policy, or a provider-local recovery command handoff.
Approved Assistant tool execution through `internal/api/router_routes_ai_relay.go`
is also adjacent API/AI action plumbing for storage/recovery consumers.
`AssistantToolExecutor` / `ApprovedAssistantToolExecutor` may execute an already
approved native Assistant tool. MCP remains an external adapter term, not an
approved-fix execution dependency; neither name grants backup visibility,
restore authority, storage-owner identity, or a storage-local command
transport.
Legacy OpenCode-style Assistant file-change routes under
`/api/ai/sessions/{id}/diff`, `/revert`, and `/unrevert` are not
storage/recovery rollback operations. If those routes are called directly, the
API must fail them as unsupported rather than presenting file diffs or reverts
as backup rollback, restore eligibility, recovery freshness, storage-owner
identity, or provider-local recovery authority.
Mock-mode Assistant chat startup through `internal/api/ai_handler.go` is also
only adjacent AI/runtime proof for storage/recovery consumers. The handler may
enable the Assistant runtime in memory during mock mode so the typed mock SSE
fixture works without real providers, but that effective config and paced mock
tool stream must not become backup visibility, recovery readiness, restore
authority, storage health evidence, or a recovery-local fixture path.
Storage and recovery may also consume unified-resource TrueNAS app and VM
metadata, TrueNAS native service metadata, plus TrueNAS network-share metadata,
as read-only workload, system, and storage-access context when explaining
appliance-owned storage or protected items. `TrueNASData.App`,
`TrueNASData.VM`, `TrueNASData.Share`, and `TrueNASData.Services` remain
unified-resource/platform truth sourced from the TrueNAS API; storage/recovery
must not reinterpret app containers, VM device inventory, service PIDs, share
paths, volumes, or update posture as storage ownership, restore entitlement, or
a separate Docker-only or TrueNAS-local inventory path.
Docker / Podman volume inventory and Kubernetes PersistentVolume /
PersistentVolumeClaim / StorageClass inventory are also read-only platform
context for this subsystem. Storage and recovery surfaces may use those records
to explain runtime attachment, provisioning class, or protected workload
context, but restore entitlement, storage risk, and recovery-point ownership
remain on the storage/recovery and provider contracts rather than on Docker
volume rows or Kubernetes PV/PVC/StorageClass rows. Other Kubernetes native
inventory objects that flow through the shared resource decoder, such as
ConfigMaps, Secrets, ServiceAccounts, Roles, ClusterRoles, RoleBindings,
ClusterRoleBindings, ResourceQuotas, LimitRanges, PodDisruptionBudgets, and
HorizontalPodAutoscalers, remain platform configuration, policy, RBAC, or
autoscaling evidence only and must not become storage/recovery ownership,
restore scope, or secret material. RBAC summary inventory in particular
never carries credentials or restore inputs. If those
ConfigMap or Secret rows are marked `metadataOnly`, storage and recovery may
surface that trust state as context only; they must not infer payload
availability, key names, or restore inputs from the row.
Docker engine `/system/df` storage-usage buckets are host-level runtime
capacity evidence for the Docker page and unified-resource Docker host facet;
they are not storage/recovery resources, recovery-point sources, or restore
entitlements. Docker Swarm node records are likewise runtime topology context,
not storage owners or recovery scope. Docker Swarm secrets and configs are
metadata-only runtime configuration context; secret/config payload bytes are
outside the storage/recovery contract and must not become restore material,
recovery scope, or a storage/recovery-owned secret source.

1. Add or change recovery-point persistence, rollups, or series derivation through `internal/recovery/`
4. Route transport changes for storage and recovery endpoints through `internal/api/` and the owning `api-contracts` proof routes
   Report branding validation and reporting request assembly in
   `internal/api/system_settings.go` and
   `internal/api/metrics_reporting_handlers.go` remain adjacent
   API/security/reporting ownership. Storage and recovery workflows may consume
   generated report output when a separate reporting surface exposes it, but
   workspace logo settings are not backup artifacts, recovery-point metadata,
   restore evidence, or storage-provider credentials.
   Update-plan readiness payloads and apply-route readiness enforcement are
   adjacent shared API context only. Storage and recovery surfaces may observe
   the resulting update state if a future settings flow links to recovery
   preparation, but they must not reinterpret agent-token,
   agent-migration-security, or server-update readiness checks as backup
   readiness. Governed AI action-target normalization in
   `internal/api/ai_handlers.go` and `internal/api/ai_resource_types.go` is
   likewise adjacent AI/API ownership: storage and recovery may consume
   resulting Assistant context if exposed by another surface, but must not
   treat resource-to-action-target coercion as recovery scope, backup
   ownership, restore authorization, or storage-provider identity.
   Mobile onboarding QR/deep-link `instance_url` sanitization in
   `internal/api/onboarding_handlers.go` is also adjacent API/relay-mobile
   ownership. Storage and recovery consumers must not treat an omitted
   non-HTTPS Pulse web handoff URL as storage source identity, recovery
   endpoint identity, backup readiness, or restore-scope evidence.
   AI provider registry and `/api/settings/ai` credential-shape changes in
   `internal/api/ai_handlers.go` are adjacent runtime configuration only:
   provider ids, default model routes, provider endpoints, API-key fields, and
   configured-state fields must not become backup-source identity, recovery
   target identity, restore authorization, storage-provider health, or
   recovery-point metadata.
   Shared API-token transport helpers may be consumed by storage/recovery-
   adjacent flows, but `owner_user_id` remains server-authored token identity
   metadata; storage/recovery extensions must not pass metadata that authors
   or overwrites that owner field. Adjacent first-run, regeneration, install,
   deploy-bootstrap, and enrollment token constructors must attach owner
   identity through the shared API/security helper, not through recovery or
   storage metadata maps.
   Shared setup-script transport may be reused by storage and recovery-linked
   setup flows, but it remains API/lifecycle-owned: generated PVE scripts must
   preserve Proxmox `authorized_keys` symlinks by resolving the target before
   filtering Pulse-managed `# pulse-` SSH key entries, instead of letting
   recovery-adjacent setup replace the symlink path with a local file.
   Shared diagnostics routes may include Docker and Podman agent health notes,
   but storage/recovery does not own a recovery-local runtime vocabulary for
   those notes. Recovery-adjacent diagnostics consumers must preserve the
   source-specific Docker / Podman wording and recovery destinations governed
   by the shared diagnostics API contract.
   Hosted Pulse Account may deep-link a client workspace handoff to reporting
   surfaces such as `/settings/support/reporting`, but that target is a signed
   tenant-local navigation hint only. Storage/recovery must not treat control-
   plane handoff target paths as recovery-point selectors, restore scope,
   backup freshness evidence, report-generation authority, or a storage-owned
   redirect channel; report and backup API payloads remain owned by their
   tenant runtime routes.
   When shared `internal/api/` handlers expose structured Patrol readiness or
   provider/model/tool causes, storage and recovery surfaces may treat them only
   as adjacent operator context and must not convert them into storage health,
   recovery execution, or backup remediation authority. The same rule covers
   the operator-facing `impact` and `recommendation` fields propagated through
   `internal/api/router.go` Finding to UnifiedFinding conversion: storage and
   recovery surfaces may render them as adjacent finding context but must not
   reinterpret them as backup, restore, or storage remediation authority.
   Shared agent event and resource-context transport follows the same adjacent
   context boundary: monitoring/read API tokens receive redacted approval,
   action, and verification command payloads (`commandRedacted:true`) unless
   they also hold action execution scope. Storage and recovery consumers may
   display those redacted records as status or evidence, and action-audit
   verification command/output/note details read back from migrated legacy rows
   must remain stable redaction markers rather than raw historical details.
   Storage and recovery consumers must not derive backup, restore, storage
   remediation, or execution authority from the event stream or resource-context
   bundle.
   The `previous_resolved_fix_summary` operational-memory field carried on
   findings across regressions follows the same scope: storage and recovery
   surfaces may render it as adjacent finding context but must not
   auto-apply the recorded fix description as a backup, restore, or storage
   remediation action; replaying a prior fix is the action broker's
   authority, not the storage/recovery surface's.
   The `trust` block on the patrol-status response
   (`FindingsTrustSummary`) follows the same scope: storage and recovery
   surfaces may read it as adjacent operator context but must not derive
   backup, restore, or storage remediation authority from any of its
   counters.
   Patrol finding lifecycle payloads exposed by shared AI handlers follow
   that same adjacent boundary: fields such as an operator resolution note are
   AI-runtime/API-contract vocabulary and persistence context, not backup
   metadata, restore evidence, storage remediation authority, or a recovery
   execution contract.
   Shared Patrol autonomy routes may also touch broad `internal/api/` wiring,
   but monitor-mode AI configuration and remediation entitlement responses stay
   AI runtime/API-contract owned and must not become recovery-local policy,
   storage remediation authority, or storage-specific license semantics.
   Clearing stale full-mode unlock state through that monitor-only save path is
   likewise an AI runtime entitlement clamp, not recovery approval state or
   storage remediation permission.
   Adjacent Docker / Podman management routes may also share `internal/api/`
   transport with storage/recovery. Storage and recovery consumers must
   preserve the API-owned Docker / Podman module or host wording for management
   responses and must not introduce recovery-local container-runtime labels.
   Pro update credential wiring and update-broker transport in
   `internal/api/router.go` and the update handlers are server self-update
   plumbing, not storage or recovery surface: storage and recovery must not
   consume them, and the pre-update backup and rollback machinery in
   `internal/updates` stays identical for community and private Pro archives
   so recovery semantics never fork by edition.
   Proxmox-side LXC Docker inventory wiring may also pass through
   `internal/api/router.go` and Proxmox agent install-command generation, but
   storage and recovery may consume the resulting app-container/resource
   context only as workload inventory. The `--enable-commands` install opt-in
   for host-side Docker-in-LXC collection must not be reinterpreted as backup
   freshness, restore coverage, storage protection evidence, or remediation
   authority for the LXC guest.
   Adjacent Proxmox VM/LXC lifecycle actions may also pass through the shared
   API action executor and Proxmox node command-agent path, but storage and
   recovery may consume their audit and refreshed resource state only as
   operator context. Start, shutdown, reboot, or hard stop capability must not
   be reinterpreted as backup freshness, restore safety, recovery entitlement,
   or storage-local remediation authority.
   Proxmox auto-registration dedupe for cluster-member endpoints is likewise
   adjacent API/lifecycle continuity: treating a non-primary cluster endpoint
   as already registered prevents accidental shared-token rotation, but it does
   not create backup visibility, recovery readiness, restore scope, or
   storage-local authority for that endpoint.
   Global resource timeline routing may also pass through shared
   `internal/api/` handlers. Storage and recovery surfaces may read canonical
   timeline records as adjacent evidence, but they must not reinterpret an
   unscoped `/api/resources/timeline` provider activity row as backup
   freshness, restore coverage, storage protection, or remediation authority.
   That same adjacent API/security boundary owns CSRF replacement-token
   concurrency for browser mutations. Storage and recovery forms may benefit
   from the shared retry behavior when parallel requests receive replacement
   CSRF cookies, but they must not define storage-local CSRF retention,
   alternate retry tokens, or recovery-specific auth bypass semantics.
   That same adjacent API boundary also owns TrueNAS feature-default semantics for
   provider-backed recovery: storage and recovery must treat `truenas_disabled`
   as an explicit platform opt-out, not as the baseline onboarding state for a
   supported platform.
   That same adjacent API boundary also owns organization-share target-consent
   semantics. When `internal/api/org_handlers.go` and adjacent route wiring
   evolve cross-organization sharing, storage and recovery may consume the
   resulting org-scoped access only after the canonical target organization has
   accepted the share; they must not treat pending share requests as live
   recovery access or invent storage-local approval bypasses.
   That same adjacent API boundary also owns signed release-asset download
   continuity when shared helpers serve installer or unified-agent assets:
   storage- and recovery-adjacent callers may reuse the shared
   `internal/api/unified_agent.go` path, but they must not fork a second
   checksum or detached-signature vocabulary away from the canonical
   `X-Checksum-Sha256`, `X-Signature-Ed25519`, and base64-encoded
   `X-Signature-SSHSIG` contract.
   When that adjacent unified-agent download path is missing a local dev
   binary, its operator guidance must remain scoped to the requested
   OS/architecture. Storage/recovery-adjacent support flows may observe that
   error, but they must not hard-code a Linux-only rebuild command or infer
   recovery ownership from a missing macOS, Windows, FreeBSD, or Linux agent
   artifact.
   That same adjacent API boundary also owns pre-auth local recovery
   containment. Storage- and recovery-adjacent quick setup or break-glass
   routes may exist before auth is configured, but they must stay
   direct-loopback only, keep recovery-token validation bound to the
   generating client IP, and mint or clear browser recovery sessions instead
   of toggling a shared `.auth_recovery` file for every localhost caller.
   Shared auth probes and router bypasses on that adjacent API boundary must
   preserve route ownership: missing credentials must return an explicit auth
   response, route-specific setup/recovery errors must not be overwritten by a
   second generic auth body, and `/api/config/export` or `/api/config/import`
   bypass entries must still leave public-network and credential decisions to
   their route-local handlers.
   That same adjacent token boundary does not make Relay mobile credentials
   available to storage/recovery flows. `POST /api/security/tokens/relay-mobile`
   must stay API/security and Relay-entitlement owned, require the paid `relay`
   feature before minting, and must not be reused as a recovery session,
   export/import bypass, or storage-local credential transport.
   That same adjacent API boundary also owns monitored-system impact preview
   transport for provider-backed setup context. `/api/truenas/connections/preview`,
   `/api/truenas/connections/{id}/preview`, `/api/vmware/connections/preview`,
   and `/api/vmware/connections/{id}/preview` may surface canonical
   current/projected grouped systems for setup and support clarity, but
   storage and recovery must not reinterpret those routes as recovery-local
   onboarding, restore APIs, or commercial limit verdicts.
   That same adjacent monitored-system boundary also depends on restart-safe
   standalone host continuity. Storage- and recovery-adjacent setup or support
   flows may observe a returning host after server restart, but they must not
   reinterpret that gap as a new counted system or invent a storage-local
   grace rule when the shared API and monitoring boundary already carry recent
   host continuity.
   Any adjacent list surfaces that reuse `internal/api/resources.go` must also
   preserve the canonical unified-resource `name -> type -> id` order so
   duplicate-name storage and recovery resources do not reshuffle between cold
   hydrate, paginated reads, and later live runtime updates.
   If those same surfaces cold-hydrate from websocket `state.resources`, the
   state payload must publish the same canonical resource types and display
   labels as `/api/resources` so storage and recovery do not momentarily switch
   between legacy and canonical infrastructure identities within one session.
   WebSocket current-state refreshes are an API/monitoring invalidation
   mechanism. Storage and recovery consumers may observe refreshed canonical
   resource, backup, and recovery state after the hub resolves the coalesced
   payload, but they must not treat the refresh event itself as backup
   evidence, recovery coverage proof, recovery-job state, or storage-health
   verification.
   Adjacent list surfaces that build `/api/resources` queries with standard
   browser encoders must also treat `%2C` separators the same as literal
   comma-separated type filters, so storage and recovery consumers do not lose
   canonical Docker host, agent, or workload rows when sharing the unified
   resource endpoint.
   That same adjacent API boundary now also owns SSO outbound discovery and metadata fetch trust: storage- and recovery-adjacent surfaces may share `internal/api/sso_outbound.go`, `internal/api/saml_service.go`, and `internal/api/oidc_service.go`, but they must not fork separate metadata/discovery HTTP clients, redirect policies, or credential-file read rules when they depend on shared backend auth helpers.
   That same adjacent security-status boundary now also owns paid prompt
   suppression for storage/recovery-adjacent primitives. History and recovery
   copy may describe unavailable local capability, but ordinary self-hosted v6
   installs must not reinterpret `presentationPolicy.hideUpgrade` as a license
   upsell opportunity or surface paid history/recovery upgrade prompts by
   default.
5. Route canonical storage/recovery resource selection through `frontend-modern/src/hooks/useUnifiedResources.ts` and the owning `unified-resources` contract
   That shared hook now also projects resource `clusterId` through the shared cluster-name helper, so storage and recovery links keep the same cluster-context label as other unified-resource consumers instead of rebuilding a local fallback chain.
   That shared hook plus the adjacent websocket/store adapter path must keep
   realtime transport merges canonical for storage/recovery consumers too:
   thinner websocket `state.resources` payloads may refresh status and metrics,
   but they must not downgrade richer REST-hydrated platform summary fields or
   synthesize standalone `clusterId` values from resource names while the same
   session is open. For the default immediate-hydration path, storage and
   recovery consumers must also wait for the first canonical REST snapshot
   instead of painting thinner websocket transport rows first and then
   rehydrating into a richer canonical shape a moment later.
   Websocket-first unified-resource hydration is an explicit consumer opt-in,
   not the storage/recovery default. Infrastructure may use that opt-in for
   connected-system continuity only with delayed canonical REST revalidation;
   storage and recovery routes must continue to require canonical REST first
   unless their own contract is updated with equivalent shape-stability proof.
   The shared `useUnifiedResources()` scope lifecycle is also a stability
   boundary for storage/recovery consumers: org-scope or enabled-state changes
   must invalidate stale in-flight REST refreshes before their errors or
   request-guard cleanup can leak into the active resource snapshot.
   Shared chart transports in `internal/api/router.go` must follow the same
   rule in mock mode: `/api/storage-charts` and adjacent infrastructure chart
   payloads must read through `GetUnifiedReadStateOrSnapshot()` so storage and
   recovery consumers stay aligned with the canonical mock unified snapshot
   instead of slipping onto the live store graph.
   That same `internal/api/` demo boundary must keep runtime-admin operations
   hidden from public preview sessions: `/api/diagnostics`,
   `/api/diagnostics/docker/prepare-token`, and `/api/logs/*` must not remain
   readable side channels while storage or recovery demo routes are otherwise
   presented as read-only product surfaces. `GET` and `HEAD` reads for
   `/api/admin/users` and manual discovery at `/api/discover` must stay hidden
   for the same reason; recovery-adjacent pages must not treat those
   admin-oriented read routes as safe public-demo evidence.
   Storage/recovery-adjacent diagnostics copy that references Docker / Podman
   runtime coverage must also inherit the canonical installed-agent identity:
   the coverage comes from Docker / Podman modules inside `pulse-agent`, not
   from a separate Docker-specific agent product.
   Storage and recovery consumers must also inherit the hook's canonical
   `ResourceType` normalization for route/query filters, so storage subtypes
   such as `physical_disk` stay on the same cache-backed snapshot instead of
   relying on storage-local filter aliases.
   Storage and recovery consumers that need estate data-governance posture
   must read the hook's resource API-backed `policyPosture()` accessor rather
   than deriving sensitivity, routing, or redaction counts from storage-local
   tables, AI summary payloads, or route filters.
   Optional selector shells that only surface storage/recovery counts when they
   are visible must now pass an explicit enabled gate into that shared hook and
   any adjacent recovery-rollup query, so hidden workload-route selectors do
   not hydrate storage/recovery transport on the protected hot path.
6. Preserve API-owned node identity continuity in shared `internal/api/` helpers so storage and recovery transport attachments do not fork by hostname-versus-IP drift across the same runtime.
   That same adjacent `internal/api/` boundary also owns canonical
   host-alias propagation on grouped-system and attached-connection payloads.
   `internal/api/connections_types.go`,
   `internal/api/connections_grouping.go`, and
   `internal/api/connections_aggregator.go` must publish normalized
   `hostAliases` whenever shared discovery or provider-backed inventory could
   observe the same node by hostname and IP, so storage- and
   recovery-adjacent consumers inherit one canonical represented-host identity
   instead of inventing local merge heuristics.
   The settings-level manual discovery refresh at `/api/discovery/run` belongs
   to that adjacent discovery/API boundary. Storage and recovery surfaces may
   consume refreshed workload discovery records, but they must not reinterpret
   the sweep as a recovery scan, protected-system admission, or storage-local
   ownership signal. Because that sweep can dispatch agent-backed discovery
   commands, adjacent storage/recovery surfaces must also inherit the
   API/runtime gate: `settings:write` plus enabled Discovery are required
   before command-backed refresh, and `monitoring:write` remains insufficient.
   Storage and recovery may also observe `/api/connections` agent
   command-policy and config-rollout state from this adjacent API boundary, but
   that state must already be token-gated by agent lifecycle. It must not be
   reinterpreted as storage readiness, protected-system ownership, backup
   visibility, or a recovery-local command grant.
   If the shared discovery boundary repairs a fresh unknown workload record
   into a known service identity and endpoint candidate from canonical resource
   metadata, stored facts, or safe command evidence, storage and recovery may
   consume the repaired context only as read-only explanation; that repair does
   not create backup visibility, restore authority, storage ownership, or a
   recovery-local endpoint contract.
   Approved-action tool invocation parsing in `internal/api/router_routes_ai_relay.go`
   is also adjacent API infrastructure rather than storage/recovery grammar:
   storage and recovery surfaces may consume governed Pulse tool execution
   results, but parsing `pulse_*` text invocations, `default_api:` prefixes, and
   quoted arguments must stay in `internal/agentcapabilities`.
   Native Assistant workflow-prompt rendering through
   `POST /api/ai/workflow-prompts/render` is the same kind of adjacent
   AI-runtime/API-contract transport: storage and recovery may provide context
   that makes a manifest-owned Assistant starter usable, but the shared
   `BuildPulseWorkflowPromptFromManifest` render contract, rendered prompt
   text, prompt argument validation, and starter availability must not become
   recovery-local route state, restore authority, or backup workflow grammar.
   That same adjacent `internal/api/` boundary also keeps public hosted signup
   commercial-only: storage and recovery surfaces must not infer tenant
   existence, email issuance, or readiness from `/api/public/signup` response
   codes or payload fields when shared backend API helpers change nearby, and
   they must not treat that response as a source for owner identity because the
   stable hosted owner principal is server-side org metadata resolved later
   through magic-link verification.
7. Preserve fail-closed API assignment and lookup behavior in shared `internal/api/` helpers so storage and recovery surfaces do not inherit orphaned profile or resource references from unrelated transport mutations.
   Preserve fail-closed proxy-auth administrator evaluation in those same
   shared helpers as an adjacent security/API boundary: storage and recovery
   surfaces may consume an already-authorized admin request, but they must not
   infer storage, restore, or protected-system authority from a proxy-auth user
   whose configured role header is missing or blank.
   SSO session display labels in those shared helpers are also adjacent
   security/API presentation state only. Storage and recovery surfaces may show
   the current user's display label when a parent shell provides it, but backup
   visibility, restore authority, protected-system ownership, and
   recovery-local audit attribution must continue to use backend-authenticated
   stable principals and storage/recovery resource identities.
8. Preserve canonical configured public endpoint selection in shared `internal/api/` helpers so recovery and storage links do not inherit loopback-local scheme drift from admin-originated setup/install flows.
9. Preserve trailing-slash normalization in those shared install-command helpers so recovery-adjacent transport and link surfaces do not inherit double-slash installer paths or slash-suffixed public endpoint drift from canonical backend install payloads.
10. Preserve canonical /api/auto-register token-action truth in shared `internal/api/` helpers so adjacent setup and recovery-adjacent transport flows stay on caller-supplied credential completion instead of reviving deleted alternate completion modes.
11. Preserve the canonical setup-script `source="script"` marker through those same shared auto-register helpers, and reject non-canonical source labels there, so later canonical reruns can keep treating script-confirmed tokens differently from agent-created tokens without reviving arbitrary caller-label compatibility.
12. Preserve the canonical auto-register node-type boundary in those same shared helpers so only supported `pve` and `pbs` registrations can complete, and unsupported runtime labels cannot bleed fake node identities into adjacent transport or recovery-adjacent state.
13. Preserve the canonical auto-register token-identity boundary in those same shared helpers so only Pulse-managed `pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>` token IDs matching the requested node type can complete, and arbitrary, cross-type, or non-Pulse-managed token identities cannot bleed into adjacent transport or recovery-adjacent state.
    Preserve canonical auto-register event intent in those same shared helpers:
    only first-time node creation may emit the toast-bearing
    `node_auto_registered` WebSocket event, while idempotent existing-node
    refreshes must stay on non-toast configuration-change events so adjacent
    storage/recovery transport does not infer a second protected-system
    admission from a credential refresh. The event data must preserve the
    canonical registration `source` so script-created and agent-created
    lifecycle events remain distinguishable without re-reading setup state.
    That same adjacent transport boundary must also preserve disabled
    provider-connection admission truth. Storage- and recovery-adjacent setup
    surfaces may reflect zero-delta or removal-only monitored-system previews
    for disabled TrueNAS and VMware connections, but they must not reinterpret
    those responses as active counted storage capacity.
    That same shared helper boundary also owns script teardown symmetry:
    `/api/auto-unregister` must remove the matching script-managed PVE/PBS node
    immediately, return the canonical success/noop envelope, and trigger the
    same discovery-refresh plus node-deleted websocket side effects as a manual
    delete so adjacent recovery/storage surfaces do not retain stale provider
    context after Pulse credentials have been removed from the host.
14. Preserve canonical /api/auto-register DHCP continuity in those shared helpers so a PVE or PBS node that reruns registration from a new IP with the same canonical node name and deterministic Pulse-managed token identity updates in place instead of duplicating the inventory record.
    That same shared helper boundary now also owns runtime-side Proxmox
    `candidateHosts` selection from Pulse's network view: storage and
    recovery-adjacent transport flows may not bypass server-side reachable-host
    selection or persist the caller's first preferred host when the canonical
    auto-register helper has already chosen a different reachable endpoint.
    That same shared dependency also assumes the helper only persists
    `VerifySSL=true` for the selected Proxmox host when Pulse actually captured
    that host's certificate fingerprint, so adjacent setup and recovery-linked
    transport flows do not inherit a false strict-TLS claim for self-signed
    nodes that never completed fingerprint capture.
    That same shared dependency now also owns stale-marker repair truth:
    setup-token-authenticated `checkRegistration` calls may omit token
    completion fields and answer only `{registered:boolean}`, so adjacent
    transport flows must not reintroduce local marker trust or token rotation
    when the canonical auto-register helper can verify whether Pulse still has
    a matching node.
15. Preserve the governed root-or-sudo Unix wrapper in shared backend install-command helpers so storage- and recovery-adjacent transport surfaces do not inherit a stale raw `| bash -s --` install payload shape from the canonical agent-install-command API and hosted Proxmox install responses.
16. Preserve optional-auth tokenless behavior in those same shared backend install-command helpers so adjacent transport surfaces do not implicitly persist API tokens and flip auth-configured state when an operator only requested a Proxmox install command on a token-optional Pulse instance.
17. Preserve backend-owned Pulse Mobile relay runtime credential minting in those same shared `internal/api/` auth/security helpers so storage- and recovery-adjacent transport surfaces do not inherit browser-authored wildcard token bundles when they depend on the canonical security helper layer.
18. Preserve the dedicated backend-owned `relay:mobile:access` capability and its governed backward-compatible route inventory plus the shared helper call sites around it, so storage- and recovery-adjacent transport surfaces do not treat the mobile relay credential as a general AI scope bundle.
    That same shared `internal/api/` machine boundary also owns hosted
    entitlement refresh targeting: when storage- or recovery-adjacent hosted
    routes execute under a tenant org without org-local billing state, the
    refresh path must repair the instance-level `default` lease and evaluator
    instead of rewriting the empty tenant org, so AI-guided recovery and
    hosted diagnostics do not collapse into false free-tier behavior.
19. Preserve shipped local security-doc guidance in shared `internal/api/` config/setup helpers so storage- and recovery-adjacent transport surfaces do not reintroduce GitHub `main` security links when the running build already serves its own local security documentation route.
20. Keep shared `internal/api/` Patrol transport and alert-trigger edits feature-isolated: Patrol-specific recency fields, callback fan-out, or alert-bridge wiring changes must not leak into recovery queries, storage links, or recovery-adjacent install/setup flows unless this contract changes in the same slice.
    The same adjacency rule applies to AI settings transport in `internal/api/ai_handlers.go`: provider auth state, masked-secret payload fields, provider-test model selection, safe provider preflight diagnostics, and legacy Anthropic OAuth cleanup fields remain AI/runtime plus API-contract concerns and must not be absorbed into storage/recovery transport ownership just because those handlers live under the shared backend API tree. Storage/recovery-adjacent consumers may preserve or clear legacy OAuth tokens only through the shared AI settings owner; they must not treat stored OAuth tokens or `auth_method=oauth` as recovery capability, provider readiness, restore authority, or an AI-backed storage support signal.
    Patrol readiness labels on the same settings payload, including the
    user-facing Patrol control label for the stable `configuration` check ID,
    are AI/runtime plus API-contract wording and must not be reinterpreted as
    backup visibility, restore authority, recovery readiness, or storage-source
    health.
    The same adjacency rule applies to Pulse Assistant chat SSE progress in
    `internal/api/ai_handler.go`: neutral `workflow_state` transport liveness
    such as `stream_idle`, provider startup, retry, fallback, and
    model-thinking status is Assistant/API progress only, not recovery
    acquisition, restore identity, backup task freshness, storage-provider
    health, or storage/recovery job progress.
    The native Assistant surface-tool inventory at
    `GET /api/ai/assistant/surface-tools` follows the same adjacent boundary:
    storage/recovery surfaces may display Assistant tool availability, but must
    not treat it as backup visibility, restore authority, recovery readiness,
    storage-health evidence, or storage/recovery job progress.
    Direct alert-investigation execution controls in `internal/api/ai_handlers.go`
    follow that same split: request-scoped `AutonomousMode:false` and
    `RequireCommandApproval:true` are AI action-governance constraints, not
    storage/recovery restore approval, recovery freshness, or storage diagnostic
    payload semantics.
    Visible `stream_idle` workflow progress on legacy Assistant SSE routes in
    `internal/api/ai_handlers.go`, including direct execute and alert
    investigation streams, is likewise Assistant/API transport liveness only,
    not recovery acquisition progress, backup task freshness, restore readiness,
    provider health, or storage/recovery job status.
    The AI-runtime model-boundary sanitizer that governs how much Assistant
    infrastructure context reaches cloud models (credentials and local-only
    resources always withheld) is not a storage/recovery restore approval, backup
    freshness, recovery-scope, or restore-command signal.
    Patrol finding chat handoff execution controls in `internal/api/ai_handler.go`
    follow the same boundary: backend-forced `autonomous_mode:false` for
    `finding_id` handoffs with model-only Patrol briefing, resource, or action
    context is Assistant action-governance, not a storage/recovery approval,
    recovery freshness, or restore-command signal.
    If the resolved `finding_id` request also carries recognized same-finding
    Patrol product handoff context, resources, or action references, the merged
    model-only context remains secondary to backend-refreshed finding context
    and must not become storage freshness, restore eligibility, recovery
    execution authority, or a storage-local approval shortcut.
    Patrol run chat handoffs through that same shared handler follow the same
    adjacent-boundary rule. A run ID may let AI runtime rebuild model-only run
    context from Patrol history, but scoped storage resources or runtime failure
    details attached to that briefing are review context only and must not
    become backup freshness evidence, restore eligibility, storage health truth,
    or recovery execution authority.
    Alert, incident, and Patrol assessment Assistant handoffs that send bounded
    model-only `handoff_context`, `handoff_resources`, or `handoff_actions`
    through `/api/ai/chat` without a `finding_id` stay on that same adjacent
    AI/runtime boundary. Storage and recovery surfaces may consume the
    explanation context, but must not treat the handoff resource or action
    reference as backup freshness, restore eligibility, recovery execution
    authority, storage health truth, or a storage-local approval shortcut.
    Resource-context Assistant handoffs through `internal/api/ai_handler.go`
    follow the same adjacent-boundary rule: storage or recovery resources may
    enter Assistant only as selected-resource, model-only context, not as a
    provider command, recovery action, raw path/config disclosure, or
    storage/recovery execution authority unless a governed action or recovery
    contract explicitly owns that operation.
    That same adjacent boundary also keeps the retired Patrol quickstart
    contract out of storage/recovery ownership: shared AI handlers no longer
    expose active quickstart credit, token, or hosted-model provider state, and
    storage/recovery surfaces must not reintroduce local quickstart accounting,
    token lifecycle, anonymous bootstrap identity, fake activation records, or
    commercial identity rules. That shared AI settings payload is also
    intentionally vendor-neutral: storage/recovery-adjacent consumers may see
    old `quickstart:*` values only as compatibility data being cleared by the
    shared settings helpers, and they must not treat vendor model IDs or
    quickstart upstream-model defaults as part of storage/recovery transport
    ownership or route behavior.
    Structured Patrol investigation records follow that same adjacent-boundary
    rule. Storage and recovery surfaces may consume the resource context in a
    shared `investigation_record`, but they must not reinterpret that record as
    recovery freshness, restore support, backup cadence, or storage-local
    action authority. Assistant chat summaries built from `finding_id` remain
    AI/runtime context; storage and recovery may read the resulting guidance
    only as adjacent investigation context, not as a recovery support verdict
    or restore execution contract. If that guidance is passed as model-only
    Assistant handoff context instead of persisted prompt text, the boundary is
    unchanged. If the same handoff seeds Assistant resolved-resource scope, that
    scope remains AI/runtime action-validation context only and still cannot
    become backup freshness, restore eligibility, or storage-local recovery
    authority. If Assistant stores the originating finding ID to refresh current
    unified finding and investigation-record context on follow-up turns, that
    stored reference is still an adjacent AI/runtime context selector and must
    not become backup freshness, restore support, recovery execution authority,
    or storage-local lifecycle state. Clearing that handoff when the current
    finding no longer resolves is adjacent AI/runtime invalidation, not a
    recovery freshness or restore-support decision. Structured action or
    approval references carried by that handoff are also adjacent AI/runtime
    review metadata only, including when Assistant recovers the current live
    Patrol approval by finding ID before building model-only action context.
    Unified finding lifecycle facts, latest lifecycle
    event briefing lines, and detailed lifecycle context carried by the same
    handoff remain Patrol/AI review metadata and must not become backup recency,
    restore support, or storage-local lifecycle state. Primary finding
    recency, evidence, verification, and governed action artifact facts in
    the finding briefing and related root-cause or correlated finding summaries
    resolved for that Assistant handoff are also adjacent AI/runtime explanation
    context only, including any recency or latest lifecycle facts attached to
    those related summaries; storage and recovery surfaces must not reinterpret
    those related records or their seeded handoff resources as backup freshness,
    restore eligibility, recovery execution authority, or storage-local
    capability truth. Current resource-state, source-health,
    incident, metric, and capability summaries carried by that handoff are also
    adjacent AI/runtime context and must not become backup freshness, restore
    eligibility, recovery execution authority, or storage-local capability
    truth.
    storage and recovery surfaces must not reinterpret approval IDs, action IDs,
    action lifecycle state, fix IDs, risk, or target labels as restore support,
    backup freshness, recovery execution authority, or a storage-local approval
    bypass. Any refreshed approval or action-audit status snapshot in Assistant
    handoff context is still read-only AI/runtime context and must not become
    recovery freshness, restore eligibility, or storage-local execution state.
    Finding briefings generated for those same Assistant finding handoffs are
    also adjacent AI/runtime context only: storage and recovery surfaces may read
    their Patrol conclusion and bounded evidence as investigation context, but
    must not reinterpret the briefing as backup recency, restore support,
    Patrol-authored remediation guidance, or storage-local action authority.
    Saved Assistant message history exposed by the shared AI endpoints follows
    that same adjacent boundary. Storage and recovery surfaces may consume only
    the API-owned client-safe transcript projection; hidden provider reasoning,
    raw `pulse_*` / `patrol_*` tool-call prose, token accounting text, and
    provider thinking text are not recovery evidence, backup freshness, restore
    eligibility, storage-local approval state, or recovery execution authority.
    API-facing Assistant chat tool calls projected through
    `internal/api/chat_service_adapter.go` must stay on the shared
    `agentcapabilities` provider-call shape; storage and recovery consumers must
    not reinterpret Assistant transcript tool-call IDs, inputs, output, success
    flags, or provider continuation metadata as backup coverage, recovery
    freshness, restore support, or storage-local action authority.
    Searchable Assistant session-list queries on `GET /api/ai/sessions` remain
    the same adjacent browser-safe history navigation projection: storage and
    recovery surfaces may not reinterpret search hits, handoff summaries, or
    message counts as backup coverage, recovery freshness, restore capability,
    or storage-local action authority.
    The Pulse Intelligence agent capability manifest in
    `internal/agentcapabilities/manifest.go`
    follows the same adjacency rule: external-agent tool metadata,
    action-mode governance, shared external-tool projection helpers, shared
    schema-envelope helpers, and typed MCP argument schemas may mention
    storage-adjacent resources, but they are not recovery-point sources,
    restore authority, storage ownership, or
    storage/recovery-owned API contracts.
    Native Pulse Assistant provider seams and native tool-adapter names in
    shared `internal/api/ai_handler.go`, `internal/api/agent_profiles_tools.go`,
    `internal/api/router.go`, and `internal/api/router_routes_ai_relay.go`
    follow that same adjacent boundary. `MCP` remains an external protocol,
    manifest, and wire-schema term, while the in-app Assistant `ToolAdapter`
    family is AI/runtime plus API-contract state; storage and recovery may
    consume the resulting governed context, but must not fork a recovery-local
    tool transport or reinterpret native Assistant adapter naming as backup
    coverage, restore support, recovery freshness, or storage-local action
    authority.
    Pulse Intelligence operations-loop external-agent readiness in
    `internal/api/agent_resource_context.go` is likewise adjacent
    AI-runtime/API-contract state only: storage and recovery surfaces may
    observe the resulting content-free readiness boolean when explaining a
    guided investigation, but they must not reinterpret the Pulse MCP token
    scope check as backup access, restore authority, provider credential
    readiness, or recovery-job capability.
    Patrol control completed/resolved outcome evidence exposed through
    `patrolAutonomy*` compatibility fields, `patrolAutonomyValueState`, and
    the operator-readable `progressLabel` on that same status projection are
    derived from API-contract owned Patrol status and the shared
    `internal/telemetry` count-only classifier without external-agent readiness
    as an input. The
    legacy Pro activation starter field is entry-point-specific, while legacy
    completed/resolved/value `proActivation*` status fields are compatibility
    aliases only. Storage and
    recovery may not fork those branch rules or status labels, enrich them with
    backup or appliance identifiers, or treat MCP readiness, status wording, or
    any completed/resolved Patrol state as recovery coverage, restore verification, storage health, or
    recovery-job authority.
    Assistant runtime identity strings exposed by those shared API handlers
    follow the same ownership boundary: they must name the first-party surface
    as Pulse Assistant, not a legacy generic `Pulse AI` runtime, and
    storage/recovery surfaces may not reinterpret that naming as recovery-local
    readiness or execution authority.
    Shared AI settings persistence, Patrol preflight, profile-suggestion, and
    remediation-impact copy exposed through those same adjacent handlers must
    keep Pulse Intelligence and Assistant & Patrol naming at the AI/API
    boundary; storage/recovery surfaces may observe that copy but must not
    reinterpret it as backup coverage, restore support, recovery freshness, or
    storage-local action authority.
    The `can_redo` flag on that same session-list projection is only Assistant
    conversation repair state. It must not be interpreted as recovery
    reversibility, restore availability, backup freshness, or any storage-local
    undo capability.
    That same adjacent `internal/api/` boundary still carries Patrol-run
    execution identity. Storage and recovery may observe shared Patrol
    transport through `internal/api/chat_service_adapter.go`, but they must not
    drop, rewrite, or reinterpret the execution identifier that describes one
    Patrol run across agentic provider turns.
    Retired quickstart block reasons exposed through stale compatibility state
    are likewise not storage/recovery onboarding truth; adjacent pages may
    normalize the message, but they must not reinterpret it as a storage install
    prerequisite, provider-connection error, or
    recovery-capability verdict.
21. Keep provider-backed recovery onboarding on the adjacent platform-connections contract. When `internal/api/` grows or changes TrueNAS connection CRUD, masked-secret preservation, saved-connection retest routes, edit-form saved-test payload overlays, or similar provider setup flows, storage and recovery may consume the resulting recovery points but must not absorb that connection-management ownership into storage/recovery-local handlers or page flows. That same adjacency also covers per-surface scope as it flows through the unified connections aggregator: when `internal/api/connections_aggregator.go` projects TrueNAS `MonitorDatasets`/`MonitorPools`/`MonitorReplication` flags into the aggregator's `scope` map, storage and recovery may observe that projection to explain dataset/pool/replication coverage to operators but must not reinterpret those flags as a recovery freshness verdict, restore-capability gate, or storage-local scope registry.
   That same adjacent platform-connections boundary now also owns
   source-oriented `systems[]` grouping on `/api/connections`. Storage and
   recovery may observe grouped source composition to explain whether a
   platform row is collecting additional host telemetry through Pulse Agent,
   but they must not reinterpret attached agents as separate protected
   systems, duplicate recovery inventory rows, or a storage-local ownership
   model. The same shared `/api/connections` contract also owns compact
   `agentIdentity` facts for agent-backed rows; storage and recovery may read
   that metadata, including host-profile ids such as `unraid`, when they need
   to label a represented host, but they must not rebuild OS/endpoint identity
   from recovery inventory or alias heuristics, or reinterpret an agent
   host-profile id as a storage provider platform.
   When that grouped platform row is a Proxmox cluster, storage and
   recovery must also treat the backend-authored cluster moniker as the
   canonical row identity instead of re-expanding cluster-member agents into
   sibling host rows or per-node storage owners. If the grouped row carries
   backend-authored cluster member nodes, adjacent storage/recovery surfaces
   may use that composition for explanatory UI only; they must not promote
   those child nodes into a second top-level grouped-system taxonomy or infer
   per-node storage ownership from the settings payload. The same shared
   `/api/connections` payload also owns any agent-version/update facts and
   fleet-governance posture carried alongside those grouped rows; adjacent
   storage or recovery surfaces may reuse that signal for operator context,
   but must not fork their own version-comparison semantics, desired/applied
   config-drift classifier, rollout-state classifier, credential-health
   classifier, command-policy vocabulary, or another agent lifecycle
   vocabulary. If `/api/connections` reports agent config drift as pending or
   unknown because no trustworthy applied fingerprint exists, storage and
   recovery must preserve that uncertainty instead of translating it into a
   storage-local current/drifted verdict. If `/api/connections` reports
   `configDrift: not-applicable` with a current applied rollout because no
   managed host-agent config override is assigned, storage and recovery must
   preserve that no-rollout state instead of translating it into a pending
   storage/recovery problem.
   When `/api/connections` attaches an exact-match host agent to a blocked
   Proxmox API source without fresh node inventory, storage and recovery must
   treat that as one represented source with host telemetry, not as a second
   protected system or a storage-local duplicate host.
22. Keep backend-native platform actions on the adjacent AI/runtime and platform contracts. When `internal/api/` wires native TrueNAS app control for Assistant, storage and recovery may consume the refreshed recovery points afterward, but they must not grow a parallel recovery-local action transport or action-specific payload shape.
23. Keep backend-native platform diagnostics on the adjacent AI/runtime and platform contracts. When `internal/api/` wires native TrueNAS app log reads for Assistant, storage and recovery may use those diagnostics during investigation, but they must not grow a parallel recovery-local log transport or diagnostic payload shape.
    The same adjacent-boundary rule applies to `GET /api/agents/diagnostics`:
    storage and recovery may read Agent Fleet Doctor evidence as operational
    context for stale agents, version drift, profile drift, or identity split
    investigation, but must not treat that endpoint as a storage/recovery
    health source, repair API, or recovery-local fleet payload shape.
24. Keep backend-native platform configuration reads on the adjacent AI/runtime and platform contracts. When `internal/api/` wires native TrueNAS app config for Assistant, storage and recovery may use that runtime shape during investigation, but they must not grow a parallel recovery-local config transport or provider-shaped configuration payload.
25. Keep provider-backed poll cadence and settings-runtime health on the adjacent platform-connections contract. When shared `internal/api/` and poller wiring expose TrueNAS last-sync status, failure summaries, discovered contribution counts, manual saved-test status refresh, or platform handoff links in settings, storage and recovery may consume the resulting datasets, apps, disks, and recovery artifacts but must not redefine those settings-runtime health semantics or connection-level handoffs in storage/recovery-local transport or page flows.
26. Keep recovery filter/query state on the shared route-state parsing contract without restoring standalone recovery navigation. When platform pages or other embedded owners expose TrueNAS recovery context, they may reuse the canonical recovery query vocabulary with owned `platform` and `node` fields, but they must land inside an owning platform/runtime route instead of inventing drawer-local recovery URLs, treating PBS services as the only recovery path, or sending operators to the retired Recovery aggregate route.
    That same shared route-helper boundary also owns exact workload handoffs
    when storage or recovery surfaces send operators back to node-scoped
    workloads for investigation context. Proxmox VM and system-container links
    must carry the canonical workload identity (`<instance>:<node>:<vmid>`) in
    the shared workload query state rather than an opaque unified resource id,
    so recovery/storage drill-downs reopen the intended platform-owned
    workload drawer instead of landing on an unselected table state.
    Non-storage route constants in that same shared helper, including the
    Patrol control anchor, must stay owned by their product surface and
    must not be reused as recovery entry points or storage/recovery navigation
    aliases.
27. Keep alert-side recovery drill-ins on that same embedded-owner route-state contract. When alert investigation surfaces such as resource-incident panels expose recovery follow-up links for TrueNAS or future API-backed platforms, they must route through an owning platform/runtime destination using canonical recovery query vocabulary instead of freezing alert-local recovery URLs, reviving the retired Recovery aggregate route, or introducing another provider-shaped recovery handoff vocabulary.
28. Keep VMware onboarding runtime and recovery semantics separate on that same adjacent platform-connections contract. When `internal/api/router.go`, `internal/api/router_routes_registration.go`, or `internal/api/vmware_handlers.go` evolve VMware connection CRUD, poller-owned `poll` / `observed` summary payloads, saved-test refresh, or observed datastore/VM snapshot visibility, storage and recovery may consume the resulting shared context but must not treat those onboarding/runtime payloads as canonical recovery artifacts, restore capability, or recovery-local control transport.
29. Keep VMware datastore projection on the shared unified-resource and storage-source contracts. When `frontend-modern/src/hooks/useUnifiedResources.ts` or shared `internal/api/router.go` wiring starts surfacing VMware-backed canonical `storage` resources, storage and recovery may expose those datastores through the owned `vmware-vsphere` source/platform vocabulary for inventory, capacity, and handoff flows only; they must not reinterpret that projection as VMware recovery support, restore semantics, or a provider-local protection surface.
    The same shared unified-resource boundary also covers canonical
    Resource.Uptime fallback on the consumer side. When
    `frontend-modern/src/hooks/useUnifiedResources.ts`'s `toResource`
    mapping extends the uptime fallback chain to land on `v2.uptime`
    after the existing platform-specific carve-outs, storage and
    recovery must treat that fallback as descriptive host/VM uptime
    only; they must not reinterpret it as backup freshness, recovery
    point recency, or protection cadence.
    Docker / Podman `DockerData` container lifecycle, Podman metadata, and
    cumulative block I/O totals remain unified-resource runtime context.
    Storage and recovery may use those fields only as workload description
    when linking to an owning runtime/platform page; they must not reinterpret
    container block I/O totals as backup throughput, recovery-point evidence,
    protection cadence, or storage-health ownership.
30. Keep VMware placement, cluster service state, guest-detail, VM snapshot-tree, VM virtual-hardware configuration, VMware Tools, VM hardware Ethernet, VM hardware disk, and network enrichment descriptive on that same shared unified-resource contract. When `internal/vmware/provider.go`, `internal/unifiedresources/types.go`, and `frontend-modern/src/hooks/useUnifiedResources.ts` project datacenter, cluster, `vmware.clusterHaEnabled`, `vmware.clusterDrsEnabled`, folder, runtime-host, datastore-attachment, guest-hostname, guest-IP, `vmware.currentSnapshotId`, `vmware.snapshotTree`, snapshot creation/state/quiesce/current markers, child snapshot metadata, `vmware.hardware`, virtual hardware version, hardware upgrade policy/version/status/error, boot type/order/retry/setup-mode flags, CPU cores-per-socket and hot-add/remove flags, memory hot-add settings, `vmware.tools`, Tools run state, version status, version number/string, install type, upgrade policy, auto-update support, install-attempt count, guest reboot requests, `vmware.networkAdapters`, adapter MAC address/type, backing network id/name, backing type, connection state, start-connected / guest-control flags, `vmware.virtualDisks`, virtual disk label/type, IDE/SCSI/SATA/NVMe placement, VMDK path, backing type, datastore name, capacity, `vmware.networkType`, `vmware.networkHostNames`, or `vmware.networkVmNames` onto canonical VMware `agent` / `vm` / `storage` / `network` resources, storage and recovery may use that detail for labeling, navigation, and VM investigation context only; they must not promote those topology, cluster-service, guest, snapshot-tree, virtual-hardware, VMware Tools, vNIC, virtual disk, or network fields into recovery ownership, restore targeting, protection grouping, compliance scoring, or a VMware-local recovery taxonomy without a separately governed slice.
31. Keep VMware datastore classification neutral on the shared storage adapter contract. When `frontend-modern/src/features/storageBackups/resourceStorageMapping.ts`, `frontend-modern/src/features/storageBackups/resourceStoragePresentation.ts`, and `frontend-modern/src/features/storageBackups/storageAdapters.ts` evolve canonical storage-record mapping, VMware-backed datastores must continue to land on the shared storage route as inventory-only datastores with neutral protection fallback, not as backup repositories, backup targets, or recovery-protected resources.
    That same shared storage adapter boundary also owns canonical platform
    family vocabulary through the governed platform manifest.
    `frontend-modern/src/features/storageBackups/models.ts`,
    `frontend-modern/src/features/storageBackups/storageAdapterCore.ts`, and
    adjacent shared storage presenters that need known provider ids or
    `onprem` / `container` / `virtualization` / `cloud` family mapping must
    derive that truth from
    `docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json` through
    `frontend-modern/src/utils/platformSupportManifest.ts`, not from
    storage-local hard-coded provider arrays.
    Agent host-profile appliance identity is adjacent context for storage
    rows, not a storage-local platform classifier: Unraid storage may render
    through the generated host-profile label and runtime platform fallback, but
    storage/recovery must not reintroduce `unraid` as a first-class provider or
    recovery platform. Shared `AgentData.hostProfile` may carry that profile id
    for presentation, while storage and recovery must continue to treat
    `AgentData.platform` as the normalized runtime platform.
32. Keep agentless availability endpoints neutral on the shared unified-resource and API contracts. When `internal/api/availability_handlers.go`, `internal/api/connections_handlers.go`, `internal/api/platform_mock_connections.go`, or `frontend-modern/src/hooks/useUnifiedResources.ts` surface `network-endpoint` availability resources, storage and recovery may consume their liveness as infrastructure context only; they must not reinterpret ping/TCP/HTTP endpoints as storage providers, backup targets, recovery repositories, or protected-workload evidence.
    That neutrality includes availability targets whose `targetKind` is
    `machine`. A Linux server, desktop, laptop, or Mac mini monitored by an
    agentless reachability check still belongs to Availability checks rather
    than Standalone Machines; storage and recovery must treat the row as
    liveness context unless a separate storage/recovery-owned relationship ties
    it to backup or restore evidence.
33. Keep infrastructure summary chart bucketing and short response caching presentation-only on the adjacent shared API boundary. When `internal/api/router.go` normalizes mixed-cadence infrastructure history into equal-time summary buckets or serves a cached summary payload for repeated operator-facing summary-card requests, storage and recovery may consume the resulting visual context only; they must not reinterpret those normalized chart samples, cached timestamps, or cache hits as recovery freshness windows, backup cadence, or restore evidence.
34. Keep workload chart downsampling and short response caching presentation-only on that same adjacent shared API boundary. When `internal/api/router.go` caps mixed-cadence workload history into equal-time buckets or serves a cached workload-summary payload for repeated operator-facing workload-card requests, storage and recovery may consume the resulting visual context only; they must not reinterpret those shaped chart samples, cached timestamps, or cache hits as recovery freshness windows, backup cadence, or restore evidence.
    The same adjacent chart boundary now covers compact storage capacity
    transport. `internal/api/router.go` may batch only the canonical `used`
    and `avail` storage series for `/api/charts/storage-summary`, but storage
    and recovery must not treat the omitted `usage` or `total` series as lost
    recovery truth or widen that compact route back into the full storage-page
    payload.
    That same adjacent API boundary also owns summary-request minimization:
    storage/recovery-adjacent consumers may rely on filtered infrastructure or
    guest summary payloads, but they must not widen a scoped chart request back
    into full guest metric fan-out just because adjacent pages carry richer
    detail charts elsewhere.
    In mock mode, that same compact route must stay aggregate-only and
    sampler-prewarmed; storage and recovery must not trigger per-pool chart
    reconstruction on the first dashboard request after each mock refresh.
36. Keep shared `frontend-modern/src/App.tsx` public-route ownership explicit by
    surface. Storage/recovery preview entrypoints such as
    `/preview/setup-complete` may remain public app-shell routes, but unrelated
    commercial compatibility handoffs like `/pricing` must stay separate thin
    route exits rather than borrowing storage/recovery preview framing,
    first-session copy, or page-state assumptions. The same route-ownership
    rule applies to Patrol aliases on the shared app shell: `/patrol` is the
    authenticated canonical surface while retired `/ai` browser entry points
    stay unregistered, and storage/recovery route owners must not depend on or
    borrow that retired path for their own preview or compatibility entrypoints.
    The same route-ownership rule keeps retired self-hosted trial and
    managed-model acquisition banners out of shared app chrome: storage and
    recovery routes must not inherit commercial nudges simply because
    `frontend-modern/src/App.tsx` owns the authenticated shell.
    Cloud signup follows the same boundary: `/cloud` and `/cloud/signup` must
    not return through storage/recovery preview framing or any other ordinary
    self-hosted app route when Cloud acquisition belongs to the account/control
    plane boundary.
    Authenticated `/login` must follow that same shared app-shell contract:
    once login succeeds, `frontend-modern/src/App.tsx` must hand the browser
    back to the frontend-primitives-owned provider-first landing route instead
    of leaving storage/recovery-adjacent authenticated shells on a page-local
    not-found route. Storage/recovery must not redefine Standalone landing
    eligibility or restore legacy Infrastructure as the default
    storage/recovery-adjacent operational surface.
    Authenticated-shell demo organization suppression on `frontend-modern/src/App.tsx`
    may hide top-bar org chrome for public demo posture, but it must not leak
    into storage/recovery preview route ownership, first-session recovery copy,
    or route-level framing decisions.
    Retired `/operations/*` browser entry points are unregistered. They must
    not grow a second authenticated shell boundary that competes with
    storage/recovery route ownership.
    That same shared app-shell boundary must also respect blocking shared
    dialogs: background assistant affordances may hide while a modal owns the
    viewport, but storage/recovery routes must not grow their own parallel
    modal-stack bookkeeping just because they share `App.tsx`.
    App-shell route preloading may include Storage and Recovery modules so
    top-level tabs are warm after authentication, but it must not fetch storage
    summary charts, recovery history, provider state, or preview data from
    `frontend-modern/src/App.tsx` itself. The shared runtime bootstrap must
    likewise avoid prewarming Infrastructure or Workloads chart caches
    as a generic authenticated-shell side effect; storage/recovery chart data
    stays owned by the route or interaction that renders it.
    The shared app shell's authenticated landing and primary-tab routing may
    use the infrastructure navigation model that includes both owning platform
    pages and runtime lenses such as Containers. Storage and recovery route
    owners must treat that as shell-owned navigation evidence, not as a reason
    to restore Storage or Recovery as equal primary tabs or depend on
    platform-only tab terminology.
37. Keep public self-hosted purchase handoff and activation routes on the
    adjacent commercial/auth boundary. When `internal/api/router.go`,
    `internal/api/router_routes_cloud.go`, `internal/api/licensing_handlers.go`,
    or `internal/api/demo_mode_commercial.go` evolve
    `/auth/license-purchase-start` or `/auth/license-purchase-activate`,
    storage and recovery may coexist with
    those shared public-route helpers but must not reuse the commercial-owned
    `portal_handoff_id`, server-resolved checkout intent, purchase-return tokens, activation-bridge
    callbacks, owned billing purchase-arrival states, or demo-hidden
    commercial route policy as recovery identity, restore proof, preview
    framing, or backup/recovery-local transport. The adjacent licensing
    boundary also owns public-vs-Pro runtime build attribution for activated
    installs; storage and recovery surfaces may consume runtime-capability
    blocks, but must not infer paid runtime status from restore context,
    provider inventory, public image tags, or backup transport state. That same
    adjacent commercial
    boundary also owns the canonical self-hosted purchase intent label:
    storage- and recovery-adjacent surfaces may observe `self_hosted_plan`, but
    they must not emit or reinterpret legacy `max_monitored_systems` intent or
    bypass the shared secure callback policy that limits self-hosted commercial
    return URLs to HTTPS instance origins or direct-loopback HTTP and keeps
    hosted commercial follow-up fetches on the restricted outbound client.
    The same adjacent commercial boundary owns the retired
    `/api/upgrade-metrics/*` and `/api/admin/upgrade-metrics-funnel` route
    family: storage and recovery flows must not synthesize recovery-local
    stats, health, config, or funnel-report fallbacks when those routes are
    absent from the normal customer product API.
    That same adjacent commercial boundary also owns the plan-owned callback
    framing for those routes: storage and recovery may coexist beside
    `/settings/system/billing/plan`, but they must treat `Plans` and
    `Plans` as the canonical destination naming and must not
    reinterpret purchase-return bridge titles, retry actions, or success
    states as a recovery-local `Pulse Pro billing` surface.
    query state as a recovery-local contract once uncapped self-hosted
    monitoring is canonical.
    That same adjacent commercial
    boundary treats migrated-v5 monitored-system grandfathering as retired
    compatibility metadata: storage and recovery may tolerate resulting legacy
    entitlement fields while loading old records, but they must not infer a
    capacity floor from protected inventory, backup counts, recovery-point
    presence, billing-status reads, or continuity-verification payloads.
    That same adjacent commercial boundary also owns authenticated
    install-version attribution: storage and recovery may read the resulting
    licensed build context as commercial metadata, but they must not cache a
    second recovery-local version floor, derive restore eligibility from
    activation-version payloads, or backfill release lineage from protected
    inventory when the shared licensing runtime already sends the canonical
    process version.
    That same adjacent commercial boundary also owns internal demo-fixture
    capability handling: storage and recovery may render the resulting demo
    runtime as populated mock inventory, but they must not expose
    `demo_fixtures`, billing identity, or alternate entitlement semantics as
    recovery-local transport or operator-facing storage metadata.
39. Keep storage and recovery route framing additive and owner-neutral.
    `frontend-modern/src/components/Storage/Storage.tsx` and storage/recovery-
    adjacent route composition may use the shared `PageHeader` shell for
    top-level route framing, but that header must stay additive on top of the
    canonical storage page model, recovery presenters, and shared summary
    caches. Header chrome must not become a second owner for storage filters,
    recovery posture, commercial purchase state, or transport selection.
40. Keep the unified connections ledger owner-neutral toward storage and
    recovery. Shared `internal/api/router.go` may mount the
    `/api/connections` and `/api/connections/probe` routes alongside the
    existing storage/recovery-adjacent API surfaces, and
    `internal/api/config_handlers.go` and `internal/api/config_node_handlers.go`
    may carry the new per-instance `Enabled`/`Disabled` round-trip, but
    storage and recovery consumers must not reinterpret the derived
    connection `state` (active/paused/unauthorized/unreachable/stale/pending)
    as storage health, backup-job posture, or recovery verification state;
    they also must not repurpose the shared probe route as a generic internal
    reachability scanner. Metadata, link-local, multicast, and unspecified
    destinations remain fail-closed before dial on that shared route, and any
    future storage/recovery adjacency must preserve that same boundary.
    Storage and recovery UI must keep sourcing those signals from their
    existing canonical page models instead of polling the connections
    ledger for per-datastore or per-backup truth.
    Platform-first top-level pages may embed `StorageSurface` and
    `RecoverySurface` with `embedded tableOnly` and forced source or
    platform filters (e.g. `forcedSourceFilter`, `forcedPlatformFilter`)
    so platform-scoped storage and recovery rows render through the same
    canonical surfaces rather than a forked per-platform table.
    `frontend-modern/src/App.tsx` may carry the platform-page route
    registrations that mount those embedded canonical surfaces, but the
    routes themselves must derive their paths from the canonical builders
    in `frontend-modern/src/routing/resourceLinks.ts`; ad hoc storage or
    recovery route strings inside per-platform features are not permitted.
    Platform-page default sub-tab choices must land the user on a
    canonical surface that actually populates. The canonical TrueNAS
    adapter already emits the top-level TrueNAS system as a unified
    `agent` row tagged with the `truenas` platform, so the platform
    page defaults to `/truenas/overview` (the Systems sub-tab) and the
    embedded `StorageSurface` lives at `/truenas/storage`. The Source
    filter chip in `StoragePageControls` is also suppressed when a
    platform page locks source scope through `forcedSourceFilter` (via
    `suppressSourceFilter`, auto-applied whenever `forcedSourceFilter`
    is set), so the user never sees the platform's name pinned as a
    removable filter chip inside the embedded surface.
    Platform pages that embed `StorageSurface` reuse the canonical
    `StoragePageControls` toolbar through the `showFilterToolbar` prop on
    `StorageProps`. The page keeps `tableOnly` to hide the storage summary
    section but opts in to the shared search, status, group-by, sort,
    node, view, and chart-collapse controls so platform operators get
    dense-table storage controls on every embedded storage tab without
    forking the toolbar. The source scope flows through
    `forcedSourceFilter` as a typed page input; the source filter remains
    available in the toolbar only when not forced.
    Storage filter option semantics stay storage-owned, but FilterBar chip
    presentation is frontend-primitives-owned: storage status leading dots must
    use `filterChipStatusDot` rather than storage-local span factories.
41. Keep agent memory composition descriptive on the shared unified-resource
    contract. `internal/unifiedresources/types.go` carries the reclaimable
    page-cache split (`AgentMemoryMeta.cache`, holding used + cache + free
    within the reported total) as host RAM description for machine surfaces.
    Storage and recovery must not reinterpret that reclaimable RAM figure as
    disk cache, ZFS ARC sizing, storage-tier health, or capacity-planning
    evidence; disk and pool truth stays on the canonical storage and
    physical-disk resources.
    `internal/unifiedresources/types.go` also carries `AvailabilityData`
    (probe protocol, address, latency, failure state) as a resource facet for
    agentless monitoring. Storage and recovery must not treat that availability
    facet as protection status, backup health, or recovery readiness; an
    unreachable probe on a storage resource is monitoring evidence, not a
    backup or recovery failure. Unknown `LastChecked` and `LastSuccess` values
    remain absent optional timestamps on that shared facet; storage and recovery
    must not reinterpret absence or a year-one zero-time serialization as
    recovery age, missed backup cadence, or restore freshness.

## Forbidden Paths

1. Reintroducing storage or recovery product logic as ad hoc dashboard-only summaries without a canonical page-surface owner
2. Duplicating recovery-point normalization or rollup derivation outside `internal/recovery/`
3. Letting storage health presentation rules drift between `frontend-modern/src/components/Storage/` and `frontend-modern/src/features/storageBackups/`
4. Treating storage and recovery as implicit leftovers inside broad monitoring or E2E lanes instead of governed product surfaces
5. Writing internal `NormalizedHealth` values directly to the storage URL status param; the URL must use the canonical option values from `STORAGE_STATUS_FILTER_OPTIONS` (e.g., `available` for the Healthy filter) so that shared links and bookmarks reflect the same values that the filter dropdowns present to operators
6. Letting whitespace-padded storage route params hydrate non-canonical page state; shared storage URLs must trim and normalize `tab`, `source`, `status`, `node`, `group`, `sort`, `order`, `query`, and deep-link `resource` before the page model consumes them so pasted or hand-edited links resolve to the same canonical state as UI-authored routes without dropping adjacent unmanaged params
7. Letting storage `source` aliases or case drift survive in canonical route state; shared storage URLs must rewrite pasted values like `PVE`, `pbs`, or `ALL` to the owned source option values (for example `proxmox-pve`) or the canonical unset state so copied links match the same source filter values the storage toolbar presents
8. Letting explicit storage `all` sentinels survive in canonical route state; shared storage URLs must collapse case- or whitespace-variant `all` values for the managed `node` filter back to the canonical unset state so copied links do not preserve a fake active node filter
9. Letting whitespace-padded recovery timeline params fall off canonical route state; shared recovery URLs must trim and normalize `day`, `range`, `scope`, `status`, `verification`, `cluster`, `node`, `namespace`, `itemType`, and adjacent history filters before the page model validates them so pasted or hand-edited links resolve to the same canonical timeline and filter state as UI-authored routes
10. Letting explicit recovery `all` sentinels survive in canonical route state; shared recovery URLs must collapse case- or whitespace-variant `all` values for `cluster`, `node`, `namespace`, and `itemType` back to the canonical unset route state so copied links do not preserve fake active filters
11. Letting non-canonical recovery platform values survive in route or transport state; shared recovery URLs must collapse unsupported or fake `platform` values back to the canonical unset state, and only owned source-platform options or canonical legacy aliases may reach rollups, points, series, and facets transport filters
    11c. Letting route-owned recovery platform selections disappear while filter options are still hydrating; the recovery page state owner must keep the current canonical `platform` query value present in the platform option set until transport-backed facets and records arrive so shared filter selects keep the user-visible TrueNAS or other owned platform selection instead of flashing back to `All platforms`
    11d. Letting recovery filter default labels drift between protected inventory and recovery events; both recovery filter surfaces must consume the shared `recoveryTablePresentation` labels (`All item types`, `All platforms`, and `Any item`) instead of hard-coding title-case local variants.
    11a. Letting adjacent workload route-state changes in shared `frontend-modern/src/routing/resourceLinks.ts` perturb recovery parse/build semantics; expanding canonical workload platform scoping must not alter the owned recovery `platform` and `itemType` vocabulary, legacy alias rewrites, or recovery drill-down workspace selection
    11b. Letting adjacent storage route-state additions in shared `frontend-modern/src/routing/resourceLinks.ts` perturb recovery route semantics; expanding canonical storage deep links for unified resources must not reuse recovery-owned query names or alter the owned recovery parse/build contract while those surfaces continue sharing the same route-helper module
    11e. Letting adjacent platform-route additions in shared `frontend-modern/src/routing/resourceLinks.ts` perturb storage or recovery route semantics; adding canonical `/standalone/machines`, `/standalone/availability`, or other platform paths must not reuse storage/recovery query names, alter storage or recovery parse/build behavior, or convert agent-platform membership into storage/recovery ownership
    11f. Letting adjacent Patrol control starter route state in shared `frontend-modern/src/routing/resourceLinks.ts` perturb storage or recovery route semantics; `patrolControlStarter=patrol_control` is a first-party Patrol control handoff flag only, legacy `operationsLoopStarter=patrol_control`, `operationsLoopStarter=patrol_autonomy`, and `operationsLoopStarter=pulse_pro_activation` are only compatibility aliases, and storage/recovery parse-build contracts must not consume any of them as recovery state, storage focus, platform scope, or proof of backup/recovery posture
    11g. Letting adjacent Pulse Intelligence external-agent setup anchors in shared `frontend-modern/src/routing/resourceLinks.ts` perturb storage or recovery route semantics; `/settings/pulse-intelligence/assistant#external-agent-setup` is the canonical external-agent setup hash, `/settings/security/api#external-agent-setup` and `/settings/security/api#pulse-mcp-setup` are legacy compatibility hashes, and storage/recovery parse-build contracts must not consume any of them as recovery state, storage focus, platform scope, or proof of backup/recovery posture
12. Letting protected-inventory protection posture overload recovery-event outcome filtering; the protected inventory protection-state control must drive the route-backed `state` field and local rollup posture filtering, while the recovery events `status` field remains the canonical outcome filter for points, series, and facets transport filters
13. Letting visible protected-item filters fall out of shared recovery links; protected inventory state such as stale, failed, warning, running, unknown, healthy, and never-succeeded must restore from the canonical recovery URL and rewrite to the owned `state=<value>` route form, with legacy `stale=1` accepted only as compatibility input
14. Reintroducing stacked full-width recovery tables as the primary desktop layout; the governed recovery surface must expose one primary data region at a time with recovery events as the default workspace and protection coverage as an explicit secondary review so Pulse does not collapse back into a single-platform backup screen
15. Letting secondary recovery workspace state drift out of canonical route state; explicit `view=inventory` protection-coverage links must round-trip, while default recovery-events state should serialize without a redundant `view=events` query unless compatibility input is being normalized
16. Treating a selected protected-item rollup as row-click-only or header-only state instead of a canonical history filter; when a protected-item row focuses recovery history, the governed recovery events controls must surface that focus inside the shared filter surface through the same user-creatable item filter control, count it with the rest of the active filters, and let the same filter reset path clear it
17. Letting recovery-event focus leak as hidden state on the protection coverage surface; opening coverage must clear event-only `rollupId` and `day` context, and coverage drill-ins must open recovery events without preserving an invisible day filter that can make valid history look empty.
18. Letting the recovery details panel lead with transport-shaped payloads; operator-facing details must summarize outcome, artifact, target, restore readiness, and readable metadata labels first, while raw JSON and provider-specific keys stay behind an explicitly technical disclosure.
19. Letting protection coverage navigation bypass the canonical recovery workspace and route-state owner; coverage actions may focus stale inventory, attention inventory, or all protected items, but they must not mutate local-only filter state or revive passive posture-counter cards as the navigation owner.
20. Letting storage or recovery surfaces invoke retired self-hosted trial acquisition; `POST /api/license/trial/start` and `/auth/trial-activate` must stay closed on the ordinary self-hosted router, and storage/recovery-adjacent billing or support handoffs must not treat trial activation as recovery identity, restore proof, or backup transport state.
21. Treating an `AvailabilityData` facet on a storage resource as backup, protection, or recovery evidence; an agentless probe attached to a NAS, PBS, or datastore resource is monitoring reachability, not backup success, snapshot health, or restore readiness. Storage and recovery must read protection status from the canonical backup and storage resources, not from the availability probe facet.

## Completion Obligations

1. Update this contract when canonical storage or recovery entry points move. Routes added under the shared `internal/api/` extension point that are clearly outside storage/recovery ownership (for example `POST /api/ai/patrol/preflight`, the `patrol_preflight` snapshot field added to `/api/settings/ai`, the auto-trigger preflight dispatch on settings save, the startup-seed dispatch in `NewAISettingsHandler`, and the cached-preflight integration into the Patrol `tools` readiness check — all owned by ai-runtime) do not extend this subsystem's contract; they live in their owning subsystem.
   Content-free Pulse Intelligence telemetry rollups under shared
   `internal/api/` are also adjacent-only. Storage and recovery may consume
   underlying recovery artifacts, action outcomes, or Patrol context through
   their owned surfaces, but anonymous action-plan, approval,
   approved-action-decision, rejected-action-decision, external-agent,
   Assistant, or Patrol usage counters are not backup inventory, restore
   capability, recovery freshness, or storage ownership evidence.
   Content-free update funnel counters under shared `internal/api/` are also
   adjacent-only. Update attempts, successes, failures, rolled-back counts, and
   coarse failure categories are release-adoption evidence, not backup
   coverage, storage-health proof, restore readiness, recovery-job state, or
   evidence that any storage or recovery endpoint was read, changed, or
   verified.
   External-agent activity may be counted for narrow tokens that satisfy the
   called manifest capability scope, including read-only context calls. That
   keeps MCP collaboration measurable, but storage and recovery must not treat
   the resulting counter as evidence that a backup, restore, dataset, or
   storage endpoint was read or mutated.
   Approved action decision telemetry may use shared action lifecycle evidence
   or approved approval records, but the exported rollup remains an anonymous
   approve/reject journey counter. Storage and recovery must not reinterpret
   that counter as proof that a backup, restore, dataset, storage appliance, or
   recovery endpoint was approved, changed, inspected, or verified.
   Approved execution attempt telemetry may be backed by shared action
   lifecycle events, including refused-before-dispatch failures, but the
   exported rollup remains an anonymous operations-loop counter. Storage and
   recovery must not reinterpret that counter as proof that a backup, restore,
   dataset, storage appliance, or recovery endpoint was changed or verified.
   Approved action success telemetry may use the same governed audit stream
   only as a content-free count of approved actions that completed
   successfully. The approved execution counter remains attempt-based, and the
   success counter must not export resource identifiers, actor identifiers,
   command text, command output, verification details, backup scope, restore
   proof, or storage appliance state.
   Rejected action decision telemetry may use the same governed audit stream
   only as a content-free count of actions rejected before execution. It must
   not be treated as backup denial, restore denial, storage policy state, or
   proof that a recovery target was inspected or changed.
   The external-agent recent-use counter is backed by content-free authenticated
   agent/MCP capability activity for manifest-capable API tokens; storage and
   recovery must not reinterpret it as proof that a backup, restore, dataset,
   or recovery endpoint was used.
   The MCP adapter recent-use counter is likewise only adapter-origin
   collaboration telemetry for `pulse-mcp` requests. Storage and recovery may
   use the resulting reports for aggregate Pulse Intelligence adoption, but
   must not treat that bit as evidence that a backup, restore, dataset, storage
   appliance, or recovery endpoint was read, mutated, or verified.
   The operations-loop status projection is also adjacent-only. Its content-free
   stage, next-action, Patrol evidence, contextual collaboration, pending
approval, governed action, verified outcome, and optional token-backed MCP
readiness fields may describe Pulse Intelligence activation progress, but storage and
recovery must not treat them as backup coverage, restore readiness, storage
health verification, appliance access, dataset access, API-token authority
for recovery paths, or recovery mutation proof.
If an aggregate active Patrol finding or pending approval outranks older
completed/resolved loop proof in that projection, that precedence remains only
current operator orientation; it is not backup freshness, restore authority, or
storage-local remediation proof.
Operations-loop workflow starter request counts are even narrower: they are
content-free markers that a native Assistant surface rendered, a first-party
Patrol control handoff started, a legacy Pro activation entry-point handoff
started, or a Pulse MCP surface rendered the manifest-owned
`pulse_operations_loop` starter. The aggregate Patrol control starter count may
include native Patrol, legacy Patrol autonomy, and legacy Pro activation starts,
while `proActivationOperationsLoopStarterCount` remains the legacy entry-point
count. Storage and recovery may observe those aggregate activation reports, but
must not treat starter
   access as backup coverage, recovery freshness, restore readiness, storage
   health verification, dataset access, appliance access, or evidence that any
   recovery endpoint was used.
2. Keep recovery store/runtime changes aligned with the storage and recovery frontend proofs in `registry.json`
3. Tighten guardrails when legacy storage or recovery presentation paths are removed
4. Preserve the dependency split: API payload ownership stays in `api-contracts`, settings shell ownership stays in `frontend-primitives`, and canonical resource truth stays in `unified-resources`
   That same adjacent API boundary now includes shared agent-target hostname
   equivalence. Storage- and recovery-adjacent surfaces that reuse
   `internal/api/router_routes_ai_relay.go` or other shared agent lookup
   helpers may match a short host against the same agent's FQDN, but they
   must keep that logic on the canonical
   `internal/unifiedresources/hostname_equivalence.go` contract instead of
   widening it into a broad short-name collapse across distinct FQDNs.
   Approved-action replay through `internal/api/router_routes_ai_relay.go`
   is likewise API/AI-owned transport: storage and recovery may consume the
   resulting incident context, but must not define storage-local approval
   argument keys or bypass the shared `internal/agentcapabilities` helper.
5. Keep recovery history table width budgeting derived from the canonical column specs in `frontend-modern/src/utils/recoveryTablePresentation.ts`, not from raw visible-column counts, so normalized subject labels and optional column sets cannot drift the right-edge badges and controls off-screen
6. Keep at least one browser-level desktop recovery proof in the governed `recovery-product-surface` policy so right-edge column visibility and wrapper-fit regressions are caught at rendered layout time instead of only through unit-level width math
7. Keep the retired dashboard route from becoming a passive no-resources
   compatibility shell. First-session handoff now belongs to Infrastructure
   and Add infrastructure, and storage/recovery must not restore dashboard
   composition just to route operators to setup.
8. Keep storage/recovery summaries on their owning snapshots after dashboard
   retirement. They may reuse the canonical `all-resources` cache key from
   `frontend-modern/src/hooks/useUnifiedResources.ts` only through their owned
   page or shared drawer surfaces, not as a dashboard-only resource snapshot.
9. Keep shared OIDC/SAML callback redirects on the canonical local-target
   helper contract when storage- or recovery-adjacent routes inherit shared
   auth browser handoff through `internal/api/`, so adjacent surfaces do not
   revive per-handler absolute-target acceptance or raw `returnTo`
   concatenation.
10. Keep dependent first-session reset behavior honest on the shared `internal/api/`
    boundary: when `/api/security/dev/reset-first-run` is used to reopen the
    setup wizard in browser proof, the resulting status payload must genuinely
    expose unauthenticated setup so storage/recovery-owned empty-state and
    dashboard handoff proof does not silently fall back to an authenticated
    dashboard path.
11. Keep recovery support claims aligned with
    `docs/release-control/v6/internal/PLATFORM_SUPPORT_MODEL.md`. Forward-
    compatible provider strings are not support declarations by themselves, and
    a platform should be treated as recovery-capable only when that model marks
    recovery as part of its support floor and the owning ingest/projection path
    exists in the same governed slice.
12. Keep runtime mock inventory on the same bounded support contract. When
    `/api/system/mock-mode` surfaces mock TrueNAS pools/datasets or mock
    VMware datastores through shared storage/recovery-adjacent pages, that
    data remains inventory-only context and must not be treated as proof of
    restore capability, recovery artifacts, or widened platform recovery
    support.
13. Keep runtime mock platform context derived from one shared fixture graph.
    When shared `internal/api/` and monitoring wiring surface mock
    storage/recovery-adjacent inventory or recovery artifacts, that data must
    come from the canonical `internal/mock/fixture_graph.go` owner so legacy
    snapshot-backed platforms, provider-backed fixtures, unified inventory,
    and recovery/storage context stay aligned instead of drifting through
    recovery-local fixture assembly or partial mock helper APIs.
14. Keep adjacent shared install-script fallback semantics honest on the
    `internal/api/` boundary. When storage- or recovery-adjacent routes reuse
    shared public endpoint or installer helpers, dev prerelease runtime
    versions such as `v6.0.0-dev` and build-metadata versions must not be
    treated as published GitHub release assets; only stable or explicit RC
    tags may back the shared installer fallback that those adjacent surfaces
    inherit. Published release-tagged local assets on that shared boundary
    should also preserve their detached `.sig` sidecars so recovery- and
    storage-adjacent flows do not silently downgrade installer or agent
    download trust back to unsigned local files during upgrade or repair.
    The served install-script endpoints themselves have no GitHub fallback: they
    serve the locally bundled AGENT installer or fail closed, never proxying the
    top-level GitHub install.sh SERVER installer, so a correct local script
    (signed or not) is always preferred over a wrong-identity proxied one on the
    unverified curl-piped-into-bash agent path (issue #1470).
15. Keep storage summary chart identity and sticky-shell behavior on the
    shared storage path. Pool rows, disk rows, storage summary cards, and
    storage detail charts must all address history through the canonical
    unified-resource metrics-target IDs, and the storage page must reuse the
    shared sticky summary primitive instead of a storage-local scroll wrapper.
    Storage-page pool growth readouts belong to that same contract: the table
    may derive per-pool used-capacity deltas from the shared
    `/api/storage-charts` summary payload, but it must not fan out row-local
    `/api/metrics-store/history` calls, invent a second storage-history cache,
    or drift onto storage-page-only metric identifiers.
    Dashboard storage trends belong to that same owned summary contract: the
    dashboard may derive a 24-hour storage capacity delta from
    `/api/charts/storage-summary`, but it must not rebuild storage summary
    behavior by fanning out per-pool `/api/metrics-store/history` reads, by
    pulling the full storage-page `/api/storage-charts` payload, or by
    inventing a dashboard-only storage history transport.
15a. Keep shared diagnostics cache scope honest when storage/recovery-adjacent
    surfaces reuse `internal/api/diagnostics.go`. The shared diagnostics
    payload must not include local commercial funnel summaries or
    infrastructure-onboarding analytics, so recovery-adjacent diagnostics do
    not inherit commerce telemetry, cross-tenant leakage, or hosted/local
    semantic drift through the shared backend route. Pulse Assistant runtime
    status in that shared payload must remain native Assistant availability
    (`assistantRuntimeConnected`), not MCP transport state, and storage/recovery
    consumers must not reinterpret it as backup coverage, recovery readiness,
    or storage-local action authority.
16. Keep storage summary interaction scoped through the same canonical IDs.
17. Keep adjacent AI settings persistence vendor-neutral on the shared
    `internal/api/` boundary. When storage- or recovery-adjacent hosted flows
    load or save AI settings through shared helpers, any historical hosted
    quickstart model IDs must be cleared before adjacent surfaces read or
    re-emit that state. Legacy Anthropic OAuth tokens follow the same shared-owner rule: adjacent storage/recovery code may not use them as provider configuration and must leave cleanup to the AI settings contract.
17a. Keep adjacent AI paid-control state entitlement-effective on that shared
    `internal/api/` boundary. Storage- and recovery-adjacent flows may preserve
    stored Assistant or Patrol preferences in config, but they must not treat
    stored autonomous, auto-remediation, or alert-triggered analysis settings
    as active restore, recovery, or support capability unless the shared AI
    runtime entitlement clamp exposes them as currently effective.
    AI settings control-refresh callbacks in `internal/api/ai_handlers.go` are
    likewise native Assistant tool-visibility plumbing, not MCP transport
    state and not storage/recovery execution authority.
    When operators hover or focus pools versus physical disks, the storage
    summary must reuse one resolved active-series ID across card state and
    chart highlighting so pool-only cards demote cleanly during disk focus and
    disk-temperature cards demote cleanly during pool focus, instead of
    leaving stale row-local IDs or storage-local hover branches on the page.
    Any page, group, or entity scope that becomes pinned through storage
    interaction must stay row-first: the pinned row or group remains the
    visible scoped state, and explicit clearing belongs to the shared storage
    content-card header action plus the shared `Escape` reset path rather than
    an extra storage-local strip, search-row widget, or filter-bar badge.
    Background whitespace clearing may remain a convenience, but storage must
    not rely on it as the only reversible control.
    When that scope is a storage
    pool group, member pool rows should expose shared
    `data-summary-group-member-active="preview|pinned"` state so the grouped
    block reads as one scoped set without adding storage-local outlines, pill
    buttons, or heavy full-row fills.
18. Keep storage summary remount caches versioned with the chart contract.
    `frontend-modern/src/components/Storage/StorageSummary.tsx` may keep a
    bounded in-memory cache for same-tab remounts, but its cache key must carry
    an explicit summary contract version so long-lived demo sessions do not
    rehydrate stale pool or disk sparkline shapes after the storage summary
    chart model changes.
19. Keep cross-surface workload handoffs on canonical IDs too. Shared workload
    chart transport may look up provider-backed VM history through unified
    metrics targets, but infrastructure/workloads/storage/recovery navigation
    and focus handoffs must stay on canonical workload IDs instead of
    provider-native metric keys.
    The same cross-surface rule applies when recovery-adjacent drawers or
    summaries hand off Kubernetes pod history. Those surfaces may request pod
    metrics only through the unified prefixed target
    `k8s:<cluster>:pod:<uid>` and must rely on API-side canonicalization for
    any legacy bare pod ID instead of inventing a recovery-local pod history
    key.
20. Keep storage row emphasis on the shared frontend primitive contract. Pool
    rows and physical-disk rows that mirror the active summary entity must
    expose that state through `data-summary-row-active` and let the shared row
    presentation owned by `frontend-modern/src/index.css` render the emphasis,
    rather than carrying storage-local sky fill classes that drift from the
    rest of the product or obscure inline capacity bars. Storage pool rows,
    physical-disk rows, and storage group headers must also route pointer,
    and focus preview through
    `frontend-modern/src/components/shared/summaryInteractionA11y.ts`. Pool
    rows and physical-disk rows may keep deliberate expand/pin ownership on
    `frontend-modern/src/components/shared/SummaryRowActionButton.tsx`, but
    storage group headers should pin through the row itself and must not add a
    separate scope/pinned pill button beside the disclosure chevron. Touch
    users still must not inherit synthetic hover branches, and storage must
    not keep a special trailing expand column once the shared leading action
    contract exists.
    Static subgroup header emphasis for storage group rows and recovery history
    day headers must also route through
    `frontend-modern/src/components/shared/groupedTableRowPresentation.ts` and
    the shared `.grouped-table-row` CSS contract in `frontend-modern/src/index.css`,
    rather than storage- or recovery-local background classes or left-accent
    marker variants.
    Storage pool rows must also keep sizing and alert accents on canonical
    class/data-attribute presentation rather than row-local inline style maps,
    so the public storage page stays CSP-safe under both normal and
    alert-highlighted demo/runtime states.
21. Keep recovery transport refreshes inside the recovery-owned feature state.
    `frontend-modern/src/features/recovery/useRecoverySurfaceState.ts` and the
    recovery data hooks may retain the last fulfilled rollups, points, facets,
    and series while the next request is in flight through
    the shared `frontend-modern/src/hooks/createNonSuspendingQuery.ts`, but
    that retained-value behavior must stay route-owned and filter-owned through
    the canonical recovery state model instead of recreating page-local
    suspense escape hatches in `Recovery.tsx` or the recovery sections.
22. Keep storage/recovery-adjacent resource metadata on the shared unified
    resource contract. When canonical storage resources expose provider-backed
    identity such as Proxmox storage `pool`, storage and recovery consumers
    must inherit that field through `frontend-modern/src/hooks/useUnifiedResources.ts`
    and `frontend-modern/src/types/resource.ts` instead of rebuilding backing
    pool identity from labels, paths, or storage-row-local heuristics.
23. Keep storage route writes on the shared route-state scheduler. Storage page
    filter and tab updates may still own their query keys locally, but
    `frontend-modern/src/components/Storage/useStorageRouteState.ts` must route
    same-route replace navigation through the shared
    `createRouteStateNavigateScheduler` helper so back-to-back storage filter
    changes coalesce against the current location instead of reintroducing a
    storage-local timeout queue.
24. Keep storage/recovery-adjacent config-import reload safety on the shared
    `internal/api/` boundary. When storage or recovery setup flows depend on
    `internal/api/config_export_import_handlers.go`, post-import reloads must
    tolerate absent notification managers and other optional runtime managers
    so adjacent browser surfaces inherit a fail-closed API response instead of
    a panic after the archive import succeeds.

## Current State

Unified Agent lifecycle fields added to the shared host and connections API are
adjacent monitoring/API state only. Applied config fingerprints, updater
status, and Host, Docker/Podman, or Kubernetes module readiness do not become
storage health, protection state, recovery points, backup verification, or
restore authorization. Storage and recovery consumers may use the canonical
host identity carried by those payloads, but must not derive recovery semantics
from agent process readiness or update success.

Authentication-cookie, SSO configured-file, deployment concurrency, and cloud
handoff redirect hardening in shared `internal/api/` routes is adjacent
API/security work only. It creates no storage-health, recovery-point, backup,
restore-authorization, or provider-coverage semantics. Analyzer-visible cookie
sink separation and constant-capacity allocation refinements preserve that
boundary and introduce no storage persistence or recovery contract.

Denied Patrol investigation-fix approvals passing through shared
`internal/api/` handlers are adjacent AI-runtime/action-governance state only.
The `fix_rejected` finding outcome means an operator declined a proposed Patrol
fix before execution; it must not become protection state, recovery-point
state, backup verification state, or storage/recovery-local remediation
semantics.

Default-org token scoping and notification-settings fan-out on shared
`internal/api/` handlers are likewise adjacent only: they are
api-contract/security owned and create no storage, recovery-point, or
backup-surface semantics.

The shared PVE setup-script SMART wrapper remains a storage/recovery dependency
only for disk-temperature evidence. Storage surfaces may depend on its explicit
`-d sat` and `-d scsi` retries for active direct Linux SATA/SAT-style disks, but
they must not fork a storage-local disk-temperature collector or replace the
API-owned setup-script contract. Storage physical-disk rows also depend on the
unified-resource disk contract preserving Proxmox node/instance metadata and
SMART capacity when Proxmox inventory and host-agent SMART telemetry merge;
storage/recovery consumers must read that canonical merged physical-disk
resource rather than rebuilding a local Proxmox/S.M.A.R.T. join.

Notification webhook management changes on shared `internal/api/` handlers are
likewise adjacent only: the webhook `signingSecret` payload field and its
masking semantics are notifications/API-contract owned and create no storage,
recovery-point, or backup-surface semantics.

SSO provider-detail payload changes on shared `internal/api/identity_sso_handlers.go`
are API-contract/security-settings owned. Nested OIDC/SAML edit fields,
restriction lists, role mappings, and masked secret-presence markers create no
storage health, recovery-point, backup verification, restore authorization, or
provider-coverage semantics.

Alert delivery diagnosis on shared `internal/api/alerts.go` is likewise
adjacent only: `/api/alerts/delivery-diagnosis` exposes alerts/API-contract
read-only notification-policy evidence and creates no storage health,
recovery-point, backup verification, restore authorization, or provider
coverage semantics.

Kubernetes pod metadata decoded by `frontend-modern/src/hooks/useUnifiedResources.ts`
is shared inventory context for storage/recovery handoffs only; Pod phase,
container readiness, owner, image, and restart fields do not become protection
state or recovery-local workload taxonomy.

The alert payload the router-wired alert bridge (`internal/api/router.go`,
`internal/api/ai_handlers.go`) now carries into a scoped patrol — metric type,
value, threshold, resource identifier, level, and message — is read-only
investigation context for that single run. It must not be persisted as
protection state, recovery-local workload taxonomy, or a backup/recovery
artifact; storage and recovery state stays owned by their canonical surfaces.

The Storage and Recovery cross-jump builders
(`buildStorageHrefForResource`, `buildRecoveryHrefForResource`) were deleted
from `frontend-modern/src/routing/resourceLinks.ts` on 2026-05-16 alongside
the platform-first migration. The chip strips that previously rendered them
inside the alert resource-incidents panel and Patrol findings panel were
retired in the same pass; storage and recovery drilldowns now stay inside
the platform-page sub-tabs rather than offering external surface jumps.
Future cross-surface storage or recovery affordances must compose against
the embedded `StorageSurface` / `RecoverySurface` consumers rather than
reintroducing top-level URL builders.
The remaining storage and recovery route builders in
`frontend-modern/src/routing/resourceLinks.ts` are query-state serializers, not
destination builders: callers must append `buildStorageRouteSearch()` or
`buildRecoveryRouteSearch()` to the current platform-owned pathname such as
`/proxmox/storage`, `/proxmox/backups`, or `/truenas/protection`. They must not
emit `/storage` or `/recovery` as hidden compatibility paths.

Storage and Recovery can now be embedded by a platform page in table-only mode
with a forced platform source/filter. Proxmox uses that embedding for
source-scoped storage and recovery history, while the TrueNAS Protection tab
uses the same Recovery surface with a forced `truenas` platform filter and the
protection-coverage workspace as its default entry point. The embedded mode
suppresses standalone page chrome, summary charts, and full filter chrome, but
the Storage surface must keep the canonical Storage / Physical Disks view
selector inside the table header unless the embedding explicitly locks a
`forcedView`, and the Recovery surface must keep protection/events workspace
state in `frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`.
The canonical route-backed filter state, fetch builders, table rendering, and
storage/recovery vocabulary remain owned by the Storage and Recovery surfaces.
Platform pages must compose those owners rather than cloning storage pools,
physical disks, recovery events, or protected-inventory tables under
platform-specific data contracts.

The investigation enrichment path reads operator-state from the
in-memory provider already wired against the durable
`resource_operator_state` SQLite table, so an operator's
commitments survive across restarts on the investigation read path
the same way they do on the suppression read path — both flow
through the same provider over the same durable table.

The agent SSE stream at `/api/agent/events` is in-memory and
stateless. No persistence; each connection starts fresh from the
moment of subscribe. Agents that need to catch up across reconnects
fetch the read endpoints (findings list, approvals list, audit
list) for replay — those are the durable surfaces. The
`approval.pending` and `action.completed` events fire after the
canonical record (approval row or action-audit row) has already
been persisted, so a missed event is recoverable by reading the
backing endpoint; the stream is a doorbell for recent activity,
not the source of truth. Heartbeat events are stream-local
keepalives and do not create recovery records, action-audit rows,
or storage/recovery freshness evidence. The verification projection on
`action.completed` reads from the same persisted
`ActionAuditRecord.Result.Verification` field that agents would
recover from /api/actions/{id} after a reconnect — the event
carries a copy, not the original, so the durable record remains
the canonical source.
Storage/recovery consumers must treat the event vocabulary as a shared
API/AI-owned contract from `internal/agentcapabilities`. Event names and
transport markers may be used to distinguish doorbells from keepalives, but
storage/recovery must not define local event-name registries or infer recovery
freshness from `stream.connected` or `heartbeat`.

The agent capabilities manifest at `/api/agent/capabilities` is
read-only and stateless — no persistence is involved. The manifest
is hand-authored in `internal/agentcapabilities/manifest.go` and served by
the API handler; storage flows are not affected.
Its action mode and approval policy metadata are API/AI-owned governance
posture for agent tool selection. Storage and recovery may observe that
metadata when agents explain available tools, but must not reinterpret it as
backup ownership, restore capability, recovery freshness, or storage-local
action authority.
Manifest `inputSchema` metadata is likewise an API-owned agent argument
contract. It may help an external agent call action or finding lifecycle tools,
or replace operator-state, but it does not introduce storage/recovery
persistence, restore authority, or backup-specific mutation semantics.
When that manifest exposes provisioning tools over `/api/discover`
and `/api/config/nodes`, storage and recovery may observe the resulting
configured sources and backup evidence only after the canonical node
lifecycle writes complete. Discovery candidates, credential test
payloads, token or password secrets, and add/update/remove source
commands remain API/agent-lifecycle onboarding concerns, not recovery
state, backup ownership, restore entitlement, or storage-local
credential material.

The action governance loop endpoints (`/api/actions/plan`,
`/api/actions/{id}/decision`, `/api/actions/{id}/execute`) joined
the agent surface but introduce no new persistence. Plan, decide,
and execute all write to the same `ActionAuditRecord` /
`ActionLifecycleEvent` durable storage the action audit store
already manages; the only delta is the wire shape returned to
clients on error (now the agent-stable envelope rather than the
platform-wide `APIError` shape). Recovery posture is identical:
when the action audit store rehydrates on startup, the action
endpoints recover the same lifecycle records they always did.

The Patrol finding lifecycle endpoints advertised in the agent
capabilities manifest (`acknowledge_finding`, `snooze_finding`,
`dismiss_finding`, `resolve_finding`) follow the same storage boundary:
their agent-facing stable error envelope is an API/AI-runtime wire
contract only and is shared through `internal/agentcapabilities`.
Successful calls still mutate the existing Patrol finding store, unified
finding store, and learning feedback store exactly as the UI path does; no
MCP-specific, agent-specific, or recovery-specific persistence is introduced
by the manifest error-code declarations or by the shared error envelope.

The agent-consumable bundled context endpoint
`/api/agent/resource-context/{id}` reads the same durable
`resource_operator_state` table and `action_audits` table through the
canonical `unified.ResourceStore` accessors, and additionally
filters the in-memory approval store (durably persisted to the
`approvals.json` file the approval store already manages, hydrated
on startup). No new persistence is introduced for the
pending-approvals section either; the bundle is still a read-only
bundle over existing storage, so storage/recovery flows that
rehydrate the unified-resources store and the approval store
already cover everything the agent endpoint surfaces.

The fleet view at `/api/agent/fleet-context` introduces no new
persistence either — it walks the registry once and reads the same
durable `resource_operator_state` table per resource, the same
in-memory findings store, and the same in-memory approval store.
Fleet pending-approval counts may be grouped by canonical resource
id from one bounded approval-store scan, but that is still a
read-only projection over `approvals.json`, not a new recovery
artifact or storage/recovery freshness signal. The companion
`/api/agent/resource-capabilities/{id}` endpoint is the same kind
of read-only projection: it returns the in-memory
`Resource.Capabilities` slice the registry already assembles, so
no new persistence or recovery artifact is introduced. The
optional additive filter query params on fleet-context
(hasFindings, severity, technology, resourceType) likewise
introduce no new persistence: they only narrow the in-memory
registry walk, so the recovery posture is unchanged. The recovery posture
is identical to the per-resource bundle: when the unified-resources
store and the approval store rehydrate on startup, the fleet view
recovers the same situated picture without any fleet-specific
rehydration step.

The Patrol-control status endpoint at
`/api/agent/patrol-control/status` introduces no new persistence. The legacy
`/api/agent/operations-loop/status` URL remains a compatibility alias. It reads the
registry, active findings, pending approval counts, recent action-audit records,
and recent action lifecycle events over the existing evidence window, then
returns only aggregate stage state and counts. Lifecycle events may rehydrate a
recent approval or rejection for a plan created before the window, but the
projection still resolves the event back to the canonical action audit before
counting governance or verification; approved and rejected decision counts stay
separate so a rejected-only decision can complete the no-execution branch
without being mistaken for verified remediation. The four-step operator rollup
follows the same adjacent-only evidence boundary: governance step counts may
reflect pending approvals or later decision evidence, active aggregate Patrol
findings may keep the next action on current operator work, the Assistant step
count may reflect contextual collaboration, and verification step counts may
reflect verified outcomes or terminal rejected decisions. Optional MCP readiness
stays in `externalAgentReady`, but storage and recovery must not treat those
counts or readiness as backup freshness, restore authority, or storage-local
remediation proof. The approved-success telemetry predicate follows that same
boundary: execution success alone is not verified outcome proof unless the
approved action also carries `VerificationOutcome.Status=verified` or a
canonical verification result that ran and succeeded, and storage/recovery
surfaces must not use those Patrol-control values as backup, restore, or
protected-state evidence. Recovery posture is therefore identical to the underlying
stores: once unified resources, findings, approvals, action audits, and
lifecycle events have rehydrated, the status projection is available without an
Patrol-control specific recovery artifact.

The agent capabilities manifest at `/api/agent/capabilities` is
hand-authored static data. There is no persistence to recover —
the manifest is a constant compiled into the binary; on every
restart it serves identical content as soon as the HTTP listener
is up. The endpoint sits in the router's `publicPaths` list so
the global auth middleware does not gate it, which makes
discovery available before any bootstrap-token / first-run flow
completes.

The findings runtime reads operator-set state through the same
durable `resource_operator_state` SQLite table on every
new-finding-add. Both the time-bounded maintenance window and the
indefinite `IntentionallyOffline` flag persist across restarts; the
operator commitment in either form survives without needing a
re-entry on startup. The provider adapter returns one projection
covering all signals so the storage path is touched once per
finding regardless of which signal is active.

The `resource_operator_state` SQLite table introduced by the
unified-resources store keeps operator-set per-resource intent
(intentionally offline, never auto-remediate, maintenance window,
criticality) durably alongside the rest of the unified-resource
durable state. The `/api/resources/{id}/operator-state` API surface in
`internal/api/resources_operator_state.go` reads and writes that table
through the canonical store, so storage / recovery flows that rehydrate
the unified-resources store also rehydrate operator-set state — a
maintenance window persists across restarts the same way action audits
and resource overrides do.

The patrol findings-recovery sync in `internal/api/router.go` also keeps
the will_fix_later wake-up deadline alongside the rest of the finding's
durable state when re-hydrating findings from disk into the unified
store. Persisted `Finding.RemindAt` values must round-trip through that
recovery path so an operator commitment recorded before a process
restart is not silently dropped when findings reload.
That same recovery path also preserves the operator-vs-Pulse
attribution captured in `Finding.AutoResolved`. The router boundary
must copy that flag onto `UnifiedFinding.AutoResolved` during both
live wire-up and persistence resync so a finding the operator
manually closed before a restart still reads as "Resolved by you"
afterward, instead of being misattributed to Pulse's auto-detection.

`StorageSummary.tsx`, `StoragePageSummary.tsx`, and `useStoragePageSummary.ts`
now surface `poolsDegraded` and `disksFailing` health indicators alongside
pool/disk counts. These additions project from existing websocket pool/disk
state; they must not introduce new API polling or widen the storage-fetch
boundary.
Agentless availability endpoints are adjacent infrastructure context only, not
storage or recovery inventory. Storage/recovery consumers may receive
`network-endpoint` resources through shared unified-resource snapshots, but
they must not fold those endpoints into protected-item counts, storage health
rollups, recovery evidence, or storage/recovery licensing or readiness
messages unless a separately governed storage/recovery relationship is added.
The agentless `machine` target kind does not change that boundary: it is
availability presentation vocabulary for computer-shaped reachability targets,
not Standalone Machines membership, storage ownership, repository membership,
backup coverage, or restore authority.
That same owned summary path now also runs through
`useStorageSummaryCharts.ts`: the storage page owns one page-scoped summary
range and one shared storage-summary history fetch, and both the sticky
summary cards and per-pool growth column reuse that payload instead of
forking separate row-local history reads or duplicate polling loops.
Storage physical-disk requirements copy now consumes the shared
`frontend-modern/src/utils/infrastructureSettingsPresentation.ts` Settings
Infrastructure target label. Disk-health guidance may refer to Proxmox node
requirements, but it must not revive removed nested Pulse settings paths such
as `Settings → Infrastructure → Proxmox`.
Recovery item-type labels now route through
`frontend-modern/src/utils/recoveryItemTypePresentation.ts`. Recovery surfaces
must not render a bare `Cluster` item type for Kubernetes protected subjects;
use the canonical `K8s Cluster` label so protected inventory and event filters
do not confuse Kubernetes clusters with Proxmox clusters or table grouping.

This subsystem now sits under the dedicated storage and recovery lane so the
operator-facing storage page, recovery timeline, and recovery-point persistence
engine stop hiding inside broader monitoring and E2E buckets.
That same first-session recovery boundary also treats the bootstrap token as a
local secret, not a log artifact. Storage and recovery surfaces may surface the
bootstrap token file path when first-run auth is missing, but automatic runtime
logs must never print the bootstrap token value itself. That same recovery
surface must also keep bootstrap token validation rate-limited per client so
the local recovery transport does not become an unbounded online guessing path.
Storage and recovery browser helpers now also keep one transport-tolerant
normalization edge. Recovery display models must accept legacy subject-label
fields and nullable mode/kind metadata before presenting canonical item labels,
while storage detail drawers and filter controls must route summary series IDs,
source tones, and disk metrics through the shared storage helpers instead of
reconstructing them from local table state.
Storage and recovery may depend on the adjacent Patrol-control status
projection staying content-free, including its Patrol control starter count,
completed/resolved loop counts, `patrolControlValueState`, legacy
`patrolAutonomy*` aliases, and legacy `proActivation*` compatibility fields
when shared API helpers are touched. The legacy Pro activation starter field is
entry-point-specific, while the legacy completed/resolved/value fields mirror
Patrol control values. Those values are classified by the shared
`internal/telemetry` Patrol control proof helper and are not recovery
verification, storage health, backup coverage, restore readiness, or
action-outcome proof, and storage/recovery surfaces must not use them to imply a
protected or resolved state. Approved-success telemetry is likewise
Patrol-control proof only when the action has verified post-action evidence;
successful command completion without canonical verification remains outside
storage/recovery readiness.
Storage and recovery's adjacent `internal/api/` contract must also preserve
the product-facing remediation vocabulary used by shared API denials. When
storage/recovery-adjacent browser sessions encounter AI or Patrol remediation
license responses while sharing the app shell, those API messages must use
safe remediation wording and must not revive `Auto-Fix` as customer-facing
paid copy.
That same adjacent `internal/api/` router boundary now also keeps usage-data
transport descriptive-only for storage and recovery. Shared storage/recovery
surfaces may coexist with `/api/upgrade-metrics/*` config reads and telemetry
preview routes under the licensing/settings router, but they must not treat
local-only upgrade-event toggles, telemetry preview payloads, or normalized
release-classification fields as storage freshness, recovery evidence, or
operator-facing protection state.
That same shared `internal/api/` dependency now also expects replacement-aware
monitored-system grouping and fail-closed preview availability. Storage- or
recovery-adjacent setup, deploy, and API-backed update helpers may reuse the
canonical monitored-system grouping boundary, but they must preserve remaining
grouped sources on a monitored host, must not reinterpret unavailable usage as
an empty estate, and must not surface that adjacent boundary as license-slot,
capacity, or upgrade-plan copy inside storage or recovery-adjacent flows.
Configured Proxmox, PBS, and PMG node replacements on that adjacent API
boundary must identify the replaced source-owned surface through the shared
monitored-system replacement selector, not storage- or recovery-local matching
rules, so a platform host edit preserves non-replaced grouped evidence before
storage or recovery consumes the resulting runtime context.
That same adjacent `internal/api/` boundary now also governs public-demo
commercial redaction for storage and recovery viewers. Shared storage/recovery
surfaces may run beside a demo runtime that has real internal entitlements,
but `DEMO_MODE` must still 404 license-status, billing-state, and monitored-
system-ledger reads so adjacent recovery or storage pages do not leak
commercial identity or upgrade posture into a public demo. Storage/recovery
must consume that redacted boundary as presentation truth rather than
reintroducing mock-only license bypasses or page-local commercial fallbacks.
Browser-facing storage/recovery surfaces must also treat
`/api/security/status` as the canonical public-demo bootstrap contract. The
backend capability fact remains `sessionCapabilities.demoMode`, but storage
and recovery browsers must consume the shared resolved `presentationPolicy`
instead of inferring demo posture from headers, `/api/health`, or hostname
heuristics.
Shared licensing routes under `internal/api/` may retain legacy
`upgrade-metrics` names for local commercial handoff telemetry, but storage and
recovery surfaces must continue to consume the presentation policy instead of
using those route names as a cue to render paid history prompts in ordinary
self-hosted sessions.
Legacy-named hosted entitlement verifier wiring under shared `internal/api/`
is the same kind of boundary-only compatibility: storage and recovery surfaces
may consume the resolved hosted entitlement, but they must not infer trial
acquisition, restore identity, or recovery-progress state from
`TrialActivation*` names or the retained
`PULSE_TRIAL_ACTIVATION_PUBLIC_KEY` literal.
That same shared boundary now also owns the one runtime-safe exception:
storage and recovery may inherit demo-safe `/api/license/runtime-capabilities`
reads for capability and history-retention truth, but
`/api/license/commercial-posture`, `/api/license/entitlements`, and
`/auth/license-purchase-start` stay hidden and those surfaces must not expect
licensed identity, upgrade prompts, trial urgency or eligibility reasons,
checkout handoff state, or observed usage counts to remain present once the
public-demo contract is applied.
That same runtime-safe exception now also keeps monitored-system capacity
posture absent. Storage/recovery surfaces may keep demo-safe capability and
retention truth from `/api/license/runtime-capabilities`, but they must not
expect `monitored_system_capacity`, admission-freeze copy, or observed plan
overage to exist.
Storage/recovery consumers must also treat paid-runtime block records in that
payload as runtime executable truth only. A `paid_runtime_required` block may
explain why a licensed private Pro hook is unavailable in the community
runtime, but it must not become recovery protection state, restore identity,
capacity posture, or storage-history entitlement proof.
Storage detail surfaces with page-local history selectors must also treat that
runtime retention truth as the selector contract: pool and disk detail ranges
must filter and clamp through the storage-owned range access helper so ordinary
self-hosted users see only usable history windows, while Relay/Pro longer
history remains available only when the runtime capability advertises it.
That same adjacent commercial boundary also owns one-time checkout-return
lookup. Storage and recovery may coexist with the shared purchase return routes
in the app shell, but they must not cache, derive, or replay the
server-resolved portal checkout state or owned billing purchase-arrival state
as recovery route state, restore evidence, or storage-local navigation
context. The same rule now also covers purchase-start failures: storage and
recovery surfaces may coexist with the shared `/auth/license-purchase-start`
route, but they must not absorb `purchase=unavailable` as recovery-local state
or replace the owned billing retry/recovery notice with storage-specific error
chrome.
That same adjacent licensing boundary now also owns internal demo-fixture
runtime gating for storage- and recovery-adjacent surfaces. Release builds may
authorize mock fixture rewiring only through the backend-owned
`demo_fixtures` entitlement, but storage and recovery browsers must continue
to consume the redacted public runtime/commercial contracts and must not infer
internal fixture grants or persisted mock state from those shared licensing
routes.
Storage- or recovery-adjacent commercial helpers must therefore wait for the
shared presentation policy to resolve before attempting any read that could
otherwise hit a hidden commercial route during bootstrap.
Physical-disk live I/O drawers now also sit on the canonical storage surface.
Storage disk drawers may show read, write, busy, and SMART history, but every
chart must route through the shared `HistoryChart` API contract using the disk
resource's canonical history target. Storage must not keep a drawer-local live
metrics collector, agent-id/device fallback stream, or separate real-time
history store once monitoring and `/api/metrics-store/history` already own the
disk timeline.
Storage pool and disk detail range selectors must mirror the shared history
chart entitlement sequence. They must expose `14d` between `7d` and `30d` and
pass the selected range through to `HistoryChart` unchanged, rather than
inventing storage-local range catalogs, paid-tier labels, or alternate
metrics-history gating.
The storage pool detail's ZFS Pool card is the canonical home for
device-level ZFS health.
`frontend-modern/src/features/storageBackups/storagePoolDetailPresentation.ts`
builds the pool summary (state, scan activity, pool error totals) and the
per-device report (name, vdev type, state, R/W/C error counts, message) from
the record's `details.zfsPool`, which
`frontend-modern/src/features/storageBackups/storageAdapters.ts` and
`resourceStorageMapping.ts` resolve meta-first from canonical
`storage.zfsPool` with flat `platformData.zfsPool` fallback.
`frontend-modern/src/components/Storage/StoragePoolDetail.tsx` renders that
report inside the row expansion so degraded pools name the failing device and
running scrub/resilver without re-promoting per-device noise into table rows.
The pools table must not double-list Ceph-backed storage. Cluster-internal
pool rows synthesized from Ceph cluster telemetry (`models.StorageFromCephPool`:
type `ceph` homed on the `cluster` pseudo-node) are consolidated into the PVE
storage rows that mount them by
`consolidateCephClusterPoolRecords` in
`frontend-modern/src/features/storageBackups/cephRecordPresentation.ts`,
applied by `frontend-modern/src/components/Storage/useStorageModel.ts` before
filtering, sorting, and summary.
The pools table column set is Storage / State / Type / Host / Protection /
Usage / Growth at the platform-standard 32px row height
(`STORAGE_POOL_ROW_HEIGHT_CLASS`). Source platform identity is not a table
column: every live embedding forces a single platform
(`forcedSourceFilter`), so a per-row Source badge degenerates into repeated
noise (the same failure 809e2c900 removed from the disks table). The
record's source platform lives in the row expansion's Configuration card
(`buildStoragePoolDetailConfigRows`' `Source` row) and stays filterable
through the FilterBar source chip on unforced embeddings. Backup-repository
host labels must resolve through name-bearing candidates (parent name, PBS
instance name, owning PVE node, storage nodes) before falling back to raw
parent/platform resource ids, so PVE-configured PBS storage rows show the
owning node instead of opaque `agent-`/`storage-` identifiers. The pool row's worse health is lifted onto
the surviving mount row (state, status detail, issue summary) so a degraded
cluster stays visible, and pool rows with no mounting sibling are kept so
clusters monitored without PVE storage entries do not lose their only
capacity row. Raw pool accounting stays on the Ceph tab's cluster drawer,
which remains the canonical home for per-pool stored/available bytes.
Shared chart transport that storage and recovery coexist with must also stay
on rendered-metric budgets. When `internal/api/router.go` batches workload
history for adjacent overview or shared summary cards, it may parallelize the
provider reads, but it must not widen the shared hot path to disk read/write
or fetch-all metrics just because storage or recovery also mount nearby chart
shells.
That adjacent shared chart transport may also expose host-agent or Proxmox node
CPU temperature as `metric=temperature` for node drawers. Storage and recovery
may consume the surrounding context, but they must not reinterpret that
agent/node CPU temperature history as physical-disk SMART temperature, backup
freshness, restore evidence, or a storage-owned thermal timeline.
The same boundary applies to host-agent `thermalState`: macOS pressure and
throttling limits may appear as host context, but storage and recovery must not
reinterpret pressure state as disk temperature, pool risk, backup freshness, or
a storage-owned thermal timeline.
That same boundary applies to typed GPU host sensor metadata carried through
`internal/unifiedresources/types.go`: GPU temperature, utilization, and VRAM
readings may appear as descriptive host context, but storage and recovery must
not reinterpret those values as disk cache, storage-tier health, backup
freshness, restore evidence, or protection readiness.
That same boundary applies to host-agent `powerWatts` metadata carried through
`internal/unifiedresources/types.go`: wattage readings may appear as
descriptive host context, but storage and recovery must not reinterpret those
values as disk cache, storage-tier health, backup freshness, restore evidence,
power-protection evidence, or protection readiness.

Storage and recovery still consume the shared unified-resource contract, but
they do not own the timeline store itself. The canonical resource-change
history now lives in `internal/unifiedresources/store.go` and is surfaced
through the shared API/resource wiring, which keeps storage and recovery focused
on presentation and query shape rather than re-implementing change persistence.
That same shared `internal/api/` dependency now also assumes AI settings stay
vendor-neutral on that boundary. Storage- or recovery-adjacent settings pages
may coexist with AI controls, but they must keep consuming the canonical AI
settings payload rather than reviving storage-local provider defaults, modal
setup logic, or route-specific BYOK model guesses when shared handlers change.
The retained-value recovery transport helper is now shared too.
Recovery still owns when rollups, points, facets, and series refetch, but the
non-suspending query primitive itself now lives under the shared frontend
primitives contract so other governed surfaces can reuse the same app-shell
fallback boundary without forking it.
That same shared-resource dependency now also assumes frontend compatibility
normalization collapses any legacy top-level `truenas` payload into canonical
`agent` plus `platformType: 'truenas'` before shared route or filter logic
runs. Storage and recovery links may consume that normalized platform truth,
but they must not preserve a second top-level `truenas` type contract in
storage/recovery-local route, handoff, or filter code.
That same shared `internal/api/` dependency now also assumes Assistant-facing
resource transport behaves the same way: any legacy top-level `truenas`
resource or mention type that still reaches shared AI handlers must collapse
to canonical `agent` before storage/recovery-adjacent links, filters, or
drill-ins consume it, so those surfaces never inherit a second live host-type
contract from chat or alert investigation ingress.
Assistant finding-briefing action metadata assembled from recovered Patrol
handoff action references is also AI/runtime review context only. Storage and
recovery may consume that explanation as incident context,
including safe approval status, request/expiry timestamps, action plan identity,
approval policy, plan expiry, and dry-run posture, but they must not reinterpret
a recovered approval or clearer action artifact metadata as backup freshness,
restore authority, recovery proof, or storage-local remediation execution
state. Those handoffs must remain context-only for the configured model and
must not be converted by storage/recovery code into pre-filled prompts,
suggested prompt chips, or recovery-owned next-step instructions.
Patrol run `handoff_metadata` retained for saved Assistant sessions is also
AI/runtime review identity only. Storage and recovery may display or link from
the safe run ID, run type/status, runtime-failure flag, or scoped resource label
as incident context, but they must not reinterpret it as backup freshness,
restore proof, storage health authority, or recovery-local remediation state.
Patrol finding handoffs that force approval-required Assistant mode from a
non-empty `finding_id` follow the same adjacent API boundary: storage and
recovery may treat the resulting Assistant session as incident context, but
must not reinterpret the finding ID or approval-required chat mode as backup
freshness, restore authorization, storage remediation permission, or recovery
transport state, and must not treat the context-only handoff as a
storage/recovery-authored diagnostic prompt.
Proxmox VM QEMU guest-agent outage incidents follow the same incident-context
boundary. Storage and recovery may display or link from the resulting
`availability_unreachable` resource incident when explaining why a VM lacks
fresh guest telemetry, but they must not reinterpret `guestAgentStatus`,
`guestAgentExpected`, or source `qemu-guest-agent` as backup freshness, restore
eligibility, protection status, or recovery ownership.
Patrol queued-fix approvals that seed shared action-audit records follow the
same rule: storage and recovery may display the resulting action history as
incident-adjacent context, including the requester identity that distinguishes
Patrol-origin proposals from generic Assistant work, but they must not treat the
pending action state, approval policy, requester, or preflight posture as
recovery proof or storage-local execution permission.
Backend-refreshed Assistant handoffs may recover the same requester identity
from a live approval record before action audit hydration, but storage and
recovery still consume it only as incident-adjacent provenance.
That same storage ownership also includes the shared storage-source presentation
contract in `frontend-modern/src/utils/storageSources.ts`: storage pages and
cross-surface storage links must reuse one canonical ordering, label, tone, and
default-option model for sources like PVE, PBS, Ceph, and TrueNAS instead of
re-sorting or re-presenting those source options locally.
Storage filter option labels for grouped views, node/host filters, sort
controls, and source selectors are also canonical presentation contracts:
storage surfaces must consume `frontend-modern/src/components/Storage/storagePageState.ts`
and `frontend-modern/src/utils/storageSources.ts` rather than re-declaring
page-local title casing or alternate all-option labels. The storage toolbar may
own sort/filter semantics, but native select label/id/chrome and dynamic value
sync must come from the frontend-primitives-owned `FormSelect` rather than a
storage-local raw `<select>` wrapper.
Recovery all-history, all-item-type, and all-platform defaults follow the same
shared filter-option contract through
`frontend-modern/src/utils/recoveryTablePresentation.ts`, so recovery history
and protected-item tables do not invent separate default-filter wording.
Physical-disk role and group filter defaults plus disk-type display labels
must likewise come from `frontend-modern/src/features/storageBackups/diskPresentation.ts`;
storage pages must not reintroduce local `All Roles`, `All Groups`, or
`NVME Disk` strings that drift away from the shared filter-label and hardware
acronym presentation contract.
That same storage ownership also includes the physical-disk detail identity
contract in `frontend-modern/src/components/Storage/` and
`frontend-modern/src/features/storageBackups/`: historical disk charts must
resolve through the canonical disk metrics target when one exists, then fall
back to stable hardware identity, and operator-facing fallback copy must
describe that identity gap instead of prescribing agent installation on
API-backed platforms like TrueNAS.
That same storage surface must also read the canonical physical-disk risk
payload as its disk-health truth. When API-backed platforms such as TrueNAS
raise SMART-backed disk incidents, those reasons must surface through
`physicalDisk.risk.reasons` so storage rows and disk detail use the same shared
disk-health contract instead of depending on incident-only side channels.
Linked-disk health indicators on storage pool detail rows keep the same owner
split: storage derives the health semantics through
`getLinkedDiskHealthDotVariant`, while the shared frontend primitive
`StatusDot` owns the visible dot chrome, size, color-token, and aria behavior.
Storage components and `features/storageBackups` presentation helpers must not
recreate raw green/yellow rounded-dot classes locally.
That same storage page ownership now also includes contextual focus behavior
for pools and disks. Expanding a storage row may set a focused metrics-target
ID for shared summary emphasis, but `frontend-modern/src/components/Storage/StorageSummary.tsx`
must keep the storage summary page-scoped instead of collapsing its sparklines
to the single expanded row or replacing the page overview with row-local empty
states.
That same page-scoped summary contract now also owns canonical hover-isolation
behavior. Pool and disk rows must publish the resolved metrics-target ID into
the shared summary contract so pool usage, used capacity, and available space
cards can isolate the active row through the shared sparkline primitive while
non-matching cards such as disk temperature demote to inactive context instead
of rebuilding a row-local summary surface.
That same shared summary contract now also owns chart-driven emphasis.
Hovering one storage summary chart must promote the same canonical metrics
target ID through sibling cards, so pool charts cross-highlight the same pool
while non-matching cards such as disk temperature demote to inactive context
instead of keeping chart-local hover state. When a sibling storage card can map
that same entity into its own series set, it must also surface the synchronized
value as a compact card-header readout instead of opening a second floating
tooltip away from the pointer.
That same storage summary contract now uses the shared contextual-focus owner.
`frontend-modern/src/components/Storage/StorageSummary.tsx` must route
interactive-series filtering, focused-label lookup, and active-series
resolution through `frontend-modern/src/components/shared/contextualFocus.ts`
so storage keeps the same page-scoped focus semantics as infrastructure and
workloads instead of preserving a storage-local hover/focus branch.
That same storage ownership now also governs summary-to-table reveal. Hovering
pool or disk charts may highlight the matching row when the active view already
shows it, but storage hover must not auto-filter or auto-scroll the table.
When the active chart entity is off-screen or hidden behind the other storage
view, the page must use the shared summary-table focus bridge and reveal the
target row only through an explicit `Jump to row` action, switching views or
expanding the owning group only for that deliberate reveal path.
That same reveal contract now also owns inline-detail expansion. When a pool or
disk row is deliberately focused and its inline detail opens on the storage
page, the detail row must publish the same canonical summary series ID through
`data-inline-detail-for`, and the shared contextual-focus helper may still
reveal only enough of that drawer to show the row header plus the top of the
detail instead of reverting to storage-local centering or a second row/detail
ID map. Storage should not fork that behavior into a fully in-place
infrastructure-style shell handoff unless a separately governed product model
changes the storage interaction contract.

The recovery backend is a real product boundary, not just a helper package:
`internal/recovery/` owns per-tenant SQLite persistence, rollup derivation,
query filtering, and recovery-point indexing for the `/api/recovery/*`
surfaces.
That same recovery boundary now also assumes mock recovery context is projected
from one canonical mock graph. `internal/mock/recovery_points.go` may synthesize
inventory-only recovery artifacts for supported mock platforms, but those
subjects must derive from the shared `internal/mock/fixture_graph.go` owner
instead of a separate hardcoded recovery cache, so recovery filters, rollups,
and shared route handoffs see the same platform set as settings and
infrastructure.
That same graph-owned mock boundary also owns demo-readiness for storage and
recovery surfaces. Mock summary cards, seeded history, and provider-backed
storage/recovery counts must come from the same canonical fixture graph so
storage and recovery demos show realistic healthy-versus-attention balance
instead of blank history, stale provider context, or page-local fixture drift.
That same recovery-facing demo contract also owns subject readability. When
mock recovery points project inventory-only Kubernetes PVC protection, the
subject identity shown to operators must stay human-readable
`<cluster>/<namespace>/<pvc>` context from the canonical graph instead of
opaque hash-like IDs that break demo trust and cross-surface recognition.
That same adjacent chart boundary now also assumes seeded and live mock
storage timelines are one continuous series. Disk-temperature, pool-usage,
used-capacity, and available-space cards may consume shaped chart payloads for
presentation, but those payloads must still reflect one canonical mock metric
timeline instead of a seeded seven-day sparkline with a second live tail
stitched on afterward.
That same chart boundary now also owns row-hover summary filtering. Storage
pool and disk rows may focus the summary cards, but the shared storage summary
must filter every supported card through the same canonical metrics-target
identity rather than letting temperature, capacity, or detail cards drift onto
page-local row identifiers.
That same shared `internal/api/` dependency also assumes auth-persistence
teardown is synchronous when recovery-adjacent runtimes reinitialize. Session,
CSRF, and recovery-token workers may not leave stale background goroutines or
half-shutdown path ownership behind, because hosted handoff, recovery
inspection, and adjacent temp-path tests all depend on the same canonical
runtime data-dir authority being replaceable without hangs or leaked state,
and router teardown must close the exact session, CSRF, and recovery-token
workers that router initialized instead of assuming a later global auth-store
binding will clean them up.
That same shared lifecycle discipline now also applies to Assistant approval
store cleanup when `internal/api/ai_handler.go` is touched from shared router
work. Approval persistence must not bind its cleanup loop to one request-scoped
context and silently disappear after a settings save, because recovery- and
storage-adjacent runtime proofs depend on the same owned backend lifetime model
instead of opportunistic request ownership for long-lived background workers.
That same runtime data-dir authority also assumes file-backed stores keep
canonical filenames opaque and machine-owned. Recovery-adjacent session,
knowledge, and discovery records may discover legacy identifier-derived files
only for migration, and the next successful write must replace those legacy
paths with hashed canonical names so operator-controlled identifiers do not
become durable filesystem path segments.
That same hosted handoff dependency also assumes the exchange path authorizes
tenant org access before redirecting the browser into protected routes.
Recovery- and storage-adjacent hosted pages that open immediately after
control-plane handoff must see a real tenant member session backed by a
pre-existing owner/member record, not a freshly minted browser cookie created
by appending missing members or upgrading roles from the handoff token.
Missing tenant membership, blank-owner orgs, and role-escalation claims must
all fail closed before protected recovery routes load.
That same shared `internal/api/` organization boundary also assumes self-hosted
org access changes require invited-user consent before recovery-adjacent
routes treat the operator as a tenant member. Recovery settings and related
storage surfaces may observe `/api/orgs/{id}/members` mutations, but manager
submissions for a new `userId` must stay pending invitations until the
invited account explicitly accepts. Recovery-adjacent owner transfer therefore
remains restricted to existing members, may not be satisfied by an unaccepted
invitation record or a guessed account identifier, and must require a fresh
browser session minted for the acting owner before the permanent ownership
change is accepted.
That shared `internal/api/` dependency now also assumes hosted tenant AI
bootstrap and chat-runtime reads resolve through one effective hosted billing
lease before storage- or recovery-adjacent runtime consumers inspect
assistant availability, so recovery points, restore guidance, and related
operator surfaces do not read a tenant-org AI readiness state that diverges
from the machine-owned hosted entitlement already governing the instance.
That same shared `internal/api/` dependency also now assumes hosted runtime
websocket upgrades trust the cloud proxy only through explicit tenant
`PULSE_TRUSTED_PROXY_CIDRS` wiring, so storage- and recovery-adjacent live
status surfaces do not fall into reconnect loops after a hosted workspace
handoff. That shared proxy-trust boundary must also reject wildcard trust
ranges such as `0.0.0.0/0` or `::/0` at startup, and storage/recovery-adjacent
forwarded-header reads must fail closed if invalid wildcard proxy trust
configuration is present.
That same shared `internal/api/` dependency also assumes telemetry
transparency stays on its governed system-settings trust surface. When shared
router or config-system files move under storage- or recovery-adjacent work,
telemetry preview and install-ID reset routes must keep reusing the canonical
system-settings boundary and the server-owned telemetry runtime instead of
being treated as generic storage/recovery transport fallout.
That same shared `internal/api/ai_handlers.go` dependency also now assumes
Patrol-specific AI settings and status transport stay isolated from
storage/recovery product state. When shared AI handlers add split Patrol
trigger-source fields, scoped-activity recency, or queued-trigger status,
recovery queries, storage links, and recovery-adjacent setup flows must treat
those as Patrol-only runtime facts rather than inheriting them as recovery
verification or storage-health transport.
That same shared helper layer also now assumes the Pulse Mobile relay runtime
credential reaches only the explicit backend-owned route inventory, so
storage- and recovery-adjacent transport work cannot accidentally widen that
credential into a broader AI access bundle by touching neighboring routes.
The recovery frontend now also separates that ownership more explicitly:
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts` owns
canonical route parsing, filter/query state, transport hook inputs, and URL
synchronization, while `frontend-modern/src/components/Recovery/Recovery.tsx`
is the composition root for the operator-facing recovery surface and the split
section owners under `frontend-modern/src/components/Recovery/` hold the
protection coverage, activity, and history presentation layers. The history
surface is further split so `RecoveryHistorySection.tsx` owns the toolbar and
controller boundary, `useRecoveryHistorySectionState.ts` owns local section UI
state, and `RecoveryHistoryTable.tsx` owns the row/detail renderer.
That composition root now also owns one primary recovery workspace rather than
stacking protection-coverage and event-history tables on the same desktop page.
The governed default is event-first so operators land on concrete
backup/snapshot/replication history, while the header action and compatibility
`view=inventory` links open the secondary protection coverage review.
That same operator-facing workspace must lead with current protection status
rather than only the latest backup outcome. Protection coverage should surface
stale, never-succeeded, failed, warning, and running rollups as the primary
monitoring status so an item with an old successful point does not scan as
healthy when it still needs operator attention.
That same workspace contract also keeps Pulse's provider-neutral recovery model
explicit in the page language: recovery sections should talk about protected
items, recovery events, and latest points so PBS backups, TrueNAS snapshots,
Kubernetes artifacts, and future providers all fit the same first-class UI
frame without removing the source badges and row-level cues that make Proxmox
operators productive.
Operator-facing filter and detail labels should likewise prefer `platform`
wording over implementation-facing `provider` wording, so the recovery surface
describes the monitored platform families Pulse covers rather than exposing
backend transport vocabulary as the primary UI model.
That same operator-facing vocabulary should also prefer `item` over backend
`subject` wording, and `platform` over generic `source` wording, across the
primary recovery headers, tables, filter controls, and detail metadata labels.
The data model can keep its internal subject/provider fields, but the page
frame that operators read should present one consistent protected-item and
platform model from summary through drill-in. Shared recovery URLs and
transport filters should likewise treat `platform` as the canonical
operator-facing query field, with legacy `provider` aliases accepted only as
compatibility input that rewrites back to canonical `platform` route state.
Shared recovery link builders should therefore accept canonical `platform`
inputs only; legacy `provider` belongs at parse-time compatibility boundaries,
not in new caller-facing recovery route helpers.
Cross-surface recovery drill-in links must also target the correct primary
workspace without relying on legacy inventory-first defaults. When a platform
service surface such as PBS links into recovery activity, that shared entry
point should land on the default recovery events workspace and describe the
destination as recovery events rather than reverting to PBS-backup wording.
That same recovery contract should keep response payloads canonical as well:
recovery points and protected rollups should expose `platform` and
`platforms` as the primary transport fields, while any legacy
`provider` / `providers` aliases stay compatibility-only so the page does not
silently drift back to backend-shaped vocabulary during decode.
That same shared recovery table contract should keep its runtime column model
canonical as well. Recovery inventory and event-history columns should use
`item` and `platform` identities rather than preserving `subject` and `source`
as the primary runtime model, and any saved legacy column IDs must migrate at
the shared column-visibility boundary instead of forcing recovery renderers to
carry deleted column identities indefinitely. Once that migration exists,
recovery tables and shared table presenters should not continue accepting
legacy `subject` and `source` ids in the live runtime path.
That same runtime-helper contract should prefer `item` terminology in shared
recovery presenters too. Helper exports that resolve labels or item-type badges
should expose canonical item-facing names, while any retained `subject` aliases
remain compatibility wrappers instead of the primary runtime boundary.
That same shared badge contract applies to table rendering too. Recovery item
type cells should use the same compact monitoring-table badge base that
workloads uses for `VM` and `Container`, rather than copying only the colors
and drifting on padding or visual weight.
The same rule applies inside recovery-owned helpers and selectors. Shared
summary helpers and platform filter renderers should use canonical `item` and
`platform` naming internally once compatibility boundaries already exist,
rather than keeping fresh `subject` or `provider` terminology alive in the
live recovery runtime path.
The same runtime vocabulary rule applies to cross-section recovery props too.
Live page-to-section boundaries should carry item-focused names like
`selectedHistoryItemLabel` instead of preserving `subject` labels after the
shared recovery presenters already expose canonical item terminology.
That same rule applies to recovery detail helpers. Provider-specific helper
names like `isPbsProvider` should become platform-specific helpers like
`isPbsPlatform` once the runtime recovery model is already canonically
platform-first.
The same canonical boundary applies to linked-resource identifiers. Recovery
API payloads, query filters, and normalized frontend runtime models should use
`itemResourceId` as the canonical field while accepting or emitting
`subjectResourceId` only as a compatibility alias during the transition.
That same canonical boundary also applies to external item references. Recovery
API payloads and normalized frontend runtime models should use `itemRef` as
the canonical item-reference field while treating `subjectRef` only as a
compatibility alias during the transition.
That same presenter boundary should also own canonical item-type derivation.
Recovery surfaces must resolve rollup and point item types through one shared
item-type helper instead of repeating `display.itemType` / `subjectType` /
`subjectRef.type` fallback chains across state, summary, details, and table
renderers.
That same recovery-store decode boundary must fail soft on malformed persisted
metadata. If a stored recovery row contains bad `subject_ref_json`,
`repository_ref_json`, or `details_json`, the list endpoints should log and
drop only the malformed derived field for that row rather than returning `500`
for the entire recovery points or rollups surface.
That same fail-soft contract also applies to downstream consumers that reuse
those shared store reads, including recovery-backed reporting, alert rollups,
and tenant-scoped AI recovery-point adapters. A malformed metadata blob may
degrade row-local enrichment, but it must not take down adjacent readers that
consume the same canonical recovery store.
That same recovery-store migration boundary must keep legacy schema upgrades in
dependency order. When a persisted `recovery.db` predates columns such as
`item_type`, the store must add the migrated columns before creating indexes or
running query paths that reference them, so opening a legacy store backfills
cleanly instead of returning `500` from `/api/recovery/points` or
`/api/recovery/rollups` during schema initialization.
That same recovery-store key boundary must keep `subject_key` genuinely stable
across ingest generations. Protected rollups must not split one Proxmox guest
into stale and fresh rows just because older points stored legacy linked IDs
like `lxc-*` or raw source IDs while newer points carry hashed canonical
resource IDs, and proxmox guest external keys must ignore display-name churn so
renaming a backup comment does not fork the protected inventory from recent
event history. That same store-owned continuity contract also applies when
Proxmox PBS guest points temporarily lose unified-resource linkage or drift
between historical PBS namespaces: if recovery history already proves one
canonical linked guest identity for the same friendly label, guest type, and
VMID/CTID, the store must relink later unresolved PBS points and backfill older
split rows onto that canonical protected item instead of leaving protected
inventory freshness to disagree with recovery events.
That same hook-boundary normalization also owns the runtime recovery display
model. Canonical recovery points and rollups must expose `display.itemLabel`
and `display.itemType` to recovery consumers, while legacy transport fields
such as `subjectLabel` and `subjectType` remain decode-only compatibility
aliases in the shared normalization layer instead of leaking into runtime
presenters.
That same canonical item-label boundary must prefer recognizable protected-item
names over raw entity IDs. When unresolved Proxmox-backed recovery points only
have a VMID/CTID in the subject ref but still carry a richer backup comment or
notes label, the canonical recovery index and store backfill must promote that
human-readable label into the persisted subject/item label instead of leaving
protected inventory rows to lead with bare numeric IDs.
That same operator-facing row-identity rule should still preserve the governed
entity identifier as secondary context when it exists. Recovery inventory and
event rows should lead with the canonical item name, then show a muted
secondary compact `VMID`/`CTID`/`ID` cue when `display.entityIdLabel` is
available, so operators can disambiguate familiar names without turning the
primary scan path back into raw numeric identifiers or bloating the table with
an extra recovery-only identity row.
That same shared presentation layer also owns the distinction between
aggregate recovery-method language and single-record recovery-method language.
Timeline legends and daily breakdowns must use aggregate labels such as
`Snapshots`, `Local Copies`, and `Remote Copies`, while event rows, filters,
and point details must use the singular operator-facing forms `Snapshot`,
`Local Copy`, and `Remote Copy`. Recovery point detail summaries must also
humanize backend fields like kind, mode, outcome, and boolean state into
operator-facing labels such as `Point Type`, `Method`, `Outcome`, `Verified`,
and `Encrypted` instead of leaking raw transport values like `backup`,
`remote`, or lowercase outcome tokens into the primary drawer surface.
That primary workspace selection now also lives in canonical recovery route
state through `frontend-modern/src/routing/resourceLinks.ts` and
`frontend-modern/src/features/recovery/useRecoverySurfaceState.ts`, so copied
links and browser restores reopen explicit protection-coverage state instead
of silently falling back to page-local UI state. Focused recovery routes with
an active `rollupId` or `day` remain recovery-events-first by default, and
default event routes should omit redundant `view=events` query state.
That same shared route-helper contract now also has to preserve exact storage
and recovery handoffs for unified resources discovered outside the storage or
recovery pages. When alerts, Patrol, or infrastructure drawers route a
TrueNAS-backed disk, app, or system into an owning platform/runtime surface,
the shared helper must keep the owned `source`, `node`, `platform`, `view`,
and exact `resource` semantics intact instead of collapsing those handoffs
back to provider-local URLs, retired aggregate workspace routes, or generic
top-level tabs.
That history table layout now also derives its minimum width from the same
canonical column-width spec that owns the header sizing in
`frontend-modern/src/utils/recoveryTablePresentation.ts`, so longer governed
subject labels do not force the trailing outcome/status columns off-screen by
budget drift.
That same recovery product proof surface now also includes a browser-level
desktop layout guard in `tests/integration/tests/17-recovery-layout.spec.ts`,
which opens the recovery page against deterministic recovery payloads and
fails when the history table needs horizontal scrolling or lets the outcome
column drift outside the visible wrapper at desktop width.
That same shared `internal/api/` dependency now also assumes tenant-scoped
resource handlers seed registries from canonical unified resources only:
recovery- and storage-adjacent API helpers may not fall back to raw tenant
`StateSnapshot` seeding once `UnifiedResourceSnapshotForTenant` is available.
That same shared `internal/api/` dependency now also assumes tenant AI
handlers stay on canonical Patrol runtime wiring: recovery- and
storage-adjacent API helpers must not revive tenant snapshot-provider bridges
through `internal/api/ai_handlers.go` once Patrol can initialize from tenant
`ReadState` and unified-resource providers directly.
That same adjacent AI handler boundary now also keeps Patrol runtime
availability explicit as API-owned state. Storage and recovery consumers may
share the handler layer, but they must not treat a blocked Patrol runtime as
healthy only because the last completed summary snapshot remained green.
That same shared dependency also assumes the Patrol-backed recent-changes
API surface reads through the canonical intelligence facade first, so
storage and recovery handlers do not bypass the shared unified timeline
through the older detector-only path.
The same shared boundary applies to the Patrol-backed correlation API
surface, which must read through the canonical intelligence facade before it
exposes learned relationship context to adjacent storage and recovery flows.
The same shared API runtime also exposes unified-resource action, lifecycle,
and export audit reads, but storage and recovery must continue to treat that
as adjacent governed API ownership rather than timeline-store ownership. The
storage and recovery lanes still own their own persistence and query
contracts, while the control-plane execution trail remains governed by the
unified-resource and audit contracts.
That adjacent audit-read surface also now requires the dedicated `audit:read`
token scope rather than the broader `settings:read` scope, so storage- and
recovery-adjacent settings visibility cannot silently expand into enterprise
audit history reads.
That same shared `internal/api/` dependency now also includes monitored-system
ledger explanation reads: storage- and recovery-adjacent surfaces may coexist
with counted monitored-system inventory, but any support-facing count
reasoning must come from the canonical unified-resource grouping explanation
payload rather than from storage or recovery heuristics.
That same shared hosted-entitlement refresh path must preserve historical
quickstart fields only as billing-state compatibility. Storage- and
recovery-adjacent hosted tenants may share Patrol-backed investigation and
recovery context with the rest of the app, but the shared `internal/api/`
lease refresh must not turn old quickstart inventory into active AI runtime
state or lead adjacent product surfaces to infer a hosted-model entitlement.
Recovery-adjacent code must not compensate for missing AI provider setup by
fabricating local activation state or a quickstart-backed runtime.
That adjacent ledger read must also preserve canonical grouped system status,
including `warning`, so recovery- and storage-adjacent support views do not
flatten governed degraded state into a fake `unknown` label when the shared
unified-resource resolver already computed the top-level status.
That same adjacent ledger read now also carries backend-owned status
explanation copy, and support-facing details must render it beside the
counting rationale so operators can interpret warning, offline, and unknown
states without inventing page-local status wording.
The same API resource serializer also refreshes canonical identity and policy
metadata through the shared unified-resource helper before it writes resource
payloads, so storage and recovery links inherit the same canonical metadata
pass instead of carrying local attach wrappers in adjacent transport code.
The shared unified-resource facet bundle that storage-adjacent detail views
consume now also carries grouped `recentChangeKinds` counts by canonical change
kind, so storage and recovery surfaces can show the distribution of restarts,
anomalies, relationships, and capability changes without re-deriving their own
timeline breakdowns.
That same facet bundle may include the selected resource's canonical
capabilities and relationships for shared detail drawers, but storage and
recovery surfaces must treat that topology/action metadata as adjacent
API/unified-resource context rather than storage protection, restore, or
recovery ownership.
Derived parent relationships in that facet bundle are still topology context:
storage and recovery may render or link them through the shared resource
drawer, but they must not reinterpret those parent edges as backup coverage,
restore eligibility, or recovery-event evidence.
That same shared facet bundle now also carries grouped
`recentChangeSourceTypes` counts by canonical source type, so storage and
recovery surfaces can separate platform events, pulse diffs, heuristics,
user actions, and agent actions without inferring provenance from the loaded
slice.
That same shared facet bundle now also carries grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so storage
and recovery surfaces can separate Docker, Proxmox, TrueNAS, and ops-helper
provenance without inferring integration origin from the loaded slice.
Those same resource timeline records also preserve `relatedResources`
relationship context for non-relationship changes, so storage and recovery
views can still link neighboring resources when the timeline entry is a
restart, anomaly, or config update rather than only when the edge itself
changes.
Those unified audit list endpoints also clamp oversized `limit` requests to
the governed maximum, so adjacent recovery and storage workflows do not turn
bounded history reads into unbounded collection scans.
The adjacent enterprise audit-log read path now also preserves structured
store-failure codes (`audit_store_busy`, `audit_store_unavailable`) instead of
generic 500s. Storage and recovery surfaces may coexist with that API layer,
but they must not reinterpret audit-store health as backup, restore, or
recovery-state evidence.
The same shared API runtime now also exposes dedicated
`/api/resources/{id}/timeline` reads plus the bundled
`/api/resources/{id}/facets` surface, but storage and recovery must continue
to treat those as adjacent governed API ownership rather than storage/recovery
timeline ownership.
That same adjacent API layer now also exposes a VM inventory CSV export for the
reporting surface. Storage and recovery workflows may consume similar current-
state VM facts, but `internal/api/reporting_inventory_handlers.go` and
`internal/api/router_routes_licensing.go` remain API/reporting transport
ownership rather than storage/recovery contract ownership.
That adjacent reporting transport now also includes a reporting catalog route
whose nested VM inventory definition owns panel copy, stable column schema,
and filename prefix. Storage and recovery flows may read those facts when they
need fleet context, but they must not fork their own reporting or inventory
column contract.
That catalog route is intentionally metadata-readable without the
`advanced_reporting` feature gate so locked admin reporting shells can stay on
the same API-owned definition before upsell; storage- and recovery-adjacent
surfaces must not treat that metadata visibility as permission to execute paid
report/export routes.
That same API-owned performance-report definition also governs transport-side
validation and attachment naming. Storage and recovery flows may consume those
downloads, but they must treat allowed formats, multi-resource caps, optional
metric/title support, default fallback range windows, attachment filename
stems, and invalid-format validation copy as API/reporting contract rather than
rebuilding local reporting constants.
That same transport contract also owns report time-window validation. Storage-
and recovery-adjacent flows may omit `start`/`end` to use the canonical default
window, but when they provide either bound it must be RFC3339 and `end` must
not be earlier than `start`; invalid values fail as `400 invalid_time_range`
instead of silently shifting the exported reporting window.
That same adjacent API/reporting transport also owns the optional reporting
field limits and multi-report request parsing. Storage and recovery consumers
must treat `metricType`, `title`, request-body size, unknown JSON fields, and
trailing payload rejection as API-owned validation semantics rather than
counting on permissive backend coercion.
Those same transport rules now also carry explicit failure modes that adjacent
storage and recovery automation must preserve: bad or oversized multi-report
payloads fail as `invalid_body` or `body_too_large`, and overlong report
windows or invalid optional fields fail through the API-owned reporting
validation contract instead of being clipped or normalized locally.
That same API-owned contract also classifies those validation failures with
stable error codes, so storage and recovery flows must not derive behavior by
inspecting human-readable error text from adjacent reporting calls.
That adjacent export contract now also includes canonical Proxmox pool
membership for each VM row. Storage and recovery flows may use those current-
state facts when they need fleet context, but they must consume the API-owned
pool column rather than rebuilding pool membership from storage-side queries.
Those resource timeline reads now also accept governed kind and source-type
filters plus source-adapter filters, with filtered history counts owned by the
unified-resource store so storage and recovery views can consume the same
canonical history contract without re-deriving their own timeline slices.
Those same dedicated timeline and facet reads are relationship-aware at the API
boundary: storage and recovery detail views may consume direct changes plus
changes whose `relatedResources` names the current canonical resource, but they
must not rebuild a storage-local cross-resource timeline join or widen the
direct-only history default used by non-resource-detail callers.
Invalid `sourceAdapter` values are rejected at the API boundary, which keeps
storage and recovery reads aligned with the canonical adapter set instead of
turning the timeline filter into an arbitrary free-text escape hatch.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so tenant-scoped storage and recovery
pages do not hit a missing-provider 500 before the monitor is fully wired.
The shared unified-resource consumer hook now also preserves `recentChanges`,
`facetCounts`, `policy`, and `aiSafeSummary` fields when storage and recovery
surfaces read unified resources, so those pages see the same control-plane
timeline facets and recent-change totals as the dedicated resource drawer
instead of flattening them away locally.
That shared policy payload now remains intentionally minimal as well: storage
and recovery consumers should expect only routing scope and redaction hints;
the cloud-summary decision is derived from scope rather than stored as a
separate boolean flag.
The same storage-facing runtime paths now also normalize org scope through
`frontend-modern/src/utils/orgScope.ts` before building cache keys or
multi-tenant fetch state, so Dashboard, StorageSummary, and other storage
adjacent consumers do not each keep a local `getOrgID() || 'default'`
fallback.

The frontend storage and recovery surfaces are also first-class embedded
runtime entry points. Platform page route shells own the path, while
`frontend-modern/src/components/Storage/Storage.tsx` is the operator-facing
storage surface owner, and `frontend-modern/src/components/Storage/` plus
`frontend-modern/src/features/storageBackups/` define the storage health model
and presentation. The retired standalone storage page shell must not be
recreated. Storage/recovery empty and detail cards that use the compact
bordered surface frame must compose the frontend-primitives `InfoCardFrame`
contract, including `INFO_CARD_FRAME_CLASS` from storage presentation
constants, rather than recreating the frame string in storage-local helpers.
The storage page's readiness now stays route-owned as well:
`frontend-modern/src/components/Storage/useStoragePageModel.ts` and
`frontend-modern/src/features/storageBackups/storagePageStatus.ts` must derive
loading, reconnect, and disconnect presentation from the storage unified-resource
fetch contract before consulting websocket churn so the storage surface does
not present healthy REST-backed data as down or stale.
Meanwhile,
`frontend-modern/src/components/Recovery/` and the recovery hooks define the
event timeline, protection-coverage review, and recovery-history UX. The
governed page frame is event-first: operators land directly on concrete
backup/snapshot/replication history, with `RecoveryActivitySection.tsx` acting
as compact orientation for the visible event table. The former decorative
recovery summary-card strip is retired because workloads already owns the
object-level "has a backup" scan, while Recovery owns concrete recovery events
and an explicit secondary protection-coverage review.
That top recovery frame must rely on solid elevated operator panels and border
hierarchy rather than decorative gradients so the page reads like a monitoring
workspace instead of a marketing-style dashboard shell. The page must stay
compact enough to keep the activity chart and primary event table in the first
scroll window instead of stacking dashboard-like slabs above the work surface.
Protection coverage remains a secondary review, opened through the explicit
header action or canonical compatibility route state when the operator needs to
audit stale, failed, warning, running, unknown, or never-succeeded items. It
must expose a reciprocal page-header action back to Recovery events, so opening
coverage never leaves the operator dependent on the global Recovery nav item or
small table chrome to return to the primary page. It must not return as an
equal-weight subtab or a top-level posture-card action strip without a separate
governed product decision.
Recovery is also intentionally outside the interactive page/group/entity
summary-card contract used by workloads, infrastructure, and storage. The
recovery route must not adopt `summaryCardInteraction.ts`, synchronized card
hover, row-driven summary scope, or shared `SummaryPanel` card framing simply
because adjacent monitoring pages use those primitives. Recovery may still show
coverage breadth and platform context inside the Protection coverage table and
event filters, but the page must read item-first so the unified recovery model
is not visually anchored to one platform family.
That same item-first rule also applies to the protection coverage table:
`RecoveryProtectedInventorySection.tsx` must surface protected item type as a
first-class column in the main inventory grid rather than leaving platform as
the only structural classifier beside the item label. Platform badges remain
important supporting operator context, especially for Proxmox-heavy fleets,
but the table frame itself must make protected item class explicit.
That same inventory contract must keep the protected-items grid operationally
bounded. The governed desktop recovery surface should not dump the entire
protected estate into one endless slab; it should page or otherwise bound the
primary inventory table so the workspace, filters, and adjacent activity panel
remain readable as one monitoring surface instead of dissolving into a raw list
dump.
That same protected-inventory surface should carry compact operator orientation
inside the table shell itself. `RecoveryProtectedInventorySection.tsx` should
expose the current bounded range, page, and sort state near the primary grid,
and the first column should carry enough secondary item metadata to read as a
monitored inventory row rather than a bare export line.
That same hierarchy rule also applies to the activity timeline. The governed
recovery surface should not append `RecoveryActivitySection.tsx` underneath the
default protected-items view as if trend telemetry were a second page bolted
onto inventory. The timeline owns recovery-event day selection, so it belongs
inside the `Recovery events` workspace and should read as history analysis for
the selected window rather than as a second copy of the page-level posture
summary.
The same owned vocabulary applies to recovery events as well:
`frontend-modern/src/utils/recoveryTablePresentation.ts` must keep the
history-table `type` column labeled as `Item Type` within recovery surfaces so
event history does not fall back to a generic shared `Type` header once the
recovery lane has already established item-first operator vocabulary.
That same item-first vocabulary must carry through the point-details drawer:
when a recovery point includes canonical item-class metadata,
`RecoveryPointDetails.tsx` must surface it as `Item Type` in the summary grid
instead of jumping directly from item identity to platform and point-method
metadata.
That same shared presentation layer also owns recovery placement vocabulary.
Cluster, node, and namespace facets remain valid supporting filters for
Proxmox-heavy and Kubernetes-heavy operators, but the governed recovery
surface must present them through platform-neutral labels such as
`Cluster / Site`, `Host / Agent`, and `Namespace / Group` across advanced
filters, active chips, table headers, column-picker entries, and point
details so the page treats placement as optional context inside a
multi-platform recovery model rather than a Proxmox-native spine. When
normalized display labels are present, the visible history rows must prefer
those labels over raw transport values for the same placement dimensions.
The recovery table presentation helper now owns the canonical subject-type
label fallback for recovery rows and delegates its title-casing to the shared
`frontend-modern/src/utils/textPresentation.ts` helper rather than keeping a
local recovery-only formatter, so subject and outcome labels stay aligned with
the shared frontend label contract. Protected-inventory and recovery-event
filters, table headers, and column-picker labels must use that helper for
artifact fields such as `Item Type`, so the recovery tabs do not drift into
near-identical page-local casing.
That same recovery drill-in surface now also keeps provider-specific metadata
inside a provider-neutral detail shell through
`frontend-modern/src/components/Recovery/RecoveryPointDetails.tsx`, so PBS
datastore and verification enrichments remain available without presenting the
details drawer as a PBS-only surface.
The point-details drawer also owns restore-safe operator guidance. It may
surface restore readiness, verification provenance, and chain coverage for the
selected point, but it must stay read-side until the backend exposes a governed
restore execution contract. The drawer must not present a freestanding restore
runbook or next-action path that reads as an approved restore workflow; target
confirmation and isolated test-restore planning belong in a future governed
action or restore flow, not in the evidence drawer. Chain context must be
derived from the current recovery result set only when at least two concrete
stages are visible, so mixed PVE/PBS/TrueNAS history can explain adjacent local
snapshot, local copy, and remote copy stages without filling the drawer with
missing-only cards. Raw transport IDs, provider refs, provider task IDs, and
raw JSON copy actions belong behind `Technical details`; the primary drawer
should keep human metadata, recorded verification provenance, target health,
and collapsed file lists without repeating the same verification fact in
provider-specific sections or rendering empty verifier/evidence placeholders
when no verification record exists. Verification provenance should translate
provider states such as PBS catalog `ok` into operator language instead of
surfacing raw transport status tokens. Container recovery points should present
container ids with operator vocabulary such as `CTID`, and duplicated placement
or target values should not be repeated under lower-priority metadata labels.
Provider-specific metadata must not recast the event drawer itself as if PBS
were the native recovery model. Provider-owned repository data should sit under
target-oriented wording such as `Target Details`, `Repository owner`, and
`Target Health`; when target-specific technical labels are surfaced, they
should prefer neutral wording such as `Target Ref` and `Target Resource`.
Those transport hooks are direct governed runtime surfaces, not just page
implementation detail: `frontend-modern/src/hooks/useRecoveryPoints.ts`,
`frontend-modern/src/hooks/useRecoveryPointsFacets.ts`,
`frontend-modern/src/hooks/useRecoveryPointsSeries.ts`, and
`frontend-modern/src/hooks/useRecoveryRollups.ts` must stay on the explicit
`recovery-product-surface` proof path instead of inheriting release-control
coverage only through a retired standalone Recovery page shell.
Those same hooks now also own recovery transport normalization at the frontend
boundary: raw compatibility fields such as `provider` / `providers` may be
accepted from older `/api/recovery/*` payloads, but the runtime values they
return to the rest of the recovery UI must be canonical `platform` /
`platforms` models.
The retired dashboard recovery and storage entry points must stay removed:
`useDashboardRecovery`, `DashboardRecoveryStatusPanel`,
`DashboardStoragePanel`, dashboard storage/recovery presentation helpers, and
dashboard widget orchestration must not return as direct proof surfaces. New
storage or recovery summary proof must attach to the owning Storage,
Recovery, Infrastructure drawer, or shared summary component instead of
borrowing coverage through a broader dashboard shell.
The shared recovery type contract must be pinned the same way:
`frontend-modern/src/types/recovery.ts` must stay on the explicit
`recovery-product-surface` proof path instead of riding indirectly on route or
component coverage.
That same direct proof rule applies to the shared recovery date helper:
`frontend-modern/src/utils/recoveryDatePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery status helper:
`frontend-modern/src/utils/recoveryStatusPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
The default protected-inventory recovery route must also keep its primary
table shell on class-driven sizing (`table-fixed` plus owned width classes)
instead of inline `table-layout` / `min-width` styles, so the public recovery
surface stays CSP-safe without drifting from the shared table contract.
That same direct proof rule also applies to the shared recovery record helper:
`frontend-modern/src/utils/recoveryRecordPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That shared recovery record contract now also includes rollup-side display
payload continuity: the recovery backend must preserve the latest normalized
subject label on rollups, and recovery UI helpers must prefer that canonical
display label before raw subject ids whenever the live unified-resource map is
missing or only resolves to opaque machine identifiers.
That same direct proof rule also applies to the shared recovery outcome helper:
`frontend-modern/src/utils/recoveryOutcomePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery action helper:
`frontend-modern/src/utils/recoveryActionPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery artifact mode
helper: `frontend-modern/src/utils/recoveryArtifactModePresentation.ts` must
stay on the explicit `recovery-product-surface` proof path instead of
inheriting coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery empty-state
helper: `frontend-modern/src/utils/recoveryEmptyStatePresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery filter-chip
helper: `frontend-modern/src/utils/recoveryFilterChipPresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery issue helper:
`frontend-modern/src/utils/recoveryIssuePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery table helper:
`frontend-modern/src/utils/recoveryTablePresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery timeline-chart
helper: `frontend-modern/src/utils/recoveryTimelineChartPresentation.ts` must
stay on the explicit `recovery-product-surface` proof path instead of
inheriting coverage only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery timeline
helper: `frontend-modern/src/utils/recoveryTimelinePresentation.ts` must stay
on the explicit `recovery-product-surface` proof path instead of inheriting
coverage only through pages or higher-level recovery components.

Those recovery transport surfaces now also share one normalized filter
contract: protection rollups, point history, facets, and chart series must
all honor the same canonical `platform`, canonical `itemType`, cluster, node, namespace,
workload-scope, verification, and route-backed free-text `q` filter so the
protection coverage list cannot drift from the timeline and facet state under the
same active recovery view. That same recovery filter contract now depends on
the canonical recovery index carrying a normalized `itemType` instead of
forcing each UI surface to re-derive protected item classes from raw
provider-native `subjectType` values.
That same recovery product surface keeps the primary workspace visually ahead
of secondary analytics: `frontend-modern/src/components/Recovery/Recovery.tsx`
must lead directly into the route-backed recovery events workspace, with
recovery events owning both the activity timeline and the event table together.
Protection coverage opens only through the explicit header action or
compatibility route state. The activity timeline remains required even when
point-history loading fails, but it belongs to the events workspace rather than
hanging beside coverage as a competing page-level peer.
That same recovery product surface must not reintroduce a four-card posture,
freshness, coverage, or activity strip as decorative orientation. Workloads is
the better object-level place to scan whether a resource has backup coverage;
Recovery should reserve its first viewport for concrete recovery history and
only expose coverage rollups when the operator asks for the secondary review.
That same page-level ownership applies to the recovery time window. The
canonical range selector now lives inside `RecoveryActivitySection.tsx` because
it controls the event timeline and the event query window, not a page-level KPI
strip. Range changes must clear selected-day focus and return the events table
to page one through the route-state owner so the chart and table remain aligned.
That same activity-section contract should stay compact and scan-first:
recovery-point volume, active-day count, stale count, and average rate may
appear as concise inline readouts beside the chart controls, but not as separate
top-level metric cards or nested mini-panels. The activity strip must also avoid
interpreted anomaly callouts such as "lowest active day" unless the recovery
model has canonical schedule expectations that make the signal actionable.
Item-type labels should continue to render through canonical workload/resource
badge classes in the event and coverage tables instead of adding recovery-only
wrapper chrome around VM, container, or other resource badges.
That same event-first shell rule should hand straight from the activity section
into the active events workspace without an extra page-local spacer band,
default tab row, or duplicate status strip that makes the surface softer than
storage or workloads.
That same scan-first rule applies to the secondary coverage surface. Recovery
should not show equal workspace tab labels and then repeat the same workspace
count as standalone text; protection coverage cues should focus on issues,
drill-in context, and active filters.
That same coverage surface should also follow the established monitoring-table
scan pattern in its first column. Coverage rows should lead with a clear
status cue, the primary item name, and compact badge-backed item/platform
context instead of relying on recovery-only rails or plain-text metadata lines
that make the table read like a report instead of an operational grid.
That same triage rule applies to the coverage sort. Protection coverage should
open with attention-state rollups first instead of defaulting to
newest-successful backups, so operators land on failed, never-succeeded, stale,
warning, and running items before the healthy catalog.
That same row contract should avoid duplicating context that already has a
dedicated column. When `Item Type` and `Platform` columns are visible, the
primary item cell should not restate those same badges on desktop; duplicate
context belongs only as a small-screen fallback when those columns collapse.
That same scan rule applies to the supporting columns themselves. Recovery
tables should keep one dominant identity cue and one canonical platform cue.
`Item Type` should use the same shared workload/resource badge treatment that
other Pulse tables use for `VM` and `Container`, while `Method` and similar
supporting fields stay on restrained metadata text instead of turning every
adjacent column into another colored badge.
That same item-identity contract also applies to synthetic Proxmox task
recovery points. When the persisted subject label is just a raw
`pve-task:*`/`UPID:*` identifier or `vmid=0`, the canonical recovery index
should derive a readable task label and `task` item type from point details so
recovery tables scan by operator meaning instead of transport IDs.
That same coverage surface should stay on the flat monitoring-table pattern
already used elsewhere in Pulse. Protection coverage should surface posture through
row-level status cues, outcome pills, and filters rather than inserting extra
`Needs Attention` / `Healthy Coverage` section rows that add height and turn
the table into a recovery-only grouped report.
That same protection-coverage table should also avoid recovery-local pagination
chrome. The workspace already holds the filtered rollups client-side, so it
should read as one continuous monitoring table with a simple coverage-item
count instead of introducing `Prev` / `Next` buttons and page counters that do
not match the canonical Pulse scan pattern.
That same table-shell contract must avoid duplicate framing once the summary
action has established the secondary coverage workspace.
`RecoveryProtectedInventorySection.tsx` should keep page/count/sort
orientation inside a slim table-shell status row and let the filter strip lead
directly into the grid instead of reintroducing a second large inventory header
card above the same table.
That same shell rule should also avoid low-signal bookkeeping above the grid.
The protected-items status row should surface the active workspace, protected
item count, and issue cues, but page-number and sort-direction bookkeeping
belongs in the table chrome itself rather than competing with the primary scan
path before operators even reach the rows.
That same table density rule also applies to recovery table chrome and filter
rows. Recovery inventory and event tables should use the same restrained
title-case header typography, compact control heights, and thin row density as
the established Pulse monitoring tables instead of drifting into report-style
uppercase headers or oversized filter chrome.
That same protected-items table contract should stay on the canonical shared
table separator treatment used by the rest of Pulse. Recovery inventory should
use the standard shared header/body dividers and avoid both local suppression
of those separators and local duplicate row or header borders that make the
lines read heavier than other monitoring tables.
That same workspace-shell rule should also avoid a dedicated recovery-only
status strip above the control bar. Recovery should use an event-first handoff:
activity context, a shared controls card, then a data card. Protection coverage
may use its own shared controls card and table when explicitly opened, but
Recovery should not collapse controls and content back into one fused workspace
slab or bury secondary workspace navigation inside the filter row.
That same strip should not repeat page-level counts or posture cues as a
replacement for the retired summary strip. Protection coverage controls should
stay focused on drill-in context and active filters instead of echoing page-wide
posture pills above the same table.
That same workspace handoff should stay on shared primitive styling too.
Recovery events and protection coverage should keep using shared `FilterBar`,
`TableCard`, and `TableCardHeader` primitives instead of inventing a
recovery-only variant or recovery-only class stack.
That same canonical-row rule also means the subtabs row should stand on its own
full-width shell instead of sharing a flex line with recovery-only chips or
adjacent badges that break the storage-style border and spacing treatment.
That same shared page-controls contract applies to recovery search width too.
The protected-items and recovery-events workspaces should keep the search field
on the standard full-width shared search row, and any counts or utility cues
should live in the toolbar actions instead of narrowing the search row through
recovery-local grid overrides or width hacks. Protected-items controls should
also use the same shared `Reset all` page-controls action pattern as storage
and workloads when visible filters are active, instead of forcing operators to
clear each inventory filter manually.
That same handoff should keep Recovery out of shared summary-card density
tuning unless a new governed product decision reintroduces a first-viewport
summary owner. The current route should spend its top-level density budget on
the activity section, compact controls, and one primary data card.
That same shell rule applies to the recovery-events workspace.
`RecoveryHistorySection.tsx` should use the same slim status-row-plus-filter-row
pattern as the protected inventory surface, not a separate large titled header
bar plus another full toolbar slab. Event filter labels should also stay on the
canonical short Pulse vocabulary like `Platform` and `Status` instead of
recovery-only variants such as `History platform` or `History status`. Both
recovery toolbars should also stay on compact shared select sizing instead of
inflating the row with recovery-local min-width overrides that make the
controls denser and wider than storage for the same amount of operator input.
That same events-workspace rule should keep the activity strip as orientation
for the event list rather than burying it at the bottom. The events workspace
should move from the subtabs row to `RecoveryActivitySection.tsx`, then shared
controls, then the recovery history table as sibling sections, so the timeline
frames the event list without turning the page back into stacked primary
tables or embedding the activity strip inside the history card.
That same events-shell contract should avoid repeating page-state bookkeeping
ahead of the history grid. Recovery events should keep the toolbar utility area
focused on actual controls like advanced filters and column visibility instead
of passive `day groups` narration; day grouping should stay legible through the
history surface itself, while current page and other table bookkeeping remain
in the table footer instead of competing with the scan path above the filters.
That same activity panel should stay compact and analytical rather than
becoming a second dashboard header. `RecoveryActivitySection.tsx` should keep a
single slim telemetry header, compact active-filter chips, a shorter chart
frame, reduced vertical insets, and a smaller legend footprint so the events
workspace hands off quickly from activity context to the history table instead
of spending a disproportionate slice of the screen on chart chrome. The range
picker and legend should share one compact control row, and the activity strip
should not burn a separate descriptive subtitle row once the headline metrics
already explain the chart context.
That same timeline contract must keep long-range activity fully constrained to
the card width. Extended ranges such as `365d` should compress their day
columns to fit the available plot width instead of carrying per-column minimum
widths that make the chart overflow its containing card.
That telemetry header should also avoid derivative pace rows once the chart
already carries the rhythm. Total points, active days, and issue cues can
stay, but average-per-day style readouts should not re-expand the strip into a
second mini report above the event table.
That same events-table contract should also keep the default column set on a
monitoring-style scan path rather than a report-export path. Recovery events
should default to the concise columns operators need to triage quickly, while
secondary fields such as verification, size, target, and details remain
available through the shared column picker instead of crowding the baseline
desktop view. When the responsive event table collapses columns on mobile, the
primary item cell must retain enough method, platform, target, and verification
context to preserve restore-readiness meaning rather than hiding all secondary
evidence behind desktop-only columns in the desktop grid.
That same event-row scan rule should mirror the protected-items table in the
primary identity cell. `RecoveryHistoryTable.tsx` should lead each event row
with a compact outcome status cue plus the canonical item name, so operators
can scan event health by row without relying only on the far-right outcome
column.
That same density rule should also keep history grouping and badges restrained.
Day-group headers should read as slim dividers instead of banner rows, and
platform/method/outcome pills should stay compact enough that the event grid
still scans like a monitoring table rather than a report export.
That shared unified-resource dependency now also includes policy-governed
resource metadata on the frontend decode path: storage and recovery surfaces
that route through `frontend-modern/src/hooks/useUnifiedResources.ts` must
preserve canonical `policy` and `aiSafeSummary` fields so storage-bearing
resources do not silently lose their routing or redaction posture when they
cross from unified-resource ownership into storage or recovery presentation.
That same decode path now trusts the backend canonical `policy` and
`aiSafeSummary` values directly, so storage and recovery surfaces keep the
canonical summary text aligned with the policy-aware resource contract
instead of reformatting or re-normalizing it locally.
That same shared `internal/api/` dependency now also routes resource-timeline
filters through the unified-resource change parser, so storage and recovery
surfaces do not inherit a second local decoder for `kind`, `sourceType`, or
`sourceAdapter` values.
That same shared `internal/api/` dependency now also assumes the resource
timeline parser is owned by unified resources, so storage and recovery
surfaces rely on one canonical change-filter contract instead of re-decoding
timeline query values in the handler layer.
That same shared `internal/api/` dependency now also assumes canonical install
payload URLs are slash-normalized before they become response fields or helper
attachments, so recovery-adjacent links and transport surfaces cannot inherit
double-slash installer paths from backend public-endpoint configuration.
That same shared `internal/api/` dependency must also preserve the governed
agent-lifecycle shell payload shape when adjacent diagnostics responses expose
install transport: `router.go` may not reintroduce stale lifecycle flag aliases
or raw sudo-only install pipes in container-runtime migration payloads that
share the same backend response surface.
That same shared dependency now also assumes those diagnostics install payloads
route through the canonical backend install-command helper, so recovery-adjacent
transport surfaces do not inherit handler-local drift in token omission,
plain-HTTP `--insecure`, or trailing-slash normalization.
That same shared `internal/api/` dependency also assumes diagnostics memory
source breakdowns backfill canonical fallback reasons even when a raw legacy
snapshot reaches `internal/api/diagnostics.go` without one, so
recovery-adjacent consumers do not observe alias-normalized sources paired
with empty or drifted fallback-reason payloads.
That same shared `internal/api/` dependency now also assumes local commercial
and onboarding analytics remain outside the shared diagnostics payload, so
recovery-adjacent diagnostics surfaces can safely share the backend route
without inheriting commerce telemetry, cross-tenant leakage, or hosted/local
semantic drift.
That same shared diagnostics payload may expose native Pulse Assistant runtime
availability as `assistantRuntimeConnected`, but storage/recovery consumers
must not revive MCP diagnostic fields or treat Assistant runtime availability
as backup freshness, restore capability, or storage-local execution authority.
That same shared `internal/api/` dependency now also assumes auth persistence
compatibility stays on an explicit migration/import boundary: legacy
raw-token `sessions.json` and `csrf_tokens.json` files may load for upgrade
continuity, but `session_store.go` and `csrf_store.go` must immediately
rewrite hashed canonical persistence on load so adjacent storage and recovery
transport does not keep running against primary-path raw-token files.
That same shared `internal/api/` dependency also assumes customer-visible
commercial acquisition stays out of storage and recovery surfaces by default:
storage- or recovery-adjacent flows must not invoke or advertise the retired
`POST /api/license/trial/start` route, and the normal self-hosted router must
return `404` for that path without mutating entitlements. The retired
`/auth/trial-activate` self-hosted callback must also stay absent from
storage/recovery-local retry or backoff behavior. Retired
`trial_eligible`/`trial_eligibility_reason` payload fields are compatibility
only and must not become storage/recovery prompt, identity, or restore state.
Local commercial metrics reporting routes are part of the same adjacent
API/cloud-paid admin settings boundary: storage and recovery surfaces must not
read stats, health, config, or funnel reports as customer-visible recovery,
storage-health, or setup state. In the normal customer product API those
retired local commercial analytics routes must stay unregistered, so storage
and recovery cannot use their absence as a reason to synthesize recovery-local
commercial reporting or fallback state.
That same shared `internal/api/` dependency now also assumes adjacent
commercial helper surfaces speak in monitored-system terms: recovery- or
storage-adjacent API wiring may consume the canonical monitored-system ledger
helpers, but it must not revive deleted agent-era helper names, cap helpers,
or imply that API-backed infrastructure sits outside the counted system model.
That same shared `internal/api/` dependency now also assumes monitored-system
ledger status details stay canonical and source-aware: storage- or recovery-
adjacent consumers may read the ledger’s nested status explanation, but they
must preserve the backend-provided reason list for stale or offline grouped
sources, including the canonical `reported_at` timestamp, instead of reducing
those mixed fresh/stale system states back to a generic label.
That same ledger dependency also treats the canonical `latest_included_signal`
object as the freshest grouped observation. Storage- or recovery-adjacent
consumers must not present that data with bare single-source `Last Seen`
wording that hides grouped stale/offline conditions, and should use the
canonical object when they need attribution for which grouped surface most
recently reported. Retired flat alias fields must not reappear as separate
freshness signals or adjacent contract wording.
That same shared `internal/api/` dependency now also assumes self-hosted
commercial counting is canonical at the top-level monitored-system boundary:
shared setup, deploy, entitlement, and API-backed monitoring helpers may not
preserve an API-only exemption that would let storage- or recovery-adjacent
systems consume no commercial slot when the same monitored system is visible
through canonical unified-resource roots.
That same shared boundary now also assumes replacement-aware monitored-system
projection. When a storage- or recovery-adjacent update replaces one source on
an already-counted host, the API helper must strip only that source from the
prospective grouped system and preserve any remaining top-level evidence such
as agent or sibling API ownership, rather than briefly freeing a slot or
double-counting the same monitored system.
When storage- or recovery-adjacent settings or support flows need to explain
that result, they must rely on the shared monitored-system ledger preview
contract for current/projected grouped systems instead of reconstructing
preview copy, limit verdicts, or cap copy from page-local recovery inventory or
provider-local connection details.
That same adjacent preview contract also treats disabled provider connections
as non-counting candidates. Storage- or recovery-adjacent flows may use the
shared zero-delta or removal-only preview state for explanation, but they must
not reinterpret a disabled TrueNAS or VMware connection as active counted
capacity until the canonical provider configuration is explicitly re-enabled.
That same shared boundary now also assumes settled monitored-system usage
readiness. Storage- or recovery-adjacent transport flows may not interpret the
first store-backed monitor view as commercial truth when provider-owned
supplemental platforms such as TrueNAS or VMware are still between initial
connection wiring and the first rebuilt canonical store; until that baseline
settles, adjacent surfaces must use the canonical ledger/preview unavailable
state and avoid sealing any migration or support decision against a transient
undercount.
That same shared `internal/api/` dependency also assumes session-carried OIDC
refresh tokens stay fail-closed at rest: `session_store.go` may only persist
or recover those tokens through encrypted-at-rest session payloads, and any
missing-crypto or invalid-ciphertext path must drop the refresh token instead
of preserving plaintext-at-rest session state that storage and recovery
surfaces might inherit through shared auth runtime helpers.
That same shared `internal/api/` dependency also assumes shared OIDC/SAML
callbacks finish on canonical local redirect targets. Storage- or
recovery-adjacent routes that rely on shared auth helpers may not reintroduce
per-handler `returnTo` concatenation or absolute-target acceptance when they
inherit those browser handoff paths through the common API router surface.
That same shared `internal/api/` dependency also assumes notification test
handlers stay decode-and-delegate only: `internal/api/notifications.go` may
share the API helper boundary with storage-adjacent routes, but service-template
selection and generic webhook-test payload fallback must remain
notifications-owned instead of becoming a second API-layer owner.
That same shared API boundary also assumes legacy service-specific webhook
aliases are rewritten at ingress only: `internal/api/notifications.go` may
accept compatibility keys like Pushover `app_token` / `user_token`, but it
must return and forward only canonical `token` / `user` fields so storage-
adjacent shared `internal/api/` helpers do not inherit a second live alias
contract.
That same shared `internal/api/` dependency now also assumes recovery-token
persistence follows the same canonical rule: raw recovery secrets may be minted
for immediate operator use, but `recovery_tokens.go` must persist only token
hashes and treat any legacy plaintext-token file as a one-time migration input
that is rewritten immediately into hashed canonical persistence on load.
That same storage-adjacent persistence rule also applies to
`internal/config/persistence.go` API token metadata: `api_tokens.json` may hold
only hashed token records, but a legacy plaintext metadata file may only be
migration input and must be rewritten immediately into encrypted-at-rest
storage on load instead of staying on the runtime primary path.
That same shared `internal/api/` dependency also assumes those auth stores stay
owned by the configured router data path: session, CSRF, and recovery-token
runtime state may not silently bind to hidden `/etc/pulse` fallback
initialization or leak old-path store contents forward after reconfiguration.
That same path-ownership rule also governs adjacent hosted billing and
bootstrap artifacts that share the `internal/api/` boundary: webhook dedupe
state, customer indexes, and bootstrap-token lookup must resolve their base
directory through the shared runtime data-dir helper instead of carrying
neighboring `/etc/pulse` fallback logic of their own.
That same shared boundary also assumes manual auth env writes and auth-status
reads resolve `.env` through the shared auth-path helper, so storage-adjacent
recovery and setup flows do not keep neighboring `/etc/pulse/.env` fallback
logic alive after the runtime data-dir authority has been centralized.
That same shared `internal/api/` dependency also assumes config import reloads
degrade safely when optional runtime managers are missing. Storage- and
recovery-adjacent restore or support flows may drive the shared
`/api/config/import` boundary before every notification or monitoring manager
exists, but `internal/api/config_export_import_handlers.go` must still apply
the imported configuration without panicking on absent optional managers.
The same boundary also owns first-session reset cleanup during managed-backend
proof: the dev-only `/api/security/dev/reset-first-run` route must clear auth
env and persisted API-token state through the shared helpers, and adjacent test
or recovery tooling may not delete those files directly.
That same shared `internal/api/` dependency also assumes generated developer
warnings keep the local browser/runtime split accurate: the embedded frontend
notice under `internal/api/DO_NOT_EDIT_FRONTEND_HERE.md` may describe `:7655`
as the proxied backend dependency, but it must preserve
`http://127.0.0.1:5173` as the hot-reload browser entrypoint so storage- and
recovery-adjacent setup guidance does not drift back to the backend port.
That same shared boundary now also owns writable auth-env fallback order, so
storage-adjacent setup and recovery flows may not keep per-handler config-path
write branches with private data-path fallback logic once the shared helper
exists.
That same shared `internal/api/` dependency also assumes bootstrap-token
persistence follows the same boundary discipline: the first-session setup
secret may remain recoverable through the supported `pulse bootstrap-token`
command, but `.bootstrap_token` may not stay a primary-path plaintext secret
file. Canonical runtime persistence must encrypt that token at rest, and any
legacy plaintext bootstrap-token file must be rewritten immediately into the
encrypted canonical format on load.
That same shared `internal/api/` dependency now also assumes Proxmox
setup-command payloads stay on the governed fail-fast shell transport, so
adjacent setup and recovery-linked flows do not inherit stale `curl -sSL`
quick-setup commands from handler-local string assembly.
That same dependency also assumes the generated setup scripts echo the same
fail-fast guidance back to operators during retry and validation failures, so
adjacent setup flows do not preserve stale `curl -sSL` examples after the API
response itself has already moved to the governed transport.
That same shared dependency now also assumes those quick-setup commands and
embedded retry examples preserve root-or-sudo continuity, so adjacent setup
flows do not regress to direct-root-only command guidance on hosts where the
operator enters through a non-root shell.
That same dependency also assumes the embedded retry examples preserve the
active setup token through those root-or-sudo paths, so adjacent setup flows
do not regress from non-interactive reruns back to prompt-only recovery.
That same shared dependency also assumes generated setup scripts hydrate
`PULSE_SETUP_TOKEN` from any embedded setup token before they print rerun
guidance, so adjacent setup flows do not lose non-interactive continuity just
because the next hop entered through a generated script body instead of the
original API command.
That same shared dependency also assumes `/api/setup-script-url` and the
generated rerun guidance draw from one canonical bootstrap artifact builder, so
adjacent setup and recovery-linked transport flows do not drift on download
URLs, script filenames, token hints, or env-wrapped rerun command shape across
the setup bootstrap boundary.
That same shared dependency also assumes generated PVE setup scripts actually
remove the discovered legacy token sets they enumerate during cleanup, so
adjacent operator recovery flows do not present a fake cleanup option that
quietly leaves stale `pve` and `pam` Pulse tokens behind. That same cleanup
dependency also assumes candidate discovery stays on the canonical
Pulse-managed token prefix for the active Pulse URL, so adjacent setup flows
do not drift onto IP-pattern token matching that misses hostname-scoped legacy
tokens. That dependency applies to both generated PVE and PBS setup scripts,
so adjacent setup flows do not fork cleanup discovery rules by node type.
That same generated PVE setup-script dependency also assumes temperature-key
setup and removal preserve Proxmox-managed `/root/.ssh/authorized_keys`
symlinks: adjacent storage and recovery setup flows may depend on the shared
script renderer, but they must not replace the symlink path with a local file
when filtering Pulse-managed `# pulse-` SSH key entries.
That same dependency also assumes the shared PVE setup script binds
temperature-monitoring SSH keys to `/usr/local/sbin/pulse-sensors` and emits
SMART disk temperatures in the wrapper payload, including explicit `-d sat`
and `-d scsi` retries for direct Linux SATA/SAT-style disks whose smartctl
auto-detection returns no temperature. Storage and recovery disk temperature
surfaces may depend on that monitoring-owned SMART merge path, but they must
not reintroduce raw `sensors -j` as the setup contract or build a storage-local
disk-temperature collector.
Pressure-only host-agent telemetry remains outside that storage collector
contract: storage surfaces may read the shared host context, but may not add a
parallel macOS thermal collector or fold `thermalState` into disk SMART state.
That same shared discovery dependency also assumes runtime discovery state owns
only structured errors, while adjacent API and WebSocket payloads may derive
the deprecated string `errors` list only as a compatibility field from those
canonical structured errors.
That same dependency also assumes rerun token-rotation detection uses exact
managed token-name matches, so adjacent setup flows do not collide with
unrelated partial-name tokens and rotate the wrong state.
That same dependency also assumes generated PBS setup scripts only print the
token-copy banner after a successful token-create result, so adjacent setup
flows do not advertise a non-existent token on failure.
That same dependency also assumes generated PBS setup scripts only print the
auto-register attempt banner on the real request path, so adjacent setup flows
do not claim an in-flight registration attempt on branches that are actually
skipped before any request is sent.
That same shared dependency also assumes generated setup scripts preserve the
canonical encoded rerun URL contract, so adjacent setup flows do not drop the
selected `host`, `pulse_url`, or `backup_perms` state when operators rerun the
embedded quick-setup command from the script body.
That same shared dependency also assumes generated setup scripts fail closed on
auto-register success parsing, so adjacent setup flows do not misreport
registration success when a shared backend response still carries a
`success:false` payload.
That same dependency also assumes those generated setup scripts fail closed on
auto-register HTTP and transport failures, so adjacent setup flows do not
reinterpret shared backend stderr or HTTP-failure output as a successful
registration payload.
That same shared dependency also assumes generated setup scripts preserve
setup-token auth guidance, so adjacent setup flows do not regress back to
stale API-token instructions after the backend has already standardized on the
one-time setup-token contract.
That same dependency also assumes generated setup scripts preserve truthful
registration-outcome messaging, so adjacent setup flows do not claim a node
was successfully registered when the shared backend path actually fell back to
manual completion.
That same dependency also assumes manual completion stays on the canonical
node-add path, so adjacent setup flows do not regress back to a stale
secondary registration-token rerun contract after the backend already emitted
manual token details for Pulse Settings → Nodes.
That same dependency also assumes auth-failure messaging stays truthful once a
shared setup script has already entered the registration-request path, so
adjacent setup flows do not regress into missing-token copy when the real next
step is to fetch a fresh setup token from Pulse Settings → Nodes and rerun.
That same auth-failure state must also suppress the later manual-details
footer, so adjacent setup flows do not contradict the rerun contract.
That same dependency also assumes the auto-register failure summary stays on
that canonical node-add path, so adjacent setup flows do not regress to vague
"manual configuration may be needed" wording once the backend already emitted
the exact Pulse Settings → Nodes completion path.
That same dependency also assumes the immediate failure branch reuses that same
manual-completion contract instead of drifting into a numbered manual-setup
list before the final token-details footer, including request-failure branches
that never receive a parseable backend response.
That same dependency also assumes those manual-add instructions preserve the
canonical node host already known to the script, so adjacent setup flows do
not regress to placeholder host guidance when shared backend continuity is
otherwise intact.
That same dependency also assumes the PBS setup script binds that canonical
host before setup-token gating can skip auto-registration, so adjacent manual
fallback output does not lose the host URL just because setup-token input was
omitted.
That same dependency also assumes the canonical PBS host is already bound
before token-creation failure fallback, so adjacent manual completion output
does not drop the host URL just because token minting failed earlier in the
same shared script.
That same dependency also assumes token-creation failure stays truthful in
those generated setup scripts, so adjacent flows do not regress into fake token
details or a false "token setup completed" state after shared backend token
minting already failed.
That same dependency also assumes token-extraction failure stays on the same
rerun-after-fix path, so adjacent setup flows do not regress into a false
manual-registration fallback when the shared backend still has not produced a
usable token secret, and do not enter the shared manual-completion footer until
that usable secret actually exists.
That same shared setup dependency also assumes skipped PBS auto-register paths
stay truthful, so adjacent flows do not regress into a fake request-failure
banner when the backend intentionally never attempted registration.
That same shared setup dependency also assumes missing-host script payloads
stay fail closed, so adjacent flows do not regress into placeholder manual
registration targets when the backend never received a canonical node URL.
That same dependency also assumes PBS follows the identical host rule, so
adjacent setup flows do not regress from the backend-requested canonical PBS
host to a runtime-local interface address when manual completion is rendered.
That same dependency also assumes those manual-add instructions preserve
canonical Settings → Nodes phrasing across node types, so adjacent setup flows
do not drift into inconsistent manual-completion language for equivalent
fallback paths.
That same dependency also assumes the earlier auto-register failure branch uses
that identical Settings → Nodes destination, so adjacent setup flows do not
observe one manual-completion path in the immediate error guidance and a
different one in the final backend-owned footer.
That same dependency also assumes the off-host PVE fallback stays on the
canonical rerun-on-host contract, so adjacent setup flows do not regress into
a separate manual `pveum` plus Pulse Settings token-entry path that the shared
backend no longer owns.
That same dependency also assumes direct script launches preserve the canonical
root requirement wording, so adjacent setup flows do not regress to a stale
"Please run this script as root" branch while the governed retry transport
already uses the newer privilege guidance.
That same dependency also assumes manual-add token placeholder text stays
canonical across those generated setup branches, so adjacent setup flows do
not surface conflicting "see above" instructions for the same backend-owned
token continuity contract.
That same dependency also assumes successful generated setup flows preserve one
canonical success message across node types, so adjacent setup surfaces do not
drift into type-specific completion wording for the same backend-confirmed
registration state.
That same dependency also assumes token-extraction failures stop before shared
registration assembly, so adjacent setup flows do not proceed with an empty
token secret after the backend already determined the generated token value was
unavailable.
That same dependency also assumes canonical PVE auto-register payloads carry
real caller-supplied token secrets only, so adjacent setup flows do not treat
placeholder response state as a usable credential or persist dead pending
branches into shared node state.
That same shared `internal/api/` dependency also assumes the canonical
`/api/auto-register` success payload keeps canonical node identity in
`nodeId` instead of the raw host URL or requested server name, so adjacent
setup and recovery-linked transport attachments do not fork between stored
node name and request-form identity.
That same dependency also assumes the shared `node_auto_registered` event from
canonical /api/auto-register keeps the normalized stored host and canonical node
identity in its payload, so adjacent transport surfaces do not fork between
saved node state and raw request-form event data.
That same shared dependency also assumes canonical /api/auto-register success
responses mirror that stored identity and normalized host through `type`,
`source`, `host`, `nodeId`, and `nodeName`, so installer and runtime-side
Unified Agent success paths cannot drift into a second local identity after
registration.
That same dependency also assumes the setup-token bootstrap response from
`/api/setup-script-url` carries canonical `type`, normalized `host`, and live
expiry metadata, so adjacent setup and recovery-linked transport surfaces do
not consume a mismatched bootstrap token after host normalization.
That same shared dependency also assumes installer and runtime-side Unified
Agent callers fail closed on already-expired bootstrap responses instead of
treating any populated `expires` field as sufficient.
That same shared dependency also assumes Pulse-managed Proxmox monitor-token
names stay bound to the canonical Pulse endpoint across setup/bootstrap
surfaces, so adjacent setup and recovery-linked flows may not derive token
scope from request-local `Host` fallbacks and accidentally fork monitor-token
identity for the same Pulse instance.
That same shared dependency also assumes generated PVE setup scripts, runtime
host-agent setup, and installer auto-registration keep backup visibility
permissions effective on privilege-separated tokens: optional `/storage`
`PVEDatastoreAdmin` grants must be mirrored to the service user and the concrete
token id, not left as a user-only ACL that the token cannot use.
That same shared dependency also assumes `/api/setup-script` stays on one
canonical artifact contract: manual setup downloads must ship as
`text/x-shellscript` attachments with deterministic `pulse-setup-*.sh`
filenames, so adjacent setup and recovery-linked transport surfaces do not
flatten governed script delivery into untyped text blobs.
That same shared dependency also assumes `/api/setup-script-url` carries that
canonical setup-script filename as bootstrap metadata, so adjacent setup and
recovery-linked surfaces do not reintroduce hardcoded local filenames that can
drift from the downloaded artifact.
That same shared dependency also assumes settings quick-setup treats
`/api/setup-script-url` as one canonical bootstrap artifact per active
endpoint, so adjacent setup and recovery-linked surfaces do not fork copy and
manual-download behavior onto separate lane-local bootstrap requests.
That same shared dependency now also assumes that bootstrap artifact is owned
by one shared backend install-artifact model rather than mirrored local
bootstrap structs and response envelopes, so adjacent setup and
recovery-linked surfaces do not inherit drift between downloads, rerun
guidance, and script rendering. Generated PVE/PBS setup-script bodies must also
come from the same shared backend render helpers instead of a handler-local
template engine, so recovery-linked copy and rerun flows do not fork the shell
transport contract by route implementation.
guidance, and the setup-script-url payload.
That same shared dependency also assumes that bootstrap artifact includes a
dedicated token-bearing `downloadURL`, so manual setup-script downloads remain
non-interactive without forcing adjacent surfaces to re-expose the raw setup
token or rebuild a second setup-script request from partial bootstrap state.
That same shared dependency also assumes runtime-side Unified Agent and
installer consumers keep the full setup bootstrap envelope coherent: adjacent
transport surfaces may not silently accept `/api/setup-script-url` responses
that drop canonical script URL, filename, or command metadata while still
returning a token.
That same shared dependency also assumes `/api/setup-script-url` keeps a strict
canonical request shape: adjacent setup and recovery-linked surfaces may not
quietly accept unknown request fields or trailing JSON on that bootstrap route,
because typo-compatible or concatenated payloads would fork the governed setup
artifact contract from the direct handler proofs.
That same shared dependency also assumes bootstrap backup permissions stay on
the canonical PVE-only path: adjacent setup and recovery-linked surfaces may
not accept `backup_perms` / `backupPerms` for PBS and then silently drift onto
an unsupported no-op request contract.
That same shared dependency also assumes both setup routes keep canonical host
identity explicit: adjacent setup and recovery-linked surfaces may not allow
`/api/setup-script` to fall back to placeholder host artifacts after
`/api/setup-script-url` already requires a real normalized `host`.
That same shared dependency also assumes those setup routes share one canonical
type and host-normalization boundary: adjacent setup and recovery-linked
surfaces may not allow `/api/setup-script` to treat unknown `type` values as
implicit PBS requests or emit unnormalized host state after
`/api/setup-script-url` has already normalized the bootstrap node identity.
That same shared dependency also assumes both setup routes keep canonical
Pulse identity explicit: adjacent setup and recovery-linked surfaces may not
allow `/api/setup-script` to rebuild `pulse_url` from request-local origin
state after `/api/setup-script-url` already binds the canonical Pulse URL into
the returned bootstrap artifact.
That same shared dependency also assumes `/api/setup-script` now rejects
missing `pulse_url` input outright, so adjacent setup and recovery-linked
surfaces may not rely on request-local origin fallback once the bootstrap
artifact already carries an explicit canonical Pulse URL.
That same shared dependency also assumes `/api/setup-script` keeps one
bootstrap token name end to end: embedded setup-script bootstrap uses the
canonical `setup_token` query and the rendered script body uses only
`PULSE_SETUP_TOKEN`, so adjacent setup and recovery-linked reruns do not drift
across alias variables or deleted query naming. That same recovery-linked
bootstrap path may surface the local token file path, but it must not print
the bootstrap token value itself into automatic runtime logs.
That same shared `internal/api/` dependency also assumes canonical /api/auto-register
responses keep `nodeId` on the resolved stored node identity after name
disambiguation, so adjacent setup and recovery-linked transport attachments do
not fork between saved node state and raw requested server names.
That same shared dependency also assumes canonical /api/auto-register triggers the same
canonical post-registration refresh and live event flow as legacy
auto-register, so adjacent transport surfaces do not miss discovery refresh or
canonical node event payloads just because the node entered through the
path.
That same shared `internal/api/` dependency also assumes canonical /api/auto-register
accepts caller-supplied token completion directly on that contract, so
adjacent lifecycle transport stays on one explicit-token registration contract
instead of forking a second completion path.
That same shared `internal/api/` dependency also assumes the primary runtime
ingest surface is the Pulse Unified Agent boundary in
`internal/api/agent_ingest.go` and `internal/api/router*.go`, so adjacent
storage and recovery transport may not depend on `host`-named handler or
router state as if `/api/agents/host/*` were still a first-class API family
instead of a compatibility alias.
That same shared `internal/api/` dependency also assumes the canonical
Unified Agent route family remains the primary auth/management surface:
adjacent storage and recovery-linked transport must treat `/api/agents/agent/*`
as the owned route family, while `/api/agents/host/*` and legacy
`host-agent:*` scope names remain compatibility aliases only.
That same owned route family must also fail closed on ambiguous hostname
lookups: `/api/agents/agent/lookup` may resolve a unique hostname match, but
it must not pick an arbitrary agent when exact, display-name, or short-hostname
matches are duplicated across the live inventory.
That same shared `internal/api/` dependency also assumes adjacent recovery and
storage-linked transport continues to describe those legacy names as
compatibility aliases rather than active product surfaces, so route/auth
guidance does not drift back into “host-agent” ownership language once the
canonical `agent:*` and `/api/agents/agent/*` boundary is set.
That same shared dependency now also assumes generated setup scripts use that
canonical caller-supplied completion path, so adjacent setup and recovery-linked
transport stay on the canonical registration payload shape.
That same shared dependency also assumes /api/auto-register uses one canonical
caller-supplied completion payload: transport must send `tokenId` and
`tokenValue` directly, so adjacent surfaces do not preserve a mode-switch
field or alternate payload gate.
That same shared dependency also assumes one-time setup-token auth uses the
canonical `authToken` request field only, so adjacent transport does not keep a
duplicate `setupCode` payload alias alive after the canonical field is set.
That same shared dependency also assumes the live runtime keeps that
terminology canonical after the contract cleanup: auto-register auth failures
and handler ownership paths must refer to setup tokens rather than preserving
setup-code residue.
That same shared dependency also assumes missing-token requests fail with the
canonical setup-token requirement itself rather than a generic authentication
message, so adjacent transport and setup-linked recovery proof keep the route
narrowed to one-time setup-token auth.
That same shared dependency also assumes canonical field-validation failures
stay specific on `/api/auto-register`: mismatched `tokenId`/`tokenValue` input
may not collapse into generic missing-field output, and other missing
canonical fields must return explicit `Missing required canonical
auto-register fields: ...` guidance.
That same shared dependency also assumes the public `/api/auto-register` route
and the direct canonical handler path keep those validation failures aligned,
so adjacent shared helpers do not inherit diverging missing-field or token-pair
messages from two nearby entry points on the same runtime surface.
That same shared dependency also assumes canonical auto-register callers send
explicit `serverName` identity, so the backend does not recreate node identity
from `host` and drift adjacent shared state onto handler-local fallback rules.
That same shared dependency also assumes overlap and rerun continuity logs stay
on canonical `/api/auto-register` wording, so adjacent shared helpers do not
reintroduce a deleted "secure auto-register" split while describing resolved
host matches, DHCP continuity matches, or in-place token updates.
That same shared dependency also assumes token-completion validation logs stay
on canonical `/api/auto-register` wording, so adjacent shared helpers do not
reintroduce deleted "secure token completion" wording when `tokenId` and
`tokenValue` drift out of sync.
That same shared dependency also assumes hostagent-driven canonical
/api/auto-register requests use that same request-body `authToken` field
for one-time setup-token auth instead of a header-auth fallback or direct
admin-token completion, so adjacent transport and recovery-linked proof do not
preserve parallel authentication paths.
That same shared `internal/api/` dependency also assumes the canonical helper
and proof surface describe one /api/auto-register path instead of a fake
/api/auto-register/secure sibling, so adjacent transport and governed
evidence do not drift onto a route split that the runtime does not actually
expose.
That same filter contract applies to the advanced history facets transport as a
whole: changing node or namespace filters must narrow the facets request too,
so node and namespace option sets cannot drift back to the broader chart window
while the visible history table is already scoped to a smaller recovery slice.
That same narrowing rule now also applies when a timeline day is selected: the
facets request must use the same narrowed day window as the points request so
node and namespace option sets stay coherent with the visible history slice
instead of showing options from the full chart range while the table is already
scoped to a single day.
The recovery timeline drill-down now also treats day selection as a real
history transport boundary: choosing a day in the "Backups By Date" chart must
narrow the point-history request window to that selected local day rather than
only updating local selection chrome while the table remains on the broader
chart window.
That selected-day boundary must also be durable route state: the recovery URL
must preserve the active timeline day so reload, navigation, and shared links
reconstruct the same point-history window instead of silently widening back to
the broader chart range.
That same route continuity rule also applies to the selected chart window
itself: changing the recovery timeline range to `7d`, `90d`, or `1y` must stay
in canonical recovery route state so reload, navigation, and shared links
reconstruct the same rollup and series transport window instead of widening
back to the default `30d` range.

This lane intentionally depends on other governed boundaries instead of
overreaching into them. API transport and payload contract ownership remain in
`api-contracts`, the settings recovery panel remains in `frontend-primitives`,
and canonical resource identity stays in `unified-resources`.

That same shared `internal/api/` resource boundary now also carries governed
policy-aware metadata. Storage and recovery consumers that read canonical
resource payloads must preserve backend-derived `policy` and `aiSafeSummary`
fields for storage, backup, and data-bearing resources instead of rebuilding
their own sensitivity or routing guesses in page-local presentation code. The
shared `frontend-modern/src/hooks/useUnifiedResources.ts` decode path now
trusts the backend canonical metadata directly instead of re-normalizing it
locally, so storage and recovery views see the same policy posture the API
publishes. The same hook and the resource-identity helpers it depends on now
share the canonical trimmed-string utility instead of each surface rebuilding
its own whitespace cleanup, so storage and recovery identity checks stay
aligned with the other unified-resource consumers. That same decode path
also projects Kubernetes cluster identity through the shared cluster-context
helper, so storage and recovery surfaces see the same canonical cluster
prefix as the dashboard and unified-resource store instead of rebuilding
their own fallback. That same boundary now also owns the backend facet-bundle
route for timeline history and related change counts, so storage and recovery
surfaces must continue to consume the shared bundle rather than issuing
separate local resource-detail fetches.
That same shared `internal/api/` dependency now also assumes canonical
security-token lifecycle reads. Storage- and recovery-adjacent consumers of
shared auth/security helpers may inspect token metadata, but they must not
treat a displayed relay pairing token as disposable once canonical metadata
shows `lastUsedAt`. Shared transport mutations must preserve used-token
continuity instead of deleting a credential that an already paired device
still depends on. Storage- and recovery-adjacent helpers must also treat
`owner_user_id` as API-authored identity metadata, not caller-supplied storage
or recovery metadata, so shared token records cannot be rebound through
extension-specific metadata maps.
That same shared backend helper layer now also owns hosted relay bootstrap
continuity. Storage- and recovery-adjacent consumers may read relay status or
mobile onboarding payloads, but they must not assume hosted tenants need a
manual relay settings write before those reads become valid. When hosted
billing state grants relay, the shared runtime helper must persist the
canonical hosted relay config and keep subsequent reads on that same
machine-owned state instead of letting adjacent surfaces invent their own
fallback bootstrap or disable heuristics.
That same shared `internal/api/` boundary also owns hosted browser-session
precedence for adjacent protected reads. Storage- and recovery-adjacent hosted
surfaces may run without local auth configured, but a valid tenant
`pulse_session` must still authenticate before any API-only token fallback or
the anonymous optional-auth fallback so hosted recovery, onboarding, and
support flows do not silently degrade into unauthenticated state or bearer-
token-only mode after cloud handoff.
That same recovery-adjacent auth boundary also owns local bootstrap and
break-glass containment. Before auth is configured, quick setup and recovery
ingress must stay direct-loopback only, recovery-token validation must remain
bound to the generating client IP, and break-glass recovery must clear or mint
browser sessions rather than toggling a shared `.auth_recovery` file for every
localhost caller.
That same adjacent auth boundary may consume SSO-authenticated browser
sessions, but storage/recovery code must not reinterpret SAML or
multi-provider SSO availability as a recovery entitlement. SSO license and
provider-route truth stays on the shared API/security boundary, where OIDC,
SAML, and multi-provider SSO are Community-tier authentication capabilities.
That same shared `internal/api/` boundary also owns hosted AI bootstrap
continuity. Storage- and recovery-adjacent hosted flows may surface Patrol-
backed investigation or AI-guided recovery guidance before an operator has
ever opened AI Settings, but the shared hosted runtime helper must return the
same unconfigured provider-setup state as self-hosted unless an explicit AI
config exists. Adjacent recovery surfaces must not invent their own hosted AI
fallback, synthetic activation state, or quickstart-backed machine default.
That same hosted entitlement continuity also depends on the shared refresh path
repairing the correct billing owner. When recovery-adjacent hosted requests run
under a tenant org without org-local billing state, `internal/api/` must
refresh, persist, and re-evaluate the instance-level `default` lease instead of
rewriting the empty tenant org. Otherwise AI-guided recovery can degrade to a
false free-tier state even though the hosted machine still has a valid refresh
token and signed entitlement lease.
That same shared persistence path must also clear historical hosted quickstart
model IDs before adjacent recovery or storage flows read AI settings state.
Support and recovery surfaces must not inherit or re-emit stale vendor IDs from
old `ai.enc` payloads just because the shared settings helper touched
persistence on the way through.
That same shared settings helper layer must then preserve canonical
org-management privilege for non-default tenant requests. Storage- and
recovery-adjacent hosted flows that reuse settings-bound helpers must allow
the current org owner/admin membership to continue through privileged tenant
routes after cloud handoff instead of requiring a separate configured local
admin identity that hosted tenants do not carry.
That same hosted continuity assumption also applies when operators arrive via
the older direct tenant magic-link path. Recovery- and storage-adjacent hosted
opens that still redirect through `/auth/cloud-handoff` must carry enough
canonical account/role identity for the tenant runtime to validate existing
membership and derive the stored effective role before protected routes load,
not just the newer portal exchange path. The direct path must not repair org
membership, claim a blank owner, promote email into a missing handoff subject,
or honor role upgrades from handoff claims.
That same adjacent onboarding boundary must also keep the dedicated
relay-mobile bootstrap credential sufficient for QR, deep-link, and
connection-validation reads, so hosted recovery/support flows that hand a
paired device back into onboarding do not need to escalate the mobile token to
the broader settings-read privilege just to fetch the canonical bootstrap
payload.
Those adjacent reads must also preserve the API-owned readiness semantics:
when Remote Access is disabled, relay registration has not supplied a connected
`instance_id`, or the dedicated mobile credential is missing, recovery and
support surfaces must surface the backend `409 onboarding_not_ready`
diagnostics rather than constructing a partial mobile pairing payload from
relay settings or retrying with broader settings credentials.
That same adjacent `internal/api/` boundary also owns provider-backed recovery
onboarding. Storage and recovery may consume resulting TrueNAS snapshots and
replication points, but connection CRUD, masked-secret preservation on update,
saved-connection retest paths, and platform polling setup must stay on the
adjacent platform-connections contract instead of being rebuilt as
storage/recovery-local connection flows. In particular, re-testing one saved
TrueNAS recovery source must stay on the server-owned stored-config path rather
than forcing storage or recovery surfaces to round-trip redacted secrets back
through the draft-test API; when edit-form payload overlays are present, that
same saved-connection path must merge unchanged secrets server-side instead of
making recovery-owned surfaces collect credentials again just to test updated
host or TLS fields before saving.
That same provider-backed boundary also owns connection poll cadence, last-sync
health, failure summaries, and discovered platform-contribution counts exposed
in the TrueNAS settings workspace. Storage and recovery may consume the
resulting datasets, apps, disks, and recovery artifacts, but they must not
redefine those settings-runtime health semantics or handoff routes inside
storage/recovery-local transport or page contracts.
That same adjacent platform boundary also owns the feature-default truth for
TrueNAS: storage and recovery must treat provider-backed TrueNAS recovery as
available by default and only treat `truenas_disabled` as an explicit platform
opt-out, not as the baseline onboarding state for a supported platform.
That same shared boundary also owns the line between recovery data and
assistant control. Backend-native TrueNAS app actions may refresh the poller
and recovery ingest after a control event, but storage and recovery surfaces
must continue to consume the resulting canonical recovery points instead of
growing a second recovery-local control transport or action-specific payload
contract.
That same boundary also owns the line between recovery data and assistant
diagnostics. Backend-native TrueNAS app log reads may route through shared
AI/runtime wiring and the poller's provider selection path, but storage and
recovery surfaces must not grow a second recovery-local log transport or
diagnostic payload contract just because those reads can inform operator
investigation.
That same boundary also owns the line between recovery data and assistant
configuration reads. Backend-native TrueNAS app config may route through
shared AI/runtime wiring and the poller's provider selection path, but storage
and recovery surfaces must not grow a second recovery-local config transport
or provider-shaped configuration payload just because those reads can inform
operator investigation.
Provider preflight diagnostics returned by shared AI settings handlers are the
same AI runtime readiness context. Storage and recovery surfaces may use the
resulting safe recommendation to direct an operator back to Assistant & Patrol
settings, but they must not reinterpret provider auth, provider connection, or
model-selection causes as recovery-source health, backup readiness, or
storage-control capability.
That bounded projection is the current TrueNAS floor for storage and recovery:
operators can inspect TrueNAS pools, datasets, disks, snapshots, and
replication artifacts through the shared storage and recovery pages plus
cross-surface handoffs. Storage and recovery do not promise a TrueNAS-local
onboarding path, restore/control plane, or separate diagnostic transport;
backend-native app actions, logs, and config reads stay on the adjacent
AI/runtime path and only feed refreshed canonical recovery/state afterward.
The TrueNAS platform page may embed that same canonical recovery surface as a
scoped Protection tab, but it must keep the `truenas` platform filter forced,
reuse recovery-owned protection/event workspace state, and avoid growing a
TrueNAS-only snapshot or replication table contract.
VMware vSphere is the current admitted narrower phase-1 direction. Storage and
recovery may consume vCenter-backed
datastore inventory plus VM snapshot-tree visibility as shared storage and
workload context, but VMware recovery stays out of the support claim.
Storage and recovery must not treat vSphere snapshots, changed-disk/block
visibility, or datastore presence as canonical Pulse recovery artifacts,
restore capability, or recovery-backed Assistant control until a later
governed slice adds those contracts explicitly. The shared VMware
`Resource.vmware.snapshotTree` projection may show current snapshot, creation
time, state, quiesce, and child-snapshot context on VM details, but it remains
descriptive workload evidence and must not feed recovery policy, restore
targeting, protection grouping, or backup/snapshot compliance scoring.
That same storage/recovery boundary also keeps the onboarding runtime separate
from recovery semantics. `internal/api/router.go`,
`internal/api/router_routes_registration.go`, and
`internal/api/vmware_handlers.go` may expose VMware connection CRUD, saved-test
health refresh, poller-owned `poll` / `observed` runtime summaries, and
observed datastore/VM snapshot visibility on the shared
platform-connections surface, but storage and recovery must treat that data as
setup/runtime context only, not as proof that VMware has joined the canonical
recovery artifact or restore plane.
The same rule applies to `/api/truenas/connections*/preview` and
`/api/vmware/connections*/preview`: monitored-system previews may surface
current/projected grouped systems for setup and support clarity, but they do
not imply commercial limit verdicts, recovery-local onboarding, recovery
artifact ownership, or restore support.
That same bounded phase-1 slice now also includes the shared unified-resource
adapter floor. `frontend-modern/src/hooks/useUnifiedResources.ts`,
`frontend-modern/src/routing/resourceLinks.ts`, and adjacent storage filters
may surface VMware-backed datastores as canonical `storage` resources under
the shared `vmware-vsphere` source/platform vocabulary, but that operator-
facing storage visibility still ends at inventory, capacity, and navigation.
That same shared storage adapter must classify those VMware-backed records as
inventory-only datastores on platform-owned storage surfaces, not as backup
repositories or recovery-protected targets. Shared storage rows may show
datastore topology, capacity, accessibility, and multi-host context, but they
must keep fallback protection semantics neutral unless a separately governed
recovery slice adds real VMware protection contracts.
That same inventory-only contract also applies when the operator turns on
runtime mock data. Mock TrueNAS pools/datasets and mock VMware datastores may
surface through the shared storage and recovery-adjacent pages as canonical
inventory context, but that visibility still does not widen Pulse's recovery
support claim or imply restore capability for either platform.
That same bounded mock contract now also requires one shared fixture authority.
When storage or recovery surfaces render mock TrueNAS pools, datasets,
snapshots, replication context, or mock VMware datastores, that inventory must
come from the same `internal/mock/` platform fixture owner that drives settings
payloads and unified runtime inventory, rather than recovery-local fixture
assembly or page-local compatibility fallbacks.
Storage and recovery must not infer VMware restore support, recovery rollups,
or VMware-local protection semantics from the presence of those datastores or
VM snapshot-read context on the shared pages.
That same shared adapter floor also now carries richer VMware placement,
cluster-service, guest-detail, VM virtual-hardware, VMware Tools, and network
metadata through the canonical `agent` / `vm` / `storage` / `network`
resources that storage and recovery can inspect on shared pages.
`internal/vmware/provider.go`, `internal/unifiedresources/types.go`, and
`frontend-modern/src/hooks/useUnifiedResources.ts` may expose datacenter,
cluster, cluster HA/DRS service state, folder, runtime-host,
datastore-attachment, guest-hostname, and guest-IP detail plus VM
virtual-hardware version, boot, CPU/memory hot-add, VMware Tools run-state,
version, policy, install-attempt, error, guest-reboot context, and network
attachment context as inventory context, but those fields stay descriptive
only. Storage and recovery must not treat topology labels, cluster-service
flags, datastore attachments, guest identity, network attachments,
virtual-hardware posture, or VMware Tools posture as recovery
ownership, restore targeting, protection grouping, or a new VMware-local
storage/recovery taxonomy until a separately governed slice explicitly promotes
them into recovery contracts.
That same storage/recovery surface now also owns physical-disk live I/O
presentation through the canonical chart boundary. Storage disk drawers may
show read, write, busy, and SMART history, but they must route every chart
through the shared `HistoryChart` API contract using the disk resource's
canonical history target. Storage must not keep a drawer-local live metrics
collector, agent-id/device fallback stream, or separate "real-time" history
store once monitoring and `/api/metrics-store/history` already own the disk
timeline.
The same shell/runtime split now applies to websocket consumers:
`frontend-modern/src/components/Recovery/RecoveryPointDetails.tsx` and
`frontend-modern/src/components/Storage/useStoragePageResources.ts` may consume
websocket state only through `frontend-modern/src/contexts/appRuntime.ts`.
They must not import `@/App` or create storage/recovery-local shell coupling,
because provider placement remains app-shell-owned and storage/recovery
surfaces must stay lazy-load safe.
That same adjacent `internal/api/` boundary now also governs public-demo
commercial redaction for storage and recovery viewers. Shared storage/recovery
surfaces may run beside a demo runtime that has real internal entitlements,
but `DEMO_MODE` must still 404 license-status, billing-state, and monitored-
system-ledger reads so adjacent recovery or storage pages do not leak
commercial identity or upgrade posture into a public demo. Storage/recovery
must consume that redacted boundary as presentation truth rather than
reintroducing mock-only license bypasses or page-local commercial fallbacks.
Browser-facing storage/recovery surfaces must also treat
`/api/security/status.sessionCapabilities.demoMode` as the canonical
public-demo bootstrap signal instead of inferring demo posture from headers,
`/api/health`, or hostname heuristics.
That same adjacent platform-connections boundary now also assumes direct
TrueNAS and VMware connection writes fail closed while canonical
monitored-system usage is unavailable. Storage and recovery may depend on the
resulting provider setup state only after `internal/api/truenas_handlers.go`,
`internal/api/vmware_handlers.go`, and the shared monitored-system grouping
helpers have returned a safe preview verdict; VMware write handling must not
collect external vCenter inventory before that canonical usage state is safe.
Storage and recovery browser helpers now also keep one transport-tolerant
normalization edge. Recovery display models must accept legacy subject-label
fields and nullable mode/kind metadata before presenting canonical item labels,
while storage detail drawers and filter controls must route summary series IDs,
source tones, and disk metrics through the shared storage helpers instead of
reconstructing them from local table state.
