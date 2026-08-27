# Pulse v6.4.0-rc.6

_This changelog describes the changes since `v6.4.0-rc.5` included in
`v6.4.0-rc.6`._

## Added

- SMART UDMA CRC counter growth now raises a disk-health warning after the initial baseline sample.
- Standalone PBS backup-server entries expose the full merged host resource drawer when agent hardware evidence is available.

## Changed

- Alert configuration resolves through one declarative policy fold for enabled state, thresholds, delays, and built-in defaults.
- Legacy transition-tracking maps have been removed now that reducer-owned alert lifecycle state is authoritative.
- Release builds use Go 1.26.7 across the public server, agent, provider control plane, and release scripts.

## Fixed

- Standalone PBS rows open the canonical resource drawer instead of a limited four-field summary, restoring system, hardware, network, disks, thermals, services, history, and management context ([#1723](https://github.com/rcourtman/Pulse/issues/1723)).
- SMART CRC growth treats resets and disk replacements as a new baseline and resolves an active growth event on a stable follow-up sample ([#1776](https://github.com/rcourtman/Pulse/issues/1776)).
- Alert-event queries allocate from the bounded effective result limit rather than the raw client-supplied limit.
- Alert policy evaluation no longer relies on duplicated resource-family lookup branches or legacy mutable transition mirrors.

## Release Metadata

- Version: `v6.4.0-rc.6`
- Previous candidate tag: `v6.4.0-rc.5`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable; Windows may show an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`; the changed PBS browser detail, SMART alerting, and alert evaluation internals preserve the existing mobile, Relay, onboarding, route, request/response, pairing, and push contracts, so no companion build or public store rollout is required
