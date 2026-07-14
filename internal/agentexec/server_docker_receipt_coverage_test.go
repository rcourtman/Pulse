package agentexec

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func TestMatchesPendingDockerUpdateOperation_Coverage(t *testing.T) {
	const agentID = "a1"

	baseResult := DockerContainerUpdateResultPayload{
		RequestID:        "req-1",
		ActionID:         "act-1",
		Operation:        "docker.container.update",
		OperationVersion: 3,
		RequestDigest:    "sha256:abc",
		ContainerID:      "AbC123",
	}
	baseIdentity := operationreceipt.Identity{
		AttemptID:        baseResult.RequestID,
		ActionID:         baseResult.ActionID,
		OperationKind:    baseResult.Operation,
		OperationVersion: baseResult.OperationVersion,
		RequestDigest:    baseResult.RequestDigest,
		AgentID:          strings.TrimSpace(agentID),
	}
	baseSubjectID := strings.ToLower(strings.TrimSpace(baseResult.ContainerID))

	storePending := func(s *Server, identity operationreceipt.Identity, subjectID string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.pendingHostOperations[pendingRequestKey(agentID, baseResult.RequestID)] = pendingHostOperation{
			identity:  identity,
			subjectID: subjectID,
		}
	}

	cases := []struct {
		name   string
		setup  func(s *Server)
		result DockerContainerUpdateResultPayload
		want   bool
	}{
		{
			name: "matching pending operation returns true",
			setup: func(s *Server) {
				storePending(s, baseIdentity, baseSubjectID)
			},
			result: baseResult,
			want:   true,
		},
		{
			name: "absent pending key returns false",
			setup: func(s *Server) {
			},
			result: baseResult,
			want:   false,
		},
		{
			name: "mismatched identity returns false",
			setup: func(s *Server) {
				different := baseIdentity
				different.ActionID = "different-action"
				storePending(s, different, baseSubjectID)
			},
			result: baseResult,
			want:   false,
		},
		{
			name: "mismatched container id returns false",
			setup: func(s *Server) {
				storePending(s, baseIdentity, "different-container")
			},
			result: baseResult,
			want:   false,
		},
		{
			name: "normalized padded mixed case container id still matches",
			setup: func(s *Server) {
				storePending(s, baseIdentity, baseSubjectID)
			},
			result: DockerContainerUpdateResultPayload{
				RequestID:        baseResult.RequestID,
				ActionID:         baseResult.ActionID,
				Operation:        baseResult.Operation,
				OperationVersion: baseResult.OperationVersion,
				RequestDigest:    baseResult.RequestDigest,
				ContainerID:      "  AbC123  ",
			},
			want: true,
		},
		{
			name: "mismatched request digest returns false",
			setup: func(s *Server) {
				different := baseIdentity
				different.RequestDigest = "sha256:different"
				storePending(s, different, baseSubjectID)
			},
			result: baseResult,
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(allowAllTestTokens)
			tc.setup(s)
			got := s.matchesPendingDockerUpdateOperation(agentID, tc.result)
			if got != tc.want {
				t.Fatalf("matchesPendingDockerUpdateOperation(%q, %+v) = %v, want %v", agentID, tc.result, got, tc.want)
			}
		})
	}
}

func TestAgentOperationReceiptVersion_Coverage(t *testing.T) {
	cases := []struct {
		name  string
		build func() (*Server, string)
		want  int
	}{
		{
			name:  "nil receiver returns zero",
			build: func() (*Server, string) { var s *Server; return s, "a1" },
			want:  0,
		},
		{
			name:  "empty server returns zero",
			build: func() (*Server, string) { return NewServer(allowAllTestTokens), "a1" },
			want:  0,
		},
		{
			name: "unknown agent returns zero",
			build: func() (*Server, string) {
				s := NewServer(allowAllTestTokens)
				s.mu.Lock()
				s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", OperationReceiptVersion: 7}}
				s.mu.Unlock()
				return s, "a2"
			},
			want: 0,
		},
		{
			name: "known agent returns its receipt version",
			build: func() (*Server, string) {
				s := NewServer(allowAllTestTokens)
				s.mu.Lock()
				s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", OperationReceiptVersion: 7}}
				s.mu.Unlock()
				return s, "a1"
			},
			want: 7,
		},
		{
			name: "whitespace padded agent id still resolves to known agent",
			build: func() (*Server, string) {
				s := NewServer(allowAllTestTokens)
				s.mu.Lock()
				s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", OperationReceiptVersion: 7}}
				s.mu.Unlock()
				return s, "  a1  "
			},
			want: 7,
		},
		{
			name: "known agent with zero receipt version returns zero",
			build: func() (*Server, string) {
				s := NewServer(allowAllTestTokens)
				s.mu.Lock()
				s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", OperationReceiptVersion: 0}}
				s.mu.Unlock()
				return s, "a1"
			},
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, agentID := tc.build()
			got := s.AgentOperationReceiptVersion(agentID)
			if got != tc.want {
				t.Fatalf("AgentOperationReceiptVersion(%q) = %d, want %d", agentID, got, tc.want)
			}
		})
	}
}
