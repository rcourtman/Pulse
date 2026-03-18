package licensing

func IsHostedSubscriptionValid(subState SubscriptionState, hasTrialEnd bool) bool {
	switch subState {
	case SubStateActive, SubStateGrace:
		return true
	case SubStateTrial:
		// Only allow trials with an explicit end date to prevent "infinite free Cloud".
		return hasTrialEnd
	default:
		return false
	}
}
