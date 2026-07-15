package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/mutationregistry"
)

// ToolActionMode describes the state-changing capability of a registered tool.
// It aliases the shared Pulse Intelligence capability action mode so Assistant
// and external-agent manifests cannot drift on the read/mixed/write vocabulary.
type ToolActionMode = agentcapabilities.ActionMode

const (
	ToolActionRead  ToolActionMode = agentcapabilities.ActionModeRead
	ToolActionMixed ToolActionMode = agentcapabilities.ActionModeMixed
	ToolActionWrite ToolActionMode = agentcapabilities.ActionModeWrite
)

// ToolApprovalPolicy describes whether a tool can run with its granted scope or
// must participate in Pulse's governed action-plan approval lifecycle. It
// aliases the shared Pulse Intelligence approval vocabulary so Assistant
// prompts and external-agent manifests cannot drift.
type ToolApprovalPolicy = agentcapabilities.ApprovalPolicy

const (
	ToolApprovalScopeOnly  ToolApprovalPolicy = agentcapabilities.ApprovalPolicyScopeOnly
	ToolApprovalActionPlan ToolApprovalPolicy = agentcapabilities.ApprovalPolicyActionPlan
)

// ControlLevel represents the Assistant permission level for infrastructure
// control. It aliases the shared Pulse Intelligence control vocabulary so
// Assistant tool availability and external-agent adapters cannot drift.
type ControlLevel = agentcapabilities.ControlLevel

const (
	// ControlLevelReadOnly - AI can only query, no control tools available
	ControlLevelReadOnly ControlLevel = agentcapabilities.ControlLevelReadOnly
	// ControlLevelControlled - AI can execute with per-command approval
	ControlLevelControlled ControlLevel = agentcapabilities.ControlLevelControlled
	// ControlLevelAutonomous - AI executes without approval (requires Pro license)
	ControlLevelAutonomous ControlLevel = agentcapabilities.ControlLevelAutonomous
)

// ToolGovernance records the operator-facing governance contract for a tool.
// It aliases the shared Pulse Intelligence shape so Assistant and
// external-agent governance descriptors cannot drift.
type ToolGovernance = agentcapabilities.ToolGovernance

// ToolGovernanceDescriptor is the read-only manifest used by Assistant prompts
// and action-governance surfaces.
type ToolGovernanceDescriptor = agentcapabilities.ToolGovernanceDescriptor

// ToolHandler is a function that handles tool execution
type ToolHandler func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error)

// RegisteredTool combines a tool definition with its handler
type RegisteredTool struct {
	Definition     Tool
	Handler        ToolHandler
	RequireControl bool // If true, only available when control level is not read_only
	Governance     ToolGovernance
	// Invocation optionally overrides the canonical invocation
	// descriptor. Canonical Pulse tools must leave it nil (their
	// descriptor comes from the shared agentcapabilities table); it
	// exists for test doubles and extension tools that are not part of
	// the canonical manifest. Register resolves and stores the
	// effective descriptor, so execution and projection always consult
	// the same classification the registration validated.
	Invocation *agentcapabilities.InvocationDescriptor
}

// ToolRegistry manages tool registration and execution
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]RegisteredTool
	order []string // Preserve registration order
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]RegisteredTool),
		order: make([]string, 0),
	}
}

// InvocationPolicy is the request-scoped safety policy the registry
// enforces on every invocation: the session control level plus the
// execution profile's restrictions (deny infrastructure mutations, and
// optionally an allowlist of tools permitted to mutate Pulse state). It
// is core-owned, never serialized, and consulted by both provider
// projection and runtime execution so the offered schema and the
// enforcement boundary can never disagree.
type InvocationPolicy struct {
	ControlLevel ControlLevel
	// Execute authority is bound at authenticated transport boundaries.
	// Unbound policies are reserved for trusted internal/test callers;
	// external request paths must always bind an explicit true/false value.
	HasExecuteAuthority         bool
	ExecuteAuthorityBound       bool
	DenyInfrastructureMutations bool
	// PulseStateAllowlist restricts pulse-state mutations to the named
	// tools. Nil and an empty map both deny all pulse-state mutations. An
	// allowlist rather than a boolean: Patrol detection must record and
	// resolve findings without also being able to dismiss alerts or
	// write knowledge.
	PulseStateAllowlist map[string]bool
	// Profile is the execution posture; profile-restricted tools (the
	// investigation-only proposal capture) key on it.
	Profile ExecutionProfile
}

func clonePulseStateAllowlist(allowlist map[string]bool) map[string]bool {
	if allowlist == nil {
		return nil
	}
	clone := make(map[string]bool, len(allowlist))
	for name, allowed := range allowlist {
		clone[name] = allowed
	}
	return clone
}

// Allows reports whether the policy permits an invocation class for the
// named tool. Infrastructure mutations require a control level that
// allows control tools and are always blocked under the deny restriction;
// pulse-state mutations honor the profile allowlist; non-mutating
// invocations are not gated here (handlers keep their own
// defense-in-depth checks). Unknown mutation targets are denied outright,
// independent of registration validation, so a class that somehow
// bypasses Validate still cannot execute.
func (p InvocationPolicy) Allows(toolName string, class agentcapabilities.InvocationClass) bool {
	if !p.Profile.Valid() || !class.Valid() {
		return false
	}
	// Proposal and action-catalog tools are investigation-profile-only: a
	// fabricated call under any other posture is rejected here, before the
	// handler, and the same check keeps both out of every other profile's
	// projected manifest.
	if isPatrolInvestigationOnlyTool(toolName) && p.Profile != ProfilePatrolInvestigation {
		return false
	}
	switch class.Mutation {
	case agentcapabilities.MutationNone:
		return true
	case agentcapabilities.MutationPulseState:
		return p.PulseStateAllowlist[toolName]
	case agentcapabilities.MutationInfrastructure:
		if p.Profile != ProfileInteractiveAssistant || (p.ExecuteAuthorityBound && !p.HasExecuteAuthority) || p.DenyInfrastructureMutations {
			return false
		}
		return agentcapabilities.ControlLevelAllowsControlTools(p.ControlLevel)
	default:
		return false
	}
}

func isPatrolInvestigationOnlyTool(toolName string) bool {
	switch toolName {
	case agentcapabilities.PatrolProposeActionToolName, agentcapabilities.PatrolActionCapabilitiesToolName:
		return true
	default:
		return false
	}
}

// Registration authority is split so extension code can never claim or
// replace a canonical tool. registerBuiltin is the construction-time path
// for canonical Pulse tools; RegisterExtension is the only path exposed
// outside executor construction and rejects every canonical name. Both
// paths are append-only: a name registers exactly once, so a later
// registration can never swap out an already-governed handler.

// registerBuiltin adds a canonical Pulse tool during executor
// construction. The tool must classify through the shared descriptor
// table (no override) and its descriptor must validate against the schema
// enum; a tool that cannot be classified must not be registerable, so
// this panics on programmer error rather than degrading to an
// unclassified (and therefore ungovernable) tool.
func (r *ToolRegistry) registerBuiltin(tool RegisteredTool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool.Definition = tool.Definition.NormalizeCollections()
	name := tool.Definition.Name
	if tool.Invocation != nil {
		// A canonical tool name must classify through the shared table;
		// an override could silently relax its safety classification.
		panic(fmt.Sprintf("tool %q is builtin; its invocation descriptor comes from agentcapabilities/invocation.go and cannot be overridden", name))
	}
	canonical, isCanonical := agentcapabilities.InvocationDescriptorFor(name)
	if !isCanonical {
		panic(fmt.Sprintf("tool %q has no canonical invocation descriptor; declare one in agentcapabilities/invocation.go", name))
	}
	r.store(name, tool, canonical)
}

// RegisterExtension adds a non-canonical tool (tests, extensions). It
// rejects every canonical tool name outright - even without a descriptor
// override, re-registering a canonical name would swap out its governed
// handler - and requires the extension to declare its own invocation
// descriptor.
func (r *ToolRegistry) RegisterExtension(tool RegisteredTool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool.Definition = tool.Definition.NormalizeCollections()
	name := tool.Definition.Name
	if _, retiredAlias := mutationregistry.LookupModelInvocation(name, nil); retiredAlias {
		panic(fmt.Sprintf("tool %q is a retired mutation alias and cannot be registered as an extension", name))
	}
	if _, isCanonical := agentcapabilities.InvocationDescriptorFor(name); isCanonical {
		panic(fmt.Sprintf("tool %q is a canonical Pulse tool and cannot be registered as an extension", name))
	}
	if tool.Invocation == nil {
		panic(fmt.Sprintf("extension tool %q must declare its own invocation descriptor", name))
	}
	r.store(name, tool, tool.Invocation.Clone())
}

// store validates and appends one registration. Callers hold r.mu.
func (r *ToolRegistry) store(name string, tool RegisteredTool, descriptor agentcapabilities.InvocationDescriptor) {
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tool %q is already registered; registry entries are append-only", name))
	}
	if err := descriptor.Validate(name, discriminatorEnum(tool.Definition, descriptor.Discriminator)); err != nil {
		panic(err.Error())
	}
	tool.Invocation = &descriptor
	r.order = append(r.order, name)
	r.tools[name] = tool
}

// StaticInvocation builds a static invocation descriptor. Convenience for
// extension and test tool registrations that are not part of the
// canonical descriptor table.
func StaticInvocation(kind agentcapabilities.ToolCallKind, mutation agentcapabilities.MutationTarget) *agentcapabilities.InvocationDescriptor {
	class := agentcapabilities.InvocationClass{Kind: kind, Mutation: mutation}
	return &agentcapabilities.InvocationDescriptor{Static: &class}
}

// discriminatorEnum returns the schema enum for the descriptor's
// discriminator property, or nil for static descriptors.
func discriminatorEnum(definition Tool, discriminator string) []string {
	if discriminator == "" {
		return nil
	}
	property, ok := definition.InputSchema.Properties[discriminator]
	if !ok {
		return nil
	}
	return property.Enum
}

// ListTools returns all tools available under the given invocation
// policy. Mixed tools whose discriminator has forbidden subactions are
// offered with those enum values removed; tools with no permitted
// invocation are dropped entirely. The same descriptor drives runtime
// enforcement in Execute, so the offered schema and the enforcement
// boundary always agree.
func (r *ToolRegistry) ListTools(policy InvocationPolicy) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, name := range r.order {
		tool := r.tools[name]
		// Legacy tool-level gate, kept as defense in depth.
		if tool.RequireControl && !agentcapabilities.ControlLevelAllowsControlTools(policy.ControlLevel) {
			continue
		}
		projected, ok := projectToolForPolicy(tool, policy)
		if !ok {
			continue
		}
		result = append(result, projected.Definition.NormalizeCollections())
	}
	return result
}

// ListToolGovernance returns the governed tool manifest available under
// the given invocation policy, with each tool's action mode recomputed
// from the subactions the policy actually offers.
func (r *ToolRegistry) ListToolGovernance(policy InvocationPolicy) []ToolGovernanceDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolGovernanceDescriptor, 0, len(r.tools))
	for _, name := range r.order {
		tool := r.tools[name]
		if tool.RequireControl && !agentcapabilities.ControlLevelAllowsControlTools(policy.ControlLevel) {
			continue
		}
		projected, ok := projectToolForPolicy(tool, policy)
		if !ok {
			continue
		}
		result = append(result, agentcapabilities.NewToolGovernanceDescriptor(
			projected.Definition.Name,
			projected.Definition.Description,
			projected.RequireControl,
			projected.Governance,
		))
	}
	return result
}

// projectToolForPolicy filters one registered tool against the policy.
// Static tools pass or drop whole; discriminator-based tools are offered
// with forbidden enum values removed and their governance action mode
// recomputed from what remains. Returns false when no invocation of the
// tool is permitted.
func projectToolForPolicy(tool RegisteredTool, policy InvocationPolicy) (RegisteredTool, bool) {
	if tool.Invocation == nil {
		// Unregisterable in practice (Register resolves and stores the
		// descriptor), but fail closed.
		return RegisteredTool{}, false
	}
	descriptor := *tool.Invocation
	if descriptor.Static != nil {
		if !policy.Allows(tool.Definition.Name, *descriptor.Static) {
			return RegisteredTool{}, false
		}
		return applyProjectedGovernance(tool, []agentcapabilities.InvocationClass{*descriptor.Static}), true
	}

	property, ok := tool.Definition.InputSchema.Properties[descriptor.Discriminator]
	if !ok {
		return RegisteredTool{}, false
	}
	allowed := make([]string, 0, len(property.Enum))
	classes := make([]agentcapabilities.InvocationClass, 0, len(property.Enum))
	for _, value := range property.Enum {
		class := descriptor.Classify(map[string]interface{}{descriptor.Discriminator: value})
		if !policy.Allows(tool.Definition.Name, class) {
			continue
		}
		allowed = append(allowed, value)
		classes = append(classes, class)
	}
	if len(allowed) == 0 {
		return RegisteredTool{}, false
	}

	projected := tool
	if len(allowed) != len(property.Enum) {
		projected.Definition.InputSchema.Properties = make(map[string]PropertySchema, len(tool.Definition.InputSchema.Properties))
		for key, value := range tool.Definition.InputSchema.Properties {
			projected.Definition.InputSchema.Properties[key] = value
		}
		property.Enum = allowed
		projected.Definition.InputSchema.Properties[descriptor.Discriminator] = property
	}
	return applyProjectedGovernance(projected, classes), true
}

// applyProjectedGovernance recomputes the offered governance from the
// mutation targets the policy actually permits: the action mode reflects
// what the offered invocations can change (not their workflow kind), and
// a tool whose remaining invocations mutate nothing carries scope-only
// approval metadata instead of a stale action-plan requirement.
func applyProjectedGovernance(tool RegisteredTool, classes []agentcapabilities.InvocationClass) RegisteredTool {
	sawMutating := false
	sawNonMutating := false
	for _, class := range classes {
		if class.Mutation == agentcapabilities.MutationNone {
			sawNonMutating = true
		} else {
			sawMutating = true
		}
	}
	switch {
	case sawMutating && sawNonMutating:
		tool.Governance.ActionMode = agentcapabilities.ActionModeMixed
	case sawMutating:
		tool.Governance.ActionMode = agentcapabilities.ActionModeWrite
	default:
		tool.Governance.ActionMode = agentcapabilities.ActionModeRead
		if tool.Governance.ApprovalPolicy != ToolApprovalScopeOnly {
			// Downgrading from action-plan approval: the registered
			// approval summary no longer applies, so clear it and let
			// the shared normalization supply the scope-only default.
			tool.Governance.ApprovalPolicy = ToolApprovalScopeOnly
			tool.Governance.ApprovalSummary = ""
		}
	}
	return tool
}

// allNames returns the canonical list of registered tool names in
// registration order. Internal helper for KnownToolNames.
func (r *ToolRegistry) allNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Execute runs a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, e *PulseToolExecutor, name string, args map[string]interface{}) (CallToolResult, error) {
	params, invalidResult, ok := agentcapabilities.PrepareToolRegistryExecution(name, args)
	if !ok {
		return invalidResult, nil
	}
	name = params.Name
	args = params.Arguments
	if mutation, classified := mutationregistry.LookupModelInvocation(name, args); classified && mutation.Disposition == mutationregistry.DispositionRetiredDenied {
		return NewErrorResult(fmt.Errorf("mutation %s is retired and denied; use an advertised typed resource action", mutation.ID)), nil
	}

	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()

	if !exists {
		return agentcapabilities.NewUnknownToolResult(name), nil
	}
	// Invocation-level policy enforcement, before the handler runs.
	// The classification fails closed (missing/unknown discriminators
	// count as infrastructure writes), so a fabricated or hidden enum
	// value can never reach a handler under a policy that forbids it.
	class := agentcapabilities.FailClosedInvocationClass()
	if tool.Invocation != nil {
		class = tool.Invocation.Classify(args)
	}
	policy := e.invocationPolicy()
	if !policy.Allows(name, class) {
		if class.Mutation == agentcapabilities.MutationInfrastructure &&
			!policy.DenyInfrastructureMutations &&
			!agentcapabilities.ControlLevelAllowsControlTools(policy.ControlLevel) {
			// Refused purely by control level: keep the actionable
			// operator guidance for enabling control tools.
			return agentcapabilities.NewControlToolsDisabledToolResult(), nil
		}
		return agentcapabilities.NewInvocationBlockedToolResult(name, class), nil
	}

	// Legacy tool-level control check, kept as defense in depth.
	if tool.RequireControl {
		if !agentcapabilities.ControlLevelAllowsControlTools(e.controlLevel) {
			return agentcapabilities.NewControlToolsDisabledToolResult(), nil
		}
	}
	if err := agentcapabilities.ValidateDeclaredToolArguments(tool.Definition.InputSchema, args); err != nil {
		return agentcapabilities.NewInvalidToolCallParamsResult(err), nil
	}
	if name == agentcapabilities.PatrolProposeActionToolName {
		for key := range args {
			if agentcapabilities.IsInternalToolArgument(key) {
				return agentcapabilities.NewInvalidToolCallParamsResult(fmt.Errorf("argument %q is server-internal and cannot be model-authored", key)), nil
			}
		}
	}

	result, err := tool.Handler(ctx, e, args)
	return result.NormalizeCollections(), err
}
