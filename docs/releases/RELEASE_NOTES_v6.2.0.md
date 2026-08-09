# Pulse v6.2.0 Release Notes

`v6.2.0` is a stable minor release for the Pulse v6 line. It follows stable
`v6.1.2` and promotes the monitoring, alerting, agent lifecycle, security,
responsive-interface, and release-reliability work exercised across the
`v6.2.0-rc.1` through `v6.2.0-rc.11` line, plus the bounded fixes included in
the final stable cutoff.

## Highlights

- Expanded monitoring covers libvirt, XCP-ng, external probes, certificates,
  hardware, LXC filesystems, ZFS datasets, and PBS disks.
- Actions provides a dedicated inbox; Operational Trust keeps evidence,
  approval, execution, posture, and verification in one governed loop.
- Safer agents, responsive role-correct UI, and an immutable release pipeline
  improve supported platforms from install through rollout.

## Added

- Agent-assigned external probes with signed configuration delivery and
  governed outage alerts.
- Local libvirt and XCP-ng monitoring, secure custom numeric sensors, Windows
  NVIDIA and hardware metrics, LXC filesystem inventory, ZFS datasets, and PBS
  physical-disk reporting.
- Certificate-validity monitoring, VMware vCenter tags, application identity
  customization, in-place API-token scope editing, and OpenShift-safe Helm
  deployment options.
- Resource-tag notification routing, per-resource alert delays, explicit
  per-metric off controls, Docker update-target details, and in-app rendering
  for Pulse documentation.

## Improved

- Canonical resource identity and live-state convergence across Proxmox, PBS,
  vCenter, agent auto-registration, WebSocket deltas, migrations, and safe
  hostname repair.
- Agent update selection, rollback, service recovery, process matching,
  runtime discovery, watchdog shutdown, credential repair, and version-drift
  presentation.
- Responsive navigation, filters, settings, tables, workload identity, focus
  restoration, touch targets, and localized date presentation.
- Patrol and alert posture remain coherent when resources stop reporting,
  backups remain protected, or a request must recover from stale live state.
- Self-hosted commercial surfaces retain the deliberate opt-in boundary while
  unproved plan and cadence transitions remain unavailable.

## Fixed

- Blocked untrusted request-derived hosts, schemes, forwarded values, and SSH
  cleanup targets from entering installer, diagnostic, sign-in, magic-link, or
  legacy-removal paths.
- Preserved configured notification metadata, provider incidents,
  acknowledgements, disabled grouping, maintenance ingestion, protection
  posture, and platform-scoped threshold identity.
- Corrected Proxmox workload, storage, availability, RRD, LXC memory, PBS
  attribution, QNAP RAID, TrueNAS, ZFS, disk I/O, Docker digest, and Kubernetes
  cluster-scope behavior.
- Prevented stale or oversized WebSocket state from becoming the canonical
  recovery baseline and kept later resource deltas applied to raw server state.
- Removed viewer-only access leaks, inaccessible Settings routes, privileged
  background polling, responsive panel clipping, misleading Agent Doctor
  states, stale agent-version warnings, and browser credential autofill in AI
  provider controls.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 26 release
  gates passed, with `release_ready=True` at the stable cutoff.
- `v6.2.0-rc.11` was published through the governed prerelease path. The
  release owner explicitly waived the remainder of its normal 72-hour soak for
  v6.2.0; that version-bound decision is risk acceptance, not soak evidence.
- The exact pushed stable SHA must pass the no-publication Release Dry Run
  before the same SHA enters the single-build publication workflow.
- Windows Unified Agent binaries in v6.2.0 are not Authenticode-signed and may
  show an Unknown Publisher warning. Verify their checksums and detached
  `.sig`/`.sshsig` signatures. This is a v6.2.0-only owner exception while the
  SignPath release certificate CSR remains pending; later releases restore the
  signing requirement unless separately approved.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0`. Existing configurations
remain valid and no manual data migration is required.

The rollback target is `v6.1.2`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

This server release is compatible with the existing Pulse Mobile candidate.
Pulse Mobile 1.0.0 iOS build 12 remains on the TestFlight public beta and
Android versionCode 9 remains on Play open testing, both using runtime version
2. No companion upload or public mobile-store rollout is part of this server
release.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
Unproved self-service commercial plan or billing-cadence transitions remain
disabled and are not introduced by this release.
