# Pulse v6.1.0-rc.4

_This changelog describes the changes since `v6.1.0-rc.3`.
`v6.1.0-rc.4` remains a prerelease and rolls back to stable `v6.0.5`._

## Added

- Canonical Operational Trust lifecycle evidence and notification linkage.
- Protection posture and availability facets on unified resources.
- A Patrol attention workbench for protection, availability, alert, and action
  work.
- Governed Docker restart review, dispatch, durable receipts, and independent
  verification.
- Report-only Unified Agent observer destinations with per-destination
  identity, credentials, retry health, and transport policy.
- Mobile typed-action proposals behind mandatory approval.
- Branch-coverage proof for critical presentation, API client, trust, recovery,
  model, cloud-control-plane, and agent helper paths.

## Changed

- Alert detection, activation, notification, and resolution use one canonical
  state boundary.
- Protection and availability evidence is projected through the shared resource
  model and surfaced in the relevant monitoring and Patrol contexts.
- Assistant width is user-adjustable and its composer remains visible.
- Responsive workload columns are keyed from the canonical wide breakpoint.
- Container updates use one review step over the existing governed lifecycle.
- Backup view controls and table alignment use the current shared presentation
  contracts.
- Pulse Mobile candidate runtime 1 serves OTA update group
  `9b78b108-2586-4b0f-91d3-afbed19b49b3` from commit
  `fddb091e683e84902de6aac680b08a47862b738b` to iOS build 10 and Android
  versionCode 8.

## Fixed

- Canonical alert evaluation guards resolved-state maps during concurrent
  transitions.
- High-percentage thresholds remain valid when the critical cap collides with
  another threshold boundary.
- PBS datastore identities and override-key formats are deduplicated.
- Directory storages whose names begin with `pbs-` retain vzdump backup
  discovery.
- Stale Docker host records are superseded after agent re-enrollment.
- Remember-me browser sessions persist across tab closure.
- Wide workload and backup-table layouts remain coherent at their responsive
  boundaries.
- Current cookie-session, onboarding, Assistant, storage, and commercial
  integration journeys run against the evolved product surfaces.
- Pulse Mobile conversation loading, keyboard avoidance, source rename,
  large-text header controls, and deletion flows are corrected.

## Security

- Every non-loopback plaintext observer destination requires explicit
  destination-local consent; it cannot inherit the primary server override.
- Observer acknowledgements cannot replace authoritative agent configuration.
- Observer reporting does not grant command execution or control authority.
- Governed actions remain bound to their reviewed plan, policy provenance,
  action identity, receipt, and verification result.

## Release Metadata

- Version: `v6.1.0-rc.4`
- Previous candidate: `v6.1.0-rc.3`
- Rollback target: `v6.0.5`
- Rollback command: `./scripts/install.sh --version v6.0.5`
- Promotion path: exact-SHA single-build release candidate from `main`
- Mobile companion candidates: iOS build 10 and Android versionCode 8 on
  TestFlight and Google Play internal testing, updated through the candidate
  OTA channel at runtime version 1
