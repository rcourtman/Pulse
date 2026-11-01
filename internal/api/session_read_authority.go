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
	admitted := *marker
	// A recovery binding is evaluated against the original immutable request,
	// not a request that returning middleware may still be observing.
	original := req.Clone(req.Context())
	return func() bool {
		sessionStoreMu.Lock()
		currentStore := sessionStore
		sessionStoreMu.Unlock()
		if !admitted.valid || currentStore == nil || currentStore != admitted.store {
			return false
		}
		current := currentStore.GetSession(admitted.token)
		if current == nil || !time.Now().Before(current.ExpiresAt) ||
			!current.CreatedAt.Equal(admitted.createdAt) || current.Username != admitted.username ||
			current.RecoveryBypass != admitted.recovery {
			return false
		}
		return !current.RecoveryBypass || requestMatchesRecoverySession(original, current)
	}
}
