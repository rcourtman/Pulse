package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSecurityTokens_ListCreateDelete(t *testing.T) {
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
	rr = httptest.NewRecorder()
	router.handleDeleteAPIToken(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	if !cfg.IsEnvMigrationSuppressed("hash-migrated") {
		t.Fatalf("expected env migration suppression to be recorded")
	}
}
