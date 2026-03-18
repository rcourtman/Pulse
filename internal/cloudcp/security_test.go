package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
)

func TestCPSecurityHeaders_SetsCSPWithNonce(t *testing.T) {
	var capturedNonce string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedNonce = cpsec.NonceFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := CPSecurityHeaders(inner)
	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedNonce == "" {
		t.Fatal("expected nonce in context, got empty")
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header")
	}
	if !strings.Contains(csp, "'nonce-"+capturedNonce+"'") {
		t.Fatalf("CSP does not contain nonce: %q", csp)
	}
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("CSP should not contain unsafe-inline when nonce is present: %q", csp)
	}
	if strings.Contains(csp, "'unsafe-eval'") {
		t.Fatalf("CSP should not contain unsafe-eval: %q", csp)
	}
}

func TestCPSecurityHeaders_CSPDirectives(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CPSecurityHeaders(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")

	requiredDirectives := []string{
		"default-src 'self'",
		"script-src 'self' 'nonce-",
		"style-src 'self' 'nonce-",
		"img-src 'self' data:",
		"connect-src 'self'",
		"font-src 'self'",
		"form-action 'self' https:",
		"frame-ancestors 'none'",
	}
	for _, d := range requiredDirectives {
		if !strings.Contains(csp, d) {
			t.Errorf("CSP missing directive %q in: %q", d, csp)
		}
	}
}

func TestCPSecurityHeaders_SetsOtherHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CPSecurityHeaders(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "0",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for k, want := range expected {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}

	pp := rec.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Error("expected Permissions-Policy header")
	}
}

func TestCPSecurityHeaders_UniqueNoncePerRequest(t *testing.T) {
	var nonces []string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonces = append(nonces, cpsec.NonceFromContext(r.Context()))
		w.WriteHeader(http.StatusOK)
	})

	handler := CPSecurityHeaders(inner)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	seen := make(map[string]bool)
	for _, n := range nonces {
		if n == "" {
			t.Fatal("got empty nonce")
		}
		if seen[n] {
			t.Fatalf("duplicate nonce: %q", n)
		}
		seen[n] = true
	}
}

func TestCPCSPNonce_ContextRoundTrip(t *testing.T) {
	// Verify the cpsec package round-trips correctly.
	ctx := cpsec.WithNonce(t.Context(), "test-nonce-123")
	if got := cpsec.NonceFromContext(ctx); got != "test-nonce-123" {
		t.Fatalf("NonceFromContext = %q, want %q", got, "test-nonce-123")
	}
}

func TestCPCSPNonce_EmptyContext(t *testing.T) {
	if got := cpsec.NonceFromContext(t.Context()); got != "" {
		t.Fatalf("NonceFromContext on bare context = %q, want empty", got)
	}
}
