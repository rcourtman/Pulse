package configapi

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

func TestHandleImportConfigRejectsIncompleteArchiveBeforeApply(t *testing.T) {
	for _, plaintext := range []string{"null", "{}", `{"version":"4.4"}`, `{"nodes":null,"alerts":{},"email":{},"webhooks":null,"system":{}}`} {
		t.Run(plaintext, func(t *testing.T) {
			const passphrase = "synthetic-import-validation-passphrase"
			cfg := &config.Config{DataPath: t.TempDir(), PublicURL: "https://keep.invalid", AuthUser: "keep-admin", AuthPass: "synthetic-auth-hash"}
			p := config.NewConfigPersistence(cfg.DataPath)
			settings := config.DefaultSystemSettings()
			settings.PublicURL = cfg.PublicURL
			if err := p.SaveSystemSettings(*settings); err != nil {
				t.Fatal(err)
			}
			if err := p.SaveAPITokens([]config.APITokenRecord{{ID: "keep-token", Hash: "synthetic-token-hash", OrgID: "default"}}); err != nil {
				t.Fatal(err)
			}
			beforeFiles := make(map[string]string)
			entries, err := os.ReadDir(cfg.DataPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(cfg.DataPath, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				beforeFiles[entry.Name()] = string(data)
			}
			beforeConfig := cfg.DeepCopy()
			reloads, audits := 0, 0
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: p, reloadFunc: func() error { reloads++; return nil }}
			h.SetRuntimeDependencies(RuntimeDependencies{AuditEvent: func(_, event, _, _, _ string, success bool, _ string) {
				audits++
				if event != "config_imported" || success {
					t.Error("incomplete backup did not produce a failed import audit")
				}
			}})
			request := ImportConfigRequest{Data: encryptedImportValidationFixture(t, []byte(plaintext), passphrase), Passphrase: passphrase}
			body, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			h.HandleImportConfig(rec, httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("incomplete archive status = %d, want 400", rec.Code)
			}
			if rec.Body.String() != "Failed to import configuration. Verify the backup file and passphrase are correct.\n" {
				t.Error("unexpected import failure envelope")
			}
			if reloads != 0 || audits != 1 || !reflect.DeepEqual(beforeConfig, cfg.DeepCopy()) {
				t.Error("rejected archive changed the runtime or triggered apply")
			}
			afterEntries, err := os.ReadDir(cfg.DataPath)
			if err != nil || len(afterEntries) != len(beforeFiles) {
				t.Fatal("rejected archive created or removed persisted files")
			}
			for _, entry := range afterEntries {
				data, err := os.ReadFile(filepath.Join(cfg.DataPath, entry.Name()))
				if err != nil || beforeFiles[entry.Name()] != string(data) {
					t.Error("rejected archive changed persisted settings or credentials")
				}
			}
			// Only synthetic request/response bytes leave this control. Browser
			// proof consumes this exact producer result in the existing dialog.
			readback, err := json.Marshal(map[string]any{"request": request, "status": rec.Code, "contentType": rec.Header().Get("Content-Type"), "body": rec.Body.String()})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("SYNTHETIC_IMPORT_REJECTION_READBACK %s", readback)
		})
	}
}

func encryptedImportValidationFixture(t *testing.T, plaintext []byte, passphrase string) string {
	t.Helper()
	// Fixed public salt/nonce keep this synthetic malformed fixture stable;
	// production export encryption still uses independent crypto/rand bytes.
	salt := sha256.Sum256([]byte("synthetic-import-validation-salt"))
	key := pbkdf2.Key([]byte(passphrase), salt[:], 600000, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(append(salt[:], ciphertext...))
}
