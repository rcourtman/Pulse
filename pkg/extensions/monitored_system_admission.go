package extensions

import "context"

// MonitoredSystemAdmissionInput captures the public counted-system admission
// state computed by the OSS runtime before any private commercial policy hook
// is consulted.
type MonitoredSystemAdmissionInput struct {
	Current                  int
	Additional               int
	Limit                    int
	UsageAvailable           bool
	UsageUnavailableReason   string
	CandidateCountsTowardCap bool
}

// MonitoredSystemAdmissionDecision captures the commercial admission outcome
// that a private build may return. This intentionally stays scoped to backend
// admission semantics rather than customer-facing billing/posture messaging.
type MonitoredSystemAdmissionDecision struct {
	Current                int
	Additional             int
	Limit                  int
	UsageAvailable         bool
	UsageUnavailableReason string
	Exceeded               bool
}

// ResolveMonitoredSystemAdmissionPolicyFunc allows private builds to own the
// commercial monitored-system admission decision without importing internal API
// packages. The public runtime remains the source of truth for counted-system
// projection and may call this hook in a later migration slice.
type ResolveMonitoredSystemAdmissionPolicyFunc func(context.Context, MonitoredSystemAdmissionInput) MonitoredSystemAdmissionDecision
