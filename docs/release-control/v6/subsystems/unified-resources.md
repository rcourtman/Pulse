# Unified Resources Contract

## Contract Metadata

```json
{
  "subsystem_id": "unified-resources",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/subsystems/unified-resources.md",
  "status_file": "docs/release-control/v6/status.json",
  "registry_file": "docs/release-control/v6/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

Own canonical resource identity, type normalization, typed views, and
cross-source deduplication.

## Canonical Files

1. `internal/unifiedresources/types.go`
2. `internal/unifiedresources/views.go`
3. `internal/unifiedresources/read_state.go`
4. `internal/unifiedresources/adapters.go`
5. `internal/unifiedresources/monitor_adapter.go`
6. `internal/unifiedresources/canonical_identity.go`

## Shared Boundaries

1. `frontend-modern/src/components/Infrastructure/infrastructureSelectors.ts` shared with `performance-and-scalability`: the infrastructure selector pipeline is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
2. `frontend-modern/src/components/Infrastructure/resourceDetailMappers.ts` shared with `performance-and-scalability`: resource detail mappers are both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
3. `frontend-modern/src/components/Infrastructure/UnifiedResourceTable.tsx` shared with `performance-and-scalability`: the unified resource table is both a canonical unified-resource consumer surface and a fleet-scale performance hot-path boundary.
4. `internal/api/resources.go` shared with `api-contracts`: the unified resource endpoint is both a backend payload contract surface and a unified-resource runtime boundary.

## Extension Points

1. Add new resource types and identity fields in `internal/unifiedresources/types.go`
2. Add typed accessors and views in `internal/unifiedresources/views.go`
3. Add source ingestion/adaptation in the adapter layer only

## Forbidden Paths

1. New ad hoc resource-type aliases outside unified resource normalization
2. New duplicate ID normalization logic outside unified resources
3. Reintroducing legacy runtime resource contracts as live truth

## Completion Obligations

1. Update this contract when canonical resource identity or type rules change
2. Update contract and guardrail tests when a new resource type is added
3. Route runtime changes through the explicit unified-resource proof policies in `registry.json`; default fallback proof routing is not allowed
4. Tighten banned-path tests when a compatibility bridge is removed

## Current State

The unified resource core is strong and canonical, but monitoring and some
frontend/API consumers are still being tightened around it.

Canonical storage metadata now carries runtime `enabled` and `active` flags so
monitoring and API export paths can derive `models.Storage` from unified views
without depending on legacy snapshot ownership.

Canonical Proxmox node metadata now carries node-only boundary fields such as
guest URL, connection health, temperature details, and pending-update metadata
so monitoring can derive `models.Node` from unified views without depending on
legacy snapshot ownership.

Canonical host-agent metadata now carries host-only runtime fields such as CPU
count, load average, machine/report identity, command capability, exclude
patterns, and host I/O rates so monitoring can derive `models.Host` from
unified views without depending on legacy snapshot ownership.

Canonical Docker host metadata now carries Docker-host-only runtime fields such
as display-name identity, CPU/memory sizing, interval/load averages, raw
container membership, and host I/O rates so monitoring can derive
`models.DockerHost` from unified views without depending on legacy snapshot
ownership.

Canonical Proxmox guest metadata now carries workload boundary fields such as
guest OS identity, guest agent version, guest network interfaces, VM disk
status reason, and container OCI/Docker-detection metadata so monitoring can
derive `models.VM` and `models.Container` from unified views without depending
on legacy snapshot ownership.

Canonical PBS metadata now carries full instance boundary payload such as host
and guest URLs, full datastore details, and PBS job arrays so monitoring can
derive `models.PBSInstance` from unified views without depending on legacy
snapshot ownership.

Canonical physical-disk views now expose the full disk identity and SMART
metadata needed by monitoring refresh paths, so physical-disk temperature and
SMART merges can run from unified `ReadState` instead of from snapshot-owned
disk arrays.

Canonical identity now also treats Proxmox-backed infrastructure parents as
node-owned resources first: when an agent resource carries canonical Proxmox
node metadata, `canonicalIdentity.primaryId` must remain stable as
`node:<proxmox-source-id>` even if agent discovery metadata is also present,
so merged node-plus-agent views do not drift to transient agent identifiers.

Frontend/API consumers and backend support files now require explicit registry
path-policy coverage, so new unified-resource-owned runtime files must be added
to a concrete proof route instead of falling back to subsystem-default
verification.

The infrastructure table, selector, and detail-mapper frontend consumers are
now governed as explicit shared boundaries with the performance lane rather
than implicit downstream usage. That means future fleet-scale table changes
must preserve both canonical unified-resource semantics and the table
performance proof route.
