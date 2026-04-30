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
