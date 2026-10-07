# 🔔 Webhooks

Pulse includes built-in templates for popular services and a generic JSON template for custom endpoints.

## 🚀 Quick Setup

1. Go to **Alerts → Notifications**.
2. Click **Add Webhook**.
3. Click the current service label (Generic by default) to open the service picker, choose the destination type, and paste the URL.

A successful **Test** is not proof of queued alert delivery. For missing alerts
or rejected requests, follow the [notification troubleshooting guide](TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing)
before retrying retained failures. In particular, a destination edit does not
replace the settings saved in an old queued delivery; a retry can still use its
original URL and credentials. For built-in Telegram's "message text is empty"
error despite a successful Test, see the [static-header check](TROUBLESHOOTING.md#telegram-test-works-but-real-alerts-say-message-text-is-empty).

## 📝 Service URLs

| Service | URL Format |
|---------|------------|
| **Discord** | `https://discord.com/api/webhooks/{id}/{token}` |
| **Slack** | `https://hooks.slack.com/services/...` |
| **Teams** | `https://{tenant}.webhook.office.com/webhookb2/{webhook_path}` |
| **Teams (Adaptive Card)** | `https://{tenant}.webhook.office.com/webhookb2/{webhook_path}` |
| **Telegram** | `https://api.telegram.org/bot{bot_token}/sendMessage?chat_id={chat_id}` |
| **PagerDuty** | `https://events.pagerduty.com/v2/enqueue` |
| **Pushover** | `https://api.pushover.net/1/messages.json` |
| **Gotify** | `https://gotify.example.com/message?token={token}` |
| **ntfy** | `https://ntfy.sh/{topic}` |
| **Generic** | `https://example.com/webhook` |

## 🎨 Custom Templates

For generic webhooks, use Go templates to format the JSON payload.

**Variables (common):**
- `{{.ID}}`, `{{.Event}}`, `{{.MessageKey}}`, `{{.Level}}`, `{{.Type}}`
- `{{.ResourceName}}`, `{{.ResourceID}}`, `{{.ResourceType}}`, `{{.Node}}`, `{{.NodeDisplayName}}`
- `{{.Message}}`, `{{.Value}}`, `{{.Threshold}}`, `{{.Duration}}`, `{{.Timestamp}}`
- `{{.Instance}}` (Pulse public URL if configured)
- `{{.TenantID}}`, `{{.TenantName}}` (tenant identity in multi-tenant orgs and MSP client runtimes; empty on plain single-tenant installs)
- `{{.CustomFields.<name>}}` (user-defined fields in the UI)
- `{{.Metadata}}` (alert metadata map)
- `{{.AlertCount}}`, `{{.Alerts}}` (grouped alerts)
- `{{.Mention}}` (platform-specific mention, if configured)

**Convenience fields:**
- `{{.ValueFormatted}}`, `{{.ThresholdFormatted}}`
- `{{.StartTime}}`, `{{.Acknowledged}}`, `{{.AckTime}}`, `{{.AckUser}}`

**Template helpers:** `title`, `upper`, `lower`, `printf`, `urlquery`/`urlencode`, `urlpath`/`pathescape`, `jsonString`

`jsonString` is the safe way to embed string values inside a JSON payload — it escapes quotes, backslashes, and control characters without wrapping the value in surrounding quotes, so you can write `"text": "{{.Message | jsonString}}"` and stay valid JSON even when the message contains `"` or newlines. Pulse's shipped templates use it extensively; prefer it over manual escaping in custom templates.

**Service-specific notes:**
- **Telegram**: include `chat_id` in the URL query string.
- **Telegram templates**: `{{.ChatID}}` is populated from the URL query string.
- **PagerDuty**: set `routing_key` as a custom field (or header) in the webhook config.
- **Pushover**: add `token` and `user` custom fields (required). Legacy `app_token` and `user_token` inputs are migrated automatically.
- **ntfy**: choose **ntfy** in the service picker before entering the topic URL. Leave the service as Generic only when you want to send a custom JSON payload.

**Example Payload:**
```json
{
  "text": "Alert: {{.Level | jsonString}} - {{.Message | jsonString}}",
  "value": {{.Value}}
}
```

Keep `jsonString` inside the JSON string's quotes. Leave numeric `.Value`
unquoted. A simple test message can work without escaping while a real alert
containing quotes, backslashes or newlines produces invalid or altered JSON.

This minimal payload is a text summary, not a structured record of every
group member or its firing/recovery identity. Use the [full PSA payload](#sample-psa-payloads)
when a receiver needs those fields; do not use the summary alone to deduplicate
incidents or close tickets.

## 📦 Delivery Contract

These fields and behaviors are stable; ticket-routing integrations can rely on them.

**Events.** `{{.Event}}` is `"alert"` or `"resolved"` — there is no separate
"info" event class. Recovery delivery depends on **Notify on resolve** and a
successful firing-delivery receipt for that occurrence and destination. Do not
assume that every configured webhook receives both events.

**Severity.** Metric alerts commonly use `"warning"` or `"critical"`;
informational conditions can use `"info"`. Event and severity are separate:
an informational firing still has event `"alert"`. Preserve and flag an
unrecognised level rather than silently dropping the notification. Minimum
severity and other delivery policies still apply.

**Alert type.** `{{.Type}}` is the metric or condition that fired: `cpu`, `memory`, `disk`, `diskRead`, `diskWrite`, `networkIn`, `networkOut`, `connectivity`, and similar. The alert ID (`{{.ID}}`) correlates firing and recovery, but can be reused when the same condition fires again. It is not a unique incident or delivery ID.

**Message key.** `{{.MessageKey}}` is the stable, language-neutral condition key for rebuilding or translating a notification. Canonical alerts use `<kind>.<type>` (for example, `metric-threshold.disk`); older alert paths fall back to `{{.Type}}`. Combine it with `{{.Event}}` and `{{.ResourceType}}` when the receiving system needs separate wording for firing/recovery events or different resource classes. Unlike a numeric message index, the symbolic key does not change when another alert type is added.

**Tenant identity.** In multi-tenant organizations and MSP client runtimes, `{{.TenantID}}` and `{{.TenantName}}` identify which tenant fired the alert. Client runtimes get identity from the `PULSE_TENANT_ID` / `PULSE_TENANT_NAME` environment; shared-process organizations stamp the org ID and display name automatically.

**Resource tag routing.** Email and each alert webhook can be limited to resources with selected tags in **Alerts → Notifications**. An empty filter receives every alert. With multiple tags, choose **Match all tags** or **Match any tag**. Matching ignores case. Proxmox tags are matched as shown; Docker container and service labels are exposed as `key:value` tags (or `key` when the label value is empty). Recovery notifications follow the destinations that received the firing alert, even if a resource's tags change before recovery.

**Retries and retained failures.**

Normal queued firing and recovery webhooks have a budget of **up to three
queue delivery attempts, including the initial attempt**. This is a ceiling,
not a promise to retry every failure or a guarantee of delivery. The normal
queued webhook sender has no extra transport retry loop; do not assume three
extra HTTP retries or a provider's `Retry-After` wait on this path.

| Observed failure | Automatic queue behaviour |
| --- | --- |
| Authentication, configuration or rejected request (most HTTP 4xx, including 400, 401 and 403) | Stops as soon as this failure is classified, even with attempts left; retains the delivery as a terminal failure. |
| HTTP 408, 421, 423, 425 or 429; HTTP 5xx; connectivity or unknown failure | Can retry with backoff while the saved attempt budget remains; exhaustion retains a terminal failure. |
| TLS failure | Can retry within the saved budget, but retrying does not repair certificate trust, expiry or hostname errors. Do not disable verification. |

A terminal failure means **no further automatic retry**, not that the delivery
was successful or its history was deleted. In **Alerts → Notifications**, use
**Recent delivery activity** to check the destination, timestamp, failure class
and HTTP status. A pending or held delivery is not a terminal failure; quiet
hours and other delivery policies can postpone it. There is no fixed delivery
deadline promised by the attempt count.

Before choosing **Retry retained deliveries**, follow
[retained-failure recovery](TROUBLESHOOTING.md#recover-retained-delivery-failures).
It acts on all retained terminal failures, not just one webhook, and keeps their
original destination settings. A successful **Test** uses current settings and
does not validate or resend those saved deliveries. Do not repeat tests or
batch retries to diagnose rate limiting. A receiver can see a duplicate if an
earlier request was accepted but Pulse did not receive its response; preserve
[receiver deduplication](#receiver-correlation-and-deduplication).

**Correlation header.** Alert webhooks carry `X-Pulse-Event-ID` in the form
`<alertID>:<event>` (e.g. `a1b2c3:alert`, `a1b2c3:resolved`). Retries retain it,
but later occurrences, severity changes and reminders can share it too. A group
uses its primary alert's ID, not a unique ID for every member or batch. **Do not
deduplicate permanently on this header or on alert ID and event alone:** doing
so can silently discard a later incident or a changed group. See
[receiver correlation and deduplication](#receiver-correlation-and-deduplication).

**Signed deliveries.** Set a `signingSecret` on the webhook config to enable HMAC signing. Signed requests carry:

- `X-Pulse-Timestamp`: Unix seconds at send time.
- `X-Pulse-Signature`: `v1=` + hex HMAC-SHA256 over `timestamp + "." + body`, keyed with the shared secret.

Verify before parsing the JSON or performing any action. Read the original body
as bytes and the complete `X-Pulse-Timestamp` and `X-Pulse-Signature` header
values; reject missing or duplicate signing headers. Do not re-serialize JSON,
trim the body, or trust a timestamp from the payload instead of the header.

This Python example accepts timestamps within five minutes of the receiver's
clock, rejects malformed headers, and compares the HMAC in constant time. Keep
both machines' clocks synchronised. Load the shared secret from private receiver
configuration, not from the request, a command argument or a log. Pulse trims
surrounding whitespace from its configured secret; the verifier does the same.
An empty secret must never authenticate a request.

```python
import hashlib
import hmac
import re
import time

MAX_SKEW_SECONDS = 300


def verify(secret: str, timestamp: str, body: bytes, signature: str) -> bool:
    secret = secret.strip()
    if not secret or not isinstance(body, bytes):
        return False
    if not isinstance(timestamp, str) or re.fullmatch(r"[0-9]{1,12}", timestamp) is None:
        return False
    if not isinstance(signature, str) or re.fullmatch(r"v1=[0-9a-f]{64}", signature) is None:
        return False
    if abs(time.time() - int(timestamp)) > MAX_SKEW_SECONDS:
        return False
    expected = "v1=" + hmac.new(
        secret.encode(), timestamp.encode() + b"." + body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature)
```

A time window rejects old captures, **not every replay**: the same signed
request can arrive more than once within that window, and Pulse signs each
retry with its current send time. After verification, deduplicate atomically
using the [receiver rules below](#receiver-correlation-and-deduplication),
not the signature, send timestamp or alert ID alone. An `alert` and its later
`resolved` are different events, not duplicates. The `X-Pulse-Event-ID` header is a useful
correlation hint, but is **not covered by this HMAC**: do not let a changed
header bypass deduplication of an otherwise identical signed event. The sample
PSA payload below includes each member's occurrence identity for this purpose. A valid HMAC
establishes integrity, not that processing succeeded; retain your receiver's
normal durable processing and retry handling. Never log the secret or full
credential-bearing request while diagnosing a rejection.

The secret is write-only through the API: list responses mask it, and an update that echoes the masked placeholder keeps the stored secret.

```http
POST /api/notifications/webhooks
Content-Type: application/json

{
  "name": "PSA Bridge",
  "url": "https://psa.example.com/inbound/pulse",
  "service": "generic",
  "enabled": true,
  "signingSecret": "<random 32+ byte secret>"
}
```

### Receiver correlation and deduplication

Keep ticket correlation separate from suppressing repeated processing:

- Scope durable receiver records to the **configured sender and destination**
  and tenant, not just a resource name or alert ID. Read identity from the
  authenticated payload's fields after verifying the request; retain it across
  receiver restarts. The tenant field is context, not permission to act in an
  arbitrary account.
- For normal queued firing and recovery notifications, use each member of
  `{{.Alerts}}`, not only the primary `{{.ID}}`. The [full PSA template below](#sample-psa-payloads)
  includes each member's `alertId` and full-precision UTC `startedAt`. These
  identify its observed occurrence; a later start with the same ID must be
  processed as a new occurrence. Firing and recovery use the same occurrence
  record, so an old delayed recovery must not close a newer incident.
- Within that record, handle `event` **and severity**. A warning becoming
  critical is an update, not a duplicate warning. Decide explicitly whether
  reminders update the existing ticket; do not create another ticket for each
  retry or discard a severity increase. Group membership can change without
  changing the header: process every member, even when the primary is unchanged.
- The primary `{{.StartTime}}` string has only whole-second precision. Member
  `StartTime` values in `{{.Alerts}}` are Go times and can be formatted with
  fractional seconds as below. Do not assume the shorter string is universally
  unique. A test or legacy payload can lack member identity; keep incomplete or
  ambiguous events for reconciliation rather than silently treating them as
  duplicates. **Send test** alone does not validate this lifecycle.
- Commit receiver state and the ticket action atomically, or use your ticket
  system's durable idempotent operation. Do not mark an event processed before
  its action succeeds. Exact signed-byte replays can be rejected separately,
  but a queue retry may render a new body or send timestamp: hashing those alone
  is not logical-event deduplication. There is no universal exactly-once key in
  this legacy header.

Validate in an authorised test environment with firing, retry, warning-to-critical,
recovery, recurrence of the same condition and a changed multi-alert group.
Check actual tickets and receiver restart behaviour before enabling automatic
actions. Do not cause a production outage or notification storm for this test.

## 🛡️ Security

- **Private IPs**: By default, webhooks to private IPs are blocked. Allow them in **Settings → System → Network → Webhook Security**.
- **Headers**: Add custom headers (e.g., `Authorization: Bearer ...`) in the webhook config.
- **Signing**: Prefer `signingSecret` (above) over bare bearer headers when the receiver supports verification — it authenticates the payload itself, not just the connection.

## 🧾 Audit Webhooks (Pro/legacy Pro+/Cloud)

Pro, legacy Pro+, and Cloud support dedicated audit webhooks for security event compliance. Unlike alert notifications, these webhooks deliver the raw, signed JSON payload of every security-relevant action (login, config change, group mapping).

### Setup
1. Go to **Settings → Security → Audit Webhooks**.
2. Add your endpoint URL (e.g., `https://siem.corp.local/ingest/pulse`).

### Security
Audit webhooks are dispatched asynchronously. The payload includes a `signature` field which can be verified using the per-instance HMAC key stored (encrypted) at `.audit-signing.key` in the Pulse data directory. There is no `PULSE_AUDIT_SIGNING_KEY` override.

## 🏢 Provider-hosted MSP webhooks

See [MSP.md](MSP.md) for the full provider operations guide (topology, ingress isolation, reports).

Provider-hosted MSP runs one isolated Pulse runtime per client workspace. That means alert routes and webhook destinations are configured inside the client runtime, not in one shared cross-client alert table. A webhook for Client A only sees Client A alerts because Client A has its own Pulse runtime, data, tokens, and notification config.

Built-in webhook templates include Gotify, PagerDuty, Slack, and Generic. Use the Generic webhook for systems that accept custom inbound payloads, including ConnectWise and similar PSA or ITSM tools. This is webhook routing, not a bespoke PSA integration.

Typical MSP setup:

1. Open the client workspace from Pulse Account.
2. Add that client's notification destinations in **Alerts → Notifications**.
3. Use Gotify, PagerDuty, Slack, or Generic depending on where the client or provider team wants alerts to land.
4. Keep each destination scoped to the client runtime so alert payloads and resolved events never cross into another client's workflow.

## 🏢 Multi-tenant organization integrations

In shared-process multi-tenant mode (self-hosted with `PULSE_MULTI_TENANT_ENABLED=true` and an Enterprise license with the `multi_tenant` capability) alerts and notification destinations are isolated **per organization**. Every alert and webhook request resolves an organization and operates only on that org's own alert state and webhook config.

Use this for one owner separating internal sites, departments, teams, or environments. It is not the canonical Pulse MSP model for separate customer businesses; MSP uses isolated client workspaces with their own runtime boundaries.

The organization for a request is resolved in this order:

1. `X-Pulse-Org-ID: <orgID>` header (the way API clients or internal middleware should target a specific organization).
2. `pulse_org_id` session cookie (browser sessions).
3. An org-bound API token (a token scoped to a single org needs no header).
4. Fallback: the `default` org.

Suspended or pending-deletion organizations return `403`, and an unknown org ID returns `400`.

### Wiring organization alerts into external systems

There are two integration models. The push model is usually the right fit when tickets or incidents should open and close automatically.

**Push (recommended): one outbound webhook per organization.** Create a
**Generic** webhook for each organization and point it at your external system's
inbound endpoint (an ITSM/PSA inbound webhook, an email connector, or middleware
that opens service tickets). Shape the JSON with a [custom template](#-custom-templates)
to match the receiving system's schema. The bridge can open or update a ticket
on `alert` and resolve that occurrence on `resolved`, subject to the [delivery
conditions above](#-delivery-contract). Add authentication as a custom header
in the settings form; keep its value private.

Configure it from the UI (**Alerts → Notifications → Add Webhook**) per org, or programmatically with an org-bound admin token:

```http
POST /api/notifications/webhooks
X-Pulse-Org-ID: acme-corp
Authorization: Bearer <token with settings:write>
Content-Type: application/json

{
  "name": "ConnectWise (Acme)",
  "url": "https://psa.example.com/inbound/pulse",
  "method": "POST",
  "service": "generic",
  "enabled": true,
  "headers": { "Authorization": "Bearer <psa-token>" },
  "template": "{\"summary\":\"{{.Level | jsonString}}: {{.ResourceName | jsonString}} {{.Message | jsonString}}\",\"event\":\"{{.Event | jsonString}}\",\"alertId\":\"{{.ID | jsonString}}\",\"startedAt\":\"{{.StartTime | jsonString}}\"}"
}
```

The exact ticket fields differ by platform (ConnectWise, Autotask, Halo, and
others each expect their own inbound shape), so map the template to your
platform's contract. This short example carries only the primary alert and a
whole-second start. For a bridge handling real grouped notifications, use the
member-aware template below; do not use the short example as an exactly-once key.

### Sample PSA payloads

A fuller template for normal queued firing and recovery notifications includes
tenant context and every member's occurrence, severity and condition. The
primary fields remain convenient summary context, **not the whole batch**:

```json
{
  "event": "{{.Event | jsonString}}",
  "alertId": "{{.ID | jsonString}}",
  "messageKey": "{{.MessageKey | jsonString}}",
  "severity": "{{.Level | jsonString}}",
  "alertType": "{{.Type | jsonString}}",
  "tenantId": "{{.TenantID | jsonString}}",
  "tenantName": "{{.TenantName | jsonString}}",
  "resource": "{{.ResourceName | jsonString}}",
  "resourceType": "{{.ResourceType | jsonString}}",
  "node": "{{.Node | jsonString}}",
  "summary": "{{.Message | jsonString}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}},
  "startedAt": "{{.StartTime | jsonString}}",
  "duration": "{{.Duration | jsonString}}",
  "alertCount": {{.AlertCount}},
  "alerts": [{{$comma := ""}}{{range .Alerts}}{{if .}}{{$comma}}
    {
      "alertId": "{{.ID | jsonString}}",
      "startedAt": "{{.StartTime.UTC.Format "2006-01-02T15:04:05.999999999Z07:00" | jsonString}}",
      "severity": "{{.Level | jsonString}}",
      "alertType": "{{.Type | jsonString}}",
      "resourceId": "{{.ResourceID | jsonString}}",
      "resource": "{{.ResourceName | jsonString}}",
      "summary": "{{.Message | jsonString}}"
    }{{$comma = ","}}{{end}}{{end}}
  ]
}
```

An abridged primary summary for a **critical** alert (the template also emits
the `alerts` member array):

```json
{
  "event": "alert",
  "alertId": "f3a9c2d1",
  "severity": "critical",
  "alertType": "cpu",
  "tenantId": "client-acme",
  "tenantName": "Acme Corp",
  "resource": "web-01",
  "node": "pve1",
  "summary": "CPU usage 95.2% exceeds threshold 90%",
  "value": 95.2,
  "threshold": 90,
  "startedAt": "2026-06-10T14:03:00Z",
  "duration": "5m"
}
```

A **warning** alert is identical except `"severity": "warning"`. Map
informational levels too; a two-priority mapping does not cover every condition.

The **resolved** event retains the original member `alertId` and `startedAt`,
letting the bridge close that occurrence's ticket rather than a later incident
with the same ID. Its primary summary is:

```json
{
  "event": "resolved",
  "alertId": "f3a9c2d1",
  "severity": "critical",
  "alertType": "cpu",
  "tenantId": "client-acme",
  "tenantName": "Acme Corp",
  "resource": "web-01",
  "node": "pve1",
  "summary": "web-01 on pve1 is now healthy",
  "value": 95.2,
  "threshold": 90,
  "startedAt": "2026-06-10T14:03:00Z",
  "duration": "22m"
}
```

For ConnectWise specifically, point the webhook at a ConnectWise inbound API
callback (or middleware that calls the ConnectWise REST API). Map each member's
severity to ticket priority and the tenant to your configured company. Combine
[`signingSecret`](#-delivery-contract) with the [receiver correlation rules](#receiver-correlation-and-deduplication),
not header-only deduplication. Verify your platform's processing and recovery
behaviour before treating the bridge as production-ready.

**Pull (poll): org-scoped read API.** Issue a `monitoring:read` token bound to each organization and poll that org's alerts. Send `X-Pulse-Org-ID` (or rely on the org-bound token) so you get only that organization's data:

- `GET /api/alerts/active` — currently firing alerts for the org.
- `GET /api/alerts/history` — historical alerts for the org.

To acknowledge or clear from the PSA side, use a `monitoring:write` token: `POST /api/alerts/acknowledge` and `POST /api/alerts/clear`.

### Scope and targeting summary

| Action | Endpoint | Scope |
|--------|----------|-------|
| Create / update / delete per-org webhook | `POST` / `PUT` / `DELETE /api/notifications/webhooks` | `settings:write` (admin) |
| List per-org webhooks | `GET /api/notifications/webhooks` | `settings:read` (admin) |
| Read active / historical alerts | `GET /api/alerts/active`, `GET /api/alerts/history` | `monitoring:read` |
| Acknowledge / clear alerts | `POST /api/alerts/acknowledge`, `POST /api/alerts/clear` | `monitoring:write` |

Target an organization with the `X-Pulse-Org-ID: <orgID>` header or an org-bound API token. See [API.md](API.md) for the full endpoint and token reference.
