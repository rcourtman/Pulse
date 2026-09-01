# Pulse v6.4.0-rc.12 Release Notes

`v6.4.0-rc.12` restores reliable in-app upgrades when GitHub rate limiting sends release checks through the Atom feed fallback. It also includes the latest Unraid empty-slot correction and retains the complete v6.4 candidate feature set.

## What's improved

- **Reliable in-app updates during GitHub rate limits** - The Atom fallback now supplies the exact signed release archive for the current Linux architecture, so the confirmation dialog can start the update instead of receiving only a version number.
- **Complete fallback release details** - Feed-based update checks retain the release timestamp as well as the installable asset URL, keeping update metadata consistent with normal GitHub API responses.
- **Visible update-start failures** - If a future update check ever lacks an installable URL, Pulse now reports a clear error instead of leaving the Start Update button apparently unresponsive.
- **Accurate Unraid empty-slot handling** - Sentinel entries used for unassigned array slots remain neutral across agent collection and monitoring, preventing empty slots from being reported as real disks or degraded storage.
- **Correct standalone Proxmox identity reuse** - Separate standalone Proxmox connections can reuse node names without borrowing one another's linked agent or collapsing distinct machines.
- **Atomic credential lifecycle** - API token creation, agent enrollment, and agent removal roll back live state when durable persistence fails, preventing undisclosed credentials, accidental eviction, or revocations that reverse after restart.
- **Accurate Anthropic budgets** - Current Opus and Haiku versions use version-specific first-party rates, so stale estimates no longer stop Patrol early or understate current Haiku usage.
- **Stronger disk identity and risk** - PVE and agent observations of the same RAID volume merge across normalized serial and WWN forms, while dismissed TrueNAS SMART alerts continue to flag uniquely identified disks with critical hardware evidence.
- **More stable incident investigation** - Mobile alert timelines keep context and actions together, history charts expose readable axes, grouped charts synchronize hover timestamps, and View preferences expand inline without covering tables.
- **Durable alert lifecycles** - Alert history and active state rebuild from the event log after restarts, with persisted identities migrated automatically and false all-clear states prevented during hydration.
- **Better notification control** - Alerts can be snoozed individually, maintenance can recur by scope, escalation repeats can target specific destinations, delivery routes can filter by severity, and informational events remain distinct from warnings.
- **Earlier capacity warnings** - Rolling metric windows and predictive storage forecasts surface sustained pressure and likely exhaustion before a single threshold breach becomes an outage.
- **More accurate host disk health** - SMART sector, media, endurance, spare, and CRC thresholds can be tuned per host, while empty Unraid slots remain neutral and Proxmox-linked agents avoid duplicate disk-risk alerts.
- **External monitoring for Pulse itself** - Pulse can send a health signal to a Healthchecks-compatible watchdog on another host, with configuration and recovery state persisted.
- **Clearer infrastructure details** - Resource drawers, Proxmox backup views, and alert timelines present more complete and consistent context across desktop and narrow layouts.
- **Phone-friendly Settings** - On narrow layouts, phone Settings use a searchable index, sticky section title, compact controls, and full-size touch targets.
- **Safer governed actions** - Docker action results carry independently observed post-action state, while deployment enrollment and credential changes persist atomically.
- **Safer recovery and watchdog isolation** - Alert lifecycle failures retain a crash-safe restart snapshot until SQLite is durably repaired, and external dead-man targets fail closed when any address points to a Pulse host interface.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid. Alert identity and history migrations run automatically, with no manual data migration required.
- Existing Pulse Mobile iOS build 12 and Android versionCode 9 remain compatible. The new `alert_fired` push uses the already-supported `view_alert` action and existing informational severity presentation, so no companion update is required for this candidate.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate. Use the published checksum and detached signature when verifying Windows agent downloads.
