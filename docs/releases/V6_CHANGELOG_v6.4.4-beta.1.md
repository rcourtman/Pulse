# Pulse v6.4.4-beta.1

This changelog describes the changes since `v6.4.1`, the latest published
stable release. The `v6.4.2` tag was staged on 2026-08-31 but never activated
as a public release. This beta therefore carries the complete `v6.4.2` change
set, the published `v6.4.3-rc.1` foundation, and the bounded release-line
regression repairs below.

## Alert delivery and lifecycle

- Resolved incidents cannot be revived by later retry work, and destinations
  disabled before dispatch are recorded as cancelled rather than delivered.
- Delivery diagnosis preserves the newest failure, dismissal, or unavailable
  state when timer, retry, and manual refresh work overlap.
- Webhook, Slack, Discord, Telegram, encoded-query, and ntfy diagnostics mask
  recognised credentials without discarding useful failure context.
- Storage and PBS alert identity persists through missing observations,
  configuration reload, restart, measured recovery, and recurrence.
- TrueNAS replication completion and NOTICE severity, PBS capacity/task
  callbacks, and pool-only or empty Unraid layouts retain their intended alert
  semantics.

## Host, update, identity, and interface reliability

- Host and Docker CPU collection use independent baselines, agent disk
  collection follows its mount namespace, and Docker update completion requires
  independent success evidence.
- Same-name systems stay separate across provider scope; delayed startup,
  reconnect, settings navigation, skip-link order, badges, landmarks, and
  compact controls retain the reviewed recovery and accessibility behavior.
- Availability-suggestion backfill derives from the current discovery under
  lock, so a concurrent manual repair remains intact and deleted records are
  not resurrected.
- Current OpenAI model requests use the supported completion-token parameter
  and may learn that requirement from bounded API responses.

## Carried from the unpublished v6.4.2 packet and RC1

- Failed backups and completed PBS-to-PBS sync copies no longer pin a guest in
  Backup Running, and incomplete artifacts are excluded from recoverable
  latest-backup pointers (#1815).
- Two standalone sites that reuse one short node name and one shared install
  token no longer collapse into a single host or Docker record (#1753).
- Windows Unified Agent auto-update no longer fails with HTTP 404; canonical
  signed `.exe` assets and detached signatures are served from release assets
  (#1820).
- Infrastructure actions and SSO sessions retain the canonical administrator
  boundary, sensitive request bodies are bounded, large Availability estates
  use the compact fleet view, slow starts offer recovery, and Disk I/O totals
  avoid numbered-partition double counting.

## Known beta limits

- Issue #1761 has a 7 September installed report that both retained-failure
  actions return HTTP 503 on stable `v6.4.1` after CSRF validation. The beta's
  integrated queue-finality and warning-reconciliation changes have not been
  tested on that reporter's installation, so the beta does not claim to fix this
  failure. Non-destructive Retry and Dismiss behavior remains an explicit test.
- Issue #1812 shows that recovery controls and delivery-attempt details were not
  discoverable from a `v6.4.1` incident timeline. The beta includes the Overview
  controls and Notifications activity view, but installed discoverability is
  unverified; broader task orchestration is outside this checkpoint.
- Issue #1966 reports 50-66 GB/day of process writes and duplicate incident IDs
  on one idle `v6.4.1` LXC. This beta does not establish reduced aggregate
  writes, migrate old duplicates, or resolve that installed report.
- A paired advisory benchmark measured UUID route-segment normalization at
  62.50 ns/op versus 51.64 ns/op (+21.04%, zero allocations). Candidate-only
  CI passed its budget, but the paired result remains adverse evidence for
  beta observation and requires a new disposition before RC.
- Permanent SMTP authentication, configuration, or rejection errors may still
  consume the existing inner retry budget before final failure.
- Ordinary off-LAN native receipt, terminated-process replay, Doze, expiry,
  and failed-WAN cases are not qualified by this server checkpoint.
- Historical notification rows are not rewritten or blindly replayed.

## Release Metadata

- Version: `v6.4.4-beta.1`
- Previous published preview: `v6.4.3-rc.1`
- Previous stable: `v6.4.1`
- Rollback target: `v6.4.1`
- Rollback command: `sudo /bin/update --version v6.4.1`
- Promotion path: exact-SHA single-build release candidate from `release/v6.4`
- Windows signing decision: prereleases publish checksum- and
  detached-signature-verified Windows agents without Authenticode while
  SignPath remains unavailable. Windows Unified Agent binaries may display an
  Unknown Publisher warning
- Mobile decision: `no-mobile-impact`. No governed mobile route, payload,
  Relay, pairing, approval, push, or onboarding contract changed from
  `v6.4.1`, so no companion mobile build or store rollout is required
