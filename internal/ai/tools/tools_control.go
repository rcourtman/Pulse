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

	if loc.Found {
		result.ResolvedKind = loc.ResourceType
		// Route based on resource type
		switch loc.ResourceType {
		case "agent":
			for _, agent := range agents {
				if unifiedresources.HostnamesEquivalent(agent.Hostname, loc.TargetHost) || agent.AgentID == loc.TargetID {
					result.AgentID = agent.AgentID
					result.AgentHostname = agent.Hostname
					result.ResolvedKind = "agent"
					return result
				}
			}

		case "node":
			// Direct hypervisor node
			nodeAgentID := e.findAgentForNode(loc.Node)
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
			nodeAgentID := e.findAgentForNode(loc.Node)
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
			nodeAgentID := e.findAgentForNode(loc.Node)
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
				nodeAgentID := e.findAgentForNode(loc.Node)
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
				nodeAgentID := e.findAgentForNode(loc.Node)
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
			// Standalone Docker host - find agent directly
			for _, agent := range agents {
				if unifiedresources.HostnamesEquivalent(agent.Hostname, loc.TargetHost) || agent.AgentID == loc.TargetHost {
					result.AgentID = agent.AgentID
					result.AgentHostname = agent.Hostname
					return result
				}
			}
		}
		// A known resource must never fall through to a different hostname match.
		return result
	}

	// STEP 2: FALLBACK — agent hostname match.
	// Only used when the state doesn't know about this resource at all.
	// This handles standalone hosts without Proxmox topology.
	for _, agent := range agents {
		if unifiedresources.HostnamesEquivalent(agent.Hostname, targetHost) || agent.AgentID == targetHost {
			result.AgentID = agent.AgentID
			result.AgentHostname = agent.Hostname
			result.ResolvedKind = "agent"
			return result
		}
	}

	return result
}

// unavailableCommandConnection preserves monitoring context without inferring
// installation, permission, or guest capability from an absent connection.
func unavailableCommandConnection(target string, routing CommandRoutingResult) CallToolResult {
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

func (e *PulseToolExecutor) findAgentForNode(nodeName string) string {
	if e.agentServer == nil {
		return ""
	}

	agents := e.agentServer.GetConnectedAgents()
	for _, agent := range agents {
		if unifiedresources.HostnamesEquivalent(agent.Hostname, nodeName) {
			return agent.AgentID
		}
	}

	rs, err := e.readStateForControl()
	if err != nil {
		return ""
	}

	// Map linked node IDs to node names for quick lookup.
	nodeNamesByID := make(map[string]string)
	for _, node := range rs.Nodes() {
		name := node.Name()
		if name == "" {
			name = node.NodeName()
		}
		if node.ID() != "" {
			nodeNamesByID[node.ID()] = name
		}
	}

	for _, host := range rs.Hosts() {
		linked := host.LinkedNodeID()
		if linked == "" {
			continue
		}
		if !unifiedresources.HostnamesEquivalent(nodeNamesByID[linked], nodeName) {
			continue
		}
		for _, agent := range agents {
			if unifiedresources.HostnamesEquivalent(agent.Hostname, host.Hostname()) || agent.AgentID == host.ID() {
				return agent.AgentID
			}
		}
	}

	return ""
}

// Formatting helpers for control tools
