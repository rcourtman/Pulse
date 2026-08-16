package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// gzipMinContentLength skips compression for responses that declare a body
// too small for gzip to pay for its own header and CPU cost. Responses with
// no declared Content-Length (e.g. streamed JSON) are still compressed.
const gzipMinContentLength = 1024

var gzipWriterPool = sync.Pool{
	New: func() any {
		// BestSpeed keeps CPU cost low on multi-megabyte state payloads while
		// still collapsing repetitive monitoring JSON by roughly an order of
		// magnitude.
		gw, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return gw
	},
}

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
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
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
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		encoding := part
		if idx := strings.IndexByte(encoding, ';'); idx >= 0 {
			// A quality value of 0 opts the encoding out.
			params := strings.TrimSpace(encoding[idx+1:])
			if strings.HasPrefix(params, "q=0") && !strings.HasPrefix(params, "q=0.") {
				continue
			}
			encoding = encoding[:idx]
		}
		if strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
			return true
		}
	}
	return false
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
	if w.decided {
		return
	}
	w.decided = true

	header := w.Header()
	header.Add("Vary", "Accept-Encoding")

	if status == http.StatusNoContent || status == http.StatusNotModified {
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
	if w.gz == nil {
		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(w.ResponseWriter)
		w.gz = gz
	}
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
		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(w.ResponseWriter)
		w.gz = gz
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

func (w *gzipResponseWriter) close() {
	if w.gz == nil {
		return
	}
	_ = w.gz.Close()
	w.gz.Reset(io.Discard)
	gzipWriterPool.Put(w.gz)
	w.gz = nil
}
