# Pulse v6.2.0-rc.7 Release Notes

`v6.2.0-rc.7` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.6`. This candidate focuses on
resource identity continuity, platform correctness, responsive operator
workflows, in-app documentation, and safer install and release recovery.

## Highlights

- Proxmox guests keep one canonical identity when they move between cluster
  nodes, and host identity forks caused by hostname changes heal without
  detaching findings or operator state.
- TrueNAS ZFS ARC is treated as reclaimable cache, host-relative memory trends
  are calculated consistently, and valid virtio, Xen, and non-SMART block
  devices remain eligible for host disk I/O collection.
- Platform tables, filter bars, saved views, tabs, and View menus are more
  consistent and remain usable on narrow screens.
- The documentation set linked from Pulse now ships with the application and
  renders in-app, including a dedicated transparency statement for Pulse AI
  features and operator-facing agent integration guidance.
- Install and release paths gained safer auto-update bootstrap, Windows command
  handling, bounded configuration-backup retention, and recoverable private Pro
  publication retries.

## Fixed

- Preserved Proxmox guest identity, guest metadata, alert overrides, and finding
  references across cluster node migrations and hostname changes.
- Corrected Proxmox workload type and attention filtering, filtered totals,
  backup saved views, and active tab visibility.
- Corrected TrueNAS memory accounting for ARC cache and fixed host-relative
  memory history so current and historical values use the same trust model.
- Kept virtio, Xen, and other real block devices in disk I/O collection while
  continuing to exclude virtual pseudo-devices.
- Prevented resource state reads from queueing behind registry rebuilds and
  removed the obsolete `resource_metadata` shadow table.
- Showed digest-pinned container images as `Pinned`, named both relevant digests
  when an update preflight refuses a change, and kept empty Docker View menus
  out of the interface.
- Normalized filter reset, saved-view, Add filter, active-tab, and horizontal
  scrolling behavior across Alerts, Patrol, Storage, Machines, Proxmox,
  Kubernetes, and Docker surfaces.
- Created the configuration directory before enabling auto-updates, kept copied
  Windows install commands on one line, and moved the installer TLS callback to
  a compiled type compatible with Windows PowerShell 5.1.
- Rotated configuration backups instead of allowing unbounded growth.
- Classified license-server transport failures as retryable and made
  promotion-only private Pro publication failures reuse the already built proof
  packet instead of attempting to overwrite it.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 25 release
  gates passed for this release-preparation checkpoint.
- Release publication still builds and validates one immutable `main` SHA before
  creating or publishing the GitHub prerelease, Docker image, Helm chart, and
  private Pro packet.
- Browser-verification receipts are now enforced for user-visible frontend
  changes, including desktop and narrow viewport proof tied to staged content.
- Targeted regression coverage locks Proxmox identity succession, hostname-rename
  healing, TrueNAS ARC memory, workload filtering, installer bootstrap, Windows
  TLS command execution, responsive tables, saved views, and release retry
  behavior.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.7` only when you are
comfortable testing an RC. The rollback target is stable `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid and no manual data migration is required.
The identity-healing migrations are conservative: ambiguous matches remain
separate instead of being merged automatically.

This server candidate is compatible with the current Pulse Mobile 1.0.0 beta
candidates. iOS build 12 is distributed through the TestFlight public beta link,
and Android versionCode 9 remains available through Play open testing; both use
runtime version 2. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
