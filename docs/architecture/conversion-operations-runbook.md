# Conversion Pipeline Operations Runbook

Status: Active
Owner: Pulse Operations
Date: 2026-02-09

Related plan:
- `docs/architecture/conversion-operations-plan-2026-02.md`

## Overview

The conversion pipeline tracks user interactions with paywalls, upgrade links, trial activation, and limit enforcement. Events flow from frontend surfaces through `POST /api/conversion/events` to an in-memory `WindowedAggregator` with Prometheus instrumentation.

**Key characteristics:**
- In-memory only — events are lost on process restart
- Fire-and-forget — frontend never blocks on event recording
- Auth-required — all endpoints need authenticated session
- Bounded cardinality — max 1000 keys per tenant per event type

## Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---|---|---|
| `/api/conversion/events` | POST | RequireAuth | Record a conversion event |
| `/api/conversion/stats` | GET | RequireAuth | Non-destructive snapshot of current aggregation window |
| `/api/conversion/health` | GET | RequireAuth | Pipeline health status (healthy/stale/degraded) |
| `/api/conversion/config` | GET/PUT | RequireAuth (admin) | Runtime collection controls (kill switch) |

## Health Status Definitions

| Status | Meaning | Action |
|---|---|---|
| `healthy` | Events flowing, >=50% event type coverage, last event <5 min ago | No action needed |
| `stale` | No events received in >5 minutes, OR no events since startup (>5 min ago) | Investigate — see Pipeline Stall below |
| `degraded` | Events flowing but <50% of known event types seen | Investigate — see Emission Gap below |

## Incident Playbooks

### Pipeline Stall (health: "stale")

**Symptoms:**
- `GET /api/conversion/health` returns `status: "stale"`
- `pulse_conversion_events_total` rate drops to 0
- `last_event_age_seconds` > 300

**Diagnosis Steps:**
1. Check if Pulse backend is running and healthy: `GET /api/health`
2. Check if users are active (WebSocket connections, API requests)
3. Test event ingestion manually:
   ```bash
   curl -s -u admin:admin -X POST http://127.0.0.1:7655/api/conversion/events \
     -H 'Content-Type: application/json' \
     -d '{"type":"paywall_viewed","capability":"test","surface":"runbook_test","timestamp":'$(date +%s000)',"idempotency_key":"runbook-test-'$(date +%s)'"}'
   ```
   Expected: `202 Accepted` with `{"accepted": true}`
4. If manual test succeeds but no organic events: frontend is not emitting. Check if `conversionEvents.ts` is loaded.
5. If manual test fails with 401/403: auth issue. Check session cookies.
6. If manual test fails with 500: backend handler issue. Check `/tmp/pulse-debug.log`.

**Resolution Actions:**
- Frontend not emitting: check browser console for `conversionEvents.ts` errors. Verify `apiFetch` is available.
- Backend handler broken: restart Pulse (`touch cmd/pulse/main.go` in dev, or service restart in production).
- Auth broken: separate issue — fix auth, conversion will resume automatically.

**Escalation:**
- If pipeline stall persists >1 hour after investigation: disable collection via `PUT /api/conversion/config` and file a P1 incident for the conversion team.

### Emission Gap (health: "degraded")

**Symptoms:**
- `GET /api/conversion/health` returns `status: "degraded"`
- `events_by_type` shows some types at 0

**Diagnosis Steps:**
1. Check `events_by_type` in health response. Identify which types are missing.
2. For missing `paywall_viewed`: verify HistoryChart, AIIntelligence, and settingsFeatureGates are rendering lock states.
3. For missing `upgrade_clicked`: verify upgrade link onClick handlers are wired in HistoryChart and AIIntelligence.
4. For missing backend-emitted types (`trial_started`, `license_activated`, `limit_blocked`): these are deferred — backend emission is not yet wired. **This is expected.**
5. Cross-reference with `GET /api/conversion/stats` to see actual event counts.

**Resolution Actions:**
- Frontend emission missing: check for JavaScript errors preventing `createEffect` hooks from running.
- Expected missing types (deferred): no action needed. Degraded status is expected until backend emission is wired.
- Unexpected missing types: investigate specific surface — may be a UI regression that removed the paywall gate.

### Cardinality Overflow

**Symptoms:**
- `pulse_conversion_events_invalid_total{reason="cardinality_exceeded"}` incrementing
- Some events being silently dropped (no error to frontend)

**Diagnosis Steps:**
1. Check `GET /api/conversion/stats` bucket count. If >500 unique buckets in window, cardinality is high.
2. Review event `Key` field patterns. Key is `${surface}:${capability}` — should be bounded by known surfaces and capabilities.
3. If keys contain user-generated data (e.g., resource IDs), there's a frontend bug emitting unbounded values.

**Resolution Actions:**
- Identify the surface/capability combination generating unbounded keys.
- Fix frontend to use only known surface/capability constants.
- As a temporary measure, disable the offending surface via `PUT /api/conversion/config`.

### Memory Pressure

**Symptoms:**
- Pulse process memory growing over time
- `GET /api/conversion/stats` shows very large bucket counts

**Diagnosis Steps:**
1. Check `BucketCount()` via stats endpoint. Normal range: 10-100 buckets per window.
2. Check if `Flush()` is being called periodically (metering pipeline should flush hourly).
3. If flush is not happening, the aggregator window grows indefinitely.

**Resolution Actions:**
- If flush is not happening: restart Pulse. The aggregator resets on restart.
- If flush is happening but memory still growing: investigate for aggregator leak (unlikely with current implementation).
- Last resort: disable conversion collection to stop memory growth.

## Kill Switch Usage

### Disable All Collection
```bash
curl -s -u admin:admin -X PUT http://127.0.0.1:7655/api/conversion/config \
  -H 'Content-Type: application/json' \
  -d '{"enabled": false, "disabled_surfaces": []}'
```

### Disable Specific Surface
```bash
curl -s -u admin:admin -X PUT http://127.0.0.1:7655/api/conversion/config \
  -H 'Content-Type: application/json' \
  -d '{"enabled": true, "disabled_surfaces": ["history_chart"]}'
```

### Re-enable Collection
```bash
curl -s -u admin:admin -X PUT http://127.0.0.1:7655/api/conversion/config \
  -H 'Content-Type: application/json' \
  -d '{"enabled": true, "disabled_surfaces": []}'
```

### Check Current Config
```bash
curl -s -u admin:admin http://127.0.0.1:7655/api/conversion/config | jq
```

**Important:** Kill switch is runtime-only. Config resets to default (enabled) on process restart.

## Incident Severity and Response

- P1: Data loss, security breach, or broad outage. Response: immediate acknowledgement and containment start in < 15 minutes.
- P2: Degraded functionality or single-feature outage. Response: acknowledge and begin mitigation in < 1 hour.
- P3: Non-blocking degradation or monitoring gaps. Response: triage and mitigation plan in < 4 hours.
- P4: Cosmetic or documentation-only issue. Response: address by next business day.

## Rollback

1. Disable conversion pipeline via kill-switch API.
2. In-memory events are lost on restart (by design, with no persistent side effects).
3. Re-enable the pipeline after investigation and verification.

## Prometheus Metrics Reference

| Metric | Type | Labels | Description |
|---|---|---|---|
| `pulse_conversion_events_total` | Counter | `type`, `surface` | Successfully recorded conversion events |
| `pulse_conversion_events_invalid_total` | Counter | `reason` | Events rejected by validation |
| `pulse_conversion_events_skipped_total` | Counter | `reason` | Events skipped due to disabled collection (CVO-04) |

## Known Limitations

1. **In-memory only**: All conversion data is lost on process restart. Persistent storage is deferred.
2. **No historical query**: Stats endpoint only shows current aggregation window. No historical data access.
3. **Backend emission deferred**: `trial_started`, `license_activated`, `license_activation_failed`, `checkout_started`, `limit_warning_shown`, `limit_blocked` are contract-ready but not yet wired into backend handlers.
4. **No external export**: Events stay in-process. No Segment/Mixpanel/PostHog integration.
5. **Kill switch resets on restart**: Runtime config is not persisted.
