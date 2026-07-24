# Known RC Issue Closure For GA Proxmox Node Display Name Record

- Date: `2026-07-24`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1517`
- Result: `fixed-main-proof`

## Context

A single configured Proxmox VE API endpoint discovers every member of its
cluster. Pulse previously presented those members by combining the connection
label and native node name, producing rows such as `pve1 (pve2)` and
`pve1 (pve3)`. The display string was assembled downstream and there was no
durable per-member configuration boundary where an operator could replace it.

A display-only field could not safely be attached to the endpoint name alone.
Native names can change, equal node names can exist in separate clusters,
members can disappear and return, and API-backed nodes can merge with Pulse
agents. Reusing presentation as identity would risk moving metrics, alerts,
credentials, actions, or connection authority to the wrong machine.

## Disposition

Each PVE connection now owns a retained cluster-node identity ledger. Migration
assigns deterministic identities compatible with historical node state IDs;
once assigned, an identity is never regenerated. Membership correlation uses
the stored identity first, the Proxmox numeric node ID second, then
unambiguous native-name or address evidence. Native rename, re-IP, reload,
restart, temporary absence, confirmed removal, and later reappearance preserve
the identity and optional display override. Prior native names remain aliases.
Ambiguous name or address evidence fails closed.

Settings writes an optional, trimmed display name against the immutable
identity. Unicode is supported up to 128 code points, control characters and
invalid UTF-8 are rejected, duplicate display values are allowed, and clearing
the value restores the current native Proxmox name. The override is
connection-scoped, deep-copied across tenant config views, and preserved by
config import/export.

Monitoring and unified resources project the preferred name through Overview,
Infrastructure Settings, Storage, Backups, alerts and resolved history,
search/filter, REST/websocket payloads, and responsive/mobile resource rows.
Current and prior native names plus numeric and immutable IDs remain available
for diagnostics and search. Display values never participate in configuration
matching, provider scoping, polling, credentials, TLS fingerprints, URLs,
metrics, parent/guest identity, discovery targets, actions, restore matching,
or alert lifecycle keys.

## Proof

- configuration migration, persistence, import/export, same-name cluster,
  Unicode/case collision, clear, stale-member, rename/re-IP/reappearance,
  deep-copy tenant isolation, and immutable identity tests
- API validation, response diagnostics, full update/reload authority, manual
  discovery merge, ambiguity fail-closed, connection grouping, contract, and
  concurrent immutable-copy tests
- monitoring, model, unified-resource, agent-link, alert/history,
  storage/backup, search/filter, settings payload, and responsive presentation
  tests
- issue-owned Go race suites; frontend unit, type, lint, and production build
  checks; desktop Chromium and mobile browser coverage
- v6 control-plane, status, registry, contract, completion, and readiness
  audits

## Outcome

The change is on `main` for a future v6 release. It is not retroactively part
of `v6.1.1`, no v5 backport or publication date is claimed, and `#1517`
remains open for reporter confirmation after a release containing the change.

The deliberate limitation is legacy identity with no recorded numeric Proxmox
node ID whose native name and every known address all change before a later
observation. Pulse treats that observation as a new member instead of guessing
and risking transfer of an override to the wrong node.
