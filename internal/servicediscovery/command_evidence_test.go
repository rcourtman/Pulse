package servicediscovery

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type commandEvidenceExecutor struct {
	*stubExecutor
	mode string
}

func (e *commandEvidenceExecutor) ExecuteCommand(_ context.Context, _ string, p ExecuteCommandPayload) (*CommandResultPayload, error) {
	if e.mode == "nil" {
		return nil, nil
	}
	if e.mode == "healthy" || (e.mode == "partial" && p.Command == "hostname") {
		return &CommandResultPayload{RequestID: p.RequestID, Success: true, ExitCode: 0, Stdout: "verified observation", Stderr: "warning is not evidence"}, nil
	}
	return &CommandResultPayload{RequestID: p.RequestID, Success: e.mode == "contradictory", ExitCode: 7, Stdout: "misleading service identity", Stderr: "private failure text"}, nil
}

func testDiscoveryRejectsFailedCommandEvidence(t *testing.T) {
	for _, rt := range []ResourceType{ResourceTypeVM, ResourceTypeDockerVM, ResourceTypeSystemContainer, ResourceTypeDocker, ResourceTypeAgent} {
		for _, mode := range []string{"failure", "nil", "contradictory"} {
			t.Run(string(rt)+"/"+mode, func(t *testing.T) {
				id := "105"
				if rt == ResourceTypeDockerVM {
					id = "105:app"
				}
				e := &commandEvidenceExecutor{stubExecutor: &stubExecutor{agents: []ConnectedAgent{{AgentID: "node", Hostname: "node"}}}, mode: mode}
				scanner := NewDeepScanner(e)
				var terminal *DiscoveryProgress
				scanner.SetProgressCallback(func(p *DiscoveryProgress) { copy := *p; terminal = &copy })
				result, err := scanner.Scan(context.Background(), DiscoveryRequest{ResourceType: rt, ResourceID: id, TargetID: "node"})
				if result == nil || len(result.CommandOutputs) != 0 || err == nil || terminal == nil || terminal.Error == "" || terminal.PercentComplete == 100 {
					t.Fatalf("failed commands became fresh evidence: result=%+v err=%v progress=%+v", result, err, terminal)
				}
				if strings.Contains(err.Error(), "private") || strings.Contains(terminal.Error, "private") {
					t.Fatal("command output entered the scan-level error")
				}
			})
		}
	}
	for _, mode := range []string{"healthy", "partial"} {
		t.Run(mode, func(t *testing.T) {
			e := &commandEvidenceExecutor{stubExecutor: &stubExecutor{agents: []ConnectedAgent{{AgentID: "node", Hostname: "node"}}}, mode: mode}
			scanner := NewDeepScanner(e)
			result, err := scanner.ScanVM(context.Background(), "node", "node", "105")
			want := len(GetCommandsForResource(ResourceTypeVM))
			if mode == "partial" {
				want = 1 // Optional failures do not discard independent healthy evidence.
			}
			if err != nil || result == nil || len(result.CommandOutputs) != want {
				t.Fatalf("independent successful observations lost: %+v %v", result, err)
			}
			for _, output := range result.CommandOutputs {
				if output != "verified observation" {
					t.Fatalf("warning/failure became evidence: %q", output)
				}
			}
		})
	}
}

func testDiscoveryAllFailedCommandsKeepSavedEvidence(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.crypto = nil
	saved := &ResourceDiscovery{ID: MakeResourceID(ResourceTypeVM, "node", "105"), ResourceType: ResourceTypeVM, ResourceID: "105", TargetID: "node", ServiceName: "saved observation", UserNotes: "keep notes", RawCommandOutput: map[string]string{"old": "retain"}, UpdatedAt: time.Now().Add(-time.Hour), DiscoveredAt: time.Now().Add(-time.Hour)}
	if err := store.Save(saved); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Get(saved.ID)
	beforeJSON, _ := json.Marshal(before)
	e := &commandEvidenceExecutor{stubExecutor: &stubExecutor{agents: []ConnectedAgent{{AgentID: "node", Hostname: "node"}}}, mode: "failure"}
	scanner := NewDeepScanner(e)
	cfg := DefaultConfig()
	cfg.CommandScanning = true
	service := NewService(store, scanner, cfg)
	analyzer := &stubAnalyzer{response: `{"service_type":"nginx","confidence":0.9}`}
	service.SetAIAnalyzer(analyzer)
	req := DiscoveryRequest{ResourceType: ResourceTypeVM, ResourceID: "105", TargetID: "node", Force: true}
	if fresh, err := service.DiscoverResource(context.Background(), req); fresh != nil || err == nil {
		t.Fatalf("failed manual scan became replacement: %+v %v", fresh, err)
	}
	after, _ := store.Get(saved.ID)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) || analyzer.calls != 0 {
		t.Fatal("failed scan changed saved evidence/notes or sent failures to the analyzer")
	}
	if got := service.enhanceWithDeepScan(context.Background(), saved, DockerHost{}); got != saved || analyzer.calls != 0 {
		t.Fatal("failed automatic scan fabricated new evidence")
	}
	// Recovery is an explicit new scan, not a retry of the failed command.
	e.mode = "healthy"
	if fresh, err := service.DiscoverResource(context.Background(), req); fresh == nil || err != nil || analyzer.calls != 1 || fresh.UserNotes != saved.UserNotes {
		t.Fatalf("fresh explicit scan failed to resume: %+v %v analyzer=%d", fresh, err, analyzer.calls)
	}
}
