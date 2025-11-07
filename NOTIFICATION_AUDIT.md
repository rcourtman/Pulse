# Pulse Notification System - Comprehensive Security & Architecture Audit

**Date:** 2025-11-06
**Auditors:** Claude Code + OpenAI Codex
**Scope:** Complete webhook and email notification system (backend + frontend)

---

## Executive Summary

The Pulse notification system is architecturally sound with sophisticated features (persistent queue, retry logic, SSRF protection, rate limiting). However, it contains **critical correctness, security, and reliability issues** that undermine the guarantees it promises to users.

**Key findings:**
- **Correctness**: Cooldown timing marked before actual delivery causes silent notification drops
- **Security**: DNS rebinding vulnerability in webhook delivery, secrets logged in plaintext
- **Reliability**: Queue initialization fails silently, single-threaded worker creates head-of-line blocking
- **Observability**: Queue/DLQ features exist but are completely hidden from UI
- **Concurrency**: Multiple race conditions in email manager, webhook rate limiter, and shared state

**Recommendation:** Address all P0 issues before the next release. P1 issues should be fixed within 2 releases.

---

## Priority 0 - Critical Issues (Fix Immediately)

### 1. Cooldown Marked Before Delivery Success
**File:** `internal/notifications/notifications.go:649-656`
**Severity:** Critical - Silent notification loss

**Issue:**
`sendGroupedAlerts` marks `lastNotified[alert.ID]` immediately after enqueuing (line 649-656), before confirming the queue accepted the notification or delivery succeeded. If SQLite rejects the enqueue or the notification later moves to DLQ, subsequent `SendAlert` calls see the cooldown and bail, causing complete silence for 5 minutes.

**Impact:**
Users experience alert suppression when notifications fail - Pulse appears to acknowledge incidents but never notifies anyone.

**Root cause:**
```go
// Line 642-656
if n.queue != nil {
    n.enqueueNotifications(emailConfig, webhooks, appriseConfig, alertsToSend)
} else {
    n.sendNotificationsDirect(emailConfig, webhooks, appriseConfig, alertsToSend)
}

// Update last notified time for all alerts
now := time.Now()
for _, alert := range alertsToSend {
    n.lastNotified[alert.ID] = notificationRecord{
        lastSent:   now,
        alertStart: alert.StartTime,
    }
}
```

**Fix:**
Move cooldown stamping to:
1. Queue path: After successful enqueue confirmation + after successful dequeue/send
2. Direct path: After actual delivery success

---

### 2. Queue Directory Not Created
**File:** `internal/notifications/queue.go:62-68`
**Severity:** Critical - Queue silently disabled on fresh installs

**Issue:**
`NewNotificationQueue` assumes `/etc/pulse/notifications` (or `utils.GetDataDir()/notifications`) exists. It never calls `os.MkdirAll`, so `sql.Open` fails with "unable to open database file" on fresh installs or bare-metal deployments.

**Impact:**
Queue initialization fails silently, falling back to fire-and-forget sending with no retry capability. The "persistent queue" feature is disabled by default on most installations.

**Code:**
```go
// Line 62-68
if dataDir == "" {
    dataDir = filepath.Join(utils.GetDataDir(), "notifications")
}
dbPath := filepath.Join(dataDir, "notification_queue.db")
db, err := sql.Open("sqlite3", dbPath)  // FAILS if directory doesn't exist
```

**Fix:**
```go
if dataDir == "" {
    dataDir = filepath.Join(utils.GetDataDir(), "notifications")
}
if err := os.MkdirAll(dataDir, 0755); err != nil {
    return nil, fmt.Errorf("failed to create queue directory: %w", err)
}
dbPath := filepath.Join(dataDir, "notification_queue.db")
```

---

### 3. DNS Rebinding Vulnerability (SSRF)
**Files:** `internal/api/notifications.go:204, 261` + `internal/notifications/notifications.go:1396-1533`
**Severity:** Critical - SSRF bypass via DNS rebinding

**Issue:**
`ValidateWebhookURL` runs only during webhook creation/update but never at send time. An attacker can create a webhook pointing to a legitimate domain, then later update DNS to point to `169.254.169.254` (cloud metadata), `127.0.0.1`, or private IPs. The validation is never re-run in `sendWebhookRequest`.

**Attack scenario:**
1. Admin creates webhook for `https://attacker.com/webhook` (passes validation)
2. Attacker updates DNS: `attacker.com` → `169.254.169.254`
3. Alert triggers → Pulse POSTs to cloud metadata service with OAuth tokens

**Code path:**
- Validation: `api/notifications.go:204` (CreateWebhook), `261` (UpdateWebhook)
- Send: `notifications.go:1429` - `http.NewRequest` uses URL as-is, no re-validation

**Fix:**
Re-run `ValidateWebhookURL` at send time in `sendWebhookRequest` before creating HTTP request.

**Apprise SSRF:**
Apprise HTTP mode (`sendAppriseViaHTTP` at line 904-949) has the same issue - no validation of `serverUrl` against private IPs.

---

### 4. Secrets Logged in Plaintext
**File:** `internal/notifications/notifications.go:1458-1465`
**Severity:** Critical - Credential exposure in logs

**Issue:**
Debug logging for Telegram/Gotify webhooks logs the full URL (containing bot tokens) and complete JSON payload (containing routing keys, API keys) at debug level.

**Code:**
```go
// Line 1458-1465
if webhook.Service == "telegram" || webhook.Service == "gotify" {
    log.Debug().
        Str("webhook", webhook.Name).
        Str("service", webhook.Service).
        Str("url", webhookURL).          // Contains bot token
        Str("payload", string(jsonData)). // Contains all secrets
        Msg("Sending webhook with payload")
}
```

**Example leaked data:**
- Telegram: `https://api.telegram.org/bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11/sendMessage`
- PagerDuty: `{"routing_key": "R123ABC456DEF789GHI012JKL"}`

**Fix:**
Remove debug logging or redact using existing `redactSecretsFromURL` function.

---

### 5. Memory Leak - Unbounded `lastNotified` Map
**File:** `internal/notifications/notifications.go:649-656`
**Severity:** Critical - Memory leak causing eventual OOM

**Issue:**
The `lastNotified` map grows indefinitely. Every alert ID is added on send (line 652-655) but never removed when alerts resolve. Long-running clusters accumulate every historical alert ID.

**Impact:**
Memory usage grows unbounded, mutex contention increases proportionally.

**Fix:**
Delete entries in:
1. `CancelAlert` - when alert resolves
2. Queue processor - after DLQ or final success
3. Periodic cleanup - purge entries older than 24 hours

---

### 6. HTTP Client Created Per Request
**File:** `internal/notifications/notifications.go:1469`
**Severity:** High - Performance degradation

**Issue:**
Every webhook send creates a new `http.Client` via `createSecureWebhookClient`, preventing TLS connection reuse and causing repeated TLS handshakes.

**Code:**
```go
// Line 1469
client := createSecureWebhookClient(WebhookTimeout)
resp, err := client.Do(req)
```

**Impact:**
High-volume webhook endpoints experience significant CPU overhead and latency from repeated TLS handshakes.

**Fix:**
Create a shared `http.Client` at NotificationManager initialization, reuse for all webhook requests.

---

### 7. Queue Enqueue Failures Only Logged
**File:** `internal/notifications/notifications.go:660-718`
**Severity:** High - Silent notification loss

**Issue:**
When `queue.Enqueue()` fails (disk full, SQLite error), the error is only logged - no metric, no retry, no fallback to direct sending.

**Code:**
```go
// Line 683-686
if err := n.queue.Enqueue(notif); err != nil {
    log.Error().Err(err).Str("type", "email").Msg("Failed to enqueue email notification")
}
```

**Impact:**
Transient disk errors silently drop entire batches of alerts.

**Fix:**
1. Fall back to direct sending when enqueue fails
2. Expose queue health metric in `/api/notifications/health`
3. Surface queue errors in UI

---

### 8. No Queue Cancellation Mechanism
**Files:** `internal/notifications/notifications.go:571-614` + `internal/notifications/queue.go`
**Severity:** High - Resolved alerts still trigger notifications

**Issue:**
`CancelAlert` only removes alerts from `pendingAlerts` buffer (line 582-618). Once alerts are serialized to SQLite queue, there's no mechanism to mark them cancelled. The queue will still send notifications for resolved incidents.

**Scenario:**
1. Alert triggers, enters grouping window
2. 10 seconds later, alert resolves → `CancelAlert` removes from pending
3. 20 seconds later, grouping window expires, alert already in queue
4. Queue sends notification for an incident that cleared 20 seconds ago

**Fix:**
Add cancellation tracking:
1. Add `cancelled` status to queue
2. When `CancelAlert` fires, mark queued notifications as cancelled
3. Queue processor skips cancelled items

---

### 9. Single-Threaded Queue Worker
**File:** `internal/notifications/queue.go:520-534`
**Severity:** High - Head-of-line blocking

**Issue:**
Queue processor is single-threaded, processes max 10 items per 5-second tick. Slow SMTP/webhook calls (10s timeout) block the entire pipeline.

**Code:**
```go
// Line 520-534
pending, err := nq.GetPending(10)  // Max 10 items
// ...
for _, notif := range pending {
    nq.processNotification(notif)  // Sequential, blocking
}
```

**Impact:**
A single slow webhook endpoint (10s response time) reduces throughput to 6 notifications/minute instead of documented 120/minute.

**Fix:**
Process notifications concurrently with worker pool (e.g., 5 workers).

---

### 10. Queue DB Operations Not Atomic
**File:** `internal/notifications/queue.go:538-548`
**Severity:** High - Orphaned queue entries on crash

**Issue:**
`processNotification` updates attempts and status in separate SQL statements. Crashes between these operations leave orphaned rows.

**Code:**
```go
// Line 538-548
if err := nq.IncrementAttempt(notif.ID); err != nil { ... }  // UPDATE 1
if err := nq.UpdateStatus(notif.ID, QueueStatusSending, ""); err != nil { ... }  // UPDATE 2
// ... processor runs ...
```

**Impact:**
If Pulse crashes between IncrementAttempt and UpdateStatus, row stays in "pending" with incremented attempts, or gets stuck in "sending" forever.

**Fix:**
Use single atomic UPDATE or transaction for state transitions.

---

## Priority 1 - Important Issues (Fix Soon)

### 11. Webhook Rate Limiter Race Condition
**File:** `internal/notifications/notifications.go:1356-1394`
**Severity:** Medium - Rate limiting ineffective under concurrency

**Issue:**
`checkWebhookRateLimit` reads/writes `sentCount` and `lastSent` under the global `n.mu` mutex, but callers release the lock before sending. Concurrent sends can both pass the limit check before either increments the counter.

**Fix:**
Use per-URL mutex or atomic counters for rate limiting.

---

### 12. Email Manager Not Thread-Safe
**Files:** `internal/notifications/notifications.go:1010-1050` + `internal/notifications/email_enhanced.go:21-99`
**Severity:** Medium - Race condition in concurrent email sends

**Issue:**
`sendHTMLEmailWithError` grabs `n.emailManager` under read lock, then releases before calling `SendEmailWithRetry`. Meanwhile, the rate limiter mutates `sentCount` without any mutex.

**Fix:**
Add mutex to EnhancedEmailManager or use atomic counters for rate limiter.

---

### 13. Double Retry Logic (Queue + Transport)
**Files:** `internal/notifications/email_enhanced.go:38-78` + `queue.go:538-579`
**Severity:** Medium - Confusing retry semantics

**Issue:**
Email notifications configured with `MaxRetries=3` will retry 3 times per dequeue attempt, and the queue will retry the notification 3 more times, resulting in up to 9 total SMTP attempts.

**Fix:**
Document behavior clearly or disable transport-level retries when using queue.

---

### 14. Grouping Timer Leak on Disable
**File:** `internal/notifications/notifications.go:560-568`
**Severity:** Medium - Goroutine leak

**Issue:**
`SetEnabled(false)` doesn't cancel `groupTimer`. Timers created before disabling continue running and may fire after notifications are disabled.

**Fix:**
Call `groupTimer.Stop()` in `SetEnabled(false)`.

---

### 15. Webhook 429 Retry-After Double Sleep
**File:** `internal/notifications/webhook_enhanced.go:208-233`
**Severity:** Low - Longer delays than intended

**Issue:**
Enhanced webhooks sleep twice on 429 responses: once for backoff, again for Retry-After header.

**Fix:**
Use `max(backoff, retryAfter)` instead of adding them.

---

## Priority 2 - UI/Observability Gaps

### 16. Queue/DLQ Endpoints Missing from Frontend
**File:** `frontend-modern/src/api/notifications.ts`
**Severity:** Medium - Flagship features invisible to users

**Missing endpoints:**
- `/api/notifications/health` - Queue health status
- `/api/notifications/queue/stats` - Queue statistics
- `/api/notifications/dlq` - Dead letter queue
- `/api/notifications/dlq/retry` - Retry DLQ items
- `/api/notifications/dlq/delete` - Delete DLQ items
- `/api/notifications/webhook-history` - Webhook delivery history

**Impact:**
Users cannot observe queue depth, drain DLQ, or debug webhook failures without curl.

---

### 17. Apprise Test Notifications Not Supported in UI
**File:** `frontend-modern/src/api/notifications.ts:73-79, 169-190`
**Severity:** Low - Feature gap

**Issue:**
UI test functionality only supports `email` and `webhook` types, even though backend supports `apprise` test notifications.

**Impact:**
Apprise users cannot validate configuration from UI.

---

### 18. Enhanced Webhook Features Not Exposed
**File:** `frontend-modern/src/components/Alerts/WebhookConfig.tsx`
**Severity:** Low - Advanced features inaccessible

**Missing UI controls:**
- Retry configuration (`RetryEnabled`, `RetryCount`)
- Filter rules (by level, type, node, resource)
- Response logging toggle
- Custom payload templates (partially supported)

**Impact:**
Half the "enhanced webhook" feature set is inaccessible without manual API calls.

---

### 19. No Plaintext Config Warning
**File:** Frontend settings pages
**Severity:** Low - Security transparency

**Issue:**
UI never warns when `ConfigPersistence` lacks encryption, so admins don't know their SMTP passwords and webhook URLs are stored in plaintext.

**Fix:**
Show warning icon in settings when encryption is disabled.

---

## Security Deep Dive

### SSRF Protection Analysis

**Current protections (CreateWebhook/UpdateWebhook only):**
- Blocks localhost: `127.0.0.1`, `::1`, `127.*`
- Blocks private IPs: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
- Blocks link-local: `169.254.*`, `fe80::*`
- Blocks cloud metadata: `169.254.169.254`, `metadata.google.internal`
- Requires DNS resolution

**Bypasses:**
1. DNS rebinding (validated once, never re-validated)
2. Apprise HTTP mode (no validation at all)
3. URL templating with custom fields (can inject private IPs via template variables)

**Recommendation:**
Re-validate at send time, add Apprise URL validation, sanitize template outputs.

---

### Secrets at Rest

**Current behavior:**
- `ConfigPersistence` with crypto provider → AES-256 encrypted
- `ConfigPersistence` without crypto → plaintext JSON

**Files affected:**
- `/etc/pulse/email.json` - SMTP passwords
- `/etc/pulse/webhooks.json` - Webhook URLs with tokens
- `/etc/pulse/apprise.json` - Apprise API keys

**Recommendation:**
Either enforce encryption (fail hard without crypto) or emit warnings/health failures when encryption is disabled.

---

## Performance & Scalability

### Current Throughput Limits

**Email:**
- Rate limit: 60/minute
- Queue worker: 10 items per 5s = 120/minute theoretical
- Actual: ~6-10/minute with SMTP latency
- Retry overhead: 3x transport + 3x queue = 9x amplification

**Webhooks:**
- Rate limit: 10/minute per URL
- Queue worker: Same 10 items per 5s bottleneck
- Actual: ~6/minute with slow endpoints

**Recommendations:**
1. Concurrent queue workers (5-10 workers)
2. Separate queues for email vs webhooks
3. Shared HTTP client with connection pooling
4. Document actual vs theoretical throughput

---

## Recommendations Summary

### Immediate Actions (P0)
1. Fix cooldown timing - mark after delivery success
2. Add `os.MkdirAll` to queue initialization
3. Re-validate webhook URLs at send time
4. Remove secret logging or add redaction
5. Implement lastNotified cleanup
6. Use shared HTTP client for webhooks
7. Add fallback when enqueue fails
8. Implement queue cancellation for resolved alerts
9. Make queue worker concurrent
10. Use atomic DB operations for queue state

### Short-term Actions (P1)
1. Fix webhook rate limiter concurrency
2. Fix email manager thread safety
3. Document/fix double retry behavior
4. Fix timer leaks
5. Fix webhook 429 retry logic

### Medium-term Actions (P2)
1. Add queue/DLQ UI components
2. Add webhook history viewer
3. Add Apprise test support to UI
4. Expose enhanced webhook features in UI
5. Add encryption status warnings
6. Add queue health monitoring

---

## Testing Recommendations

### Unit Tests Needed
- Cooldown timing with queue failures
- Queue directory creation on fresh installs
- Concurrent sends (email + webhooks)
- Rate limiter under concurrency
- lastNotified cleanup
- Queue cancellation

### Integration Tests Needed
- DNS rebinding attack simulation
- Queue crash recovery (orphaned rows)
- Slow webhook blocking pipeline
- Enqueue failures with fallback
- End-to-end queue→DLQ flow

### Security Tests Needed
- SSRF bypass attempts (DNS rebinding, Apprise)
- Secret leakage in logs (debug level enabled)
- Plaintext config file exposure

---

## Conclusion

The notification system has strong foundations but critical reliability and security gaps. The queue infrastructure is well-designed but undermined by initialization failures, concurrency bugs, and missing observability.

**Most critical finding:** Cooldown timing + queue initialization failures combine to create a scenario where alerts are silently dropped, violating the core promise of a monitoring system.

**Most critical security finding:** DNS rebinding vulnerability allows SSRF attacks against cloud metadata services, potentially exposing credentials.

**Most critical UX finding:** Queue/DLQ features exist but are completely hidden from users, making the "persistent queue" feature essentially invisible.

All P0 issues should be addressed before the next release to restore trust in notification delivery.
