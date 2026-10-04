package servicediscovery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

type deferringGuestExecutor struct {
	*stubExecutor
	calls, deferAt int
}

func (e *deferringGuestExecutor) ExecuteCommand(ctx context.Context, agentID string, p ExecuteCommandPayload) (*CommandResultPayload, error) {
	e.calls++
	if e.calls == e.deferAt {
		return &CommandResultPayload{RequestID: p.RequestID, Success: false, ExitCode: 0, Stdout: "in-flight output is not fresh evidence", Error: agentexec.GuestExecDeferred(agentexec.GuestExecVMLocked).Error()}, nil
	}
	return &CommandResultPayload{RequestID: p.RequestID, Success: true, Stdout: "earlier observation"}, nil
}

func testDiscoveryGuestSafetyStopsAndPreservesSavedEvidence(t *testing.T) {
	for _, rt := range []ResourceType{ResourceTypeVM, ResourceTypeDockerVM} {
		for _, deferAt := range []int{1, 2} {
			t.Run(string(rt)+"/"+time.Duration(deferAt).String(), func(t *testing.T) {
				id := "105"
				if rt == ResourceTypeDockerVM {
					id = "105:app"
				}
				e := &deferringGuestExecutor{stubExecutor: &stubExecutor{agents: []ConnectedAgent{{AgentID: "node", Hostname: "node"}}}, deferAt: deferAt}
				scanner := NewDeepScanner(e)
				var terminal *DiscoveryProgress
				scanner.SetProgressCallback(func(p *DiscoveryProgress) { copy := *p; terminal = &copy })
				req := DiscoveryRequest{ResourceType: rt, ResourceID: id, TargetID: "node", Force: true}
				got, err := scanner.Scan(context.Background(), req)
				if err == nil || !agentexec.IsGuestExecDeferred(err.Error()) || got == nil || e.calls != deferAt || len(got.CommandOutputs) != deferAt-1 {
					t.Fatalf("scan continued: %+v %v calls=%d", got, err, e.calls)
				}
				if terminal == nil || terminal.Error == "" || terminal.PercentComplete == 100 || scanner.IsScanning(MakeResourceID(rt, "node", id)) {
					t.Fatalf("false completion/progress: %+v", terminal)
				}
				for _, out := range got.CommandOutputs {
					if strings.Contains(out, "in-flight") {
						t.Fatal("unverified output became fresh evidence")
					}
				}
				store, err := NewStore(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				store.crypto = nil
				saved := &ResourceDiscovery{ID: MakeResourceID(rt, "node", id), ResourceType: rt, ResourceID: id, TargetID: "node", ServiceName: "saved observation", UserNotes: "keep", RawCommandOutput: map[string]string{"old": "retain"}, UpdatedAt: time.Now().Add(-time.Hour), DiscoveredAt: time.Now().Add(-time.Hour)}
				if err := store.Save(saved); err != nil {
					t.Fatal(err)
				}
				before, _ := store.Get(saved.ID)
				beforeJSON, _ := json.Marshal(before)
				cfg := DefaultConfig()
				cfg.CommandScanning = true
				service := NewService(store, scanner, cfg)
				analyzer := &stubAnalyzer{response: `{"service_type":"nginx","confidence":0.9}`}
				service.SetAIAnalyzer(analyzer)
				e.calls = 0
				if fresh, err := service.DiscoverResource(context.Background(), req); fresh != nil || err == nil || !agentexec.IsGuestExecDeferred(err.Error()) {
					t.Fatalf("manual deferral became success: %+v %v", fresh, err)
				}
				after, _ := store.Get(saved.ID)
				afterJSON, _ := json.Marshal(after)
				if string(beforeJSON) != string(afterJSON) || analyzer.calls != 0 || e.calls != deferAt {
					t.Fatal("manual pause replaced saved evidence, analysed partial output, or retried")
				}
				e.calls = 0
				if got := service.enhanceWithDeepScan(context.Background(), saved, DockerHost{}); got != saved || analyzer.calls != 0 || e.calls != deferAt {
					t.Fatal("automatic deferral fabricated fresh evidence")
				}
			})
		}
	}
}

func TestVMDiscoveryRequiresUniqueOwningNode(t *testing.T) {
	for _, tc := range []struct {
		name, target, hint string
		agents             []ConnectedAgent
	}{
		{"sole unrelated agent", "node", "other", []ConnectedAgent{{AgentID: "other", Hostname: "other"}}},
		{"ambiguous short hostname", "node", "", []ConnectedAgent{{AgentID: "a", Hostname: "node.one"}, {AgentID: "b", Hostname: "node.two"}}},
		{"empty target", "", "node", []ConnectedAgent{{AgentID: "a", Hostname: "node"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := &deferringGuestExecutor{stubExecutor: &stubExecutor{agents: tc.agents}, deferAt: 1}
			scanner := NewDeepScanner(e)
			if got, err := scanner.Scan(context.Background(), DiscoveryRequest{ResourceType: ResourceTypeVM, ResourceID: "105", TargetID: tc.target, Hostname: tc.hint}); got != nil || err == nil || !agentexec.IsGuestExecDeferred(err.Error()) || e.calls != 0 {
				t.Fatalf("unverified node dispatched: %+v %v calls=%d", got, err, e.calls)
			}
		})
	}
}
