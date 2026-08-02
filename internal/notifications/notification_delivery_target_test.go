package notifications

import (
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestInitialNotifyTargetNormalization(t *testing.T) {
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	defer manager.Stop()

	if actual := manager.GetInitialNotifyTarget(); actual != "all" {
		t.Fatalf("default initial target = %q, want all", actual)
	}

	tests := map[string]string{
		"EMAIL":     "email",
		" webhooks": "webhook",
		"apprise":   "apprise",
		"invalid":   "all",
	}
	for input, expected := range tests {
		manager.SetInitialNotifyTarget(input)
		if actual := manager.GetInitialNotifyTarget(); actual != expected {
			t.Fatalf("initial target after %q = %q, want %q", input, actual, expected)
		}
	}
}

func TestNotificationDeliveryJobsRespectTarget(t *testing.T) {
	email := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.example.test",
		SMTPPort: 587,
		From:     "pulse@example.test",
		To:       []string{"ops@example.test"},
	}
	webhooks := []WebhookConfig{
		{ID: "hook-1", Name: "First", URL: "https://hooks.example.test/one", Enabled: true},
		{ID: "hook-2", Name: "Second", URL: "https://hooks.example.test/two", Enabled: true},
	}
	apprise := AppriseConfig{Enabled: true, Targets: []string{"ntfy://example.test/pulse"}}
	alertList := []*alerts.Alert{{
		ID:           "node-1-cpu",
		ResourceName: "node-1",
		StartTime:    time.Now(),
	}}

	tests := []struct {
		name     string
		target   notificationDeliveryTarget
		expected []string
	}{
		{name: "all", target: notificationDeliveryTargetAll, expected: []string{"apprise", "email", "webhook", "webhook"}},
		{name: "email", target: notificationDeliveryTargetEmail, expected: []string{"email"}},
		{name: "webhook", target: notificationDeliveryTargetWebhook, expected: []string{"webhook", "webhook"}},
		{name: "apprise", target: notificationDeliveryTargetApprise, expected: []string{"apprise"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jobs := buildNotificationDeliveryJobsForTarget(
				email,
				webhooks,
				apprise,
				alertList,
				eventAlert,
				time.Time{},
				test.target,
			)
			actual := make([]string, 0, len(jobs))
			for _, job := range jobs {
				actual = append(actual, job.Type)
			}
			sort.Strings(actual)
			sort.Strings(test.expected)
			if len(actual) != len(test.expected) {
				t.Fatalf("job types = %#v, want %#v", actual, test.expected)
			}
			for index := range actual {
				if actual[index] != test.expected[index] {
					t.Fatalf("job types = %#v, want %#v", actual, test.expected)
				}
			}
		})
	}
}

func TestGroupedAlertsUseInitialNotifyTarget(t *testing.T) {
	queue, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}
	defer func() { _ = queue.Stop() }()

	manager := &NotificationManager{
		enabled:       true,
		initialTarget: notificationDeliveryTargetApprise,
		emailConfig: EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.test",
			SMTPPort: 587,
			From:     "pulse@example.test",
			To:       []string{"ops@example.test"},
		},
		webhooks: []WebhookConfig{{
			ID: "hook-1", Name: "Webhook", URL: "https://hooks.example.test/pulse", Enabled: true,
		}},
		appriseConfig: AppriseConfig{
			Enabled: true,
			Targets: []string{"ntfy://example.test/pulse"},
		},
		lastNotified:     make(map[string]notificationRecord),
		deliveryReceipts: make(map[string]struct{}),
		queue:            queue,
	}
	manager.pendingAlerts = []*alerts.Alert{{
		ID:           "node-1-cpu",
		ResourceName: "node-1",
		StartTime:    time.Now(),
	}}

	manager.sendGroupedAlerts()

	rows, err := queue.db.Query(`SELECT type FROM notification_queue ORDER BY created_at`)
	if err != nil {
		t.Fatalf("query queued types: %v", err)
	}
	defer rows.Close()

	var queuedTypes []string
	for rows.Next() {
		var queuedType string
		if err := rows.Scan(&queuedType); err != nil {
			t.Fatalf("scan queued type: %v", err)
		}
		queuedTypes = append(queuedTypes, queuedType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate queued types: %v", err)
	}
	if len(queuedTypes) != 1 || queuedTypes[0] != "apprise" {
		t.Fatalf("queued types = %#v, want only apprise", queuedTypes)
	}
}
