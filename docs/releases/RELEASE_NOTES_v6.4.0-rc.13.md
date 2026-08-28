# Pulse v6.4.0-rc.13 Release Notes

`v6.4.0-rc.13` reduces Docker daemon load on hosts with many stopped containers and restores correct host-agent links when separate standalone Proxmox sites reuse the same node name. It retains the complete v6.4 candidate feature set.

## What's improved

- **Lower Docker load for historical containers** - Unchanged stopped-container details are refreshed every 15 minutes instead of being re-inspected every 30 seconds, while running and changing containers remain live.
- **Correct same-name Proxmox agent links** - Separate standalone sites that reuse a short node name can link to their own host agents through unique provider-observed addresses without borrowing another site's agent.
- **Reliable in-app updates during GitHub rate limits** - The Atom fallback supplies the exact signed release archive for the current Linux architecture, so the confirmation dialog can start the update instead of receiving only a version number.
- **Accurate Unraid empty-slot handling** - Sentinel entries used for unassigned array slots remain neutral across agent collection and monitoring, preventing empty slots from being reported as real disks or degraded storage.
- **Atomic credential lifecycle** - API token creation, agent enrollment, and agent removal roll back live state when durable persistence fails, preventing undisclosed credentials, accidental eviction, or revocations that reverse after restart.
- **Accurate Anthropic budgets** - Current Opus and Haiku versions use version-specific first-party rates, so stale estimates no longer stop Patrol early or understate current Haiku usage.
- **Stronger disk identity and risk** - Duplicate RAID volumes merge across normalized serial and WWN forms. Dismissed TrueNAS SMART alerts still flag uniquely identified disks and expose supported uncorrectable-error and spare-reserve values.
- **More stable incident investigation** - Mobile alert timelines keep context and actions together, history charts expose readable axes, grouped charts synchronize hover timestamps, and View preferences expand inline without covering tables.
- **Durable alert lifecycles** - Alert history and active state rebuild from the event log after restarts, with persisted identities migrated automatically and false all-clear states prevented during hydration.
- **Better notification control** - Alerts can be snoozed individually, maintenance can recur by scope, escalation repeats can target specific destinations, delivery routes can filter by severity, and informational events remain distinct from warnings.
- **Earlier capacity warnings** - Rolling metric windows and predictive storage forecasts surface sustained pressure and likely exhaustion before a single threshold breach becomes an outage.
- **More accurate host disk health** - SMART sector, media, endurance, spare, and CRC thresholds can be tuned per host, while empty Unraid slots remain neutral and Proxmox-linked agents avoid duplicate disk-risk alerts.
- **External availability monitoring** - Dead-man checks can notify when an expected external signal stops arriving, with configuration and recovery state persisted.
- **Clearer infrastructure details** - Resource drawers, Proxmox backup views, and alert timelines present more complete and consistent context across desktop and narrow layouts.
- **Phone-friendly Settings** - On narrow layouts, phone Settings use a searchable index, sticky section title, compact controls, and full-size touch targets.
- **Safer governed actions** - Docker action results carry independently observed post-action state, while deployment enrollment and credential changes persist atomically.
- **Safer recovery and watchdog isolation** - Alert lifecycle failures retain a crash-safe restart snapshot until SQLite is durably repaired, and external dead-man targets fail closed when any address points to a Pulse host interface.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid. Alert identity and history migrations run automatically, with no manual data migration required.
- Existing Pulse Mobile iOS build 12 and Android versionCode 9 remain compatible. The new `alert_fired` push uses the already-supported `view_alert` action, and the server and Unified Agent corrections in this candidate do not alter mobile routes, pairing, push, or resource payload contracts.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate. Use the published checksum and detached signature when verifying Windows agent downloads.
