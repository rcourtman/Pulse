# Webhook Testing & Troubleshooting Guide

## Overview

Pulse now includes comprehensive webhook testing capabilities to help diagnose and fix webhook notification issues.

## Quick Test

1. Open Settings (gear icon)
2. Navigate to the Webhook section
3. Click "Advanced Testing" button
4. Use the testing interface to verify your webhooks

## Testing Features

### 1. Get Test Endpoint
- Click "Get Test Endpoint" to create a free webhook.site endpoint
- Use this to test webhooks without setting up your own server
- The endpoint is valid for 7 days
- View all received webhooks at the provided URL

### 2. Test Types
- **Basic Test**: Tests simple connectivity
- **Alert Format**: Tests the actual alert notification format
- **All Alert Types**: Tests CPU, Memory, Disk, and Network alerts
- **Run All Tests**: Comprehensive testing suite

### 3. Webhook Type Detection
Pulse automatically detects and formats webhooks for:
- Discord webhooks
- Slack webhooks
- Generic JSON webhooks

## Common Issues & Solutions

### Webhooks Not Sending

1. **Check Global Settings**
   ```bash
   curl http://localhost:7655/api/webhook-status
   ```
   - Ensure `enabled: true`
   - Ensure `configured: true`

2. **Check Cooldowns**
   - Default cooldown: 60 minutes between alerts
   - Debounce delay: 3 minutes for new alerts
   - Check active cooldowns in the status endpoint

3. **Check Alert Rules**
   - Each alert rule must have webhooks enabled
   - Check the alert configuration in Settings > Alerts

### Discord Webhook Issues

Discord webhooks require specific formatting:
- Embeds array with proper structure
- Color must be a number (not hex string)
- Timestamp must be ISO-8601 format

### Slack Webhook Issues

Slack webhooks require:
- `text` field for the main message
- `attachments` array for detailed information
- Unix timestamp (not ISO-8601)

## API Endpoints

### Test Endpoints
- `GET /api/webhook-test/endpoint` - Get a test webhook.site endpoint
- `POST /api/webhook-test/run` - Run webhook tests
- `POST /api/webhook-test/verify` - Verify webhook delivery
- `GET /api/webhook-test/history` - Get test history

### Status Endpoint
- `GET /api/webhook-status` - Get current webhook configuration and status

## Debugging Steps

1. **Enable Debug Logging**
   - Webhook logs now include detailed debugging information
   - Check logs with: `journalctl -u pulse -f | grep -i webhook`

2. **Test with webhook.site**
   - Use the testing interface to get a webhook.site endpoint
   - Configure Pulse to use this endpoint
   - Trigger an alert and check if it appears on webhook.site

3. **Check Response Details**
   - The improved logging shows response status, headers, and body
   - Look for rate limiting or authentication errors

4. **Verify Payload Format**
   - Use the testing interface to see exact payload formats
   - Compare with your webhook service's documentation

## Configuration Tips

### Cooldown Settings
Edit cooldown configuration to reduce delays:
```javascript
// In your alert rules or configuration
webhookCooldownConfig: {
    defaultCooldownMinutes: 30,  // Reduced from 60
    debounceDelayMinutes: 1,     // Reduced from 3
    maxCallsPerHour: 20          // Increased from 10
}
```

### Per-Guest Cooldowns
Configure different cooldown settings per VM/container for critical systems.

## Troubleshooting Checklist

- [ ] Global webhook setting enabled (`GLOBAL_WEBHOOK_ENABLED=true`)
- [ ] Webhook URL configured (`WEBHOOK_URL` environment variable)
- [ ] Individual alert rules have webhooks enabled
- [ ] No active cooldowns blocking notifications
- [ ] Webhook URL is accessible from the server
- [ ] Correct payload format for webhook type
- [ ] No rate limiting from webhook provider
- [ ] Alert thresholds are actually being exceeded

## Need Help?

1. Run the comprehensive test suite
2. Check `/api/webhook-status` for configuration
3. Review logs with webhook grep filter
4. Test with webhook.site to isolate issues
5. Report issues at https://github.com/anthropics/claude-code/issues