package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func recoveryUnreadInventory(t *testing.T, data []byte) *RecoveryTokenStore {
	t.Helper()
	store := newTestRecoveryStore(t)
	if err := os.WriteFile(filepath.Join(store.dataPath, "recovery_tokens.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	store.load()
	return store
}

func recoveryPreservedInventory(t *testing.T, store *RecoveryTokenStore, want []byte) {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(store.dataPath, ".recovery-tokens-unread-*"))
	if err != nil || len(dirs) != 1 {
		t.Fatalf("expected one preserved unread inventory, got %d: %v", len(dirs), err)
	}
	info, err := os.Lstat(dirs[0])
	if err != nil || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0700) {
		t.Fatal("unread inventory does not have its own private directory")
	}
	entries, err := os.ReadDir(dirs[0])
	if err != nil || len(entries) != 1 || entries[0].Name() != "recovery_tokens.json" {
		t.Fatal("preservation directory contains unexpected entries")
	}
	path := filepath.Join(dirs[0], "recovery_tokens.json")
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatal("preserved inventory is not a private regular file")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("unread inventory bytes were lost or changed")
	}
}

func recoveryNoPreservedInventory(t *testing.T, store *RecoveryTokenStore) {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(store.dataPath, ".recovery-tokens-unread-*"))
	if err != nil || len(dirs) != 0 {
		t.Fatal("known or absent inventory was unnecessarily preserved")
	}
}

func TestRecoveryUnreadDecodePreservesBytes(t *testing.T) {
	for name, data := range map[string][]byte{
		"truncated":         []byte(`[{"token_hash":"synthetic-incomplete`),
		"unsupported_shape": []byte(`"synthetic unsupported inventory"`),
		"invalid_legacy":    []byte(`{"synthetic-legacy":42}`),
	} {
		t.Run(name, func(t *testing.T) {
			store := recoveryUnreadInventory(t, data)
			if len(store.tokens) != 0 {
				t.Fatal("failed decode granted partial token authority")
			}
			store.cleanup()
			if !bytes.Equal(recoveryPersistenceBytes(t, store), data) {
				t.Fatal("no-op cleanup overwrote the unread inventory")
			}
			raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil || raw == "" {
				t.Fatalf("fresh issuance could not preserve a regular unread file: %v", err)
			}
			recoveryPreservedInventory(t, store, data)
			if !store.IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) || !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) {
				t.Fatal("fresh issuance was not published as the only known inventory")
			}
			if !store.ValidateRecoveryTokenConstantTime(raw, recoveryBindIP) || reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) {
				t.Fatal("fresh authority lost durable single-use consumption")
			}
			recoveryPreservedInventory(t, store, data)
		})
	}
}

func TestRecoveryUnreadPermissionDoesNotRestoreDormantAuthority(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary-user POSIX file permissions")
	}
	original := newTestRecoveryStore(t)
	unused, err := original.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil {
		t.Fatal(err)
	}
	used, err := original.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil || !original.ValidateRecoveryTokenConstantTime(used, recoveryBindIP) {
		t.Fatal("could not prepare the saved consumed marker")
	}
	before := recoveryPersistenceBytes(t, original)
	path := filepath.Join(original.dataPath, "recovery_tokens.json")
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	store := reloadRecoveryPersistence(original)
	if len(store.tokens) != 0 || store.IsRecoveryTokenValidConstantTime(unused, recoveryBindIP) || store.ValidateRecoveryTokenConstantTime(used, recoveryBindIP) {
		t.Fatal("failed startup read admitted unknown authority")
	}
	// Repair readability without restarting or reloading stale authority. Even a
	// permissive legacy mode must become private in the retained directory.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	fresh, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPreservedInventory(t, store, before)
	loaded := reloadRecoveryPersistence(store)
	if len(loaded.tokens) != 1 || !loaded.IsRecoveryTokenValidConstantTime(fresh, recoveryBindIP) || loaded.IsRecoveryTokenValidConstantTime(unused, recoveryBindIP) || loaded.ValidateRecoveryTokenConstantTime(used, recoveryBindIP) {
		t.Fatal("restart restored dormant/consumed authority or lost the fresh token")
	}
}

func TestRecoveryUnreadPreservationFailureRecovers(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary-user POSIX directory permissions")
	}
	before := []byte(`{"synthetic-broken`)
	store := recoveryUnreadInventory(t, before)
	t.Cleanup(func() { _ = os.Chmod(store.dataPath, 0700) })
	if err := os.Chmod(store.dataPath, 0500); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP); err == nil || raw != "" {
			t.Fatal("issuance acknowledged failed preservation")
		}
		if len(store.tokens) != 0 || !bytes.Equal(recoveryPersistenceBytes(t, store), before) {
			t.Fatal("failed preservation changed saved bytes or retained an unreturned token")
		}
		recoveryNoPreservedInventory(t, store)
	}
	if err := os.Chmod(store.dataPath, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil || !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) {
		t.Fatal("deliberate issuance did not recover after storage repair")
	}
	recoveryPreservedInventory(t, store, before)
}

func TestRecoveryUnreadNonRegularPathStaysUntouched(t *testing.T) {
	for _, kind := range []string{"directory", "symlink", "dangling_symlink"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" && kind != "directory" {
				t.Skip("requires unprivileged symlink creation")
			}
			store := newTestRecoveryStore(t)
			path := filepath.Join(store.dataPath, "recovery_tokens.json")
			target := filepath.Join(store.dataPath, "unowned-target")
			before := []byte("synthetic unread target")
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(path, "marker")
			} else if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if kind != "dangling_symlink" {
				if err := os.WriteFile(target, before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			original, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			store.load()
			if raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP); err == nil || raw != "" {
				t.Fatal("issuance replaced an unread nonregular path")
			}
			current, err := os.Lstat(path)
			if err != nil || !os.SameFile(original, current) || len(store.tokens) != 0 {
				t.Fatal("failed preservation changed the original path or live authority")
			}
			if kind == "dangling_symlink" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatal("issuance created the missing symlink target")
				}
			} else if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, before) {
				t.Fatal("preservation changed a nonregular path's contents or target")
			}
			recoveryNoPreservedInventory(t, store)
		})
	}
}

func TestRecoveryUnreadRemovedFileAllowsFreshInventory(t *testing.T) {
	store := recoveryUnreadInventory(t, []byte("synthetic broken inventory"))
	if err := os.Remove(filepath.Join(store.dataPath, "recovery_tokens.json")); err != nil {
		t.Fatal(err)
	}
	raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
	if err != nil || !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) {
		t.Fatal("deliberately removed unread file still held new issuance")
	}
	recoveryNoPreservedInventory(t, store)
}

func TestRecoveryUnreadConcurrentIssuancePreservesOnce(t *testing.T) {
	before := []byte("synthetic broken inventory")
	store := recoveryUnreadInventory(t, before)
	const count = 12
	start := make(chan struct{})
	results := make(chan string, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			<-start
			raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil {
				t.Error("concurrent issuance failed", err)
			}
			results <- raw
		})
	}
	close(start)
	wg.Wait()
	close(results)
	loaded := reloadRecoveryPersistence(store)
	if len(store.tokens) != count || len(loaded.tokens) != count {
		t.Fatal("concurrent publication lost returned authority")
	}
	for raw := range results {
		if raw == "" || !loaded.IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) {
			t.Fatal("a returned concurrent token did not survive reload")
		}
	}
	recoveryPreservedInventory(t, store, before)
}

func TestRecoveryUnreadKnownOrAbsentInventoriesStayUsable(t *testing.T) {
	for _, kind := range []string{"absent", "null", "empty_array", "canonical", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			store := newTestRecoveryStore(t)
			var previous string
			switch kind {
			case "null", "empty_array":
				data := []byte("null")
				if kind == "empty_array" {
					data = []byte("[]")
				}
				if err := os.WriteFile(filepath.Join(store.dataPath, "recovery_tokens.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "canonical":
				var err error
				previous, err = store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
				if err != nil {
					t.Fatal(err)
				}
			case "legacy":
				previous = "synthetic-legacy-token"
				data, err := json.Marshal(map[string]*legacyRecoveryToken{previous: {
					Token: previous, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), IP: recoveryBindIP,
				}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store.dataPath, "recovery_tokens.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			store = reloadRecoveryPersistence(store)
			raw, err := store.GenerateRecoveryToken(time.Hour, recoveryBindIP)
			if err != nil {
				t.Fatal(err)
			}
			loaded := reloadRecoveryPersistence(store)
			if !loaded.IsRecoveryTokenValidConstantTime(raw, recoveryBindIP) || (previous != "" && !loaded.IsRecoveryTokenValidConstantTime(previous, recoveryBindIP)) {
				t.Fatal("known inventory lost existing or fresh authority")
			}
			recoveryNoPreservedInventory(t, store)
		})
	}
}

// Run through the actual registered native recovery route. There is no
// frontend caller of this token-generation API; this is not browser proof.
func testRecoveryUnreadStartupHTTP(t *testing.T) {
	for _, kind := range []string{"regular", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "symlink" && runtime.GOOS == "windows" {
				t.Skip("requires unprivileged symlink creation")
			}
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
			resetTrustedProxyConfig()
			dir := t.TempDir()
			path := filepath.Join(dir, "recovery_tokens.json")
			before := []byte("synthetic broken inventory")
			target := path
			if kind == "symlink" {
				target = filepath.Join(dir, "unowned-target")
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(target, before, 0600); err != nil {
				t.Fatal(err)
			}
			hashed, err := internalauth.HashPassword("SyntheticPassword!1")
			if err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{AuthUser: "admin", AuthPass: hashed, DataPath: dir, ConfigPath: dir}
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")
			cleanupTestRouter(t, router)
			store := GetRecoveryTokenStore()
			generate := func(remote string) (*httptest.ResponseRecorder, bool, string) {
				t.Helper()
				req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"generate_token"}`))
				req.RemoteAddr = remote
				req.Host = "localhost"
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				var result struct {
					Success bool   `json:"success"`
					Token   string `json:"token"`
					Message string `json:"message"`
				}
				if rec.Code == http.StatusOK {
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if !result.Success && result.Message != "Failed to generate recovery token" {
						t.Fatal("failed issuance did not retain its generic native response")
					}
				}
				if len(rec.Result().Cookies()) != 0 || strings.Contains(rec.Body.String(), dir) || strings.Contains(rec.Body.String(), string(before)) || strings.Contains(rec.Body.String(), "token_hash") {
					t.Fatal("issuance disclosed stored authority or established a session")
				}
				return rec, result.Success, result.Token
			}
			if rec, _, _ := generate("203.0.113.42:12345"); rec.Code != http.StatusForbidden {
				t.Fatal("startup failure widened remote token issuance")
			}
			recoveryNoPreservedInventory(t, store)
			rec, success, raw := generate("127.0.0.1:12345")
			if rec.Code != http.StatusOK || success != (kind == "regular") || (raw != "") != success {
				t.Fatal("native issuance reported the wrong preservation outcome")
			}
			if kind == "symlink" {
				if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 || len(store.tokens) != 0 {
					t.Fatal("refused native issuance changed the unread symlink or authority")
				}
				if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, before) {
					t.Fatal("native refusal changed the symlink target")
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				rec, success, raw = generate("127.0.0.1:12345")
				if rec.Code != http.StatusOK || !success || raw == "" {
					t.Fatal("native issuance did not recover after deliberate path repair")
				}
				recoveryNoPreservedInventory(t, store)
			} else {
				recoveryPreservedInventory(t, store, before)
			}
			if !reloadRecoveryPersistence(store).IsRecoveryTokenValidConstantTime(raw, "127.0.0.1") || store.IsRecoveryTokenValidConstantTime(raw, "203.0.113.42") {
				t.Fatal("native issuance lost persistence or IP binding")
			}
			if cfg.AuthUser != "admin" || cfg.AuthPass != hashed || len(cfg.APITokens) != 0 {
				t.Fatal("recovery inventory maintenance replaced installed credentials")
			}
		})
	}
}
