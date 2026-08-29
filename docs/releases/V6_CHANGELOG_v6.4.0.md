# Pulse v6.4.0

This changelog describes the complete stable release train since `v6.3.2`.

## Added

- Rolling-window CPU evaluation detects sustained pressure from retained metric history while holding incident state when the sample window is incomplete.
- Predictive storage-capacity alerts estimate exhaustion risk from retained usage trends and recover when trustworthy evidence moves the forecast outside the risk window.
- Per-alert snooze, recurring scoped maintenance, destination severity routing, repeating escalation schedules, and external Healthchecks-compatible dead-man monitoring expand operator control over alert delivery.
- Resolved host SMART policy covers overall health, sector counters, media errors, remaining life, NVMe spare, and CRC growth without creating duplicate Proxmox disk alerts.
- Informational alerts retain an explicit `info` severity through configuration, persistence, API responses, filtering, notification routing, and presentation.
- Canonical `alert_fired` push events use the existing mobile `view_alert` navigation action.
- Diagnostics can re-enrol a blocked agent, and the generated agent install flow makes the newly issued token available for a deliberate copy action.

## Changed

- The append-only event log is authoritative for alert history and active lifecycle reconstruction, including restart recovery, acknowledgement, snooze, resolution, suppression, notification, and migration evidence.
- Alert identities and persisted history migrate to canonical resource keys, while active state uses durable atomic snapshots and ordered recovery without a false all-clear during hydration.
- Alert delivery and recovery decisions are destination-specific, repeated holds are coalesced, and destination updates persist before the active runtime changes.
- Large infrastructure views use windowed rows, keyed deltas, incremental hydration, and interaction-aware live update deferral to keep navigation and scrolling responsive.
- Resource tables and detail drawers share consistent information, filtering, history, touch, and responsive layout patterns across infrastructure, Docker, storage, Proxmox, and alerts.
- Phone Settings use a searchable two-level workspace with a sticky section title and touch-sized controls.
- Narrow alert investigations use a dedicated timeline dialog, while shared history charts synchronize hover timestamps and retain readable axes.
- Standalone Proxmox identity reconciliation uses provider endpoints and TLS evidence to distinguish installations that reuse node names without borrowing another site's linked agent.
- Proxmox and PBS views retain source-specific node, network, backup, RAID, LXC filesystem, inventory, and history context.
- Unchanged inactive Docker container details are cached for 15 minutes. Running, paused, restarting, changed, and incompletely described containers remain on the live inspection path.
- Docker lifecycle results distinguish command acceptance from independently observed post-action state, and Patrol requires observed health recovery before closing a finding.
- API token creation, agent install issuance, agent removal, and deployment enrollment share atomic live-and-durable credential boundaries.
- GitHub Atom fallback parsing retains release timestamps and selects the current Linux architecture's signed archive from a validated version tag.
- Patrol retries provider startup on each scheduled run, reconciles existing actionable findings when enabled, and reports the blocking reason when the provider remains unavailable.
- Anthropic cost accounting distinguishes current Opus and Haiku generations from legacy aliases when enforcing Patrol budgets.
- Release, telemetry, update, and GitHub-star prompts share a quieter non-blocking presentation and do not claim published notes for development builds.

## Fixed

- Docker hosts with many stopped containers no longer re-inspect every historical container on each 30-second report. Lifecycle changes invalidate cached detail immediately and daemon reconnects clear the cache.
- Separate standalone Proxmox sites that reuse a short node name no longer lose correct agent links when provider-observed addresses uniquely disambiguate them.
- Sequential standalone Proxmox connections no longer reuse the first connection's linked agent when names are ambiguous or endpoints do not match.
- Proxmox backup details remain on the Backups surface with complete current server and datastore context instead of being duplicated or displaced on Overview.
- Configured PVE node interface names and IPv4 or IPv6 addresses remain visible even when no host agent is linked.
- TrueNAS SMART alerts retain uniquely resolved critical disk evidence and supported uncorrectable-error and spare-reserve counters instead of allowing a damaged disk to appear healthy.
- Empty Unraid array slots no longer materialize as physical disks or contribute a false degraded state.
- PVE and agent observations of the same RAID-controller volume no longer render duplicate resources when serial and WWN framing differ.
- Fresh rolling-window metric data remains authoritative when persisted history contains an older point at the same timestamp, and concurrent history replacement keeps one stable evaluation snapshot.
- Unchanged remote agent configuration refreshes no longer create repetitive log noise.
- GitHub API rate limiting no longer leaves in-app release checks with a new version but no archive URL or publication timestamp.
- Failed token persistence no longer leaves an undisclosed token active, evicts an older valid token, or allows an apparent revocation to reverse after restart.
- Mobile alert timelines remain scroll-stable, selected incident history revalidates correctly, chart labels stay readable, and grouped hover timestamps remain aligned.
- Restart recovery, history queries, and mock alert timelines preserve lifecycle order, observation time, and complete incident evidence.
- Proxmox backup health, inventory refresh, offline fixtures, and drawer detail presentation retain complete current context.
- Current Anthropic Opus usage is no longer overestimated at legacy rates, and Haiku 4.5 is no longer underestimated at Haiku 3 rates.
- Informational active-alert cards use the blue severity palette, while unknown severity values fail safe to warning presentation.
- Alert-lifecycle failures synchronously checkpoint a crash-safe recovery envelope before shutdown, and malformed degraded-state markers cannot make startup trust a stale SQLite projection.
- Dead-man configuration and dial-time DNS validation reject every address assigned to the Pulse host.

## Release Metadata

- Version: `v6.4.0`
- Previous stable: `v6.3.2`
- Promoted prerelease: `v6.4.0-rc.12`
- Runtime content cutoff: `18b22d1ebbfe542484652e419320fc7643a792f0`
- Rollback target: `v6.3.2`
- Rollback command: `sudo /bin/update --version v6.3.2`
- Promotion path: owner-approved expedited exact-SHA stable cutoff from `main`
- Promotion decision: the release owner accepted a shortened soak to deliver bounded fixes for active monitoring correctness and collection-load harm. This is version-bound risk acceptance, not soak evidence or a standing exception.
- Windows signing decision: the standing SignPath-unavailable policy applies. Windows agents are not Authenticode-signed, may show an Unknown Publisher warning, and retain exact-SHA checksums, detached signatures, immutable-manifest verification, and published-digest verification.
- Mobile decision: `existing-mobile-build-compatible`
- Mobile evidence: Pulse Mobile iOS build 12 and Android versionCode 9 already support `action_type=view_alert`. Server and Unified Agent changes do not alter mobile routes, pairing, push, or resource payload contracts, so no companion upload is required.
