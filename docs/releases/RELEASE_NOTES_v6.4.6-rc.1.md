# Pulse v6.4.6-rc.1 Release Notes

This release candidate opens the `v6.4.6` candidate line after stable `v6.4.5`, which remains the rollback target.

This maintenance preview keeps Backup Server History attached to the right host and makes updates and startup checks more reliable.

## What's improved

- **Backup Server History stays with its host.** History no longer carries readings from a replaced or ambiguously identified host into another host's charts.
- **Update progress keeps moving.** The update window follows an update through completion even when the progress stream goes quiet, and waits for the updated server to be ready before offering a reload.
- **Startup checks handle both address families.** Local service checks try IPv4 and IPv6 within a bounded timeout, avoiding misleading failures on single-family systems.
- **More dependable metrics storage.** Updated SQLite dependencies replace the affected low-level memory-copy implementation.
- **Automatic updates stay opt-in.** New installations enable unattended updates only after an affirmative choice.

## Before you upgrade

- Back up your Pulse configuration and data before trying this preview. Keep production installations on the stable channel unless you intend to test a preview.
- If you sign in through SSO, map at least one trusted IdP group to the built-in `admin` role so an administrator can still manage settings.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.4.5`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.4.5` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.5` and recreate the container.
