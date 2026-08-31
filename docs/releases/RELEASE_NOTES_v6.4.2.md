# Pulse v6.4.2 Release Notes

`v6.4.2` is a stable patch release for the Pulse v6 line. It follows stable
`v6.4.1` and restores explicit administrator boundaries for infrastructure
actions and SSO-only deployments.

## What's improved

- **Infrastructure actions honor role boundaries** - Browser and proxy users must now be administrators or hold an explicit action permission before they can plan, approve, view, or execute infrastructure actions.
- **SSO access no longer implies administrator access** - An authenticated SSO user now needs an effective RBAC `admin` grant for administrator routes. Unassigned, `operator`, and `viewer` users remain non-administrative even when no local administrator exists.
- **SAML allowlists fail closed** - A configured domain or email allowlist now rejects a SAML assertion that omits the email claim instead of bypassing the allowlist.

## Before you upgrade

- Upgrade promptly when Pulse has organization members, proxy-authenticated users, or SSO users who should not administer the instance or control infrastructure.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading so an intended administrator retains access.
- Existing configured local administrators, explicitly authorized RBAC roles, and action-scoped API tokens remain supported.
- Pulse Mobile remains compatible. This patch does not require a companion mobile release.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
