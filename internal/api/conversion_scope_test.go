package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestConversionConfigReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "conversion-config-read-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/conversion/config", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assertMissingScope(t, rec, config.ScopeSettingsRead, req.Method+" "+req.URL.Path)
}

func TestConversionConfigWriteRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "conversion-config-write-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPut, "/api/conversion/config", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	assertMissingScope(t, rec, config.ScopeSettingsWrite, req.Method+" "+req.URL.Path)
}
