package aidiscovery

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubExecutor struct {
	mu       sync.Mutex
	commands []string
	agents   []ConnectedAgent
}

func (s *stubExecutor) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	s.mu.Lock()
	s.commands = append(s.commands, cmd.Command)
	s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if strings.Contains(cmd.Command, "docker ps -a") {
		return &CommandResultPayload{
			RequestID: cmd.RequestID,
			Success:   false,
			Error:     "boom",
		}, nil
	}

	return &CommandResultPayload{
		RequestID: cmd.RequestID,
		Success:   true,
		Stdout:    cmd.Command,
		Duration:  5,
	}, nil
}

func (s *stubExecutor) GetConnectedAgents() []ConnectedAgent {
	return s.agents
}

func (s *stubExecutor) IsAgentConnected(agentID string) bool {
	for _, agent := range s.agents {
		if agent.AgentID == agentID {
			return true
		}
	}
	return false
}

type outputExecutor struct{}

func (outputExecutor) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	switch {
	case strings.Contains(cmd.Command, "docker ps -a"):
		return &CommandResultPayload{Success: true, Stdout: "out", Stderr: "err"}, nil
	case strings.Contains(cmd.Command, "docker images"):
		return &CommandResultPayload{Success: true, Stderr: "err-only"}, nil
	default:
		return &CommandResultPayload{Success: true}, nil
	}
}

func (outputExecutor) GetConnectedAgents() []ConnectedAgent {
	return []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}}
}

func (outputExecutor) IsAgentConnected(string) bool { return true }

type errorExecutor struct{}

func (errorExecutor) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	return nil, context.DeadlineExceeded
}

func (errorExecutor) GetConnectedAgents() []ConnectedAgent {
	return []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}}
}

func (errorExecutor) IsAgentConnected(string) bool { return true }

func TestDeepScanner_Scan_NestedDockerCommands(t *testing.T) {
	exec := &stubExecutor{
		agents: []ConnectedAgent{
			{AgentID: "host1", Hostname: "host1", ConnectedAt: time.Now()},
		},
	}
	scanner := NewDeepScanner(exec)

	result, err := scanner.Scan(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDockerVM,
		ResourceID:   "101:web",
		HostID:       "host1",
		Hostname:     "host1",
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(result.CommandOutputs) == 0 {
		t.Fatalf("expected command outputs")
	}
	if _, ok := result.Errors["docker_containers"]; !ok {
		t.Fatalf("expected docker_containers error, got %#v", result.Errors)
	}

	exec.mu.Lock()
	defer exec.mu.Unlock()
	foundWrapped := false
	for _, cmd := range exec.commands {
		if strings.Contains(cmd, "qm guest exec 101") && strings.Contains(cmd, "docker exec web") {
			foundWrapped = true
			break
		}
	}
	if !foundWrapped {
		t.Fatalf("expected nested docker command, got %#v", exec.commands)
	}
}

func TestDeepScanner_FindAgentAndTargetType(t *testing.T) {
	exec := &stubExecutor{
		agents: []ConnectedAgent{
			{AgentID: "a1", Hostname: "node1"},
			{AgentID: "a2", Hostname: "node2"},
		},
	}
	scanner := NewDeepScanner(exec)

	if got := scanner.findAgentForHost("a2", ""); got != "a2" {
		t.Fatalf("expected direct agent match, got %s", got)
	}
	if got := scanner.findAgentForHost("node1", "node1"); got != "a1" {
		t.Fatalf("expected hostname match, got %s", got)
	}

	exec.agents = []ConnectedAgent{{AgentID: "solo", Hostname: "only"}}
	if got := scanner.findAgentForHost("missing", "missing"); got != "solo" {
		t.Fatalf("expected single agent fallback, got %s", got)
	}
	exec.agents = nil
	if got := scanner.findAgentForHost("missing", "missing"); got != "" {
		t.Fatalf("expected no agent, got %s", got)
	}

	if scanner.getTargetType(ResourceTypeLXC) != "container" {
		t.Fatalf("unexpected target type for lxc")
	}
	if scanner.getTargetType(ResourceTypeVM) != "vm" {
		t.Fatalf("unexpected target type for vm")
	}
	if scanner.getTargetType(ResourceTypeDocker) != "host" {
		t.Fatalf("unexpected target type for docker")
	}
	if scanner.getTargetType(ResourceTypeHost) != "host" {
		t.Fatalf("unexpected target type for host")
	}
}

func TestSplitResourceID(t *testing.T) {
	parts := splitResourceID("101:web:extra")
	if len(parts) != 3 || parts[0] != "101" || parts[1] != "web" || parts[2] != "extra" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}

func TestDeepScanner_BuildCommandAndProgress(t *testing.T) {
	scanner := NewDeepScanner(&stubExecutor{})

	if cmd := scanner.buildCommand(ResourceTypeLXC, "101", "echo hi"); !strings.Contains(cmd, "pct exec 101") {
		t.Fatalf("unexpected lxc command: %s", cmd)
	}
	if cmd := scanner.buildCommand(ResourceTypeVM, "101", "echo hi"); cmd != "echo hi" {
		t.Fatalf("unexpected vm command: %s", cmd)
	}
	if cmd := scanner.buildCommand(ResourceTypeDocker, "web", "echo hi"); !strings.Contains(cmd, "docker exec web") {
		t.Fatalf("unexpected docker command: %s", cmd)
	}
	if cmd := scanner.buildCommand(ResourceTypeHost, "host", "echo hi"); cmd != "echo hi" {
		t.Fatalf("unexpected host command: %s", cmd)
	}

	dockerLXC := scanner.buildCommand(ResourceTypeDockerLXC, "201:web", "echo hi")
	if !strings.Contains(dockerLXC, "pct exec 201") || !strings.Contains(dockerLXC, "docker exec web") {
		t.Fatalf("unexpected docker lxc command: %s", dockerLXC)
	}
	if cmd := scanner.buildCommand(ResourceTypeDockerLXC, "bad", "echo hi"); cmd != "echo hi" {
		t.Fatalf("expected fallback lxc command, got %s", cmd)
	}
	dockerVM := scanner.buildCommand(ResourceTypeDockerVM, "301:web", "echo hi")
	if !strings.Contains(dockerVM, "qm guest exec 301") || !strings.Contains(dockerVM, "docker exec web") {
		t.Fatalf("unexpected docker vm command: %s", dockerVM)
	}
	if cmd := scanner.buildCommand(ResourceTypeDockerVM, "bad", "echo hi"); cmd != "echo hi" {
		t.Fatalf("expected fallback command, got %s", cmd)
	}
	if cmd := scanner.buildCommand(ResourceType("unknown"), "id", "echo hi"); cmd != "echo hi" {
		t.Fatalf("expected default command, got %s", cmd)
	}

	scanner.progress["id"] = &DiscoveryProgress{ResourceID: "id"}
	if scanner.GetProgress("id") == nil {
		t.Fatalf("expected progress")
	}
	if !scanner.IsScanning("id") {
		t.Fatalf("expected IsScanning true")
	}
	if scanner.GetProgress("missing") != nil {
		t.Fatalf("expected nil progress")
	}
	if scanner.IsScanning("missing") {
		t.Fatalf("expected IsScanning false")
	}

	noExec := NewDeepScanner(nil)
	if _, err := noExec.ScanHost(context.Background(), "host1", "host1"); err == nil {
		t.Fatalf("expected error without executor")
	}
}

func TestDeepScanner_ScanWrappers(t *testing.T) {
	exec := &stubExecutor{
		agents: []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}},
	}
	scanner := NewDeepScanner(exec)
	scanner.maxParallel = 1

	if _, err := scanner.ScanDocker(context.Background(), "host1", "host1", "web"); err != nil {
		t.Fatalf("ScanDocker error: %v", err)
	}
	if _, err := scanner.ScanLXC(context.Background(), "host1", "host1", "101"); err != nil {
		t.Fatalf("ScanLXC error: %v", err)
	}
	if _, err := scanner.ScanVM(context.Background(), "host1", "host1", "102"); err != nil {
		t.Fatalf("ScanVM error: %v", err)
	}
}

func TestDeepScanner_ScanErrors(t *testing.T) {
	exec := &stubExecutor{
		agents: []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}},
	}
	scanner := NewDeepScanner(exec)
	if _, err := scanner.Scan(context.Background(), DiscoveryRequest{
		ResourceType: ResourceType("unknown"),
		ResourceID:   "id",
		HostID:       "host1",
		Hostname:     "host1",
	}); err == nil {
		t.Fatalf("expected error for unknown resource type")
	}

	exec.agents = nil
	if _, err := scanner.Scan(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDocker,
		ResourceID:   "web",
		HostID:       "host1",
		Hostname:     "host1",
	}); err == nil {
		t.Fatalf("expected error for missing agent")
	}
}

func TestDeepScanner_OutputHandling(t *testing.T) {
	exec := outputExecutor{}
	scanner := NewDeepScanner(exec)
	scanner.maxParallel = 1

	result, err := scanner.Scan(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDockerVM,
		ResourceID:   "101:web",
		HostID:       "host1",
		Hostname:     "host1",
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if out := result.CommandOutputs["docker_containers"]; !strings.Contains(out, "--- stderr ---") {
		t.Fatalf("expected combined stderr output, got %s", out)
	}
	if out := result.CommandOutputs["docker_images"]; out != "err-only" {
		t.Fatalf("expected stderr-only output, got %s", out)
	}
}

func TestDeepScanner_CommandErrorHandling(t *testing.T) {
	scanner := NewDeepScanner(errorExecutor{})
	scanner.maxParallel = 1

	result, err := scanner.Scan(context.Background(), DiscoveryRequest{
		ResourceType: ResourceTypeDockerVM,
		ResourceID:   "101:web",
		HostID:       "host1",
		Hostname:     "host1",
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if _, ok := result.Errors["docker_containers"]; !ok {
		t.Fatalf("expected error for non-optional command")
	}
}

func TestDeepScanner_ScanCanceledContext(t *testing.T) {
	exec := &stubExecutor{
		agents: []ConnectedAgent{{AgentID: "host1", Hostname: "host1"}},
	}
	scanner := NewDeepScanner(exec)
	scanner.maxParallel = 0

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := scanner.Scan(ctx, DiscoveryRequest{
		ResourceType: ResourceTypeDockerVM,
		ResourceID:   "101:web",
		HostID:       "host1",
		Hostname:     "host1",
	}); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
}
