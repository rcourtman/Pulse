package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const securityValidationRaw = "synthetic-security-validation.12345678"

// These are the two security discovery/validation paths, not the normal auth
// middleware. They must share its exclusive token metadata writer even when
// another request is only reading the token inventory under config.Mu.RLock.
func concurrentSecurityTokenValidation(t *testing.T, cfg *config.Config, validate func(worker, iteration int) error) {
	t.Helper()
	const workers, iterations = 8, 8
	start := make(chan struct{})
	errors := make(chan error, workers+1)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Go(func() {
			<-start
			for iteration := 0; iteration < iterations; iteration++ {
				if err := validate(worker, iteration); err != nil {
					errors <- err
					return
				}
			}
		})
	}
	wg.Go(func() {
		<-start
		for i := 0; i < workers*iterations; i++ {
			config.Mu.RLock()
			record := cfg.APITokens[0].Clone()
			valid := cfg.IsValidAPIToken(securityValidationRaw)
			config.Mu.RUnlock()
			if !valid || !record.HasScope(config.ScopeMonitoringRead) {
				errors <- fmt.Errorf("concurrent inventory read lost the original token authority")
				return
			}
		}
	})
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	config.Mu.RLock()
	used := cfg.APITokens[0].LastUsedAt != nil
	config.Mu.RUnlock()
	if !used {
		t.Error("successful validation no longer updates token usage")
	}
}

func TestSecurityStatusTokenValidationConcurrent(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	record := newTokenRecord(t, securityValidationRaw, []string{config.ScopeMonitoringRead, config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	wantHint := cfg.PrimaryAPITokenHint()
	concurrentSecurityTokenValidation(t, cfg, func(_, _ int) error {
		req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
		req.Header.Set("X-API-Token", securityValidationRaw)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		var result privilegedSecurityStatusResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			return fmt.Errorf("status response status=%d: %w", rec.Code, err)
		}
		if rec.Code != http.StatusOK || result.DetailLevel != securityStatusDetailPrivileged ||
			!slices.Equal(result.TokenScopes, record.Scopes) || result.APITokenHint != wantHint ||
			!result.SettingsCapabilities.APIAccessRead || result.SettingsCapabilities.APIAccessWrite {
			return fmt.Errorf("security status changed its admitted scope, capability or hint response")
		}
		return nil
	})
}

func TestValidateTokenMetadataConcurrent(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	cfg := newTestConfigWithTokens(t, newTokenRecord(t, securityValidationRaw, []string{config.ScopeMonitoringRead}, nil))
	cfg.ProxyAuthSecret = "synthetic-security-validation-proxy"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	router := &Router{config: cfg}
	// Each synthetic client makes one request, below the real rate limit. The
	// proxy authenticates the administrator independently of the tested token.
	for i := 1; i <= 64; i++ {
		ip := fmt.Sprintf("198.51.100.%d", i)
		authLimiter.Reset(ip)
		t.Cleanup(func() { authLimiter.Reset(ip) })
	}
	concurrentSecurityTokenValidation(t, cfg, func(worker, iteration int) error {
		req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"`+securityValidationRaw+`"}`))
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", worker*8+iteration+1)
		req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "synthetic-admin")
		rec := httptest.NewRecorder()
		router.HandleValidateAPIToken(rec, req)
		var result struct {
			Valid   bool   `json:"valid"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			return fmt.Errorf("validation response status=%d: %w", rec.Code, err)
		}
		if rec.Code != http.StatusOK || !result.Valid || result.Message != "Token is valid" {
			return fmt.Errorf("validation changed: status=%d valid=%v message=%q", rec.Code, result.Valid, result.Message)
		}
		return nil
	})
}

func TestSecurityStatusTokenSnapshotStaysDetachedAndRevocable(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()
	t.Cleanup(resetAdminBypassState)
	record := newTokenRecord(t, securityValidationRaw, []string{config.ScopeMonitoringRead}, map[string]string{apiTokenMetadataOwnerUserID: "original-owner"})
	record.OrgIDs = []string{"original-org"}
	cfg := newTestConfigWithTokens(t, record)
	router := &Router{config: cfg}
	request := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
		req.Header.Set("X-API-Token", securityValidationRaw)
		return req
	}
	snapshot := router.buildSecurityStatusAuthSnapshot(request())
	if !snapshot.authenticated || snapshot.tokenRecord == nil {
		t.Fatal("initial token did not authenticate")
	}
	config.Mu.Lock()
	cfg.APITokens[0].Scopes[0] = config.ScopeSettingsWrite
	cfg.APITokens[0].OrgIDs[0] = "replacement-org"
	cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID] = "replacement-owner"
	config.Mu.Unlock()
	for _, admitted := range []*config.APITokenRecord{snapshot.tokenRecord, getAPITokenRecordFromRequest(snapshot.request)} {
		if admitted == nil || !admitted.HasScope(config.ScopeMonitoringRead) || admitted.HasScope(config.ScopeSettingsWrite) ||
			admitted.OrgIDs[0] != "original-org" || apiTokenOwnerUserID(*admitted) != "original-owner" {
			t.Fatal("live token edit changed the detached admission snapshot")
		}
	}
	config.Mu.Lock()
	past := time.Now().Add(-time.Minute)
	cfg.APITokens[0].ExpiresAt = &past
	config.Mu.Unlock()
	if router.buildSecurityStatusAuthSnapshot(request()).authenticated {
		t.Fatal("expired token authenticated")
	}
	config.Mu.Lock()
	cfg.RemoveAPIToken(record.ID)
	config.Mu.Unlock()
	if router.buildSecurityStatusAuthSnapshot(request()).authenticated {
		t.Fatal("removed token authenticated")
	}
}
