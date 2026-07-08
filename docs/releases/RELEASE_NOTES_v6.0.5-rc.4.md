# Pulse v6.0.5-rc.4 Release Notes

`v6.0.5-rc.4` is a release candidate for the next Pulse v6 patch line. It
follows stable `v6.0.4` and supersedes `v6.0.5-rc.3` with MSP report
scheduling and a provider portal alert rollup, per-disk-type temperature
thresholds, a PBS backup discovery recovery, host fingerprint auto-discovery,
surfaced agent auth failures, and additional SSO and filter improvements. It
retains the earlier support fixes for Patrol Gemini model readiness,
remembered-login submit persistence, Proxmox SMART temperature fallback for
direct SATA/SAT disks, legacy agent update token recovery, threshold-aware
temperature display severity, PBS backup polling memory bounds, physical disk
SMART/Proxmox merge identity, Proxmox token preservation diagnostics, legacy
OIDC SSO discovery with CSP nonce handling, Pulse Mobile pairing handoff
sanitization, SSO browser-session display labels, and route-aware Proxmox host
URLs.

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

Use the normal v6 install or update flow for `v6.0.5-rc.4` only when you are
comfortable testing an RC. The rollback target for this release candidate is
`v6.0.4`.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
