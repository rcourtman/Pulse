package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func danglingSessionInventory(t *testing.T, dir, kind string) (path, target, link string) {
	t.Helper()
	path = filepath.Join(dir, "sessions.json")
	target = filepath.Join(dir, "unavailable-sessions.json")
	link = filepath.Base(target)
	switch kind {
	case "absolute":
		link = target
	case "missing-parent":
		link = filepath.Join("offline-volume", "sessions.json")
		target = filepath.Join(dir, link)
	case "chain":
		link = "session-link"
		if err := os.Symlink(filepath.Base(target), filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(link, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); !os.IsNotExist(err) {
		t.Fatalf("fixture must fail the target read with not-exist: %v", err)
	}
	return path, target, link
}

func requireSessionLink(t *testing.T, path, link string, before os.FileInfo) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil || got != link {
		t.Fatalf("unread session link was replaced: %v (link %q)", err, got)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("unread session link inode changed: %v", err)
	}
}

func TestSessionStore_DanglingStartupEverySave(t *testing.T) {
	for _, kind := range []string{"relative", "absolute", "missing-parent", "chain"} {
		for _, firstSave := range []string{"periodic", "create", "recovery", "oidc", "saml", "delete", "delete-user", "shutdown"} {
			t.Run(kind+"/"+firstSave, func(t *testing.T) {
				dir := t.TempDir()
				path, target, link := danglingSessionInventory(t, dir, kind)
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				logs := captureTestLogs(t)
				store := NewSessionStore(dir)
				t.Cleanup(store.Shutdown)
				if len(store.sessions) != 0 {
					t.Fatal("unread startup granted saved authority")
				}
				writes := 0
				store.persistMu.Lock()
				store.snapshotWrite = func(f *os.File, data []byte) (int, error) {
					writes++
					return f.Write(data)
				}
				store.persistMu.Unlock()
				const raw = "private-dangling-fixture-session"
				switch firstSave {
				case "periodic":
					store.save()
				case "create":
					store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
				case "recovery":
					store.CreateRecoverySession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
				case "oidc":
					store.CreateOIDCSession(raw, time.Hour, "fixture", "127.0.0.1", "operator", &OIDCTokenInfo{RefreshToken: "private-dangling-fixture-refresh"})
				case "saml":
					store.CreateSAMLSession(raw, time.Hour, "fixture", "127.0.0.1", "operator", &SAMLTokenInfo{ProviderID: "fixture-provider"})
				case "delete":
					store.DeleteSession("unknown-session")
				case "delete-user":
					store.DeleteSessionsForUser("unknown-user")
				case "shutdown":
					store.Shutdown()
				}
				requireSessionLink(t, path, link, before)
				if firstSave != "shutdown" {
					store.CreateSession("private-live-control", time.Hour, "fixture", "127.0.0.1", "operator")
					store.CreateSession("private-withdrawn-control", time.Hour, "fixture", "127.0.0.1", "withdrawn-user")
					store.DeleteSessionsForUser("withdrawn-user")
					if !store.ValidateAndExtendSession("private-live-control") || store.ValidateSession("private-withdrawn-control") {
						t.Fatal("persistence hold blocked live authentication or withdrawal")
					}
				}
				store.Shutdown()
				requireSessionLink(t, path, link, before)
				if writes != 0 || len(sessionPreservedFiles(t, dir)) != 0 {
					t.Fatal("unread nonregular entry staged or published a snapshot")
				}
				checkNoOwnedAuthTemps(t, dir, "sessions.json")
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatalf("unread link target was created: %v", err)
				}
				if strings.Count(logs.String(), "Cannot preserve unread session file;") != 1 || !strings.Contains(logs.String(), "New sign-ins are memory-only") {
					t.Fatal("persistence hold was not explained once")
				}
				if strings.Contains(logs.String(), raw) || strings.Contains(logs.String(), "private-dangling-fixture-refresh") {
					t.Fatal("diagnostics disclosed session or refresh credentials")
				}
			})
		}
	}
}

func TestSessionStore_DanglingStartupRecoveryKeepsCurrentAuthority(t *testing.T) {
	for _, targetReturns := range []bool{false, true} {
		name := "target-absent"
		if targetReturns {
			name = "target-returns"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path, target, link := danglingSessionInventory(t, dir, "relative")
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			store := NewSessionStore(dir)
			t.Cleanup(store.Shutdown)
			store.CreateSession("current-session", time.Hour, "fixture", "127.0.0.1", "current-user")
			store.CreateSession("withdrawn-session", time.Hour, "fixture", "127.0.0.1", "withdrawn-user")
			store.DeleteSessionsForUser("withdrawn-user")
			if targetReturns {
				data, err := json.Marshal([]sessionPersisted{{Key: sessionHash("dormant-session"), Username: "old-user", ExpiresAt: time.Now().Add(time.Hour)}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, data, 0600); err != nil {
					t.Fatal(err)
				}
				store.save()
				got, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(got, data) || store.ValidateSession("dormant-session") {
					t.Fatal("a returning target changed or became authority without a deliberate restore")
				}
			}
			requireSessionLink(t, path, link, before)
			// Only deliberate movement of the unknown entry releases the hold.
			retained := filepath.Join(dir, "operator-retained-link")
			if err := os.Rename(path, retained); err != nil {
				t.Fatal(err)
			}
			store.save()
			store.Shutdown()
			if got, err := os.Readlink(retained); err != nil || got != link {
				t.Fatalf("operator-retained link changed: %v", err)
			}
			restarted := NewSessionStore(dir)
			defer restarted.Shutdown()
			if !restarted.ValidateSession("current-session") || restarted.ValidateSession("withdrawn-session") || restarted.ValidateSession("dormant-session") {
				t.Fatal("storage recovery lost current authority or revived unknown/withdrawn authority")
			}
			if len(sessionPreservedFiles(t, dir)) != 0 {
				t.Fatal("normal recovery moved a fresh inventory")
			}
		})
	}
}

func TestSessionStore_ReadableStartupLinkKeepsCompatibility(t *testing.T) {
	for _, format := range []string{"hashed", "legacy"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			const raw = "readable-link-session"
			entry := SessionData{Username: "saved-user", ExpiresAt: time.Now().Add(time.Hour)}
			var data []byte
			var err error
			if format == "legacy" {
				data, err = json.Marshal(map[string]SessionData{raw: entry})
			} else {
				data, err = json.Marshal([]sessionPersisted{{Key: sessionHash(raw), Username: entry.Username, ExpiresAt: entry.ExpiresAt}})
			}
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "readable-inventory")
			if err := os.WriteFile(target, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(target), filepath.Join(dir, "sessions.json")); err != nil {
				t.Fatal(err)
			}
			store := NewSessionStore(dir)
			defer store.Shutdown()
			if !store.ValidateSession(raw) {
				t.Fatal("readable symlink no longer loads existing authority")
			}
			store.CreateSession("new-readable-control", time.Hour, "fixture", "127.0.0.1", "new-user")
			store.Shutdown()
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, data) || len(sessionPreservedFiles(t, dir)) != 0 {
				t.Fatal("successful load changed its target or entered the unread-state hold")
			}
			restarted := NewSessionStore(dir)
			defer restarted.Shutdown()
			if !restarted.ValidateSession(raw) || !restarted.ValidateSession("new-readable-control") {
				t.Fatal("successful linked input lost normal persistence")
			}
		})
	}
}

func TestSessionStore_DanglingStartupNativeAuth(t *testing.T) {
	dir := t.TempDir()
	path, _, link := danglingSessionInventory(t, dir, "relative")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	InitSessionStore(dir)
	t.Cleanup(resetSessionStoreForTests)
	store := GetSessionStore()
	const raw = "native-memory-session"
	store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	cfg := &config.Config{AuthUser: "operator", AuthPass: "configured-fixture-password"}
	request := func(token string) bool {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: token})
		w := httptest.NewRecorder()
		return CheckAuth(cfg, w, req)
	}
	if !request(raw) || request("unissued-fixture-session") {
		t.Fatal("native auth lost the current session or manufactured saved authority")
	}
	store.DeleteSession(raw)
	if request(raw) {
		t.Fatal("native auth retained withdrawn authority during the persistence hold")
	}
	store.Shutdown()
	requireSessionLink(t, path, link, before)
	restarted := NewSessionStore(dir)
	defer restarted.Shutdown()
	if restarted.ValidateSession(raw) {
		t.Fatal("restart manufactured an unpersisted session")
	}
}
