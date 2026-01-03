package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportTransaction_StageFile_Branches(t *testing.T) {
	tempDir := t.TempDir()
	tx, err := newImportTransaction(tempDir)
	require.NoError(t, err)
	defer tx.Cleanup()

	// 1. tx.committed
	tx.committed = true
	err = tx.StageFile("test", []byte("data"), 0644)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already committed")
	tx.committed = false

	// 2. tx.staged[target] existing (re-staging)
	err = tx.StageFile("test", []byte("data1"), 0644)
	assert.NoError(t, err)
	staged1 := tx.staged["test"]

	err = tx.StageFile("test", []byte("data2"), 0644)
	assert.NoError(t, err)
	staged2 := tx.staged["test"]
	assert.NotEqual(t, staged1, staged2)
	assert.NoFileExists(t, staged1)

	// 3. base == "" or separator
	err = tx.StageFile("/", []byte("data"), 0644)
	assert.NoError(t, err)
	assert.Contains(t, tx.staged["/"], "staged")

	// 4. prefix manipulations
	// Trigger strings.ReplaceAll and suffix .tmp-*
	err = tx.StageFile("no-star", []byte("data"), 0644)
	assert.NoError(t, err)

	// 5. MkdirAll error (make a file where stagingDir should be)
	// Actually stagingDir is already created by newImportTransaction.
	// But we can try to make it unreachable?
	// If we remove stagingDir and make it a file...
	os.RemoveAll(tx.stagingDir)
	os.WriteFile(tx.stagingDir, []byte("blocker"), 0644)
	err = tx.StageFile("blocked", []byte("data"), 0644)
	assert.Error(t, err)
}

func TestImportTransaction_Commit_Branches(t *testing.T) {
	tempDir := t.TempDir()

	// Setup a transaction
	tx, err := newImportTransaction(tempDir)
	require.NoError(t, err)

	targetFile := filepath.Join(tempDir, "target.txt")
	require.NoError(t, tx.StageFile(targetFile, []byte("new-data"), 0644))

	// 1. tx.committed
	tx.committed = true
	err = tx.Commit()
	assert.Error(t, err)
	tx.committed = false

	// 2. Destination is a directory
	targetDir := filepath.Join(tempDir, "is-a-dir")
	require.NoError(t, os.Mkdir(targetDir, 0755))
	require.NoError(t, tx.StageFile(targetDir, []byte("data"), 0644))

	err = tx.Commit()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

func TestImportTransaction_Rollback_Branches(t *testing.T) {
	tempDir := t.TempDir()
	tx, err := newImportTransaction(tempDir)
	require.NoError(t, err)

	targetFile := filepath.Join(tempDir, "roll.txt")
	os.WriteFile(targetFile, []byte("old"), 0644)

	tx.StageFile(targetFile, []byte("new"), 0644)

	// Mock a backup for rollback coverage
	backupFile := targetFile + ".bak"
	os.WriteFile(backupFile, []byte("backup"), 0644)
	tx.backups[targetFile] = backupFile

	tx.Rollback()

	// Verify restore
	data, _ := os.ReadFile(targetFile)
	assert.Equal(t, "backup", string(data))
	assert.NoFileExists(t, backupFile)
	tx.Cleanup()
}
