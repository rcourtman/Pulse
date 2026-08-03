# Pulse v6.2.0-rc.7

_This changelog describes the changes since `v6.2.0-rc.6`.
`v6.2.0-rc.7` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- In-app rendering for the documentation set linked from Pulse.
- A public AI transparency statement and clearer agent integration guidance.
- Enforced, content-bound browser verification receipts for user-visible
  frontend changes.
- A provider MSP evaluation flow that can issue its bounded evaluation licence
  without requiring an operator to provision one manually.

## Improved

- Centralized platform View controls, saved views, filter presentation, active
  item visibility, and narrow-screen table behavior.
- Made registry-backed state reads independent of registry rebuild latency.
- Rotated configuration backups and strengthened auto-update bootstrap.
- Made private Pro promotion-only release failures recoverable by reusing the
  immutable proof packet from the original build.

## Fixed

- Preserved canonical Proxmox guest identity and metadata across node migrations.
- Healed safe hostname-rename identity forks while preserving findings, alert
  overrides, and operator state.
- Corrected Proxmox workload type and attention filtering, filtered totals,
  saved views, and navigation state.
- Treated TrueNAS ZFS ARC as reclaimable cache and corrected host-relative
  memory trends.
- Retained valid virtio, Xen, and non-SMART block devices in disk I/O metrics.
- Corrected digest-pinned container status and update-preflight diagnostics.
- Kept Windows install commands on one line and executed the TLS callback
  through a compiled type compatible with Windows PowerShell 5.1.
- Classified license-server transport failures as retryable.

## Release Metadata

- Version: `v6.2.0-rc.7`
- Previous candidate: `v6.2.0-rc.6`
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
  versionCode 9 remains on Play open testing, both using runtime version 2. No
  public store rollout is part of this candidate
