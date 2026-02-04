package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newLoginRouter(t *testing.T) *Router {
	t.Helper()
	hashed, err := auth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	return NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
}

func TestLoginRateLimitEnforced(t *testing.T) {
	router := newLoginRouter(t)
	ip := "203.0.113.200"
	authLimiter.Reset(ip)
	defer authLimiter.Reset(ip)

	body := `{"username":"admin","password":"Password!1","rememberMe":false}`

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
		req.RemoteAddr = ip + ":12345"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected %d, got %d (%s)", i+1, http.StatusOK, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	req.RemoteAddr = ip + ":12345"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected %d, got %d (%s)", http.StatusTooManyRequests, rec.Code, rec.Body.String())
	}
}
