package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func loadHostedAwareAIConfig(hostedMode bool, billingBaseDir, orgID string, persistence *config.ConfigPersistence) (*config.AIConfig, error) {
	if persistence == nil {
		return nil, fmt.Errorf("Pulse Assistant config persistence unavailable")
	}

	cfg, err := persistence.LoadAIConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func shouldAutoBootstrapHostedAIConfig(hostedMode bool, persistence *config.ConfigPersistence) bool {
	return false
}

func ensureHostedAIQuickstartBillingState(billingBaseDir, orgID string) (*billingState, error) {
	state, _, err := config.LoadEffectiveEntitlementBillingState(billingBaseDir, orgID)
	return state, err
}

func hostedAIAutoBootstrapEligible(state *billingState) bool {
	return false
}

func hasBillingCapability(state *billingState, feature string) bool {
	if state == nil || strings.TrimSpace(feature) == "" {
		return false
	}
	for _, capability := range state.Capabilities {
		if capability == feature {
			return true
		}
	}
	return false
}

func hostedModeEnabledFromEnv() bool {
	return os.Getenv("PULSE_HOSTED_MODE") == "true"
}
