//go:build linux

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// pauseSessionDiskWrite stops the real sessions.json.tmp write, not a mock
// persistence function. A large synthetic record fills the fixture FIFO after
// the reader acknowledges the first byte. No elapsed-time guess starts the
// assertions. Resuming drains and joins both goroutines, including on failure.
func pauseSessionDiskWrite(t *testing.T, store *SessionStore, action func()) func() []byte {
	t.Helper()
	store.mu.Lock()
	store.sessions[sessionHash("slow-disk-filler")].UserAgent = strings.Repeat("x", 1<<20)
	store.mu.Unlock()
	if err := syscall.Mkfifo(filepath.Join(store.dataPath, "sessions.json.tmp"), 0600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan error, 1)
	readDone := make(chan []byte, 1)
	actionDone := make(chan struct{})
	go func() { defer close(actionDone); action() }()
	go func() {
		file, err := os.Open(filepath.Join(store.dataPath, "sessions.json.tmp"))
		if err != nil {
			entered <- err
			readDone <- nil
			return
		}
		defer file.Close()
		var first [1]byte
		_, err = io.ReadFull(file, first[:])
		entered <- err
		<-release
		var data bytes.Buffer
		data.Write(first[:])
		_, _ = io.Copy(&data, file)
		readDone <- data.Bytes()
	}()
	var once sync.Once
	var captured []byte
	resume := func() []byte {
		t.Helper()
		once.Do(func() {
			close(release)
			select {
			case captured = <-readDone:
			case <-time.After(5 * time.Second):
				t.Error("fixture disk reader did not stop")
			}
			select {
			case <-actionDone:
			case <-time.After(5 * time.Second):
				t.Error("session writer did not stop after releasing disk")
			}
		})
		return captured
	}
	t.Cleanup(func() { resume() })
	select {
	case err := <-entered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session save did not reach the fixture disk write")
	}
	select {
	case <-actionDone:
		t.Error("session mutation returned before its persistence completed")
	default:
	}
	return resume
}

func sessionDiskStore(t *testing.T) *SessionStore {
	t.Helper()
	store := NewSessionStore(t.TempDir())
	t.Cleanup(store.Shutdown)
	for _, raw := range []string{"slow-disk-filler", "control", "target", "target-two"} {
		principal := "target-user"
		if raw == "control" || raw == "slow-disk-filler" {
			principal = "operator"
		}
		store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", principal)
	}
	return store
}

func TestSessionPersistenceMutationsReleaseMemoryBeforeIO(t *testing.T) {
	for _, operation := range []string{"periodic", "create", "recovery", "oidc", "saml", "refresh", "delete", "delete-user"} {
		t.Run(operation, func(t *testing.T) {
			store := sessionDiskStore(t)
			action := store.save
			switch operation {
			case "create":
				action = func() { store.CreateSession("new", time.Hour, "fixture", "127.0.0.1", "new-user") }
			case "recovery":
				action = func() { store.CreateRecoverySession("new", time.Hour, "fixture", "127.0.0.1", "new-user") }
			case "oidc":
				action = func() {
					store.CreateOIDCSession("new", time.Hour, "fixture", "127.0.0.1", "new-user", &OIDCTokenInfo{RefreshToken: "synthetic-refresh", AccessTokenExp: time.Now().Add(time.Hour)})
				}
			case "saml":
				action = func() {
					store.CreateSAMLSession("new", time.Hour, "fixture", "127.0.0.1", "new-user", &SAMLTokenInfo{ProviderID: "fixture-provider"})
				}
			case "refresh":
				action = func() { store.UpdateOIDCTokens("target", "synthetic-refresh", time.Now().Add(time.Hour)) }
			case "delete":
				action = func() { store.DeleteSession("target") }
			case "delete-user":
				action = func() { store.DeleteSessionsForUser("target-user") }
			}
			resume := pauseSessionDiskWrite(t, store, action)
			if !store.mu.TryLock() {
				t.Error("session disk I/O retains the authentication/withdrawal mutex")
			} else {
				store.mu.Unlock()
			}
			authenticated := make(chan bool, 1)
			go func() { authenticated <- store.ValidateAndExtendSession("control") }()
			select {
			case valid := <-authenticated:
				if !valid {
					t.Error("independent session was denied")
				}
			case <-time.After(2 * time.Second):
				t.Error("independent sliding authentication waits for disk")
				resume()
				<-authenticated
			}
			// The mutation remains synchronously persistent: it cannot finish
			// its save before the paused write has completed.
			data := resume()
			var persisted []sessionPersisted
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			var target, otherTarget, newEntry *sessionPersisted
			for i := range persisted {
				entry := &persisted[i]
				switch entry.Key {
				case sessionHash("target"):
					target = entry
				case sessionHash("target-two"):
					otherTarget = entry
				case sessionHash("new"):
					newEntry = entry
				}
			}
			if operation == "delete" && (target != nil || otherTarget == nil) || operation == "delete-user" && (target != nil || otherTarget != nil) {
				t.Error("withdrawal snapshot changed its target boundary")
			}
			if operation == "create" || operation == "recovery" || operation == "oidc" || operation == "saml" {
				if newEntry == nil || newEntry.Username != "new-user" || newEntry.RecoveryBypass != (operation == "recovery") {
					t.Error("created session snapshot changed its principal or recovery authority")
				}
			}
			if bytes.Contains(data, []byte("synthetic-refresh")) {
				t.Error("refresh token was not encrypted in detached persistence")
			}
		})
	}
}

func TestSessionPersistenceNewerWithdrawalWinsQueuedSave(t *testing.T) {
	store := sessionDiskStore(t)
	resume := pauseSessionDiskWrite(t, store, store.save)
	deleted := make(chan struct{})
	go func() { store.DeleteSession("target"); close(deleted) }()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if store.mu.TryRLock() {
			_, live := store.sessions[sessionHash("target")]
			store.mu.RUnlock()
			if !live {
				break
			}
		}
		select {
		case <-tick.C:
		case <-deadline:
			resume()
			<-deleted
			t.Fatal("withdrawal could not change live authority while a save was stalled")
		}
	}
	if store.ValidateSession("target") || !store.ValidateAndExtendSession("control") {
		t.Fatal("pending persistence weakened live withdrawal or blocked the control session")
	}
	select {
	case <-deleted:
		t.Fatal("withdrawal returned before its queued persistence completed")
	default:
	}
	old := resume()
	if !bytes.Contains(old, []byte(sessionHash("target"))) {
		t.Fatal("fixture did not pause an older authority snapshot")
	}
	select {
	case <-deleted:
	case <-time.After(5 * time.Second):
		t.Fatal("queued withdrawal did not complete")
	}
	loaded := NewSessionStore(store.dataPath)
	t.Cleanup(loaded.Shutdown)
	if loaded.GetSession("target") != nil || loaded.GetSession("control") == nil {
		t.Fatal("an older save resurrected withdrawn authority or lost an independent session")
	}
}

// Use a real registered router with an explicit empty monitor. A nil monitor
// would return 500 after successful authentication and cannot prove a usable
// bootstrap. Keep the fixture timestamp stable for before/after consumers.
func newSessionDiskRouter(t *testing.T) (*Router, *config.Config, *httptest.Server) {
	t.Helper()
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, "synthetic-session-disk-api.12345678", []string{config.ScopeWildcard}, nil))
	cfg.AuthUser = "operator"
	monitor, state, _ := newTestMonitor(t)
	state.LastUpdate = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	syncTestResourceStore(t, monitor, state)
	router := newTestRouter(t, cfg, monitor, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return router, cfg, server
}

func TestSessionPersistenceRealRoutesDuringDiskWrite(t *testing.T) {
	_, _, server := newSessionDiskRouter(t)
	store := GetSessionStore()
	for _, raw := range []string{"slow-disk-filler", "control"} {
		store.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	}
	resume := pauseSessionDiskWrite(t, store, store.save)
	defer resume()
	for _, path := range []string{"/api/security/status", "/api/state/summary"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: "control"})
		response := httptest.NewRecorder()
		done := make(chan struct{})
		go func() { server.Config.Handler.ServeHTTP(response, req); close(done) }()
		select {
		case <-done:
			if response.Code != http.StatusOK {
				t.Errorf("%s status = %d", path, response.Code)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("%s stalled behind session persistence", path)
			resume()
			<-done
		}
	}
}
