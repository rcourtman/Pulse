package notifications

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// alertListThreadID returns the incident thread ID when the email covers
// exactly one alert. Grouped emails don't thread: firing and resolved
// batches rarely contain the same alert set, so a group-level reference
// would attach messages to the wrong thread more often than the right one.
func alertListThreadID(alertList []*alerts.Alert) string {
	if len(alertList) != 1 || alertList[0] == nil {
		return ""
	}
	return alertThreadMessageID(alertList[0].ID, alertList[0].StartTime)
}

// sendGroupedEmail sends a grouped email notification
func (n *NotificationManager) sendGroupedEmail(config EmailConfig, alertList []*alerts.Alert) error {

	// Don't check for recipients here - sendHTMLEmail handles empty recipients
	// by using the From address as the recipient

	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate(alertList, false, n.publicURL)

	// Send using HTML-aware method
	return n.sendThreadedHTMLEmailWithError(subject, htmlBody, textBody, alertListThreadID(alertList), config)
}

func (n *NotificationManager) sendResolvedEmail(config EmailConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	subject, htmlBody, textBody := buildResolvedNotificationContent(alertList, resolvedAt, n.publicURL)
	if subject == "" && textBody == "" {
		return fmt.Errorf("failed to build resolved email content")
	}
	subject = resolvedEmailSubject(alertList)

	return n.sendThreadedHTMLEmailWithError(subject, htmlBody, textBody, alertListThreadID(alertList), config)
}

// sendEmail sends an email notification
func (n *NotificationManager) sendSingleEmailWithError(alert *alerts.Alert, config EmailConfig) error {
	subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true, n.publicURL)
	return n.sendThreadedHTMLEmailWithError(subject, htmlBody, textBody, alertListThreadID([]*alerts.Alert{alert}), config)
}

func (n *NotificationManager) sendEmail(alert *alerts.Alert) {
	n.mu.RLock()
	config := n.emailConfig
	n.mu.RUnlock()

	if err := n.sendSingleEmailWithError(alert, config); err != nil {
		log.Error().
			Err(err).
			Str("alertID", alert.ID).
			Msg("failed to send alert email notification")
	}
}

func effectiveEmailRecipients(config EmailConfig) []string {
	recipients := config.To
	if len(recipients) == 0 && config.From != "" {
		recipients = []string{config.From}
	}
	return recipients
}

func newEmailDeliveryManager(config EmailConfig, recipients []string) *EnhancedEmailManager {
	rl := effectiveEmailRateLimit(config.RateLimit)
	return NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{
			Provider:  config.Provider,
			From:      config.From,
			To:        recipients,
			SMTPHost:  config.SMTPHost,
			SMTPPort:  config.SMTPPort,
			Username:  config.Username,
			Password:  config.Password,
			TLS:       config.TLS,
			StartTLS:  config.StartTLS,
			RateLimit: config.RateLimit,
		},
		Provider:      config.Provider,
		StartTLS:      config.StartTLS,
		MaxRetries:    2,
		RetryDelay:    3,
		RateLimit:     rl,
		SkipTLSVerify: false,
		AuthRequired:  config.Username != "" && config.Password != "",
	})
}

func emailDeliveryTransportMatches(manager *EnhancedEmailManager, config EmailConfig) bool {
	if manager == nil {
		return false
	}
	persisted := manager.config.EmailConfig
	persistedProvider := manager.config.Provider
	if persistedProvider == "" {
		persistedProvider = persisted.Provider
	}
	return persisted.SMTPHost == config.SMTPHost &&
		persisted.SMTPPort == config.SMTPPort &&
		persisted.Username == config.Username &&
		persisted.Password == config.Password &&
		persisted.TLS == config.TLS &&
		persisted.StartTLS == config.StartTLS &&
		persistedProvider == config.Provider
}

func (n *NotificationManager) emailDeliveryManager(config EmailConfig) (*EnhancedEmailManager, []string) {
	recipients := effectiveEmailRecipients(config)

	n.mu.RLock()
	manager := n.emailManager
	n.mu.RUnlock()

	if manager == nil {
		return newEmailDeliveryManager(config, recipients), recipients
	}

	if !emailDeliveryTransportMatches(manager, config) {
		return newEmailDeliveryManager(config, recipients), recipients
	}

	// Concurrent queue workers and reports may share an SMTP transport while
	// carrying different admitted envelopes. Keep the cached manager immutable:
	// otherwise preparing one delivery changes another delivery's recipients,
	// sender and rate policy while it is being sent or waiting to retry.
	deliveryConfig := manager.config
	deliveryConfig.EmailConfig = copyEmailConfig(config)
	deliveryConfig.To = append([]string(nil), recipients...)
	deliveryConfig.Provider = config.Provider
	deliveryConfig.StartTLS = config.StartTLS
	deliveryConfig.RateLimit = effectiveEmailRateLimit(config.RateLimit)
	deliveryConfig.AuthRequired = config.Username != "" && config.Password != ""
	delivery := NewEnhancedEmailManager(deliveryConfig)
	// Isolating envelopes must not give every delivery a fresh rate budget.
	delivery.rateLimit = manager.rateLimit
	return delivery, recipients
}

// sendHTMLEmailWithError sends an HTML email with multipart content and returns any error
func (n *NotificationManager) sendHTMLEmailWithError(subject, htmlBody, textBody string, config EmailConfig) error {
	return n.sendThreadedHTMLEmailWithError(subject, htmlBody, textBody, "", config)
}

// sendThreadedHTMLEmailWithError is sendHTMLEmailWithError plus an optional
// incident thread ID (see alertThreadMessageID).
func (n *NotificationManager) sendThreadedHTMLEmailWithError(subject, htmlBody, textBody, threadID string, config EmailConfig) error {
	config = normalizeEmailConfig(config)

	recipients := effectiveEmailRecipients(config)
	if len(config.To) == 0 && config.From != "" {
		log.Info().
			Str("from", config.From).
			Msg("using From address as recipient since To is empty")
	}

	manager, recipients := n.emailDeliveryManager(config)

	log.Info().
		Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
		Str("from", config.From).
		Strs("to", recipients).
		Bool("hasAuth", manager.config.AuthRequired).
		Bool("startTLS", manager.config.StartTLS).
		Msg("attempting to send email via SMTP with enhanced support")

	err := manager.SendEmailThreaded(subject, htmlBody, textBody, threadID)

	if err != nil {
		log.Error().
			Err(err).
			Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
			Strs("recipients", recipients).
			Msg("failed to send email notification")
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Info().
		Strs("recipients", recipients).
		Int("recipientCount", len(recipients)).
		Msg("email notification sent successfully")
	return nil
}
