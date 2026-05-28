package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAgentCapabilitiesManifest_ReturnsStableShape(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	HandleAgentCapabilitiesManifest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q want application/json", got)
	}
	// Cacheable so agents can hold the manifest in memory across
	// requests; 5 minutes mirrors the typical agent session length.
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Error("manifest must be cacheable; Cache-Control header missing")
	}

	var manifest AgentCapabilitiesManifest
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.Version != "v1" {
		t.Errorf("version pin: got %q want v1", manifest.Version)
	}
	if len(manifest.Capabilities) == 0 {
		t.Fatal("manifest must declare at least one capability")
	}
}

func TestHandleAgentCapabilitiesManifest_RejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/capabilities", nil)
	HandleAgentCapabilitiesManifest(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestAgentCapabilitiesManifest_NamesAreUniqueAndSnakeCase(t *testing.T) {
	// Capability names are agent-stable identifiers — duplicates
	// would silently mask one capability behind another, and
	// non-snake_case would break the convention agents use for tool
	// names. Pin both invariants.
	seen := map[string]bool{}
	for _, cap := range agentCapabilitiesManifest.Capabilities {
		if seen[cap.Name] {
			t.Errorf("duplicate capability name %q — names are agent-stable identifiers", cap.Name)
		}
		seen[cap.Name] = true

		if cap.Name == "" {
			t.Error("capability name must not be empty")
			continue
		}
		for _, ch := range cap.Name {
			if !(ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
				t.Errorf("capability name %q must be snake_case (lowercase letters, digits, underscores only); got rune %q", cap.Name, ch)
				break
			}
		}
	}
}

func TestAgentCapabilitiesManifest_EveryCapabilityDeclaresMethodPathScope(t *testing.T) {
	// Without method/path/scope, an agent can't actually call the
	// capability. These three are the minimum viable contract.
	for _, cap := range agentCapabilitiesManifest.Capabilities {
		if cap.Method == "" {
			t.Errorf("capability %q missing method", cap.Name)
		}
		if cap.Path == "" {
			t.Errorf("capability %q missing path", cap.Name)
		}
		if cap.Scope == "" {
			t.Errorf("capability %q missing scope", cap.Name)
		}
		if cap.Description == "" {
			t.Errorf("capability %q missing description", cap.Name)
		}
		if cap.Category == "" {
			t.Errorf("capability %q missing category — agents filter the manifest by category", cap.Name)
		}
	}
}

func TestAgentCapabilitiesManifest_CategoriesAreClosed(t *testing.T) {
	// Agents filter the manifest by category. Keep the set closed
	// so a typo in a future capability doesn't fragment the surface
	// (e.g. "operator-state" vs "operator_state" would split into
	// two categories an agent might miss).
	allowed := map[string]bool{
		"context":        true,
		"provisioning":   true,
		"operator-state": true,
		"finding":        true,
		"action":         true,
	}
	for _, cap := range agentCapabilitiesManifest.Capabilities {
		if !allowed[cap.Category] {
			t.Errorf("capability %q has unknown category %q — extend the allowlist deliberately", cap.Name, cap.Category)
		}
	}
}

func TestAgentCapabilitiesManifest_DeclaresNodeProvisioningSurface(t *testing.T) {
	byName := map[string]AgentCapability{}
	for _, cap := range agentCapabilitiesManifest.Capabilities {
		byName[cap.Name] = cap
	}

	required := map[string]struct {
		method string
		path   string
		scope  string
	}{
		"list_nodes":                      {http.MethodGet, "/api/config/nodes", "settings:read"},
		"add_node":                        {http.MethodPost, "/api/config/nodes", "settings:write"},
		"update_node":                     {http.MethodPut, "/api/config/nodes/{nodeId}", "settings:write"},
		"remove_node":                     {http.MethodDelete, "/api/config/nodes/{nodeId}", "settings:write"},
		"test_node_credentials":           {http.MethodPost, "/api/config/nodes/test-config", "settings:write"},
		"test_node_connection":            {http.MethodPost, "/api/config/nodes/{nodeId}/test", "settings:write"},
		"refresh_node_cluster_membership": {http.MethodPost, "/api/config/nodes/{nodeId}/refresh-cluster", "settings:write"},
		"discover_lan":                    {http.MethodPost, "/api/discover", "settings:write"},
	}

	for name, want := range required {
		cap, ok := byName[name]
		if !ok {
			t.Fatalf("manifest missing node provisioning capability %q", name)
		}
		if cap.Category != "provisioning" {
			t.Errorf("%s category = %q, want provisioning", name, cap.Category)
		}
		if cap.Method != want.method || cap.Path != want.path || cap.Scope != want.scope {
			t.Errorf("%s method/path/scope = %s %s %s, want %s %s %s",
				name, cap.Method, cap.Path, cap.Scope, want.method, want.path, want.scope)
		}
	}

	for _, name := range []string{"add_node", "update_node", "test_node_credentials", "discover_lan"} {
		cap := byName[name]
		if cap.InputSchema == nil {
			t.Fatalf("%s must publish an inputSchema so agent clients get typed onboarding arguments", name)
		}
		raw, err := json.Marshal(cap.InputSchema)
		if err != nil {
			t.Fatalf("%s inputSchema marshal: %v", name, err)
		}
		text := string(raw)
		for _, fragment := range []string{`"type":"object"`, `"additionalProperties":false`} {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s inputSchema missing %s: %s", name, fragment, text)
			}
		}
	}

	addSchema, _ := json.Marshal(byName["add_node"].InputSchema)
	for _, fragment := range []string{`"enum":["pve","pbs","pmg"]`, `"required":["type","name","host"]`, `"tokenValue"`} {
		if !strings.Contains(string(addSchema), fragment) {
			t.Errorf("add_node inputSchema missing %s: %s", fragment, string(addSchema))
		}
	}

	updateSchema, _ := json.Marshal(byName["update_node"].InputSchema)
	if !strings.Contains(string(updateSchema), `"nodeId"`) {
		t.Errorf("update_node inputSchema must include nodeId path argument: %s", string(updateSchema))
	}

	discoverSchema, _ := json.Marshal(byName["discover_lan"].InputSchema)
	for _, fragment := range []string{`"subnet"`, `"use_cache"`} {
		if !strings.Contains(string(discoverSchema), fragment) {
			t.Errorf("discover_lan inputSchema missing %s: %s", fragment, string(discoverSchema))
		}
	}
}

func TestAgentCapabilitiesManifest_CarriesStableErrorCodes(t *testing.T) {
	// The error-code surface is the agent-branching contract. The
	// codes I've shipped this session must appear on the
	// corresponding capability so agents can branch on them. Pin a
	// few of the most consequential codes.
	wantErrorCodes := map[string][]string{
		"get_resource_context": {"resource_not_found"},
		"get_operator_state":   {"operator_state_not_set"},
		"set_operator_state":   {"operator_state_invalid"},
	}
	byName := map[string]AgentCapability{}
	for _, cap := range agentCapabilitiesManifest.Capabilities {
		byName[cap.Name] = cap
	}
	for name, expected := range wantErrorCodes {
		cap, ok := byName[name]
		if !ok {
			t.Errorf("capability %q missing from manifest", name)
			continue
		}
		for _, code := range expected {
			found := false
			for _, declared := range cap.ErrorCodes {
				if declared == code {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("capability %q must declare error code %q so agents can branch on it; declared codes: %v", name, code, cap.ErrorCodes)
			}
		}
	}
}
