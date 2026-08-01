# Pulse v6.2.0-rc.6 Release Notes

`v6.2.0-rc.6` is a release candidate focused on the next Pulse v6 minor line.
It follows stable `v6.1.2` and supersedes `v6.2.0-rc.5`. This candidate repairs
the primary regressions reported against RC5 and adds release-blocking proof
for the affected pages and Windows installer path.

## Highlights

- Proxmox VE workloads are visible again, including on the aggregate Proxmox
  view used by the main platform page.
- Alert threshold settings tolerate guest filesystems whose usage value is
  absent instead of crashing the page.
- Installer update paths detect and repair half-removed installations instead
  of reporting success while leaving the service unusable.
- Windows Unified Agent commands now preserve both custom-CA trust and Skip
  TLS verification through the first PowerShell 5.1 download.
- Direct PBS connections and their Unified Agent reports collapse into one
  connected-system identity instead of appearing as duplicates.
- Agent Doctor no longer invents a deployed `v0` or warns solely because a
  legacy deployment-status record is absent.

## Fixed

- Corrected the aggregate `proxmox-all` platform filter so PVE workloads no
  longer disappear from the Proxmox page.
- Normalized omitted disk-usage values at the API boundary and in shared disk
  presentation so Thresholds, workload rows, disk cards, and RAID cards remain
  renderable when a guest filesystem has not reported usage yet.
- Made Unix and Windows update-mode installation repair missing binaries,
  service definitions, and related partial-removal state before declaring the
  operation successful.
- Corrected Windows certificate-callback lifetime and closure handling for
  custom certificate authorities and explicit TLS-skip commands.
- Applied direct-platform attachment consistently to PVE, PBS, PMG, and
  TrueNAS while failing closed on ambiguous matches, preventing duplicate PBS
  API and agent rows without incorrectly merging VMware systems. PBS and PMG
  connections now also pair with their host agents through the hostname the
  node reports about itself and the shared machine-identity grouping, so the
  merge no longer depends on the configured address literally matching a name
  the agent reports.
- Backed container image update checks off for an hour after a registry rate
  limit instead of retrying every cycle, which kept the public rate allowance
  permanently exhausted on busy Docker hosts.
- Stopped community-registry update checks for the entitled Pro runtime
  image, which always requires authentication and produced a persistent
  false `authentication required` badge.
- Made the current connection fingerprint the authoritative applied-profile
  signal. Real failed, pending, and version-drift states still warn, while a
  missing legacy status record stays quiet and the UI reports `Assigned vX`
  rather than a fictitious deployed version.

## Release Qualification

- Every release candidate now runs the complete frontend unit suite,
  TypeScript checking, and a render-level smoke over Proxmox, Docker,
  Kubernetes, and Alert thresholds before publication can begin.
- The release smoke includes Proxmox nodes and workloads plus a threshold
  payload with omitted-zero disk usage, so the two RC5 page failures are
  exercised at the publication boundary.
- The generated Windows TLS commands are executed under Windows PowerShell 5.1
  against self-signed and custom-CA HTTPS servers before release assembly.
- Targeted backend and frontend contract tests cover connected-system identity,
  installer repair, disk normalization, and Agent Doctor profile state.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.6` only when you are
comfortable testing an RC. The rollback target is `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid. This is a corrective candidate and does
not require configuration migration. Systems that experienced a partial agent
removal can rerun the normal install command to repair the installation.

This server candidate has no new mobile-facing behavior beyond RC5. The
synchronized Pulse Mobile 1.0.0 candidate remains available to the existing
beta cohort as iOS build 11 on TestFlight and Android versionCode 9 on Play
open testing; both distributed candidates use runtime version 2. No public
mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
