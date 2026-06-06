package chat

import (
	"encoding/json"
	"testing"
)

func TestStreamEventClientSafeDropsThinking(t *testing.T) {
	data, err := json.Marshal(ThinkingData{Text: "We need to inspect private state."})
	if err != nil {
		t.Fatal(err)
	}

	event, ok := (StreamEvent{Type: "thinking", Data: data}).ClientSafe()
	if ok {
		t.Fatalf("thinking event should be dropped, got %+v", event)
	}
}

func TestStreamEventClientSafeCleansContentToolCallArtifacts(t *testing.T) {
	data, err := json.Marshal(ContentData{
		Text: "I will inspect the device nodes.\npulse_read(target_host=\"current_resource\", command=\"lsblk\")",
	})
	if err != nil {
		t.Fatal(err)
	}

	event, ok := (StreamEvent{Type: "content", Data: data}).ClientSafe()
	if !ok {
		t.Fatal("content event should remain visible")
	}

	var content ContentData
	if err := json.Unmarshal(event.Data, &content); err != nil {
		t.Fatal(err)
	}
	if content.Text != "I will inspect the device nodes." {
		t.Fatalf("content text = %q, want cleaned prose", content.Text)
	}
}

func TestStreamEventClientSafeCleansDecorativeOperationalSymbols(t *testing.T) {
	data, err := json.Marshal(ContentData{
		Text: "### 🔴 Critical Alerts\n- ⚠️ Active AI Patrol Finding\nTemperature is 58°C.",
	})
	if err != nil {
		t.Fatal(err)
	}

	event, ok := (StreamEvent{Type: "content", Data: data}).ClientSafe()
	if !ok {
		t.Fatal("content event should remain visible")
	}

	var content ContentData
	if err := json.Unmarshal(event.Data, &content); err != nil {
		t.Fatal(err)
	}
	expected := "### Critical Alerts\n- Active AI Patrol Finding\nTemperature is 58°C."
	if content.Text != expected {
		t.Fatalf("content text = %q, want %q", content.Text, expected)
	}
}
