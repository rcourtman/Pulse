# Pulse v6.4.5-rc.1 Release Notes

This release candidate gathers the accumulated monitoring, notification, agent and installation reliability repairs since stable `v6.4.1` into one build intended to become the next stable release. It carries every change from the `v6.4.2` packet that was never published, together with the corrections already exercised in `v6.4.5-beta.1`. Preview users can opt in to confirm the fixes before stable promotion. Keep stable `v6.4.1` available for rollback.

## What's improved

- **Same-name systems stay separate** - Stronger host, Docker, Kubernetes and Proxmox identity checks keep alerts, metrics, actions and removal tied to the right machine even when short names or shared tokens repeat (#1753, #1930).
- **Windows agent delivery is restored** - Unified Agent downloads resolve canonical signed `.exe` assets and their signature sidecars, so installation and auto-update no longer fail with HTTP 404 (#1820, #2125).
- **Large Availability estates scan faster** - Estates with 20 or more checks open in fleet view, and the chosen table or fleet presentation stays stable across refreshes and shareable URLs.
- **Slow starts are recoverable** - A delayed or stalled startup now shows connection status and a retry, and large alert histories catch up incrementally instead of blocking the server (#2129).
- **Disk I/O totals are more accurate** - Partition accounting no longer inflates whole-device traffic, and endurance counters stop raising false disk-wear warnings (#2112).
- **Backup alerts stay truthful** - A guest backed up locally on Proxmox and copied to a PBS is counted once, and a host that stops reporting no longer raises repeated critical backup-age alerts (#1741, #1721).
- **Alert thresholds lists stay complete** - The Alert Thresholds instance list keeps every host row visible while scrolling instead of dropping rows as the window estimate drifts (#2130).
- **Alerts overview counts and layout align** - Overview statistics and text alignment match the underlying incidents after retries and refreshes (#2119).
- **Alert checkpoint writes are bounded** - Byte-identical pending-intent checkpoints and oversized alert event-log snapshots are no longer rewritten on every cycle, reducing write volume on flash storage (#1966).
- **Guest panels order predictably** - The workload drawer lists Filesystems before Tags, matching the rest of the interface (#2121).
- **TrueNAS telemetry is accurate** - AMD temperature aggregates are rejected instead of inflated, CORE memory is read through the legacy REST path, and idle sessions follow a bounded poll cadence (#2122, #2077, #1893).
- **vSphere enrichment recovers** - VMware requests send the expected JSON type names, probe the current 8.0.3 release shape, and surface API fault detail instead of failing with HTTP 500 (#2070).
- **Container update checks match the running image** - All local RepoDigests are treated as the current image, so PostgreSQL and other containers stop reporting a false update (#2110).
- **Failed updates leave no stale state** - A failed staging attempt discards its configuration backup, and a completed update clears its trap so success is not reported as failure (#2127, #2128).
- **FreeBSD and pfSense installs verify checksums** - The installer verifies downloaded agent checksums without relying on GNU coreutils (#2123).
- **Custom sensor warnings are delivered** - Health-assessment escalations for custom sensors now send their warning notifications (#1801).
- **Derived agent identity heals safely** - Re-enrolling a forked identity honours the removal block, and the event-log volume stays bounded through the heal (#2113, #1586).
- **Host continuity survives upgrades** - v5 to v6 upgrade paths preserve live host state instead of losing continuity (#1913).
- **AI Patrol prompts are cached** - The stable system prompt prefix is cached so repeated Patrol runs cost less (#2118).
- **Ollama and security settings persist** - Ollama Basic Auth configuration, the configured-admin authorizer and security-setup preferences survive save and restart.
- **Verify SSL choices stick** - A node's "Verify SSL Certificate" setting is preserved through agent re-registration and PVE instance consolidation, so a deliberately disabled or enabled choice is no longer silently overwritten by the automatic paths (#2140).
- **Metric samples stay consistent** - Replayed unified metric samples are dropped, and metrics store shutdown, rollups and upgrades are hardened.

## Known issues

- Installed confirmation of Proxmox and PBS History rendering on API-only nodes, agent-linked nodes and standalone PBS systems remains outstanding.
- Aggregate alert-engine write volume and old duplicate incidents reported in #1966 are reduced but not fully resolved. Monitor write activity on flash storage.
- Some v5 to v6 deployment-specific upgrade problems remain unresolved (#1913).
- TrueNAS appliance acceptance and notification-provider acceptance remain incomplete.
- Notification delivery still depends on destination configuration and provider responses. Do not delete alert or queue files as a workaround.

## Before you upgrade

- This candidate carries every change from the `v6.4.2` packet, which was tagged but never published. If you run `v6.4.1`, read the `v6.4.2` notes for the administrator-boundary changes as well.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in `admin` role before upgrading so an intended administrator retains access.
- Back up the complete Pulse data directory and configuration before opting into the preview channel, and keep the backup until health and data continuity are verified.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
