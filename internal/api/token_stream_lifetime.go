package api

import (
	"context"
	"io"
	"net/http"
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
		sessionAllowed := sessionReadRevalidator(r)
		allowed := func() bool {
			if revalidate == nil {
				// Proxy/basic/bypass admission stays with the route. A
				// session read retains its exact admitted session instance.
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
		watchDone := make(chan struct{})
		watch := func() {
			defer close(watchDone)
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !writer.check() {
						return
					}
				}
			}
		}
		if revalidate != nil || sessionAllowed != nil {
			go watch()
		} else {
			// No new periodic authentication checks for other credentials.
			close(watchDone)
		}
		defer func() {
			// Join any in-flight broadcaster write before returning the HTTP
			// writer, and reject queued writes even after handler cleanup.
			writer.close()
			<-watchDone
		}()
		next(writer, r.WithContext(ctx))
	}
}

// tokenStreamWriter deliberately exposes no Unwrap/ReaderFrom escape around
// the guard. It serializes writes, flushes, idle checks and final closure;
// update broadcasts use a different goroutine from the HTTP handler.
type tokenStreamWriter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	cancel  context.CancelFunc
	allowed func() bool
	mu      sync.Mutex
	closed  bool
}

func (w *tokenStreamWriter) Header() http.Header { return w.writer.Header() }

// Forward deadline control without exposing Unwrap, which would also let a
// caller bypass the disclosure guard. Assistant streams clear the server's
// initial deadlines and bound every subsequent write, including heartbeats.
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
		w.closed = true
		w.cancel()
		return false
	}
	return true
}

func (w *tokenStreamWriter) check() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.checkLocked()
}

func (w *tokenStreamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.checkLocked() {
		return 0, io.ErrClosedPipe
	}
	n, err := w.writer.Write(p)
	if err != nil {
		w.closed = true
		w.cancel()
	}
	return n, err
}

func (w *tokenStreamWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.checkLocked() {
		w.writer.WriteHeader(status)
	}
}

func (w *tokenStreamWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.checkLocked() {
		w.flusher.Flush()
	}
}

func (w *tokenStreamWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	w.cancel()
}
