# VMware vCenter Phase-1 Onboarding Spec

Last updated: 2026-03-30
Status: PLANNED
Governance surfaces:
- `status.json.candidate_lanes.platform-admission-execution`
- `docs/release-control/v6/internal/VMWARE_VSPHERE_PHASE1_EXECUTION_PLAN.md`

## Intent

This document defines the canonical setup and proof shape for slice 1 of
`vmware-vsphere` support.

It exists to keep VMware onboarding on the shared Pulse platform-connections
path instead of becoming a provider-local island.

## Current Shared Pattern In Pulse

The shared setup model already exists:

1. `PlatformConnectionsWorkspace.tsx` is the canonical API-backed platform
   shell.
2. `platformConnectionsModel.ts` owns the platform tab model and route shape.
3. provider-specific panels and state owners sit under that shell instead of
   creating standalone setup products.
4. the current saved-connection API pattern is:
   - `GET /api/<platform>/connections`
   - `POST /api/<platform>/connections`
   - `PUT /api/<platform>/connections/{id}`
   - `DELETE /api/<platform>/connections/{id}`
   - `POST /api/<platform>/connections/test`
   - `POST /api/<platform>/connections/{id}/test`
5. the list response is not CRUD-only. It carries redacted config plus poll
   health and observed contribution summary so the settings surface can show
   live platform status and handoffs.

VMware phase 1 should conform to that exact shape unless the shared
platform-connections contract changes for all API-backed platforms in the same
slice.

## Canonical VMware Slice-1 Setup Shape

VMware phase 1 should onboard only through the shared `Platform connections`
workspace.

That means:

1. add one VMware tab under the shared platform-connections shell
2. keep VMware as a provider workspace under that shell, not a separate
   settings route family
3. preserve the shared operator flow:
   - list saved connections
   - create or edit one saved connection
   - test a draft connection before save
   - re-test a saved connection without forcing secret re-entry
   - render last-success, last-error, poll cadence, and observed contribution
     summary
4. keep direct `ESXi` out of the phase-1 setup model entirely
5. keep unified-agent install out of the bootstrap requirement for VMware

## Expected Backend Route Contract

The expected backend route family for slice 1 is:

1. `GET /api/vmware/connections`
2. `POST /api/vmware/connections`
3. `PUT /api/vmware/connections/{id}`
4. `DELETE /api/vmware/connections/{id}`
5. `POST /api/vmware/connections/test`
6. `POST /api/vmware/connections/{id}/test`

Those routes should follow the same shared semantics already used by TrueNAS:

1. admin-only settings scopes, not a public operator shortcut
2. masked-secret preservation on saved updates
3. saved-connection test path reusing stored secrets server-side
4. optional edited-form overlay payload on saved-connection test
5. list responses reloading refreshed last-success or last-error state after
   a saved-connection test

## Expected List Response Shape

The slice-1 list response should include:

1. redacted connection config
2. poll health:
   - `intervalSeconds`
   - last success/failure
   - consecutive failures
   - last error classification
3. observed contribution summary tied to the canonical phase-1 floor:
   - top-level host or monitored-system identity
   - `agent` count
   - `vm` count
   - `storage` count
   - last collected timestamp

Datacenter, cluster, folder, and resource-pool counts may be useful for debug
or topology context, but they should remain secondary metadata. They should not
redefine the shared support floor around provider-local top-level types.

## Expected Connection Input Floor

The slice-1 connection draft should be designed around what the official
VMware APIs clearly support today and what the shared Pulse settings model
already expects.

Connection fields that fit that floor cleanly:

1. `name`
2. `host`
3. optional `port`
4. `username`
5. `password`
6. `enabled`
7. `pollIntervalSeconds`
8. TLS behavior:
   - `useHttps`
   - `insecureSkipVerify`
   - optional certificate/thumbprint pinning field if the shared contract keeps
     supporting it

Why this is the correct phase-1 floor:

1. the vSphere Automation API documents session creation at `POST /api/session`
   and states that subsequent REST calls use the `vmware-api-session-id`
   header; that operation uses `basic_auth`
2. the Virtual Infrastructure JSON API documents `SessionManager.Login` with
   explicit `userName` and `password` request fields

That is enough evidence for a username/password baseline.
It is not enough evidence to promise broader token or federated auth support as
part of phase 1.

## Explicit Unknowns For Slice 1

These points should stay explicit and validated on a real environment instead
of being hard-coded into a support claim up front:

1. whether the phase-1 connection contract should support only username and
   password or also a second VMware-native auth form
2. whether certificate/thumbprint pinning should be required, optional, or
   deferred behind the shared TLS contract
3. whether one session bootstrap can be reused cleanly across the vSphere
   Automation API and the Virtual Infrastructure JSON API in Pulse’s poller
   model, or whether the two API families need distinct authenticated clients
4. the exact minimum privilege set required for inventory, datastore, alarm,
   event, snapshot, and performance reads

## Phase-1 Proof Prerequisites

Before slice 1 can be called proven, all of these must exist:

1. one real `vCenter` environment
2. non-secret local capability metadata for that environment in
   `LOCAL_CAPABILITIES.md`
3. one live validation pass for:
   - connection create
   - draft test
   - saved-connection test
   - connection list health summary
   - supported version floor
   - minimum privilege bundle

## Current Proof State

As of 2026-03-30, this workspace does not have a recorded VMware capability in
`/Volumes/Development/pulse/LOCAL_CAPABILITIES.md`.

That means:

1. the architecture recommendation is ready
2. the setup contract can be planned now
3. implementation of the shared onboarding slice may start next
4. support-floor proof is still blocked until a real `vCenter` capability is
   available and recorded

This is the remaining discovery gap that matters most.

## Start Decision

Pulse is ready to implement the shared VMware onboarding slice.

Pulse is not yet ready to declare VMware support proven, because the live proof
environment and resulting privilege/version validation are not currently
available in this workspace.

## Primary Source Basis

Official VMware/Broadcom sources that justify this onboarding floor:

1. session bootstrap for vSphere Automation API:
   [CIS Session create](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/session/post/)
2. VM inventory list:
   [Vcenter VM list](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/vm/get/)
3. datastore inventory list:
   [Vcenter Datastore list](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/api/vcenter/datastore/get/)
4. host API family:
   [Vcenter Host APIs](https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/vcenter-host/)
5. VI JSON API login:
   [Session Manager Login](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/SessionManager/moId/Login/post/)
6. VI JSON API events:
   [Event Manager QueryEvents](https://developer.broadcom.com/xapis/virtual-infrastructure-json-api/latest/sdk/vim25/release/EventManager/moId/QueryEvents/post/)
