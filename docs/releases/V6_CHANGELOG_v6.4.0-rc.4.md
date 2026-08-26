# Pulse v6.4.0-rc.4

_This changelog describes the changes since `v6.4.0-rc.3` included in
`v6.4.0-rc.4`._

## Added

- API token scope presentation includes the governed agent-lifecycle scope.
- Workload tables support deliberate column-width persistence and sorting by
  workload ID.
- Webhook presentation retains stable message keys and explicit resource
  context for language-neutral downstream handling.

## Changed

- Resources, connected-infrastructure entries, and active alerts use per-client
  keyed deltas. Unkeyed or missing-baseline transitions fail safely to a full
  payload instead of attempting a partial merge.
- Browser resource ingestion deduplicates capability catalogs, default policy
  posture, AI-safe summaries, and self-alias identity entries while restoring
  the existing consumer shape at the ingestion boundary.
- Realtime resource application defers during operator input and coalesces the
  queued changes after the gesture. Windowed workload, storage, and platform
  rows preserve identity while scrolling and while reordered snapshots arrive.
- REST state recovery is isolated from the websocket baseline, so a late REST
  response cannot advance or replace the per-connection delta source of truth.
- History drawers resolve canonical storage, Docker, host, VM, and container
  targets before querying metrics, including mock and API-only source paths.

## Fixed

- Notification queue health is reconciled immediately after committed queue
  transitions, clearing stale delivery system alerts after retry, dismissal,
  cancellation, purge, or successful delivery
  ([#1761](https://github.com/rcourtman/Pulse/issues/1761)).
- Proxmox VM/LXC lifecycle actions remain available through the governed action
  path without a QEMU guest-agent prerequisite. Assistant grounding no longer
  suggests manual `qm`/`pct` fallback for supported lifecycle requests
  ([#1782](https://github.com/rcourtman/Pulse/issues/1782)).
- Proxmox node network details render again and retain browser-qualified layout
  coverage.
- Backup recovery data retains the correct source attribution, repository
  filter, and delayed-hydration state.
- Docker container history retains its selected target through filtered
  refreshes, and workload drawer history no longer crosses resource identities.
- Keyed platform rows retain component identity when a full snapshot reorders
  otherwise unchanged entries.
- Mock storage and workload histories preserve continuity across refreshes.
- Offline-node E2E coverage returned to the qualified stable tier after its
  repaired proof passed.

## Release Metadata

- Version: `v6.4.0-rc.4`
- Previous candidate tag: `v6.4.0-rc.3`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while
  SignPath remains unavailable; Windows may show an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`; the changed keyed stream is a browser
  transport and the canonical core/mobile contract passed against Pulse Mobile
  revision `57353a83eb950d1102c90074aa8fe67e1559685b`, iOS build 12, and Android
  build 9, so no companion build or public store rollout is required
