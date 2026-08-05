# Pulse v6.2.0-rc.9 Release Notes

`v6.2.0-rc.9` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.8`. This candidate focuses on
alert-notification correctness, orderly tenant and monitor shutdown, and lower
allocation pressure on frequently read resource views.

## Highlights

- Live ntfy alert delivery now preserves the configured title, priority, and
  tags, matching the notification test path.
- Saving notification settings no longer drops provider incidents or resets
  acknowledgement state, and disabling grouping now delivers queued alerts
  individually instead of retaining stale grouped behavior.
- Grouped provider notifications describe every alert in the group rather than
  presenting one alert as though it were the whole incident set.
- Tenant offboarding and server shutdown now release per-tenant resource-store
  handles, while guest metadata writes are allowed to finish before the monitor
  stops.
- Resource APIs reuse per-generation raw and presentation lists, and mock-mode
  consumers reuse a versioned unified view between fixture ticks.

## Fixed

- Restored ntfy metadata on the live firing path and kept live/test delivery
  semantics aligned.
- Preserved non-metric provider incidents and acknowledged alert state across
  notification configuration saves.
- Honored `grouping.enabled: false`, flushed pending alerts separately, and
  represented all grouped alerts in provider payloads.
- Closed tenant-scoped resource stores during offboarding, replacement, and
  shutdown without leaking handles or racing active readers.
- Waited for guest metadata persistence before monitor teardown so accepted
  writes are not abandoned during shutdown.
- Replaced repeated registry and mock-fixture rebuilding with generation-bound
  shared read views while keeping request-local decoration isolated.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 25 release
  gates passed for this release-preparation checkpoint.
- Release publication builds and validates one immutable `main` SHA before
  creating or publishing the GitHub prerelease, Docker image, Helm chart, and
  private Pro packet.
- Notification regression suites cover live ntfy metadata, configuration-save
  incident continuity, acknowledgement preservation, disabled grouping, and
  multi-alert provider payloads.
- Resource and monitoring regression suites cover per-generation list sharing,
  mock-view caching, orderly metadata completion, and resource-store cleanup.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.2.0-rc.9` only when you are
comfortable testing an RC. The rollback target is stable `v6.1.2`.

The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.1.2
```

Existing configurations remain valid and no manual data migration is required.

This server candidate is compatible with the current Pulse Mobile 1.0.0 beta
candidates. iOS build 12 is distributed through the TestFlight public beta link,
and Android versionCode 9 remains available through Play open testing; both use
runtime version 2. The changes since RC8 do not alter mobile relay payloads,
pairing, approvals, authentication, or onboarding contracts. No public
mobile-store rollout is part of this RC.

Windows Unified Agent binaries in this candidate keep checksum and
detached-signature verification, but they are not yet Authenticode-signed and
Windows may show an unknown-publisher warning. No unsigned-Windows exception
applies to any `v6.2.0` release. Stable `v6.2.0` must publish Windows agents
through the mandatory SignPath Authenticode path.

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
