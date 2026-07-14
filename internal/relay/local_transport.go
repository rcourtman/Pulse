package relay

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// handlerTransport dispatches proxied relay requests to the local Pulse HTTP
// handler in-process instead of dialing the main listener over loopback. The
// relay client runs inside the Pulse server process, and a loopback dial is
// not faithful to that listener: with HTTPS_ENABLED the listener serves TLS
// (a plaintext dial gets a bare 400), and with a non-loopback BIND_ADDRESS
// the 127.0.0.1 dial cannot connect at all. Serving the handler directly
// traverses the exact middleware chain the real listener serves.
type handlerTransport struct {
	handler http.Handler
}

func newHandlerTransport(handler http.Handler) *handlerTransport {
	return &handlerTransport{handler: handler}
}

func (t *handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.handler == nil {
		return nil, fmt.Errorf("relay local handler transport is not configured")
	}

	outReq := req.Clone(req.Context())
	// Server-side requests carry a RequestURI; some handlers read it.
	outReq.RequestURI = req.URL.RequestURI()
	if outReq.RemoteAddr == "" {
		// Match the loopback dial this transport replaces so address-keyed
		// middleware (rate limits, lockouts, audit logs) behaves the same.
		outReq.RemoteAddr = "127.0.0.1:0"
	}
	if outReq.Body == nil {
		outReq.Body = http.NoBody
	}

	pr, pw := io.Pipe()
	rw := newPipeResponseWriter(pw)

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				rw.abort(fmt.Errorf("relay local handler panicked: %v", recovered))
				return
			}
			rw.finish()
		}()
		t.handler.ServeHTTP(rw, outReq)
	}()

	select {
	case <-rw.committed():
	case <-req.Context().Done():
		_ = pr.CloseWithError(req.Context().Err())
		return nil, req.Context().Err()
	}

	status, header := rw.committedResponse()
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          pr,
		ContentLength: -1,
		Request:       req,
	}, nil
}

// pipeResponseWriter adapts an http.Handler response onto an io.Pipe so the
// proxy's http.Client sees headers as soon as the handler commits them and
// body bytes as the handler writes them (required for SSE streaming).
//
// Header/WriteHeader/Write/Flush and finish/abort are only called from the
// handler goroutine; committedResponse is only read after the commit channel
// closes, which orders it after the commit writes.
type pipeResponseWriter struct {
	pw *io.PipeWriter

	header http.Header

	commitOnce sync.Once
	commitCh   chan struct{}
	status     int
	snapshot   http.Header
}

func newPipeResponseWriter(pw *io.PipeWriter) *pipeResponseWriter {
	return &pipeResponseWriter{
		pw:       pw,
		header:   make(http.Header),
		commitCh: make(chan struct{}),
	}
}

func (w *pipeResponseWriter) Header() http.Header {
	return w.header
}

func (w *pipeResponseWriter) WriteHeader(status int) {
	w.commitOnce.Do(func() {
		w.status = status
		w.snapshot = w.header.Clone()
		close(w.commitCh)
	})
}

func (w *pipeResponseWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.pw.Write(b)
}

// Flush satisfies http.Flusher for SSE handlers; pipe writes are already
// delivered to the reader synchronously, so there is nothing to flush.
func (w *pipeResponseWriter) Flush() {
	w.WriteHeader(http.StatusOK)
}

func (w *pipeResponseWriter) finish() {
	w.WriteHeader(http.StatusOK)
	_ = w.pw.Close()
}

func (w *pipeResponseWriter) abort(err error) {
	w.commitOnce.Do(func() {
		w.status = http.StatusInternalServerError
		w.snapshot = w.header.Clone()
		close(w.commitCh)
	})
	_ = w.pw.CloseWithError(err)
}

func (w *pipeResponseWriter) committed() <-chan struct{} {
	return w.commitCh
}

func (w *pipeResponseWriter) committedResponse() (int, http.Header) {
	return w.status, w.snapshot
}
