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
    "unified-resources"
  ]
}
```

## Purpose

Own the storage and recovery product surfaces, recovery-point persistence and
querying, and the operator-facing storage health presentation layer.

## Canonical Files

1. `internal/recovery/index.go`
2. `internal/recovery/manager/manager.go`
3. `internal/recovery/store/store.go`
4. `frontend-modern/src/components/Recovery/Recovery.tsx`
5. `frontend-modern/src/components/Storage/Storage.tsx`
6. `frontend-modern/src/features/storageBackups/storageModelCore.ts`
7. `frontend-modern/src/hooks/useRecoveryPoints.ts`
8. `frontend-modern/src/hooks/useRecoveryRollups.ts`
9. `frontend-modern/src/pages/RecoveryRoute.tsx`
10. `frontend-modern/src/pages/Dashboard.tsx`
11. `frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts`
12. `frontend-modern/src/pages/DashboardPanels/RecoveryStatusPanel.tsx`
13. `frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx`
14. `frontend-modern/src/types/recovery.ts`
15. `frontend-modern/src/utils/recoveryTablePresentation.ts`
16. `frontend-modern/src/utils/textPresentation.ts`

## Shared Boundaries

1. None.

## Extension Points

1. Add or change recovery-point persistence, rollups, or series derivation through `internal/recovery/`
2. Add or change recovery page UX through `frontend-modern/src/components/Recovery/`
3. Add or change storage page UX through `frontend-modern/src/components/Storage/` and `frontend-modern/src/features/storageBackups/`
4. Route transport changes for storage and recovery endpoints through `internal/api/` and the owning `api-contracts` proof routes
5. Route canonical storage/recovery resource selection through `frontend-modern/src/hooks/useUnifiedResources.ts` and the owning `unified-resources` contract
6. Preserve API-owned node identity continuity in shared `internal/api/` helpers so storage and recovery transport attachments do not fork by hostname-versus-IP drift across the same runtime.
7. Preserve fail-closed API assignment and lookup behavior in shared `internal/api/` helpers so storage and recovery surfaces do not inherit orphaned profile or resource references from unrelated transport mutations.
8. Preserve canonical configured public endpoint selection in shared `internal/api/` helpers so recovery and storage links do not inherit loopback-local scheme drift from admin-originated setup/install flows.
9. Preserve trailing-slash normalization in those shared install-command helpers so recovery-adjacent transport and link surfaces do not inherit double-slash installer paths or slash-suffixed public endpoint drift from canonical backend install payloads.
10. Preserve canonical /api/auto-register token-action truth in shared `internal/api/` helpers so adjacent setup and recovery-adjacent transport flows stay on caller-supplied credential completion instead of reviving deleted alternate completion modes.
11. Preserve the canonical setup-script `source="script"` marker through those same shared auto-register helpers, and reject non-canonical source labels there, so later canonical reruns can keep treating script-confirmed tokens differently from agent-created tokens without reviving arbitrary caller-label compatibility.
12. Preserve the canonical auto-register node-type boundary in those same shared helpers so only supported `pve` and `pbs` registrations can complete, and unsupported runtime labels cannot bleed fake node identities into adjacent transport or recovery-adjacent state.
13. Preserve the canonical auto-register token-identity boundary in those same shared helpers so only Pulse-managed `pulse-monitor@{pve|pbs}!pulse-<canonical-scope-slug>` token IDs matching the requested node type can complete, and arbitrary, cross-type, or non-Pulse-managed token identities cannot bleed into adjacent transport or recovery-adjacent state.
14. Preserve canonical /api/auto-register DHCP continuity in those shared helpers so a PVE or PBS node that reruns registration from a new IP with the same canonical node name and deterministic Pulse-managed token identity updates in place instead of duplicating the inventory record.
15. Preserve the governed root-or-sudo Unix wrapper in shared backend install-command helpers so storage- and recovery-adjacent transport surfaces do not inherit a stale raw `| bash -s --` install payload shape from the canonical agent-install-command API and hosted Proxmox install responses.
16. Preserve optional-auth tokenless behavior in those same shared backend install-command helpers so adjacent transport surfaces do not implicitly persist API tokens and flip auth-configured state when an operator only requested a Proxmox install command on a token-optional Pulse instance.

## Forbidden Paths

1. Reintroducing storage or recovery product logic as ad hoc dashboard-only summaries without a canonical page-surface owner
2. Duplicating recovery-point normalization or rollup derivation outside `internal/recovery/`
3. Letting storage health presentation rules drift between `frontend-modern/src/components/Storage/` and `frontend-modern/src/features/storageBackups/`
4. Treating storage and recovery as implicit leftovers inside broad monitoring or E2E lanes instead of governed product surfaces
5. Writing internal `NormalizedHealth` values directly to the storage URL status param; the URL must use the canonical option values from `STORAGE_STATUS_FILTER_OPTIONS` (e.g., `available` for the Healthy filter) so that shared links and bookmarks reflect the same values that the filter dropdowns present to operators
6. Letting whitespace-padded storage route params hydrate non-canonical page state; shared storage URLs must trim and normalize `tab`, `source`, `status`, `node`, `group`, `sort`, `order`, `query`, and deep-link `resource` before the page model consumes them so pasted or hand-edited links resolve to the same canonical state as UI-authored routes without dropping adjacent unmanaged params
7. Letting storage `source` aliases or case drift survive in canonical route state; shared storage URLs must rewrite pasted values like `PVE`, `pbs`, or `ALL` to the owned source option values (for example `proxmox-pve`) or the canonical unset state so copied links match the same source filter values the storage toolbar presents
8. Letting explicit storage `all` sentinels survive in canonical route state; shared storage URLs must collapse case- or whitespace-variant `all` values for the managed `node` filter back to the canonical unset state so copied links do not preserve a fake active node filter
9. Letting whitespace-padded recovery timeline params fall off canonical route state; shared recovery URLs must trim and normalize `day`, `range`, `scope`, `status`, `verification`, `cluster`, `node`, `namespace`, and adjacent history filters before the page model validates them so pasted or hand-edited links resolve to the same canonical timeline and filter state as UI-authored routes
10. Letting explicit recovery `all` sentinels survive in canonical route state; shared recovery URLs must collapse case- or whitespace-variant `all` values for `cluster`, `node`, and `namespace` back to the canonical unset route state so copied links do not preserve fake active filters
11. Letting non-canonical recovery provider values survive in route or transport state; shared recovery URLs must collapse unsupported or fake `provider` values back to the canonical unset state, and only owned source-platform provider options or canonical aliases may reach rollups, points, series, and facets transport filters
12. Letting protected-item recovery outcome filtering fork from the canonical history status filter; the protected inventory status control must drive the same route-backed `status` field and the same rollups, points, series, and facets transport filters as the history surface instead of keeping a protected-only local outcome branch
13. Letting visible protected-item filters fall out of shared recovery links; the protected `Stale only` toggle must restore from the canonical recovery URL and rewrite to one owned `stale=1` route form instead of disappearing on refresh or copy/paste

## Completion Obligations

1. Update this contract when canonical storage or recovery entry points move
2. Keep recovery store/runtime changes aligned with the storage and recovery frontend proofs in `registry.json`
3. Tighten guardrails when legacy storage or recovery presentation paths are removed
4. Preserve the dependency split: API payload ownership stays in `api-contracts`, settings shell ownership stays in `frontend-primitives`, and canonical resource truth stays in `unified-resources`

## Current State

This subsystem now sits under the dedicated storage and recovery lane so the
operator-facing storage page, recovery timeline, and recovery-point persistence
engine stop hiding inside broader monitoring and E2E buckets.

Storage and recovery still consume the shared unified-resource contract, but
they do not own the timeline store itself. The canonical resource-change
history now lives in `internal/unifiedresources/store.go` and is surfaced
through the shared API/resource wiring, which keeps storage and recovery focused
on presentation and query shape rather than re-implementing change persistence.

The recovery backend is a real product boundary, not just a helper package:
`internal/recovery/` owns per-tenant SQLite persistence, rollup derivation,
query filtering, and recovery-point indexing for the `/api/recovery/*`
surfaces.
That same shared `internal/api/` dependency now also assumes tenant-scoped
resource handlers seed registries from canonical unified resources only:
recovery- and storage-adjacent API helpers may not fall back to raw tenant
`StateSnapshot` seeding once `UnifiedResourceSnapshotForTenant` is available.
That same shared `internal/api/` dependency now also assumes tenant AI
handlers stay on canonical Patrol runtime wiring: recovery- and
storage-adjacent API helpers must not revive tenant snapshot-provider bridges
through `internal/api/ai_handlers.go` once Patrol can initialize from tenant
`ReadState` and unified-resource providers directly.
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
The same API resource serializer also refreshes canonical identity and policy
metadata through the shared unified-resource helper before it writes resource
payloads, so storage and recovery links inherit the same canonical metadata
pass instead of carrying local attach wrappers in adjacent transport code.
The shared unified-resource facet bundle that storage-adjacent detail views
consume now also carries grouped `recentChangeKinds` counts by canonical change
kind, so storage and recovery surfaces can show the distribution of restarts,
anomalies, relationships, and capability changes without re-deriving their own
timeline breakdowns.
That same shared facet bundle now also carries grouped
`recentChangeSourceTypes` counts by canonical source type, so storage and
recovery surfaces can separate platform events, pulse diffs, heuristics,
user actions, and agent actions without inferring provenance from the loaded
slice.
That same shared facet bundle now also carries grouped
`recentChangeSourceAdapters` counts by canonical source adapter, so storage
and recovery surfaces can separate Docker, Proxmox, TrueNAS, and ops-helper
provenance without inferring integration origin from the loaded slice.
Those same resource timeline records also preserve `relatedResources` graph
context for non-relationship changes, so storage and recovery views can still
link neighboring resources when the timeline entry is a restart, anomaly, or
config update rather than only when the edge itself changes.
Those unified audit list endpoints also clamp oversized `limit` requests to
the governed maximum, so adjacent recovery and storage workflows do not turn
bounded history reads into unbounded collection scans.
The same shared API runtime now also exposes dedicated
`/api/resources/{id}/capabilities`, `/api/resources/{id}/relationships`, and
`/api/resources/{id}/timeline` reads, but storage and recovery must continue
to treat those as adjacent governed API ownership rather than storage/recovery
timeline ownership.
Those resource timeline reads now also accept governed kind and source-type
filters plus source-adapter filters, with filtered history counts owned by the
unified-resource store so storage and recovery views can consume the same
canonical history contract without re-deriving their own timeline slices.
Invalid `sourceAdapter` values are rejected at the API boundary, which keeps
storage and recovery reads aligned with the canonical adapter set instead of
turning the timeline filter into an arbitrary free-text escape hatch.
The router now wires the tenant resource state provider during initial setup
when a multi-tenant monitor is present, so tenant-scoped storage and recovery
pages do not hit a missing-provider 500 before the monitor is fully wired.
The shared unified-resource consumer hook now also preserves `capabilities`,
`relationships`, `recentChanges`, `facetCounts`, `policy`, and `aiSafeSummary`
fields when storage and recovery surfaces read unified resources, so those
pages see the same control-plane facets as the dedicated resource drawer
instead of flattening them away locally.

The frontend storage and recovery surfaces are also first-class runtime entry
points. `frontend-modern/src/components/Storage/` plus
`frontend-modern/src/features/storageBackups/` define the operator-facing
storage health model and presentation, while
`frontend-modern/src/components/Recovery/` and the recovery hooks define the
timeline, protected-item, and recovery-summary UX.
The recovery table presentation helper now owns the canonical subject-type
label fallback for recovery rows and delegates its title-casing to the shared
`frontend-modern/src/utils/textPresentation.ts` helper rather than keeping a
local recovery-only formatter, so subject and outcome labels stay aligned with
the shared frontend label contract.
Those transport hooks are direct governed runtime surfaces, not just page
implementation detail: `frontend-modern/src/hooks/useRecoveryPoints.ts`,
`frontend-modern/src/hooks/useRecoveryPointsFacets.ts`,
`frontend-modern/src/hooks/useRecoveryPointsSeries.ts`, and
`frontend-modern/src/hooks/useRecoveryRollups.ts` must stay on the explicit
`recovery-product-surface` proof path instead of inheriting release-control
coverage only through `frontend-modern/src/pages/RecoveryRoute.tsx`.
That same rule applies to the dashboard recovery entry points too:
`frontend-modern/src/hooks/useDashboardRecovery.ts`,
`frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts`,
`frontend-modern/src/pages/DashboardPanels/RecoveryStatusPanel.tsx`, and
`frontend-modern/src/utils/dashboardRecoveryPresentation.ts` must stay on
explicit direct dashboard/recovery proof routing instead of inheriting
coverage only through the full recovery route or broader dashboard shells.
The storage dashboard entry point must be treated the same way on the storage
side: `frontend-modern/src/pages/Dashboard.tsx`,
`frontend-modern/src/pages/DashboardPanels/dashboardWidgets.ts`, and
`frontend-modern/src/pages/DashboardPanels/StoragePanel.tsx` must stay on
explicit direct dashboard/storage proof routing instead of borrowing release-
control coverage only from the broader storage page and model surfaces.
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
That same direct proof rule also applies to the shared recovery summary helper:
`frontend-modern/src/utils/recoverySummaryPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
That same direct proof rule also applies to the shared recovery record helper:
`frontend-modern/src/utils/recoveryRecordPresentation.ts` must stay on the
explicit `recovery-product-surface` proof path instead of inheriting coverage
only through pages or higher-level recovery components.
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
contract: protected-item rollups, point history, facets, and chart series must
all honor the same provider, cluster, node, namespace, workload-scope,
verification, and route-backed free-text `q` filter so the protected-items
list cannot drift from the timeline and facet state under the same active
recovery view.
That shared unified-resource dependency now also includes policy-governed
resource metadata on the frontend decode path: storage and recovery surfaces
that route through `frontend-modern/src/hooks/useUnifiedResources.ts` must
preserve canonical `policy` and `aiSafeSummary` fields so storage-bearing
resources do not silently lose their routing or redaction posture when they
cross from unified-resource ownership into storage or recovery presentation.
That same decode path also trims `aiSafeSummary` through the shared policy
normalizer, so storage and recovery surfaces keep the canonical summary text
aligned with the policy-aware resource contract instead of reformatting it
locally.
That same decode path now delegates policy string and redaction normalization
to `frontend-modern/src/utils/resourcePolicyNormalization.ts`, so storage and
recovery surfaces do not reimplement sensitivity, routing, or redaction
parsing locally.
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
That same shared `internal/api/` dependency now also assumes auth persistence
compatibility stays on an explicit migration/import boundary: legacy
raw-token `sessions.json` and `csrf_tokens.json` files may load for upgrade
continuity, but `session_store.go` and `csrf_store.go` must immediately
rewrite hashed canonical persistence on load so adjacent storage and recovery
transport does not keep running against primary-path raw-token files.
That same shared `internal/api/` dependency now also assumes adjacent
commercial helper surfaces speak in monitored-system terms: recovery- or
storage-adjacent API wiring may consume the canonical monitored-system ledger
and monitored-system cap helpers, but it must not revive deleted agent-era
helper names or imply that API-backed infrastructure sits outside the counted
system model.
That same shared `internal/api/` dependency now also assumes self-hosted
commercial counting is canonical at the top-level monitored-system boundary:
shared setup, deploy, entitlement, and API-backed monitoring helpers may not
preserve an API-only exemption that would let storage- or recovery-adjacent
systems consume no commercial slot when the same monitored system is visible
through canonical unified-resource roots.
That same shared `internal/api/` dependency also assumes session-carried OIDC
refresh tokens stay fail-closed at rest: `session_store.go` may only persist
or recover those tokens through encrypted-at-rest session payloads, and any
missing-crypto or invalid-ciphertext path must drop the refresh token instead
of preserving plaintext-at-rest session state that storage and recovery
surfaces might inherit through shared auth runtime helpers.
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
across alias variables or deleted query naming.
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
in canonical `/recovery` route state so reload, navigation, and shared links
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
their own sensitivity or routing guesses in page-local presentation code. That
same boundary now also owns the backend facet-bundle route for capability,
relationship, and timeline history reads, so storage and recovery surfaces
must continue to consume the shared bundle rather than issuing separate local
resource-detail fetches.
