# Pulse v6.1.1

_This changelog describes stable `v6.1.1` compared with stable `v6.1.0`._

## Changed

- Unified Agent operating-system reporting now provides the canonical runtime
  platform family to the update planner while retaining detailed OS identity
  for diagnostics.
- Outbound usage telemetry uses schema v2 with rotating pseudonymous identity,
  bounded aggregate operational signals, preserved operator preference, exact
  payload preview, and a one-time non-blocking upgrade disclosure.
- Privacy copy uses the accurate term **pseudonymous** and explicitly lists the
  identity, infrastructure, content, command, and clickstream categories
  excluded from telemetry.
- Infrastructure navigation, authenticated proxy/SSO bootstrap, node-edit
  routing, and PBS alert-threshold projection are more consistent.

## Fixed

- Linux manual agent updates no longer misclassify Mageia or other supported
  distributions as an unsupported update platform (#1607).
- Durable Docker update receipt recovery uses immutable action and operation
  binding, terminalizes digest-drift preflight refusals after capability loss,
  and never redispatches the rejected operation (#1608).
- Cluster node aggregation no longer conflates separate clusters that reuse a
  node name.
- Missing Patrol verdicts retain bounded follow-up.
- The Proxmox VE setup script avoids an `awk` identifier collision.

## Release Metadata

- Version: `v6.1.1`
- Previous stable: `v6.1.0`
- Rollback target: `v6.1.0`
- Rollback command: `./scripts/install.sh --version v6.1.0`
- Promotion path: stable patch hotfix from `main`, with an owner-recorded reason
  for active customer update harm and no fabricated same-version RC tag
- Windows signing decision: `v6.1.1`-only release-owner exception; Windows
  Unified Agent binaries are not Authenticode-signed and may show an Unknown
  Publisher warning, while exact-SHA candidate binding, checksums, detached
  `.sig`/`.sshsig` signatures, manifests, and published digests remain required
- Mobile decision: `existing-mobile-build-compatible`; Pulse Mobile `1.0.0`
  iOS build `11` and Android versionCode `9` require no companion upload, and
  no public store rollout is part of this server release
