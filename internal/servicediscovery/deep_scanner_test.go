package servicediscovery

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
	payloads []ExecuteCommandPayload // Track full payloads for testing
	agents   []ConnectedAgent
}

func (s *stubExecutor) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	s.mu.Lock()
	s.commands = append(s.commands, cmd.Command)
	s.payloads = append(s.payloads, cmd)
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

	// Verify the payload fields are set correctly for nested Docker:
	// - Command should contain "docker exec" (buildCommand adds this)
	// - TargetType should be "vm" (agent wraps with qm guest exec)
	// - TargetID should be "101" (extracted from "101:web")
	foundCorrectPayload := false
	for _, payload := range exec.payloads {
		hasDockerExec := strings.Contains(payload.Command, "docker exec")
		hasContainerName := strings.Contains(payload.Command, "web")
		correctTargetType := payload.TargetType == "vm"
		correctTargetID := payload.TargetID == "101"

		if hasDockerExec && hasContainerName && correctTargetType && correctTargetID {
			foundCorrectPayload = true
			break
		}
	}
	if !foundCorrectPayload {
		t.Fatalf("expected nested docker payload with docker exec, TargetType=vm, TargetID=101, got payloads: %+v", exec.payloads)
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

func TestDeepScanner_GetTargetTypeAndID(t *testing.T) {
	scanner := NewDeepScanner(&stubExecutor{})

	// Test getTargetType
	tests := []struct {
		resourceType ResourceType
		wantType     string
	}{
		{ResourceTypeLXC, "container"},
		{ResourceTypeVM, "vm"},
		{ResourceTypeDocker, "host"},
		{ResourceTypeDockerLXC, "container"}, // Docker inside LXC runs via pct exec
		{ResourceTypeDockerVM, "vm"},         // Docker inside VM runs via qm guest exec
		{ResourceTypeHost, "host"},
		{ResourceType("unknown"), "host"},
	}
	for _, tt := range tests {
		if got := scanner.getTargetType(tt.resourceType); got != tt.wantType {
			t.Errorf("getTargetType(%s) = %s, want %s", tt.resourceType, got, tt.wantType)
		}
	}

	// Test getTargetID
	idTests := []struct {
		resourceType ResourceType
		resourceID   string
		wantID       string
	}{
		{ResourceTypeLXC, "101", "101"},
		{ResourceTypeVM, "102", "102"},
		{ResourceTypeDocker, "web", "web"},
		{ResourceTypeDockerLXC, "201:nginx", "201"},   // Extract vmid for nested docker
		{ResourceTypeDockerVM, "301:postgres", "301"}, // Extract vmid for nested docker
		{ResourceTypeHost, "myhost", "myhost"},
	}
	for _, tt := range idTests {
		if got := scanner.getTargetID(tt.resourceType, tt.resourceID); got != tt.wantID {
			t.Errorf("getTargetID(%s, %s) = %s, want %s", tt.resourceType, tt.resourceID, got, tt.wantID)
		}
	}
}

func TestDeepScanner_BuildCommandAndProgress(t *testing.T) {
	scanner := NewDeepScanner(&stubExecutor{})

	// LXC: buildCommand returns raw command, agent handles pct exec wrapping
	if cmd := scanner.buildCommand(ResourceTypeLXC, "101", "echo hi"); cmd != "echo hi" {
		t.Fatalf("LXC should return raw command (agent wraps), got: %s", cmd)
	}
	// VM: buildCommand returns raw command, agent handles qm guest exec wrapping
	if cmd := scanner.buildCommand(ResourceTypeVM, "101", "echo hi"); cmd != "echo hi" {
		t.Fatalf("VM should return raw command (agent wraps), got: %s", cmd)
	}
	// Docker: buildCommand wraps with docker exec since agent doesn't handle it
	if cmd := scanner.buildCommand(ResourceTypeDocker, "web", "echo hi"); !strings.Contains(cmd, "docker exec") {
		t.Fatalf("Docker should include docker exec, got: %s", cmd)
	}
	// Host: buildCommand returns raw command
	if cmd := scanner.buildCommand(ResourceTypeHost, "host", "echo hi"); cmd != "echo hi" {
		t.Fatalf("Host should return raw command, got: %s", cmd)
	}

	// DockerLXC: buildCommand adds docker exec, agent adds pct exec
	// So we should only see docker exec in the command (agent adds pct exec at runtime)
	dockerLXC := scanner.buildCommand(ResourceTypeDockerLXC, "201:web", "echo hi")
	if !strings.Contains(dockerLXC, "docker exec") {
		t.Fatalf("DockerLXC should include docker exec, got: %s", dockerLXC)
	}
	if strings.Contains(dockerLXC, "pct exec") {
		t.Fatalf("DockerLXC should NOT include pct exec (agent adds it), got: %s", dockerLXC)
	}
	if cmd := scanner.buildCommand(ResourceTypeDockerLXC, "bad", "echo hi"); cmd != "echo hi" {
		t.Fatalf("DockerLXC with bad ID should fallback, got: %s", cmd)
	}

	// DockerVM: buildCommand adds docker exec, agent adds qm guest exec
	dockerVM := scanner.buildCommand(ResourceTypeDockerVM, "301:web", "echo hi")
	if !strings.Contains(dockerVM, "docker exec") {
		t.Fatalf("DockerVM should include docker exec, got: %s", dockerVM)
	}
	if strings.Contains(dockerVM, "qm guest exec") {
		t.Fatalf("DockerVM should NOT include qm guest exec (agent adds it), got: %s", dockerVM)
	}
	if cmd := scanner.buildCommand(ResourceTypeDockerVM, "bad", "echo hi"); cmd != "echo hi" {
		t.Fatalf("DockerVM with bad ID should fallback, got: %s", cmd)
	}

	// Unknown type: returns raw command
	if cmd := scanner.buildCommand(ResourceType("unknown"), "id", "echo hi"); cmd != "echo hi" {
		t.Fatalf("Unknown type should return raw command, got: %s", cmd)
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
