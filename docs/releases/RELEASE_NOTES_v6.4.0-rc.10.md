# Pulse v6.4.0-rc.10 Release Notes

`v6.4.0-rc.10` strengthens alerting as an operational system. Histories survive restarts, alerts can be scheduled and routed more precisely, and new forecasting detects storage risk earlier.

## What's improved

- **Durable alert lifecycles** - Alert history and active state now rebuild from the event log after restarts, with persisted identities migrated automatically and false all-clear states prevented during hydration.
- **Better notification control** - Alerts can be snoozed individually, maintenance can recur by scope, escalation repeats can target specific destinations, delivery routes can filter by severity, and informational events remain distinct from warnings.
- **Earlier capacity warnings** - Rolling metric windows and predictive storage forecasts surface sustained pressure and likely exhaustion before a single threshold breach becomes an outage.
- **More accurate host disk health** - SMART sector, media, endurance, spare, and CRC thresholds can be tuned per host, while empty Unraid slots remain neutral and Proxmox-linked agents avoid duplicate disk-risk alerts.
- **External monitoring for Pulse itself** - Pulse can send a health signal to a Healthchecks-compatible watchdog on another host, with configuration and recovery state persisted.
- **Clearer infrastructure details** - Resource drawers, Proxmox backup views, and alert timelines present more complete and consistent context across desktop and narrow layouts.
- **Safer governed actions** - Docker action results now carry independently observed post-action state, while deployment enrollment and credential changes persist atomically.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid. Alert identity and history migrations run automatically, with no manual data migration required.
- Existing Pulse Mobile iOS build 12 and Android versionCode 9 remain compatible. The new `alert_fired` push uses the already-supported `view_alert` action and existing informational severity presentation, so no companion update is required for this candidate.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate. Use the published checksum and detached signature when verifying Windows agent downloads.
