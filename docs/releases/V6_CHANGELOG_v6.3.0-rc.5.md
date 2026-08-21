# Pulse v6.3.0-rc.5

_This changelog describes the changes since `v6.3.0-rc.4`. The candidate carries
forward the complete 6.3 product packet and rolls back to stable `v6.2.1`._

## Changed

- Durable, scoped Patrol objectives and validated read-only observer missions
  remain part of the cumulative 6.3 candidate.
- A primary Actions workspace for approvals, governed plans, and action records
  remains part of the cumulative 6.3 candidate.
- Typed Unified Agent action preflight for supported host and Docker operations
  remains part of the cumulative 6.3 candidate.
- Stable pre-mutation refusal codes and fleet telemetry buckets continue to
  classify target changes, prerequisites, capability limits, and stale plans.
- Estate summaries, status facets, and canonical search on the primary
  infrastructure platform pages remain part of the cumulative candidate.
- A seven-day delivery log for real alert notification attempts remains part of
  the cumulative candidate.
- A supported least-privilege Unified Agent installation profile remains part
  of the cumulative candidate.
- Dedicated PVE runners compile the credential-free public and paid release
  payloads with persistent caches.
- Release qualification overlaps independent archive, backend, frontend,
  mobile, container, Helm, and paid-runtime checks.
- Exact-candidate container payloads are promoted directly into public and
  private images without recompiling binaries after qualification.
- Helm and paid-runtime convergence consume immutable artifacts from the source
  release run and parallelize independent verification.
- API alert delivery, configuration, enrollment-token, command-binding,
  request-context, and HTTP-scope logic now live behind explicit production
  package boundaries while compatibility adapters preserve existing routes.

## Fixed

- Release archive validation scans platform archives concurrently and avoids
  repeated decompression of the same archive.
- Candidate qualification is no longer suppressed by intentionally skipped
  signing jobs.
- Architecture-bound server signatures and Windows container symlink aliases
  are validated according to their actual release contract.
- Release activation and private child-workflow polling no longer add long
  fixed delays after evidence becomes available.
- Deterministic API test batches now enforce an encoded regex byte ceiling, so
  the low-memory one-shard path remains executable without reducing coverage.

## Security

- PVE workers receive no release mutation or signing credentials.
- Publication still requires exact-source identity, immutable manifests,
  signatures, public/private artifact integrity, installer smoke, and final
  convergence verification.
- Tenant scope and agent install/command binding remain covered at the new API
  package boundaries.

## Release Metadata

- Version: `v6.3.0-rc.5`
- Previous release: `v6.3.0-rc.4`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.3.0` restores mandatory SignPath signing
- Mobile decision: `no-mobile-impact`; changes since `v6.3.0-rc.4` preserve the
  existing mobile, Relay, onboarding, and mobile-facing API contracts, and no
  companion upload or public store rollout is required
