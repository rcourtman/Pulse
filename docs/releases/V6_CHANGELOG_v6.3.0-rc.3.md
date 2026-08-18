# Pulse v6.3.0-rc.3

_This changelog describes the changes since `v6.3.0-rc.2` and carries forward
the complete cumulative 6.3 packet since stable `v6.2.1`. `v6.3.0-rc.3` is a
prerelease and rolls back to stable `v6.2.1`._

## Added

- Durable, scoped Patrol objectives and validated read-only observer missions.
- A decision-first Patrol inbox with ranked review, progress, navigation,
  Protection, Activity, and recent-work receipt interfaces.
- A primary Actions workspace for approvals, governed plans, and action records.
- Canonical platform-admission facts on unified resources.
- Typed Unified Agent action preflight for supported host and Docker operations.
- Stable pre-mutation refusal codes and fleet telemetry buckets for target
  changes, prerequisites, contract failures, capability limits, policy, and
  stale plans.
- Production security guidance and a reusable security-review evidence packet.
- System-scoped alerts, so Pulse can report a fault in itself rather than only
  in a monitored resource. Broken notification delivery is the first, raised as
  an ordinary alert so it reaches the alert list and navigation badge.

## Changed

- Patrol is organized around operator outcomes, quiet background observation,
  scoped evidence, typed proposals, governed execution, and verified receipts.
- Patrol reviews one decision at a time and preserves the selected context
  across its inbox-and-context desktop workspace and mobile master/detail flow.
- Assistant explains the selected operational item while Actions owns approval
  and action-record review as a separate primary workspace.
- Finding and investigation lifecycles are idempotent, scope-aware, and bounded
  across continuation, provider-failure, restart, and truncation recovery.
- API responses support gzip, the client WebSocket ceiling is 32 MiB, and
  large-estate polling and resource correlation use bounded or indexed paths.
- Authenticated startup avoids the legacy full-state pull, starts canonical
  resource fetching without a WebSocket hydration dependency, and loads later
  unified-resource pages concurrently.
- Runtime version identity is bound to the packaged binary rather than a stale
  source-tree value.
- Buffered subscription CLI turns cannot extend a caller-owned idle deadline
  while a canceled descendant keeps an inherited output pipe open.

## Fixed

- Patrol rejects findings and causal proposals whose identity, evidence,
  resource scope, tool authority, or conclusion is incoherent.
- Full-mode activation and in-process provider restarts preserve Patrol runtime
  wiring and controls.
- Platform-admission state remains current across resource projections, tenant
  changes, reconnects, and shell navigation.
- Alert monitoring menus no longer clip, threshold overrides use canonical
  registry identity, Agent Doctor names the credential it judged, and agents
  surface server-side identity overrides.
- Docker health findings retain health-check dependencies and selected workload
  scope through investigation and remediation.
- ZFS storage attachment, vSphere backup presentation, thermal history, cluster
  member addressing, and discovery-analysis timeout handling now use canonical
  runtime facts.
- Informational and bodyless HTTP responses remain valid when gzip is enabled.
- The notifications surface states when alert delivery is paused, including
  that a passing test send does not prove live alerts are getting through.
  Configured destinations were previously silent with no indication.
- Degraded notification delivery is reported on the alerts overview instead of
  only on the destinations configuration tab.
- The flapping cooldown now suppresses for its configured duration. It was
  recorded and never read, so suppression ended as soon as the sliding window
  drained and a resource oscillating just under the threshold was never damped.
- Mobile tables keep narrow values readable, preserve compact replication
  values, and use a consistent density across platform surfaces.

## Security

- Observer missions cannot mutate infrastructure and cannot bypass the normal
  action-governance path.
- Action preflight carries only bounded feasibility evidence and stable reason
  codes across the agent boundary.
- Patrol tool access is derived from the requested resource scope and rejects
  unadvertised calls.

## Release Metadata

- Version: `v6.3.0-rc.3`
- Previous release: `v6.2.1`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.3.0` restores mandatory SignPath signing
- Mobile decision: `no-mobile-impact`; changes since `v6.3.0-rc.2` preserve the
  existing mobile, Relay, onboarding, and mobile-facing API contracts, and no
  companion upload or public store rollout is required
