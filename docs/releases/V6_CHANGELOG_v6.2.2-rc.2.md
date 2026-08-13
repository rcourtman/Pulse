# Pulse v6.2.2-rc.2

_This changelog describes the changes since `v6.2.2-rc.1`.
`v6.2.2-rc.2` remains a prerelease and rolls back to stable `v6.2.1`._

## Added

- Host-local registry credential discovery for Docker and Podman update checks,
  covering inline `auths`, per-registry helpers, global credential stores, and
  Podman `auth.json` files (#1706).
- An anonymous-only opt-out through
  `PULSE_DISABLE_REGISTRY_CREDENTIALS=true` or
  `--disable-registry-credentials`.

## Changed

- Registry checks can present stored Basic credentials during Bearer token
  negotiation, direct Basic challenges, Docker Hub and GHCR token requests, and
  identity-token refresh grants.
- Credential lookups use a five-minute in-memory cache and stale logins fall
  back to anonymous behavior.
- SMART standby parsing recognizes current EPC mode names in text fallback
  output.

## Fixed

- Private-registry images no longer report a permanent authentication error
  when the host already has a working Docker or Podman login (#1706).
- smartmontools 7.5 may encode guarded-probe `power_mode` as an object; both the
  historical string and current object shapes now decode without discarding
  the complete SMART document (#1690).

## Security

- Registry credentials remain host-local, credential-helper output is not
  exposed through check errors, and helper executable names are validated
  before use.

## Release Metadata

- Version: `v6.2.2-rc.2`
- Previous candidate: `v6.2.2-rc.1`
- Previous stable: `v6.2.1`
- Rollback target: `v6.2.1`
- Rollback command: `./scripts/install.sh --version v6.2.1`
- Promotion path: exact-SHA single-build release candidate from `main`,
  published as a support prerelease without moving stable or latest pointers
- Windows signing decision: the standing prerelease path publishes exact-SHA,
  checksum, and detached-signature verified Windows agents without
  Authenticode; stable `v6.2.2` restores mandatory SignPath signing
- Mobile decision: `no-mobile-impact`; changes since `v6.2.2-rc.1` do not
  modify mobile, Relay, onboarding, or mobile-facing API contracts, and no
  companion upload or public store rollout is required
