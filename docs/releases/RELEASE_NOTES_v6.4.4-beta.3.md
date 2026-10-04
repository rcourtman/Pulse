# Pulse v6.4.4-beta.3 Release Notes

This third beta is a focused History reliability update for Preview-channel
testers. It corrects chart target selection for Proxmox nodes and Proxmox
Backup Server systems while retaining the alert and API reliability repairs
from beta.2, and carries every change from the `v6.4.2` packet that was never
published. It is not an RC or stable release.

## What's improved

- **Proxmox node History uses the right source** - API-collected nodes now use
  their canonical node coordinates instead of being redirected to an inferred
  agent identity with no matching history.
- **PBS History supports more installed shapes** - Backup Server drawers can
  use standalone PBS agent telemetry and uniquely correlated agent-bearing VM
  or container telemetry.
- **Partial telemetry stays useful** - CPU and memory History can plot when
  disk data is absent. Missing network or disk series remain visibly collecting
  instead of inventing values.
- **Ambiguous identities remain safe** - A PBS system is not attached to a
  guest or agent unless the available identity evidence selects one unique
  target.
- **Monitoring lifecycle state is synchronized** - Startup and reset paths now
  coordinate state snapshots, removing a race found while validating this
  checkpoint.
- **The beta.2 reliability repairs remain included** - Recurring alert history,
  retained-delivery recovery, webhook pacing, warning reconciliation, secret
  masking, and cold tenant startup keep their repaired behavior.
- **Provider identity remains isolated** - Same-name systems stay separate
  across Proxmox providers, including installations that reuse one short node
  name or one enrollment token.
- **Windows agent delivery is restored** - Canonical executable assets and
  detached signatures remain available to Unified Agent updates.
- **Large Availability estates scan faster** - The compact fleet view remains
  included, and Slow starts are recoverable through the existing startup
  recovery path.
- **Disk I/O totals are more accurate** - Numbered partitions are not counted
  again when their parent device already supplies the total.

## Known issues

- These History corrections have focused automated and browser checks, but
  they still need confirmation on installed Proxmox and PBS systems.
- Existing History data is not rewritten. The correction selects the canonical
  stored series that already belongs to each node or uniquely matched PBS host.
- A controlled beta.2 installation passed firing, retry, exhaustion, restart,
  recurrence, suppression, dismissal, audit, and warning-retirement checks.
  Ordinary email-provider and reporter acceptance remain unconfirmed.
- Open issue [#1966](https://github.com/rcourtman/Pulse/issues/1966) reports
  high process write volume and repeated incident IDs on one stable v6.4.1 LXC.
  This beta does not claim reduced aggregate writes or migrate old duplicates.
- Advisory comparisons remain adverse. A beta.2 study measured a slower
  metrics-store statistics handler, and this candidate repeatedly measured
  slower low-level route-segment normalization despite unchanged allocations.
  Focused full History middleware measurements did not reproduce a slowdown,
  and no user-facing latency regression has been demonstrated. Both questions
  remain open before RC. High-request-rate testers should compare latency and
  CPU with their previous pin.
- Permanent SMTP authentication, configuration, or rejection errors can still
  consume the inner retry budget before queue handling reaches final failure.
- Open issue [#1913](https://github.com/rcourtman/Pulse/issues/1913) reports an
  inaccessible GUI after an upgrade from v5.1.35 to stable v6.4.1. Keep the
  prior image pin available while its deployment-specific cause is investigated.
- This server beta does not require a companion mobile release. It does not
  change Relay pairing, native push payloads, approvals, or onboarding.

## Before you upgrade

- This is an opt-in Preview-channel beta. Back up the Pulse data directory and
  keep the stable rollback pin available.
- The v6.4.2 packet had no GitHub release and was not shipped. Its changes are
  carried through the published beta line. The preceding stable release remains
  v6.4.1.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in
  `admin` role before upgrading so intended administrator access remains.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath is
  unavailable and may show an Unknown Publisher warning. Verify downloads with
  published checksums and detached signatures.
- The rollback target is stable `v6.4.1`.
- For rollback on systemd and Proxmox LXC, run `sudo /bin/update --version
  v6.4.1`.
- For Docker Compose rollback, pin `rcourtman/pulse:6.4.1` and recreate the
  container.
- For Helm rollback, run `helm upgrade --install pulse
  oci://ghcr.io/rcourtman/pulse-chart/pulse --version 6.4.1` with the values
  used by the current installation.
- After upgrading, open 7-day History for API-only and agent-linked Proxmox
  nodes. Confirm existing CPU, memory, disk, network, and temperature series
  appear where those metrics are collected.
- Open History for standalone and PVE-hosted PBS systems. Confirm CPU and memory
  plot even without disk history, and report any drawer that remains collecting
  despite fresh Overview values.
