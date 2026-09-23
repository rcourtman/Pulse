# Pulse v6.5.0-rc.1 Release Notes

This first 6.5 release candidate gathers the work selected when `release/v6.5` was cut from main. It is a preview build, not a stable update. The planned stable release will use this exact candidate only if qualification and seven days of observation are clean. Stable users remain on `v6.4.1` until then.

## What's improved

- **Same-name systems stay separate** - Host, Docker, Kubernetes and Proxmox identities are kept in their own provider scope, reducing mixed metrics, alerts and actions (#1753, #1930).
- **Windows agent delivery is restored** - Agent downloads and updates resolve the release executable and its detached signature assets rather than returning a missing-file error (#1820).
- **Large Availability estates scan faster** - Fleets with many checks open in a compact view that retains the selected presentation across refreshes.
- **Slow starts are recoverable** - Startup status and retry controls are visible, while alert-history replay catches up without repeatedly blocking server startup (#2129).
- **Disk I/O totals are more accurate** - Whole-device totals no longer double-count common numbered partition layouts (#2112).
- **Alerts remain dependable through restarts** - Notification recovery, retry handling and resolved-alert grouping reduce missed or duplicated delivery (#1801, #2160).
- **Backup and storage history is more reliable** - PBS backup age and host correlation, TrueNAS telemetry, and disk identity handling preserve the expected resource during refreshes (#2136, #1723, #2076).
- **Agent and installer diagnostics are clearer** - Copied install commands, signed-file lookup and agent log collection have been corrected where supported (#2123, #1820).
- **Security and accessibility are strengthened** - Administrative access, bounded integration responses, keyboard navigation and dependency updates improve everyday operation.

## Before you upgrade

- This candidate carries every change from the `v6.4.2` packet, which was tagged but never published. It also includes the intervening work selected on the 6.5 line. Read the 6.4 release notes for administrator-boundary changes.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify published checksums and detached signatures.
- This candidate does not require a companion mobile release. Mobile store builds and public mobile onboarding are unchanged.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. Back up data before testing this preview.
