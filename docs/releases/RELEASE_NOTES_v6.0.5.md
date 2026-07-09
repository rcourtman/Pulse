# Pulse v6.0.5 Release Notes

`v6.0.5` is a stable patch release for the Pulse v6 line. It follows
`v6.0.4` and promotes the support fixes that were validated through the
`v6.0.5-rc.4` line, plus the paid-runtime activation continuity and
license-period status fixes needed for Pro customers. It includes Patrol
Gemini model readiness, remembered-login submit persistence, Proxmox SMART
temperature fallback for direct SATA/SAT disks, and the additional
installability and monitoring fixes listed below.

## Added

- Reports can now be scheduled with weekly or monthly cadence, explicit
  resource or tag scoping, and PDF or CSV delivery by email or to disk. The
  MSP provider portal shows a per-workspace alert rollup so provider
  operators see client alert pressure at a glance.
- Physical disk temperature thresholds are now configurable per disk type,
  and explicit per-disk overrides beat the per-type defaults.
- OIDC provider settings now support editing requested scopes, and SSO
  provider restriction fields can be cleared once set.
- Kubernetes shared toolbar filters are now URL-backed so saved views keep
  exclusions, Docker container filters support `-term` search exclusions and
  persistent saved views, and saved views are reachable from the FilterBar
  mobile expanded body.

## Fixed

- Paid runtime activation now preserves a stable installation fingerprint
  across activation clears and restores, so reactivation of the same Pro
  install reuses the existing installation slot instead of consuming new
  slots with generated fingerprints.
- License grants now carry the paid billing period separately from the short
  refresh lease, so status screens can show the real subscription/trial period
  while enforcement keeps using renewable grant expiry.
- Server installation helpers now reject unsafe piped execution paths and
  keep `--version vX.Y.Z` handling aligned between self-refetch and pinned
  update flows.
- Fixed a PBS backup discovery regression from the rc.3 bounded-polling
  change: snapshot groups beyond the bound were synthesized without
  verification, size, or file data, so real deployments saw backups as
  Unverified with a collapsed timeline. Bounds are now derived from real
  snapshot data.
- Hosts now participate in the discovery fingerprint model so they
  auto-discover instead of requiring manual entry.
- Agent auth failures and staleness are now surfaced in the UI instead of an
  agent silently retrying 401 responses forever while the node stayed green,
  and the recovery guidance points at the Pulse UI.
- The Pro binary now blocks in-app self-update so a paid install can no
  longer silently downgrade itself to the community build.
- Registry rebuilds no longer write no-op relationship-change rows, which
  previously grew resource history by hundreds of thousands of rows per day
  on busy deployments.
- Mock-mode data churn is paced to realistic homelab rates, so demo and
  evaluation deployments stop generating phantom restart and state-flap
  events.
- Docker Swarm workload dedupe now picks cluster-scoped winners
  deterministically.
- Legacy 3-segment OIDC SSO login and callback paths are served again so
  bookmarked or configured legacy SSO URLs keep working after upgrade.
- Relay pairing help now points at the Pulse Mobile install page so a
  pairing QR has an obvious companion app to scan it with.
- v5 migration recovery states are clarified so interrupted migrations
  explain what happened and what to do next.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.0.5`. The rollback target for
this patch release is `v6.0.4`.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
