package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleImportConfigMetadataFailureDoesNotApplyOrSucceed(t *testing.T) {
	for _, failure := range []string{"metadata_directory", "closed_store"} {
		t.Run(failure, func(t *testing.T) {
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
			if err := os.WriteFile(filepath.Join(cfg.DataPath, ".env"), []byte("AUTO_UPDATE_ENABLED=true\nPOLLING_INTERVAL=77\n"), 0600); err != nil {
				t.Fatal(err)
			}
			store := p.GetGuestMetadataStore()
			if err := store.Set("keep:101", &config.GuestMetadata{Description: "Retained operator metadata"}); err != nil {
				t.Fatal(err)
			}
			metadataBefore := store.GetAll()
			if failure == "closed_store" {
				if err := store.Close(time.Second); err != nil {
					t.Fatal(err)
				}
			} else {
				metadataPath := filepath.Join(cfg.DataPath, "guest_metadata.json")
				if err := os.Remove(metadataPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(metadataPath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			before := metadataImportFiles(t, cfg.DataPath)
			beforeConfig := cfg.DeepCopy()
			reloads, audits := 0, 0
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: p, reloadFunc: func() error { reloads++; return nil }}
			h.SetRuntimeDependencies(RuntimeDependencies{AuditEvent: func(_, event, _, _, _ string, success bool, _ string) {
				audits++
				if event != "config_imported" || success {
					t.Error("failed metadata import produced a success audit")
				}
			}})
			plaintext, err := json.Marshal(map[string]any{
				"version": "4.4", "nodes": map[string]any{}, "alerts": map[string]any{},
				"email": map[string]any{}, "webhooks": nil, "system": map[string]any{},
				"guestMetadata": map[string]any{"imported:202": map[string]string{"description": failure}},
			})
			if err != nil {
				t.Fatal(err)
			}
			request := ImportConfigRequest{Data: encryptedImportValidationFixture(t, plaintext, passphrase), Passphrase: passphrase}
			body, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			h.HandleImportConfig(rec, httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body)))
			if rec.Code != http.StatusBadRequest || rec.Body.String() != "Failed to import configuration. Verify the backup file and passphrase are correct.\n" {
				t.Fatalf("failed metadata import response = %d %q", rec.Code, rec.Body.String())
			}
			if reloads != 0 || audits != 1 || !reflect.DeepEqual(beforeConfig, cfg.DeepCopy()) {
				t.Error("failed import changed runtime or triggered apply")
			}
			if !reflect.DeepEqual(before, metadataImportFiles(t, cfg.DataPath)) || !reflect.DeepEqual(metadataBefore, store.GetAll()) {
				t.Error("failed import changed persisted or live state")
			}
			readback, err := json.Marshal(map[string]any{"request": request, "status": rec.Code, "contentType": rec.Header().Get("Content-Type"), "body": rec.Body.String()})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("SYNTHETIC_METADATA_IMPORT_REJECTION_READBACK %s", readback)
		})
	}
}

func metadataImportFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files[path] = info.Mode().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] += string(data)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
