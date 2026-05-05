# Agent Lifecycle Proxmox Runtime Token Permission Proof

- Date: `2026-05-05`
- Lane: `L16`
- Adjacent lane: `L1`
- Trigger: Runtime-side follow-up to generated Proxmox setup permission hardening

## Context

Generated Proxmox setup scripts already use privilege-separated PVE tokens and
mirror ACLs to the concrete token id. The runtime-side host-agent setup and the
root installer auto-registration path still had their own PVE token creation
logic, so they needed the same canonical permission model instead of a parallel
shared-token or user-only ACL variant.

## Outcome

- `internal/hostagent/proxmox_setup.go` now creates and rotates PVE monitor
  tokens with privilege separation enabled, then applies `PVEAuditor`, optional
  `PulseMonitor`, and `/storage` `PVEDatastoreAdmin` grants to both
  `pulse-monitor@pve` and the concrete `pulse-monitor@pve!pulse-*` token id.
- Host-agent PVE setup keeps the non-destructive `PulseMonitor` role update
  path while applying the selected guest-agent privilege shape to the token as
  well as the service user.
- The root `install.sh` auto-registration path now uses the same
  privilege-separated PVE token posture, comma-joined `PulseMonitor` privileges,
  non-destructive role update, and user-plus-token ACL pairing before posting the
  generated token to `/api/auto-register`.

## Proof Commands

Run as part of this slice:

- `bash -n install.sh`
- `go test ./internal/hostagent -run 'TestProxmoxSetup_(SetupPVETokenUsesPrivilegeSeparatedTokenACLs|ConfigurePVEPermissions_FallsBackToGuestAgentAudit|RunForType|RunAll)|TestPrivProbeRoleName|TestProxmoxSetup_ProbePVEPrivilege' -count=1`
- `go test ./scripts/installtests -run TestRootInstallScriptAutoRegisterUsesSecureContractShape -count=1`
- `go test ./internal/hostagent -count=1`
- `go test ./scripts/installtests -count=1`
- `python3 scripts/release_control/status_audit.py --pretty`
- `python3 scripts/release_control/contract_audit.py --pretty`
- `python3 scripts/release_control/registry_audit.py --pretty`
- `python3 scripts/release_control/canonical_completion_guard.py`
- `python3 scripts/release_control/staged_commit_shape_guard.py`
- `git diff --cached --check`

## Follow-Up Guard

Any future PVE setup path that creates a Pulse-managed API token must use the
same privilege-separated token contract and mirror effective ACLs to the service
user and concrete token id. A user-only ACL or `--privsep 0` token is not a
canonical Proxmox API Inventory setup path.
