# Pulse v6.4.2 Release Notes

`v6.4.2` is a stable patch release for the Pulse v6 line. This security patch follows stable
`v6.4.1` and restores the intended permission boundary for infrastructure
actions.

## What's improved

- **Infrastructure actions honor role boundaries** - Browser and proxy users must now be administrators or hold an explicit action permission before they can plan, approve, view, or execute infrastructure actions.

## Before you upgrade

- Upgrade promptly when Pulse has organization members or proxy-authenticated users who should not control infrastructure.
- Existing administrator sessions, explicitly authorized RBAC roles, and action-scoped API tokens remain supported.
- Pulse Mobile remains compatible. This patch does not require a companion mobile release.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
