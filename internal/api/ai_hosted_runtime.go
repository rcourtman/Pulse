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
	if !shouldAutoBootstrapHostedAIConfig(hostedMode, persistence) {
		return cfg, nil
	}

	state, err := ensureHostedAIQuickstartBillingState(billingBaseDir, orgID)
	if err != nil {
		return nil, err
	}
	if !hostedAIAutoBootstrapEligible(state) {
		return cfg, nil
	}

	bootstrapped := config.NewDefaultAIConfig()
	bootstrapped.Enabled = true
	quickstartModel := config.DefaultModelForProvider(config.AIProviderQuickstart)
	bootstrapped.Model = quickstartModel
	bootstrapped.ChatModel = quickstartModel
	bootstrapped.PatrolModel = quickstartModel
	if err := persistence.SaveAIConfig(*bootstrapped); err != nil {
		return nil, fmt.Errorf("persist hosted Pulse Assistant bootstrap config: %w", err)
	}
	return bootstrapped, nil
}

func shouldAutoBootstrapHostedAIConfig(hostedMode bool, persistence *config.ConfigPersistence) bool {
	return hostedMode && persistence != nil && !persistence.HasAIConfig()
}

func ensureHostedAIQuickstartBillingState(billingBaseDir, orgID string) (*billingState, error) {
	billingBaseDir = strings.TrimSpace(billingBaseDir)
	if billingBaseDir == "" {
		return nil, nil
	}

	billingStore := config.NewFileBillingStore(billingBaseDir)
	state, effectiveOrgID, err := loadHostedEffectiveBillingState(billingStore, orgID)
	if err != nil || state == nil {
		return state, err
	}
	if !hostedAIAutoBootstrapEligible(state) || state.QuickstartCreditsGranted {
		return state, nil
	}

	updated := normalizeBillingStateFromLicensing(state)
	updated.GrantQuickstartCredits()
	if err := billingStore.SaveBillingState(effectiveOrgID, updated); err != nil {
		return nil, fmt.Errorf("save hosted Pulse Assistant quickstart billing state: %w", err)
	}
	return updated, nil
}

func hostedAIAutoBootstrapEligible(state *billingState) bool {
	if state == nil {
		return false
	}
	if strings.TrimSpace(state.EntitlementJWT) == "" && strings.TrimSpace(state.EntitlementRefreshToken) == "" {
		return false
	}
	switch state.SubscriptionState {
	case subscriptionStateActiveValue, subscriptionStateGraceValue, subscriptionStateTrialValue:
	default:
		return false
	}
	return hasBillingCapability(state, featureAIPatrolValue) || hasBillingCapability(state, featureAIAutoFixValue)
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
