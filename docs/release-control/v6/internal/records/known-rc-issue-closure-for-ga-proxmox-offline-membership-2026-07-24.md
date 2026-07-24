# Known RC Issue Closure For GA Proxmox Offline Membership Record

- Date: `2026-07-24`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1433`
- Result: `fixed-main-proof`

## Context

The earlier v6 proof for `#1433` showed that a static offline Proxmox node row
could render, and whole-instance failures later gained last-known placeholder
handling. It did not cover the normal partial-poll transition: a healthy
cluster `/nodes` response could contain only the reachable members, after which
`State.UpdateNodesForInstance` treated the telemetry slice as complete
inventory and removed the powered-off member. The canonical registry then
correctly propagated that mistaken omission as a real resource removal to
overview counts, navigation, history, API, websocket, and mobile consumers.

Setup and refresh discovery had a matching lifecycle error: once any member API
validated, individually unreachable members from `/cluster/status` were
discarded rather than persisted as unavailable members. Configuration
normalization and overview grouping also treated `ClusterName` as globally
unique, allowing unrelated clusters with the same display name to collide.

## Disposition

Proxmox cluster membership and telemetry are now separate contracts.
`/nodes` remains a live metrics observation. A quorate, complete
`/cluster/status` response is the absence-authoritative membership source.
Members present in cluster status but absent from live metrics remain canonical
with their stable node identity, last-seen and linked-agent evidence, zeroed
live CPU/uptime, and explicit offline or stale connection state.

Failed, incomplete, non-quorate, or cluster-identity-mismatched membership reads
retain the last-known union, break any pending absence sequence, and never
advance removal. A member omitted by a
healthy authoritative membership read is retained on the first observation and
retired only after a second consecutive confirmed absence. The pending member
also remains in durable endpoint configuration, so a Pulse restart resets the
volatile confirmation window instead of converting uncertainty into deletion.
Confirmed removal deletes the endpoint and node once, allowing canonical
history and monitored-system/licensing projections to emit and count the real
removal without transient churn.

Discovery persists every member returned by cluster status and records
per-endpoint API reachability as separate evidence. It can add or enrich
members but cannot delete a previously saved member before monitoring's
confirmation rule. Duplicate cluster
configuration now requires overlapping endpoint identity rather than an equal
display name. Node, storage, overview grouping, search, and guest counts use
provider-scoped identity when cluster names repeat.

The Proxmox node table renders explicit `Offline` or `Stale` text and suppresses
live uptime, temperature, bars, and sparklines when provider telemetry is not
current. The row remains navigable on desktop and mobile.

## Proof

- focused monitoring reconciliation tests covering powered-off members,
  unavailable/incomplete/non-quorate reads, cluster identity mismatch,
  immediate membership additions, repeated confirmed removal, uncertainty
  resets, restart, linked-agent evidence, and same-name cluster identity
- configuration and API discovery tests covering distinct same-name clusters
  and unreachable member persistence
- model and unified-resource tests covering provider-scoped deduplication,
  stable offline resource identity, no premature `resource_removed` history,
  and monitored-system count removal only after canonical retirement
- Proxmox page-model and node-table tests covering provider-scoped grouping,
  search, guest counts, explicit offline/stale state, and stale-metric
  suppression
- `tests/integration/tests/65-offline-proxmox-node-visibility.spec.ts` on
  desktop Chromium and the mobile browser project
- issue-owned Go race suites, frontend unit/lint/type/build checks, and v6
  control-plane, status, registry, contract, completion, and readiness audits
- a repository-wide Go race run in which the issue-owned packages passed; the
  run remained red outside this change because the Docker-agent command test
  has an asynchronous cleanup race and the managed migration proof requires
  the absent sibling `pulse-pro/license-server`

## Outcome

The fix is on `main` for a future v6 release. It is not retroactively part of
`v6.1.1`, no v5 backport or publication date is claimed, and `#1433` remains
open for reporter confirmation after a release containing the change.
