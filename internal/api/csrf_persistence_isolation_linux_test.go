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
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Pause the real temporary-file write after its first byte, not a persistence
// mock or an elapsed-time guess. A synthetic oversized hash fills the FIFO.
// Resuming drains both goroutines. A later normal save replaces the FIFO with
// a regular file before any restart; the captured first snapshot stays separate.
func pauseCSRFDiskWrite(t *testing.T, store *CSRFTokenStore, action func()) func() []byte {
	t.Helper()
	store.mu.Lock()
	store.tokens["synthetic-disk-filler"] = []*CSRFToken{{Hash: strings.Repeat("f", 1<<20), Expires: time.Now().Add(time.Hour)}}
	store.mu.Unlock()
	path := filepath.Join(store.dataPath, "csrf_tokens.json.tmp")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	entered := make(chan error, 1)
	readerDone := make(chan []byte, 1)
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); action() }()
	go func() {
		file, err := os.Open(path)
		if err != nil {
			entered <- err
			readerDone <- nil
			return
		}
		defer file.Close()
		var first [1]byte
		_, err = io.ReadFull(file, first[:])
		entered <- err
		<-release
		var captured bytes.Buffer
		captured.Write(first[:])
		_, err = io.Copy(&captured, file)
		if err != nil {
			readerDone <- nil
			return
		}
		readerDone <- captured.Bytes()
	}()
	var once sync.Once
	var captured []byte
	resume := func() []byte {
		t.Helper()
		once.Do(func() {
			close(release)
			select {
			case captured = <-readerDone:
			case <-time.After(5 * time.Second):
				t.Error("CSRF fixture reader did not join")
			}
			select {
			case <-writerDone:
			case <-time.After(5 * time.Second):
				t.Error("CSRF writer did not join after disk resumed")
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
		t.Fatal("CSRF save did not reach its fixture write")
	}
	select {
	case <-writerDone:
		t.Error("mutation returned before its save completed")
	default:
	}
	return resume
}

func csrfDiskStore(t *testing.T) (*CSRFTokenStore, string, string) {
	t.Helper()
	store := &CSRFTokenStore{tokens: make(map[string][]*CSRFToken), dataPath: t.TempDir()}
	control := store.GenerateCSRFToken("control-session")
	target := store.GenerateCSRFToken("target-session")
	return store, control, target
}

func loadCSRFDiskStore(t *testing.T, dir string) *CSRFTokenStore {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, "csrf_tokens.json"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("restart requires a regular snapshot, not the fixture FIFO: %v", err)
	}
	loaded := &CSRFTokenStore{tokens: make(map[string][]*CSRFToken), dataPath: dir}
	loaded.load()
	return loaded
}

func TestCSRFPersistenceMutationsReleaseMemoryBeforeIO(t *testing.T) {
	for _, operation := range []string{"periodic", "shutdown", "generate", "delete", "delete-keys"} {
		t.Run(operation, func(t *testing.T) {
			store, control, target := csrfDiskStore(t)
			var generated string
			action := store.save
			switch operation {
			case "shutdown":
				store.saveTicker = time.NewTicker(5 * time.Minute)
				store.stopChan = make(chan bool)
				store.workerDone = make(chan struct{})
				go store.backgroundWorker()
				t.Cleanup(store.Shutdown)
				action = store.Shutdown
			case "generate":
				action = func() { generated = store.GenerateCSRFToken("new-session") }
			case "delete":
				action = func() { store.DeleteCSRFToken("target-session") }
			case "delete-keys":
				action = func() { store.DeleteCSRFSessionKeys([]string{csrfSessionKey("target-session")}) }
			}
			resume := pauseCSRFDiskWrite(t, store, action)
			if !store.mu.TryLock() {
				t.Error("disk I/O retains the validation/withdrawal mutex")
			} else {
				store.mu.Unlock()
			}
			validated := make(chan bool, 1)
			go func() { validated <- store.ValidateCSRFToken("control-session", control) }()
			select {
			case valid := <-validated:
				if !valid {
					t.Error("independent valid token denied")
				}
			case <-time.After(2 * time.Second):
				resume()
				<-validated
				t.Fatal("validation waits for another session's disk I/O")
			}
			captured := resume()
			var records []CSRFTokenData
			if err := json.Unmarshal(captured, &records); err != nil {
				t.Fatal(err)
			}
			foundTarget := false
			for _, record := range records {
				if record.SessionKey == csrfSessionKey("target-session") {
					foundTarget = true
				}
			}
			if foundTarget != (operation != "delete" && operation != "delete-keys") {
				t.Fatal("withdrawal snapshot crossed its session boundary")
			}
			if bytes.Contains(captured, []byte(control)) || bytes.Contains(captured, []byte(target)) || (generated != "" && bytes.Contains(captured, []byte(generated))) {
				t.Fatal("snapshot disclosed a raw token")
			}
			store.save()
			loaded := loadCSRFDiskStore(t, store.dataPath)
			if !loaded.ValidateCSRFToken("control-session", control) || loaded.ValidateCSRFToken("target-session", target) != foundTarget {
				t.Fatal("restart lost control or restored a withdrawn token")
			}
			if operation == "generate" && !loaded.ValidateCSRFToken("new-session", generated) {
				t.Fatal("returned token was not durably saved")
			}
		})
	}
}

func TestCSRFPersistenceQueuedRevocationWinsOlderSave(t *testing.T) {
	for _, operation := range []string{"delete", "delete-keys"} {
		t.Run(operation, func(t *testing.T) {
			store, control, target := csrfDiskStore(t)
			replacement := store.GenerateCSRFToken("target-session")
			resume := pauseCSRFDiskWrite(t, store, store.save)
			deleted := make(chan struct{})
			go func() {
				defer close(deleted)
				if operation == "delete" {
					store.DeleteCSRFToken("target-session")
				} else {
					store.DeleteCSRFSessionKeys([]string{csrfSessionKey("target-session"), "missing-key"})
				}
			}()
			deadline := time.After(2 * time.Second)
			tick := time.NewTicker(time.Millisecond)
			defer tick.Stop()
			for {
				if store.mu.TryRLock() {
					_, present := store.tokens[csrfSessionKey("target-session")]
					store.mu.RUnlock()
					if !present {
						break
					}
				}
				select {
				case <-tick.C:
				case <-deadline:
					resume()
					<-deleted
					t.Fatal("live revocation waits for an older disk snapshot")
				}
			}
			if store.ValidateCSRFToken("target-session", target) || store.ValidateCSRFToken("target-session", replacement) || !store.ValidateCSRFToken("control-session", control) {
				t.Fatal("pending persistence weakened live withdrawal or denied control")
			}
			select {
			case <-deleted:
				t.Fatal("revocation returned before its queued save")
			default:
			}
			old := resume()
			if !bytes.Contains(old, []byte(csrfTokenHash(target))) {
				t.Fatal("fixture did not capture the older pre-revocation snapshot")
			}
			select {
			case <-deleted:
			case <-time.After(5 * time.Second):
				t.Fatal("queued revocation did not join")
			}
			loaded := loadCSRFDiskStore(t, store.dataPath)
			if loaded.ValidateCSRFToken("target-session", target) || loaded.ValidateCSRFToken("target-session", replacement) || !loaded.ValidateCSRFToken("control-session", control) {
				t.Fatal("older snapshot resurrected a revoked token set")
			}
		})
	}
}

func TestCSRFPersistenceSnapshotOwnsTokenValues(t *testing.T) {
	store, _, _ := csrfDiskStore(t)
	now := time.Now()
	store.mu.Lock()
	store.tokens = map[string][]*CSRFToken{"synthetic-session": {nil, {Hash: "synthetic-hash", Expires: now}}}
	store.mu.Unlock()
	snapshot, sessions := store.persistenceSnapshot()
	store.mu.Lock()
	store.tokens["synthetic-session"][1].Hash = "changed"
	store.tokens["synthetic-session"][1].Expires = now.Add(time.Hour)
	delete(store.tokens, "synthetic-session")
	store.mu.Unlock()
	if sessions != 1 || len(snapshot) != 1 || snapshot[0].TokenHash != "synthetic-hash" || !snapshot[0].ExpiresAt.Equal(now) || snapshot[0].SessionKey != "synthetic-session" {
		t.Fatal("snapshot aliases live token state or lost metadata")
	}
}

func TestCSRFPersistenceConcurrentMutationRestart(t *testing.T) {
	store, control, _ := csrfDiskStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			for j := 0; j < 3; j++ {
				store.GenerateCSRFToken("shared-session")
				token := store.GenerateCSRFToken("withdrawn-session")
				store.ValidateCSRFToken("withdrawn-session", token)
				store.DeleteCSRFSessionKeys([]string{csrfSessionKey("withdrawn-session")})
				store.cleanup()
				store.save()
			}
		})
	}
	wg.Wait()
	store.DeleteCSRFToken("withdrawn-session")
	loaded := loadCSRFDiskStore(t, store.dataPath)
	if !loaded.ValidateCSRFToken("control-session", control) || len(loaded.tokens[csrfSessionKey("withdrawn-session")]) != 0 || len(loaded.tokens[csrfSessionKey("shared-session")]) != maxCSRFTokensPerSession {
		t.Fatal("parallel saves lost control, restored withdrawn state or broke the token bound")
	}
	info, err := os.Stat(filepath.Join(store.dataPath, "csrf_tokens.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("saved tokens are not owner-private: %v", err)
	}
}

// Exercise the consuming registered middleware, not only store validation.
// A malformed token-create body reaches its handler (400) only after real
// authentication and CSRF admission; no actual credential is created.
func TestCSRFPersistenceRegisteredRequestWhileIssuanceStalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "synthetic-admin", AuthPass: hash}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	const session = "synthetic-csrf-disk-session"
	router.sessionStore.CreateSession(session, time.Hour, "fixture", "192.0.2.1", "synthetic-admin")
	token := router.csrfStore.GenerateCSRFToken(session)
	resume := pauseCSRFDiskWrite(t, router.csrfStore, func() { router.csrfStore.GenerateCSRFToken("unrelated-session") })
	request := httptest.NewRequest(http.MethodPost, "https://pulse.example.test/api/security/tokens", strings.NewReader("{not-json"))
	request.Header.Set("Origin", "https://pulse.example.test")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("X-CSRF-Token", token)
	request.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: session})
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); router.Handler().ServeHTTP(response, request) }()
	select {
	case <-done:
		if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "CSRF") {
			t.Fatalf("authenticated write did not reach body validation: %d %s", response.Code, response.Body.String())
		}
	case <-time.After(2 * time.Second):
		resume()
		<-done
		t.Fatal("registered authenticated write waits for unrelated token persistence")
	}
	resume()
	router.csrfStore.save()
}

// A second account revocation may find the live key already gone while an
// individual revocation's disk save is queued. It must not acknowledge early.
func TestCSRFPersistenceNoMatchRevocationWaitsForSave(t *testing.T) {
	store, control, target := csrfDiskStore(t)
	resume := pauseCSRFDiskWrite(t, store, store.save)
	first := make(chan struct{})
	go func() { defer close(first); store.DeleteCSRFToken("target-session") }()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if store.mu.TryRLock() {
			_, present := store.tokens[csrfSessionKey("target-session")]
			store.mu.RUnlock()
			if !present {
				break
			}
		}
		select {
		case <-tick.C:
		case <-deadline:
			resume()
			<-first
			t.Fatal("first revocation did not reach live state")
		}
	}
	// With no target at all, the existing empty-input optimization remains safe.
	store.DeleteCSRFSessionKeys(nil)
	started, second := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(second)
		close(started)
		store.DeleteCSRFSessionKeys([]string{csrfSessionKey("target-session")})
	}()
	<-started
	select {
	case <-second:
		resume()
		<-first
		t.Fatal("no-match revocation returned before the pending save")
	case <-time.After(100 * time.Millisecond):
	}
	resume()
	for _, done := range []chan struct{}{first, second} {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("revocation did not join after storage resumed")
		}
	}
	loaded := loadCSRFDiskStore(t, store.dataPath)
	if loaded.ValidateCSRFToken("target-session", target) || !loaded.ValidateCSRFToken("control-session", control) {
		t.Fatal("no-match revocation did not retain the full withdrawal boundary")
	}
}

func TestCSRFPersistenceWriteFailureKeepsLiveRevocation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires ordinary-user filesystem permissions")
	}
	store, control, target := csrfDiskStore(t)
	original, err := os.ReadFile(filepath.Join(store.dataPath, "csrf_tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store.dataPath, 0700) })
	if err := os.Chmod(store.dataPath, 0500); err != nil {
		t.Fatal(err)
	}
	store.DeleteCSRFSessionKeys([]string{csrfSessionKey("target-session")})
	if store.ValidateCSRFToken("target-session", target) || !store.ValidateCSRFToken("control-session", control) {
		t.Fatal("failed save blocked or weakened live revocation")
	}
	after, err := os.ReadFile(filepath.Join(store.dataPath, "csrf_tokens.json"))
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("failed save changed the previous snapshot: %v", err)
	}
	if err := os.Chmod(store.dataPath, 0700); err != nil {
		t.Fatal(err)
	}
	store.save()
	loaded := loadCSRFDiskStore(t, store.dataPath)
	if loaded.ValidateCSRFToken("target-session", target) || !loaded.ValidateCSRFToken("control-session", control) {
		t.Fatal("storage recovery lost control or restored revoked state")
	}
}
