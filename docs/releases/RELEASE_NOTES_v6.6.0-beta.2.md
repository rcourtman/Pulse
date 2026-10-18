# Pulse v6.6.0-beta.2 Release Notes

This beta follows `v6.6.0-beta.1` on the way to stable `v6.6.0`. Stable `v6.5.0` remains the rollback target.

This beta carries 328 changes merged since stable `v6.5.0`.

## What's improved

- **Fixes and improvements since v6.5.0.** The changelog for this beta lists every change it carries.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
