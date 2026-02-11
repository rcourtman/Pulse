package agentexec

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecuteCommandAgentNotConnected(t *testing.T) {
	s := NewServer(nil)
	_, err := s.ExecuteCommand(context.Background(), "missing", ExecuteCommandPayload{RequestID: "r1", Timeout: 1})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestReadFileAgentNotConnected(t *testing.T) {
	s := NewServer(nil)
	_, err := s.ReadFile(context.Background(), "missing", ReadFilePayload{RequestID: "r1"})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestExecuteCommandValidation(t *testing.T) {
	s := NewServer(nil)
	if _, err := s.ExecuteCommand(context.Background(), "", ExecuteCommandPayload{RequestID: "r1"}); err == nil {
		t.Fatalf("expected empty agent id error")
	}
	if _, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{}); err == nil {
		t.Fatalf("expected empty request id error")
	}
}

func TestExecuteCommandSecurityValidation(t *testing.T) {
	s := NewServer(nil)

	cases := []struct {
		name    string
		payload ExecuteCommandPayload
		wantErr string
	}{
		{
			name: "missing command",
			payload: ExecuteCommandPayload{
				RequestID: "r1",
			},
			wantErr: "command is required",
		},
		{
			name: "invalid target type",
			payload: ExecuteCommandPayload{
				RequestID:  "r1",
				Command:    "echo ok",
				TargetType: "node",
			},
			wantErr: "invalid target type",
		},
		{
			name: "container requires target id",
			payload: ExecuteCommandPayload{
				RequestID:  "r1",
				Command:    "echo ok",
				TargetType: "container",
			},
			wantErr: "target id is required",
		},
		{
			name: "invalid target id characters",
			payload: ExecuteCommandPayload{
				RequestID:  "r1",
				Command:    "echo ok",
				TargetType: "vm",
				TargetID:   "100; rm -rf /",
			},
			wantErr: "target id contains invalid characters",
		},
		{
			name: "negative timeout",
			payload: ExecuteCommandPayload{
				RequestID: "r1",
				Command:   "echo ok",
				Timeout:   -1,
			},
			wantErr: "timeout cannot be negative",
		},
		{
			name: "excessive timeout",
			payload: ExecuteCommandPayload{
				RequestID: "r1",
				Command:   "echo ok",
				Timeout:   maxExecuteCommandTimeoutSeconds + 1,
			},
			wantErr: "timeout cannot exceed",
		},
		{
			name: "request id too long",
			payload: ExecuteCommandPayload{
				RequestID: strings.Repeat("a", maxRequestIDLength+1),
				Command:   "echo ok",
			},
			wantErr: "request id exceeds",
		},
		{
			name: "command too long",
			payload: ExecuteCommandPayload{
				RequestID: "r1",
				Command:   strings.Repeat("a", maxExecuteCommandLength+1),
			},
			wantErr: "command exceeds",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.ExecuteCommand(context.Background(), "a1", tc.payload)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestReadFileValidation(t *testing.T) {
	s := NewServer(nil)
	if _, err := s.ReadFile(context.Background(), "", ReadFilePayload{RequestID: "r1"}); err == nil {
		t.Fatalf("expected empty agent id error")
	}
	if _, err := s.ReadFile(context.Background(), "a1", ReadFilePayload{}); err == nil {
		t.Fatalf("expected empty request id error")
	}
}

func TestReadFileSecurityValidation(t *testing.T) {
	s := NewServer(nil)

	cases := []struct {
		name    string
		payload ReadFilePayload
		wantErr string
	}{
		{
			name: "missing path",
			payload: ReadFilePayload{
				RequestID: "r1",
			},
			wantErr: "path is required",
		},
		{
			name: "path with control character",
			payload: ReadFilePayload{
				RequestID: "r1",
				Path:      "/etc/\x00passwd",
			},
			wantErr: "invalid control characters",
		},
		{
			name: "invalid target type",
			payload: ReadFilePayload{
				RequestID:  "r1",
				Path:       "/etc/passwd",
				TargetType: "node",
			},
			wantErr: "invalid target type",
		},
		{
			name: "container requires target id",
			payload: ReadFilePayload{
				RequestID:  "r1",
				Path:       "/etc/passwd",
				TargetType: "container",
			},
			wantErr: "target id is required",
		},
		{
			name: "invalid target id characters",
			payload: ReadFilePayload{
				RequestID:  "r1",
				Path:       "/etc/passwd",
				TargetType: "vm",
				TargetID:   "101; reboot",
			},
			wantErr: "target id contains invalid characters",
		},
		{
			name: "negative max bytes",
			payload: ReadFilePayload{
				RequestID: "r1",
				Path:      "/etc/passwd",
				MaxBytes:  -1,
			},
			wantErr: "max bytes cannot be negative",
		},
		{
			name: "max bytes too large",
			payload: ReadFilePayload{
				RequestID: "r1",
				Path:      "/etc/passwd",
				MaxBytes:  maxReadFileMaxBytes + 1,
			},
			wantErr: "max bytes cannot exceed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.ReadFile(context.Background(), "a1", tc.payload)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestConnectedAgentLookups(t *testing.T) {
	s := NewServer(nil)
	now := time.Now().Add(-1 * time.Minute)

	s.mu.Lock()
	s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", Hostname: "host1", ConnectedAt: now}}
	s.agents["a2"] = &agentConn{agent: ConnectedAgent{AgentID: "a2", Hostname: "host2", ConnectedAt: now}}
	s.mu.Unlock()

	if !s.IsAgentConnected("a1") {
		t.Fatalf("expected a1 to be connected")
	}
	if s.IsAgentConnected("missing") {
		t.Fatalf("expected missing to not be connected")
	}

	agentID, ok := s.GetAgentForHost("host2")
	if !ok || agentID != "a2" {
		t.Fatalf("expected GetAgentForHost(host2) = (a2, true), got (%q, %v)", agentID, ok)
	}
	if _, ok := s.GetAgentForHost("missing"); ok {
		t.Fatalf("expected missing host to return false")
	}

	agents := s.GetConnectedAgents()
	if len(agents) != 2 {
		t.Fatalf("expected 2 connected agents, got %d", len(agents))
	}
}
