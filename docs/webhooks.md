# Webhook Setup Guide

Webhooks allow Pulse to send instant notifications to your favorite services when alerts are triggered.

## Quick Start

1. Enable webhooks in Settings → Notifications
2. Get a webhook URL from your service (see below)
3. Paste the URL and click "Test Webhook"
4. Save your configuration

## Supported Services

### Discord
Pulse automatically formats beautiful embed messages for Discord.

**Setup:**
1. Open your Discord server
2. Go to Server Settings → Integrations → Webhooks
3. Click "New Webhook"
4. Choose a channel and name
5. Copy the Webhook URL
6. Paste into Pulse

**Example Discord URL:**
```
https://discord.com/api/webhooks/1234567890/abcdefghijklmnop
```

### Slack
Pulse sends formatted messages with color-coded alerts.

**Setup:**
1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Create a new app or select existing
3. Add "Incoming Webhooks" feature
4. Activate and add to workspace
5. Choose a channel
6. Copy the webhook URL

**Example Slack URL:**
```
https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX
```

### Home Assistant
Trigger automations based on Pulse alerts.

**Setup:**
1. Go to Settings → Automations
2. Create new automation
3. Add trigger type: "Webhook"
4. Copy the generated webhook ID
5. Your URL: `http://your-ha-instance:8123/api/webhook/YOUR_WEBHOOK_ID`

**Example automation:**
```yaml
trigger:
  - platform: webhook
    webhook_id: pulse-alerts
action:
  - service: notify.mobile_app
    data:
      title: "Pulse Alert"
      message: "{{ trigger.json.alert.rule.name }} on {{ trigger.json.alert.guest.name }}"
```

### Microsoft Teams
**Setup:**
1. Open Teams channel
2. Click ••• → Connectors
3. Search "Incoming Webhook"
4. Configure and name it
5. Copy the webhook URL

### Generic Webhooks
Any service that accepts JSON POST requests will work. Pulse sends:

```json
{
  "timestamp": "2024-01-20T10:30:00Z",
  "alert": {
    "id": "alert-123",
    "rule": {
      "name": "High CPU Usage",
      "description": "CPU usage exceeded 80%"
    },
    "guest": {
      "name": "web-server",
      "vmid": "100",
      "type": "qemu",
      "node": "pve1"
    },
    "value": "85",
    "threshold": "80"
  }
}
```

## Testing Your Webhook

1. Use the "Test Webhook" button to verify connectivity
2. Check that you receive the test message
3. If it fails, verify:
   - URL is correct and complete
   - No firewall blocking outbound connections
   - Service is online

## Alert Configuration

Remember to:
1. Enable webhooks globally in Settings
2. Enable webhooks for individual alert rules in the Alerts tab
3. Set appropriate cooldowns to avoid spam (default: 5 minutes)

## Troubleshooting

**Not receiving webhooks?**
- Check if webhooks are enabled globally
- Verify individual alerts have webhooks enabled
- Check cooldown settings (alerts won't repeat within cooldown period)
- Test with the "Test Webhook" button

**Discord webhooks failing?**
- Ensure URL includes the full path with token
- Check Discord server isn't blocking webhooks

**Slack webhooks failing?**
- Verify webhook app is still installed in workspace
- Check channel still exists

**Home Assistant not receiving?**
- Ensure HA is accessible from Pulse server
- Check webhook ID matches automation
- Verify no authentication blocking the webhook endpoint