# Pulse Notification System - Quick Reference

## File Structure

### Backend Core Files
```
internal/notifications/
├── notifications.go              (2,358 lines) - Main NotificationManager, email, webhook, apprise
├── webhook_enhanced.go           (605 lines)   - Enhanced webhook support with retry & filtering
├── email_enhanced.go             (415 lines)   - EnhancedEmailManager with SMTP provider support
├── email_providers.go            (217 lines)   - 11 email provider templates (Gmail, SendGrid, etc)
├── email_template.go             (200+ lines)  - HTML email templates
├── webhook_templates.go          (300+ lines)  - 8+ webhook service templates
└── queue.go                      (600+ lines)  - Persistent SQLite notification queue

internal/api/
├── notifications.go              (753 lines)   - HTTP handlers for notification endpoints
└── notification_queue.go         (80+ lines)   - Queue management endpoints
```

### Frontend Files
```
frontend-modern/src/
├── api/notifications.ts          (201 lines)   - API client for notifications
├── components/Alerts/
│   ├── WebhookConfig.tsx         (300+ lines)  - Webhook config UI
│   └── EmailProviderSelect.tsx   (200+ lines)  - Email provider selector
└── stores/notifications.ts       - Reactive state management
```

## Core Data Structures

### EmailConfig
```go
Enabled, Provider, SMTPHost, SMTPPort, Username, Password,
From, To (array), TLS, StartTLS
```

### WebhookConfig
```go
ID, Name, URL, Method, Headers (map), Enabled,
Service (discord/slack/teams/telegram/etc), Template, CustomFields (map)
```

### AppriseConfig
```go
Enabled, Mode (cli/http), Targets (array), CLIPath, TimeoutSeconds,
ServerURL, ConfigKey, APIKey, APIKeyHeader, SkipTLSVerify
```

## Key Methods

### NotificationManager
| Method | Purpose |
|--------|---------|
| `SendAlert(alert)` | Add alert to queue, start grouping timer |
| `CancelAlert(alertID)` | Remove resolved alert |
| `SetEmailConfig(config)` | Update & persist email config |
| `SetAppriseConfig(config)` | Update & persist apprise config |
| `AddWebhook/UpdateWebhook/DeleteWebhook(...)` | Webhook CRUD |
| `GetWebhooks/GetEmailConfig/GetAppriseConfig()` | Safe getters |
| `SendTestNotification(method)` | Test email/webhook/apprise |
| `ProcessQueuedNotification(notif)` | Queue processor callback |

### Alert Flow
```
SendAlert() → [Cooldown check] → [Add to pending] → [Start timer] →
[Timer expires] → sendGroupedAlerts() → [Email/Webhook/Apprise async] →
[Queue if available] → [Retry on failure]
```

## API Endpoints

### Configuration
- `GET /api/notifications/email` - Get email config (masked)
- `PUT /api/notifications/email` - Update email config
- `GET /api/notifications/webhooks` - List webhooks
- `POST /api/notifications/webhooks` - Create webhook
- `PUT /api/notifications/webhooks/{id}` - Update webhook
- `DELETE /api/notifications/webhooks/{id}` - Delete webhook
- `GET /api/notifications/apprise` - Get apprise config
- `PUT /api/notifications/apprise` - Update apprise config

### Templates & Providers
- `GET /api/notifications/email-providers` - Email provider list
- `GET /api/notifications/webhook-templates` - Webhook templates

### Testing & Monitoring
- `POST /api/notifications/test` - Test notification (email/webhook)
- `POST /api/notifications/webhooks/test` - Test specific webhook
- `GET /api/notifications/webhook-history` - Delivery history
- `GET /api/notifications/health` - Queue & health status
- `GET /api/notifications/dlq` - Dead letter queue
- `GET /api/notifications/queue/stats` - Queue statistics
- `POST /api/notifications/dlq/retry` - Retry DLQ item
- `POST /api/notifications/dlq/delete` - Delete DLQ item

## Email Providers

1. Gmail / Google Workspace (STARTTLS 587, App Password)
2. SendGrid (STARTTLS 587, apikey username)
3. Mailgun (STARTTLS 587)
4. Amazon SES (STARTTLS 587)
5. Microsoft 365 (STARTTLS 587, App Password)
6. Brevo (STARTTLS 587)
7. Postmark (STARTTLS 587, token auth)
8. SparkPost (STARTTLS 587, SMTP_Injection)
9. Resend (STARTTLS 587)
10. SMTP2GO (STARTTLS 587)
11. Custom SMTP

## Webhook Services

1. **Discord** - Embeds with colors, fields
2. **Telegram** - Markdown, requires chat_id in URL
3. **Slack** - Blocks, section fields
4. **Microsoft Teams** - Adaptive Cards
5. **PagerDuty** - Event API with routing key
6. **Gotify** - Title + message + priority
7. **Pushover** - Custom app/user tokens
8. **ntfy.sh** - Plain text with headers

## Security Features

### SSRF Protection
- DNS resolution required
- Blocks: localhost, 127.*, private ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Blocks: link-local (169.254.*, fe80::*), cloud metadata (169.254.169.254)
- Max 3 redirects

### Rate Limiting
- Email: 60 per minute (configurable)
- Webhooks: 10 per minute per URL

### Retry Logic
- Email: Up to 3 attempts, 5s delay
- Webhooks: Exponential backoff 1s→2s→4s→8s→16s→30s (max)
- Queue: Configurable max attempts, DLQ for failures

## Constants

| Setting | Default | Notes |
|---------|---------|-------|
| Webhook timeout | 30s | 10s for tests |
| Webhook response limit | 1 MB | Prevents memory exhaustion |
| Email SMTP timeout | 10s dial, 30s total | Both TLS & STARTTLS |
| Cooldown | 5 minutes | Per-alert grace period |
| Grouping window | 30 seconds | Alert batching window |
| Email rate limit | 60/minute | Configurable |
| Webhook rate limit | 10/minute per URL | Configurable |
| Queue processor | 5 seconds | Retry interval |
| Queue cleanup | 1 hour | Remove old entries |

## Template Variables

For webhook/email templates, available fields:
```
{{.ID}} {{.Level}} {{.Type}} {{.ResourceName}} {{.ResourceID}}
{{.Node}} {{.Instance}} {{.Message}} {{.Value}} {{.Threshold}}
{{.ValueFormatted}} {{.ThresholdFormatted}} {{.StartTime}} {{.Duration}}
{{.Timestamp}} {{.ResourceType}} {{.Acknowledged}} {{.AckTime}}
{{.AckUser}} {{.CustomFields}} {{.AlertCount}} {{.Alerts}}
```

## Database Schema (Queue)

### notification_queue
- id (TEXT PRIMARY KEY)
- type (TEXT) - email, webhook, apprise
- status (TEXT) - pending, sending, sent, failed, dlq, cancelled
- alerts (TEXT JSON) - Array of alerts
- config (TEXT JSON) - EmailConfig/WebhookConfig/AppriseConfig
- attempts, max_attempts, last_attempt, last_error
- created_at, next_retry_at, completed_at
- Indexes: status, next_retry_at (pending), created_at

### notification_audit
- Audit trail for all notifications
- alert_ids, alert_count, success flag

## Important Notes

### Password Handling
- Passwords NEVER logged
- Passwords masked in API responses (get returns empty)
- Passwords preserved on update if empty field sent
- For testing, must include existing password

### Webhook URL Templating
- Go template syntax: {{.FieldName}}
- Custom functions: title, upper, lower, printf, urlquery, urlencode, urlpath
- Telegram requires chat_id in URL: `?chat_id={{.ChatID}}`
- Service-specific data enrichment (Telegram chat_id, PagerDuty routing_key)

### Email Templates
- Single alert: Level-specific colors, detailed metrics
- Grouped: Multiple alerts summary
- Both: Responsive HTML + plain text multipart

### Persistent Queue
- SQLite WAL mode for concurrency
- 64MB cache, 5s busy timeout
- Background processor: 5s intervals
- Cleanup job: Hourly
- Dead letter queue for permanently failed items
