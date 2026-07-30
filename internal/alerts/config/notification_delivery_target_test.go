package config

import "testing"

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
