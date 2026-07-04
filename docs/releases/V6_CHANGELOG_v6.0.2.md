# Pulse v6.0.2

_This changelog describes the stable `v6.0.2` patch release compared with
`v6.0.1`._

## Fixed

- Legacy agent updates now recover connection details from the running agent
  process when the persisted v6 state file is missing or stale.
- Stable patch release validation now supports the governed hotfix path without
  requiring a fabricated same-version RC tag.
- Demo update verification now restores the fixture entitlement before checking
  mock-mode access after release deployment.
- Helm chart release metadata now tracks the active stable patch version.

## Release Metadata

- Version: `v6.0.2`
- Rollback target: `v6.0.1`
- Promotion path: stable patch hotfix from `pulse/v6-release`
