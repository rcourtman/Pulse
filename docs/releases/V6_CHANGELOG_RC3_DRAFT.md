# Pulse v6.0.0-rc.3 Draft Changelog

_Draft only. This changelog describes the current `pulse/v6-release` delta
since the `v6.0.0-rc.2` tag. Do not treat it as published until
the governed `v6.0.0-rc.3` prerelease exists._

## What `rc.3` changes compared with `rc.2`

`v6.0.0-rc.3` is a broad hardening RC with a corrective maintenance core. It
carries the late v5.1.29 maintenance sweep and the current RC feedback fixes
into the v6 release branch while also preserving the post-`rc.2` release
packaging, security, hosted/mobile, commercial-model, governance, and UI
readiness work that landed on the release line.

The main release risk addressed here is regression drift: fixes that users
already received on the v5 stable line should not disappear when they evaluate
v6. The secondary risk is release drift: the candidate should publish with the
same signed-artifact, validation, auth, UI, and documentation behavior that was
tested after `rc.2`.

## Commit Coverage Audit

The changelog was audited against every commit in the exact release range for
the current candidate head:

- `v6.0.0-rc.2`: `2868b44cf91b59bca85cd886711d78cd3c376fab`
- candidate commit: `158d65ccdb81077c35b9237a1652b2774ddb5d5c`
- range: `v6.0.0-rc.2..158d65ccdb81077c35b9237a1652b2774ddb5d5c`
- commit count: `605`
- changed scope: `1766` files, `113798` insertions, `72729` deletions

Those commits are grouped in this changelog rather than listed one by one. The
range includes release/install/update work, security and trust-boundary
hardening, commercial and hosted-account cleanup, infrastructure and agent
platform work, monitoring/storage/recovery corrections, AI/Patrol/action
governance, mobile/hosted proof, SSO entitlement and provider-settings
cleanup, documentation/governance records, and frontend layout and
product-surface polish.

## Major Changes

### 1. Release packaging, install/update, and rollback behavior are safer for RC retesting

The release branch now keeps the stable install/update path anchored to
v5.1.29 while preparing the `rc.3` prerelease packet.

The release and installer changes in this candidate include:

- signed installer downloads, signed release sidecars, SBOM publication, and
  release-asset validation gates
- draft-release validation that preserves GitHub draft metadata
- bounded release-asset upload retries so transient GitHub upload failures do
  not leave a partially populated RC draft
- clean VCS metadata inside released container images and release builds
- Proxmox LXC stable installs do not accidentally fall through to a v6 RC
- stable installer resolution ignores prerelease-shaped tags and downloads the
  installer from the latest stable release asset instead of trusting GitHub's
  floating latest-release redirect when an RC is current
- mock metrics store downsample proof now compares seeded history against the
  canonical bucket average used by the backend query path, avoiding
  time-of-hour-dependent backend race-test failures during RC validation
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

### 2. Security, auth, and trust boundaries are tighter

The post-`rc.2` range includes a concentrated hardening pass across the
release, hosted, local, and agent boundaries:

- workflow token permissions, CI image pins, Docker base-image digests, and
  release asset signing are restricted
- update, installer, bootstrap, and self-update token paths keep sensitive
  values out of logs and process arguments
- setup tokens, recovery/bootstrap auth, non-loopback local transport, trusted
  proxy configuration, websocket origins, and outbound HTTP helpers fail closed
  where the earlier behavior was too permissive
- org invitation, ownership transfer, hosted callback, purchase return,
  webhook, and license persistence paths were hardened
- skip-auth login handling now treats the expected 401 as a local-mode auth
  state instead of surfacing it as a client failure
- SAML login rejects unsupported HTTP methods explicitly, and SSO provider
  settings expose the OIDC groups-claim field used by allowed groups and role
  mapping

### 3. Agent identity and setup paths are less fragile

`rc.3` carries the v5/v6 agent continuity corrections from the late issue
sweep:

- Docker agents inside Proxmox LXC preserve their canonical host identity and
  reconnect token binding after restart or recreated container flows
- v5-to-v6 agent identity files remain stable instead of creating avoidable
  duplicate hosts
- Proxmox setup scripts preserve existing `authorized_keys` symlinks
- the agent privilege model is documented for operators
- Patrol discovery probes align with the agent command policy

### 4. Alerts, recovery, and storage views match late v5 fixes

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

### 5. Runtime history and AI behavior are quieter

The duplicate-metrics path is idempotent, reducing noisy historical rows during
polling overlap. Local Ollama-backed AI calls also use a shorter keep-alive
window so evaluation systems are less likely to retain model resources
unnecessarily.

### 6. Workloads, Storage, and Infrastructure are ready for another UI pass

The current branch keeps the table-first v6 operational UI corrections:

- Workloads, Storage, and Infrastructure have the current filter, saved-view,
  table, chart-toggle, and responsive wrapping fixes
- narrow Workloads layouts keep charts out of the primary mobile table flow
- Storage summary tiles keep their labels and values inside the shared sticky
  summary grid at narrow widths
- summary-chart hover tooltips now offset away from the guide line instead of
  covering the value being inspected

### 7. Commercial, hosted, and mobile readiness stays aligned with the `rc.2` model

The range keeps the `rc.2` free-first self-hosted direction while removing
stale sales, trial, and cap-era assumptions from runtime and public docs:

- self-hosted core monitoring remains included on the current public plans
- OIDC, SAML, and multi-provider SSO are included Community-tier capabilities,
  with `advanced_sso` retained only as a compatibility entitlement key
- Relay remains secure remote access to the Pulse web UI. Relay includes
  Pulse Mobile pairing for handoff, push notifications, and 14-day history
- Pro remains Relay plus AI operations, automation, advanced admin features,
  RBAC, audit logging, reporting, agent profiles, and 90-day history
- inactive self-hosted upsell, trial-start, quickstart, and customer-side
  commercial analytics paths are retired or hidden from the public runtime
- hosted signup, Pulse Account, tenant/workspace, MSP, mobile approval, and
  mobile companion-role proofs were refreshed for the RC floor

### 8. Data, resource, action, platform, and fleet governance reached the RC floor

The post-`rc.2` release line also includes the governed v6 architecture slices
that are now part of the RC candidate:

- policy-aware data handling and AI resource sanitization
- resource-change envelopes, relationship-aware timelines, and the resource
  relationship map
- approval-backed action plans, action-audit preflight, resource action
  history, and command-policy alignment
- platform support-floor projection and admitted-platform classification
- fleet-governance projection for enrollment, liveness, version drift, adapter
  health, credential posture, update posture, and remote-control posture

### 9. Dependencies, docs, and contribution guidance are current

The frontend sanitizer and Go network dependency updates are present on the
release branch. Public contribution guidance now matches the issue-first
policy for the current RC cycle while preserving the dedicated v6 feedback
template. The release packet also reflects the Dashboard retirement,
Infrastructure-first routing, and current first-session/setup documentation
instead of leaving readers on older `rc.2` assumptions.

## What existing v5 users should re-test in `rc.3`

1. v5.1.29 to v6 server upgrade and rollback to v5.1.29.
2. The explicit post-`rc.2` trust migration or manual reinstall path.
3. Docker Unified Agent in Proxmox LXC after agent restart and container
   recreation.
4. Alert notification disablement, cooldown behavior, thresholds, and PBS/Docker
   alert paths.
5. Backup, snapshot, recovery, PBS, ZFS, Synology RAID, and Ceph inventory
   views.
6. Workloads, Storage, and Infrastructure at desktop and mobile sizes,
   including sticky summary tiles, filter wrapping, saved views, and chart
   hover behavior.
7. Skip-auth local/dev login flows where enabled.
8. OIDC and SAML SSO provider creation, login, allowed groups, and role
   mapping flows.
9. AI/Patrol local discovery and Ollama-backed flows where enabled.
10. Release artifact download, checksum/signature, and installer validation
   paths before broad retesting.

## Evidence Appendix

For the code-backed evidence packet that maps these claims to the current
release line, see:

- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-blocked-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-rc3-followup-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-v5-129-delta-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-late-issue-intake-2026-05-01.md`
- `docs/release-control/v6/internal/records/known-rc-issue-closure-for-ga-summary-sparkline-tooltip-2026-05-01.md`
- `docs/release-control/v6/internal/records/documentation-currentness-and-legacy-cleanup-rc3-commit-audit-2026-05-03.md`
