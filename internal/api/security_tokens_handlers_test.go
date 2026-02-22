package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestSecurityTokens_ListCreateDelete(t *testing.T) {
	capture := &auditCaptureLogger{}
	prevLogger := audit.GetLogger()
	prevManager := GetTenantAuditManager()
	audit.SetLogger(capture)
	SetTenantAuditManager(nil)
	t.Cleanup(func() {
		audit.SetLogger(prevLogger)
		SetTenantAuditManager(prevManager)
	})

	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", nil)
	rr := httptest.NewRecorder()
	router.handleListAPITokens(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader("{bad"))
	rr = httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"]}`))
	req = req.WithContext(authpkg.WithUser(req.Context(), "alice"))
	rr = httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var createResp struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Token == "" {
		t.Fatal("expected raw token in response")
	}
	if createResp.Record.Name != "test" {
		t.Fatalf("expected record name 'test', got %q", createResp.Record.Name)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected token stored in config")
	}
	if got := cfg.APITokens[0].OrgID; got != "default" {
		t.Fatalf("expected token org binding default, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	rr = httptest.NewRecorder()
	router.handleListAPITokens(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var listResp struct {
		Tokens []apiTokenDTO `json:"tokens"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(listResp.Tokens))
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/security/tokens/", nil)
	rr = httptest.NewRecorder()
	router.handleDeleteAPIToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/security/tokens/missing", nil)
	rr = httptest.NewRecorder()
	router.handleDeleteAPIToken(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Test deletion of migrated env token suppression
	record := config.APITokenRecord{
		ID:        "migrated",
		Name:      "Migrated from .env token",
		Hash:      "hash-migrated",
		CreatedAt: time.Now(),
	}
	cfg.APITokens = append(cfg.APITokens, record)

	req = httptest.NewRequest(http.MethodDelete, "/api/security/tokens/migrated", nil)
	req = req.WithContext(authpkg.WithUser(req.Context(), "alice"))
	rr = httptest.NewRecorder()
	router.handleDeleteAPIToken(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	if !cfg.IsEnvMigrationSuppressed("hash-migrated") {
		t.Fatalf("expected env migration suppression to be recorded")
	}

	events, err := capture.Query(audit.QueryFilter{})
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}

	var sawCreate, sawDelete bool
	for _, event := range events {
		if event.EventType == "token_created" && event.Success {
			sawCreate = true
			if event.User != "alice" {
				t.Fatalf("token_created audit user = %q, want %q", event.User, "alice")
			}
			if strings.Contains(event.Details, createResp.Token) {
				t.Fatalf("token_created audit details leaked raw token")
			}
		}
		if event.EventType == "token_deleted" && event.Success {
			sawDelete = true
		}
	}
	if !sawCreate {
		t.Fatalf("expected successful token_created audit event")
	}
	if !sawDelete {
		t.Fatalf("expected successful token_deleted audit event")
	}
}

func TestSecurityTokens_Create_WithExpiresIn(t *testing.T) {
	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	start := time.Now().UTC()
	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"],"expiresIn":"1h"}`))
	rr := httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	end := time.Now().UTC()

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var createResp struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Record.ExpiresAt == nil {
		t.Fatalf("expected expiresAt to be set in response record")
	}
	if exp := *createResp.Record.ExpiresAt; exp.Before(start.Add(time.Hour)) || exp.After(end.Add(time.Hour).Add(2*time.Second)) {
		t.Fatalf("unexpected expiresAt: %v (start=%v end=%v)", exp, start, end)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ExpiresAt == nil {
		t.Fatalf("expected expiresAt to be stored in config")
	}
}

func TestSecurityTokens_Create_WithInvalidExpiresIn(t *testing.T) {
	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"],"expiresIn":"not-a-duration"}`))
	rr := httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("expected no token stored in config on invalid expiresIn")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"],"expiresIn":"30s"}`))
	rr = httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("expected no token stored in config when expiresIn < 1m")
	}
}

func TestSecurityTokens_Create_BindsTokenToRequestOrg(t *testing.T) {
	cfg := &config.Config{}
	persistence := config.NewConfigPersistence(t.TempDir())
	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(`{"name":"test","scopes":["monitoring:read"]}`))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "acme"))
	rr := httptest.NewRecorder()
	router.handleCreateAPIToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected token stored in config")
	}
	if got := cfg.APITokens[0].OrgID; got != "acme" {
		t.Fatalf("token org binding = %q, want acme", got)
	}
}
