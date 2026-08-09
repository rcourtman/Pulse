# Pulse v6.2.0

_This changelog describes stable `v6.2.0` compared with stable `v6.1.2`._

## Added

- Unified Agent monitoring for local libvirt domains, XCP-ng pools, assigned
  external probes, secure local numeric sensors, Windows NVIDIA and hardware
  metrics, Proxmox LXC filesystems, ZFS datasets, and PBS physical disks.
- Certificate-validity monitoring, real VMware vCenter tags, wildcard Docker
  alert ignores, resource-tag notification routing, per-resource delays, and
  governed external-probe outage alerts.
- Application identity customization, in-place API-token scope editing,
  OpenShift-safe Helm deployment options, Docker update-target details, and
  in-app documentation rendering.

## Changed

- Resource identity, generation-bound views, WebSocket deltas, and agent
  auto-registration converge through canonical infrastructure state.
- Platform tables, Settings, alerts, storage, navigation, filters, and shared
  interaction surfaces use responsive, role-correct presentation.
- Agent installation and update paths use stricter version, credential,
  runtime-discovery, process-identity, rollback, and service-recovery checks.
- Release publication joins frontend, backend, Windows installer, Docker, Helm,
  signed-candidate, private-runtime, activation, and customer-convergence proof
  at one exact source SHA.

## Fixed

- Notification metadata, grouping, acknowledgements, maintenance ingestion,
  protection posture, threshold identity, and terminal failure state now remain
  coherent through configuration and resource lifecycle changes.
- Proxmox, PBS, TrueNAS, QNAP, ZFS, Docker, Kubernetes, certificate, and agent
  monitoring paths reflect authoritative source and identity data more
  consistently.
- Installer, diagnostic, sign-in, magic-link, SSH cleanup, provider restore,
  and agent configuration boundaries reject untrusted or over-broad input.
- Viewer sessions no longer receive inaccessible administrator routes or
  privileged polling, and stale or oversized live state recovers without
  becoming the new canonical baseline.
- Agent credential repair, stale version warnings, offline guest backup
  posture, responsive control reachability, and AI-provider credential fields
  preserve the intended operator state.

## Release Metadata

- Version: `v6.2.0`
- Previous stable: `v6.1.2`
- Promoted prerelease lineage: `v6.2.0-rc.11`
- Content cutoff base: `b9811cdf538224e7f2870718744300ef8f80afa0`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: owner-approved exact-SHA stable cutoff from `main`, using the
  v6.2.0-only soak waiver and the single-build release workflow
- Soak decision: the release owner waived the remainder of the normal 72-hour
  RC11 soak for v6.2.0; this is version-bound risk acceptance, not soak evidence
- Windows signing decision: the v6.2.0-only owner exception permits Windows
  Unified Agent artifacts that are not Authenticode-signed while the SignPath
  release certificate CSR remains pending; users receive an Unknown Publisher
  disclosure and checksum, detached-signature, exact-SHA, manifest, and
  published-digest verification remain mandatory
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0 iOS
  build 12 and Android versionCode 9 remain the compatible beta candidates,
  both using runtime version 2, with no companion upload or public store rollout
