package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActionActorCredentialLabelNormalizationAndEquality(t *testing.T) {
	actor := NormalizeActionActor(ActionActor{
		SubjectID:       "  alice  ",
		Kind:            ActionActorAPIToken,
		CredentialID:    "  api-token:tok-1  ",
		CredentialLabel: "  Claude Code  ",
		OrgID:           "  default  ",
	})
	if actor.SubjectID != "alice" || actor.CredentialID != "api-token:tok-1" || actor.CredentialLabel != "Claude Code" || actor.OrgID != "default" {
		t.Fatalf("normalized actor = %#v", actor)
	}
	if err := ValidateActionActor(actor); err != nil {
		t.Fatalf("ValidateActionActor error = %v", err)
	}

	raw, err := json.Marshal(actor)
	if err != nil {
		t.Fatalf("marshal actor: %v", err)
	}
	if !strings.Contains(string(raw), `"credentialLabel":"Claude Code"`) {
		t.Fatalf("expected credentialLabel in JSON, got %s", raw)
	}

	withoutLabel := ActionActor{
		SubjectID:    "alice",
		Kind:         ActionActorAPIToken,
		CredentialID: "api-token:tok-1",
		OrgID:        "default",
	}
	rawWithout, err := json.Marshal(withoutLabel)
	if err != nil {
		t.Fatalf("marshal actor without label: %v", err)
	}
	if strings.Contains(string(rawWithout), "credentialLabel") {
		t.Fatalf("empty credentialLabel must be omitted for backwards compatibility: %s", rawWithout)
	}

	if !ActionActorsEqual(actor, withoutLabel) {
		t.Fatal("ActionActorsEqual must match on durable identity fields even when display label differs")
	}
}
