# Pulse v6.0.0-rc.3 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the planned `v6.0.0-rc.2` candidate. Do not treat it as published until
the governed `v6.0.0-rc.3` prerelease exists._

## What `rc.3` changes compared with `rc.2`

`v6.0.0-rc.3` is a corrective maintenance RC. It carries the late v5.1.29
maintenance sweep and the current RC feedback fixes into the v6 release branch
without reopening the product model that `rc.2` settled.

The main release risk addressed here is regression drift: fixes that users
already received on the v5 stable line should not disappear when they evaluate
v6.

## Major Changes

### 1. Installer and rollback behavior is safer for RC retesting

The release branch now keeps the stable install/update path anchored to
v5.1.29 while preparing the `rc.3` prerelease packet.

The installer changes in this candidate include:

- Proxmox LXC stable installs do not accidentally fall through to a v6 RC
- low-disk updates fail before stopping the current service
- installer bundle fallback logic works without relying on a missing external
  helper
- bootstrap-token display uses the supported `pulse bootstrap-token` path
- update progress remains closable
- the governed rollback command for this candidate is
  `./scripts/install.sh --version v5.1.29`

There is also an important prerelease update note: systems pinned to the
historical `rc.2` update trust root should use a manual reinstall or explicit
trust migration for later prerelease and GA builds. Do not assume unattended
`rc.2` to `rc.3` continuity.

### 2. Agent identity and setup paths are less fragile

`rc.3` carries the v5/v6 agent continuity corrections from the late issue
sweep:

- Docker agents inside Proxmox LXC preserve their canonical host identity and
  reconnect token binding after restart or recreated container flows
- v5-to-v6 agent identity files remain stable instead of creating avoidable
  duplicate hosts
- Proxmox setup scripts preserve existing `authorized_keys` symlinks
- the agent privilege model is documented for operators
- Patrol discovery probes align with the agent command policy

### 3. Alerts, recovery, and storage views match late v5 fixes

The candidate includes the alert and recovery correctness fixes from the
maintenance audit:

- disabled notification channels stop sending alert cooldown reminders
- Docker update-alert disable cleanup is retained
- alert-threshold colors respect configured thresholds
- PBS threshold behavior is corrected
- backup orphan/template readiness is guarded
- Synology mdstat operation state gates RAID rebuild alerts during scrubs
- Ceph monitor counts come from monitor arrays where available
- Proxmox snapshots are preserved across transient poll misses
- host-linked PBS/ZFS filesystems merge into guest overviews where ownership is
  known

### 4. Runtime history and AI behavior are quieter

The duplicate-metrics path is idempotent, reducing noisy historical rows during
polling overlap. Local Ollama-backed AI calls also use a shorter keep-alive
window so evaluation systems are less likely to retain model resources
unnecessarily.

### 5. Workloads, Storage, and Infrastructure are ready for another UI pass

The current branch keeps the table-first v6 operational UI corrections:

- Workloads, Storage, and Infrastructure have the current filter, saved-view,
  table, chart-toggle, and responsive wrapping fixes
- narrow Workloads layouts keep charts out of the primary mobile table flow
- summary-chart hover tooltips now offset away from the guide line instead of
  covering the value being inspected

### 6. Security dependencies and contribution guidance are current

The frontend sanitizer and Go network dependency updates are present on the
release branch. Public contribution guidance now matches the issue-first
policy for the current RC cycle while preserving the dedicated v6 feedback
template.

The self-hosted paid wording remains aligned with `rc.2`: Relay is secure
remote access to the Pulse web UI, Pulse Mobile pairing for handoff, push
notifications, and 14-day history, while Pro adds AI operations, automation,
advanced admin features, and 90-day history.

## What existing v5 users should re-test in `rc.3`

1. v5.1.29 to v6 server upgrade and rollback to v5.1.29.
2. The explicit post-`rc.2` trust migration or manual reinstall path.
3. Docker Unified Agent in Proxmox LXC after agent restart and container
   recreation.
4. Alert notification disablement, cooldown behavior, thresholds, and PBS/Docker
   alert paths.
5. Backup, snapshot, recovery, PBS, ZFS, Synology RAID, and Ceph inventory
   views.
6. Workloads, Storage, and Infrastructure at desktop and mobile sizes.
7. AI/Patrol local discovery and Ollama-backed flows where enabled.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the current
release line, see:

- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-blocked-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-rc3-followup-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-v5-129-delta-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-late-issue-intake-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-summary-sparkline-tooltip-2026-05-01.md`
