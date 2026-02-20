package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signHandoffToken(t *testing.T, key []byte, claims cloudHandoffClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

func makeExchangeRequest(t *testing.T, handler http.HandlerFunc, host, token string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	if token != "" {
		form.Set("token", token)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = host
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestIsSQLiteUniqueViolation(t *testing.T) {
	if isSQLiteUniqueViolation(nil) {
		t.Fatal("expected nil error to return false")
	}
	if !isSQLiteUniqueViolation(errors.New("UNIQUE constraint failed: handoff_jti.jti")) {
		t.Fatal("expected UNIQUE constraint failure to return true")
	}
	if !isSQLiteUniqueViolation(errors.New("constraint failed")) {
		t.Fatal("expected generic constraint failure to return true")
	}
	if isSQLiteUniqueViolation(errors.New("some other error")) {
		t.Fatal("expected unrelated error to return false")
	}
}

func TestTenantIDFromRequest(t *testing.T) {
	t.Run("uses env var when present", func(t *testing.T) {
		t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		t.Setenv("PULSE_TENANT_ID", "env-tenant")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "host-tenant.example.com"
		req.RemoteAddr = "198.51.100.10:4567"

		if got := tenantIDFromRequest(req); got != "env-tenant" {
			t.Fatalf("tenantIDFromRequest() = %q, want %q", got, "env-tenant")
		}
	})

	t.Run("extracts subdomain from loopback host", func(t *testing.T) {
		t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		t.Setenv("PULSE_TENANT_ID", "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "tenant.example.com:8443"
		req.RemoteAddr = "127.0.0.1:8080"

		if got := tenantIDFromRequest(req); got != "tenant" {
			t.Fatalf("tenantIDFromRequest() = %q, want %q", got, "tenant")
		}
	})

	t.Run("returns full loopback host when no dot exists", func(t *testing.T) {
		t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		t.Setenv("PULSE_TENANT_ID", "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "localhost"
		req.RemoteAddr = "127.0.0.1:8080"

		if got := tenantIDFromRequest(req); got != "localhost" {
			t.Fatalf("tenantIDFromRequest() = %q, want %q", got, "localhost")
		}
	})

	t.Run("extracts tenant from trusted proxy forwarded host", func(t *testing.T) {
		t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
		resetTrustedProxyConfig()
		t.Setenv("PULSE_TENANT_ID", "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.10:9443"
		req.Host = "ignored.example.com"
		req.Header.Set("X-Forwarded-Host", "proxy-tenant.example.com")

		if got := tenantIDFromRequest(req); got != "proxy-tenant" {
			t.Fatalf("tenantIDFromRequest() = %q, want %q", got, "proxy-tenant")
		}
	})

	t.Run("ignores untrusted remote host header", func(t *testing.T) {
		t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
		resetTrustedProxyConfig()
		t.Setenv("PULSE_TENANT_ID", "")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.20:1234"
		req.Host = "tenant.example.com"

		if got := tenantIDFromRequest(req); got != "" {
			t.Fatalf("tenantIDFromRequest() = %q, want empty", got)
		}
	})
}

func TestJTIReplayStoreCheckAndStore(t *testing.T) {
	store := &jtiReplayStore{configDir: t.TempDir()}
	expires := time.Now().Add(time.Hour)

	stored, err := store.checkAndStore("abc123", expires)
	if err != nil {
		t.Fatalf("first checkAndStore() error = %v", err)
	}
	if !stored {
		t.Fatal("first checkAndStore() = false, want true")
	}

	stored, err = store.checkAndStore("abc123", expires)
	if err != nil {
		t.Fatalf("second checkAndStore() error = %v", err)
	}
	if stored {
		t.Fatal("second checkAndStore() = true, want false")
	}

	_, err = store.checkAndStore("   ", expires)
	if err == nil {
		t.Fatal("expected empty jti to return error")
	}
}

func TestHandleHandoffExchange(t *testing.T) {
	key := []byte("test-handoff-key")
	configDir := t.TempDir()
	secretsDir := filepath.Join(configDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "handoff.key"), key, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	handler := HandleHandoffExchange(configDir)
	tenantID := "tenant-a"
	host := tenantID + ".example.com"

	t.Run("missing token returns bad request", func(t *testing.T) {
		t.Setenv("PULSE_TENANT_ID", "")
		rec := makeExchangeRequest(t, handler, host, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid token returns unauthorized", func(t *testing.T) {
		t.Setenv("PULSE_TENANT_ID", "")
		rec := makeExchangeRequest(t, handler, host, "not-a-jwt")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("missing tenant context returns internal error", func(t *testing.T) {
		t.Setenv("PULSE_TENANT_ID", "")
		rec := makeExchangeRequest(t, handler, "", "anything")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("missing exp claim returns unauthorized", func(t *testing.T) {
		t.Setenv("PULSE_TENANT_ID", "")
		token := signHandoffToken(t, key, cloudHandoffClaims{
			AccountID: "acct-1",
			Email:     "user@example.com",
			Role:      "admin",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "jti-no-exp",
				Subject:   "user-1",
				Issuer:    cloudHandoffIssuer,
				Audience:  jwt.ClaimStrings{tenantID},
				ExpiresAt: nil,
			},
		})

		rec := makeExchangeRequest(t, handler, host, token)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("successful exchange and replay rejection", func(t *testing.T) {
		t.Setenv("PULSE_TENANT_ID", "")
		token := signHandoffToken(t, key, cloudHandoffClaims{
			AccountID: "acct-123",
			Email:     "user@example.com",
			Role:      "owner",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "jti-success",
				Subject:   "user-123",
				Issuer:    cloudHandoffIssuer,
				Audience:  jwt.ClaimStrings{tenantID},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})

		first := makeExchangeRequest(t, handler, host, token)
		if first.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", first.Code, http.StatusOK)
		}
		if got := first.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want %q", got, "application/json")
		}

		var payload map[string]any
		if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil {
			t.Fatalf("response json decode error = %v", err)
		}
		if ok, _ := payload["ok"].(bool); !ok {
			t.Fatalf("payload ok = %v, want true", payload["ok"])
		}
		if got, _ := payload["tenant_id"].(string); got != tenantID {
			t.Fatalf("tenant_id = %q, want %q", got, tenantID)
		}
		if got, _ := payload["account_id"].(string); got != "acct-123" {
			t.Fatalf("account_id = %q, want %q", got, "acct-123")
		}

		second := makeExchangeRequest(t, handler, host, token)
		if second.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", second.Code, http.StatusUnauthorized)
		}
	})
}

func TestHandleHandoffExchangeKeyMissing(t *testing.T) {
	handler := HandleHandoffExchange(t.TempDir())
	t.Setenv("PULSE_TENANT_ID", "")

	rec := makeExchangeRequest(t, handler, "tenant.example.com", "anything")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
