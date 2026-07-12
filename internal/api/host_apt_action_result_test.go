package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHostAPTActionResultRefreshFailureIsPossiblePartialEffect(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result, err := hostAPTExecutionResult("agent:host-1", "host-1", agentexec.HostUpdateOperationInstall, "refresh failed", false, true, agentexec.HostUpdateVerificationInconclusive, true, time.Time{}, time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActionResultV2.Execution.Status != unified.ActionExecutionInconclusive || result.ActionResultV2.Execution.ReasonCode != "possible_partial_effect" || result.ActionResultV2.Execution.Status == unified.ActionExecutionNotRun {
		t.Fatalf("canonical execution truth = %#v", result.ActionResultV2.Execution)
	}
	if result.ActionResultV2.Compensation.Support != unified.ActionCompensationUnavailable || result.ActionResultV2.Compensation.Status != unified.ActionCompensationNotAvailable {
		t.Fatalf("compensation truth = %#v", result.ActionResultV2.Compensation)
	}
}

func TestHostAPTActionResultPreservesAgentObservedAndServerReceivedTimes(t *testing.T) {
	checkedAt := time.Date(2026, 7, 12, 8, 59, 0, 0, time.UTC)
	observedAt := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result, err := hostAPTExecutionResult("agent:host-1", "host-1", agentexec.HostUpdateOperationInstall, "updates complete", true, true, agentexec.HostUpdateVerificationVerified, true, checkedAt.Add(-time.Second), checkedAt, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationConfirmed || truth.EvidenceClass != unified.ActionEvidenceAgentAttested || len(truth.Evidence) != 1 {
		t.Fatalf("verification truth = %#v", truth)
	}
	if !truth.Evidence[0].ObservedAt.Equal(checkedAt) || !truth.Evidence[0].ReceivedAt.Equal(observedAt) {
		t.Fatalf("evidence timestamps = %#v", truth.Evidence[0])
	}
	if truth.Evidence[0].ObserverTrustDomain != truth.Evidence[0].ExecutorTrustDomain {
		t.Fatalf("same-agent evidence was falsely independent: %#v", truth.Evidence[0])
	}
}

func TestHostAPTActionResultFutureVerifiedClaimFailsClosed(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result, err := hostAPTExecutionResult("agent:host-1", "host-1", agentexec.HostUpdateOperationInstall, "updates complete", true, true, agentexec.HostUpdateVerificationVerified, true, now, now.Add(unified.HostAPTTelemetryMaxClockSkew+time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive || result.ActionResultV2.Verification.EvidenceClass != unified.ActionEvidenceNone {
		t.Fatalf("future evidence did not fail closed: %#v", result.ActionResultV2.Verification)
	}
}

func TestHostAPTActionResultStaleReadbackFailsClosed(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result, err := hostAPTExecutionResult("agent:host-1", "host-1", agentexec.HostUpdateOperationInstall, "updates complete", true, true, agentexec.HostUpdateVerificationVerified, true, now.Add(-time.Hour-time.Minute), now.Add(-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationInconclusive || truth.EvidenceClass != unified.ActionEvidenceNone || truth.ReasonCode != "stale_agent_readback" {
		t.Fatalf("stale readback truth=%#v", truth)
	}
}

func TestHostAPTActionResultBeforeStateMismatchCannotBecomeContradictionEvidence(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result, err := hostAPTExecutionResult("agent:host-1", "host-1", agentexec.HostStorageCleanupOperationPackageCache, "cleanup contradicted", true, true, agentexec.HostStorageCleanupVerificationFailed, false, now.Add(-time.Minute), now, now)
	if err != nil {
		t.Fatal(err)
	}
	truth := result.ActionResultV2.Verification
	if truth.Status != unified.ActionVerificationInconclusive || truth.EvidenceClass != unified.ActionEvidenceNone || truth.ReasonCode != "before_state_mismatch" {
		t.Fatalf("mismatched before-state truth=%#v", truth)
	}
}
