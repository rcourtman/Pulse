# Pulse v6.3.0-rc.6

_This changelog describes the changes since `v6.3.0-rc.5`. The candidate
carries forward the complete 6.3 product packet and rolls back to stable
`v6.2.1`._

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
- Chart handling and resource queries are production packages with independent
  test scheduling; root retains authorization, route composition, actions, and
  cross-domain integration ownership.
- Backend release admission requires the measured memory floor for two API
  race shards after a bounded wait for credential-free sibling compilers.
- Exact-version public Docker staging overlaps qualification, and server and
  provider control-plane products publish as parallel matrix legs.
- Private Pro packaging uses a reduced, compressed compiled payload and avoids
  unconditional hosted-runner cleanup when the runner already has safe free
  space.
- Paid-runtime public-boundary proofs reuse installed Chrome, pull the public
  image once, and run the Docker and direct-binary paths concurrently.

## Fixed

- Activation-only recovery derives readiness from the canonical DAG join.
- Helm Pages convergence binds commands to the release repository from nested
  worktrees.
- Every private-license proof establishes the tailnet connection in the job
  that performs the call.
- Insufficient two-shard PVE capacity now fails at admission instead of
  entering a release path already known to exceed the target duration.

## Security

- PVE workers receive no release mutation or signing credentials.
- Publication still requires exact-source identity, immutable manifests,
  signatures, public/private artifact integrity, installer smoke, and final
  convergence verification.
- Parallel Docker products independently revalidate the exact source checkout
  and candidate manifest before build, push, provenance, SBOM, and attestation.
- Tenant scope and root-owned mutation policy remain covered at the new chart
  and resource service boundaries.

## Release Metadata

- Version: `v6.3.0-rc.6`
- Previous release: `v6.3.0-rc.5`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.3.0` restores mandatory SignPath signing
- Mobile decision: `no-mobile-impact`; changes since `v6.3.0-rc.5` preserve the
  existing mobile, Relay, onboarding, and mobile-facing API contracts, and no
  companion upload or public store rollout is required
