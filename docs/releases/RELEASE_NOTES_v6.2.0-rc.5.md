# Pulse v6.2.0-rc.5 Release Notes

`v6.2.0-rc.5` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.4`. This candidate expands
agent, storage, alerting, and Kubernetes coverage and includes the compatibility
fixes that landed after the fourth candidate.

## Highlights

- The Unified Agent can monitor local libvirt domains and XCP-ng pools, collect
  NVIDIA GPU telemetry on Windows, read Windows CPU and storage temperatures
  through LibreHardwareMonitor, and report secure custom numeric sensors or
  REST-backed custom metrics.
- Storage visibility now includes every filesystem attached to an LXC guest,
  Proxmox ZFS datasets, and physical disks behind PBS hosts. Docker host rows
  expose their web interface and container update targets.
- Alerting gains resource-tag delivery routing, per-resource alert delays,
  guest-filesystem threshold overrides, and hardened external-probe outage
  notifications.
- Kubernetes deployments gain an OpenShift-safe Helm profile, and overview
  workload counts and navigation remain scoped to the selected cluster.
- Administrators can customize application identity, edit API-token scopes in
  place, and reopen dismissed Patrol findings.

## Added

- The Unified Agent discovers local libvirt domains and XCP-ng pools without
  requiring a separate platform connection.
- Windows agents collect NVIDIA load, temperature, memory, power, and fan
  telemetry where the driver exposes it. LibreHardwareMonitor-backed CPU and
  storage temperatures extend the same host report without weakening the
  existing collector interface.
- Custom sensors accept bounded numeric values from local commands, and REST
  custom metrics can read numeric JSON values from authenticated HTTPS
  endpoints with redacted credentials and explicit timeouts.
- Proxmox storage views surface ZFS datasets and PBS host physical disks, while
  LXC detail includes all reported guest filesystems instead of only the root
  filesystem.
- Alert destinations can be selected by resource tags. Operators can also set
  per-resource alert delays and guest-filesystem threshold overrides.
- External-probe outages use server time for freshness, emit normal governed
  alert lifecycles, and can send the new `external_probe_offline` mobile push
  type.
- The Helm chart supports OpenShift-compatible restricted-security defaults,
  including optional service-account and RBAC resources for the agent.
- General settings include configurable product name, logo, accent color, and
  support URL while preserving the default Pulse identity.

## Fixed

- Apprise Telegram deliveries retain their configured topic instead of losing
  the topic identifier in presentation or dispatch.
- QNAP installs enforce one watchdog instance and keep their singleton proof
  cross-platform.
- Discovery honors blocklisted configured subnets, and large Proxmox clusters
  retain the full polling budget instead of prematurely exhausting detail
  collection.
- PBS datastore exclusions are applied before detail polling, avoiding work
  and warnings for datastores the operator deliberately excluded.
- Metrics ingestion continues during maintenance mode so collection does not
  create an avoidable post-maintenance gap.
- Update guidance no longer recommends an unsafe LXC helper, and the remaining
  LXC instructions stay pinned to the release being installed.
- Permitted unsigned remote configuration no longer produces a misleading
  warning.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.5` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid. New local-hypervisor, hardware-sensor,
custom-metric, branding, and routing capabilities are opt-in. The OpenShift
profile changes chart permissions only when selected.

This server candidate changes the mobile push compatibility contract by adding
`external_probe_offline`. The synchronized Pulse Mobile 1.0.0 candidate is
already available to the existing beta cohort as iOS build 11 on TestFlight and
Android versionCode 9 on Play open testing; both distributed candidates use
runtime version 2. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
