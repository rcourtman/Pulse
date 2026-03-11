package unified

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnifiedFindingJSONCanonicalOutputAndLegacyInputCompatibility(t *testing.T) {
	finding := UnifiedFinding{
		ID:              "f1",
		Source:          SourceThreshold,
		Severity:        SeverityWarning,
		Category:        CategoryPerformance,
		ResourceID:      "res-1",
		Title:           "High CPU",
		AlertIdentifier: "instance:node:100::metric/cpu",
		DetectedAt:      time.Now(),
		LastSeenAt:      time.Now(),
	}

	raw, err := json.Marshal(finding)
	if err != nil {
		t.Fatalf("marshal unified finding: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode unified finding payload: %v", err)
	}
	if payload["alert_identifier"] != "instance:node:100::metric/cpu" {
		t.Fatalf("expected canonical alert_identifier, got %#v", payload["alert_identifier"])
	}
	if _, ok := payload["alert_id"]; ok {
		t.Fatalf("did not expect legacy alert_id in canonical payload, got %#v", payload["alert_id"])
	}

	var decodedCanonical UnifiedFinding
	if err := json.Unmarshal([]byte(`{
		"id":"f1",
		"source":"threshold",
		"severity":"warning",
		"category":"performance",
		"resource_id":"res-1",
		"title":"High CPU",
		"detected_at":"2026-03-11T00:00:00Z",
		"last_seen_at":"2026-03-11T00:00:00Z",
		"alert_identifier":"instance:node:100::metric/cpu"
	}`), &decodedCanonical); err != nil {
		t.Fatalf("unmarshal canonical unified finding: %v", err)
	}
	if decodedCanonical.AlertIdentifier != "instance:node:100::metric/cpu" {
		t.Fatalf("expected canonical alert_identifier to load, got %q", decodedCanonical.AlertIdentifier)
	}

	var decodedLegacy UnifiedFinding
	if err := json.Unmarshal([]byte(`{
		"id":"f1",
		"source":"threshold",
		"severity":"warning",
		"category":"performance",
		"resource_id":"res-1",
		"title":"High CPU",
		"detected_at":"2026-03-11T00:00:00Z",
		"last_seen_at":"2026-03-11T00:00:00Z",
		"alert_id":"legacy-alert-id"
	}`), &decodedLegacy); err != nil {
		t.Fatalf("unmarshal legacy unified finding: %v", err)
	}
	if decodedLegacy.AlertIdentifier != "legacy-alert-id" {
		t.Fatalf("expected legacy alert_id to load, got %q", decodedLegacy.AlertIdentifier)
	}
}

func TestFilePersistence_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersistence(dir)

	findings := map[string]*UnifiedFinding{
		"f1": {
			ID:         "f1",
			Source:     SourceThreshold,
			ResourceID: "res-1",
			DetectedAt: time.Now(),
		},
	}

	if err := p.SaveFindings(findings); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "unified_findings.json"))
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("expected mode 0600, got %o", got)
	}

	loaded, err := p.LoadFindings()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded))
	}
}

func TestFilePersistence_LoadMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	p := NewFilePersistence(dir)

	loaded, err := p.LoadFindings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty map for missing file")
	}

	path := filepath.Join(dir, "unified_findings.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err = p.LoadFindings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty map for invalid json")
	}
}

func TestVersionedPersistence_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	p := NewVersionedPersistence(dir)

	findings := map[string]*UnifiedFinding{
		"f1": {ID: "f1", Source: SourceAIPatrol, ResourceID: "res-1"},
	}

	if err := p.SaveFindings(findings); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "unified_findings.json"))
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("expected mode 0600, got %o", got)
	}

	loaded, err := p.LoadFindings()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded))
	}
}

func TestVersionedPersistence_LoadLegacy(t *testing.T) {
	dir := t.TempDir()
	p := NewVersionedPersistence(dir)

	path := filepath.Join(dir, "unified_findings.json")
	legacy := `[{"id":"f1","source":"ai-patrol","resource_id":"res-1"}]`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err := p.LoadFindings()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded))
	}
}
