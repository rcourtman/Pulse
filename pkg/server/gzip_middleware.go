package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

// gzipMinContentLength skips compression for responses that declare a body
// too small for gzip to pay for its own header and CPU cost. Responses with
// no declared Content-Length (e.g. streamed JSON) are still compressed.
const gzipMinContentLength = 1024

// gzipWriterPoolCapacity bounds idle compression state retained after bursts.
// A BestSpeed writer owns roughly a MiB of working memory. sync.Pool has no
// cardinality limit, so a burst of concurrent API responses can leave hundreds
// of MiB resident until enough GC/scavenger cycles happen to release it. Eight
// writers cover ordinary parallel browser requests without making a past
// concurrency spike the server's steady-state memory floor.
const gzipWriterPoolCapacity = 8

type boundedGzipWriterPool struct {
	idle chan *gzip.Writer
}

func newBoundedGzipWriterPool(capacity int) *boundedGzipWriterPool {
	if capacity < 0 {
		capacity = 0
	}
	return &boundedGzipWriterPool{idle: make(chan *gzip.Writer, capacity)}
}

func (p *boundedGzipWriterPool) get() *gzip.Writer {
	select {
	case gw := <-p.idle:
		return gw
	default:
		// BestSpeed keeps CPU cost low on multi-megabyte state payloads while
		// still collapsing repetitive monitoring JSON by roughly an order of
		// magnitude. The level is a constant accepted by compress/gzip.
		gw, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			panic("server: unsupported gzip compression level")
		}
		return gw
	}
}

func (p *boundedGzipWriterPool) put(gw *gzip.Writer) {
	if gw == nil {
		return
	}
	gw.Reset(io.Discard)
	select {
	case p.idle <- gw:
	default:
		// Do not retain compression state beyond the explicit idle budget.
	}
}

var gzipWriterPool = newBoundedGzipWriterPool(gzipWriterPoolCapacity)

// gzipCompressibleContentTypes lists the response types worth compressing.
// Everything else (images, fonts, event streams, already-compressed archives)
// passes through untouched. text/event-stream is deliberately absent: SSE
// responses flush per event and must not be re-buffered by a compressor.
var gzipCompressibleContentTypes = []string{
	"application/json",
	"application/javascript",
	"text/javascript",
	"text/html",
	"text/css",
	"text/plain",
	"image/svg+xml",
}

// withGzip compresses eligible responses for clients that advertise gzip
// support. WebSocket upgrades and Range requests bypass the wrapper entirely
// so hijacking and byte-range semantics keep working on the original writer.
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !acceptsGzip(r) || r.Header.Get("Range") != "" {
			// Both variants must carry Vary so a shared cache never serves a
			// compressed body to a client that cannot decode it.
			w.Header().Add("Vary", "Accept-Encoding")
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

func acceptsGzip(r *http.Request) bool {
	values := r.Header.Values("Accept-Encoding")
	if len(values) == 0 {
		return false
	}

	gzipQuality := -1
	wildcardQuality := -1
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			encoding, quality := parseAcceptedEncoding(part)
			switch encoding {
			case "gzip":
				if quality > gzipQuality {
					gzipQuality = quality
				}
			case "*":
				if quality > wildcardQuality {
					wildcardQuality = quality
				}
			}
		}
	}

	// An explicit gzip entry overrides the wildcard, including q=0.
	if gzipQuality >= 0 {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
}

// parseAcceptedEncoding returns the normalized content-coding and a quality
// value in thousandths. Invalid qvalues make that entry unacceptable. RFC
// 9110 permits at most three fractional digits and only zeroes after 1.
func parseAcceptedEncoding(part string) (string, int) {
	segments := strings.Split(part, ";")
	encoding := strings.ToLower(strings.TrimSpace(segments[0]))
	if encoding == "" {
		return "", 0
	}

	quality := 1000
	sawQuality := false
	for _, parameter := range segments[1:] {
		name, value, found := strings.Cut(parameter, "=")
		if !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		if !found || sawQuality {
			return encoding, 0
		}
		sawQuality = true
		parsed, ok := parseQualityValue(strings.TrimSpace(value))
		if !ok {
			return encoding, 0
		}
		quality = parsed
	}
	return encoding, quality
}

func parseQualityValue(value string) (int, bool) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	if len(fraction) > 3 {
		return 0, false
	}
	if hasFraction {
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return 0, false
			}
		}
	}

	switch whole {
	case "0":
		for len(fraction) < 3 {
			fraction += "0"
		}
		if fraction == "" {
			return 0, true
		}
		quality, err := strconv.Atoi(fraction)
		return quality, err == nil
	case "1":
		if strings.Trim(fraction, "0") != "" {
			return 0, false
		}
		return 1000, true
	default:
		return 0, false
	}
}

// gzipResponseWriter defers the compress-or-not decision to the first write,
// when the handler has set its response headers. It implements http.Flusher
// (SSE and other streaming handlers rely on it) and exposes Unwrap for
// http.ResponseController.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	compressing bool
	decided     bool
}

func (w *gzipResponseWriter) decide(status int) {
	if w.decided || status >= 100 && status < 200 {
		// Informational responses do not select the representation for the final
		// response. Keep the decision open so a later 2xx-5xx status can use the
		// headers and body semantics the handler establishes for that response.
		return
	}
	w.decided = true

	header := w.Header()
	header.Add("Vary", "Accept-Encoding")

	switch status {
	case http.StatusNoContent, http.StatusResetContent, http.StatusPartialContent, http.StatusNotModified:
		return
	}
	if header.Get("Content-Encoding") != "" {
		return
	}
	if lengthValue := header.Get("Content-Length"); lengthValue != "" {
		if length, err := strconv.ParseInt(lengthValue, 10, 64); err == nil && length < gzipMinContentLength {
			return
		}
	}
	contentType := header.Get("Content-Type")
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = contentType[:idx]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	compressible := false
	for _, candidate := range gzipCompressibleContentTypes {
		if contentType == candidate {
			compressible = true
			break
		}
	}
	if !compressible {
		return
	}

	// The compressed size is unknown, so any declared length is stale.
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	w.compressing = true
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.decide(status)
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.decided {
		w.decide(http.StatusOK)
	}
	if !w.compressing {
		return w.ResponseWriter.Write(data)
	}
	w.ensureGzipWriter()
	return w.gz.Write(data)
}

func (w *gzipResponseWriter) Flush() {
	// A flush commits the response headers (implicit 200), so the
	// compress-or-not decision must be made now. Deciding later, after the
	// headers are on the wire, would gzip the body without ever sending
	// Content-Encoding.
	if !w.decided {
		w.decide(http.StatusOK)
	}
	if w.compressing && w.gz == nil {
		w.ensureGzipWriter()
	}
	if w.gz != nil {
		if err := w.gz.Flush(); err != nil {
			return
		}
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) ensureGzipWriter() {
	if w.gz != nil {
		return
	}
	gz := gzipWriterPool.get()
	gz.Reset(w.ResponseWriter)
	w.gz = gz
}

func (w *gzipResponseWriter) close() {
	// WriteHeader may select gzip without a subsequent Write. Emit a complete
	// empty gzip member so Content-Encoding never describes an absent stream.
	if w.compressing && w.gz == nil {
		w.ensureGzipWriter()
	}
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	gzipWriterPool.put(w.gz)
	w.gz = nil
}
