# Pulse v6.4.0-rc.4 Release Notes

`v6.4.0-rc.4` is a release candidate for the next v6 minor release. It fixes two customer-reported trust issues, restores several Proxmox views, and makes live large-estate screens steadier while data is changing.

## What's improved

- **Steadier large-estate screens** — Resource, infrastructure, and alert updates now use keyed deltas. Pulse defers expensive live merges while an operator is interacting, then applies the queued state afterward.
- **Smaller resource snapshots** — Repeated capability descriptions and default policy metadata are deduplicated without changing the resource shape consumed by the web interface.
- **More useful workload tables** — Workload IDs can be sorted, columns can be resized deliberately, and header alignment remains stable across the table.
- **Restored Proxmox network details** — Node network interfaces and their configuration are visible again in the Proxmox detail view.
- **Clearer operational API access** — API token scopes now expose the governed agent-lifecycle capability used for lifecycle operations.

## Fixes

- Delivery warnings now clear as soon as the committed notification queue becomes healthy after a retry, dismissal, cancellation, or successful delivery ([#1761](https://github.com/rcourtman/Pulse/issues/1761)).
- Assistant no longer treats the QEMU guest agent as a prerequisite for governed Proxmox VM or LXC lifecycle actions, and no longer redirects supported requests to manual `qm` or `pct` commands ([#1782](https://github.com/rcourtman/Pulse/issues/1782)).
- Keyed resource rows retain their identity when full snapshots arrive in a different order, and REST recovery no longer contaminates the websocket delta baseline.
- Proxmox backup rows keep the correct alert attribution, filter by repository correctly, and hydrate reliably when backup data arrives after the initial page state.
- Docker container history keeps the selected container identity across filtered refreshes.
- Workload and infrastructure drawers use the canonical history target, preventing one resource from showing another resource's metrics.
- Mock storage and workload histories preserve continuity so release render checks exercise the same stable chart behavior as a live runtime.
- Proxmox network detail formatting and offline-node browser coverage are back under their qualified checks.

## Before you upgrade

- This is a release candidate. Stable installations remain on v6.3.2 unless an operator explicitly selects this version.
- Existing configurations remain valid and no manual data migration is required.
- Pulse Mobile does not consume the changed browser resource stream. The canonical route, request/response, pairing, and push compatibility checks passed against Pulse Mobile 1.0.0 build 12 on iOS and build 9 on Android, so no companion update is required.
- Windows Unified Agent binaries are checksum- and detached-signature-verified but are not Authenticode-signed, so Windows may show an Unknown Publisher warning.

## Known issues

- Windows Authenticode signing remains unavailable for this candidate; use the published checksum and detached signature when verifying Windows agent downloads.
