# Pulse v6.4.0-rc.5

_This changelog describes the changes since `v6.4.0-rc.4` included in
`v6.4.0-rc.5`._

## Added

- Active-alert delivery diagnosis exposes pending, sent, failed, and suppressed notification outcomes.
- The append-only alert event log records lifecycle transitions and notification decisions, including suppression mechanisms and reasons.
- Agent install-token responses separate the one-time token value from the generated installation command.

## Changed

- Canonical metric, lifecycle, and stateful alert families use the deterministic reducer core for activation, confirmation, recovery, acknowledgement restoration, cooldown refire, and recent-resolution state.
- Legacy tracking maps remain compatibility mirrors while reducer-owned transitions become authoritative.
- Alert delivery evidence and held notifications are projected onto the existing alert overview and delivery activity surfaces.
- Confirmation, intent, recovery, and backup-offline deferral semantics are characterized by composed parity suites before and after family cutover.

## Fixed

- API-token deletion snapshots the full inventory, persists the exact reduced inventory, and rolls the live token set and primary-token projection back on persistence failure ([#1783](https://github.com/rcourtman/Pulse/issues/1783)).
- Canonically keyed metric alerts resolve through their canonical identity rather than leaving stale active records after recovery.
- Confirmation-based lifecycle timestamps retain the first matched observation and restart correctly after an interrupted confirmation run.
- Stateful manual clears preserve recent-resolution and acknowledgement retention, preventing duplicate history on a quick refire.
- Filesystem history drawers again render loading progress.
- Guest and host memory explanations use the canonical cache-aware usage basis.

## Release Metadata

- Version: `v6.4.0-rc.5`
- Previous candidate tag: `v6.4.0-rc.4`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable; Windows may show an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`; the changed alert transition internals and browser delivery evidence preserve the existing mobile, Relay, onboarding, route, request/response, pairing, and push contracts, so no companion build or public store rollout is required
