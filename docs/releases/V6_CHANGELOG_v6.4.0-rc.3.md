# Pulse v6.4.0-rc.3

_This changelog describes the changes since `v6.4.0-rc.2` included in
`v6.4.0-rc.3`. It carries that candidate packet forward and also re-lists fixes
first shipped on the `v6.3.2` stable patch line where they remain part of the
next minor release._

## Added

- Notification delivery history exposes recent redacted delivery outcomes,
  while test sends report when delivery is paused instead of claiming success.
- A least-privilege Unified Agent install profile creates a non-login service
  account and grants only the selected system inspection commands through sudo.
- Docker, Kubernetes and host-agent diagnostics expose an **Allow re-enrol**
  recovery control when a prior removal blocks a legitimate returning agent.
- MSP evaluation mode supports provider trials, and the annual Business tier is
  available for teams that need the business plan.

## Changed

- Large-estate storage and platform navigation uses windowed rows, incremental
  route loading and stable scroll ownership. The measured storage cold path
  improved from approximately 15.6 seconds to 1.1 seconds.
- `/api/state` fell from 4.75 MB to 4.09 MB in the measured estate. Resource
  updates use changed-ID catch-up, including mobile Alerts entry, and merge
  patched keys without rebuilding unaffected resource state.
- Per-tick real-time merges and workload row updates now avoid redundant work.
- Saved views have been removed. URL-backed filters remain bookmarkable and
  shareable without a second saved-view state model.

## Fixed

- Metrics-history retention releases oversized backing arrays after dense seed
  data ages out. This correction first shipped in stable `v6.3.2` and remains
  included in the next minor line.
- Patrol retries provider initialisation on every scheduled run, exposes the
  actual blocked reason and self-recovers from its early-start race
  ([fix](https://github.com/rcourtman/Pulse/commit/7c37a85cb)).
- Proxmox backup roll-ups derive `totalPages` from the normalised page size
  ([fix](https://github.com/rcourtman/Pulse/commit/6686cdce2)).
- The Proxmox storage drawer stacks the TLS and command-execution checkboxes so
  both controls remain usable
  ([#1775](https://github.com/rcourtman/Pulse/commit/a28b3535b)).
- Storage rows mark retained readings stale and show the last good refresh age
  after a poll failure.
- Stopped Docker and Podman containers no longer re-fire health alerts from a
  retained stale health result.
- Disabled Proxmox backup collection scope persists across configuration
  reloads.
- Fixed polling intervals, disabled offline policies, command intent and
  re-enrolment cleanup remain intact across monitoring and agent recovery.
- Proxmox storage details show RAID controller member disks again.
- API-only Proxmox nodes retain node metrics history without presenting agent-
  only disk I/O; linked-agent nodes keep the full host history groups.

## Release Metadata

- Version: `v6.4.0-rc.3`
- Previous candidate tag: `v6.4.0-rc.2`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while
  SignPath remains unavailable; Windows may show an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`; no companion build or public store
  rollout is required
