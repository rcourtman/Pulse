# Pulse v6.4.5-beta.1 Release Notes

This opt-in Preview-channel beta fixes TrueNAS CORE snapshot refresh when a
REST alert has non-object arguments. It retains the release-line History and
reliability fixes and carries every change from the `v6.4.2` packet that was
never published. The previous published preview is `v6.4.4-beta.2`; the stable
rollback target remains `v6.4.1`. This is not an RC or stable release.

## What's improved

- **TrueNAS CORE snapshots survive legacy alert shapes** - String, array,
  numeric, boolean and null alert arguments no longer abort collection (#2077).
  Formatted messages remain intact. Only object fields supply disk identity
  and SMART measurements; malformed JSON and failed HTTP responses remain errors.
- **Proxmox node History selects the right series** - Existing canonical node
  series are selected for API-only and agent-linked nodes.
- **PBS History supports more installed shapes** - Standalone and PVE-hosted
  PBS history can select available CPU and memory without requiring disk history.
- **Statistics stay available during History bursts** - The release line retains
  statistics connection isolation from long-running History reads.

### Inherited reliability improvements

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

- The TrueNAS fix has synthetic repeated-snapshot and race-enabled regression
  coverage. Physical-appliance and reporter acceptance remain unconfirmed.
- History corrections still need confirmation on installed Proxmox and PBS systems.
  Existing stored history is not rewritten. Active long-running reads may still
  extend shutdown; cancellation and fault-injection coverage remain incomplete.
- Previous unpublished preview candidates did not complete qualification. This
  version number is not evidence that their stability failures were resolved.
- Performance risk remains open: earlier main-line comparisons measured slower
  numeric-ID and UUID route normalization. Passing release-line source benchmarks
  do not establish installed workload performance or invalidate earlier results.
- Alert delivery depends on destination configuration and provider responses.
  Reporter acceptance, ordinary email-provider behavior and retained-delivery
  discoverability remain incomplete. Do not delete queue or alert files as a workaround.
- Issue #1966 reports high write volume and repeated incident IDs on a stable
  installation. No reduction in aggregate writes or migration of old duplicates
  is claimed. Keep the rollback pin ready on flash-constrained systems.
- Issue #1913 reports inaccessible GUI after an upgrade from v5.1.35 to v6.4.1.
  Deployment-specific upgrade failures remain outside this fix.
- This server preview does not change mobile pairing, push payloads, approvals
  or onboarding and does not require a companion mobile release.

## Before you upgrade

- Use only published, integrity-verified artifacts. Back up the Pulse data
  directory and configuration before opting into Preview.
- The v6.4.4-beta.3 and beta.4 candidates were not published; beta.5 was withdrawn.
  Their version numbers must not be treated as available upgrade targets.
- On SSO-only deployments, map at least one trusted IdP group to the built-in
  `admin` role so intended administrator access remains available.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath is
  unavailable and may show an Unknown Publisher warning. Verify downloads with
  published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. On systemd and Proxmox LXC, run
  `sudo /bin/update --version v6.4.1`.
- For Docker Compose, pin `rcourtman/pulse:6.4.1` and recreate the container.
- For Helm, run `helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart/pulse --version 6.4.1`
  with the installation's saved values.
- Keep the pre-upgrade backup until health and data continuity are verified;
  restore it if rollback cannot read the upgraded data safely.
- Verify repeated TrueNAS snapshots and alert messages after upgrading. Check
  7-day History for Proxmox nodes and PBS systems, and confirm expected series
  appear where the provider collects those metrics.
