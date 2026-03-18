package licensing

import (
	"strings"
	"testing"
	"time"
)

func TestConversionEventValidate(t *testing.T) {
	now := time.Now().UnixMilli()

	tests := []struct {
		name      string
		event     ConversionEvent
		wantError string
	}{
		{
			name:      "missing type",
			event:     ConversionEvent{Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "type is required",
		},
		{
			name:      "unknown type",
			event:     ConversionEvent{Type: "unknown", Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "is not supported",
		},
		{
			name:      "missing surface",
			event:     ConversionEvent{Type: EventTrialStarted, Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "surface is required",
		},
		{
			name:      "missing timestamp",
			event:     ConversionEvent{Type: EventTrialStarted, Surface: "dashboard", IdempotencyKey: "id-1"},
			wantError: "timestamp is required",
		},
		{
			name:      "missing idempotency key",
			event:     ConversionEvent{Type: EventTrialStarted, Surface: "dashboard", Timestamp: now},
			wantError: "idempotency_key is required",
		},
		{
			name:      "invalid tenant mode",
			event:     ConversionEvent{Type: EventTrialStarted, Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1", TenantMode: "invalid"},
			wantError: "tenant_mode must be",
		},
		{
			name:      "paywall requires capability",
			event:     ConversionEvent{Type: EventPaywallViewed, Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "capability is required",
		},
		{
			name:      "limit warning requires limit key",
			event:     ConversionEvent{Type: EventLimitWarningShown, Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "limit_key is required",
		},
		{
			name:      "limit blocked requires limit key",
			event:     ConversionEvent{Type: EventLimitBlocked, Surface: "dashboard", Timestamp: now, IdempotencyKey: "id-1"},
			wantError: "limit_key is required",
		},
		{
			name: "valid paywall event",
			event: ConversionEvent{
				Type:           EventPaywallViewed,
				Surface:        "dashboard",
				Capability:     "relay",
				Timestamp:      now,
				IdempotencyKey: "id-1",
				TenantMode:     "single",
			},
		},
		{
			name: "valid trial started with multi tenant mode",
			event: ConversionEvent{
				Type:           EventTrialStarted,
				Surface:        "dashboard",
				Timestamp:      now,
				IdempotencyKey: "id-2",
				TenantMode:     "multi",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()
			if tc.wantError == "" && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if tc.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantError)
				}
				if !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected error containing %q, got %q", tc.wantError, err.Error())
				}
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
