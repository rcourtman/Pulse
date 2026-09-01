# Pulse v6.4.3-rc.1 Release Notes

This preview makes large and mixed estates more dependable, with stronger resource identity, safer agent operations, faster recovery from slow starts, clearer workflows, and broader accessibility support.

## What's improved

- **Same-name systems stay separate** - Stronger host, Docker, Kubernetes, and Proxmox identity checks reduce incorrect merges and keep alerts, metrics, actions, and removal tied to the right machine.
- **Agent lifecycle operations are safer** - Linux installs protect tokens under sudo, offline recovery verifies signed installers, shared credentials survive partial removal, and uninstall waits for confirmed server removal.
- **Windows agent delivery is restored** - Unified Agent downloads now resolve canonical signed `.exe` assets and signature sidecars, preventing asset lookup failures during installation and updates.
- **Large Availability estates scan faster** - Estates with 20 or more checks open in fleet view, while table and fleet choices remain stable across refreshes and shareable URLs.
- **Slow starts are recoverable** - Pulse shows connection status and a retry after delayed app startup, while large alert histories catch up incrementally in the background after server restarts.
- **Alert workflows stay focused** - Patrol opens findings for the affected resource, snooze blocks duplicate submissions, incident recovery is clearer, and delivery warnings link to detailed activity.
- **Navigation is more accessible** - Menus, dialogs, charts, toasts, filters, thresholds, and compact controls improve keyboard, screen-reader, focus, mobile, and reduced-motion support.
- **Health APIs are consistent** - Resource APIs provide canonical health verdicts and a bounded fleet summary, while missing or stale telemetry can no longer appear healthy.
- **Disk I/O totals are more accurate** - Partitions of NVMe, MMC, MD, persistent-memory, and ZFS volume devices no longer inflate whole-device traffic.
- **Privileged actions fail more safely** - Linux package and Proxmox operations run in bounded units, abandoned requests are cancelled, and uncertain outcomes are not reported as successful.
- **Oversized responses are contained** - Pulse applies response limits across agents, Proxmox, discovery, updates, remote configuration, AI providers, and other integrations to reduce memory pressure.

## Before you upgrade

- This candidate carries every change from the `v6.4.2` packet, which was tagged on 2026-08-31 but never published. If you run `v6.4.1`, read the `v6.4.2` notes for the administrator-boundary changes as well.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading so an intended administrator retains access.
- If the least-privilege agent monitors rootless Docker or Podman, ensure the collector account exposes exactly one local, collector-owned runtime socket. Ambiguous or invalid sockets may fall back to summary-only monitoring.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release.
