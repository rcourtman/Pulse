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

## 🛡️ Security

- **Private IPs**: By default, webhooks to private IPs are blocked. Allow them in **Settings → System → Network → Webhook Security**.
- **Headers**: Add custom headers (e.g., `Authorization: Bearer ...`) in the webhook config.

## 🧾 Audit Webhooks (Pro/legacy Pro+/Cloud)

Pro, legacy Pro+, and Cloud support dedicated audit webhooks for security event compliance. Unlike alert notifications, these webhooks deliver the raw, signed JSON payload of every security-relevant action (login, config change, group mapping).

### Setup
1. Go to **Settings → Security → Webhooks**.
2. Add your endpoint URL (e.g., `https://siem.corp.local/ingest/pulse`).

### Security
Audit webhooks are dispatched asynchronously. The payload includes a `signature` field which can be verified using the per-instance HMAC key stored (encrypted) at `.audit-signing.key` in the Pulse data directory. There is no `PULSE_AUDIT_SIGNING_KEY` override.

## 🏢 Provider-hosted MSP webhooks

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
