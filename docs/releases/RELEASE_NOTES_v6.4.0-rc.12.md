# Pulse v6.4.0-rc.12 Release Notes

`v6.4.0-rc.12` is a focused corrective candidate that restores reliable in-app upgrades when GitHub rate limiting sends release checks through the Atom feed fallback. It also includes the latest Unraid empty-slot correction from `main`.

## What's improved

- **Reliable in-app updates during GitHub rate limits** - The Atom fallback now supplies the exact signed release archive for the current Linux architecture, so the confirmation dialog can start the update instead of receiving only a version number.
- **Complete fallback release details** - Feed-based update checks retain the release timestamp as well as the installable asset URL, keeping update metadata consistent with normal GitHub API responses.
- **Visible update-start failures** - If a future update check ever lacks an installable URL, Pulse now reports a clear error instead of leaving the Start Update button apparently unresponsive.
- **Accurate Unraid empty-slot handling** - Sentinel entries used for unassigned array slots remain neutral across agent collection and monitoring, preventing empty slots from being reported as real disks or degraded storage.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid. No manual data migration is required.
- Existing Pulse Mobile iOS build 12 and Android versionCode 9 remain compatible. These updater and Unraid normalization changes do not alter mobile routes, pairing, push, or resource payload contracts.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate. Use the published checksum and detached signature when verifying Windows agent downloads.
