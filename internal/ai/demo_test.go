package ai

import (
	"strings"
	"testing"
)

func TestIsDemoMode(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	if !IsDemoMode() {
		t.Fatal("expected demo mode true when PULSE_MOCK_MODE=true")
	}

	t.Setenv("PULSE_MOCK_MODE", "false")
	if IsDemoMode() {
		t.Fatal("expected demo mode false when PULSE_MOCK_MODE=false")
	}
}

func TestIsMockResource(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "false")
	if !IsMockResource("node/pve1", "", "") {
		t.Fatal("expected mock resource to be detected for pattern match")
	}
	if IsMockResource("node/prod1", "production", "node1") {
		t.Fatal("expected non-mock resource to return false")
	}

	t.Setenv("PULSE_MOCK_MODE", "true")
	if IsMockResource("node/pve1", "", "") {
		t.Fatal("expected demo mode to bypass mock filtering")
	}
}

func TestPatrolService_InjectDemoFindings(t *testing.T) {
	service := NewPatrolService(nil, nil)
	if service.findings == nil || service.runHistoryStore == nil {
		t.Fatal("expected findings and run history to be initialized")
	}

	service.InjectDemoFindings()

	findings := service.findings.GetAll(nil)
	if len(findings) != 5 {
		t.Fatalf("expected 5 demo findings, got %d", len(findings))
	}
	if service.runHistoryStore.Count() != 13 {
		t.Fatalf("expected 13 demo run history entries, got %d", service.runHistoryStore.Count())
	}
}

func TestPatrolService_InjectDemoFindings_NoStore(t *testing.T) {
	service := &PatrolService{}
	service.InjectDemoFindings()
}

func TestPatrolService_InjectDemoRunHistory_NoStore(t *testing.T) {
	service := &PatrolService{}
	service.injectDemoRunHistory()
}

func TestGenerateDemoAIResponse(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"patrol", "Analyze the infrastructure for issues", "ZFS pool 'local-zfs' is 94% full"},
		{"disk", "disk is full", "Disk Usage Analysis"},
		{"memory", "memory pressure", "Memory Analysis"},
		{"backup", "pbs backup status", "Backup Status Review"},
		{"cpu", "cpu load is high", "CPU/Performance Analysis"},
		{"hello", "hello there", "Pulse AI Assistant"},
		{"default", "tell me something", "This Demo Shows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := GenerateDemoAIResponse(tt.prompt)
			if resp == nil {
				t.Fatal("expected response")
			}
			if !strings.Contains(resp.Content, tt.expected) {
				t.Fatalf("expected response to contain %q, got %q", tt.expected, resp.Content)
			}
			if resp.Model == "" {
				t.Fatal("expected model to be set")
			}
		})
	}
}

func TestGenerateDemoAIStream(t *testing.T) {
	var content strings.Builder
	done := false

	resp, err := GenerateDemoAIStream("disk usage", func(event StreamEvent) {
		switch event.Type {
		case "content":
			chunk, ok := event.Data.(string)
			if !ok {
				t.Fatalf("expected string content chunk, got %T", event.Data)
			}
			content.WriteString(chunk)
		case "done":
			done = true
		}
	})
	if err != nil {
		t.Fatalf("GenerateDemoAIStream failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if !done {
		t.Fatal("expected done event")
	}
	if content.String() != resp.Content {
		t.Fatal("expected streamed content to match response content")
	}
}
