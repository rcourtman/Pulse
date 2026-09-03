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
because the transport changed its error wording.
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
