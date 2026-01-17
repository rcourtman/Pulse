package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/mcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to infrastructure state
type StateProvider interface {
	GetState() models.StateSnapshot
}

// MCP provider type aliases for external use
type (
	MCPAlertProvider          = mcp.AlertProvider
	MCPFindingsProvider       = mcp.FindingsProvider
	MCPBaselineProvider       = mcp.BaselineProvider
	MCPPatternProvider        = mcp.PatternProvider
	MCPMetricsHistoryProvider = mcp.MetricsHistoryProvider
	MCPBackupProvider         = mcp.BackupProvider
	MCPStorageProvider        = mcp.StorageProvider
	MCPDiskHealthProvider     = mcp.DiskHealthProvider
)

// CommandPolicy evaluates command security
type CommandPolicy interface {
	Evaluate(command string) agentexec.PolicyDecision
}

// AgentServer executes commands on agents
type AgentServer interface {
	GetConnectedAgents() []agentexec.ConnectedAgent
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// convertModelToOpenCode converts Pulse model format (provider:model) to OpenCode format (provider/model)
// Also handles provider name mappings (e.g., gemini -> google)
func convertModelToOpenCode(pulseModel string) string {
	// Replace first colon with slash
	model := strings.Replace(pulseModel, ":", "/", 1)

	// Map Pulse provider names to OpenCode provider names
	// OpenCode uses "google" instead of "gemini" for Gemini models
	if strings.HasPrefix(model, "gemini/") {
		model = "google/" + strings.TrimPrefix(model, "gemini/")
	}

	return model
}

// Service manages OpenCode as Pulse's AI engine
type Service struct {
	mu sync.RWMutex

	sidecar   *Sidecar
	client    *Client
	mcpServer *mcp.Server
	executor  *mcp.PulseToolExecutor

	cfg           *config.AIConfig
	stateProvider StateProvider     // For real-time infrastructure context
	agentServer   AgentServer       // For agent connectivity info
	alertProvider mcp.AlertProvider // For real-time alert context
	started       bool
}

// Config holds OpenCode service configuration
type Config struct {
	AIConfig      *config.AIConfig
	StateProvider StateProvider
	Policy        CommandPolicy
	AgentServer   AgentServer
}

// NewService creates a new OpenCode-powered AI service
func NewService(cfg Config) *Service {
	// Create tool executor with adapted interfaces
	var stateProvider mcp.StateProvider
	var policy mcp.CommandPolicy
	var agentServer mcp.AgentServer

	if cfg.StateProvider != nil {
		stateProvider = &stateProviderAdapter{cfg.StateProvider}
	}
	if cfg.Policy != nil {
		policy = &commandPolicyAdapter{cfg.Policy}
	}
	if cfg.AgentServer != nil {
		agentServer = &agentServerAdapter{cfg.AgentServer}
	}

	// Build executor config
	execCfg := mcp.ExecutorConfig{
		StateProvider: stateProvider,
		Policy:        policy,
		AgentServer:   agentServer,
	}

	// Set control level from AI config
	if cfg.AIConfig != nil {
		execCfg.ControlLevel = mcp.ControlLevel(cfg.AIConfig.GetControlLevel())
		execCfg.ProtectedGuests = cfg.AIConfig.GetProtectedGuests()
	}

	executor := mcp.NewPulseToolExecutor(execCfg)

	return &Service{
		cfg:           cfg.AIConfig,
		stateProvider: cfg.StateProvider,
		agentServer:   cfg.AgentServer,
		executor:      executor,
	}
}

// Adapter types to convert between interfaces

type stateProviderAdapter struct {
	sp StateProvider
}

func (a *stateProviderAdapter) GetState() models.StateSnapshot {
	return a.sp.GetState()
}

type commandPolicyAdapter struct {
	p CommandPolicy
}

func (a *commandPolicyAdapter) Evaluate(command string) agentexec.PolicyDecision {
	return a.p.Evaluate(command)
}

type agentServerAdapter struct {
	s AgentServer
}

func (a *agentServerAdapter) GetConnectedAgents() []agentexec.ConnectedAgent {
	return a.s.GetConnectedAgents()
}

func (a *agentServerAdapter) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	return a.s.ExecuteCommand(ctx, agentID, cmd)
}

// Start starts the OpenCode service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if s.cfg == nil {
		return fmt.Errorf("AI config is nil")
	}

	// Determine data directory
	dataDir := s.cfg.OpenCodeDataDir
	if dataDir == "" {
		dataDir = "/tmp/pulse-opencode"
	}

	// Find a free port for MCP server
	mcpPort, err := findFreePort()
	if err != nil {
		return fmt.Errorf("failed to find MCP port: %w", err)
	}
	mcpAddr := fmt.Sprintf("127.0.0.1:%d", mcpPort)

	// Start MCP server first (OpenCode will connect to it)
	s.mcpServer = mcp.NewServer(mcpAddr, s.executor)
	go func() {
		if err := s.mcpServer.Start(); err != nil {
			log.Error().Err(err).Msg("MCP server error")
		}
	}()

	// Wait for MCP server to start
	time.Sleep(100 * time.Millisecond)

	// Start OpenCode sidecar with API keys from config
	sidecarCfg := SidecarConfig{
		DataDir: dataDir,
		MCPURL:  fmt.Sprintf("http://%s", mcpAddr),
		Port:    s.cfg.OpenCodePort,
	}
	// s.cfg is *config.AIConfig directly
	if s.cfg != nil {
		sidecarCfg.AnthropicAPIKey = s.cfg.AnthropicAPIKey
		sidecarCfg.OpenAIAPIKey = s.cfg.OpenAIAPIKey
		sidecarCfg.DeepSeekAPIKey = s.cfg.DeepSeekAPIKey
		sidecarCfg.GeminiAPIKey = s.cfg.GeminiAPIKey

		// Convert model from Pulse format (provider:model) to OpenCode format (provider/model)
		chatModel := s.cfg.GetChatModel()
		if chatModel != "" {
			sidecarCfg.Model = convertModelToOpenCode(chatModel)
		}
	}
	s.sidecar = NewSidecar(sidecarCfg)

	if err := s.sidecar.Start(ctx); err != nil {
		s.mcpServer.Stop(ctx)
		return fmt.Errorf("failed to start OpenCode: %w", err)
	}

	// Create client
	s.client = NewClient(s.sidecar.BaseURL())

	s.started = true
	log.Info().
		Str("opencode_url", s.sidecar.BaseURL()).
		Str("mcp_addr", mcpAddr).
		Msg("Pulse AI (OpenCode) started")

	return nil
}

// Stop stops the OpenCode service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	var errs []error

	if s.sidecar != nil {
		if err := s.sidecar.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.mcpServer != nil {
		if err := s.mcpServer.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	s.started = false

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping OpenCode: %v", errs)
	}

	log.Info().Msg("Pulse AI (OpenCode) stopped")
	return nil
}

// Restart restarts the OpenCode service with updated configuration
// Call this when settings change (e.g., model selection)
func (s *Service) Restart(ctx context.Context, newCfg *config.AIConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.sidecar == nil {
		return fmt.Errorf("OpenCode not started")
	}

	// Use the new config if provided, otherwise fall back to existing
	cfg := newCfg
	if cfg == nil {
		cfg = s.cfg
	}

	if cfg != nil {
		// Update stored config
		s.cfg = cfg

		chatModel := cfg.GetChatModel()
		if chatModel != "" {
			model := convertModelToOpenCode(chatModel)
			s.sidecar.UpdateModel(model)
			log.Info().Str("model", model).Msg("Updating OpenCode model")
		}

		// Update control settings on the executor (no restart needed)
		if s.executor != nil {
			s.executor.SetControlLevel(mcp.ControlLevel(cfg.GetControlLevel()))
			s.executor.SetProtectedGuests(cfg.GetProtectedGuests())
			log.Info().Str("control_level", cfg.GetControlLevel()).Msg("Updated MCP control settings")
		}
	}

	log.Info().Msg("Restarting OpenCode sidecar with new configuration")
	return s.sidecar.Restart(ctx)
}

// IsRunning returns whether the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.started || s.sidecar == nil {
		return false
	}

	return s.sidecar.IsHealthy()
}

// ExecuteRequest represents a chat request
type ExecuteRequest struct {
	Prompt    string
	SessionID string
	Model     string
}

// Execute sends a prompt and returns the response
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*PromptResponse, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.Prompt(ctx, PromptRequest{
		Prompt:    req.Prompt,
		SessionID: req.SessionID,
		Model:     req.Model,
	})
}

// ExecuteStream sends a prompt and streams the response
func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("OpenCode not initialized")
	}

	// NOTE: Context injection disabled - let the AI use MCP tools to get context when needed
	// The proactive context was making the AI too eager to take action on greetings
	// infraContext := s.buildInfraContext()

	// Stream from OpenCode
	promptModel := req.Model
	if promptModel != "" {
		promptModel = convertModelToOpenCode(promptModel)
	}

	events, errors := client.PromptStream(ctx, PromptRequest{
		Prompt:    req.Prompt,
		SessionID: req.SessionID,
		Model:     promptModel,
	})

	// Use bridge to transform specific events (tool_end -> approval_needed for approvals)
	bridge := NewBridge()

	// Relay events to callback
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errors:
			if err != nil {
				errData, _ := json.Marshal(err.Error())
				callback(StreamEvent{Type: "error", Data: errData})
				return err
			}
		case event, ok := <-events:
			if !ok {
				return nil
			}

			// Only transform tool_end events (to detect approvals)
			// Pass through all other events directly for best compatibility
			if event.Type == "tool_end" {
				transformed, err := bridge.TransformEvent(event)
				if err == nil && transformed.Type == "approval_needed" {
					// This is an approval event - marshal and send
					transformedData, _ := json.Marshal(transformed.Data)
					callback(StreamEvent{
						Type: transformed.Type,
						Data: transformedData,
					})
					continue
				}
			}

			// Pass through event directly
			callback(StreamEvent{
				Type: event.Type,
				Data: event.Data,
			})

			if event.Type == "done" || event.Type == "error" {
				return nil
			}
		}
	}
}

// ListSessions returns all chat sessions
func (s *Service) ListSessions(ctx context.Context) ([]Session, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.ListSessions(ctx)
}

// ListSessionsWithTitles returns all chat sessions with generated titles from the first user message
func (s *Service) ListSessionsWithTitles(ctx context.Context) ([]Session, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	sessions, err := client.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	// Enrich each session with title from first user message
	for i := range sessions {
		messages, err := client.GetMessages(ctx, sessions[i].ID)
		if err != nil {
			log.Debug().Err(err).Str("session_id", sessions[i].ID).Msg("Failed to get messages for session")
			continue
		}

		sessions[i].MessageCount = len(messages)

		// Find first user message to use as title
		for _, msg := range messages {
			if msg.Role == "user" && msg.Content != "" {
				sessions[i].Title = generateSessionTitle(msg.Content)
				break
			}
		}
	}

	return sessions, nil
}

// generateSessionTitle creates a title from the first user message
// Truncates to ~50 chars at word boundary and adds ellipsis if needed
func generateSessionTitle(content string) string {
	// Clean up the content - remove extra whitespace and newlines
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", " ")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	const maxLen = 50

	// Use rune count for proper Unicode handling
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}

	// Find a good break point (space) before maxLen
	truncated := string(runes[:maxLen])
	lastSpace := strings.LastIndex(truncated, " ")

	if lastSpace > 20 {
		// Break at the last space if it's not too early
		return truncated[:lastSpace] + "..."
	}

	// No good break point, just truncate
	return truncated + "..."
}

// CreateSession creates a new chat session
func (s *Service) CreateSession(ctx context.Context) (*Session, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.CreateSession(ctx)
}

// DeleteSession deletes a chat session
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("OpenCode not initialized")
	}

	return client.DeleteSession(ctx, sessionID)
}

// GetMessages returns messages from a session
func (s *Service) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.GetMessages(ctx, sessionID)
}

// AbortSession aborts the current operation
func (s *Service) AbortSession(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("OpenCode not initialized")
	}

	return client.AbortSession(ctx, sessionID)
}

// AnswerQuestion answers a pending question from OpenCode
func (s *Service) AnswerQuestion(ctx context.Context, questionID string, answers []QuestionAnswer) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("OpenCode not initialized")
	}

	return client.AnswerQuestion(ctx, questionID, answers)
}

// SummarizeSession compresses context when nearing model limits
func (s *Service) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.SummarizeSession(ctx, sessionID)
}

// GetSessionDiff returns file changes made during a session
func (s *Service) GetSessionDiff(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.GetSessionDiff(ctx, sessionID)
}

// ForkSession creates a branch point in the conversation
func (s *Service) ForkSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.ForkSession(ctx, sessionID)
}

// RevertSession reverts file changes from the session
func (s *Service) RevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.RevertSession(ctx, sessionID)
}

// UnrevertSession restores previously reverted changes
func (s *Service) UnrevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("OpenCode not initialized")
	}

	return client.UnrevertSession(ctx, sessionID)
}

// GetClient returns the underlying OpenCode client for direct access
func (s *Service) GetClient() *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// GetBaseURL returns the OpenCode server base URL for proxying
func (s *Service) GetBaseURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sidecar == nil {
		return ""
	}
	return s.sidecar.BaseURL()
}

// SetMetadataUpdater sets the metadata updater for resource URL updates
func (s *Service) SetMetadataUpdater(updater mcp.MetadataUpdater) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetMetadataUpdater(updater)
	}
}

// SetFindingsManager sets the findings manager for patrol integration
func (s *Service) SetFindingsManager(manager mcp.FindingsManager) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetFindingsManager(manager)
	}
}

// SetAlertProvider sets the alert provider for active alerts access
func (s *Service) SetAlertProvider(provider mcp.AlertProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.alertProvider = provider // Store for context injection
	if s.executor != nil {
		s.executor.SetAlertProvider(provider)
	}
}

// SetFindingsProvider sets the findings provider for patrol findings access
func (s *Service) SetFindingsProvider(provider mcp.FindingsProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetFindingsProvider(provider)
	}
}

// SetBaselineProvider sets the baseline provider for anomaly detection
func (s *Service) SetBaselineProvider(provider mcp.BaselineProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetBaselineProvider(provider)
	}
}

// SetPatternProvider sets the pattern provider for trend analysis
func (s *Service) SetPatternProvider(provider mcp.PatternProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetPatternProvider(provider)
	}
}

// SetMetricsHistory sets the metrics history provider for historical data
func (s *Service) SetMetricsHistory(provider mcp.MetricsHistoryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetMetricsHistory(provider)
	}
}

// SetBackupProvider sets the backup provider for backup status access
func (s *Service) SetBackupProvider(provider mcp.BackupProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetBackupProvider(provider)
	}
}

// SetStorageProvider sets the storage provider for storage pool information
func (s *Service) SetStorageProvider(provider mcp.StorageProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetStorageProvider(provider)
	}
}

// SetDiskHealthProvider sets the disk health provider for SMART/RAID data
func (s *Service) SetDiskHealthProvider(provider mcp.DiskHealthProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetDiskHealthProvider(provider)
	}
}

// SetAgentProfileManager sets the profile manager for agent scope updates.
func (s *Service) SetAgentProfileManager(manager mcp.AgentProfileManager) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetAgentProfileManager(manager)
	}
}

// SetControlLevel sets the AI control level (read_only, suggest, controlled, autonomous)
func (s *Service) SetControlLevel(level string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetControlLevel(mcp.ControlLevel(level))
	}
}

// SetProtectedGuests sets the list of VMIDs/names that AI cannot control
func (s *Service) SetProtectedGuests(guests []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.executor != nil {
		s.executor.SetProtectedGuests(guests)
	}
}

// UpdateControlSettings updates both control level and protected guests from config
func (s *Service) UpdateControlSettings(cfg *config.AIConfig) {
	if cfg == nil {
		return
	}
	s.SetControlLevel(cfg.GetControlLevel())
	s.SetProtectedGuests(cfg.GetProtectedGuests())
}

// buildInfraContext creates a compact real-time infrastructure context string
// This is injected into every message so the AI always has accurate infrastructure awareness
func (s *Service) buildInfraContext() string {
	s.mu.RLock()
	stateProvider := s.stateProvider
	agentServer := s.agentServer
	s.mu.RUnlock()

	if stateProvider == nil {
		return ""
	}

	state := stateProvider.GetState()

	// Build a map of connected agents for quick lookup
	connectedAgents := make(map[string]bool)
	if agentServer != nil {
		for _, agent := range agentServer.GetConnectedAgents() {
			// Key by both AgentID and Hostname for flexible matching
			connectedAgents[agent.AgentID] = true
			connectedAgents[agent.Hostname] = true
		}
	}

	var sb strings.Builder
	sb.WriteString("\n\n[CONTEXT: Infrastructure state for reference - use pulse_get_active_alerts tool to check alerts]\n")

	// Group VMs and containers by node
	nodeVMs := make(map[string][]models.VM)
	nodeLXCs := make(map[string][]models.Container)
	for _, vm := range state.VMs {
		nodeVMs[vm.Node] = append(nodeVMs[vm.Node], vm)
	}
	for _, ct := range state.Containers {
		nodeLXCs[ct.Node] = append(nodeLXCs[ct.Node], ct)
	}

	// Proxmox nodes
	if len(state.Nodes) > 0 {
		sb.WriteString("\nPROXMOX:\n")
		for _, node := range state.Nodes {
			canExec := connectedAgents[node.Name] || connectedAgents[node.ID]
			execStr := "✗"
			if canExec {
				execStr = "✓"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] exec=%s\n", node.Name, node.Status, execStr))

			// VMs on this node
			if vms := nodeVMs[node.Name]; len(vms) > 0 {
				for _, vm := range vms {
					sb.WriteString(fmt.Sprintf("    VM %d: %s [%s]\n", vm.VMID, vm.Name, vm.Status))
				}
			}

			// LXCs on this node
			if lxcs := nodeLXCs[node.Name]; len(lxcs) > 0 {
				for _, ct := range lxcs {
					sb.WriteString(fmt.Sprintf("    LXC %d: %s [%s]\n", ct.VMID, ct.Name, ct.Status))
				}
			}
		}
	}

	// Docker hosts with containers
	if len(state.DockerHosts) > 0 {
		sb.WriteString("\nDOCKER:\n")
		for _, host := range state.DockerHosts {
			canExec := connectedAgents[host.AgentID] || connectedAgents[host.Hostname]
			execStr := "✗"
			if canExec {
				execStr = "✓"
			}
			displayName := host.Hostname
			if host.DisplayName != "" {
				displayName = host.DisplayName
			}
			sb.WriteString(fmt.Sprintf("  %s exec=%s\n", displayName, execStr))

			// Containers on this host
			running, stopped := 0, 0
			for _, ct := range host.Containers {
				if ct.State == "running" {
					running++
				} else {
					stopped++
				}
			}
			if running > 0 || stopped > 0 {
				sb.WriteString(fmt.Sprintf("    %d running, %d stopped containers\n", running, stopped))
			}
		}
	}

	// Summary
	sb.WriteString(fmt.Sprintf("\nTOTALS: %d nodes, %d VMs, %d LXCs, %d docker hosts\n",
		len(state.Nodes), len(state.VMs), len(state.Containers), len(state.DockerHosts)))

	// Execution model explanation
	sb.WriteString(`
[EXECUTION MODEL]
- exec=✓ on a NODE means commands can run on that Proxmox host
- VMs/LXCs do NOT have agents - to run commands INSIDE them:
  * For LXC: use pulse_run_command on the NODE with command="pct exec <vmid> -- <your_command>"
  * For VM: use pulse_run_command on the NODE with command="qm guest exec <vmid> -- <your_command>"
  * Example: To check memory in LXC 150 on node delly, run "pct exec 150 -- ps aux" on host "delly"
- Docker hosts with exec=✓ can run docker commands directly
`)

	return sb.String()
}
