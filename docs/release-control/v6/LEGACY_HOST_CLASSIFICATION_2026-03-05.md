# Legacy Host Classification Audit

Date: 2026-03-05
Scope: `pulse` repo only, focused on v6-facing paths and release verification artifacts.

## Verdict

No additional active v6 runtime leaks were found after fixing the agent-registration journey.

The remaining `host` references in v6-adjacent code fall into three intentional buckets:

1. Compatibility boundaries that explicitly reject or normalize legacy input.
2. Internal state or wire-format shims that still bridge older model names to v6-facing output.
3. Non-resource semantics where `host` means hostname, SSH host key, backup type, or endpoint host.

## Must Remove

None found in active v6 runtime/read surfaces during this pass.

The previously-found leak is now fixed:

- `tests/integration/tests/journeys/04-agent-install-registration.spec.ts`
- `tests/integration/evals/tasks/agent-registration.md`

Those artifacts now require unified `resources[]` and `agent.type = "unified"`.

## Intentional Compatibility Boundaries

These are correct to keep because they harden v6 against old clients or old persisted inputs:

- `internal/api/resources_test.go`
  Confirms `/api/resources?type=host` is rejected.
- `internal/api/org_handlers_test.go`
  Confirms organization sharing rejects `resourceType: "host"`.
- `internal/api/ai_handler_test.go`
  Confirms legacy chat mention aliases like `host`, `container`, and `k8s` are dropped.
- `internal/api/ai_handlers_test.go`
  Confirms `target_type: "host"` is rejected for run-command normalization.
- `internal/monitoring/monitor_helpers.go`
  Keeps explicit legacy-agent detection: only `agent.type = "unified"` is canonical.

## Intentional Internal Shims

These are still legacy-shaped internally, but they are not evidence of v6 model leakage:

- `internal/monitoring/monitor.go`
  `monitorLegacyResourceType()` is the frontend wire-shape mapper for `/api/state` broadcast payloads.
  It emits canonical v6-facing types like `agent`, `node`, and `docker-host`.
- `internal/models/state_snapshot.go`
  `StateSnapshot` and `ResolveResource()` still model host-agent state for producer/wire compatibility.
  This is not the canonical read model; `ReadState` and unified resources are.

## Internal Models Follow-Up

The remaining `internal/models` host-era surface is still serving live compatibility and routing work.
It should not be treated as dead migration residue.

### Must Remain For Release

- `internal/models/models.go`
  `Host`, `HostSensorSummary`, `HostDiskSMART`, `ClearAllHosts()`, `LinkHostAgentToNode()`,
  and `UnlinkHostAgent()` still back the host-agent ingest and linking flow.
- `internal/models/models_frontend.go`
  `HostFrontend` and the `StateFrontend.Hosts` field remain part of the internal/frontend wire DTO layer.
  `/api/state` strips that array for the canonical v6 contract, but the model still exists for compatibility,
  websocket shaping, mock data, and legacy-facing internal consumers.
- `internal/models/converters.go`
  `Host -> HostFrontend` conversion is still the compatibility bridge from host-agent state into the
  older frontend DTO family.
- `internal/models/state_snapshot.go`
  `StateSnapshot.Hosts` and `ResolveResource()` still support host-agent lookup/routing for compatibility flows.

### Active Runtime Boundaries Still Using That Surface

- `internal/api/router.go`
  Metrics/history and live metric fallback still read `snap.Hosts` when the canonical resource type is `agent`.
- `internal/api/agent_ingest.go`
  Host agent lookup and registration validation still scan the live `snap.Hosts` snapshot.
- `internal/monitoring/monitor_agents.go`
  `ApplyHostReport()` still writes into `models.Host` state before unified-resource ingestion layers consume it.
- `internal/ai/chat/context_prefetch.go`
  Chat mention prefetch already uses `ReadState.Hosts()` and maps those records to canonical `agent` mentions.
  This is canonical at the read boundary even though the underlying source model is still named `Host`.

## API Runtime Follow-Up

The remaining host-era naming inside `internal/api` is concentrated in compatibility handlers and helper locals.
It is still part of the supported release path for agent install, lookup, and live metric fallback.

### Must Remain For Release

- `internal/api/agent_ingest.go`
  `HostAgentHandlers` is still the compatibility boundary for the Pulse Unified Agent runtime.
  It reads `GetLiveStateSnapshot().Hosts` to:
  - validate installer lookup requests
  - resolve config fetch scope
  - enforce token-to-agent ownership
- `internal/api/router.go`
  live metric fallback still resolves canonical `agent` resources through `snap.Hosts`
  when building instant metric points for runtime reads.
- `internal/api/host_agents_test.go`
  keeps the lookup/install compatibility behavior pinned while those handlers still exist.

### Why This Is Not A V6 Leak

- The external resource type at the API boundary is still `agent`, not `host`.
- The remaining host-era code is operating on compatibility storage names (`models.Host`, `snap.Hosts`)
  after canonical request normalization has already happened.
- Existing API tests already pin explicit rejection of removed `host` aliases in:
  - AI/chat
  - org shares
  - reporting
  - metrics history
  - resources/discovery

### Post-Release Refactor Targets

These are the next cleanup candidates once the host-agent compatibility/state layer is intentionally renamed or retired:

- `internal/api/router.go`
  local helpers like `findHost` and comments that still describe the `agent` path in host-era terms
- `internal/api/agent_ingest.go`
  `HostAgentHandlers` naming, local `host` variables, and `snap.Hosts` comments
- `internal/api/host_agents_test.go`
  test names and fixtures that still describe the canonical agent flow as `host` lookup/config management

These are naming/structure refactors, not release blockers.

### Post-Release Rename Candidates

These look like rename-only cleanup once the host-agent compatibility/state layer is intentionally retired or renamed:

- `internal/models/models.go`
  `Host`, `HostSensorSummary`, `HostDiskSMART`, `HostCephCluster`
- `internal/models/models_frontend.go`
  `HostFrontend`, `HostSensorSummaryFrontend`, `HostDiskSMARTFrontend`
- `internal/models/converters.go`
  `ToFrontend converts a Host to HostFrontend`
- `internal/models/state_snapshot.go`
  comments like `Check generic Hosts` and the `hosts` local variable naming

These are naming debt, not current v6 correctness bugs.

## Non-Resource Host Terminology

These are unrelated to the removed v5 resource type and should not be treated as migration debt:

- `internal/mock/generator.go`
  Backup payloads use `Type: "host"` to mean PMG host config backups, not unified resources.
- `internal/monitoring/knownhosts.go`
  SSH known-hosts management.
- Node configuration and endpoint code using `host` as a network address or URL host field.

## Ratchets Added

- `internal/unifiedresources/code_standards_test.go`
  Added `TestV6AgentRegistrationArtifactsStayCanonical` to:
  - scan all `tests/integration/tests/**/*.{ts,tsx}` and `tests/integration/evals/**/*.md`
    for legacy host-resource usage patterns
  - prevent regressions in:
  - `tests/integration/tests/journeys/04-agent-install-registration.spec.ts`
  - `tests/integration/evals/tasks/agent-registration.md`

The ratchet bans:

- `state.hosts`
- `hosts array`
- `type: 'host'`
- `agent.type = "host"`
- `resourceType: "host"` / `resourceType: 'host'`
- `/api/resources?type=host`

And requires:

- unified `resources[]`
- unified agent marker `type: 'unified'` / `agent.type = "unified"`

- `internal/unifiedresources/code_standards_test.go`
  Added `TestV6ReleaseFacingAPITestsCoverLegacyHostRejection` to pin release-facing API tests that
  explicitly reject removed `host` aliases in chat, AI actions, org shares, reporting, and metrics history.
