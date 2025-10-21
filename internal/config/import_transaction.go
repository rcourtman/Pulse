package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// importTransaction coordinates staging config writes during an import so they
// can be committed atomically or rolled back on failure.
type importTransaction struct {
	configDir  string
	stagingDir string
	timestamp  string

	staged  map[string]string // target path -> staged temp file
	backups map[string]string // target path -> backup file path

	committed bool
}

func newImportTransaction(configDir string) (*importTransaction, error) {
	stagingDir, err := os.MkdirTemp(configDir, ".import-staging-*")
	if err != nil {
		return nil, fmt.Errorf("create import staging dir: %w", err)
	}

	tx := &importTransaction{
		configDir:  configDir,
		stagingDir: stagingDir,
		timestamp:  time.Now().UTC().Format("20060102-150405"),
		staged:     make(map[string]string),
		backups:    make(map[string]string),
	}
	return tx, nil
}

// StageFile writes the provided data to a temporary file within the staging
// directory and records it for later commit.
func (tx *importTransaction) StageFile(target string, data []byte, perm os.FileMode) error {
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}

	if err := os.MkdirAll(tx.stagingDir, 0o700); err != nil {
		return fmt.Errorf("ensure staging dir: %w", err)
	}

	// Remove any previously staged data for this target.
	if existing, ok := tx.staged[target]; ok {
		_ = os.Remove(existing)
	}

	base := filepath.Base(target)
	if base == "" || base == string(os.PathSeparator) {
		base = "staged"
	}
	prefix := strings.ReplaceAll(base, string(os.PathSeparator), "_")
	if prefix == "" {
		prefix = "staged"
	}
	if !strings.Contains(prefix, "*") {
		prefix = prefix + ".tmp-*"
	}

	// Create the staged file.
	tmpFile, err := os.CreateTemp(tx.stagingDir, prefix)
	if err != nil {
		return fmt.Errorf("create staged file for %s: %w", target, err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		_ = os.Remove(tmpFile.Name())
		return fmt.Errorf("write staged file for %s: %w", target, err)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		_ = os.Remove(tmpFile.Name())
		return fmt.Errorf("chmod staged file for %s: %w", target, err)
	}

	tx.staged[target] = tmpFile.Name()
	return nil
}

// Commit atomically applies all staged files. If any step fails the transaction
// restores previous backups and returns an error.
func (tx *importTransaction) Commit() error {
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}
	tx.committed = true

	targets := make([]string, 0, len(tx.staged))
	for target := range tx.staged {
		targets = append(targets, target)
	}
	sort.Strings(targets)

	applied := make([]string, 0, len(targets))

	restore := func() {
		for i := len(applied) - 1; i >= 0; i-- {
			target := applied[i]
			stagedPath := tx.staged[target]

			// Ensure staged file removed (best effort).
			_ = os.Remove(stagedPath)

			// Restore backup if present.
			if backup := tx.backups[target]; backup != "" {
				if _, err := os.Stat(backup); err == nil {
					_ = os.Remove(target)
					if err := os.Rename(backup, target); err == nil {
						tx.backups[target] = ""
					}
				}
			}
		}
	}

	for _, target := range targets {
		stagedPath := tx.staged[target]

		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			restore()
			return fmt.Errorf("ensure dir for %s: %w", target, err)
		}

		// Move current file to backup (if it exists and isn't already a dir).
		if info, err := os.Stat(target); err == nil {
			if info.IsDir() {
				restore()
				return fmt.Errorf("destination %s is a directory", target)
			}
			backupPath := fmt.Sprintf("%s.import-backup-%s", target, tx.timestamp)
			if err := os.Rename(target, backupPath); err != nil {
				restore()
				return fmt.Errorf("backup existing file %s: %w", target, err)
			}
			tx.backups[target] = backupPath
		} else if !os.IsNotExist(err) {
			restore()
			return fmt.Errorf("stat destination %s: %w", target, err)
		}

		if err := os.Rename(stagedPath, target); err != nil {
			restore()
			return fmt.Errorf("apply staged file to %s: %w", target, err)
		}

		applied = append(applied, target)
	}

	// Successful commit: remove backups (best effort).
	for _, target := range applied {
		if backup := tx.backups[target]; backup != "" {
			_ = os.Remove(backup)
			tx.backups[target] = ""
		}
	}
	return nil
}

// Rollback drops all staged files and restores any backups already created.
func (tx *importTransaction) Rollback() {
	for target, stagedPath := range tx.staged {
		_ = os.Remove(stagedPath)

		if backup := tx.backups[target]; backup != "" {
			// Only attempt restore when backup still exists.
			if _, err := os.Stat(backup); err != nil {
				continue
			}
			_ = os.Remove(target)
			if err := os.Rename(backup, target); err != nil {
				continue
			}
			tx.backups[target] = ""
		}
	}
}

// Cleanup removes the staging directory.
func (tx *importTransaction) Cleanup() {
	_ = os.RemoveAll(tx.stagingDir)
}
