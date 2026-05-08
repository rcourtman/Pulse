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
	aiCfg.PatrolFullModeUnlocked = true
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
	if getResp.FullModeUnlocked {
		t.Fatalf("expected full mode to be locked for Community, got %+v", getResp)
	}

	// PUT via free adapter should allow findings-only monitor settings and
	// persist the same clamped runtime limits used by the enterprise handler.
	freeAdapter := aiAutoFixFreeAdapter{handler: handler}
	body := `{"autonomy_level":"monitor","full_mode_unlocked":true,"investigation_budget":2,"investigation_timeout_sec":30}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(body))
	putRec := httptest.NewRecorder()
	freeAdapter.HandleUpdatePatrolAutonomy(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected monitor update to be allowed for Community, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var putResp struct {
		Success  bool                   `json:"success"`
		Settings PatrolAutonomyResponse `json:"settings"`
	}
	if err := json.Unmarshal(putRec.Body.Bytes(), &putResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if !putResp.Success {
		t.Fatalf("expected successful update response, got %+v", putResp)
	}
	if putResp.Settings.AutonomyLevel != config.PatrolAutonomyMonitor {
		t.Fatalf("expected saved autonomy %q, got %q", config.PatrolAutonomyMonitor, putResp.Settings.AutonomyLevel)
	}
	if putResp.Settings.FullModeUnlocked {
		t.Fatalf("expected monitor-only update to clear full mode, got %+v", putResp.Settings)
	}
	if putResp.Settings.InvestigationBudget != 5 || putResp.Settings.InvestigationTimeoutSec != 60 {
		t.Fatalf("expected clamped budget/timeout 5/60, got %d/%d",
			putResp.Settings.InvestigationBudget, putResp.Settings.InvestigationTimeoutSec)
	}
	saved, err := persistence.LoadAIConfig()
	if err != nil {
		t.Fatalf("LoadAIConfig: %v", err)
	}
	if saved.PatrolAutonomyLevel != config.PatrolAutonomyMonitor {
		t.Fatalf("expected persisted autonomy %q, got %q", config.PatrolAutonomyMonitor, saved.PatrolAutonomyLevel)
	}
	if saved.PatrolFullModeUnlocked {
		t.Fatalf("expected persisted full mode unlock to be cleared for Community")
	}
	if saved.PatrolInvestigationBudget != 5 || saved.PatrolInvestigationTimeoutSec != 60 {
		t.Fatalf("expected persisted budget/timeout 5/60, got %d/%d",
			saved.PatrolInvestigationBudget, saved.PatrolInvestigationTimeoutSec)
	}

	// Investigation/remediation autonomy remains Pro-gated.
	approvalBody := `{"autonomy_level":"approval","investigation_budget":10,"investigation_timeout_sec":120}`
	approvalReq := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(approvalBody))
	approvalRec := httptest.NewRecorder()
	freeAdapter.HandleUpdatePatrolAutonomy(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for Community premium autonomy update, got %d: %s", approvalRec.Code, approvalRec.Body.String())
	}
	if !strings.Contains(approvalRec.Body.String(), "limited to Monitor") {
		t.Fatalf("expected Pro gate response to explain monitor limit, got %s", approvalRec.Body.String())
	}
}
