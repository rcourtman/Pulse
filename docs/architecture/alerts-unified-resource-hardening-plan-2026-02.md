# Alerts Unified-Resource Hardening Plan (Detailed Execution Spec)

Status: Draft
Owner: Pulse
Date: 2026-02-08

Progress tracker:
- `docs/architecture/alerts-unified-resource-hardening-progress-2026-02.md`

## Product Intent

Alerts must be fully compatible with the unified resource model while preserving current user-facing behavior.

This plan has two top-level goals:
1. Alerts are unified-resource-native end to end.
2. Alerts remain tenant-safe and regression-safe.

## Non-Negotiable Contracts

1. Tenant isolation contract:
- No cross-org alert events over websocket.
- No cross-org alert history/incident leakage.
- No cross-org push/AI alert side effects.

2. Unified model contract:
- Alert type and resource identity must derive from canonical unified resource metadata, not fragile string/path heuristics.
- Legacy typed monitor flows may exist only as compatibility wrappers during migration.

3. UI compatibility contract:
- Alerts UX behavior remains stable for single-tenant users.
- Unified resources become the primary source for alert resource classification in frontend.

4. Contract lock contract:
- Alert payload shapes are locked by schema/golden tests.
- Any contract drift requires explicit approval and migration notes.

5. Rollback contract:
- Rollback path exists for each high-risk packet.
- Rollback does not require data loss and is documented per packet.

## Orchestrator Operating Model

Use fixed roles per packet:
- Implementer: delegated coding agent.
- Reviewer: orchestrator.

A packet can be marked DONE only when:
- all packet checkboxes are checked,
- all listed commands are run with explicit exit codes,
- reviewer gate checklist passes,
- verdict is `APPROVED`.

## Required Review Output (Every Packet)

```markdown
Files changed:
- <path>: <reason>

Commands run + exit codes:
1. `<command>` -> exit 0
2. `<command>` -> exit 0

Gate checklist:
- P0: PASS | FAIL (<reason>)
- P1: PASS | FAIL | N/A (<reason>)
- P2: PASS | FAIL (<reason>)

Verdict: APPROVED | CHANGES_REQUESTED | BLOCKED

Residual risk:
- <risk or none>

Rollback:
- <steps>
```

## Global Validation Baseline

Run after every packet unless explicitly waived:

1. `go build ./...`
2. `go test ./internal/alerts/... -v`
3. `go test ./internal/api/... -run "Alerts|Contract|Incident" -v`
4. `go test ./internal/monitoring/... -run "Alert|Tenant|Isolation" -v`
5. `go test ./internal/websocket/... -run "Alert|Tenant" -v`
6. `go test ./internal/ai/unified/... -run "Alert" -v`
7. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
8. `npm --prefix frontend-modern exec -- vitest run src/pages/__tests__/Alerts.helpers.test.ts src/components/Alerts/__tests__/ThresholdsTable.test.tsx`

Notes:
- `go build` alone is never sufficient for approval.
- Empty or timed-out command output is not valid evidence.

## Execution Packets

### Packet 00: Surface Inventory and Risk Register

Objective:
- Build an explicit map of all alert surfaces still tied to legacy model assumptions.

Scope:
- `internal/alerts/alerts.go`
- `internal/monitoring/monitor_alerts.go`
- `internal/api/alerts*.go`
- `internal/api/router.go`
- `internal/websocket/hub.go`
- `frontend-modern/src/pages/Alerts.tsx`
- `frontend-modern/src/hooks/useResources.ts`

Implementation checklist:
1. Enumerate every alert emission path (state, event, push, AI).
2. Enumerate every resource-type derivation path and classify as canonical vs heuristic.
3. Enumerate every frontend alert UI code path still consuming legacy arrays.
4. Produce risk register with severity, owner, and mitigation sequence.

Required tests:
1. `go test ./internal/api/... -run Alerts -v`
2. `go test ./internal/alerts/... -run Alert -v`

Exit criteria:
- Risk register exists and each high-severity item is mapped to a packet.

### Packet 01: Websocket Alert Tenant Isolation Hardening

Objective:
- Eliminate cross-tenant alert event leakage.

Scope:
- `internal/monitoring/monitor_alerts.go`
- `internal/websocket/hub.go`
- `internal/websocket/*tenant*test.go`

Implementation checklist:
1. Replace global alert event broadcasts with tenant-scoped broadcast path.
2. Ensure both fired and resolved events carry tenant scope end to end.
3. Ensure default/single-tenant behavior remains correct.
4. Add regression tests proving no cross-org delivery.

Required tests:
1. `go test ./internal/websocket/... -run "Alert|Tenant|Isolation" -v`
2. `go test ./internal/monitoring/... -run "Alert|Tenant" -v`

Exit criteria:
- Tenant A never receives tenant B alert events in tests.

### Packet 02: Canonical Alert Identity and Resource-Type Contract

Objective:
- Define a single canonical contract for `resource_id`, `resource_type`, `platform_type`, and `source_type`.

Scope:
- `internal/alerts/`
- `internal/api/router.go`
- `internal/ai/unified/alerts.go`
- `frontend-modern/src/types/`

Implementation checklist:
1. Publish contract doc snippet inside plan appendix or dedicated doc section.
2. Replace path-pattern fallback logic (`/qemu/`, `/lxc/`, etc.) where feasible with canonical metadata.
3. Add explicit compatibility behavior for legacy IDs during migration.
4. Add snapshot tests to detect contract drift.

Required tests:
1. `go test ./internal/api/... -run "Contract|Alert" -v`
2. `go test ./internal/ai/unified/... -run "Alert" -v`

Exit criteria:
- No critical path depends on brittle path matching for type derivation.

### Packet 03: Unified Resource Evaluation Adapter (Backend)

Objective:
- Introduce a unified-resource-first evaluation path for alerts.

Scope:
- `internal/alerts/alerts.go`
- `internal/unifiedresources/`
- `internal/monitoring/`

Implementation checklist:
1. Implement adapter layer that evaluates alerts from unified resource records.
2. Keep existing `CheckGuest/CheckNode/...` methods as wrappers only (temporary), routing into shared evaluator logic.
3. Preserve threshold semantics and hysteresis behavior.
4. Add unit tests for evaluator parity across major resource families.

Required tests:
1. `go test ./internal/alerts/... -run "Threshold|Hysteresis|Alert" -v`
2. `go test ./internal/monitoring/... -run "Alert" -v`

Exit criteria:
- Core evaluation logic runs through unified model abstractions.

### Packet 04: Monitor Integration Migration to Unified Evaluator

Objective:
- Remove direct typed fan-out from monitoring pollers where practical.

Scope:
- `internal/monitoring/monitor_*.go`
- `internal/alerts/alerts.go`

Implementation checklist:
1. Migrate monitor integration to call unified evaluation entry points.
2. Keep typed calls only where source data is not yet represented in unified resources; document each exception.
3. Add per-source mapping table: source payload -> unified resource -> alert evaluation path.
4. Add tests for parity with current behavior.

Required tests:
1. `go test ./internal/monitoring/... -run "Alert|Backup|Snapshot" -v`
2. `go test ./internal/alerts/... -run "Backup|Snapshot" -v`

Exit criteria:
- Typed evaluation fan-out reduced to documented exceptions only.

### Packet 05: Alerts Frontend Migration to Unified Resource Source

Objective:
- Make alerts UI resource classification and lookup unified-resource-native.

Scope:
- `frontend-modern/src/pages/Alerts.tsx`
- `frontend-modern/src/hooks/useResources.ts`
- `frontend-modern/src/types/resource.ts`
- `frontend-modern/src/types/alerts.ts`

Implementation checklist:
1. Replace legacy-array-first resource lookup with unified resources as primary source.
2. Keep backward compatibility fallback only where initial load requires it.
3. Remove redundant `getResourceType` heuristics tied to legacy slices.
4. Add UI tests for mixed resource types, unknown resources, and fallback behavior.

Required tests:
1. `npm --prefix frontend-modern exec -- vitest run src/pages/__tests__/Alerts.helpers.test.ts`
2. `npm --prefix frontend-modern exec -- vitest run src/components/Alerts/__tests__/ThresholdsTable.test.tsx`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`

Exit criteria:
- Alerts page can classify/render resources from unified data without legacy arrays.

### Packet 06: Threshold Overrides and ID Normalization Hardening

Objective:
- Ensure per-resource overrides remain stable under unified identifiers.

Scope:
- `internal/alerts/alerts.go`
- `internal/api/alerts.go`
- `frontend-modern/src/pages/Alerts.tsx`

Implementation checklist:
1. Define canonical override key format and migration from legacy keys.
2. Add migration path for existing stored override IDs.
3. Ensure backups/snapshots and storage overrides resolve consistently.
4. Add regression tests for old-key compatibility.

Required tests:
1. `go test ./internal/alerts/... -run "Override|Threshold|Migration" -v`
2. `go test ./internal/api/... -run "Alerts" -v`

Exit criteria:
- Existing user overrides continue to apply after unified ID normalization.

### Packet 07: API and Incident Timeline Contract Locking

Objective:
- Lock alert/incident API contracts with strict schema tests.

Scope:
- `internal/api/alerts.go`
- `internal/api/alerts_endpoints_test.go`
- `internal/api/contract_test.go` (or alerts-focused snapshot test file)

Implementation checklist:
1. Add or extend snapshots for active alerts, history alerts, incidents, and acknowledge payloads.
2. Ensure field naming/casing consistency is explicit and tested.
3. Ensure compatibility notes are documented for mobile/frontend clients.
4. Validate error response schema consistency.

Required tests:
1. `go test ./internal/api/... -run "AlertsEndpoints|Contract|Incident" -v`

Exit criteria:
- Contract drift causes test failures before merge.

### Packet 08: AI Alert Bridge and Enrichment Parity

Objective:
- Ensure AI alert/finding bridge consumes canonical alert metadata and remains tenant-safe.

Scope:
- `internal/ai/unified/alerts.go`
- `internal/ai/unified/alerts_adapter.go`
- `internal/api/router.go`

Implementation checklist:
1. Replace heuristic resource typing in AI bridge with canonical metadata where available.
2. Ensure bridge behavior for unknown resource type is deterministic.
3. Add tests for alert-to-finding conversion parity.
4. Validate tenant context propagation through AI event surfaces.

Required tests:
1. `go test ./internal/ai/unified/... -run "Alert|Adapter|Bridge" -v`
2. `go test ./internal/api/... -run "Contract" -v`

Exit criteria:
- AI bridge output remains stable and type-correct across resource families.

### Packet 09: Operational Safety (Feature Flag, Rollout, Rollback)

Objective:
- De-risk rollout of unified alerts with controlled fallback.

Scope:
- config/feature-flag files and alert initialization wiring
- docs runbook sections for alerts rollback and validation

Implementation checklist:
1. Add or verify feature flag for unified evaluator path.
2. Document staged rollout sequence and monitoring checkpoints.
3. Document rollback command path and data compatibility expectations.
4. Add smoke checks for flag on/off parity.

Required tests:
1. `go test ./internal/alerts/... -run "Flag|Fallback|Migration" -v`
2. `go build ./...`

Exit criteria:
- Rollout and rollback are executable and test-validated.

### Packet 10: Final Certification

Objective:
- Certify alerts as unified-resource-ready and tenant-safe.

Implementation checklist:
1. Run full validation baseline.
2. Produce final parity matrix by resource family and transport channel.
3. Record known residual risks and explicit acceptance.
4. Update progress tracker and final verdict.

Required tests:
1. `go build ./...`
2. `go test ./...`
3. `frontend-modern/node_modules/.bin/tsc --noEmit -p frontend-modern/tsconfig.json`
4. `npm --prefix frontend-modern exec -- vitest run`

Exit criteria:
- Reviewer signs `APPROVED` with all gates passing.

## Acceptance Definition

Plan is complete only when:
1. Packet 00-10 are `DONE` in progress tracker.
2. Every packet has explicit reviewer evidence and verdict.
3. Alert behavior is verified tenant-safe, unified-resource-native, and UI-contract-safe.

## Appendices

### Appendix A: Alert Surface Inventory

#### A.1 Alert Emission Paths

| Path ID | Source | Transport | Tenant-Scoped | Notes |
| --- | --- | --- | --- | --- |
| A1-S01 | `internal/alerts/alerts.go:6140` (`checkMetric`) | State alert (threshold poll) | Partial | Emits via `dispatchAlert` at `internal/alerts/alerts.go:6355` (new) and `internal/alerts/alerts.go:6422` (re-notify). |
| A1-S02 | `internal/alerts/alerts.go:2529` (`CheckGuest` -> `checkMetric`) | State alert (guest CPU/mem/disk/IO thresholds) | Partial | Poll-evaluated guest metrics (`internal/alerts/alerts.go:2529-2629`). |
| A1-S03 | `internal/alerts/alerts.go:2709` (`CheckNode` -> `checkMetric`) | State alert (node thresholds) | Partial | Poll-evaluated node metrics + temperature (`internal/alerts/alerts.go:2709-2724`). |
| A1-S04 | `internal/alerts/alerts.go:2955` (`CheckHost` -> `checkMetric`) | State alert (host/host-disk thresholds) | Partial | Host CPU/memory/disk/disk-temp paths (`internal/alerts/alerts.go:2955-3047`). |
| A1-S05 | `internal/alerts/alerts.go:3583` (`CheckPBS` -> `checkMetric`) | State alert (PBS thresholds) | Partial | PBS CPU/memory threshold checks (`internal/alerts/alerts.go:3583-3585`). |
| A1-S06 | `internal/alerts/alerts.go:3931` (Docker container metrics -> `checkMetric`) | State alert (container thresholds) | Partial | Docker container CPU/memory/disk threshold checks (`internal/alerts/alerts.go:3931-3982`). |
| A1-S07 | `internal/alerts/alerts.go:5060` (`CheckStorage` -> `checkMetric`) | State alert (storage usage threshold) | Partial | Storage usage threshold path (`internal/alerts/alerts.go:5060`). |
| A1-S08 | `internal/alerts/alerts.go:7278` (`checkPMGQueueDepths`) | State alert (PMG queue thresholds) | Partial | Direct dispatch for queue-depth/deferred/hold thresholds (`internal/alerts/alerts.go:7278,7333,7388`). |
| A1-S09 | `internal/alerts/alerts.go:7470` (`checkPMGOldestMessage`) | State alert (PMG oldest message threshold) | Partial | Direct dispatch for age threshold (`internal/alerts/alerts.go:7470`). |
| A1-S10 | `internal/alerts/alerts.go:7900` (`checkQuarantineMetric`) | State alert (PMG quarantine thresholds) | Partial | Thresholded quarantine growth/volume alerts (`internal/alerts/alerts.go:7900`). |
| A1-S11 | `internal/alerts/alerts.go:8210` (`checkAnomalyMetric`) | State alert (PMG anomaly thresholds) | Partial | Baseline/anomaly threshold dispatch (`internal/alerts/alerts.go:8210`). |
| A1-S12 | `internal/alerts/alerts.go:6928` (`checkNodeOffline`) | State alert (connectivity state on poll) | Partial | Node offline path; similar offline state paths for PBS/PMG/storage (`internal/alerts/alerts.go:7056,7175,8303`). |
| A1-S13 | `internal/alerts/alerts.go:8442` (`checkGuestPoweredOff`) | State alert (guest power state) | Partial | Powered-off state alert path (`internal/alerts/alerts.go:8442`). |
| A1-E01 | `internal/alerts/alerts.go:3144` (`CheckHost`) | Event alert (RAID degraded/rebuild) | Partial | RAID discrete events emitted at `internal/alerts/alerts.go:3144,3185`. |
| A1-E02 | `internal/alerts/alerts.go:4130` (`evaluateDockerService`) | Event alert (Docker service degraded/offline) | Partial | Service-level event alerts (`internal/alerts/alerts.go:4130,4161`). |
| A1-E03 | `internal/alerts/alerts.go:3380` (`HandleHostOffline`) | Event alert (host-agent offline) | Partial | Host offline discrete event (`internal/alerts/alerts.go:3380`). |
| A1-E04 | `internal/alerts/alerts.go:4319` (`HandleDockerHostOffline`) | Event alert (Docker host offline) | Partial | Discrete Docker host offline event (`internal/alerts/alerts.go:4319`). |
| A1-E05 | `internal/alerts/alerts.go:4420` (Docker container event checks) | Event alert (container state/health/restart/OOM/memory/update) | Partial | Emission points: `4420,4486,4590,4655,4745,4886`. |
| A1-E06 | `internal/alerts/alerts.go:5335` (`CheckSnapshotsForInstance`) | Event alert (snapshot age) | Partial | Snapshot-age event alert path (`internal/alerts/alerts.go:5335`). |
| A1-E07 | `internal/alerts/alerts.go:5760` (`CheckBackups`) | Event alert (backup age) | Partial | Backup-age event alert path (`internal/alerts/alerts.go:5760`). |
| A1-E08 | `internal/alerts/alerts.go:5827` (`checkZFSPoolHealth`) | Event alert (ZFS state/errors/device) | Partial | ZFS event emission points `5827,5883,5949`. |
| A1-E09 | `internal/alerts/alerts.go:9758` (`CheckDiskHealth`) | Event alert (SMART temperature/wearout) | Partial | Disk health event emission points `9758,9828`. |
| A1-E10 | `internal/alerts/alerts.go:9314` (`LoadActiveAlerts`) | Event replay (startup critical replay) | Partial | Re-dispatches restored critical alerts after restart. |
| A1-E11 | `internal/alerts/alerts.go:6767` (`NotifyExistingAlert`) via `internal/api/alerts.go:226-233` (`ActivateAlerts`) | Event replay (activation replay) | Partial | API activation re-dispatches active critical alerts. |
| A1-E12 | `internal/alerts/alerts.go:9085` (`checkEscalations` -> `safeCallEscalateCallback`) | Escalation event | Partial | Escalation callback path from scheduled checker (`internal/alerts/alerts.go:9043-9085`). |
| A1-E13 | `internal/alerts/alerts.go:5987` (`clearAlert`), `internal/alerts/alerts.go:9864` (`clearAlertNoLock`), `internal/alerts/alerts.go:6486-6488` (`checkMetric` resolve) | Resolved event emission | Partial | Resolved callbacks emit via `onResolved` path. |
| A1-P01 | `internal/monitoring/monitor_alerts.go:41` (`handleAlertFired`) | Push fan-out | Partial | Pushes fired alert to WS (`:47`), notifications (`:55`), incident store (`:59`), AI callback (`:63-72`). |
| A1-P02 | `internal/monitoring/monitor_alerts.go:76` (`handleAlertResolved`) | Push fan-out | Partial | Pushes resolved to WS (`:80`), incidents (`:88`), AI (`:93-99`), optional resolved notification (`:118`). |
| A1-P03 | `internal/api/alerts.go:612` et al. (ack/unack/clear/bulk handlers) | Push state update | Partial | Manual alert operations call `SyncAlertState` then broadcast raw state via `h.wsHub.BroadcastState(...)` (e.g. `internal/api/alerts.go:628,710,768,812,855,910,976,1035`). |
| A1-P04 | `internal/websocket/hub.go:940` (`BroadcastAlert`), `internal/websocket/hub.go:950` (`BroadcastAlertResolved`) | WebSocket broadcast transport | No | Both use global `BroadcastMessage` (`internal/websocket/hub.go:976`) into global channel handled for all clients (`internal/websocket/hub.go:565-569`). |
| A1-A01 | `internal/alerts/alerts.go:788` (`SetAlertForAICallback`) + emitters at `internal/alerts/alerts.go:4298-4307` and `internal/alerts/alerts.go:6329-6338` | AI alert path (suppression-bypass) | Partial | AI callback intentionally bypasses activation/quiet-hour suppression. |
| A1-A02 | `internal/monitoring/monitor_alerts.go:63-72` and `internal/monitoring/monitor_alerts.go:93-99` | AI callback transport | Partial | Monitor-level triggered/resolved AI callback fan-out. |
| A1-A03 | `internal/api/router.go:3817-3852` (`WireAlertTriggeredAI`) | AI patrol trigger path | No | Wires monitor alert callback to patrol/analyzer directly; no tenant argument in callback signature. |
| A1-A04 | `internal/api/router.go:2389-2392`, `internal/api/router.go:2410-2430` | AI history ingestion path | No | Alert history feeds pattern/correlation detectors. |
| A1-A05 | `internal/api/router.go:2658-2717` | AI alert-to-finding bridge | No | Unified `AlertBridge` started and patrol trigger attached; scope carried as `resourceID/resourceType` only. |

#### A.2 Resource-Type Derivation Paths

| Path ID | Location | Method | Classification (Canonical/Heuristic) | Notes |
| --- | --- | --- | --- | --- |
| A2-RT01 | `internal/alerts/alerts.go:2306-2345` | `CheckGuest` derives type from concrete model (`models.VM` vs `models.Container`) into `guestType`. | Canonical | Typed model switch, not string-path matching. |
| A2-RT02 | `internal/alerts/alerts.go:6257-6259`, `internal/alerts/alerts.go:6374` | `checkMetric` writes/refreshes `metadata.resourceType` from explicit `resourceType` argument. | Canonical | Primary backend resource-type field propagation for threshold alerts. |
| A2-RT03 | `internal/alerts/alerts.go:6092-6130` | `canonicalResourceTypeKeys` normalizes aliases for config lookup. | Canonical | Explicit canonicalization map for threshold keys. |
| A2-RT04 | `internal/alerts/alerts.go:1714-1889` | `reevaluateActiveAlertsLocked` infers type by splitting alert IDs and checking `alert.Instance`, prefixes, and string patterns (`:storage/`, docker prefixes). | Heuristic | Fragile fallback logic for threshold re-evaluation. |
| A2-RT05 | `internal/alerts/alerts.go:9202-9227` | `LoadActiveAlerts` legacy migration infers guest alerts from `alert.Type` substring checks and `ResourceID` split parsing. | Heuristic | Legacy ID migration logic depends on naming conventions. |
| A2-RT06 | `internal/api/router.go:3858-3892` | `deriveResourceTypeFromAlert` uses `alert.Type`/`ResourceID` string matching (`/qemu/`, `/lxc/`, `docker`) and fallback `guest`. | Heuristic | Currently referenced by tests only (`internal/api/router_helpers_additional_test.go:267`). |
| A2-RT07 | `internal/api/router.go:2426` | Correlation detector stores `ResourceType: alert.Type`. | Heuristic | Conflates alert metric/event type with resource type. |
| A2-RT08 | `frontend-modern/src/pages/Alerts.tsx:4672-4728` | `getResourceType` uses `metadata.resourceType` first, then legacy-array name/id matching across VM/CT/node/storage/docker/PBS/Ceph. | Heuristic | Hybrid path; canonical metadata shortcut with heuristic fallback. |
| A2-RT09 | `frontend-modern/src/pages/Alerts.tsx:909-917` | Override mapping derives VM vs CT via `guest.type === 'qemu' ? 'VM' : 'CT'`. | Heuristic | Legacy platform-specific assumption. |
| A2-RT10 | `frontend-modern/src/hooks/useResources.ts:92-94` | Unified resources derive type from `state.resources[].type`. | Canonical | Preferred unified source path. |
| A2-RT11 | `frontend-modern/src/hooks/useResources.ts:281`, `frontend-modern/src/hooks/useResources.ts:351`, `frontend-modern/src/hooks/useResources.ts:587` | Legacy conversion infers IDs/types with string parsing (`split('-')`, `split('/')`). | Heuristic | Fragile ID reconstruction in legacy adapters. |
| A2-RT12 | `frontend-modern/src/hooks/useResources.ts:245-249` | `useResourcesAsLegacy` prefers legacy arrays over unified resources when present. | Heuristic | Preserves compatibility but cements legacy-type assumptions in consumers. |

#### A.3 Frontend Legacy Dependencies

| Path ID | File | Legacy Data Source | Description |
| --- | --- | --- | --- |
| A3-FE01 | `frontend-modern/src/pages/Alerts.tsx:482` | `useWebSocket().state` | Alerts page is state-slice-driven, not unified-resource-driven. |
| A3-FE02 | `frontend-modern/src/pages/Alerts.tsx:727-733` | `state.nodes`, `state.vms`, `state.containers`, `state.storage` | Overrides rehydration guarded on legacy arrays being present. |
| A3-FE03 | `frontend-modern/src/pages/Alerts.tsx:737-937` | `state.dockerHosts`, `state.hosts`, `state.pbs`, `state.nodes`, `state.storage`, `state.vms`, `state.containers` | Override list construction reads legacy arrays directly and branches by platform-specific assumptions. |
| A3-FE04 | `frontend-modern/src/pages/Alerts.tsx:793-799` | Override key string (`docker:host/container`) | Docker override parsing uses legacy key-shape heuristics. |
| A3-FE05 | `frontend-modern/src/pages/Alerts.tsx:830` | Override key regex (`host:<id>/disk:<mountpoint>`) | Host-disk classification derives from string pattern. |
| A3-FE06 | `frontend-modern/src/pages/Alerts.tsx:917` | `guest.type` (`qemu` vs non-`qemu`) | VM/CT classification is platform-specific and binary. |
| A3-FE07 | `frontend-modern/src/pages/Alerts.tsx:1472-1474` | `state.vms` + `state.containers` | `allGuests` memo composed from legacy arrays only. |
| A3-FE08 | `frontend-modern/src/pages/Alerts.tsx:3073-3083` | `props.state.nodes/storage/dockerHosts/pbs/pmg/...` | Thresholds table receives legacy slices as primary props. |
| A3-FE09 | `frontend-modern/src/pages/Alerts.tsx:4672-4728` | `state.vms/containers/nodes/storage/dockerHosts/pbs/cephClusters` | History resource-type fallback uses name/id matching over legacy slices. |
| A3-FE10 | `frontend-modern/src/pages/Alerts.tsx:5766-5774` | Hardcoded legacy type labels | Alert type badge styling assumes fixed set (`VM`, `CT`, `Node`, `Storage`). |
| A3-FE11 | `frontend-modern/src/hooks/useResources.ts:238-249` | Legacy arrays preferred over unified | Compatibility hook intentionally keeps legacy-first behavior. |
| A3-FE12 | `frontend-modern/src/hooks/useResources.ts:266-275` | `state.vms` | VM adapter returns legacy array whenever present. |
| A3-FE13 | `frontend-modern/src/hooks/useResources.ts:328-336` | `state.containers` | Container adapter returns legacy array whenever present. |
| A3-FE14 | `frontend-modern/src/hooks/useResources.ts:493-501` | `state.nodes` | Node adapter returns legacy array whenever present. |
| A3-FE15 | `frontend-modern/src/hooks/useResources.ts:562-570` | `state.dockerHosts` | Docker-host adapter returns legacy array whenever present. |
| A3-FE16 | `frontend-modern/src/hooks/useResources.ts:731-737` | `state.pbs` | PBS adapter falls back to legacy slice. |
| A3-FE17 | `frontend-modern/src/hooks/useResources.ts:878-919` | `state.storage` | Storage adapter merges legacy slice with synthesized entries. |
| A3-FE18 | `frontend-modern/src/hooks/useResources.ts:1004-1010` | `state.pmg` | PMG adapter falls back to legacy slice. |

### Appendix B: Risk Register

| Risk ID | Surface | Description | Severity | Mapped Packet | Mitigation |
| --- | --- | --- | --- | --- | --- |
| R-001 | WebSocket alert transport (`internal/websocket/hub.go:940-957`, `internal/monitoring/monitor_alerts.go:47,80`) | Alert fired/resolved events are broadcast on global channel, not tenant channel. | HIGH | Packet 01 | Replace alert broadcasts with tenant-targeted path and add isolation tests. |
| R-002 | Alert-adjacent API push (`internal/api/alerts.go:628,710,768,812,855,910,976,1035`) | Manual alert actions broadcast full `rawData` globally via `BroadcastState`, creating cross-tenant leakage risk. | HIGH | Packet 01 | Route these broadcasts through tenant-aware state broadcast only. |
| R-003 | Re-evaluation type inference (`internal/alerts/alerts.go:1714-1889`) | Active-alert re-evaluation depends on ID parsing and string heuristics, risking wrong threshold policy resolution. | HIGH | Packet 02 / Packet 03 | Replace with canonical `resource_type` contract and unified evaluator input model. |
| R-004 | Legacy active-alert migration (`internal/alerts/alerts.go:9202-9227`) | Startup migration infers guest identities from naming conventions; brittle across ID format drift. | MEDIUM | Packet 06 | Introduce explicit ID normalization/migration contract + regression tests. |
| R-005 | Dormant heuristic helper (`internal/api/router.go:3858-3892`) | `deriveResourceTypeFromAlert` is heuristic (`/qemu/`, `/lxc/`) and test-covered only; easy to reintroduce accidentally in runtime paths. | LOW | Packet 02 | Keep deprecated/test-only or remove after canonical contract adoption. |
| R-006 | AI correlation typing (`internal/api/router.go:2426`) | Correlation detector records `ResourceType` from `alert.Type`, causing semantic drift between alert metric and resource taxonomy. | MEDIUM | Packet 08 | Map from canonical alert metadata fields (`resource_type`, `platform_type`, `source_type`). |
| R-007 | Frontend history/type lookup (`frontend-modern/src/pages/Alerts.tsx:4672-4728`) | Fallback type derivation via legacy arrays/name matching can misclassify or fail for unified-only resources and name collisions. | HIGH | Packet 05 | Make unified resource IDs/types authoritative in alerts UI and reduce name-based matching. |
| R-008 | Frontend overrides/type assumptions (`frontend-modern/src/pages/Alerts.tsx:727-937`) | Overrides pipeline depends on legacy arrays and key-parsing conventions (`docker:`, host disk regex, qemu/CT assumptions). | HIGH | Packet 05 / Packet 06 | Migrate override lookup to canonical resource IDs/types with compatibility shim. |
| R-009 | Legacy-first resource adapter (`frontend-modern/src/hooks/useResources.ts:245-249`) | `useResourcesAsLegacy` preserves legacy slices when present, prolonging non-unified behavior and masking migration gaps. | MEDIUM | Packet 05 | Invert priority to unified-first with explicit fallback gating and deprecation timeline. |
| R-010 | AI callback suppression bypass (`internal/alerts/alerts.go:6329-6338`, `internal/alerts/alerts.go:4298-4307`) | AI callbacks fire even when notifications are suppressed; without strict tenant propagation this can cause cross-org side effects. | MEDIUM | Packet 08 | Enforce tenant context propagation and add tenant-safety tests for AI callbacks/bridge paths. |
| R-011 | Test coverage gap: WS tenant isolation for alerts (`internal/websocket/hub_tenant_test.go:58-61`, `internal/websocket/hub_more2_test.go:36-55`) | Tenant tests cover state tenant broadcast but not tenant-isolated alert event delivery. | HIGH | Packet 01 / Packet 07 | Add explicit fired/resolved tenant-isolation websocket tests. |
| R-012 | Test coverage gap: monitor alert fan-out (`internal/monitoring/monitor_alert_handling_test.go:11-60`) | Current monitor alert handling tests are smoke-level and do not validate tenant partitioning or payload correctness. | MEDIUM | Packet 07 | Add contract-level tests for fired/resolved fan-out paths and callback side effects. |

### Appendix C: Canonical Alert Identity Contract

#### C.1 Alert Identity Fields (Locked)

| Field | Source | Format | Example |
| --- | --- | --- | --- |
| `id` | Generated | `{resourceID}-{metricType}` | `cluster/qemu/100-cpu` |
| `resource_id` | Alert creation | Platform-specific ID | `cluster/qemu/100`, `host:agent-1` |
| `resource_type` | `Metadata["resourceType"]` | Capitalized backend key | `VM`, `Container`, `Node` |
| `type` | Alert creation | Metric/event type | `cpu`, `memory`, `offline`, `backup` |
| `node` | Alert creation | Node name | `pve-1` |
| `instance` | Alert creation | Sub-resource ID | `subvol-100-disk-0` |

#### C.2 Canonical Resource Type Map

| Backend Value (Metadata) | Canonical Keys (threshold lookup) | Frontend Equivalent | Platform |
| --- | --- | --- | --- |
| `VM` | `guest` | `vm` | `proxmox-pve` |
| `Container` | `guest` | `container` | `proxmox-pve` |
| `Node` | `node` | `node` | `proxmox-pve` |
| `Host` | `host`, `node` | `host` | `host-agent` |
| `Host Disk` | `host-disk`, `host`, `storage` | `storage` | `host-agent` |
| `PBS` | `pbs`, `node` | `pbs` | `proxmox-pbs` |
| `Docker Container` | `docker`, `guest` | `docker-container` | `docker` |
| `DockerHost` | `dockerhost`, `docker`, `node` | `docker-host` | `docker` |
| `Docker Service` | `docker-service`, `docker`, `guest` | `docker-service` | `docker` |
| `Storage` | `storage` | `storage` | `proxmox-pve` |
| `PMG` | `pmg`, `node` | `pmg` | `proxmox-pmg` |
| `K8s` | `k8s`, `guest` | `pod` | `kubernetes` |

#### C.3 Legacy Compatibility

During migration, heuristic derivation via `deriveResourceTypeFromAlert` (`internal/api/router.go`) is retained for test reference only. Runtime paths must use `Metadata["resourceType"]` as the primary source. If metadata is missing, the alert is classified as the default type for its metric family.
