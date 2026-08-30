# Secure agent runtime qualification foundation (2026-08-30)

## Scope

This slice closes implementation defects found while preparing a real Linux
systemd qualification of the optional collector/helper/action-runner split. It
also records a managed-development real-systemd qualification of the collector
and helper migration boundary. It does not flip the installer default and is
not exact committed release-candidate or representative live-provider proof.

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

## Managed-development systemd proof

The disposable arm64 Colima profile `pulse-agent-qual` ran Ubuntu 24.04.4,
kernel 6.8.0-117, and systemd 255. The exact working-tree installer and lab
source are bound by the hashes in
`secure-agent-runtime-systemd-receipt-2026-08-30.json`; the receipt itself has
SHA-256 `c20aee566835ac88f6163085d2d559c3fa493274b1a8eae328ed39063af06e5e`.
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
  committed safe-profile migration.

The receipt is intentionally secret-free and contains source/artifact hashes,
host facts, resulting privilege posture, report timestamps, and the eight
scenario outcomes. This is managed-development proof because it exercised the
working tree before its eventual commit identity existed.

## Proof classification and residuals

The focused protocol, update, runner, API, and installer regressions plus the
managed systemd receipt are proof for these semantics. They are not a
substitute for exact committed release-candidate evidence, representative
provider/appliance qualification, or external review.

The safe profile therefore remains opt-in. The default may not ratchet until
the exact release candidate reproduces the systemd result and proves runner
rotation/revocation plus typed action receipts; representative Proxmox, SMART,
Docker and rootless Podman parity is recorded; supported appliance residuals
are owned; and the helper, update, action credential, and migration boundaries
complete external security review.
