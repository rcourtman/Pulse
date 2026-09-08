# Pulse v6.4.4-beta.2 Release Notes

This second beta is a bounded alert-history and delivery-recovery checkpoint
for Preview-channel testers. It supersedes `v6.4.4-beta.1`, carries every
change from the `v6.4.2` packet that was tagged but never published, adds exact
occurrence boundaries for delayed lifecycle replay, and keeps retained-delivery
controls connected to the active notification queue after a runtime reload. It
is not an RC or stable release.

## What's improved

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

- The recurring-occurrence repair has focused race-enabled and protected-line
  CI evidence, but has not yet been verified on an installed beta through a
  restart and ordinary notification destination.
- Open issue [#1761](https://github.com/rcourtman/Pulse/issues/1761) reports
  HTTP 503 from both retained-failure actions on stable `v6.4.1` while alert
  emails continued.
- This beta repairs the reproduced stopped-queue ownership path with focused
  race-enabled tests and protected-line CI, but the reporter has not tested it.
  Do not delete queue or alert files as a workaround.
- A 7 September comment on [#1812](https://github.com/rcourtman/Pulse/issues/1812)
  says a `v6.4.1` user saw a retained-delivery warning but could not find recovery
  controls or delivery-attempt details from the incident timeline.
- This beta includes Overview controls and the Notifications activity view, but
  installed discoverability is unverified. Broader task-timeline and orchestration
  requests are outside this checkpoint.
- Open issue [#1966](https://github.com/rcourtman/Pulse/issues/1966) reports
  50-66 GB/day of Pulse process writes on one idle `v6.4.1` LXC and repeated
  incident IDs sharing one occurrence start.
- This beta has alert-lifecycle repairs but does not claim reduced aggregate
  writes, migration of old duplicates, or resolution on that installation. On
  flash-constrained systems, monitor write volume and keep the rollback pin ready.
- An advisory paired CI comparison measured UUID route-segment normalization
  at 62.50 ns/op versus 51.64 ns/op, a 21.04% increase with no allocations.
  Local comparisons measured roughly 9-11%.
- No end-user latency or throughput regression is demonstrated. Candidate-only
  benchmarking passed, but does not erase the paired result. This uncertainty is
  accepted for beta observation and needs a new disposition before RC.
- Permanent SMTP authentication, configuration, or rejection errors can still
  consume the inner retry budget before queue handling. This beta improves
  diagnosis and finality but excludes the separate main-line classification fix.
- Open issue [#1913](https://github.com/rcourtman/Pulse/issues/1913) reports
  an inaccessible GUI after changing a Docker deployment from v5.1.35 to stable
  v6.4.1.
- The report does not yet separate direct access, published ports, reverse-proxy
  behavior, WebSocket routing, or running image identity. Keep the prior image
  pin available and include those details when reporting results.
- This server beta does not require a companion mobile release. It does
  not change Relay pairing, native push payloads, approvals, or onboarding.
- Ordinary off-LAN receipt, terminated-process replay, Doze, expiry, and
  failed-WAN cases are not claimed as qualified by this beta.
- Historical notification rows are not rewritten or blindly replayed. The
  fixes govern new processing and operator retries.

## Before you upgrade

- This is an opt-in Preview-channel beta, not a stable release. Back up the
  Pulse data directory and keep the stable rollback pin available.
- The `v6.4.2` tag had no GitHub release and was not shipped. This beta
  carries its administrator-boundary and security changes with published rc.1
  and later alert fixes. The preceding stable release remains `v6.4.1`.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in
  `admin` role before upgrading so an intended administrator retains access.
- For least-privilege rootless Docker or Podman monitoring, expose exactly one
  local collector-owned runtime socket. Ambiguous or invalid sockets may fall
  back to summary-only monitoring.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath
  remains unavailable and may show an Unknown Publisher warning. Verify
  downloads with published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. For systemd and Proxmox LXC, run
  `sudo /bin/update --version v6.4.1`.
- For Docker Compose rollback, pin `rcourtman/pulse:6.4.1` and recreate the
  container.
- For Helm rollback, run `helm upgrade --install pulse
  oci://ghcr.io/rcourtman/pulse-chart/pulse --version 6.4.1` with the values
  used by the current installation.

After upgrading, please test:

- Trigger, resolve, and then retrigger the same alert. Restart Pulse or replay
  retained lifecycle work between transitions. Confirm the old occurrence stays
  resolved with its own acknowledgement history while the recurrence remains a
  separate current incident.
- Exercise a threshold alert through firing, a missing observation, Pulse
  restart, genuine recovery, and recurrence. Confirm only the appropriate
  firing and recovery notifications arrive.
- With a controlled webhook destination, return `Retry-After: 0` once and then
  an unhinted 429 or 503. Confirm the immediate override applies only once and
  the next retry resumes exponential pacing instead of hammering the endpoint.
- On a backed-up installation with retained delivery failures, try **Retry
  retained deliveries** and **Dismiss retained failures**. Confirm each succeeds
  without deleting queue or alert files and delivery history remains.
- Reload notification configuration or exercise a normal runtime restart before
  inspecting queue statistics, dead-letter details, Retry, and Dismiss. Confirm
  each operation follows the active queue rather than returning HTTP 503.
- Confirm the retained-delivery warning reconciles while another health refresh
  is in flight. If either action fails, report the HTTP status and sanitized logs.
- Retest PBS capacity and task transitions, TrueNAS replication and NOTICE
  events, host plus Docker CPU readings, and pool-only Unraid arrays.
- Run a manual service-discovery refresh while availability suggestions are
  backfilled. Confirm a repaired service keeps its type, name, URL, and engine
  version after restart.
- Inspect sanitized notification failures. Confirm recognised secrets are absent
  while the remaining error context is actionable.
- Upgrade a backed-up `v6.4.1` installation, including Docker behind a reverse
  proxy, and verify direct GUI access, proxied access, WebSocket reconnect, and
  `/api/version`.
- On high-request-rate installations, compare API latency and CPU with your
  previous pin and report any material change.
