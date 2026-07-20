package notifications

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestResolvedJobsRequireReceiptForSameOccurrenceAndDestination(t *testing.T) {
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	defer manager.Stop()
	start := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	alert := &alerts.Alert{ID: "vm-offline-101", StartTime: start}
	first := WebhookConfig{ID: "first", URL: "https://first.example.test", Enabled: true}
	second := WebhookConfig{ID: "second", URL: "https://second.example.test", Enabled: true}
	firingJob := notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{alert}, WebhookConfig: &first}
	manager.recordSuccessfulDelivery(firingJob, start.Add(time.Second))

	resolvedJobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{first, second}, AppriseConfig{}, []*alerts.Alert{alert}, eventResolved, start.Add(time.Minute))
	filtered := manager.filterResolvedJobsByDeliveryReceipt(resolvedJobs)
	if len(filtered) != 1 || filtered[0].WebhookConfig == nil || filtered[0].WebhookConfig.ID != "first" {
		t.Fatalf("filtered jobs = %+v", filtered)
	}

	newOccurrence := alert.Clone()
	newOccurrence.StartTime = start.Add(2 * time.Hour)
	resolvedJobs = buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{first}, AppriseConfig{}, []*alerts.Alert{newOccurrence}, eventResolved, start.Add(3*time.Hour))
	if got := manager.filterResolvedJobsByDeliveryReceipt(resolvedJobs); len(got) != 0 {
		t.Fatalf("new occurrence inherited old receipt: %+v", got)
	}
}

func TestResolvedJobsDoNotReuseReceiptAfterWebhookURLChanges(t *testing.T) {
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	defer manager.Stop()
	start := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	alert := &alerts.Alert{ID: "vm-offline-102", StartTime: start}
	original := WebhookConfig{ID: "ops", URL: "https://first.example.test", Enabled: true}
	reconfigured := WebhookConfig{ID: "ops", URL: "https://second.example.test", Enabled: true}
	manager.recordSuccessfulDelivery(notificationDeliveryJob{
		Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{alert}, WebhookConfig: &original,
	}, start.Add(time.Second))

	resolvedJobs := buildNotificationDeliveryJobs(
		EmailConfig{}, []WebhookConfig{reconfigured}, AppriseConfig{}, []*alerts.Alert{alert}, eventResolved, start.Add(time.Minute),
	)
	if got := manager.filterResolvedJobsByDeliveryReceipt(resolvedJobs); len(got) != 0 {
		t.Fatalf("reconfigured webhook inherited receipt for old URL: %+v", got)
	}
}

func TestDeliveryReceiptPersistsAndIsClearedAfterRecovery(t *testing.T) {
	dataDir := t.TempDir()
	start := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	alert := &alerts.Alert{ID: "disk-critical-1", StartTime: start}
	webhook := WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true}

	first := NewNotificationManagerWithDataDir("", dataDir)
	first.recordSuccessfulDelivery(notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{alert}, WebhookConfig: &webhook}, start.Add(time.Second))
	first.Stop()

	second := NewNotificationManagerWithDataDir("", dataDir)
	defer second.Stop()
	resolved := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{alert}, WebhookConfig: &webhook}
	if got := second.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(got) != 1 {
		t.Fatalf("persisted receipt not found: %+v", got)
	}
	second.recordSuccessfulDelivery(resolved, start.Add(time.Minute))
	if got := second.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(got) != 0 {
		t.Fatalf("receipt remained after recovery delivery: %+v", got)
	}
}
