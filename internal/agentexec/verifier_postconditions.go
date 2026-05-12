package agentexec

import (
	"sort"
	"strings"
	"time"
)

// PostconditionField names the field on the post-write read that must be
// inspected to decide whether the action's intended state landed. Field
// values are exec-language-agnostic strings - the verifier substrate is
// responsible for mapping them to the right provider read.
//
// Closed enum to keep drift detectable in tests; the postcondition map
// below is the only place these are allowed to appear.
type PostconditionField string

const (
	// VM and container status as reported by Proxmox.
	FieldVMStatus        PostconditionField = "status"
	FieldContainerStatus PostconditionField = "status"

	// Systemd unit fields read via DBus or systemctl show.
	FieldUnitActiveState           PostconditionField = "ActiveState"
	FieldUnitSubState              PostconditionField = "SubState"
	FieldUnitActiveEnterTimestamp  PostconditionField = "ActiveEnterTimestamp"

	// Docker container fields.
	FieldDockerStatus      PostconditionField = "status"
	FieldDockerLastStarted PostconditionField = "last_started"

	// Kubernetes deployment fields.
	FieldDeploymentReadyReplicas    PostconditionField = "readyReplicas"
	FieldDeploymentDesiredReplicas PostconditionField = "desiredReplicas"
)

// PostconditionComparator names the comparison the verifier applies between
// the observed value and the expected target. The closed set keeps the wire
// format inspectable instead of pushing free-form expressions into the audit
// trail.
type PostconditionComparator string

const (
	// Equals: observed value must equal Expected (case-insensitive for strings).
	CompareEquals PostconditionComparator = "equals"
	// AfterOrEqual: observed timestamp must be >= action_started_at + offset.
	// Used for the "did the unit actually restart, not just stay up" check.
	CompareAfterOrEqualActionStart PostconditionComparator = "after_or_equal_action_start"
	// EqualsField: observed value must equal another observed field on the
	// same read (e.g. readyReplicas == desiredReplicas).
	CompareEqualsField PostconditionComparator = "equals_field"
)

// PostconditionCheck is one assertion the verifier evaluates against the
// post-write read. A capability's postcondition is the AND of its checks -
// all must hold for the outcome to be VerificationVerified.
type PostconditionCheck struct {
	// Field is the dotted-path or named field on the verification read.
	Field PostconditionField `json:"field"`
	// Comparator names how Expected is interpreted.
	Comparator PostconditionComparator `json:"comparator"`
	// Expected is the literal value (CompareEquals) or the peer field name
	// (CompareEqualsField). Empty for CompareAfterOrEqualActionStart.
	Expected string `json:"expected,omitempty"`
}

// CapabilityPostcondition describes how to verify a single tool capability.
// Window is the maximum wall-clock window the verifier waits for the
// postcondition to be observed; capabilities with no natural settle
// (a single-shot status read) use the default.
type CapabilityPostcondition struct {
	Capability  string                 `json:"capability"`
	VerifyRead  string                 `json:"verifyRead"`
	Window      time.Duration          `json:"window"`
	Description string                 `json:"description"`
	Checks      []PostconditionCheck   `json:"checks"`
}

// defaultVerifyWindow is the per-capability fallback window. The agentexec
// policy carries an operator-tunable verify_window that overrides this; the
// per-capability values here are the substrate's "what is reasonable for
// this surface" hint.
const defaultVerifyWindow = 2 * time.Minute

// CapabilityPostconditions returns the canonical capability -> postcondition
// map for the verifier substrate. The returned map is a copy so callers
// cannot mutate the shared substrate.
//
// Capabilities not present in this map have no automated postcondition and
// will surface as VerificationUnknown on the audit record.
func CapabilityPostconditions() map[string]CapabilityPostcondition {
	out := make(map[string]CapabilityPostcondition, len(capabilityPostconditions))
	for k, v := range capabilityPostconditions {
		copyChecks := make([]PostconditionCheck, len(v.Checks))
		copy(copyChecks, v.Checks)
		v.Checks = copyChecks
		out[k] = v
	}
	return out
}

// LookupCapabilityPostcondition returns the postcondition for the given
// capability name, or false if no postcondition is registered.
func LookupCapabilityPostcondition(capability string) (CapabilityPostcondition, bool) {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return CapabilityPostcondition{}, false
	}
	p, ok := capabilityPostconditions[capability]
	if !ok {
		return CapabilityPostcondition{}, false
	}
	copyChecks := make([]PostconditionCheck, len(p.Checks))
	copy(copyChecks, p.Checks)
	p.Checks = copyChecks
	return p, true
}

// CapabilityPostconditionNames returns the sorted list of capabilities with
// registered postconditions. Useful for diagnostics and tests that need a
// deterministic order.
func CapabilityPostconditionNames() []string {
	names := make([]string, 0, len(capabilityPostconditions))
	for k := range capabilityPostconditions {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// capabilityPostconditions is the closed registry of postconditions the
// verifier substrate knows how to evaluate. New tool capabilities must add
// an entry here AND a corresponding test in verifier_postconditions_test.go
// so a missing wiring shows up immediately.
var capabilityPostconditions = map[string]CapabilityPostcondition{
	"qm.start": {
		Capability:  "qm.start",
		VerifyRead:  "qm status <vmid>",
		Window:      defaultVerifyWindow,
		Description: "Proxmox VM transitioned to running after start command",
		Checks: []PostconditionCheck{
			{Field: FieldVMStatus, Comparator: CompareEquals, Expected: "running"},
		},
	},
	"pct.start": {
		Capability:  "pct.start",
		VerifyRead:  "pct status <vmid>",
		Window:      defaultVerifyWindow,
		Description: "Proxmox CT transitioned to running after start command",
		Checks: []PostconditionCheck{
			{Field: FieldContainerStatus, Comparator: CompareEquals, Expected: "running"},
		},
	},
	"docker.restart": {
		Capability:  "docker.restart",
		VerifyRead:  "docker inspect <container>",
		Window:      defaultVerifyWindow,
		Description: "Docker container is running and last_started is no earlier than the action start",
		Checks: []PostconditionCheck{
			{Field: FieldDockerStatus, Comparator: CompareEquals, Expected: "running"},
			{Field: FieldDockerLastStarted, Comparator: CompareAfterOrEqualActionStart},
		},
	},
	"systemctl.restart": {
		Capability:  "systemctl.restart",
		VerifyRead:  "systemctl show <unit>",
		Window:      defaultVerifyWindow,
		Description: "Systemd unit is active/running and ActiveEnterTimestamp is no earlier than the action start",
		Checks: []PostconditionCheck{
			{Field: FieldUnitActiveState, Comparator: CompareEquals, Expected: "active"},
			{Field: FieldUnitSubState, Comparator: CompareEquals, Expected: "running"},
			{Field: FieldUnitActiveEnterTimestamp, Comparator: CompareAfterOrEqualActionStart},
		},
	},
	"kubectl.rollout": {
		Capability:  "kubectl.rollout",
		VerifyRead:  "kubectl get deployment <name> -o json",
		Window:      defaultVerifyWindow,
		Description: "Deployment's readyReplicas matches desiredReplicas after the rollout window",
		Checks: []PostconditionCheck{
			{Field: FieldDeploymentReadyReplicas, Comparator: CompareEqualsField, Expected: string(FieldDeploymentDesiredReplicas)},
		},
	},
}
