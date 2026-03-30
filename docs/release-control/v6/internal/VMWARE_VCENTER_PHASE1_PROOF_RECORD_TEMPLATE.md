# VMware vCenter Phase-1 Proof Record Template

Use this template when the first real VMware `vCenter` environment is available
and `VC-0` through `VC-7` from
`docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md` are
being exercised.

Save successful proof records under:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-YYYY-MM-DD.md`

Keep the blocked state in:

`docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-blocked-2026-03-30.md`

until the first successful real-environment pass is complete.

## Required Capability Entry Shape

Before writing the proof record, add a non-secret entry to
`/Volumes/Development/pulse/LOCAL_CAPABILITIES.md` using this shape:

```md
### VMware vCenter (<environment-alias>)
- Purpose: live VMware vSphere phase-1 proof environment for Pulse platform-admission validation.
- Access:
  - endpoint: `https://<vcenter-host-or-fqdn>`
  - operator path: `<vpn / tailscale / jump-host / local network note>`
- Verification:
  - `curl -kI https://<vcenter-host-or-fqdn>`
  - `<any additional non-secret reachability command>`
- Expected floor:
  - at least one ESXi host
  - at least one VM
  - at least one datastore
  - recent event or task history visible through `vCenter`
  - ideally one VM snapshot for snapshot-tree visibility proof
- Secret location alias:
  - `<alias only, for example secrets/vmware/<environment-alias>.md>`
- Safe-use notes:
  - `<read-first / do not mutate production workloads / maintenance window / tenancy note>`
```

Do not record usernames, passwords, tokens, certificate contents, private keys,
or raw inventory identifiers that should remain private.

## Successful Proof Record Template

---

# VMware vCenter Phase-1 Proof Record

- Date: `YYYY-MM-DD`
- Candidate lane: `platform-admission-execution`
- Platform: `vmware-vsphere`
- Result: `pass` or `fail`

## Environment

- Environment alias: `...`
- `LOCAL_CAPABILITIES.md` entry heading: `...`
- `vCenter` URL or host label: `...`
- `vCenter` version/build: `...`
- Operator: `...`
- Proof scope:
  - `vCenter` only
  - API-first
  - read-first

## Privilege Bundle

- Service-account role name or bundle label: `...`
- Source of privilege bundle definition: `...`
- Notes on any privilege uncertainty that remains: `...`

## Automated Proof Baseline

- `/api/vmware/connections*` contract tests: `pass` or `fail`
- VMware projection/runtime tests: `pass` or `fail`
- Shared alerts/history tests: `pass` or `fail`
- Assistant-read classification tests: `pass` or `fail`
- Notes:
  - `...`

## Capability Registration (`VC-0`)

Steps run:

1. `...`
2. `...`

Observed:

- capability entry present: `yes` or `no`
- non-secret environment label captured: `yes` or `no`
- safe verification command works: `yes` or `no`

## Draft Connection Test (`VC-1`)

Steps run:

1. `...`
2. `...`

Observed:

- draft test result: `...`
- failure classification exercise performed: `yes` or `no`
- green result required both API families: `yes` or `no`
- notes: `...`

## Saved Connection Retest (`VC-2`)

Steps run:

1. `...`
2. `...`

Observed:

- masked-secret preservation: `pass` or `fail`
- saved-connection retest without secret re-entry: `pass` or `fail`
- connection-list health refreshed: `pass` or `fail`
- notes: `...`

## Inventory Projection Floor (`VC-3`)

Observed:

- ESXi hosts projected as `agent`: `pass` or `fail`
- VMs projected as `vm`: `pass` or `fail`
- datastores projected as `storage`: `pass` or `fail`
- provider-local top-level types absent: `pass` or `fail`
- notes: `...`

## Storage And Snapshot Visibility (`VC-4`)

Observed:

- datastore capacity/free-space visibility: `pass` or `fail`
- datastore accessibility visibility: `pass` or `fail`
- snapshot-tree visibility: `pass`, `fail`, or `n/a`
- recovery claim remained out of scope: `yes` or `no`
- notes: `...`

## Alerts And History Floor (`VC-5`)

Observed:

- shared alert/incident surfacing works: `pass` or `fail`
- VMware-backed event/task context reachable on shared paths: `pass` or `fail`
- provider-local alarm/history shell absent: `pass` or `fail`
- notes: `...`

## Assistant Read Floor (`VC-6`)

Observed:

- `agent` inspection works: `pass` or `fail`
- `vm` inspection works: `pass` or `fail`
- `storage` inspection works: `pass` or `fail`
- VMware-specific tools not required: `pass` or `fail`
- control remained absent: `pass` or `fail`
- notes: `...`

## Exclusion Integrity (`VC-7`)

Observed:

- direct `ESXi` still excluded: `yes` or `no`
- unified-agent-required bootstrap still excluded: `yes` or `no`
- recovery support still excluded: `yes` or `no`
- assistant control still read-only / absent: `yes` or `no`
- broader VMware admin-plane promises absent: `yes` or `no`

## Captured Evidence

- connection-list screenshots or payloads: `...`
- canonical `agent` / `vm` / `storage` evidence: `...`
- alerts / incidents / history evidence: `...`
- Assistant read transcript or screenshots: `...`
- version/build evidence: `...`
- privilege bundle evidence: `...`

## Outcome

- `pass` or `fail`
- Support claim status:
  - `still blocked`
  - `ready to ratchet`
- Summary:
  - `...`
  - `...`
  - `...`

## Follow-Ups

- `none`, or:
  - `...`
  - `...`
