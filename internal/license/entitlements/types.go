package entitlements

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

// SubscriptionState represents the subscription lifecycle state.
type SubscriptionState = pkglicensing.SubscriptionState

const (
	SubStateTrial     = pkglicensing.SubStateTrial
	SubStateActive    = pkglicensing.SubStateActive
	SubStateGrace     = pkglicensing.SubStateGrace
	SubStateExpired   = pkglicensing.SubStateExpired
	SubStateSuspended = pkglicensing.SubStateSuspended
	SubStateCanceled  = pkglicensing.SubStateCanceled
)

// LimitCheckResult represents the result of evaluating a limit.
type LimitCheckResult = pkglicensing.LimitCheckResult

const (
	LimitAllowed   = pkglicensing.LimitAllowed
	LimitSoftBlock = pkglicensing.LimitSoftBlock
	LimitHardBlock = pkglicensing.LimitHardBlock
)
