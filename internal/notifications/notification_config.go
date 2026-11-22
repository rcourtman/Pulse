package notifications

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// copyEmailConfig returns a defensive copy of EmailConfig including its slices to avoid data races.
func copyEmailConfig(cfg EmailConfig) EmailConfig {
	copy := cfg
	if len(cfg.To) > 0 {
		copy.To = append([]string(nil), cfg.To...)
	}
	if len(cfg.TagFilter) > 0 {
		copy.TagFilter = append([]string(nil), cfg.TagFilter...)
	}
	return copy
}

// copyWebhookConfigs deep-copies webhook configurations to isolate concurrent writers from background senders.
func copyWebhookConfigs(webhooks []WebhookConfig) []WebhookConfig {
	if len(webhooks) == 0 {
		return nil
	}

	copies := make([]WebhookConfig, 0, len(webhooks))
	for _, webhook := range webhooks {
		clone := webhook
		if len(clone.Headers) > 0 {
			headers := make(map[string]string, len(webhook.Headers))
			for k, v := range clone.Headers {
				headers[k] = v
			}
			clone.Headers = headers
		}
		if len(clone.CustomFields) > 0 {
			custom := make(map[string]string, len(clone.CustomFields))
			for k, v := range clone.CustomFields {
				custom[k] = v
			}
			clone.CustomFields = custom
		}
		if len(clone.TagFilter) > 0 {
			clone.TagFilter = append([]string(nil), clone.TagFilter...)
		}
		copies = append(copies, clone)
	}

	return copies
}

func copyWebhookConfig(webhook WebhookConfig) WebhookConfig {
	clones := copyWebhookConfigs([]WebhookConfig{webhook})
	if len(clones) == 0 {
		return WebhookConfig{}
	}
	return clones[0]
}

// NormalizeWebhookConfig canonicalizes service-specific webhook configuration fields
// at ingress and persistence boundaries. Runtime delivery paths should already
// carry canonical webhook state and must not rely on legacy alias migration here.
func NormalizeWebhookConfig(webhook WebhookConfig) WebhookConfig {
	normalized := webhook
	normalized.TagFilter = normalizeNotificationTagFilter(normalized.TagFilter)
	normalized.MinimumSeverity = normalizeNotificationMinimumSeverity(normalized.MinimumSeverity)
	if len(normalized.TagFilter) == 0 {
		normalized.TagMode = ""
	} else {
		normalized.TagMode = normalizeNotificationTagMode(normalized.TagMode)
	}
	if strings.EqualFold(strings.TrimSpace(normalized.Service), "pushover") {
		normalized.CustomFields = normalizePushoverWebhookCustomFields(normalized.CustomFields)
	}
	normalized.Headers = normalizeWebhookHeaders(normalized.Headers)
	return normalized
}

// normalizeWebhookHeaders drops headers with a blank name or value. A cleared
// header field in the editor means "no header"; sending it empty would replace
// the default Content-Type and leave receivers unable to parse the body.
func normalizeWebhookHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			normalized[key] = value
		}
	}
	return normalized
}

func normalizePushoverWebhookCustomFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}

	normalized := make(map[string]string, len(fields))
	for key, value := range fields {
		normalized[key] = value
	}

	token := strings.TrimSpace(normalized["token"])
	if token == "" {
		if legacy := strings.TrimSpace(normalized["app_token"]); legacy != "" {
			normalized["token"] = legacy
		}
	}

	user := strings.TrimSpace(normalized["user"])
	if user == "" {
		if legacy := strings.TrimSpace(normalized["user_token"]); legacy != "" {
			normalized["user"] = legacy
		}
	}

	delete(normalized, "app_token")
	delete(normalized, "user_token")

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func copyAppriseConfig(cfg AppriseConfig) AppriseConfig {
	copy := cfg
	if len(cfg.Targets) > 0 {
		copy.Targets = append([]string(nil), cfg.Targets...)
	}
	return copy
}

// annotateResolvedMetadata stores the resolution timestamp on the alert metadata for queue persistence.
func annotateResolvedMetadata(alert *alerts.Alert, resolvedAt time.Time) {
	if alert == nil {
		return
	}
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{})
	}
	alert.Metadata[metadataResolvedAt] = resolvedAt.Format(time.RFC3339Nano)
}

// resolvedAlertMessage is the one-line account of a resolved alert. A close
// that was not a recovery, such as a node alert that moved to the Pulse agent
// now monitoring the machine, says what happened instead of "now healthy".
func resolvedAlertMessage(alert *alerts.Alert) string {
	if summary := resolvedAlertNotRecoveredSummary(alert); summary != "" {
		return summary
	}
	return fmt.Sprintf("%s on %s is now healthy", alert.ResourceName, alert.Node)
}

// resolvedAlertNotRecoveredSummary names the alert in its resolution summary,
// so one of several closes for a machine says which alert moved. It is empty
// for an ordinary recovery.
func resolvedAlertNotRecoveredSummary(alert *alerts.Alert) string {
	label := alertTypeDisplay(alert.Type)
	if label != "" {
		label += " alert"
	}
	return alert.Resolution.Describe(label)
}

// resolvedAlertsIncludeNotRecovered reports whether any close in a resolved
// notification was not a recovery, so recovery visuals must not be used.
func resolvedAlertsIncludeNotRecovered(alertList []*alerts.Alert) bool {
	for _, alert := range alertList {
		if alert != nil && alert.Resolution.Outcome() != "" {
			return true
		}
	}
	return false
}

// resolvedAlertListEntry names one alert in a grouped recovery list.
func resolvedAlertListEntry(alert *alerts.Alert) string {
	entry := fmt.Sprintf("%s on %s", alert.ResourceName, alert.Node)
	if outcome := alert.Resolution.Outcome(); outcome != "" {
		entry += ", " + outcome
	}
	return entry
}

func quietHoursReplayAtForAlert(alert *alerts.Alert, now time.Time) *time.Time {
	if alert == nil || alert.Metadata == nil {
		return nil
	}
	raw, ok := alert.Metadata[alerts.MetadataQuietHoursReplayAt]
	if !ok {
		return nil
	}

	var parsed time.Time
	switch value := raw.(type) {
	case string:
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err != nil {
			return nil
		}
		parsed = ts
	case time.Time:
		parsed = value
	case float64:
		if value <= 0 {
			return nil
		}
		parsed = time.Unix(int64(value), 0)
	default:
		return nil
	}

	if !parsed.After(now) {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func notificationQueueBucketsForJob(job notificationDeliveryJob, now time.Time) []notificationQueueBucket {
	if len(job.Alerts) == 0 {
		return []notificationQueueBucket{{job: job}}
	}

	buckets := make([]notificationQueueBucket, 0, len(job.Alerts))
	bucketIndexes := make(map[string]int, len(job.Alerts))
	for _, alert := range job.Alerts {
		nextRetryAt := quietHoursReplayAtForAlert(alert, now)
		key := "immediate"
		if nextRetryAt != nil {
			key = nextRetryAt.Format(time.RFC3339Nano)
		}

		index, exists := bucketIndexes[key]
		if !exists {
			bucketJob := job
			bucketJob.Alerts = nil
			bucket := notificationQueueBucket{job: bucketJob}
			if nextRetryAt != nil {
				replayAt := *nextRetryAt
				bucket.nextRetryAt = &replayAt
			}
			index = len(buckets)
			bucketIndexes[key] = index
			buckets = append(buckets, bucket)
		}
		buckets[index].job.Alerts = append(buckets[index].job.Alerts, alert)
	}

	return buckets
}

// NormalizeAppriseConfig cleans and normalizes Apprise configuration values.
func NormalizeAppriseConfig(cfg AppriseConfig) AppriseConfig {
	normalized := cfg
	normalized.MinimumSeverity = normalizeNotificationMinimumSeverity(normalized.MinimumSeverity)

	mode := strings.ToLower(strings.TrimSpace(string(normalized.Mode)))
	switch mode {
	case string(AppriseModeHTTP):
		normalized.Mode = AppriseModeHTTP
	default:
		normalized.Mode = AppriseModeCLI
	}

	normalized.CLIPath = "apprise" // Force default binary for security

	if normalized.TimeoutSeconds <= 0 {
		normalized.TimeoutSeconds = appriseDefaultTimeoutSecs
	} else if normalized.TimeoutSeconds > appriseMaxTimeoutSecs {
		normalized.TimeoutSeconds = appriseMaxTimeoutSecs
	} else if normalized.TimeoutSeconds < appriseMinTimeoutSecs {
		normalized.TimeoutSeconds = appriseMinTimeoutSecs
	}

	cleanTargets := make([]string, 0, len(normalized.Targets))
	seen := make(map[string]struct{}, len(normalized.Targets))
	for _, target := range normalized.Targets {
		trimmed := strings.TrimSpace(target)
		// A target is a destination. One starting with a dash would reach the
		// Apprise CLI as an option, such as --attach with a server file.
		if trimmed == "" || strings.HasPrefix(trimmed, "-") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		cleanTargets = append(cleanTargets, trimmed)
	}
	normalized.Targets = cleanTargets

	normalized.ServerURL = strings.TrimSpace(normalized.ServerURL)
	normalized.ServerURL = strings.TrimRight(normalized.ServerURL, "/")

	normalized.ConfigKey = strings.TrimSpace(normalized.ConfigKey)

	normalized.APIKey = strings.TrimSpace(normalized.APIKey)
	normalized.APIKeyHeader = strings.TrimSpace(normalized.APIKeyHeader)
	if normalized.APIKeyHeader == "" {
		normalized.APIKeyHeader = "X-API-KEY"
	}

	switch normalized.Mode {
	case AppriseModeCLI:
		if len(normalized.Targets) == 0 {
			normalized.Enabled = false
		}
	case AppriseModeHTTP:
		if normalized.ServerURL == "" {
			normalized.Enabled = false
		}
	}

	return normalized
}

type notificationRecord struct {
	successOrder int64
	lastSent     time.Time
	alertStart   time.Time
	level        alerts.AlertLevel // Highest delivered severity for this occurrence.
	resolved     bool
}

// Alert represents an alert (interface to avoid circular dependency)
type Alert interface {
	GetID() string
	GetResourceName() string
	GetType() string
	GetLevel() string
	GetValue() float64
	GetThreshold() float64
	GetMessage() string
	GetNode() string
	GetInstance() string
	GetStartTime() time.Time
}

// EmailConfig holds email notification settings
type EmailConfig struct {
	Enabled         bool     `json:"enabled"`
	Provider        string   `json:"provider"` // Email provider name (Gmail, SendGrid, etc.)
	SMTPHost        string   `json:"server"`   // Changed from smtpHost to server for frontend consistency
	SMTPPort        int      `json:"port"`     // Changed from smtpPort to port for frontend consistency
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	From            string   `json:"from"`
	To              []string `json:"to"`
	TLS             bool     `json:"tls"`
	StartTLS        bool     `json:"startTLS"`  // STARTTLS support
	RateLimit       int      `json:"rateLimit"` // Max emails per minute (0 = default 60)
	TagFilter       []string `json:"tagFilter,omitempty"`
	TagMode         string   `json:"tagFilterMode,omitempty"`   // "all" (default) or "any"
	MinimumSeverity string   `json:"minimumSeverity,omitempty"` // "all" (default), "warning", or "critical"
	SubjectPrefix   string   `json:"subjectPrefix,omitempty"`   // Optional custom subject prefix (replaces "[Pulse Alert]")
	BrandName       string   `json:"brandName,omitempty"`       // Optional custom header/footer brand name (replaces "Pulse Monitoring")
	FooterNote      string   `json:"footerNote,omitempty"`      // Optional plain-text note appended to email footers
}

// WebhookConfig holds webhook settings
type WebhookConfig struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Enabled         bool              `json:"enabled"`
	Service         string            `json:"service"`  // discord, slack, teams, etc.
	Template        string            `json:"template"` // Custom payload template
	CustomFields    map[string]string `json:"customFields,omitempty"`
	Mention         string            `json:"mention,omitempty"` // Platform-specific mention (e.g., @everyone, @channel, <@USER_ID>)
	TagFilter       []string          `json:"tagFilter,omitempty"`
	TagMode         string            `json:"tagFilterMode,omitempty"`   // "all" (default) or "any"
	MinimumSeverity string            `json:"minimumSeverity,omitempty"` // "all" (default), "warning", or "critical"
	// SigningSecret enables HMAC-SHA256 signing of outbound deliveries.
	// When set, requests carry X-Pulse-Timestamp and X-Pulse-Signature
	// (v1=hex(hmac_sha256(secret, timestamp + "." + body))) so receivers
	// can verify origin and integrity.
	SigningSecret string `json:"signingSecret,omitempty"`
}

// AppriseMode identifies how Pulse should deliver notifications through Apprise.
type AppriseMode string

const (
	AppriseModeCLI  AppriseMode = "cli"
	AppriseModeHTTP AppriseMode = "http"
)

const (
	defaultSMTPPort       = 587
	defaultEmailRateLimit = 60
)

// AppriseConfig holds Apprise notification settings.
type AppriseConfig struct {
	Enabled         bool        `json:"enabled"`
	Mode            AppriseMode `json:"mode,omitempty"`
	Targets         []string    `json:"targets"`
	CLIPath         string      `json:"cliPath,omitempty"`
	TimeoutSeconds  int         `json:"timeoutSeconds,omitempty"`
	ServerURL       string      `json:"serverUrl,omitempty"`
	ConfigKey       string      `json:"configKey,omitempty"`
	APIKey          string      `json:"apiKey,omitempty"`
	APIKeyHeader    string      `json:"apiKeyHeader,omitempty"`
	SkipTLSVerify   bool        `json:"skipTlsVerify,omitempty"`
	MinimumSeverity string      `json:"minimumSeverity,omitempty"` // "all" (default), "warning", or "critical"
}

func normalizeEmailConfig(cfg EmailConfig) EmailConfig {
	normalized := cfg
	normalized.Provider = strings.TrimSpace(normalized.Provider)
	normalized.SMTPHost = strings.TrimSpace(normalized.SMTPHost)
	normalized.Username = strings.TrimSpace(normalized.Username)
	normalized.From = strings.TrimSpace(normalized.From)
	normalized.TagFilter = normalizeNotificationTagFilter(normalized.TagFilter)
	normalized.MinimumSeverity = normalizeNotificationMinimumSeverity(normalized.MinimumSeverity)
	branding := normalized.Branding()
	normalized.SubjectPrefix = branding.SubjectPrefix
	normalized.BrandName = branding.BrandName
	normalized.FooterNote = branding.FooterNote
	if len(normalized.TagFilter) == 0 {
		normalized.TagMode = ""
	} else {
		normalized.TagMode = normalizeNotificationTagMode(normalized.TagMode)
	}

	if normalized.SMTPPort <= 0 || normalized.SMTPPort > 65535 {
		log.Warn().
			Int("smtpPort", normalized.SMTPPort).
			Int("defaultPort", defaultSMTPPort).
			Msg("Invalid SMTP port in email config, using default")
		normalized.SMTPPort = defaultSMTPPort
	}

	if normalized.RateLimit < 0 {
		log.Warn().
			Int("rateLimit", normalized.RateLimit).
			Msg("Invalid negative email rate limit, using default behavior")
		normalized.RateLimit = 0
	}

	if len(normalized.To) > 0 {
		cleaned := make([]string, 0, len(normalized.To))
		seen := make(map[string]struct{}, len(normalized.To))
		for _, recipient := range normalized.To {
			trimmed := strings.TrimSpace(recipient)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			cleaned = append(cleaned, trimmed)
		}
		normalized.To = cleaned
	}

	return normalized
}

func effectiveEmailRateLimit(configured int) int {
	if configured <= 0 {
		return defaultEmailRateLimit
	}
	return configured
}
