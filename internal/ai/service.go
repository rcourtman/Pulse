package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
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
	mu            sync.RWMutex
	persistence   *config.ConfigPersistence
	provider      providers.Provider
	cfg           *config.AIConfig
	agentServer   *agentexec.Server
	policy        *agentexec.CommandPolicy
	stateProvider StateProvider
}

// NewService creates a new AI service
func NewService(persistence *config.ConfigPersistence, agentServer *agentexec.Server) *Service {
	return &Service{
		persistence: persistence,
		agentServer: agentServer,
		policy:      agentexec.DefaultPolicy(),
	}
}

// SetStateProvider sets the state provider for infrastructure context
func (s *Service) SetStateProvider(sp StateProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateProvider = sp
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

// IsAutonomous returns true if autonomous mode is enabled
func (s *Service) IsAutonomous() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg != nil && s.cfg.AutonomousMode
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
	Command   string `json:"command"`
	ToolID    string `json:"tool_id"`    // ID to reference when approving
	ToolName  string `json:"tool_name"`  // "run_command", "read_file", etc.
	RunOnHost bool   `json:"run_on_host"`
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
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
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
			callback(StreamEvent{Type: "error", Data: err.Error()})
			return nil, fmt.Errorf("AI request failed: %w", err)
		}

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens
		model = resp.Model
		finalContent = resp.Content

		log.Debug().
			Int("tool_calls", len(resp.ToolCalls)).
			Str("stop_reason", resp.StopReason).
			Int("iteration", i+1).
			Int("total_input_tokens", totalInputTokens).
			Int("total_output_tokens", totalOutputTokens).
			Msg("AI streaming iteration complete")

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			log.Info().
				Int("tool_calls", len(resp.ToolCalls)).
				Str("stop_reason", resp.StopReason).
				Int("iteration", i+1).
				Msg("AI streaming loop ending - no more tool calls or stop_reason != tool_use")
			break
		}

		// Add assistant's response with tool calls to messages
		messages = append(messages, providers.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and add results
		for _, tc := range resp.ToolCalls {
			toolInput := s.getToolInputDisplay(tc)

			// Check if this command needs approval
			needsApproval := false
			if tc.Name == "run_command" {
				cmd, _ := tc.Input["command"].(string)
				runOnHost, _ := tc.Input["run_on_host"].(bool)

				isAuto := s.IsAutonomous()
				isReadOnly := isReadOnlyCommand(cmd)
				log.Debug().
					Bool("autonomous", isAuto).
					Bool("read_only", isReadOnly).
					Str("command", cmd).
					Msg("Checking command approval")

				// In non-autonomous mode, non-read-only commands need approval
				if !isAuto && !isReadOnly {
					needsApproval = true
					// Send approval needed event
					callback(StreamEvent{
						Type: "approval_needed",
						Data: ApprovalNeededData{
							Command:   cmd,
							ToolID:    tc.ID,
							ToolName:  tc.Name,
							RunOnHost: runOnHost,
						},
					})
				}
			}

			var result string
			var execution ToolExecution

			if needsApproval {
				// Don't execute - tell the AI the command needs user approval
				result = "This command requires user approval. The command was not executed. Please suggest the command to the user and let them decide whether to run it."
				execution = ToolExecution{
					Name:    tc.Name,
					Input:   toolInput,
					Output:  result,
					Success: false,
				}
				toolExecutions = append(toolExecutions, execution)
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
			Description: "Execute a shell command. By default runs on the current target, but you can override to run on the Proxmox host for operations like resizing disks, managing containers, etc.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (e.g., 'ps aux --sort=-%mem | head -20')",
					},
					"run_on_host": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, run on the Proxmox host instead of inside the container/VM. Use this for pct/qm commands like 'pct resize 101 rootfs +10G'",
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
		execution.Input = command
		if runOnHost {
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
				// For now, just inform the AI. Future: implement approval workflow
				execution.Output = fmt.Sprintf("This command requires user approval: %s\nThe command was not executed. Please suggest the command to the user and let them decide whether to run it.", command)
				execution.Success = true // Not an error, just needs approval
				return execution.Output, execution
			}
		}

		// If run_on_host is true, override the target type to run on host
		execReq := req
		if runOnHost {
			execReq.TargetType = "host"
			execReq.TargetID = ""
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

	default:
		execution.Output = fmt.Sprintf("Unknown tool: %s", tc.Name)
		return execution.Output, execution
	}
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

// executeOnAgent executes a command via the agent WebSocket
func (s *Service) executeOnAgent(ctx context.Context, req ExecuteRequest, command string) (string, error) {
	if s.agentServer == nil {
		return "", fmt.Errorf("agent server not available")
	}

	// Find the appropriate agent
	agents := s.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return "", fmt.Errorf("no agents connected")
	}

	// Route to the correct agent based on target
	// For containers/VMs, we need to route to the PVE host that owns them
	agentID := ""
	targetNode := ""

	// CRITICAL: For pct/qm commands, extract the VMID from the command itself
	// and look up the authoritative node from our state. This prevents the AI
	// from trying to run commands on the wrong node.
	//
	// Commands are classified as:
	// - Node-specific (pct exec, qm start, etc): MUST run on the node that owns the guest
	// - Cluster-aware (vzdump, etc): Can run from any cluster node
	if vmid, requiresOwnerNode, found := extractVMIDFromCommand(command); found {
		// Try to get instance from context for multi-cluster disambiguation
		targetInstance := ""
		if inst, ok := req.Context["instance"].(string); ok {
			targetInstance = inst
		}

		// Look up guests with this VMID, optionally filtered by instance
		guests := s.lookupGuestsByVMID(vmid, targetInstance)

		if len(guests) == 1 && requiresOwnerNode {
			// Single match - route to the owning node
			log.Info().
				Int("vmid", vmid).
				Str("actual_node", guests[0].Node).
				Str("guest_name", guests[0].Name).
				Str("guest_type", guests[0].Type).
				Str("instance", guests[0].Instance).
				Bool("requires_owner_node", requiresOwnerNode).
				Msg("Auto-routing command to correct node based on VMID lookup")
			targetNode = strings.ToLower(guests[0].Node)
		} else if len(guests) > 1 && requiresOwnerNode {
			// Multiple matches - VMID collision across instances
			// Try to disambiguate using context
			if targetInstance != "" {
				// Filter by instance
				for _, g := range guests {
					if g.Instance == targetInstance {
						log.Info().
							Int("vmid", vmid).
							Str("actual_node", g.Node).
							Str("guest_name", g.Name).
							Str("instance", g.Instance).
							Msg("Resolved VMID collision using instance context")
						targetNode = strings.ToLower(g.Node)
						break
					}
				}
			}
			if targetNode == "" {
				// Can't disambiguate - log warning and use first match
				log.Warn().
					Int("vmid", vmid).
					Int("matches", len(guests)).
					Msg("VMID collision detected - using first match, may route to wrong cluster")
				targetNode = strings.ToLower(guests[0].Node)
			}
		} else if len(guests) == 1 {
			// Cluster-aware command with single match - log for debugging
			log.Debug().
				Int("vmid", vmid).
				Str("actual_node", guests[0].Node).
				Str("guest_name", guests[0].Name).
				Bool("requires_owner_node", requiresOwnerNode).
				Msg("Cluster-aware command, using default routing")
		} else if requiresOwnerNode {
			// VMID not found in our state - this could be a problem
			// Log a warning but let it proceed (might be a newly created guest)
			log.Warn().
				Int("vmid", vmid).
				Str("command", command).
				Msg("VMID not found in state - command may fail if routed to wrong node")
		}
	}

	// Fall back to context-based routing if VMID lookup didn't find anything
	if targetNode == "" {
		// For host targets, use the hostname directly from context
		if req.TargetType == "host" {
			if hostname, ok := req.Context["hostname"].(string); ok && hostname != "" {
				targetNode = strings.ToLower(hostname)
				log.Debug().
					Str("hostname", hostname).
					Str("target_type", req.TargetType).
					Msg("Using hostname from context for host target routing")
			}
		}
		// For VMs/containers, extract node info from target ID (e.g., "delly-135" -> "delly")
		// or from context (guest_node field)
		if targetNode == "" {
			if node, ok := req.Context["guest_node"].(string); ok && node != "" {
				targetNode = strings.ToLower(node)
			} else if req.TargetID != "" {
				parts := strings.Split(req.TargetID, "-")
				if len(parts) >= 2 {
					targetNode = strings.ToLower(parts[0])
				}
			}
		}
	}

	// Try to find an agent that matches the target node
	if targetNode != "" {
		for _, agent := range agents {
			if strings.ToLower(agent.Hostname) == targetNode ||
				strings.Contains(strings.ToLower(agent.Hostname), targetNode) ||
				strings.Contains(targetNode, strings.ToLower(agent.Hostname)) {
				agentID = agent.AgentID
				log.Debug().
					Str("target_node", targetNode).
					Str("matched_agent", agent.Hostname).
					Str("agent_id", agentID).
					Msg("Routed command to matching agent")
				break
			}
		}
	}

	// If no direct match, try to find an agent on a cluster peer
	if agentID == "" && targetNode != "" {
		agentID = s.findClusterPeerAgent(targetNode, agents)
	}

	// Fall back to first agent if no match found
	if agentID == "" {
		agentID = agents[0].AgentID
		log.Debug().
			Str("target_node", targetNode).
			Str("fallback_agent", agents[0].Hostname).
			Msg("No matching agent found, using first available")
	}

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
	cmd := agentexec.ExecuteCommandPayload{
		RequestID:  requestID,
		Command:    command,
		TargetType: req.TargetType,
		TargetID:   targetID,
		Timeout:    300, // 5 minutes - commands like du, backups, etc. can take a while
	}

	result, err := s.agentServer.ExecuteCommand(ctx, agentID, cmd)
	if err != nil {
		return "", err
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
	RunOnHost  bool   `json:"run_on_host"` // If true, run on host instead of target
	VMID       string `json:"vmid,omitempty"`
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

## Using Context Data
Pulse provides real metrics in "Current Metrics and State". Use this data directly - don't ask users to check things you already know.

## Command Execution
- run_on_host=true: Run on PVE host (pct, qm, vzdump commands)
- run_on_host=false: Run inside the container/VM
- Execute commands to investigate, don't just explain what commands to run

## CRITICAL: Proxmox Command Routing
Commands like 'pct exec', 'pct enter', 'qm guest exec' MUST run on the specific PVE node where the guest lives.
- Check the 'node' field in the target context to know which node hosts this guest
- If the guest is on node X but you only have an agent on node Y (even in the same cluster), pct/qm commands will FAIL
- Error "Configuration file does not exist" means the guest is on a different node than where you're running the command
- In clusters, vzdump and pvesh commands can run from any node, but pct exec/qm guest exec cannot

Before running pct/qm commands:
1. Check which node hosts the guest (from context 'node' field)
2. Check if that specific node has an agent connected
3. If no agent on that node, tell the user you cannot run commands inside this guest

If no agent is connected to the host where the target lives, tell the user you cannot reach it.`

	// Add connected infrastructure info
	prompt += s.buildInfrastructureContext()

	// Add user annotations from all resources (global context)
	prompt += s.buildUserAnnotationsContext()

	// Add target context if provided
	if req.TargetType != "" {
		guestName := ""
		if name, ok := req.Context["guestName"].(string); ok {
			guestName = name
		} else if name, ok := req.Context["name"].(string); ok {
			guestName = name
		}

		if guestName != "" {
			prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing **%s** (%s)", guestName, req.TargetType)
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

// buildInfrastructureContext returns detailed info about what Pulse can see and control
// This enables the AI to reason about cross-system troubleshooting paths
func (s *Service) buildInfrastructureContext() string {
	var sections []string

	// Load nodes config for infrastructure info
	nodesConfig, err := s.persistence.LoadNodesConfig()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to load nodes config for AI context")
		return ""
	}

	// Build agent lookup map for matching
	agentsByHostname := make(map[string]string) // hostname -> agentID
	var connectedAgents []string
	if s.agentServer != nil {
		agents := s.agentServer.GetConnectedAgents()
		for _, agent := range agents {
			agentsByHostname[strings.ToLower(agent.Hostname)] = agent.AgentID
			connectedAgents = append(connectedAgents, agent.Hostname)
		}
	}

	// List PVE instances with agent status and cluster info
	if nodesConfig != nil && len(nodesConfig.PVEInstances) > 0 {
		sections = append(sections, "### Proxmox VE Nodes")

		// Group nodes by cluster for better context
		clusterNodes := make(map[string][]string) // clusterName -> list of node names
		nodeHasAgent := make(map[string]bool)

		for _, pve := range nodesConfig.PVEInstances {
			hasAgent := false
			for _, hostname := range connectedAgents {
				// Check if agent hostname matches or contains PVE name
				if strings.Contains(strings.ToLower(hostname), strings.ToLower(pve.Name)) ||
					strings.Contains(strings.ToLower(pve.Name), strings.ToLower(hostname)) {
					hasAgent = true
					break
				}
			}
			nodeHasAgent[pve.Name] = hasAgent

			agentStatus := "NO AGENT"
			if hasAgent {
				agentStatus = "HAS AGENT"
			}

			// Build cluster membership info
			clusterInfo := ""
			if pve.IsCluster && pve.ClusterName != "" {
				clusterInfo = fmt.Sprintf(" [cluster: %s]", pve.ClusterName)
				clusterNodes[pve.ClusterName] = append(clusterNodes[pve.ClusterName], pve.Name)

				// Also add cluster endpoints as members
				for _, ep := range pve.ClusterEndpoints {
					if ep.NodeName != pve.Name {
						clusterNodes[pve.ClusterName] = append(clusterNodes[pve.ClusterName], ep.NodeName)
					}
				}
			}

			sections = append(sections, fmt.Sprintf("- **%s** (%s): %s%s", pve.Name, pve.Host, agentStatus, clusterInfo))
		}

		// Add cluster-aware execution hints
		for clusterName, nodes := range clusterNodes {
			// Check if any node in the cluster has an agent
			var agentNodes []string
			for _, node := range nodes {
				if nodeHasAgent[node] {
					agentNodes = append(agentNodes, node)
				}
			}
			if len(agentNodes) > 0 {
				// Remove duplicates from nodes list
				uniqueNodes := make(map[string]bool)
				for _, n := range nodes {
					uniqueNodes[n] = true
				}
				var nodeList []string
				for n := range uniqueNodes {
					nodeList = append(nodeList, n)
				}
				sections = append(sections, fmt.Sprintf("\n**Cluster %s**: Nodes %v share storage and can manage each other's VMs/containers. Agent on %v can run pct/qm commands for guests on ANY node in this cluster.",
					clusterName, nodeList, agentNodes))
			}
		}
	}

	// List PBS instances
	if nodesConfig != nil && len(nodesConfig.PBSInstances) > 0 {
		sections = append(sections, "\n### Proxmox Backup Servers (PBS)")
		sections = append(sections, "Pulse automatically fetches backup data from these PBS servers. Backup status in guest context comes from here.")
		for _, pbs := range nodesConfig.PBSInstances {
			sections = append(sections, fmt.Sprintf("- **%s** (%s)", pbs.Name, pbs.Host))
		}
	} else {
		sections = append(sections, "\n### Proxmox Backup Servers: NONE")
		sections = append(sections, "No PBS connected - backup_status will show 'NEVER' for all guests")
	}

	// List connected agents with their capabilities
	if len(connectedAgents) > 0 {
		sections = append(sections, "\n### Connected Agents (Command Execution)")
		sections = append(sections, "These hosts have the pulse-agent installed. You can run commands on them.")
		for _, hostname := range connectedAgents {
			// Check if this agent is a Docker host by looking at docker metadata
			dockerMeta, _ := s.persistence.LoadDockerMetadata()
			hasDocker := false
			if dockerMeta != nil {
				for id := range dockerMeta {
					if strings.Contains(strings.ToLower(id), strings.ToLower(hostname)) {
						hasDocker = true
						break
					}
				}
			}

			capabilities := "shell commands, file reads"
			if hasDocker {
				capabilities = "shell commands, file reads, Docker (docker ps, docker exec, docker logs)"
			}
			sections = append(sections, fmt.Sprintf("- **%s**: %s", hostname, capabilities))
		}
	} else {
		sections = append(sections, "\n### Connected Agents: NONE")
		sections = append(sections, "No agents connected - cannot execute commands on any host")
	}

	// Add full guest inventory if state provider is available
	s.mu.RLock()
	stateProvider := s.stateProvider
	s.mu.RUnlock()

	if stateProvider != nil {
		state := stateProvider.GetState()

		// Group guests by node for better readability
		guestsByNode := make(map[string][]string)

		// Add containers
		for _, ct := range state.Containers {
			if ct.Template {
				continue // Skip templates
			}
			ips := ""
			if len(ct.IPAddresses) > 0 {
				ips = " - " + strings.Join(ct.IPAddresses, ", ")
			}
			entry := fmt.Sprintf("  - **%s** (LXC %d)%s [%s]", ct.Name, ct.VMID, ips, ct.Status)
			guestsByNode[ct.Node] = append(guestsByNode[ct.Node], entry)
		}

		// Add VMs
		for _, vm := range state.VMs {
			if vm.Template {
				continue // Skip templates
			}
			ips := ""
			if len(vm.IPAddresses) > 0 {
				ips = " - " + strings.Join(vm.IPAddresses, ", ")
			}
			entry := fmt.Sprintf("  - **%s** (VM %d)%s [%s]", vm.Name, vm.VMID, ips, vm.Status)
			guestsByNode[vm.Node] = append(guestsByNode[vm.Node], entry)
		}

		if len(guestsByNode) > 0 {
			sections = append(sections, "\n### All Guests (VMs & Containers)")
			sections = append(sections, "Complete list of guests Pulse knows about. Use VMID with pct/qm commands.")
			for node, guests := range guestsByNode {
				sections = append(sections, fmt.Sprintf("\n**Node %s:**", node))
				for _, guest := range guests {
					sections = append(sections, guest)
				}
			}
		}
	}

	if len(sections) == 0 {
		return ""
	}

	return "\n\n## Infrastructure Map\n" + strings.Join(sections, "\n")
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

// findClusterPeerAgent looks for an agent on a node that's in the same Proxmox cluster
// as the target node. This allows running pct/qm commands for guests on other cluster nodes.
func (s *Service) findClusterPeerAgent(targetNode string, agents []agentexec.ConnectedAgent) string {
	nodesConfig, err := s.persistence.LoadNodesConfig()
	if err != nil || nodesConfig == nil {
		return ""
	}

	// Find which cluster the target node belongs to
	var targetCluster string
	var clusterNodes []string

	for _, pve := range nodesConfig.PVEInstances {
		if !pve.IsCluster || pve.ClusterName == "" {
			continue
		}

		// Check if target node matches this PVE instance or its cluster endpoints
		isInCluster := strings.EqualFold(pve.Name, targetNode)
		if !isInCluster {
			for _, ep := range pve.ClusterEndpoints {
				if strings.EqualFold(ep.NodeName, targetNode) {
					isInCluster = true
					break
				}
			}
		}

		if isInCluster {
			targetCluster = pve.ClusterName
			// Collect all nodes in this cluster
			clusterNodes = append(clusterNodes, pve.Name)
			for _, ep := range pve.ClusterEndpoints {
				clusterNodes = append(clusterNodes, ep.NodeName)
			}
			break
		}
	}

	if targetCluster == "" {
		return ""
	}

	// Look for an agent on any node in the same cluster
	for _, agent := range agents {
		agentHostLower := strings.ToLower(agent.Hostname)
		for _, clusterNode := range clusterNodes {
			if strings.EqualFold(clusterNode, agent.Hostname) ||
				strings.Contains(agentHostLower, strings.ToLower(clusterNode)) ||
				strings.Contains(strings.ToLower(clusterNode), agentHostLower) {
				log.Debug().
					Str("target_node", targetNode).
					Str("cluster", targetCluster).
					Str("peer_agent", agent.Hostname).
					Str("agent_id", agent.AgentID).
					Msg("Found cluster peer agent for cross-node command execution")
				return agent.AgentID
			}
		}
	}

	return ""
}
