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

// A delayed recovery for an earlier occurrence must consume only its own
// destination receipt, even when a recurrence has already fired before restart.
func TestDelayedRecoveryPreservesRecurringAndOtherDestinationReceipts(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 9, 8, 1, 0, 0, 123, time.UTC)
	old := &alerts.Alert{ID: "cpu-critical-1", StartTime: start}
	current := old.Clone()
	current.StartTime = start.Add(time.Nanosecond)
	destinations := []WebhookConfig{
		{ID: "ops", URL: "https://ops.example.test", Enabled: true},
		{ID: "backup", URL: "https://backup.example.test", Enabled: true},
	}
	job := func(a *alerts.Alert, destination int, event notificationEvent) notificationDeliveryJob {
		return notificationDeliveryJob{Type: "webhook", Event: event,
			Alerts: []*alerts.Alert{a}, WebhookConfig: &destinations[destination]}
	}
	open := func() *NotificationManager {
		m := NewNotificationManagerWithDataDir("", dir)
		t.Cleanup(m.Stop)
		return m
	}
	m := open()
	for _, a := range []*alerts.Alert{old, current} {
		for destination := range destinations {
			m.recordSuccessfulDelivery(job(a, destination, eventAlert), start.Add(time.Minute))
		}
	}
	m.Stop()
	m = open()
	assertReceipt := func(a *alerts.Alert, destination int, want bool) {
		t.Helper()
		got := m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job(a, destination, eventResolved)})
		if (len(got) == 1) != want {
			t.Fatalf("occurrence %s destination %d: eligible jobs=%d, want receipt=%t", a.StartTime, destination, len(got), want)
		}
	}
	for _, a := range []*alerts.Alert{old, current} {
		for destination := range destinations {
			assertReceipt(a, destination, true)
		}
	}
	// Duplicate completion must be harmless as well as the first completion.
	for range 2 {
		m.recordSuccessfulDelivery(job(old, 0, eventResolved), start.Add(2*time.Minute))
	}
	m.Stop()
	m = open()
	assertReceipt(old, 0, false)
	assertReceipt(old, 1, true)
	assertReceipt(current, 0, true)
	assertReceipt(current, 1, true)
	// The newer incident can still complete independently at either endpoint.
	m.recordSuccessfulDelivery(job(current, 0, eventResolved), start.Add(3*time.Minute))
	assertReceipt(current, 0, false)
	assertReceipt(current, 1, true)
	assertReceipt(old, 1, true)
}
