package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewImportTransaction(t *testing.T) {
	t.Run("creates staging directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		if tx.stagingDir == "" {
			t.Error("stagingDir should not be empty")
		}

		// Verify staging directory exists
		info, err := os.Stat(tx.stagingDir)
		if err != nil {
			t.Errorf("staging directory should exist: %v", err)
		}
		if !info.IsDir() {
			t.Error("stagingDir should be a directory")
		}

		// Verify staging dir is under config dir
		if !strings.HasPrefix(tx.stagingDir, tmpDir) {
			t.Errorf("stagingDir %q should be under configDir %q", tx.stagingDir, tmpDir)
		}

		// Verify staging dir contains expected prefix
		if !strings.Contains(tx.stagingDir, ".import-staging-") {
			t.Errorf("stagingDir %q should contain .import-staging- prefix", tx.stagingDir)
		}
	})

	t.Run("initializes maps", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		if tx.staged == nil {
			t.Error("staged map should be initialized")
		}
		if tx.backups == nil {
			t.Error("backups map should be initialized")
		}
	})

	t.Run("sets configDir", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		if tx.configDir != tmpDir {
			t.Errorf("configDir = %q, want %q", tx.configDir, tmpDir)
		}
	})

	t.Run("sets timestamp", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		if tx.timestamp == "" {
			t.Error("timestamp should not be empty")
		}

		// Verify timestamp format (YYYYMMDD-HHMMSS)
		if len(tx.timestamp) != 15 {
			t.Errorf("timestamp length = %d, expected 15", len(tx.timestamp))
		}
		if tx.timestamp[8] != '-' {
			t.Errorf("timestamp format incorrect, expected dash at position 8: %q", tx.timestamp)
		}
	})

	t.Run("fails with invalid directory", func(t *testing.T) {
		_, err := newImportTransaction("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})
}

func TestImportTransaction_StageFile(t *testing.T) {
	t.Run("stages file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "test.txt")
		data := []byte("test content")

		err = tx.StageFile(targetPath, data, 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		// Verify file is staged
		if _, ok := tx.staged[targetPath]; !ok {
			t.Error("file should be recorded in staged map")
		}

		// Verify staged file exists and has correct content
		stagedPath := tx.staged[targetPath]
		content, err := os.ReadFile(stagedPath)
		if err != nil {
			t.Fatalf("failed to read staged file: %v", err)
		}
		if string(content) != "test content" {
			t.Errorf("staged content = %q, want %q", string(content), "test content")
		}
	})

	t.Run("replaces previously staged file", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "test.txt")

		// Stage first version
		err = tx.StageFile(targetPath, []byte("version 1"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}
		firstStagedPath := tx.staged[targetPath]

		// Stage second version
		err = tx.StageFile(targetPath, []byte("version 2"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}
		secondStagedPath := tx.staged[targetPath]

		// Verify old staged file was removed
		if firstStagedPath == secondStagedPath {
			t.Error("second staged file should have different path")
		}
		if _, err := os.Stat(firstStagedPath); !os.IsNotExist(err) {
			t.Error("first staged file should be removed")
		}

		// Verify new content
		content, err := os.ReadFile(secondStagedPath)
		if err != nil {
			t.Fatalf("failed to read staged file: %v", err)
		}
		if string(content) != "version 2" {
			t.Errorf("staged content = %q, want %q", string(content), "version 2")
		}
	})

	t.Run("sets file permissions", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "test.txt")
		err = tx.StageFile(targetPath, []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		stagedPath := tx.staged[targetPath]
		info, err := os.Stat(stagedPath)
		if err != nil {
			t.Fatalf("failed to stat staged file: %v", err)
		}

		// Check permissions (mask with 0777 to ignore umask effects)
		perm := info.Mode().Perm() & 0o777
		if perm != 0o600 {
			t.Errorf("file permissions = %o, want %o", perm, 0o600)
		}
	})

	t.Run("fails after commit", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		// Commit empty transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Try to stage after commit
		targetPath := filepath.Join(tmpDir, "test.txt")
		err = tx.StageFile(targetPath, []byte("test"), 0o644)
		if err == nil {
			t.Error("expected error when staging after commit")
		}
		if !strings.Contains(err.Error(), "already committed") {
			t.Errorf("error should mention already committed: %v", err)
		}
	})

	t.Run("handles empty target base name", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		// Stage with target ending in separator (edge case)
		targetPath := filepath.Join(tmpDir, "subdir") + string(os.PathSeparator)
		err = tx.StageFile(targetPath, []byte("test"), 0o644)
		// Should not panic
		if err == nil {
			// It staged something - verify it's recorded
			if _, ok := tx.staged[targetPath]; !ok {
				t.Error("file should be recorded even with edge case path")
			}
		}
	})
}

func TestImportTransaction_Commit(t *testing.T) {
	t.Run("applies staged files", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "committed.txt")
		err = tx.StageFile(targetPath, []byte("committed content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify file exists at target
		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read committed file: %v", err)
		}
		if string(content) != "committed content" {
			t.Errorf("committed content = %q, want %q", string(content), "committed content")
		}
	})

	t.Run("creates backup of existing file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create existing file
		targetPath := filepath.Join(tmpDir, "existing.txt")
		err := os.WriteFile(targetPath, []byte("original content"), 0o644)
		if err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		err = tx.StageFile(targetPath, []byte("new content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify new content
		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read committed file: %v", err)
		}
		if string(content) != "new content" {
			t.Errorf("committed content = %q, want %q", string(content), "new content")
		}

		// Verify backup was removed on successful commit
		matches, _ := filepath.Glob(targetPath + ".import-backup-*")
		if len(matches) > 0 {
			t.Error("backup file should be removed after successful commit")
		}
	})

	t.Run("creates target directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		// Target in non-existent subdirectory
		targetPath := filepath.Join(tmpDir, "subdir", "nested", "file.txt")
		err = tx.StageFile(targetPath, []byte("nested content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify file exists
		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read committed file: %v", err)
		}
		if string(content) != "nested content" {
			t.Errorf("committed content = %q, want %q", string(content), "nested content")
		}
	})

	t.Run("fails if destination is directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create directory at target path
		targetPath := filepath.Join(tmpDir, "targetdir")
		err := os.Mkdir(targetPath, 0o755)
		if err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		err = tx.StageFile(targetPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		err = tx.Commit()
		if err == nil {
			t.Error("expected error when committing to directory")
		}
		if !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("error should mention directory: %v", err)
		}
	})

	t.Run("cannot commit twice", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		err = tx.Commit()
		if err != nil {
			t.Fatalf("first Commit() error = %v", err)
		}

		err = tx.Commit()
		if err == nil {
			t.Error("expected error on second commit")
		}
		if !strings.Contains(err.Error(), "already committed") {
			t.Errorf("error should mention already committed: %v", err)
		}
	})

	t.Run("commits multiple files atomically", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		// Stage multiple files
		files := map[string]string{
			filepath.Join(tmpDir, "file1.txt"): "content 1",
			filepath.Join(tmpDir, "file2.txt"): "content 2",
			filepath.Join(tmpDir, "file3.txt"): "content 3",
		}

		for path, content := range files {
			err = tx.StageFile(path, []byte(content), 0o644)
			if err != nil {
				t.Fatalf("StageFile(%s) error = %v", path, err)
			}
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Verify all files exist with correct content
		for path, expectedContent := range files {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("failed to read %s: %v", path, err)
				continue
			}
			if string(content) != expectedContent {
				t.Errorf("content of %s = %q, want %q", path, string(content), expectedContent)
			}
		}
	})
}

func TestImportTransaction_Rollback(t *testing.T) {
	t.Run("removes staged files", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "test.txt")
		err = tx.StageFile(targetPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		stagedPath := tx.staged[targetPath]

		// Verify staged file exists
		if _, err := os.Stat(stagedPath); err != nil {
			t.Fatalf("staged file should exist: %v", err)
		}

		tx.Rollback()

		// Verify staged file is removed
		if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
			t.Error("staged file should be removed after rollback")
		}
	})

	t.Run("restores backup files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create existing file
		targetPath := filepath.Join(tmpDir, "existing.txt")
		err := os.WriteFile(targetPath, []byte("original content"), 0o644)
		if err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		err = tx.StageFile(targetPath, []byte("new content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		// Manually simulate partial commit state by creating backup
		backupPath := targetPath + ".import-backup-" + tx.timestamp
		err = os.Rename(targetPath, backupPath)
		if err != nil {
			t.Fatalf("failed to create backup: %v", err)
		}
		tx.backups[targetPath] = backupPath

		tx.Rollback()

		// Verify original file is restored
		content, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("failed to read restored file: %v", err)
		}
		if string(content) != "original content" {
			t.Errorf("restored content = %q, want %q", string(content), "original content")
		}
	})

	t.Run("handles rollback with no backups", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		targetPath := filepath.Join(tmpDir, "new.txt")
		err = tx.StageFile(targetPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		// Rollback should not panic even with no backups
		tx.Rollback()

		// Target should not exist
		if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
			t.Error("target should not exist after rollback of new file")
		}
	})
}

func TestImportTransaction_Cleanup(t *testing.T) {
	t.Run("removes staging directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}

		stagingDir := tx.stagingDir

		// Verify staging directory exists
		if _, err := os.Stat(stagingDir); err != nil {
			t.Fatalf("staging directory should exist: %v", err)
		}

		tx.Cleanup()

		// Verify staging directory is removed
		if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
			t.Error("staging directory should be removed after cleanup")
		}
	})

	t.Run("removes staging directory with files", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}

		// Stage a file
		targetPath := filepath.Join(tmpDir, "test.txt")
		err = tx.StageFile(targetPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		stagingDir := tx.stagingDir

		tx.Cleanup()

		// Verify staging directory is removed even with files
		if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
			t.Error("staging directory should be removed after cleanup")
		}
	})

	t.Run("cleanup is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}

		// Call cleanup multiple times - should not panic
		tx.Cleanup()
		tx.Cleanup()
		tx.Cleanup()
	})
}

func TestImportTransaction_CommitRestoreOnFailure(t *testing.T) {
	t.Run("restores backups on commit failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create existing file
		existingPath := filepath.Join(tmpDir, "existing.txt")
		err := os.WriteFile(existingPath, []byte("original"), 0o644)
		if err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		tx, err := newImportTransaction(tmpDir)
		if err != nil {
			t.Fatalf("newImportTransaction() error = %v", err)
		}
		defer tx.Cleanup()

		// Stage replacement for existing file
		err = tx.StageFile(existingPath, []byte("replacement"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		// Stage file that will fail (target is a directory)
		failPath := filepath.Join(tmpDir, "willFail")
		err = os.Mkdir(failPath, 0o755)
		if err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		err = tx.StageFile(failPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("StageFile() error = %v", err)
		}

		// Commit should fail
		err = tx.Commit()
		if err == nil {
			t.Fatal("expected commit to fail")
		}

		// Verify original file is restored
		content, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read restored file: %v", err)
		}
		if string(content) != "original" {
			t.Errorf("restored content = %q, want %q", string(content), "original")
		}
	})
}

func TestStageFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tx, err := newImportTransaction(tmpDir)
	if err != nil {
		t.Fatalf("newImportTransaction: %v", err)
	}
	defer tx.Cleanup()

	target := filepath.Join(tmpDir, "test.txt")
	data := []byte("test content")

	err = tx.StageFile(target, data, 0600)
	if err != nil {
		t.Fatalf("StageFile: %v", err)
	}

	// Verify file was staged
	if _, ok := tx.staged[target]; !ok {
		t.Error("target should be in staged map")
	}
}

func TestStageFile_AlreadyCommitted(t *testing.T) {
	tmpDir := t.TempDir()
	tx, err := newImportTransaction(tmpDir)
	if err != nil {
		t.Fatalf("newImportTransaction: %v", err)
	}
	defer tx.Cleanup()

	// Mark as committed
	tx.committed = true

	target := filepath.Join(tmpDir, "test.txt")
	err = tx.StageFile(target, []byte("data"), 0600)
	if err == nil {
		t.Error("expected error for already committed transaction")
	}
	if err != nil && err.Error() != "transaction already committed" {
		t.Errorf("expected 'transaction already committed' error, got: %v", err)
	}
}

func TestStageFile_ReplacesExistingStaged(t *testing.T) {
	tmpDir := t.TempDir()
	tx, err := newImportTransaction(tmpDir)
	if err != nil {
		t.Fatalf("newImportTransaction: %v", err)
	}
	defer tx.Cleanup()

	target := filepath.Join(tmpDir, "test.txt")

	// Stage first version
	err = tx.StageFile(target, []byte("first"), 0600)
	if err != nil {
		t.Fatalf("first StageFile: %v", err)
	}
	firstStaged := tx.staged[target]

	// Stage second version (should replace)
	err = tx.StageFile(target, []byte("second"), 0600)
	if err != nil {
		t.Fatalf("second StageFile: %v", err)
	}
	secondStaged := tx.staged[target]

	// Verify different file was staged
	if firstStaged == secondStaged {
		t.Error("second staging should create new file")
	}

	// Verify first staged file was removed
	if _, err := os.Stat(firstStaged); !os.IsNotExist(err) {
		t.Error("first staged file should have been removed")
	}
}

func TestStageFile_EmptyBasename(t *testing.T) {
	tmpDir := t.TempDir()
	tx, err := newImportTransaction(tmpDir)
	if err != nil {
		t.Fatalf("newImportTransaction: %v", err)
	}
	defer tx.Cleanup()

	// Target with trailing slash gets empty basename
	target := tmpDir + string(os.PathSeparator)
	err = tx.StageFile(target, []byte("data"), 0600)
	if err != nil {
		t.Fatalf("StageFile with empty basename: %v", err)
	}
}
