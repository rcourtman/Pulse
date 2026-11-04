package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerControlTools registers the pulse_control tool
func (e *PulseToolExecutor) registerControlTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name:        agentcapabilities.PulseControlToolName,
			Description: `Plan one typed action for a canonical resource that explicitly advertises the requested capability. The plan is persisted on Pulse's shared action lifecycle; this tool never executes commands or contacts infrastructure directly. Proxmox VM and LXC lifecycle actions do not require the QEMU guest agent; they use the advertised hypervisor capability.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Canonical control type. Only resource is supported.",
						Enum:        []string{"resource"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Discovered resource name or canonical resource ID from pulse_query",
					},
					"action": {
						Type:        "string",
						Description: "Advertised resource capability name, such as start, stop, shutdown, reboot, or restart. Proxmox lifecycle capabilities operate at the hypervisor boundary and do not require in-guest QEMU agent functionality.",
						Enum:        []string{"start", "stop", "shutdown", "reboot", "restart"},
					},
				},
				Required: []string{"type", "resource_id", "action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeControl(ctx, args)
		},
		RequireControl: true,
		Governance: ToolGovernance{
			ActionMode:      ToolActionWrite,
			ApprovalPolicy:  ToolApprovalActionPlan,
			ApprovalSummary: "hidden in read-only mode; approval required in controlled mode",
			Summary:         "Plans shared Pulse resource actions; approval and execution stay on the canonical action lifecycle.",
		},
	})
}

// executeControl routes to the appropriate control handler based on type
func (e *PulseToolExecutor) executeControl(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	controlType, _ := args["type"].(string)
	switch controlType {
	case "resource":
		return e.executeControlResource(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("control type %q is retired and denied; use type=resource with an advertised capability", controlType)), nil
	}
}

func (e *PulseToolExecutor) executeControlResource(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceRef, _ := args["resource_id"].(string)
	resourceRef = strings.TrimSpace(resourceRef)
	action, _ := args["action"].(string)
	action = strings.TrimSpace(action)
	if resourceRef == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}
	if action == "" {
		return NewErrorResult(fmt.Errorf("action is required")), nil
	}

	// Bind the reference to a canonical resource. The legacy per-executor
	// allowed-action list is deliberately not consulted here: it predates
	// canonical capabilities (Proxmox guests advertise "reboot", the legacy
	// list only knew "restart"), and whether the action is available is the
	// action lifecycle's decision at plan time, from the resource's own
	// advertised capabilities.
	target, blocked := e.resolveControlTarget(resourceRef, action)
	if blocked != nil {
		return *blocked, nil
	}

	if e.typedActionPlanner == nil {
		return NewErrorResult(fmt.Errorf("canonical action planning is unavailable")), nil
	}
	resourceID := target.canonicalID()
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource %q has no canonical resource id", resourceRef)), nil
	}
	plan, err := e.typedActionPlanner.PlanTypedAction(ctx, e.orgID, unifiedresources.ActionRequest{
		RequestID:      actionRequestIDForInvocation(ctx),
		ResourceID:     resourceID,
		CapabilityName: action,
		Reason:         fmt.Sprintf("Assistant proposed %s for %s", action, resourceID),
		RequestedBy:    "pulse_assistant",
	})
	if err != nil {
		return controlPlanFailureResult(target, action, err), nil
	}
	capability := action
	if target.canonical != nil {
		if advertised, ok := advertisedActionName(*target.canonical, action); ok {
			capability = advertised
		}
	}
	return NewJSONResult(map[string]interface{}{
		"planned":             true,
		"plan":                plan,
		"action_url":          "/actions?action=" + url.QueryEscape(plan.ActionID),
		"execution_requested": false,
		"action_id":           plan.ActionID,
		"resource_id":         resourceID,
		"resource_name":       target.displayName(),
		"requested_action":    action,
		"capability":          capability,
		"requires_approval":   plan.RequiresApproval,
		"approval_policy":     plan.ApprovalPolicy,
		"plan_hash":           plan.PlanHash,
		"expires_at":          plan.ExpiresAt,
		"message":             "Plan saved in Pulse Actions. This tool did not request execution. Approval and execution are separate recorded steps. The current Actions review provides the approval and run controls. Read pulse_query with action=action and this action_id for the current persisted decision and outcome. Only that recorded action outcome can establish execution and independent verification.",
	}), nil
}

func resolvedResourceDisplayName(resource ResolvedResourceInfo) string {
	if resource == nil {
		return ""
	}
	aliases := resource.GetAliases()
	if len(aliases) > 0 {
		if name := strings.TrimSpace(aliases[0]); name != "" {
			return name
		}
	}
	if providerUID := strings.TrimSpace(resource.GetProviderUID()); providerUID != "" {
		return providerUID
	}
	return strings.TrimSpace(resource.GetResourceID())
}

// Helper methods for control tools

// CommandRoutingResult contains full routing information for command execution.
// This provides the provenance needed to verify where commands actually run.
type CommandRoutingResult struct {
	// Routing info for agent
	AgentID    string // The agent that will execute the command
	TargetType string // "agent", "container", or "vm"
	TargetID   string // VMID for LXC/VM, empty for agent

	// Provenance info
	AgentHostname string // Hostname of the agent
	ResolvedKind  string // Technology/transport kind: "node", "system-container", "vm", "app-container", "docker-host", "agent" (drives routing decisions)
	ResolvedNode  string // Hypervisor node name (if applicable)
	Transport     string // How command will be executed: "direct", "pct_exec", "qm_guest_exec"
}

// resolveTargetForCommandFull resolves a target_host to full routing info including provenance.
// Use this for write operations where you need to verify execution context.
func (e *PulseToolExecutor) resolveTargetForCommandFull(targetHost string) CommandRoutingResult {
	var agents []agentexec.ConnectedAgent
	if e.agentServer != nil {
		agents = e.agentServer.GetConnectedAgents()
	}
	// With no read state, a target can only match a connected agent.
	rs, _ := e.readStateForControl()
	return routeCommandTarget(rs, agents, targetHost)
}

// routeCommandTarget decides where a command for a target goes. pulse_query flags
// ask it too, so a flag cannot disagree with what a command would do.
//
// CRITICAL ORDERING: Topology resolution (state.ResolveResource) happens FIRST.
// Agent hostname matching is a FALLBACK only when the state doesn't know the resource.
// This prevents the "hostname collision" bug where an agent with hostname matching an LXC name
// causes commands to execute on the node instead of inside the LXC via pct exec.
func routeCommandTarget(rs unifiedresources.ReadState, agents []agentexec.ConnectedAgent, targetHost string) CommandRoutingResult {
	result := CommandRoutingResult{
		TargetType: "agent",
		Transport:  "direct",
	}

	if targetHost == "" {
		// No target_host specified - require exactly one agent or fail
		if len(agents) != 1 {
			return result
		}
		result.AgentID = agents[0].AgentID
		result.AgentHostname = agents[0].Hostname
		result.ResolvedKind = "agent"
		return result
	}

	// STEP 1: Consult topology (state) FIRST — this is authoritative.
	// If the state knows about this resource, use topology-based routing.
	// This prevents hostname collisions from masquerading as host targets.
	loc := resourceLocationInReadState(rs, targetHost)

	if loc.Ambiguous {
		result.ResolvedKind = "ambiguous"
		return result
	}
	nodeAgent := func() string {
		if loc.AgentID != "" {
			for _, agent := range agents {
				if agent.AgentID == loc.AgentID {
					return agent.AgentID
				}
			}
			return ""
		}
		reference := loc.NodeResourceID
		if reference == "" {
			reference = loc.Node
		}
		agent, _ := connectedAgentForNodeReference(rs, reference, agents)
		return agent.AgentID
	}
	// throughGuest sends a command inside a guest through its node's agent. With no
	// node agent connected, an agent known by identity to run in that guest can take
	// it directly. It runs in the same place, and no name can select that agent.
	throughGuest := func(targetType, transport string, vmid int) CommandRoutingResult {
		nodeAgentID := nodeAgent()
		if nodeAgentID == "" {
			if agent, ok := connectedAgentByID(agents, loc.GuestAgentID); ok {
				result.AgentID, result.AgentHostname = agent.AgentID, agent.Hostname
				return result
			}
		}
		result.TargetType, result.TargetID, result.Transport = targetType, fmt.Sprintf("%d", vmid), transport
		if agent, ok := connectedAgentByID(agents, nodeAgentID); ok {
			result.AgentID, result.AgentHostname = agent.AgentID, agent.Hostname
		}
		return result
	}
	if loc.Found {
		result.ResolvedKind = loc.ResourceType
		// Route based on resource type
		switch loc.ResourceType {
		case "agent":
			reference := loc.AgentID
			if reference == "" {
				reference = loc.TargetID
			}
			if reference == "" {
				reference = loc.TargetHost
			}
			var agent agentexec.ConnectedAgent
			var ok bool
			if loc.AgentID != "" || loc.TargetID != "" {
				agent, ok = connectedAgentByID(agents, reference)
			} else {
				agent, ok = uniqueConnectedAgent(agents, reference)
			}
			if ok {
				result.AgentID, result.AgentHostname = agent.AgentID, agent.Hostname
			}
			return result

		case "node":
			// Direct hypervisor node
			nodeAgentID := nodeAgent()
			result.AgentID = nodeAgentID
			result.ResolvedKind = "node"
			result.ResolvedNode = loc.Node
			for _, agent := range agents {
				if agent.AgentID == nodeAgentID {
					result.AgentHostname = agent.Hostname
					break
				}
			}
			return result

		case "system-container":
			// System container - route through node agent via pct exec
			result.ResolvedNode = loc.Node
			return throughGuest("container", "pct_exec", loc.VMID)

		case "vm":
			// VM - route through node agent via qm guest exec
			result.ResolvedNode = loc.Node
			return throughGuest("vm", "qm_guest_exec", loc.VMID)

		case "app-container", "docker-host":
			// Docker container or Docker host
			result.ResolvedKind = loc.ResourceType
			result.ResolvedNode = loc.Node

			if loc.DockerHostType == "system-container" {
				return throughGuest("container", "pct_exec", loc.DockerHostVMID)
			}
			if loc.DockerHostType == "vm" {
				return throughGuest("vm", "qm_guest_exec", loc.DockerHostVMID)
			}
			// Standalone parents keep an explicit agent identity when available.
			reference := loc.AgentID
			if reference == "" {
				reference = loc.TargetHost
			}
			var agent agentexec.ConnectedAgent
			var ok bool
			if loc.AgentID != "" {
				agent, ok = connectedAgentByID(agents, reference)
			} else {
				agent, ok = uniqueConnectedAgent(agents, reference)
			}
			if ok {
				result.AgentID, result.AgentHostname = agent.AgentID, agent.Hostname
			}

		}
		// A known resource must never fall through to a different hostname match.
		return result
	}

	// A connected agent absent from the inventory is a bounded legacy boundary.
	if agent, ok := uniqueConnectedAgent(agents, targetHost); ok {
		result.AgentID, result.AgentHostname, result.ResolvedKind = agent.AgentID, agent.Hostname, "agent"
	}

	return result
}

// unavailableCommandConnection preserves monitoring context without inferring
// installation, permission, or guest capability from an absent connection.
func unavailableCommandConnection(target string, routing CommandRoutingResult) CallToolResult {
	if routing.ResolvedKind == "ambiguous" {
		return NewToolResponseResult(ToolResponse{
			OK: false,
			Error: &ToolError{
				Code: ErrCodeStrictResolution, Failed: true,
				Message: fmt.Sprintf("Target %q matches multiple monitored resources. Select the exact canonical resource ID from the inventory. The requested operation did not run.", target),
				Details: map[string]any{"target": target, "resource_kind": "ambiguous"},
			},
		})
	}
	message := "No command connection is available. Specify a target with diagnostic access."
	details := map[string]any{"target": target}
	if target != "" {
		message = fmt.Sprintf("No command connection is available for %q. Check its diagnostic access and connection status.", target)
	}
	if routing.ResolvedKind != "" {
		details["resource_kind"] = routing.ResolvedKind
		message = fmt.Sprintf("Pulse monitoring knows %q, but no command connection is available for this target.", target)
		if routing.ResolvedNode != "" {
			details["parent_node"] = routing.ResolvedNode
			message += fmt.Sprintf(" Check diagnostic access and connection status on its node %q.", routing.ResolvedNode)
		} else {
			message += " Check its diagnostic access and connection status."
		}
	}
	message += " The requested operation did not run."
	return NewToolResponseResult(ToolResponse{
		OK:    false,
		Error: &ToolError{Code: ErrCodeNoAgent, Message: message, Failed: true, Details: details},
	})
}

func (e *PulseToolExecutor) readStateForControl() (unifiedresources.ReadState, error) {
	if rs := e.getReadState(); rs != nil {
		return rs, nil
	}
	return nil, fmt.Errorf("read state not available")
}

func connectedAgentByID(agents []agentexec.ConnectedAgent, id string) (agentexec.ConnectedAgent, bool) {
	if id != "" {
		for _, agent := range agents {
			if agent.AgentID == id {
				return agent, true
			}
		}
	}
	return agentexec.ConnectedAgent{}, false
}

func uniqueConnectedAgent(agents []agentexec.ConnectedAgent, reference string) (agentexec.ConnectedAgent, bool) {
	if strings.TrimSpace(reference) == "" {
		return agentexec.ConnectedAgent{}, false
	}
	for _, agent := range agents {
		if agent.AgentID == reference {
			return agent, true
		}
	}
	var result agentexec.ConnectedAgent
	found := false
	for _, agent := range agents {
		if !unifiedresources.HostnamesEquivalent(agent.Hostname, reference) {
			continue
		}
		if found && result.AgentID != agent.AgentID {
			return agentexec.ConnectedAgent{}, false
		}
		result, found = agent, true
	}
	return result, found
}

// connectedAgentForNodeReference answers for a canonical node ID, or for a bare
// node name with no node record behind it.
func connectedAgentForNodeReference(rs unifiedresources.ReadState, reference string, agents []agentexec.ConnectedAgent) (agentexec.ConnectedAgent, bool) {
	for _, node := range rs.Nodes() {
		if node != nil && node.ID() == reference {
			return connectedAgentForNode(rs, node, agents)
		}
	}
	if !bareNodeNameCanRoute(rs, reference) {
		return agentexec.ConnectedAgent{}, false
	}
	return uniqueConnectedAgent(agents, reference)
}

// connectedAgentForNode names the connected agent that runs commands on a known
// node. Topology and pulse_query report the same answer, so their hints match
// where a command goes.
func connectedAgentForNode(rs unifiedresources.ReadState, node *unifiedresources.NodeView, agents []agentexec.ConnectedAgent) (agentexec.ConnectedAgent, bool) {
	if node == nil || len(agents) == 0 {
		return agentexec.ConnectedAgent{}, false
	}
	nodeID := node.ID()
	owned := unifiedresources.ResolveResourceContext(rs, nodeID)
	if owned.Resource != nil && owned.Resource.Agent != nil && owned.Resource.Agent.AgentID != "" {
		return connectedAgentByID(agents, owned.Resource.Agent.AgentID)
	}
	var candidate agentexec.ConnectedAgent
	linked, found := false, false
	for _, host := range rs.Hosts() {
		if host == nil || host.LinkedNodeID() == "" {
			continue
		}
		if canonicalLinkedResourceID(rs, host.LinkedNodeID()) != nodeID {
			continue
		}
		linked = true
		agent, connected := connectedAgentByID(agents, host.AgentID())
		if !connected {
			continue
		}
		if found && candidate.AgentID != agent.AgentID {
			return agentexec.ConnectedAgent{}, false
		}
		candidate, found = agent, true
	}
	// A linked node belongs to its agent. While that agent is offline, another
	// machine with the same hostname cannot stand in for it.
	if found || linked {
		return candidate, found
	}
	// A hostname can route an old unlinked node only when both sides are unique.
	hostname := firstNonEmptyString(node.NodeName(), node.Name())
	for _, other := range rs.Nodes() {
		if other != nil && other.ID() != nodeID && unifiedresources.HostnamesEquivalent(other.NodeName(), hostname) {
			return agentexec.ConnectedAgent{}, false
		}
	}
	return uniqueConnectedAgent(agents, hostname)
}

// canonicalLinkedResourceID turns the source ID an agent's link holds into the
// canonical ID its resource goes by.
func canonicalLinkedResourceID(rs unifiedresources.ReadState, link string) string {
	if resolver, ok := rs.(unifiedresources.ReferenceReadState); ok {
		if _, id, resolved := resolver.GetByReference(link); resolved {
			return id
		}
	}
	return link
}

// bareNodeNameCanRoute reports whether a node name with no node record behind it
// still stands for one machine. Placement already tried to find the guest's
// node, so a node with this name is some other machine, and guests naming it
// from two connections mean two machines.
func bareNodeNameCanRoute(rs unifiedresources.ReadState, name string) bool {
	for _, node := range rs.Nodes() {
		if node != nil && unifiedresources.HostnamesEquivalent(node.NodeName(), name) {
			return false
		}
	}
	connections := make(map[string]struct{})
	for _, vm := range rs.VMs() {
		if vm != nil && unifiedresources.HostnamesEquivalent(vm.Node(), name) {
			connections[vm.Instance()] = struct{}{}
		}
	}
	for _, ct := range rs.Containers() {
		if ct != nil && unifiedresources.HostnamesEquivalent(ct.Node(), name) {
			connections[ct.Instance()] = struct{}{}
		}
	}
	return len(connections) <= 1
}

// Formatting helpers for control tools
