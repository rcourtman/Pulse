# Operational Trust: Availability Check Identity Correction

Date: 2026-07-23

## Decision

Every configured availability target is a source-owned canonical
`network-endpoint` resource. Correlation is additive: a uniquely matched VM,
host, workload, or service may also carry the check as an availability facet,
but that projection never replaces or hides the configured check row.

The check resource owns current probe state, incidents, alert lifecycle,
history, canonical evidence, and the outgoing `checks` relationship. A matched
resource receives only the availability facet and a cloned evidence envelope
bound to that resource. Projection does not merge the checked service address,
name, status, incidents, tags, metrics, or last-seen value into the matched
resource.

## Root Cause Corrected

The original registry path used the generic identity merge when correlation
found a resource. That merge mapped the availability source target directly to
the VM or host, copied unrelated service identity and incident state into it,
and returned before creating the source-owned endpoint. The frontend then
excluded `attached` availability resources, codifying the missing row and
incorrect count.

The corrected registry path creates or replaces the deterministic
source-owned endpoint first, projects only its facet onto the match, and stores
the relationship on the check. Rehydration seeds availability source mappings
only from availability-owned endpoints, and manual resource links cannot fold
those endpoints into adjacent resources.

## Runtime Contract

- Two services correlated to one monitored VM produce two availability rows
  and two facets on that VM.
- Standalone, ambiguous, unresolved, disabled, and attached targets all retain
  one row per configured target.
- REST and websocket payloads preserve both source-owned rows and plural host
  projections.
- Atomic refresh, reload, and restart preserve the check ID. Editing replaces
  stale endpoint state and moves the projection; deletion removes both the row
  and only that target's projection.
- Availability incidents and alerts remain single-owned by the check. The
  matched resource does not receive a duplicate incident.
- Check creation, configuration changes, health transitions, and deletion are
  recorded against the check ID. Observation timestamps and latency changes
  alone do not create history churn.
- Resource registries and browser caches remain tenant-scoped. Identical target
  IDs in different tenants resolve only against resources in their own tenant.

## Proof

Backend regression coverage:

- `internal/unifiedresources/availability_link_test.go`
- `internal/unifiedresources/monitor_adapter_read_state_test.go`
- `internal/unifiedresources/change_emission_test.go`
- `internal/alerts/unified_incidents_test.go`
- `internal/monitoring/canonical_guardrails_test.go`

Frontend regression coverage:

- `frontend-modern/src/features/standalone/__tests__/standalonePageModel.test.ts`
- `frontend-modern/src/features/standalone/__tests__/StandalonePageSurface.test.tsx`
- `frontend-modern/src/hooks/__tests__/useUnifiedResources.test.ts`

The governed browser scenario covers two attached services on one monitored
host plus standalone endpoints and verifies that the Availability checks tab
shows the configured total while the host retains both facet details.

## User Evidence

- [#1568: Machines - Availability Checks does not show all checks](https://github.com/rcourtman/Pulse/issues/1568)
- [#1460: Simple ping-based monitoring](https://github.com/rcourtman/Pulse/issues/1460)
- [#1565: UDP/service availability without an agent](https://github.com/rcourtman/Pulse/issues/1565)

## Governance

- Owning lane: L13
- Dependent contracts: monitoring, unified resources, alerts, API contracts,
  frontend primitives, and performance/scalability
