# Adaptive Polling Management Endpoints (Future Enhancement)

## Status: DEFERRED

**Decision Date:** 2025-10-20  
**Reviewed After:** Phase 2 integration/soak testing

## Assessment

After completing comprehensive integration and soak testing (including 2-minute validation with 80 instances), we determined that manual circuit breaker and DLQ management endpoints are **not required for Phase 2 production rollout**.

### Test Results

- **Integration test:** 55 seconds, 12 instances
  - Circuit breakers opened and closed automatically
  - Transient failures recovered without intervention
  - Permanent failures correctly routed to DLQ

- **Soak test:** 2-240 minutes, 80 instances
  - Heap: 2.3MB → 3.1MB (healthy growth)
  - Goroutines: 16 → 6 (no leak)
  - No scenarios requiring manual intervention

### Current Capabilities

**Sufficient for Phase 2:**

1. **Read-only health API:** \`/api/monitoring/scheduler/health\`
   - Full visibility: queue depth, breakers, DLQ contents, staleness

2. **Operator workarounds:**
   - Restart service: clears breaker/DLQ state
   - Toggle \`ADAPTIVE_POLLING_ENABLED\` flag

3. **Grafana alerting:**
   - Queue depth, staleness, DLQ growth, stuck breakers

### Why Defer?

- **No operational need demonstrated:** Tests showed automatic recovery works
- **Implementation cost:** Requires auth/RBAC, audit logging, UI integration
- **Wait for data:** Production usage will reveal actual pain points

---

## Future Implementation (When Needed)

### Proposed Endpoints

1. **POST /api/monitoring/breakers/{instance}/reset** - Reset circuit breaker
2. **POST /api/monitoring/dlq/retry** - Retry all DLQ tasks
3. **POST /api/monitoring/dlq/{instance}/retry** - Retry specific task
4. **DELETE /api/monitoring/dlq/{instance}** - Remove from DLQ

See full design spec in this document.

---

## Re-evaluation Criteria

**Implement if:**
- Operators request manual controls >3x in first 30 days
- Rollout troubleshooting steps inadequate
- Service restarts become disruptive
- Production incidents need surgical controls

