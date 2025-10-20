# Adaptive Polling Management Endpoints (Future Enhancement)

## Status: DEFERRED

**Decision Date:** 2025-10-20
**Re-evaluated:** v4.24.0 GA release
**Current Status:** Not implemented in v4.24.0

## Overview

Manual circuit breaker and dead-letter queue (DLQ) management endpoints are **not included in v4.24.0**. The read-only scheduler health API (`/api/monitoring/scheduler/health`) provides full visibility, and automatic recovery mechanisms have proven sufficient during testing and early production rollouts.

---

## What's Available in v4.24.0

### Read-Only Scheduler Health API

**Endpoint:** `GET /api/monitoring/scheduler/health`

Provides complete visibility into:
- Queue depth and task distribution
- Circuit breaker states per instance
- Dead-letter queue contents and retry schedules
- Per-instance staleness tracking
- Failure streaks and error categorization

**Documentation:** See [Scheduler Health API](../api/SCHEDULER_HEALTH.md) for complete reference.

### Existing Management Options

**v4.24.0 operators can:**

1. **Toggle adaptive polling** (no restart required)
   - Via UI: **Settings → System → Monitoring**
   - Via API: Update `system.json` with `adaptivePollingEnabled: false`

2. **Service restart** (clears transient state)
   ```bash
   # Systemd
   sudo systemctl restart pulse

   # Docker
   docker restart pulse

   # LXC
   pct restart <ctid>
   ```
   - Clears all circuit breakers
   - Resets DLQ (tasks re-queued with fresh state)
   - Useful for recovering from stuck states

3. **Version rollback** (if broader issues)
   - Via UI: **Settings → System → Updates → Restore previous version**
   - Via CLI: `pulse config rollback`
   - Documented in [Operations Runbook](ADAPTIVE_POLLING_ROLLOUT.md)

4. **Per-instance configuration fixes**
   - Update node credentials if authentication failures cause DLQ entries
   - Adjust network/firewall if connectivity issues trigger breakers
   - Fix underlying infrastructure problems

---

## Why Endpoints Are Deferred

### Test Results Demonstrate Sufficient Automation

**Integration testing** (55 seconds, 12 instances):
- Circuit breakers opened and closed automatically
- Transient failures recovered without intervention
- Permanent failures correctly routed to DLQ

**Soak testing** (2-240 minutes, 80 instances):
- Heap: 2.3MB → 3.1MB (healthy growth)
- Goroutines: 16 → 6 (no leak)
- No scenarios requiring manual intervention

**Production rollout** (v4.24.0):
- Automatic recovery working as designed
- Service restart sufficient for edge cases
- No operator requests for manual controls

### Implementation Cost vs. Benefit

**Would require:**
- Authentication and RBAC integration
- Comprehensive audit logging
- UI integration in Settings → System → Monitoring
- Additional testing and maintenance burden

**Current workarounds proven effective:**
- Adaptive polling toggle (immediate, no restart)
- Service restart (clears all state in < 30 seconds)
- Version rollback (if systematic issues)

---

## Future Implementation Plan

### Proposed Endpoints (When Needed)

If production usage reveals operational gaps, implement:

#### 1. Reset Circuit Breaker
```
POST /api/monitoring/breakers/{key}/reset
Authorization: Required (session or API token)
```

**Request:**
```json
{
  "reason": "Manual reset after infrastructure fix"
}
```

**Response:**
```json
{
  "success": true,
  "key": "pve::pve-node1",
  "previousState": "open",
  "newState": "closed",
  "resetBy": "admin",
  "resetAt": "2025-10-20T15:30:00Z"
}
```

**Use case:** Immediately retry a specific instance after fixing underlying issue (e.g., restored network connectivity)

#### 2. Retry All DLQ Tasks
```
POST /api/monitoring/dlq/retry
Authorization: Required (session or API token)
```

**Response:**
```json
{
  "success": true,
  "tasksRetried": 5,
  "keys": ["pve::pve-node1", "pbs::backup-server"]
}
```

**Use case:** Bulk retry after fixing widespread issue (e.g., certificate renewal)

#### 3. Retry Specific DLQ Task
```
POST /api/monitoring/dlq/{key}/retry
Authorization: Required (session or API token)
```

**Response:**
```json
{
  "success": true,
  "key": "pve::pve-node1",
  "previousRetryCount": 5,
  "scheduledFor": "2025-10-20T15:35:00Z"
}
```

**Use case:** Targeted retry of single instance

#### 4. Remove from DLQ
```
DELETE /api/monitoring/dlq/{key}
Authorization: Required (session or API token)
```

**Response:**
```json
{
  "success": true,
  "key": "pve::decomissioned-node",
  "reason": "Instance permanently decommissioned"
}
```

**Use case:** Remove decommissioned instances from DLQ

### Security Requirements

All management endpoints would require:
- **Authentication:** Valid session cookie or API token
- **RBAC:** Admin-level permissions
- **Audit logging:** Every action logged with:
  - Operator username/IP
  - Instance key affected
  - Reason provided
  - Timestamp
  - Previous and new states
- **Rate limiting:** Prevent abuse (e.g., 10 requests/minute)

---

## Re-evaluation Criteria

**Implement management endpoints if:**

1. **Operator demand:** >3 requests in first 60 days of v4.24.0 deployment
2. **Service restart frequency:** >5 restarts per week due to stuck breakers/DLQ
3. **Incident impact:** Manual controls would have prevented or accelerated recovery from >1 production incident
4. **Feedback from operations runbook:** [ADAPTIVE_POLLING_ROLLOUT.md](ADAPTIVE_POLLING_ROLLOUT.md) troubleshooting inadequate

**Don't implement if:**
- Current workarounds remain effective
- Automatic recovery continues to handle 99%+ of scenarios
- No clear operational pain points emerge

---

## Monitoring Current State

### Check Circuit Breakers
```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.breaker.state != "closed") | {key, state: .breaker.state, since: .breaker.since}'
```

### Check Dead-Letter Queue
```bash
curl -s http://<host>:7655/api/monitoring/scheduler/health \
  | jq '.instances[] | select(.deadLetter.present) | {key, reason: .deadLetter.reason, retryCount: .deadLetter.retryCount, nextRetry: .deadLetter.nextRetry}'
```

### Track Recovery Times
```bash
# Monitor breaker state changes
journalctl -u pulse | grep -E "circuit breaker|dead-letter"
```

---

## Feedback & Requests

If you encounter scenarios where manual management endpoints would be valuable:

1. **Document the use case**
   - What problem occurred?
   - Why wasn't automatic recovery sufficient?
   - How would manual control have helped?

2. **File an issue**
   - [GitHub Issues](https://github.com/rcourtman/Pulse/issues)
   - Include: scheduler health API output, logs, timeline

3. **Track frequency**
   - If the pattern recurs >3 times, escalate for implementation

---

## Related Documentation

- [Scheduler Health API](../api/SCHEDULER_HEALTH.md) - Complete API reference
- [Operations Runbook](ADAPTIVE_POLLING_ROLLOUT.md) - Steady-state operations and troubleshooting
- [Adaptive Polling Architecture](../monitoring/ADAPTIVE_POLLING.md) - Technical details
- [Configuration Guide](../CONFIGURATION.md) - Adaptive polling settings
