package aidiscovery

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

// DeepScanner runs discovery commands on resources.
type DeepScanner struct {
	executor    CommandExecutor
	mu          sync.RWMutex
	progress    map[string]*DiscoveryProgress // resourceID -> progress
	maxParallel int
	timeout     time.Duration
}

// NewDeepScanner creates a new deep scanner.
func NewDeepScanner(executor CommandExecutor) *DeepScanner {
	return &DeepScanner{
		executor:    executor,
		progress:    make(map[string]*DiscoveryProgress),
		maxParallel: 3, // Run up to 3 commands in parallel per resource
		timeout:     30 * time.Second,
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

	// Initialize progress
	s.mu.Lock()
	s.progress[resourceID] = &DiscoveryProgress{
		ResourceID:  resourceID,
		Status:      DiscoveryStatusRunning,
		CurrentStep: "initializing",
		StartedAt:   time.Now(),
	}
	s.mu.Unlock()

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
	}
	s.mu.Unlock()

	// Run commands with limited parallelism
	semaphore := make(chan struct{}, s.maxParallel)
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

			// Execute the command
			cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()

			cmdResult, err := s.executor.ExecuteCommand(cmdCtx, agentID, ExecuteCommandPayload{
				RequestID:  uuid.New().String(),
				Command:    actualCmd,
				TargetType: s.getTargetType(req.ResourceType),
				TargetID:   req.ResourceID,
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

			// Update progress
			s.mu.Lock()
			if prog, ok := s.progress[resourceID]; ok {
				prog.CompletedSteps++
			}
			s.mu.Unlock()
		}(cmd)
	}

	wg.Wait()
	result.CompletedAt = time.Now()

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
	case ResourceTypeHost:
		return "host"
	default:
		return "host"
	}
}

// findAgentForHost finds the agent ID for a given host.
func (s *DeepScanner) findAgentForHost(hostID, hostname string) string {
	agents := s.executor.GetConnectedAgents()

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

// GetProgress returns the current progress of a scan.
func (s *DeepScanner) GetProgress(resourceID string) *DiscoveryProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if prog, ok := s.progress[resourceID]; ok {
		return prog
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
