package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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
// enforces on every invocation: the session control level plus an
// optional deny-infrastructure-mutations restriction (e.g. Patrol
// investigations). It is core-owned, never serialized, and consulted by
// both provider projection and runtime execution so the offered schema
// and the enforcement boundary can never disagree.
type InvocationPolicy struct {
	ControlLevel                ControlLevel
	DenyInfrastructureMutations bool
}

// Allows reports whether the policy permits an invocation class.
// Infrastructure mutations require a control level that allows control
// tools and are always blocked under the deny restriction; pulse-state
// and non-mutating invocations are not control-gated here (handlers keep
// their own defense-in-depth checks).
func (p InvocationPolicy) Allows(class agentcapabilities.InvocationClass) bool {
	if class.Mutation != agentcapabilities.MutationInfrastructure {
		return true
	}
	if p.DenyInfrastructureMutations {
		return false
	}
	return agentcapabilities.ControlLevelAllowsControlTools(p.ControlLevel)
}

// Register adds a tool to the registry. Every registered tool must have a
// canonical invocation descriptor whose cases exactly cover the schema
// enum of its discriminator; a tool that cannot be classified must not be
// registerable, so this panics on programmer error rather than degrading
// to an unclassified (and therefore ungovernable) tool.
func (r *ToolRegistry) Register(tool RegisteredTool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool.Definition = tool.Definition.NormalizeCollections()
	name := tool.Definition.Name
	descriptor := agentcapabilities.InvocationDescriptor{}
	if tool.Invocation != nil {
		descriptor = *tool.Invocation
	} else {
		canonical, ok := agentcapabilities.InvocationDescriptorFor(name)
		if !ok {
			panic(fmt.Sprintf("tool %q has no canonical invocation descriptor; declare one in agentcapabilities/invocation.go", name))
		}
		descriptor = canonical
	}
	if err := descriptor.Validate(name, discriminatorEnum(tool.Definition, descriptor.Discriminator)); err != nil {
		panic(err.Error())
	}
	tool.Invocation = &descriptor
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
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
		if !policy.Allows(*descriptor.Static) {
			return RegisteredTool{}, false
		}
		return tool, true
	}

	property, ok := tool.Definition.InputSchema.Properties[descriptor.Discriminator]
	if !ok {
		return RegisteredTool{}, false
	}
	allowed := make([]string, 0, len(property.Enum))
	sawWrite := false
	sawRead := false
	for _, value := range property.Enum {
		class := descriptor.Classify(map[string]interface{}{descriptor.Discriminator: value})
		if !policy.Allows(class) {
			continue
		}
		allowed = append(allowed, value)
		if class.Kind == agentcapabilities.ToolCallKindWrite {
			sawWrite = true
		} else {
			sawRead = true
		}
	}
	if len(allowed) == 0 {
		return RegisteredTool{}, false
	}
	if len(allowed) == len(property.Enum) {
		return tool, true
	}

	projected := tool
	projected.Definition.InputSchema.Properties = make(map[string]PropertySchema, len(tool.Definition.InputSchema.Properties))
	for key, value := range tool.Definition.InputSchema.Properties {
		projected.Definition.InputSchema.Properties[key] = value
	}
	property.Enum = allowed
	projected.Definition.InputSchema.Properties[descriptor.Discriminator] = property

	switch {
	case sawWrite && sawRead:
		projected.Governance.ActionMode = agentcapabilities.ActionModeMixed
	case sawWrite:
		projected.Governance.ActionMode = agentcapabilities.ActionModeWrite
	default:
		projected.Governance.ActionMode = agentcapabilities.ActionModeRead
	}
	return projected, true
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
	if class.Mutation == agentcapabilities.MutationInfrastructure {
		if e.invocationPolicy().DenyInfrastructureMutations {
			return agentcapabilities.NewInvocationBlockedToolResult(name, class), nil
		}
		if !agentcapabilities.ControlLevelAllowsControlTools(e.controlLevel) {
			return agentcapabilities.NewControlToolsDisabledToolResult(), nil
		}
	}

	// Legacy tool-level control check, kept as defense in depth.
	if tool.RequireControl {
		if !agentcapabilities.ControlLevelAllowsControlTools(e.controlLevel) {
			return agentcapabilities.NewControlToolsDisabledToolResult(), nil
		}
	}

	result, err := tool.Handler(ctx, e, args)
	return result.NormalizeCollections(), err
}
