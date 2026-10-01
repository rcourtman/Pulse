# Pulse v6.4.5

The first stable release since v6.4.1. It brings everyone the fixes that have been in the v6.4.5 previews over the past month: quieter and more accurate alerts, fewer disk writes, working updates and Windows agent installs, and better PBS, TrueNAS, vSphere and Docker support.

## Highlights

- **Far fewer false disk-wear alerts.** Endurance counters no longer raise false disk-wear warnings, disk I/O totals are no longer inflated, and a recovered or excluded disk stops re-alerting (#2112).
- **Stable updates work again.** The updater reads the full release information and offers the right build, including on ARMv6 and ARMv7 (#2282).
- **Windows agent installs and auto-updates work again.** Downloads no longer fail with HTTP 404 (#1820, #2125).
- **Machines with the same name stay separate.** Hosts, Docker, Kubernetes and Proxmox systems that share a short name or token no longer get their alerts, metrics or actions mixed up (#1753, #1930).
- **Less writing to SSD and flash storage.** Pulse no longer rewrites unchanged alert state on every cycle (#1966).
- **Large alert histories no longer freeze Pulse.** They load gradually instead of blocking the server, and a slow start now shows its status with a retry button (#2129, #2146).

## Alerts and notifications

- A problem that continues after its parent recovers now gets its own notification, and grouped alerts no longer repeat (#2336).
- Many alerts recovering at once are grouped into one notification instead of flooding your notification queue (#2160).
- Notifications due at startup wait until your webhook, email and Apprise settings have loaded, so they are no longer dropped (#2160).
- A warning that becomes critical is now delivered, escalations respect current acknowledgements and snoozes, and an automatic acknowledgement survives the alert firing again (#1801, #2173).
- Removed or suppressed resources stop creating alerts (#2237).
- The Alerts overview counts now match the incidents shown, alert-card footers line up, and the Alert Thresholds list keeps every host visible while scrolling (#2119, #2130).

## Disks and storage

- The same USB disk seen by both Proxmox and the agent can now be shown once when one source lacks a stable ID, while ambiguous or conflicting disks stay separate (#2076).
- VMs and containers show the physical disks, SMART and RAID details reported by their linked agent (#2263).
- Long mount paths in the Filesystems panel are shown in full instead of being cut to identical prefixes (#2121).

## Proxmox, PBS and backups

- PBS host and config backups no longer trigger repeated guest backup-age alerts, and a guest backed up locally and copied to PBS is counted once (#1741, #1721, #2136).
- PBS History keeps each host and datastore separate across refreshes (#1723).
- PBS datastore health uses your warning and critical thresholds (#1448).
- Proxmox LXC memory uses the reading from an online Pulse agent inside the container, which excludes the page cache, when the agent's total matches the container's limit (#2148).
- The resource drawer stays on the tab you picked during live refreshes (#1723).
- Your "Verify SSL Certificate" choice for a node or PBS is kept through agent re-registration and cluster changes (#2140).

## TrueNAS, vSphere and Docker

- TrueNAS: AMD temperatures are no longer inflated, TrueNAS CORE memory is read correctly, CPU and memory charts keep their data, and idle sessions poll less often (#2122, #2077, #1893).
- vSphere: enrichment works again with vSphere 8.0.3 and shows the actual API error instead of HTTP 500 (#2070).
- Docker: containers such as PostgreSQL no longer show a false "update available" (#2110).

## Install, updates and agents

- A failed update no longer leaves stale backups behind, and a successful one is no longer reported as failed (#2127, #2128).
- FreeBSD and pfSense agent installs verify checksums without GNU tools, and the copied install command keeps its line breaks (#2123).
- Agents that re-enroll with a changed identity heal cleanly without a burst of events, and a removed agent stays removed (#2113, #1586).
- Upgrades from v5 keep your existing hosts (#1913).
- The upgrade guide warns that on some Proxmox community-script containers `/bin/update` is the community-scripts updater, and shows the signed Pulse installer instead (#2129).
- Previews stuck on the old broken self-updater may need one manual, version-pinned update (#2282).

## Pulse Pro, AI and hosted

- AI Patrol caches its system prompt, so repeated runs cost less (#2118), and a saved Patrol objective whose watcher was lost or rejected is restored instead of going unchecked (#2147).
- AI knowledge saves no longer conflict when made quickly one after another.
- Ollama Basic Auth, the configured-admin setting and security setup choices survive saving and restarting.
- Provider (MSP) installs keep each client's networks separate and client workspaces healthy through upgrades, and agent install tokens are created in the right client workspace (#2247, #2226, #2209).
- Organization owners can reach the settings their organization allows (#2208).

## Other improvements

- Availability views with 20 or more checks open in the fleet view, and your chosen table or fleet view stays put across refreshes and shared links.
- The metrics store drops replayed samples and is more robust across shutdowns, rollups and upgrades.

## Known issues

- **PBS History for a replaced PBS host can reappear.** If a PBS host's identity changes, its old History may show again. No monitoring data is lost. The fix is planned for v6.4.6 (#2343).
- **The update screen can stay on "Downloading update… 10%".** The update still completes in the background. Refresh the page after a minute to see the new version. A fix is planned for v6.4.6.
- Some PBS History layouts that combine the API and an agent are still not fully resolved (#1723).
- Disk writes are much lower but not yet at their final level on busy installs. Keep an eye on write activity if you run on flash storage, and some older duplicate incidents may remain (#1966).
- A small number of v5 to v6 upgrade problems remain for specific setups (#1913).

## Before you upgrade

- Back up your Pulse data directory and configuration, and keep the backup until you have checked everything works.
- This release includes the changes from v6.4.2, which was never published. If you use SSO only, map at least one trusted identity-provider group to the built-in `admin` role before upgrading so you keep admin access.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable, so Windows may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile works with this release unchanged.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC installs, use `sudo /bin/update --version v6.4.1`. For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
