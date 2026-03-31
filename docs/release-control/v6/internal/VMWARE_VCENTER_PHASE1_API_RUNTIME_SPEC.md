# VMware vCenter Phase-1 API Runtime Spec

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `docs/release-control/v6/internal/VMWARE_VSPHERE_PHASE1_EXECUTION_PLAN.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_ONBOARDING_SPEC.md`
- `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`

## Intent

This document locks the canonical backend contract for VMware phase 1 before
runtime work starts.

It answers the backend questions that should not be guessed during
implementation:

1. what the public Pulse API surface is
2. how the two official VMware API families fit under one connection
3. who owns poll health and saved-connection test state
4. what error classifications the shared settings surface can rely on
5. what public API surface must explicitly not exist in phase 1

## Governing Rule

If `vmware-vsphere` implementation starts, Pulse phase 1 should expose one
saved VMware connection model and one public settings API boundary, even if the
backend needs more than one authenticated VMware client internally.

That means:

1. the operator configures one `vCenter` connection
2. the public Pulse API surface remains `/api/vmware/connections*`
3. the backend may use both the vSphere Automation API and the VI JSON API
   under that one connection
4. the operator must not manage those API families separately
5. phase-1 health is only green when the declared phase-1 floor is actually
   reachable through the chosen runtime path

## Public API Boundary

The canonical public backend route family for phase 1 is:

1. `GET /api/vmware/connections`
2. `POST /api/vmware/connections`
3. `PUT /api/vmware/connections/{id}`
4. `DELETE /api/vmware/connections/{id}`
5. `POST /api/vmware/connections/test`
6. `POST /api/vmware/connections/{id}/test`

That route family is:

1. admin-only
2. platform-connections-only
3. the only public setup and health API for VMware phase 1

Phase 1 must not add public operator routes such as:

1. `/api/vmware/hosts`
2. `/api/vmware/vms`
3. `/api/vmware/datastores`
4. `/api/vmware/events`
5. `/api/vmware/tasks`
6. `/api/vmware/alarms`
7. `/api/vmware/...` control routes for VM, host, datastore, or cluster
   actions

Shared product surfaces should consume VMware through canonical resources,
shared alerts, shared monitoring history, and shared AI tools instead of
growing a provider-local public API family.

## Session And Authentication Contract

The official APIs clearly show two different read paths:

1. vSphere Automation API session bootstrap through `POST /api/session`
2. VI JSON session bootstrap through `SessionManager.Login`

Phase-1 contract:

1. one saved VMware connection may own more than one authenticated upstream
   client under the backend runtime
2. Pulse must not make the support floor depend on unproven cross-family
   session reuse
3. until live validation proves otherwise, the safe contract is:
   - one connection record
   - one provider instance
   - distinct authenticated clients for Automation API and VI JSON API when
     the declared floor needs both

This is a design choice informed by the official docs, not a claim that shared
session reuse is impossible. Shared session reuse may be adopted later if live
proof shows it is reliable across the supported floor.

## Saved-Connection Test Contract

Draft and saved-connection tests are support-floor gates, not simple TCP
checks.

`POST /api/vmware/connections/test` and
`POST /api/vmware/connections/{id}/test` should therefore validate:

1. endpoint reachability
2. TLS behavior under the selected trust mode
3. Automation API session bootstrap
4. VI JSON login
5. enough read access to prove the declared phase-1 floor is realistic

Phase-1 rule:

1. a connection test should not report healthy if only one required VMware API
   family succeeds
2. partial upstream success should classify as a floor failure, not as a green
   setup result
3. saved-connection tests must reuse stored secrets server-side and may accept
   an edit overlay payload without requiring masked-secret re-entry
4. exhausting the implemented VI JSON release probe floor should classify as
   `unsupported_version`, not as a generic endpoint error

## Runtime Health And Poll Summary Contract

The list response on `GET /api/vmware/connections` should remain the canonical
operator-facing runtime summary for those configured connections.

That summary should include:

1. redacted config
2. poll health:
   - `intervalSeconds`
   - last success
   - last failure
   - consecutive failures
   - last error classification
3. observed contribution summary aligned to the declared support floor:
   - top-level host or monitored-system identity
   - `agent` count
   - `vm` count
   - `storage` count
   - last collected timestamp

Monitoring owns that summary at runtime. Saved-connection test paths must
update the same canonical health owner so row-level re-tests in settings do not
drift from the next list read.

## Provider Ownership Contract

Phase-1 VMware runtime should follow one-provider-per-saved-connection
ownership.

That means:

1. each saved VMware connection owns one provider instance
2. that provider instance may own both the Automation API client and the VI
   JSON client
3. config changes that affect connectivity or auth should replace the live
   provider instance instead of leaving stale clients in memory
4. poll cadence, next poll, last success, and last error belong to that one
   provider owner

The backend may optimize internally later, but the contract boundary stays one
connection and one operator-facing health owner.

## Failure Classification Floor

Where the backend can distinguish the failure mode honestly, the shared VMware
settings surface should get stable classifications at least for:

1. `endpoint_unreachable`
2. `tls_validation_failed`
3. `auth_invalid`
4. `permission_denied`
5. `unsupported_version_floor`
6. `phase1_floor_unmet`
7. `unknown`

If the upstream API does not provide enough detail, the backend should fail
closed into `unknown` or `phase1_floor_unmet` rather than guessing at a more
specific class.

## Negative Space

This backend contract also owns what phase 1 must not do:

1. no public VMware inventory-read API family parallel to canonical resource
   APIs
2. no public VMware alarm/event/task browser API family
3. no public VMware control API family
4. no backend shortcut that bypasses shared AI/runtime tools for VMware read
   or control

That means Assistant read remains on shared `pulse_read` / `pulse_query`, and
Assistant control remains out of scope.

## Current Blocker

As of 2026-03-30, there is still no recorded live VMware capability in
`/Volumes/Development/pulse/LOCAL_CAPABILITIES.md`.

So this contract is ready for implementation planning, but these points still
need live proof before VMware can be called supported:

1. exact dual-client or shared-session behavior across the supported version
   floor
2. exact minimum read set that is strong enough for a green connection result
3. exact error mapping quality on real auth, TLS, and permission failures
4. exact provider-rebind behavior after connection edits

## Primary Source Basis

1. Automation API session bootstrap:
   [CIS Session create](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/session/post/)
2. VI JSON login:
   [Session Manager Login](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/SessionManager/moId/Login/post/)
3. event retrieval:
   [Event Manager Query Events](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/EventManager/moId/QueryEvents/post/)
4. host-plus-child performance:
   [Performance Manager Query Perf Composite](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/PerformanceManager/moId/QueryPerfComposite/post/)
5. recent-task context:
   [Virtual Machine Get Recent Task](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/VirtualMachine/moId/recentTask/get/)
