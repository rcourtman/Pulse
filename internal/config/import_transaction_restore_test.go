package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Failed replacement must restore both overwritten files and the absence of
// files first created by this transaction, before returning to its caller.
func TestImportTransactionFailedCommitRestoresEveryTarget(t *testing.T) {
	for _, failure := range []string{"later_directory", "missing_staged_existing", "missing_staged_new"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			existing := filepath.Join(dir, "a-existing")
			created := filepath.Join(dir, "b-new")
			blocked := filepath.Join(dir, "z-directory")
			if err := os.WriteFile(existing, []byte("original"), 0640); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(blocked, 0700); err != nil {
				t.Fatal(err)
			}
			tx, err := newImportTransaction(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Cleanup()
			for _, target := range []string{existing, created, blocked} {
				if err := tx.StageFile(target, []byte("replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if failure != "later_directory" {
				target := existing
				if failure == "missing_staged_new" {
					target = created
				}
				if err := os.Remove(tx.staged[target]); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err == nil {
				t.Fatal("expected failed commit")
			}
			assertRestored := func() {
				t.Helper()
				data, err := os.ReadFile(existing)
				if err != nil || string(data) != "original" {
					t.Error("failed commit did not restore the original file")
				}
				info, err := os.Stat(existing)
				if err != nil || info.Mode().Perm() != 0640 {
					t.Error("failed commit did not restore original permissions")
				}
				if _, err := os.Stat(created); !os.IsNotExist(err) {
					t.Error("failed commit left a newly created configuration file")
				}
				if info, err := os.Stat(blocked); err != nil || !info.IsDir() {
					t.Error("failed commit changed the blocking directory")
				}
			}
			assertRestored()
			tx.Rollback()
			assertRestored()
		})
	}
}
