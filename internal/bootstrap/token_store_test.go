package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createToken() (string, error) {
	return "0123456789abcdef0123456789abcdef0123456789abcdef", nil
}

func TestLoadOrCreate_CreatesEncryptedTokenFile(t *testing.T) {
	dataPath := t.TempDir()

	token, created, path, migrated, err := LoadOrCreate(dataPath, createToken)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if migrated {
		t.Fatal("expected migrated=false")
	}
	if token == "" {
		t.Fatal("expected token")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), token) {
		t.Fatalf("bootstrap token persisted in plaintext: %q", string(data))
	}

	reloaded, loadPath, migrated, err := Load(dataPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if migrated {
		t.Fatal("expected migrated=false on encrypted read")
	}
	if loadPath != path {
		t.Fatalf("load path = %q, want %q", loadPath, path)
	}
	if reloaded != token {
		t.Fatalf("reloaded token = %q, want %q", reloaded, token)
	}
}

func TestLoadOrCreate_MigratesLegacyPlaintextFile(t *testing.T) {
	dataPath := t.TempDir()
	path := filepath.Join(dataPath, TokenFilename)
	const legacyToken = "legacy-bootstrap-token"
	if err := os.WriteFile(path, []byte(legacyToken+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	token, created, gotPath, migrated, err := LoadOrCreate(dataPath, createToken)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("expected created=false")
	}
	if !migrated {
		t.Fatal("expected migrated=true")
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if token != legacyToken {
		t.Fatalf("token = %q, want %q", token, legacyToken)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), legacyToken) {
		t.Fatalf("expected legacy token to be rewritten encrypted, got %q", string(data))
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, _, _, err := Load(t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error = %v, want ErrNotExist", err)
	}
}

func TestLoad_InvalidEncryptedPayload(t *testing.T) {
	dataPath := t.TempDir()
	path := filepath.Join(dataPath, TokenFilename)
	if err := os.WriteFile(path, []byte(`{"version":2,"token_ciphertext":"broken","token_hash":"abc"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, _, _, err := Load(dataPath)
	if err == nil || !strings.Contains(err.Error(), "decrypt bootstrap token") {
		t.Fatalf("Load() error = %v, want decrypt bootstrap token", err)
	}
}
