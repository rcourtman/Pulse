package notifications

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	pkgsecurityutil "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/rs/zerolog/log"
)

// createSecureWebhookClient creates an HTTP client with security controls
func (n *NotificationManager) createSecureWebhookClient(timeout time.Duration) *http.Client {
	return n.createSecureWebhookClientWithTLS(timeout, false)
}

// createSecureWebhookClientWithTLS creates a secure HTTP client with optional TLS verification override.
func (n *NotificationManager) createSecureWebhookClientWithTLS(timeout time.Duration, skipTLSVerify bool) *http.Client {
	return n.createSecureWebhookClientWithURLDiagnostics(timeout, skipTLSVerify, RedactWebhookURLSecrets)
}

func (n *NotificationManager) createSecureWebhookClientWithURLDiagnostics(timeout time.Duration, skipTLSVerify bool, diagnosticURL func(string) string) *http.Client {
	// dedicated transport that pins DNS resolution to prevent rebinding
	transport := &http.Transport{
		// Proxy intentionally nil — outbound proxies would bypass DialContext
		// SSRF checks by resolving the target on the proxy side.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Extract hostname and port
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, FailfWithClass(NotificationFailureConfiguration, "parse webhook address %q: %w", addr, err)
			}

			// Validate IP if it's already an IP
			if ip := net.ParseIP(host); ip != nil {
				if ip.IsUnspecified() {
					return nil, FailfWithClass(NotificationFailureConfiguration, "blocked unspecified IP: %s", ip)
				}
				if isPrivateIP(ip) && !n.isIPInAllowlist(ip) {
					return nil, FailfWithClass(NotificationFailureConfiguration, "blocked private IP: %s", ip)
				}
				return pkgsecurityutil.DialPinnedIPs(ctx, network, port, []net.IP{ip}, 10*time.Second)
			}

			// Resolve hostname
			ips, err := n.lookupWebhookHostIPs(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve webhook host %q: %w", host, err)
			}

			// Collect every permitted IP in resolution order
			var permittedIPs []net.IP
			for _, ip := range ips {
				if !isPrivateIP(ip) || n.isIPInAllowlist(ip) {
					permittedIPs = append(permittedIPs, ip)
				}
			}

			if len(permittedIPs) == 0 {
				return nil, FailfWithClass(NotificationFailureConfiguration, "hostname %s resolves to blocked private IPs", host)
			}

			// Log if we filtered some IPs
			if len(ips) > 1 {
				log.Debug().
					Str("host", host).
					Int("resolved", len(ips)).
					Int("permitted", len(permittedIPs)).
					Msg("dns resolution pinned for webhook security")
			}

			return pkgsecurityutil.DialPinnedIPs(ctx, network, port, permittedIPs, 10*time.Second)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if skipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// SSRF admission is not permission to forward private alert bodies,
			// custom authentication headers or signatures to a different receiver.
			if len(via) == 0 || via[0] == nil || !sameNotificationOrigin(via[0].URL, req.URL) {
				return FailWithClass(NotificationFailureConfiguration, errNotificationRedirectOrigin)
			}
			// Origin refusal takes precedence even when the redirect budget is
			// exhausted: a foreign Location must not enter diagnostics or become
			// an unknown/retryable error instead of a local configuration failure.
			if len(via) >= WebhookMaxRedirects {
				return FailfWithClass(NotificationFailureConfiguration, "stopped after %d redirects", WebhookMaxRedirects)
			}
			// Re-validate strictly on an admitted redirect, including DNS/IP
			// changes. Foreign origins are rejected before resolution or logging.
			return n.validateWebhookURLWithContext(req.Context(), req.URL.String(), diagnosticURL)
		},
	}
}

type webhookRenderMode string

const (
	webhookRenderModeSingle   webhookRenderMode = "single"
	webhookRenderModeGrouped  webhookRenderMode = "grouped"
	webhookRenderModeResolved webhookRenderMode = "resolved"
)

func (n *NotificationManager) prepareWebhookDeliveryContext(webhook WebhookConfig, data WebhookPayloadData) (WebhookConfig, WebhookPayloadData, error) {
	renderedURL, renderErr := renderWebhookURL(webhook.URL, data)
	if renderErr != nil {
		return webhook, data, fmt.Errorf("render webhook URL template: %w", renderErr)
	}
	webhook.URL = renderedURL

	switch webhook.Service {
	case "telegram":
		chatID, chatErr := extractTelegramChatID(renderedURL)
		if chatErr != nil {
			return webhook, data, fmt.Errorf("extract Telegram chat_id: %w", chatErr)
		}
		if chatID != "" {
			data.ChatID = chatID
		}
	case "pagerduty":
		if data.CustomFields == nil {
			data.CustomFields = make(map[string]interface{})
		}
		if routingKey, ok := webhook.Headers["routing_key"]; ok {
			data.CustomFields["routing_key"] = routingKey
		}
	}

	// Without a custom template Pulse renders the service's built-in body, so
	// the template's headers describe that body. The stored headers are only a
	// copy the UI seeded at creation, and an edit can blank or replace them;
	// Telegram then ignores the JSON and answers "message text is empty"
	// (#2540). Test sends always applied these headers, so they passed while
	// every real alert failed.
	if strings.TrimSpace(webhook.Template) == "" {
		if tmpl, ok := builtInWebhookServiceTemplate(webhook.Service); ok {
			webhook = withWebhookTemplateHeaders(webhook, tmpl.Headers)
		}
	}

	return webhook, data, nil
}

// builtInWebhookServiceTemplate returns the template Pulse renders for a
// service without a custom template. Generic webhooks send the fallback
// payload instead, so they have none.
func builtInWebhookServiceTemplate(service string) (WebhookTemplate, bool) {
	if service == "" || service == "generic" {
		return WebhookTemplate{}, false
	}
	for _, tmpl := range GetWebhookTemplates() {
		if tmpl.Service == service {
			return tmpl, true
		}
	}
	return WebhookTemplate{}, false
}

// withWebhookTemplateHeaders returns a copy of webhook whose headers carry the
// template's static headers, replacing stored values under any spelling of the
// same name. Templated values are skipped, as executeWebhookRequest skips them.
func withWebhookTemplateHeaders(webhook WebhookConfig, templateHeaders map[string]string) WebhookConfig {
	webhook = copyWebhookConfig(webhook)
	for key, value := range templateHeaders {
		if strings.Contains(value, "{{") {
			continue
		}
		if webhook.Headers == nil {
			webhook.Headers = make(map[string]string, len(templateHeaders))
		}
		for stored := range webhook.Headers {
			if strings.EqualFold(stored, key) {
				delete(webhook.Headers, stored)
			}
		}
		webhook.Headers[key] = value
	}
	return webhook
}

func webhookServiceTemplate(webhook WebhookConfig, mode webhookRenderMode) (string, bool) {
	if webhook.Service == "" || webhook.Service == "generic" {
		return "", false
	}

	for _, tmpl := range GetWebhookTemplates() {
		if tmpl.Service != webhook.Service {
			continue
		}
		switch mode {
		case webhookRenderModeResolved:
			if tmpl.ResolvedPayloadTemplate != "" {
				return tmpl.ResolvedPayloadTemplate, true
			}
		default:
			if tmpl.PayloadTemplate != "" {
				return tmpl.PayloadTemplate, true
			}
		}
		break
	}

	return "", false
}

func (n *NotificationManager) renderWebhookPayloadJSON(webhook WebhookConfig, data WebhookPayloadData, mode webhookRenderMode, fallback func() ([]byte, error)) ([]byte, error) {
	if webhook.Template != "" && strings.TrimSpace(webhook.Template) != "" {
		jsonData, err := n.generatePayloadFromTemplateWithService(webhook.Template, data, webhook.Service)
		if err != nil {
			return nil, fmt.Errorf("generate webhook payload from custom template: %w", err)
		}
		return jsonData, nil
	}

	if templateStr, ok := webhookServiceTemplate(webhook, mode); ok {
		var jsonData []byte
		var err error
		if webhook.Service == "telegram" {
			jsonData, err = n.renderBuiltInTelegram(templateStr, data, mode)
		} else {
			jsonData, err = n.generatePayloadFromTemplateWithService(templateStr, data, webhook.Service)
		}
		if err != nil {
			return nil, fmt.Errorf("generate webhook payload for service %s: %w", webhook.Service, err)
		}
		return jsonData, nil
	}

	return fallback()
}

func withNtfyAlertHeaders(webhook WebhookConfig, alertList []*alerts.Alert) WebhookConfig {
	webhook = copyWebhookConfig(webhook)
	if webhook.Headers == nil {
		webhook.Headers = make(map[string]string)
	}

	level := alerts.AlertLevelInfo
	var primary *alerts.Alert
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		if primary == nil {
			primary = alert
		}
		alertLevel := alerts.NormalizeAlertLevel(alert.Level)
		if alertLevel == alerts.AlertLevelCritical {
			level = alerts.AlertLevelCritical
			break
		}
		if alertLevel == alerts.AlertLevelWarning {
			level = alerts.AlertLevelWarning
		}
	}

	levelLabel := "INFO"
	priority := "default"
	severityTag := "information_source"
	if level == alerts.AlertLevelCritical {
		levelLabel = "CRITICAL"
		priority = "urgent"
		severityTag = "rotating_light"
	} else if level == alerts.AlertLevelWarning {
		levelLabel = "WARNING"
		priority = "high"
		severityTag = "warning"
	}

	titleSubject := fmt.Sprintf("%d alerts", len(alertList))
	typeTag := "grouped"
	if len(alertList) == 1 && primary != nil {
		titleSubject = primary.ResourceName
		if primary.Node != "" {
			titleSubject += " on " + primary.Node
		}
		if primary.Type != "" {
			typeTag = primary.Type
		}
	}

	webhook.Headers["Content-Type"] = "text/plain"
	webhook.Headers["Title"] = webhookHeaderValue(fmt.Sprintf("%s: %s", levelLabel, titleSubject))
	webhook.Headers["Priority"] = priority
	webhook.Headers["Tags"] = fmt.Sprintf("%s,pulse,%s", severityTag, typeTag)
	return webhook
}

// sendGroupedWebhook sends a grouped webhook notification
func groupedWebhookAlertLine(alert *alerts.Alert) string {
	label := alert.ResourceName
	if alert.Node != "" {
		label += " on " + alert.Node
	}
	if alert.Message != "" {
		return fmt.Sprintf("• %s: %s", label, alert.Message)
	}
	return fmt.Sprintf("• %s: %.1f%%", label, alert.Value)
}

func (n *NotificationManager) sendGroupedWebhook(webhook WebhookConfig, alertList []*alerts.Alert) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	// Create a shallow copy of the primary alert to avoid mutating the original memory
	// when we modify the message for grouped summaries.
	originalPrimary := alertList[0]
	alertCopy := *originalPrimary
	primaryAlert := &alertCopy
	usesRenderedAlert := strings.TrimSpace(webhook.Template) != "" || (webhook.Service != "" && webhook.Service != "generic")
	if usesRenderedAlert && len(alertList) > 1 {
		otherAlerts := make([]string, 0, len(alertList)-1)
		for i := 1; i < len(alertList); i++ {
			if alertList[i] == nil {
				continue
			}
			otherAlerts = append(otherAlerts, groupedWebhookAlertLine(alertList[i]))
		}
		if len(otherAlerts) > 0 {
			if webhook.Service == "discord" {
				primaryAlert.Message = fmt.Sprintf("%s | %d alerts: %s", primaryAlert.Message, len(alertList), strings.Join(otherAlerts, ", "))
			} else if strings.TrimSpace(webhook.Template) != "" {
				primaryAlert.Message = fmt.Sprintf("%s\\n\\nAll %d alerts:\\n%s", primaryAlert.Message, len(alertList), strings.Join(otherAlerts, "\\n"))
			} else {
				primaryAlert.Message = fmt.Sprintf("%s\n\nAll %d alerts:\n%s", primaryAlert.Message, len(alertList), strings.Join(otherAlerts, "\n"))
			}
		}
	}

	// Prepare template data only after enriching the primary alert. Preparing it
	// first captured the original message and silently omitted grouped alerts
	// from built-in service payloads.
	customFields := convertWebhookCustomFields(webhook.CustomFields)
	data := n.prepareWebhookData(primaryAlert, customFields)
	data.AlertCount = len(alertList)
	data.Alerts = alertList
	data.AlertURL = firstNonEmpty(alertsPageURL(n.publicURL, alertList, false), data.Instance)
	data.Mention = webhook.Mention
	if webhook.Service == "ntfy" {
		webhook = withNtfyAlertHeaders(webhook, alertList)
	}

	var err error
	webhook, data, err = n.prepareWebhookDeliveryContext(webhook, data)
	if err != nil {
		return fmt.Errorf("prepare grouped webhook context: %w", err)
	}

	jsonData, err := n.renderWebhookPayloadJSON(webhook, data, webhookRenderModeGrouped, func() ([]byte, error) {
		payload := map[string]interface{}{
			"alerts":    alertList,
			"count":     len(alertList),
			"timestamp": time.Now().Unix(),
			"source":    "pulse-monitoring",
			"grouped":   true,
		}
		if link := alertsPageURL(n.publicURL, alertList, false); link != "" {
			payload["alertUrl"] = link
		}
		return json.Marshal(payload)
	})
	if err != nil {
		return fmt.Errorf("render grouped webhook payload: %w", err)
	}

	// Send using same request logic
	return n.sendWebhookRequest(webhook, jsonData, "grouped", webhookEventID(data.ID, data.Event))
}

func (n *NotificationManager) sendResolvedWebhook(webhook WebhookConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	if !webhook.Enabled {
		return FailfWithClass(NotificationFailureConfiguration, "webhook is disabled")
	}

	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}

	// ntfy needs plain-text body + headers, not JSON
	if webhook.Service == "ntfy" {
		return n.sendResolvedWebhookNtfy(webhook, alertList, resolvedAt)
	}

	// Use the first non-nil alert for template data (most common case is single-alert recovery)
	var alert *alerts.Alert
	for _, a := range alertList {
		if a != nil {
			alert = a
			break
		}
	}
	if alert == nil {
		return fmt.Errorf("all alerts in resolved list are nil")
	}

	// Prepare template data using the same pipeline as the firing path
	customFields := convertWebhookCustomFields(webhook.CustomFields)
	pulseBase := n.GetPublicURL()
	data := n.prepareWebhookDataAtPublicURL(alert, customFields, pulseBase)

	// Carry the configured mention into the resolved payload too, matching the
	// firing/grouped paths so a configured @everyone/@channel is not dropped on
	// resolved notifications (#1118).
	data.Mention = webhook.Mention

	// Override fields for resolved context
	data.Event = "resolved"
	data.ResolvedAt = resolvedAt.Format(time.RFC3339)
	data.ResolvedAtISO = resolvedAt.Format(time.RFC3339)
	data.Duration = formatWebhookDuration(resolvedAt.Sub(alert.StartTime))
	data.Message = resolvedAlertMessage(alert)
	data.NotRecovered = resolvedAlertsIncludeNotRecovered(alertList)
	data.AlertCount = len(alertList)
	data.Alerts = alertList
	pulseAlertURL := alertsPageURL(pulseBase, alertList, true)
	data.AlertURL = firstNonEmpty(pulseAlertURL, data.Instance)
	if len(alertList) > 1 {
		names := make([]string, 0, len(alertList))
		for _, a := range alertList {
			if a != nil {
				names = append(names, resolvedAlertListEntry(a))
			}
		}
		data.Message = fmt.Sprintf("%d alerts resolved: %s", len(alertList), strings.Join(names, "; "))
	}

	var err error
	webhook, data, err = n.prepareWebhookDeliveryContext(webhook, data)
	if err != nil {
		return fmt.Errorf("prepare resolved webhook context: %w", err)
	}
	if usesBuiltInTelegramBody(webhook) {
		// Link only to the Pulse base, never a monitored instance. Keep
		// the existing template data for recipient selection and custom bodies.
		data.AlertURL = ""
		base, parseErr := url.Parse(pulseBase)
		if parseErr == nil && (base.Scheme == "http" || base.Scheme == "https") && base.Hostname() != "" &&
			base.User == nil && base.RawQuery == "" && !base.ForceQuery && !strings.Contains(pulseBase, "#") {
			data.AlertURL = pulseAlertURL
		}
	}

	jsonData, err := n.renderWebhookPayloadJSON(webhook, data, webhookRenderModeResolved, func() ([]byte, error) {
		payload := map[string]interface{}{
			"event":         string(eventResolved),
			"alerts":        alertList,
			"count":         len(alertList),
			"resolvedAt":    resolvedAt.Unix(),
			"resolvedAtIso": resolvedAt.Format(time.RFC3339),
			"source":        "pulse-monitoring",
		}

		if pulseBase != "" {
			payload["dashboard"] = pulseBase
		}

		if len(alertList) == 1 {
			payload["alertIdentifier"] = alert.ID
		}
		if pulseAlertURL != "" {
			payload["alertUrl"] = pulseAlertURL
		}
		return json.Marshal(payload)
	})
	if err != nil {
		return fmt.Errorf("render resolved webhook payload: %w", err)
	}

	return n.sendWebhookRequest(webhook, jsonData, "resolved", webhookEventID(data.ID, data.Event))
}

// sendResolvedWebhookNtfy sends a resolved webhook formatted for ntfy (plain text + headers)
func (n *NotificationManager) sendResolvedWebhookNtfy(webhook WebhookConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	// Re-validate webhook URL
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	if !n.checkWebhookRateLimit(webhook.URL) {
		return FailfWithClass(NotificationFailureRateLimited, "rate limit exceeded for webhook %s", webhook.Name)
	}

	// Build plain-text body and title. A single alert that moved rather than
	// recovered says so in both, and no batch holding such a close carries
	// the green check.
	var body strings.Builder
	title := "RESOLVED"
	tags := "white_check_mark,pulse,resolved"
	if resolvedAlertsIncludeNotRecovered(alertList) {
		tags = "arrow_right,pulse,resolved"
	}
	if len(alertList) == 1 && alertList[0] != nil {
		a := alertList[0]
		title = fmt.Sprintf("RESOLVED: %s", a.ResourceName)
		if summary := resolvedAlertNotRecoveredSummary(a); summary != "" {
			title = fmt.Sprintf("MOVED: %s", a.ResourceName)
			body.WriteString(summary)
		} else {
			fmt.Fprintf(&body, "Resolved: %s", resolvedAlertMessage(a))
		}
	} else {
		title = fmt.Sprintf("RESOLVED: %d alerts", len(alertList))
		fmt.Fprintf(&body, "%d alerts resolved at %s:\n", len(alertList), resolvedAt.Format(time.RFC822))
		for _, a := range alertList {
			if a != nil {
				fmt.Fprintf(&body, "- %s\n", resolvedAlertListEntry(a))
			}
		}
	}

	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	payload := []byte(body.String())
	req, err := http.NewRequest(method, webhook.URL, bytes.NewReader(payload))
	if err != nil {
		return FailfWithClass(NotificationFailureConfiguration, "failed to create ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Title", webhookHeaderValue(title))
	req.Header.Set("Priority", "default")
	req.Header.Set("Tags", tags)
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")

	// Apply any custom headers from webhook config
	for key, value := range webhook.Headers {
		if !strings.Contains(value, "{{") {
			req.Header.Set(key, value)
		}
	}

	// Recovery follows the same signing contract as firing, over the exact
	// plain-text bytes ntfy receives. Custom headers cannot replace integrity.
	eventID := ""
	for _, alert := range alertList {
		if alert != nil {
			eventID = webhookEventID(alert.ID, "resolved")
			break
		}
	}
	setWebhookIntegrityHeaders(req, webhook.SigningSecret, eventID, payload)

	resp, err := n.webhookClient.Do(req)
	if err != nil {
		err = redactWebhookTransportError(err)
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Msg("failed to send resolved ntfy webhook")
		return fmt.Errorf("failed to send ntfy webhook: %w", err)
	}
	defer resp.Body.Close()

	result, err := readWebhookResponse(resp)
	if err == nil {
		log.Info().
			Str("webhook", webhook.Name).
			Str("service", "ntfy").
			Str("type", "resolved").
			Int("status", resp.StatusCode).
			Int("alertCount", len(alertList)).
			Msg("resolved ntfy webhook sent successfully")
		return nil
	}

	log.Warn().
		Err(err).
		Str("webhook", webhook.Name).
		Str("service", "ntfy").
		Int("status", resp.StatusCode).
		Int64("responseBytes", result.responseBytes).
		Bool("responseLimitReached", result.responseLimitReached).
		Bool("responseIncomplete", result.responseIncomplete).
		Msg("resolved ntfy webhook response did not complete successfully")
	return fmt.Errorf("ntfy %w", err)
}

// checkWebhookRateLimit checks if a webhook can be sent based on rate limits
func (n *NotificationManager) checkWebhookRateLimit(webhookURL string) bool {
	return n.takeWebhookRateSlot(webhookURL).IsZero()
}

// takeWebhookRateSlot reports the reset time when no slot was available.
// A zero time means one slot was consumed.
func (n *NotificationManager) takeWebhookRateSlot(webhookURL string) time.Time {
	n.webhookRateMu.Lock()
	defer n.webhookRateMu.Unlock()

	now := time.Now()
	n.cleanupWebhookRateLimitsLocked(now)
	limit, exists := n.webhookRateLimits[webhookURL]

	if !exists {
		// First time sending to this webhook
		n.webhookRateLimits[webhookURL] = &webhookRateLimit{
			lastSent:  now,
			sentCount: 1,
		}
		return time.Time{}
	}

	// Check if we're still in the rate limit window
	if now.Sub(limit.lastSent) > WebhookRateLimitWindow {
		// Window expired, reset counter
		limit.lastSent = now
		limit.sentCount = 1
		return time.Time{}
	}

	// Still in window, check if we've exceeded the limit
	if limit.sentCount >= WebhookRateLimitMax {
		log.Warn().
			Str("webhookURL", RedactWebhookURLSecrets(webhookURL)).
			Int("sentCount", limit.sentCount).
			Dur("window", WebhookRateLimitWindow).
			Msg("webhook rate limit exceeded")
		return limit.lastSent.Add(WebhookRateLimitWindow)
	}

	// Increment counter and allow
	limit.sentCount++
	return time.Time{}
}

func (n *NotificationManager) cleanupWebhookRateLimitsLocked(now time.Time) {
	if now.Sub(n.webhookRateCleanup) < WebhookRateLimitWindow {
		return
	}

	cutoff := now.Add(-WebhookRateLimitWindow)
	cleaned := 0
	for webhookURL, limit := range n.webhookRateLimits {
		if limit.lastSent.Before(cutoff) {
			delete(n.webhookRateLimits, webhookURL)
			cleaned++
		}
	}
	n.webhookRateCleanup = now

	if cleaned > 0 {
		log.Debug().
			Int("cleaned", cleaned).
			Int("remaining", len(n.webhookRateLimits)).
			Msg("Cleaned up stale webhook rate limit entries")
	}
}

// sendWebhookRequest sends the actual webhook request
func normalizeWebhookRequestURL(webhook WebhookConfig) string {
	// For Telegram webhooks, strip chat_id from URL if present.
	// The chat_id belongs in the payload, not in the request URL.
	if webhook.Service == "telegram" && strings.Contains(webhook.URL, "chat_id=") {
		if u, err := url.Parse(webhook.URL); err == nil {
			q := u.Query()
			q.Del("chat_id")
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	return webhook.URL
}

func (n *NotificationManager) executeWebhookRequest(webhook WebhookConfig, payload []byte, opts webhookRequestOptions) (*webhookHTTPResult, error) {
	if opts.validateURL {
		if err := n.ValidateWebhookURL(webhook.URL); err != nil {
			return nil, fmt.Errorf("webhook URL validation failed: %w", err)
		}
	}

	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	targetURL, err := securityutil.NormalizeAbsoluteHTTPURL(normalizeWebhookRequestURL(webhook))
	if err != nil {
		return nil, FailfWithClass(NotificationFailureConfiguration, "failed to normalize webhook request URL: %w", err)
	}

	req, err := securityutil.NewValidatedRequestWithContext(context.Background(), method, targetURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, FailfWithClass(NotificationFailureConfiguration, "failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", opts.userAgent)
	if webhook.Service == "ntfy" {
		req.Header.Set("Content-Type", "text/plain")
	}

	for key, value := range webhook.Headers {
		if strings.Contains(value, "{{") {
			continue
		}
		req.Header.Set(key, value)
	}

	setWebhookIntegrityHeaders(req, webhook.SigningSecret, opts.eventID, payload)

	client := n.webhookClient
	if client == nil || (opts.timeout > 0 && opts.timeout != WebhookTimeout) {
		client = n.createSecureWebhookClient(opts.timeout)
		// Release only the request-owned transport; ordinary sends reuse n.webhookClient.
		defer client.CloseIdleConnections()
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send webhook: %w", redactWebhookTransportError(err))
	}
	defer resp.Body.Close()

	result, err := readWebhookResponse(resp)
	if result.responseLimitReached {
		log.Warn().
			Str("webhook", webhook.Name).
			Int64("responseBytes", result.responseBytes).
			Int("maxSize", WebhookMaxResponseSize).
			Msg("webhook response read limit reached; body withheld")
	}

	if opts.responseLogging || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().
			Str("webhook", webhook.Name).
			Int("status", resp.StatusCode).
			Int64("responseBytes", result.responseBytes).
			Bool("responseLimitReached", result.responseLimitReached).
			Bool("responseIncomplete", result.responseIncomplete).
			Msg("webhook response")
	}
	return result, err
}

func (n *NotificationManager) sendWebhookRequest(webhook WebhookConfig, jsonData []byte, alertType string, eventID string) error {
	// Re-validate webhook URL to prevent DNS rebinding attacks
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("url", RedactWebhookURLSecrets(webhook.URL)).
			Msg("webhook URL validation failed at send time - possible DNS rebinding")
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Check rate limit before sending
	if retryAt := n.takeWebhookRateSlot(webhook.URL); !retryAt.IsZero() {
		if usesBuiltInTelegramBody(webhook) {
			return &telegramRateHold{retryAt: retryAt}
		}
		log.Warn().
			Str("webhook", webhook.Name).
			Str("url", RedactWebhookURLSecrets(webhook.URL)).
			Msg("Webhook request dropped due to rate limiting")
		return FailfWithClass(NotificationFailureRateLimited, "rate limit exceeded for webhook %s", webhook.Name)
	}

	result, err := n.executeWebhookRequest(webhook, jsonData, webhookRequestOptions{
		alertType:       alertType,
		eventID:         eventID,
		timeout:         WebhookTimeout,
		userAgent:       "Pulse-Monitoring/2.0",
		responseLogging: false,
		validateURL:     false,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("failed to send webhook")
		return err
	}

	if result.statusCode >= 200 && result.statusCode < 300 {
		log.Info().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", result.statusCode).
			Int("payloadSize", len(jsonData)).
			Msg("webhook notification sent successfully")

		if result.responseBytes > 0 {
			log.Debug().
				Str("webhook", webhook.Name).
				Int64("responseBytes", result.responseBytes).
				Bool("responseLimitReached", result.responseLimitReached).
				Msg("webhook response body withheld")
		}
		return nil
	} else {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", result.statusCode).
			Msg("webhook returned non-success status")
		return FailfWithClass(ClassFromHTTPStatus(result.statusCode), "webhook returned HTTP %d (response body withheld)", result.statusCode)
	}
}

func convertWebhookCustomFields(fields map[string]string) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}

	converted := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		converted[key] = value
	}
	return converted
}

func isEmptyInterface(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case fmt.Stringer:
		return strings.TrimSpace(v.String()) == ""
	case nil:
		return true
	default:
		return false
	}
}

// setWebhookIntegrityHeaders applies the shared delivery contract after all
// custom headers. Both JSON webhooks and ntfy's plain-text recovery sign the
// actual wire payload, without changing empty-secret (unsigned) behavior.
func setWebhookIntegrityHeaders(req *http.Request, secret, eventID string, payload []byte) {
	if eventID != "" {
		req.Header.Set("X-Pulse-Event-ID", webhookHeaderValue(eventID))
	}
	if secret = strings.TrimSpace(secret); secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("X-Pulse-Timestamp", timestamp)
		req.Header.Set("X-Pulse-Signature", "v1="+signWebhookPayload(secret, timestamp, payload))
	}
}

// maxWebhookHeaderValueBytes caps alert-derived header values so an oversized
// agent-reported identifier cannot exceed HTTP receiver header line limits.
const maxWebhookHeaderValueBytes = 2048

// webhookHeaderValue makes a value built from alert data safe to send as a
// request header. Alert IDs and resource names carry identifiers reported by
// agents and monitored systems, and net/http refuses a whole request when a
// header value holds a control byte other than tab, so one such alert would
// dead-letter every alert grouped with it. Those bytes are percent-encoded,
// which stays stable across retries, and oversized values are deterministically
// shortened with a SHA-256 suffix.
func webhookHeaderValue(value string) string {
	var safe strings.Builder
	for i := 0; i < len(value); i++ {
		if c := value[i]; (c < 0x20 && c != '\t') || c == 0x7f {
			fmt.Fprintf(&safe, "%%%02X", c)
		} else {
			safe.WriteByte(c)
		}
	}
	encoded := safe.String()
	if len(encoded) <= maxWebhookHeaderValueBytes {
		return encoded
	}
	sum := sha256.Sum256([]byte(encoded))
	suffix := ":" + hex.EncodeToString(sum[:8])
	prefixLen := maxWebhookHeaderValueBytes - len(suffix)
	return strings.ToValidUTF8(encoded[:prefixLen], "") + suffix
}

// signWebhookPayload computes the hex HMAC-SHA256 of "timestamp.body" with
// the webhook's signing secret. Binding the timestamp into the MAC lets
// receivers reject replayed deliveries.
func signWebhookPayload(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// webhookEventID builds the legacy correlation header, stable across retries.
// It omits occurrence start, severity and grouped membership, so it is not a
// unique incident/delivery identity or a sufficient durable deduplication key.
func webhookEventID(alertID, event string) string {
	if alertID == "" {
		return ""
	}
	if event == "" {
		event = "alert"
	}
	return alertID + ":" + event
}

// prepareWebhookData prepares data for template rendering
func (n *NotificationManager) prepareWebhookData(alert *alerts.Alert, customFields map[string]interface{}) WebhookPayloadData {
	return n.prepareWebhookDataAtPublicURL(alert, customFields, n.GetPublicURL())
}

func (n *NotificationManager) prepareWebhookDataAtPublicURL(alert *alerts.Alert, customFields map[string]interface{}, publicURL string) WebhookPayloadData {
	duration := time.Since(alert.StartTime)

	// Construct full Pulse URL if publicURL is configured
	// The Instance field should contain the full URL to the Pulse dashboard
	instance := ""
	if publicURL != "" {
		// Remove trailing slash from publicURL if present
		instance = strings.TrimRight(publicURL, "/")
	} else if alert.Instance != "" && (strings.HasPrefix(alert.Instance, "http://") || strings.HasPrefix(alert.Instance, "https://")) {
		// If publicURL is not set but alert.Instance contains a full URL, use it
		instance = alert.Instance
	}

	resourceType := ""
	messageKey := strings.TrimSpace(alert.Type)
	if alert.Metadata != nil {
		if rt, ok := alert.Metadata["resourceType"].(string); ok {
			resourceType = rt
		}
		if kind, ok := alert.Metadata["canonicalAlertKind"].(string); ok && strings.TrimSpace(kind) != "" {
			messageKey = strings.TrimSpace(kind)
			if alertType := strings.TrimSpace(alert.Type); alertType != "" {
				messageKey += "." + alertType
			}
		}
	}

	var metadataCopy map[string]interface{}
	if alert.Metadata != nil {
		metadataCopy = make(map[string]interface{}, len(alert.Metadata))
		for k, v := range alert.Metadata {
			metadataCopy[k] = v
		}
	}

	var ackTime string
	if alert.AckTime != nil {
		ackTime = alert.AckTime.Format(time.RFC3339)
	}

	// Round Value and Threshold to 1 decimal place for cleaner webhook payloads
	roundedValue := math.Round(alert.Value*10) / 10
	roundedThreshold := math.Round(alert.Threshold*10) / 10

	tenantID, tenantName := n.TenantIdentity()

	return WebhookPayloadData{
		ID:                 alert.ID,
		MessageKey:         messageKey,
		Level:              string(alert.Level),
		Type:               alert.Type,
		ResourceName:       alert.ResourceName,
		ResourceID:         alert.ResourceID,
		Node:               alert.Node,
		NodeDisplayName:    alertNodeDisplay(alert),
		Instance:           instance,
		AlertURL:           firstNonEmpty(alertLinkURL(publicURL, alert), instance),
		Message:            alert.Message,
		Value:              roundedValue,
		Threshold:          roundedThreshold,
		ValueFormatted:     formatMetricValue(alert.Type, alert.Value),
		ThresholdFormatted: formatMetricThreshold(alert.Type, alert.Threshold),
		StartTime:          alert.StartTime.Format(time.RFC3339),
		Duration:           formatWebhookDuration(duration),
		Timestamp:          time.Now().Format(time.RFC3339),
		ResourceType:       resourceType,
		Acknowledged:       alert.Acknowledged,
		AckTime:            ackTime,
		AckUser:            alert.AckUser,
		Event:              "alert",
		TenantID:           tenantID,
		TenantName:         tenantName,
		Metadata:           metadataCopy,
		CustomFields:       customFields,
		AlertCount:         1,
	}
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		},
		"jsonString": func(v interface{}) string {
			encoded, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			if len(encoded) >= 2 && encoded[0] == '"' && encoded[len(encoded)-1] == '"' {
				return string(encoded[1 : len(encoded)-1])
			}
			return string(encoded)
		},
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"printf":    fmt.Sprintf,
		"urlquery":  template.URLQueryEscaper,
		"urlencode": template.URLQueryEscaper,
		"urlpath":   url.PathEscape,
		"pathescape": func(s string) string {
			return url.PathEscape(s)
		},
	}
}

// generatePayloadFromTemplateWithService renders the payload using Go templates with service-specific handling
func (n *NotificationManager) generatePayloadFromTemplateWithService(templateStr string, data WebhookPayloadData, service string) ([]byte, error) {
	tmpl, err := webhookPayloadTemplate(templateStr)
	if err != nil {
		return nil, failNotificationConfiguration("invalid template", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, failNotificationConfiguration("template execution failed", err)
	}

	// Skip JSON validation for services that use plain text payloads
	if service == "ntfy" {
		// ntfy uses plain text, not JSON
		return buf.Bytes(), nil
	}

	// Validate that the generated payload is valid JSON for other services
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		log.Error().
			Int("payloadBytes", buf.Len()).
			Msg("generated webhook payload is invalid JSON")
		return nil, failNotificationConfiguration("template produced invalid JSON", err)
	}

	return buf.Bytes(), nil
}

// renderWebhookURL applies template rendering to webhook URLs and ensures the result is a valid URL
func renderWebhookURL(urlTemplate string, data WebhookPayloadData) (string, error) {
	trimmed := strings.TrimSpace(urlTemplate)
	if trimmed == "" {
		return "", FailfWithClass(NotificationFailureConfiguration, "webhook URL cannot be empty")
	}

	if !strings.Contains(trimmed, "{{") {
		return trimmed, nil
	}

	tmpl, err := template.New("webhook_url").Funcs(templateFuncMap()).Parse(trimmed)
	if err != nil {
		return "", FailfWithClass(NotificationFailureConfiguration, "invalid webhook URL template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", FailfWithClass(NotificationFailureConfiguration, "webhook URL template execution failed: %w", err)
	}

	rendered := strings.TrimSpace(buf.String())
	if rendered == "" {
		return "", FailfWithClass(NotificationFailureConfiguration, "webhook URL template produced empty URL")
	}

	parsed, err := url.Parse(rendered)
	if err != nil {
		return "", FailfWithClass(NotificationFailureConfiguration, "webhook URL template produced invalid URL: %w", err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", FailfWithClass(NotificationFailureConfiguration, "webhook URL template produced invalid URL: missing scheme or host")
	}

	return parsed.String(), nil
}

// formatWebhookDuration formats a duration in a human-readable way
func formatWebhookDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// extractTelegramChatID extracts and validates the chat_id from a Telegram webhook URL
func extractTelegramChatID(webhookURL string) (string, error) {
	if !strings.Contains(webhookURL, "chat_id=") {
		return "", FailfWithClass(NotificationFailureConfiguration, "telegram webhook URL missing chat_id parameter")
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return "", FailfWithClass(NotificationFailureConfiguration, "invalid URL format: %w", err)
	}

	chatID := u.Query().Get("chat_id")
	if chatID == "" {
		return "", FailfWithClass(NotificationFailureConfiguration, "chat_id parameter is empty")
	}

	// Validate that chat_id is numeric (Telegram chat IDs are always numeric)
	// Handle negative IDs (group chats) and positive IDs (private chats)
	if strings.HasPrefix(chatID, "-") {
		if !isNumeric(chatID[1:]) {
			return "", FailfWithClass(NotificationFailureConfiguration, "chat_id must be numeric, got: %s", chatID)
		}
	} else if !isNumeric(chatID) {
		return "", FailfWithClass(NotificationFailureConfiguration, "chat_id must be numeric, got: %s", chatID)
	}

	return chatID, nil
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(s) > 0
}

// addWebhookDelivery adds a webhook delivery record to the history
func (n *NotificationManager) addWebhookDelivery(delivery WebhookDelivery) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Add to history
	n.webhookHistory = append(n.webhookHistory, delivery)

	// Keep only last 100 entries
	if len(n.webhookHistory) > 100 {
		// Remove oldest entry
		n.webhookHistory = n.webhookHistory[1:]
	}
}

// GetWebhookHistory returns recent webhook delivery history
func (n *NotificationManager) GetWebhookHistory() []WebhookDelivery {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	history := make([]WebhookDelivery, len(n.webhookHistory))
	copy(history, n.webhookHistory)
	return history
}
