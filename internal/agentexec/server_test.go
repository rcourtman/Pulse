package agentexec

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHostStorageCleanupPayloadIsClosedAndSelectorFree(t *testing.T) {
	typeOf := reflect.TypeOf(HostStorageCleanupPayload{})
	want := []string{"RequestID", "ActionID", "Operation", "OperationVersion", "RequestDigest", "ExpectedFingerprint", "Timeout"}
	if typeOf.NumField() != len(want) {
		t.Fatalf("host storage cleanup payload fields = %d, want %d", typeOf.NumField(), len(want))
	}
	for i, name := range want {
		if field := typeOf.Field(i); field.Name != name {
			t.Fatalf("field %d = %s, want %s", i, field.Name, name)
		}
	}
}

func allowAllTestTokens(string, string, string) bool { return true }

func TestNewServerRequiresValidateToken(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when validateToken is nil")
		}
	}()

	_ = NewServer(nil)
}

func TestExecuteCommandAgentNotConnected(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	_, err := s.ExecuteCommand(context.Background(), "missing", ExecuteCommandPayload{RequestID: "r1", Timeout: 1})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestReadFileAgentNotConnected(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	_, err := s.ReadFile(context.Background(), "missing", ReadFilePayload{RequestID: "r1"})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestExecuteCommandValidation(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	if _, err := s.ExecuteCommand(context.Background(), "", ExecuteCommandPayload{RequestID: "r1"}); err == nil {
		t.Fatalf("expected empty agent id error")
	}
	if _, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{}); err == nil {
		t.Fatalf("expected empty request id error")
	}
}

func TestExecuteCommandSecurityValidation(t *testing.T) {
	s := NewServer(allowAllTestTokens)

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
			name: "unsupported host target type rejected",
			payload: ExecuteCommandPayload{
				RequestID:  "r1",
				Command:    "echo ok",
				TargetType: "host",
			},
			wantErr: `invalid target type "host"`,
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

func TestExecuteCommandSecurityValidation_RequiresApprovalIDWhenPolicyRequiresApproval(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	_, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{
		RequestID: "r1",
		Command:   "echo ok",
		Timeout:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected approval-required error, got %v", err)
	}
}

func TestAuthorizeCommandPayload_TrustedBypassesApproval(t *testing.T) {
	s := NewServer(allowAllTestTokens)

	// A command in the require-approval list (docker exec wraps Discovery probes).
	cmd := ExecuteCommandPayload{
		RequestID: "r1",
		Command:   "docker exec pbs sh -c 'cat /etc/os-release'",
		Timeout:   1,
	}

	if err := s.authorizeCommandPayload(cmd); err == nil {
		t.Fatalf("baseline: expected approval-required error without Trusted, got nil")
	}

	cmd.Trusted = true
	if err := s.authorizeCommandPayload(cmd); err != nil {
		t.Fatalf("Trusted payload should bypass approval gate, got %v", err)
	}

	// Blocked commands are still blocked even when Trusted (defense in depth).
	blocked := ExecuteCommandPayload{
		RequestID: "r2",
		Command:   "rm -rf /",
		Trusted:   true,
		Timeout:   1,
	}
	if err := s.authorizeCommandPayload(blocked); err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("Trusted must not bypass PolicyBlock; got %v", err)
	}
}

func TestExecuteCommand_TrustedSkipsApprovalGrantMint(t *testing.T) {
	// Regression for the auto-grant path: even when the policy says
	// RequireApproval, a Trusted command without an ApprovalID must not
	// trigger NewCommandApprovalGrant (which errors "approval id is
	// required"). The grant is only meaningful for user-driven approvals.
	s := NewServer(allowAllTestTokens)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()
	ac := &agentConn{
		conn:             serverConn,
		agent:            ConnectedAgent{AgentID: "a1"},
		approvalGrantKey: []byte("not-empty-so-the-mint-path-is-reachable"),
		done:             make(chan struct{}),
		writeMu:          sync.Mutex{},
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := s.ExecuteCommand(ctx, "a1", ExecuteCommandPayload{
		RequestID: "r1",
		Command:   "docker exec pbs sh -c 'cat /etc/os-release'",
		Trusted:   true,
		Timeout:   1,
	})
	// Expected: command is dispatched and we time out waiting for a result
	// from the fake conn (no agent code on the other side). What must NOT
	// appear is the "approval id is required" / "failed to issue approval
	// grant" mint failure.
	if err != nil && (strings.Contains(err.Error(), "failed to issue approval grant") || strings.Contains(err.Error(), "approval id is required")) {
		t.Fatalf("Trusted command must skip auto-grant mint, got %v", err)
	}
}

func TestReadFileValidation(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	if _, err := s.ReadFile(context.Background(), "", ReadFilePayload{RequestID: "r1"}); err == nil {
		t.Fatalf("expected empty agent id error")
	}
	if _, err := s.ReadFile(context.Background(), "a1", ReadFilePayload{}); err == nil {
		t.Fatalf("expected empty request id error")
	}
}

func TestReadFileSecurityValidation(t *testing.T) {
	s := NewServer(allowAllTestTokens)

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
			name: "unsupported host target type rejected",
			payload: ReadFilePayload{
				RequestID:  "r1",
				Path:       "/etc/passwd",
				TargetType: "host",
			},
			wantErr: `invalid target type "host"`,
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
	s := NewServer(allowAllTestTokens)
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

func TestGetAgentForHostNormalizesFQDNAndCase(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	now := time.Now().Add(-1 * time.Minute)

	s.mu.Lock()
	s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", Hostname: "prox97.seftic.local", ConnectedAt: now}}
	s.mu.Unlock()

	cases := []string{"prox97", "PROX97", "prox97.seftic.local", "Prox97.Seftic.Local"}
	for _, lookup := range cases {
		agentID, ok := s.GetAgentForHost(lookup)
		if !ok || agentID != "a1" {
			t.Errorf("GetAgentForHost(%q) = (%q, %v), want (a1, true)", lookup, agentID, ok)
		}
	}

	if _, ok := s.GetAgentForHost(""); ok {
		t.Errorf("GetAgentForHost(\"\") should return false")
	}
}

func TestGetAgentForHostKeepsDistinctFQDNsSeparate(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	now := time.Now().Add(-1 * time.Minute)

	s.mu.Lock()
	s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", Hostname: "prox97.a.local", ConnectedAt: now}}
	s.agents["a2"] = &agentConn{agent: ConnectedAgent{AgentID: "a2", Hostname: "prox97.b.local", ConnectedAt: now}}
	s.mu.Unlock()

	agentID, ok := s.GetAgentForHost("prox97.a.local")
	if !ok || agentID != "a1" {
		t.Fatalf("GetAgentForHost(%q) = (%q, %v), want (a1, true)", "prox97.a.local", agentID, ok)
	}

	agentID, ok = s.GetAgentForHost("prox97.b.local")
	if !ok || agentID != "a2" {
		t.Fatalf("GetAgentForHost(%q) = (%q, %v), want (a2, true)", "prox97.b.local", agentID, ok)
	}
}
