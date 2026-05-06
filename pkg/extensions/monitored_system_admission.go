package extensions

// MonitoredSystemAdmissionInput describes the counted-system state that a
// commercial runtime can use to decide whether adding a candidate should be
// admitted.
type MonitoredSystemAdmissionInput struct {
	Current                  int
	Additional               int
	Limit                    int
	UsageAvailable           bool
	UsageUnavailableReason   string
	CandidateCountsTowardCap bool
}

// MonitoredSystemAdmissionDecision is the runtime admission decision returned
// by commercial policy hooks.
type MonitoredSystemAdmissionDecision struct {
	Current                int
	Additional             int
	Limit                  int
	UsageAvailable         bool
	UsageUnavailableReason string
	Exceeded               bool
}
