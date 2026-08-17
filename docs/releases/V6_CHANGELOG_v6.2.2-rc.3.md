# Pulse v6.2.2-rc.3

_This changelog describes the changes since `v6.2.2-rc.2`.
`v6.2.2-rc.3` remains a prerelease and rolls back to stable `v6.2.1`._

## Added

- Durable, scoped Patrol objectives and validated read-only observer missions.
- Patrol objective, recent-work receipt, and expanded attention interfaces.
- Typed Unified Agent action preflight for supported host and Docker operations.
- Stable pre-mutation refusal codes and fleet telemetry buckets for target
  changes, prerequisites, contract failures, capability limits, policy, and
  stale plans.
- Production security guidance and a reusable security-review evidence packet.

## Changed

- Patrol is organized around operator outcomes, quiet background observation,
  scoped evidence, typed proposals, governed execution, and verified receipts.
- Finding and investigation lifecycles are idempotent, scope-aware, and bounded
  across continuation, provider-failure, restart, and truncation recovery.
- API responses support gzip, the client WebSocket ceiling is 32 MiB, and
  large-estate polling and resource correlation use bounded or indexed paths.
- Runtime version identity is bound to the packaged binary rather than a stale
  source-tree value.
- Buffered subscription CLI turns cannot extend a caller-owned idle deadline
  while a canceled descendant keeps an inherited output pipe open.

## Fixed

- Patrol rejects findings and causal proposals whose identity, evidence,
  resource scope, tool authority, or conclusion is incoherent.
- Full-mode activation and in-process provider restarts preserve Patrol runtime
  wiring and controls.
- Docker health findings retain health-check dependencies and selected workload
  scope through investigation and remediation.
- ZFS storage attachment, vSphere backup presentation, thermal history, cluster
  member addressing, and discovery-analysis timeout handling now use canonical
  runtime facts.
- Informational and bodyless HTTP responses remain valid when gzip is enabled.

## Security

- Observer missions cannot mutate infrastructure and cannot bypass the normal
  action-governance path.
- Action preflight carries only bounded feasibility evidence and stable reason
  codes across the agent boundary.
- Patrol tool access is derived from the requested resource scope and rejects
  unadvertised calls.

## Release Metadata

- Version: `v6.2.2-rc.3`
- Previous candidate: `v6.2.2-rc.2`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.2.2` restores mandatory SignPath signing
- Mobile decision: `no-mobile-impact`; changes since `v6.2.2-rc.2` preserve the
  existing mobile, Relay, onboarding, and mobile-facing API contracts, and no
  companion upload or public store rollout is required
