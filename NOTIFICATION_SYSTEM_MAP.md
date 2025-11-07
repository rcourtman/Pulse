# Pulse Notification System Architecture - Complete Map

## Overview
Pulse has a sophisticated multi-channel notification system supporting Email, Webhooks, and Apprise. It features persistent queuing, retry logic, rate limiting, and security controls.

---

## BACKEND IMPLEMENTATION

### 1. Core Notification Manager
**File**: `/opt/pulse/internal/notifications/notifications.go` (2,358 lines)

#### Key Structures:
- **NotificationManager** (lines 107-126)
  - Orchestrates email, webhook, and Apprise notifications
  - Manages alert grouping with time-based windows
  - Implements cooldown and rate limiting
  - Maintains webhook delivery history
  - Integrates with persistent notification queue

- **EmailConfig** (lines 266-278)
  ```go
  type EmailConfig struct {
    Enabled  bool
    Provider string  // Gmail, SendGrid, etc.
    SMTPHost string  // "server" in JSON
    SMTPPort int     // "port" in JSON
    Username string
    Password string
    From     string
    To       []string
    TLS      bool
    StartTLS bool   // STARTTLS support
  }
  ```

- **WebhookConfig** (lines 280-291)
  ```go
  type WebhookConfig struct {
    ID           string
    Name         string
    URL          string
    Method       string
    Headers      map[string]string
    Enabled      bool
    Service      string  // discord, slack, teams, etc.
    Template     string  // Custom payload template
    CustomFields map[string]string
  }
  ```

- **AppriseConfig** (lines 301-313)
  ```go
  type AppriseConfig struct {
    Enabled        bool
    Mode           AppriseMode  // "cli" or "http"
    Targets        []string
    CLIPath        string
    TimeoutSeconds int
    ServerURL      string
    ConfigKey      string
    APIKey         string
    APIKeyHeader   string
    SkipTLSVerify  bool
  }
  ```

#### Key Methods:

**Initialization & Configuration**:
- `NewNotificationManager(publicURL string)` (lines 315-361)
  - Creates persistent queue if available
  - Initializes alert grouping system
  - Wires up queue processor
  
- `SetEmailConfig(config)` (lines 388-406)
  - Creates new email manager with provider config
  - Updates configuration in-memory

- `SetAppriseConfig(config)` (lines 408-413)
  - Normalizes Apprise configuration
  - Validates mode and timeout settings

- `GetEmailConfig() / GetWebhooks() / GetAppriseConfig()` (lines 502-420)
  - Safe accessor methods with locking

**Webhook Management**:
- `AddWebhook(webhook)` (lines 453-458)
- `UpdateWebhook(id, webhook)` (lines 460-472)
- `DeleteWebhook(id)` (lines 474-486)
- `GetWebhooks()` (lines 488-500)

**Alert Sending**:
- `SendAlert(alert)` (lines 516-569)
  - Implements cooldown check
  - Adds alert to pending alerts queue
  - Starts alert grouping timer
  - Checks for rate limits

- `CancelAlert(alertID)` (lines 571-614)
  - Removes resolved alert from pending list
  - Stops grouping timer if no more pending alerts

- `sendGroupedAlerts()` (lines 616-657)
  - Sends all pending alerts as single batch
  - Updates last notified timestamps
  - Enqueues to persistent queue OR sends directly

**Email Sending**:
- `sendGroupedEmail(config, alertList)` (lines 753-764)
  - Uses email template
  - Calls enhanced email manager with retry

- `sendHTMLEmailWithError(subject, body, config)` (lines 1010-1076)
  - Handles HTML + text multipart emails
  - Returns error for caller
  - Uses shared email manager for rate limiting

- `sendHTMLEmail(subject, body, config)` (lines 1078-1133)
  - Creates EnhancedEmailManager instance
  - Sends with retries (2 attempts, 3 second delay)

**Webhook Sending**:
- `sendGroupedWebhook(webhook, alertList)` (lines 1141-1354)
  - Handles single/grouped alert scenarios
  - Applies service-specific templates (Discord, Slack, Teams, etc.)
  - Supports custom templates
  - Service-specific data enrichment (Telegram chat_id, PagerDuty routing key)

- `sendWebhookRequest(webhook, jsonData, alertType)` (lines 1396-1533)
  - Performs rate limit check
  - Creates HTTP request with headers
  - Sends using secure webhook client
  - Tracks delivery history

- `prepareWebhookData(alert, customFields)` (lines 1708-1771)
  - Builds WebhookPayloadData struct
  - Formats metrics and duration
  - Includes metadata and custom fields

**Apprise Integration**:
- `sendGroupedApprise(config, alertList)` (lines 766-803)
  - Routes to CLI or HTTP mode
  - Validates configuration before sending

- `sendAppriseViaCLI(config, title, body)` (lines 866-902)
  - Executes apprise CLI with timeout
  - Passes targets as arguments

- `sendAppriseViaHTTP(config, title, body, type)` (lines 904-992)
  - Makes HTTP POST to Apprise server
  - Supports custom endpoints with configKey
  - Handles API key header
  - Optional TLS verification skip

**Webhook Validation & Security**:
- `ValidateWebhookURL(webhookURL)` (lines 1921-1988)
  - Prevents SSRF attacks
  - Blocks localhost/loopback (127.0.0.1, ::1)
  - Blocks link-local addresses (169.254.*, fe80::)
  - Blocks private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
  - Blocks cloud metadata services (169.254.169.254, metadata.google.internal)
  - DNS resolution required for security
  - Warns on numeric IPs with HTTPS

- `isPrivateIP(ip)` (lines 1990-2014)
  - Checks IPv4 and IPv6 private ranges

**Webhook URL Templating**:
- `renderWebhookURL(urlTemplate, data)` (lines 1829-1865)
  - Parses Go template syntax
  - Validates rendered URL is valid
  - Supports {{.}} field references

**Template Support**:
- `generatePayloadFromTemplate(templateStr, data)` (lines 1793-1796)
- `generatePayloadFromTemplateWithService(templateStr, data, service)` (lines 1798-1827)
  - Parses and executes Go text templates
  - Custom function map: title, upper, lower, printf, urlquery, urlencode, urlpath, pathescape
  - JSON validation (except ntfy service)

**Telegram-specific**:
- `extractTelegramChatID(webhookURL)` (lines 1882-1909)
  - Extracts and validates chat_id parameter
  - Handles negative IDs (group chats)
  - Validates numeric format

**Test Notifications**:
- `SendTestNotification(method)` (lines 2117-2195)
  - Creates test alert with predefined values
  - Routes to email/webhook/apprise

- `SendTestWebhook(webhook)` (lines 2197-2229)
  - Tests specific webhook with sample alert

- `SendTestNotificationWithConfig(method, config, nodeInfo)` (lines 2231-2300)
  - Tests email with provided config (without saving)
  - Uses actual node info from monitor state

**Queue Integration**:
- `enqueueNotifications(emailConfig, webhooks, appriseConfig, alerts)` (lines 659-722)
  - Marshals configs to JSON
  - Creates QueuedNotification structs
  - Adds to persistent queue

- `ProcessQueuedNotification(notif)` (lines 2302-2335)
  - Unmarshals config based on type
  - Routes to appropriate sender
  - Called by queue processor

**Utility Functions**:
- `checkWebhookRateLimit(webhookURL)` (lines 1356-1394)
  - Per-webhook rate limiting (10 per minute)
  - Tracks sent count per window

- `formatWebhookDuration(duration)` (lines 1867-1880)
  - Human-readable duration: "5s", "2m", "1h 30m", "2d 3h"

- `NormalizeAppriseConfig(config)` (lines 176-240)
  - Cleans whitespace
  - Validates mode and server URL
  - Removes duplicate targets
  - Enforces timeout bounds (5-120 seconds)

---

### 2. Enhanced Webhook Support
**File**: `/opt/pulse/internal/notifications/webhook_enhanced.go` (605 lines)

#### Key Structures:
- **EnhancedWebhookConfig** (lines 17-27)
  ```go
  type EnhancedWebhookConfig struct {
    WebhookConfig
    Service         string
    PayloadTemplate string
    RetryEnabled    bool
    RetryCount      int
    FilterRules     WebhookFilterRules
    CustomFields    map[string]interface{}
    ResponseLogging bool
  }
  ```

- **WebhookFilterRules** (lines 29-35)
  - Filter by alert level, type, node, resource type

- **WebhookPayloadData** (lines 37-66)
  - Complete template data structure
  - Includes alerts array for grouped notifications
  - ChatID for Telegram webhooks

#### Key Methods:
- `SendEnhancedWebhook(webhook, alert)` (lines 68-124)
- `shouldSendWebhook(webhook, alert)` (lines 129-194)
- `sendWebhookWithRetry(webhook, payload)` (lines 196-326)
  - Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s (max)
  - Respects Retry-After header for 429 responses
  - Max 3 retries by default
  - Logs attempt counts
  
- `isRetryableWebhookError(err)` (lines 328-365)
  - Network errors: timeout, connection refused/reset, no such host
  - Retryable HTTP: 429, 502, 503, 504, 5xx
  - Non-retryable: 4xx client errors

- `sendWebhookOnceWithResponse(webhook, payload)` (lines 367-428)
  - Returns response for inspection
  - Size-limited response reading

- `sendWebhookOnce(webhook, payload)` (lines 430-434)
  - Wrapper for compatibility

- `TestEnhancedWebhook(webhook)` (lines 438-604)
  - Tests webhook with realistic sample alert
  - Handles Telegram chat_id extraction
  - PagerDuty routing key injection
  - ntfy service special header handling
  - Returns (statusCode, responseBody, error)

---

### 3. Email Implementation
**File**: `/opt/pulse/internal/notifications/email_enhanced.go` (415 lines)

#### Key Structures:
- **EnhancedEmailManager** (lines 15-19)
  ```go
  type EnhancedEmailManager struct {
    config    EmailProviderConfig
    rateLimit *RateLimiter
  }
  ```

- **RateLimiter** (lines 21-26)
  - Simple per-minute rate limiting

#### Key Methods:
- `NewEnhancedEmailManager(config)` (lines 28-36)
- `SendEmailWithRetry(subject, htmlBody, textBody)` (lines 38-78)
  - Up to 3 retry attempts (configurable)
  - 5 second delay between retries (configurable)
  - Checks rate limit before each attempt

- `checkRateLimit()` (lines 80-99)
  - Per-minute limiter
  - Default 60 emails/minute

- `sendEmailOnce(subject, htmlBody, textBody)` (lines 101-138)
  - Builds multipart MIME message
  - Sets standard headers (Date, Message-ID, MIME-Version)
  - Creates text + HTML parts
  - Calls provider-specific send

- `sendViaProvider(msg)` (lines 140-183)
  - Provider-specific username/password handling
  - SendGrid: "apikey" as username
  - Postmark: API token for both
  - SparkPost: "SMTP_Injection" username
  - Resend: "resend" username

- `sendTLS(addr, auth, msg)` (lines 185-245)
  - TLS from start (port 465)
  - Uses tls.DialWithDialer with timeout
  - Sets connection deadline (30s)

- `sendStartTLS(addr, auth, msg)` (lines 247-309)
  - Plain TCP then STARTTLS upgrade
  - 10s dial timeout, 30s connection deadline

- `sendPlain(addr, auth, msg)` (lines 362-414)
  - Plain SMTP without encryption
  - 10s dial timeout, 30s connection deadline

- `TestConnection()` (lines 311-360)
  - Tests SMTP connectivity
  - Supports TLS and STARTTLS modes
  - Tests authentication if configured

---

### 4. Email Configuration & Templates
**Files**:
- `/opt/pulse/internal/notifications/email_providers.go` (217 lines)
- `/opt/pulse/internal/notifications/email_template.go` (200+ lines)

#### Email Providers (email_providers.go):
- **GetEmailProviders()** returns templates for:
  1. Gmail / Google Workspace (STARTTLS, port 587)
  2. SendGrid (STARTTLS, port 587)
  3. Mailgun (STARTTLS, port 587)
  4. Amazon SES (STARTTLS, port 587)
  5. Microsoft 365 / Outlook (STARTTLS, port 587, App Password required)
  6. Brevo / Sendinblue (STARTTLS, port 587)
  7. Postmark (STARTTLS, port 587)
  8. SparkPost (STARTTLS, port 587)
  9. Resend (STARTTLS, port 587)
  10. SMTP2GO (STARTTLS, port 587)
  11. Custom SMTP Server

- Each provider includes:
  - SMTP host and port
  - TLS/STARTTLS settings
  - Authentication requirements
  - Setup instructions with links

#### Email Templates (email_template.go):
- **EmailTemplate(alertList, isSingle)** (lines 11-17)
  - Routes to single or grouped template

- **singleAlertTemplate(alert)** (lines 19-40+)
  - Professional HTML email with:
    - Responsive design
    - Level-specific colors (red for critical, yellow for warning)
    - Alert metrics display
    - Details section with duration, node, type
    - Links to Pulse dashboard
    - Footer with Pulse logo and branding

- **groupedAlertTemplate(alertList)** (lines 100+)
  - Multiple alert summary
  - All alerts listed with values
  - Single cohesive email

---

### 5. Webhook Templates
**File**: `/opt/pulse/internal/notifications/webhook_templates.go` (300+ lines)

#### GetWebhookTemplates() provides for:
1. **Discord Webhook**
   - Uses embed format with color coding
   - Fields: Resource, Node, Type, Value, Threshold, Duration
   - Includes timestamp and footer

2. **Telegram Bot**
   - Markdown formatted message
   - Inline link to Pulse dashboard
   - Requires chat_id parameter

3. **Slack Incoming Webhook**
   - Header block with alert level
   - Section with message
   - 6-field layout (Resource, Node, Type, Value, Threshold, Duration)
   - Links to Proxmox and Pulse

4. **Microsoft Teams**
   - Adaptive Card format
   - Facts section with details
   - Color-coded by alert level

5. **PagerDuty**
   - Event API format
   - Routing key from headers

6. **Gotify**
   - Title + message format
   - Priority based on level

7. **Pushover**
   - Custom field aliases (app_token, user_token)
   - Priority encoding

8. **ntfy.sh**
   - Plain text content
   - Title and Priority headers
   - Tags header with emoji/type

Each template includes:
- Go template syntax for field substitution
- Service-specific format requirements
- Instructions for setup

---

### 6. Persistent Notification Queue
**File**: `/opt/pulse/internal/notifications/queue.go` (600+ lines)

#### Key Structures:
- **QueuedNotification** (lines 29-45)
  ```go
  type QueuedNotification struct {
    ID          string
    Type        string  // email, webhook, apprise
    Method      string
    Status      NotificationQueueStatus  // pending, sending, sent, failed, dlq, cancelled
    Alerts      []*alerts.Alert
    Config      json.RawMessage
    Attempts    int
    MaxAttempts int
    LastAttempt *time.Time
    LastError   string
    CreatedAt   time.Time
    NextRetryAt *time.Time
    CompletedAt *time.Time
    PayloadBytes int
  }
  ```

- **NotificationQueue** (lines 47-58)
  - SQLite-backed persistent queue
  - Background processors:
    - `processQueue()`: Periodic retry processor (5s interval)
    - `cleanupOldEntries()`: Cleanup job (1 hour interval)
  - Notification channel for signaling new items

#### Database Schema (lines 115-150):
- **notification_queue** table
  - Primary key: id (TEXT)
  - Statuses: pending, sending, sent, failed, dlq, cancelled
  - Indexes: status, next_retry_at (for pending), created_at
  - WAL mode for concurrency
  - NORMAL synchronous for durability

- **notification_audit** table (lines 139-150)
  - Audit trail for all notifications
  - Alert IDs and count tracking
  - Success/failure metrics

#### Key Methods:
- `NewNotificationQueue(dataDir)` (lines 60-113)
  - Opens/creates SQLite database
  - Configures pragmas:
    - WAL mode for better concurrency
    - NORMAL synchronous for balance
    - 5s busy timeout
    - 64MB cache
  - Initializes schema
  - Starts background processors

- `Enqueue(notif)` (enqueues notifications)
- `GetQueueStats()` (returns queue statistics)
- `GetDLQ(limit)` (retrieves dead letter queue)
- `RetryDLQItem(id)` (retries failed notification)
- `DeleteDLQItem(id)` (removes from DLQ)
- `SetProcessor(func)` (sets notification processor)

---

## API LAYER

### 1. Notification Endpoints Handler
**File**: `/opt/pulse/internal/api/notifications.go` (753 lines)

#### NotificationHandlers Structure (lines 17-27):
- Reference to Monitor for accessing notification manager and config persistence

#### GET Endpoints:
| Path | Method | Handler | Auth Scope | Purpose |
|------|--------|---------|-----------|---------|
| `/api/notifications/email` | GET | GetEmailConfig | SettingsRead | Retrieve current email config (password masked) |
| `/api/notifications/webhooks` | GET | GetWebhooks | SettingsRead | List all webhooks (sensitive fields masked) |
| `/api/notifications/webhook-templates` | GET | GetWebhookTemplates | SettingsRead | Get available webhook templates |
| `/api/notifications/webhook-history` | GET | GetWebhookHistory | SettingsRead | Get recent webhook delivery history |
| `/api/notifications/email-providers` | GET | GetEmailProviders | SettingsRead | Get email provider templates |
| `/api/notifications/apprise` | GET | GetAppriseConfig | SettingsRead | Get Apprise configuration |
| `/api/notifications/health` | GET | GetNotificationHealth | SettingsRead | Get queue and channel health stats |

#### PUT/UPDATE Endpoints:
| Path | Method | Handler | Auth Scope | Purpose |
|------|--------|---------|-----------|---------|
| `/api/notifications/email` | PUT | UpdateEmailConfig | SettingsWrite | Update email config (preserves password) |
| `/api/notifications/apprise` | PUT | UpdateAppriseConfig | SettingsWrite | Update Apprise config |
| `/api/notifications/webhooks/{id}` | PUT | UpdateWebhook | SettingsWrite | Update specific webhook |

#### POST Endpoints:
| Path | Method | Handler | Auth Scope | Purpose |
|------|--------|---------|-----------|---------|
| `/api/notifications/webhooks` | POST | CreateWebhook | SettingsWrite | Create new webhook |
| `/api/notifications/webhooks/test` | POST | TestWebhook | SettingsWrite | Test webhook with sample alert |
| `/api/notifications/test` | POST | TestNotification | SettingsWrite | Test email/webhook/apprise |

#### DELETE Endpoints:
| Path | Method | Handler | Auth Scope | Purpose |
|------|--------|---------|-----------|---------|
| `/api/notifications/webhooks/{id}` | DELETE | DeleteWebhook | SettingsWrite | Delete webhook |

#### Key Handler Methods:

**GetEmailConfig (lines 34-43)**:
- Returns masked config (password redacted)
- JSON response

**UpdateEmailConfig (lines 45-89)**:
- Reads raw body
- Preserves existing password if new one empty
- Updates in-memory config
- Persists to storage via ConfigPersistence

**GetWebhooks (lines 142-186)**:
- Masks header and customField values
- Shows only keys, not secrets
- Preserves template if present

**CreateWebhook (lines 188-233)**:
- Generates ID if not provided
- Validates URL (SSRF protection)
- Saves all webhooks to persistent storage
- Returns full webhook data

**UpdateWebhook (lines 235-289)**:
- Validates URL before update
- Updates webhook in manager
- Persists all webhooks to storage
- Returns updated webhook

**DeleteWebhook (lines 291-317)**:
- Removes webhook from manager
- Persists remaining webhooks

**TestNotification (lines 319-421)**:
- Supports "email" or "webhook" method
- Accepts optional config for testing without saving
- Gets actual node info from monitor state
- Routes to appropriate test sender

**TestWebhook (lines 487-601)**:
- Decodes webhook configuration
- Applies service-specific templates
- Calls enhanced webhook test
- Returns (statusCode, response body)

**GetNotificationHealth (lines 603-648)**:
- Returns queue statistics:
  - pending, sending, sent, failed, dlq counts
  - Healthy flag
- Email status: enabled, configured
- Webhook status: total, enabled count
- Overall health status

**GetWebhookHistory (lines 431-444)**:
- Returns recent webhook deliveries (last 100)
- Redacts secrets from URLs
- Redacts Telegram bot tokens
- Redacts query parameter secrets (token, apikey, key, secret, password)

**redactSecretsFromURL (lines 446-477)**:
- Telegram: /botXXXX:REDACTED/
- Query params: token=REDACTED, apikey=REDACTED, etc.

#### Security Features:
- All handlers require authentication (RequireAdmin wrapper)
- Sensitive fields masked/redacted
- SSRF protection on webhook URLs
- Password preservation on updates
- Scope-based access control (SettingsRead/SettingsWrite)

---

### 2. Notification Queue Endpoints Handler
**File**: `/opt/pulse/internal/api/notification_queue.go` (80+ lines)

#### NotificationQueueHandlers Structure:
- Reference to Monitor

#### Endpoints:
| Path | Method | Handler | Auth Scope |
|------|--------|---------|-----------|
| `/api/notifications/dlq` | GET | GetDLQ | MonitoringRead |
| `/api/notifications/queue/stats` | GET | GetQueueStats | MonitoringRead |
| `/api/notifications/dlq/retry` | POST | RetryDLQItem | MonitoringRead |
| `/api/notifications/dlq/delete` | POST | DeleteDLQItem | MonitoringRead |

#### Key Methods:
- **GetDLQ (lines 27-56)**:
  - Returns dead letter queue items
  - Supports limit parameter (default 100, max 1000)
  - Returns JSON array

- **GetQueueStats (lines 58-80)**:
  - Returns dictionary of queue statistics
  - Pending, sending, sent, failed, dlq counts
  - Healthy flag

---

### 3. Configuration Persistence
Integration with ConfigPersistence (mentioned in notifications.go):
- `SaveEmailConfig(config)` - Persists email settings
- `SaveWebhooks(webhooks)` - Persists all webhook configurations
- `SaveAppriseConfig(config)` - Persists Apprise settings

These are called after every modification to ensure durability.

---

### 4. Router Integration
**File**: `/opt/pulse/internal/api/router.go`

#### Notification Handler Registration (lines 147-148, 900-926):
```go
r.notificationHandlers = NewNotificationHandlers(r.monitor)
r.notificationQueueHandlers = NewNotificationQueueHandlers(r.monitor)

// Main notification endpoints
r.mux.HandleFunc("/api/notifications/", RequireAdmin(r.config, r.notificationHandlers.HandleNotifications))

// Queue management endpoints
r.mux.HandleFunc("/api/notifications/dlq", RequireAdmin(r.config, r.notificationQueueHandlers.GetDLQ))
r.mux.HandleFunc("/api/notifications/queue/stats", RequireAdmin(r.config, r.notificationQueueHandlers.GetQueueStats))
r.mux.HandleFunc("/api/notifications/dlq/retry", RequireAdmin(r.config, r.notificationQueueHandlers.RetryDLQItem))
r.mux.HandleFunc("/api/notifications/dlq/delete", RequireAdmin(r.config, r.notificationQueueHandlers.DeleteDLQItem))
```

#### Monitor Reference Update (line 1125-1126):
- Notification handlers updated when Monitor reference changes
- Ensures handlers always use latest monitor instance

#### Public URL Detection (line 1516):
- Public URL detected from inbound requests for webhook payload generation

---

## FRONTEND IMPLEMENTATION

### 1. Notification API Client
**File**: `/opt/pulse/frontend-modern/src/api/notifications.ts` (201 lines)

#### Interface Types:

**EmailConfig**:
```typescript
interface EmailConfig {
  enabled: boolean;
  provider: string;
  server: string;
  port: number;
  username: string;
  password?: string;
  from: string;
  to: string[];
  tls: boolean;
  startTLS: boolean;
}
```

**Webhook**:
```typescript
interface Webhook {
  id: string;
  name: string;
  url: string;
  method: string;
  headers: Record<string, string>;
  template?: string;
  enabled: boolean;
  service?: string;
  customFields?: Record<string, string>;
}
```

**AppriseConfig**:
```typescript
interface AppriseConfig {
  enabled: boolean;
  mode?: 'cli' | 'http';
  targets?: string[];
  cliPath?: string;
  timeoutSeconds?: number;
  serverUrl?: string;
  configKey?: string;
  apiKey?: string;
  apiKeyHeader?: string;
  skipTlsVerify?: boolean;
}
```

#### API Methods (NotificationsAPI class):

**Email Configuration**:
- `getEmailConfig()` → EmailConfig
  - Returns config with password field
- `updateEmailConfig(config)` → { success: boolean }
  - Sends: server, port, tls, startTLS (not smtpHost/smtpPort)
  - Sends: enabled, provider, username, password, from, to

**Webhook Management**:
- `getWebhooks()` → Webhook[]
- `createWebhook(webhook)` → Webhook
- `updateWebhook(id, webhook)` → Webhook
- `deleteWebhook(id)` → { success: boolean }

**Apprise Configuration**:
- `getAppriseConfig()` → AppriseConfig
- `updateAppriseConfig(config)` → AppriseConfig

**Templates & Providers**:
- `getEmailProviders()` → EmailProvider[]
- `getWebhookTemplates()` → WebhookTemplate[]

**Testing**:
- `testNotification(request)` → { success: boolean; message?: string }
  - Request: { type: 'email'|'webhook', config?: object, webhookId?: string }
- `testWebhook(webhook)` → { success: boolean; message?: string }

---

### 2. Frontend Components

#### Email Provider Selector
**File**: `/opt/pulse/frontend-modern/src/components/Alerts/EmailProviderSelect.tsx` (200+ lines)

Features:
- Loads provider templates dynamically
- Dropdown select with provider options
- Auto-fills SMTP host, port, TLS/STARTTLS based on selection
- Displays setup instructions for selected provider
- Advanced settings toggle (Reply-To, Max Retries, Rate Limit)
- Test connection button

Key States:
- `providers` - Loaded email provider templates
- `showAdvanced` - Toggle advanced settings display
- `showInstructions` - Show provider-specific setup instructions

Key Functions:
- `applyProvider(provider)` - Updates form with provider defaults
- `handleProviderChange(value)` - Routes provider selection
- `currentProvider()` - Gets currently selected provider

#### Webhook Configuration
**File**: `/opt/pulse/frontend-modern/src/components/Alerts/WebhookConfig.tsx` (300+ lines)

Features:
- Create/update/delete webhooks
- Service-specific configuration (Discord, Slack, Teams, Telegram, etc.)
- Custom headers management
- Custom fields management (e.g., Pushover app_token, user_token)
- Template selection from provided templates
- Custom payload template support
- Test webhook functionality
- URL template rendering support

Key States:
- `webhooks` - List of configured webhooks
- `editingId` - Currently editing webhook ID
- `selectedService` - Selected service type
- `customHeaders` - Header key-value inputs
- `customFields` - Custom field inputs based on service
- `templates` - Available webhook templates

Key Features:
- Service-specific custom field presets (Pushover fields, etc.)
- Template preview with field substitution
- URL template support with {{.}} placeholders
- Payload validation
- Header value masking (shows "***REDACTED**" for sensitive values)

#### Notification Stores
**File**: `/opt/pulse/frontend-modern/src/stores/notifications.ts`

Reactive state management for:
- Email configuration
- Webhook list
- Apprise configuration
- UI state (loading, errors, testing)

#### Settings Integration
**File**: `/opt/pulse/frontend-modern/src/components/Settings/Settings.tsx`

Integration point for:
- Email notification settings page
- Webhook management interface
- Apprise configuration
- Notification health monitoring

---

## DATA FLOW & SEQUENCES

### Alert Notification Flow
```
Alert Triggered (in monitoring)
    ↓
monitor.GetNotificationManager().SendAlert(alert)
    ↓
CheckCooldown → Skip if in cooldown
    ↓
AddToPendingAlerts → Start GroupingTimer
    ↓
GroupingWindow Timer Expires (default 30s)
    ↓
sendGroupedAlerts()
    ├→ Snapshot configurations (email, webhooks, apprise)
    ├→ If PersistentQueue available:
    │   └→ enqueueNotifications() → Queue.Enqueue()
    │
    └→ If No Queue:
        ├→ sendNotificationsDirect()
        ├→ For each enabled webhook:
        │   └→ sendGroupedWebhook() → async goroutine
        ├→ If email enabled:
        │   └→ sendGroupedEmail() → async goroutine
        └→ If apprise enabled:
            └→ sendGroupedApprise() → async goroutine
```

### Webhook Sending Flow
```
sendGroupedWebhook(webhook, alertList)
    ↓
Check for custom template
    ├→ If custom template exists:
    │   └→ generatePayloadFromTemplateWithService()
    │
    └→ If service-specific (Discord, Slack, etc.):
        ├→ Lookup service template
        ├→ Apply service-specific data enrichment
        ├→ generatePayloadFromTemplateWithService()
        │
        └→ If no template found:
            └→ Use generic JSON payload
    ↓
sendWebhookRequest(webhook, payload)
    ├→ checkWebhookRateLimit()
    ├→ Create HTTP request
    ├→ Set headers
    ├→ Send with secure client (SSRF protection)
    ├→ Read response (max 1MB)
    └→ addWebhookDelivery() to history
```

### Email Sending Flow
```
sendGroupedEmail(config, alertList)
    ↓
EmailTemplate(alertList, isSingle)
    ├→ Grouped vs single template
    └→ Generate HTML + text body
    ↓
sendHTMLEmailWithError(subject, htmlBody, textBody, config)
    ├→ Use From as To if To empty
    ├→ Get/create EnhancedEmailManager
    └→ SendEmailWithRetry()
        ├→ Attempt 1: sendEmailOnce()
        ├→ Check rate limit
        ├→ Build MIME message
        ├→ sendViaProvider()
        │   ├→ If TLS: sendTLS() [port 465]
        │   ├→ If STARTTLS: sendStartTLS() [port 587]
        │   └→ Else: sendPlain()
        └→ Retry up to 3 times with 5s delays
```

### Persistent Queue Flow
```
NotificationQueue starts (2 background goroutines)
    ├→ processQueue() - runs every 5 seconds
    │   ├→ SELECT WHERE status='pending' AND next_retry_at <= now
    │   ├→ For each notification:
    │   │   ├→ Update status to 'sending'
    │   │   ├→ Call processor.ProcessQueuedNotification()
    │   │   ├→ If success → Update status to 'sent', set completed_at
    │   │   └→ If failed → Update status to 'failed', set next_retry_at
    │   └→ After max attempts → Move to 'dlq' status
    │
    └→ cleanupOldEntries() - runs hourly
        └→ Delete entries older than retention period
```

---

## KEY SECURITY FEATURES

### 1. SSRF Prevention
- DNS resolution required for all webhook URLs
- Blocks localhost: 127.0.0.1, ::1, "localhost", 127.*
- Blocks private ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
- Blocks link-local: 169.254.*, fe80::*
- Blocks cloud metadata: 169.254.169.254, metadata.google.internal, metadata.goog
- Redirect validation: max 3 redirects allowed
- Numeric IP warning for HTTPS connections

### 2. Webhook Security
- Secure client with redirect limits
- Response size limit (1MB max)
- Timeout: 30s default, 10s for tests
- Rate limiting: 10 per minute per webhook URL
- URL validation before creation/update

### 3. Email Security
- Password not logged or returned in responses
- Password preserved on config update if empty
- TLS/STARTTLS enforcement
- Optional TLS verification skip (for self-signed certs)
- SMTP authentication with proper auth methods

### 4. API Security
- Scope-based access control (SettingsRead/SettingsWrite)
- Admin requirement on all notification endpoints
- Sensitive field masking in API responses
- Token/secret redaction in webhook history
- Raw body reading (not automatic JSON decode) for sensitive endpoints

### 5. Queue Security
- Persistent database in user data directory
- SQLite WAL mode for atomicity
- Foreign key constraints
- Audit trail table for all notifications

---

## CONFIGURATION PERSISTENCE

### Storage Mechanism
Configurations are persisted via ConfigPersistence interface:
- Implementations handle storage (likely YAML/JSON files or database)
- Called after every modification
- Survive service restarts

### Persisted Configs
1. Email configuration (EmailConfig struct)
2. All webhook configurations ([]WebhookConfig)
3. Apprise configuration (AppriseConfig struct)
4. Alert notification settings (cooldown, grouping, etc.)

### Startup Sequence
1. Load configs from persistent storage
2. Initialize NotificationManager with loaded configs
3. Initialize EnhancedEmailManager with email config
4. Initialize persistent queue (if enabled)
5. Start background processors

---

## NOTIFICATION TYPES & TRIGGERS

### Alert Levels
- **Critical** (red, #ff6b6b)
- **Warning** (yellow, #ffd93d)
- **Info** (green)

### Alert Types
- cpu
- memory
- disk
- io
- diskRead
- diskWrite
- temperature (from sensor proxy)

### Metrics Formatting
- CPU/Memory: percentages (e.g., 95.5%)
- Disk I/O: MB/s (e.g., 150.2 MB/s)
- Temperature: degrees (e.g., 78.5°C)

### Grouping Configuration
- Cooldown: Per-alert grace period (default 5 minutes)
- Grouping window: Alert batching window (default 30 seconds)
- Group by node: Organize alerts by node
- Group by guest type: Organize alerts by VM/LXC/host

---

## ERROR HANDLING & RETRY LOGIC

### Email Retries
- Max 3 attempts (configurable)
- 5 second delay between attempts (configurable)
- Rate limiting: 60 per minute (configurable)

### Webhook Retries
- Max 3 attempts (configurable per webhook)
- Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (max)
- Respects Retry-After header for 429 responses
- Retryable errors: timeouts, network errors, 5xx, 429
- Non-retryable: 4xx client errors (except 429)

### Queue Retries
- Configurable max attempts per notification
- Periodic retry processor (5 second intervals)
- Dead letter queue for permanently failed items
- Manual DLQ retry/delete via API

### Logging
- Structured logging with zerolog
- All critical operations logged with context:
  - Alert ID, resource name, type
  - Webhook name, service, URL (redacted)
  - Email recipients and SMTP server
  - Retry counts and errors

---

## CONSTANTS & LIMITS

### Webhook Settings
- Timeout: 30 seconds
- Test timeout: 10 seconds
- Max response size: 1 MB
- Max redirects: 3
- Rate limit: 10 per minute per webhook URL
- Initial backoff: 1 second
- Max backoff: 30 seconds
- Default retries: 3
- History size: Last 100 deliveries

### Email Settings
- Dial timeout: 10 seconds
- Connection deadline: 30 seconds
- Default rate limit: 60 per minute
- Default retries: 3
- Default retry delay: 5 seconds

### Queue Settings
- Processor interval: 5 seconds
- Cleanup interval: 1 hour
- Database journal: WAL mode
- Cache: 64 MB
- Busy timeout: 5 seconds

### Apprise Settings
- Timeout bounds: 5-120 seconds (default 15)
- Max targets: Unlimited (but must have >0)
- CLI path default: "apprise"
- API key header default: "X-API-KEY"

---

## FILES SUMMARY TABLE

| File | Lines | Purpose |
|------|-------|---------|
| `notifications.go` | 2,358 | Core notification manager, alert grouping, webhook/email/apprise sending |
| `webhook_enhanced.go` | 605 | Enhanced webhook support, retry logic, filtering |
| `email_enhanced.go` | 415 | Email manager, SMTP connections, provider support |
| `email_providers.go` | 217 | Email provider templates and instructions |
| `email_template.go` | 200+ | HTML email templates for single/grouped alerts |
| `webhook_templates.go` | 300+ | Templates for Discord, Slack, Teams, Telegram, etc. |
| `queue.go` | 600+ | Persistent notification queue, SQLite backend |
| `notifications.go` (API) | 753 | HTTP handlers for all notification endpoints |
| `notification_queue.go` (API) | 80+ | HTTP handlers for queue management |
| `notifications.ts` (Frontend) | 201 | TypeScript API client for notification endpoints |
| `EmailProviderSelect.tsx` | 200+ | Email provider configuration UI component |
| `WebhookConfig.tsx` | 300+ | Webhook configuration UI component |

---

## INTEGRATION POINTS WITH OTHER SYSTEMS

### Monitor Integration
- `monitor.GetNotificationManager()` - Access notification manager
- `monitor.GetConfigPersistence()` - Persist configurations
- `monitor.GetState()` - Get node and instance info for test notifications

### Alert System Integration
- Receives Alert objects with:
  - ID, Type, Level, ResourceName, ResourceID
  - Node, Instance, Message
  - Value, Threshold, StartTime, LastSeen
  - Metadata (resourceType, etc.)
  - Acknowledged, AckTime, AckUser

### Router Integration
- `/api/notifications/` main handler routes to sub-endpoints
- All endpoints require admin auth + scope
- Integrated in main API router initialization

### Config System
- Notifications loaded/saved via ConfigPersistence
- Can survive service restarts
- Loaded at startup into NotificationManager

---

## WORKFLOW EXAMPLES

### Example 1: Configure Gmail Notifications
```
User → Settings → Email Notifications
  ↓
Select "Gmail / Google Workspace" from provider dropdown
  ↓
Auto-filled:
  - Server: smtp.gmail.com
  - Port: 587
  - STARTTLS: true
  - Display: Setup instructions
  ↓
User enters:
  - From: user@gmail.com
  - To: [alerts@company.com]
  - Username: user@gmail.com
  - Password: <app-password>
  ↓
PUT /api/notifications/email
  ↓
Backend: UpdateEmailConfig() → SetEmailConfig() → SaveEmailConfig()
  ↓
Test → POST /api/notifications/test {method: "email"}
  ↓
SendTestNotification() → sendEmail() → EnhancedEmailManager.SendEmailWithRetry()
  ↓
SMTP → gmail via STARTTLS → Success/Error logged
  ↓
Frontend: Display success/error
```

### Example 2: Add Discord Webhook
```
User → Settings → Webhooks → Add New
  ↓
Select Service: Discord
  ↓
Copy webhook URL from Discord server
  ↓
Enter URL: https://discord.com/api/webhooks/...
  ↓
Backend: ValidateWebhookURL() → DNS check → Private IP check → SSRF check
  ↓
POST /api/notifications/webhooks
  ↓
Backend: GenerateID() → AddWebhook() → SaveWebhooks()
  ↓
Test: POST /api/notifications/webhooks/test
  ↓
Backend: sendWebhook() → Apply Discord template
  ↓
Generate payload with embed format → Send HTTP POST
  ↓
Discord returns 204 → Success logged to webhook history
  ↓
Frontend: Display success + sample embed preview
```

### Example 3: Alert Triggering Grouped Notifications
```
Monitoring Detects: CPU > 95% on node1
  ↓
Create Alert → monitor.GetNotificationManager().SendAlert(alert)
  ↓
NotificationManager.SendAlert():
  - Check cooldown → Not in cooldown
  - Add to pendingAlerts list
  - Start groupingTimer (30s)
  ↓
Wait 5s → Another alert: Memory > 90% on node1
  - Add to pendingAlerts list
  - Timer still running
  ↓
Wait 25s → Grouping timer expires
  ↓
sendGroupedAlerts() with 2 alerts:
  - Snapshot config (2 webhooks, email enabled)
  - If queue: Enqueue 3 notifications (email + 2 webhooks)
  - If no queue: Send directly (3 async goroutines)
  ↓
For each webhook:
  - sendGroupedWebhook(webhook, [alert1, alert2])
  - Prepare template data with AlertCount=2
  - Generate combined message
  - Send single HTTP POST
  ↓
For email:
  - sendGroupedEmail(emailConfig, [alert1, alert2])
  - Template generates grouped email
  - Send via SMTP with retries
  ↓
Queue (if enabled):
  - Background processor retries every 5s
  - Updates status: pending → sending → sent/failed
  - Failed items retry with backoff
  - After 3 attempts, move to DLQ
  ↓
Update last notified timestamps
```

