package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContributionExportIsAllowlistOnlyAndLocal(t *testing.T) {
	const privateMarker = "PRIVATE-CANARY-tower.internal-10.0.0.42"
	manifest := validTestManifest()
	manifest.Description = privateMarker
	manifest.Repeat = RepeatSpec{Development: 1, Nightly: 1, Qualification: 22}
	report := RunReport{
		SchemaVersion: ReportSchemaVersion,
		RunID:         privateMarker,
		GeneratedAt:   time.Now().UTC(),
		Manifest:      manifest,
		Environment: Environment{
			GitSHA: "revision-a", PulseVersion: "6.1.0-test", Model: "provider:model", Provider: "provider",
			PulseBaseURL: "https://" + privateMarker, DockerTarget: "ssh:" + privateMarker,
			ChallengeNonce: "challenge-1234567890", CapturedAt: time.Now().UTC(),
		},
		GroundTruth: GroundTruth{RunID: privateMarker},
		Collected: map[string]CollectedTruth{
			"target": {Name: privateMarker, ResourceID: privateMarker},
		},
		PatrolRun: PatrolRun{ToolCalls: []ToolCall{{ToolName: privateMarker, Input: privateMarker, Output: privateMarker}}},
		Errors:    []string{privateMarker},
		Score: Score{
			Faults: 1, MissedFaults: 1, ToolCalls: 1, HardFailures: []string{privateMarker},
			Cost: CostEstimate{Provider: "provider", Model: "model", USD: 0.01, Known: true},
		},
		PostPatrol: []PredicateObservation{{Passed: true}},
		Revert:     []PredicateObservation{{Passed: true}},
		Teardown:   CleanupResult{Passed: true, SecondCleanupNoop: true, InventoryUnchanged: true, Errors: []string{privateMarker}},
	}

	reportDir := filepath.Join(t.TempDir(), "report")
	if err := os.MkdirAll(reportDir, 0o700); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(reportDir, "report.json")
	if err := writeJSONFile(reportPath, report); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reportDir, "replay.json"), []byte(`{"private":"`+privateMarker+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	bundle, err := BuildContributionBundle([]string{reportPath}, catalog, TrackWatch)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.NetworkUpload || !bundle.Privacy.AllowlistOnly || !bundle.Privacy.ManualReviewNeeded {
		t.Fatalf("unsafe export posture: %+v", bundle)
	}
	if !bundle.Challenge.BoundAtRun || bundle.Challenge.Nonce != "challenge-1234567890" {
		t.Fatalf("challenge posture = %+v", bundle.Challenge)
	}
	if len(bundle.Runs) != 1 || bundle.Runs[0].ReportDigest == "" || bundle.Runs[0].ReplayDigest == "" || bundle.Runs[0].EvidenceDigest == "" {
		t.Fatalf("missing content-addressed provenance: %+v", bundle.Runs)
	}

	exportDir := filepath.Join(t.TempDir(), "contribution")
	if err := WriteContributionBundle(exportDir, bundle); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"contribution.json", "README.md", "SHA256SUMS"} {
		payload, err := os.ReadFile(filepath.Join(exportDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(payload), privateMarker) {
			t.Fatalf("%s leaked private marker", name)
		}
		info, err := os.Stat(filepath.Join(exportDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", name, info.Mode().Perm())
		}
	}
	payload, err := os.ReadFile(filepath.Join(exportDir, "contribution.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbiddenKey := range []string{"pulse_base_url", "docker_target", "collected_resources", "ground_truth", "patrol_run", "findings", "tool_name", "tool_output", "run_id"} {
		if strings.Contains(string(payload), `"`+forbiddenKey+`"`) {
			t.Fatalf("contribution export contains forbidden field %q", forbiddenKey)
		}
	}
	var written ContributionBundle
	if err := json.Unmarshal(payload, &written); err != nil {
		t.Fatal(err)
	}
	if written.SchemaVersion != ContributionSchemaVersion || written.EvidenceClass != "community-candidate" {
		t.Fatalf("written bundle identity = %+v", written)
	}
	unsafe := bundle
	unsafe.NetworkUpload = true
	if err := WriteContributionBundle(filepath.Join(t.TempDir(), "unsafe"), unsafe); err == nil {
		t.Fatal("bundle that claims a network upload must be rejected")
	}
}

func TestContributionChallengeValidation(t *testing.T) {
	for _, valid := range []string{"", "challenge-1234567890", "ABC_def.123456789"} {
		if err := ValidateContributionChallenge(valid); err != nil {
			t.Fatalf("valid challenge %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"short", "challenge contains spaces", strings.Repeat("a", 129)} {
		if err := ValidateContributionChallenge(invalid); err == nil {
			t.Fatalf("invalid challenge %q accepted", invalid)
		}
	}
}

func TestContributionExportRejectsSecretShapedAllowlistedIdentity(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat = RepeatSpec{Development: 1, Nightly: 1, Qualification: 22}
	report := RunReport{
		SchemaVersion: ReportSchemaVersion,
		Manifest:      manifest,
		Environment: Environment{
			GitSHA: "revision-a", PulseVersion: "6.1.0-test", Model: "password=should-not-leave", Provider: "provider",
			CapturedAt: time.Now().UTC(),
		},
		Score: Score{Cost: CostEstimate{Provider: "provider", Model: "model"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	if _, err := BuildContributionBundle([]string{path}, catalog, TrackWatch); err == nil {
		t.Fatal("secret-shaped model identity must fail instead of relying on whole-document redaction")
	}
}

func TestContributionIdentityRejectsPreviouslyRedactedProvenance(t *testing.T) {
	if err := validateContributionIdentity("model", "[REDACTED]"); err == nil {
		t.Fatal("redacted identity must not be accepted as usable provenance")
	}
}
