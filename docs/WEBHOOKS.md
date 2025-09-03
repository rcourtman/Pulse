# Webhook Configuration Guide

Pulse supports sending alert notifications to various webhook services including Discord, Slack, Microsoft Teams, Telegram, Gotify, ntfy, PagerDuty, and any custom webhook endpoint.

## Quick Start

1. Navigate to **Alerts** → **Notifications** tab
2. Configure email settings or add webhooks
3. Select your service type (Discord, Slack, Teams, Telegram, etc.)
4. Enter the webhook URL and configure settings
5. Test the webhook to ensure it's working
6. Save your configuration

![Alert Configuration](images/04-alerts.png)
*Alert configuration interface showing notification settings*

## Supported Services

### Discord
```
URL Format: https://discord.com/api/webhooks/{webhook_id}/{webhook_token}
```
1. In Discord, go to Server Settings → Integrations → Webhooks
2. Create a new webhook and copy the URL
3. Paste the URL in Pulse

### Telegram
```
URL Format: https://api.telegram.org/bot{bot_token}/sendMessage?chat_id={chat_id}
```
1. Create a bot with @BotFather on Telegram
2. Get your bot token from BotFather
3. Get your chat ID by messaging the bot and visiting: `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`
4. In Pulse, select "Telegram Bot" as the service type
5. Use the URL format: `https://api.telegram.org/bot<BOT_TOKEN>/sendMessage?chat_id=<CHAT_ID>`
6. **IMPORTANT**: The chat_id MUST be included in the URL as a parameter
7. Pulse automatically sends rich formatted messages with emojis and full alert details

### Slack
```
URL Format: https://hooks.slack.com/services/{webhook_path}
```
1. In Slack, go to Apps → Incoming Webhooks
2. Add to Slack and choose a channel
3. Copy the webhook URL

### Microsoft Teams
```
URL Format: https://{tenant}.webhook.office.com/webhookb2/{webhook_path}
```
1. In Teams channel, click ... → Connectors
2. Configure Incoming Webhook
3. Copy the URL

### Gotify
```
URL Format: https://your-gotify-server/message?token={your-app-token}
```
1. In Gotify, create a new application
2. Copy the application token
3. Use the URL format: `https://your-gotify-server/message?token=YOUR_APP_TOKEN`
4. The token MUST be included as a URL parameter
5. Pulse will send rich markdown-formatted notifications with emojis and full alert details

### ntfy
```
URL Format: https://ntfy.sh/{topic} or https://your-ntfy-server/{topic}
```
1. Choose a unique topic name (e.g., 'pulse-alerts-x7k9m2')
   - **Important**: Anyone who knows your topic name can send you notifications
   - Use a unique/random suffix for privacy
2. For ntfy.sh: Use `https://ntfy.sh/YOUR_TOPIC`
3. For self-hosted: Use `https://your-ntfy-server/YOUR_TOPIC`
4. Subscribe to the same topic in your ntfy mobile/desktop app
5. For authentication (optional):
   - Click "Custom Headers" section in webhook config
   - Add header: `Authorization`
   - Value: `Bearer YOUR_TOKEN` or `Basic base64_encoded_credentials`
6. Notifications include dynamic priority levels and emoji tags based on alert severity

### PagerDuty
```
URL: https://events.pagerduty.com/v2/enqueue
```
1. In PagerDuty, go to Configuration → Services
2. Add an integration → Events API V2
3. Copy the Integration Key
4. Add the key as a header: `routing_key: YOUR_KEY`

## Custom Headers

For webhooks that require authentication or custom headers:

1. In the webhook configuration, expand the **Custom Headers** section
2. Click **+ Add Header** to add a new header
3. Enter the header name (e.g., `Authorization`, `X-API-Key`, `X-Auth-Token`)
4. Enter the header value (e.g., `Bearer YOUR_TOKEN`, your API key, etc.)
5. Add multiple headers as needed
6. Headers are sent with every webhook request

### Common Header Examples

| Service | Header Name | Header Value Format |
|---------|-------------|-------------------|
| Bearer Token | `Authorization` | `Bearer YOUR_TOKEN_HERE` |
| Basic Auth | `Authorization` | `Basic base64_encoded_user:pass` |
| API Key | `X-API-Key` | `your-api-key-here` |
| Custom Token | `X-Auth-Token` | `your-auth-token` |
| ntfy Auth | `Authorization` | `Bearer tk_your_ntfy_token` |
| Custom Service | `X-Service-Key` | `service-specific-key` |

## Custom Payload Templates

For generic webhooks, you can define custom JSON payloads using Go template syntax.

### Available Variables

| Variable | Description | Example Value |
|----------|-------------|---------------|
| `{{.ID}}` | Alert ID | "alert-123" |
| `{{.Level}}` | Alert level | "warning", "critical" |
| `{{.Type}}` | Resource type | "cpu", "memory", "disk" |
| `{{.ResourceName}}` | Name of the resource | "Web Server VM" |
| `{{.ResourceID}}` | Resource identifier | "vm-100" |
| `{{.Node}}` | Proxmox node name | "pve-node-01" |
| `{{.Instance}}` | Proxmox instance URL | "https://192.168.1.100:8006" |
| `{{.Message}}` | Alert message | "CPU usage exceeded 90%" |
| `{{.Value}}` | Current metric value | 95.5 |
| `{{.Threshold}}` | Alert threshold | 90.0 |
| `{{.Duration}}` | How long alert has been active | "5m" |
| `{{.Timestamp}}` | Current timestamp | "2024-01-15T10:30:00Z" |
| `{{.StartTime}}` | When alert started | "2024-01-15T10:25:00Z" |

### Template Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{.Level \| title}}` | Capitalize first letter | "Warning" |
| `{{.Level \| upper}}` | Uppercase | "WARNING" |
| `{{.Level \| lower}}` | Lowercase | "warning" |
| `{{printf "%.1f" .Value}}` | Format numbers | "95.5" |

### Example Templates

#### Simple JSON
```json
{
  "text": "Alert: {{.Level}} - {{.Message}}",
  "resource": "{{.ResourceName}}",
  "value": {{.Value}},
  "threshold": {{.Threshold}}
}
```

#### Formatted Alert
```json
{
  "alert": {
    "level": "{{.Level | upper}}",
    "message": "{{.Message}}",
    "details": {
      "resource": "{{.ResourceName}}",
      "node": "{{.Node}}",
      "current_value": "{{printf "%.1f" .Value}}%",
      "threshold": "{{printf "%.0f" .Threshold}}%",
      "duration": "{{.Duration}}"
    }
  },
  "timestamp": "{{.Timestamp}}"
}
```

#### Slack-Compatible Custom Format
```json
{
  "text": "Pulse Alert",
  "attachments": [{
    "color": "{{if eq .Level "critical"}}danger{{else}}warning{{end}}",
    "title": "{{.Level | title}} Alert: {{.ResourceName}}",
    "text": "{{.Message}}",
    "fields": [
      {"title": "Value", "value": "{{printf "%.1f" .Value}}%", "short": true},
      {"title": "Threshold", "value": "{{printf "%.0f" .Threshold}}%", "short": true},
      {"title": "Node", "value": "{{.Node}}", "short": true},
      {"title": "Duration", "value": "{{.Duration}}", "short": true}
    ],
    "footer": "Pulse Monitoring",
    "ts": {{.Timestamp}}
  }]
}
```

#### Home Assistant
```json
{
  "title": "Pulse Alert: {{.Level | title}}",
  "message": "{{.Message}}",
  "data": {
    "entity_id": "sensor.{{.Node | lower}}_{{.Type}}",
    "state": {{.Value}},
    "attributes": {
      "resource": "{{.ResourceName}}",
      "threshold": {{.Threshold}},
      "duration": "{{.Duration}}"
    }
  }
}
```

#### n8n / Node-RED
```json
{
  "workflow": "pulse_alert",
  "data": {
    "alert_id": "{{.ID}}",
    "level": "{{.Level}}",
    "resource": "{{.ResourceName}}",
    "node": "{{.Node}}",
    "metric": {
      "type": "{{.Type}}",
      "value": {{.Value}},
      "threshold": {{.Threshold}}
    },
    "message": "{{.Message}}",
    "timestamp": "{{.Timestamp}}"
  }
}
```

## Testing Webhooks

1. After configuring a webhook, click the **Test** button
2. Pulse will send a test alert to verify the webhook is working
3. Check the receiving service to confirm the message arrived
4. If the test fails, verify:
   - The URL is correct and accessible
   - Any required authentication tokens are included
   - The payload format matches what the service expects

## Troubleshooting

### Webhook Returns 400 Bad Request
- Check if the payload format is correct for your service
- For Telegram, ensure chat_id is in the URL (Pulse handles it automatically)
- Verify all required fields are present in custom templates

### Webhook Returns 401/403
- Check authentication tokens/keys
- Verify the webhook URL hasn't expired
- Ensure IP restrictions allow Pulse server

### No Notifications Received
- Verify the webhook is enabled
- Check alert thresholds are configured correctly
- Ensure notification cooldown period has passed
- Test the webhook manually using the Test button

## API Reference

### Create Webhook
```bash
POST /api/notifications/webhooks
Content-Type: application/json

{
  "name": "My Webhook",
  "url": "https://example.com/webhook",
  "method": "POST",
  "service": "generic",
  "enabled": true,
  "template": "{\"alert\": \"{{.Message}}\"}"
}
```

### Test Webhook
```bash
POST /api/notifications/webhooks/test
Content-Type: application/json

{
  "name": "Test",
  "url": "https://example.com/webhook",
  "service": "generic",
  "template": "{\"test\": true}"
}
```

### Update Webhook
```bash
PUT /api/notifications/webhooks/{id}
Content-Type: application/json

{
  "name": "Updated Webhook",
  "url": "https://example.com/new-webhook",
  "enabled": false
}
```

### Delete Webhook
```bash
DELETE /api/notifications/webhooks/{id}
```

### List Webhooks
```bash
GET /api/notifications/webhooks
```

## Security Considerations

- **Never expose webhook URLs publicly** - they often contain authentication tokens
- **Use HTTPS URLs** when possible to encrypt data in transit
- **Rotate webhook URLs periodically** if they contain embedded tokens
- **Test webhooks carefully** to avoid sending test data to production channels
- **Limit webhook permissions** in the receiving service where possible
