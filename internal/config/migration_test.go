package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateToMultiTenant(t *testing.T) {
	// Setup temp dir
	dataDir := t.TempDir()

	// Create dummy legacy files
	legacyFiles := []string{"nodes.enc", "system.json", "alerts.json"}
	for _, f := range legacyFiles {
		err := os.WriteFile(filepath.Join(dataDir, f), []byte("dummy content"), 0644)
		require.NoError(t, err)
	}

	// Run migration
	err := MigrateToMultiTenant(dataDir)
	require.NoError(t, err)

	// Verify files moved to default org
	defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
	for _, f := range legacyFiles {
		// Check file exists in new location
		content, err := os.ReadFile(filepath.Join(defaultOrgDir, f))
		require.NoError(t, err, "File %s should exist in default org dir", f)
		require.Equal(t, "dummy content", string(content))

		// Check symlink exists in old location
		info, err := os.Lstat(filepath.Join(dataDir, f))
		require.NoError(t, err, "Symlink %s should exist in root data dir", f)
		require.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "%s should be a symlink", f)
	}

	// Verify marker
	_, err = os.Stat(filepath.Join(defaultOrgDir, ".migrated"))
	require.NoError(t, err, "Migration marker should exist")

	// Run again (should be idempotent)
	err = MigrateToMultiTenant(dataDir)
	require.NoError(t, err)
}

func TestIsMigrationNeeded(t *testing.T) {
	dataDir := t.TempDir()

	// Empty dir - no migration needed
	require.False(t, IsMigrationNeeded(dataDir))

	// With files - migration needed
	os.WriteFile(filepath.Join(dataDir, "system.json"), []byte("{}"), 0644)
	require.True(t, IsMigrationNeeded(dataDir))

	// After migration - not needed
	MigrateToMultiTenant(dataDir)
	require.False(t, IsMigrationNeeded(dataDir))
}
