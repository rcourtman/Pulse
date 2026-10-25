package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func metadataImportArchive(t *testing.T, metadata map[string]*GuestMetadata) string {
	t.Helper()
	settings := DefaultSystemSettings()
	settings.PublicURL = "https://imported.invalid"
	data, err := json.Marshal(ExportData{
		Version: "4.4", System: *settings,
		Email:         notifications.EmailConfig{SMTPHost: "imported.invalid", Password: "synthetic-imported-secret"},
		APITokens:     []APITokenRecord{{ID: "imported-token", Hash: "synthetic-imported-hash", OrgID: "default"}},
		GuestMetadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	return encryptImportValidationData(t, data)
}

func TestImportConfigMetadataFailurePreservesState(t *testing.T) {
	for _, failure := range []string{"metadata_directory", "closed_store", "later_destination", "later_destination_without_metadata_file"} {
		t.Run(failure, func(t *testing.T) {
			cp := seedImportValidationState(t)
			store := cp.GetGuestMetadataStore()
			metadataBefore := store.GetAll()
			metadataFile := filepath.Join(cp.configDir, "guest_metadata.json")
			var blocker string
			switch failure {
			case "metadata_directory":
				blocker = metadataFile
			case "closed_store":
				if err := store.Close(time.Second); err != nil {
					t.Fatal(err)
				}
			case "later_destination_without_metadata_file":
				if err := os.Remove(metadataFile); err != nil {
					t.Fatal(err)
				}
				blocker = cp.systemFile
			case "later_destination":
				blocker = cp.systemFile
			}
			if blocker != "" {
				if err := os.Remove(blocker); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(blocker, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("retained blocker"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := importValidationFiles(t, cp.configDir)
			err := cp.ImportConfig(metadataImportArchive(t, map[string]*GuestMetadata{
				"imported:202": {Description: "Imported metadata", Tags: []string{"imported"}},
			}), "synthetic-import-validation-passphrase")
			if err == nil {
				t.Error("failed metadata recovery was reported as successful")
			}
			if !reflect.DeepEqual(before, importValidationFiles(t, cp.configDir)) {
				t.Error("failed import changed files, credentials or transaction artifacts")
			}
			if !reflect.DeepEqual(metadataBefore, store.GetAll()) {
				t.Error("failed import replaced live metadata")
			}
		})
	}
}

func TestImportConfigCommitsMetadataWithConfiguration(t *testing.T) {
	for _, inventory := range []string{"nil", "empty", "populated"} {
		t.Run(inventory, func(t *testing.T) {
			cp := seedImportValidationState(t)
			var metadata map[string]*GuestMetadata
			expected := map[string]*GuestMetadata{}
			if inventory == "empty" {
				metadata = map[string]*GuestMetadata{}
			}
			if inventory == "populated" {
				metadata = map[string]*GuestMetadata{
					"imported:202": {ID: "wrong-id", Description: "Imported description", Notes: []string{"retained note"}},
					"null-entry":   nil,
				}
				expected["imported:202"] = &GuestMetadata{ID: "imported:202", Description: "Imported description", Notes: []string{"retained note"}, Tags: []string{}}
			}
			if err := cp.ImportConfig(metadataImportArchive(t, metadata), "synthetic-import-validation-passphrase"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cp.GetGuestMetadataStore().GetAll(), expected) {
				t.Error("successful import did not publish normalized metadata")
			}
			reloaded := NewGuestMetadataStore(cp.configDir, nil)
			if !reflect.DeepEqual(reloaded.GetAll(), expected) {
				t.Error("fresh metadata reader does not match the imported live state")
			}
			if err := cp.GetGuestMetadataStore().Load(); err != nil {
				t.Fatalf("committed metadata cannot be reloaded: %v", err)
			}
			settings, err := cp.LoadSystemSettings()
			if err != nil || settings.PublicURL != "https://imported.invalid" {
				t.Fatal("successful import did not persist settings")
			}
			tokens, err := cp.LoadAPITokens()
			if err != nil || len(tokens) != 1 || tokens[0].ID != "imported-token" {
				t.Fatal("successful import did not persist credentials")
			}
			info, err := os.Stat(filepath.Join(cp.configDir, "guest_metadata.json"))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("imported metadata is not a private file")
			}
			for path := range importValidationFiles(t, cp.configDir) {
				if strings.HasPrefix(filepath.Base(path), ".import-") || strings.Contains(filepath.Base(path), ".import-backup-") {
					t.Error("successful import retained transaction staging")
				}
			}
		})
	}
}
