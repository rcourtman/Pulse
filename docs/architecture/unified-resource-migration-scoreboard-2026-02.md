# Unified Resource Migration Scoreboard (Code-Derived Snapshot)

Status: Active
Date: 2026-02-08
Method: Code-first inventory (frontend + backend). Documentation treated as non-authoritative.

## Parallel Worker Guardrails

Active worker-owned lanes (do not overlap in new packets right now):
1. Storage GA hardening remains active: `docs/architecture/storage-page-ga-hardening-progress-2026-02.md:6`, with remaining packets 06-10 at `docs/architecture/storage-page-ga-hardening-progress-2026-02.md:30`.
2. Settings stabilization remains active: `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md:6`, with remaining packets 02-07 at `docs/architecture/settings-navigation-performance-stabilization-progress-2026-02.md:26`.

This scoreboard marks those surfaces as `IN_FLIGHT (worker-owned)` and avoids assigning overlapping implementation scope.

## Migration States

- `UNIFIED`: primary reads are from unified resources (`/api/v2/resources`) with no critical legacy dependency.
- `BRIDGED`: unified foundation exists, but surface still depends on compatibility adapters/legacy-shaped state.
- `IN_FLIGHT`: active worker lane, avoid parallel edits in same scope.
- `LEGACY_HEAVY`: surface primarily consumes legacy arrays/state and should be prioritized for migration.

## Scoreboard

| Surface | State | Current Data Path | Key Evidence | What Is Missing |
|---|---|---|---|---|
| Backend resource core | UNIFIED | Registry + v2 handlers + link/unlink/report-merge routes | `internal/api/router_routes_monitoring.go:39`, `internal/api/resources_v2.go:52`, `internal/api/resources_v2.go:223` | Stable; continue treating as canonical source of truth. |
| Legacy API bridge | BRIDGED | `/api/resources` served by `LegacyAdapter` projected from unified snapshot | `internal/api/resource_handlers.go:23`, `internal/api/resource_handlers.go:35`, `internal/unifiedresources/legacy_adapter.go:15` | Retire endpoint consumers once frontend surfaces are migrated. |
| Infrastructure page | UNIFIED | Frontend reads unified resources directly | `frontend-modern/src/pages/Infrastructure.tsx:33` | Mostly done; maintain parity tests only. |
| Workloads dashboard | UNIFIED | Workloads fetched from v2 resources endpoint via dedicated hook | `frontend-modern/src/hooks/useV2Workloads.ts:5`, `frontend-modern/src/components/Dashboard/Dashboard.tsx:412` | Mostly done; maintain parity/contract tests only. |
| Storage page | IN_FLIGHT (worker-owned) | Dual routing + V2/legacy coexistence | `frontend-modern/src/App.tsx:979`, `frontend-modern/src/App.tsx:1012`, `frontend-modern/src/routing/storageBackupsMode.ts:43` | Complete remaining storage GA packets before any additional migration edits here. |
| Backups page | IN_FLIGHT (worker-owned) | V2 exists; legacy unified bridge still used in parallel (`useResourcesAsLegacy`) | `frontend-modern/src/components/Backups/UnifiedBackups.tsx:66`, `frontend-modern/src/hooks/useResources.ts:238` | Finish active storage lane before parallel work. |
| Settings surfaces | IN_FLIGHT (worker-owned) | Heavy direct websocket legacy state usage, plus `/api/resources` in org sharing | `frontend-modern/src/components/Settings/useInfrastructureSettingsState.ts:133`, `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx:133` | Complete settings stabilization lane first; resource-model cleanup is a follow-on lane. |
| Alerts page | LEGACY_HEAVY | Direct reads from `state.nodes/vms/containers/storage/hosts/dockerHosts` | `frontend-modern/src/pages/Alerts.tsx:765`, `frontend-modern/src/pages/Alerts.tsx:919`, `frontend-modern/src/pages/Alerts.tsx:3111` | Move alert resource resolution to unified selectors; remove legacy-array coupling. |
| AI chat UI summary | LEGACY_HEAVY | Cluster/context synthesis from legacy websocket arrays | `frontend-modern/src/components/AI/Chat/index.tsx:334`, `frontend-modern/src/components/AI/Chat/index.tsx:338` | Build unified-resource selector for AI UI hints and mentions. |
| AI backend context | BRIDGED | Unified adapter injected, but shape exposed as legacy resource contract | `internal/api/router.go:334`, `internal/ai/resource_context.go:11`, `internal/ai/resource_context.go:39` | Introduce true v2-first AI resource provider contract and migrate call sites. |
| WebSocket state contract | BRIDGED | Broadcast includes both legacy arrays and unified `resources` | `frontend-modern/src/stores/websocket.ts:478`, `frontend-modern/src/hooks/useResources.ts:238` | Plan staged retirement of legacy payload fields after surface migration. |

## Where Pulse Is Furthest Behind (Excluding Active Worker Lanes)

1. Alerts surface migration to unified selectors (largest remaining frontend legacy coupling).
2. AI chat frontend context derivation from unified resources instead of legacy arrays.
3. Organization sharing resource picker dependency on legacy `/api/resources` endpoint.
4. Final retirement plan for websocket legacy payload fields after above surfaces are migrated.

## Recommended Next Worker Plan (Non-Overlapping)

Use the next free worker on `Alerts + AI + Org Sharing` only, while storage/settings workers continue.

Packet sequence:
1. Packet 00 - Contract freeze and migration inventory for alert/ai/org-sharing surfaces.
2. Packet 01 - Add unified selector layer for alerts resource lookup (no behavior change).
3. Packet 02 - Migrate alerts page consumers to selector layer; keep compatibility fallback for one packet.
4. Packet 03 - Migrate AI chat UI resource summary/mention source to unified selector layer.
5. Packet 04 - Switch org sharing picker from `/api/resources` to `/api/v2/resources` query path.
6. Packet 05 - Remove no-longer-used legacy fallback paths and add regression guardrail tests.

Scope guardrail for this plan:
- Do not edit storage/backups/settings files currently under active worker ownership.
- Allowed scopes: `frontend-modern/src/pages/Alerts.tsx`, `frontend-modern/src/components/AI/Chat/index.tsx`, `frontend-modern/src/components/Settings/OrganizationSharingPanel.tsx`, shared selector/hook files, and associated tests.

## Exit Criteria for Declaring Migration Complete

1. No critical product surfaces depend on direct legacy websocket arrays for primary rendering decisions.
2. No frontend product surface calls `/api/resources` for primary resource browsing.
3. Websocket legacy payload fields are either removed or formally deprecated behind explicit compatibility window.
4. Unified-resource contract tests cover alerts, AI context, and sharing flows.

