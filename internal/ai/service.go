package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to the current infrastructure state
type StateProvider interface {
	GetState() models.StateSnapshot
}

// Service orchestrates AI interactions
type Service struct {
	mu               sync.RWMutex
	persistence      *config.ConfigPersistence
	provider         providers.Provider
	cfg              *config.AIConfig
	agentServer      *agentexec.Server
	policy           *agentexec.CommandPolicy
	stateProvider    StateProvider
	alertProvider    AlertProvider
	knowledgeStore   *knowledge.Store
	resourceProvider ResourceProvider // Unified resource model provider (Phase 2)
	patrolService    *PatrolService   // Background AI monitoring service
	metadataProvider MetadataProvider // Enables AI to update resource URLs

	// Alert-triggered analysis - token-efficient real-time AI insights
	alertTriggeredAnalyzer *AlertTriggeredAnalyzer
}

// NewService creates a new AI service
func NewService(persistence *config.ConfigPersistence, agentServer *agentexec.Server) *Service {
	// Initialize knowledge store
	var knowledgeStore *knowledge.Store
	if persistence != nil {
		var err error
		knowledgeStore, err = knowledge.NewStore(persistence.DataDir())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize knowledge store")
		}
	}

	return &Service{
		persistence:    persistence,
		agentServer:    agentServer,
		policy:         agentexec.DefaultPolicy(),
		knowledgeStore: knowledgeStore,
	}
}

// SetStateProvider sets the state provider for infrastructure context
func (s *Service) SetStateProvider(sp StateProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateProvider = sp

	// Initialize patrol service if not already done
	if s.patrolService == nil && sp != nil {
		s.patrolService = NewPatrolService(s, sp)
	}

	// Initialize alert-triggered analyzer if not already done
	if s.alertTriggeredAnalyzer == nil && sp != nil && s.patrolService != nil {
		s.alertTriggeredAnalyzer = NewAlertTriggeredAnalyzer(s.patrolService, sp)
	}
}

// GetPatrolService returns the patrol service for background monitoring
func (s *Service) GetPatrolService() *PatrolService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.patrolService
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for token-efficient real-time analysis
func (s *Service) GetAlertTriggeredAnalyzer() *AlertTriggeredAnalyzer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alertTriggeredAnalyzer
}

// GetAIConfig returns the current AI configuration
func (s *Service) GetAIConfig() *config.AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// SetPatrolThresholdProvider sets the threshold provider for patrol
// This should be called with an AlertThresholdAdapter to connect patrol to user-configured thresholds
func (s *Service) SetPatrolThresholdProvider(provider ThresholdProvider) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetThresholdProvider(provider)
	}
}

// StartPatrol starts the background patrol service
func (s *Service) StartPatrol(ctx context.Context) {
	s.mu.RLock()
	patrol := s.patrolService
	alertAnalyzer := s.alertTriggeredAnalyzer
	cfg := s.cfg
	s.mu.RUnlock()

	if patrol == nil {
		log.Debug().Msg("Patrol service not initialized, cannot start")
		return
	}

	if cfg == nil || !cfg.IsPatrolEnabled() {
		log.Debug().Msg("AI Patrol not enabled")
		return
	}

	// Configure patrol from AI config
	patrolCfg := PatrolConfig{
		Enabled:              true,
		QuickCheckInterval:   cfg.GetPatrolInterval(),
		DeepAnalysisInterval: 6 * time.Hour,
		AnalyzeNodes:         cfg.PatrolAnalyzeNodes,
		AnalyzeGuests:        cfg.PatrolAnalyzeGuests,
		AnalyzeDocker:        cfg.PatrolAnalyzeDocker,
		AnalyzeStorage:       cfg.PatrolAnalyzeStorage,
	}
	patrol.SetConfig(patrolCfg)
	patrol.Start(ctx)

	// Configure alert-triggered analyzer
	if alertAnalyzer != nil {
		alertAnalyzer.SetEnabled(cfg.IsAlertTriggeredAnalysisEnabled())
		log.Info().
			Bool("enabled", cfg.IsAlertTriggeredAnalysisEnabled()).
			Msg("Alert-triggered AI analysis configured")
	}
}

// StopPatrol stops the background patrol service
func (s *Service) StopPatrol() {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.Stop()
	}
}

// ReconfigurePatrol updates the patrol configuration without restarting
// Call this after changing patrol settings to apply them immediately
func (s *Service) ReconfigurePatrol() {
	s.mu.RLock()
	patrol := s.patrolService
	alertAnalyzer := s.alertTriggeredAnalyzer
	cfg := s.cfg
	s.mu.RUnlock()

	if patrol == nil || cfg == nil {
		return
	}

	// Update patrol configuration
	patrolCfg := PatrolConfig{
		Enabled:              cfg.IsPatrolEnabled(),
		QuickCheckInterval:   cfg.GetPatrolInterval(),
		DeepAnalysisInterval: 6 * time.Hour,
		AnalyzeNodes:         cfg.PatrolAnalyzeNodes,
		AnalyzeGuests:        cfg.PatrolAnalyzeGuests,
		AnalyzeDocker:        cfg.PatrolAnalyzeDocker,
		AnalyzeStorage:       cfg.PatrolAnalyzeStorage,
	}
	patrol.SetConfig(patrolCfg)

	log.Info().
		Bool("enabled", patrolCfg.Enabled).
		Dur("interval", patrolCfg.QuickCheckInterval).
		Msg("Patrol configuration updated")

	// Update alert-triggered analyzer
	if alertAnalyzer != nil {
		alertAnalyzer.SetEnabled(cfg.IsAlertTriggeredAnalysisEnabled())
	}
}

// GuestInfo contains information about a guest (VM or container) found by VMID lookup
type GuestInfo struct {
	Node     string
	Name     string
	Type     string // "lxc" or "qemu"
	Instance string // PVE instance ID (for multi-cluster disambiguation)
}

// lookupNodeForVMID looks up which node owns a given VMID using the state provider
// Returns the node name and guest name if found, empty strings otherwise
// If targetInstance is provided, only matches guests from that instance (for multi-cluster setups)
func (s *Service) lookupNodeForVMID(vmid int) (node string, guestName string, guestType string) {
	guests := s.lookupGuestsByVMID(vmid, "")
	if len(guests) == 1 {
		return guests[0].Node, guests[0].Name, guests[0].Type
	}
	if len(guests) > 1 {
		// Multiple matches - VMID collision across instances
		// Log warning and return first match (caller should use lookupGuestsByVMID with instance filter)
		log.Warn().
			Int("vmid", vmid).
			Int("matches", len(guests)).
			Msg("VMID collision detected - multiple guests with same VMID across instances")
		return guests[0].Node, guests[0].Name, guests[0].Type
	}
	return "", "", ""
}

// lookupGuestsByVMID finds all guests with the given VMID
// If targetInstance is non-empty, only returns guests from that instance
func (s *Service) lookupGuestsByVMID(vmid int, targetInstance string) []GuestInfo {
	s.mu.RLock()
	sp := s.stateProvider
	s.mu.RUnlock()

	if sp == nil {
		return nil
	}

	state := sp.GetState()
	var results []GuestInfo

	// Check containers
	for _, ct := range state.Containers {
		if ct.VMID == vmid {
			if targetInstance == "" || ct.Instance == targetInstance {
				results = append(results, GuestInfo{
					Node:     ct.Node,
					Name:     ct.Name,
					Type:     "lxc",
					Instance: ct.Instance,
				})
			}
		}
	}

	// Check VMs
	for _, vm := range state.VMs {
		if vm.VMID == vmid {
			if targetInstance == "" || vm.Instance == targetInstance {
				results = append(results, GuestInfo{
					Node:     vm.Node,
					Name:     vm.Name,
					Type:     "qemu",
					Instance: vm.Instance,
				})
			}
		}
	}

	return results
}

// extractVMIDFromCommand parses pct/qm/vzdump commands to extract the VMID being targeted
// Returns the VMID, whether it requires node-specific routing, and whether found
// Some commands (like vzdump) can run from any cluster node, others (like pct exec) must run on the owning node
func extractVMIDFromCommand(command string) (vmid int, requiresOwnerNode bool, found bool) {
	// Commands that MUST run on the node that owns the guest
	// These interact directly with the container/VM runtime
	nodeSpecificPatterns := []string{
		// pct commands - match any pct subcommand followed by VMID
		// Covers: exec, enter, start, stop, shutdown, reboot, status, push, pull, mount, unmount, etc.
		`(?:^|\s)pct\s+\w+\s+(\d+)`,
		// qm commands - match any qm subcommand followed by VMID
		// Covers: start, stop, shutdown, reset, status, guest exec, monitor, etc.
		`(?:^|\s)qm\s+(?:guest\s+)?\w+\s+(\d+)`,
	}

	// Commands that can run from any cluster node (cluster-aware)
	// vzdump uses the cluster to route to the right node automatically
	clusterAwarePatterns := []string{
		`(?:^|\s)vzdump\s+(\d+)`,
		// pvesh commands can specify node in path, so we don't force routing
	}

	// Check node-specific commands first (higher priority)
	for _, pattern := range nodeSpecificPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(command); len(matches) > 1 {
			if v, err := strconv.Atoi(matches[1]); err == nil {
				return v, true, true
			}
		}
	}

	// Check cluster-aware commands (don't force node routing)
	for _, pattern := range clusterAwarePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(command); len(matches) > 1 {
			if v, err := strconv.Atoi(matches[1]); err == nil {
				return v, false, true
			}
		}
	}

	return 0, false, false
}

// LoadConfig loads the AI configuration and initializes the provider
func (s *Service) LoadConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := s.persistence.LoadAIConfig()
	if err != nil {
		return fmt.Errorf("failed to load AI config: %w", err)
	}

	s.cfg = cfg

	// Don't initialize provider if AI is not enabled or not configured
	if cfg == nil || !cfg.Enabled || !cfg.IsConfigured() {
		s.provider = nil
		return nil
	}

	provider, err := providers.NewFromConfig(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize AI provider")
		s.provider = nil
		return nil // Don't fail startup if provider can't be initialized
	}

	s.provider = provider
	log.Info().
		Str("provider", cfg.Provider).
		Str("model", cfg.GetModel()).
		Bool("autonomous_mode", cfg.AutonomousMode).
		Msg("AI service initialized")

	return nil
}

// IsEnabled returns true if AI is enabled and configured
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg != nil && s.cfg.Enabled && s.provider != nil
}

// GetConfig returns a copy of the current AI config
func (s *Service) GetConfig() *config.AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg == nil {
		return nil
	}
	cfg := *s.cfg
	return &cfg
}

// GetDebugContext returns debug information about what context would be sent to the AI
func (s *Service) GetDebugContext(req ExecuteRequest) map[string]interface{} {
	s.mu.RLock()
	stateProvider := s.stateProvider
	agentServer := s.agentServer
	cfg := s.cfg
	s.mu.RUnlock()

	result := make(map[string]interface{})

	// State provider info
	result["has_state_provider"] = stateProvider != nil
	if stateProvider != nil {
		state := stateProvider.GetState()
		result["state_summary"] = map[string]interface{}{
			"nodes":         len(state.Nodes),
			"vms":           len(state.VMs),
			"containers":    len(state.Containers),
			"docker_hosts":  len(state.DockerHosts),
			"hosts":         len(state.Hosts),
			"pbs_instances": len(state.PBSInstances),
		}
		
		// List some VMs/containers for verification
		var vmNames []string
		for _, vm := range state.VMs {
			vmNames = append(vmNames, fmt.Sprintf("%s (VMID:%d, node:%s)", vm.Name, vm.VMID, vm.Node))
		}
		if len(vmNames) > 10 {
			vmNames = vmNames[:10]
		}
		result["sample_vms"] = vmNames

		var ctNames []string
		for _, ct := range state.Containers {
			ctNames = append(ctNames, fmt.Sprintf("%s (VMID:%d, node:%s)", ct.Name, ct.VMID, ct.Node))
		}
		if len(ctNames) > 10 {
			ctNames = ctNames[:10]
		}
		result["sample_containers"] = ctNames

		var hostNames []string
		for _, h := range state.Hosts {
			hostNames = append(hostNames, h.Hostname)
		}
		result["host_names"] = hostNames

		var dockerHostNames []string
		for _, dh := range state.DockerHosts {
			dockerHostNames = append(dockerHostNames, fmt.Sprintf("%s (%d containers)", dh.DisplayName, len(dh.Containers)))
		}
		result["docker_host_names"] = dockerHostNames
	}

	// Agent info
	result["has_agent_server"] = agentServer != nil
	if agentServer != nil {
		agents := agentServer.GetConnectedAgents()
		var agentNames []string
		for _, a := range agents {
			agentNames = append(agentNames, a.Hostname)
		}
		result["connected_agents"] = agentNames
	}

	// Config info
	result["has_config"] = cfg != nil
	if cfg != nil {
		result["custom_context_length"] = len(cfg.CustomContext)
		if len(cfg.CustomContext) > 200 {
			result["custom_context_preview"] = cfg.CustomContext[:200] + "..."
		} else {
			result["custom_context_preview"] = cfg.CustomContext
		}
	}

	// Build and include the system prompt
	systemPrompt := s.buildSystemPrompt(req)
	result["system_prompt_length"] = len(systemPrompt)
	result["system_prompt"] = systemPrompt

	return result
}

// IsAutonomous returns true if autonomous mode is enabled
func (s *Service) IsAutonomous() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg != nil && s.cfg.AutonomousMode
}

// isDangerousCommand checks if a command is too dangerous to auto-execute
// These commands ALWAYS require approval, even in autonomous mode
func isDangerousCommand(cmd string) bool {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	baseCmd := parts[0]
	if baseCmd == "sudo" && len(parts) > 1 {
		baseCmd = parts[1]
	}

	// Commands that are too dangerous to ever auto-execute
	dangerousCommands := map[string]bool{
		// Deletion commands
		"rm":     true,
		"rmdir":  true,
		"unlink": true,
		"shred":  true,
		// Disk/filesystem destructive operations
		"dd":          true,
		"mkfs":        true,
		"fdisk":       true,
		"parted":      true,
		"wipefs":      true,
		"sgdisk":      true,
		"gdisk":       true,
		"zpool":       true, // Allow reads but not modifications
		"zfs":         true, // Allow reads but not modifications
		"lvremove":    true,
		"vgremove":    true,
		"pvremove":    true,
		// System state changes
		"reboot":      true,
		"shutdown":    true,
		"poweroff":    true,
		"halt":        true,
		"init":        true,
		"systemctl":   true, // could stop critical services
		"service":     true,
		// User/permission changes
		"chmod":       true,
		"chown":       true,
		"useradd":     true,
		"userdel":     true,
		"passwd":      true,
		// Package management
		"apt":         true,
		"apt-get":     true,
		"dpkg":        true,
		"yum":         true,
		"dnf":         true,
		"pacman":      true,
		"pip":         true,
		"npm":         true,
		// Proxmox destructive
		"vzdump":      true,
		"vzrestore":   true,
		"pveam":       true,
		// Network changes
		"iptables":    true,
		"nft":         true,
		"firewall-cmd": true,
	}

	if dangerousCommands[baseCmd] {
		// Special case: allow read-only apt/apt-get operations
		if baseCmd == "apt" || baseCmd == "apt-get" {
			// First, check if it's a dry-run/simulate command (safe even for upgrade/install)
			for _, part := range parts {
				if part == "--dry-run" || part == "-s" || part == "--simulate" || part == "--just-print" {
					return false // Dry-run is always safe
				}
			}
			// Check for inherently read-only operations
			safeAptOps := []string{"update", "list", "show", "search", "policy", "madison", "depends", "rdepends", "changelog"}
			for _, safeOp := range safeAptOps {
				if len(parts) > 1 && parts[1] == safeOp {
					return false // Safe read-only operation
				}
				// Also handle sudo apt <op>
				if len(parts) > 2 && parts[0] == "sudo" && parts[2] == safeOp {
					return false
				}
			}
		}
		// Special case: allow read-only systemctl operations
		if baseCmd == "systemctl" {
			safeSystemctlOps := []string{"status", "show", "list-units", "list-unit-files", "is-active", "is-enabled", "is-failed", "cat"}
			for _, safeOp := range safeSystemctlOps {
				if len(parts) > 1 && parts[1] == safeOp {
					return false
				}
				if len(parts) > 2 && parts[0] == "sudo" && parts[2] == safeOp {
					return false
				}
			}
		}
		// Special case: allow read-only dpkg operations  
		if baseCmd == "dpkg" {
			safeDpkgOps := []string{"-l", "--list", "-L", "--listfiles", "-s", "--status", "-S", "--search", "-p", "--print-avail", "--get-selections"}
			for _, safeOp := range safeDpkgOps {
				if len(parts) > 1 && parts[1] == safeOp {
					return false
				}
				if len(parts) > 2 && parts[0] == "sudo" && parts[2] == safeOp {
					return false
				}
			}
		}
		return true
	}

	// Detect dangerous patterns in the full command
	dangerousPatterns := []string{
		"rm -rf", "rm -fr", "rm -r",
		"> /dev/", "| tee /",
		"mkfs.", "dd if=", "dd of=",
		":(){ :|:& };:", // fork bomb
		"chmod -R 777", "chmod 777",
		"drop database", "drop table",
		"truncate ",
	}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}

	return false
}

// isReadOnlyCommand checks if a command is read-only (doesn't modify state)
// Read-only commands can be executed without approval even in non-autonomous mode
func isReadOnlyCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	// Get the base command (first word, ignoring sudo)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	baseCmd := parts[0]
	if baseCmd == "sudo" && len(parts) > 1 {
		baseCmd = parts[1]
	}

	// List of read-only commands that are safe to auto-execute
	readOnlyCommands := map[string]bool{
		// File/disk inspection
		"ls": true, "ll": true, "dir": true,
		"cat": true, "head": true, "tail": true, "less": true, "more": true,
		"df": true, "du": true, "stat": true, "file": true,
		"find": true, "locate": true, "which": true, "whereis": true,
		"wc": true, "diff": true, "cmp": true,
		// Process inspection
		"ps": true, "top": true, "htop": true, "pgrep": true,
		"lsof": true, "fuser": true,
		// System info
		"uname": true, "hostname": true, "uptime": true, "whoami": true, "id": true,
		"free": true, "vmstat": true, "iostat": true, "sar": true,
		"lscpu": true, "lsmem": true, "lsblk": true, "lspci": true, "lsusb": true,
		"dmesg": true, "journalctl": true,
		// Network inspection
		"ip": true, "ifconfig": true, "netstat": true, "ss": true,
		"ping": true, "traceroute": true, "tracepath": true, "mtr": true,
		"dig": true, "nslookup": true, "host": true,
		"curl": true, "wget": true, // typically read-only when not writing files
		// Docker inspection
		"docker": true, // we'll check subcommands below
		// Package info (not install/remove)
		"dpkg": true, "rpm": true, "apt-cache": true,
		// Text processing (read-only)
		"grep": true, "egrep": true, "fgrep": true, "rg": true,
		"awk": true, "sed": true, // can be used read-only
		"sort": true, "uniq": true, "cut": true, "tr": true,
		"jq": true, "yq": true,
		// Proxmox inspection
		"pct": true, "qm": true, "pvesh": true, "pvecm": true,
		"zpool": true, "zfs": true,
	}

	if !readOnlyCommands[baseCmd] {
		return false
	}

	// Special handling for commands with dangerous subcommands
	cmdLower := strings.ToLower(cmd)

	// Docker: only allow inspection subcommands
	if baseCmd == "docker" {
		dangerousDockerCmds := []string{
			"docker rm", "docker rmi", "docker kill", "docker stop", "docker start",
			"docker restart", "docker prune", "docker pull", "docker push",
			"docker exec", "docker run", "docker build", "docker compose",
			"docker volume rm", "docker network rm", "docker system prune",
			"docker image prune", "docker container prune", "docker builder prune",
		}
		for _, dangerous := range dangerousDockerCmds {
			if strings.Contains(cmdLower, dangerous) {
				return false
			}
		}
		return true
	}

	// Proxmox: only allow list/status subcommands
	if baseCmd == "pct" || baseCmd == "qm" {
		safeSubcmds := []string{"list", "status", "config", "pending", "snapshot"}
		for _, safe := range safeSubcmds {
			if strings.Contains(cmdLower, " "+safe) {
				return true
			}
		}
		// If no safe subcommand found, assume dangerous
		return len(parts) == 1 // bare "pct" or "qm" is safe (shows help)
	}

	// journalctl with --vacuum is not read-only
	if baseCmd == "journalctl" && strings.Contains(cmdLower, "vacuum") {
		return false
	}

	return true
}

// ConversationMessage represents a message in conversation history
type ConversationMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// ExecuteRequest represents a request to execute an AI prompt
type ExecuteRequest struct {
	Prompt       string                 `json:"prompt"`
	TargetType   string                 `json:"target_type,omitempty"`   // "host", "container", "vm", "node"
	TargetID     string                 `json:"target_id,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`       // Current metrics, state, etc.
	SystemPrompt string                 `json:"system_prompt,omitempty"` // Override system prompt
	History      []ConversationMessage  `json:"history,omitempty"`       // Previous conversation messages
}

// ExecuteResponse represents the AI's response
type ExecuteResponse struct {
	Content      string       `json:"content"`
	Model        string       `json:"model"`
	InputTokens  int          `json:"input_tokens"`
	OutputTokens int          `json:"output_tokens"`
	ToolCalls    []ToolExecution `json:"tool_calls,omitempty"` // Commands that were executed
}

// ToolExecution represents a tool that was executed during the AI conversation
type ToolExecution struct {
	Name    string `json:"name"`
	Input   string `json:"input"`   // Human-readable input (e.g., the command)
	Output  string `json:"output"`  // Result of execution
	Success bool   `json:"success"`
}

// StreamEvent represents an event during AI execution for streaming
type StreamEvent struct {
	Type string      `json:"type"` // "tool_start", "tool_end", "content", "done", "error", "approval_needed"
	Data interface{} `json:"data,omitempty"`
}

// StreamCallback is called for each event during streaming execution
type StreamCallback func(event StreamEvent)

// ToolStartData is sent when a tool execution begins
type ToolStartData struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

// ToolEndData is sent when a tool execution completes
type ToolEndData struct {
	Name    string `json:"name"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// ApprovalNeededData is sent when a command needs user approval
type ApprovalNeededData struct {
	Command    string `json:"command"`
	ToolID     string `json:"tool_id"`     // ID to reference when approving
	ToolName   string `json:"tool_name"`   // "run_command", "read_file", etc.
	RunOnHost  bool   `json:"run_on_host"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host to route to
}

// Execute sends a prompt to the AI and returns the response
// If tools are available and the AI requests them, it executes them in a loop
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.cfg
	agentServer := s.agentServer
	s.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("AI is not enabled or configured")
	}

	// Build the system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt(req)
	}

	// Check if agent is available for this target
	hasAgent := s.hasAgentForTarget(req)

	// Build tools list if agent is available
	var tools []providers.Tool
	if hasAgent && agentServer != nil {
		tools = s.getTools()
		systemPrompt += `

## Available Tools
You have access to tools to execute commands on the target system. You should:
1. Use run_command to investigate issues, gather information, and PERFORM actions
2. Actually execute the commands - don't just explain what commands to run
3. For Proxmox operations (resize disk, manage containers/VMs), run commands on the HOST (target_type=host)
4. For operations inside a container, run commands on the container (target_type=container)

Examples of actions you can perform:
- Resize LXC disk: pct resize <vmid> rootfs +10G (run on host)
- Check disk usage: df -h (run on container)
- View processes: ps aux --sort=-%mem | head -20
- Check logs: tail -100 /var/log/syslog

Always execute the commands rather than telling the user how to do it.`
	}

	// Inject previously learned knowledge about this guest
	if s.knowledgeStore != nil {
		guestID := s.getGuestID(req)
		if guestID != "" {
			if knowledgeContext := s.knowledgeStore.FormatForContext(guestID); knowledgeContext != "" {
				systemPrompt += knowledgeContext
			}
		}
	}

	// Build initial messages with conversation history
	var messages []providers.Message
	for _, histMsg := range req.History {
		messages = append(messages, providers.Message{
			Role:    histMsg.Role,
			Content: histMsg.Content,
		})
	}
	messages = append(messages, providers.Message{Role: "user", Content: req.Prompt})

	var toolExecutions []ToolExecution
	totalInputTokens := 0
	totalOutputTokens := 0
	var finalContent string
	var model string

	// Agentic loop - keep going while AI requests tools
	maxIterations := 10 // Safety limit
	for i := 0; i < maxIterations; i++ {
		resp, err := provider.Chat(ctx, providers.ChatRequest{
			Messages:  messages,
			Model:     cfg.GetModel(),
			System:    systemPrompt,
			MaxTokens: 4096,
			Tools:     tools,
		})
		if err != nil {
			return nil, fmt.Errorf("AI request failed: %w", err)
		}

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens
		model = resp.Model
		finalContent = resp.Content

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			break
		}

		// Add assistant's response with tool calls to messages
		messages = append(messages, providers.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent, // DeepSeek thinking mode
			ToolCalls:        resp.ToolCalls,
		})

		// Execute each tool call and add results
		for _, tc := range resp.ToolCalls {
			result, execution := s.executeTool(ctx, req, tc)
			toolExecutions = append(toolExecutions, execution)

			// Add tool result to messages
			messages = append(messages, providers.Message{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: tc.ID,
					Content:   result,
					IsError:   !execution.Success,
				},
			})
		}
	}

	return &ExecuteResponse{
		Content:      finalContent,
		Model:        model,
		InputTokens:  totalInputTokens,
		OutputTokens: totalOutputTokens,
		ToolCalls:    toolExecutions,
	}, nil
}

// ExecuteStream sends a prompt to the AI and streams events via callback
// This allows the UI to show real-time progress during tool execution
func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) (*ExecuteResponse, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.cfg
	agentServer := s.agentServer
	s.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("AI is not enabled or configured")
	}

	// Build the system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt(req)
	}

	// Debug log the system prompt length and key sections
	log.Debug().
		Int("prompt_length", len(systemPrompt)).
		Bool("has_infrastructure_map", strings.Contains(systemPrompt, "## Infrastructure Map")).
		Bool("has_docker_hosts", strings.Contains(systemPrompt, "### Docker Hosts")).
		Bool("has_standalone_hosts", strings.Contains(systemPrompt, "### Standalone Hosts")).
		Bool("has_guests", strings.Contains(systemPrompt, "### All Guests")).
		Msg("AI system prompt built")

	// Check if agent is available for this target
	hasAgent := s.hasAgentForTarget(req)

	// Build tools list if agent is available
	var tools []providers.Tool
	if hasAgent && agentServer != nil {
		tools = s.getTools()
		systemPrompt += `

## Available Tools
You have access to tools to execute commands on the target system. You should:
1. Use run_command to investigate issues, gather information, and PERFORM actions
2. Actually execute the commands - don't just explain what commands to run
3. For Proxmox operations (resize disk, manage containers/VMs), run commands on the HOST (target_type=host)
4. For operations inside a container, run commands on the container (target_type=container)

Examples of actions you can perform:
- Resize LXC disk: pct resize <vmid> rootfs +10G (run on host)
- Check disk usage: df -h (run on container)
- View processes: ps aux --sort=-%mem | head -20
- Check logs: tail -100 /var/log/syslog

Always execute the commands rather than telling the user how to do it.`
	}

	// Inject previously learned knowledge about this guest
	if s.knowledgeStore != nil {
		guestID := s.getGuestID(req)
		if guestID != "" {
			if knowledgeContext := s.knowledgeStore.FormatForContext(guestID); knowledgeContext != "" {
				log.Debug().
					Str("guest_id", guestID).
					Int("context_length", len(knowledgeContext)).
					Msg("Injecting saved knowledge into AI context")
				systemPrompt += knowledgeContext
			} else {
				log.Debug().Str("guest_id", guestID).Msg("No saved knowledge for guest")
			}
		}
	}

	// Build initial messages with conversation history
	var messages []providers.Message
	for _, histMsg := range req.History {
		messages = append(messages, providers.Message{
			Role:    histMsg.Role,
			Content: histMsg.Content,
		})
	}
	messages = append(messages, providers.Message{Role: "user", Content: req.Prompt})

	var toolExecutions []ToolExecution
	totalInputTokens := 0
	totalOutputTokens := 0
	var finalContent string
	var model string

	// Agentic loop - keep going while AI requests tools
	// No artificial iteration limit - the context timeout (5 minutes) provides the safety net
	iteration := 0
	for {
		iteration++
		log.Debug().
			Int("iteration", iteration).
			Int("message_count", len(messages)).
			Int("system_prompt_length", len(systemPrompt)).
			Int("tools_count", len(tools)).
			Msg("Calling AI provider...")

		// Send a processing event so the frontend knows we're making an AI call
		// This is especially important after tool execution when the next AI call can take a while
		if iteration > 1 {
			callback(StreamEvent{Type: "processing", Data: fmt.Sprintf("Analyzing results (iteration %d)...", iteration)})
		}

		resp, err := provider.Chat(ctx, providers.ChatRequest{
			Messages:  messages,
			Model:     cfg.GetModel(),
			System:    systemPrompt,
			MaxTokens: 4096,
			Tools:     tools,
		})
		if err != nil {
			log.Error().Err(err).Int("iteration", iteration).Msg("AI provider call failed")
			callback(StreamEvent{Type: "error", Data: err.Error()})
			return nil, fmt.Errorf("AI request failed: %w", err)
		}

		log.Debug().Int("iteration", iteration).Msg("AI provider returned successfully")

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens
		model = resp.Model
		finalContent = resp.Content

		// Stream thinking/reasoning content if present (DeepSeek reasoner)
		if resp.ReasoningContent != "" {
			callback(StreamEvent{Type: "thinking", Data: resp.ReasoningContent})
		}

		// Stream intermediate content so users see the AI's explanations between tool calls
		// This gives users visibility into the AI's reasoning as it works, not just at the end
		if resp.Content != "" {
			callback(StreamEvent{Type: "content", Data: resp.Content})
		}

		log.Debug().
			Int("tool_calls", len(resp.ToolCalls)).
			Str("stop_reason", resp.StopReason).
			Int("iteration", iteration).
			Int("total_input_tokens", totalInputTokens).
			Int("total_output_tokens", totalOutputTokens).
			Int("content_length", len(resp.Content)).
			Bool("has_content", resp.Content != "").
			Msg("AI streaming iteration complete")

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			log.Info().
				Int("tool_calls", len(resp.ToolCalls)).
				Str("stop_reason", resp.StopReason).
				Int("iteration", iteration).
				Msg("AI streaming loop ending - no more tool calls or stop_reason != tool_use")
			break
		}

		// Add assistant's response with tool calls to messages
		messages = append(messages, providers.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent, // DeepSeek thinking mode
			ToolCalls:        resp.ToolCalls,
		})

		// Execute each tool call and add results
		// Track if any command needs approval - if so, we'll stop the loop after processing
		anyNeedsApproval := false
		for _, tc := range resp.ToolCalls {
			toolInput := s.getToolInputDisplay(tc)

			// Check if this command needs approval
			needsApproval := false
			if tc.Name == "run_command" {
				cmd, _ := tc.Input["command"].(string)
				runOnHost, _ := tc.Input["run_on_host"].(bool)
				targetHost, _ := tc.Input["target_host"].(string)

				// If AI didn't specify target_host, try to get it from request context
				// This is crucial for proper routing when the command is approved
				if targetHost == "" {
					if node, ok := req.Context["node"].(string); ok && node != "" {
						targetHost = node
					} else if node, ok := req.Context["hostname"].(string); ok && node != "" {
						targetHost = node
					} else if node, ok := req.Context["host_name"].(string); ok && node != "" {
						targetHost = node
					}
				}

				isAuto := s.IsAutonomous()
				isReadOnly := isReadOnlyCommand(cmd)
				isDangerous := isDangerousCommand(cmd)
				log.Debug().
					Bool("autonomous", isAuto).
					Bool("read_only", isReadOnly).
					Bool("dangerous", isDangerous).
					Str("command", cmd).
					Str("target_host", targetHost).
					Msg("Checking command approval")

				// In autonomous mode, NO commands need approval - full trust
				// In non-autonomous mode:
				//   - Dangerous commands always need approval
				//   - Non-read-only commands need approval
				if !isAuto && (isDangerous || !isReadOnly) {
					needsApproval = true
					anyNeedsApproval = true
					// Send approval needed event
					callback(StreamEvent{
						Type: "approval_needed",
						Data: ApprovalNeededData{
							Command:    cmd,
							ToolID:     tc.ID,
							ToolName:   tc.Name,
							RunOnHost:  runOnHost,
							TargetHost: targetHost,
						},
					})
				}
			}


			var result string
			var execution ToolExecution

			if needsApproval {
				// Don't execute - command needs user approval
				// We'll break out of the loop after processing all tool calls
				// Note: We don't add to toolExecutions here because the approval_needed event
				// already tells the frontend to show the approval UI
				result = fmt.Sprintf("Awaiting user approval: %s", toolInput)
			} else {
				// Stream tool start event
				callback(StreamEvent{
					Type: "tool_start",
					Data: ToolStartData{Name: tc.Name, Input: toolInput},
				})

				result, execution = s.executeTool(ctx, req, tc)
				toolExecutions = append(toolExecutions, execution)

				// Stream tool end event
				callback(StreamEvent{
					Type: "tool_end",
					Data: ToolEndData{Name: tc.Name, Input: toolInput, Output: result, Success: execution.Success},
				})
			}

			// Truncate large results to prevent context bloat
			// Keep first and last parts for context
			resultForContext := result
			const maxResultSize = 8000 // ~8KB per tool result
			if len(result) > maxResultSize {
				halfSize := maxResultSize / 2
				resultForContext = result[:halfSize] + "\n\n[... output truncated (" +
					fmt.Sprintf("%d", len(result)-maxResultSize) + " bytes omitted) ...]\n\n" +
					result[len(result)-halfSize:]
				log.Debug().
					Int("original_size", len(result)).
					Int("truncated_size", len(resultForContext)).
					Msg("Truncated large tool result")
			}

			// Add tool result to messages
			messages = append(messages, providers.Message{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: tc.ID,
					Content:   resultForContext,
					IsError:   !execution.Success,
				},
			})
		}

		// If any command needed approval, stop the agentic loop here.
		// Don't call the AI again with "COMMAND_BLOCKED" results - this causes duplicate
		// approval requests and confusing "click the button" messages.
		// The frontend will show approval buttons, and user action will continue the conversation.
		if anyNeedsApproval {
			log.Info().
				Int("pending_approvals", len(resp.ToolCalls)).
				Int("iteration", iteration).
				Msg("Stopping AI loop - commands need user approval")
			// Use the AI's current response as final content (if any)
			// This preserves any explanation the AI provided before requesting the command
			break
		}
	}

	// Stream the final content
	callback(StreamEvent{Type: "content", Data: finalContent})
	callback(StreamEvent{Type: "done"})

	return &ExecuteResponse{
		Content:      finalContent,
		Model:        model,
		InputTokens:  totalInputTokens,
		OutputTokens: totalOutputTokens,
		ToolCalls:    toolExecutions,
	}, nil
}

// getToolInputDisplay returns a human-readable display of tool input
func (s *Service) getToolInputDisplay(tc providers.ToolCall) string {
	switch tc.Name {
	case "run_command":
		cmd, _ := tc.Input["command"].(string)
		if runOnHost, ok := tc.Input["run_on_host"].(bool); ok && runOnHost {
			return fmt.Sprintf("[host] %s", cmd)
		}
		return cmd
	case "read_file":
		path, _ := tc.Input["path"].(string)
		return path
	case "fetch_url":
		url, _ := tc.Input["url"].(string)
		return url
	case "set_resource_url":
		resourceType, _ := tc.Input["resource_type"].(string)
		url, _ := tc.Input["url"].(string)
		return fmt.Sprintf("Set %s URL: %s", resourceType, url)
	default:
		return fmt.Sprintf("%v", tc.Input)
	}
}

// hasAgentForTarget checks if we have an agent connection for the given target
func (s *Service) hasAgentForTarget(req ExecuteRequest) bool {
	if s.agentServer == nil {
		return false
	}

	// For now, just check if any agent is connected
	// TODO: Map target to specific agent based on hostname/node
	agents := s.agentServer.GetConnectedAgents()
	return len(agents) > 0
}

// getTools returns the available tools for AI
func (s *Service) getTools() []providers.Tool {
	tools := []providers.Tool{
		{
			Name:        "run_command",
			Description: "Execute a shell command. By default runs on the current target (container/VM), but set run_on_host=true for Proxmox host commands. IMPORTANT: For targets on different nodes, specify target_host to route to the correct PVE node.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (e.g., 'ps aux --sort=-%mem | head -20')",
					},
					"run_on_host": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, run on the Proxmox/Docker host instead of inside the container/VM. Use for pct/qm commands like 'pct resize 101 rootfs +10G'. When true, you should also set target_host.",
					},
					"target_host": map[string]interface{}{
						"type":        "string",
						"description": "Optional hostname of the specific host/node to run the command on. Use this to explicitly route pct/qm/docker commands to the correct Proxmox node or Docker host. Check the 'node' or 'PVE Node' field in the target's context.",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file on the target. Use this to examine configuration files or logs.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path to the file (e.g., '/etc/nginx/nginx.conf')",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file on the target. Use this to create or modify configuration files, scripts, or other text files. Creates parent directories if needed.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Absolute path to the file (e.g., '/etc/myapp/config.yaml')",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "Optional file permissions in octal (e.g., '0644' for rw-r--r--, '0755' for executable). Defaults to '0644'.",
					},
					"append": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, append to the file instead of overwriting. Defaults to false.",
					},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "fetch_url",
			Description: "Fetch content from a URL. Use this to check if web services are responding, read API endpoints, or fetch documentation. Works with local network URLs and public sites.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to fetch (e.g., 'http://192.168.1.50:8080/api/health' or 'https://example.com/docs')",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "set_resource_url",
			Description: "Set the web URL for a resource in Pulse after discovering a web service. Use this when you've found a web server running on a guest/container/host and want to save it for quick access. The URL will appear as a clickable link in the Pulse dashboard.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of resource: 'guest' for VMs/LXC containers, 'docker' for Docker containers/services, or 'host' for standalone hosts",
						"enum":        []string{"guest", "docker", "host"},
					},
					"resource_id": map[string]interface{}{
						"type":        "string",
						"description": "The resource ID from the context (e.g., 'pve1-delly-101' for guests, 'dockerhost:container:abc123' for Docker). Use the ID from the current context.",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The discovered URL (e.g., 'http://192.168.1.50:8096' for Jellyfin). Use the IP/hostname and port you discovered.",
					},
				},
				"required": []string{"resource_type", "resource_id", "url"},
			},
		},
	}

	// Add web search tool for Anthropic provider
	if s.provider != nil && s.provider.Name() == "anthropic" {
		tools = append(tools, providers.Tool{
			Type:    "web_search_20250305",
			Name:    "web_search",
			MaxUses: 3, // Limit searches per request to control costs
		})
	}

	return tools
}

// executeTool executes a tool call and returns the result
func (s *Service) executeTool(ctx context.Context, req ExecuteRequest, tc providers.ToolCall) (string, ToolExecution) {
	execution := ToolExecution{
		Name:    tc.Name,
		Success: false,
	}

	switch tc.Name {
	case "run_command":
		command, _ := tc.Input["command"].(string)
		runOnHost, _ := tc.Input["run_on_host"].(bool)
		targetHost, _ := tc.Input["target_host"].(string)
		execution.Input = command
		if runOnHost && targetHost != "" {
			execution.Input = fmt.Sprintf("[%s] %s", targetHost, command)
		} else if runOnHost {
			execution.Input = fmt.Sprintf("[host] %s", command)
		}

		if command == "" {
			execution.Output = "Error: command is required"
			return execution.Output, execution
		}

		// Check security policy (skip if autonomous mode is enabled)
		if !s.IsAutonomous() {
			decision := s.policy.Evaluate(command)
			if decision == agentexec.PolicyBlock {
				execution.Output = "Error: This command is blocked by security policy"
				return execution.Output, execution
			}
			if decision == agentexec.PolicyRequireApproval {
				// Direct the AI to tell the user about the approval button
				execution.Output = fmt.Sprintf("COMMAND_BLOCKED: This command (%s) requires user approval and was NOT executed. "+
					"An approval button has been displayed to the user. "+
					"DO NOT attempt to run this command again. "+
					"Tell the user to click the 'Run' button to execute it.", command)
				execution.Success = true // Not an error, just needs approval
				return execution.Output, execution
			}
		}

		// Build execution request with proper targeting
		execReq := req
		
		// If target_host is explicitly specified by AI, use it for routing
		if targetHost != "" {
			// Ensure Context map exists
			if execReq.Context == nil {
				execReq.Context = make(map[string]interface{})
			} else {
				// Make a copy to avoid modifying the original
				newContext := make(map[string]interface{})
				for k, v := range req.Context {
					newContext[k] = v
				}
				execReq.Context = newContext
			}
			// Set the node explicitly - this takes priority in routing
			execReq.Context["node"] = targetHost
			log.Debug().
				Str("target_host", targetHost).
				Str("command", command).
				Msg("AI explicitly specified target_host for command routing")
		}
		
		// If run_on_host is true, override the target type to run on host
		if runOnHost {
			log.Debug().
				Str("command", command).
				Str("target_host", targetHost).
				Str("original_target_type", req.TargetType).
				Str("original_target_id", req.TargetID).
				Msg("run_on_host=true - overriding target type to 'host'")
			execReq.TargetType = "host"
			execReq.TargetID = ""
		} else {
			log.Debug().
				Str("command", command).
				Str("target_type", req.TargetType).
				Str("target_id", req.TargetID).
				Bool("run_on_host", runOnHost).
				Msg("Executing command with current target type")
		}

		// Execute via agent
		result, err := s.executeOnAgent(ctx, execReq, command)
		if err != nil {
			execution.Output = fmt.Sprintf("Error executing command: %s", err)
			return execution.Output, execution
		}

		execution.Output = result
		execution.Success = true
		return result, execution

	case "read_file":
		path, _ := tc.Input["path"].(string)
		execution.Input = path

		if path == "" {
			execution.Output = "Error: path is required"
			return execution.Output, execution
		}

		// Use cat command to read file (simple approach)
		command := fmt.Sprintf("cat %q 2>&1 | head -c 65536", path)
		result, err := s.executeOnAgent(ctx, req, command)
		if err != nil {
			execution.Output = fmt.Sprintf("Error reading file: %s", err)
			return execution.Output, execution
		}

		execution.Output = result
		execution.Success = true
		return result, execution

	case "write_file":
		path, _ := tc.Input["path"].(string)
		content, _ := tc.Input["content"].(string)
		mode, _ := tc.Input["mode"].(string)
		appendMode, _ := tc.Input["append"].(bool)
		execution.Input = path

		if path == "" {
			execution.Output = "Error: path is required"
			return execution.Output, execution
		}
		if content == "" {
			execution.Output = "Error: content is required"
			return execution.Output, execution
		}

		// Size limit: 1MB max to prevent filling disk
		const maxFileSize = 1024 * 1024 // 1MB
		if len(content) > maxFileSize {
			execution.Output = fmt.Sprintf("Error: content too large (%d bytes). Maximum allowed is %d bytes (1MB)", len(content), maxFileSize)
			return execution.Output, execution
		}

		// Path blocklist: prevent writes to critical system files
		blockedPaths := []string{
			"/etc/passwd", "/etc/shadow", "/etc/group", "/etc/gshadow",
			"/etc/sudoers", "/etc/ssh/sshd_config",
			"/boot/", "/lib/", "/lib64/", "/usr/lib/",
			"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
			"/proc/", "/sys/", "/dev/",
		}
		cleanPath := filepath.Clean(path)
		for _, blocked := range blockedPaths {
			if cleanPath == blocked || strings.HasPrefix(cleanPath, blocked) {
				execution.Output = fmt.Sprintf("Error: writing to %s is blocked for safety. This is a critical system path.", path)
				return execution.Output, execution
			}
		}

		// Default mode if not specified
		if mode == "" {
			mode = "0644"
		}

		// Build the write command using base64 to safely handle any content
		// This avoids issues with special characters, quotes, newlines, etc.
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		
		var command string
		if appendMode {
			// Append mode: decode and append to file (no backup needed for append)
			command = fmt.Sprintf("echo %q | base64 -d >> %q && echo 'Content appended to %s (%d bytes)'", encoded, path, path, len(content))
		} else {
			// Overwrite mode with safety features:
			// 1. Create parent directory if needed
			// 2. Backup existing file if it exists (atomic - only if backup succeeds)
			// 3. Write to temp file first
			// 4. Atomic move temp file to target
			// 5. Set permissions
			dir := filepath.Dir(path)
			tempFile := path + ".pulse-tmp"
			backupFile := path + ".bak"
			
			// Build a safe multi-step command:
			// - mkdir -p for parent dir
			// - if file exists, copy to .bak
			// - write content to temp file
			// - mv temp file to target (atomic)
			// - chmod to set permissions
			command = fmt.Sprintf(
				"mkdir -p %q && "+
					"([ -f %q ] && cp %q %q 2>/dev/null || true) && "+
					"echo %q | base64 -d > %q && "+
					"mv %q %q && "+
					"chmod %s %q && "+
					"echo 'Written %d bytes to %s (backup: %s.bak if existed)'",
				dir,
				path, path, backupFile,
				encoded, tempFile,
				tempFile, path,
				mode, path,
				len(content), path, path,
			)
		}

		result, err := s.executeOnAgent(ctx, req, command)
		if err != nil {
			execution.Output = fmt.Sprintf("Error writing file: %s", err)
			return execution.Output, execution
		}

		execution.Output = result
		execution.Success = true
		return result, execution

	case "fetch_url":
		urlStr, _ := tc.Input["url"].(string)
		execution.Input = urlStr

		if urlStr == "" {
			execution.Output = "Error: url is required"
			return execution.Output, execution
		}

		// Fetch the URL
		result, err := s.fetchURL(ctx, urlStr)
		if err != nil {
			execution.Output = fmt.Sprintf("Error fetching URL: %s", err)
			return execution.Output, execution
		}

		execution.Output = result
		execution.Success = true
		return result, execution

	case "set_resource_url":
		resourceType, _ := tc.Input["resource_type"].(string)
		resourceID, _ := tc.Input["resource_id"].(string)
		url, _ := tc.Input["url"].(string)
		execution.Input = fmt.Sprintf("%s %s -> %s", resourceType, resourceID, url)

		if resourceType == "" {
			execution.Output = "Error: resource_type is required (use 'guest', 'docker', or 'host')"
			return execution.Output, execution
		}
		if resourceID == "" {
			// Try to get the resource ID from the request context
			if req.TargetID != "" {
				resourceID = req.TargetID
			} else {
				execution.Output = "Error: resource_id is required"
				return execution.Output, execution
			}
		}
		if url == "" {
			execution.Output = "Error: url is required"
			return execution.Output, execution
		}

		// Update the metadata
		if err := s.SetResourceURL(resourceType, resourceID, url); err != nil {
			execution.Output = fmt.Sprintf("Error setting URL: %s", err)
			return execution.Output, execution
		}

		execution.Output = fmt.Sprintf("✅ Successfully set URL for %s '%s' to: %s\nThe URL is now visible in the Pulse dashboard as a clickable link.", resourceType, resourceID, url)
		execution.Success = true
		return execution.Output, execution

	default:
		execution.Output = fmt.Sprintf("Unknown tool: %s", tc.Name)
		return execution.Output, execution
	}
}

// getGuestID returns a unique identifier for the guest based on the request
func (s *Service) getGuestID(req ExecuteRequest) string {
	// Build a consistent guest ID from the target information
	if req.TargetType == "" || req.TargetID == "" {
		return ""
	}
	
	// For Proxmox targets, include the node info
	// Format: instance-node-type-vmid or instance-targetid
	return fmt.Sprintf("%s-%s", req.TargetType, req.TargetID)
}

// GetGuestKnowledge returns all knowledge for a guest
func (s *Service) GetGuestKnowledge(guestID string) (*knowledge.GuestKnowledge, error) {
	if s.knowledgeStore == nil {
		return nil, fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.GetKnowledge(guestID)
}

// SaveGuestNote saves a note for a guest
func (s *Service) SaveGuestNote(guestID, guestName, guestType, category, title, content string) error {
	if s.knowledgeStore == nil {
		return fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.SaveNote(guestID, guestName, guestType, category, title, content)
}

// DeleteGuestNote deletes a note from a guest
func (s *Service) DeleteGuestNote(guestID, noteID string) error {
	if s.knowledgeStore == nil {
		return fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.DeleteNote(guestID, noteID)
}

// fetchURL fetches content from a URL with size limits and timeout
func (s *Service) fetchURL(ctx context.Context, urlStr string) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "Pulse-AI/1.0")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response with size limit (64KB)
	const maxSize = 64 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Build result with status info
	result := fmt.Sprintf("HTTP %d %s\n", resp.StatusCode, resp.Status)
	result += fmt.Sprintf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	result += fmt.Sprintf("Content-Length: %d bytes\n\n", len(body))
	result += string(body)

	if len(body) == maxSize {
		result += "\n\n[Response truncated at 64KB]"
	}

	return result, nil
}

// sanitizeError cleans up error messages to remove internal networking details
// that are not helpful to users or AI models (IP addresses, port numbers, etc.)
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	
	errMsg := err.Error()
	
	// Replace raw TCP connection details with generic message
	// e.g., "write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout" 
	// becomes "connection to agent timed out"
	if strings.Contains(errMsg, "i/o timeout") {
		if strings.Contains(errMsg, "failed to send command") {
			return fmt.Errorf("connection to agent timed out - the agent may be disconnected or unreachable")
		}
		return fmt.Errorf("network timeout - the target may be unreachable")
	}
	
	// Replace "write tcp ... connection refused" style errors
	if strings.Contains(errMsg, "connection refused") {
		return fmt.Errorf("connection refused - the agent may not be running on the target host")
	}
	
	// Replace "no such host" errors
	if strings.Contains(errMsg, "no such host") {
		return fmt.Errorf("host not found - verify the hostname is correct and DNS is working")
	}
	
	// Replace "context deadline exceeded" with friendlier message
	if strings.Contains(errMsg, "context deadline exceeded") {
		return fmt.Errorf("operation timed out - the command may have taken too long")
	}
	
	return err
}

// executeOnAgent executes a command via the agent WebSocket
func (s *Service) executeOnAgent(ctx context.Context, req ExecuteRequest, command string) (string, error) {
	if s.agentServer == nil {
		return "", fmt.Errorf("agent server not available")
	}

	// Find the appropriate agent using robust routing
	agents := s.agentServer.GetConnectedAgents()
	
	// Use the new robust routing logic
	routeResult, err := s.routeToAgent(req, command, agents)
	if err != nil {
		// Check if this is a routing error that should ask for clarification
		if routingErr, ok := err.(*RoutingError); ok && routingErr.AskForClarification {
			// Return a message that encourages the AI to ask the user for clarification
			// instead of just failing with an error
			return routingErr.ForAI(), nil
		}
		// Return actionable error message for other errors
		return "", err
	}

	// Log any warnings from routing
	for _, warning := range routeResult.Warnings {
		log.Warn().Str("warning", warning).Msg("Routing warning")
	}

	agentID := routeResult.AgentID
	
	log.Debug().
		Str("agent_id", agentID).
		Str("agent_hostname", routeResult.AgentHostname).
		Str("target_node", routeResult.TargetNode).
		Str("routing_method", routeResult.RoutingMethod).
		Bool("cluster_peer", routeResult.ClusterPeer).
		Msg("Command routed to agent")

	// Extract numeric VMID from target ID (e.g., "delly-135" -> "135")
	targetID := req.TargetID
	if req.TargetType == "container" || req.TargetType == "vm" {
		// Look for vmid in context first
		if vmid, ok := req.Context["vmid"]; ok {
			switch v := vmid.(type) {
			case float64:
				targetID = fmt.Sprintf("%.0f", v)
			case int:
				targetID = fmt.Sprintf("%d", v)
			case string:
				targetID = v
			}
		} else if req.TargetID != "" {
			// Extract number from end of ID like "delly-135" or "instance-135"
			parts := strings.Split(req.TargetID, "-")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				// Check if it's numeric
				if _, err := fmt.Sscanf(lastPart, "%d", new(int)); err == nil {
					targetID = lastPart
				}
			}
		}
	}

	requestID := uuid.New().String()

	// Automatically force non-interactive mode for package managers
	// This prevents hanging when apt/dpkg asks for confirmation or configuration
	if strings.Contains(command, "apt") || strings.Contains(command, "dpkg") {
		if !strings.Contains(command, "DEBIAN_FRONTEND=") {
			command = "export DEBIAN_FRONTEND=noninteractive; " + command
		}
	}

	cmd := agentexec.ExecuteCommandPayload{
		RequestID:  requestID,
		Command:    command,
		TargetType: req.TargetType,
		TargetID:   targetID,
		Timeout:    300, // 5 minutes - commands like du, backups, etc. can take a while
	}

	result, err := s.agentServer.ExecuteCommand(ctx, agentID, cmd)
	if err != nil {
		return "", sanitizeError(err)
	}

	if !result.Success {
		if result.Error != "" {
			return "", fmt.Errorf("%s", result.Error)
		}
		if result.Stderr != "" {
			return result.Stderr, nil // Return stderr as output, not error
		}
	}

	output := result.Stdout
	if result.Stderr != "" && result.Stdout != "" {
		output = fmt.Sprintf("%s\n\nSTDERR:\n%s", result.Stdout, result.Stderr)
	} else if result.Stderr != "" {
		output = result.Stderr
	}

	return output, nil
}

// RunCommandRequest represents a request to run a single command
type RunCommandRequest struct {
	Command    string `json:"command"`
	TargetType string `json:"target_type"` // "host", "container", "vm"
	TargetID   string `json:"target_id"`
	RunOnHost  bool   `json:"run_on_host"`  // If true, run on host instead of target
	VMID       string `json:"vmid,omitempty"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host for routing
}

// RunCommandResponse represents the result of running a command
type RunCommandResponse struct {
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// RunCommand executes a single command via the agent (used for approved commands)
func (s *Service) RunCommand(ctx context.Context, req RunCommandRequest) (*RunCommandResponse, error) {
	if s.agentServer == nil {
		return &RunCommandResponse{Success: false, Error: "Agent server not available"}, nil
	}

	// Build an ExecuteRequest from the RunCommandRequest
	execReq := ExecuteRequest{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    make(map[string]interface{}),
	}

	// If running on host, override target type
	if req.RunOnHost {
		execReq.TargetType = "host"
		// Keep the original target info for routing
	}

	// Add VMID to context if provided
	if req.VMID != "" {
		execReq.Context["vmid"] = req.VMID
	}

	// If target_host is specified, set it in context for routing
	if req.TargetHost != "" {
		execReq.Context["node"] = req.TargetHost
		log.Debug().
			Str("target_host", req.TargetHost).
			Str("command", req.Command).
			Msg("RunCommand using explicit target_host for routing")
	}


	output, err := s.executeOnAgent(ctx, execReq, req.Command)
	if err != nil {
		return &RunCommandResponse{
			Success: false,
			Error:   err.Error(),
			Output:  output,
		}, nil
	}

	return &RunCommandResponse{
		Success: true,
		Output:  output,
	}, nil
}

// buildSystemPrompt creates the system prompt based on the request context
func (s *Service) buildSystemPrompt(req ExecuteRequest) string {
	prompt := `You are Pulse's diagnostic assistant - a built-in tool for investigating Proxmox and Docker homelab issues.

## Response Style
- Be DIRECT and CONCISE. No greetings, no "I'll help you", no "Let me check"
- Report findings, not process. Don't narrate what you're doing
- NO emojis, NO markdown tables, NO excessive formatting
- Use simple lists only when showing multiple items
- Final response should be 2-4 sentences summarizing findings and any actions taken
- When you perform cleanup/fixes, report: what was done + the result (e.g., "Freed 6GB. Disk now at 81%.")

## When Investigating
- Execute commands silently in the background
- Only show the conclusion, not every command run
- If something fails, briefly state what and why

## When Performing Actions
- Just do it. Don't ask "Would you like me to..."
- Report the result, not the intent
- If destructive (delete data, stop services), execute but clearly state what was done

## Response Format
BAD: "I'll check that for you. Let me run some commands..."
GOOD: State findings directly.

BAD: "Would you like me to..."
GOOD: Do it, then report the result.

BAD: Tables, headers, bullet-heavy summaries
GOOD: Plain prose, 2-4 sentences.

## ACTION BIAS - AVOID INVESTIGATION LOOPS
When the user asks you to DO something (install, fix, update, configure), ACT IMMEDIATELY:
- Don't extensively investigate before acting. Run 1-2 diagnostic commands max, then DO the thing.
- Don't explain what you're about to do - just do it.
- Don't ask for confirmation. The user asked you to do it, so do it.
- If the first approach fails, try the next most obvious approach. Don't stop to report.
- Complete the task END TO END. If asked to "install and run X", you're not done until X is running.

INVESTIGATION ANTI-PATTERNS TO AVOID:
- Running 10+ diagnostic commands before taking action
- Explaining each step before doing it
- Stopping to report partial progress
- Asking "would you like me to proceed?" after the user already asked you to do it
- Checking version, checking config, checking service, checking ports, checking this, checking that... JUST ACT.

GOOD PATTERN for "install X and make sure it's running":
1. Download/install X (1 command)
2. Start X (1 command)  
3. Verify X is running (1 command)
4. Report: "Installed X vN.N. Service is running on port NNNN."

BAD PATTERN:
1. Check current version... 2. Check if installed... 3. Check service status... 4. Check config file... 
5. Check another config... 6. Try to enable something... 7. Check if it worked... 8. Read a script...
9. Check yet another file... [user: "do it"] 10. Still investigating... [user: "DO IT"]

## Using Context Data
Pulse provides real metrics in "Current Metrics and State". Use this data directly - don't ask users to check things you already know.

## Command Execution
- run_on_host=true: Run on PVE/Docker host (pct, qm, vzdump, docker commands)
- run_on_host=false: Run inside the container/VM
- target_host: ALWAYS set this when using run_on_host=true! Use the node/hostname from target context
- Execute commands to investigate, don't just explain what commands to run

## CRITICAL: Command Routing with target_host
When running commands that require a specific host (pct, qm, docker, vzdump), you MUST specify target_host to route correctly.

Example for LXC 106 on node 'minipc':
- To run 'df -h' inside the container: run_command(command="df -h", run_on_host=false)
- To run 'pct exec 106 -- df -h' on the host: run_command(command="pct exec 106 -- df -h", run_on_host=true, target_host="minipc")

Always check the target's context for the 'node' or 'PVE Node' field and pass it as target_host.
If you don't specify target_host when run_on_host=true, the command may route to the wrong host!

Rules:
1. Look at the target context for 'node', 'guest_node', or 'PVE Node' field
2. When running pct/qm commands: set run_on_host=true AND target_host=<node>
3. When running commands inside the guest: just set run_on_host=false (no target_host needed)
4. Error "Configuration file does not exist" means wrong host - check target_host

## Infrastructure Architecture - LXC Management
Pulse manages LXC containers agentlessly from the PVE host.
- DO NOT check for a Pulse agent process or service inside an LXC. It does not exist.
- Use run_command with run_on_host=false to execute commands inside the LXC. Pulse handles the routing.
- For pct commands, always use run_on_host=true and set target_host to the container's node.

## URL Discovery Feature
When asked to find the web URL for a guest/container/host, or when you discover a web service:

1. **Inspect for web servers**: Check for listening ports (ss -tlnp), running services (nginx, apache, node, etc.)
2. **Get the IP address**: Use 'hostname -I' or 'ip addr' to find the IP
3. **Test the URL**: Use fetch_url to verify the service is responding
4. **Save the URL**: Use set_resource_url tool to save it to Pulse

Common discovery commands:
- Check listening ports: ss -tlnp | grep LISTEN
- Check nginx: systemctl status nginx && grep -r 'listen' /etc/nginx/
- Check running processes: ps aux | grep -E 'node|python|java|nginx|apache|httpd'
- Get IP: hostname -I | awk '{print $1}'

When you find a web service and are confident, use set_resource_url to save it. The resource_id should match the ID from the current context.

## Installing/Updating Pulse Itself
If asked to install or update Pulse itself, use the official install script. DO NOT investigate configs/services first.
Quick install/update command (x86_64 Linux):
` + "`" + `curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash` + "`" + `

To install a specific version: 
` + "`" + `curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version vX.Y.Z` + "`" + `

After install, enable and start the service:
` + "`" + `systemctl enable pulse && systemctl start pulse` + "`" + `

The latest version can be found at: https://api.github.com/repos/rcourtman/Pulse/releases/latest
This is a 3-command job. Don't over-investigate.`


	// Add custom context from AI settings (user's infrastructure description)
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg != nil && cfg.CustomContext != "" {
		prompt += "\n\n## User's Infrastructure Description\n"
		prompt += "The user has provided this context about their infrastructure:\n\n"
		prompt += cfg.CustomContext
	}

	// Add connected infrastructure info via unified resource model
	s.mu.RLock()
	hasResourceProvider := s.resourceProvider != nil
	s.mu.RUnlock()
	
	if hasResourceProvider {
		prompt += s.buildUnifiedResourceContext()
	} else {
		log.Warn().Msg("AI context: resource provider not available, infrastructure context will be limited")
	}

	// Add user annotations from all resources (global context)
	prompt += s.buildUserAnnotationsContext()

	// Add current alert status - this gives AI awareness of active issues
	prompt += s.buildAlertContext()

	// Add all saved knowledge when no specific target is selected
	// This gives the AI context about everything learned from previous sessions
	if req.TargetType == "" && s.knowledgeStore != nil {
		prompt += s.knowledgeStore.FormatAllForContext()
	}

	// Add target context if provided
	if req.TargetType != "" {
		guestName := ""
		if name, ok := req.Context["guestName"].(string); ok {
			guestName = name
		} else if name, ok := req.Context["name"].(string); ok {
			guestName = name
		}

		if guestName != "" {
			// Include the node in the focus header so AI can't miss it for routing
			nodeName := ""
			if node, ok := req.Context["node"].(string); ok && node != "" {
				nodeName = node
			} else if node, ok := req.Context["guest_node"].(string); ok && node != "" {
				nodeName = node
			}
			if nodeName != "" {
				prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing **%s** (%s on node **%s**)\n**ROUTING: When using run_on_host=true, set target_host=\"%s\"**",
					guestName, req.TargetType, nodeName, nodeName)
			} else {
				prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing **%s** (%s)", guestName, req.TargetType)
			}
		} else if req.TargetID != "" {
			prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing %s '%s'", req.TargetType, req.TargetID)
		}
	}


	// Add any provided context in a structured way
	if len(req.Context) > 0 {
		prompt += "\n\n## Current Metrics and State"

		// Group metrics by category for better readability
		categories := map[string][]string{
			"Identity":     {"name", "guestName", "type", "vmid", "node", "guest_node", "status", "uptime"},
			"CPU":          {"cpu_usage", "cpu_cores"},
			"Memory":       {"memory_used", "memory_total", "memory_usage", "memory_balloon", "swap_used", "swap_total"},
			"Disk":         {"disk_used", "disk_total", "disk_usage"},
			"I/O Rates":    {"disk_read_rate", "disk_write_rate", "network_in_rate", "network_out_rate"},
			"Backup":       {"backup_status", "last_backup", "days_since_backup"},
			"System Info":  {"os_name", "os_version", "guest_agent", "ip_addresses", "tags"},
			"User Context": {"user_notes", "user_annotations"},
		}

		categoryOrder := []string{"Identity", "User Context", "Backup", "CPU", "Memory", "Disk", "I/O Rates", "System Info"}

		for _, category := range categoryOrder {
			keys := categories[category]
			hasValues := false
			categoryContent := ""

			for _, k := range keys {
				if v, ok := req.Context[k]; ok && v != nil && v != "" {
					if !hasValues {
						categoryContent = fmt.Sprintf("\n### %s", category)
						hasValues = true
					}
					categoryContent += fmt.Sprintf("\n- %s: %v", formatContextKey(k), v)
				}
			}

			if hasValues {
				prompt += categoryContent
			}
		}

		// Add any remaining context that wasn't categorized
		for k, v := range req.Context {
			found := false
			for _, keys := range categories {
				for _, key := range keys {
					if k == key {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found && v != nil && v != "" {
				prompt += fmt.Sprintf("\n- %s: %v", formatContextKey(k), v)
			}
		}
	}

	return prompt
}

// formatContextKey converts snake_case keys to readable labels
func formatContextKey(key string) string {
	replacements := map[string]string{
		"guestName":          "Guest Name",
		"name":               "Name",
		"type":               "Type",
		"vmid":               "VMID",
		"node":               "PVE Node (host)",
		"guest_node":         "PVE Node (host)",
		"status":             "Status",
		"uptime":             "Uptime",
		"cpu_usage":          "CPU Usage",
		"cpu_cores":          "CPU Cores",
		"memory_used":        "Memory Used",
		"memory_total":       "Memory Total",
		"memory_usage":       "Memory Usage",
		"memory_balloon":     "Memory Balloon",
		"swap_used":          "Swap Used",
		"swap_total":         "Swap Total",
		"disk_used":          "Disk Used",
		"disk_total":         "Disk Total",
		"disk_usage":         "Disk Usage",
		"disk_read_rate":     "Disk Read Rate",
		"disk_write_rate":    "Disk Write Rate",
		"network_in_rate":    "Network In Rate",
		"network_out_rate":   "Network Out Rate",
		"backup_status":      "Backup Status",
		"last_backup":        "Last Backup",
		"days_since_backup":  "Days Since Backup",
		"os_name":            "OS Name",
		"os_version":         "OS Version",
		"guest_agent":        "Guest Agent",
		"ip_addresses":       "IP Addresses",
		"tags":               "Tags",
		"user_notes":         "User Notes",
		"user_annotations":   "User Annotations",
	}

	if label, ok := replacements[key]; ok {
		return label
	}
	return key
}

// buildUserAnnotationsContext gathers all user annotations from guests and docker containers
// These provide infrastructure context that the AI should know about for any query
func (s *Service) buildUserAnnotationsContext() string {
	var annotations []string

	// Load guest metadata
	guestMeta, err := s.persistence.LoadGuestMetadata()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load guest metadata for AI context")
	} else {
		log.Debug().Int("count", len(guestMeta)).Msg("Loaded guest metadata for AI context")
		for id, meta := range guestMeta {
			if meta != nil && len(meta.Notes) > 0 {
				// Use LastKnownName if available, otherwise use ID
				name := meta.LastKnownName
				if name == "" {
					name = id
				}
				for _, note := range meta.Notes {
					annotations = append(annotations, fmt.Sprintf("- Guest '%s': %s", name, note))
				}
			}
		}
	}

	// Load docker metadata - include host info for context
	dockerMeta, err := s.persistence.LoadDockerMetadata()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load docker metadata for AI context")
	} else {
		log.Debug().Int("count", len(dockerMeta)).Msg("Loaded docker metadata for AI context")
		for id, meta := range dockerMeta {
			if meta != nil && len(meta.Notes) > 0 {
				// Extract host and container info from ID (format: hostid:container:containerid)
				name := id
				hostInfo := ""
				parts := strings.Split(id, ":")
				if len(parts) >= 3 {
					hostInfo = parts[0] // First part is the host identifier
					containerID := parts[2]
					if len(containerID) > 12 {
						containerID = containerID[:12]
					}
					name = fmt.Sprintf("Docker container %s", containerID)
				}
				log.Debug().Str("name", name).Str("host", hostInfo).Int("notes", len(meta.Notes)).Msg("Found docker container with annotations")
				for _, note := range meta.Notes {
					if hostInfo != "" {
						annotations = append(annotations, fmt.Sprintf("- %s (on host '%s'): %s", name, hostInfo, note))
					} else {
						annotations = append(annotations, fmt.Sprintf("- %s: %s", name, note))
					}
				}
			}
		}
	}

	log.Debug().Int("total_annotations", len(annotations)).Msg("Built user annotations context")

	if len(annotations) == 0 {
		return ""
	}

	return "\n\n## User Infrastructure Notes\nThe user has added these annotations to describe their infrastructure. USE THESE to understand relationships between systems:\n" + strings.Join(annotations, "\n")
}

// TestConnection tests the AI provider connection
func (s *Service) TestConnection(ctx context.Context) error {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		// Try to create a temporary provider from config to test
		cfg, err := s.persistence.LoadAIConfig()
		if err != nil {
			return fmt.Errorf("failed to load AI config: %w", err)
		}
		if cfg == nil || cfg.APIKey == "" {
			return fmt.Errorf("API key not configured")
		}

		tempProvider, err := providers.NewFromConfig(cfg)
		if err != nil {
			return err
		}
		provider = tempProvider
	}

	return provider.TestConnection(ctx)
}

// Reload reloads the AI configuration (call after settings change)
func (s *Service) Reload() error {
	return s.LoadConfig()
}

