# Pulse v6.4.5-beta.2 Release Notes

This opt-in Preview-channel beta combines monitoring, notification,
agent and installation reliability improvements with the fixes below. It follows
`v6.4.5-beta.1` and carries every change from the `v6.4.2` packet that was never
published. It is not a stable release. Keep stable `v6.4.1` available for rollback.

## What's improved

- **Creating tenant API tokens avoids inventory startup** - Session-authenticated
  token creation no longer starts a tenant monitor. Organization membership,
  lifecycle, CSRF, token ownership and scopes remain enforced.
- **Tenant inventory starts with one provider fill** - Initial resource-store
  wiring batches supplemental providers while preserving synchronous readiness.
- **Notification severity preferences stay saved** - Email and webhook settings
  preserve “Warnings and critical alerts” through save, reload and display (#2069).
- **Fewer unchanged alert checkpoint writes** - Byte-identical pending-intent
  checkpoints are not rewritten. Changed state still persists and stale files are
  repaired. This does not resolve all reported write volume (#1966).
- **More disks remain visible** - Disks without a serial number keep distinct
  inventory identities (#2076).
- **All disk mounts stay reachable** - The Disks card no longer clips its mount
  list inside a nested scroller. Mounts remain in the outer scrolling flow,
  including keyboard access to the final mount (#2051).
- **Container image checks tolerate registry differences** - Digest discovery
  can fall back to a validated manifest response when HEAD lacks a digest (#2048).
- **VM-linked agent memory is used correctly** - Fresh available agent memory can
  supply the linked VM reading without confusing provider instances (#1962).


- **TrueNAS CORE snapshots survive legacy alert shapes** - Non-object arguments
  no longer abort snapshots (#2077). Messages stay intact. Only objects supply
  disk identity and SMART data. Malformed JSON and HTTP failures remain errors.
- **Proxmox node History selects the right series** - Existing canonical node
  series are selected for API-only and agent-linked nodes.
- **PBS History supports more installed shapes** - Standalone and PVE-hosted
  PBS history can select available CPU and memory without requiring disk history.
- **Statistics stay available during History bursts** - The release line retains
  statistics connection isolation from long-running History reads.

- **Recurring incidents keep their own history** - Delayed firing,
  resolution, or acknowledgement replay for an older occurrence cannot reopen
  it, close a newer recurrence, or merge both occurrences into one timeline.
- **Resolved alerts stay resolved** - Pending, failed, and dead-lettered firing
  notifications cannot be revived by a later retry after the incident resolves.
  Disabled destinations are cancelled instead of being reported as delivered.
- **Retained-delivery recovery survives reloads** - Queue statistics,
  dead-letter details, **Retry retained deliveries**, and **Dismiss retained
  failures** now follow the active notification queue instead of keeping a
  stopped queue that returns HTTP 503.
- **Webhook retries keep safe pacing** - A response-specific `Retry-After: 0`
  can permit one immediate retry without erasing the exponential delay for later
  unhinted 429 or 503 responses.
- **Delivery warnings show the newest truth** - Concurrent timer, retry,
  dismissal, and refresh work can no longer let an older healthy snapshot hide
  a newer delivery failure or resurrect a warning that was just cleared.
- **Delivery diagnosis is more actionable** - Failure status and reason
  ordering remain current after retries and refreshes, unavailable queue health
  is visible, and operators are directed to the affected notification settings.
- **Notification diagnostics keep credentials private** - Webhook userinfo,
  Slack and Discord paths, Telegram bot tokens, encoded query credentials, and
  ntfy transport URLs are masked while useful status and destination context is
  retained.
- **Cold tenants no longer stall unrelated API traffic** - Resource-store
  construction now runs outside the shared cache lock while same-tenant opens
  remain deduplicated and eviction waits safely for in-flight construction.
- **Recovery requires real observations** - Storage and PBS incidents do not
  clear merely because a metric or provider sample is absent. Incident identity
  and history survive configuration reloads and restarts until measured recovery.
- **Provider alert semantics are more accurate** - TrueNAS CORE 12 endpoints
  are accepted, completed replication is recognised, informational events stay
  out of acknowledgement work, NOTICE events remain actionable, and critical
  transitions can notify correctly.
- **PBS behavior is restart-safe** - Capacity, task, partial-metric, and webhook
  paths remain bound to actual observations across restart and recurrence.
- **Host telemetry is more dependable** - Host and Docker CPU collectors use
  separate sampling baselines, disk collection follows the agent mount
  namespace, and pool-only or empty Unraid layouts no longer manufacture parity
  warnings.
- **Updates fail more truthfully** - Docker updates require independent success
  evidence, oversized GitHub release metadata falls back to verified assets,
  and portable installer state retains correct ownership.
- **Identity and reconnect behavior is safer** - Same-name systems stay
  separate across Proxmox providers, delayed browser startup offers a truthful
  retry path, and infrastructure edits stay mounted during background polling.
- **Discovery repairs stay repaired** - Availability-suggestion backfill no
  longer writes an old discovery snapshot over a concurrent manual refresh,
  drops the repaired URL or engine version, or resurrects a deleted record.
- **Navigation and settings are more accessible** - The skip link is first in
  keyboard order, badges remain readable, landmarks are distinct, and settings,
  Patrol, mobile navigation, and compact controls have stronger focus and
  screen-reader behavior.
- **Current OpenAI models are handled correctly** - GPT-5-family requests use
  the supported completion-token parameter and can learn the required parameter
  from bounded API responses.
- **The RC1 reliability fixes remain included** - Windows agent delivery is
  restored, Large Availability estates scan faster, Slow starts are recoverable,
  and Disk I/O totals are more accurate.
- **The safer operational foundation remains included** - Resource identity,
  safer agent lifecycle operations, canonical health APIs, bounded privileged
  actions, and response size limits remain in this checkpoint.

## Known issues

- Switching repeatedly into newly created organizations can leave mobile Safari
  settings waiting for capabilities or inventory. Sharing and billing navigation
  remain under investigation. Passing token tests do not establish that this
  transition is fixed. Avoid treating a loading page as a completed switch.
- TrueNAS appliance acceptance and installed Proxmox/PBS History confirmation
  remain outstanding. Existing history is not rewritten.
- Aggregate high write volume and old duplicate incidents reported in #1966
  remain unresolved. Keep backups and monitor write activity on flash storage.
- Some v5-to-v6 deployment-specific upgrade problems remain unresolved (#1913).
- Notification delivery still depends on destination configuration and provider
  responses. Do not delete alert or queue files as a workaround.
- This server release does not change mobile pairing, push payloads, approvals
  or onboarding and does not require a companion mobile release.

## Before you upgrade

- Use published, integrity-verified artifacts and back up the complete Pulse data
  directory and configuration before opting into Preview.
- On SSO-only deployments, map at least one trusted IdP group to the built-in
  `admin` role so intended administrator access remains available.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath is
  unavailable and may show an Unknown Publisher warning. Verify published
  checksums and detached signatures.
- The rollback target is stable `v6.4.1`. For systemd and Proxmox LXC, run
  `sudo /bin/update --version v6.4.1`.
- For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
- For Helm, use chart version `6.4.1` with your saved values.
- Keep the pre-upgrade backup until health and data continuity are verified.
  Restore it if rollback cannot read the upgraded data safely.
- After upgrading, verify inventory, organization switching, repeated snapshots,
  History and notification delivery against your actual installed systems.
