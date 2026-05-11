package reporting

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNarrativeBullet_JSONUsesLowercaseKeys pins the wire shape that
// the pulse_summarize chat tool returns. Without JSON tags the
// embedded struct serialises with capital field names (Text/Severity),
// which is inconsistent with the lowercase schema the AI narrator's
// system prompt asks for. A downstream model consuming the tool
// response then has to handle two different cases for the same
// shape. Caught by exercising the chat path against a real model.
func TestNarrativeBullet_JSONUsesLowercaseKeys(t *testing.T) {
	b, err := json.Marshal(NarrativeBullet{Text: "hello", Severity: "ok"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"text":"hello"`) {
		t.Errorf("expected lowercase 'text' key, got %s", got)
	}
	if !strings.Contains(got, `"severity":"ok"`) {
		t.Errorf("expected lowercase 'severity' key, got %s", got)
	}
	if strings.Contains(got, `"Text"`) || strings.Contains(got, `"Severity"`) {
		t.Errorf("capital-case keys leaked into JSON: %s", got)
	}
}

// TestFleetOutlier_JSONUsesLowercaseKeys is the fleet counterpart.
// FleetOutlier appears in summarizeFleetResponse.Outliers and feeds
// the same chat-tool path that surfaced the casing inconsistency.
func TestFleetOutlier_JSONUsesLowercaseKeys(t *testing.T) {
	b, err := json.Marshal(FleetOutlier{
		ResourceID:   "node-a",
		ResourceName: "alpha",
		Reason:       "memory at 92%",
		Severity:     "warning",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	wants := []string{
		`"resource_id":"node-a"`,
		`"resource_name":"alpha"`,
		`"reason":"memory at 92%"`,
		`"severity":"warning"`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("expected key %q in %s", w, got)
		}
	}
	if strings.Contains(got, `"ResourceID"`) || strings.Contains(got, `"Severity"`) {
		t.Errorf("capital-case keys leaked: %s", got)
	}
}

// TestNarrative_JSONUsesSnakeCaseKeys verifies the parent Narrative
// envelope also serialises with the documented schema. The chat tool
// doesn't currently return the whole Narrative directly (it copies
// fields into summarizeResourceResponse), but if a future caller
// marshals the type directly the wire shape should match the
// system-prompt schema.
func TestNarrative_JSONUsesSnakeCaseKeys(t *testing.T) {
	b, err := json.Marshal(Narrative{
		Source:        NarrativeSourceAI,
		HealthStatus:  "HEALTHY",
		HealthMessage: "ok",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"source":"ai"`) {
		t.Errorf("expected source key: %s", got)
	}
	if !strings.Contains(got, `"health_status":"HEALTHY"`) {
		t.Errorf("expected health_status key: %s", got)
	}
	if !strings.Contains(got, `"health_message":"ok"`) {
		t.Errorf("expected health_message key: %s", got)
	}
}
