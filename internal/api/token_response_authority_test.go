package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type tokenResponseStatusRecorder struct {
	*httptest.ResponseRecorder
	statuses []int
}

func (w *tokenResponseStatusRecorder) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
	w.ResponseRecorder.WriteHeader(status)
}

func TestTokenResponseWriterWithholdsWithdrawnHeadersAndBody(t *testing.T) {
	for _, explicitStatus := range []bool{false, true} {
		t.Run(fmt.Sprint(explicitStatus), func(t *testing.T) {
			base := &tokenResponseStatusRecorder{ResponseRecorder: httptest.NewRecorder()}
			base.Header().Set("X-Outer-Policy", "preserved")
			writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return false }}
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Length", "9999")
			writer.Header().Set("Content-Encoding", "synthetic")
			writer.Header().Set("X-Private-Result", assistantPrivateResult)
			if explicitStatus {
				writer.WriteHeader(http.StatusOK)
			}
			if n, err := writer.Write([]byte(assistantPrivateResult)); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("withdrawn write = %d, %v", n, err)
			}
			if base.Code != http.StatusUnauthorized || base.Body.String() != "Authentication required\n" || strings.Contains(base.Header().Get("Content-Type"), "json") {
				t.Fatalf("withdrawn response = %d %v %s", base.Code, base.Header(), base.Body)
			}
			if base.Header().Get("X-Outer-Policy") != "preserved" || base.Header().Get("X-Private-Result") != "" || base.Header().Get("Content-Encoding") != "" || base.Header().Get("Content-Length") != "" {
				t.Fatal("handler headers escaped before authorised commitment, or outer headers were lost")
			}
		})
	}
}

func TestTokenResponseWriterStopsAdmittedResponseWithoutAppendingError(t *testing.T) {
	base := httptest.NewRecorder()
	allowed := true
	writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return allowed }}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusAccepted)
	if _, err := writer.Write([]byte(`{"admitted":`)); err != nil {
		t.Fatal(err)
	}
	allowed = false
	if n, err := writer.Write([]byte(assistantPrivateResult)); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("withdrawn later write = %d, %v", n, err)
	}
	// Even a regrant must not resurrect an already refused response.
	allowed = true
	writer.WriteHeader(http.StatusInternalServerError)
	if _, err := writer.Write([]byte("late result")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("denied response reopened")
	}
	if base.Code != http.StatusAccepted || base.Body.String() != `{"admitted":` || base.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response changed after its admitted prefix: %d %s", base.Code, base.Body)
	}
}

func TestTokenResponseAuthorityBodylessClosureAndPanic(t *testing.T) {
	for _, stage := range []string{"authorised", "bodyless_withdrawal", "panic"} {
		t.Run(stage, func(t *testing.T) {
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, scopedReadToken, []string{config.ScopeAIExecute}, nil))
			var retained http.ResponseWriter
			original := httptest.NewRequest(http.MethodPost, "/api/ai/execute", nil)
			original.Header.Set("X-API-Token", scopedReadToken)
			handler := retainTokenResponseAuthority(cfg, config.ScopeAIExecute, true, func(w http.ResponseWriter, req *http.Request) {
				retained = w
				if req != original && req.Context().Done() != original.Context().Done() {
					t.Fatal("response policy changed request cancellation")
				}
				if stage == "bodyless_withdrawal" {
					config.Mu.Lock()
					cfg.RemoveAPIToken(cfg.APITokens[0].ID)
					config.Mu.Unlock()
				}
				if stage == "panic" {
					panic("synthetic handler failure")
				}
			})
			base := &tokenResponseStatusRecorder{ResponseRecorder: httptest.NewRecorder()}
			func() {
				defer func() {
					if r := recover(); (r != nil) != (stage == "panic") {
						t.Fatalf("handler panic changed: %v", r)
					}
				}()
				AuthContextMiddleware(cfg, nil, handler).ServeHTTP(base, original)
			}()
			if stage == "bodyless_withdrawal" && base.Code != http.StatusUnauthorized {
				t.Fatal("bodyless handler returned false success after withdrawal")
			}
			if stage == "panic" && len(base.statuses) != 0 {
				t.Fatal("guard wrote a response while a handler panic was unwinding")
			}
			for _, supported := range []bool{
				implementsResponseFlusher(retained),
				func() bool { _, ok := retained.(http.Hijacker); return ok }(),
				func() bool { _, ok := retained.(io.ReaderFrom); return ok }(),
				func() bool { _, ok := retained.(interface{ Unwrap() http.ResponseWriter }); return ok }(),
			} {
				if supported {
					t.Fatal("synchronous result writer exposed a bypass")
				}
			}
			if _, err := retained.Write([]byte("late result")); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("write after handler return = %v", err)
			}
		})
	}
}

func TestTokenResponseAuthorityRefusesStaleAdmissionBeforeWork(t *testing.T) {
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, scopedReadToken, []string{config.ScopeAIExecute}, nil))
	called := false
	guard := retainTokenResponseAuthority(cfg, config.ScopeAIExecute, true, func(http.ResponseWriter, *http.Request) { called = true })
	between := func(w http.ResponseWriter, r *http.Request) {
		config.Mu.Lock()
		cfg.RemoveAPIToken(cfg.APITokens[0].ID)
		config.Mu.Unlock()
		guard(w, r)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/execute", nil)
	req.Header.Set("X-API-Token", scopedReadToken)
	base := httptest.NewRecorder()
	AuthContextMiddleware(cfg, nil, between).ServeHTTP(base, req)
	if called || base.Code != http.StatusUnauthorized {
		t.Fatal("stale admission started work")
	}
}

func implementsResponseFlusher(w http.ResponseWriter) bool { _, ok := w.(http.Flusher); return ok }

func TestTokenResponseWriterKeepsTransportFailure(t *testing.T) {
	base := &failingWriter{}
	writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return true }}
	_, first := writer.Write([]byte("admitted"))
	if first == nil || errors.Is(first, io.ErrClosedPipe) {
		t.Fatalf("transport error changed: %v", first)
	}
	if _, err := writer.Write([]byte("later")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("transport failure did not close result writer")
	}
}
