# Pulse v6.2.0-rc.9 Release Notes

`v6.2.0-rc.9` is a release candidate for the next Pulse v6 minor line. It
follows stable `v6.1.2` and supersedes `v6.2.0-rc.8`. This candidate combines
monitoring and alerting correctness fixes with safer agent lifecycle handling,
stronger security boundaries, and a broad responsive-interface pass.

## Highlights

- Alert delivery preserves configured ntfy metadata and acknowledgement state,
  honors disabled grouping, supports wildcard Docker ignore rules, and keeps
  platform-scoped threshold hosts correctly separated.
- Certificate-expiry monitoring, QNAP RAID bitmap parsing, real vCenter tags,
  and resource WebSocket deltas improve the accuracy and freshness of
  infrastructure state.
- Agent install and update paths reject older served binaries, report
  multi-architecture image digests correctly, avoid cross-install process
  matches, and shut down watchdog and wrapper processes within bounded waits.
- Provider-hosted restore writes and automatic agent identity matching now use
  tighter authorization and canonical identity boundaries, alongside fixes for
  the outstanding CodeQL findings and the `js-yaml` security advisory.
- Phone and narrow-screen layouts retain workload identity, keep filters and
  settings reachable, standardize touch targets, preserve focus around inline
  details and token dialogs, and format dates in the viewer's locale.
- Tenant resource stores close cleanly during offboarding and shutdown, guest
  metadata writes finish before monitor teardown, and generation-bound resource
  views reduce repeated allocation work.

## Fixed

- Restored ntfy title, priority, and tags on live delivery and kept grouped
  provider payloads representative of every included alert.
- Preserved provider incidents and acknowledged alerts when notification
  settings are saved, and flushed queued alerts individually when grouping is
  disabled.
- Keyed Docker alert-threshold overrides by container name and increased the
  request limits for fleet-scale threshold and bulk-acknowledgement updates.
- Parsed QNAP RAID role bitmaps correctly and surfaced real vCenter tags instead
  of provenance placeholders.
- Prevented a server from serving an older Unified Agent binary and corrected
  the version warning shown for already-current agent installations.
- Prevented installer cleanup from matching a co-installed sibling agent,
  stopped the previous Unraid watchdog before replacement, and bounded wrapper
  and supervisor shutdown.
- Kept mock infrastructure isolated from configured real sources and delivered
  resource deltas after the initial WebSocket state.
- Removed narrow-screen clipping and reachability issues across platform,
  alert, storage, settings, navigation, and shared filter surfaces.

## Release Qualification

- The v6 control plane reports all 44 readiness assertions and all 25 release
  gates passed for this release-preparation checkpoint.
- Release publication builds and validates one immutable `main` SHA before
  creating or publishing the GitHub prerelease, Docker image, Helm chart, and
  private Pro packet.
- The release workflow runs the complete frontend unit suite, TypeScript check,
  deterministic render smoke, backend tests, installer tests, mobile-impact
  gate, signed candidate assembly, and post-publication asset/install checks.
- Targeted regressions cover the alert, monitoring, agent-install, resource
  lifecycle, security, and responsive-interface changes summarized above.

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
