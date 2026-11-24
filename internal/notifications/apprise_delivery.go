package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
)

func (n *NotificationManager) sendGroupedApprise(config AppriseConfig, alertList []*alerts.Alert) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	cfg := NormalizeAppriseConfig(config)
	if !cfg.Enabled {
		return FailfWithClass(NotificationFailureConfiguration, "apprise not enabled")
	}

	title, body, notifyType := buildApprisePayload(alertList, n.publicURL)
	if title == "" && body == "" {
		return fmt.Errorf("failed to build apprise payload")
	}

	switch cfg.Mode {
	case AppriseModeHTTP:
		if err := n.sendAppriseViaHTTP(cfg, title, body, notifyType); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Msg("failed to send Apprise notification via API")
			return fmt.Errorf("apprise HTTP send failed: %w", err)
		}
	default:
		if err := n.sendAppriseViaCLI(cfg, title, body); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Int("targetCount", len(cfg.Targets)).
				Msg("failed to send Apprise notification")
			return fmt.Errorf("apprise CLI send failed: %w", err)
		}
	}
	return nil
}

func buildApprisePayload(alertList []*alerts.Alert, publicURL string) (string, string, string) {
	validAlerts := make([]*alerts.Alert, 0, len(alertList))
	var primary *alerts.Alert
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		if primary == nil {
			primary = alert
		}
		validAlerts = append(validAlerts, alert)
	}

	if len(validAlerts) == 0 || primary == nil {
		return "", "", "info"
	}

	title := fmt.Sprintf("Pulse alert: %s", primary.ResourceName)
	if len(validAlerts) > 1 {
		title = fmt.Sprintf("Pulse alerts (%d)", len(validAlerts))
	}

	var bodyBuilder strings.Builder
	bodyBuilder.WriteString(primary.Message)
	bodyBuilder.WriteString("\n\n")

	for _, alert := range validAlerts {
		bodyBuilder.WriteString(fmt.Sprintf("[%s] %s", strings.ToUpper(string(alert.Level)), alert.ResourceName))
		bodyBuilder.WriteString(fmt.Sprintf(" — value %.2f (threshold %.2f)\n", alert.Value, alert.Threshold))
		if alert.Node != "" {
			bodyBuilder.WriteString(fmt.Sprintf("Node: %s\n", alertNodeDisplay(alert)))
		}
		if alert.Instance != "" && alert.Instance != alert.Node {
			bodyBuilder.WriteString(fmt.Sprintf("Instance: %s\n", alert.Instance))
		}
		if link := alertLinkURL(publicURL, alert); link != "" {
			bodyBuilder.WriteString("View in Pulse: " + link + "\n")
		}
		bodyBuilder.WriteString("\n")
	}

	if publicURL != "" {
		bodyBuilder.WriteString("Dashboard: " + publicURL + "\n")
	}

	return title, bodyBuilder.String(), resolveAppriseNotificationType(validAlerts)
}

func buildResolvedNotificationContent(alertList []*alerts.Alert, resolvedAt time.Time, publicURL string) (string, string, string) {
	validAlerts := make([]*alerts.Alert, 0, len(alertList))
	var primary *alerts.Alert
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		if primary == nil {
			primary = alert
		}
		validAlerts = append(validAlerts, alert)
	}

	if len(validAlerts) == 0 || primary == nil {
		return "", "", ""
	}

	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	resolvedLabel := resolvedAt.Format(time.RFC3339)

	title := fmt.Sprintf("Pulse alert resolved: %s", primary.ResourceName)
	if primary.Resolution.Outcome() != "" {
		title = fmt.Sprintf("Pulse alert moved: %s", primary.ResourceName)
	}
	if len(validAlerts) > 1 {
		title = fmt.Sprintf("Pulse alerts resolved (%d)", len(validAlerts))
	}

	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Resolved at ")
	bodyBuilder.WriteString(resolvedLabel)
	bodyBuilder.WriteString("\n\n")

	for _, alert := range validAlerts {
		bodyBuilder.WriteString(fmt.Sprintf("[%s] %s\n", strings.ToUpper(string(alert.Level)), alert.ResourceName))
		if alert.Message != "" {
			bodyBuilder.WriteString(alert.Message)
			bodyBuilder.WriteString("\n")
		}
		if summary := resolvedAlertNotRecoveredSummary(alert); summary != "" {
			bodyBuilder.WriteString(summary)
			bodyBuilder.WriteString("\n")
		}
		if !alert.StartTime.IsZero() {
			bodyBuilder.WriteString("Started: ")
			bodyBuilder.WriteString(alert.StartTime.Format(time.RFC3339))
			bodyBuilder.WriteString("\n")
		}
		bodyBuilder.WriteString("Cleared: ")
		bodyBuilder.WriteString(resolvedLabel)
		bodyBuilder.WriteString("\n")
		if alert.Node != "" {
			bodyBuilder.WriteString("Node: ")
			bodyBuilder.WriteString(alertNodeDisplay(alert))
			bodyBuilder.WriteString("\n")
		}
		if alert.Instance != "" && alert.Instance != alert.Node {
			bodyBuilder.WriteString("Instance: ")
			bodyBuilder.WriteString(alert.Instance)
			bodyBuilder.WriteString("\n")
		}
		if alert.Threshold != 0 || alert.Value != 0 {
			bodyBuilder.WriteString(fmt.Sprintf("Last value %.2f (threshold %.2f)\n", alert.Value, alert.Threshold))
		}
		if link := alertLinkURL(publicURL, alert); link != "" {
			bodyBuilder.WriteString("View in Pulse: ")
			bodyBuilder.WriteString(link)
			bodyBuilder.WriteString("\n")
		}
		bodyBuilder.WriteString("\n")
	}

	if publicURL != "" {
		bodyBuilder.WriteString("Dashboard: ")
		bodyBuilder.WriteString(publicURL)
		bodyBuilder.WriteString("\n")
	}

	textBody := bodyBuilder.String()
	htmlBody := "<pre style=\"font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, \\\"Liberation Mono\\\", \\\"Courier New\\\", monospace\">" +
		html.EscapeString(textBody) + "</pre>"

	return title, htmlBody, textBody
}

func resolveAppriseNotificationType(alertList []*alerts.Alert) string {
	notifyType := "info"
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		switch alert.Level {
		case alerts.AlertLevelCritical:
			return "failure"
		case alerts.AlertLevelWarning:
			notifyType = "warning"
		}
	}
	return notifyType
}

func (n *NotificationManager) sendAppriseViaCLI(cfg AppriseConfig, title, body string) error {
	if len(cfg.Targets) == 0 {
		return FailfWithClass(NotificationFailureConfiguration, "no Apprise targets configured for CLI delivery")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	// Without -b the CLI reads the body from standard input.
	args := []string{"-t", appriseCLITitle(title)}
	args = append(args, cfg.Targets...)

	execFn := n.appriseExec
	if execFn == nil {
		execFn = defaultAppriseExec
	}

	outputBytes, err := execFn(ctx, args, appriseCLIBody(body))
	if err != nil {
		if outputBytes > 0 {
			log.Debug().
				Str("cliPath", "apprise").
				Int("targetCount", len(cfg.Targets)).
				Int64("outputBytes", outputBytes).
				Msg("apprise CLI output (error)")
		}
		return safeAppriseError("execute apprise CLI", err)
	}

	if outputBytes > 0 {
		log.Debug().
			Str("cliPath", "apprise").
			Int("targetCount", len(cfg.Targets)).
			Int64("outputBytes", outputBytes).
			Msg("apprise CLI output")
	}
	return nil
}

func (n *NotificationManager) validatedWebhookBaseURL(rawURL string) (*url.URL, error) {
	baseURL, err := securityutil.NormalizeHTTPBaseURL(rawURL, "")
	if err != nil {
		return nil, err
	}
	if err := n.ValidateWebhookURL(baseURL.String()); err != nil {
		return nil, err
	}
	return baseURL, nil
}

func (n *NotificationManager) sendAppriseViaHTTP(cfg AppriseConfig, title, body, notifyType string) error {
	if cfg.ServerURL == "" {
		return FailfWithClass(NotificationFailureConfiguration, "apprise server URL is not configured")
	}

	serverURL := cfg.ServerURL
	lowerURL := strings.ToLower(serverURL)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		return FailfWithClass(NotificationFailureConfiguration, "apprise server URL must start with http or https")
	}

	validatedBaseURL, err := securityutil.NormalizeHTTPBaseURL(serverURL, "")
	if err == nil {
		err = n.validateWebhookURL(validatedBaseURL.String(), appriseDiagnosticURL)
	}
	if err != nil {
		err = safeAppriseError("apprise server URL validation failed", err)
		log.Error().
			Err(err).
			Msg("apprise server URL validation failed - possible SSRF attempt")
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	notifyEndpoint := "/notify"
	if cfg.ConfigKey != "" {
		notifyEndpoint = "/notify/" + url.PathEscape(cfg.ConfigKey)
	}

	targetURL, err := securityutil.ResolveRelativeURL(validatedBaseURL, notifyEndpoint)
	if err != nil {
		return safeAppriseError("apprise server URL validation failed", err)
	}

	payload := map[string]any{
		"body":  body,
		"title": title,
	}
	if len(cfg.Targets) > 0 {
		payload["urls"] = cfg.Targets
	}
	if notifyType != "" {
		payload["type"] = notifyType
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return FailfWithClass(NotificationFailureConfiguration, "failed to marshal Apprise payload: %w", err)
	}

	req, err := securityutil.NewValidatedRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return safeAppriseError("failed to create Apprise request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if cfg.APIKey != "" {
		if cfg.APIKeyHeader == "" {
			req.Header.Set("X-API-KEY", cfg.APIKey)
		} else {
			req.Header.Set(cfg.APIKeyHeader, cfg.APIKey)
		}
	}

	client := n.createSecureWebhookClientWithURLDiagnostics(
		time.Duration(cfg.TimeoutSeconds)*time.Second,
		validatedBaseURL.Scheme == "https" && cfg.SkipTLSVerify,
		appriseDiagnosticURL,
	)
	// This client owns one delivery, not a reusable manager pool.
	defer client.CloseIdleConnections()

	resp, err := client.Do(req)
	if err != nil {
		return safeAppriseError("failed to reach Apprise server", err)
	}
	defer resp.Body.Close()

	// Share the bounded response verdict with webhooks. Receiving 2xx headers
	// does not make a failed body read successful delivery or a durable receipt.
	result, err := readWebhookResponse(resp)
	if err != nil {
		operation := "read Apprise server response"
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			operation = fmt.Sprintf("apprise server returned HTTP %d", resp.StatusCode)
		}
		return safeAppriseError(operation, err)
	}

	if result.responseBytes > 0 {
		log.Debug().
			Str("mode", string(cfg.Mode)).
			Int("statusCode", resp.StatusCode).
			Int64("responseBytes", result.responseBytes).
			Msg("apprise API response")
	}

	return nil
}

func (n *NotificationManager) sendResolvedApprise(config AppriseConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	cfg := NormalizeAppriseConfig(config)
	if !cfg.Enabled {
		return FailfWithClass(NotificationFailureConfiguration, "apprise not enabled")
	}

	title, _, body := buildResolvedNotificationContent(alertList, resolvedAt, n.publicURL)
	if title == "" && body == "" {
		return fmt.Errorf("failed to build resolved apprise payload")
	}

	switch cfg.Mode {
	case AppriseModeHTTP:
		if err := n.sendAppriseViaHTTP(cfg, title, body, "info"); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Msg("failed to send resolved Apprise notification via API")
			return fmt.Errorf("apprise HTTP send failed: %w", err)
		}
	default:
		if err := n.sendAppriseViaCLI(cfg, title, body); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Int("targetCount", len(cfg.Targets)).
				Msg("failed to send resolved Apprise notification")
			return fmt.Errorf("apprise CLI send failed: %w", err)
		}
	}
	return nil
}
