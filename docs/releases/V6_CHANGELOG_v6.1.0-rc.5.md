# Pulse v6.1.0-rc.5

_This changelog describes the changes since `v6.1.0-rc.4`.
`v6.1.0-rc.5` remains a prerelease and rolls back to stable `v6.0.5`._

## Added

- Unified Agent `--report-ip` propagation from installer to host identity.
- SAS transport detection and SCSI SMART attribute parsing.
- Routed Agent Doctor page with status filters and copyable diagnostic reports.
- Canonical SignPath request, verification, signer validation, and evidence
  plumbing for stable Windows release candidates.
- Branch-level proof for connection, monitoring, workload metadata, setup,
  Unraid, retry, and agent-update helper paths.

## Changed

- Physical interface addresses lead virtual bridge addresses in host identity.
- Integration-observed machines are modeled separately from real agents, while
  workload-only agents remain visible through their canonical projection.
- Removed-agent records retain their last-known platform for scoped host-local
  cleanup handoff.
- ESXi hosts group under their owning vCenter connection.
- Workload guest metadata subscribes to the canonical metadata event.
- Pro updates request an explicit channel from the dual-channel broker.
- Metrics reads can proceed concurrently with writes.
- Audit retention uses incremental vacuum and bounded page reclamation.

## Fixed

- Manual agent update commands remain available unless an update is applying.
- Removed agents receive platform-correct local uninstall guidance without
  implying remote uninstall authority.
- TrueNAS TLS failures are surfaced through the infrastructure manage flow.
- Proxmox disk health values normalize to the canonical presentation states.
- Patrol assessment lookup recovers active findings from mangled identifiers.
- Patrol action origins retain proposal evidence.
- ICMP probe failures explain a missing `CAP_NET_RAW` capability.
- Integration-monitored machines no longer create fabricated ledger agent rows
  or misleading Agent Doctor entries.

## Security

- Reviewed Patrol actions retain their proposal evidence at the origin
  boundary.
- Stable Windows publication has an exact-SHA SignPath signing and verification
  path; prerelease Windows binaries continue under the explicit unsigned-RC
  exception until that external project is configured.

## Release Metadata

- Version: `v6.1.0-rc.5`
- Previous candidate: `v6.1.0-rc.4`
- Rollback target: `v6.0.5`
- Rollback command: `./scripts/install.sh --version v6.0.5`
- Promotion path: exact-SHA single-build release candidate from `main`
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is part of this candidate
