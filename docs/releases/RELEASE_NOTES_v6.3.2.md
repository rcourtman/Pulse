# Pulse v6.3.2 Release Notes

`v6.3.2` is a stable patch release for the Pulse v6 line. It follows
stable `v6.3.1` and corrects memory retention, offline-alert policy handling,
self-container update detection, and fixed polling intervals.

## Highlights

- Metrics history releases oversized backing arrays after dense historical
  data ages out, preventing memory-limit wedges.
- Disabled offline policies suppress platform connection alerts, including
  per-resource overrides and expected-offline intent.
- Pulse no longer reports its own container as outdated when the product
  update service says the installation is current.

## Fixed

- Metrics-history retention compacts trimmed windows and sheds seed-sized
  capacity once only a small live window remains
  ([fix](https://github.com/rcourtman/Pulse/commit/f18f15bf7b1fbdf1078bf412754c059694c8b996)).
- Platform connection alerts honor disabled resource and global offline
  policies, expected-offline intent, and offline quiet-hours routing
  ([#1721](https://github.com/rcourtman/Pulse/issues/1721)).
- Pulse-managed public container references use the product update service and
  no longer create a false self-update badge
  ([#1774](https://github.com/rcourtman/Pulse/issues/1774)).
- Availability targets keep their configured cadence when adaptive scheduling
  is off rather than being re-armed on every monitor tick
  ([#1745](https://github.com/rcourtman/Pulse/issues/1745)).

## Release Qualification

- The release uses the urgent stable-patch path because metrics-history memory
  growth can wedge Pulse under container memory limits and disabled offline
  policies can still generate alert noise on stable.
- The exact pushed release SHA must pass the governed single-build release
  pipeline and its integrated exact-SHA candidate checks before publication.
- Windows Unified Agent binaries are not Authenticode-signed for `v6.3.2` and
  may display an Unknown Publisher warning. The standing SignPath-unavailable
  policy changes only Authenticode: exact-SHA candidate binding, checksums,
  detached signatures, immutable-manifest verification, and published-digest
  verification remain mandatory.
- No mobile-facing path changed between `v6.3.1` and this release, so the
  mobile decision is `no-mobile-impact`; no companion build or store rollout
  is required.

## Upgrade Notes

Use the normal v6 install or update flow for `v6.3.2`. Existing configurations
remain valid and no manual data migration is required.

The rollback target is `v6.3.1`. The exact rollback reinstall command is:

```bash
./scripts/install.sh --version v6.3.1
```

Paid Pulse Pro, Relay, and eligible legacy customers should continue to use the
private download page and private runtime image for paid runtime features.
