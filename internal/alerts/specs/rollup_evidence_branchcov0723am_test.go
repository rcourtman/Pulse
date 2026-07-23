package specs

import "testing"

// TestBranchcov0723AmResourceIncidentRollupEvidenceValidate exercises every
// branch of ResourceIncidentRollupEvidence.Validate (types.go:950), which has
// two arms (empty code, non-positive incident count) plus the happy path.
func TestBranchcov0723AmResourceIncidentRollupEvidenceValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		evidence ResourceIncidentRollupEvidence
		wantErr  string // exact error message; empty means Validate must return nil
	}{
		{
			name:     "zero value surfaces code error first",
			evidence: ResourceIncidentRollupEvidence{},
			wantErr:  "code is required",
		},
		{
			// Non-empty string that trims to empty: pins the strings.TrimSpace
			// behaviour of the guard (a plain "Code == \"\"" check would pass this).
			name:     "whitespace-only code is treated as missing",
			evidence: ResourceIncidentRollupEvidence{Code: "   \t\n", IncidentCount: 5},
			wantErr:  "code is required",
		},
		{
			// Ordering proof: when the code guard passes but the count guard
			// fails, the count error is what surfaces.
			name:     "valid code with zero count rejected at count guard",
			evidence: ResourceIncidentRollupEvidence{Code: "capacity_runway_low", IncidentCount: 0},
			wantErr:  "incident count must be positive",
		},
		{
			// Boundary: one above the <= 0 threshold is the minimum valid count.
			name:     "minimum positive count is valid",
			evidence: ResourceIncidentRollupEvidence{Code: "capacity_runway_low", IncidentCount: 1},
			wantErr:  "",
		},
		{
			// Confirms there is no upper bound on incident count.
			name:     "large positive count is valid",
			evidence: ResourceIncidentRollupEvidence{Code: "replica_flapping", IncidentCount: 10_000},
			wantErr:  "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.evidence.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error %q, got nil", tc.wantErr)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}
