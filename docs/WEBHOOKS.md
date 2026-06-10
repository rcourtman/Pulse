# 🔔 Webhooks

Pulse includes built-in templates for popular services and a generic JSON template for custom endpoints.

## 🚀 Quick Setup

1. Go to **Alerts → Notification Destinations**.
2. Click **Add Webhook**.
3. Select service type and paste the URL.

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
- `{{.ID}}`, `{{.Level}}`, `{{.Type}}`
- `{{.ResourceName}}`, `{{.ResourceID}}`, `{{.ResourceType}}`, `{{.Node}}`
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

**Example Payload:**
```json
{
  "text": "Alert: {{.Level}} - {{.Message}}",
  "value": {{.Value}}
}
```

## 📦 Delivery Contract

These fields and behaviors are stable; ticket-routing integrations can rely on them.

**Events.** Every webhook fires on both `alert` and `resolved` events. `{{.Event}}` is `"alert"` or `"resolved"` — there is no separate "info" event class.

**Severity.** `{{.Level}}` is `"warning"` or `"critical"`. Pulse has exactly these two alert levels.

**Alert type.** `{{.Type}}` is the metric or condition that fired: `cpu`, `memory`, `disk`, `diskRead`, `diskWrite`, `networkIn`, `networkOut`, `connectivity`, and similar. The alert ID (`{{.ID}}`) is stable for the lifetime of an alert occurrence, so the `resolved` event carries the same ID as the `alert` event it closes.

**Tenant identity.** In multi-tenant organizations and MSP client runtimes, `{{.TenantID}}` and `{{.TenantName}}` identify which tenant fired the alert. Client runtimes get identity from the `PULSE_TENANT_ID` / `PULSE_TENANT_NAME` environment; shared-process organizations stamp the org ID and display name automatically.

**Retries.** Failed deliveries retry with exponential backoff. The persistent notification queue makes up to 3 delivery attempts per notification; webhooks configured with transport-level retry add up to 3 more HTTP retries per attempt (1s doubling to a 30s cap, honoring `Retry-After` on HTTP 429). A receiver may therefore see the same logical event more than once.

**Idempotency.** Every alert delivery carries an `X-Pulse-Event-ID` header of the form `<alertID>:<event>` (e.g. `a1b2c3:alert`, `a1b2c3:resolved`). It is identical across all retries of the same logical event — deduplicate on it.

**Signed deliveries.** Set a `signingSecret` on the webhook config to enable HMAC signing. Signed requests carry:

- `X-Pulse-Timestamp`: Unix seconds at send time.
- `X-Pulse-Signature`: `v1=` + hex HMAC-SHA256 over `timestamp + "." + body`, keyed with the shared secret.

To verify: recompute the HMAC over the received timestamp and raw body, compare with constant-time equality, and reject requests whose timestamp is outside your tolerance window (e.g. 5 minutes) to block replays.

```python
import hashlib, hmac

def verify(secret: str, timestamp: str, body: bytes, signature: str) -> bool:
    expected = "v1=" + hmac.new(secret.encode(), f"{timestamp}.".encode() + body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

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

## 🛡️ Security

- **Private IPs**: By default, webhooks to private IPs are blocked. Allow them in **Settings → System → Network → Webhook Security**.
- **Headers**: Add custom headers (e.g., `Authorization: Bearer ...`) in the webhook config.
- **Signing**: Prefer `signingSecret` (above) over bare bearer headers when the receiver supports verification — it authenticates the payload itself, not just the connection.

## 🧾 Audit Webhooks (Pro/legacy Pro+/Cloud)

Pro, legacy Pro+, and Cloud support dedicated audit webhooks for security event compliance. Unlike alert notifications, these webhooks deliver the raw, signed JSON payload of every security-relevant action (login, config change, group mapping).

### Setup
1. Go to **Settings → Security → Webhooks**.
2. Add your endpoint URL (e.g., `https://siem.corp.local/ingest/pulse`).

### Security
Audit webhooks are dispatched asynchronously. The payload includes a `signature` field which can be verified using the per-instance HMAC key stored (encrypted) at `.audit-signing.key` in the Pulse data directory. There is no `PULSE_AUDIT_SIGNING_KEY` override.

## 🏢 Provider-hosted MSP webhooks

See [MSP.md](MSP.md) for the full provider operations guide (topology, ingress isolation, reports).

Provider-hosted MSP runs one isolated Pulse runtime per client workspace. That means alert routes and webhook destinations are configured inside the client runtime, not in one shared cross-client alert table. A webhook for Client A only sees Client A alerts because Client A has its own Pulse runtime, data, tokens, and notification config.

Built-in webhook templates include Gotify, PagerDuty, Slack, and Generic. Use the Generic webhook for systems that accept custom inbound payloads, including ConnectWise and similar PSA or ITSM tools. This is webhook routing, not a bespoke PSA integration.

Typical MSP setup:

1. Open the client workspace from Pulse Account.
2. Add that client's notification destinations in **Alerts → Notification Destinations**.
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

**Push (recommended): one outbound webhook per organization.** Create a **Generic** webhook for each organization and point it at your external system's inbound endpoint (an ITSM/PSA inbound webhook, an email connector, or middleware that opens service tickets). Shape the JSON with a [custom template](#-custom-templates) so it matches the receiving system's expected schema: every template variable listed above is available. Pulse fires on both `alert` and `resolved` events (`{{.Event}}` is `"alert"` or `"resolved"`), so the receiving system can open a ticket on alert and auto-resolve it on recovery. Add authentication as a custom header (e.g. `Authorization: Bearer ...`).

Configure it from the UI (**Alerts → Notification Destinations → Add Webhook**) per org, or programmatically with an org-bound admin token:

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
  "template": "{\"summary\":\"{{.Level}}: {{.ResourceName}} {{.Message | jsonString}}\",\"event\":\"{{.Event}}\",\"alertId\":\"{{.ID}}\"}"
}
```

The exact ticket fields differ by platform (ConnectWise, Autotask, Halo, and others each expect their own inbound shape), so map the template to your platform's contract. The `alertId` round-trips through `{{.ID}}`, which lets the receiving system correlate the later `resolved` event to the ticket it opened.

### Sample PSA payloads

A fuller template suited to ticket routing, including tenant identity and the
stable severity/type fields from the [delivery contract](#-delivery-contract):

```json
{
  "event": "{{.Event}}",
  "alertId": "{{.ID | jsonString}}",
  "severity": "{{.Level | jsonString}}",
  "alertType": "{{.Type | jsonString}}",
  "tenantId": "{{.TenantID | jsonString}}",
  "tenantName": "{{.TenantName | jsonString}}",
  "resource": "{{.ResourceName | jsonString}}",
  "node": "{{.Node | jsonString}}",
  "summary": "{{.Message | jsonString}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}},
  "startedAt": "{{.StartTime | jsonString}}",
  "duration": "{{.Duration | jsonString}}"
}
```

What the receiver sees for a **critical** alert:

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

A **warning** alert is identical except `"severity": "warning"` — warning and critical are the only two severities Pulse emits, so a two-priority PSA mapping covers the full range.

The **resolved** event reuses the same `alertId`, letting the bridge close the ticket it opened:

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

For ConnectWise specifically, point the webhook at a ConnectWise inbound API callback (or middleware that calls the ConnectWise REST API) and map `severity` to ticket priority, `tenantName` to the company, and `alertId` to your correlation field. Combine with a [`signingSecret`](#-delivery-contract) and the `X-Pulse-Event-ID` dedup header for a production-grade bridge.

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
