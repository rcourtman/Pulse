package config

import (
	"reflect"
	"testing"
)

func TestNormalizeNotificationDeliveryTarget(t *testing.T) {
	tests := map[string]string{
		"":          "all",
		"all":       "all",
		"EMAIL":     "email",
		" webhook ": "webhook",
		"webhooks":  "webhook",
		"Apprise":   "apprise",
		"unknown":   "all",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if actual := NormalizeNotificationDeliveryTarget(input); actual != expected {
				t.Fatalf("NormalizeNotificationDeliveryTarget(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestNormalizeEscalationConfigPreservesLegacyAndBoundsExactRouting(t *testing.T) {
	config := EscalationConfig{
		Enabled:        true,
		RepeatCritical: true,
		RepeatEvery:    1,
		Levels: []EscalationLevel{{
			After:          15,
			Notify:         " WEBHOOKS ",
			DestinationIDs: []string{" webhook:ops ", "email", "webhook:ops", "https://secret.example", "webhook:in\nvalid"},
		}},
	}

	NormalizeEscalationConfig(&config)

	if config.Levels[0].Notify != "webhook" {
		t.Fatalf("legacy fallback = %q, want webhook", config.Levels[0].Notify)
	}
	if got, want := config.Levels[0].DestinationIDs, []string{"webhook:ops", "email"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("destination IDs = %v, want %v", got, want)
	}
	if config.RepeatEvery != DefaultEscalationRepeatMinutes {
		t.Fatalf("repeat interval = %d, want %d", config.RepeatEvery, DefaultEscalationRepeatMinutes)
	}

	legacy := EscalationConfig{Enabled: true, Levels: []EscalationLevel{{After: 30, Notify: "apprise"}}}
	NormalizeEscalationConfig(&legacy)
	if legacy.RepeatCritical || legacy.RepeatEvery != 0 || legacy.Levels[0].DestinationIDs != nil {
		t.Fatalf("legacy config gained new behavior: %+v", legacy)
	}
}
