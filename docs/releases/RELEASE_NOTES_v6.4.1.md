# Pulse v6.4.1 Release Notes

`v6.4.1` is a stable patch release for the Pulse v6 line. It follows stable
`v6.4.0` and restores the embedded container agent while correcting first-load
Proxmox details and several storage, persistence, and AI budget edge cases.

## What's improved

- **Container agents start correctly** - The embedded Unified Agent is executable in the server image, restoring Helm deployments with `agent.enabled=true` and direct agent entrypoints.
- **Proxmox details are correct on first load** - VM and LXC backup status, uptime, and related guest details no longer appear missing until a later live update arrives.
- **TrueNAS SMART evidence is parsed safely** - Spare-block reserve percentages accept supported native TrueNAS representations while malformed, fractional, or out-of-range values are ignored.
- **Credential changes fail safely** - Token creation, migration, runtime preparation, and first-run reset restore prior live state when durable persistence fails.
- **AI budgets cover current models** - Anthropic Sonnet 5, Fable 5, and Mythos 5 usage remains accurately priced and enforceable in Assistant and Patrol summaries.

## Before you upgrade

- Existing configurations remain valid and no manual data migration is required.
- Pulse Mobile remains compatible. This patch does not require a companion mobile release.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- The rollback target is stable `v6.4.0`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.0` and recreate the container.
