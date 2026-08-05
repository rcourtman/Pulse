# Pulse v6.2.0-rc.9

_This changelog describes the changes since `v6.2.0-rc.8`.
`v6.2.0-rc.9` remains a prerelease and rolls back to stable `v6.1.2`._

## Improved

- Shared per-generation resource lists across API requests while preserving
  request-local decoration and canonical presentation behavior.
- Cached mock unified views against fixture versions instead of rebuilding
  registries for every read between ticks.
- Released per-tenant resource-store handles during offboarding, replacement,
  and shutdown.
- Let accepted guest metadata writes complete before monitor shutdown.

## Fixed

- Preserved configured ntfy title, priority, and tags on live alert delivery.
- Kept provider incidents and acknowledged state intact across notification
  configuration saves.
- Honored disabled alert grouping and flushed queued alerts individually.
- Included every grouped alert in provider notification payloads.
- Replaced two non-discriminating audit telemetry fields with meaningful,
  privacy-preserving signals.

## Release Metadata

- Version: `v6.2.0-rc.9`
- Previous candidate: `v6.2.0-rc.8`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile 1.0.0 iOS
  build 12 is distributed through the TestFlight public beta link and Android
  versionCode 9 remains on Play open testing, both using runtime version 2. The
  changes since RC8 do not alter mobile relay payloads, pairing, approvals,
  authentication, or onboarding contracts. No public store rollout is part of
  this candidate
