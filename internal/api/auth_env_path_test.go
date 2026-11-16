package api

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

const authEnvOldFixture = "# synthetic saved configuration\nPULSE_AUTH_USER='operator'\nPULSE_AUTH_PASS='old-fixture-hash'\nPULSE_PORT='7654'\n"
const authEnvNextFixture = "# synthetic saved configuration\nPULSE_AUTH_USER='operator'\nPULSE_AUTH_PASS='next-fixture-hash'\nPULSE_PORT='7654'\n"

func checkAuthEnvPublication(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("published environment differs from the complete selected bytes", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("published environment is not a regular file", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("published environment mode = %o, want 600", info.Mode().Perm())
	}
	checkNoOwnedAuthTemps(t, filepath.Dir(path), filepath.Base(path))
}

func TestAuthEnvPublicationPrivateFiles(t *testing.T) {
	for _, original := range []string{"absent", "private", "legacy-wide"} {
		t.Run(original, func(t *testing.T) {
			dir, fallback := t.TempDir(), t.TempDir()
			path := filepath.Join(dir, ".env")
			if original != "absent" {
				if err := os.WriteFile(path, []byte(authEnvOldFixture), 0600); err != nil {
					t.Fatal(err)
				}
				if original == "legacy-wide" {
					if err := os.Chmod(path, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			fallbackPath := filepath.Join(fallback, ".env")
			if err := os.WriteFile(fallbackPath, []byte(authEnvOldFixture), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := writeAuthEnvFile(dir, fallback, []byte(authEnvNextFixture))
			if err != nil || got != path {
				t.Fatal("preferred environment publication failed", err)
			}
			checkAuthEnvPublication(t, path, []byte(authEnvNextFixture))
			checkAuthEnvPublication(t, fallbackPath, []byte(authEnvOldFixture))
		})
	}
}

func TestAuthEnvPublicationKeepsOpenReaders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX replacement while a previous descriptor is open")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(authEnvOldFixture), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := writeAuthEnvFile(dir, dir, []byte(authEnvNextFixture)); err != nil {
		t.Fatal(err)
	}
	old, err := io.ReadAll(reader)
	if err != nil || string(old) != authEnvOldFixture {
		t.Fatal("save rewrote an already-open configuration instead of publishing a complete replacement", err)
	}
	checkAuthEnvPublication(t, path, []byte(authEnvNextFixture))
}

func TestAuthEnvPublicationKeepsLinkedTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory and file link fixtures")
	}
	for _, link := range []string{"symlink", "hardlink"} {
		t.Run(link, func(t *testing.T) {
			dir := t.TempDir()
			configured := filepath.Join(t.TempDir(), "linked-data")
			if err := os.Symlink(dir, configured); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "old-environment")
			if err := os.WriteFile(target, []byte(authEnvOldFixture), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(configured, ".env")
			linkFile := os.Symlink
			if link == "hardlink" {
				linkFile = os.Link
			}
			if err := linkFile(target, path); err != nil {
				t.Fatal(err)
			}
			if got, err := writeAuthEnvFile(configured, configured, []byte(authEnvNextFixture)); err != nil || got != path {
				t.Fatal("publication in linked data directory failed", err)
			}
			checkAuthEnvPublication(t, path, []byte(authEnvNextFixture))
			checkAuthEnvPublication(t, target, []byte(authEnvOldFixture))
			if got, err := os.Readlink(configured); err != nil || got != dir {
				t.Fatal("publication changed the configured directory link", err)
			}
		})
	}
}

func TestAuthEnvPublicationFailureAndFallback(t *testing.T) {
	for _, fault := range []string{"directory", "create", "rename"} {
		t.Run(fault, func(t *testing.T) {
			if fault == "create" && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("ordinary-user POSIX directory permissions")
			}
			dir, fallback := t.TempDir(), t.TempDir()
			path := filepath.Join(dir, ".env")
			switch fault {
			case "directory":
				dir = filepath.Join(dir, "blocked-directory")
				if err := os.WriteFile(dir, []byte("directory blocker"), 0600); err != nil {
					t.Fatal(err)
				}
			case "create":
				if err := os.WriteFile(path, []byte(authEnvOldFixture), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(dir, 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			case "rename":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			got, err := writeAuthEnvFile(dir, fallback, []byte(authEnvNextFixture))
			if err != nil || got != filepath.Join(fallback, ".env") {
				t.Fatal("failed preferred publication did not reach its data-path fallback", err)
			}
			checkAuthEnvPublication(t, got, []byte(authEnvNextFixture))
			if fault == "directory" {
				if got, err := os.ReadFile(dir); err != nil || string(got) != "directory blocker" {
					t.Fatal("save changed its directory blocker", err)
				}
			} else {
				checkNoOwnedAuthTemps(t, dir, ".env")
				if fault == "create" {
					checkAuthEnvPublication(t, path, []byte(authEnvOldFixture))
				} else if info, err := os.Stat(path); err != nil || !info.IsDir() {
					t.Fatal("save changed its rename blocker", err)
				}
			}
		})
	}
}

func TestAuthEnvPublicationAllDestinationsFailThenRecover(t *testing.T) {
	dirs := []string{t.TempDir(), t.TempDir()}
	for _, dir := range dirs {
		path := filepath.Join(dir, ".env")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if path, err := writeAuthEnvFile(dirs[0], dirs[1], []byte(authEnvNextFixture)); err == nil || path != "" {
		t.Fatal("save acknowledged failed publication")
	}
	for _, dir := range dirs {
		checkNoOwnedAuthTemps(t, dir, ".env")
		if err := os.Remove(filepath.Join(dir, ".env")); err != nil {
			t.Fatal(err)
		}
	}
	got, err := writeAuthEnvFile(dirs[0], dirs[1], []byte(authEnvNextFixture))
	if err != nil || got != filepath.Join(dirs[0], ".env") {
		t.Fatal("publication did not recover on the preferred path", err)
	}
	checkAuthEnvPublication(t, got, []byte(authEnvNextFixture))
	if _, err := os.Stat(filepath.Join(dirs[1], ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("recovered preferred publication also wrote the fallback", err)
	}
}

func TestAuthEnvPublicationRollbackPrivateExactBytes(t *testing.T) {
	dir, fallback := t.TempDir(), t.TempDir()
	for _, base := range []string{dir, fallback} {
		path := filepath.Join(base, ".env")
		if err := os.WriteFile(path, []byte(authEnvOldFixture), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := snapshotAuthEnvFiles(dir, fallback)
	if err != nil || len(snapshots) != 2 {
		t.Fatal("snapshot failed", err)
	}
	if err := removeAuthEnvFiles(dir, fallback); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{dir, fallback} {
		// Even an existing wider file must not preserve its mode on rollback.
		if err := os.WriteFile(filepath.Join(base, ".env"), []byte(authEnvNextFixture), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(base, ".env"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := restoreAuthEnvFiles(snapshots); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{dir, fallback} {
		checkAuthEnvPublication(t, filepath.Join(base, ".env"), []byte(authEnvOldFixture))
	}
}

func TestAuthEnvPublicationRollbackPreservesAbsenceAndContinuesAfterFailure(t *testing.T) {
	dir, fallback := t.TempDir(), t.TempDir()
	primary, secondary := filepath.Join(dir, ".env"), filepath.Join(fallback, ".env")
	if err := os.WriteFile(secondary, []byte(authEnvOldFixture), 0600); err != nil {
		t.Fatal(err)
	}
	snapshots, err := snapshotAuthEnvFiles(dir, fallback)
	if err != nil || len(snapshots) != 2 {
		t.Fatal("snapshot failed", err)
	}
	if err := os.WriteFile(primary, []byte(authEnvNextFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreAuthEnvFiles(snapshots); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(primary); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rollback did not restore original absence", err)
	}
	// A failed earlier restoration must be reported without skipping another
	// saved inventory. No unsafe in-place write escapes a rename refusal.
	if err := os.Mkdir(primary, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondary, []byte(authEnvNextFixture), 0600); err != nil {
		t.Fatal(err)
	}
	snapshots[0].exists = true
	snapshots[0].data = []byte(authEnvOldFixture)
	if err := restoreAuthEnvFiles(snapshots); err == nil {
		t.Fatal("rollback hid an earlier failed publication")
	}
	checkNoOwnedAuthTemps(t, dir, ".env")
	if info, err := os.Stat(primary); err != nil || !info.IsDir() {
		t.Fatal("failed rollback changed its blocker", err)
	}
	checkAuthEnvPublication(t, secondary, []byte(authEnvOldFixture))
}

func TestAuthEnvPublicationConcurrentCompleteReaders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX concurrent open-reader replacement")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	versions := [][]byte{bytes.Repeat([]byte("synthetic-a\n"), 8192), bytes.Repeat([]byte("synthetic-b\n"), 8192)}
	if _, err := writeAuthEnvFile(dir, dir, versions[0]); err != nil {
		t.Fatal(err)
	}
	var writers sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, 2)
	for _, version := range versions {
		writers.Go(func() {
			<-start
			for i := 0; i < 32; i++ {
				if _, err := writeAuthEnvFile(dir, dir, version); err != nil {
					failures <- err
					return
				}
			}
		})
	}
	close(start)
	for i := 0; i < 128; i++ {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, versions[0]) && !bytes.Equal(got, versions[1]) {
			t.Error("reader observed a missing, partial or mixed configuration", err)
			break
		}
	}
	writers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	checkNoOwnedAuthTemps(t, dir, ".env")
}

func TestAuthEnvPublicationCanonicalDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	paths := resolveAuthEnvWritePaths("", "")
	if len(paths) != 1 || paths[0] != filepath.Join(dir, ".env") {
		t.Fatal("default configuration/data path was not canonical and deduplicated")
	}
	got, err := writeAuthEnvFile("", "", []byte(authEnvNextFixture))
	if err != nil || got != paths[0] {
		t.Fatal("publication did not use the canonical runtime data directory", err)
	}
	checkAuthEnvPublication(t, got, []byte(authEnvNextFixture))
}
