package config

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// Authentication of the encrypted bytes is not proof that their JSON describes
// a backup. An incomplete envelope must not become a destructive zero-value
// configuration, even when the operator has supplied the correct passphrase.
func TestImportConfigRejectsIncompleteArchiveWithoutMutation(t *testing.T) {
	for _, name := range []string{"null", "empty", "metadata_only", "missing_nodes", "null_nodes", "missing_alerts", "null_alerts", "missing_email", "null_email", "missing_webhooks", "missing_system", "null_system", "wrong_type"} {
		t.Run(name, func(t *testing.T) {
			cp := seedImportValidationState(t)
			before := importValidationFiles(t, cp.GetConfigDir())
			metadataBefore := cp.GetGuestMetadataStore().GetAll()
			data := incompleteImportValidationData(t, name)
			err := cp.ImportConfig(encryptImportValidationData(t, data), "synthetic-import-validation-passphrase")
			if err == nil {
				t.Error("incomplete encrypted backup was accepted")
			}
			if !reflect.DeepEqual(before, importValidationFiles(t, cp.GetConfigDir())) {
				t.Error("rejected backup changed persisted configuration, credentials or import artifacts")
			}
			if !reflect.DeepEqual(metadataBefore, cp.GetGuestMetadataStore().GetAll()) {
				t.Error("rejected backup changed live guest metadata")
			}
		})
	}
}

func incompleteImportValidationData(t *testing.T, name string) []byte {
	t.Helper()
	switch name {
	case "null":
		return []byte("null")
	case "empty":
		return []byte("{}")
	case "metadata_only":
		return []byte(`{"version":"4.4","exportedAt":"2026-10-25T00:00:00Z"}`)
	}
	data := map[string]json.RawMessage{
		"version": json.RawMessage(`"4.4"`), "nodes": json.RawMessage(`{}`),
		"alerts": json.RawMessage(`{}`), "email": json.RawMessage(`{}`),
		"webhooks": json.RawMessage(`null`), "system": json.RawMessage(`{}`),
	}
	for _, field := range []string{"nodes", "alerts", "email", "webhooks", "system"} {
		if name == "missing_"+field {
			delete(data, field)
		}
		if name == "null_"+field {
			data[field] = json.RawMessage(`null`)
		}
	}
	if name == "wrong_type" {
		data["nodes"] = json.RawMessage(`[]`)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func encryptImportValidationData(t *testing.T, data []byte) string {
	t.Helper()
	encrypted, err := encryptWithPassphrase(data, "synthetic-import-validation-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encrypted)
}

func seedImportValidationState(t *testing.T) *ConfigPersistence {
	t.Helper()
	cp := NewConfigPersistence(t.TempDir())
	if err := cp.SaveNodesConfig([]PVEInstance{{Name: "keep-node", Host: "https://keep.invalid:8006", TokenValue: "synthetic-node-secret", Disabled: true, VerifySSL: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	settings := DefaultSystemSettings()
	settings.PublicURL = "https://keep.invalid"
	if err := cp.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	if err := cp.SaveAPITokens([]APITokenRecord{{ID: "keep-token", Hash: "synthetic-hash", OrgID: "default", Scopes: []string{ScopeSettingsRead}}}); err != nil {
		t.Fatal(err)
	}
	if err := cp.SaveEmailConfig(notifications.EmailConfig{SMTPHost: "smtp.invalid", Password: "synthetic-mail-secret", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := cp.GetGuestMetadataStore().Set("keep:101", &GuestMetadata{Description: "Keep existing operator metadata"}); err != nil {
		t.Fatal(err)
	}
	return cp
}

type importValidationFile struct {
	Mode fs.FileMode
	Data string
}

func importValidationFiles(t *testing.T, dir string) map[string]importValidationFile {
	t.Helper()
	files := make(map[string]importValidationFile)
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[path] = importValidationFile{Mode: fs.ModeDir}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = importValidationFile{Mode: info.Mode(), Data: string(data)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestImportConfigAcceptsCompleteLegacyAndEmptyArchives(t *testing.T) {
	for _, version := range []string{"4.0", "4.1", "4.2", "4.3", "4.4", "", "future"} {
		t.Run("version_"+version, func(t *testing.T) {
			cp := seedImportValidationState(t)
			// All five sections have been emitted since 4.0. In particular,
			// optional modern sections need not exist in a legitimate backup.
			data := map[string]any{
				"nodes": map[string]any{}, "alerts": map[string]any{},
				"email": map[string]any{}, "system": map[string]any{},
				"webhooks": nil, "unknownFutureField": "must remain compatible",
			}
			if version != "" {
				data["version"] = version
			}
			if version == "4.4" {
				data["webhooks"] = []any{}
			}
			encoded, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := cp.ImportConfig(encryptImportValidationData(t, encoded), "synthetic-import-validation-passphrase"); err != nil {
				t.Fatalf("complete archive rejected: %v", err)
			}
			nodes, err := cp.LoadNodesConfig()
			if err != nil || len(nodes.PVEInstances) != 0 {
				t.Fatal("intentional empty node inventory was not applied")
			}
			tokens, err := cp.LoadAPITokens()
			if err != nil {
				t.Fatal(err)
			}
			if version == "4.0" {
				if len(tokens) != 1 || tokens[0].ID != "keep-token" {
					t.Error("4.0 import did not preserve the existing token inventory")
				}
			} else if len(tokens) != 0 {
				t.Error("complete archive no longer applies an intentionally empty token inventory")
			}
		})
	}
}
