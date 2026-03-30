# VMware vCenter Phase-1 Proof Blocked Record

- Date: `2026-03-30`
- Candidate lane: `platform-admission-execution`
- Platform: `vmware-vsphere`
- Result: `blocked`

## Blocking Facts

1. This workspace still has no VMware capability recorded in
   `LOCAL_CAPABILITIES.md`.
2. `VC-0` from
   `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`
   is therefore not satisfied.
3. Without that recorded environment, Pulse cannot honestly prove the supported
   `vCenter` version floor, the minimum read privilege bundle, or the live
   connection-test behavior required for the phase-1 floor.
4. Future agents also do not yet have a non-secret discovery path for the first
   real VMware proof environment.
5. The governing architecture, onboarding, projection, alerts/Assistant, and
   backend API/runtime contracts are already locked, so implementation planning
   or code work may continue against those contracts even while the support
   claim remains blocked.

## Why The Support Claim Cannot Be Cleared Yet

Pulse has enough evidence to say what the VMware support model should be, but
not enough evidence to say VMware is supported in practice.

The support-claim ratchet now requires both:

1. one real `vCenter` capability recorded in `LOCAL_CAPABILITIES.md`
2. one successful live pass of `VC-0` through `VC-7` in
   `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`

Until those conditions hold together, product wording, settings surfaces, and
Assistant surfaces must not claim VMware support.

## Required Unblock Steps

1. Add a non-secret VMware capability entry to `LOCAL_CAPABILITIES.md`
   including purpose, safe verification command, access notes, and the alias of
   the secret location.
2. Confirm the environment exposes at least one ESXi host, one VM, one
   datastore, one recent event or task, and ideally one VM snapshot.
3. Run `VC-0` through `VC-7` from
   `docs/release-control/v6/internal/VMWARE_VCENTER_PHASE1_PROOF_MATRIX.md`
   against that real environment.
4. Capture the result in a dated record at
   `docs/release-control/v6/internal/records/vmware-vcenter-phase1-proof-<YYYY-MM-DD>.md`.
5. Replace this blocked record only after the real proof pass holds and the
   support wording still matches the narrow governed floor exactly.

## Scope Of This Blocked Record

This record captures the current absence of live VMware proof.

It does not mean VMware implementation must stop. It means Pulse must not cross
the line from planned platform candidate to supported platform claim until the
first real proof run is recorded successfully.
