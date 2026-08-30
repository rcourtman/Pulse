package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestReduceCollectorAuthorityRemovesExecutionScopeIrreversibly(t *testing.T) {
	raw := "collector-authority-test.12345678"
	record, err := config.NewAPITokenRecord(raw, "legacy collector", []string{
		config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeAgentManage, config.ScopeAgentExec,
	})
	if err != nil {
		t.Fatal(err)
	}
	record.OrgID = "org-a"
	record.Metadata = map[string]string{
		"bound_agent_id": "agent-a", "bound_hostname": "node.example",
		agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindLegacyFullTrust,
	}
	cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(t.TempDir())}
	body, _ := json.Marshal(collectorAuthorityReductionRequest{AgentID: "agent-a", Hostname: "node.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	attachAPITokenRecord(req, record)
	response := httptest.NewRecorder()
	router.handleReduceCollectorAuthority(response, req)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("reduced scopes = %v", cfg.APITokens)
	}
	if cfg.APITokens[0].HasScope(config.ScopeAgentManage) {
		t.Fatalf("reduced collector retained cross-host management scope: %v", cfg.APITokens[0].Scopes)
	}
	if got := cfg.APITokens[0].Metadata[agenttokens.RuntimeRoleMetadataKey]; got != agenttokens.CredentialKindMonitoringCollector {
		t.Fatalf("runtime role = %q", got)
	}
	if got := cfg.APITokens[0].Metadata[agenttokens.CommandPolicyIntentMetadataKey]; got != agenttokens.CommandPolicyIntentDisabled {
		t.Fatalf("command policy = %q", got)
	}
}

func TestReduceCollectorAuthorityRejectsCrossHostBinding(t *testing.T) {
	record := &config.APITokenRecord{ID: "token-a", OrgID: "org-a", Scopes: []string{config.ScopeAgentReport, config.ScopeAgentExec}, Metadata: map[string]string{
		"bound_agent_id": "agent-a", "bound_hostname": "node.example",
	}}
	cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(t.TempDir())}
	body, _ := json.Marshal(collectorAuthorityReductionRequest{AgentID: "agent-b", Hostname: "other.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	attachAPITokenRecord(req, record)
	response := httptest.NewRecorder()
	router.handleReduceCollectorAuthority(response, req)
	if response.Code != http.StatusForbidden || !cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("status=%d scopes=%v", response.Code, cfg.APITokens[0].Scopes)
	}
}

func TestReduceCollectorAuthorityRejectsUnboundCredential(t *testing.T) {
	record := &config.APITokenRecord{ID: "token-a", OrgID: "org-a", Scopes: []string{config.ScopeAgentReport, config.ScopeAgentExec}}
	cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(t.TempDir())}
	body, _ := json.Marshal(collectorAuthorityReductionRequest{AgentID: "agent-a", Hostname: "node.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	attachAPITokenRecord(req, record)
	response := httptest.NewRecorder()
	router.handleReduceCollectorAuthority(response, req)
	if response.Code != http.StatusForbidden || !cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("status=%d scopes=%v", response.Code, cfg.APITokens[0].Scopes)
	}
}

func TestReduceCollectorAuthorityRollsBackMemoryWhenPersistenceFails(t *testing.T) {
	raw := "collector-authority-persistence.12345678"
	record, err := config.NewAPITokenRecord(raw, "legacy collector", []string{config.ScopeAgentReport, config.ScopeAgentExec})
	if err != nil {
		t.Fatal(err)
	}
	record.OrgID = "org-a"
	record.Metadata = map[string]string{
		"bound_agent_id": "agent-a", "bound_hostname": "node.example",
		agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindLegacyFullTrust,
	}
	cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	persistence := config.NewConfigPersistence(t.TempDir())
	persistence.SetFileSystem(actionRunnerFailingPersistenceFS{})
	router := &Router{config: cfg, persistence: persistence}
	body, _ := json.Marshal(collectorAuthorityReductionRequest{AgentID: "agent-a", Hostname: "node.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	attachAPITokenRecord(req, record)
	response := httptest.NewRecorder()
	router.handleReduceCollectorAuthority(response, req)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if !cfg.APITokens[0].HasScope(config.ScopeAgentExec) || cfg.APITokens[0].Metadata[agenttokens.RuntimeRoleMetadataKey] != agenttokens.CredentialKindLegacyFullTrust {
		t.Fatalf("failed persistence changed live authority: %#v", cfg.APITokens[0])
	}
}

func TestReduceCollectorAuthorityRejectsWildcardCredential(t *testing.T) {
	record := &config.APITokenRecord{ID: "wildcard", OrgID: "org-a", Scopes: []string{config.ScopeWildcard}, Metadata: map[string]string{
		"bound_agent_id": "agent-a", "bound_hostname": "node.example",
	}}
	cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(t.TempDir())}
	body, _ := json.Marshal(collectorAuthorityReductionRequest{AgentID: "agent-a", Hostname: "node.example"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-a"))
	attachAPITokenRecord(req, record)
	response := httptest.NewRecorder()
	router.handleReduceCollectorAuthority(response, req)
	if response.Code != http.StatusForbidden || !cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("wildcard reduction status=%d scopes=%v", response.Code, cfg.APITokens[0].Scopes)
	}
}
