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
message. Acknowledgement, snooze, schedules and delivery failures can still
prevent it. See [reminders and recovery](CONFIGURATION.md#alert-reminders-and-recovery-notifications).

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

If readings look frozen while the monitored systems still respond, compare the
newest sample times in **History** and the reporting agents' **Seen** times in
**Machines**. Allow for normal poll and report intervals, including retry delays.
If you already scrape Prometheus, compare `pulse_monitor_poll_total` by
`result`. Rising errors mean Pulse is still recording poll outcomes. If
several expected sources give no new results, check whether collection has
stopped. [Check real alert delivery separately](#choose-where-notifications-go).

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
`result="error"` means polls are still returning failures. No new results
despite working scrapes does not tell you whether the poll is slow or stuck.
Use the [missing-data checks](FAQ.md#no-data-showing) to narrow it down.
Neither alert checks whether agents reported or Pulse evaluated its rules.
