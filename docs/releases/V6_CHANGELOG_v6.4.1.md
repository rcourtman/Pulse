# Pulse v6.4.1

_This changelog describes stable `v6.4.1` compared with stable `v6.4.0`._

## Changed

- Anthropic cost accounting recognizes current Sonnet 5, Fable 5, and Mythos
  5 model identifiers before broad family fallbacks so budget summaries remain
  enforceable in both overestimate and unknown-price cases.
- Release qualification verifies executable mode on the embedded Unified Agent
  and non-executable mode on detached signature sidecars before publication.
- Published releases remain immutable when a later validation event reports a
  problem. Remediation moves forward through an explicit patch release.

## Fixed

- The prebuilt server image restores executable mode on copied Unified Agent
  payloads, so `/usr/local/bin/pulse-agent` works for Helm agent workloads and
  direct container entrypoints ([#1795](https://github.com/rcourtman/Pulse/issues/1795)).
- Proxmox VM and LXC REST resources preserve source-authored backup, uptime, and
  guest metadata on first paint instead of waiting for a WebSocket update
  ([#1792](https://github.com/rcourtman/Pulse/issues/1792)).
- TrueNAS SMART spare-block reserve evidence accepts supported scalar and nested
  integer representations without overflowing or projecting malformed,
  fractional, or out-of-range values as percentages.
- API token creation, whole-inventory regeneration, migration, and container
  runtime preparation restore complete prior live state when persistence fails.
- First-run credential reset no longer commits a password change when the
  paired API token update cannot be persisted.
- Stable rollback guidance uses the installed `/bin/update` helper instead of
  referring to a repository-relative script that may not exist on the server.

## Release Metadata

- Version: `v6.4.1`
- Previous stable: `v6.4.0`
- Rollback target: `v6.4.0`
- Rollback command: `sudo /bin/update --version v6.4.0`
- Promotion path: emergency stable patch from `main`, using the single-build
  release workflow after exact-SHA qualification
- Emergency reason: the `v6.4.0` server image cannot execute its embedded
  Unified Agent, which breaks Helm agent workloads and direct container-agent
  entrypoints on the current stable release
- Windows signing decision: the standing SignPath-unavailable policy publishes
  unsigned Windows Unified Agent binaries. They may display an Unknown
  Publisher warning while exact-SHA candidate binding, checksums, detached
  signatures, immutable-manifest verification, and published-digest
  verification remain mandatory
- Mobile decision: `no-mobile-impact`. No governed mobile-facing path changed
  from `v6.4.0`, so no companion build or public store rollout is required
