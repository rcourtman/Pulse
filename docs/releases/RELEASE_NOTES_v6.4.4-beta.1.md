# Pulse v6.4.4-beta.1 Release Notes

This beta is a bounded alert-reliability checkpoint for Preview-channel
testers. It supersedes `v6.4.3-rc.1`, carries every change from the `v6.4.2`
packet that was tagged but never published, and adds integrated fixes for
notification finality, delivery-warning ordering, restart-safe alert state,
provider edge cases, notification-log confidentiality, discovery refreshes,
host telemetry, update reporting, reconnect behavior, and accessibility. It is
not an RC or stable release.

## What's improved

- **Resolved alerts stay resolved** - Pending, failed, and dead-lettered firing
  notifications cannot be revived by a later retry after the incident resolves.
  Disabled destinations are cancelled instead of being reported as delivered.
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
- **The RC1 foundation remains included** - Windows agent delivery is restored,
  Large Availability estates scan faster, Slow starts are recoverable, and Disk
  I/O totals are more accurate. Resource identity, safer agent lifecycle
  operations, canonical health APIs, bounded privileged actions, and response
  size limits also remain in this checkpoint.

## What to test

- Exercise a threshold alert through firing, a missing observation, Pulse
  restart, genuine recovery, and recurrence. Confirm only the appropriate
  firing and recovery notifications arrive.
- On a backed-up installation with retained delivery failures, try both **Retry
  retained deliveries** and **Dismiss retained failures**. Confirm each request
  succeeds without deleting queue or alert files, delivery history remains, and
  the warning reconciles promptly even while another health refresh is in flight.
  If either action fails, report the HTTP status and sanitized logs.
- Retest PBS capacity and task transitions, TrueNAS replication and NOTICE
  events, host plus Docker CPU readings, and pool-only Unraid arrays.
- Run a manual service-discovery refresh while availability suggestions are
  being backfilled. Confirm a repaired service keeps its type, name, URL, and
  engine version after restart, and inspect sanitized notification failures to
  confirm secrets are absent while the error remains actionable.
- Upgrade a backed-up `v6.4.1` installation, including Docker behind a reverse
  proxy, and verify direct GUI access, proxied access, WebSocket reconnect, and
  `/api/version`.
- On high-request-rate installations, compare API latency and CPU with your
  previous pin and report any material change.

## Known issues and beta limits

- Open issue [#1761](https://github.com/rcourtman/Pulse/issues/1761) has a
  7 September report from stable `v6.4.1` where both retained-failure actions
  returned HTTP 503 after CSRF validation. This candidate has integrated source
  repairs and tests around queue finality and warning reconciliation, but it has
  not been tested on that reporter's installation and is not claimed to fix the
  503. Do not delete `notification_queue.db` or alert files as a workaround; keep
  the stable rollback pin and report sanitized action results.
- A 7 September comment on [#1812](https://github.com/rcourtman/Pulse/issues/1812)
  shows that a `v6.4.1` user could see a retained-delivery warning but could not
  find its recovery controls or delivery-attempt details from the incident
  timeline. This candidate includes the Overview controls and Notifications
  activity view, but installed discoverability is not yet verified. The broader
  task-timeline and orchestration requests are not part of this checkpoint.
- Open issue [#1966](https://github.com/rcourtman/Pulse/issues/1966) reports
  50-66 GB/day of Pulse process writes on one idle `v6.4.1` LXC, plus repeated
  incident IDs sharing one occurrence start. This beta contains alert-lifecycle
  repairs but does not claim reduced aggregate write bytes, migration of old
  duplicates, or resolution on that installation. On flash-constrained test
  systems, monitor Pulse write volume and keep the stable rollback pin ready.
- An advisory paired CI comparison measured UUID route-segment normalization at
  62.50 ns/op versus 51.64 ns/op, a 21.04% increase with no allocations. Local
  comparisons measured roughly 9-11%, and no end-user latency or throughput
  regression has been demonstrated. The integrated candidate-only benchmark
  passed, but that does not erase the paired result. This uncertainty is
  accepted for beta observation only and must be resolved or dispositioned
  again before RC.
- Permanent SMTP authentication, configuration, or rejection errors can still
  consume the existing inner retry budget before queue handling, causing
  avoidable delay. This beta improves diagnosis and queue finality but does not
  include the separate main-line retry-classification repair.
- Open issue [#1913](https://github.com/rcourtman/Pulse/issues/1913) reports an
  inaccessible GUI after changing a Docker deployment from v5.1.35 to stable
  v6.4.1. It does not yet distinguish direct access, published ports,
  reverse-proxy behavior, WebSocket routing, or exact running image identity.
  Keep the prior image pin available and include those details when reporting
  results.
- Pulse Mobile does not require a companion mobile release. This server beta
  does not change Relay pairing, native push payload fields, approvals, or
  onboarding contracts. Ordinary off-LAN receipt, terminated-process replay,
  Doze, expiry, and failed-WAN cases are not claimed as qualified by this beta.
- Historical notification rows are not rewritten or blindly replayed. The
  fixes govern new processing and operator retries.

## Before you upgrade

- This is an opt-in Preview-channel beta, not a stable release. Back up the
  Pulse data directory and keep the stable rollback pin available.
- The `v6.4.2` tag was never accompanied by a GitHub release and was not
  shipped. This beta carries that packet's administrator-boundary and security
  changes together with the published rc.1 and subsequent alert fixes. The
  preceding published stable release remains `v6.4.1`.
- On an SSO-only deployment, map at least one trusted IdP group to the built-in
  `admin` role before upgrading so an intended administrator retains access.
- For least-privilege rootless Docker or Podman monitoring, expose exactly one
  local collector-owned runtime socket. Ambiguous or invalid sockets may fall
  back to summary-only monitoring.
- Windows Unified Agent binaries are not Authenticode-signed while SignPath
  remains unavailable and may show an Unknown Publisher warning. Verify
  downloads with published checksums and detached signatures.
- The rollback target is stable `v6.4.1`. For systemd and Proxmox LXC, run
  `sudo /bin/update --version v6.4.1`. For Docker Compose, pin
  `rcourtman/pulse:6.4.1` and recreate the container. For Helm, run
  `helm upgrade --install pulse oci://ghcr.io/rcourtman/pulse-chart/pulse
  --version 6.4.1` with the values used by the current installation.
