# Pulse v6.2.0-rc.5

_This changelog describes the changes since `v6.2.0-rc.4`.
`v6.2.0-rc.5` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- Unified Agent monitoring for local libvirt domains and XCP-ng pools.
- Windows NVIDIA GPU metrics and LibreHardwareMonitor-backed CPU and storage
  temperatures.
- Secure local numeric sensors and authenticated REST custom metrics.
- Proxmox LXC filesystem inventory, ZFS datasets, and PBS host physical disks.
- Resource-tag alert delivery routing, per-resource alert delays, and
  guest-filesystem threshold overrides.
- Governed external-probe outage alerts and `external_probe_offline` mobile
  push notifications.
- An OpenShift-safe Helm deployment profile with optional agent service-account
  and RBAC resources.
- Application identity customization, in-place API-token scope editing, and
  reopening for dismissed Patrol findings.
- Docker host web-interface links, container update-target details, and
  host-relative guest memory presentation.

## Fixed

- Windows Unified Agent installer downloads honor the selected Skip TLS
  verification setting and retain custom-CA certificate state correctly.
- Machine rows aggregate agent-reported disks for multi-volume Windows hosts
  when no canonical disk summary is present.
- Telegram topic selection survives Apprise presentation and dispatch.
- QNAP watchdog installation and runtime enforce a single active watchdog.
- Configured-subnet discovery blocklists are honored.
- Large Proxmox clusters retain their full poll budget, and PBS datastore
  exclusions run before detail polling.
- Metrics continue ingesting during maintenance mode.
- LXC update guidance stays release-local and no longer recommends the unsafe
  helper path.
- Permitted unsigned remote configuration no longer raises a false warning.
- Agent collector interface compatibility and cross-platform verification are
  preserved across the new collectors.

## Changed

- Kubernetes overview workload state is scoped by cluster.
- GPU load is promoted to a first-class metric across collection, storage,
  presentation, thresholds, and history.

## Release Metadata

- Version: `v6.2.0-rc.5`
- Previous candidate: `v6.2.0-rc.4`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0
  iOS build 11 and Android versionCode 9, both on runtime version 2, were
  distributed to the existing beta cohort on 2026-07-30. No public store
  rollout is part of this candidate
