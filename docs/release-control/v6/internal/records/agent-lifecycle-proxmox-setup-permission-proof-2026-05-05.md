# Agent Lifecycle Proxmox Setup Permission Proof

- Date: `2026-05-05`
- Lane: `L16`
- Trigger: Follow-up to Proxmox API-first onboarding and root-agent hardening

## Context

The Proxmox API Inventory path should be the default for ordinary PVE/PBS
inventory, but that only holds if the generated setup script creates credentials
that are narrow in steady state. PVE API tokens with privilege separation need
their own ACLs, and those token ACLs remain bounded by the associated user.

## Outcome

Generated Proxmox setup scripts now pin the permission model directly:

- PVE setup creates the `pulse-monitor@pve!pulse-*` token with privilege
  separation enabled.
- PVE setup applies `PVEAuditor`, optional `PulseMonitor`, and optional
  `/storage` `PVEDatastoreAdmin` grants to both `pulse-monitor@pve` and the
  concrete generated token id.
- PBS setup proof now asserts the generated script grants only `Audit` to both
  `pulse-monitor@pbs` and the concrete generated token id.
- PVE backup permission warnings and manual repair docs now include the token
  `/storage` ACL as well as the service-user ACL, so operators do not apply a
  user-only repair that privilege-separated tokens still cannot use.
- Operator-facing security docs and release-control contracts now record that
  API Inventory is a one-time privileged setup action followed by API-token
  monitoring, not a root Pulse agent deployment.

## Proof Commands

Run as part of this slice:

- `go test ./internal/api -run 'TestPVESetupScript|TestPBSSetupScript|TestHandleSetupScript|TestRenderSetupScript' -count=1`
- `go test ./internal/monitoring -run TestPVEBackupPermissionWarning -count=1`
- `go test ./internal/api -count=1`
- `go test ./internal/monitoring -count=1`
- `python3 scripts/release_control/status_audit.py --pretty`
- `python3 scripts/release_control/contract_audit.py --pretty`
- `python3 scripts/release_control/registry_audit.py --pretty`

## Follow-Up Guard

Future generated Proxmox setup changes must keep PVE token ACLs and user ACLs
paired. A user-only ACL is not enough for privilege-separated PVE tokens, and a
shared/unprivileged token is not the canonical API Inventory posture.
