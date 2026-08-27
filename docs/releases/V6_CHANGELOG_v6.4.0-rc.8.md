# Pulse v6.4.0-rc.8

_This changelog describes the changes since `v6.4.0-rc.7` included in
`v6.4.0-rc.8`. The `v6.4.0-rc.7` candidate was quarantined before public
activation, so the customer-visible entries below remain cumulative from
`v6.4.0-rc.6`._

## Added

- Rolling-window metric evaluation supports sustained CPU and memory policies, including workload inheritance from host defaults.
- Predictive storage-capacity alerts estimate exhaustion risk from retained usage history and recover when the forecast clears.
- Per-alert snooze, recurring scoped maintenance, destination severity routing, repeatable escalation schedules, and external dead-man monitoring expand operator control over alert delivery.
- Resolved host SMART policy covers health failure, sector counters, media errors, remaining life, NVMe spare, and CRC growth without creating duplicate Proxmox disk alerts.
- Canonical `alert_fired` push events use the existing mobile `view_alert` navigation action.
- Informational alerts now retain an explicit `info` severity through configuration, persistence, API responses, filtering, notification routing, and display.

## Changed

- The append-only event log is the authority for alert history and active lifecycle reconstruction, including restart recovery, acknowledgement, resolution, suppression, notification, and migration evidence.
- Alert identities and persisted history migrate to canonical resource keys, while active state uses durable atomic snapshots and ordered recovery.
- Escalation and delivery decisions are destination-specific, repeated holds are coalesced, and destination updates persist before the active runtime changes.
- Resource detail drawers use shared information-card and detail-table primitives across infrastructure, Docker, storage, and Proxmox backup surfaces.
- Docker lifecycle results distinguish command acceptance from independently observed post-action state, and deployment enrollment plus credential updates commit atomically.
- Email, ntfy, and mobile push presentation preserve informational priority instead of elevating non-warning events to warning treatment.

## Fixed

- Alert hydration no longer exposes a false all-clear state before persisted incidents are restored.
- Restart recovery, history queries, and mock alert timelines preserve lifecycle order, observation time, and complete incident evidence.
- Fresh rolling-window metric data remains authoritative, including when older samples or counter resets are present.
- Offline mock hosts remain on the normal host-alert lifecycle instead of losing active incidents during refresh.
- API token watcher updates remain ordered across successive persistence mutations.
- Proxmox backup health, inventory refresh, offline fixtures, and drawer detail presentation retain complete current context.

## Release Metadata

- Version: `v6.4.0-rc.8`
- Previous candidate tag: `v6.4.0-rc.7`
- Previous published candidate: `v6.4.0-rc.6`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable. Windows may show an Unknown Publisher warning.
- Mobile decision: `existing-mobile-build-compatible`. Published iOS build 12 and Android versionCode 9 already route `action_type=view_alert`, accept severity as a string, and render informational severity, so no companion upload is required.
