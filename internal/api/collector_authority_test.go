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
		config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeDockerReport, config.ScopeKubernetesReport,
		config.ScopeAgentManage, config.ScopeAgentExec, config.ScopeSettingsWrite, config.ScopeActionsExecute,
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
	if cfg.APITokens[0].HasScope(config.ScopeSettingsWrite) || cfg.APITokens[0].HasScope(config.ScopeActionsExecute) {
		t.Fatalf("reduced collector retained unrelated authority: %v", cfg.APITokens[0].Scopes)
	}
	if !cfg.APITokens[0].HasScope(config.ScopeDockerReport) || !cfg.APITokens[0].HasScope(config.ScopeKubernetesReport) {
		t.Fatalf("reduced host collector lost provider reporting authority: %v", cfg.APITokens[0].Scopes)
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

func TestCollectorAuthorityHostnamesEquivalent(t *testing.T) {
	for _, test := range []struct {
		name      string
		bound     string
		requested string
		want      bool
	}{
		{name: "short to FQDN", bound: "node-a", requested: "node-a.example.test", want: true},
		{name: "FQDN to short", bound: "node-a.example.test", requested: "node-a", want: true},
		{name: "same FQDN case and trailing dot", bound: "Node-A.Example.Test.", requested: "node-a.example.test", want: true},
		{name: "distinct same-label FQDN", bound: "node-a.one.test", requested: "node-a.two.test", want: false},
		{name: "same IPv4 literal", bound: "192.0.2.10", requested: "192.0.2.10", want: true},
		{name: "distinct IPv4 literal", bound: "192.0.2.10", requested: "192.0.2.11", want: false},
		{name: "same IPv6 literal", bound: "2001:DB8::10", requested: "2001:db8::10", want: true},
		{name: "IP and hostname", bound: "192.0.2.10", requested: "node-a", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := collectorAuthorityHostnamesEquivalent(test.bound, test.requested); got != test.want {
				t.Fatalf("collectorAuthorityHostnamesEquivalent(%q, %q) = %v, want %v", test.bound, test.requested, got, test.want)
			}
		})
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
	record, err := config.NewAPITokenRecord(raw, "legacy collector", []string{config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeAgentExec})
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
