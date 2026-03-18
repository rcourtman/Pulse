package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestSecurityTokens_CreateAndListIncludeOwnerUserID(t *testing.T) {
	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"]}`))
	req = req.WithContext(authpkg.WithUser(req.Context(), "alice"))
	rr := httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 creating token, got %d", rr.Code)
	}

	var createResp struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Record.OwnerUserID != "alice" {
		t.Fatalf("record ownerUserId = %q, want alice", createResp.Record.OwnerUserID)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one stored token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID]; got != "alice" {
		t.Fatalf("stored owner_user_id = %q, want alice", got)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	listRR := httptest.NewRecorder()
	router.handleListAPITokens(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 listing tokens, got %d", listRR.Code)
	}

	var listResp struct {
		Tokens []apiTokenDTO `json:"tokens"`
	}
	if err := json.NewDecoder(listRR.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Tokens) != 1 {
		t.Fatalf("expected one listed token, got %d", len(listResp.Tokens))
	}
	if listResp.Tokens[0].OwnerUserID != "alice" {
		t.Fatalf("listed ownerUserId = %q, want alice", listResp.Tokens[0].OwnerUserID)
	}
}

func TestSecurityTokens_CreateInheritsOwnerUserIDFromTokenCaller(t *testing.T) {
	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	caller, err := config.NewAPITokenRecord("caller-token-123.12345678", "caller", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new caller token record: %v", err)
	}
	caller.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "alice"}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"child","scopes":["monitoring:read"]}`))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	req = req.WithContext(authpkg.WithAPIToken(req.Context(), caller))
	rr := httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 creating child token, got %d", rr.Code)
	}

	var createResp struct {
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Record.OwnerUserID != "alice" {
		t.Fatalf("child ownerUserId = %q, want alice", createResp.Record.OwnerUserID)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one stored token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID]; got != "alice" {
		t.Fatalf("stored child owner_user_id = %q, want alice", got)
	}
	if got := cfg.APITokens[0].OrgID; got != "acme" {
		t.Fatalf("stored child org = %q, want acme", got)
	}
}
