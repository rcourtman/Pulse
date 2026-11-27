# Set up alerting

Pulse can notify you about infrastructure problems without keeping the dashboard
open. Core monitoring and threshold-based alerts are included in Community.
An LLM is optional: [Patrol](AI.md#pulse-patrol) adds scheduled analysis, while
Pro adds investigations and governed fixes.

Start with one resource and one notification destination. A saved rule,
an active incident and a delivered message are three separate things to check.

## Choose what needs attention

Open **Alerts → Thresholds**, choose the platform and resource, and review the
units and inherited defaults before changing a rule. Save Changes, then reload
to confirm the saved value. A resource override takes precedence over its group
default. Turning a metric rule Off leaves its readings visible.

Usage alerts have separate trigger and clear values to avoid repeatedly opening
and closing around one threshold. Evaluation windows and activation or recovery
delays can also apply. See [thresholds and inheritance](CONFIGURATION.md#metric-thresholds-off-and-inheritance)
for the exact semantics. Do not lower a threshold or interrupt a workload just
to make a test alert fire.

### Built-in provider health alerts

Some provider health conditions raise alerts automatically while alerting is
enabled, even though they do not appear as configurable rows under
**Alerts → Thresholds**.

For Ceph clusters monitored through Proxmox or a host agent, `HEALTH_WARN`
opens a warning alert and `HEALTH_ERR` opens a critical alert after two
consecutive degraded observations. The alert clears after two consecutive
`HEALTH_OK` observations, and its message includes the active Ceph health check
codes and summaries reported by the cluster. These alerts use normal alert
history, acknowledgement, snooze, quiet hours and notification routing, subject
to each destination's minimum severity. Because built-in Ceph cluster health
alerts carry no resource tags, destinations with a tag filter will not match
them. Only Ceph pool capacity thresholds are configured under
**Alerts → Thresholds**.

## Choose where notifications go

Open **Alerts → Notifications** to configure email or a webhook destination,
including supported chat and notification services. Check that notifications
and the destination are enabled. For ordinary firing alerts, the destination's
minimum severity and resource-tag filter must both match.

Use the destination's Test to check its connection. A successful Test sends
directly and bypasses normal alert routing, schedules and the persistent
delivery queue. It does not prove that an ordinary incident will reach you.

For an incident that occurs naturally, check all three steps:

1. In **Alerts → Overview**, find the resource, incident and current reading.
2. In **Alerts → Notifications**, inspect **Recent delivery activity** for the
   matching destination. Distinguish a policy hold from a failed delivery.
3. Confirm that the intended recipient received the message. Provider
   acceptance alone does not establish receipt or that a person read it.

TLS certificate trust, expiry, hostname, and HTTP-on-HTTPS errors stop
immediately as terminal failures across email, webhook, and Apprise
destinations instead of retrying across the delivery queue or SMTP retry
budget. Fix the server certificate or TLS settings rather than disabling
verification. Email and webhook destinations do not provide a Skip TLS Verify
toggle.

To customize how alert emails appear, expand **Show advanced options** in the
email settings form to open **Email branding & subject**. You can set a
**Subject prefix** (up to 64 characters, default `[Pulse Alert]`), a
**Brand name** (up to 80 characters, default `Pulse Monitoring`), and an
optional plain-text **Footer note** (up to 240 characters). Pulse strips line
breaks and control characters, collapses extra whitespace, and HTML-escapes
those fields across single, grouped, resolved, and **Send test email**
messages. Leave a field blank to restore the default. Raw HTML and custom email
template files are not supported. Editing email branding fields does not change
the email destination continuity fingerprint.

Saved SMTP passwords and Apprise API keys are write-only and stay preserved
only while editing the same destination. Changing the SMTP server host or port,
or turning off TLS or STARTTLS encryption on an encrypted SMTP configuration,
requires entering the SMTP password again for both saving and **Send test
email**. Changing the Apprise server origin or turning off its certificate
check requires entering the Apprise API key again for both saving and testing.
A blank SMTP server host is refused as a configuration error instead of dialing
loopback.

Across queued email, webhook, and Apprise destinations, when a later live alert
or recovery succeeds for the exact same saved destination configuration, Pulse
automatically re-queues up to 25 recent (24-hour) retryable retained failures
for that destination once, subject to a five-minute cooldown per destination.
Authentication, configuration, rejected, and TLS failures, edited destinations,
rehearsals, and **Test** sends do not trigger automatic replay.

### Rehearse the saved delivery path

In **Alerts → Notifications**, expand **Delivery rehearsal**. Save or discard
settings edits first: the rehearsal uses saved email, webhook and Apprise
destinations, never an unsaved destination draft.

1. Choose a severity and optional comma-separated routing tags, then select
   **Send rehearsal alert**. The message is clearly labelled as a rehearsal.
2. Inspect **Notification admission** and each destination's firing result.
   **Passed** means the destination accepted the delivery. **Failed** means a
   delivery failed. **Suppressed** means policy prevented delivery. **Pending**
   and **Deferred** remain unfinished, including grouping, quiet hours and retries.
   **Unavailable** means evidence cannot establish the outcome.
3. Confirm receipt in the intended inbox or service. When ready, select
   **Send recovery and finish rehearsal**. This cancels pending firing messages
   and sends recovery only to destinations with a successful firing receipt for
   that rehearsal occurrence, subject to the saved recovery policy.

Rehearsals use the ordinary routing, grouping, persistent queue, transports,
retry budgets and recovery receipts. They create no infrastructure incident and
do not test metric detection, resource acknowledgement/maintenance, push delivery,
or the independent external watchdog. Failed queue admission never falls back
to a direct rehearsal send. Recent delivery activity retains actual attempts.
Failed rehearsals affect ordinary delivery health. Expand **Delivery details**
for redacted transport diagnostics and failure categories.
The sample uses the performance-alert quiet-hours category, including the saved
setting for suppressing critical performance alerts.

Results survive reloads and restarts. The latest 20 rehearsals are shown from a
seven-day history. Finish the current rehearsal before starting another. An
unconfirmed request must be refreshed or retried with its original ID, never
assumed failed and resent as a new rehearsal. Expiration cancels remaining
rehearsal work without inventing delivery or recovery.

If the Test succeeds but the incident does not arrive, follow
[missing alert notifications](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
and [severity and tag routing](CONFIGURATION.md#destination-severity-and-tag-routing).

## Control reminders and handle an incident

**Alerts → Schedule** controls delivery timing. A cooldown sets the minimum
interval before an ordinary reminder, not a guaranteed delivery time. Turning
cooldown Off stops ordinary reminders for that occurrence after its first
notification. It does not turn off monitoring or initial delivery.

In **Alerts → Overview**:

| Action | What it means |
| --- | --- |
| Acknowledge | Marks the incident as seen and suppresses further firing, escalation and recovery notifications while acknowledged. Monitoring continues. |
| Snooze | Pauses that incident's notifications and escalation until the selected time. Monitoring can still detect recovery. |
| Unacknowledge or Resume | Removes the corresponding hold. Other delivery policies still apply, so this does not guarantee an immediate resend. |

Use **Show acknowledged** to inspect acknowledged incidents. An incident hidden
from the default list has not necessarily recovered. These actions do not
repair a workload or retry a failed notification.

Quiet hours are also delivery controls. Non-critical notifications are held;
critical notifications remain eligible unless you explicitly suppress their
categories. Resource maintenance is separate and applies to the selected
resource scope. See [quiet hours](CONFIGURATION.md#quiet-hours-and-notification-holds)
and [incident actions](CONFIGURATION.md#acknowledge-and-snooze-existing-alerts)
before relying on a hold during maintenance.

## Confirm recovery separately

Inspect fresh readings and the incident in **Overview** or **History**. A
disabled rule or missing reading is not measured recovery.

When recovery notifications are enabled, a destination needs a successful
firing-delivery record for that same occurrence to receive an eligible recovery
message. Each destination's recovery notification keeps the severity level from
the firing notification that destination actually received, even if a metric
alert stepped down from critical to warning before clearing. If an external
automation pairs firing and recovery webhooks, match on the alert `ID` rather
than relying on severity alone. Acknowledgement, snooze, schedules and delivery
failures can still prevent recovery delivery. See
[reminders and recovery](CONFIGURATION.md#alert-reminders-and-recovery-notifications).

## Detect an outage of Pulse itself

Pulse needs its server and an outbound connection to send notifications.
Configure the **External watchdog** under **Alerts → Notifications** with an
independent Healthchecks-compatible service outside the power and network
failure you want to detect. The [watchdog setup and verification recipe](TROUBLESHOOTING.md#no-alert-when-pulse-power-or-internet-goes-down)
explains heartbeat timing and a controlled outage test.

For website, port and device reachability, use [availability checks](AVAILABILITY.md).
Pro remote probe agents report back to Pulse. They do not replace the independent
watchdog or deliver notifications while Pulse is down.

A successful watchdog heartbeat means the receiver heard from Pulse. Pulse
sends it based on recent activity in the monitoring loop, not on whether polls
finished, agent reports were accepted or alert rules ran. A `200` from
`/api/health` only means the web server answered.

If readings look frozen while the monitored systems still respond, check each
collection path separately, allowing for normal poll and report intervals and
retry delays:

- **Polled platforms:** Check the newest stored sample times in
  [History](METRICS_HISTORY.md#read-a-history-panel) and the live readings on
  each expected connection. **Latest from device** and **Latest recorded** can
  include points the device kept during a Pulse outage, so a new chart point
  alone does not prove Pulse polled again.
- **Agents:** Check **Last seen** (shown as **Seen** on narrow screens) and
  the readings you need in **Machines**. Check **Docker** hosts separately:
  container reports use their own stream and can stall even while machine
  reports keep arriving from the same agent.
- **Alert rule evaluation:** If a threshold alert is already open,
  [check whether it was reevaluated](#check-whether-an-open-threshold-alert-was-reevaluated)
  through `GET /api/alerts/active` without changing a rule or forcing an alert.
- **Prometheus scrapes (if already enabled):** Compare
  `pulse_monitor_poll_total` by `result`. Rising errors mean Pulse is still
  recording poll outcomes. If several expected sources give no new results,
  check whether collection has stopped.

[Check real alert delivery separately](#choose-where-notifications-go).

### Warn when expected polls stop succeeding

If you already scrape Pulse with Prometheus, set two alerts there rather than
in the Pulse instance you want to watch. Keep the alert evaluator and its
notification path outside the failure you want to detect. Use your existing
protected scrape path. Do not open the metrics port to set up these rules.

**Pulse metrics unavailable.** Alert when Prometheus cannot scrape the expected
Pulse target or has no scrape series for it:

```promql
up{job="pulse", instance="pulse-a.example:9091"} == 0
or absent(up{job="pulse", instance="pulse-a.example:9091"})
```

**No recent successful poll.** Alert for a connection you expect to keep
polling. This checks for an old or missing success time only while the matching
Pulse scrape works:

```promql
(
  time() - pulse_monitor_poll_last_success_timestamp{
    job="pulse", instance="pulse-a.example:9091",
    instance_type="pve", exported_instance="home-pve"
  } > 300
  or absent(pulse_monitor_poll_last_success_timestamp{
    job="pulse", instance="pulse-a.example:9091",
    instance_type="pve", exported_instance="home-pve"
  })
)
and on (job, instance)
  (up{job="pulse", instance="pulse-a.example:9091"} == 1)
```

Replace the example labels and check that the timestamp series appears during
normal polling before enabling the rule. With Prometheus's default label
handling, `instance` is the Pulse scrape target and `exported_instance` is the
connection name reported by Pulse. If you keep exporter labels or relabel
metrics, use the actual labels and match on the labels that identify the scrape
target. For PBS, use `instance_type="pbs"` and its own connection name.

The 300 seconds is an example, not a Pulse default. Allow for the connection's
normal poll interval, poll duration and retry delays. Give each alert a
Prometheus `for` duration. For the poll alert, make it long enough to allow the
first success after a normal restart. Update or pause the rule if you remove
or intentionally pause the target.

A poll warning means Pulse has not recorded a fresh success for that
connection. An increasing `pulse_monitor_poll_total` series with
`result="error"` means polls are still returning failures. Each scrape emits
`pulse_monitor_poll_last_success_timestamp` and
`pulse_monitor_poll_staleness_seconds` from one snapshot of the latest
successful completion, so a delayed older result still counts in
`pulse_monitor_poll_total` without moving that completion backwards.

If scrapes keep succeeding with no new poll outcomes, check the scheduler and
agent series already on that scrape target (an absent series on an older build
is not zero):

- **Backoff still active:** `pulse_scheduler_breaker_retry_seconds` for the
  same connection shows time left until the recorded retry deadline. Zero
  means the deadline has passed or none was recorded, not that a poll ran.
- **Poll waiting in queue or on a worker:** compare
  `pulse_scheduler_queued_poll_overdue_seconds` and
  `pulse_scheduler_claimed_poll_overdue_seconds` (`stage="waiting_for_slot"`
  and `stage="running"`) across fresh scrapes. Growing ages show queued or
  claimed polls falling behind their schedule or timeout budget. A stopped
  scheduler drops out of `pulse_scheduler_queued_poll_overdue_seconds` even if
  a blocked worker has not returned yet, so keep checking
  `pulse_scheduler_claimed_poll_overdue_seconds` for that worker. Zero does not
  prove polling recovered.
- **Parked poll failures:** `pulse_scheduler_dead_letter_depth` counts terminal
  poll entries still parked in running schedulers at scrape time by
  `instance_type` and `instance`. After a connection recovers, or its scheduler
  stops or is replaced, a label with no remaining entries reads `0` for up
  to five minutes after its last positive scrape, then the series disappears.
  Neither zero nor an absent series proves polling is running. Compare with
  `pulse_monitor_poll_total` and `pulse_monitor_poll_last_success_timestamp`
  in the same window.
- **Agent reports:** inspect `pulse_http_requests_total` with `method="POST"`
  separately for `/api/agents/agent/report` (or legacy
  `/api/agents/host/report`) and `/api/agents/docker/report`, split by
  `status`. Do not add the routes together: machine reports can keep
  succeeding while Docker reports stop.

Use the [missing-data checks](FAQ.md#no-data-showing) to narrow it down
without restarting Pulse or sending test traffic. Neither alert above checks
whether Pulse evaluated its alert rules.

### Check whether an open threshold alert was reevaluated

An open alert can be evaluated again without firing a new notification or
clearing. Active alert counts and notification history do not prove each
evaluation ran.

For a threshold alert that is already open, read
[Active alerts](API.md#active-alerts) (`GET /api/alerts/active`) in your
signed-in Pulse browser, or with a `monitoring:read` token from a private
header file. Take two readings on the same Pulse server and workspace,
separated by the normal poll or agent report interval for that rule.

1. Match the same alert occurrence by both `id` and `startTime`, because a
   later firing of the same rule can reuse `id`.
2. Compare `metricStatus.observedAt` between the two readings. When it moves
   forward, Pulse evaluated that threshold again even if the value stayed
   inside the hysteresis band (`metricStatus.phase` set to `breaching`,
   `latched`, or `recovering`).
3. Keep `lastSeen` and `metricStatus.lastBreachAt` separate from `observedAt`:
   they date the latest reading that met the trigger, not a newer latched or
   recovery check.

`metricStatus` is live in-memory state and is omitted from durable checkpoints,
so it is absent immediately after a Pulse restart until the next evaluation.
Non-threshold alerts and older builds can also omit it. A missing field,
missing alert, unchanged timestamp, or failed request does not establish
progress or prove a stall on its own. Checking one open rule does not prove
other rules ran or that any destination accepted a notification. Keep the full
response private, and do not lower thresholds or force a test alert just to
run this check.
