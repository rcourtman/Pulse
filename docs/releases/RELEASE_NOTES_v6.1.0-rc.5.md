# Pulse v6.1.0-rc.5 Release Notes

`v6.1.0-rc.5` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.0.5` and supersedes `v6.1.0-rc.4` with more accurate
infrastructure identity, a dedicated Agent Doctor workflow, broader disk-health
coverage, and reliability fixes from continued release-candidate testing.

## Highlights

- Infrastructure now distinguishes real Unified Agents from machines observed
  through an integration, while still surfacing workload-only agents where
  they are useful.
- Agent Doctor is now a dedicated routed page with status filters, copyable
  diagnostic reports, and host-local cleanup handoffs for removed agents.
- Host identity prefers operator-supplied and physical interface addresses over
  virtual bridges, reducing duplicate or misleading resource identities.
- Disk monitoring understands SAS transport and SCSI SMART attributes, and
  Proxmox disk health uses one canonical vocabulary across runtime and UI.
- Metrics reads no longer queue behind unrelated writes, and retained audit
  data is reclaimed incrementally after retention cleanup.
- Agent update controls, TrueNAS connection errors, vCenter membership, and
  Patrol finding continuity are more actionable and consistent.

## Added

- `--report-ip` can set the reported host address during Unified Agent install
  and becomes the leading host-identity signal.
- SAS devices and SCSI SMART attributes participate in disk-health collection.
- Stable release infrastructure can submit exact-SHA Windows agent artifacts
  through the governed SignPath signing and verification path once the external
  project is configured.
- Agent Doctor status filters and copyable reports make large fleet diagnostic
  sets easier to triage and share.
- Additional branch-level proof covers connection grouping, setup
  classification, agent update commands, workload metadata, Unraid merging,
  retry handling, and monitoring helpers.

## Improved

- Physical interface addresses sort ahead of virtual bridge addresses when
  composing host identity.
- ESXi hosts appear as members of their owning vCenter connection.
- Workload guest metadata follows the canonical metadata event instead of a
  legacy event path.
- The infrastructure ledger keeps its manage flow synchronized and shows
  actionable TrueNAS TLS failures.
- Post-update What's New highlights open in a readable dialog instead of a
  full-width banner, leaving the dashboard unobstructed while preserving the
  existing once-per-release dismissal behavior.
- Removed-agent diagnostics retain the last-known platform so Agent Doctor can
  provide the correct host-local uninstall command without granting remote
  removal authority.
- Pro updates request their explicit update channel from the dual-channel
  broker.
- ICMP failures caused by a missing `CAP_NET_RAW` grant explain the required
  runtime permission.

## Fixed

- Integration-monitored machines no longer appear as fabricated agent rows or
  false Agent Doctor targets.
- Agent Doctor no longer lives inside a constrained settings modal; direct
  navigation and back/forward history preserve the diagnostic workflow.
- Manual agent update commands remain available until an update is actually
  applying.
- Patrol assessments recover active findings when an identifier was mangled,
  and action origins retain the proposal evidence that led to review.
- High-volume metric reads can proceed concurrently with writes.
- Audit retention uses incremental vacuuming so deleted history does not leave
  the database permanently inflated.

## Security

- Patrol action origins preserve proposal evidence alongside the reviewed
  action boundary.
- The release pipeline now contains the canonical SignPath submission,
  signature verification, signer check, and evidence-recording path required
  for stable Windows publication.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.0-rc.5` only when you are
comfortable testing an RC. The rollback target is `v6.0.5`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.0.5
```

This server candidate has no mobile compatibility change and does not require
a companion build upload. No public mobile-store rollout is part of this RC.

Windows Unified Agent binaries retain checksum and detached-signature
verification, but they are not yet Authenticode-signed and Windows may show an
unknown-publisher warning. Public Windows Authenticode signing remains required
before stable promotion.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
