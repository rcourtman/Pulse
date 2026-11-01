package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// These paths and credentials belong only to each test's temporary directory.
// A stale predictable temp path must not be a write target, even during legacy
// migration, and must not lend its permissions to the new canonical record.
func TestPersistProtectsStaleTemporaryPaths(t *testing.T) {
	for _, migrate := range []bool{false, true} {
		for _, kind := range []string{"regular", "symlink", "hardlink", "directory"} {
			t.Run(fmt.Sprintf("migration_%t/%s", migrate, kind), func(t *testing.T) {
				root := t.TempDir()
				dir := filepath.Join(root, "data")
				path, err := Persist(dir, "initial-synthetic-setup-token", time.Unix(1700000000, 0))
				if err != nil {
					t.Fatal(err)
				}
				keyPath := filepath.Join(dir, ".encryption.key")
				keyBefore, err := os.ReadFile(keyPath)
				if err != nil {
					t.Fatal(err)
				}
				unrelated := filepath.Join(root, "unrelated-state")
				const untouched = "unrelated state must remain intact"
				if err := os.WriteFile(unrelated, []byte(untouched), 0o600); err != nil {
					t.Fatal(err)
				}
				stale := path + ".tmp"
				switch kind {
				case "regular":
					err = os.WriteFile(stale, []byte(untouched), 0o600)
					if err == nil {
						err = os.Chmod(stale, 0o666)
					}
				case "symlink":
					err = os.Symlink(unrelated, stale)
				case "hardlink":
					err = os.Link(unrelated, stale)
				case "directory":
					err = os.Mkdir(stale, 0o700)
				}
				if err != nil {
					if runtime.GOOS == "windows" && (kind == "symlink" || kind == "hardlink") {
						t.Skipf("test filesystem cannot create %s: %v", kind, err)
					}
					t.Fatal(err)
				}
				staleInfo, err := os.Lstat(stale)
				if err != nil {
					t.Fatal(err)
				}
				const token = "replacement-synthetic-setup-token"
				createdAt := time.Unix(1700000100, 0).UTC()
				if migrate {
					if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					loaded, gotPath, migrated, err := Load(dir)
					if err != nil || !migrated || gotPath != path || loaded != token {
						t.Fatalf("legacy migration did not preserve the setup token: migrated=%t path=%q error=%v", migrated, gotPath, err)
					}
				} else if gotPath, err := Persist(dir, token, createdAt); err != nil || gotPath != path {
					t.Fatalf("Persist() path=%q error=%v", gotPath, err)
				}

				if got, err := os.ReadFile(unrelated); err != nil || string(got) != untouched {
					t.Errorf("persistence changed unrelated state: error=%v", err)
				}
				if got, err := os.Lstat(stale); err != nil {
					t.Errorf("persistence removed the stale %s path: %v", kind, err)
				} else if !os.SameFile(staleInfo, got) || got.Mode() != staleInfo.Mode() {
					t.Errorf("persistence replaced or changed permissions of the stale %s path", kind)
				}
				if kind != "directory" {
					if got, err := os.ReadFile(stale); err != nil || string(got) != untouched {
						t.Errorf("persistence changed the stale %s contents: error=%v", kind, err)
					}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(data, []byte(token)) {
					t.Error("canonical record contains the plaintext setup token")
				}
				var record persistedToken
				if err := json.Unmarshal(data, &record); err != nil || record.Version != persistedFormatVersion || record.TokenCiphertext == "" || record.TokenHash == "" {
					t.Fatalf("canonical encrypted record is incomplete: %v", err)
				}
				if !migrate && !record.CreatedAt.Equal(createdAt) {
					t.Error("persistence changed the supplied creation time")
				}
				if got, gotPath, migrated, err := Load(dir); err != nil || got != token || gotPath != path || migrated {
					t.Errorf("encrypted record did not round-trip: migrated=%t path=%q error=%v", migrated, gotPath, err)
				}
				if runtime.GOOS != "windows" {
					if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
						t.Errorf("canonical record is not owner-only: error=%v", err)
					}
				}
				if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, keyBefore) {
					t.Errorf("persistence changed the encryption key: error=%v", err)
				}
				assertNoBootstrapTemporaryFiles(t, dir)
			})
		}
	}
}

func TestPersistFailedReplacementPreservesExistingState(t *testing.T) {
	dir := t.TempDir()
	path, err := Persist(dir, "initial-synthetic-setup-token", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	keyBefore, err := os.ReadFile(filepath.Join(dir, ".encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "preserved-state")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	gotPath, err := Persist(dir, "replacement-synthetic-setup-token", time.Now())
	if err == nil || gotPath != path {
		t.Fatalf("replacement of a directory must fail with its resolved path: path=%q error=%v", gotPath, err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Errorf("failed replacement changed existing state: error=%v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, ".encryption.key")); err != nil || !bytes.Equal(got, keyBefore) {
		t.Errorf("failed replacement changed the encryption key: error=%v", err)
	}
	assertNoBootstrapTemporaryFiles(t, dir)
}

func TestPersistConcurrentWritesUseIndependentTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Persist(dir, "initial-synthetic-setup-token", time.Now()); err != nil {
		t.Fatal(err)
	}
	// Seed the existing key before concurrent writes. Key creation and
	// cross-process first-run coordination are not this writer's contract.
	const writers = 12
	errors := make(chan error, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := Persist(dir, fmt.Sprintf("concurrent-synthetic-setup-token-%02d", i), time.Now())
			errors <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent persistence failed: %v", err)
		}
	}
	token, _, migrated, err := Load(dir)
	if err != nil || migrated {
		t.Fatalf("final encrypted record cannot be loaded: migrated=%t error=%v", migrated, err)
	}
	valid := false
	for i := 0; i < writers; i++ {
		valid = valid || token == fmt.Sprintf("concurrent-synthetic-setup-token-%02d", i)
	}
	if !valid {
		t.Error("final record is not one complete successful write")
	}
	assertNoBootstrapTemporaryFiles(t, dir)
}

func assertNoBootstrapTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "."+TokenFilename+".*.tmp"))
	if err != nil || len(paths) != 0 {
		t.Errorf("private temporary files remain after persistence: count=%d error=%v", len(paths), err)
	}
}
