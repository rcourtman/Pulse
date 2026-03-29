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
Email single-alert, grouped, resolved, and HTML send paths must follow that
same ownership rule: they may expose different calling surfaces, but they must
all route through one canonical enhanced email executor instead of rebuilding
separate manager/config setup paths.
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
That same transport boundary also owns webhook request normalization. Rendered
webhook URLs must reject userinfo during validation, and request construction
must route through a validated absolute URL object instead of reparsing raw URL
strings at send time.
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
That same delivery boundary also owns SMTP mailbox normalization. `From`,
recipient, and `Reply-To` inputs must be parsed as canonical mailboxes before
headers or SMTP envelope commands are constructed, so notification delivery
cannot treat raw config strings as header fragments or `RCPT TO` input.
That same SMTP boundary also owns MIME-safe body construction. Text and HTML
payloads must be emitted through canonical multipart writers with encoded body
parts instead of being concatenated directly into handcrafted message bodies.
That same queue ownership also governs persistent queue storage roots. The
notifications queue database must normalize its owned data directory and
resolve the fixed `notification_queue.db` leaf through the shared storage-path
helper instead of joining raw caller-provided directory strings.

`internal/api/notifications.go` and
`frontend-modern/src/api/notifications.ts` are shared boundaries with
`api-contracts`: they are the product-facing control surface for
notification-management transport, while canonical payload-shape governance
still remains explicit in the shared API contract boundary.
