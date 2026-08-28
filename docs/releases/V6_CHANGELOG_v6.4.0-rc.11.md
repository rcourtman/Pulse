# Pulse v6.4.0-rc.11

This changelog describes the changes since `v6.4.0-rc.10`. The candidate also retains the complete v6.4 alerting change set summarized below.

## Added

- Rolling-window metric evaluation supports sustained CPU and memory policies, including workload inheritance from host defaults.
- Predictive storage-capacity alerts estimate exhaustion risk from retained usage history and recover when the forecast clears.
- Per-alert snooze, recurring scoped maintenance, destination severity routing, repeatable escalation schedules, and external dead-man monitoring expand operator control over alert delivery.
- Resolved host SMART policy covers health failure, sector counters, media errors, remaining life, NVMe spare, and CRC growth without creating duplicate Proxmox disk alerts.
- Canonical `alert_fired` push events use the existing mobile `view_alert` navigation action.
- Informational alerts now retain an explicit `info` severity through configuration, persistence, API responses, filtering, notification routing, and display.

## Changed

- API token creation, agent install issuance, and agent-removal revocation now share an atomic live-and-durable inventory boundary.
- Anthropic pricing distinguishes current Opus and Haiku model generations from legacy aliases when estimating retained usage and enforcing budgets.
- Cross-source disk identity normalizes complete serial and WWN framing without truncating hardware identifiers.
- Alert-lifecycle append failures synchronously checkpoint a lock-independent active-state recovery projection, while an epoch guard prevents overlapping stale checkpoints from clearing degraded state.
- Dead-man configuration and dial-time DNS validation reject every address assigned to the Pulse host and fail closed when local-interface enumeration is unavailable.
- Alert investigation on narrow layouts uses a dedicated timeline dialog, while shared history charts synchronize hover timestamps and retain readable axes.
- Remembered table View preferences expand in a responsive inline disclosure with consistent controls instead of a floating panel over the monitored resources.
- The append-only event log is the authority for alert history and active lifecycle reconstruction, including restart recovery, acknowledgement, resolution, suppression, notification, and migration evidence.
- Alert identities and persisted history migrate to canonical resource keys, while active state uses durable atomic snapshots and ordered recovery.
- Escalation and delivery decisions are destination-specific, repeated holds are coalesced, and destination updates persist before the active runtime changes.
- Resource detail drawers use shared information-card and detail-table primitives across infrastructure, Docker, storage, and Proxmox backup surfaces.
- Docker lifecycle results distinguish command acceptance from independently observed post-action state, and deployment enrollment plus credential updates commit atomically.
- Email, ntfy, and mobile push presentation preserve informational priority instead of elevating non-warning events to warning treatment.
- Active-alert cards give informational severity an explicit blue presentation while preserving warning as the fail-safe for unknown values.

## Fixed

- Failed token persistence no longer leaves an undisclosed new token active, evicts an older valid token, or allows an apparent agent-token revocation to reverse after restart.
- Current Anthropic Opus usage is no longer overestimated at legacy rates, and Haiku 4.5 is no longer underestimated at Haiku 3 rates.
- PVE and agent observations of the same RAID-controller volume no longer render duplicate resources when one source frames the NAA identity as a serial and the other as a WWN.
- Dismissed TrueNAS SMART alerts retain uniquely resolved critical disk evidence instead of allowing a damaged disk to return to healthy presentation.
- Failed durable lifecycle writes no longer risk losing current active alerts or resurrecting alerts resolved before restart, and stale periodic checkpoints cannot overwrite the failure snapshot.
- A watchdog hostname or private-LAN address that resolves back to Pulse can no longer masquerade as an external progress signal.
- Mobile alert timelines remain scroll-stable, selected incident history revalidates correctly, chart axis labels remain readable, and grouped hover timestamps stay aligned.
- View preferences and the nested column selector no longer clip, cover table content, or leave mismatched controls on narrow and desktop layouts.
- Alert hydration no longer exposes a false all-clear state before persisted incidents are restored.
- Restart recovery, history queries, and mock alert timelines preserve lifecycle order, observation time, and complete incident evidence.
- Fresh rolling-window metric data remains authoritative, including when older samples or counter resets are present.
- Offline mock hosts remain on the normal host-alert lifecycle instead of losing active incidents during refresh.
- API token watcher updates remain ordered across successive persistence mutations.
- Proxmox backup health, inventory refresh, offline fixtures, and drawer detail presentation retain complete current context.
- Informational active-alert cards use the blue severity palette, while unknown severity values fail safe to warning presentation.
- Empty Unraid storage slots no longer make an otherwise healthy array report a degraded state.

## Release Metadata

- Version: `v6.4.0-rc.11`
- Previous candidate: `v6.4.0-rc.10`
- Previous published candidate: `v6.4.0-rc.10`
- Previous stable: `v6.3.2`
- Rollback target: `v6.3.2`
- Rollback command: `./scripts/install.sh --version v6.3.2`
- Promotion path: exact-SHA single-build release candidate from `main`
- Windows signing decision: prereleases publish checksum- and detached-signature-verified Windows agents without Authenticode while SignPath remains unavailable. Windows may show an Unknown Publisher warning.
- Mobile decision: `existing-mobile-build-compatible`. Published iOS build 12 and Android versionCode 9 already route `action_type=view_alert`, accept severity as a string, and render informational severity, so no companion upload is required.
