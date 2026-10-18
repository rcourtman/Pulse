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
//
// CRITICAL ORDERING: Topology resolution (state.ResolveResource) happens FIRST.
// Agent hostname matching is a FALLBACK only when the state doesn't know the resource.
// This prevents the "hostname collision" bug where an agent with hostname matching an LXC name
// causes commands to execute on the node instead of inside the LXC via pct exec.
func (e *PulseToolExecutor) resolveTargetForCommandFull(targetHost string) CommandRoutingResult {
	result := CommandRoutingResult{
		TargetType: "agent",
		Transport:  "direct",
	}

	var agents []agentexec.ConnectedAgent
	if e.agentServer != nil {
		agents = e.agentServer.GetConnectedAgents()
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
	loc := e.resolveResourceLocation(targetHost)

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
		return e.findAgentForNode(reference)
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
			nodeAgentID := nodeAgent()
			result.ResolvedKind = "system-container"
			result.ResolvedNode = loc.Node
			result.TargetType = "container"
			result.TargetID = fmt.Sprintf("%d", loc.VMID)
			result.Transport = "pct_exec"
			if nodeAgentID != "" {
				result.AgentID = nodeAgentID
				for _, agent := range agents {
					if agent.AgentID == nodeAgentID {
						result.AgentHostname = agent.Hostname
						break
					}
				}
			}
			return result

		case "vm":
			// VM - route through node agent via qm guest exec
			nodeAgentID := nodeAgent()
			result.ResolvedKind = "vm"
			result.ResolvedNode = loc.Node
			result.TargetType = "vm"
			result.TargetID = fmt.Sprintf("%d", loc.VMID)
			result.Transport = "qm_guest_exec"
			if nodeAgentID != "" {
				result.AgentID = nodeAgentID
				for _, agent := range agents {
					if agent.AgentID == nodeAgentID {
						result.AgentHostname = agent.Hostname
						break
					}
				}
			}
			return result

		case "app-container", "docker-host":
			// Docker container or Docker host
			result.ResolvedKind = loc.ResourceType
			result.ResolvedNode = loc.Node

			if loc.DockerHostType == "system-container" {
				nodeAgentID := nodeAgent()
				result.TargetType = "container"
				result.TargetID = fmt.Sprintf("%d", loc.DockerHostVMID)
				result.Transport = "pct_exec"
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					for _, agent := range agents {
						if agent.AgentID == nodeAgentID {
							result.AgentHostname = agent.Hostname
							break
						}
					}
				}
				return result
			}
			if loc.DockerHostType == "vm" {
				nodeAgentID := nodeAgent()
				result.TargetType = "vm"
				result.TargetID = fmt.Sprintf("%d", loc.DockerHostVMID)
				result.Transport = "qm_guest_exec"
				if nodeAgentID != "" {
					result.AgentID = nodeAgentID
					for _, agent := range agents {
						if agent.AgentID == nodeAgentID {
							result.AgentHostname = agent.Hostname
							break
						}
					}
				}
				return result
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

func (e *PulseToolExecutor) findAgentForNode(reference string) string {
	if e.agentServer == nil {
		return ""
	}
	agents := e.agentServer.GetConnectedAgents()
	rs, err := e.readStateForControl()
	if err != nil {
		if agent, found := uniqueConnectedAgent(agents, reference); found {
			return agent.AgentID
		}
		return ""
	}
	var nodeID, hostname string
	for _, node := range rs.Nodes() {
		if node != nil && node.ID() == reference {
			nodeID, hostname = node.ID(), node.NodeName()
			break
		}
	}
	if nodeID == "" {
		for _, node := range rs.Nodes() {
			if node == nil || !unifiedresources.HostnamesEquivalent(node.NodeName(), reference) {
				continue
			}
			if nodeID != "" && nodeID != node.ID() {
				return ""
			}
			nodeID, hostname = node.ID(), node.NodeName()
		}
	}
	if nodeID == "" {
		if agent, found := uniqueConnectedAgent(agents, reference); found {
			return agent.AgentID
		}
		return ""
	}
	owned := unifiedresources.ResolveResourceContext(rs, nodeID)
	if owned.Resource != nil && owned.Resource.Agent != nil && owned.Resource.Agent.AgentID != "" {
		for _, agent := range agents {
			if agent.AgentID == owned.Resource.Agent.AgentID {
				return agent.AgentID
			}
		}
		return ""
	}
	candidate := ""
	for _, host := range rs.Hosts() {
		if host == nil || host.LinkedNodeID() == "" {
			continue
		}
		linkedID := host.LinkedNodeID()
		if resolver, ok := rs.(unifiedresources.ReferenceReadState); ok {
			_, id, found := resolver.GetByReference(linkedID)
			if found {
				linkedID = id
			}
		}
		if linkedID != nodeID {
			continue
		}
		agent, found := connectedAgentByID(agents, host.AgentID())
		if !found {
			continue
		}
		if candidate != "" && candidate != agent.AgentID {
			return ""
		}
		candidate = agent.AgentID
	}
	if candidate != "" {
		return candidate
	}
	// A hostname can route an old unlinked node only when both sides are unique.
	for _, node := range rs.Nodes() {
		if node != nil && node.ID() != nodeID && unifiedresources.HostnamesEquivalent(node.NodeName(), hostname) {
			return ""
		}
	}
	if agent, found := uniqueConnectedAgent(agents, hostname); found {
		return agent.AgentID
	}
	return ""
}

// Formatting helpers for control tools
