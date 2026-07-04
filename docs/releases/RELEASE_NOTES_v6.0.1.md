# Pulse v6.0.1 Release Notes

`v6.0.1` is a stable patch release for the Pulse v6 line. It follows the
`v6.0.0` GA release and focuses on upgrade recovery, monitoring correctness,
large backup-set behaviour, and release-day publication cleanup.

## Fixes

- Fixed v5.1.x to v6 upgrade recovery when legacy agent update state is missing
  or stale.
- Preserved scoped Workloads status filters when returning to platform pages
  such as Proxmox Overview.
- Corrected host memory pressure so Linux cache is not counted as unavailable
  memory.
- Reapplied router system settings after every monitor reload path, including
  mock-setting and auto-registration reloads.
- Bounded PBS backup snapshot polling workers for large backup sets to reduce
  release-line memory pressure.
- Retired the separate v6 preview demo target now that v6 is the stable release
  line.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.1`. The rollback target for
this patch release is `v6.0.0`.
