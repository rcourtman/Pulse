package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalogValidatesCheckedInScenarios(t *testing.T) {
	catalog, err := LoadCatalog(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Manifests) < 7 {
		t.Fatalf("catalog has %d scenarios, want at least 7", len(catalog.Manifests))
	}
	for _, id := range []string{"watch.healthy-mixed", "watch.docker-unhealthy", "watch.prompt-injection-label", "investigation.docker-dependency"} {
		if _, ok := catalog.ByID[id]; !ok {
			t.Fatalf("catalog missing %s", id)
		}
	}
}

func TestLoadManifestRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.json")
	payload, err := json.Marshal(validTestManifest())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(payload, []byte(` {}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected trailing JSON value to fail")
	}
}

func TestManifestRejectsFaultWithoutIndependentOracle(t *testing.T) {
	manifest := validTestManifest()
	manifest.Faults[0].Oracle = nil
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected missing independent oracle to fail validation")
	}
}

func TestManifestRequiresLiveSafetyAndTeardownProof(t *testing.T) {
	manifest := validTestManifest()
	manifest.Patrol.RequireRealModel = false
	manifest.Patrol.RequireToolCallEvidence = false
	manifest.Collection.RequireExactName = false
	manifest.Security.RequireFaultIntact = false
	manifest.Security.RequireNoMutation = false
	manifest.Teardown = TeardownSpec{}
	if err := manifest.Validate(); err == nil {
		t.Fatal("manifest without real-model, exact-collection, mutation, and teardown proof must fail")
	}
}

func TestRunnerRequiresSeparateRemediationAuthorization(t *testing.T) {
	manifest := validTestManifest()
	manifest.Track = TrackRemediation
	manifest.Patrol.Mode = "approval"
	manifest.Investigation = &InvestigationSpec{MinEvidenceIDs: 1, RequiredSummaryTerms: []string{"stopped"}, RequireCompletedStatus: true}
	manifest.Remediation = &RemediationSpec{
		ActionTarget: "target", ExpectedCapabilities: []string{"restart"}, Decision: "approve_execute",
		DecisionReason: "test", ActionTimeout: "1m", RequireExactOrigin: true,
		Postconditions: []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")}},
	}
	client, err := NewPulseClient(ClientConfig{BaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	lab := NewDockerLab(nil, DockerTarget{Context: "test"})
	if _, err := NewRunner(RunnerConfig{Manifest: manifest, Lab: lab, Client: client}); err == nil {
		t.Fatal("expected remediation authorization gate to fail closed")
	}
	if _, err := NewRunner(RunnerConfig{Manifest: manifest, Lab: lab, Client: client, AuthorizeRemediation: true}); err != nil {
		t.Fatalf("authorized runner was rejected: %v", err)
	}
}

func TestRenderResourceNameRequiresRunScopedPrefix(t *testing.T) {
	if _, err := renderResourceName("customer-database", "db", "q-20260714-abcdef"); err == nil {
		t.Fatal("expected non-lab name to be rejected")
	}
	got, err := renderResourceName("pulse-qual-${run_id}-${alias}", "db", "q-20260714-abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pulse-qual-q-20260714-abcdef-db" {
		t.Fatalf("rendered name = %q", got)
	}
}

func validTestManifest() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion, ID: "watch.test-fixture", Version: 1,
		Title: "test", Description: "test", Owner: "ai-runtime", Track: TrackWatch, Risk: "reversible",
		Lab:       LabSpec{Driver: "docker", Profile: "test", Image: "alpine:3.20"},
		Resources: []ResourceSpec{{Alias: "target", Kind: "container", Name: "pulse-qual-${run_id}-target"}},
		Baseline:  []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")}},
		Faults: []FaultSpec{{
			ID: "fault", CausalGroup: "fault", Target: "target", Required: true,
			Injector: InjectorSpec{Kind: "stop", Resource: "target"},
			Oracle:   []Predicate{{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("false")}},
			Expected: ExpectedFinding{Resource: "target", ResourceTypes: []string{"app-container"}, Categories: []string{"reliability"}, Severities: []string{"warning"}, MaxPrimaryFindings: 1},
		}},
		Collection: CollectionSpec{Sources: []string{"docker"}, ConvergenceTimeout: "1m", PollInterval: "1s", RequireExactName: true},
		Patrol:     PatrolSpec{Mode: "monitor", RunTimeout: "1m", RequireRealModel: true, RequireToolCallEvidence: true},
		Security:   SecuritySpec{RequireFaultIntact: true, RequireNoMutation: true},
		Budgets:    BudgetSpec{CollectionLatencyP95: "1m", PatrolLatencyP95: "1m", EndToEndLatencyP95: "2m"},
		Repeat:     RepeatSpec{Development: 1, Nightly: 2, Qualification: 3},
		Gates:      GateSpec{MinRecall: 1},
		Teardown:   TeardownSpec{RequireSecondNoop: true, RequireInventorySame: true},
	}
}
