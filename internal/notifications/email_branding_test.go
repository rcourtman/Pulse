package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestEmailBrandingDefaultMatchesExistingTemplate(t *testing.T) {
	alert := &alerts.Alert{
		ID:           "a1",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "pve1-vm-101",
		ResourceName: "web-01",
		Node:         "pve1",
		Instance:     "prod",
		Message:      "CPU above threshold",
		Value:        95.4,
		Threshold:    90,
		StartTime:    time.Now().Add(-5 * time.Minute),
	}

	defaultSubj, defaultHTML, defaultText := EmailTemplate([]*alerts.Alert{alert}, true, "https://pulse.example.com")
	emptySubj, emptyHTML, emptyText := EmailTemplateWithBranding([]*alerts.Alert{alert}, true, "https://pulse.example.com", EmailBranding{})

	if defaultSubj != emptySubj {
		t.Fatalf("single subject drift: got %q want %q", emptySubj, defaultSubj)
	}
	if defaultHTML != emptyHTML {
		t.Fatalf("single HTML drift between default and empty branding")
	}
	if defaultText != emptyText {
		t.Fatalf("single text drift between default and empty branding")
	}

	group := []*alerts.Alert{
		alert,
		{
			ID:           "a2",
			Type:         "memory",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "pve1-vm-102",
			ResourceName: "db-01",
			Node:         "pve1",
			Instance:     "prod",
			Message:      "Memory above threshold",
			Value:        88.2,
			Threshold:    85,
			StartTime:    time.Now().Add(-2 * time.Minute),
		},
	}

	gDefaultSubj, gDefaultHTML, gDefaultText := EmailTemplate(group, false, "https://pulse.example.com")
	gEmptySubj, gEmptyHTML, gEmptyText := EmailTemplateWithBranding(group, false, "https://pulse.example.com", EmailBranding{})

	if gDefaultSubj != gEmptySubj {
		t.Fatalf("grouped subject drift: got %q want %q", gEmptySubj, gDefaultSubj)
	}
	if gDefaultHTML != gEmptyHTML {
		t.Fatalf("grouped HTML drift between default and empty branding")
	}
	if gDefaultText != gEmptyText {
		t.Fatalf("grouped text drift between default and empty branding")
	}
}

func TestEmailBrandingAppliesCustomSubjectBrandAndFooter(t *testing.T) {
	branding := EmailBranding{
		SubjectPrefix: "[Acme Ops]",
		BrandName:     "Acme Infrastructure",
		FooterNote:    "On-call runbook: https://wiki.acme.invalid/alerts",
	}

	alert := &alerts.Alert{
		ID:           "a1",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "pve1-vm-101",
		ResourceName: "web-01",
		Node:         "pve1",
		Instance:     "prod",
		Message:      "CPU above threshold",
		Value:        95.4,
		Threshold:    90,
		StartTime:    time.Now().Add(-5 * time.Minute),
	}

	subject, htmlBody, textBody := EmailTemplateWithBranding([]*alerts.Alert{alert}, true, "", branding)
	if !strings.HasPrefix(subject, "[Acme Ops] Critical: CPU on web-01 (pve1)") {
		t.Fatalf("unexpected branded single subject: %q", subject)
	}
	if !strings.Contains(htmlBody, "<h1>Acme Infrastructure Alert</h1>") {
		t.Fatalf("expected branded HTML heading in single email")
	}
	if !strings.Contains(htmlBody, "<p>This is an automated notification from Acme Infrastructure</p>") {
		t.Fatalf("expected branded HTML footer attribution in single email")
	}
	if !strings.Contains(htmlBody, "<p>On-call runbook: https://wiki.acme.invalid/alerts</p>") {
		t.Fatalf("expected custom HTML footer note in single email")
	}
	if !strings.HasPrefix(textBody, "ACME INFRASTRUCTURE ALERT\n") {
		t.Fatalf("expected uppercase branded text heading, got prefix %q", textBody[:40])
	}
	if !strings.Contains(textBody, "This is an automated notification from Acme Infrastructure.") {
		t.Fatalf("expected branded text footer attribution")
	}
	if !strings.Contains(textBody, "On-call runbook: https://wiki.acme.invalid/alerts") {
		t.Fatalf("expected custom text footer note")
	}

	groupedSubj, groupedHTML, groupedText := EmailTemplateWithBranding([]*alerts.Alert{alert, alert}, false, "", branding)
	if !strings.HasPrefix(groupedSubj, "[Acme Ops] 2 Critical alerts:") {
		t.Fatalf("unexpected branded grouped subject: %q", groupedSubj)
	}
	if !strings.Contains(groupedHTML, "<h1>Acme Infrastructure Alert Summary</h1>") {
		t.Fatalf("expected branded HTML heading in grouped email")
	}
	if !strings.Contains(groupedHTML, "<p>This is an automated notification from Acme Infrastructure</p>") {
		t.Fatalf("expected branded HTML footer attribution in grouped email")
	}
	if !strings.Contains(groupedHTML, "<p>On-call runbook: https://wiki.acme.invalid/alerts</p>") {
		t.Fatalf("expected custom HTML footer note in grouped email")
	}
	if !strings.HasPrefix(groupedText, "ACME INFRASTRUCTURE ALERT SUMMARY\n\n") {
		t.Fatalf("expected uppercase branded grouped text heading")
	}
	if !strings.Contains(groupedText, "On-call runbook: https://wiki.acme.invalid/alerts") {
		t.Fatalf("expected custom grouped text footer note")
	}
}

func TestEmailBrandingSanitizesAndEscapesInput(t *testing.T) {
	branding := NormalizeEmailBranding(EmailBranding{
		SubjectPrefix: "  [Acme]\r\nBcc: evil@example.com  ",
		BrandName:     "Acme <script>alert(1)</script> & Co\n\tOps",
		FooterNote:    "<b>Contact</b> ops@acme.invalid\r\nEscalation desk",
	})

	if strings.ContainsAny(branding.SubjectPrefix, "\r\n") {
		t.Fatalf("subject prefix retained line breaks: %q", branding.SubjectPrefix)
	}
	if branding.SubjectPrefix != "[Acme] Bcc: evil@example.com" {
		t.Fatalf("unexpected sanitized subject prefix: %q", branding.SubjectPrefix)
	}
	if strings.ContainsAny(branding.BrandName, "\r\n\t") {
		t.Fatalf("brand name retained control chars: %q", branding.BrandName)
	}
	if strings.ContainsAny(branding.FooterNote, "\r\n") {
		t.Fatalf("footer note retained line breaks: %q", branding.FooterNote)
	}

	longPrefix := strings.Repeat("A", MaxEmailSubjectPrefixRunes+25)
	longBrand := strings.Repeat("B", MaxEmailBrandNameRunes+25)
	longFooter := strings.Repeat("C", MaxEmailFooterNoteRunes+50)
	bounded := NormalizeEmailBranding(EmailBranding{
		SubjectPrefix: longPrefix,
		BrandName:     longBrand,
		FooterNote:    longFooter,
	})
	if len([]rune(bounded.SubjectPrefix)) != MaxEmailSubjectPrefixRunes {
		t.Fatalf("subject prefix rune length = %d, want %d", len([]rune(bounded.SubjectPrefix)), MaxEmailSubjectPrefixRunes)
	}
	if len([]rune(bounded.BrandName)) != MaxEmailBrandNameRunes {
		t.Fatalf("brand name rune length = %d, want %d", len([]rune(bounded.BrandName)), MaxEmailBrandNameRunes)
	}
	if len([]rune(bounded.FooterNote)) != MaxEmailFooterNoteRunes {
		t.Fatalf("footer note rune length = %d, want %d", len([]rune(bounded.FooterNote)), MaxEmailFooterNoteRunes)
	}

	alert := &alerts.Alert{
		ID:           "a1",
		Type:         "cpu",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Node:         "node-1",
		Message:      "High CPU",
		Value:        85,
		Threshold:    80,
		StartTime:    time.Now().Add(-time.Minute),
	}

	_, htmlBody, _ := EmailTemplateWithBranding([]*alerts.Alert{alert}, true, "", branding)
	if strings.Contains(htmlBody, "<script>") || strings.Contains(htmlBody, "<b>Contact</b>") {
		t.Fatalf("raw HTML was not escaped in rendered email body")
	}
	if !strings.Contains(htmlBody, "&lt;script&gt;alert(1)&lt;/script&gt; &amp; Co Ops") {
		t.Fatalf("expected escaped brand name in HTML body")
	}
	if !strings.Contains(htmlBody, "&lt;b&gt;Contact&lt;/b&gt; ops@acme.invalid Escalation desk") {
		t.Fatalf("expected escaped footer note in HTML body")
	}
}

func TestResolvedEmailSubjectWithBranding(t *testing.T) {
	single := []*alerts.Alert{{
		ID:           "a1",
		ResourceName: "web-01",
		Node:         "pve1",
	}}
	moved := []*alerts.Alert{{
		ID:           "a2",
		ResourceName: "web-02",
		Node:         "pve1",
		Resolution:   &alerts.AlertResolution{Reason: alerts.AlertResolutionMovedToAgent},
	}}
	multi := []*alerts.Alert{
		single[0],
		{ID: "a3", ResourceName: "db-01", Node: "pve2"},
	}

	if got := resolvedEmailSubjectWithBranding(single, EmailBranding{}); got != "Pulse alert resolved: web-01 (pve1)" {
		t.Fatalf("default resolved subject = %q", got)
	}
	if got := resolvedEmailSubjectWithBranding(single, EmailBranding{SubjectPrefix: "[Acme Ops]"}); got != "[Acme Ops] Alert resolved: web-01 (pve1)" {
		t.Fatalf("branded single resolved subject = %q", got)
	}
	if got := resolvedEmailSubjectWithBranding(moved, EmailBranding{SubjectPrefix: "[Acme Ops]"}); got != "[Acme Ops] Alert moved: web-02 (pve1)" {
		t.Fatalf("branded moved subject = %q", got)
	}
	if got := resolvedEmailSubjectWithBranding(multi, EmailBranding{SubjectPrefix: "[Acme Ops]"}); got != "[Acme Ops] Alerts resolved (2): db-01 (pve2), web-01 (pve1)" {
		t.Fatalf("branded multi resolved subject = %q", got)
	}
}

func TestEmailBrandingDoesNotChangeDestinationContinuityIdentity(t *testing.T) {
	base := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		From:     "alerts@example.com",
		To:       []string{"ops@example.com"},
	}
	branded := base
	branded.SubjectPrefix = "[Acme Ops]"
	branded.BrandName = "Acme Infrastructure"
	branded.FooterNote = "Support desk"

	baseKey := notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &base})
	brandedKey := notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &branded})
	if baseKey == "" || baseKey != brandedKey {
		t.Fatalf("branding changed destination continuity key: base=%q branded=%q", baseKey, brandedKey)
	}
}
