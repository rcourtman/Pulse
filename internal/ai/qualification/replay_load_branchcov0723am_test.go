package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// qualreplay0723write is a tiny helper that writes payload to a fresh file
// inside the test's temp dir and returns its path. Every LoadReplayBundle
// fixture is produced from scratch in t.TempDir(); no checked-in fixtures are
// read.
func qualreplay0723write(t *testing.T, name string, payload []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", path, err)
	}
	return path
}

// TestBranchcov0723AmLoadReplayBundle covers every branch of LoadReplayBundle
// in replay.go: the os.ReadFile error path (both missing path and a directory
// where a file is expected), the json.Decode failure path (malformed JSON and
// an empty file yielding io.EOF), the trailing-data block (both the
// multiple-values arm where the second decode succeeds and the trailing-garbage
// arm where it fails), DisallowUnknownFields rejection, the NewReplaySession
// validation passthrough (unsupported schema, incomplete capture, malformed
// exchange), and the success path asserting every ReplayBundle field
// round-trips through disk + JSON.
func TestBranchcov0723AmLoadReplayBundle(t *testing.T) {
	validBundle := ReplayBundle{
		SchemaVersion:  ReplaySchemaVersion,
		CapturedAt:     time.Unix(1_700_000_000, 0).UTC(),
		RunID:          "run-abc",
		ManifestID:     "watch.test-fixture",
		ManifestDigest: "sha256:deadbeef",
		Model:          "provider:test-model",
		GroundTruth: GroundTruth{
			SchemaVersion:  "patrol.qualification.ground_truth/v1",
			ManifestID:     "watch.test-fixture",
			ManifestDigest: "sha256:deadbeef",
			RunID:          "run-abc",
			CreatedAt:      time.Unix(1_699_999_999, 0).UTC(),
			Baseline: []PredicateObservation{{
				Predicate: Predicate{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("true")},
				Observed:  true, Passed: true, CheckedAt: time.Unix(1_699_999_998, 0).UTC(),
			}},
			Faults: []FaultTruth{{
				ID: "fault", CausalGroup: "fault", TargetAlias: "target", TargetName: "pulse-target",
				ResourceType: "app-container", Active: true, ConfirmedAt: time.Unix(1_699_999_997, 0).UTC(),
				Observations: []PredicateObservation{{
					Predicate: Predicate{Probe: "docker.running", Target: "target", Operator: "eq", Value: json.RawMessage("false")},
					Observed:  false, Passed: true, CheckedAt: time.Unix(1_699_999_996, 0).UTC(),
				}},
			}},
			Negative: []NegativeTruth{{Alias: "noise", Name: "pulse-noise", Reason: "scraper sidecar"}},
			Resources: map[string]CollectedTruth{
				"target": {Alias: "target", Name: "pulse-target", ResourceID: "id-target", ResourceType: "app-container", Status: "running", ObservedAt: time.Unix(1_699_999_995, 0).UTC()},
			},
		},
		Exchanges: []ToolExchange{{
			Sequence: 1, ToolCallID: "call-1", ToolName: "get_resource",
			CanonicalInput: `{}`, Output: `{"status":"ok"}`, Success: true,
		}},
		AIAnalysis: "analysis text",
		Findings: []Finding{{
			ID: "f1", Key: "k1", Severity: "warning", Category: "reliability",
			ResourceID: "id-target", ResourceName: "pulse-target", ResourceType: "app-container",
			Title: "t", Description: "d", Recommendation: "r", Evidence: "e",
			DetectedAt: time.Unix(1_699_999_994, 0).UTC(), LastSeenAt: time.Unix(1_699_999_993, 0).UTC(),
		}},
		Replayable: true,
	}
	validPayload, err := json.Marshal(validBundle)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	for _, tc := range []struct {
		name      string
		path      func(t *testing.T) string
		wantErr   bool
		errSubstr string
		// result is only checked on the success path; it must equal validBundle.
		checkResult func(t *testing.T, got ReplayBundle)
	}{
		{
			name: "missing path surfaces os read error",
			path: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist.json")
			},
			wantErr: true,
		},
		{
			name: "directory where file is expected surfaces read error",
			path: func(t *testing.T) string {
				return t.TempDir() // a directory, not a file
			},
			wantErr: true,
		},
		{
			name: "invalid json returns decoder error",
			path: func(t *testing.T) string {
				return qualreplay0723write(t, "bundle.json", []byte("{not json"))
			},
			wantErr: true,
		},
		{
			name: "empty file returns decoder error",
			path: func(t *testing.T) string {
				return qualreplay0723write(t, "empty.json", []byte{})
			},
			wantErr: true,
		},
		{
			name: "trailing second value rejected as multiple json values",
			path: func(t *testing.T) string {
				// `{}` decodes into ReplayBundle with zero values; the trailing
				// `{}` decodes successfully, hitting the multiple-values arm.
				return qualreplay0723write(t, "multi.json", []byte("{}{}"))
			},
			wantErr:   true,
			errSubstr: "multiple JSON values",
		},
		{
			name: "trailing garbage rejected with decode error",
			path: func(t *testing.T) string {
				// First `{}` decodes; ` junk` fails to decode, hitting the
				// trailing-garbage arm (err != nil && err != io.EOF).
				return qualreplay0723write(t, "garbage.json", []byte("{} junk"))
			},
			wantErr: true,
		},
		{
			name: "unknown field rejected by DisallowUnknownFields",
			path: func(t *testing.T) string {
				return qualreplay0723write(t, "unknown.json", []byte(`{"schema_version":"`+ReplaySchemaVersion+`","surprise_field":1}`))
			},
			wantErr: true,
		},
		{
			name: "missing required schema version surfaces replay session error",
			path: func(t *testing.T) string {
				// `{}` has no schema_version; NewReplaySession rejects it.
				return qualreplay0723write(t, "noschema.json", []byte("{}"))
			},
			wantErr:   true,
			errSubstr: "unsupported replay schema",
		},
		{
			name: "wrong schema version surfaces replay session error",
			path: func(t *testing.T) string {
				return qualreplay0723write(t, "badschema.json", []byte(`{"schema_version":"wrong"}`))
			},
			wantErr:   true,
			errSubstr: "unsupported replay schema",
		},
		{
			name: "non-empty replay issues block load via session validation",
			path: func(t *testing.T) string {
				bundle := validBundle
				bundle.Replayable = false
				bundle.ReplayIssues = []string{"tool call x input is not complete JSON: unexpected EOF"}
				payload, err := json.Marshal(bundle)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				return qualreplay0723write(t, "issues.json", payload)
			},
			wantErr:   true,
			errSubstr: "replay capture is incomplete",
		},
		{
			name: "malformed exchange in bundle blocks load via session validation",
			path: func(t *testing.T) string {
				bundle := validBundle
				bundle.Exchanges = []ToolExchange{{Sequence: 1, ToolName: "", CanonicalInput: "{}"}}
				payload, err := json.Marshal(bundle)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				return qualreplay0723write(t, "badexch.json", payload)
			},
			wantErr:   true,
			errSubstr: "invalid replay exchange at index 0",
		},
		{
			name: "valid bundle round-trips every field",
			path: func(t *testing.T) string {
				return qualreplay0723write(t, "valid.json", validPayload)
			},
			checkResult: func(t *testing.T, got ReplayBundle) {
				if !reflect.DeepEqual(got, validBundle) {
					t.Fatalf("round-trip mismatch:\n got  = %+v\n want = %+v", got, validBundle)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoadReplayBundle(tc.path(t))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("LoadReplayBundle error = nil, want non-nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("LoadReplayBundle error = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadReplayBundle error = %v, want nil", err)
			}
			if tc.checkResult != nil {
				tc.checkResult(t, got)
			}
		})
	}
}

// TestBranchcov0723AmEnsureArtifactRoot covers every branch of
// EnsureArtifactRoot in runner.go: the empty-root early error, the fresh-path
// creation with the source's 0o700 permission, idempotence over an
// already-existing directory, and the failure when a parent path is a regular
// file rather than a directory. All filesystem activity is contained inside
// per-subtest t.TempDir() roots.
func TestBranchcov0723AmEnsureArtifactRoot(t *testing.T) {
	t.Run("empty root returns the dedicated error", func(t *testing.T) {
		err := EnsureArtifactRoot("")
		if err == nil {
			t.Fatalf("EnsureArtifactRoot(\"\") error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "artifact root is empty") {
			t.Fatalf("EnsureArtifactRoot(\"\") error = %q, want substring %q", err.Error(), "artifact root is empty")
		}
	})

	t.Run("fresh path creates directory with expected permissions", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "fresh-artifacts")
		if err := EnsureArtifactRoot(root); err != nil {
			t.Fatalf("EnsureArtifactRoot(%q) error = %v, want nil", root, err)
		}
		info, err := os.Stat(root)
		if err != nil {
			t.Fatalf("stat created root: %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("EnsureArtifactRoot(%q) did not create a directory: mode=%v", root, info.Mode())
		}
		if got := info.Mode() & os.ModePerm; got != 0o700 {
			t.Fatalf("EnsureArtifactRoot(%q) perm = %o, want 0o700", root, got)
		}
	})

	t.Run("already existing directory is idempotent", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "existing-artifacts")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("seed existing dir: %v", err)
		}
		if err := EnsureArtifactRoot(root); err != nil {
			t.Fatalf("EnsureArtifactRoot on existing dir error = %v, want nil", err)
		}
		info, err := os.Stat(root)
		if err != nil {
			t.Fatalf("stat existing root after call: %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("EnsureArtifactRoot clobbered existing directory: mode=%v", info.Mode())
		}
	})

	t.Run("parent is a regular file returns mkdir error", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "i-am-a-file")
		if err := os.WriteFile(parent, []byte("not a dir"), 0o600); err != nil {
			t.Fatalf("seed blocker file: %v", err)
		}
		target := filepath.Join(parent, "child")
		err := EnsureArtifactRoot(target)
		if err == nil {
			t.Fatalf("EnsureArtifactRoot(%q) error = nil, want non-nil", target)
		}
		// The target must not be usable as a directory: stat either fails
		// outright (ENOTDIR propagated through the regular-file parent) or
		// reports something that is not a directory.
		info, statErr := os.Stat(target)
		if statErr == nil && info.IsDir() {
			t.Fatalf("EnsureArtifactRoot(%q) unexpectedly created a usable directory", target)
		}
	})
}
