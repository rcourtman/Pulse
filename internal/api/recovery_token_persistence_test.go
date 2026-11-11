package api

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise actual storage failures rather than replacing the save with a mock.
// The original authority file is restored byte-for-byte after each fault.
func blockRecoveryPersistence(t *testing.T, store *RecoveryTokenStore, stage string) func() {
	t.Helper()
	dir := store.dataPath
	path := filepath.Join(dir, "recovery_tokens.json")
	var restore func()
	switch stage {
	case "mkdir":
		backup := dir + ".saved"
		if err := os.Rename(dir, backup); err != nil {
			t.Fatal(err)
		}
		restore = func() {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				t.Error(err)
			}
			if err := os.Rename(backup, dir); err != nil {
				t.Error(err)
			}
		}
		if err := os.WriteFile(dir, []byte("directory unavailable"), 0600); err != nil {
			restore()
			t.Fatal(err)
		}
	case "write":
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("POSIX non-root permission fault")
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
		restore = func() {
			if err := os.Chmod(dir, info.Mode().Perm()); err != nil {
				t.Error(err)
			}
		}
	case "rename":
		backup := path + ".saved"
		if err := os.Rename(path, backup); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			_ = os.Rename(backup, path)
			t.Fatal(err)
		}
		restore = func() {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Error(err)
			}
		}
	default:
		t.Fatalf("unknown failure stage %q", stage)
	}
	var once sync.Once
	result := func() { once.Do(restore) }
	t.Cleanup(result)
	return result
}

func recoveryPersistenceBytes(t *testing.T, store *RecoveryTokenStore) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.dataPath, "recovery_tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func reloadRecoveryPersistence(store *RecoveryTokenStore) *RecoveryTokenStore {
	reloaded := &RecoveryTokenStore{tokens: make(map[string]*RecoveryToken), dataPath: store.dataPath}
	reloaded.load()
	return reloaded
}

func TestRecoveryTokenPersistenceIssueFailure(t *testing.T) {
	for _, stage := range []string{"mkdir", "write", "rename"} {
		t.Run(stage, func(t *testing.T) {
			store := newTestRecoveryStore(t)
			previous, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil {
				t.Fatal(err)
			}
			before := recoveryPersistenceBytes(t, store)
			restore := blockRecoveryPersistence(t, store, stage)
			issued, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err == nil || issued != "" {
				t.Fatal("issuance acknowledged failed persistence")
			}
			if len(store.tokens) != 1 || !store.IsRecoveryTokenValidConstantTime(previous, recoveryBindIP) {
				t.Fatal("failed issuance changed existing live authority or retained an unreturned token")
			}
			restore()
			if !bytes.Equal(before, recoveryPersistenceBytes(t, store)) {
				t.Fatal("failed issuance changed saved authority")
			}
			if !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(previous, recoveryBindIP) {
				t.Fatal("existing authority did not survive reload")
			}
			if _, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP); err != nil {
				t.Fatalf("issuance did not recover after storage repair: %v", err)
			}
		})
	}
}

func TestRecoveryTokenPersistenceConsumeFailure(t *testing.T) {
	for _, stage := range []string{"mkdir", "write", "rename"} {
		t.Run(stage, func(t *testing.T) {
			store := newTestRecoveryStore(t)
			token, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil {
				t.Fatal(err)
			}
			other, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil {
				t.Fatal(err)
			}
			before := recoveryPersistenceBytes(t, store)
			restore := blockRecoveryPersistence(t, store, stage)
			var accepted atomic.Int32
			var wg sync.WaitGroup
			for range 12 {
				wg.Go(func() {
					if store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
						accepted.Add(1)
					}
				})
			}
			wg.Wait()
			if accepted.Load() != 0 {
				t.Fatal("consumption authorized before successful persistence")
			}
			if !store.IsRecoveryTokenValidConstantTime(token, recoveryBindIP) || !store.IsRecoveryTokenValidConstantTime(other, recoveryBindIP) {
				t.Fatal("refused consumption changed live authority")
			}
			restore()
			if !bytes.Equal(before, recoveryPersistenceBytes(t, store)) {
				t.Fatal("refused consumption changed saved authority")
			}
			if !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(token, recoveryBindIP) {
				t.Fatal("never-authorized token should remain usable after storage repair")
			}
			if !store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
				t.Fatal("consumption did not recover after storage repair")
			}
			reloaded := reloadRecoveryPersistence(store)
			if reloaded.IsRecoveryTokenValidConstantTime(token, recoveryBindIP) || reloaded.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
				t.Fatal("accepted consumption was reusable after reload")
			}
			if !reloaded.IsRecoveryTokenValidConstantTime(other, recoveryBindIP) {
				t.Fatal("consumption removed unrelated authority")
			}
		})
	}
}

func TestRecoveryTokenPersistenceAcceptedUseSurvivesReload(t *testing.T) {
	store := newTestRecoveryStore(t)
	token, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil {
		t.Fatal(err)
	}
	if store.ValidateRecoveryTokenConstantTime(token, "10.0.0.2") {
		t.Fatal("wrong IP consumed token")
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 24 {
		wg.Go(func() {
			<-start
			if store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
				accepted.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d concurrent consumers, want 1", accepted.Load())
	}
	reloaded := reloadRecoveryPersistence(store)
	if reloaded.IsRecoveryTokenValidConstantTime(token, recoveryBindIP) || reloaded.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("accepted token became reusable after reload")
	}
	if !reloaded.tokens[recoveryTokenHash(token)].Used || reloaded.tokens[recoveryTokenHash(token)].UsedAt.IsZero() {
		t.Fatal("saved consumption lost its audit timestamp")
	}
}

func TestRecoveryTokenPersistenceEncodeFailure(t *testing.T) {
	store := newTestRecoveryStore(t)
	token, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil {
		t.Fatal(err)
	}
	before := recoveryPersistenceBytes(t, store)
	// time.Time refuses to encode a year outside JSON's supported range.
	store.tokens["invalid-date-fixture"] = &RecoveryToken{ExpiresAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}
	if raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP); err == nil || raw != "" {
		t.Fatal("issuance acknowledged failed encoding")
	}
	if store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("consumption authorized before successful encoding")
	}
	if len(store.tokens) != 2 || !store.IsRecoveryTokenValidConstantTime(token, recoveryBindIP) || !bytes.Equal(before, recoveryPersistenceBytes(t, store)) {
		t.Fatal("failed encoding changed live or saved authority")
	}
	delete(store.tokens, "invalid-date-fixture")
	if !store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("encoding repair did not restore useful consumption")
	}
}

func TestRecoveryTokenPersistencePrivatePublication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode and symlink fixture")
	}
	store := newTestRecoveryStore(t)
	target := filepath.Join(t.TempDir(), "unrelated")
	const sentinel = "unrelated content"
	if err := os.WriteFile(target, []byte(sentinel), 0644); err != nil {
		t.Fatal(err)
	}
	legacyTemp := filepath.Join(store.dataPath, "recovery_tokens.json.tmp")
	if err := os.Symlink(target, legacyTemp); err != nil {
		t.Fatal(err)
	}
	token, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.dataPath, "recovery_tokens.json")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if !store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("valid consumption failed")
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatal("published authority is not a private regular file")
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != sentinel {
		t.Fatal("publication followed a pre-existing temporary symlink")
	}
	if bytes.Contains(recoveryPersistenceBytes(t, store), []byte(token)) {
		t.Fatal("raw recovery secret was persisted")
	}
	restore := blockRecoveryPersistence(t, store, "rename")
	if _, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP); err == nil {
		t.Fatal("rename fault did not fail issuance")
	}
	restore()
	leftovers, err := filepath.Glob(filepath.Join(store.dataPath, ".recovery_tokens.*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("failed publication left an owned temporary file")
	}
	if _, err := os.Lstat(legacyTemp); err != nil {
		t.Fatal("publication removed an unowned temporary path")
	}
}

func TestRecoveryTokenPersistenceLegacyFailure(t *testing.T) {
	store := newTestRecoveryStore(t)
	const token = "synthetic-legacy-recovery-persistence-token"
	legacy := []byte(`{"` + token + `":{"created_at":"2026-01-01T00:00:00Z","expires_at":"2099-01-01T00:00:00Z","used":false,"ip":"` + recoveryBindIP + `"}}`)
	if err := os.WriteFile(filepath.Join(store.dataPath, "recovery_tokens.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	restore := blockRecoveryPersistence(t, store, "write")
	store.load()
	if !store.IsRecoveryTokenValidConstantTime(token, recoveryBindIP) {
		t.Fatal("failed rewrite lost the known legacy inventory")
	}
	if store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("legacy consumption authorized before its hashed rewrite succeeded")
	}
	if !bytes.Equal(legacy, recoveryPersistenceBytes(t, store)) {
		t.Fatal("failed legacy rewrite changed original bytes")
	}
	restore()
	if !store.ValidateRecoveryTokenConstantTime(token, recoveryBindIP) {
		t.Fatal("known legacy token did not recover after storage repair")
	}
	if bytes.Contains(recoveryPersistenceBytes(t, store), []byte(token)) || reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(token, recoveryBindIP) {
		t.Fatal("successful legacy use lost hashed, single-use persistence")
	}
}
