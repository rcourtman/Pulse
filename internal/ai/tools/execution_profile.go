package tools

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// ExecutionProfile is the core-owned, request-local execution posture for
// one Assistant/Patrol run. It is deliberately an opaque integer with no
// JSON tags, string round-trip, or public constructor from external input:
// enterprise and transport code can never serialize, select, or relax a
// profile. Profiles are applied by core call sites only (chat service and
// the investigation adapter), and their policy is enforced twice - during
// provider projection and again at registry execution.
type ExecutionProfile int

const (
	// ProfileInteractiveAssistant is the default interactive chat
	// posture: the operator's configured control level and autonomy
	// govern, and the interactive question tool is available.
	ProfileInteractiveAssistant ExecutionProfile = iota
	// ProfilePatrolDetection is the scheduled Patrol detection posture:
	// non-interactive, no infrastructure mutations, and Pulse-state
	// mutations restricted to the finding lifecycle tools
	// (patrol_report_finding / patrol_resolve_finding).
	ProfilePatrolDetection
	// ProfilePatrolInvestigation is the Patrol investigation posture:
	// non-interactive and structurally read-only - no infrastructure and
	// no Pulse-state mutations. Typed remediation leaves the profile only
	// as a side-effect-free, mutation-none action proposal.
	ProfilePatrolInvestigation
)

// Valid reports whether the profile is one of the closed set. Like the
// invocation-class vocabulary, execution profiles are a closed
// vocabulary: an unknown value must be rejected, never silently treated
// as the permissive interactive default.
func (p ExecutionProfile) Valid() bool {
	switch p {
	case ProfileInteractiveAssistant, ProfilePatrolDetection, ProfilePatrolInvestigation:
		return true
	default:
		return false
	}
}

// NonInteractive reports whether the profile forbids interactive user
// input. This is deliberately independent of autonomous mode: suppressing
// questions grants no mutation authority, and mutation policy never
// loosens because a run is interactive. Unknown profiles are treated as
// non-interactive: an unclassifiable posture must never gain the
// interactive default's permissions.
func (p ExecutionProfile) NonInteractive() bool {
	return p != ProfileInteractiveAssistant
}

// patrolDetectionPulseStateAllowlist names the only tools whose
// pulse-state mutations the detection profile permits. A blanket
// pulse-state allowance would also permit alert dismissal and knowledge
// writes, which detection has no business performing.
func patrolDetectionPulseStateAllowlist() map[string]bool {
	return map[string]bool{
		agentcapabilities.PatrolReportFindingToolName:  true,
		agentcapabilities.PatrolResolveFindingToolName: true,
	}
}

// ApplyExecutionProfile applies the profile's policy to this executor
// instance (normally a request-scoped clone). Both Patrol profiles deny
// infrastructure mutations, clear any inherited autonomous mode, and mark
// the executor non-interactive; they differ only in pulse-state policy.
// Unknown profiles are rejected outright (panic on programmer error)
// rather than falling through to the interactive default's permissions.
func (e *PulseToolExecutor) ApplyExecutionProfile(profile ExecutionProfile) {
	if !profile.Valid() {
		panic(fmt.Sprintf("unknown execution profile %d: profiles are a closed vocabulary and cannot default to interactive permissions", int(profile)))
	}
	e.executionProfile = profile
	switch profile {
	case ProfilePatrolDetection:
		e.isAutonomous = false
		e.denyInfrastructureMutations = true
		e.pulseStateAllowlist = patrolDetectionPulseStateAllowlist()
	case ProfilePatrolInvestigation:
		e.isAutonomous = false
		e.denyInfrastructureMutations = true
		e.pulseStateAllowlist = map[string]bool{}
	default:
		e.denyInfrastructureMutations = false
		e.pulseStateAllowlist = nil
	}
}

// ExecutionProfile returns the profile applied to this executor instance.
func (e *PulseToolExecutor) ExecutionProfile() ExecutionProfile {
	if e == nil {
		return ProfileInteractiveAssistant
	}
	return e.executionProfile
}
