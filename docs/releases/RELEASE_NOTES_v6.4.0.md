# Pulse v6.4.0 Release Notes

`v6.4.0` is a stable minor release that makes alerts more trustworthy and actionable, detects capacity and hardware risk earlier, and keeps large or mixed infrastructure estates fast, accurate, and easier to operate.

## What's improved

- **Alerts that survive restarts** - Active incidents, acknowledgements, snoozes, resolutions, and delivery evidence now rebuild from a durable event log without briefly showing a false all-clear.
- **More control over notifications** - Use informational severity, snooze alerts, schedule scoped recurring maintenance, route destinations by severity, repeat critical escalations, and see why delivery was sent, held, suppressed, or failed.
- **Earlier resource warnings** - Rolling CPU averages detect sustained pressure instead of reacting to one sample, while predictive storage alerts warn when current growth could exhaust capacity.
- **Stronger disk-health monitoring** - Expanded SMART policies cover sector, media, endurance, spare, and CRC risks. TrueNAS evidence stays visible, empty Unraid slots stay neutral, and duplicate Proxmox disks merge.
- **External monitoring for Pulse itself** - Pulse can ping a Healthchecks-compatible service every minute, report interruptions after restart, and reject endpoints on the Pulse host that could mask an outage.
- **Faster large estates** - Windowed tables and keyed live updates cut the measured Storage cold load from about 15.6 seconds to 1.1 seconds. Docker also avoids re-inspecting unchanged stopped containers every 30 seconds.
- **Clearer desktop and mobile workflows** - Consistent tables, drawers, timelines, charts, touch controls, and searchable phone Settings make infrastructure details and alert investigations easier to navigate.
- **More accurate Proxmox and PBS coverage** - Node identity, history, networking, backups, RAID members, LXC filesystems, and linked agents now retain the correct source and context, even when installations reuse node names.
- **More reliable agents and actions** - Credential changes are atomic, re-enrolment is available from diagnostics, install tokens are easier to copy, and Proxmox VM or LXC actions no longer require a QEMU guest agent.
- **More dependable Patrol runs** - Patrol retries provider startup on each schedule, reconciles existing findings when enabled, verifies Docker recovery, and uses current Anthropic model pricing for budget enforcement.
- **Updates that work through rate limits** - If GitHub releases are rate limited, Pulse selects the correct release and signed archive for the current Linux architecture so in-app updates can still start.

## Before you upgrade

- Existing configurations remain valid. Alert identity and history migrations run automatically, with no manual data migration required.
- Pulse Mobile iOS build 12 and Android versionCode 9 remain compatible. The new `alert_fired` push uses the existing `view_alert` action, so no companion mobile release is required.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- The rollback target is stable `v6.3.2`. Use `./scripts/install.sh --version v6.3.2` if you need to return to the previous stable release.
