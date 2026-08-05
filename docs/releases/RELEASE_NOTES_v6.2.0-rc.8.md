# Pulse v6.2.0-rc.8 Release Notes

`v6.2.0-rc.8` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.7`. This candidate focuses on
runtime resilience, authorization consistency, protection-state correctness,
responsive operator workflows, and privacy-preserving product telemetry.

## Highlights

- Pulse now aligns the Go runtime memory limit with Docker, Kubernetes, and
  systemd cgroup limits, reducing the risk of kernel OOM termination on
  memory-capped installs.
- Settings, discovery, configuration, password, and platform-administration
  routes now share one session-administrator model, including correct behavior
  for OIDC-only and organization-scoped instances.
- TrueNAS completed init containers no longer create permanent critical alerts,
  Proxmox protection evidence retains timestamp precision, and backup status
  reserves red for genuinely missing backups.
- Large threshold sections and expanded infrastructure tables remain usable at
  desktop and narrow widths, with consistent attention filters and prioritized
  responsive columns.
- Product telemetry now measures content-free adoption counts for licensed
  features while removing a Patrol autofix counter that could never become
  non-zero.

## Fixed

- Matched cross-site auto-registration to canonical Proxmox identity without
  merging ambiguous instances.
- Kept accepted and removed Unified Agent inventory synchronized with canonical
  resource state, serialized metric and remediation-history persistence, and
  restored release-candidate metrics and bundle behavior.
- Treated every successfully read WebSocket frame as client liveness and widened
  keepalive tolerance for background tabs and middleboxes that delay control
  frames.
- Stopped healthy TrueNAS applications with completed one-shot init containers
  from raising permanent critical incidents.
- Made settings capabilities agree with route enforcement, closed the
  non-admin change-password session path, restored OIDC-only administrator
  parity, and kept organization-scoped tenants outside platform administration.
- Removed the fixed-height ceiling that clipped long alert-threshold sections
  and improved responsive tables, settings controls, and attention filters.
- Reduced mock-mode startup memory by defaulting synthetic trend seeding to 48
  hours and eliminated repeated allocations in metric-role classification.
- Added privacy-preserving counts for RBAC, persistent audit logging, scheduled
  reports, alert-triggered analysis, and agent profiles without collecting
  names, permissions, recipients, report scope, or audit-event content.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 25 release
  gates passed for this release-preparation checkpoint.
- Release publication builds and validates one immutable `main` SHA before
  creating or publishing the GitHub prerelease, Docker image, Helm chart, and
  private Pro packet.
- Hardware-related fix claims now require version-bound live-runtime proof in
  addition to source and automated-test evidence.
- Targeted regression coverage locks cgroup memory-limit detection, WebSocket
  liveness, session-admin parity, TrueNAS container semantics, Proxmox
  protection posture, resource-state refresh, telemetry privacy, and responsive
  threshold and table behavior.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.8` only when you are
comfortable testing an RC. The rollback target is stable `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid and no manual data migration is required.

This server candidate is compatible with the current Pulse Mobile 1.0.0 beta
candidates. iOS build 12 is distributed through the TestFlight public beta link,
and Android versionCode 9 remains available through Play open testing; both use
runtime version 2. The changes since RC7 do not alter mobile relay payloads,
pairing, approvals, or onboarding contracts. No public mobile-store rollout is
part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
