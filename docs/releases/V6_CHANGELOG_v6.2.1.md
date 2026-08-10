# Pulse v6.2.1

_This changelog describes stable `v6.2.1` compared with stable `v6.2.0`._

## Changed

- The compiled Pro edition now supplies commercial presentation context even
  before a license is activated, while demo and white-label suppression remains
  authoritative.
- Fresh Pro setup points operators directly to license activation.
- The server updates panel shows the age of the check behind cached "Up to date"
  verdicts (#1601).

## Fixed

- Agent artifact preflight follows redirects and validates the checksum and
  signature headers on the final response (#1696).
- Agent Doctor repairs stale installed-agent reporting credentials.
- Subscription-backed Patrol tools use strict provider-compatible JSON
  schemas (#1697).
- Activation completion clears stale demo presentation state.

## Release Metadata

- Version: `v6.2.1`
- Previous stable: `v6.2.0`
- Rollback target: `v6.2.0`
- Rollback command: `./scripts/install.sh --version v6.2.0`
- Promotion path: emergency stable patch from `main`, using the integrated
  exact-SHA candidate and definitive release verdict
- Emergency reason: active customer harm in agent download preflight,
  installed-agent credential recovery, subscription-backed tool execution, and
  fresh Pro activation discovery
- Windows signing decision: version-bound unsigned-Windows exception approved
  for `v6.2.1`; the binaries are not Authenticode-signed and may display an
  Unknown Publisher warning, while checksum, detached-signature, manifest, and
  published-digest verification remain mandatory
- Mobile decision: `no-mobile-impact`; no companion build upload or public
  store rollout is required
