package licensing

import (
	"fmt"
	"strings"
)

const (
	EventPaywallViewed               = "paywall_viewed"
	EventTrialStarted                = "trial_started"
	EventLicenseActivated            = "license_activated"
	EventLicenseActivationFailed     = "license_activation_failed"
	EventUpgradeClicked              = "upgrade_clicked"
	EventCheckoutStarted             = "checkout_started"
	EventCheckoutCompleted           = "checkout_completed"
	EventLimitWarningShown           = "limit_warning_shown"
	EventLimitBlocked                = "limit_blocked"
	EventAgentInstallTokenGenerated  = "agent_install_token_generated"
	EventAgentInstallCommandCopied   = "agent_install_command_copied"
	EventAgentInstallProfileSelected = "agent_install_profile_selected"
	EventAgentFirstConnected         = "agent_first_connected"
)

var supportedConversionEventTypes = map[string]struct{}{
	EventPaywallViewed:               {},
	EventTrialStarted:                {},
	EventLicenseActivated:            {},
	EventLicenseActivationFailed:     {},
	EventUpgradeClicked:              {},
	EventCheckoutStarted:             {},
	EventCheckoutCompleted:           {},
	EventLimitWarningShown:           {},
	EventLimitBlocked:                {},
	EventAgentInstallTokenGenerated:  {},
	EventAgentInstallCommandCopied:   {},
	EventAgentInstallProfileSelected: {},
	EventAgentFirstConnected:         {},
}

// ConversionEvent is the canonical conversion instrumentation envelope.
type ConversionEvent struct {
	Type           string `json:"type"`
	OrgID          string `json:"org_id,omitempty"`
	Capability     string `json:"capability,omitempty"`
	Surface        string `json:"surface"`
	TenantMode     string `json:"tenant_mode,omitempty"`
	LimitKey       string `json:"limit_key,omitempty"`
	CurrentValue   int64  `json:"current_value,omitempty"`
	LimitValue     int64  `json:"limit_value,omitempty"`
	Timestamp      int64  `json:"timestamp"`
	IdempotencyKey string `json:"idempotency_key"`
}

// Validate ensures required fields and event-specific constraints are present.
func (e ConversionEvent) Validate() error {
	eventType := strings.TrimSpace(e.Type)
	if eventType == "" {
		return fmt.Errorf("type is required")
	}
	if !IsKnownConversionEventType(eventType) {
		return fmt.Errorf("type %q is not supported", eventType)
	}

	if strings.TrimSpace(e.Surface) == "" {
		return fmt.Errorf("surface is required")
	}
	if e.Timestamp <= 0 {
		return fmt.Errorf("timestamp is required")
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return fmt.Errorf("idempotency_key is required")
	}

	switch strings.TrimSpace(e.TenantMode) {
	case "", "single", "multi":
	default:
		return fmt.Errorf("tenant_mode must be \"single\" or \"multi\"")
	}

	switch eventType {
	case EventPaywallViewed:
		if strings.TrimSpace(e.Capability) == "" {
			return fmt.Errorf("capability is required for %s", EventPaywallViewed)
		}
	case EventLimitWarningShown, EventLimitBlocked:
		if strings.TrimSpace(e.LimitKey) == "" {
			return fmt.Errorf("limit_key is required for %s", eventType)
		}
	}

	return nil
}

// IsKnownConversionEventType reports whether the event type is supported.
func IsKnownConversionEventType(eventType string) bool {
	_, ok := supportedConversionEventTypes[strings.TrimSpace(eventType)]
	return ok
}
