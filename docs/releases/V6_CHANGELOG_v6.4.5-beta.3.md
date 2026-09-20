# Pulse v6.4.5-beta.3

This changelog describes the changes since `v6.4.5-beta.2`. The
`v6.4.5-beta.2` preparation did not complete publication, so the previous
published preview remains `v6.4.5-beta.1`. It carries the complete `v6.4.2`
change set.

## Reliability corrections since beta.1

- **Creating tenant API tokens avoids inventory startup** - Session-authenticated
  token creation no longer starts a tenant monitor. Organization membership,
  lifecycle, CSRF, token ownership and scopes remain enforced.
- **Tenant inventory starts with one provider fill** - Initial resource-store
  wiring batches supplemental providers while preserving synchronous readiness.
- **Notification severity preferences stay saved** - Email and webhook settings
  preserve “Warnings and critical alerts” through save, reload and display (#2069).
- **Fewer unchanged alert checkpoint writes** - Byte-identical pending-intent
  checkpoints are not rewritten. Changed state still persists and stale files are
  repaired. This does not resolve all reported write volume (#1966).
- **More disks remain visible** - Disks without a serial number keep distinct
  inventory identities (#2076).
- **All disk mounts stay reachable** - The Disks card no longer clips its mount
  list inside a nested scroller. Mounts remain in the outer scrolling flow,
  including keyboard access to the final mount (#2051).
- **Container image checks tolerate registry differences** - Digest discovery
  can fall back to a validated manifest response when HEAD lacks a digest (#2048).
- **VM-linked agent memory is used correctly** - Fresh available agent memory can
  supply the linked VM reading without confusing provider instances (#1962).

## TrueNAS CORE snapshots

Non-object REST alert arguments no longer abort snapshots (#2077). Alert text
is preserved and only object fields supply disk identity or SMART evidence.

## Proxmox node History

- Unified Proxmox node resources now retain canonical metrics coordinates in
  the frontend model.
- API-collected Proxmox nodes target the `node` store family instead of an
  inferred `agent` family. Explicit agent targets remain available when that is
  the actual stored source.
- Existing History groups continue to omit node disk I/O where that series is
  not supported.

## Proxmox Backup Server History

- The Backups resource query now includes standalone PBS agent telemetry.
- A PBS system can correlate to a uniquely identified standalone agent or an
  agent-bearing VM or system container.
- Duplicate resource snapshots are removed before the Backups model is built.
- CPU and memory series can plot without disk data. Missing network and disk
  series remain visible as collecting.
- Ambiguous or absent identity evidence does not guess a History target.

## Metrics statistics isolation

- Metrics tier statistics now use one dedicated read-only connection rather
  than sharing the bounded History and write pool.
- The reader returns current committed counts and rejects writes; it does not
  introduce a cache or estimate.
- Normal and timed-out shutdown paths close both owned database handles.
- The existing 500-node mixed-endpoint workload and its latency budgets remain
  unchanged.

## Monitoring lifecycle synchronization

- Monitoring state snapshots use the same synchronization boundary as startup
  and reset writes.
- The correction removes a race found during release validation.

## Verification scope

- Focused Go race checks cover canonical metrics targets and monitoring
  lifecycle synchronization.
- Focused metrics-store checks hold every History connection while validating
  current committed statistics, read-only behavior, clearing, and closure.
- Three controlled hosted comparisons completed 30 of 30 candidate checks and
  kept mixed statistics p95 below the existing four-second budget.
- Focused frontend model, adapter, drawer, and Proxmox surface checks cover
  node and PBS target selection, missing data, and ambiguity guards.
- Source-built Chromium checks at desktop and phone widths verify the exact VM
  and agent History requests and CPU and memory plotting without disk data.
- These checks use synthetic inventory and History responses. Installed
  Proxmox and PBS acceptance remains a beta observation task.

## Carried reliability repairs

- Failed backups and completed PBS-to-PBS sync copies no longer pin a guest in
  Backup Running, and incomplete copies stay out of recoverable latest-backup
  pointers (#1815).
- Separate standalone sites that reuse one short node name and one enrollment
  token no longer collapse into a single host or Docker record (#1753).
- Windows Unified Agent auto-update no longer fails with HTTP 404. Canonical
  executable assets and detached signatures remain available (#1820).

## Known beta limits

- Installed History rendering for API-only nodes, agent-linked nodes,
  standalone PBS systems, and PVE-hosted PBS systems is not yet confirmed.
- Installed high-request-rate statistics behavior is not established. Active
  long-running reads can extend shutdown, and query cancellation and constructor
  fault injection remain RC follow-up items.
- An advisory paired comparison measured route normalization at 2.495 ns versus
  2.185 ns. Route code is unchanged and no user-facing regression is
  established; preserve the adverse result for RC disposition.
- Beta.1 alert acceptance passed on a controlled installation, but ordinary
  email-provider and reporter acceptance remain incomplete.
- Issue #1966 aggregate write volume and duplicate-incident evidence remain
  unresolved and are not claimed as repaired by this checkpoint.
- Permanent SMTP failures may still consume the inner retry budget before final
  queue handling.
- No governed mobile, Relay, pairing, approval, push, or onboarding contract
  changed.

## Release metadata

- Version: `v6.4.5-beta.3`
- Previous published preview: `v6.4.5-beta.1`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA single-build release candidate from `release/v6.4`
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while the
  standing SignPath-unavailable policy applies
- Mobile decision: `no-mobile-impact`. No product mobile contract changed, so
  no companion mobile build or store rollout is required
