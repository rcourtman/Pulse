# Pulse v6.1.2 Release Notes

`v6.1.2` is a stable patch release following `v6.1.1`. It consolidates
monitoring, storage, agent-lifecycle, security, and operator-experience fixes
completed since the previous stable cut.

## Highlights

- Proxmox monitoring is more authoritative across cluster membership, guest
  sampling, Ceph MON/MGR counts, physical disks, backup posture, and scheduled
  multi-guest `vzdump` jobs.
- TrueNAS monitoring uses the supported JSON-RPC transport, restores pool and
  replication health classification, and keeps storage topology and SMART
  evidence aligned.
- Unified Agent enrollment and update recovery are more resilient. Constrained
  QNAP and Unraid installs now fail early on insufficient disk space and use
  bounded rotating logs instead of unbounded watchdog output.
- Polling cadence changes apply to live monitors immediately, connection
  staleness scales with the configured interval, and PBS health remains owned
  by current probe evidence.
- Resource links, availability summaries, organization switching, recovery
  posture, and operator overrides behave consistently across the main
  infrastructure surfaces.

## Changed

- Evidence-based pool-health alerts preserve the provider's authoritative
  topology and health discriminator.
- Scheduled Proxmox backup jobs are projected into stable per-guest task
  records, including running, completed, and failed guest outcomes.
- SATA devices behind SAS HBAs retain SATA transport handling for SMART
  collection, while native Unraid and TrueNAS topology remains authoritative.
- Metrics persistence reduces SQLite write amplification and keeps bounded
  history caches without weakening current-state freshness.
- Local AI provider readiness checks exercise the representative Patrol
  runtime path and provide more actionable compatibility results.

## Fixed

- Changing backup and PBS polling intervals now updates active monitors without
  requiring a service restart (#1619).
- Proxmox Quincy and Squid Ceph status payloads report the correct MON and MGR
  counts (#1626).
- Scheduled multi-guest Proxmox backup jobs no longer disappear from
  guest-centric coverage and alert-intent views.
- QNAP and Unraid agent installs detect insufficient temporary or destination
  space before downloading, honor `TMPDIR`, and prevent appliance watchdog logs
  from growing without bound (#1617).
- Operator-state reads normalize an absent automatic-action policy to
  `capabilityNames: []`, preventing expanded resource rows from crashing.
  Intentionally-offline and never-auto-remediate controls remain available for
  stopped Docker containers (#1621, #1622).
- Proxmox cluster membership, guest CPU rate sampling, physical-disk identity,
  agent registration, and workload refresh remain coherent across nodes.
- TrueNAS boot-pool, replication, pool-health, disk, and alert-threshold
  evidence no longer falls back to stale or unsupported interpretations.
- Availability summaries no longer write into the live resource registry,
  grace-period guests stop generating metrics, and configured discovery
  suppression fails closed.
- Webhook URLs and PBS probe failures are redacted from logs and diagnostics,
  local-provider requests retain SSRF protections, and legacy RBAC recovery
  remains reachable when an old import fails.
- Resource web-interface links are canonicalized consistently, and live queries
  refresh after an organization switch.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.2`.

Windows Unified Agent binaries in `v6.1.2` are not Authenticode-signed. The
SignPath company-verification application is still processing, and the release
owner approved a version-bound unsigned-Windows exception for this release.
Windows may therefore show an **Unknown Publisher** warning during installation
or update. The binaries remain bound to the exact release SHA and protected by
the published checksums, detached signatures, candidate manifest, and
post-publication digest verification.

The rollback target for this patch release is `v6.1.1`. The exact rollback
reinstall command is:

```bash
./scripts/install.sh --version v6.1.1
```

The server/mobile decision is `no-mobile-impact`. No mobile-facing relay,
pairing, onboarding, or trust-contract path changed from `v6.1.1`, so this
release does not upload a companion mobile build or start a public
mobile-store rollout.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
