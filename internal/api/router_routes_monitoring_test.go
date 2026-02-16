package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeprecatedV2ResourceHandler_RewritesPathAndSetsHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/resources/host-1/children", nil)

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/api/resources/host-1/children" {
			t.Fatalf("rewritten path = %q, want %q", r.URL.Path, "/api/resources/host-1/children")
		}
		w.WriteHeader(http.StatusOK)
	}

	deprecatedV2ResourceHandler(next)(rec, req)

	if !called {
		t.Fatalf("expected next handler to be called")
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want %q", got, "true")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
