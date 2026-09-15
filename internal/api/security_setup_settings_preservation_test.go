package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuickSecuritySetupForcePreservesSystemSettings(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	t.Setenv("PULSE_DOCKER", "true")
	resetTrustedProxyConfig()
	dir := t.TempDir()
	InitPersistentAuthStores(dir)
	hash, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("ab", 32)
	record, err := config.NewAPITokenRecord(token, "synthetic", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AuthUser: "admin", AuthPass: hash, DataPath: dir, ConfigPath: dir, APITokens: []config.APITokenRecord{*record}}
	cfg.SortAPITokens()
	persistence := config.NewConfigPersistence(dir)
	before := config.DefaultSystemSettings()
	before.FullWidthMode = true
	before.ConnectionTimeout = 23
	if err := persistence.SaveSystemSettings(*before); err != nil {
		t.Fatal(err)
	}
	router := &Router{config: cfg, persistence: persistence}
	authLimiter.Reset("203.0.113.10")
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(`{"username":"rotated","password":"RotatedPassword!1","apiToken":"`+token+`","force":true}`))
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-API-Token", token)
	response := httptest.NewRecorder()
	handleQuickSecuritySetupFixed(router)(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("setup returned %d", response.Code)
	}
	after, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after == nil {
		t.Fatal("settings missing")
	}
	if !after.FullWidthMode || after.ConnectionTimeout != 23 {
		t.Fatalf("non-auth settings overwritten: fullWidthMode=%v connectionTimeout=%v", after.FullWidthMode, after.ConnectionTimeout)
	}
}
