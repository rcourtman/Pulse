// Package ai provides AI-powered diagnostic and command execution capabilities.
// This file contains the robust agent routing logic for executing commands on the correct host.
package ai

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// RoutingResult contains the result of agent routing
type RoutingResult struct {
	AgentID       string   // ID of the selected agent
	AgentHostname string   // Hostname of the selected agent
	TargetNode    string   // The node we're trying to reach
	TargetVMID    string   // The VMID (for container/VM targets)
	RoutingMethod string   // How we determined the route (for debugging)
	ClusterPeer   bool     // True if routing via a cluster peer
	Warnings      []string // Any warnings encountered during routing
}

// RoutingError represents a routing failure with actionable information
type RoutingError struct {
	TargetNode          string
	TargetVMID          int
	AvailableAgents     []string
	Reason              string
	Suggestion          string
	AskForClarification bool // If true, AI should ask the user which host to use
}

func (e *RoutingError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s. %s", e.Reason, e.Suggestion)
	}
	return e.Reason
}

// ForAI returns a message suitable for returning to the AI as a tool result
// This encourages the AI to ask the user for clarification rather than just failing
func (e *RoutingError) ForAI() string {
	if e.AskForClarification && len(e.AvailableAgents) > 0 {
		return fmt.Sprintf(
			"ROUTING_CLARIFICATION_NEEDED: %s\n\n"+
				"Available hosts: %s\n\n"+
				"Please ask the user which host they want to run this command on. "+
				"Do NOT try the command again until the user specifies which host. "+
				"Present the available hosts in a friendly way and ask them to clarify.",
			e.Reason, strings.Join(e.AvailableAgents, ", "))
	}
	return e.Error()
}

// routeToAgent determines which agent should execute a command.
// This is the authoritative routing function that should be used for all command execution.
//
// Routing priority:
// 1. VMID lookup from state (most reliable for pct/qm commands)
// 2. Explicit "node" field in context
// 3. Explicit "guest_node" field in context  
// 4. "hostname" field for host targets
// 5. VMID extracted from target ID (last resort)
//
// Agent matching is EXACT only - no substring matching to prevent false positives.
// If no direct match, cluster peer routing is attempted.
// If all else fails, returns an explicit error rather than silently using wrong agent.
func (s *Service) routeToAgent(req ExecuteRequest, command string, agents []agentexec.ConnectedAgent) (*RoutingResult, error) {
	result := &RoutingResult{}

	if len(agents) == 0 {
		return nil, &RoutingError{
			Reason:     "No agents are connected to Pulse",
			Suggestion: "Install pulse-agent on at least one host",
		}
	}

	// Build a map of available agents for quick lookup and error messages
	agentMap := make(map[string]agentexec.ConnectedAgent) // lowercase hostname -> agent
	var agentHostnames []string
	for _, agent := range agents {
		hostname := strings.TrimSpace(strings.ToLower(agent.Hostname))
		agentMap[hostname] = agent
		agentHostnames = append(agentHostnames, agent.Hostname)
	}

	// Step 1: Try VMID-based routing (most authoritative for pct/qm commands)
	if vmid, requiresOwnerNode, found := extractVMIDFromCommand(command); found && requiresOwnerNode {
		targetInstance := ""
		if inst, ok := req.Context["instance"].(string); ok {
			targetInstance = inst
		}

		guests := s.lookupGuestsByVMID(vmid, targetInstance)

		if len(guests) == 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("VMID %d not found in Pulse state - routing based on context", vmid))
		} else if len(guests) == 1 {
			result.TargetNode = strings.ToLower(guests[0].Node)
			result.RoutingMethod = "vmid_lookup"
			log.Info().
				Int("vmid", vmid).
				Str("node", guests[0].Node).
				Str("guest_name", guests[0].Name).
				Msg("Routed command via VMID state lookup")
		} else {
			// Multiple matches - try to disambiguate
			if targetInstance != "" {
				for _, g := range guests {
					if strings.EqualFold(g.Instance, targetInstance) {
						result.TargetNode = strings.ToLower(g.Node)
						result.RoutingMethod = "vmid_lookup_with_instance"
						log.Info().
							Int("vmid", vmid).
							Str("node", g.Node).
							Str("instance", g.Instance).
							Msg("Resolved VMID collision using instance")
						break
					}
				}
			}
			if result.TargetNode == "" {
				// Return explicit error for VMID collision
				var locations []string
				for _, g := range guests {
					locations = append(locations, fmt.Sprintf("%s on %s (%s)", g.Name, g.Node, g.Instance))
				}
				return nil, &RoutingError{
					TargetVMID:      vmid,
					AvailableAgents: agentHostnames,
					Reason: fmt.Sprintf("VMID %d exists on multiple nodes: %s",
						vmid, strings.Join(locations, ", ")),
					Suggestion: "Specify the instance/cluster in your query to disambiguate",
				}
			}
		}
	}

	// Step 2: Try context-based routing (explicit node information)
	if result.TargetNode == "" {
		if node, ok := req.Context["node"].(string); ok && node != "" {
			result.TargetNode = strings.ToLower(node)
			result.RoutingMethod = "context_node"
			log.Debug().
				Str("node", node).
				Str("command", command).
				Msg("Routing via explicit 'node' in context")
		} else if node, ok := req.Context["guest_node"].(string); ok && node != "" {
			result.TargetNode = strings.ToLower(node)
			result.RoutingMethod = "context_guest_node"
			log.Debug().
				Str("guest_node", node).
				Str("command", command).
				Msg("Routing via 'guest_node' in context")
		} else if req.TargetType == "host" {
			// Check multiple possible keys for hostname - frontend uses host_name
			hostname := ""
			if h, ok := req.Context["hostname"].(string); ok && h != "" {
				hostname = h
			} else if h, ok := req.Context["host_name"].(string); ok && h != "" {
				hostname = h
			}
			if hostname != "" {
				result.TargetNode = strings.ToLower(hostname)
				result.RoutingMethod = "context_hostname"
				log.Debug().
					Str("hostname", hostname).
					Str("command", command).
					Msg("Routing via hostname in context")
			} else {
				// For host target type with no node info, log a warning
				// This is a common source of routing issues
				log.Warn().
					Str("target_type", req.TargetType).
					Str("target_id", req.TargetID).
					Str("command", command).
					Msg("Host command with no node/hostname in context - may route to wrong agent")
				result.Warnings = append(result.Warnings,
					"No target host specified in context. Use target_host parameter for reliable routing.")
			}
		}
	}


	// Step 3: Extract VMID from target ID and look up in state
	if result.TargetNode == "" && req.TargetID != "" {
		if vmid := extractVMIDFromTargetID(req.TargetID); vmid > 0 {
			result.TargetVMID = strconv.Itoa(vmid)

			// Try instance from context
			targetInstance := ""
			if inst, ok := req.Context["instance"].(string); ok {
				targetInstance = inst
			}

			guests := s.lookupGuestsByVMID(vmid, targetInstance)
			if len(guests) == 1 {
				result.TargetNode = strings.ToLower(guests[0].Node)
				result.RoutingMethod = "target_id_vmid_lookup"
				log.Debug().
					Int("vmid", vmid).
					Str("node", guests[0].Node).
					Str("target_id", req.TargetID).
					Msg("Resolved node from target ID VMID lookup")
			}
		}
	}

	// Step 4: Try to find exact matching agent
	if result.TargetNode != "" {
		targetNodeClean := strings.TrimSpace(strings.ToLower(result.TargetNode))

		// EXACT match only - no substring matching
		if agent, exists := agentMap[targetNodeClean]; exists {
			result.AgentID = agent.AgentID
			result.AgentHostname = agent.Hostname
			log.Debug().
				Str("target_node", result.TargetNode).
				Str("agent", agent.Hostname).
				Str("method", result.RoutingMethod).
				Msg("Exact agent match found")
			return result, nil
		}

		// Try cluster peer routing
		if peerAgentID := s.findClusterPeerAgent(targetNodeClean, agents); peerAgentID != "" {
			for _, agent := range agents {
				if agent.AgentID == peerAgentID {
					result.AgentID = peerAgentID
					result.AgentHostname = agent.Hostname
					result.ClusterPeer = true
					log.Info().
						Str("target_node", result.TargetNode).
						Str("peer_agent", agent.Hostname).
						Msg("Routing via cluster peer agent")
					return result, nil
				}
			}
		}

		// No agent available for this node
		return nil, &RoutingError{
			TargetNode:      result.TargetNode,
			AvailableAgents: agentHostnames,
			Reason: fmt.Sprintf("No agent connected to node %q", result.TargetNode),
			Suggestion: fmt.Sprintf("Install pulse-agent on %q, or ensure it's in a cluster with %s",
				result.TargetNode, strings.Join(agentHostnames, ", ")),
		}
	}

	// Step 5: No target node determined - for host commands with no context, use first agent
	if req.TargetType == "host" && len(agents) == 1 {
		result.AgentID = agents[0].AgentID
		result.AgentHostname = agents[0].Hostname
		result.RoutingMethod = "single_agent_fallback"
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("No target node specified, using the only connected agent (%s). For multi-agent setups, specify target_host.", agents[0].Hostname))
		log.Info().
			Str("agent", agents[0].Hostname).
			Str("target_type", req.TargetType).
			Msg("Routing via single-agent fallback")
		return result, nil
	}

	// Cannot determine where to route
	// Provide actionable error with available agents listed
	log.Error().
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Strs("available_agents", agentHostnames).
		Msg("Routing failed - cannot determine target agent")

	return nil, &RoutingError{
		AvailableAgents:     agentHostnames,
		Reason:              "Cannot determine which host should execute this command",
		Suggestion:          fmt.Sprintf("Please specify which host: %s", strings.Join(agentHostnames, ", ")),
		AskForClarification: true,
	}

}

// extractVMIDFromTargetID extracts a numeric VMID from various target ID formats.
// Handles formats like:
// - "delly-minipc-106" -> 106
// - "minipc-106" -> 106
// - "106" -> 106
// - "lxc-106" -> 106
// - "vm-106" -> 106
func extractVMIDFromTargetID(targetID string) int {
	if targetID == "" {
		return 0
	}

	// Try parsing the whole thing as a number first
	if vmid, err := strconv.Atoi(targetID); err == nil && vmid > 0 {
		return vmid
	}

	// Split by hyphen and take the last numeric part
	parts := strings.Split(targetID, "-")
	for i := len(parts) - 1; i >= 0; i-- {
		if vmid, err := strconv.Atoi(parts[i]); err == nil && vmid > 0 {
			return vmid
		}
	}

	return 0
}

// findClusterPeerAgent finds an agent that can execute commands for a node in the same cluster.
// For PVE clusters, any node can execute pvesh/vzdump commands, but pct exec/qm guest exec
// require the agent to be on the specific node.
func (s *Service) findClusterPeerAgent(targetNode string, agents []agentexec.ConnectedAgent) string {
	// Check for nil persistence
	if s.persistence == nil {
		return ""
	}
	
	// Load nodes config to check cluster membership
	nodesConfig, err := s.persistence.LoadNodesConfig()
	if err != nil || nodesConfig == nil {
		return ""
	}

	// Find which cluster the target node belongs to
	var targetCluster string
	var clusterEndpoints []config.ClusterEndpoint

	for _, pve := range nodesConfig.PVEInstances {
		if strings.EqualFold(pve.Name, targetNode) {
			if pve.IsCluster && pve.ClusterName != "" {
				targetCluster = pve.ClusterName
				clusterEndpoints = pve.ClusterEndpoints
			}
			break
		}
		// Also check cluster endpoints
		for _, ep := range pve.ClusterEndpoints {
			if strings.EqualFold(ep.NodeName, targetNode) {
				if pve.IsCluster && pve.ClusterName != "" {
					targetCluster = pve.ClusterName
					clusterEndpoints = pve.ClusterEndpoints
				}
				break
			}
		}
	}

	if targetCluster == "" {
		return ""
	}

	// Build list of cluster member nodes
	clusterNodes := make(map[string]bool)
	for _, ep := range clusterEndpoints {
		clusterNodes[strings.ToLower(ep.NodeName)] = true
	}

	// Find an agent on any cluster member
	for _, agent := range agents {
		agentHostname := strings.ToLower(agent.Hostname)
		if clusterNodes[agentHostname] {
			log.Debug().
				Str("target_node", targetNode).
				Str("cluster", targetCluster).
				Str("peer_agent", agent.Hostname).
				Msg("Found cluster peer agent")
			return agent.AgentID
		}
	}

	return ""
}
