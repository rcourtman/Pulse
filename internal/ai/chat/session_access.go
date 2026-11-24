package chat

import (
	"context"
	"strings"
)

// SessionAccess is the caller a session request is made for. A session
// belongs to the caller whose turn or create request made it, and its
// transcript holds whatever the Assistant read for that caller, host command
// output included, so another caller may only open it when it may run those
// commands itself.
type SessionAccess struct {
	// Owner is the caller's stable server-side identity. Empty means the
	// caller could not be identified and owns nothing.
	Owner string
	// All admits every session of the organization: other callers' sessions,
	// sessions stored without an owner, and Pulse's own background sessions
	// (IsSystemSessionID). It is the caller's canonical action-execute
	// capability, the one that lets a chat turn run commands on hosts.
	All bool
}

type sessionAccessContextKey struct{}

// WithSessionAccess binds the caller that session reads and writes on this
// context are made for. Server-authored; never taken from a request payload.
func WithSessionAccess(ctx context.Context, access SessionAccess) context.Context {
	access.Owner = strings.TrimSpace(access.Owner)
	return context.WithValue(ctx, sessionAccessContextKey{}, access)
}

// SessionAccessFromContext reports the caller bound by WithSessionAccess.
// Absent means the work is Pulse's own (Patrol, investigations, scheduled
// runs) and is not confined to one caller's sessions; a boundary that serves
// callers must bind one.
func SessionAccessFromContext(ctx context.Context) (SessionAccess, bool) {
	access, ok := ctx.Value(sessionAccessContextKey{}).(SessionAccess)
	return access, ok
}

func sessionAccessFromContext(ctx context.Context) SessionAccess {
	if access, ok := SessionAccessFromContext(ctx); ok {
		return access
	}
	return SessionAccess{All: true}
}

// allows reports whether the caller may use the session with this ID and
// stored owner. Background session IDs are never a caller's own, whatever
// owner a file under that ID carries.
func (a SessionAccess) allows(sessionID, owner string) bool {
	if a.All {
		return true
	}
	if a.Owner == "" || IsSystemSessionID(sessionID) {
		return false
	}
	return owner == a.Owner
}

func (a SessionAccess) visibleSessions(sessions []Session) []Session {
	if a.All {
		return sessions
	}
	visible := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if a.allows(session.ID, session.Owner) {
			visible = append(visible, session)
		}
	}
	return visible
}
