package server

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Use a different source directory: constructing persistence generates a key,
// which is retained destination state, not evidence of a blank installation.
func newAutoImportExport(t *testing.T) (string, string) {
	t.Helper()
	source := config.NewConfigPersistence(t.TempDir())
	if err := source.SaveAPITokens([]config.APITokenRecord{{
		ID: "synthetic-import-record", Hash: "synthetic-import-digest",
		Scopes: []string{config.ScopeAgentReport},
	}}); err != nil {
		t.Fatal(err)
	}
	const passphrase = "synthetic-startup-import-passphrase"
	payload, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	return payload, passphrase
}

func setAutoImportInputs(t *testing.T, dir, payload, passphrase string) {
	t.Helper()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_INIT_CONFIG_DATA", payload)
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", passphrase)
}

type autoImportEntry struct {
	Mode fs.FileMode
	Data string
}

func snapshotAutoImportDestination(t *testing.T, dir string) map[string]autoImportEntry {
	t.Helper()
	entries := make(map[string]autoImportEntry)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		snapshot := autoImportEntry{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot.Data = string(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			snapshot.Data, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		entries[name] = snapshot
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestAutoImportRetainsDestinationState(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	for _, name := range []string{
		"nodes.enc", "system.json", "api_tokens.json", "sessions.json", ".encryption.key",
		"sso.enc", "oidc.enc", "truenas.enc", "vmware.enc", "availability_targets.enc",
		"host_agents.json", "docker_agents.json", "alerts.json", "guest_metadata.json",
		"orgs/default/system.json", "unknown-state", "metrics.db", "import.enc",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			// Inert sentinels, including .db: no database or executable is opened.
			if err := os.WriteFile(path, []byte("synthetic-retained-state"), 0600); err != nil {
				t.Fatal(err)
			}
			setAutoImportInputs(t, dir, payload, passphrase)
			before := snapshotAutoImportDestination(t, dir)
			if ShouldAutoImport() {
				t.Error("existing destination was admitted by the startup gate")
			}
			capture := setCaptureAuditLogger(t)
			if err := PerformAutoImport(); err == nil {
				t.Error("direct startup import replaced a nonempty destination")
			}
			if after := snapshotAutoImportDestination(t, dir); !reflect.DeepEqual(before, after) {
				t.Error("startup import changed retained destination bytes, paths or modes")
			}
			if len(capture.events) != 1 || capture.events[0].Success ||
				!strings.Contains(capture.events[0].Details, "reason=destination_not_empty") {
				t.Error("expected a classified refusal audit without destination data")
			}
		})
	}
}

func TestAutoImportAllowsBlankDestination(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	for _, scenario := range []string{"empty", "absent", "deployment-env", "file-input"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			if scenario == "absent" {
				dir = filepath.Join(dir, "new-data")
			}
			const deploymentEnv = "PULSE_AUTH_USER=synthetic-local-admin\n"
			if scenario == "deployment-env" {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(deploymentEnv), 0600); err != nil {
					t.Fatal(err)
				}
			}
			setAutoImportInputs(t, dir, payload, passphrase)
			if scenario == "file-input" {
				file := filepath.Join(t.TempDir(), "private-export.enc")
				if err := os.WriteFile(file, []byte(payload), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PULSE_INIT_CONFIG_FILE", file)
				t.Setenv("PULSE_INIT_CONFIG_DATA", "deliberately-invalid-data")
			}
			if !ShouldAutoImport() {
				t.Fatal("blank destination was not admitted")
			}
			if scenario == "absent" {
				if _, err := os.Lstat(dir); !os.IsNotExist(err) {
					t.Fatal("admission probe created the destination")
				}
			} else if _, err := os.Lstat(filepath.Join(dir, ".encryption.key")); !os.IsNotExist(err) {
				t.Fatal("admission probe generated an encryption key")
			}
			if err := PerformAutoImport(); err != nil {
				t.Fatal(err)
			}
			records, err := config.NewConfigPersistence(dir).LoadAPITokens()
			if err != nil || len(records) != 1 || records[0].ID != "synthetic-import-record" {
				t.Fatal("blank destination did not receive the encrypted source token records")
			}
			if ShouldAutoImport() {
				t.Fatal("a completed bootstrap was admitted again")
			}
			if scenario == "deployment-env" {
				raw, err := os.ReadFile(filepath.Join(dir, ".env"))
				if err != nil || string(raw) != deploymentEnv {
					t.Fatal("bootstrap did not retain destination-local deployment access")
				}
			}
		})
	}
}

func TestAutoImportRechecksDestinationBeforeWriting(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	dir := t.TempDir()
	setAutoImportInputs(t, dir, payload, passphrase)
	if !ShouldAutoImport() {
		t.Fatal("blank destination not admitted")
	}
	if err := os.WriteFile(filepath.Join(dir, "api_tokens.json"), []byte("synthetic-later-state"), 0600); err != nil {
		t.Fatal(err)
	}
	before := snapshotAutoImportDestination(t, dir)
	if err := PerformAutoImport(); err == nil {
		t.Error("stale admission permitted replacement of later state")
	}
	if after := snapshotAutoImportDestination(t, dir); !reflect.DeepEqual(before, after) {
		t.Error("late destination state was changed")
	}
}

func TestAutoImportRejectsUncertainDestination(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	for _, scenario := range []string{
		"non-directory", "symlink-directory", "dangling-directory-link", "unreadable-directory",
		"nodes-directory", "nodes-dangling-link", "env-directory", "env-symlink", "input-inside-destination",
	} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			setAutoImportInputs(t, dir, payload, passphrase)
			switch scenario {
			case "non-directory":
				dir = filepath.Join(dir, "not-a-directory")
				if err := os.WriteFile(dir, []byte("synthetic-existing-state"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink-directory", "dangling-directory-link":
				target := t.TempDir()
				if scenario == "dangling-directory-link" {
					target = filepath.Join(target, "absent")
				}
				dir = filepath.Join(dir, "data-link")
				if err := os.Symlink(target, dir); err != nil {
					t.Fatal(err)
				}
			case "unreadable-directory":
				if os.Geteuid() == 0 {
					t.Skip("permission control requires the ordinary proof user")
				}
				if err := os.Chmod(dir, 0000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(dir, 0700); err != nil {
						t.Error(err)
					}
				})
			case "nodes-directory", "env-directory":
				leaf := "nodes.enc"
				if scenario == "env-directory" {
					leaf = ".env"
				}
				if err := os.Mkdir(filepath.Join(dir, leaf), 0700); err != nil {
					t.Fatal(err)
				}
			case "nodes-dangling-link", "env-symlink":
				leaf := "nodes.enc"
				if scenario == "env-symlink" {
					leaf = ".env"
				}
				if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), filepath.Join(dir, leaf)); err != nil {
					t.Fatal(err)
				}
			case "input-inside-destination":
				file := filepath.Join(dir, "private-export.enc")
				if err := os.WriteFile(file, []byte(payload), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PULSE_INIT_CONFIG_FILE", file)
			}
			t.Setenv("PULSE_DATA_DIR", dir)
			if ShouldAutoImport() {
				t.Error("unknown or nonempty destination was admitted")
			}
			if err := PerformAutoImport(); err == nil {
				t.Error("unknown or nonempty destination allowed direct startup import")
			} else if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), payload) || strings.Contains(err.Error(), passphrase) {
				t.Error("destination refusal disclosed private path or input")
			}
		})
	}
}

func TestAutoImportFileFailureHasNoDataFallback(t *testing.T) {
	payload, passphrase := newAutoImportExport(t)
	for _, scenario := range []string{"missing-file", "empty-file", "missing-passphrase"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			setAutoImportInputs(t, dir, payload, passphrase)
			file := filepath.Join(t.TempDir(), "private-export.enc")
			if scenario == "empty-file" {
				if err := os.WriteFile(file, []byte(" \n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing-passphrase" {
				t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "")
			} else {
				t.Setenv("PULSE_INIT_CONFIG_FILE", file)
			}
			if !ShouldAutoImport() {
				t.Fatal("blank destination must reach ordinary input validation")
			}
			before := snapshotAutoImportDestination(t, dir)
			capture := setCaptureAuditLogger(t)
			if err := PerformAutoImport(); err == nil {
				t.Error("invalid selected file or missing passphrase admitted an import")
			}
			if after := snapshotAutoImportDestination(t, dir); !reflect.DeepEqual(before, after) {
				t.Error("input failure changed the destination")
			}
			wantReason := map[string]string{
				"missing-file": "read_config_file_failed", "empty-file": "normalize_payload_failed", "missing-passphrase": "missing_passphrase",
			}[scenario]
			if len(capture.events) != 1 || capture.events[0].Success ||
				!strings.Contains(capture.events[0].Details, fmt.Sprintf("reason=%s", wantReason)) {
				t.Error("input failure audit lost its actual phase")
			}
		})
	}
}
