package api

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

func TestHandleImportConfigAcceptsLegacyVersion40Bundle(t *testing.T) {
	targetDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", targetDir)

	cfg := &config.Config{
		DataPath:   targetDir,
		ConfigPath: targetDir,
	}
	handler := newTestConfigHandlers(t, cfg)
	handler.reloadFunc = func() error { return nil }

	// Build source export payload with distinct node data.
	sourceDir := t.TempDir()
	sourcePersistence := config.NewConfigPersistence(sourceDir)
	sourceNodes := []config.PVEInstance{
		{
			Name:     "pve-legacy-source",
			Host:     "https://legacy-source.example:8006",
			User:     "root@pam",
			Password: "legacy-secret",
		},
	}
	if err := sourcePersistence.SaveNodesConfig(sourceNodes, nil, nil); err != nil {
		t.Fatalf("failed to save source nodes: %v", err)
	}
	if err := sourcePersistence.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatalf("failed to save source system settings: %v", err)
	}

	passphrase := "legacy-import-passphrase"
	exported, err := sourcePersistence.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("failed to export source config: %v", err)
	}

	legacyPayload := mustRewriteExportVersion(t, exported, passphrase, "4.0", true)

	// Seed target with baseline values and API tokens to verify 4.0 token preservation.
	targetPersistence := config.NewConfigPersistence(targetDir)
	handler.legacyPersistence = targetPersistence
	if err := targetPersistence.SaveNodesConfig([]config.PVEInstance{
		{
			Name: "pve-baseline",
			Host: "https://baseline.example:8006",
			User: "root@pam",
		},
	}, nil, nil); err != nil {
		t.Fatalf("failed to save baseline nodes: %v", err)
	}
	if err := targetPersistence.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatalf("failed to save baseline system settings: %v", err)
	}

	baselineTokens := []config.APITokenRecord{
		{
			ID:        "token-baseline",
			Name:      "baseline",
			Hash:      "hash-baseline",
			Prefix:    "hashb",
			Suffix:    "line",
			CreatedAt: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeSettingsRead},
		},
	}
	if err := targetPersistence.SaveAPITokens(baselineTokens); err != nil {
		t.Fatalf("failed to save baseline api tokens: %v", err)
	}

	// Required by handleImportConfig -> config.Load().
	if err := os.WriteFile(filepath.Join(targetDir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write config.json: %v", err)
	}

	body, _ := json.Marshal(ImportConfigRequest{
		Data:       legacyPayload,
		Passphrase: passphrase,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleImportConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. body=%s", rec.Code, rec.Body.String())
	}

	nodesAfter, err := targetPersistence.LoadNodesConfig()
	if err != nil {
		t.Fatalf("failed to load nodes after import: %v", err)
	}
	if len(nodesAfter.PVEInstances) != 1 || nodesAfter.PVEInstances[0].Name != "pve-legacy-source" {
		t.Fatalf("expected imported legacy nodes, got %+v", nodesAfter.PVEInstances)
	}

	tokensAfter, err := targetPersistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("failed to load api tokens after import: %v", err)
	}
	if len(tokensAfter) != 1 || tokensAfter[0].ID != "token-baseline" {
		t.Fatalf("expected baseline api tokens to remain for 4.0 import, got %+v", tokensAfter)
	}
}

func mustRewriteExportVersion(t *testing.T, payload, passphrase, version string, stripAPITokens bool) string {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	plaintext, err := decryptExportCompatPayload(raw, passphrase)
	if err != nil {
		t.Fatalf("decrypt export payload failed: %v", err)
	}

	var data config.ExportData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		t.Fatalf("unmarshal export payload failed: %v", err)
	}

	data.Version = version
	if stripAPITokens {
		data.APITokens = nil
	}

	updatedPlaintext, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal updated export payload failed: %v", err)
	}

	updatedCiphertext, err := encryptExportCompatPayload(updatedPlaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt updated export payload failed: %v", err)
	}

	return base64.StdEncoding.EncodeToString(updatedCiphertext)
}

func encryptExportCompatPayload(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return append(salt, ciphertext...), nil
}

func decryptExportCompatPayload(ciphertext []byte, passphrase string) ([]byte, error) {
	if len(ciphertext) < 32 {
		return nil, io.ErrUnexpectedEOF
	}

	salt := ciphertext[:32]
	cipherbody := ciphertext[32:]

	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(cipherbody) < gcm.NonceSize() {
		return nil, io.ErrUnexpectedEOF
	}

	nonce := cipherbody[:gcm.NonceSize()]
	payload := cipherbody[gcm.NonceSize():]
	return gcm.Open(nil, nonce, payload, nil)
}
