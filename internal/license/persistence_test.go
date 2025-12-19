package license

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistence(t *testing.T) {
	// Create a temporary directory for config
	tmpDir, err := os.MkdirTemp("", "pulse-license-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	testLicenseKey := "test-license-key-123"

	t.Run("Save and Load", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		if !p.Exists() {
			t.Error("License file should exist")
		}

		loadedKey, err := p.Load()
		if err != nil {
			t.Fatalf("Failed to load license: %v", err)
		}

		if loadedKey != testLicenseKey {
			t.Errorf("Expected license key %s, got %s", testLicenseKey, loadedKey)
		}
	})

	t.Run("Save and Load with Grace Period", func(t *testing.T) {
		gracePeriodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()
		err := p.SaveWithGracePeriod(testLicenseKey, &gracePeriodEnd)
		if err != nil {
			t.Fatalf("Failed to save license with grace period: %v", err)
		}

		persisted, err := p.LoadWithMetadata()
		if err != nil {
			t.Fatalf("Failed to load license with metadata: %v", err)
		}

		if persisted.LicenseKey != testLicenseKey {
			t.Errorf("Expected license key %s, got %s", testLicenseKey, persisted.LicenseKey)
		}

		if persisted.GracePeriodEnd == nil || *persisted.GracePeriodEnd != gracePeriodEnd {
			t.Errorf("Expected grace period end %v, got %v", gracePeriodEnd, persisted.GracePeriodEnd)
		}
	})

	t.Run("Load non-existent", func(t *testing.T) {
		tmpDirEmpty, _ := os.MkdirTemp("", "pulse-license-test-empty-*")
		defer os.RemoveAll(tmpDirEmpty)
		
		pEmpty, _ := NewPersistence(tmpDirEmpty)
		key, err := pEmpty.Load()
		if err != nil {
			t.Fatalf("Expected no error for non-existent license, got %v", err)
		}
		if key != "" {
			t.Errorf("Expected empty key, got %s", key)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		if !p.Exists() {
			t.Fatal("License should exist before delete")
		}

		err = p.Delete()
		if err != nil {
			t.Fatalf("Failed to delete license: %v", err)
		}

		if p.Exists() {
			t.Error("License should not exist after delete")
		}
	})

	t.Run("Encryption check", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		// Read the file directly - it should be base64 encoded and encrypted
		licensePath := filepath.Join(tmpDir, LicenseFileName)
		data, err := os.ReadFile(licensePath)
		if err != nil {
			t.Fatalf("Failed to read license file: %v", err)
		}

		if string(data) == testLicenseKey {
			t.Error("License file should be encrypted, not raw text")
		}
		
		// Ensure it's not JSON either in raw form
		if data[0] == '{' {
			t.Error("License file should be encrypted, not raw JSON")
		}
	})
	
	t.Run("Decrypt with wrong key material", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}
		
		// Create a new persistence with different machine ID
		pWrong := &Persistence{
			configDir: tmpDir,
			machineID: "different-machine-id",
		}
		
		_, err = pWrong.Load()
		if err == nil {
			t.Error("Expected error when decrypting with wrong key material")
		}
	})
}
