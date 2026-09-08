# Notifications Contract

## Contract Metadata

```json
{
  "subsystem_id": "notifications",
  "lane": "L6",
  "contract_file": "docs/release-control/v6/internal/subsystems/notifications.md",
  "status_file": "docs/release-control/v6/internal/status.json",
  "registry_file": "docs/release-control/v6/internal/subsystems/registry.json",
  "dependency_subsystem_ids": []
}
```

## Purpose

### Queue recovery handler ownership after reload

Router monitor replacement must refresh the existing queue/DLQ handler as well
as the normal notification handler. A stopped manager intentionally clears its
queue; retaining that manager after reload must not leave Retry, Dismiss or
queue reads permanently returning `Notification queue not initialized` while
the replacement manager is available. Handler monitor reads and replacement
are synchronised; missing monitors/managers remain an explicit 503, not a panic.
This changes neither queue state semantics nor notification delivery transport.
Requests already in flight during shutdown may still fail and can be retried.

Verification: `TestRouterSetMonitorRefreshesNotificationQueue` stops the old
notifier, replaces the router monitor, and exercises stats, Retry and Dismiss;
`TestNotificationQueueHandlers_MonitorReplacementConcurrent` covers concurrent
replacement and unavailable-manager reads with the race detector. This is
lifecycle regression proof, not installed SMTP receipt or reporter confirmation.


Own notification delivery transport, provider configuration, queueing, and
notification-management API surfaces.
The alert schedule selects firing, grouped, and matching recovery delivery
through one normalized target (`all`, `email`, `webhook`, or `apprise`).
Escalation delivery remains independently targetable per level, allowing an
Apprise/ntfy first notification to escalate through email, or the reverse.
Unknown and absent persisted targets preserve historical all-destination
behavior, and destination tag filters still apply after target selection.
Grouping is an explicit runtime policy: `grouping.enabled=false` or a zero
window delivers each alert independently, and disabling grouping flushes any
pending alerts as individual deliveries. Grouped provider payloads must retain
every alert, while live ntfy firing deliveries and webhook tests share the same
severity-derived title, priority, and tags.
Email rendering treats `patrol_finding` as a finding rather than a numeric
threshold alert. Single and grouped HTML/text bodies show the finding message
and optional category, omit meaningless zero-value metrics, preserve mixed
group headings, and HTML-escape every resource, message, and category field.
The alerts-owned append-only event log may record notification dispatch,
quiet-hours deferral, and suppression decisions at the alert manager's policy
seams. Those events explain why delivery was or was not attempted, but they do
not replace the notifications-owned queue, delivery log, retry/DLQ state, or
provider receipts and must not be treated as proof that a destination accepted
or displayed a notification.

## Canonical Files

1. `internal/notifications/notifications.go`
2. `internal/notifications/queue.go`
3. `internal/notifications/email_enhanced.go`
4. `internal/notifications/webhook_enhanced.go`
5. `internal/api/alerting/notifications.go`
6. `frontend-modern/src/api/notifications.ts`
7. `internal/operationaltrust/contracts.go`
8. `internal/api/alerting/notification_queue.go`
9. `internal/notifications/tag_routing.go`
10. `internal/notifications/delivery_health.go`
11. `internal/notifications/deadman_config.go`
12. `internal/notifications/failure_class.go`

## Shared Boundaries

1. `frontend-modern/src/api/notifications.ts` shared with `api-contracts`: the notifications frontend client is both a notification delivery control surface and a canonical API payload contract boundary.
2. `internal/api/alerting/notification_queue.go` shared with `api-contracts`: the notification queue and DLQ handler is both a notification delivery consequence surface and a canonical API payload boundary for operational transition links.
3. `internal/api/alerting/notifications.go` shared with `api-contracts`: notification handlers are both a notification delivery control surface and a canonical API payload contract boundary.
4. `internal/operationaltrust/contracts.go` shared with `alerts`: the operational trust contract is jointly consumed by canonical alert lifecycle ownership and notification delivery linkage without making delivery state operational truth.

## Extension Points

1. Add or change provider delivery, queue processing, or retry behavior through `internal/notifications/`
2. Add or change notification-management request or response handling through `internal/api/alerting/notifications.go`
3. Add or change notification-management frontend transport through `frontend-modern/src/api/notifications.ts`
4. Add or change the delivery-health verdict through
   `internal/notifications/delivery_health.go`. `ClassifyQueueHealth` is the
   single rule for whether configured destinations are reaching anyone, and it
   lives beside the queue that produces the counts so consumers cannot each
   carry their own copy. Only terminal failed or dead-letter outcomes count as
   unhealthy; recoverable retry attempts do not. A queue that cannot be read
   reports `unavailable` and never `healthy`, because an unreadable queue must
   not be mistaken for successful delivery. `internal/api/alerting/notifications.go`
   delegates to this rule rather than reimplementing it, and `monitoring`
   consumes it to raise the notification-delivery system alert. The queue also
   emits one process-local health-changed callback after a relevant transition
   has committed and after its database lock is released. Monitoring installs
   that callback and owns the alert projection; notification code must not
   raise or clear alerts directly. Retry and dismissal reconcile even when no
   row changed, because an already-repaired queue may still have a persisted
   stale warning from an earlier process.

## Forbidden Paths

1. Reintroducing notification delivery behavior as implicit side effects under `alerts` or generic monitoring ownership
2. Duplicating webhook or email delivery safety checks outside `internal/notifications/`
3. Letting notification queue, DLQ, or provider test paths drift away from the explicit proof routes in `registry.json`

## Completion Obligations

1. Update this contract when canonical notification entry points move
2. Keep notification API transport and backend delivery proofs aligned in `registry.json`
3. Preserve explicit queue, webhook-security, and provider-delivery coverage when notification behavior changes

The persistent queue and audit log carry typed `NotificationLink` entries for
every linked alert in a grouped delivery. A link keeps one notification id,
operational record id, lifecycle transition id, cause key, destination id, and
delivery state across queue retries. Queue state is delivery evidence only:
it must never create, resolve, acknowledge, suppress, or count operational
records. Resolution notifications link to the recovery-evidence transition.
Partial cancellation of a grouped firing delivery removes only the links for
the resolved alerts, while retry, failure, cancellation, and dead-letter state
remain inspectable in the queue and audit records. Destination identities are
stable opaque routing identities and must not expose credentials.

4. Webhook URLs are credential-bearing (Gotify, ntfy and Telegram all
   carry tokens in the path or query), so every surface that emits one
   must pass it through `RedactWebhookURLSecrets` first. This covers logs
   and returned errors alike, including the rate-limit drop paths in
   `checkWebhookRateLimit` and the enhanced sender, which are the sites
   most likely to fire repeatedly for a misconfigured destination.
   Redaction is proven by capturing log output rather than by reading the
   call sites, because a missed site is invisible to a source scan.
   Regression coverage: `TestWebhookRateLimitLogsRedactURLSecrets` and
   `TestRedactWebhookTransportErrorPreservesBehaviorWithoutToken` in
   `internal/notifications/webhook_url_redaction_test.go`.


## Current State

### Webhook retry delay conversion

`parseRetryAfterBackoff` bounds parsed integer seconds before multiplying by
`time.Second`, so large provider values cannot overflow into an immediate
retry. Signed 64-bit integer parsing makes this independent of native integer
width. Positive parsed seconds above the existing `WebhookMaxBackoff` cap
return that cap (30 seconds); non-positive parsed values retain the existing
immediate fallback. HTTP-date parsing and invalid-value fallback are unchanged;
integers outside signed 64-bit range remain invalid. Negative-value acceptance
is compatibility behaviour, not the non-negative delay-seconds grammar of
[RFC 9110 section 10.2.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3).

`TestParseRetryAfterBackoff` in
`internal/notifications/webhook_enhanced_test.go` pins both large positive and
negative overflow cases alongside ordinary seconds, dates, whitespace and
invalid inputs. This is pure parser proof: it does not establish elapsed HTTP
retry timing, queue persistence, provider acceptance or recipient receipt.

For HTTP 429 and 503 transport retries, a valid `Retry-After` overrides only the wait
following that response. It must not replace the independent exponential
backoff schedule. In particular, a zero-delay response cannot cause later
headerless 429 or 503 failures to exhaust their remaining retries immediately.
The schedule continues doubling up to `WebhookMaxBackoff`; retry budgets,
classification, parsing and the existing header-delay cap remain unchanged.
503 recovery hints use the same bounded parser as 429 rate-limit hints, as
described by RFC 9110 section 10.2.3. Invalid or absent hints retain exponential
backoff; other status codes do not gain header-based delay handling.

`TestSendWebhookWithRetry_ServiceUnavailableRetryAfter` in
`internal/notifications/webhook_enhanced_test.go` verifies that a 503 carrying
`Retry-After: 2` delays the next HTTP request by at least two seconds, rather
than the initial one-second backoff, and that a malformed hint retains that
initial backoff. It uses an owned loopback destination, not a live provider.

`TestSendWebhookWithRetry_ZeroRetryAfterPreservesLaterBackoff` in
`internal/notifications/webhook_enhanced_test.go` sends a zero-delay 429 or 503,
then a headerless 429 or 503, then success through a local HTTP fixture. It
verifies three requests and the two-second exponential wait before the final
request. This is transport timing proof, not durable queue retry, installed
provider acceptance or recipient receipt.

### Webhook transport retry exhaustion

The enhanced retry sender makes one initial HTTP attempt plus `RetryCount`
retries; non-positive counts select `WebhookDefaultRetries`. Repeated valid
zero-delay 429 hints do not reset that budget. When all attempts fail, the
sender returns the total attempt count and records one failed webhook history
entry with the final status and the number of retries, not one entry per HTTP
attempt. Every attempt retains the event ID and payload. A queue delivery that
uses this sender has a transport ceiling of effective retry count plus one;
layering up to `MaxAttempts` queue deliveries multiplies that ceiling, but does
not guarantee destination receipt. Permanent errors may stop retries earlier.

`TestSendWebhookWithRetry_RateLimitExhaustion` in
`internal/notifications/webhook_enhanced_test.go` verifies configured counts
1 and 2 and default selection for 0 and -1 against an owned HTTP fixture that
always returns 429 with `Retry-After: 0`. It checks request identity and body,
attempt exhaustion, and the single failed history record's status, retry
count, payload size and error text. This is transport-budget and local history
proof only, not queue persistence, provider receipt or installed qualification.

Notification-management HTTP production and its unit/contract proof now live
together under `internal/api/alerting/`. Router-level scope and integration
tests remain in `internal/api`, while the compatibility aliases there keep the
existing extension surface stable. This gives notification qualification a
native Go package scheduling boundary without changing routes or payloads.

This subsystem now makes email, webhook, Apprise, queueing, and delivery
safety explicit inside the current architecture lane instead of leaving them
implied by the broader alerts surface. A later lane split can still promote
alerts and notification delivery into their own product lane once the governed
floor is ready.

`internal/notifications/` is the live delivery engine. It owns provider
selection, secure webhook transport, Apprise delivery, queue persistence, DLQ
handling, retry policy, and delivery observability for alert-driven
notifications. Queue dequeue, direct-delivery fallback, and enqueue-failure
recovery now run through one owned notification-delivery executor instead of
keeping separate primary send paths. Single-alert, grouped, and resolved
webhook delivery now also share one owned rendering path for URL rendering,
service-specific enrichment, and template selection. When persistent queue
bootstrap fails, the runtime now falls back to an in-memory queue owner
instead of dropping into a separate nil-queue direct-send mode. Service-specific
	webhook compatibility like Pushover `app_token` / `user_token` legacy fields is
now canonicalized at webhook-config ownership boundaries, so runtime delivery
only handles canonical `token` / `user` fields instead of injecting aliases
mid-flight. That boundary includes config persistence plus API/UI ingress for
create, update, and ad hoc test requests; `internal/notifications/` may not
silently rewrite those legacy keys once webhook state is already live in the
runtime.

The webhook template registry is also the canonical source of truth for the
alert webhook service set and the metadata the frontend uses to build its
service chooser. Frontend presentation may format the options, but it must
derive the available services, labels, and descriptions from the backend
template registry instead of keeping a second hardcoded service list. Mention
field visibility plus mention-placeholder/help copy for supported services
must also come from the same backend registry so the editor does not carry a
second service-specific presentation map.
That same template-registry boundary owns JSON-safe string rendering for the
built-in webhook providers. Canonical JSON templates must render runtime
strings through the shared notification template helper that JSON-escapes
quoted, multi-line, and path-like alert content before validation, instead of
injecting raw alert fields directly into JSON bodies. Custom user templates
may still choose their own formatting, but the shipped provider templates may
not rely on callers to pre-sanitize alert text or resource names just to keep
their JSON payloads valid.
Teams Adaptive Card templates are part of that same built-in provider boundary:
resolved alert titles and resource-name text must pass through the JSON string
helper before entering Adaptive `TextBlock.text`, so a resource name containing
quotes or backslashes cannot invalidate the webhook payload.
Email single-alert, grouped, resolved, and HTML send paths must follow that
same ownership rule: they may expose different calling surfaces, but they must
all route through one canonical enhanced email executor instead of rebuilding
separate manager/config setup paths.
That enhanced email executor owns the production-manager reuse boundary as
well as the transport send itself. A test or ad hoc send whose SMTP host, port,
username, password, TLS, STARTTLS, or provider differs from the shared manager
must build an isolated delivery manager and leave the production manager
untouched, so unsaved relay-mode tests cannot inherit stale saved SMTP auth.
When the transport identity matches, the shared manager may still update
From/To and rate-limit presentation state so grouped and resolved sends keep
their persistent limiter continuity.
Notification test APIs must follow that same truthfulness rule: test email and
webhook paths may keep dedicated top-level entry points, but they must route
through canonical error-returning single-delivery executors instead of
fire-and-forget wrappers that can report false success.
Webhook test service/template synthesis must also stay inside
`internal/notifications/`: API handlers may decode the request and delegate,
but service-template selection, safe header copying, and generic test-template
fallback may not live as a parallel owner path under `internal/api/`.
Saved-webhook test actions, generic webhook test actions, and ad hoc webhook
test actions must also share the same enhanced webhook test executor so
service-template ownership, header normalization, and validation cannot drift
by entry point.
Enhanced webhook test/live delivery must follow that same ownership model:
`webhook_enhanced.go` may expose a richer config shape, but it must bridge back
into the canonical webhook render and transport path instead of maintaining a
parallel URL-rendering, enrichment, Telegram URL sanitization, or single-send
HTTP stack.
That same webhook payload boundary also owns tenant identity stamping. Webhook
payload data carries the tenant ID and display name of the runtime or
organization that fired the alert: environment-provided identity
(`PULSE_TENANT_ID` / `PULSE_TENANT_NAME`) is the construction-time default for
isolated client runtimes, the display name falls back to the ID, and
shared-process multi-tenant deployments override it through an org-backed
resolver installed by the monitoring subsystem. Templates consume it as
`{{.TenantID}}` / `{{.TenantName}}`, and the canonical generic template emits
its tenant block only when an identity is present so single-tenant payloads
keep their existing shape. PSA/ticket-bridge receivers must get tenant routing
identity from this payload boundary, not by inferring it from webhook endpoint
configuration.
That payload boundary also owns the language-neutral `MessageKey` exposed to
custom templates. Canonical alerts derive it from `canonicalAlertKind` and the
alert type (for example `metric-threshold.disk`); legacy alert paths use the
alert type directly. The generic JSON templates must emit the message key,
event, resource type, node display name and formatted metric values explicitly
so external receivers can translate or reconstruct notifications without
parsing English message text or reaching into the metadata map.
That same transport boundary also owns outbound delivery integrity. A webhook
config may carry an optional signing secret; when present, every JSON delivery
through the canonical webhook transport must send `X-Pulse-Timestamp` and
`X-Pulse-Signature` (`v1=` + hex HMAC-SHA256 over `timestamp + "." + body`),
computed at the single request-construction choke point and set after custom
headers so user-provided header maps cannot shadow them. Alert deliveries also
send `X-Pulse-Event-ID` (`alertID:event`) as the idempotency token; it must be
stable across both transport-layer and queue-layer retries of the same alert
occurrence. The management API must mask a configured signing secret on read
and preserve the stored secret when an update echoes the masked placeholder,
the same ownership rule it applies to header and custom-field secrets.
That same transport boundary also owns webhook request normalization. Rendered
webhook URLs must reject userinfo during validation, and request construction
must route through a validated absolute URL object instead of reparsing raw URL
strings at send time. The same SSRF guard must reject unspecified direct
targets such as `0.0.0.0` and `::` before delivery and must not allow webhook
private-CIDR allowlists to include networks that contain those unspecified
addresses.
The DNS-rebinding pin in the secure webhook dialer is part of that same guard:
hostname dials may connect only to resolver-validated IPs, and the dialer must
try every permitted resolved address in resolution order rather than pinning
the first, so an IPv6-first resolution (`::1` ahead of `127.0.0.1`) or a dead
leading A record cannot fail delivery to a host that is reachable on a later
permitted address.
That same ownership includes webhook retry classification. The canonical
retry gate in `webhook_enhanced.go` must parse provider failures from both
`status 429`-style and `HTTP 429`-style error strings before it decides
whether to retry, so a non-retryable `HTTP 400` result cannot be retried just
because the transport changed its error wording. Explicit HTTP status also
wins over network-like diagnostic text in the response body: a terminal 403
mentioning "authentication timeout" must stop transport retries, retain the
403 in delivery history, and record zero retries when rejected on the first
attempt. The existing transient HTTP exceptions and retryable fallback for
network or unclassified failures remain unchanged.
`TestIsRetryableWebhookError_StatusOverridesBody` and
`TestWebhookRetryRejectsForbiddenTimeoutBody` in
`internal/notifications/webhook_retry_test.go` pin this precedence with a
status/body matrix and a queue-free loopback receiver that checks request
count and delivery history. These are synthetic transport proofs, not installed
recipient receipts or a change to persistent-queue retry policy.
That same notification transport boundary also owns outbound Apprise HTTP URL
normalization. Server URLs must be validated as absolute HTTP(S) endpoints
without userinfo before request construction, and the `/notify` plus optional
config-key path must append to the canonical server base instead of being
rebuilt through raw string concatenation in local delivery code.
That same Apprise boundary also treats `ServerURL` as a canonical base URL,
not a request URL template. The owned runtime must reject query or fragment
state on that base, preserve any mounted base path, and resolve `/notify`
plus the optional config-key segment through shared URL helpers so delivery
cannot silently drop subpaths or reinterpret appended path segments.
That same delivery boundary also owns SMTP mailbox normalization. `From`,
recipient, and `Reply-To` inputs must be parsed as canonical mailboxes before
headers or SMTP envelope commands are constructed, so notification delivery
cannot treat raw config strings as header fragments or `RCPT TO` input.
That same SMTP boundary also owns MIME-safe body construction. Text and HTML
payloads must be emitted through canonical multipart writers with encoded body
parts instead of being concatenated directly into handcrafted message bodies.
That same SMTP boundary also owns email threading identity. An email that
covers exactly one alert occurrence must carry In-Reply-To and References
headers set to a deterministic incident thread ID derived from the alert ID
plus firing start time, so mail clients thread the firing, re-notification,
and resolved messages of one incident together. The per-send Message-ID must
stay unique: re-notified incidents emit multiple emails, and providers that
de-duplicate on Message-ID would silently drop repeats, so incident identity
may ride only in the threading headers. Grouped emails covering multiple
alerts must not carry a group-level thread identity, because firing and
resolved batches are not guaranteed to contain the same alert set and a
group-level reference would attach messages to the wrong thread.
Scheduled report delivery uses that same enhanced email boundary. Report
attachments must be emitted as MIME attachment parts by
`internal/notifications/email_enhanced.go`, and oversized-report fallback copy
belongs to the reporting scheduler. SMTP transport, recipient parsing,
headers, body encoding, and attachment encoding remain notification-owned.
That same queue ownership also governs persistent queue storage roots. The
notifications queue database must normalize its owned data directory and
resolve the fixed `notification_queue.db` leaf through the shared storage-path
helper instead of joining raw caller-provided directory strings.
That same queue owner also governs alert-resolution cancellation policy.
Cancelling queued work by alert identifier must remove outstanding firing
deliveries for that alert, but it must preserve already-queued resolved
notifications so recovery deliveries cannot be dropped just because the alert
was resolved before the queue drained. Cancellation must run even when there is
no in-memory grouped notification pending, because the persistent queue and the
delivery cooldown map are also notification-owned state for the alert
occurrence.
That same queue boundary also owns processor attachment semantics. The
canonical queue may persist pending notifications before a delivery processor is
configured, but it must not mark those entries sending, failed, or sent until a
processor exists. When a processor is attached, the queue owner must wake the
pending backlog through the same canonical batch path instead of relying on a
separate direct-send shortcut or waiting for an unrelated timer tick.
Alert delivery cooldown is also owned at this boundary. Normal alert delivery
must suppress duplicate sends for the same active alert occurrence when
cooldown is disabled or still active; scheduled escalation delivery is the
explicit exception and must route through the dedicated escalation send path so
the alert schedule, not transport cooldown, controls escalation cadence and
channel targeting.
The grouping timer is also notification-owned delivery state. Live alert
configuration must apply enabled, window, node, and guest grouping fields as
one policy update. Turning grouping off must stop the timer and deliver every
already-pending alert separately; service templates must be rendered only
after the grouped summary contains every alert, so provider-specific payloads
cannot silently collapse to the first alert.

`internal/api/alerting/notifications.go` and
`frontend-modern/src/api/notifications.ts` are shared boundaries with
`api-contracts`: they are the product-facing control surface for
notification-management transport, while canonical payload-shape governance
still remains explicit in the shared API contract boundary.

### Operational Trust delivery observability

Notification queue rows retain the exact operational record and lifecycle
transition IDs through grouping, retry, restart, send, cancellation, and dead
letter. Restart during a queued retry must reopen the same durable delivery
with the same transition links; it must not synthesize a new operational
transition. `internal/notifications/queue.go` records bounded queue,
retry/sent/failed/dead-letter/cancelled outcomes and open-to-enqueue latency
without destination, record, resource, or evidence labels. Delivery state
remains notification truth only and cannot resolve or reopen the alert
lifecycle.

The queue owner may expose a read-only, content-free telemetry aggregate over
its retained per-attempt audit rows: total delivery attempts, including
retries, plus successful deliveries and terminal failed/dead-letter outcomes
since a caller-supplied cutoff. A failed attempt that is returned to `pending`
for retry is not a terminal delivery failure. The aggregate must be computed in storage,
must not return or copy notification IDs, alert links, destinations,
recipients, endpoint URLs, titles, bodies, error text, or timestamps, and must
not mutate queue or alert state. The install-wide monitoring owner may sum that
aggregate across provisioned tenants for the outbound seven-day notification
outcome counters. Because completed queue rows are retention-bounded, this is a
seven-day delivery signal only and must not be presented as lifetime delivery
history or as proof that an alert was resolved.

Queue health is retention-bounded delivery truth. `GetQueueStats` counts every
row still retained by the queue: `sent`, `failed`, and `cancelled` rows for
seven days, dead-letter rows for 30 days, and nonterminal rows until they
complete. The notification health API must report `degraded` when any retained
`failed` or `dlq` row exists and `unavailable`, never healthy, when those
counts cannot be read. Pending retries do not degrade health. The response
must expose fixed reason codes and retention metadata rather than raw queue
errors or notification content.

Operator recovery is owned by the queue, not by database-file deletion. The
settings-write retry action returns all retained `failed` and `dlq` rows to
`pending`, resets their queue-attempt counters to a fresh retry budget, keeps
their operational links, and wakes the processor; it never rewrites existing
per-attempt audit rows. The settings-write dismiss action transitions those
same rows to `cancelled`, preserves both queue and audit history, and clears
the active health warning. Both actions are transactional across the selected
terminal set and report the number of rows actually transitioned.

### User-facing delivery log and honest test sends

The queue owner exposes its retained per-attempt audit rows to the local
notification-management API as a bounded, newest-first delivery log
(`GetDeliveryLog` in `internal/notifications/delivery_log.go`). Each entry
carries the attempt outcome (`sent`, `retry`, `failed`, `dead_letter`,
`cancelled`), derived by the same single rule that feeds the delivery outcome
metric in `RecordAudit`, plus alert identifiers, normalized destination
identity, attempt count, failure class, error text, and timestamp. Audit rows
persist destination identity in a dedicated `destination_id` column; rows
written before that column existed resolve destination identity from their
retained operational links. `GET /api/notifications/delivery-log`
(settings-read scope) serves the log as local operator evidence: webhook
secrets are redacted from error text at the API boundary, the payload names
its retention windows instead of presenting itself as lifetime history, and an
unreadable queue is an error, never an empty log. The API queries the longest
retained class (30 days) and reports `completed_retention_days: 7` plus
`dead_letter_retention_days: 30`, because completed audit rows are removed with
their seven-day queue rows while dead-letter audit rows remain with their
30-day queue rows. Reads default to 50 entries and are capped at 200; the
operator evidence view requests that bounded maximum. This per-attempt surface is
deliberately distinct from the content-free telemetry aggregate above, which
remains identity-free. Test sends bypass the queue and must not appear in the
delivery log, and the destinations UI says so where the log renders.
The frontend client normalizes the `entries` collection through the shared API
collection helper before validating each record, so a malformed collection or
row cannot bypass the canonical client boundary or create a module-local
fallback shape.

Because test sends also bypass the alert activation gate, a bare success
result is exactly how installs come to believe delivery works while every
real alert is suppressed. Successful responses from
`POST /api/notifications/test` and `POST /api/notifications/webhooks/test`
must therefore report `deliveryPaused: true` whenever the notification
manager is disabled, and the destinations UI must surface that as a warning
instead of a plain success toast.

### Occurrence-bound delivery receipts

The notification owner records successful firing delivery by exact alert ID,
nanosecond start time, and a normalized destination identity. Email recipients,
webhook ID plus URL, and Apprise mode/base/config/targets are part of that
identity, so a later occurrence or reconfigured destination cannot inherit an
older receipt. A resolved notification is eligible only for destinations that
received that exact firing occurrence. Successful recovery delivery deletes
the receipt; persistent receipts survive restart and are retention-bounded.
Receipt read failure suppresses recovery rather than inventing prior delivery.

### Destination alert routing

Email and every enabled alert webhook may define a normalized resource-tag
filter and an `all` or `any` match mode. Empty filters preserve global
delivery. Matching is exact after trimming and case folding, and grouped
firing alerts are filtered independently for each destination; Apprise remains
global by resource tag. Tags may arrive through the canonical `tags`, `resourceTags`,
`hostTags`, or `serviceTags` alert metadata keys.

Email, every enabled alert webhook, and Apprise may also define a normalized
minimum severity of `all`, `warning`, or `critical`. Omitted or unknown values
preserve backwards-compatible all-alert delivery. Grouped firing alerts are
filtered independently per destination, after both tag and severity policy are
applied, so one incident batch may produce different destination payloads.
The meanings are exact: `all` accepts info, warning, and critical; `warning`
accepts warning and critical but excludes info; `critical` accepts only
critical. API normalization, frontend editing, and persisted configuration
must round-trip the warning floor rather than reducing it to all-alert
delivery.

The delivered payload must preserve the selected alerts' canonical severity.
Single and grouped email subjects, summary counts, alert rows, and plain-text
fallbacks represent Info separately from Warning. ntfy derives the highest
severity in a group without promoting an all-info group: informational
delivery uses the `INFO` title prefix, default priority, and
`information_source` tag; warning and critical retain their higher attention
postures. Destination filtering is not complete if the final template relabels
or visually promotes the alert.

Resolved delivery deliberately bypasses current tag and severity matching and
is filtered by occurrence-bound delivery receipts instead. This guarantees that a
destination which received a firing occurrence can receive its recovery even
if resource tags or destination policy change. Configuration copies must
isolate filter slices, mixed-version updates must preserve omitted routing
fields, and explicit empty filters must remain the supported clear operation.

Cooldown state is published only after firing receipts are recorded. Persistent
workers atomically claim a still-pending row after taking per-alert delivery
gates, and resolution cancellation takes the corresponding exclusive gates.
This prevents a stale pending snapshot from being sent after cancellation while
preserving grouped-row and operational-link lifecycle semantics. A destination
with delivery disabled or failed has no receipt and receives no misleading
recovery. These rules remain delivery truth only and cannot resolve the
alerts-owned lifecycle.

`internal/notifications/delivery_receipts_test.go`,
`internal/notifications/notifications_test.go`, and
`internal/notifications/queue_test.go` prove occurrence and destination
isolation, persistence, cleanup, and cancellation/claim ordering.

### External watchdog transport is a credential-bearing destination boundary

`internal/notifications/deadman_config.go` owns normalization and validation
for healthchecks-compatible success-ping URLs. Possession of the URL can forge
healthy state, so configuration is encrypted at rest, exported only inside the
passphrase-encrypted configuration bundle, and represented to API clients by a
redacted sentinel. URLs are bounded to HTTP(S), exclude userinfo, fragments,
localhost, loopback, unspecified, and link-local targets, and must name the
base success endpoint rather than `/start`, `/fail`, or `/log`.
Endpoint suffix validation inspects the once-decoded URL path, including
encoded separators and an encoded trailing slash, so percent-encoded event
names cannot bypass this success-only guard. Encoded base tokens and suffix
words in query values remain valid; the configured URL is not rewritten.
`TestValidateDeadManPingURL` covers these rejected and accepted forms.
`TestDeadManRunCycleRejectsEncodedNonSuccessEndpoint` proves rejection before
transport, a misconfigured state without a success timestamp, and no token in
the diagnostic. These are local validation/runtime proofs, not installed
watchdog acceptance or recipient delivery evidence.
Literal addresses are also compared against every Pulse host interface, while
the monitoring dialer repeats that comparison after DNS resolution. Failure to
enumerate local interfaces fails closed. A private LAN watchdog remains valid
only when its resolved address belongs to a different machine; pointing a
hostname or normal LAN address back at Pulse is rejected as same-host.

The monitoring-owned sender uses a dedicated transport that bypasses ambient
HTTP proxies, revalidates DNS answers at dial time, never follows redirects,
and returns sanitized error classes that cannot disclose the URL or token.
Private LAN watchdogs remain valid when separately hosted. Network and 5xx
failures receive two bounded retries; permanent response failures do not. A
healthy signal is GET, while canonical-loop stall and restart-gap diagnostics
are bounded text POSTs containing Pulse health and UTC timing only—never alert
content, infrastructure names, destination credentials, or tenant data. This
watchdog path is deliberately independent of notification queue activation,
quiet hours, grouping, and escalation routing.

### Destination configuration publishes only committed state

Email, Apprise, and webhook create, update, and delete handlers treat encrypted
configuration persistence as the publication boundary. They serialize
destination writes, build complete webhook candidate inventories, persist the
candidate first, and only then update the live notification manager. A failed
write returns `500` and leaves the live destination inventory unchanged, so an
operator cannot receive success for routing state that a restart would undo or
have queued delivery cancelled by an uncommitted disable action. The forced
failure paths are pinned in
`internal/api/alerting/notifications_test.go`; the API ordering proof lives in
`internal/api/contract_test.go`.

### Escalation delivery can select exact logical destinations

Escalation dispatch accepts optional logical destination IDs in addition to the
legacy channel target. Exact selection is applied before destination tag and
severity policy: `email` selects the enabled email job, `apprise` selects the
enabled Apprise job, and `webhook:<id>` selects only that enabled webhook.
Unknown or unavailable IDs produce no job and never fall back to a broader
channel. A no-job exact selection also leaves notification cooldown state
untouched so configuration drift cannot masquerade as successful paging.

Destination IDs never contain endpoint URLs, credentials, or Apprise targets.
All retry, queue, receipt, grouping, and recovery semantics remain owned by the
selected notification job after routing.

### Terminal destination failures distinguish rejection from server failure

The closed notification failure-class contract uses `rejected` for newly
recorded destination HTTP 4xx responses after the dedicated authentication and
rate-limit classes, and `server_error` for newly recorded HTTP 5xx responses.
Explicit HTTP 5xx status evidence is classified before generic timeout wording
so a 504 remains a destination server failure rather than connectivity.
Pre-upgrade audit rows retain their original class until retention removes them.
Both classes are recorded only on local audit rows and exported, when telemetry
is enabled, as identity-free aggregate terminal failure counts; raw response
content and destination identity never leave Pulse.

`internal/notifications/queue_test.go` pins the local classification order and
the terminal-only retry/dead-letter accounting boundary.

### Delivery failure class is declared by the sender, never read from a response

The failure class is authoritative where the sender knows it and heuristic only
where nobody does. `internal/notifications/failure_class.go` owns that order:
a class declared by the sender through `NotificationFailureError` wins, then
Go's own error types decide (`*textproto.Error` carries the SMTP reply code,
x509 and TLS errors mean `tls`, `net.DNSError` and timeouts mean
`connectivity`), and only then is the error message text consulted.

Two rules follow and are not optional. A destination's response body must never
determine its own failure class: every site that builds an error from an HTTP
status classifies from the status code, so the body is retained for the
operator's audit row but cannot make a 500 that says "rate limit" record as
`rate_limited`. And an SMTP reply code is not read like an HTTP status: a 5xx
reply is the destination refusing the message (`rejected`, or `authentication`
for 530/534/535/538 and `configuration` for the 500-504 syntax replies), while
only the transient 4xx replies are `server_error`.

`RecordAuditError` is the canonical audit entry point because it is the one
that preserves a declared class; `RecordAudit` re-derives from text and is
retained only for callers that never had the error value.

`internal/notifications/failure_class_test.go` pins the precedence order, the
SMTP reply-code mapping, and the rule that response-body text cannot steer the
recorded class.

### The retry ladder is gated on the failure class

Delivery retries are for conditions that can clear. `authentication`,
`configuration`, and `rejected` are verdicts about the request itself: the same
payload, sent again to the same destination with the same credentials, gets the
same answer. Those three dead-letter on the attempt that produced them, without
consuming the remaining ladder. `connectivity`, `rate_limited`, `server_error`,
`tls`, and an unclassified failure keep the full ladder, because nothing about
them proves a later attempt fails. TLS is deliberately on the retrying side: a
handshake can fail transiently during rotation, and the cost of one wasted
ladder is lower than the cost of dropping a recoverable notification.

`NotificationFailureClass.Retryable` is the single owner of that split. Neither
the queue nor any destination type may keep its own list. This generalises to
every destination the decision webhook delivery already made for HTTP 4xx in
`isRetryableWebhookError`.

The SMTP transport's inner retry loop must apply that same classifier after
an unsuccessful send, before sleeping or attempting another connection.
Permanent authentication, configuration, and rejection failures return on that
attempt; transient failures retain up to `MaxRetries + 1` transport attempts.
The returned error preserves the structured cause and reports the actual
attempt count, not the configured maximum. This applies equally to ordinary,
threaded, and attachment email through `sendEmailWithOptions`.

`internal/notifications/email_retry_class_test.go` exercises the real sender
with in-memory SMTP handshake failures: permanent 550/535/554/501 replies stop
at one attempt, while transient 421 retains three configured attempts even
when its prose mentions authentication. It checks classification and the
reported attempt count. This is transport-only proof, not queue persistence,
post-DATA acceptance, or an installed recipient receipt.

Dead-lettering early must not lose the notification: `RetryTerminalFailures`
remains the operator's recovery path, returning eligible retained terminal
failures to the queue with a fresh budget once the credentials or configuration
are fixed. Resolution removes obsolete firing entries from that eligibility;
retry must not resurrect an incident which has already recovered.
A dead-letter row records `failureClass` and a `deadLetterReason` of
`failure_class_not_retryable` or `max_retries_exhausted` so the two are
distinguishable in local logs.

`internal/notifications/failure_class_test.go` pins the retryable split and
that a deterministic failure dead-letters on its first attempt.


### Resolution remains final across terminal retries and restart

Resolution cancellation covers pending, sending, failed, and dead-lettered
firing rows. A wholly obsolete row becomes cancelled; a grouped row retains
only unrelated firing alerts and their operational links. Recovery jobs are
not cancelled by this operation. Only removed pending entries contribute to
the pending-suppression return count; terminal or interrupted sends must not
be counted as proof that firing was never delivered.

The cancellation verdict persists across queue reopen and bulk operator
retry. Per-item retry also rejects cancelled and already-sent rows atomically,
so a stale retry request cannot bypass resolution or duplicate a completed
delivery. Pending, sending, failed, and dead-lettered rows remain eligible for
the existing retry scheduler.

Cancelling a row retains its failed-attempt audit history and announces the
changed queue-health verdict only after releasing both the database mutex and
per-alert delivery gates. Clearing obsolete retained failures does not prove
that a destination has been repaired. Nor does this operation retrospectively
identify obsolete rows whose resolution happened before this behaviour was
installed; historical backlogs still require incident reconciliation.

`internal/notifications/queue_resolution_retry_test.go` proves failed and
dead-lettered cancellation across durable reopen and actual queue processing,
preservation of unrelated grouped firing and recovery jobs, retained failed
attempts, callback lock release and committed-state visibility, and the
per-item retry eligibility matrix. These are component proofs, not installed
receiver receipts or exactly-once delivery guarantees.

### Disabled delivery is cancellation, not a receipt

At processing time, globally disabled delivery or a disabled/removed destination
returns `ErrNotificationDeliverySkipped`. The queue persists that job as
cancelled with the policy reason and cancelled operational links. It does not
write a provider-attempt audit, a successful receipt, or a delivery failure, and
operator retry does not replay the cancelled job. Existing attempt history is
retained. Queue health reconciliation runs after releasing the database mutex
and alert delivery gates.

`queue_disabled_delivery_test.go` exercises the real manager/queue boundary for
email, webhook and Apprise, firing and recovery, and global versus destination
disablement. This corrects false successful queue/audit records; it does not
establish maintenance-window expiry, stop an already-started provider request,
or repair historical false-success records.

### SMTP transaction-stage retry regression coverage

`TestEmailRetryRespectsSMTPTransactionReplies` extends the greeting-failure
fixture through MAIL, RCPT, DATA command and completed-message replies. At each
stage, structured 550 replies stop after one attempt despite temporary prose;
451 replies retain the three-attempt configured budget, and a 451 followed by
acceptance stops after the second attempt. This checks the sender's wrapped
errors and retry termination, not only the failure classifier. The in-memory
plain-SMTP fixture sends no external mail and does not qualify TLS, installed
recipient receipt or queue persistence.

### Webhook diagnostic userinfo confidentiality

`RedactWebhookURLSecrets` masks the entire URL userinfo (including username-only
credentials), before its existing Telegram-path and query-secret redaction.
Unparseable URLs produce `[invalid webhook URL]`, not a raw credential-bearing
fallback. Valid destination host/path and non-secret query fields remain useful
for diagnosis. Transport-error redaction copies the URL error and retains its
underlying cause without changing the original error or the configured URL.
This follows the credential-exclusion principle in the
[OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html#data-to-exclude).

Regression tests cover plain/encoded/user-only credentials, malformed URLs,
non-authority at signs, combined path/query redaction, error unwrapping and
actual rate-limit log output. These queue-free tests establish local diagnostic
redaction, not destination receipt, installed recovery or release qualification.
No claim is made that arbitrary custom path/query secrets are recognised.

Delivery-log errors use `RedactWebhookDiagnosticSecrets` so URLs embedded in
otherwise useful error text receive the same masking without discarding the
surrounding status context. Malformed embedded URLs still fail closed.

### Slack webhook diagnostic path confidentiality

The same helper masks paths on the exact `hooks.slack.com` and
`hooks.slack-gov.com` hosts. `/services/` remains as a diagnostic marker; legacy
paths become `/REDACTED`. Matching uses the parsed, case-insensitive hostname
and clears the encoded path representation, so ports and escaped path segments
do not bypass masking. Other hosts retain their diagnostic paths. Userinfo and
known query credentials remain redacted; configured destinations are unchanged.

[Slack's incoming-webhook documentation](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/)
identifies the webhook URL as secret and documents GovSlack's separate domain.
Queue-free regression tests cover both hosts, encoded and legacy paths,
lookalike/unrelated hosts, transport errors and actual rate-limit log output.
This does not establish customer exposure, recipient receipt or recognition of
arbitrary custom webhook secrets.

### Discord webhook diagnostic path confidentiality

On exact `discord.com` and legacy `discordapp.com` hosts, the shared redactor
masks the suffix after `/webhooks/`, including the webhook ID and token.
Versioned API prefixes remain visible; encoded paths, host casing and ports
cannot bypass masking. Unrelated paths and lookalike hosts are unchanged.
Configured destinations and transport error causes are not modified.

[Discord's webhook reference](https://docs.discord.com/developers/resources/webhook)
identifies the secure webhook token and token-authorised operations. Focused
synthetic regressions cover helper output, transport diagnostics and actual
rate-limit logs. This is not evidence of customer exposure, recipient receipt,
release qualification, or protection of arbitrary custom-host credentials.

### Telegram diagnostic path parsing

Telegram bot-path masking operates on the parsed, decoded URL path and clears
RawPath after replacement. This covers percent-encoded bot prefixes without
mistaking a hostname or a URL inside a query for a bot path. Method suffixes,
query diagnostics and fragments remain intact; configured destinations and
transport-error causes are unchanged. Host-independent masking is retained for
local API servers, which are supported by the
[Telegram API documentation](https://core.telegram.org/bots/api#making-requests).

Focused regression tests cover escaped prefixes/tokens, local servers, missing
method suffixes, query URLs, fragments, transport errors and rate-limit logs.
This is diagnostic containment, not evidence of customer exposure or recipient
delivery. Arbitrary path secrets and unrecognised query credentials remain
outside this bounded change.

### Bounded diagnostic confidentiality: query representations and bypass callers

Recognised query names are exactly token, apikey, api_key, key, secret and
password after one URL query decode. Every repeated occurrence is masked,
including mixed literal/escaped names. Unrelated names, ordering and values
remain intact; invalid name escapes fail closed. This is diagnostic projection,
not mutation of configured destinations or a claim to recognise arbitrary secrets.

Resolved ntfy must apply the same transport-error projection before both its
error log and returned error. Common HTTP execution preserves payload bytes,
event identity and error causes; URLs containing userinfo remain rejected by
outbound validation even though historical diagnostic userinfo is masked.

The caller matrix and retained Delivery regression tests exercise URL/message
helpers, actual rate-limit logs, common transport and resolved-ntfy transport
errors/logs with synthetic secrets. HTTP delivery-log regression verifies encoded
and repeated query credentials while retaining diagnostic context and entry
identity. Existing exact-output tables bound Slack/GovSlack/legacy, Discord,
Telegram/local paths, malformed URLs and non-secret lookalikes. Earlier proof
missed decoded query representations and a separate ntfy transport caller:
provider-only helper examples were not sufficient sink coverage. This contract
does not assert arbitrary response-body/third-party error secrecy, installed
recipient delivery, candidate qualification or historical customer exposure.

### Saved notification choices at monitor construction

`internal/monitoring/monitor_notification_startup_test.go` exercises the actual
monitor constructor twice against the same saved configuration without applying
notification-manager setters in the fixture. It checks initial routing, both
resolve choices, enabled/activation gating and encrypted webhook configuration
restoration. Email and HTTP Apprise receiver settings are also saved and
compared after each construction, including synthetic credentials, recipient
lists, severity filters, email tag filters and enabled/disabled choices.
No transport is invoked. The cases include webhook, Apprise, email and all
destinations.

This boundary uses the configuration persistence API to save choices, not the
HTTP or browser save path. It does not start a new operating-system process,
drive an alert lifecycle or establish recipient delivery. Those installed
acceptance obligations remain separate from constructor restoration coverage.

### Interrupted webhook rejection bodies retain the HTTP verdict

When webhook response headers establish a non-2xx status, a subsequent body
read failure retains that HTTP status in the error and its authoritative
notification failure class. Terminal rejections must not become connectivity
retries merely because the diagnostic body is truncated. Response headers,
including Retry-After, and the underlying read error remain available; partial
body text is not added to the error. Existing handling of 2xx body read errors
is unchanged: it remains a read failure, not proof of recipient receipt.

Transport-only `TestWebhookTruncatedResponseClassification` covers 200, 401,
403, 422, 429 and 503 with interrupted bodies. `TestWebhookRetryTruncatedResponse`
proves a truncated 403 stops after one attempt with terminal history, while a
truncated 503 can retry to 204 with its event identity intact. No queue or
storage workers are started by these tests; this is not installed delivery
acceptance or a change to retry budgets.

### HTTP retry classification agrees across delivery layers

HTTP 421 retains a connectivity class; 423 and 425 retain a server-error class
for temporary receiver conditions. These are already retryable exceptions in
the webhook transport, and must not become terminal rejections when a wrapped
transport error reaches the queue. Authentication, configuration and other
permanent HTTP rejections still stop early; attempt limits remain unchanged.

`TestWebhookHTTPRetryPolicyMatchesQueueClassification` exercises every status
400–599 through a synthetic HTTP transport with misleading diagnostic text,
wraps its returned error and checks both transport retry policy and the shared
class predicate used by the queue. `TestClassFromHTTPStatus` pins the reason
classes. This proves the classification boundary without starting queue/storage
workers; it does not establish installed receipt or queue scheduling execution.
