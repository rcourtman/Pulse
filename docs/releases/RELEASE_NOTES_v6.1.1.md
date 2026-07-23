# Pulse v6.1.1 Release Notes

`v6.1.1` is a stable patch release following `v6.1.0`. It takes the governed
emergency stable-patch path to resolve active customer harm in the manual
Unified Agent update flow and durable Docker update recovery. It also carries
the reliability, infrastructure, and privacy-disclosure improvements already
completed on `main` after the `v6.1.0` cutoff.

## Highlights

- Manual Unified Agent updates now use the canonical operating-system family,
  so Mageia and other supported Linux distributions receive the Linux update
  path instead of being rejected by a distribution-specific platform value.
- Docker update actions recover their durable terminal receipt by immutable
  action and operation identity. A failed or no-effect digest-drift preflight
  no longer leaves an update stuck after the live capability disappears.
- Outbound usage telemetry moves to a documented schema v2 with coarse,
  pseudonymous operational signals, an exact payload preview, and a one-time
  non-blocking upgrade notice.
- Infrastructure pages retain navigation through stream reconnects, proxy and
  SSO bootstrap is more reliable, node edits use the correct update endpoint,
  and PBS datastore alert overrides appear on the thresholds page.

## Changed

- Agent runtime platform reporting is normalized to the canonical Go operating
  system family while preserving the original operating-system identity for
  diagnostics. Unsupported platforms still fail closed.
- Outbound usage telemetry remains enabled by default unless an operator has
  disabled it, and an existing enabled or disabled preference is preserved on
  upgrade. The rotating pseudonymous installation ID continues to rotate every
  30 days.
- Telemetry schema v2 adds deliberately coarse deployment-method, install-age,
  activation-stage, time-to-first-monitored-resource, and estate-size buckets;
  authentication-configured and monitoring-active booleans; configured
  connection count; aggregate alert outcome counts; aggregate notification
  attempt, delivery, and failure counts; and an operational-outcome boolean.
- Existing installations receive one non-blocking **Telemetry payload
  updated** notice with direct links to the exact payload preview, disable
  action, and privacy disclosure. Fresh installations do not receive a
  duplicate notice.
- Public privacy terminology now describes this rotating-identifier payload as
  **pseudonymous**, not anonymous.

Telemetry does not include identities, account details, hostnames, credentials,
resource identifiers, IP addresses, URLs, paths, locale, recipients,
notification endpoints, alert or notification content, prompts, chat
messages, command text or output, token values, browser events, or an
event-level journey or clickstream. Telemetry rows are retained server-side for
up to 90 days; request IP addresses are used transiently for rate limiting and
are not stored in telemetry rows.

## Fixed

- Manual Unified Agent updates on Mageia and other supported Linux
  distributions no longer fail because the update planner receives a distro
  identifier instead of the Linux platform family (#1607).
- Docker update actions that reach a terminal digest-drift preflight refusal
  now recover as failed or no-effect without redispatch, even when the live
  update capability is no longer advertised (#1608).
- Node state aggregation keeps clusters separate when they reuse the same node
  names.
- Missing Patrol verdicts are swept with bounded follow-up instead of remaining
  indefinitely unresolved.
- Platform navigation survives stream reconnects, and authenticated bootstrap
  works correctly through proxy and SSO configurations.
- Infrastructure node edits route to the update endpoint.
- PBS datastore alert overrides are projected onto the thresholds page.
- The Proxmox VE setup script avoids an `awk` variable name that conflicts with
  implementations where `exp` is reserved.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.1.1`.

Windows Unified Agent binaries in `v6.1.1` are not Authenticode-signed and may
show an Unknown Publisher warning. Verify published checksums and detached
`.sig` or `.sshsig` signatures before installation. This is a `v6.1.1`-only
release-owner exception; later stable releases restore the Windows
Authenticode requirement.

The rollback target for this patch release is `v6.1.0`. The exact rollback
reinstall command is:

```bash
./scripts/install.sh --version v6.1.0
```

The server/mobile decision is `existing-mobile-build-compatible`. Pulse Mobile
`1.0.0` iOS build `11` and Android versionCode `9` remain the compatible
candidate builds. No production relay or mobile trust contract changed after
`v6.1.0`; the matched mobile-facing path is test-only branch coverage. This
release does not upload a companion build or start a public mobile-store
rollout.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
