package license

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistenceCompatibilityRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("NewPersistence() error = %v", err)
	}

	wantKey := "test-license-key-123"
	gracePeriodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()
	if err := p.SaveWithGracePeriod(wantKey, &gracePeriodEnd); err != nil {
		t.Fatalf("SaveWithGracePeriod() error = %v", err)
	}

	if !p.Exists() {
		t.Fatal("Exists() = false, want true")
	}

	got, err := p.LoadWithMetadata()
	if err != nil {
		t.Fatalf("LoadWithMetadata() error = %v", err)
	}
	if got.LicenseKey != wantKey {
		t.Fatalf("LicenseKey = %q, want %q", got.LicenseKey, wantKey)
	}
	if got.GracePeriodEnd == nil || *got.GracePeriodEnd != gracePeriodEnd {
		t.Fatalf("GracePeriodEnd = %v, want %d", got.GracePeriodEnd, gracePeriodEnd)
	}

	if err := p.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if p.Exists() {
		t.Fatal("Exists() = true after Delete(), want false")
	}
}

func TestPersistenceCompatibilityRejectsSymlinkPersistentKey(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "key-target")
	if err := os.WriteFile(target, []byte("test-key"), 0o600); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	keyPath := filepath.Join(tmpDir, PersistentKeyFileName)
	if err := os.Symlink(target, keyPath); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	if _, err := NewPersistence(tmpDir); err == nil {
		t.Fatal("expected error for symlink persistent key path")
	}
}
