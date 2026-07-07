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

## Canonical Files

1. `internal/notifications/notifications.go`
2. `internal/notifications/queue.go`
3. `internal/notifications/email_enhanced.go`
4. `internal/notifications/webhook_enhanced.go`
5. `internal/api/notifications.go`
6. `frontend-modern/src/api/notifications.ts`

## Shared Boundaries

1. `frontend-modern/src/api/notifications.ts` shared with `api-contracts`: the notifications frontend client is both a notification delivery control surface and a canonical API payload contract boundary.
2. `internal/api/notifications.go` shared with `api-contracts`: notification handlers are both a notification delivery control surface and a canonical API payload contract boundary.

## Extension Points

1. Add or change provider delivery, queue processing, or retry behavior through `internal/notifications/`
2. Add or change notification-management request or response handling through `internal/api/notifications.go`
3. Add or change notification-management frontend transport through `frontend-modern/src/api/notifications.ts`

## Forbidden Paths

1. Reintroducing notification delivery behavior as implicit side effects under `alerts` or generic monitoring ownership
2. Duplicating webhook or email delivery safety checks outside `internal/notifications/`
3. Letting notification queue, DLQ, or provider test paths drift away from the explicit proof routes in `registry.json`

## Completion Obligations

1. Update this contract when canonical notification entry points move
2. Keep notification API transport and backend delivery proofs aligned in `registry.json`
3. Preserve explicit queue, webhook-security, and provider-delivery coverage when notification behavior changes

## Current State

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

`internal/api/notifications.go` and
`frontend-modern/src/api/notifications.ts` are shared boundaries with
`api-contracts`: they are the product-facing control surface for
notification-management transport, while canonical payload-shape governance
still remains explicit in the shared API contract boundary.
