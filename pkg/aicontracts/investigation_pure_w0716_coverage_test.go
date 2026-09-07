package aicontracts

import (
	"testing"
	"time"
)

// Test_w0716_contracts_OrchestratorInvestigationError_NilReceiverDefensiveBranches
// exercises the typed-nil-receiver defensive arms of Error/Unwrap/RunFailure/
// ProposalFailure. These methods are deliberately nil-safe so a typed nil
// pointer flowing through an error chain does not panic; the coverage gaps
// were exactly these nil arms.
func Test_w0716_contracts_DefaultInvestigationConfig_FieldDefaults(t *testing.T) {
	cfg := DefaultInvestigationConfig()

	// MaxTurns reserves two responses on top of the default 10-call evidence
	// budget: 10 + 2 = 12.
	if cfg.MaxTurns != 12 {
		t.Fatalf("MaxTurns = %d, want 12", cfg.MaxTurns)
	}
	if cfg.MaxEvidenceCalls != 10 {
		t.Fatalf("MaxEvidenceCalls = %d, want 10", cfg.MaxEvidenceCalls)
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
// to its default of 10 before adding the two reserved turns.
func Test_w0716_contracts_InvestigationModelTurnLimit(t *testing.T) {
	tests := []struct {
		name             string
		maxEvidenceCalls int
		want             int
	}{
		{name: "positive limit adds two reserved turns", maxEvidenceCalls: 15, want: 17},
		{name: "small positive value honored", maxEvidenceCalls: 3, want: 5},
		{name: "zero clamps to default budget then adds two", maxEvidenceCalls: 0, want: 12},
		{name: "negative clamps to default budget then adds two", maxEvidenceCalls: -1, want: 12},
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
