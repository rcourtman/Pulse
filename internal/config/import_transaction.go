package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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
func (tx *importTransaction) StageFile(target string, data []byte, perm os.FileMode) (retErr error) {
	if tx.committed {
		return fmt.Errorf("transaction already committed")
	}

	if err := os.MkdirAll(tx.stagingDir, 0o700); err != nil {
		return fmt.Errorf("ensure staging dir: %w", err)
	}

	// Remove any previously staged data for this target.
	if existing, ok := tx.staged[target]; ok {
		if err := os.Remove(existing); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove previously staged file for %s: %w", target, err)
		}
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
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close staged file for %s: %w", target, closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}

			// Do not leave an open/possibly incomplete file staged on close failure.
			retErr = wrappedCloseErr
			if removeErr := os.Remove(tmpFile.Name()); removeErr != nil && !os.IsNotExist(removeErr) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup staged file %s after close failure: %w", tmpFile.Name(), removeErr))
			}
			if stagedPath, ok := tx.staged[target]; ok && stagedPath == tmpFile.Name() {
				delete(tx.staged, target)
			}
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		retErr = fmt.Errorf("write staged file for %s: %w", target, err)
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil && !os.IsNotExist(removeErr) {
			retErr = errors.Join(retErr, fmt.Errorf("cleanup staged file %s after write failure: %w", tmpFile.Name(), removeErr))
		}
		return retErr
	}

	if err := tmpFile.Chmod(perm); err != nil {
		retErr = fmt.Errorf("chmod staged file for %s: %w", target, err)
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil && !os.IsNotExist(removeErr) {
			retErr = errors.Join(retErr, fmt.Errorf("cleanup staged file %s after chmod failure: %w", tmpFile.Name(), removeErr))
		}
		return retErr
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

	restore := func() error {
		var restoreErr error
		joinRestoreErr := func(err error) {
			if err == nil {
				return
			}
			if restoreErr == nil {
				restoreErr = err
				return
			}
			restoreErr = errors.Join(restoreErr, err)
		}

		for i := len(applied) - 1; i >= 0; i-- {
			target := applied[i]
			stagedPath := tx.staged[target]

			if err := os.Remove(stagedPath); err != nil && !os.IsNotExist(err) {
				joinRestoreErr(fmt.Errorf("remove staged file %s during restore: %w", stagedPath, err))
			}

			// Restore backup if present.
			if backup := tx.backups[target]; backup != "" {
				if _, err := os.Stat(backup); err != nil {
					if !os.IsNotExist(err) {
						joinRestoreErr(fmt.Errorf("stat backup %s during restore: %w", backup, err))
					}
					continue
				}

				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					joinRestoreErr(fmt.Errorf("remove target %s during restore: %w", target, err))
					continue
				}
				if err := os.Rename(backup, target); err != nil {
					joinRestoreErr(fmt.Errorf("restore backup %s to %s: %w", backup, target, err))
					continue
				}
				tx.backups[target] = ""
			}
		}
		return restoreErr
	}

	failWithRestore := func(opErr error) error {
		if restoreErr := restore(); restoreErr != nil {
			return errors.Join(opErr, fmt.Errorf("restore transaction state: %w", restoreErr))
		}
		return opErr
	}

	for _, target := range targets {
		stagedPath := tx.staged[target]

		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return failWithRestore(fmt.Errorf("ensure dir for %s: %w", target, err))
		}

		// Move current file to backup (if it exists and isn't already a dir).
		if info, err := os.Stat(target); err == nil {
			if info.IsDir() {
				return failWithRestore(fmt.Errorf("destination %s is a directory", target))
			}
			backupPath := fmt.Sprintf("%s.import-backup-%s", target, tx.timestamp)
			if err := os.Rename(target, backupPath); err != nil {
				return failWithRestore(fmt.Errorf("backup existing file %s: %w", target, err))
			}
			tx.backups[target] = backupPath
		} else if !os.IsNotExist(err) {
			return failWithRestore(fmt.Errorf("stat destination %s: %w", target, err))
		}

		if err := os.Rename(stagedPath, target); err != nil {
			return failWithRestore(fmt.Errorf("apply staged file to %s: %w", target, err))
		}

		applied = append(applied, target)
	}

	// Successful commit: remove backups. Keep success behavior, but surface cleanup failures in logs.
	for _, target := range applied {
		if backup := tx.backups[target]; backup != "" {
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("backup_path", backup).Str("target", target).Msg("Failed to remove import backup after commit")
				continue
			}
			tx.backups[target] = ""
		}
	}
	return nil
}

// Rollback drops all staged files and restores any backups already created.
func (tx *importTransaction) Rollback() {
	for target, stagedPath := range tx.staged {
		if err := os.Remove(stagedPath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("staged_path", stagedPath).Str("target", target).Msg("Failed to remove staged file during rollback")
		}

		if backup := tx.backups[target]; backup != "" {
			// Only attempt restore when backup still exists.
			if _, err := os.Stat(backup); err != nil {
				if !os.IsNotExist(err) {
					log.Warn().Err(err).Str("backup_path", backup).Str("target", target).Msg("Failed to stat backup during rollback")
				}
				continue
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("target", target).Str("backup_path", backup).Msg("Failed to remove target during rollback")
				continue
			}
			if err := os.Rename(backup, target); err != nil {
				log.Warn().Err(err).Str("target", target).Str("backup_path", backup).Msg("Failed to restore backup during rollback")
				continue
			}
			tx.backups[target] = ""
		}
	}
}

// Cleanup removes the staging directory.
func (tx *importTransaction) Cleanup() {
	if err := os.RemoveAll(tx.stagingDir); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("staging_dir", tx.stagingDir).Msg("Failed to remove import staging directory")
	}
}
