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
- SSO authentication no longer becomes instance-administrator authority when
  no local administrator is configured. SSO browser sessions must carry an
  effective RBAC `admin` grant on `*` before they can use administrator-only
  settings, discovery, configuration transfer, platform, or infrastructure
  action routes.
- SAML domain and email allowlists now reject assertions without an email
  claim when either allowlist is configured.

## Upgrade requirement

- An SSO-only installation must map at least one trusted IdP group to the
  built-in `admin` role before upgrading. Sessions mapped only to `operator`,
  `viewer`, or no role intentionally lose the administrator access that older
  versions granted implicitly.

## Release Metadata

- Version: `v6.4.2`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: emergency stable patch from `main`, using the single-build
  release workflow after exact-SHA qualification
- Emergency reason: current stable releases contain two authorization
  boundary failures. A non-administrator organization member may reach the
  infrastructure action control plane, and every authenticated IdP user on an
  SSO-only instance may inherit instance-administrator authority without an
  explicit grant
- Windows signing decision: the standing SignPath-unavailable policy publishes
  unsigned Windows Unified Agent binaries. They may display an Unknown
  Publisher warning while exact-SHA candidate binding, checksums, detached
  signatures, immutable-manifest verification, and published-digest
  verification remain mandatory
- Mobile decision: `no-mobile-impact`. The patch changes no governed mobile
  route, payload, Relay, pairing, approval, push, or onboarding contract, so no
  companion mobile build or store rollout is required.
