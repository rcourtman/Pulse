# Pulse v6.0.0-rc.3 Draft Release Notes

_Draft only. Do not treat this as published until the governed
`v6.0.0-rc.3` tag and GitHub prerelease exist._

`v6.0.0-rc.3` is intended to be the second corrective RC after the public
`rc.1` release and the first candidate after the `v5.1.29` maintenance sweep.
Pulse v5.1.29 remains the current stable line.

The purpose of this RC is to carry late v5 maintenance fixes and recent RC
feedback into the v6 candidate before broader retesting:

- v5-to-v6 installs and updates should avoid avoidable installer, disk-space,
  token-display, and rollback surprises
- agent identity and reconnect behavior should remain stable across Docker LXC,
  recreated containers, and Proxmox setup-script paths
- alert, backup, snapshot, and storage views should not regress from the
  current v5 maintenance line
- Workloads, Storage, and Infrastructure should be retestable with the current
  table-first and responsive UI corrections
- the support packet should be explicit about the post-`rc.2` update-signer
  transition and the stable rollback target

## Support Stance

- Pulse v5.1.29 remains the current stable line.
- Pulse v6 `rc.3` is still an opt-in evaluation build, not the default
  production recommendation.
- Existing v5 users should still prefer staging, lab, or otherwise controlled
  evaluation first.
- Hosts already pinned to the historical `rc.2` update trust root should not
  assume unattended auto-update continuity into later prerelease or GA builds.
  Use a manual reinstall or an explicit trust migration path for `rc.3`.
- The stable rollback target for this candidate is `v5.1.29`:
  `./scripts/install.sh --version v5.1.29`

## What Changed Since `rc.2`

### Installer And Update Continuity

- The stable install path remains anchored to v5.1.29 instead of accidentally
  resolving to a v6 RC from the fresh Proxmox LXC script path.
- The installer now checks available disk space before stopping the current
  service, so a low-space update should fail before interrupting a working
  install.
- The installer bundle fallback no longer depends on an external helper being
  available at the point where the fallback is needed.
- Bootstrap-token display now uses the supported `pulse bootstrap-token`
  command path instead of surfacing encrypted `.bootstrap_token` JSON.
- The update-progress modal remains dismissible after an update completes or
  fails.
- The release packet records `v5.1.29` as the governed rollback target for
  `rc.3`.

### Agent Identity, Reconnect, And Host Setup

- Docker agents running inside Proxmox LXC keep the canonical host identity and
  reconnect token binding after agent restarts or container recreation.
- The v5-to-v6 server and agent continuity path keeps stable identity files
  and avoids avoidable duplicate-host behavior.
- Proxmox setup scripts preserve existing `authorized_keys` symlinks instead
  of replacing the symlink target with a regular file.
- Agent privilege expectations are documented for RC testing, including why the
  current agent still needs the documented local privileges.
- Patrol local discovery probes are aligned with the agent command policy.

### Alerts, Backup, Snapshot, And Storage Correctness

- Disabled alert notification channels no longer continue sending cooldown
  reminders after the user turns notification delivery off.
- Docker update-alert disable cleanup and alert-threshold metric coloring match
  the configured alert state.
- PBS threshold handling is corrected for the recent v5 maintenance feedback.
- Backup orphan/template readiness and recovery inventory handling now avoid
  stale or missing-state regressions.
- Synology RAID scrub activity is gated on mdstat operation state so normal
  scrubs are less likely to look like unexpected rebuild alerts.
- Ceph monitor count handling uses monitor arrays where available.
- Proxmox snapshots are preserved for guests that are temporarily missed by a
  poll.
- PBS and host-linked ZFS filesystems merge into guest overviews where the host
  relationship is known.

### Monitoring History, AI, And Runtime Behavior

- Duplicate metrics are handled idempotently instead of creating noisy history
  rows.
- Local Ollama calls use a shorter keep-alive window so test systems are less
  likely to hold local model resources longer than needed.
- Backup, recovery, and inventory guardrails were rechecked against the late
  RC3 issue sweep.

### Workloads, Storage, And Infrastructure UI

- Workloads, Storage, and Infrastructure keep the current table-first layout,
  filter deck, saved-view, chart-toggle, and responsive wrapping fixes from the
  v6 release branch.
- On narrow/mobile Workloads views, summary charts stay out of the primary
  table flow so the table remains usable.
- Summary-chart hover tooltips now offset from the guide line instead of
  covering the exact value under the cursor.

### Security And Public Feedback Flow

- Frontend sanitization and Go network dependencies are current for this
  release branch.
- Public contribution guidance now steers unsolicited work through issues and
  discussions first while keeping the dedicated v6 RC feedback path available.
- The late open issue and discussion pass did not identify another unhandled
  narrow RC3 code blocker in the current candidate.

### Paid-Plan Wording Stays On The `rc.2` Model

`rc.3` does not reopen the self-hosted paid model settled in `rc.2`. Relay is
still described as secure remote access to the Pulse web UI,
Pulse Mobile pairing for handoff, push notifications, and 14-day history. Pro
remains Relay plus AI operations, automation, advanced admin features, and
90-day history.

## What Existing v5 Users Should Re-Test In `rc.3`

1. Server upgrade from the current v5 stable line to v6, including the manual
   or explicit trust-migration path needed for builds after `rc.2`.
2. Fresh Proxmox LXC install and rollback to v5.1.29.
3. Docker Unified Agent inside LXC after restart and container recreation.
4. Alert notification settings, cooldown behavior, PBS thresholds, Docker
   update alerts, and configured alert-threshold colors.
5. Backup, snapshot, recovery, and storage views, including Proxmox snapshots
   and PBS/ZFS filesystem attribution.
6. Workloads, Storage, and Infrastructure table layouts at desktop and mobile
   sizes, including summary-chart hover behavior.
7. AI/Patrol local discovery and Ollama-backed flows if those features are in
   use.

## Feedback

Use the `Pulse v6 pre-release feedback` issue template for regressions, upgrade
failures, licensing continuity problems, platform-specific breakage, or
actionable UX friction:

- `https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml`

When reporting an `rc.3` problem, include:

- Pulse version
- upgrade path or fresh-install path
- installation type
- whether the host was previously on `rc.1`, `rc.2`, or v5
- whether a manual reinstall or trust migration was used after `rc.2`
- what you expected
- what happened instead
- sanitized logs, screenshots, or diagnostics when helpful

## Operator References

- `docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md`
- `docs/releases/V6_CHANGELOG_RC3_DRAFT.md`
- `docs/UPGRADE_v6.md`
- `docs/AGENT_SECURITY.md`
- `docs/MIGRATION_UNIFIED_NAV.md`
