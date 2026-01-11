package notifications

import (
	"strings"
	"testing"
	"text/template"
)

func TestGetEmailProviders(t *testing.T) {
	providers := GetEmailProviders()

	if len(providers) == 0 {
		t.Fatal("GetEmailProviders() returned empty slice")
	}

	// Track seen names to ensure uniqueness
	seenNames := make(map[string]bool)

	for i, p := range providers {
		t.Run(p.Name, func(t *testing.T) {
			// Name must be non-empty
			if p.Name == "" {
				t.Errorf("Provider %d has empty Name", i)
			}

			// Name must be unique
			if seenNames[p.Name] {
				t.Errorf("Duplicate provider name: %q", p.Name)
			}
			seenNames[p.Name] = true

			// Instructions must be non-empty
			if p.Instructions == "" {
				t.Errorf("Provider %q has empty Instructions", p.Name)
			}

			// Port must be valid (except custom which can be 0)
			if p.SMTPHost != "" && (p.SMTPPort < 0 || p.SMTPPort > 65535) {
				t.Errorf("Provider %q has invalid SMTPPort: %d", p.Name, p.SMTPPort)
			}

			// Non-custom providers should have a host
			if p.Name != "Custom SMTP Server" && p.SMTPHost == "" {
				t.Errorf("Provider %q has empty SMTPHost", p.Name)
			}
		})
	}
}

func TestGetEmailProviders_KnownProviders(t *testing.T) {
	providers := GetEmailProviders()

	// Expected providers that should exist
	expectedProviders := []string{
		"Gmail / Google Workspace",
		"SendGrid",
		"Mailgun",
		"Amazon SES (US East)",
		"Microsoft 365 / Outlook",
		"Custom SMTP Server",
	}

	providerMap := make(map[string]EmailProvider)
	for _, p := range providers {
		providerMap[p.Name] = p
	}

	for _, name := range expectedProviders {
		if _, exists := providerMap[name]; !exists {
			t.Errorf("Expected provider %q not found", name)
		}
	}
}

func TestGetEmailProviders_GmailSettings(t *testing.T) {
	providers := GetEmailProviders()

	var gmail *EmailProvider
	for i := range providers {
		if providers[i].Name == "Gmail / Google Workspace" {
			gmail = &providers[i]
			break
		}
	}

	if gmail == nil {
		t.Fatal("Gmail provider not found")
	}

	if gmail.SMTPHost != "smtp.gmail.com" {
		t.Errorf("Gmail SMTPHost = %q, want smtp.gmail.com", gmail.SMTPHost)
	}
	if gmail.SMTPPort != 587 {
		t.Errorf("Gmail SMTPPort = %d, want 587", gmail.SMTPPort)
	}
	if !gmail.StartTLS {
		t.Error("Gmail StartTLS should be true")
	}
	if !gmail.AuthRequired {
		t.Error("Gmail AuthRequired should be true")
	}
}

func TestGetEmailProviders_SendGridSettings(t *testing.T) {
	providers := GetEmailProviders()

	var sendgrid *EmailProvider
	for i := range providers {
		if providers[i].Name == "SendGrid" {
			sendgrid = &providers[i]
			break
		}
	}

	if sendgrid == nil {
		t.Fatal("SendGrid provider not found")
	}

	if sendgrid.SMTPHost != "smtp.sendgrid.net" {
		t.Errorf("SendGrid SMTPHost = %q, want smtp.sendgrid.net", sendgrid.SMTPHost)
	}
	if sendgrid.SMTPPort != 587 {
		t.Errorf("SendGrid SMTPPort = %d, want 587", sendgrid.SMTPPort)
	}
	// Verify instructions mention apikey username
	if !strings.Contains(sendgrid.Instructions, "apikey") {
		t.Error("SendGrid instructions should mention 'apikey' as username")
	}
}

func TestGetEmailProviders_CustomServerSettings(t *testing.T) {
	providers := GetEmailProviders()

	var custom *EmailProvider
	for i := range providers {
		if providers[i].Name == "Custom SMTP Server" {
			custom = &providers[i]
			break
		}
	}

	if custom == nil {
		t.Fatal("Custom SMTP Server provider not found")
	}

	// Custom server should have empty host (user fills it in)
	if custom.SMTPHost != "" {
		t.Errorf("Custom SMTP Server SMTPHost = %q, want empty", custom.SMTPHost)
	}
}

func TestGetWebhookTemplates(t *testing.T) {
	templates := GetWebhookTemplates()

	if len(templates) == 0 {
		t.Fatal("GetWebhookTemplates() returned empty slice")
	}

	// Track seen services to ensure uniqueness
	seenServices := make(map[string]bool)

	for i, tmpl := range templates {
		t.Run(tmpl.Name, func(t *testing.T) {
			// Service must be non-empty
			if tmpl.Service == "" {
				t.Errorf("Template %d has empty Service", i)
			}

			// Service must be unique
			if seenServices[tmpl.Service] {
				t.Errorf("Duplicate template service: %q", tmpl.Service)
			}
			seenServices[tmpl.Service] = true

			// Name must be non-empty
			if tmpl.Name == "" {
				t.Errorf("Template %q has empty Name", tmpl.Service)
			}

			// Method must be valid HTTP method
			validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
			if !validMethods[tmpl.Method] {
				t.Errorf("Template %q has invalid Method: %q", tmpl.Service, tmpl.Method)
			}

			// Instructions must be non-empty
			if tmpl.Instructions == "" {
				t.Errorf("Template %q has empty Instructions", tmpl.Service)
			}

			// PayloadTemplate must be non-empty
			if tmpl.PayloadTemplate == "" {
				t.Errorf("Template %q has empty PayloadTemplate", tmpl.Service)
			}

			// Headers must include Content-Type
			if _, hasContentType := tmpl.Headers["Content-Type"]; !hasContentType {
				t.Errorf("Template %q missing Content-Type header", tmpl.Service)
			}
		})
	}
}

func TestGetWebhookTemplates_KnownServices(t *testing.T) {
	templates := GetWebhookTemplates()

	// Expected services that should exist
	expectedServices := []string{
		"discord",
		"telegram",
		"slack",
		"teams",
		"pagerduty",
		"generic",
	}

	serviceMap := make(map[string]WebhookTemplate)
	for _, tmpl := range templates {
		serviceMap[tmpl.Service] = tmpl
	}

	for _, service := range expectedServices {
		if _, exists := serviceMap[service]; !exists {
			t.Errorf("Expected service %q not found", service)
		}
	}
}

func TestGetWebhookTemplates_DiscordSettings(t *testing.T) {
	templates := GetWebhookTemplates()

	var discord *WebhookTemplate
	for i := range templates {
		if templates[i].Service == "discord" {
			discord = &templates[i]
			break
		}
	}

	if discord == nil {
		t.Fatal("Discord template not found")
	}

	if !strings.Contains(discord.URLPattern, "discord.com") {
		t.Errorf("Discord URLPattern should contain discord.com, got %q", discord.URLPattern)
	}
	if discord.Method != "POST" {
		t.Errorf("Discord Method = %q, want POST", discord.Method)
	}
	if discord.Headers["Content-Type"] != "application/json" {
		t.Errorf("Discord Content-Type = %q, want application/json", discord.Headers["Content-Type"])
	}
	// Verify payload contains Discord-specific fields
	if !strings.Contains(discord.PayloadTemplate, "embeds") {
		t.Error("Discord PayloadTemplate should contain 'embeds'")
	}
}

func TestGetWebhookTemplates_SlackSettings(t *testing.T) {
	templates := GetWebhookTemplates()

	var slack *WebhookTemplate
	for i := range templates {
		if templates[i].Service == "slack" {
			slack = &templates[i]
			break
		}
	}

	if slack == nil {
		t.Fatal("Slack template not found")
	}

	if !strings.Contains(slack.URLPattern, "hooks.slack.com") {
		t.Errorf("Slack URLPattern should contain hooks.slack.com, got %q", slack.URLPattern)
	}
	if slack.Method != "POST" {
		t.Errorf("Slack Method = %q, want POST", slack.Method)
	}
	// Verify payload contains Slack-specific fields
	if !strings.Contains(slack.PayloadTemplate, "blocks") {
		t.Error("Slack PayloadTemplate should contain 'blocks'")
	}
}

func TestGetWebhookTemplates_TelegramSettings(t *testing.T) {
	templates := GetWebhookTemplates()

	var telegram *WebhookTemplate
	for i := range templates {
		if templates[i].Service == "telegram" {
			telegram = &templates[i]
			break
		}
	}

	if telegram == nil {
		t.Fatal("Telegram template not found")
	}

	if !strings.Contains(telegram.URLPattern, "api.telegram.org") {
		t.Errorf("Telegram URLPattern should contain api.telegram.org, got %q", telegram.URLPattern)
	}
	if telegram.Method != "POST" {
		t.Errorf("Telegram Method = %q, want POST", telegram.Method)
	}
	// Verify payload contains Telegram-specific fields
	if !strings.Contains(telegram.PayloadTemplate, "chat_id") {
		t.Error("Telegram PayloadTemplate should contain 'chat_id'")
	}
	if !strings.Contains(telegram.PayloadTemplate, "parse_mode") {
		t.Error("Telegram PayloadTemplate should contain 'parse_mode'")
	}
}

func TestGetWebhookTemplates_PagerDutySettings(t *testing.T) {
	templates := GetWebhookTemplates()

	var pagerduty *WebhookTemplate
	for i := range templates {
		if templates[i].Service == "pagerduty" {
			pagerduty = &templates[i]
			break
		}
	}

	if pagerduty == nil {
		t.Fatal("PagerDuty template not found")
	}

	if !strings.Contains(pagerduty.URLPattern, "pagerduty.com") {
		t.Errorf("PagerDuty URLPattern should contain pagerduty.com, got %q", pagerduty.URLPattern)
	}
	if pagerduty.Method != "POST" {
		t.Errorf("PagerDuty Method = %q, want POST", pagerduty.Method)
	}
	// PagerDuty requires specific Accept header
	if pagerduty.Headers["Accept"] == "" {
		t.Error("PagerDuty should have Accept header")
	}
	// Verify payload contains PagerDuty-specific fields
	if !strings.Contains(pagerduty.PayloadTemplate, "routing_key") {
		t.Error("PagerDuty PayloadTemplate should contain 'routing_key'")
	}
	if !strings.Contains(pagerduty.PayloadTemplate, "event_action") {
		t.Error("PagerDuty PayloadTemplate should contain 'event_action'")
	}
}

func TestGetWebhookTemplates_GenericSettings(t *testing.T) {
	templates := GetWebhookTemplates()

	var generic *WebhookTemplate
	for i := range templates {
		if templates[i].Service == "generic" {
			generic = &templates[i]
			break
		}
	}

	if generic == nil {
		t.Fatal("Generic template not found")
	}

	// Generic should have empty URL pattern (user fills it in)
	if generic.URLPattern != "" {
		t.Errorf("Generic URLPattern = %q, want empty", generic.URLPattern)
	}
	if generic.Method != "POST" {
		t.Errorf("Generic Method = %q, want POST", generic.Method)
	}
}

func TestGetWebhookTemplates_PayloadTemplatesHaveRequiredFields(t *testing.T) {
	templates := GetWebhookTemplates()

	// All templates should include these alert fields in their payload
	requiredFields := []string{
		"{{.Level}}",
		"{{.ResourceName}}",
		"{{.Node}}",
		"{{.Type}}",
	}

	for _, tmpl := range templates {
		t.Run(tmpl.Service, func(t *testing.T) {
			for _, field := range requiredFields {
				// Check for field or its variations (e.g., {{.Level | title}})
				fieldBase := strings.TrimPrefix(strings.TrimSuffix(field, "}}"), "{{.")
				if !strings.Contains(tmpl.PayloadTemplate, "."+fieldBase) {
					t.Errorf("Template %q PayloadTemplate missing field %s", tmpl.Service, field)
				}
			}
		})
	}
}

func TestEmailProviderConfig_Fields(t *testing.T) {
	cfg := EmailProviderConfig{
		Provider:      "gmail",
		ReplyTo:       "reply@example.com",
		MaxRetries:    3,
		RetryDelay:    5,
		RateLimit:     60,
		StartTLS:      true,
		SkipTLSVerify: false,
		AuthRequired:  true,
	}

	if cfg.Provider != "gmail" {
		t.Errorf("Provider = %q, want gmail", cfg.Provider)
	}
	if cfg.ReplyTo != "reply@example.com" {
		t.Errorf("ReplyTo = %q, want reply@example.com", cfg.ReplyTo)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 5 {
		t.Errorf("RetryDelay = %d, want 5", cfg.RetryDelay)
	}
	if cfg.RateLimit != 60 {
		t.Errorf("RateLimit = %d, want 60", cfg.RateLimit)
	}
	if !cfg.StartTLS {
		t.Error("StartTLS should be true")
	}
	if cfg.SkipTLSVerify {
		t.Error("SkipTLSVerify should be false")
	}
	if !cfg.AuthRequired {
		t.Error("AuthRequired should be true")
	}
}

func TestWebhookTemplate_Fields(t *testing.T) {
	tmpl := WebhookTemplate{
		Service:         "test",
		Name:            "Test Webhook",
		URLPattern:      "https://example.com/webhook",
		Method:          "POST",
		Headers:         map[string]string{"Content-Type": "application/json"},
		PayloadTemplate: `{"message": "test"}`,
		Instructions:    "Test instructions",
	}

	if tmpl.Service != "test" {
		t.Errorf("Service = %q, want test", tmpl.Service)
	}
	if tmpl.Name != "Test Webhook" {
		t.Errorf("Name = %q, want Test Webhook", tmpl.Name)
	}
	if tmpl.URLPattern != "https://example.com/webhook" {
		t.Errorf("URLPattern = %q, want https://example.com/webhook", tmpl.URLPattern)
	}
	if tmpl.Method != "POST" {
		t.Errorf("Method = %q, want POST", tmpl.Method)
	}
	if tmpl.Headers["Content-Type"] != "application/json" {
		t.Errorf("Headers[Content-Type] = %q, want application/json", tmpl.Headers["Content-Type"])
	}
	if tmpl.PayloadTemplate != `{"message": "test"}` {
		t.Errorf("PayloadTemplate mismatch")
	}
	if tmpl.Instructions != "Test instructions" {
		t.Errorf("Instructions = %q, want Test instructions", tmpl.Instructions)
	}
}

func TestEmailProvider_Fields(t *testing.T) {
	provider := EmailProvider{
		Name:         "Test Provider",
		SMTPHost:     "smtp.test.com",
		SMTPPort:     587,
		TLS:          false,
		StartTLS:     true,
		AuthRequired: true,
		Instructions: "Test instructions",
	}

	if provider.Name != "Test Provider" {
		t.Errorf("Name = %q, want Test Provider", provider.Name)
	}
	if provider.SMTPHost != "smtp.test.com" {
		t.Errorf("SMTPHost = %q, want smtp.test.com", provider.SMTPHost)
	}
	if provider.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587", provider.SMTPPort)
	}
	if provider.TLS {
		t.Error("TLS should be false")
	}
	if !provider.StartTLS {
		t.Error("StartTLS should be true")
	}
	if !provider.AuthRequired {
		t.Error("AuthRequired should be true")
	}
}

func TestGetWebhookTemplates_TemplatesAreValid(t *testing.T) {
	templates := GetWebhookTemplates()

	for _, tmpl := range templates {
		t.Run(tmpl.Service, func(t *testing.T) {
			_, err := template.New("test").Funcs(templateFuncMap()).Parse(tmpl.PayloadTemplate)
			if err != nil {
				t.Errorf("Template for service %q is invalid: %v", tmpl.Service, err)
			}
		})
	}
}
