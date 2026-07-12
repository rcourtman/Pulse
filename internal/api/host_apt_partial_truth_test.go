package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHostUpdatePartialTruthProjectsPhaseHealthRemainingAndRecovery(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	snapshot := func(hash string, pending int, checkedAt time.Time) agentexec.HostPackageUpdateSnapshot {
		return agentexec.HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hash, PendingCount: pending, CheckedAt: checkedAt}
	}
	cases := []struct {
		name             string
		payload          agentexec.HostUpdateResultPayload
		wantExecution    unified.ActionExecutionStatus
		wantVerification unified.ActionVerificationStatus
		wantSummary      []string
	}{
		{
			name: "preflight refusal",
			payload: agentexec.HostUpdateResultPayload{RequestID: "preflight.dispatch.1", ActionID: "preflight", ExecutionPhase: agentexec.HostUpdatePhasePreflight,
				Before: snapshot(testHostPackageInventoryHash, 3, now), After: snapshot(testHostPackageInventoryHash, 3, now), Verification: agentexec.HostUpdateVerificationInconclusive},
			wantExecution: unified.ActionExecutionNotRun, wantVerification: unified.ActionVerificationInconclusive,
			wantSummary: []string{"phase=preflight", "3 pending after", "health: unknown", "recovery required: false"},
		},
		{
			name: "refresh failure",
			payload: agentexec.HostUpdateResultPayload{RequestID: "refresh.dispatch.1", ActionID: "refresh", ExecutionPhase: agentexec.HostUpdatePhaseRefresh,
				Before: snapshot(testHostPackageInventoryHash, 3, now), After: snapshot(testHostPackageInventoryHash, 3, now), Verification: agentexec.HostUpdateVerificationInconclusive},
			wantExecution: unified.ActionExecutionNotRun, wantVerification: unified.ActionVerificationInconclusive,
			wantSummary: []string{"phase=refresh", "3 pending after", "health: unknown", "recovery required: false"},
		},
		{
			name: "partial install unhealthy",
			payload: agentexec.HostUpdateResultPayload{RequestID: "install.dispatch.1", ActionID: "install", ExecutionPhase: agentexec.HostUpdatePhaseInstall, MutationStarted: true, HealthChecked: true, PackageManagerHealthy: false, RecoveryRequired: true,
				Before: snapshot(testHostPackageInventoryHash, 3, now.Add(-time.Second)), After: snapshot(testHostPackageInventoryHash, 2, now), Verification: agentexec.HostUpdateVerificationFailed},
			wantExecution: unified.ActionExecutionInconclusive, wantVerification: unified.ActionVerificationContradicted,
			wantSummary: []string{"phase=install", "2 pending after", "health: unhealthy", "recovery required: true"},
		},
		{
			name: "verify failure with remaining updates",
			payload: agentexec.HostUpdateResultPayload{RequestID: "verify.dispatch.1", ActionID: "verify", Success: true, ExecutionPhase: agentexec.HostUpdatePhaseVerify, MutationStarted: true, HealthChecked: true, PackageManagerHealthy: true, RecoveryRequired: true,
				Before: snapshot(testHostPackageInventoryHash, 3, now.Add(-time.Second)), After: snapshot(testHostPackageEmptyInventoryHash, 1, now), Verification: agentexec.HostUpdateVerificationFailed},
			wantExecution: unified.ActionExecutionSucceeded, wantVerification: unified.ActionVerificationContradicted,
			wantSummary: []string{"phase=verify", "1 pending after", "health: healthy", "recovery required: true"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := agentexec.ValidateHostUpdateResultPayload(&tc.payload); err != nil {
				t.Fatalf("valid typed result rejected: %v", err)
			}
			summary := hostUpdateResultSummary(tc.payload)
			result, err := hostAPTExecutionResult("agent:host-1", "agent-1", agentexec.HostUpdateOperationInstall, summary, tc.payload.Success, tc.payload.MutationStarted, tc.payload.Verification, true, true, tc.payload.HealthChecked, tc.payload.PackageManagerHealthy, tc.payload.RecoveryRequired, tc.payload.Before.CheckedAt, tc.payload.After.CheckedAt, now, now)
			if err != nil {
				t.Fatal(err)
			}
			if result.ActionResultV2.Execution.Status != tc.wantExecution || result.ActionResultV2.Verification.Status != tc.wantVerification {
				t.Fatalf("truth=%#v", result.ActionResultV2)
			}
			for _, want := range tc.wantSummary {
				if !strings.Contains(result.ActionResultV2.Execution.Summary, want) {
					t.Fatalf("execution summary %q missing %q", result.ActionResultV2.Execution.Summary, want)
				}
			}
			if result.ActionResultV2.Compensation.Support != unified.ActionCompensationUnavailable || result.ActionResultV2.Compensation.Status != unified.ActionCompensationNotAvailable {
				t.Fatalf("compensation=%#v", result.ActionResultV2.Compensation)
			}
			if tc.payload.RecoveryRequired {
				if !strings.Contains(result.ActionResultV2.Compensation.Summary, "separately governed recovery is required") {
					t.Fatalf("partial update compensation=%#v", result.ActionResultV2.Compensation)
				}
			} else if strings.Contains(result.ActionResultV2.Compensation.Summary, "recovery is required") || !strings.Contains(result.ActionResultV2.Compensation.Summary, "no recovery action is required") {
				t.Fatalf("non-recovery update compensation=%#v", result.ActionResultV2.Compensation)
			}
		})
	}
}

func TestHostStorageCleanupPartialTruthIsNonRollbackableAndRequiresRescan(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		phase     string
		reclaimed int64
	}{
		{name: "clean failure", phase: agentexec.HostStorageCleanupPhaseClean},
		{name: "verify failure with measured effect", phase: agentexec.HostStorageCleanupPhaseVerify, reclaimed: 128 * 1024 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := agentexec.HostStorageCleanupResultPayload{
				RequestID: "cleanup.dispatch.1", ActionID: "cleanup", ExecutionPhase: tc.phase, MutationStarted: true,
				Before:         agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupFingerprint, ReclaimableBytes: 512 * 1024 * 1024, CheckedAt: now.Add(-time.Second)},
				After:          agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: testHostStorageCleanupAfterFingerprint, ReclaimableBytes: 512*1024*1024 - tc.reclaimed, CheckedAt: now},
				ReclaimedBytes: tc.reclaimed, Verification: agentexec.HostStorageCleanupVerificationFailed,
			}
			if tc.phase == agentexec.HostStorageCleanupPhaseVerify {
				payload.Success = true
			}
			if err := agentexec.ValidateHostStorageCleanupResultPayload(&payload); err != nil {
				t.Fatal(err)
			}
			summary := hostStorageCleanupResultSummary(payload)
			result, err := hostAPTExecutionResult("agent:host-cleanup", "agent-1", agentexec.HostStorageCleanupOperationPackageCache, summary, payload.Success, true, payload.Verification, true, false, false, false, false, payload.Before.CheckedAt, payload.After.CheckedAt, now, now)
			if err != nil {
				t.Fatal(err)
			}
			wantExecution := unified.ActionExecutionInconclusive
			if tc.phase == agentexec.HostStorageCleanupPhaseVerify {
				wantExecution = unified.ActionExecutionSucceeded
			}
			if result.ActionResultV2.Execution.Status != wantExecution || result.ActionResultV2.Verification.Status != unified.ActionVerificationContradicted || !strings.Contains(summary, "rollback available: false") || !strings.Contains(summary, "rescan required: true") || !strings.Contains(summary, "phase="+tc.phase) {
				t.Fatalf("truth=%#v summary=%q", result.ActionResultV2, summary)
			}
			if !strings.Contains(result.ActionResultV2.Compensation.Summary, "irreversible and non-rollbackable") || !strings.Contains(result.ActionResultV2.Compensation.Summary, "never restoration") {
				t.Fatalf("cleanup compensation=%#v", result.ActionResultV2.Compensation)
			}
		})
	}
}

func TestHostUpdateResultRejectsImpossibleHealthAndRecoveryCombinations(t *testing.T) {
	base := agentexec.HostUpdateResultPayload{RequestID: "bad.dispatch.1", ActionID: "bad", ExecutionPhase: agentexec.HostUpdatePhaseInstall, MutationStarted: true, RecoveryRequired: true, Verification: agentexec.HostUpdateVerificationInconclusive}
	cases := map[string]func(*agentexec.HostUpdateResultPayload){
		"refresh cannot claim install mutation": func(p *agentexec.HostUpdateResultPayload) { p.ExecutionPhase = agentexec.HostUpdatePhaseRefresh },
		"healthy without check":                 func(p *agentexec.HostUpdateResultPayload) { p.PackageManagerHealthy = true; p.HealthChecked = false },
		"partial install without recovery": func(p *agentexec.HostUpdateResultPayload) {
			p.ExecutionPhase = agentexec.HostUpdatePhaseInstall
			p.RecoveryRequired = false
		},
		"success complete with recovery": func(p *agentexec.HostUpdateResultPayload) {
			p.Success = true
			p.ExecutionPhase = agentexec.HostUpdatePhaseComplete
			p.HealthChecked = true
			p.PackageManagerHealthy = true
		},
		"preflight recovery without mutation": func(p *agentexec.HostUpdateResultPayload) {
			p.ExecutionPhase = agentexec.HostUpdatePhasePreflight
			p.MutationStarted = false
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			payload := base
			mutate(&payload)
			if err := agentexec.ValidateHostUpdateResultPayload(&payload); err == nil {
				t.Fatalf("impossible result accepted: %#v", payload)
			}
		})
	}
}

func TestHostAPTActionTruthRedactsRawAgentAndPackageDetail(t *testing.T) {
	now := time.Now().UTC()
	payload := agentexec.HostUpdateResultPayload{
		RequestID: "redact.dispatch.1", ActionID: "redact", ExecutionPhase: agentexec.HostUpdatePhaseInstall, MutationStarted: true, RecoveryRequired: true,
		Before:       agentexec.HostPackageUpdateSnapshot{PendingCount: 2, Packages: []agentexec.HostPackageUpdate{{Name: "private-package"}}, Error: "repo token secret", CheckedAt: now.Add(-time.Second)},
		After:        agentexec.HostPackageUpdateSnapshot{PendingCount: 1, Packages: []agentexec.HostPackageUpdate{{Name: "private-package"}}, Error: "stderr /private/cache/path", CheckedAt: now},
		Verification: agentexec.HostUpdateVerificationInconclusive, Error: "raw stderr token secret",
	}
	result, err := hostAPTExecutionResult("agent:host", "agent", agentexec.HostUpdateOperationInstall, hostUpdateResultSummary(payload), false, true, payload.Verification, true, true, false, false, true, payload.Before.CheckedAt, payload.After.CheckedAt, now, now)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Result  *unified.ExecutionResult `json:"result"`
		Finding string                   `json:"finding"`
		Notice  string                   `json:"notification"`
	}{Result: result, Finding: "APT update requires recovery", Notice: "Host update outcome needs review"})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-package", "repo token", "raw stderr", "/private/cache/path", "token secret"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("raw detail %q escaped canonical projections: %s", forbidden, encoded)
		}
	}
}
