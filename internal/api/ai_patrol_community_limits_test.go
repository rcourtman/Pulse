package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

type communityLicenseChecker struct{}

func (c communityLicenseChecker) HasFeature(feature string) bool {
	// Community tier: ai_patrol is allowed, but investigation/autofix is Pro-gated.
	if feature == license.FeatureAIAutoFix {
		return false
	}
	return true
}

func (c communityLicenseChecker) GetLicenseStateString() (string, bool) {
	// "none" is the closest representation of Community/free in this interface.
	return "none", false
}

func TestReinvestigateCommunityReturns402(t *testing.T) {
	rawToken := "ai-reinvestigate-community-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/f-1/reinvestigate", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for Community reinvestigate, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPatrolCommunityAutonomyLockedToMonitor(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetLicenseChecker(communityLicenseChecker{})

	// Persist a higher autonomy level to ensure read-time clamping works.
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// GET should clamp effective autonomy to monitor for Community.
	getReq := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/autonomy", nil)
	getRec := httptest.NewRecorder()
	handler.HandleGetPatrolAutonomy(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var getResp PatrolAutonomyResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if getResp.AutonomyLevel != config.PatrolAutonomyMonitor {
		t.Fatalf("expected autonomy %q for Community, got %q", config.PatrolAutonomyMonitor, getResp.AutonomyLevel)
	}

	// PUT should reject any autonomy above monitor for Community.
	body := `{"autonomy_level":"approval","investigation_budget":10,"investigation_timeout_sec":120}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(body))
	putRec := httptest.NewRecorder()
	handler.HandleUpdatePatrolAutonomy(putRec, putReq)
	if putRec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for Community autonomy update, got %d: %s", putRec.Code, putRec.Body.String())
	}
}
