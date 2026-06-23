package agentcapabilities

import (
	"fmt"
	"strings"
)

const (
	PulseWorkflowPromptTriageFleet         = "pulse_triage_fleet"
	PulseWorkflowPromptInvestigateResource = "pulse_investigate_resource"
	PulseWorkflowPromptReviewFinding       = "pulse_review_finding"
	PulseWorkflowPromptOperationsLoop      = "pulse_operations_loop"

	PulseWorkflowPromptPresentationFleet    = "fleet"
	PulseWorkflowPromptPresentationResource = "resource"
	PulseWorkflowPromptPresentationFinding  = "finding"
	PulseWorkflowPromptPresentationWorkflow = "workflow"

	// Compatibility aliases for MCP callers that already refer to the prompt
	// names through the protocol projection package.
	MCPPromptTriageFleet         = PulseWorkflowPromptTriageFleet
	MCPPromptInvestigateResource = PulseWorkflowPromptInvestigateResource
	MCPPromptReviewFinding       = PulseWorkflowPromptReviewFinding
	MCPPromptOperationsLoop      = PulseWorkflowPromptOperationsLoop
)

// PulseWorkflowPrompt describes a reusable Pulse Intelligence workflow starter.
// Protocol surfaces such as MCP project this neutral shape into their own wire
// structs instead of owning separate workflow catalogues.
type PulseWorkflowPrompt struct {
	Name             string                        `json:"name"`
	Label            string                        `json:"label,omitempty"`
	PresentationKind string                        `json:"presentationKind,omitempty"`
	Description      string                        `json:"description,omitempty"`
	Arguments        []PulseWorkflowPromptArgument `json:"arguments,omitempty"`
}

// NormalizeCollections returns a prompt with detached argument metadata.
func (p PulseWorkflowPrompt) NormalizeCollections() PulseWorkflowPrompt {
	p.Name = strings.TrimSpace(p.Name)
	p.Label = strings.TrimSpace(p.Label)
	p.PresentationKind = strings.TrimSpace(p.PresentationKind)
	p.Description = strings.TrimSpace(p.Description)
	p.Arguments = append([]PulseWorkflowPromptArgument(nil), p.Arguments...)
	if p.Arguments == nil {
		p.Arguments = []PulseWorkflowPromptArgument{}
	}
	return p
}

// PulseWorkflowPromptArgument describes an argument for a reusable workflow
// starter.
type PulseWorkflowPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PulseWorkflowPromptParams names one workflow prompt and supplies its
// string-valued arguments.
type PulseWorkflowPromptParams struct {
	Name      string
	Arguments map[string]string
}

// NormalizeCollections returns detached workflow prompt params with stable
// argument maps.
func (p PulseWorkflowPromptParams) NormalizeCollections() PulseWorkflowPromptParams {
	p.Name = strings.TrimSpace(p.Name)
	if p.Arguments == nil {
		p.Arguments = map[string]string{}
		return p
	}
	args := make(map[string]string, len(p.Arguments))
	for k, v := range p.Arguments {
		args[k] = v
	}
	p.Arguments = args
	return p
}

// PulseWorkflowPromptResult is a rendered workflow starter before any protocol
// surface wraps it in its own message/content envelope.
type PulseWorkflowPromptResult struct {
	Description string
	Text        string
}

// PulseWorkflowPromptRenderOptions lets protocol surfaces add affordance-
// specific access hints while leaving the workflow decision tree in the shared
// Pulse Intelligence contract.
type PulseWorkflowPromptRenderOptions struct {
	ResourceContextInstruction func(resourceID string) string
}

// ManifestPulseWorkflowPrompts returns the manifest-owned workflow prompt
// catalogue. Older manifests that predate workflowPrompts are projected from
// capabilities for compatibility, but present workflowPrompts always win.
func ManifestPulseWorkflowPrompts(manifest Manifest) []PulseWorkflowPrompt {
	if manifest.WorkflowPrompts != nil {
		return ClonePulseWorkflowPrompts(manifest.WorkflowPrompts)
	}
	return ProjectPulseWorkflowPrompts(manifest.Capabilities)
}

// ProjectPulseWorkflowPrompts projects canonical manifest capabilities into
// reusable Pulse Intelligence workflow starters. The prompts contain no separate
// business logic; they guide agents toward the same capability names and
// resource identities already advertised by the manifest.
func ProjectPulseWorkflowPrompts(capabilities []Capability) []PulseWorkflowPrompt {
	prompts := []PulseWorkflowPrompt{}
	if _, ok := FindCapability(capabilities, FleetContextCapabilityName); ok {
		prompts = append(prompts, PulseWorkflowPrompt{
			Name:             PulseWorkflowPromptTriageFleet,
			Label:            "Triage fleet",
			PresentationKind: PulseWorkflowPromptPresentationFleet,
			Description:      "Triage the Pulse fleet using the canonical fleet context capability, then choose where deeper investigation is warranted.",
		})
	}
	if pulseOperationsLoopPromptAvailable(capabilities) {
		prompts = append(prompts, PulseWorkflowPrompt{
			Name:             PulseWorkflowPromptOperationsLoop,
			Label:            "Ask Patrol to handle an issue",
			PresentationKind: PulseWorkflowPromptPresentationWorkflow,
			Description:      "Have Patrol investigate active findings, follow the configured Patrol mode, take approved actions, verify the outcome, and record what happened.",
		})
	}
	if _, ok := FindCapability(capabilities, ResourceContextCapabilityName); ok {
		prompts = append(prompts, PulseWorkflowPrompt{
			Name:             PulseWorkflowPromptInvestigateResource,
			Label:            "Investigate resource",
			PresentationKind: PulseWorkflowPromptPresentationResource,
			Description:      "Investigate one Pulse resource using the canonical resource context capability and resource URI projection.",
			Arguments: []PulseWorkflowPromptArgument{{
				Name:        ResourceIDArgumentName,
				Description: "Canonical Pulse resource id, such as vm:101.",
				Required:    true,
			}},
		})
	}
	if _, ok := FindCapability(capabilities, ListFindingsCapabilityName); ok {
		prompts = append(prompts, PulseWorkflowPrompt{
			Name:             PulseWorkflowPromptReviewFinding,
			Label:            "Review finding",
			PresentationKind: PulseWorkflowPromptPresentationFinding,
			Description:      "Review one Patrol finding and propose the safest governed next step.",
			Arguments: []PulseWorkflowPromptArgument{{
				Name:        FindingIDArgumentName,
				Description: "Patrol finding id returned by " + ListFindingsCapabilityName + ".",
				Required:    true,
			}},
		})
	}
	for i := range prompts {
		prompts[i] = prompts[i].NormalizeCollections()
	}
	return prompts
}

// BuildPulseWorkflowPrompt renders one shared workflow prompt from canonical
// manifest capabilities and caller-supplied string arguments. It is the
// compatibility entrypoint for older callers that predate manifest-owned
// workflowPrompts; new surface projections should pass the full manifest to
// BuildPulseWorkflowPromptFromManifest.
func BuildPulseWorkflowPrompt(capabilities []Capability, params PulseWorkflowPromptParams) (PulseWorkflowPromptResult, error) {
	return BuildPulseWorkflowPromptWithOptions(capabilities, params, PulseWorkflowPromptRenderOptions{})
}

// BuildPulseWorkflowPromptWithOptions renders one shared workflow prompt with
// optional surface-specific affordance hints. It keeps older capability-backed
// callers on the legacy fallback projection; manifest-backed surfaces should use
// BuildPulseWorkflowPromptFromManifestWithOptions.
func BuildPulseWorkflowPromptWithOptions(capabilities []Capability, params PulseWorkflowPromptParams, opts PulseWorkflowPromptRenderOptions) (PulseWorkflowPromptResult, error) {
	return BuildPulseWorkflowPromptFromManifestWithOptions(Manifest{Capabilities: capabilities}, params, opts)
}

// BuildPulseWorkflowPromptFromManifest renders one manifest-declared shared
// workflow prompt with caller-supplied string arguments.
func BuildPulseWorkflowPromptFromManifest(manifest Manifest, params PulseWorkflowPromptParams) (PulseWorkflowPromptResult, error) {
	return BuildPulseWorkflowPromptFromManifestWithOptions(manifest, params, PulseWorkflowPromptRenderOptions{})
}

// BuildPulseWorkflowPromptFromManifestWithOptions renders one shared workflow
// prompt after resolving availability from manifest-owned workflowPrompts.
func BuildPulseWorkflowPromptFromManifestWithOptions(manifest Manifest, params PulseWorkflowPromptParams, opts PulseWorkflowPromptRenderOptions) (PulseWorkflowPromptResult, error) {
	params = params.NormalizeCollections()
	if !pulseWorkflowPromptDeclared(ManifestPulseWorkflowPrompts(manifest), params.Name) {
		return PulseWorkflowPromptResult{}, fmt.Errorf("unknown prompt: %s", params.Name)
	}
	return buildPulseWorkflowPrompt(params, opts)
}

func buildPulseWorkflowPrompt(params PulseWorkflowPromptParams, opts PulseWorkflowPromptRenderOptions) (PulseWorkflowPromptResult, error) {
	switch params.Name {
	case PulseWorkflowPromptTriageFleet:
		return PulseWorkflowPromptResult{
			Description: "Pulse fleet triage",
			Text:        "Use get_fleet_context to identify resources with active findings, pending approvals, operator-state flags, or stale action history. Then choose the highest-risk resource and read deeper context before recommending any write action. Summarize what needs attention, why it matters, and the safest next governed step.",
		}, nil
	case PulseWorkflowPromptInvestigateResource:
		resourceID, err := RequiredPulseWorkflowPromptArgument(params.Arguments, ResourceIDArgumentName)
		if err != nil {
			return PulseWorkflowPromptResult{}, err
		}
		resourceInstruction := defaultPulseWorkflowResourceContextInstruction(resourceID)
		if opts.ResourceContextInstruction != nil {
			if custom := strings.TrimSpace(opts.ResourceContextInstruction(resourceID)); custom != "" {
				resourceInstruction = custom
			}
		}
		return PulseWorkflowPromptResult{
			Description: "Pulse resource investigation",
			Text:        fmt.Sprintf("Investigate Pulse resource %q. %s. Explain current state, active findings, pending approvals, recent actions, and operator-state flags. Do not execute write tools unless the operator explicitly asks for a governed action.", resourceID, resourceInstruction),
		}, nil
	case PulseWorkflowPromptReviewFinding:
		findingID, err := RequiredPulseWorkflowPromptArgument(params.Arguments, FindingIDArgumentName)
		if err != nil {
			return PulseWorkflowPromptResult{}, err
		}
		return PulseWorkflowPromptResult{
			Description: "Pulse finding review",
			Text:        fmt.Sprintf("Review Patrol finding %q. Call %s, locate that finding, then read the affected resource context when available. Explain whether to acknowledge, snooze, dismiss, resolve, or plan a governed action, and name the evidence that supports the recommendation.", findingID, ListFindingsCapabilityName),
		}, nil
	case PulseWorkflowPromptOperationsLoop:
		return PulseWorkflowPromptResult{
			Description: "Patrol issue handling",
			Text:        fmt.Sprintf("Handle an infrastructure issue with Patrol. Start with %s to understand the current Patrol work status, issue evidence, pending approvals, verified outcomes, and any Assistant or external-agent handoff activity. Use %s to find active findings, pending approvals, and stale action history. Use %s for the relevant finding and %s for the affected resource before recommending any write. Explain the issue, evidence, and risk in operator language. If a governed change is warranted, call %s first and follow the returned approval policy: ask the operator only when policy requires approval, call %s to record an approval or rejection when a decision is required, and call %s only when the plan is approved or allowed without approval. After execution, re-read Patrol status, context, and findings to verify the outcome before calling %s. If the operator or policy rejects the plan, record that decision and propose the next safest follow-up without executing.", OperationsLoopStatusCapabilityName, FleetContextCapabilityName, ListFindingsCapabilityName, ResourceContextCapabilityName, PlanActionCapabilityName, DecideActionCapabilityName, ExecuteActionCapabilityName, ResolveFindingCapabilityName),
		}, nil
	}
	return PulseWorkflowPromptResult{}, fmt.Errorf("unknown prompt: %s", params.Name)
}

// RequiredPulseWorkflowPromptArgument returns a trimmed required prompt argument
// or the stable validation error shared by prompt projections.
func RequiredPulseWorkflowPromptArgument(args map[string]string, name string) (string, error) {
	value := strings.TrimSpace(args[name])
	if value == "" {
		return "", fmt.Errorf("prompt argument %s is required", name)
	}
	return value, nil
}

func pulseWorkflowPromptDeclared(prompts []PulseWorkflowPrompt, name string) bool {
	for _, prompt := range prompts {
		if prompt.Name == name {
			return true
		}
	}
	return false
}

func defaultPulseWorkflowResourceContextInstruction(resourceID string) string {
	return fmt.Sprintf("Call %s with %s %q", ResourceContextCapabilityName, ResourceIDArgumentName, resourceID)
}

func pulseOperationsLoopPromptAvailable(capabilities []Capability) bool {
	for _, name := range OperationsLoopCapabilityNames() {
		if _, ok := FindCapability(capabilities, name); !ok {
			return false
		}
	}
	return true
}
