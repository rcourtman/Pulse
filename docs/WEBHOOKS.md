# 🔔 Webhooks

Pulse supports Discord, Slack, Teams, Telegram, Gotify, ntfy, and generic webhooks.

## 🚀 Quick Setup

1. Go to **Alerts → Notifications**.
2. Click **Add Webhook**.
3. Select service type and paste the URL.

## 📝 Service URLs

| Service | URL Format |
|---------|------------|
| **Discord** | `https://discord.com/api/webhooks/{id}/{token}` |
| **Slack** | `https://hooks.slack.com/services/...` |
| **Teams** | `https://{tenant}.webhook.office.com/...` |
| **Telegram** | `https://api.telegram.org/bot{token}/sendMessage?chat_id={id}` |
| **Gotify** | `https://gotify.example.com/message?token={token}` |
| **ntfy** | `https://ntfy.sh/{topic}` |

## 🎨 Custom Templates

For generic webhooks, use Go templates to format the JSON payload.

**Variables:**
- `{{.Message}}`: Alert text
- `{{.Level}}`: warning/critical
- `{{.Node}}`: Node name
- `{{.Value}}`: Metric value (e.g. 95.5)

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
