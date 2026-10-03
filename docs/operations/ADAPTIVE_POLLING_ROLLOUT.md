# 🚀 Adaptive Polling Rollout

Adaptive polling is disabled by default. Change it during a maintenance window,
not as an unmeasured remedy for high CPU, excessive writes or a stalled backup.
A Pulse restart interrupts monitoring and notifications until it is running again.

## 📋 Pre-Flight

1. Record the running version, fleet size, current polling settings and whether
   Pulse dashboards are open. Save the existing environment overrides and
   `adaptivePollingEnabled` value so rollback restores the same configuration.
2. Take one [scheduler health snapshot](../api/SCHEDULER_HEALTH.md#-common-queries-jq).
   Use a signed-in browser or the private header-file recipe there. Keep the
   response local: instance names, addresses and error text can be sensitive.
3. Establish each instance's last successful poll and current errors before
   changing settings. A small or empty queue alone does not prove healthy polls;
   scheduled tasks may be waiting for their next run, and polling may be disabled.

## 🟢 Enable

There is no dedicated UI toggle. Choose one configuration source:

- **Managed environment**: Set `ADAPTIVE_POLLING_ENABLED=true` in the existing
  service, Compose or Kubernetes deployment configuration, then restart or
  redeploy Pulse using that deployment's normal procedure.
- **Persistent settings**: In the instance's actual data directory, edit only
  `adaptivePollingEnabled` in `system.json`, preserving its other settings,
  ownership and permissions, then restart Pulse. Back up the original privately
  first; this file can contain secrets. Do not replace it through a shared
  `/tmp/system.json` or replace the whole document with a single property.

An explicit environment value overrides `system.json`. Do not change both
sources at once or assume a file edit took effect while an override remains.
See [configuration](../monitoring/ADAPTIVE_POLLING.md#-configuration) for intervals.
After restart, the health response's `enabled` field confirms the effective flag,
not successful monitoring.

## 🔍 Monitor (First 15m)

For a responsive install, take another snapshot after about 1, 5 and 15 minutes;
run the local jq queries against each saved file, not a separate HTTP request per
query. Stop collection if the system becomes unresponsive. Do not add an
unbounded `watch` loop, enable Debug or expose a metrics port for this check.

Compare per-instance `pollStatus.lastSuccess`, `consecutiveFailures`, breaker
state and dead-letter tasks with the baseline. Last-success times should advance
in line with the effective polling intervals. Check that the affected dashboard
data still refreshes. A snapshot's `updatedAt` is its collection time, not a
successful provider poll. There is no fleet-independent queue-depth success
threshold, and an open breaker is a backoff state, not proof of a new regression.

If using an already configured Prometheus scrape, compare the queue and error
metrics too. Missing metrics or a failed read are unavailable evidence, not zero
errors. Do not prolong a failure merely to finish the 15-minute observation.

## ↩️ Rollback

If freshness deteriorates or failures rise after the change:

1. Restore the saved environment and persistent settings. For an originally
   disabled install, an explicit `ADAPTIVE_POLLING_ENABLED=false` override disables
   the flag; merely removing `true` can uncover a persisted `true` value.
2. Restart or redeploy through the same procedure. This pauses monitoring again.
3. Take a fresh snapshot to confirm the effective flag, then verify successful
   polls and dashboard freshness against the baseline. An empty queue or
   `enabled: false` alone is not recovery evidence.

Keep the versions, times, changed settings and relevant redacted observations
when reporting a problem. Do not share the header file, raw health response,
deployment environment or `system.json`.
