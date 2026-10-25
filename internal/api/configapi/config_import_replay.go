package configapi

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	configImportReplayWindow = 10 * time.Minute
	configImportRequestLimit = 128
)

type configImportKey struct {
	orgID       string
	fingerprint [sha256.Size]byte
}

type configImportAttempt struct {
	key         configImportKey
	completedAt time.Time // Zero while persistence or runtime apply is in flight.
}

type configImportRejection struct {
	status  int
	message string
}

// configImportRequests protects the existing no-request-ID API without keeping
// archives, passphrases or responses. It is deliberately process-local and
// bounded: one in-flight import per org and at most 128 attempts for ten minutes
// after completion. Never evict an unexpired attempt to admit a new one.
type configImportRequests struct {
	mu       sync.Mutex
	attempts map[configImportKey]*configImportAttempt
}

func (g *configImportRequests) begin(orgID string, req ImportConfigRequest, now time.Time) (*configImportAttempt, *configImportRejection) {
	if orgID == "" {
		orgID = "default"
	}
	// Length-prefix the data, including malformed input. Include the
	// passphrase so a different password cannot reuse an earlier admission.
	// Only this full-request digest survives the request, never either input.
	hash := sha256.New()
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(req.Data)))
	_, _ = hash.Write(length[:])
	_, _ = io.WriteString(hash, req.Data)
	_, _ = io.WriteString(hash, req.Passphrase)
	key := configImportKey{orgID: orgID}
	hash.Sum(key.fingerprint[:0])

	g.mu.Lock()
	defer g.mu.Unlock()
	for k, attempt := range g.attempts {
		if !attempt.completedAt.IsZero() && now.Sub(attempt.completedAt) >= configImportReplayWindow {
			delete(g.attempts, k)
		}
	}
	if previous := g.attempts[key]; previous != nil && !previous.completedAt.IsZero() {
		// Do not return a cached success: the configuration may have changed
		// since that attempt. A replay must not claim a fresh restoration.
		return nil, &configImportRejection{http.StatusConflict, "This backup was submitted recently and may already have changed the configuration. It has not been imported again; check the current configuration before retrying in 10 minutes."}
	}
	for k, attempt := range g.attempts {
		if k.orgID == orgID && attempt.completedAt.IsZero() {
			return nil, &configImportRejection{http.StatusConflict, "A configuration import is still running for this workspace. This request has not been applied; wait for it to finish and check the configuration before retrying."}
		}
	}
	if len(g.attempts) >= configImportRequestLimit {
		return nil, &configImportRejection{http.StatusServiceUnavailable, "Configuration imports are temporarily unavailable. This request has not been applied; try again in 10 minutes."}
	}
	if g.attempts == nil {
		g.attempts = make(map[configImportKey]*configImportAttempt)
	}
	attempt := &configImportAttempt{key: key}
	g.attempts[key] = attempt
	return attempt, nil
}

func (g *configImportRequests) finish(attempt *configImportAttempt, retain bool, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if retain {
		attempt.completedAt = now
	} else {
		delete(g.attempts, attempt.key)
	}
}
