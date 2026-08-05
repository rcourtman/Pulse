# Pulse v6.2.0-rc.8

_This changelog describes the changes since `v6.2.0-rc.7`.
`v6.2.0-rc.8` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- Cgroup-aware Go memory-limit configuration for container and systemd memory
  caps.
- Privacy-preserving adoption counts for licensed RBAC, audit, reporting,
  alert-analysis, and agent-profile features.
- Version-bound live-runtime evidence for hardware-related release claims.

## Improved

- Unified session-administrator decisions across settings capabilities,
  discovery, configuration, password, and platform-administration routes.
- Accepted any inbound WebSocket frame as proof of liveness and increased
  tolerance for delayed control frames.
- Serialized metrics and remediation-history persistence and reduced mock-mode
  startup memory and allocation pressure.
- Prioritized responsive infrastructure columns, standardized attention
  filters, and made narrow settings controls and expanded tables fit reliably.

## Fixed

- Matched cross-site auto-registration to canonical Proxmox identity.
- Restored Proxmox protection-posture evidence without losing timestamp
  precision.
- Stopped completed TrueNAS init containers from creating permanent critical
  incidents.
- Refreshed canonical resource state after accepted or removed agent inventory.
- Corrected authorization and capability parity for non-admin, OIDC-only, and
  organization-scoped sessions.
- Reserved red backup status for missing backups and removed the fixed-height
  ceiling that clipped long threshold sections.
- Removed the structurally inert Patrol autofix telemetry counter.

## Release Metadata

- Version: `v6.2.0-rc.8`
- Previous candidate: `v6.2.0-rc.7`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0 iOS
  build 12 is distributed through the TestFlight public beta link and Android
  versionCode 9 remains on Play open testing, both using runtime version 2. The
  changes since RC7 do not alter mobile relay payloads, pairing, approvals, or
  onboarding contracts. No public store rollout is part of this candidate
