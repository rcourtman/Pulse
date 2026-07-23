package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests in this file use the TestBranchcov0723pm prefix so the scoped run
//
//	go test ./internal/config/ -run '^TestBranchcov0723pm' -count=1
//
// selects only them. They raise branch coverage for two previously-uncovered
// filesystem functions in migration.go:
//   - copyFile              (migration.go:153)
//   - RunMigrationIfNeeded  (migration.go:202)
//
// Every case is isolated under t.TempDir(); nothing escapes the test process.
// Both targets are pure filesystem functions (no network, SSH, daemon or live
// database), so neither is skipped on purity grounds.

// TestBranchcov0723pm_CopyFile exercises every return path of copyFile.
func TestBranchcov0723pm_CopyFile(t *testing.T) {
	t.Run("missing source returns read error and creates no destination", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "does-not-exist")
		dst := filepath.Join(dir, "dst")

		err := copyFile(missing, dst)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read source file",
			"missing source must surface the read-error branch")
		// The destination must not be created when the copy aborts at read time.
		_, statErr := os.Stat(dst)
		require.True(t, os.IsNotExist(statErr),
			"destination must not exist after a failed source read")
	})

	t.Run("destination parent missing returns write error", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))
		// dst sits under a directory that does not exist; os.WriteFile does not
		// create parent directories, so this deterministically hits the
		// write-error branch of copyFile.
		dst := filepath.Join(dir, "no-such-parent", "dst")

		err := copyFile(src, dst)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "write destination file",
			"uncreatable destination must surface the write-error branch")
	})

	t.Run("success copies bytes and creates a previously-absent destination", func(t *testing.T) {
		dir := t.TempDir()
		want := []byte("line one\nline two\n")
		src := filepath.Join(dir, "src")
		require.NoError(t, os.WriteFile(src, want, 0o600))
		dst := filepath.Join(dir, "dst")

		// Precondition: prove the destination does not yet exist, so a passing
		// copy genuinely *creates* it rather than copying over something that
		// was already there.
		_, preStatErr := os.Stat(dst)
		require.True(t, os.IsNotExist(preStatErr),
			"precondition: destination must not exist before copyFile runs")

		require.NoError(t, copyFile(src, dst))

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, want, got, "copied bytes must be byte-identical to the source")
	})

	t.Run("success propagates the source file's permission bits", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		// Owner-only mode is deliberately distinct from a typical 0o644 default,
		// so a copyFile that hardcoded a mode instead of stat-ing the source
		// would fail this assertion. (Umask is absorbed by reading src's actual
		// on-disk mode rather than asserting a literal.)
		require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))
		srcInfo, err := os.Stat(src)
		require.NoError(t, err)
		wantPerm := srcInfo.Mode().Perm()

		dst := filepath.Join(dir, "dst")
		require.NoError(t, copyFile(src, dst))

		dstInfo, err := os.Stat(dst)
		require.NoError(t, err)
		assert.Equal(t, wantPerm, dstInfo.Mode().Perm(),
			"copyFile must write the destination using the source's permission mode")
	})

	t.Run("existing destination is overwritten with source bytes", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src")
		require.NoError(t, os.WriteFile(src, []byte("new contents"), 0o600))
		dst := filepath.Join(dir, "dst")
		// Pre-existing destination with distinct contents that must be replaced.
		require.NoError(t, os.WriteFile(dst, []byte("old contents"), 0o600))

		require.NoError(t, copyFile(src, dst))

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "new contents", string(got),
			"existing destination must be overwritten with the source bytes")
	})
}

// TestBranchcov0723pm_RunMigrationIfNeeded exercises every return path of
// RunMigrationIfNeeded: the no-op short-circuit (already migrated, nothing to
// migrate, and empty data dir), a successful real migration, and the
// migration-error wrapping branch.
func TestBranchcov0723pm_RunMigrationIfNeeded(t *testing.T) {
	t.Run("already migrated short-circuits and leaves legacy files untouched", func(t *testing.T) {
		dataDir := t.TempDir()

		// Seed a legacy file at the data root so migration *would* be needed if
		// the marker were absent.
		legacyPath := filepath.Join(dataDir, "system.json")
		require.NoError(t, os.WriteFile(legacyPath, []byte("legacy-system"), 0o600))

		// Plant the migration marker; its presence makes IsMigrationNeeded
		// return false, so RunMigrationIfNeeded must short-circuit.
		defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
		require.NoError(t, os.MkdirAll(defaultOrgDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(defaultOrgDir, ".migrated"), []byte("done"), 0o600))

		require.NoError(t, RunMigrationIfNeeded(dataDir),
			"already-migrated data dir must short-circuit with nil error")

		// The short-circuit must not have run the migration: the legacy file is
		// still a regular file (not a symlink) at the data root with its
		// original bytes...
		info, err := os.Lstat(legacyPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0), info.Mode()&os.ModeSymlink,
			"short-circuit must not turn the legacy file into a symlink")
		got, err := os.ReadFile(legacyPath)
		require.NoError(t, err)
		assert.Equal(t, "legacy-system", string(got),
			"short-circuit must not alter legacy file contents")
		// ...and the file must not have been copied into the default org dir.
		_, err = os.Stat(filepath.Join(defaultOrgDir, "system.json"))
		require.True(t, os.IsNotExist(err),
			"short-circuit must not migrate the file into the default org dir")
	})

	t.Run("nothing to migrate on empty data dir returns nil", func(t *testing.T) {
		dataDir := t.TempDir()

		require.NoError(t, RunMigrationIfNeeded(dataDir),
			"data dir with no legacy files and no marker must return nil")

		// A no-op run must not materialize the orgs tree at all.
		_, err := os.Stat(filepath.Join(dataDir, "orgs"))
		require.True(t, os.IsNotExist(err),
			"nothing-to-migrate must not create the orgs directory")
	})

	t.Run("empty data dir string returns nil", func(t *testing.T) {
		// IsMigrationNeeded returns false for an empty string, so the very first
		// guard in RunMigrationIfNeeded must short-circuit before any I/O.
		require.NoError(t, RunMigrationIfNeeded(""),
			"empty data dir must short-circuit with nil error")
	})

	t.Run("real migration moves files, symlinks originals, and writes marker", func(t *testing.T) {
		dataDir := t.TempDir()

		// Seed every file listed in filesToMigrate with distinctive contents so
		// the resulting on-disk layout can be asserted precisely.
		contents := map[string]string{
			"nodes.enc":          "enc-bytes",
			"system.json":        "{\"k\":\"v\"}",
			"alerts.json":        "[]",
			"notifications.json": "{}",
			"audit.db":           "SQLITE-HEADER",
		}
		for _, name := range filesToMigrate {
			require.NoError(t, os.WriteFile(filepath.Join(dataDir, name), []byte(contents[name]), 0o600))
		}

		require.NoError(t, RunMigrationIfNeeded(dataDir),
			"seeded data dir must migrate without error")

		defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
		for _, name := range filesToMigrate {
			// File relocated into the default org dir with byte-identical contents.
			moved := filepath.Join(defaultOrgDir, name)
			got, err := os.ReadFile(moved)
			require.NoError(t, err, "file %s should exist in default org dir", name)
			assert.Equal(t, contents[name], string(got),
				"migrated file %s must keep its original bytes", name)

			// Original location is now a backward-compat symlink that resolves
			// back to the relocated bytes.
			linkInfo, err := os.Lstat(filepath.Join(dataDir, name))
			require.NoError(t, err, "symlink %s should remain at data root", name)
			assert.Equal(t, os.ModeSymlink, linkInfo.Mode()&os.ModeSymlink,
				"%s should be a symlink after migration", name)
			viaLink, err := os.ReadFile(filepath.Join(dataDir, name))
			require.NoError(t, err)
			assert.Equal(t, contents[name], string(viaLink),
				"backward-compat symlink for %s must resolve to the moved file", name)
		}

		// Marker must exist, and re-checking must now report no migration needed.
		_, err := os.Stat(filepath.Join(defaultOrgDir, ".migrated"))
		require.NoError(t, err, "migration marker must be written")
		assert.False(t, IsMigrationNeeded(dataDir),
			"after a successful migration the data dir must no longer need migrating")
	})

	t.Run("migration failure is wrapped with run-migration context", func(t *testing.T) {
		dataDir := t.TempDir()

		// Seed a legacy file so IsMigrationNeeded returns true and execution
		// reaches MigrateToMultiTenant.
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "system.json"), []byte("{}"), 0o600))

		// Make "orgs" a regular file rather than a directory, so MigrateToMultiTenant's
		// os.MkdirAll(dataDir/orgs/default) fails with a real filesystem error
		// (a path component is not a directory). This is the only deterministic,
		// non-mock way to drive the error arm of RunMigrationIfNeeded.
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "orgs"), []byte("not a dir"), 0o600))

		err := RunMigrationIfNeeded(dataDir)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run multi-tenant migration",
			"RunMigrationIfNeeded must wrap the underlying migration error with its own context")
	})
}
