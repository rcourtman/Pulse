# Pulse v6.2.1 Release Notes

`v6.2.1` is a stable patch release following `v6.2.0`. It is promoted through
the emergency hotfix path because it repairs active customer failures in agent
installation and subscription-backed automation, and makes license activation
discoverable on fresh Pulse Pro installs.

## Highlights

- Agent download preflight follows redirects and validates final checksum and
  signature headers.
- Agent Doctor recovers stale credentials; subscription-backed Patrol tools use
  strict provider schemas.
- Fresh Pro installs expose activation; compiled Pro builds show commercial
  context outside demo and white-label modes.

## Fixed

- Redirecting agent artifact endpoints no longer lose the final response
  headers needed for checksum and signature preflight (#1696).
- Agent Doctor repairs stale local agent credentials through the governed
  recovery flow instead of leaving the agent unable to report.
- Subscription-provider tools no longer send schemas rejected for missing
  strict object-field requirements (#1697).
- New Pro deployments can reach Plans & Billing before activation and keep the
  expected commercial navigation after activation.
- Demo state converges after successful activation instead of retaining stale
  pre-activation presentation.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.1`.

This is an emergency hotfix because the agent download issue crosses the
installer/updater risk boundary and causes active customer harm. The governed
release workflow still requires the integrated exact-SHA candidate checks,
immutable artifacts, published-digest verification, and definitive release
verdict.

Windows Unified Agent binaries in `v6.2.1` are required to be
Authenticode-signed through SignPath. No unsigned-Windows exception is
authorized for this release.

The rollback target is `v6.2.0`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.2.0
```

The server/mobile decision is `no-mobile-impact`. No mobile-facing API, Relay,
pairing, approval, push, authentication, or onboarding contract changed, so no
companion build upload or mobile-store rollout is required.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
