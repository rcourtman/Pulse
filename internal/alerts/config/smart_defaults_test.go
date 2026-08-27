package config_test

import (
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

func smartInt(value int) *int { return &value }

func smartInt64(value int64) *int64 { return &value }

func TestNormalizeAgentDefaultsSMARTBackwardCompatibilityAndDisableSemantics(t *testing.T) {
	config := &alertconfig.AlertConfig{}
	alertconfig.NormalizeAgentDefaults(config)

	if config.AgentDefaults.SMARTHealthFailure == nil || *config.AgentDefaults.SMARTHealthFailure != 1 ||
		config.AgentDefaults.SMARTCRCErrorDelta == nil || *config.AgentDefaults.SMARTCRCErrorDelta != 1 ||
		config.AgentDefaults.SMARTLifeWarning == nil || *config.AgentDefaults.SMARTLifeWarning != 10 {
		t.Fatalf("missing SMART defaults were not seeded: %+v", config.AgentDefaults)
	}

	config.AgentDefaults.SMARTHealthFailure = smartInt(0)
	config.AgentDefaults.SMARTPending = smartInt64(0)
	config.AgentDefaults.SMARTCRCErrorDelta = smartInt64(0)
	config.AgentDefaults.SMARTLifeWarning = smartInt(0)
	config.AgentDefaults.SMARTLifeCritical = smartInt(0)
	alertconfig.NormalizeAgentDefaults(config)

	if *config.AgentDefaults.SMARTHealthFailure != 0 || *config.AgentDefaults.SMARTPending != 0 ||
		*config.AgentDefaults.SMARTCRCErrorDelta != 0 || *config.AgentDefaults.SMARTLifeWarning != 0 ||
		*config.AgentDefaults.SMARTLifeCritical != 0 {
		t.Fatalf("explicit zero SMART rules must stay disabled: %+v", config.AgentDefaults)
	}
}

func TestNormalizeAgentDefaultsSMARTClampsInvalidRanges(t *testing.T) {
	config := &alertconfig.AlertConfig{AgentDefaults: alertconfig.ThresholdConfig{
		SMARTHealthFailure: smartInt(9),
		SMARTPending:       smartInt64(-2),
		SMARTLifeWarning:   smartInt(150),
		SMARTLifeCritical:  smartInt(120),
		SMARTSpareWarning:  smartInt(12),
		SMARTSpareCritical: smartInt(40),
	}}
	alertconfig.NormalizeAgentDefaults(config)

	if *config.AgentDefaults.SMARTHealthFailure != 1 || *config.AgentDefaults.SMARTPending != 1 {
		t.Fatalf("invalid discrete SMART values were not normalized: %+v", config.AgentDefaults)
	}
	if *config.AgentDefaults.SMARTLifeWarning != 100 || *config.AgentDefaults.SMARTLifeCritical != 100 {
		t.Fatalf("life thresholds were not clamped: %+v", config.AgentDefaults)
	}
	if *config.AgentDefaults.SMARTSpareWarning != 12 || *config.AgentDefaults.SMARTSpareCritical != 12 {
		t.Fatalf("critical spare threshold must not exceed warning: %+v", config.AgentDefaults)
	}
}
