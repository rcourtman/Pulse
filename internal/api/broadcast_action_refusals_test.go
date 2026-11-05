package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/actionlifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestBroadcastActionRefusalsFollowTheCommandAgent(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	agents := &fakeDockerActionAgentCommander{connected: map[string]bool{"agent-1": false}}
	h.SetActionExecutor(newRoutedActionExecutor(h, newDockerContainerActionExecutor(h, agents)))

	row := func() []unified.Resource {
		return []unified.Resource{dockerContainerActionResource("app-container:api", "docker", "running", now)}
	}

	disconnected := row()
	h.AttachBroadcastActionRefusals("default", disconnected)
	refusal, refused := dockerActionReadinessByName(disconnected[0].ActionReadiness, "restart")
	if !refused || refusal.ReasonCode != "command_agent_disconnected" || refusal.Reason != "Docker / Podman command agent is not connected." {
		t.Fatalf("readiness = %#v, want the disconnected refusal", disconnected[0].ActionReadiness)
	}
	if got := dockerActionCapabilityNames(disconnected[0].Capabilities); len(got) != 1 || got[0] != "restart" {
		t.Fatalf("capabilities = %v, want restart kept on the broadcast row", got)
	}

	agents.connected["agent-1"] = true
	connected := row()
	h.AttachBroadcastActionRefusals("default", connected)
	if connected[0].ActionReadiness != nil {
		t.Fatalf("readiness = %#v, want none once the agent connects", connected[0].ActionReadiness)
	}

	agents.connected["agent-1"] = false
	gone := row()
	h.AttachBroadcastActionRefusals("default", gone)
	if _, refused := dockerActionReadinessByName(gone[0].ActionReadiness, "restart"); !refused {
		t.Fatalf("readiness = %#v, want the refusal back once the agent leaves", gone[0].ActionReadiness)
	}
}

func TestBroadcastActionRefusalsWithoutAnExecutorLeaveRowsAlone(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	rows := []unified.Resource{dockerContainerActionResource("app-container:api", "docker", "running", time.Now().UTC())}
	h.AttachBroadcastActionRefusals("default", rows)
	if rows[0].ActionReadiness != nil {
		t.Fatalf("readiness = %#v, want none before an executor is wired", rows[0].ActionReadiness)
	}
}

// countingActionAgents records every lookup the executor asks the agent server.
type countingActionAgents struct {
	*scopedFakeDockerActionAgentCommander
	calls map[string]int
}

func newCountingActionAgents(connected bool) *countingActionAgents {
	return &countingActionAgents{
		scopedFakeDockerActionAgentCommander: &scopedFakeDockerActionAgentCommander{
			fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{connected: map[string]bool{"agent-1": connected}},
		},
		calls: map[string]int{},
	}
}

func (c *countingActionAgents) IsAgentConnectedForOrganization(organizationID, agentID string) bool {
	c.calls["connected"]++
	return c.scopedFakeDockerActionAgentCommander.IsAgentConnectedForOrganization(organizationID, agentID)
}

func (c *countingActionAgents) GetAgentForHostForOrganization(organizationID, hostname string) (string, bool) {
	c.calls["host"]++
	return c.scopedFakeDockerActionAgentCommander.GetAgentForHostForOrganization(organizationID, hostname)
}

func (c *countingActionAgents) AgentOperationReceiptVersionForOrganization(string, string) int {
	c.calls["receipt"]++
	return c.AgentOperationReceiptVersion("")
}

func TestAvailabilityPassAnswersEachAgentQuestionOnce(t *testing.T) {
	now := time.Now().UTC()
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	agents := newCountingActionAgents(true)
	h.SetActionExecutor(newRoutedActionExecutor(h, newDockerContainerActionExecutor(h, agents)))

	rows := make([]unified.Resource, 0, 50)
	for i := range 50 {
		rows = append(rows, dockerContainerActionResource(fmt.Sprintf("app-container:%d", i), "docker", "running", now))
	}

	h.AttachBroadcastActionRefusals("default", rows)
	if agents.calls["connected"] != 1 || agents.calls["receipt"] != 1 {
		t.Fatalf("lookups after one pass over 50 containers = %v, want one of each", agents.calls)
	}

	// A later pass reads live again: the memo must not outlive its pass.
	h.AttachBroadcastActionRefusals("default", rows)
	if agents.calls["connected"] != 2 || agents.calls["receipt"] != 2 {
		t.Fatalf("lookups after a second pass = %v, want a fresh read", agents.calls)
	}

	// Planning and dispatch check outside a pass and must never read a memo.
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	before := agents.calls["connected"]
	for range 2 {
		executor.CheckActionAvailable(context.Background(), unified.ActionRequest{ResourceID: rows[0].ID, CapabilityName: "restart"}, rows[0])
	}
	if got := agents.calls["connected"] - before; got != 2 {
		t.Fatalf("lookups outside a pass = %d, want 2", got)
	}
}

func TestMemoizedAgentLookupKeepsQuestionsApart(t *testing.T) {
	ctx := withAgentLookupMemo(context.Background())
	agents := &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{
			connected:   map[string]bool{"agent-a": true, "agent-b": false},
			agentByHost: map[string]string{"host-a": "agent-a"},
		},
		tokenAgents: map[string]string{"token-a": "agent-a"},
	}
	if !isAgentCommandConnected(ctx, agents, "agent-a") || isAgentCommandConnected(ctx, agents, "agent-b") {
		t.Fatal("connected lookups for two agents shared one answer")
	}
	if id, ok := commandAgentForIdentity(ctx, agents, "agent-a", "host-a"); !ok || id != "agent-a" {
		t.Fatalf("identity lookup = %q, %v", id, ok)
	}
	if _, ok := commandAgentForIdentity(ctx, agents, "agent-a", "host-b"); ok {
		t.Fatal("identity lookup for another host shared the first answer")
	}
	if id, ok := commandAgentForToken(ctx, agents, "token-a"); !ok || id != "agent-a" {
		t.Fatalf("token lookup = %q, %v", id, ok)
	}
	// The same string asked as a different question must not hit the first answer.
	if id, ok := commandAgentForToken(ctx, agents, "agent-a"); ok || id != "" {
		t.Fatalf("token lookup reused the identity answer: %q, %v", id, ok)
	}
}

// configureMonitorDependencies covers the startup monitor, reload replacements
// and tenant monitors, so this is the one place a monitor gains the projection.
func TestConfigureMonitorDependenciesAttachesRefusalsToTheBroadcast(t *testing.T) {
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	agents := &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{connected: map[string]bool{}},
		tokenAgents:                    map[string]string{},
	}
	h.SetActionExecutor(newRoutedActionExecutor(h, newDockerContainerActionExecutor(h, agents)))
	router := &Router{
		resourceHandlers:        h,
		monitorResourceAdapter:  unified.NewMonitorAdapter(unified.NewRegistry(nil)),
		monitorResourceAdapters: make(map[string]*unified.MonitorAdapter),
	}
	monitor := newWiringTestMonitor(t)
	router.configureMonitorDependencies(monitor)

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "agent-1", Version: "6.8.0"},
		Host:  agentsdocker.HostInfo{Hostname: "docker-host-1", MachineID: "machine-1"},
		Containers: []agentsdocker.Container{{
			ID:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Name:  "web",
			Image: "nginx:latest",
			State: "running",
		}},
		Timestamp: time.Now().UTC(),
	}
	if _, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-1"}); err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}

	container := func(state models.StateFrontend) (models.ResourceFrontend, bool) {
		for _, row := range state.Resources {
			if row.Type == "app-container" {
				return row, true
			}
		}
		return models.ResourceFrontend{}, false
	}

	state := monitor.BuildFrontendState()
	row, ok := container(state)
	if !ok {
		t.Fatalf("no container row in the broadcast: %d rows", len(state.Resources))
	}
	if row.ActionReadinessRef == "" {
		t.Fatalf("container row has no refusal ref while its agent is disconnected: %+v", row)
	}
	entry := string(state.ActionReadinessCatalog[row.ActionReadinessRef])
	if !strings.Contains(entry, "command_agent_disconnected") {
		t.Fatalf("catalog entry = %s, want the disconnected refusal", entry)
	}
	var refusals []unified.ResourceActionReadiness
	if err := json.Unmarshal([]byte(entry), &refusals); err != nil {
		t.Fatalf("decode catalog entry: %v", err)
	}
	for _, refusal := range refusals {
		if refusal.Detail != "" {
			t.Fatalf("catalog entry carries host detail: %s", entry)
		}
	}

	agents.tokenAgents["token-1"] = "agent-1"
	agents.connected["agent-1"] = true
	state = monitor.BuildFrontendState()
	row, ok = container(state)
	if !ok {
		t.Fatal("container row left the broadcast")
	}
	if row.ActionReadinessRef != "" || state.ActionReadinessCatalog != nil {
		t.Fatalf("refusal survived the agent connecting: ref %q, catalog %v", row.ActionReadinessRef, state.ActionReadinessCatalog)
	}
}

// benchmarkActionAgents stands in for agentexec.Server: its host and token
// lookups scan every session, and each session check is another pass over the
// token inventory, which is what makes an unbatched tick expensive.
type benchmarkActionAgents struct {
	*fakeDockerActionAgentCommander
	sessions map[string]benchmarkActionSession
}

type benchmarkActionSession struct {
	agentID, hostname, tokenID string
}

func (b *benchmarkActionAgents) validate(tokenID string) bool {
	matched := false
	for _, session := range b.sessions {
		matched = matched || session.tokenID == tokenID
	}
	return matched
}

func (b *benchmarkActionAgents) IsAgentConnectedForOrganization(_, agentID string) bool {
	session, ok := b.sessions[agentID]
	return ok && b.validate(session.tokenID)
}

func (b *benchmarkActionAgents) GetAgentForHostForOrganization(_, hostname string) (string, bool) {
	for _, session := range b.sessions {
		if unified.HostnamesEquivalent(session.hostname, hostname) {
			return session.agentID, b.validate(session.tokenID)
		}
	}
	return "", false
}

func (b *benchmarkActionAgents) GetAgentForTokenForOrganization(_, tokenID string) (string, bool) {
	for _, session := range b.sessions {
		if strings.TrimSpace(session.tokenID) == tokenID {
			return session.agentID, b.validate(session.tokenID)
		}
	}
	return "", false
}

func (b *benchmarkActionAgents) AgentOperationReceiptVersionForOrganization(_, agentID string) int {
	if b.IsAgentConnectedForOrganization("", agentID) {
		return 1
	}
	return 0
}

func benchmarkBroadcastEstate(hosts, containersPerHost, connectedHosts int) (*benchmarkActionAgents, []unified.Resource) {
	now := time.Now().UTC()
	agents := &benchmarkActionAgents{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{},
		sessions:                       make(map[string]benchmarkActionSession, connectedHosts),
	}
	rows := make([]unified.Resource, 0, hosts*containersPerHost)
	for host := range hosts {
		agentID, hostname, tokenID := fmt.Sprintf("agent-%d", host), fmt.Sprintf("docker-%d.lan", host), fmt.Sprintf("token-%d", host)
		if host < connectedHosts {
			agents.sessions[agentID] = benchmarkActionSession{agentID: agentID, hostname: hostname, tokenID: tokenID}
		}
		for container := range containersPerHost {
			row := dockerContainerActionResource(fmt.Sprintf("app-container:%d:%d", host, container), "docker", "running", now)
			row.Docker.AgentID, row.Docker.Hostname, row.Docker.TokenID = agentID, hostname, tokenID
			base := row.Capabilities[0]
			row.Capabilities = []unified.ResourceCapability{base, base, base}
			row.Capabilities[0].Name = "start"
			row.Capabilities[1].Name = "stop"
			rows = append(rows, row)
		}
	}
	return agents, rows
}

// BenchmarkBroadcastActionRefusals measures one broadcast's refusal pass over
// a 5,000-container estate. The unbatched case is the same pass without the
// memo, which is why the memo exists.
func BenchmarkBroadcastActionRefusals(b *testing.B) {
	for _, connected := range []int{100, 0} {
		agents, rows := benchmarkBroadcastEstate(100, 50, connected)
		cfg := &config.Config{DataPath: b.TempDir()}
		h := NewResourceHandlers(cfg)
		executor := newDockerContainerActionExecutor(h, agents)
		routed, _ := newRoutedActionExecutor(h, executor).(actionlifecycle.AvailabilityChecker)
		unbatched, _ := executor.(actionlifecycle.AvailabilityChecker)
		ctx := context.Background()
		name := func(kind string) string { return fmt.Sprintf("%s/connected=%d", kind, connected) }

		b.Run(name("batched"), func(b *testing.B) {
			work := make([]unified.Resource, len(rows))
			b.ReportAllocs()
			for b.Loop() {
				copy(work, rows)
				actionlifecycle.AttachActionRefusals(ctx, routed, work)
			}
		})
		b.Run(name("unbatched"), func(b *testing.B) {
			work := make([]unified.Resource, len(rows))
			b.ReportAllocs()
			for b.Loop() {
				copy(work, rows)
				actionlifecycle.AttachActionRefusals(ctx, unbatched, work)
			}
		})
	}
}
