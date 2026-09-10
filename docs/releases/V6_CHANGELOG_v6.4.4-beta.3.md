# Pulse v6.4.4-beta.3

This changelog describes the changes since `v6.4.4-beta.2`. The bounded delta
corrects History target selection. The alert, recovery, API, host, update,
identity, reconnect, and accessibility repairs from the published beta.2 remain
included. This beta carries the complete `v6.4.2` change set through the
published preview line.

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

## Verification scope

- Focused Go race checks cover canonical metrics targets.
- Focused frontend model, adapter, drawer, and Proxmox surface checks cover
  node and PBS target selection, missing data, and ambiguity guards.
- Source-built Chromium checks at desktop and phone widths verify the exact
  VM and agent History requests and CPU and memory plotting without disk data.
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
- Beta.2 alert acceptance passed on a controlled installation, but ordinary
  email-provider and reporter acceptance remain incomplete.
- Issue #1966 aggregate write volume and duplicate-incident evidence remain
  unresolved and are not claimed as repaired by this checkpoint.
- The retained beta.2 metrics-handler advisory result remains open for a fresh
  disposition before RC.
- Permanent SMTP failures may still consume the inner retry budget before final
  queue handling.
- No governed mobile, Relay, pairing, approval, push, or onboarding contract
  changed.

## Release metadata

- Version: `v6.4.4-beta.3`
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
