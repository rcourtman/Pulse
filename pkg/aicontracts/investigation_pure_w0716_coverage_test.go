package aicontracts

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Test_w0716_contracts_OrchestratorInvestigationError_NilReceiverDefensiveBranches
// exercises the typed-nil-receiver defensive arms of Error/Unwrap/RunFailure/
// ProposalFailure. These methods are deliberately nil-safe so a typed nil
// pointer flowing through an error chain does not panic; the coverage gaps
// were exactly these nil arms.
func Test_w0716_contracts_OrchestratorInvestigationError_NilReceiverDefensiveBranches(t *testing.T) {
	// Typed nil pointer of *OrchestratorInvestigationError.
	var nilTypedErr *OrchestratorInvestigationError = (*OrchestratorInvestigationError)(nil)

	if got := nilTypedErr.Error(); got != "" {
		t.Fatalf("nil receiver Error() = %q, want %q", got, "")
	}
	if got := nilTypedErr.Unwrap(); got != nil {
		t.Fatalf("nil receiver Unwrap() = %v, want nil", got)
	}
	if got := nilTypedErr.RunFailure(); got != nil {
		t.Fatalf("nil receiver RunFailure() = %v, want nil", got)
	}
	if got := nilTypedErr.ProposalFailure(); got != nil {
		t.Fatalf("nil receiver ProposalFailure() = %v, want nil", got)
	}
}

// Test_w0716_contracts_OrchestratorInvestigationError_PopulatedErrorAndUnwrap
// covers the populated arms of Error() (0% covered — never called directly by
// existing tests) and Unwrap() (populated arm), asserting the joined string
// carries both failure channels and Unwrap returns exactly the two errors.
func Test_w0716_contracts_OrchestratorInvestigationError_PopulatedErrorAndUnwrap(t *testing.T) {
	runFailure := errors.New("provider unavailable")
	err := NewOrchestratorInvestigationError(runFailure, ErrInvestigationProposalAmbiguous)
	oie, ok := err.(*OrchestratorInvestigationError)
	if !ok {
		t.Fatalf("expected *OrchestratorInvestigationError, got %T", err)
	}

	msg := oie.Error()
	if !strings.Contains(msg, "provider unavailable") {
		t.Fatalf("Error() missing run failure message: %q", msg)
	}
	if !strings.Contains(msg, ErrInvestigationProposalAmbiguous.Error()) {
		t.Fatalf("Error() missing proposal failure message: %q", msg)
	}

	unwrapped := oie.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("Unwrap() returned %d errors, want 2", len(unwrapped))
	}
	if unwrapped[0] != runFailure {
		t.Fatalf("Unwrap()[0] = %v, want %v", unwrapped[0], runFailure)
	}
	if unwrapped[1] != ErrInvestigationProposalAmbiguous {
		t.Fatalf("Unwrap()[1] = %v, want %v", unwrapped[1], ErrInvestigationProposalAmbiguous)
	}
}

// Test_w0716_contracts_NewOrchestratorInvestigationError_NilArm covers the
// uncovered both-nil arm of NewOrchestratorInvestigationError, which must
// return a literal nil (not a typed nil pointer) so errors.Is/!=nil checks
// behave correctly downstream.
func Test_w0716_contracts_NewOrchestratorInvestigationError_NilArm(t *testing.T) {
	if got := NewOrchestratorInvestigationError(nil, nil); got != nil {
		t.Fatalf("NewOrchestratorInvestigationError(nil, nil) = %#v, want nil", got)
	}
}

// Test_w0716_contracts_NewOrchestratorInvestigationError_PartialFailure
// covers the constructor's populated arm when only one channel is set,
// confirming the other accessor returns nil and the build path works for a
// run-only failure (mirrors a completed-but-proposal-rejected outcome).
func Test_w0716_contracts_NewOrchestratorInvestigationError_PartialFailure(t *testing.T) {
	runFailure := errors.New("provider unavailable")
	err := NewOrchestratorInvestigationError(runFailure, nil)
	if err == nil {
		t.Fatal("expected non-nil error when only runFailure set")
	}
	oie, ok := err.(*OrchestratorInvestigationError)
	if !ok {
		t.Fatalf("expected *OrchestratorInvestigationError, got %T", err)
	}
	if oie.RunFailure() != runFailure {
		t.Fatalf("RunFailure() = %v, want %v", oie.RunFailure(), runFailure)
	}
	if oie.ProposalFailure() != nil {
		t.Fatalf("ProposalFailure() = %v, want nil", oie.ProposalFailure())
	}
}

// Test_w0716_contracts_DefaultInvestigationConfig_FieldDefaults asserts every
// default field value of the returned InvestigationConfig. The function is
// pure and previously had 0% coverage; pinning each field guards the safety
// ceilings and budgets against silent drift.
func Test_w0716_contracts_DefaultInvestigationConfig_FieldDefaults(t *testing.T) {
	cfg := DefaultInvestigationConfig()

	// MaxTurns reserves two responses on top of the default 15-call evidence
	// budget: 15 + 2 = 17.
	if cfg.MaxTurns != 17 {
		t.Fatalf("MaxTurns = %d, want 17", cfg.MaxTurns)
	}
	if cfg.MaxEvidenceCalls != 15 {
		t.Fatalf("MaxEvidenceCalls = %d, want 15", cfg.MaxEvidenceCalls)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Fatalf("Timeout = %v, want 10m", cfg.Timeout)
	}
	if cfg.MaxConcurrent != 3 {
		t.Fatalf("MaxConcurrent = %d, want 3", cfg.MaxConcurrent)
	}
	if cfg.MaxAttemptsPerFinding != 3 {
		t.Fatalf("MaxAttemptsPerFinding = %d, want 3", cfg.MaxAttemptsPerFinding)
	}
	if cfg.CooldownDuration != 1*time.Hour {
		t.Fatalf("CooldownDuration = %v, want 1h", cfg.CooldownDuration)
	}
	if cfg.TimeoutCooldownDuration != 10*time.Minute {
		t.Fatalf("TimeoutCooldownDuration = %v, want 10m", cfg.TimeoutCooldownDuration)
	}
	if cfg.VerificationDelay != 30*time.Second {
		t.Fatalf("VerificationDelay = %v, want 30s", cfg.VerificationDelay)
	}
}

// Test_w0716_contracts_InvestigationModelTurnLimit covers both the positive
// path (budget + 2) and the <=0 clamp branch, which resets the evidence budget
// to its default of 15 before adding the two reserved turns.
func Test_w0716_contracts_InvestigationModelTurnLimit(t *testing.T) {
	tests := []struct {
		name             string
		maxEvidenceCalls int
		want             int
	}{
		{name: "positive limit adds two reserved turns", maxEvidenceCalls: 15, want: 17},
		{name: "small positive value honored", maxEvidenceCalls: 3, want: 5},
		{name: "zero clamps to default budget then adds two", maxEvidenceCalls: 0, want: 17},
		{name: "negative clamps to default budget then adds two", maxEvidenceCalls: -1, want: 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InvestigationModelTurnLimit(tt.maxEvidenceCalls); got != tt.want {
				t.Fatalf("InvestigationModelTurnLimit(%d) = %d, want %d", tt.maxEvidenceCalls, got, tt.want)
			}
		})
	}
}

// Test_w0716_contracts_InvestigationRecordFix_NormalizeCollections covers the
// zero-value normalization path: a nil Commands slice must become a non-nil
// empty slice so downstream JSON/wire projections emit "commands":[] rather
// than omitting the field.
func Test_w0716_contracts_InvestigationRecordFix_NormalizeCollections(t *testing.T) {
	fix := InvestigationRecordFix{}.NormalizeCollections()
	if fix.Commands == nil {
		t.Fatal("NormalizeCollections must replace nil Commands with a non-nil empty slice")
	}
	if len(fix.Commands) != 0 {
		t.Fatalf("Commands len = %d, want 0", len(fix.Commands))
	}
}
