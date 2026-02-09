# TrueNAS Operational Runbook

This runbook defines operator procedures for TrueNAS enablement, disablement, immediate deactivation (kill-switch), code rollback, and data cleanup.

Status: Active  
Owner: Pulse Operations  
Last Updated: 2026-02-09

## Prerequisites

1. Ensure Pulse is reachable at `http://localhost:7655`.
2. Ensure `curl`, `jq`, `git`, and `go` are installed.
3. Ensure you can restart the Pulse process in your environment (systemd, container, or dev runner).
4. Ensure you have admin API access for write operations:
   - `POST /api/truenas/connections`
   - `POST /api/truenas/connections/test`
   - `DELETE /api/truenas/connections/:id`
5. Set an optional auth header for protected endpoints if your environment requires it:

```bash
AUTH_HEADER=(-H "Authorization: Bearer $PULSE_TOKEN")
```

## Ownership and Escalation Routing

### Owner routing

| Scope | Primary Owner | Escalation Owner |
|---|---|---|
| TrueNAS poller health, staleness, and connectivity incidents | Pulse Operations | Engineering |
| Kill-switch execution and disable/enable operations | Pulse Operations | Engineering |
| Code rollback and release-level incident coordination | Engineering | Pulse Operations |
| Security-impacting TrueNAS incidents (P1) | Security Response Owner | Engineering |

### Escalation routing

1. Classify incident severity using `## Incident Severity and Response`.
2. Route response ownership by severity:
   - `P1`: Page `Pulse Operations` immediately and notify `Security Response Owner` + `Engineering` immediately. Start containment within 15 minutes using `## 3) Kill-Switch` or `## 2) Disable Path`.
   - `P2`: Assign to `Pulse Operations` immediately. Escalate to `Engineering` if mitigation is not active within 1 hour.
   - `P3`/`P4`: Assign to `Pulse Operations`. Escalate to `Engineering` if unresolved by next business day.
3. If kill-switch or rollback is executed, open an incident record and capture owner handoff timestamps, actions taken, and final resolution owner.

## 1) Enable Path

1. Set the feature flag:

```bash
export PULSE_ENABLE_TRUENAS=true
# Also accepted (case-insensitive): 1, yes, on
```

2. Restart Pulse.

3. Verify the TrueNAS connections endpoint is enabled and returns an array:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: [] (no configured connections yet) OR existing configured connections
```

4. Verify logs for poller startup activity:

```bash
journalctl -u pulse -n 200 --no-pager | rg -i "TrueNAS poller started|TrueNASPoller"
```

5. Add a connection:

```bash
curl -s "${AUTH_HEADER[@]}" \
  -X POST http://localhost:7655/api/truenas/connections \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"truenas.local","api_key":"REPLACE_ME"}' | jq .
```

6. Test the connection:

```bash
# Current implementation endpoint:
curl -s "${AUTH_HEADER[@]}" \
  -X POST http://localhost:7655/api/truenas/connections/test \
  -H "Content-Type: application/json" \
  -d '{"name":"nas-1","host":"truenas.local","api_key":"REPLACE_ME"}' | jq .

# Contract path (if available in your branch):
# POST /api/truenas/connections/:id/test
```

7. Verify TrueNAS resources appear:

```bash
# Unified v2 (preferred for source-level verification)
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources \
  | jq '.data[] | select((.sources // []) | index("truenas"))'

# Legacy view
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/resources \
  | jq '.resources[] | select(.platformType == "truenas")'
```

## 2) Disable Path

1. Disable the feature flag:

```bash
unset PULSE_ENABLE_TRUENAS
# Or explicitly disable:
export PULSE_ENABLE_TRUENAS=false
```

2. Restart Pulse.

3. Verify the poller is NOT running by checking logs — no poll activity should appear:

```bash
# The poller gate at truenas_poller.go:48 prevents Start() when flag is off.
# Note: The connections API still responds (handlers are always registered),
# but the poller does not poll, so no new resource data is ingested.
journalctl -u pulse -n 200 --no-pager | rg -i "TrueNAS poll"
# Expected: no recent poll activity
```

4. Wait at least 120 seconds for TrueNAS source staleness transition.

5. Verify source status transitions to stale and aggregate status becomes warning:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources | jq '
  .data[]
  | select((.sources // []) | index("truenas"))
  | {
      id,
      name,
      resourceStatus: .status,
      truenasSourceStatus: (.sourceStatus.truenas.status // "missing")
    }'
# Expected after threshold: truenasSourceStatus == "stale", resourceStatus typically "warning"
```

## 3) Kill-Switch (Immediate Deactivation Without Restart)

1. List current TrueNAS connection IDs:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq -r '.[].id'
```

2. ⚠️ Delete all TrueNAS connections:

```bash
for id in $(curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq -r '.[].id'); do
  curl -s "${AUTH_HEADER[@]}" -X DELETE "http://localhost:7655/api/truenas/connections/$id" | jq .
done
```

3. Wait for `syncConnections()` to run on the next tick (poll interval: 60 seconds).

4. Verify no configured connections remain:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: []
```

5. Wait up to 120 seconds more and verify TrueNAS source status becomes stale:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/v2/resources | jq '
  .data[]
  | select((.sources // []) | index("truenas"))
  | {id, name, truenasSourceStatus: (.sourceStatus.truenas.status // "missing")}'
```

6. Confirm no restart or env var change was required for this deactivation path.

## 4) Code Rollback

1. Identify rollback boundaries:
   - TrueNAS implementation range: `f9680ef8...687ecd79` (TN-00 through TRR-00)
   - First feature commit: `100494a7` (TN-01, REST client scaffold)

2. ⚠️ Revert the TrueNAS range:

```bash
git revert --no-edit f9680ef8^..687ecd79
```

3. Resolve conflicts if any, then verify the repository builds cleanly:

```bash
go build ./...
# Required: exit code 0
```

4. Restart Pulse and verify TrueNAS routes are absent/disabled.

5. Verify frontend behavior:
   - TrueNAS-specific badges and filters should no longer render (UI is additive and should self-prune once backend support is removed).

## 5) Data Cleanup

1. Determine the active data directory:

```bash
echo "${PULSE_DATA_DIR:-/etc/pulse}"
```

2. Locate the TrueNAS config file:
   - Default: `/etc/pulse/truenas.enc`
   - Dev: `tmp/dev-config/truenas.enc`

3. Confirm file exists before cleanup:

```bash
ls -l "${PULSE_DATA_DIR:-/etc/pulse}/truenas.enc"
```

4. ⚠️ Remove only `truenas.enc`:

```bash
rm -f "${PULSE_DATA_DIR:-/etc/pulse}/truenas.enc"
```

5. Verify cleanup result:

```bash
curl -s "${AUTH_HEADER[@]}" http://localhost:7655/api/truenas/connections | jq .
# Expected: [] (LoadTrueNASConfig() returns empty slice when file is missing)
```

6. Verify encryption key remains intact:

```bash
ls -l "${PULSE_DATA_DIR:-/etc/pulse}/.encryption.key"
```

7. Do not delete `.encryption.key`; remove only `truenas.enc`.

## Canary Rollout Strategy

### Phase 1: Internal / Dev

1. Activation: Set `PULSE_ENABLE_TRUENAS=true` on internal/dev instances only.
2. Duration: Run for a minimum of 48 hours.
3. Monitoring checkpoint:
   1. `pulse_monitor_poll_total{instance_type="truenas",result="error"}` error rate remains below 5% over a rolling 1-hour window.
   2. `pulse_monitor_poll_staleness_seconds{instance_type="truenas"}` remains below 300 seconds continuously.
   3. `pulse_monitor_poll_errors_total{instance_type="truenas",error_type="auth"}` does not increment (no auth errors).
   4. Memory and CPU baseline is captured for comparison in later phases.
4. Exit gate: All monitoring checks remain green for at least 24 continuous hours.
5. Rollback: Unset `PULSE_ENABLE_TRUENAS` and restart Pulse (see `## 2) Disable Path` above).

### Phase 2: Opt-in Early Adopters

1. Activation: Users manually set `PULSE_ENABLE_TRUENAS=true` in their environments.
2. Duration: Run for a minimum of 1 week.
3. Monitoring checkpoint:
   1. Maintain all Phase 1 metric checks across all opt-in instances:
      - `pulse_monitor_poll_total{instance_type="truenas",result="error"}`
      - `pulse_monitor_poll_staleness_seconds{instance_type="truenas"}`
      - `pulse_monitor_poll_errors_total{instance_type="truenas",error_type="auth"}`
   2. User-reported issues are triaged within 24 hours.
   3. No data corruption or cross-platform interference is observed.
4. Exit gate: Zero critical issues for at least 5 continuous days across all opt-in instances.
5. Rollback: Users unset `PULSE_ENABLE_TRUENAS` and restart Pulse.

### Phase 3: Default-On

1. Activation: Change the `PULSE_ENABLE_TRUENAS` feature-flag default to `true` in code.
2. Duration: Permanent (standard operation).
3. Opt-out: Users can set `PULSE_ENABLE_TRUENAS=false` to disable.
4. Monitoring checkpoint:
   1. Maintain all Phase 2 checks and track aggregate fleet-wide poll error rate using `pulse_monitor_poll_total{instance_type="truenas",result="error"}`.
   2. Keep automated staleness alerts active from `## Alert Thresholds` for `pulse_monitor_poll_staleness_seconds{instance_type="truenas"}`.
5. Rollback: Revert the default-on commit and release a patch.

### Abort Criteria (All Phases)

1. If poll failure rate exceeds 50% across all connections for more than 5 minutes (derived from `pulse_monitor_poll_total{instance_type="truenas",result="error"}`), perform an immediate abort.
2. If any data corruption or cross-platform resource interference is detected, perform an immediate abort.
3. If memory or CPU regresses by more than 20% and is attributable to the TrueNAS poller, abort within 1 hour.
4. If auth errors persist for more than 10 minutes without resolution (`pulse_monitor_poll_errors_total{instance_type="truenas",error_type="auth"}`), abort and investigate.
5. If more than 3 user-reported critical bugs occur in any 24-hour window, abort and investigate.

### Phase Transition Checklist

For each phase transition, the operator must verify:

1. All monitoring checkpoints from the current phase are green.
2. No open critical or high-severity issues are attributed to TrueNAS.
3. Rollback procedure has been tested in staging.
4. Phase exit gate duration has been met.
5. Stakeholder sign-off has been recorded.

## Alert Thresholds

| Metric | Condition | Severity | Action |
|---|---|---|---|
| `pulse_monitor_poll_total{instance_type="truenas",result="error"}` | > 3 consecutive errors | WARNING | Check TrueNAS connection, verify API key |
| `pulse_monitor_poll_staleness_seconds{instance_type="truenas"}` | > 300s | WARNING | Investigate poll failures, check network |
| `pulse_monitor_poll_staleness_seconds{instance_type="truenas"}` | > 600s | CRITICAL | TrueNAS data is stale, operator intervention needed |
| `pulse_monitor_poll_errors_total{instance_type="truenas",error_type="auth"}` | Any increment | CRITICAL | API key revoked or expired, immediate attention |
| `pulse_monitor_poll_errors_total{instance_type="truenas",error_type="timeout"}` | > 5 in 10 minutes | WARNING | TrueNAS API response time degraded |
| `pulse_monitor_poll_errors_total{instance_type="truenas",error_type="connection"}` | Any increment | WARNING | Network connectivity to TrueNAS lost |
| `pulse_monitor_poll_duration_seconds{instance_type="truenas"}` | p95 > 10s | WARNING | TrueNAS API latency elevated |

## Incident Severity and Response

- P1: Data loss, security breach, or broad outage. Response: immediate acknowledgement and containment start in < 15 minutes.
- P2: Degraded functionality or single-feature outage. Response: acknowledge and begin mitigation in < 1 hour.
- P3: Non-blocking degradation or monitoring gaps. Response: triage and mitigation plan in < 4 hours.
- P4: Cosmetic or documentation-only issue. Response: address by next business day.

## Quick Reference

| Operation | Primary Action | Restart Required | Propagation Window | Verification |
|---|---|---|---|---|
| Enable | `PULSE_ENABLE_TRUENAS=true` | Yes | Immediate after restart | `GET /api/truenas/connections` returns `[]` or entries |
| Disable | unset or `PULSE_ENABLE_TRUENAS=false` | Yes | 120s staleness window | Poller stops (no poll activity in logs); v2 resources show `sourceStatus.truenas.status="stale"` after threshold |
| Kill-Switch | Delete all `/api/truenas/connections/:id` | No | 60s provider removal + 120s staleness | Connections list becomes `[]`; v2 resources show stale TrueNAS source |
| Code Rollback | `git revert --no-edit f9680ef8^..687ecd79` | Yes (apply change) | Immediate after deploy | `go build ./...` exits `0`; TrueNAS UI/backend paths removed |
| Data Cleanup | Remove `truenas.enc` only | No (API reads empty config) | Immediate | `GET /api/truenas/connections` returns `[]`; `.encryption.key` still exists |
