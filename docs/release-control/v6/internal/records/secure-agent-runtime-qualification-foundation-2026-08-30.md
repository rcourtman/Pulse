# Secure agent runtime qualification foundation (2026-08-30)

## Scope

This slice closes implementation defects found while preparing a real Linux
systemd qualification of the optional collector/helper/action-runner split. It
also records an exact committed-main real-systemd qualification of the
collector/helper migration boundary and the separately credentialed action
runner lifecycle. It does not flip the installer default and is not exact
committed release-candidate or representative live-provider proof.

## Landed runtime semantics

- Collector startup exercises `helper.health` over the versioned local
  protocol. Once configured, helper failures omit SMART or Proxmox privileged
  snapshots instead of falling back to local privileged collection.
- Helper-backed update activation persists a pending identity, active and
  last-known-good digests, and a bounded rollback deadline. The replacement
  process commits only after local readiness and acceptance of its newly
  collected authoritative report. Interrupted, expired, or uncommitted state
  recovers to last-known-good, and terminal transitions reap fixed staging and
  quarantine artifacts.
- The action runner uses the canonical enrollment hostname. Durable rotation
  invalidates exactly the superseded live organization/token/agent/hostname
  session after token storage succeeds. Runner teardown attempts exact
  bearer-authenticated self-revocation before removing local remediation state;
  remote failure remains visible and does not stop local removal.
- Safe-profile apply rejects foreign effective fragments and all systemd
  drop-ins before mutation. Commit requires the intended non-root unit, a live
  helper protocol response, and a server registration timestamp newer than the
  stopped legacy collector. Rollback restores full state-tree metadata and the
  Proxmox registration markers touched by installation. Transient runtime
  files that disappear while the collector is stopped do not prevent rollback
  from restoring every surviving pre-migration entry. Rootful Docker is
  disabled unless the collector owns a usable rootless runtime socket. Local
  readiness and helper health receive a bounded startup window before the
  separate authoritative registration proof. Installed collector binaries are
  pinned to mode `0755` rather than inheriting `mktemp` permissions.

## Exact committed-main systemd proof

The disposable arm64 Colima profile `pulse-agent-qual` ran Ubuntu 24.04.4,
kernel 6.8.0-117, and systemd 255. A clean detached worktree at commit
`cb843e37e8a92c56d88a8a2922c23ccd2e4fd21b` built every exercised artifact and
the lab binary. The source hashes in
`secure-agent-runtime-systemd-receipt-2026-08-30.json` match that commit; the
receipt has SHA-256
`adeeec6bf02a73c27483da31d9e058a8686900324e32960399985e847487a271` and the
separate `secure-agent-runtime-committed-main-attestation-2026-08-30.json`
binds the receipt, commit, clean checkout, and fresh disposable VM without
altering the generated receipt.
The guarded lab passed all of these destructive scenarios from a clean VM:

- legacy root/command-capable install with rootful Docker enabled;
- read-only profile inspection and fail-closed rejection of a systemd drop-in;
- migration to a non-root `pulse-agent` collector with no ambient capabilities,
  monitoring-only authority, a healthy typed helper, fresh server `lastSeen`,
  explicit rootful-Docker degradation, and continued reporting;
- exact explicit rollback and automatic rollback when server freshness was
  frozen, including stable binary, unit, credential-state, ownership, and mode
  identity;
- ordinary update without implicit privilege migration, followed by a final
  committed safe-profile migration;
- separate root action-runner installation with a canonical hostname and
  independently scoped credential while the non-root collector continued
  reporting;
- a closed typed storage-cleanup request that refused before mutation, produced
  a durable terminal receipt, replayed through receipt query, and could not be
  widened into generic command dispatch;
- credential replacement that rejected mismatched invalidation, closed exactly
  the superseded live session, and admitted the replacement binding;
- authenticated self-revoke followed by runner-only uninstall, with the
  collector and helper still healthy and reporting.

The receipt is intentionally secret-free and contains source/artifact hashes,
host facts, resulting privilege posture, report timestamps, and the twelve
scenario outcomes. This is exact committed-main proof. It is still not exact
release-candidate proof because no release candidate was designated for this
run.

## Proof classification and residuals

The focused protocol, update, runner, API, and installer regressions plus the
committed-main systemd receipt are proof for these semantics. They are not a
substitute for exact committed release-candidate evidence, representative
provider/appliance qualification, or external review.

The safe profile therefore remains opt-in. The default may not ratchet until
the exact release candidate reproduces the complete systemd result;
representative Proxmox, SMART, Docker and rootless Podman parity is recorded;
supported appliance residuals are owned; and the helper, update, action
credential, and migration boundaries complete external security review.
