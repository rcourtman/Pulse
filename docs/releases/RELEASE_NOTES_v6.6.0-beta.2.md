# Pulse v6.6.0-beta.2 Release Notes

This beta follows `v6.6.0-beta.1` on the way to stable `v6.6.0`. The changes below compare it with stable `v6.5.0`, which remains the rollback target. The [complete cumulative changelog](https://github.com/rcourtman/Pulse/blob/v6.6.0-beta.2/docs/releases/V6_CHANGELOG_v6.6.0-beta.2.md#changes-since-v650) also covers changes since `v6.5.0`, not just the previous beta.

## Highlights

- Clearer guest readings, backup explanations and alert history.
- Saved alert settings and notification credentials retain their intended behaviour.
- Safer token permissions, sign-in and agent removal recovery.

## What's improved

- **Guest reading age** - Guest drawers distinguish current observations from retained readings and explain why a reading is unavailable.
- **Backup explanations** - Guest backup and reading-safety explanations can be read with a keyboard or on a touch screen, without relying on hover.
- **Windows guest queries** - OS and filesystem queries can proceed when the optional QEMU memory query is unsupported, rather than losing all three readings together.
- **Memory context** - Proxmox guest memory labels explain when hypervisor readings may include cache. Missing guest evidence is not presented as guest pressure.
- **Filesystem totals** - Guest disk totals use complete filesystem observations and avoid double-counting mount aliases. Partial results remain explicitly incomplete.
- **Debian identity** - The agent recognises Debian from its OS identity files, including Debian 13.7, without requiring package or operating-system changes.
- **Polling freshness** - Connection and resource freshness follows the live polling cadence, so adaptive polling is not judged against an unrelated fixed interval.
- **Guest read ownership** - Results from a replaced guest poll or connection generation cannot overwrite the newer monitoring state.
- **Operator links** - An explicit split keeps a Proxmox node and agent apart, instead of monitoring automatically linking them again.
- **Physical disk identity** - Disks with the same serial on different machines remain separate. A replacement disk does not inherit the old disk's alert history.
- **PBS connections** - The connections ledger carries the PBS-reported node name rather than borrowing a different host's label.
- **Retained disk readings** - Silent agents' disk usage, temperatures and RAID state are labelled last known, not shown as fresh measurements.
- **Disk heat** - A disk that is only hot is described as Running Hot, not Replace Now. Temperature alerts use the reporting host's configured policy.
- **Off stays off** - Disabled Docker, Mail Gateway and backup alert thresholds remain off after saving instead of returning to inherited defaults.
- **Threshold consistency** - Guest and filesystem alerts use the same effective thresholds when polling and when settings are saved.
- **Alert editing** - Desktop and phone alert tables share numeric editing, focus changes and Cancel behaviour, preserving a draft when the layout changes.
- **Notification settings** - Saving one notification setting preserves unrelated settings and masked credentials. Webhook drafts survive refreshes of saved configuration.
- **Webhook delivery** - Built-in webhook template headers are applied to ordinary alerts as well as test messages. Masked header values are resolved per key and remain masked in responses.
- **Redirect privacy** - Notifications refuse redirects to a different origin before forwarding private alert bodies, authentication headers or signatures.
- **Notification feedback** - Retry and Dismiss results stay in the delivery view, with distinct feedback for requeueing versus discarding a failed delivery.
- **Alert History** - Messages have more room and wrap to two lines on narrower tables. Longer text retains its full title, with consistent row heights for scrolling.
- **Truthful alert endings** - History distinguishes an alert moving or being withdrawn from a recovered condition, instead of calling every closure a recovery.
- **Alert attribution** - Docker hosts, services and containers keep distinct alert references, including containers and services without IDs.
- **Startup notifications** - Restored backup alerts are revalidated before startup notifications are sent, rather than blindly notifying from saved state.
- **Drawer identity** - Long identity values remain readable in phone drawers instead of being cut off or repeated.
- **Storage attention** - Storage tables lead with capacity pressure and explain why a resource needs attention, rather than relying on an unexplained health pill.
- **Backup coverage** - Backup health counts explain what they count, and selecting a count filters Coverage to the matching workloads.
- **Docker details** - Container tables and drawers show health checks and uptime. Image names retain their repository and tag at narrower widths.
- **Availability checks** - Machines and Settings agree on availability-check state, and a failing check no longer leaves its node labelled online.
- **Cluster agent setup** - An authorised, covered sibling can install cluster member agents in one step, with the covered connection identified in setup.
- **Bootstrap authentication** - Non-Windows agents exchange a bootstrap token before starting monitoring modules, rather than beginning collection with an unenrolled credential.
- **Removal recovery** - On systemd, OpenRC and SysV installs, agent removal retains credentials and recovery files when it cannot confirm shutdown. It does not automatically restart after a partial stop.
- **Unattended updates** - Unattended updates preserve the recovery backup after a failed or incomplete rollback and verify the running executable before declaring success.
- **Token scope** - Token callers cannot widen or bypass their scopes. Token management and diagnostic token listings stay within the caller's authority.
- **Uninstall authority** - Non-token agent uninstall requires an administrator. Credential rotation by token requires instance-wide authority.
- **SAML sign-in** - SAML responses are bound to the browser that started sign-in, preventing an unrelated browser from consuming the response.
- **Certificate pins** - Proxmox reconnects preserve existing certificate pins. Fingerprint capture respects the configured endpoint and port rather than guessing a different authority.
- **Patrol navigation** - The Patrol mode selector supports direct links to its controls. Host filesystem alerts retain the correct target for investigation.
- **Discovery settings** - The separate AI Discovery surface has been removed. Ordinary infrastructure monitoring and agent setup remain the places to manage monitored systems.
- **Kiosk layout** - Kiosk mode hides platform tabs and filter toolbars and removes table preview caps, keeping the display focused on monitoring.
- **Workload links** - Saved web links can be edited in bulk and opened from the guest drawer header. Read-only tokens do not receive editing controls.
- **Lower temporary churn** - Complete monitoring updates avoid unnecessary temporary objects and repeated identity work. This is not a confirmed fix for a higher resident-memory baseline.

## Known issues

- This beta is not a confirmed fix for every reported guest freeze, Windows restart or higher server-memory baseline. If monitoring was stopped after a frozen guest, do not restart it just to test this beta.

## Before you upgrade

- Windows Unified Agent binaries are not Authenticode-signed while SignPath remains unavailable and may show an Unknown Publisher warning. Verify downloads with the published checksums and detached signatures.
- Pulse Mobile remains compatible. This candidate does not require a companion mobile release.
- The rollback target is stable `v6.5.0`. On systemd and Proxmox LXC installs, use this command only when `/bin/update` was installed by the Pulse server installer: `sudo /bin/update --version v6.5.0` to return to the previous stable release. For Docker Compose, pin `rcourtman/pulse:6.5.0` and recreate the container.
