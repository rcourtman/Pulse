# Pulse v6.4.0-rc.6 Release Notes

`v6.4.0-rc.6` focuses on more complete host data and earlier hardware-fault detection, while making alert behavior more predictable ahead of the stable release.

## What's improved

- **Complete standalone PBS details** — Standalone PBS entries retain agent-reported system, hardware, network, disk, thermal, service, history, and management details in the full drawer ([#1723](https://github.com/rcourtman/Pulse/issues/1723)).
- **Complete LXC filesystem coverage** — Filesystem usage continues to populate across larger LXC hosts instead of later containers being left without mount data when earlier collection is slow ([#1477](https://github.com/rcourtman/Pulse/issues/1477)).
- **Earlier disk-cabling warnings** — SMART UDMA CRC growth now raises a warning, resets its baseline after a counter reset or disk replacement, and clears the active event after a stable sample ([#1776](https://github.com/rcourtman/Pulse/issues/1776)).
- **More predictable alerts** — Enabled state, configured values, delays, and built-in defaults now follow the same policy path across resource types, with the superseded lifecycle state removed.
- **Safer alert-history queries** — Oversized requested result limits are bounded before memory is reserved, preventing the request from driving an unnecessarily large allocation.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid and no manual data migration is required.
- Pulse Mobile does not consume the changed PBS browser detail or alert evaluation internals. Existing mobile, Relay, onboarding, pairing, push, and mobile-facing API contracts are unchanged, so no companion update is required.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate; use the published checksum and detached signature when verifying Windows agent downloads.
