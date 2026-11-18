package api

import (
	"context"
	"net/http"
	"time"
)

type sessionReadAdmissionKey struct{}

// sessionReadAdmission records actual successful session authentication, not
// the presence of an ambient cookie or a mutable response header. Other
// credentials retain their existing precedence and cannot fall back to a
// new session after admission. Never log or serialize this private identity.
type sessionReadAdmission struct {
	store     *SessionStore
	token     string
	createdAt time.Time
	username  string
	recovery  bool
	valid     bool
}

func attachSessionReadAdmission(req *http.Request, token string, session *SessionData) {
	admission := &sessionReadAdmission{store: GetSessionStore(), token: token}
	if session != nil {
		admission.createdAt = session.CreatedAt
		admission.username = session.Username
		admission.recovery = session.RecoveryBypass
		admission.valid = true
	}
	*req = *req.WithContext(context.WithValue(req.Context(), sessionReadAdmissionKey{}, admission))
}

// sessionReadRevalidator retains only the admitted local session's validity,
// principal and instance identity. It does not extend expiry, refresh an IdP,
// rerun authentication, change tenant/admin policy or cancel independent work.
// A nil callback means this request was not authenticated by a session.
func sessionReadRevalidator(req *http.Request) func() bool {
	marker, _ := req.Context().Value(sessionReadAdmissionKey{}).(*sessionReadAdmission)
	if marker == nil {
		return nil
	}
	if !marker.valid {
		return func() bool { return false }
	}
	// One lookup key for this read; each check still consults live authority.
	key := sessionHash(marker.token)
	store := marker.store
	createdAt, username, recovery := marker.createdAt, marker.username, marker.recovery
	var original *http.Request
	if recovery {
		// Check the current binding against the original immutable request.
		// Ordinary sessions do not need any of the request headers.
		original = req.Clone(req.Context())
	}
	return func() bool {
		sessionStoreMu.Lock()
		currentStore := sessionStore
		sessionStoreMu.Unlock()
		if currentStore == nil || currentStore != store {
			return false
		}
		ip, ok := currentStore.sessionReadBinding(key, createdAt, username, recovery)
		return ok && (!recovery || requestMatchesRecoveryBinding(original, ip))
	}
}

// sessionReadBinding checks the live session without copying its full data
// (including identity-provider credentials). Only the recovery binding can
// leave the lock, and request inspection stays outside the lock.
func (s *SessionStore) sessionReadBinding(key string, createdAt time.Time, username string, recovery bool) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.sessions[key]
	if current == nil || !time.Now().Before(current.ExpiresAt) ||
		!current.CreatedAt.Equal(createdAt) || current.Username != username ||
		current.RecoveryBypass != recovery {
		return "", false
	}
	if recovery {
		return current.IP, true
	}
	return "", true
}
