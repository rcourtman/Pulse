# Pulse v6 RC3 Draft Operator Support Pack

_Draft only. Use this as the working support brief for the planned
`v6.0.0-rc.3` candidate until the final prerelease notes are published._

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.3` is an opt-in evaluation build, not the default production
  recommendation.
- `rc.3` should be described as a broad hardening RC with a corrective
  maintenance core: it carries late v5 maintenance fixes, current RC feedback,
  release packaging hardening, security/auth tightening, and post-`rc.2`
  readiness work into the v6 candidate before broader retesting.
- Systems pinned to the historical `rc.2` update trust root should use a manual
  reinstall or explicit trust migration for later prerelease or GA builds.

## Short Answers

### Is `rc.3` the stable release?

No. The current stable release is v5.1.29. `rc.3` is still a v6 prerelease for
controlled evaluation.

### Should production v5 users upgrade immediately?

No. The recommended RC posture is still staging, lab, or controlled evaluation
first.

### What is the rollback target?

Use v5.1.29:

`./scripts/install.sh --version v5.1.29`

### Can `rc.2` systems auto-update directly to `rc.3`?

Do not promise unattended continuity from `rc.2` to `rc.3`. Hosts pinned to the
historical `rc.2` update trust root need a manual reinstall or explicit trust
migration path for later prerelease or GA builds.

### Does self-hosted v6 still cap monitored systems?

No for the current public self-hosted plans. Community, Relay, and Pro include
core self-hosted monitoring by default, as described in the `rc.2` support
packet.

Current plan shorthand:

- Community:
  core monitoring included, 7-day history
- Relay:
  core monitoring included, secure remote access to the Pulse web UI,
  Pulse Mobile pairing for handoff, push notifications, and 14-day history
- Pro:
  Relay plus AI operations, automation, advanced admin features, and 90-day
  history

### What happens to existing paid Pulse Pro customers in v6?

Use this cohort breakdown:

- Legacy recurring monthly or annual subscribers from v5 or earlier who were
  already active before the public v6 pricing cutover:
  keep the current recurring price, with self-hosted monitoring and
  child-resource volume not metered while the subscription remains continuously
  active under the current v6 policy.
- Existing lifetime customers:
  remain permanently valid, with self-hosted monitoring and child-resource
  volume not metered under the current v6 policy.
- Legacy paid v5 licenses migrated into v6 outside the recurring grandfathered
  path:
  can still exchange into the v6 activation model without repurchasing.
- Former recurring subscribers who already canceled or later lapse:
  any later return uses current public v6 pricing rather than resuming the old
  grandfathered terms.

### What changed from `rc.2` that users should notice immediately?

- release artifact validation, draft metadata preservation, upload retries,
  signing, and clean version metadata are hardened for the RC path
- stable install paths stay on v5.1.29 unless the user explicitly opts into v6
- installer disk preflight runs before stopping the current service
- bootstrap-token display uses the supported command path
- Docker agents in Proxmox LXC keep host identity after restart/recreation
- alert notification disablement, thresholds, and PBS/Docker alert paths are
  corrected
- backup, snapshot, and linked filesystem views carry the late v5 fixes
- Storage summary tiles wrap cleanly in the shared sticky summary grid
- Workloads summary-chart tooltips no longer cover the guide line
- skip-auth local/dev login handles the expected unauthenticated response
  without surfacing a request failure

### What if a fresh Proxmox LXC stable install lands on a v6 RC?

Treat that as a release-blocking install regression. The stable path should
resolve to v5.1.29 unless the user intentionally chose a v6 prerelease.

### What if a bootstrap-token command shows encrypted JSON?

Treat that as a bug. Operators should use the supported `pulse bootstrap-token`
command path, and the installer/support flow should not ask them to decode the
encrypted `.bootstrap_token` file.

### What if a Docker agent duplicates or loses identity after recreation?

Collect logs and escalate. `rc.3` includes the canonical reconnect-token and
host-identity binding fix for Docker agents inside Proxmox LXC.

### What if alert behavior still looks wrong?

Ask for the alert rule, delivery-channel state, threshold values, current Pulse
version, and screenshots or logs. `rc.3` includes fixes for disabled
notification cooldowns, Docker update-alert disable cleanup, configured
threshold colors, and PBS threshold handling.

### What if issue `#1451` still reports a restore error after `rc.3`?

Check whether the host still has the stale legacy
`/run/pulse-sensor-proxy` LXC mount from an old install path. The token-format
part of the report is covered by the current installer path; a stale host mount
is an environment cleanup item unless it reproduces on a clean `rc.3` install.

### Are public issue comments or closures required for the RC?

No public GitHub state changes are required just to prepare this packet. Draft
comments, closures, or retitles still need explicit maintainer approval before
posting.

## Recommended Evaluation Path

1. Back up the current system and keep direct console access available.
2. Confirm the current stable rollback command:
   `./scripts/install.sh --version v5.1.29`
3. If the host was on `rc.2`, use a manual reinstall or explicit trust
   migration path rather than assuming unattended auto-update continuity.
4. Upgrade the Pulse server in a staging or otherwise controlled environment.
5. Verify server health, version, logs, and update UI before upgrading agents.
6. Re-test Docker Unified Agent inside Proxmox LXC after restart and container
   recreation.
7. Re-test alerts, backup/recovery, snapshots, PBS/ZFS attribution, Synology
   RAID scrub handling, and Ceph inventory.
8. Re-test Workloads, Storage, and Infrastructure at desktop and mobile sizes.
9. Re-test release artifact download, checksum/signature, installer, and draft
   validation paths before broader retesting.
10. Upgrade agents separately only when the user is explicitly testing the
   v5-to-v6 agent path.

## Ask For These Details

When a user reports an `rc.3` problem, ask for:

- current version and prior version
- install type
- whether the host was previously on v5, `rc.1`, or `rc.2`
- whether a manual reinstall or trust migration was used after `rc.2`
- whether the issue happened during server upgrade, agent upgrade, alerting,
  backup/recovery, platform inventory, or first use
- whether Unified Agents were upgraded yet
- expected result
- actual result
- sanitized logs, screenshots, and diagnostics

## Escalate Immediately

Escalate without asking the user to keep experimenting when the report involves:

- failed install or failed upgrade with no recovery path
- stable install path unexpectedly landing on a v6 prerelease
- duplicate or missing agent identity after a v5-to-v6 upgrade
- Docker LXC agent reconnect-token or identity loss after recreation
- monitoring or reporting that stops entirely after upgrade
- alert storms after notifications were explicitly disabled
- rollback failure or inability to return to v5.1.29
- data-loss, destructive behavior, or security-sensitive regressions

## Canonical References

- `docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC3_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
