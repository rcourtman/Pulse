# Pulse v6.0.1

_This changelog describes the stable `v6.0.1` patch release compared with
`v6.0.0`._

## Fixed

- v5.1.x to v6 upgrade recovery now handles missing or stale legacy agent
  update state.
- Workloads status filters now survive the return path back to platform pages,
  including Proxmox Overview.
- Host memory pressure no longer treats Linux cache as unavailable memory.
- Router system settings are reapplied consistently after monitor reload paths,
  including mock-setting and auto-registration reloads.
- PBS backup snapshot polling now uses bounded workers for large backup sets.
- Stable release publication now uses the single post-GA demo target rather
  than keeping a separate v6 preview demo path alive.

## Release Metadata

- Version: `v6.0.1`
- Rollback target: `v6.0.0`
- Promotion path: stable patch hotfix from `pulse/v6-release`
