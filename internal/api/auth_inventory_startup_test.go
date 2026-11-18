package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Use the real initializers (and their workers) rather than a reader-only seam.
// Tests using the globally selected CSRF/recovery stores must not be parallel.
type startupAuthInventory struct {
	session  *SessionStore
	csrf     *CSRFTokenStore
	recovery *RecoveryTokenStore
}

func startAuthInventory(kind, dir string) startupAuthInventory {
	switch kind {
	case "sessions":
		return startupAuthInventory{session: NewSessionStore(dir)}
	case "csrf_tokens":
		return startupAuthInventory{csrf: ensureCSRFStore(dir)}
	case "recovery_tokens":
		return startupAuthInventory{recovery: ensureRecoveryTokenStore(dir)}
	default:
		panic("unknown test inventory")
	}
}

func (s startupAuthInventory) shutdown() {
	if s.session != nil {
		s.session.Shutdown()
	}
	if s.csrf != nil {
		s.csrf.Shutdown()
		resetCSRFStoreForTests()
	}
	if s.recovery != nil {
		s.recovery.Shutdown()
		resetRecoveryStoreForTests()
	}
}

func (s startupAuthInventory) requireUnread(t *testing.T) {
	t.Helper()
	switch {
	case s.session != nil:
		s.session.mu.RLock()
		empty := len(s.session.sessions) == 0
		s.session.mu.RUnlock()
		s.session.persistMu.Lock()
		held := s.session.preserveBeforeSave
		s.session.persistMu.Unlock()
		if !empty || !held {
			t.Fatal("unread sessions granted authority or authorized replacement")
		}
	case s.csrf != nil:
		s.csrf.mu.RLock()
		empty := len(s.csrf.tokens) == 0
		s.csrf.mu.RUnlock()
		s.csrf.saveMu.Lock()
		held := s.csrf.preserveBeforeSave
		s.csrf.saveMu.Unlock()
		if !empty || !held {
			t.Fatal("unread CSRF inventory granted authority or authorized replacement")
		}
	case s.recovery != nil:
		s.recovery.mu.RLock()
		defer s.recovery.mu.RUnlock()
		if len(s.recovery.tokens) != 0 || !s.recovery.preserveBeforeSave {
			t.Fatal("unread recovery inventory granted authority or authorized replacement")
		}
	}
}

// Session/CSRF grants remain memory-only while the existing save hold is in
// force. Recovery issuance must fail and roll back its unpersisted grant.
func (s startupAuthInventory) exerciseHeldWrites(t *testing.T) {
	t.Helper()
	const raw = "synthetic-startup-current"
	switch {
	case s.session != nil:
		s.session.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "fixture-user")
		if !s.session.ValidateSession(raw) {
			t.Fatal("nonregular storage blocked a current in-memory sign-in")
		}
		s.session.DeleteSession(raw)
	case s.csrf != nil:
		token := s.csrf.GenerateCSRFToken(raw)
		if token == "" || !s.csrf.ValidateCSRFToken(raw, token) {
			t.Fatal("nonregular storage blocked a current in-memory CSRF token")
		}
		s.csrf.DeleteCSRFToken(raw)
	case s.recovery != nil:
		if token, err := s.recovery.GenerateRecoveryToken(time.Hour, "127.0.0.1"); err == nil || token != "" {
			t.Fatal("unpersisted recovery token was issued")
		}
	}
	s.requireUnread(t)
}

func testAuthInventoryRegularLink(t *testing.T, kind string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file-link compatibility control")
	}
	for _, legacy := range []bool{false, true} {
		format := "hashed"
		if legacy {
			format = "legacy"
		}
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			const raw = "synthetic-startup-saved"
			now, expires := time.Now(), time.Now().Add(time.Hour)
			var inventory any
			switch kind {
			case "sessions":
				inventory = []sessionPersisted{{Key: sessionHash(raw), Username: "fixture-user", CreatedAt: now, ExpiresAt: expires}}
				if legacy {
					inventory = map[string]SessionData{raw: {Username: "fixture-user", CreatedAt: now, ExpiresAt: expires}}
				}
			case "csrf_tokens":
				inventory = []*CSRFTokenData{{SessionKey: csrfSessionKey(raw), TokenHash: csrfTokenHash(raw), ExpiresAt: expires}}
				if legacy {
					inventory = map[string]*legacyCSRFTokenData{raw: {SessionID: raw, Token: raw, ExpiresAt: expires}}
				}
			case "recovery_tokens":
				inventory = []*RecoveryToken{{TokenHash: recoveryTokenHash(raw), CreatedAt: now, ExpiresAt: expires, IP: "loopback"}}
				if legacy {
					inventory = map[string]*legacyRecoveryToken{raw: {Token: raw, CreatedAt: now, ExpiresAt: expires, IP: "loopback"}}
				}
			}
			data, err := json.Marshal(inventory)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "saved-inventory.json")
			if err := os.WriteFile(target, data, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(target), filepath.Join(dir, kind+".json")); err != nil {
				t.Fatal(err)
			}
			s := startAuthInventory(kind, dir)
			t.Cleanup(s.shutdown)
			valid := false
			switch kind {
			case "sessions":
				valid = s.session.ValidateSession(raw) && s.session.GetSession(raw).Username == "fixture-user"
			case "csrf_tokens":
				valid = s.csrf.ValidateCSRFToken(raw, raw)
			case "recovery_tokens":
				valid = s.recovery.IsRecoveryTokenValidConstantTime(raw, "127.0.0.1")
			}
			if !valid {
				t.Fatal("regular-file link lost saved authority")
			}
			s.shutdown()
			after, err := os.Stat(target)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("startup/migration/shutdown replaced the linked original")
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("startup/migration/shutdown changed the linked original")
			}
		})
	}
}
