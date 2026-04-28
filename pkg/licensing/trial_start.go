package licensing

import (
	"strings"
	"time"
)

const DefaultTrialDuration = 14 * 24 * time.Hour

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
