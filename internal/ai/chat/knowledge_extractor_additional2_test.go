package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractMetricsFacts_UnknownAction(t *testing.T) {
	facts := ExtractFacts("pulse_metrics", map[string]interface{}{"action": "mystery"}, `{}`)
	assert.Empty(t, facts)
}

func TestExtractMetricsFacts_TypeFallbackSummary(t *testing.T) {
	input := map[string]interface{}{
		"type":        "performance",
		"resource_id": "vm123",
	}
	result := `{"summary":{"other":{"avg_cpu":5,"max_cpu":10,"avg_memory":20,"max_memory":30,"trend":"up"}}}`

	facts := ExtractFacts("pulse_metrics", input, result)
	if assert.Len(t, facts, 1) {
		assert.Equal(t, "metrics:vm123", facts[0].Key)
		assert.Contains(t, facts[0].Value, "avg_cpu=5.0%")
		assert.Contains(t, facts[0].Value, "trend=up")
	}
}

func TestExtractAlertsFacts_UnknownAction(t *testing.T) {
	facts := ExtractFacts("pulse_alerts", map[string]interface{}{"type": "mystery"}, `{}`)
	assert.Empty(t, facts)
}

func TestExtractAlertsListFacts_ResolvedFiltered(t *testing.T) {
	input := map[string]interface{}{"action": "list"}
	result := `{"alerts":[{"resource_name":"vm101","type":"cpu","severity":"critical","value":95.2,"threshold":80,"status":"resolved"},{"resource_name":"ct200","type":"memory","severity":"warning","value":82.0,"threshold":90,"status":"active"}]}`

	facts := ExtractFacts("pulse_alerts", input, result)
	if assert.Len(t, facts, 3) {
		assert.Equal(t, "alerts:overview", facts[0].Key)
		assert.Contains(t, facts[0].Value, "1 active alerts")
	}
}

func TestExtractKubernetesFacts_UnknownAction(t *testing.T) {
	facts := ExtractFacts("pulse_kubernetes", map[string]interface{}{"action": "mystery"}, `{}`)
	assert.Empty(t, facts)
}

func TestExtractPMGFacts_UnknownAction(t *testing.T) {
	facts := ExtractFacts("pulse_pmg", map[string]interface{}{"action": "mystery"}, `{}`)
	assert.Empty(t, facts)
}

func TestStrFromMap_EdgeCases(t *testing.T) {
	if got := strFromMap(nil, "key"); got != "" {
		t.Fatalf("expected empty for nil map, got %q", got)
	}
	if got := strFromMap(map[string]interface{}{"key": 123}, "key"); got != "" {
		t.Fatalf("expected empty for non-string value, got %q", got)
	}
	if got := strFromMap(map[string]interface{}{"other": "value"}, "key"); got != "" {
		t.Fatalf("expected empty for missing key, got %q", got)
	}
}
