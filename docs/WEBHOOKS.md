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

**Convenience fields:**
- `{{.ValueFormatted}}`, `{{.ThresholdFormatted}}`
- `{{.StartTime}}`, `{{.Acknowledged}}`, `{{.AckTime}}`, `{{.AckUser}}`

**Template helpers:** `title`, `upper`, `lower`, `printf`, `urlquery`/`urlencode`, `urlpath`

**Service-specific notes:**
- **Telegram**: include `chat_id` in the URL query string.
- **Telegram templates**: `{{.ChatID}}` is populated from the URL query string.
- **PagerDuty**: set `routing_key` as a custom field (or header) in the webhook config.
- **Pushover**: add `app_token` and `user_token` custom fields (required).

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

## 🧾 Audit Webhooks (Pro)

Pulse Pro supports dedicated audit webhooks for security event compliance. Unlike alert notifications, these webhooks deliver the raw, signed JSON payload of every security-relevant action (login, config change, group mapping).

### Setup
1. Go to **Settings → Security → Webhooks**.
2. Add your endpoint URL (e.g., `https://siem.corp.local/ingest/pulse`).

### Security
Audit webhooks are dispatched asynchronously. The payload includes a `signature` field which can be verified using your `PULSE_AUDIT_SIGNING_KEY` to ensure the event has not been tampered with in transit.
