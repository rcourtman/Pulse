package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNodeUnmarshalCanonicalLinkedAgentID(t *testing.T) {
	var node Node
	if err := json.Unmarshal([]byte(`{"id":"node-1","linkedAgentId":"agent-canonical"}`), &node); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if node.LinkedAgentID != "agent-canonical" {
		t.Fatalf("LinkedAgentID = %q, want %q", node.LinkedAgentID, "agent-canonical")
	}
}

func TestNodeMarshalUsesCanonicalLinkedAgentID(t *testing.T) {
	node := Node{ID: "node-1", LinkedAgentID: "agent-canonical"}
	encoded, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	serialized := string(encoded)
	if !strings.Contains(serialized, `"linkedAgentId":"agent-canonical"`) {
		t.Fatalf("expected canonical linkedAgentId in output: %s", serialized)
	}
}
