package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// retainTokenStreamAuthority runs AFTER the route's auth/admin/scope gates.
// Recheck the admitted credential at the actual writer, not just when an event
// is produced: replay and queued broadcasts can otherwise disclose after a
// credential is withdrawn. Session withdrawal is checked too; other
// authentication policy stays with the route.
func retainTokenStreamAuthority(cfg *config.Config, scope string, instanceAdmin bool, next http.HandlerFunc) http.HandlerFunc {
	return retainTokenStreamAuthorityForScopes(cfg, []string{scope}, instanceAdmin, next)
}

func retainTokenStreamAuthorityForScopes(cfg *config.Config, scopes []string, instanceAdmin bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		revalidate := tokenStreamSnapshotRevalidator(cfg, r, scopes...)
		sessionAllowed := combineAuthorization(sessionReadRevalidator(r), orgAccessRevalidator(r))
		allowed := func() bool {
			if revalidate == nil {
				// Proxy/basic/bypass admission stays with the route. A
				// session read retains its exact admitted session instance,
				// and a signed-in user keeps the organization access policy.
				return sessionAllowed == nil || sessionAllowed()
			}
			current, ok := revalidate()
			return ok && (!instanceAdmin || tokenActsForInstance(current))
		}
		if !allowed() {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			// Do not turn a non-streaming writer into a streaming one. Leave
			// the handler's existing unsupported-transport response intact.
			next(w, r)
			return
		}

		// Only this read is cancelled. Updaters, Patrol runs and deployment
		// jobs retain their own lifetime; withdrawal is not a cancel action.
		ctx, cancel := context.WithCancel(r.Context())
		writer := &tokenStreamWriter{writer: w, flusher: flusher, ctx: ctx, cancel: cancel, allowed: allowed}
		// Every guarded read is watched for request-context cancellation,
		// which ends the read and expires a transport write it admitted: the
		// handler may itself be blocked inside that write and unable to reach
		// its deferred closure. Only token and session reads also get the
		// periodic authority recheck; other credentials get no new checks.
		var recheck <-chan time.Time
		if revalidate != nil || sessionAllowed != nil {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			recheck = ticker.C
		}
		watchDone := make(chan struct{})
		go func() {
			defer close(watchDone)
			for {
				select {
				case <-ctx.Done():
					writer.end()
					return
				case <-recheck:
					if !writer.check() {
						return
					}
				}
			}
		}()
		defer func() {
			// Reject queued writes even after handler cleanup, interrupt any
			// transport write still in progress, and join it and the watcher
			// before returning the HTTP writer.
			writer.close()
			<-watchDone
		}()
		next(writer, r.WithContext(ctx))
	}
}

// tokenStreamWriter deliberately exposes no Unwrap/ReaderFrom escape around
// the guard. update broadcasts use a different goroutine from the HTTP
// handler.
//
// Two locks keep a stalled peer from extending the stream's lifetime. ioMu
// serializes transport writes, header commits and flushes. mu guards the
// authority state and is never held across transport I/O, so closure,
// withdrawal and deadline control are never queued behind a blocked write.
// Lock order is ioMu, then mu. Every transition to closed expires a
// transport operation still in progress through the forwarded write
// deadline, and close() then joins it before the HTTP writer is returned.
type tokenStreamWriter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	cancel  context.CancelFunc
	allowed func() bool
	ioMu    sync.Mutex
	mu      sync.Mutex
	closed  bool
	started bool
	// inIO is set, under mu, from a passed authority check until the
	// transport operation it admitted returns.
	inIO bool
}

func (w *tokenStreamWriter) Header() http.Header { return w.writer.Header() }

// Forward deadline control without exposing Unwrap, which would also let a
// caller bypass the disclosure guard. Assistant streams clear the server's
// initial deadlines and bound every subsequent write, including heartbeats.
// Deadline control takes only mu, so it can interrupt a transport write that
// is blocked on a stalled peer; transport deadline setters are safe to call
// concurrently with a write.
func (w *tokenStreamWriter) SetWriteDeadline(deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.checkLocked() {
		return io.ErrClosedPipe
	}
	return http.NewResponseController(w.writer).SetWriteDeadline(deadline)
}

func (w *tokenStreamWriter) SetReadDeadline(deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.checkLocked() {
		return io.ErrClosedPipe
	}
	return http.NewResponseController(w.writer).SetReadDeadline(deadline)
}

func (w *tokenStreamWriter) checkLocked() bool {
	if w.closed {
		return false
	}
	if w.ctx.Err() != nil || !w.allowed() {
		w.closeLocked()
		return false
	}
	return true
}

// closeLocked ends the read: later operations are refused, the request
// context is cancelled, and a transport operation still in progress is
// expired so it cannot hold the stream open on a stalled peer. mu must be
// held.
func (w *tokenStreamWriter) closeLocked() {
	if !w.closed {
		w.closed = true
		w.cancel()
	}
	if w.inIO {
		_ = http.NewResponseController(w.writer).SetWriteDeadline(time.Now())
	}
}

// beginIO admits one transport operation under the current authority and
// commits the guarded media first when needed. It returns false when the read
// has ended.
func (w *tokenStreamWriter) beginIO(commits bool, status int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.checkLocked() {
		return false
	}
	if commits && !w.started {
		contentType := w.prepareMediaLocked()
		w.writer.Header().Set("Content-Type", contentType)
		w.writer.Header().Set("X-Content-Type-Options", "nosniff")
		w.started = status >= 200 || status == http.StatusSwitchingProtocols
	}
	w.inIO = true
	return true
}

// errTransportPanicked ends a read whose transport operation panicked.
var errTransportPanicked = errors.New("stream transport operation panicked")

// endIO clears the admitted operation and ends the read on failure. Callers
// defer it, so a panicking transport operation cannot leave inIO set and
// cause a later, spurious deadline expiry.
func (w *tokenStreamWriter) endIO(completed *bool, err *error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.inIO = false
	failure := *err
	if !*completed {
		failure = errTransportPanicked
	}
	if failure != nil {
		w.closeLocked()
	}
}

// end closes the read without an authority check, for request-context
// cancellation.
func (w *tokenStreamWriter) end() {
	w.mu.Lock()
	w.closeLocked()
	w.mu.Unlock()
}

func (w *tokenStreamWriter) check() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.checkLocked()
}

// All guarded stream routes carry SSE data or structured/plain errors. Pin
// their media before any operation can commit the transport, including Flush.
func (w *tokenStreamWriter) prepareMediaLocked() string {
	header := w.writer.Header()
	contentType := responseContentType(header, responseMediaEventStream)
	for key := range header {
		if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "X-Content-Type-Options") {
			delete(header, key)
		}
	}
	return contentType
}

func (w *tokenStreamWriter) Write(p []byte) (n int, err error) {
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if !w.beginIO(true, http.StatusOK) {
		return 0, io.ErrClosedPipe
	}
	completed := false
	defer w.endIO(&completed, &err)
	n, err = w.writer.Write(p)
	completed = true
	return n, err
}

func (w *tokenStreamWriter) WriteHeader(status int) {
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if !w.beginIO(true, status) {
		return
	}
	completed := false
	var err error
	defer w.endIO(&completed, &err)
	w.writer.WriteHeader(status)
	completed = true
}

func (w *tokenStreamWriter) Flush() {
	w.ioMu.Lock()
	defer w.ioMu.Unlock()
	if !w.beginIO(true, http.StatusOK) {
		return
	}
	completed := false
	var err error
	defer w.endIO(&completed, &err)
	w.flusher.Flush()
	completed = true
}

// close ends the read before the HTTP writer returns to the server. It
// expires a transport operation still in progress, so a stalled peer cannot
// hold the handler, then joins that operation.
func (w *tokenStreamWriter) close() {
	w.mu.Lock()
	w.closeLocked()
	w.mu.Unlock()

	w.ioMu.Lock()
	w.ioMu.Unlock() //nolint:staticcheck // joins the expired transport operation
}
