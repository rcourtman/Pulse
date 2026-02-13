package unified

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
