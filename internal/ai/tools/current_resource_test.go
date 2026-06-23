package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestCurrentResourceReferenceUsesSharedAgentCapabilityVocabulary(t *testing.T) {
	for _, value := range []string{
		agentcapabilities.CurrentResourceHandle,
		"attached_resource",
		"selected_resource",
		"this_resource",
		"redacted by policy",
	} {
		if !IsCurrentResourceReference(value) {
			t.Fatalf("IsCurrentResourceReference(%q) = false, want shared placeholder", value)
		}
	}
	if IsCurrentResourceReference("resource-1") {
		t.Fatalf("ordinary resource id should not be treated as current_resource")
	}
}
