# Pulse v6.2.0-rc.10

_This changelog describes the changes since `v6.2.0-rc.9`.
`v6.2.0-rc.10` remains a prerelease and rolls back to stable `v6.1.2`._

## Added

- Typed prerelease containment records for historically reachable credentials,
  backed by provider-observed closure and replacement or retirement evidence.
- Fail-closed origin validation for hosted sign-in, diagnostics, and copied
  installer command surfaces.
- Canonical worktree claim helpers and conflict-aware release-control routing
  for concurrent maintainers.

## Improved

- Viewer-safe Settings and workload navigation, including responsive panels
  and role-aware update-status polling.
- Oversized WebSocket snapshot recovery through authenticated REST resync while
  retaining raw-state delta correctness.
- Unified Agent update convergence, PBS node identity reuse, and merged-resource
  alert intent handling.
- Release artifact staging, customer promotion convergence, frontend dependency
  audits, historical secret scanning, and provider-MSP evaluation delivery.
- Self-hosted commercial opt-in posture and plan-selection upgrade routing.

## Fixed

- Blocked untrusted request-derived hosts and schemes from installer, hosted
  diagnostics, and magic-link responses.
- Enforced SSH host-key verification during legacy sensor-proxy cleanup.
- Removed inaccessible admin routes and background requests from viewer
  sessions while preserving authorized health summaries.
- Prevented oversized WebSocket snapshots from becoming a corrupt recovery
  baseline and applied later deltas to canonical raw server state.
- Corrected repeated PBS node-name reads, retry classification, and responsive
  Settings clipping.
- Removed the reverted proactive commercial prompt, telemetry, and checkout
  attribution cluster from the final candidate.

## Release Metadata

- Version: `v6.2.0-rc.10`
- Previous candidate: `v6.2.0-rc.9`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Code-backed validation-risk head:
  `5ff0855882cdbcfc9d4c8f8d87a1ffa3972db818` (61 commits and 226 changed files
  since `v6.2.0-rc.9`)
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0 iOS
  build 12 is distributed through the TestFlight public beta link and Android
  versionCode 9 remains on Play open testing, both using runtime version 2. The
  checked-in mobile compatibility proof passes for this server revision. No
  public store rollout is part of this candidate
