package notifications

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func taggedRoutingAlert(id string, tags interface{}) *alerts.Alert {
	return &alerts.Alert{
		ID:        id,
		StartTime: time.Unix(1_700_000_000, 0),
		Metadata:  map[string]interface{}{"tags": tags},
	}
}

func TestBuildNotificationDeliveryJobsRoutesAlertsByDestinationTags(t *testing.T) {
	alpha := taggedRoutingAlert("alpha", []string{"customer:alpha", "critical"})
	beta := taggedRoutingAlert("beta", []interface{}{"customer:beta", "critical"})
	untagged := &alerts.Alert{ID: "untagged", StartTime: time.Unix(1_700_000_000, 0)}

	jobs := buildNotificationDeliveryJobs(
		EmailConfig{
			Enabled:   true,
			TagFilter: []string{"customer:alpha", "critical"},
			TagMode:   notificationTagModeAll,
		},
		[]WebhookConfig{
			{ID: "alpha", Enabled: true, TagFilter: []string{"customer:alpha"}},
			{ID: "critical", Enabled: true, TagFilter: []string{"critical"}, TagMode: notificationTagModeAny},
			{ID: "global", Enabled: true},
			{ID: "missing", Enabled: true, TagFilter: []string{"does-not-exist"}},
		},
		AppriseConfig{},
		[]*alerts.Alert{alpha, beta, untagged},
		eventAlert,
		time.Time{},
	)

	if len(jobs) != 4 {
		t.Fatalf("jobs = %d, want email plus three matching webhooks", len(jobs))
	}
	if got := alertIDs(jobs[0].Alerts); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("email alerts = %v, want [alpha]", got)
	}
	if got := alertIDs(jobs[1].Alerts); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("alpha webhook alerts = %v, want [alpha]", got)
	}
	if got := alertIDs(jobs[2].Alerts); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("critical webhook alerts = %v, want [alpha beta]", got)
	}
	if got := alertIDs(jobs[3].Alerts); !reflect.DeepEqual(got, []string{"alpha", "beta", "untagged"}) {
		t.Fatalf("global webhook alerts = %v, want all alerts", got)
	}
}

func TestTagRoutingNormalizesCaseSourcesAndModes(t *testing.T) {
	alert := &alerts.Alert{
		ID: "mixed",
		Metadata: map[string]interface{}{
			"tags":         " Customer:Alpha ; production ",
			"resourceTags": []string{"Critical"},
			"hostTags":     []interface{}{"eu-west", 7},
		},
	}

	if !notificationAlertMatchesTags(alert, []string{"customer:alpha", "CRITICAL"}, "all") {
		t.Fatal("expected case-insensitive all-tag match across metadata sources")
	}
	if !notificationAlertMatchesTags(alert, []string{"missing", "EU-WEST"}, "ANY") {
		t.Fatal("expected case-insensitive any-tag match")
	}
	if notificationAlertMatchesTags(alert, []string{"missing"}, "any") {
		t.Fatal("unexpected match for absent tag")
	}
	if !notificationAlertMatchesTags(alert, nil, "all") {
		t.Fatal("empty filter must preserve global delivery")
	}
}

func TestResolvedTagRoutingDefersToDeliveryReceipts(t *testing.T) {
	alert := taggedRoutingAlert("changed-tags", []string{"customer:beta"})
	routed := routeNotificationAlerts(
		[]*alerts.Alert{alert},
		[]string{"customer:alpha"},
		notificationTagModeAll,
		notificationMinimumSeverityAll,
		eventResolved,
	)
	if len(routed) != 1 {
		t.Fatal("resolution must reach the receipt filter even after tags change")
	}
}

func TestBuildNotificationDeliveryJobsRoutesGroupedAlertsByDestinationSeverity(t *testing.T) {
	info := &alerts.Alert{ID: "info", Level: alerts.AlertLevelInfo, StartTime: time.Unix(1_699_999_999, 0)}
	warning := &alerts.Alert{ID: "warning", Level: alerts.AlertLevelWarning, StartTime: time.Unix(1_700_000_000, 0)}
	critical := &alerts.Alert{ID: "critical", Level: alerts.AlertLevelCritical, StartTime: time.Unix(1_700_000_001, 0)}

	jobs := buildNotificationDeliveryJobs(
		EmailConfig{Enabled: true, MinimumSeverity: notificationMinimumSeverityCritical},
		[]WebhookConfig{
			{ID: "all", Enabled: true, MinimumSeverity: notificationMinimumSeverityAll},
			{ID: "warning", Enabled: true, MinimumSeverity: notificationMinimumSeverityWarning},
			{ID: "critical", Enabled: true, MinimumSeverity: notificationMinimumSeverityCritical},
		},
		AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: "https://apprise.example.test", MinimumSeverity: notificationMinimumSeverityCritical},
		[]*alerts.Alert{info, warning, critical},
		eventAlert,
		time.Time{},
	)

	if len(jobs) != 5 {
		t.Fatalf("jobs = %d, want email, three webhooks, and Apprise", len(jobs))
	}
	want := [][]string{{"critical"}, {"info", "warning", "critical"}, {"warning", "critical"}, {"critical"}, {"critical"}}
	for index, expected := range want {
		if got := alertIDs(jobs[index].Alerts); !reflect.DeepEqual(got, expected) {
			t.Fatalf("job %d alerts = %v, want %v", index, got, expected)
		}
	}
}

func TestBuildNotificationDeliveryJobsTargetsExactEscalationDestinations(t *testing.T) {
	alert := &alerts.Alert{ID: "critical", Level: alerts.AlertLevelCritical, StartTime: time.Unix(1_700_000_000, 0)}
	jobs := buildNotificationDeliveryJobsForSelection(
		EmailConfig{Enabled: true},
		[]WebhookConfig{
			{ID: "ops", Enabled: true},
			{ID: "pager", Enabled: true},
		},
		AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: "https://apprise.example.test"},
		[]*alerts.Alert{alert},
		eventAlert,
		time.Time{},
		notificationDeliveryTargetAll,
		[]string{"webhook:pager", "apprise"},
	)

	if len(jobs) != 2 || jobs[0].WebhookConfig == nil || jobs[0].WebhookConfig.ID != "pager" || jobs[1].AppriseConfig == nil {
		t.Fatalf("exact escalation jobs = %+v, want pager webhook plus Apprise", jobs)
	}

	jobs = buildNotificationDeliveryJobsForSelection(
		EmailConfig{Enabled: true},
		[]WebhookConfig{{ID: "ops", Enabled: true}},
		AppriseConfig{Enabled: true},
		[]*alerts.Alert{alert},
		eventAlert,
		time.Time{},
		notificationDeliveryTargetAll,
		[]string{"webhook:deleted"},
	)
	if len(jobs) != 0 {
		t.Fatalf("unknown exact destination widened delivery: %+v", jobs)
	}
}

func TestResolvedSeverityRoutingDefersToOccurrenceReceipts(t *testing.T) {
	warning := &alerts.Alert{ID: "warning", Level: alerts.AlertLevelWarning}
	routed := routeNotificationAlerts(
		[]*alerts.Alert{warning},
		nil,
		"",
		notificationMinimumSeverityCritical,
		eventResolved,
	)
	if len(routed) != 1 {
		t.Fatal("resolution must reach the receipt filter after minimum severity changes")
	}
}

func TestNotificationMinimumSeverityNormalizationAndMatching(t *testing.T) {
	if got := normalizeNotificationMinimumSeverity(" WARNING "); got != notificationMinimumSeverityWarning {
		t.Fatalf("normalized severity = %q, want warning", got)
	}
	if got := normalizeNotificationMinimumSeverity(" CRITICAL "); got != notificationMinimumSeverityCritical {
		t.Fatalf("normalized severity = %q, want critical", got)
	}
	if got := normalizeNotificationMinimumSeverity("unexpected"); got != notificationMinimumSeverityAll {
		t.Fatalf("unknown severity = %q, want all", got)
	}
	if notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelWarning}, notificationMinimumSeverityCritical) {
		t.Fatal("warning alert unexpectedly met critical-only floor")
	}
	if !notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelCritical}, notificationMinimumSeverityCritical) {
		t.Fatal("critical alert did not meet critical-only floor")
	}
	if !notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelInfo}, notificationMinimumSeverityAll) {
		t.Fatal("all-alert floor excluded an informational alert")
	}
	if notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelInfo}, notificationMinimumSeverityWarning) {
		t.Fatal("warning floor included an informational alert")
	}
	if !notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelWarning}, notificationMinimumSeverityWarning) {
		t.Fatal("warning floor excluded a warning alert")
	}
	if !notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: alerts.AlertLevelCritical}, notificationMinimumSeverityWarning) {
		t.Fatal("warning floor excluded a critical alert")
	}
	if !notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: "error"}, notificationMinimumSeverityCritical) {
		t.Fatal("critical floor excluded the supported legacy error alias")
	}
	if notificationAlertMeetsMinimumSeverity(&alerts.Alert{Level: "unexpected"}, notificationMinimumSeverityCritical) {
		t.Fatal("unknown alert severity was promoted to critical")
	}
}

func TestNotificationTagConfigNormalizationAndCopies(t *testing.T) {
	email := normalizeEmailConfig(EmailConfig{
		TagFilter: []string{" Customer:Alpha ", "customer:alpha", "", "Critical"},
		TagMode:   "unexpected",
	})
	if !reflect.DeepEqual(email.TagFilter, []string{"Customer:Alpha", "Critical"}) {
		t.Fatalf("normalized email filter = %v", email.TagFilter)
	}
	if email.TagMode != notificationTagModeAll {
		t.Fatalf("normalized email mode = %q, want all", email.TagMode)
	}
	if email.MinimumSeverity != notificationMinimumSeverityAll {
		t.Fatalf("normalized email minimum severity = %q, want all", email.MinimumSeverity)
	}

	webhook := NormalizeWebhookConfig(WebhookConfig{
		TagFilter: []string{" prod ", "PROD", "customer:beta"},
		TagMode:   "ANY",
	})
	if !reflect.DeepEqual(webhook.TagFilter, []string{"prod", "customer:beta"}) {
		t.Fatalf("normalized webhook filter = %v", webhook.TagFilter)
	}
	if webhook.TagMode != notificationTagModeAny {
		t.Fatalf("normalized webhook mode = %q, want any", webhook.TagMode)
	}
	if webhook.MinimumSeverity != notificationMinimumSeverityAll {
		t.Fatalf("normalized webhook minimum severity = %q, want all", webhook.MinimumSeverity)
	}
	if got := NormalizeWebhookConfig(WebhookConfig{TagMode: notificationTagModeAny}); got.TagMode != "" {
		t.Fatalf("unfiltered webhook mode = %q, want empty for legacy compatibility", got.TagMode)
	}

	emailCopy := copyEmailConfig(email)
	webhookCopy := copyWebhookConfig(webhook)
	emailCopy.TagFilter[0] = "mutated"
	webhookCopy.TagFilter[0] = "mutated"
	if email.TagFilter[0] == "mutated" || webhook.TagFilter[0] == "mutated" {
		t.Fatal("notification config copies must isolate tag-filter slices")
	}
}

func TestNtfyHeadersPreserveInformationalSeverity(t *testing.T) {
	webhook := withNtfyAlertHeaders(WebhookConfig{}, []*alerts.Alert{{
		ID:           "info",
		Level:        alerts.AlertLevelInfo,
		ResourceName: "pulse-host",
		Type:         "system_update",
	}})

	if got := webhook.Headers["Title"]; got != "INFO: pulse-host" {
		t.Fatalf("ntfy title = %q, want informational title", got)
	}
	if got := webhook.Headers["Priority"]; got != "default" {
		t.Fatalf("ntfy priority = %q, want default", got)
	}
	if got := webhook.Headers["Tags"]; got != "information_source,pulse,system_update" {
		t.Fatalf("ntfy tags = %q, want informational routing tags", got)
	}
}

func alertIDs(alertList []*alerts.Alert) []string {
	ids := make([]string, 0, len(alertList))
	for _, alert := range alertList {
		ids = append(ids, alert.ID)
	}
	return ids
}
