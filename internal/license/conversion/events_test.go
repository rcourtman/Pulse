package conversion

import (
	"testing"
	"time"
)

func TestConversionEventValidateAcceptsUnifiedAgentOnboardingTypes(t *testing.T) {
	now := time.Now().UnixMilli()
	testCases := []string{
		EventAgentInstallTokenGenerated,
		EventAgentInstallCommandCopied,
		EventAgentInstallProfileSelected,
		EventAgentFirstConnected,
	}

	for _, eventType := range testCases {
		t.Run(eventType, func(t *testing.T) {
			event := ConversionEvent{
				Type:           eventType,
				Surface:        "settings_unified_agents",
				Capability:     "auto",
				Timestamp:      now,
				IdempotencyKey: eventType + ":settings_unified_agents:auto:1",
			}
			if err := event.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestConversionEventValidateRejectsUnknownType(t *testing.T) {
	event := ConversionEvent{
		Type:           "not_a_real_event",
		Surface:        "settings_unified_agents",
		Timestamp:      time.Now().UnixMilli(),
		IdempotencyKey: "not_a_real_event:settings_unified_agents::1",
	}

	if err := event.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsupported type error")
	}
}
