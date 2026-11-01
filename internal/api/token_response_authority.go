package api

import (
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// retainTokenResponseAuthority is for slow, synchronous result handlers, after
// their normal auth/admin/scope gates. A token may be withdrawn while the
// handler waits for a provider. Keep the original work/context lifetime, but
// recheck authority at disclosure, including headers and error responses.
func retainTokenResponseAuthority(cfg *config.Config, scope string, instanceAdmin bool, next http.HandlerFunc) http.HandlerFunc {
	return retainResponseAuthority(cfg, scope, instanceAdmin, responseMediaJSON, next)
}

// responseMediaPolicy is a closed transport choice. Adding report downloads
// must not let a report renderer widen Assistant results or diagnostic ZIPs.
type responseMediaPolicy uint8

const (
	responseMediaJSON responseMediaPolicy = iota
	responseMediaZIP
	responseMediaReport
)

// retainResponseAuthority serves slow synchronous results and private downloads.
// It preserves the route's existing gates and work lifetime; the media policy
// selects only that transport's data formats, never arbitrary executable media.
func retainResponseAuthority(cfg *config.Config, scope string, instanceAdmin bool, media responseMediaPolicy, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		revalidate := tokenStreamSnapshotRevalidator(cfg, r, scope)
		sessionAllowed := sessionReadRevalidator(r)
		if revalidate == nil && sessionAllowed == nil {
			// No new authentication policy for proxy/basic/bypass requests.
			next(w, r)
			return
		}
		allowed := func() bool {
			if revalidate == nil {
				return sessionAllowed()
			}
			current, ok := revalidate()
			return ok && (!instanceAdmin || tokenActsForInstance(current))
		}
		if !allowed() {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		writer := &tokenResponseWriter{writer: w, header: w.Header().Clone(), allowed: allowed, media: media}
		defer writer.close()
		next(writer, r)
		// A bodyless handler must not implicitly return 200 after withdrawal.
		writer.WriteHeader(http.StatusOK)
	}
}

// Only synchronous HTTP response operations are exposed: no Flusher, Hijacker,
// ReaderFrom or Unwrap bypass. Payload bytes are never buffered. A denied first
// write produces a generic 401, not false-success/empty JSON; after any admitted
// response bytes, denial only stops further output, never appends an auth error.
type tokenResponseWriter struct {
	writer  http.ResponseWriter
	header  http.Header
	allowed func() bool
	media   responseMediaPolicy
	mu      sync.Mutex
	started bool
	closed  bool
}

func (w *tokenResponseWriter) Header() http.Header { return w.header }

func (w *tokenResponseWriter) permitLocked() bool {
	if w.closed {
		return false
	}
	if !w.allowed() {
		w.closed = true
		if !w.started {
			// Handler-specific headers remain private until an authorised
			// commit; preserve only the outer middleware's existing headers.
			http.Error(w.writer, "Authentication required", http.StatusUnauthorized)
		}
		return false
	}
	return true
}

func (w *tokenResponseWriter) commitLocked(status int) {
	if !w.started {
		header := w.writer.Header()
		clear(header)
		for key, values := range w.header {
			if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "X-Content-Type-Options") {
				continue
			}
			header[key] = append([]string(nil), values...)
		}
		// Results serve only their pinned media type or plain-text errors, never HTML.
		// Bind non-executable media to the actual transport, not just the
		// handler's private header map. Missing, malformed or unexpected
		// types must not allow HTTP sniffing or an extension to render HTML.
		// Leave payload bytes alone: HTML-escaping JSON would corrupt data.
		mediaType, _, err := mime.ParseMediaType(w.header.Get("Content-Type"))
		pinnedType := "text/plain; charset=utf-8"
		if err == nil {
			switch w.media {
			case responseMediaJSON:
				if mediaType == "application/json" {
					pinnedType = mediaType
				}
			case responseMediaZIP:
				if mediaType == "application/zip" {
					pinnedType = mediaType
				}
			case responseMediaReport:
				// Existing PDF/CSV attachments and structured API errors.
				// CSV is UTF-8; never forward renderer-supplied parameters.
				switch mediaType {
				case "application/pdf", "application/json":
					pinnedType = mediaType
				case "text/csv":
					pinnedType = "text/csv; charset=utf-8"
				}
			}
		}
		header.Set("Content-Type", pinnedType)
		header.Set("X-Content-Type-Options", "nosniff")
		w.writer.WriteHeader(status)
		// Informational headers do not commit the final response.
		w.started = status >= 200 || status == http.StatusSwitchingProtocols
	}
}

func (w *tokenResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.permitLocked() {
		w.commitLocked(status)
	}
}

func (w *tokenResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.permitLocked() {
		return 0, io.ErrClosedPipe
	}
	w.commitLocked(http.StatusOK)
	n, err := w.writer.Write(p)
	if err != nil {
		w.closed = true
	}
	return n, err
}

func (w *tokenResponseWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}
