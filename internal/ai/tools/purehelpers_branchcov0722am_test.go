package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
)

// This file raises branch coverage for two pure helpers that were previously
// 0.0% covered:
//   - dockerRuntimeCommand (tools_docker.go) — switch on a normalised runtime
//     string with a nil-host guard and a default arm.
//   - approvalDecisionActor (action_audit.go) — precedence chain of
//     req.DecidedBy -> fallback -> approvalAuditActor constant.
//
// Both helpers are pure (no I/O) and return exact strings, so every case
// asserts the concrete return value. approvalAuditActor is defined in
// action_audit.go as approval.RequesterPulseAssistant ("pulse_assistant"); the
// default-arm cases assert that literal so a constant change would surface here.

// TestBranchcov0722R2DockerRuntimeCommand covers every branch arm of
// dockerRuntimeCommand: the nil-host guard, both recognised switch cases
// ("podman", "docker"), the surrounding-whitespace + mixed-case normalisation
// path (TrimSpace + ToLower), an unrecognised runtime falling through to the
// default, and an empty runtime falling through to the default. Every arm is
// asserted against the exact command string it returns.
func TestBranchcov0722R2DockerRuntimeCommand(t *testing.T) {
	tests := []struct {
		name string
		host *models.DockerHost
		want string
	}{
		// --- nil-host guard: default command returned. ---
		{name: "nil_host_returns_docker", host: nil, want: "docker"},

		// --- recognised switch arms (exact, already-canonical input). ---
		{name: "podman_runtime_returns_podman", host: &models.DockerHost{Runtime: "podman"}, want: "podman"},
		{name: "docker_runtime_returns_docker", host: &models.DockerHost{Runtime: "docker"}, want: "docker"},

		// --- normalisation: TrimSpace + ToLower route mixed input to a case. ---
		{name: "podman_with_whitespace_and_mixed_case", host: &models.DockerHost{Runtime: "  PoDmAn  "}, want: "podman"},
		{name: "docker_with_whitespace_and_upper_case", host: &models.DockerHost{Runtime: "\tDOCKER\n"}, want: "docker"},

		// --- default arm: unrecognised runtime falls through to "docker". ---
		{name: "unrecognised_runtime_falls_through", host: &models.DockerHost{Runtime: "containerd"}, want: "docker"},

		// --- default arm: empty runtime falls through to "docker". ---
		{name: "empty_runtime_falls_through", host: &models.DockerHost{Runtime: ""}, want: "docker"},
		{name: "whitespace_only_runtime_falls_through", host: &models.DockerHost{Runtime: "   "}, want: "docker"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dockerRuntimeCommand(tc.host)
			assert.Equal(t, tc.want, got, "dockerRuntimeCommand mismatch")
		})
	}
}

// TestBranchcov0722R2ApprovalDecisionActor covers every branch arm of
// approvalDecisionActor. The function implements a three-step precedence chain:
//  1. req.DecidedBy (trimmed, if req != nil and non-empty) wins;
//  2. otherwise the fallback argument (trimmed, if non-empty);
//  3. otherwise the package constant approvalAuditActor ("pulse_assistant").
//
// Note: the function inspects ONLY req.DecidedBy — there are no further
// candidate fields. Cases below exercise each precedence step, the
// whitespace-only fall-through for both DecidedBy and fallback, trimming of the
// returned value, and the all-empty path (including an empty fallback).
func TestBranchcov0722R2ApprovalDecisionActor(t *testing.T) {
	// approvalAuditActor is approval.RequesterPulseAssistant == "pulse_assistant".
	const wantDefaultActor = "pulse_assistant"
	assert.Equal(t, wantDefaultActor, approvalAuditActor,
		"precondition: approvalAuditActor must equal %q", wantDefaultActor)
	assert.Equal(t, wantDefaultActor, approval.RequesterPulseAssistant,
		"precondition: approvalAuditActor tracks approval.RequesterPulseAssistant")

	tests := []struct {
		name     string
		req      *approval.ApprovalRequest
		fallback string
		want     string
	}{
		// --- nil request: falls straight through to fallback / constant. ---
		{name: "nil_req_uses_fallback", req: nil, fallback: "alice", want: "alice"},
		{name: "nil_req_empty_fallback_uses_constant", req: nil, fallback: "", want: wantDefaultActor},
		{name: "nil_req_whitespace_fallback_uses_constant", req: nil, fallback: "   ", want: wantDefaultActor},

		// --- DecidedBy wins (step 1), including over a populated fallback. ---
		{name: "decidedby_wins", req: &approval.ApprovalRequest{DecidedBy: "bob"}, fallback: "", want: "bob"},
		{name: "decidedby_wins_over_fallback", req: &approval.ApprovalRequest{DecidedBy: "bob"}, fallback: "alice", want: "bob"},
		{name: "decidedby_is_trimmed", req: &approval.ApprovalRequest{DecidedBy: "  bob  "}, fallback: "", want: "bob"},

		// --- DecidedBy whitespace-only/empty falls through to fallback (step 2). ---
		{name: "whitespace_decidedby_falls_to_fallback", req: &approval.ApprovalRequest{DecidedBy: "   "}, fallback: "alice", want: "alice"},
		{name: "empty_decidedby_falls_to_fallback", req: &approval.ApprovalRequest{DecidedBy: ""}, fallback: "alice", want: "alice"},
		{name: "fallback_is_trimmed_when_used", req: &approval.ApprovalRequest{DecidedBy: ""}, fallback: "  alice  ", want: "alice"},

		// --- all candidates empty: constant is the final default (step 3). ---
		{name: "empty_decidedby_empty_fallback_uses_constant", req: &approval.ApprovalRequest{DecidedBy: ""}, fallback: "", want: wantDefaultActor},
		{name: "whitespace_decidedby_whitespace_fallback_uses_constant", req: &approval.ApprovalRequest{DecidedBy: "   "}, fallback: "   ", want: wantDefaultActor},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := approvalDecisionActor(tc.req, tc.fallback)
			assert.Equal(t, tc.want, got, "approvalDecisionActor mismatch")
		})
	}
}
