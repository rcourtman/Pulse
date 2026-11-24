package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestDockerReportHostIndexKeepsTokenAndAgentScopes(t *testing.T) {
	rows := []unified.Resource{
		{Docker: &unified.DockerData{TokenID: " shared ", AgentID: " agent-a ", HostSourceID: " host-a "}},
		{Docker: &unified.DockerData{TokenID: "shared", AgentID: "agent-a", HostSourceID: "host-a"}}, // Another row on the same host.
		{Docker: &unified.DockerData{TokenID: "shared", AgentID: "agent-a", HostSourceID: " "}},
		{Docker: &unified.DockerData{TokenID: "shared", AgentID: "agent-b", HostSourceID: "host-b"}},
		{Docker: &unified.DockerData{TokenID: " shared ", AgentID: "agent-b", HostSourceID: "host-c"}},
		{Docker: &unified.DockerData{TokenID: "different", AgentID: "agent-a", HostSourceID: "host-d"}},
		{Docker: &unified.DockerData{TokenID: "shared", HostSourceID: "host-e"}},
		{Docker: &unified.DockerData{TokenID: "no-agent", HostSourceID: "host-a"}},
		{Docker: &unified.DockerData{TokenID: "no-agent", HostSourceID: "host-b"}},
		{},
	}
	batch := indexDockerReportHosts(rows, "")
	for _, tt := range []struct {
		token, agent        string
		singleSession, want bool
	}{
		{"shared", "agent-a", false, false},
		{"shared", "agent-a", true, true},
		{"shared", "agent-b", false, true},
		{"different", "agent-a", true, false},
		{"no-agent", "", false, false},
		{"no-agent", "", true, true},
		{"absent", "agent-a", true, false},
	} {
		if got := batch.ambiguous(tt.token, tt.agent, tt.singleSession); got != tt.want {
			t.Errorf("batch(%q, %q, %t) = %t, want %t", tt.token, tt.agent, tt.singleSession, got, tt.want)
		}
		direct := indexDockerReportHosts(rows, tt.token)
		if got := direct.ambiguous(tt.token, tt.agent, tt.singleSession); got != tt.want {
			t.Errorf("direct(%q, %q, %t) = %t, want %t", tt.token, tt.agent, tt.singleSession, got, tt.want)
		}
	}
}

type countingDockerReadinessSeeds struct {
	seeds map[string]resourceUnifiedSeedProvider
	reads map[string]int
}

func (p *countingDockerReadinessSeeds) ReadSnapshot() models.StateSnapshot {
	return p.seeds["default"].snapshot
}

func (p *countingDockerReadinessSeeds) UnifiedResourceSnapshot() ([]unified.Resource, time.Time) {
	return p.UnifiedResourceSnapshotForTenant("default")
}

func (p *countingDockerReadinessSeeds) UnifiedReadStateForTenant(orgID string) unified.ReadState {
	return SnapshotReadState(p.seeds[orgID].snapshot)
}

func (p *countingDockerReadinessSeeds) UnifiedResourceSnapshotForTenant(orgID string) ([]unified.Resource, time.Time) {
	p.reads[orgID]++
	return p.seeds[orgID].UnifiedResourceSnapshot()
}

func TestDockerReadinessUsesOneTenantInventoryPerPassAndFreshDispatch(t *testing.T) {
	now := time.Now().UTC()
	rows := []unified.Resource{
		dockerContainerActionResource("app-container:web", "docker", "running", now),
		dockerContainerActionResource("app-container:worker", "docker", "running", now),
	}
	for i := range rows {
		rows[i].Docker.TokenID = "shared"
		rows[i].Docker.HostSourceID = "primary-host"
		rows[i].Docker.Hostname = "primary"
	}
	otherHost := unified.Resource{
		ID: "docker-host:other", Name: "Other host", Type: unified.ResourceTypeAgent,
		LastSeen: now, Sources: []unified.DataSource{unified.SourceDocker},
		Docker: &unified.DockerData{TokenID: "shared", AgentID: "agent-2", HostSourceID: "other-host"},
	}
	twoHosts := append(append([]unified.Resource(nil), rows...), otherHost)
	seeds := &countingDockerReadinessSeeds{
		seeds: map[string]resourceUnifiedSeedProvider{
			"default": {snapshot: models.StateSnapshot{LastUpdate: now}, resources: rows},
			"other":   {snapshot: models.StateSnapshot{LastUpdate: now}, resources: twoHosts},
		},
		reads: make(map[string]int),
	}
	h := newActionTestResourceHandlers(t, &config.Config{DataPath: t.TempDir()})
	t.Cleanup(func() { _ = h.CloseStores() })
	h.SetStateProvider(seeds)
	h.SetTenantStateProvider(seeds)
	agents := &scopedFakeDockerActionAgentCommander{
		fakeDockerActionAgentCommander: &fakeDockerActionAgentCommander{connected: map[string]bool{"agent-1": true}},
		tokenAgents:                    map[string]string{"shared": "agent-1"},
	}
	executor := newDockerContainerActionExecutor(h, agents).(dockerContainerActionExecutor)
	h.SetActionExecutor(newRoutedActionExecutor(h, executor))

	checkPass := func(orgID string, wantRefusal bool) {
		t.Helper()
		work := append([]unified.Resource(nil), rows...)
		readsBefore := seeds.reads[orgID]
		h.AttachBroadcastActionRefusals(orgID, work)
		for _, row := range work {
			readiness, refused := dockerActionReadinessByName(row.ActionReadiness, "restart")
			if refused != wantRefusal || (refused && (readiness.Available || readiness.ReasonCode != "command_agent_disconnected")) {
				t.Fatalf("%s: readiness for %s = %#v, want refusal %t", orgID, row.ID, row.ActionReadiness, wantRefusal)
			}
			if len(row.Capabilities) != 1 || row.Capabilities[0].Name != "restart" {
				t.Fatalf("broadcast changed advertised capability: %#v", row.Capabilities)
			}
		}
		if got := seeds.reads[orgID] - readsBefore; got != 1 {
			t.Fatalf("%s: registry source read %d times in one pass, want 1", orgID, got)
		}
	}
	checkPass("default", false)
	checkPass("other", true) // The second host is not in the projected rows.
	checkPass("default", false)

	record := dockerContainerActionRecord("fixture-action", rows[0].ID, "restart")
	dockerActionDispatchContext(t, executor, record) // Binding works before the inventory changes.
	seeds.seeds["default"] = resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now.Add(time.Second)}, resources: twoHosts,
	}
	// Do not perform a new display pass before dispatch: an earlier ready row
	// must not let a pending action reuse the old registry or agent answer.
	attempt, err := unified.NewActionDispatchAttempt(record.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = executor.BindActionDispatch(context.Background(), record, attempt)
	if err == nil || !strings.Contains(err.Error(), "is reporting multiple Docker hosts") {
		t.Fatalf("dispatch binding after inventory changed = %v, want ambiguity refusal", err)
	}
	checkPass("default", true)
	seeds.seeds["default"] = resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: now.Add(2 * time.Second)}, resources: rows,
	}
	checkPass("default", false)
}
