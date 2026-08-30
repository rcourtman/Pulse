# Secure agent runtime qualification foundation (2026-08-30)

## Scope

This record now distinguishes the historical 2026-08-30 run from the hardened
qualification contract that supersedes it. The historical receipt remains
unchanged so its hash and provenance can still be checked; it must not be
presented as qualification of the current collector/helper/action-runner split.

## Landed runtime semantics

- Collector startup exercises `helper.health` over the versioned local
  protocol. Once configured, helper failures omit SMART or Proxmox privileged
  snapshots instead of falling back to local privileged collection.
- Helper-backed update activation persists a pending identity, active and
  last-known-good digests, and a bounded rollback deadline. The replacement
  process commits only after local readiness and acceptance of its newly
  collected authoritative report. Interrupted, expired, or uncommitted state
  recovers to last-known-good, and terminal transitions reap fixed staging and
  quarantine artifacts. Current code additionally verifies the root-staged
  command reports the requested advancing collector version and binds commit
  to the activating process after it execs the active digest.
- The action runner uses the canonical enrollment hostname. Durable rotation
  invalidates exactly the superseded live organization/token/agent/hostname
  session after token storage succeeds. Runner teardown attempts exact
  bearer-authenticated self-revocation before removing local remediation state;
  remote failure remains visible and does not stop local removal.
- Safe-profile apply rejects foreign effective fragments and all systemd
  drop-ins before mutation. Commit requires the intended non-root unit, a live
  helper protocol response, and a server registration timestamp newer than the
  stopped legacy collector. Before any local privilege transition it durably
  removes execution and cross-host management scope from the exact host-bound
  collector credential.
  Rollback restores installer-owned state-root metadata and the Proxmox
  registration markers, but never traverses collector-controlled descendants
  or resurrects `--enable-commands`. Rootful Docker is
  disabled unless the collector owns a usable rootless runtime socket. Local
  readiness and helper health receive a bounded startup window before the
  separate authoritative registration proof. Installed collector binaries are
  pinned to mode `0755` rather than inheriting `mktemp` permissions.

## Historical systemd evidence and corrected classification

The disposable arm64 Colima profile `pulse-agent-qual` ran Ubuntu 24.04.4,
kernel 6.8.0-117, and systemd 255. A clean detached worktree at commit
`cb843e37e8a92c56d88a8a2922c23ccd2e4fd21b` built every exercised artifact and
the lab binary. The source hashes in
`secure-agent-runtime-systemd-receipt-2026-08-30.json` match that commit; the
receipt has SHA-256
`adeeec6bf02a73c27483da31d9e058a8686900324e32960399985e847487a271` and the
separate `secure-agent-runtime-committed-main-attestation-2026-08-30.json`
binds the receipt, commit, detached checkout, and supplied artifact hashes
without altering the generated receipt. The attestation was originally
produced with `scripts/release_control/secure_runtime_attestation.py`, which
fails closed unless the checkout is detached at the full commit, all tracked
files are clean, untracked files are confined to `.lab-artifacts`, the commit
is reachable from main, every receipt source hash matches the commit blob,
all four artifact hashes match the supplied files, and the canonical twelve
scenario records say they passed in order.

That is useful integrity evidence, but the schema-v2 secret-free receipt is
not authenticated independently of the operator who supplied it. The old tool
also reported the disposable-VM guard as verified without consuming evidence
for that assertion, accepted arbitrary matching artifact bytes without Go VCS
build identity, and did not require all boundary sources. The current verifier
requires schema v3, validates the guard claim rather than calling it
independently verified, checks exact command package and clean-commit Go build
stamps, and classifies the receipt as artifact-bound self-attestation.
The guarded lab passed all of these destructive scenarios from a clean VM:

- legacy root/command-capable install with rootful Docker enabled;
- read-only profile inspection and fail-closed rejection of a systemd drop-in;
- migration to a non-root `pulse-agent` collector with no ambient capabilities,
  monitoring-only authority, a healthy typed helper, fresh server `lastSeen`,
  explicit rootful-Docker degradation, and continued reporting;
- explicit and automatic local rollback when server freshness was frozen;
  this predates irreversible server-side credential reduction and therefore
  does not prove the current rollback contract;
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
scenario outcomes. It proves neither a successful action-runner host mutation
nor the post-audit credential, rollback, update-artifact, and runner-unit
hardening. It is retained as historical, hash-bound self-attested evidence.

## Proof classification and residuals

Focused regressions cover the current semantics. A fresh schema-v3 guarded
systemd receipt is still required and must demonstrate the real apt-cache
mutation plus stale-fingerprint refusal. Even that self-attested receipt is not
a substitute for exact release-candidate evidence, representative
provider/appliance qualification, or external review.

The safe profile therefore remains opt-in. The default may not ratchet until
the exact release candidate reproduces the complete systemd result;
representative Proxmox, SMART, Docker and rootless Podman parity is recorded;
supported appliance residuals are owned; and the helper, update, action
credential, and migration boundaries complete external security review.
