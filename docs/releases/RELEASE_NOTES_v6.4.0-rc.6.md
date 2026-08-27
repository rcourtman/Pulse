# Pulse v6.4.0-rc.6 Release Notes

`v6.4.0-rc.6` is a release candidate for the next v6 minor release. It restores complete hardware detail for standalone Proxmox Backup Server hosts, warns when SMART CRC counters grow, and completes the alert engine's policy consolidation.

## What's improved

- **Complete standalone PBS details** — Backup-server rows now open the full resource drawer with system, hardware, network, disk, thermal, service, history, and management information when a Pulse Agent source is available.
- **Earlier disk-cabling warnings** — Pulse establishes a SMART UDMA CRC baseline and raises a disk-health warning when the counter grows, helping expose link or cabling faults that a current-state health check can miss.
- **More consistent alerts** — Alert policy is resolved consistently through one declarative path, with the superseded transition-tracking maps removed.
- **Current release toolchain** — Release builds use Go 1.26.7 and its current security fixes.

## Fixes

- Standalone PBS hosts no longer lose their agent-reported hardware details when they are represented on the Proxmox backup surface ([#1723](https://github.com/rcourtman/Pulse/issues/1723)).
- SMART UDMA CRC growth now creates a warning, treats a counter reset or disk replacement as a new baseline, and resolves the active growth event after a stable sample ([#1776](https://github.com/rcourtman/Pulse/issues/1776)).
- Alert configuration values, built-in defaults, and enabled state now use one policy fold across resource families instead of parallel resolution paths.
- Alert-event queries now allocate from the bounded effective result limit rather than an untrusted requested limit.
- Legacy alert transition state has been removed after the reducer cutover, eliminating a second mutable source of lifecycle truth.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid and no manual data migration is required.
- Pulse Mobile does not consume the changed PBS browser detail or alert evaluation internals. Existing mobile, Relay, onboarding, pairing, push, and mobile-facing API contracts are unchanged, so no companion update is required.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate; use the published checksum and detached signature when verifying Windows agent downloads.
