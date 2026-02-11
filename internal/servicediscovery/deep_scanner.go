package servicediscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// CommandExecutor executes commands on infrastructure.
type CommandExecutor interface {
	ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error)
	GetConnectedAgents() []ConnectedAgent
	IsAgentConnected(agentID string) bool
}

// ExecuteCommandPayload mirrors agentexec.ExecuteCommandPayload
type ExecuteCommandPayload struct {
	RequestID  string `json:"request_id"`
	Command    string `json:"command"`
	TargetType string `json:"target_type"`         // "host", "container", "vm"
	TargetID   string `json:"target_id,omitempty"` // VMID for container/VM
	Timeout    int    `json:"timeout,omitempty"`
}

// CommandResultPayload mirrors agentexec.CommandResultPayload
type CommandResultPayload struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms"`
}

// ConnectedAgent mirrors agentexec.ConnectedAgent
type ConnectedAgent struct {
	AgentID     string
	Hostname    string
	Version     string
	Platform    string
	Tags        []string
	ConnectedAt time.Time
}

// ProgressCallback is called when discovery progress changes.
type ProgressCallback func(*DiscoveryProgress)

// DeepScanner runs discovery commands on resources.
type DeepScanner struct {
	executor         CommandExecutor
	mu               sync.RWMutex
	progress         map[string]*DiscoveryProgress // resourceID -> progress
	maxParallel      int
	timeout          time.Duration
	progressCallback ProgressCallback
}

const (
	defaultDeepScannerMaxParallel = 3
	defaultDeepScannerTimeout     = 30 * time.Second
)

// NewDeepScanner creates a new deep scanner.
func NewDeepScanner(executor CommandExecutor) *DeepScanner {
	return &DeepScanner{
		executor:    executor,
		progress:    make(map[string]*DiscoveryProgress),
		maxParallel: defaultDeepScannerMaxParallel, // Run up to 3 commands in parallel per resource
		timeout:     defaultDeepScannerTimeout,
	}
}

func (s *DeepScanner) runtimeSettings() (int, time.Duration) {
	s.mu.RLock()
	maxParallel := s.maxParallel
	timeout := s.timeout
	s.mu.RUnlock()

	if maxParallel <= 0 {
		log.Warn().
			Int("max_parallel", maxParallel).
			Int("default", defaultDeepScannerMaxParallel).
			Msg("Invalid deep scanner max parallelism; using default")
		maxParallel = defaultDeepScannerMaxParallel
	}
	if timeout <= 0 {
		log.Warn().
			Dur("timeout", timeout).
			Dur("default", defaultDeepScannerTimeout).
			Msg("Invalid deep scanner timeout; using default")
		timeout = defaultDeepScannerTimeout
	}

	return maxParallel, timeout
}

// SetProgressCallback sets a callback function that will be called when discovery progress changes.
func (s *DeepScanner) SetProgressCallback(callback ProgressCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progressCallback = callback
}

// notifyProgress calls the progress callback if set.
func (s *DeepScanner) notifyProgress(progress *DiscoveryProgress) {
	s.mu.RLock()
	callback := s.progressCallback
	s.mu.RUnlock()

	if callback != nil && progress != nil {
		// Calculate elapsed time and percent complete
		progressCopy := *progress
		if !progress.StartedAt.IsZero() {
			progressCopy.ElapsedMs = time.Since(progress.StartedAt).Milliseconds()
		}
		if progress.TotalSteps > 0 {
			progressCopy.PercentComplete = float64(progress.CompletedSteps) / float64(progress.TotalSteps) * 100
		}
		callback(&progressCopy)
	}
}

// ScanResult contains the results of a deep scan.
type ScanResult struct {
	ResourceType   ResourceType
	ResourceID     string
	HostID         string
	Hostname       string
	CommandOutputs map[string]string
	Errors         map[string]string
	StartedAt      time.Time
	CompletedAt    time.Time
}

// Scan runs discovery commands on a resource and returns the outputs.
func (s *DeepScanner) Scan(ctx context.Context, req DiscoveryRequest) (*ScanResult, error) {
	resourceID := MakeResourceID(req.ResourceType, req.HostID, req.ResourceID)
	startTime := time.Now()
	maxParallel, timeout := s.runtimeSettings()

	// Initialize progress
	s.mu.Lock()
	s.progress[resourceID] = &DiscoveryProgress{
		ResourceID:  resourceID,
		Status:      DiscoveryStatusRunning,
		CurrentStep: "initializing",
		StartedAt:   startTime,
	}
	initialProgress := *s.progress[resourceID]
	s.mu.Unlock()

	// Broadcast scan start
	s.notifyProgress(&initialProgress)

	defer func() {
		s.mu.Lock()
		delete(s.progress, resourceID)
		s.mu.Unlock()
	}()

	result := &ScanResult{
		ResourceType:   req.ResourceType,
		ResourceID:     req.ResourceID,
		HostID:         req.HostID,
		Hostname:       req.Hostname,
		CommandOutputs: make(map[string]string),
		Errors:         make(map[string]string),
		StartedAt:      time.Now(),
	}

	// Check if we have an agent for this host
	if s.executor == nil {
		return nil, fmt.Errorf("no command executor available")
	}

	// Find the agent for this host
	agentID := s.findAgentForHost(req.HostID, req.Hostname)
	if agentID == "" {
		return nil, fmt.Errorf("no connected agent for host %s (%s)", req.HostID, req.Hostname)
	}

	// Get commands for this resource type
	commands := GetCommandsForResource(req.ResourceType)
	if len(commands) == 0 {
		return nil, fmt.Errorf("no commands defined for resource type %s", req.ResourceType)
	}

	// Update progress
	s.mu.Lock()
	if prog, ok := s.progress[resourceID]; ok {
		prog.TotalSteps = len(commands)
		prog.CurrentStep = "running commands"
		progressCopy := *prog
		s.mu.Unlock()
		s.notifyProgress(&progressCopy)
	} else {
		s.mu.Unlock()
	}

	// Run commands with limited parallelism
	semaphore := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, cmd := range commands {
		wg.Add(1)
		go func(cmd DiscoveryCommand) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			// Build the actual command to run
			actualCmd := s.buildCommand(req.ResourceType, req.ResourceID, cmd.Command)

			// Get the target ID for the agent
			targetID := s.getTargetID(req.ResourceType, req.ResourceID)

			// Only validate TargetID when it will be interpolated into shell commands
			// by the agent (container/vm types). Host/docker types don't use TargetID
			// in command wrapping, so they can have any format (including colons for IPv6).
			targetType := s.getTargetType(req.ResourceType)
			if targetType == "container" || targetType == "vm" {
				if err := ValidateResourceID(targetID); err != nil {
					mu.Lock()
					result.Errors[cmd.Name] = fmt.Sprintf("invalid target ID: %v", err)
					mu.Unlock()
					return
				}
			}

			// Execute the command
			cmdCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmdResult, err := s.executor.ExecuteCommand(cmdCtx, agentID, ExecuteCommandPayload{
				RequestID:  uuid.New().String(),
				Command:    actualCmd,
				TargetType: s.getTargetType(req.ResourceType),
				TargetID:   targetID,
				Timeout:    cmd.Timeout,
			})

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if !cmd.Optional {
					result.Errors[cmd.Name] = err.Error()
				}
				log.Debug().
					Err(err).
					Str("command", cmd.Name).
					Str("resource", resourceID).
					Msg("Command failed during discovery")
				return
			}

			if cmdResult != nil {
				output := cmdResult.Stdout
				if cmdResult.Stderr != "" && output != "" {
					output += "\n--- stderr ---\n" + cmdResult.Stderr
				} else if cmdResult.Stderr != "" {
					output = cmdResult.Stderr
				}

				if output != "" {
					result.CommandOutputs[cmd.Name] = output
				}

				if !cmdResult.Success && cmdResult.Error != "" && !cmd.Optional {
					result.Errors[cmd.Name] = cmdResult.Error
				}
			}

			// Update progress and broadcast
			s.mu.Lock()
			if prog, ok := s.progress[resourceID]; ok {
				prog.CompletedSteps++
				prog.CurrentCommand = cmd.Name
				progressCopy := *prog
				s.mu.Unlock()
				s.notifyProgress(&progressCopy)
			} else {
				s.mu.Unlock()
			}
		}(cmd)
	}

	wg.Wait()
	result.CompletedAt = time.Now()

	// Broadcast scan completion
	completionProgress := DiscoveryProgress{
		ResourceID:      resourceID,
		Status:          DiscoveryStatusCompleted,
		CurrentStep:     "completed",
		TotalSteps:      len(commands),
		CompletedSteps:  len(commands),
		StartedAt:       startTime,
		ElapsedMs:       result.CompletedAt.Sub(startTime).Milliseconds(),
		PercentComplete: 100,
	}
	s.notifyProgress(&completionProgress)

	log.Info().
		Str("resource", resourceID).
		Int("outputs", len(result.CommandOutputs)).
		Int("errors", len(result.Errors)).
		Dur("duration", result.CompletedAt.Sub(result.StartedAt)).
		Msg("Deep scan completed")

	return result, nil
}

// buildCommand wraps the command appropriately for the resource type.
// NOTE: For LXC/VM, the agent handles wrapping via pct exec / qm guest exec
// based on TargetType, so we don't wrap here. We only wrap for Docker containers
// since Docker isn't a recognized TargetType in the agent.
func (s *DeepScanner) buildCommand(resourceType ResourceType, resourceID string, cmd string) string {
	switch resourceType {
	case ResourceTypeLXC:
		// Agent wraps with pct exec based on TargetType="container"
		return cmd
	case ResourceTypeVM:
		// Agent wraps with qm guest exec based on TargetType="vm"
		return cmd
	case ResourceTypeDocker:
		// Docker needs wrapping here since agent doesn't handle it
		return BuildDockerCommand(resourceID, cmd)
	case ResourceTypeHost:
		// Commands run directly on host
		return cmd
	case ResourceTypeDockerLXC:
		// Docker inside LXC - agent wraps with pct exec, we just add docker exec
		// resourceID format: "vmid:container_name"
		parts := splitResourceID(resourceID)
		if len(parts) >= 2 {
			return BuildDockerCommand(parts[1], cmd)
		}
		return cmd
	case ResourceTypeDockerVM:
		// Docker inside VM - agent wraps with qm guest exec, we just add docker exec
		parts := splitResourceID(resourceID)
		if len(parts) >= 2 {
			return BuildDockerCommand(parts[1], cmd)
		}
		return cmd
	default:
		return cmd
	}
}

// getTargetType returns the target type for the agent execution payload.
func (s *DeepScanner) getTargetType(resourceType ResourceType) string {
	switch resourceType {
	case ResourceTypeLXC:
		return "container"
	case ResourceTypeVM:
		return "vm"
	case ResourceTypeDocker:
		return "host" // Docker commands run on host via docker exec
	case ResourceTypeDockerLXC:
		return "container" // Docker inside LXC: agent wraps with pct exec
	case ResourceTypeDockerVM:
		return "vm" // Docker inside VM: agent wraps with qm guest exec
	case ResourceTypeHost:
		return "host"
	default:
		return "host"
	}
}

// getTargetID returns the target ID for the agent execution payload.
// For nested Docker (docker_lxc/docker_vm), this extracts just the vmid.
func (s *DeepScanner) getTargetID(resourceType ResourceType, resourceID string) string {
	switch resourceType {
	case ResourceTypeDockerLXC, ResourceTypeDockerVM:
		// resourceID format: "vmid:container_name" - extract just vmid
		parts := splitResourceID(resourceID)
		if len(parts) >= 1 {
			return parts[0]
		}
		return resourceID
	default:
		return resourceID
	}
}

// findAgentForHost finds the agent ID for a given host.
func (s *DeepScanner) findAgentForHost(hostID, hostname string) string {
	agents := s.executor.GetConnectedAgents()

	log.Debug().
		Str("hostID", hostID).
		Str("hostname", hostname).
		Int("connected_agents", len(agents)).
		Msg("Finding agent for host")

	// Log connected agents for debugging
	for _, agent := range agents {
		log.Debug().
			Str("agent_id", agent.AgentID).
			Str("agent_hostname", agent.Hostname).
			Msg("Connected agent")
	}

	// First try exact match on agent ID
	for _, agent := range agents {
		if agent.AgentID == hostID {
			return agent.AgentID
		}
	}

	// Then try hostname match
	for _, agent := range agents {
		if agent.Hostname == hostname || agent.Hostname == hostID {
			return agent.AgentID
		}
	}

	// If only one agent connected, use it
	if len(agents) == 1 {
		return agents[0].AgentID
	}

	return ""
}

// GetProgress returns a copy of the current progress of a scan.
// Returns nil if no scan is in progress for the resource.
// A copy is returned to avoid data races with the scan goroutine.
func (s *DeepScanner) GetProgress(resourceID string) *DiscoveryProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if prog, ok := s.progress[resourceID]; ok {
		// Return a copy to avoid race with scan goroutine
		copy := *prog
		return &copy
	}
	return nil
}

// IsScanning returns whether a resource is currently being scanned.
func (s *DeepScanner) IsScanning(resourceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.progress[resourceID]
	return ok
}

// splitResourceID splits a compound resource ID (e.g., "101:container_name").
func splitResourceID(id string) []string {
	var parts []string
	start := 0
	for i, c := range id {
		if c == ':' {
			parts = append(parts, id[start:i])
			start = i + 1
		}
	}
	if start < len(id) {
		parts = append(parts, id[start:])
	}
	return parts
}

// ScanDocker runs discovery on Docker containers via the host.
func (s *DeepScanner) ScanDocker(ctx context.Context, hostID, hostname, containerName string) (*ScanResult, error) {
	req := DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   containerName,
		HostID:       hostID,
		Hostname:     hostname,
	}
	return s.Scan(ctx, req)
}

// ScanLXC runs discovery on an LXC container.
func (s *DeepScanner) ScanLXC(ctx context.Context, hostID, hostname, vmid string) (*ScanResult, error) {
	req := DiscoveryRequest{
		ResourceType: ResourceTypeLXC,
		ResourceID:   vmid,
		HostID:       hostID,
		Hostname:     hostname,
	}
	return s.Scan(ctx, req)
}

// ScanVM runs discovery on a VM via QEMU guest agent.
func (s *DeepScanner) ScanVM(ctx context.Context, hostID, hostname, vmid string) (*ScanResult, error) {
	req := DiscoveryRequest{
		ResourceType: ResourceTypeVM,
		ResourceID:   vmid,
		HostID:       hostID,
		Hostname:     hostname,
	}
	return s.Scan(ctx, req)
}

// ScanHost runs discovery on a host system.
func (s *DeepScanner) ScanHost(ctx context.Context, hostID, hostname string) (*ScanResult, error) {
	req := DiscoveryRequest{
		ResourceType: ResourceTypeHost,
		ResourceID:   hostID,
		HostID:       hostID,
		Hostname:     hostname,
	}
	return s.Scan(ctx, req)
}
