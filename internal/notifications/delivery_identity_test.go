package notifications

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestDeliveryDestinationIdentityNormalization(t *testing.T) {
	email := func(to ...string) notificationDeliveryJob {
		return notificationDeliveryJob{Type: "email", EmailConfig: &EmailConfig{To: to}}
	}
	apprise := func(server, key string, targets ...string) notificationDeliveryJob {
		return notificationDeliveryJob{Type: "apprise", AppriseConfig: &AppriseConfig{ServerURL: server, ConfigKey: key, Targets: targets}}
	}
	tests := []struct {
		name                          string
		original, equivalent, changed notificationDeliveryJob
	}{
		{"email recipients", email("b@example.test", "a@example.test"), email(" a@example.test ", "b@example.test"), email("c@example.test", "a@example.test")},
		{"apprise targets", apprise("https://relay.example.test", "ops", "ntfy://b", "ntfy://a"), apprise(" https://relay.example.test ", " ops ", " ntfy://a ", "ntfy://b"), apprise("https://relay.example.test", "ops", "ntfy://c", "ntfy://a")},
		{"apprise server", apprise("https://one.example.test", "ops", "ntfy://a"), apprise("https://one.example.test", "ops", "ntfy://a"), apprise("https://two.example.test", "ops", "ntfy://a")},
		{"apprise configuration", apprise("https://relay.example.test", "ops", "ntfy://a"), apprise("https://relay.example.test", "ops", "ntfy://a"), apprise("https://relay.example.test", "other", "ntfy://a")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input []string
			if tt.original.EmailConfig != nil {
				input = tt.original.EmailConfig.To
			} else {
				input = tt.original.AppriseConfig.Targets
			}
			before := append([]string(nil), input...)
			key := notificationDeliveryDestinationKey(tt.original)
			if key == "" {
				t.Fatal("missing destination identity")
			}
			if key != notificationDeliveryDestinationKey(tt.equivalent) {
				t.Fatal("equivalent destination changed identity")
			}
			if key == notificationDeliveryDestinationKey(tt.changed) {
				t.Fatal("changed destination inherited identity")
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("identity calculation mutated configured order")
			}
		})
	}
}

func TestDeliveryReceiptIdentityRequiresOccurrence(t *testing.T) {
	start := time.Date(2026, 9, 8, 7, 0, 0, 123, time.UTC)
	original := &alerts.Alert{ID: "cpu-1", StartTime: start}
	key := notificationDeliveryReceiptKey(original, "destination")
	if key == "" {
		t.Fatal("missing occurrence identity")
	}
	next := original.Clone()
	next.StartTime = start.Add(time.Nanosecond)
	if key == notificationDeliveryReceiptKey(next, "destination") {
		t.Fatal("adjacent recurrence reused identity")
	}
	if key == notificationDeliveryReceiptKey(original, "other") {
		t.Fatal("other destination reused identity")
	}
	for _, alert := range []*alerts.Alert{nil, {}, {ID: "cpu-1"}, {StartTime: start}} {
		if notificationDeliveryReceiptKey(alert, "destination") != "" {
			t.Fatal("incomplete occurrence gained receipt identity")
		}
	}
	if notificationDeliveryReceiptKey(original, "") != "" {
		t.Fatal("missing destination gained receipt identity")
	}
}
