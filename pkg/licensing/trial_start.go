package licensing

import (
	"strings"
	"time"
)

const DefaultTrialDuration = 14 * 24 * time.Hour

type TrialStartDenialReason string

const (
	TrialStartAllowed            TrialStartDenialReason = ""
	TrialStartDeniedLicense      TrialStartDenialReason = "license_active"
	TrialStartDeniedAlreadyUsed  TrialStartDenialReason = "already_used"
	TrialStartDeniedSubscription TrialStartDenialReason = "subscription_active"
)

type TrialStartDecision struct {
	Allowed bool
	Reason  TrialStartDenialReason
}

func EvaluateTrialStartEligibility(hasActiveLicense bool, existing *BillingState) TrialStartDecision {
	if hasActiveLicense {
		return TrialStartDecision{Allowed: false, Reason: TrialStartDeniedLicense}
	}
	if existing == nil {
		return TrialStartDecision{Allowed: true, Reason: TrialStartAllowed}
	}
	if existing.TrialStartedAt != nil {
		return TrialStartDecision{Allowed: false, Reason: TrialStartDeniedAlreadyUsed}
	}
	switch existing.SubscriptionState {
	case SubStateActive, SubStateGrace, SubStateSuspended:
		return TrialStartDecision{Allowed: false, Reason: TrialStartDeniedSubscription}
	default:
		return TrialStartDecision{Allowed: true, Reason: TrialStartAllowed}
	}
}

func TrialStartError(reason TrialStartDenialReason) (code, message string, includeOrgID bool) {
	switch reason {
	case TrialStartDeniedLicense:
		return "trial_not_available", "Trial cannot be started while a license is active", false
	case TrialStartDeniedAlreadyUsed:
		return "trial_already_used", "Trial has already been used for this organization", true
	case TrialStartDeniedSubscription:
		return "trial_not_available", "Trial cannot be started while a subscription is active", true
	default:
		return "", "", false
	}
}

func TrialWindow(now time.Time, duration time.Duration) (startedAt, endsAt int64) {
	if duration <= 0 {
		duration = DefaultTrialDuration
	}
	startedAt = now.Unix()
	endsAt = now.Add(duration).Unix()
	return startedAt, endsAt
}

func BuildTrialBillingState(now time.Time, capabilities []string) *BillingState {
	return BuildTrialBillingStateWithPlan(now, capabilities, string(SubStateTrial), DefaultTrialDuration)
}

func BuildTrialBillingStateWithPlan(now time.Time, capabilities []string, planVersion string, duration time.Duration) *BillingState {
	startedAt, endsAt := TrialWindow(now, duration)
	planVersion = strings.TrimSpace(planVersion)
	if planVersion == "" {
		planVersion = string(SubStateTrial)
	}

	return &BillingState{
		Capabilities:      append([]string(nil), capabilities...),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       planVersion,
		SubscriptionState: SubStateTrial,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
	}
}
