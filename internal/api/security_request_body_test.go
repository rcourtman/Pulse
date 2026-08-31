package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestDecodeSecurityRequestBodyEnforcesWholeDocumentLimit(t *testing.T) {
	prefix := `{"value":"`
	suffix := `"}`
	exact := prefix + strings.Repeat("a", int(maxSecurityRequestBodyBytes)-len(prefix)-len(suffix)) + suffix

	var decoded struct {
		Value string `json:"value"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(exact))
	if err := decodeSecurityRequestBody(httptest.NewRecorder(), req, &decoded); err != nil {
		t.Fatalf("exact-limit body rejected: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(exact+" "))
	err := decodeSecurityRequestBody(httptest.NewRecorder(), req, &decoded)
	var maxBytesErr *http.MaxBytesError
	if !errors.As(err, &maxBytesErr) {
		t.Fatalf("over-limit trailing input error = %v, want *http.MaxBytesError", err)
	}
}

func TestLoginRejectsOversizedAndConcatenatedJSON(t *testing.T) {
	router := newLoginRouter(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "oversized trailing input",
			body:       `{"username":"admin","password":"wrong"}` + strings.Repeat(" ", int(maxSecurityRequestBodyBytes)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "concatenated objects",
			body:       `{"username":"admin","password":"wrong"}{"username":"admin","password":"Password!1"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(tt.body))
			req.RemoteAddr = "192.0.2.80:1234"
			rec := httptest.NewRecorder()

			router.handleLogin(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestPublicBootstrapHandlersRejectOversizedJSONBeforeCredentialUse(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		router := &Router{
			bootstrapTokenHash:              "configured",
			bootstrapTokenValidationLimiter: NewRateLimiter(10, 5*time.Minute),
		}
		t.Cleanup(router.bootstrapTokenValidationLimiter.Stop)

		body := `{"token":"` + strings.Repeat("a", int(maxSecurityRequestBodyBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(body))
		req.RemoteAddr = "192.0.2.81:1234"
		rec := httptest.NewRecorder()

		router.handleValidateBootstrapToken(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
		}
	})

	t.Run("quick setup", func(t *testing.T) {
		cfg := &config.Config{DataPath: t.TempDir(), ConfigPath: t.TempDir()}
		router := &Router{config: cfg}
		clientIP := "192.0.2.82"
		authLimiter.Reset(clientIP)
		t.Cleanup(func() { authLimiter.Reset(clientIP) })

		body := `{"username":"` + strings.Repeat("a", int(maxSecurityRequestBodyBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
		req.RemoteAddr = clientIP + ":1234"
		rec := httptest.NewRecorder()

		handleQuickSecuritySetupFixed(router)(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
		}
		if cfg.AuthUser != "" || cfg.AuthPass != "" || cfg.HasAPITokens() {
			t.Fatalf("oversized request changed auth state: user=%q pass_set=%v tokens=%d", cfg.AuthUser, cfg.AuthPass != "", len(cfg.APITokens))
		}
	})

	t.Run("recovery", func(t *testing.T) {
		router := &Router{mux: http.NewServeMux(), config: &config.Config{}}
		router.registerAuthSecurityInstallRoutes()

		body := `{"action":"` + strings.Repeat("a", int(maxSecurityRequestBodyBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()

		router.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
		}
	})
}

func FuzzDecodeSecurityRequestBody(f *testing.F) {
	f.Add(`{"username":"admin","password":"Password!1"}`)
	f.Add(`{"username":"admin"}{"password":"Password!1"}`)
	f.Add("{")
	f.Add("")

	f.Fuzz(func(t *testing.T, body string) {
		var decoded map[string]any
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		err := decodeSecurityRequestBody(httptest.NewRecorder(), req, &decoded)
		if err == nil && int64(len(body)) > maxSecurityRequestBodyBytes {
			t.Fatalf("accepted %d-byte body above %d-byte limit", len(body), maxSecurityRequestBodyBytes)
		}
	})
}
