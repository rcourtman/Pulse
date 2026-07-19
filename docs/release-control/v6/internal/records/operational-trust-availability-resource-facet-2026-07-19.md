# Operational Trust: Availability as a Resource Facet

Date: 2026-07-19

## Decision

Availability is evidence about a canonical resource, not a parallel inventory
taxonomy.

Pulse attaches a saved check in this order:

1. an explicit canonical resource link, which is authoritative and fails closed
2. one exact normalized IP address match
3. one exact normalized hostname match

Zero automatic matches remain standalone. Multiple matches are ambiguous.
Invalid explicit links are unresolved and do not fall back to address matching.
Availability-owned endpoints never become attachment candidates.

Every attached check is retained in `availabilityChecks`; the singular
`availability` field remains an additive compatibility summary. Each attached
target emits a `checks` relationship and a canonical evidence envelope bound to
the owning resource. A second check attached to the same resource remains on
that resource and does not reappear as duplicate standalone inventory.

## Runtime Result

- `internal/monitoring/availability_poller.go` authors freshness-bounded
  operational-trust evidence. A never-observed target is partial/unknown.
- `internal/unifiedresources/availability.go` owns plural facet normalization,
  compatibility-summary selection, and exact target lookup.
- `internal/unifiedresources/registry.go` owns explicit, unique address, and
  unique hostname correlation plus typed attached/standalone/ambiguous/
  unresolved outcomes.
- Attached evidence is rebound to the canonical subject and records the exact
  identity-correlation rule.
- `internal/alerts/unified_incidents.go` routes availability incidents on any
  resource type through the canonical alert lifecycle and selects evidence by
  the incident's exact target id.
- REST, websocket, workload, Docker, and Standalone projections preserve the
  additive plural/correlation/evidence fields without a per-row fetch.
- The owning platform row keeps one compact summary. Workload and Docker detail
  surfaces render protocol, complete target, latest result, latency, freshness,
  and last observation for every attached check.
- Expired successful evidence renders `Stale`, never `Up` or `Responding
  normally`; an unobserved check renders `Not checked`.
- Attached resources are excluded from the standalone Availability checks
  inventory.

## User Lens

User job: “Tell me whether this machine or service is reachable, on the resource
I already know, and show me when that answer stopped being trustworthy.”

Live exercise:

- Opened Docker Overview in the authenticated product.
- The `Tower` host row showed one compact `TCP` availability facet.
- Expanded the deepest host detail.
- The detail showed `Availability`, target `192.168.0.8:8007`, method
  `TCP 8007`, result `Up`, latency `1ms`, checked age, and `fresh`.
- Opened Machines > Availability checks.
- The attached `Tower` host was absent; only the two genuinely standalone local
  targets remained.

Distance to answer is one platform navigation plus one row expansion. Every
default-row element is actionable: the compact facet signals whether to open
detail; protocol, target, result, freshness, and observation time explain what
was tested and whether it can still be trusted. Provider-forensic correlation
reason and evidence stay in detail rather than widening the row.

Keep / demote / cut:

- Keep one compact availability summary on the owning row.
- Keep complete current-state and freshness detail in the expansion.
- Demote correlation/evidence forensics to detail and API payloads.
- Cut attached targets from standalone primary inventory.
- Cut green reassurance for stale successful observations.
- Cut guessed correlation and endpoint-only lifecycle forks.

## User Evidence

- [#1460: Simple ping-based monitoring](https://github.com/rcourtman/Pulse/issues/1460)
  asks Pulse to monitor devices that cannot run an agent or SSH.
- [#1565: UDP/service availability without an agent](https://github.com/rcourtman/Pulse/issues/1565)
  describes the burden of maintaining a separate `nmap` plus email script.
- [#1568: not all availability checks are shown](https://github.com/rcourtman/Pulse/issues/1568)
  demonstrates that missing or duplicate check inventory is a trust defect.
- [#1582: failure threshold timing mismatch](https://github.com/rcourtman/Pulse/issues/1582)
  demonstrates that observation timing and freshness must be explicit.
- [Discussion #1508: crashed VM remains green without an agent](https://github.com/rcourtman/Pulse/discussions/1508)
  asks for reachability loss to affect the existing VM rather than require a
  separate monitoring tool.
- [#1519: clock drift creates stale/offline loops](https://github.com/rcourtman/Pulse/issues/1519)
  reinforces the separation between fresh receipt evidence and stale state.

No public report explicitly requested merging an availability target into a
VM/container row. The canonical decision is therefore an inference from the
resource-coherence and missing-check evidence above, not a claimed direct user
quote.

## Comparative Evidence

- [Checkmk host/service model](https://docs.checkmk.com/latest/en/monitoring_basics.html)
  treats checks as services of a host and distinguishes unknown, pending, and
  stale from down.
- [Zabbix host availability](https://www.zabbix.com/documentation/current/en/manual/web_interface/frontend_sections/data_collection/hosts)
  attaches availability to host interfaces and preserves available,
  unavailable, mixed, and unknown states.
- [Uptime Kuma](https://github.com/louislam/uptime-kuma) centers independent
  monitor objects and keeps pending/maintenance distinct from down.
- [Grafana Synthetic Monitoring checks](https://grafana.com/docs/grafana-cloud/testing/synthetic-monitoring/create-checks/checks/)
  centers standalone checks and uses labels for correlation.
- [Grafana missing-data behavior](https://grafana.com/docs/grafana/latest/alerting/guides/missing-data/)
  preserves No Data, Error, and MissingSeries rather than silently resolving.

Pulse follows the attached host/resource facet pattern because it already owns a
canonical cross-platform resource model. It retains standalone endpoints only
where no canonical owner exists, while preserving the shared industry rule that
missing, stale, pending, and unknown evidence are not healthy.

## Proof

Focused backend proof covers:

- explicit link precedence and fail-closed invalid links
- exact IP and hostname attachment
- ambiguous candidate rejection
- plural checks on one canonical resource
- `checks` relationships
- evidence validation, canonical rebinding, freshness, and pre-first-probe
  partial/unknown state
- attached Docker service failure through the canonical operational lifecycle
- mock graph attachment without standalone duplication

Focused frontend proof covers:

- row and detail presentation
- plural attached cards
- stale-success and unobserved truthfulness
- standalone duplicate exclusion
- freshness-aware status/filter behavior
- shared primitive and no-per-row-fetch guardrails

Deterministic browser proof:

- `tests/integration/tests/92-operational-trust-availability-facet.spec.ts`

Live browser proof was performed against the current authenticated development
runtime after the implementation build, including the Docker row, expanded host
detail, and Standalone Availability checks inventory described above.

## Governance

- Candidate lane: `availability-as-resource-facet`
- Owning lanes: L8 and L13
- Owning contracts: monitoring, unified resources, alerts, API contracts,
  frontend primitives, performance and scalability, Patrol intelligence
- This record is the durable evidence for resolving and removing the candidate
  and its completed availability coverage gap.
