# Pulse v6.5.0-rc.1

This changelog describes the changes since `v6.4.1`, the latest published stable release. The cut source is main `a766cd41880cacc51abb671e427a1d8835a65586`. Changes merged after that cut, including the Proxmox peer-sensor opt-out, are not in this candidate. The candidate carries the complete `v6.4.2` change set from the unpublished packet and the subsequent selected main work.

## Monitoring and alerting

- Separate sites with reused short names no longer collapse into a single host or Docker record (#1753). Provider-scoped identity preserves Proxmox links and actions.
- PBS backup history follows the reported host target even when a connection uses an IP or alias (#1723). Backup status and alert age handling no longer pin a guest in Backup Running (#1815).
- Notification retries, persisted due alerts and grouped resolved notifications recover more faithfully across restarts (#1801, #2160). Alert state and delivery indicators avoid stale reversions.
- TrueNAS and Proxmox collection handle optional telemetry and resource identity more robustly. Disk partition accounting avoids inflated whole-device I/O (#2112).

## Installation and interface

- Windows Unified Agent auto-update no longer fails with HTTP 404 when looking up canonical executable and detached-signature assets (#1820).
- Copied install commands and diagnostic log capture have been corrected (#2123). Versioned installer and update metadata remain tied to the selected candidate.
- Large Availability fleets default to a compact view. Startup offers status and retry. Resource drawer tabs, History charts and alert controls retain their state more consistently (#2129, #1723).
- Keyboard, screen-reader and reduced-motion support improves across dialogs, navigation, charts and controls.

## Release Metadata

- Version: `v6.5.0-rc.1`
- Previous candidate: none on the 6.5 line
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA release candidate from `release/v6.5`; stable promotion requires this same candidate and a clean seven-day soak
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable. Windows may display an Unknown Publisher warning
- Mobile decision: `no-mobile-impact`; no companion mobile build or store rollout is required
