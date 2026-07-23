# Known RC Issue Closure For GA Host Agent Re-enrollment Record

- Date: `2026-07-23`
- Gate: `known-rc-issue-closure-for-ga`
- Issue: `#1581`
- Result: `fixed-main-proof`

## Context

Issue `#1581` reported that deleting a host could leave its machine identity
permanently rejected, preventing the agent from re-enrolling. The first fix,
commit `2bc48774c216ff53c6e8060e321f2edbb6d368fd`, shipped in `v6.1.0-rc.2`
and is present in `v6.1.1`. It made a token created after removal explicit
re-enrollment intent and corrected expiry of the removal entries stored in the
live monitor state.

The issue's restart test did not reconstruct a monitor from disk. It created a
new in-memory state and manually inserted a removal entry, while the production
`models.State` removal list is not persisted. A real server restart therefore
lost the deny boundary rather than proving its expiry or re-enrollment
semantics. A report already in flight could also complete after deletion and
recreate the host.

## Disposition

The existing durable `host_continuity.json` identity record is now the atomic
host lifecycle journal. Deletion writes a tombstone before revoking an unused
token or cleaning live resources. Monitor construction hydrates tombstones
before accepting reports. Active continuity, monitored-system, licensing, and
remote-config reads exclude removed entries. Journal load, write, and expiry
fail closed; failed deletion restores the live host.

Re-enrollment requires a different token created after removal plus the
retained canonical or agent/report alias, compatible machine ID, and compatible
hostname. It preserves the canonical host ID across Linux systemd machine IDs,
Docker unified-agent IDs, and Windows MachineGuid IDs. The detached old token
remains denied for that identity after re-enrollment, even when it is shared
and valid for another active agent. Manual admin `allow-reenroll` remains the
explicit override.

A dedicated lifecycle read/write lock orders report ingestion against delete,
allow, and expiry. Concurrent reports complete before deletion or observe the
tombstone afterward; a successful delete cannot be undone by an in-flight
report.

## Proof

- `go test ./internal/config -run TestHostContinuityStore -count=1`
- `go test ./internal/monitoring -run 'TestHostAgentRemovalLifecycle|TestApplyHostReport_FreshTokenClearsRemovalBlock|TestAllowHostAgentReenroll_ClearsStateMirrorWithoutMemoryEntry|TestCleanupRemovedHostAgents_ExpiresStateMirrorEntries|TestMatchHostConfigContinuity' -count=1`
- `go test -race ./internal/monitoring -run TestHostAgentRemovalLifecycle -count=1`
- `go test ./internal/api -run TestHostAgentRemovalLifecycleThroughAuthenticatedRouterAndRestart -count=1`
- `go test ./scripts/installtests -run 'TestPulseAgentStateDirLifecycleIntegration|TestInstallSHRecoversSavedStateForPartialUninstallContext|TestInstallPS1OwnsWindowsServiceLoggingAndRecovery|TestInstallPS1PersistsAndRecoversConnectionIdentity' -count=1`
- `go test ./internal/config -count=1`
- `go test ./internal/monitoring -count=1`
- `go test ./internal/api -count=1`
- `go vet ./internal/config ./internal/monitoring ./internal/api`
- subsystem registry, contract, status, control-plane, staged-shape, and
  canonical-completion audits

## Outcome

`v6.1.1` already contains the `rc.2` fresh-token fix for the reported
same-process symptom. The durable restart, detached-shared-token, and
concurrent-report hardening in this record is on `main` for a future release;
it is not retroactively part of `v6.1.1`, and no publication date is promised
by this proof. The gate remains satisfied for current source because both the
original regression and the residual production lifecycle gaps have objective
coverage.
