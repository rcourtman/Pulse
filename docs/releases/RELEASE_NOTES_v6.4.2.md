# Pulse v6.4.2 Release Notes

`v6.4.2` is a stable patch release for the Pulse v6 line. It follows stable
`v6.4.1` and restores explicit administrator boundaries for infrastructure
actions and SSO-only deployments.

## What's improved

- **Infrastructure actions honor role boundaries** - Browser and proxy users must now be administrators or hold an explicit action permission before they can plan, approve, view, or execute infrastructure actions.
- **SSO access no longer implies administrator access** - An authenticated SSO user now needs an effective RBAC `admin` grant for administrator routes. Unassigned, `operator`, and `viewer` users remain non-administrative even when no local administrator exists.
- **SAML allowlists fail closed** - A configured domain or email allowlist now rejects a SAML assertion that omits the email claim instead of bypassing the allowlist.
- **Security-sensitive setup requests are bounded** - Bootstrap, setup, repair, and recovery endpoints now reject oversized JSON request bodies before decoding them.
- **PBS backup state returns to idle reliably** - Failed backups, interrupted snapshots, and completed PBS-to-PBS sync copies no longer leave guests stuck in a Backup Running state. Incomplete artifacts remain visible as failed rather than appearing recoverable.
- **Delivery warnings can be resolved from Overview** - Retained notification failures can now be retried or dismissed directly from the Alerts overview while delivery history remains available.
- **Assistant command help behaves as a complete dialog** - Command help now traps focus, isolates the background, closes consistently, returns focus to its trigger, and uses the same responsive dialog behavior as the rest of Pulse.
- **Agent URL migration guidance is now included** - The built-in migration guide explains how to rerun the agent installer with the new server URL after moving or renaming a Pulse server.
- **Preview releases now distinguish beta and RC maturity** - Update settings explain that beta builds are for user testing, while release candidates may become stable without product changes.
- **Systemd journal severity is preserved** - Pulse service logs now carry native journal priorities so `journalctl -p` and downstream forwarding can filter by severity without changing file, terminal, or live-log output.
- **Same-name Proxmox estates stay distinct after restart** - Durable identity recovery now uses qualified provider endpoints and fails closed on reused short names or IP addresses, preventing one estate from absorbing another during provider-first startup.
- **Partial Proxmox cluster coverage is visible** - Infrastructure settings now names uncovered nodes, explains that each needs its own agent for host telemetry, and offers node-level install actions instead of treating partial coverage as complete.
- **Agent Doctor reports privilege-helper degradation** - Failures in the typed local privilege helper now appear as a bounded diagnostic reason while affected telemetry is omitted without widening the collector's privileges.
- **Release workflow checkouts retain their security boundary** - Build, qualification, publication, recovery, and deployment workflows now use the protected checkout baseline and reject privileged fork checkout paths.

## Before you upgrade

- Upgrade promptly when Pulse has organization members, proxy-authenticated users, or SSO users who should not administer the instance or control infrastructure.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading so an intended administrator retains access.
- Existing configured local administrators, explicitly authorized RBAC roles, and action-scoped API tokens remain supported.
- Pulse Mobile remains compatible. This patch does not require a companion mobile release.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
