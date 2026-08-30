# Pulse v6.4.2

_This changelog describes stable `v6.4.2` compared with stable `v6.4.1`._

## Fixed

- The OSS default authorizer no longer treats ordinary authenticated
  organization membership as permission to use the infrastructure action
  control plane. Browser and proxy sessions must satisfy the canonical
  administrator boundary, while explicitly scoped API tokens and real RBAC
  action grants retain their governed paths.
- Route and lifecycle guardrails cover planning, review, approval, execution,
  refresh, and recovery action surfaces so the permission check cannot be
  bypassed through a sibling action endpoint.

## Release Metadata

- Version: `v6.4.2`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: emergency stable patch from `release/v6.4.2`, using the
  single-build release workflow after exact-SHA qualification
- Emergency reason: the current stable action control plane can grant
  infrastructure mutation authority to an authenticated organization member
  who has neither administrator status nor an explicit action grant
- Windows signing decision: the standing SignPath-unavailable policy publishes
  unsigned Windows Unified Agent binaries. They may display an Unknown
  Publisher warning while exact-SHA candidate binding, checksums, detached
  signatures, immutable-manifest verification, and published-digest
  verification remain mandatory
- Mobile decision: `no-mobile-impact`. The patch changes no governed mobile
  route, payload, Relay, pairing, approval, push, or onboarding contract, so no
  companion mobile build or store rollout is required.
