# Pulse v6.2.0-rc.3

_This changelog describes the changes since `v6.2.0-rc.2`.
`v6.2.0-rc.3` remains a prerelease and rolls back to stable `v6.1.2`._

## Fixed

- Same-named Proxmox clusters from different organizations remain distinct
  when overlapping node names and private addresses carry contradicting TLS
  certificate fingerprints.
- The connection add handler now uses the shared identity predicate instead of
  treating a matching internal cluster name as sufficient proof of identity.
- A regression test reproduces the reported sequence where adding `rewo`
  removed `enacon`, then adding `enacon` removed `rewo`.

## Release Metadata

- Version: `v6.2.0-rc.3`
- Previous candidate: `v6.2.0-rc.2`
- Previous stable: `v6.1.2`
- Rollback target: `v6.1.2`
- Rollback command: `./scripts/install.sh --version v6.1.2`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease that does not move stable or latest
  install pointers
- Windows signing decision: Authenticode through SignPath is the mandatory
  signing backend and no unsigned-Windows exception applies to any `v6.2.0`
  release
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is part of this candidate
