# Pulse v6.4.4-beta.4

This changelog describes the changes since published `v6.4.4-beta.2`.
Beta.3 did not complete publication. Beta.4 fixes that History checkpoint
forward with metrics-statistics connection isolation while retaining the alert,
recovery, API, host, update, identity, reconnect, and accessibility repairs from
beta.2. It also carries the complete `v6.4.2` change set through the published
preview line.

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
- Beta.2 alert acceptance passed on a controlled installation, but ordinary
  email-provider and reporter acceptance remain incomplete.
- Issue #1966 aggregate write volume and duplicate-incident evidence remain
  unresolved and are not claimed as repaired by this checkpoint.
- Permanent SMTP failures may still consume the inner retry budget before final
  queue handling.
- No governed mobile, Relay, pairing, approval, push, or onboarding contract
  changed.

## Release metadata

- Version: `v6.4.4-beta.4`
- Previous published preview: `v6.4.4-beta.2`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA single-build release candidate from `release/v6.4`
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while the
  standing SignPath-unavailable policy applies
- Mobile decision: `no-mobile-impact`. No product mobile contract changed, so
  no companion mobile build or store rollout is required
