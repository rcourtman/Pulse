package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContract_DiagnosticsInfoExcludesInternalAnalytics(t *testing.T) {
	payload := EmptyDiagnosticsInfo()

	got, err := json.Marshal(payload.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal diagnostics info: %v", err)
	}

	for _, forbidden := range []string{
		"commercialFunnel",
		"infrastructureOnboarding",
		"pricing_viewed",
		"credentials_opened",
	} {
		if strings.Contains(string(got), forbidden) {
			t.Fatalf("diagnostics contract leaked internal analytics field %q: %s", forbidden, got)
		}
	}
}

func TestContract_AssistantDiagnosticsUseNativeRuntimeVocabulary(t *testing.T) {
	payload := EmptyDiagnosticsInfo()
	payload.AIChat = &AIChatDiagnostic{
		Enabled:                   true,
		Running:                   true,
		Healthy:                   true,
		Model:                     "ollama:llama3",
		AssistantRuntimeConnected: true,
	}

	got, err := json.Marshal(payload.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal diagnostics info: %v", err)
	}
	body := string(got)

	if !strings.Contains(body, `"assistantRuntimeConnected":true`) {
		t.Fatalf("diagnostics payload must expose native Assistant runtime connectivity: %s", body)
	}
	for _, forbidden := range []string{
		"mcpConnected",
		"mcpToolCount",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("diagnostics payload must not expose legacy MCP diagnostic field %q: %s", forbidden, body)
		}
	}
}
