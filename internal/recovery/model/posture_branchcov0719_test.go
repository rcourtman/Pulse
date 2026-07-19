package model

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// validProviderState returns a ProtectionProviderState that is known to pass
// Validate. Test cases mutate a single field off this baseline to trip one
// validation branch at a time.
func validProviderState() ProtectionProviderState {
	return ProtectionProviderState{
		Provider:            ProviderProxmoxPVE,
		Source:              "source-1",
		Scope:               "scope-1",
		JobState:            OutcomeSuccess,
		HistoryCompleteness: ProtectionHistoryComplete,
		Permissions:         operationaltrust.EvidencePermissionsSufficient,
		EvidenceIDs:         []string{"evidence-1"},
	}
}

// validPosture returns a ProtectionPosture that is known to pass Validate.
func validPosture() ProtectionPosture {
	at := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	return ProtectionPosture{
		SubjectResourceID:     "resource-1",
		State:                 ProtectionStateProtected,
		LastAttemptAt:         &at,
		LastSuccessfulPointAt: &at,
		LastVerifiedAt:        &at,
		Freshness:             ProtectionFreshnessCurrent,
		Verification:          ProtectionVerificationVerified,
		Coverage:              ProtectionCoverageComplete,
		ProviderStates:        []ProtectionProviderState{validProviderState()},
		RepositoryResourceIDs: []string{"repo-1"},
		EvidenceIDs:           []string{"evidence-1"},
		Explanation:           "explanation-1",
		EvaluatedAt:           at,
	}
}

func TestProtectionState_Valid(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state ProtectionState
		want  bool
	}{
		{"protected", ProtectionStateProtected, true},
		{"attention", ProtectionStateAttention, true},
		{"unprotected", ProtectionStateUnprotected, true},
		{"unknown", ProtectionStateUnknown, true},
		{"bogus", ProtectionState("bogus"), false},
		{"empty", ProtectionState(""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.Valid(); got != tc.want {
				t.Fatalf("ProtectionState(%q).Valid() = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestProtectionFreshness_Valid(t *testing.T) {
	for _, tc := range []struct {
		name      string
		freshness ProtectionFreshness
		want      bool
	}{
		{"current", ProtectionFreshnessCurrent, true},
		{"stale", ProtectionFreshnessStale, true},
		{"unknown", ProtectionFreshnessUnknown, true},
		{"bogus", ProtectionFreshness("bogus"), false},
		{"empty", ProtectionFreshness(""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.freshness.Valid(); got != tc.want {
				t.Fatalf("ProtectionFreshness(%q).Valid() = %v, want %v", tc.freshness, got, tc.want)
			}
		})
	}
}

func TestProtectionVerification_Valid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		verification ProtectionVerification
		want         bool
	}{
		{"verified", ProtectionVerificationVerified, true},
		{"unverified", ProtectionVerificationUnverified, true},
		{"stale", ProtectionVerificationStale, true},
		{"unknown", ProtectionVerificationUnknown, true},
		{"bogus", ProtectionVerification("bogus"), false},
		{"empty", ProtectionVerification(""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.verification.Valid(); got != tc.want {
				t.Fatalf("ProtectionVerification(%q).Valid() = %v, want %v", tc.verification, got, tc.want)
			}
		})
	}
}

func TestProtectionCoverage_Valid(t *testing.T) {
	for _, tc := range []struct {
		name     string
		coverage ProtectionCoverage
		want     bool
	}{
		{"complete", ProtectionCoverageComplete, true},
		{"partial", ProtectionCoveragePartial, true},
		{"none", ProtectionCoverageNone, true},
		{"unknown", ProtectionCoverageUnknown, true},
		{"bogus", ProtectionCoverage("bogus"), false},
		{"empty", ProtectionCoverage(""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.coverage.Valid(); got != tc.want {
				t.Fatalf("ProtectionCoverage(%q).Valid() = %v, want %v", tc.coverage, got, tc.want)
			}
		})
	}
}

func TestProtectionHistoryCompleteness_Valid(t *testing.T) {
	for _, tc := range []struct {
		name         string
		completeness ProtectionHistoryCompleteness
		want         bool
	}{
		{"complete", ProtectionHistoryComplete, true},
		{"partial", ProtectionHistoryPartial, true},
		{"unavailable", ProtectionHistoryUnavailable, true},
		{"unknown", ProtectionHistoryUnknown, true},
		{"bogus", ProtectionHistoryCompleteness("bogus"), false},
		{"empty", ProtectionHistoryCompleteness(""), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.completeness.Valid(); got != tc.want {
				t.Fatalf("ProtectionHistoryCompleteness(%q).Valid() = %v, want %v", tc.completeness, got, tc.want)
			}
		})
	}
}

func TestProtectionProviderState_Clone(t *testing.T) {
	at := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	state := ProtectionProviderState{
		Provider:             ProviderProxmoxPVE,
		Source:               "source-1",
		Scope:                "scope-1",
		JobState:             OutcomeSuccess,
		HistoryCompleteness:  ProtectionHistoryComplete,
		Permissions:          operationaltrust.EvidencePermissionsSufficient,
		LastAttemptAt:        &at,
		LastSuccessAt:        &at,
		LastVerifiedAt:       &at,
		EvidenceIDs:          []string{"evidence-1", "evidence-2"},
		VerificationExpected: true,
	}

	t.Run("evidence slice mutation is isolated", func(t *testing.T) {
		clone := state.Clone()
		clone.EvidenceIDs[0] = "evidence-mutated"
		clone.EvidenceIDs = append(clone.EvidenceIDs, "evidence-3")
		if state.EvidenceIDs[0] != "evidence-1" {
			t.Fatalf("clone slice element mutation propagated to original: state.EvidenceIDs[0] = %q", state.EvidenceIDs[0])
		}
		if len(state.EvidenceIDs) != 2 {
			t.Fatalf("clone append propagated to original: len(state.EvidenceIDs) = %d, want 2", len(state.EvidenceIDs))
		}
	})

	t.Run("time pointer mutation is isolated", func(t *testing.T) {
		clone := state.Clone()
		if clone.LastAttemptAt == state.LastAttemptAt {
			t.Fatal("Clone aliased LastAttemptAt pointer instead of copying")
		}
		*clone.LastAttemptAt = at.Add(time.Hour)
		if !state.LastAttemptAt.Equal(at) {
			t.Fatalf("clone pointer mutation propagated to original: state.LastAttemptAt = %v, want %v", state.LastAttemptAt, at)
		}
	})

	t.Run("nil pointers and slices stay nil", func(t *testing.T) {
		empty := ProtectionProviderState{}
		clone := empty.Clone()
		if clone.LastAttemptAt != nil || clone.LastSuccessAt != nil || clone.LastVerifiedAt != nil {
			t.Fatalf("Clone of nil pointers produced non-nil pointers: %+v", clone)
		}
		if clone.EvidenceIDs != nil {
			t.Fatalf("Clone of nil EvidenceIDs produced non-nil: %v", clone.EvidenceIDs)
		}
	})
}

func TestProtectionProviderState_Validate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modify  func(s *ProtectionProviderState)
		wantErr string // substring of error message; "" means expect success
	}{
		{
			name:    "passes when fully populated",
			modify:  func(s *ProtectionProviderState) {},
			wantErr: "",
		},
		{
			name:    "missing provider",
			modify:  func(s *ProtectionProviderState) { s.Provider = "" },
			wantErr: "protection provider is required",
		},
		{
			name:    "whitespace-only provider",
			modify:  func(s *ProtectionProviderState) { s.Provider = Provider("   ") },
			wantErr: "protection provider is required",
		},
		{
			name:    "missing source",
			modify:  func(s *ProtectionProviderState) { s.Source = "" },
			wantErr: "protection provider source is required",
		},
		{
			name:    "missing scope",
			modify:  func(s *ProtectionProviderState) { s.Scope = "" },
			wantErr: "protection provider scope is required",
		},
		{
			name:    "invalid job state",
			modify:  func(s *ProtectionProviderState) { s.JobState = Outcome("bogus") },
			wantErr: "is invalid",
		},
		{
			name:    "invalid history completeness",
			modify:  func(s *ProtectionProviderState) { s.HistoryCompleteness = ProtectionHistoryCompleteness("bogus") },
			wantErr: "is invalid",
		},
		{
			name:    "invalid permissions",
			modify:  func(s *ProtectionProviderState) { s.Permissions = operationaltrust.EvidencePermissions("bogus") },
			wantErr: "are invalid",
		},
		{
			name:    "unsorted evidence ids",
			modify:  func(s *ProtectionProviderState) { s.EvidenceIDs = []string{"b", "a"} },
			wantErr: "must be sorted and unique",
		},
		{
			name:    "duplicate evidence ids",
			modify:  func(s *ProtectionProviderState) { s.EvidenceIDs = []string{"a", "a"} },
			wantErr: "must be sorted and unique",
		},
		{
			name:    "blank evidence id",
			modify:  func(s *ProtectionProviderState) { s.EvidenceIDs = []string{"  "} },
			wantErr: "must be sorted and unique",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := validProviderState()
			tc.modify(&state)
			err := state.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestProtectionPosture_Clone(t *testing.T) {
	at := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	posture := ProtectionPosture{
		SubjectResourceID:     "resource-1",
		State:                 ProtectionStateProtected,
		LastAttemptAt:         &at,
		LastSuccessfulPointAt: &at,
		LastVerifiedAt:        &at,
		Freshness:             ProtectionFreshnessCurrent,
		Verification:          ProtectionVerificationVerified,
		Coverage:              ProtectionCoverageComplete,
		ProviderStates: []ProtectionProviderState{
			{
				Provider:            ProviderProxmoxPVE,
				Source:              "source-1",
				Scope:               "scope-1",
				JobState:            OutcomeSuccess,
				HistoryCompleteness: ProtectionHistoryComplete,
				Permissions:         operationaltrust.EvidencePermissionsSufficient,
				EvidenceIDs:         []string{"evidence-1"},
			},
		},
		RepositoryResourceIDs: []string{"repo-1"},
		EvidenceIDs:           []string{"evidence-1"},
		Explanation:           "explanation-1",
		EvaluatedAt:           at,
	}

	t.Run("nested provider state slice is deep-copied", func(t *testing.T) {
		clone := posture.Clone()
		clone.ProviderStates[0].EvidenceIDs[0] = "evidence-mutated"
		clone.ProviderStates[0].Source = "source-mutated"
		if posture.ProviderStates[0].EvidenceIDs[0] != "evidence-1" {
			t.Fatalf("nested evidence slice mutation propagated to original: %q", posture.ProviderStates[0].EvidenceIDs[0])
		}
		if posture.ProviderStates[0].Source != "source-1" {
			t.Fatalf("nested field mutation propagated to original: %q", posture.ProviderStates[0].Source)
		}
	})

	t.Run("provider slice append is isolated", func(t *testing.T) {
		clone := posture.Clone()
		clone.ProviderStates = append(clone.ProviderStates, ProtectionProviderState{
			Provider: ProviderKubernetes,
		})
		if len(posture.ProviderStates) != 1 {
			t.Fatalf("clone append propagated to original: len(posture.ProviderStates) = %d, want 1", len(posture.ProviderStates))
		}
	})

	t.Run("time pointers are deep-copied", func(t *testing.T) {
		clone := posture.Clone()
		if clone.LastAttemptAt == posture.LastAttemptAt {
			t.Fatal("Clone aliased LastAttemptAt pointer instead of copying")
		}
		*clone.LastAttemptAt = at.Add(time.Hour)
		if !posture.LastAttemptAt.Equal(at) {
			t.Fatalf("clone pointer mutation propagated to original: posture.LastAttemptAt = %v, want %v", posture.LastAttemptAt, at)
		}
	})

	t.Run("string slices are deep-copied", func(t *testing.T) {
		clone := posture.Clone()
		clone.RepositoryResourceIDs[0] = "repo-mutated"
		clone.EvidenceIDs[0] = "evidence-mutated"
		if posture.RepositoryResourceIDs[0] != "repo-1" {
			t.Fatalf("RepositoryResourceIDs mutation propagated to original: %q", posture.RepositoryResourceIDs[0])
		}
		if posture.EvidenceIDs[0] != "evidence-1" {
			t.Fatalf("EvidenceIDs mutation propagated to original: %q", posture.EvidenceIDs[0])
		}
	})
}

func TestProtectionPosture_Validate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modify  func(p *ProtectionPosture)
		wantErr string
	}{
		{
			name:    "passes when fully populated",
			modify:  func(p *ProtectionPosture) {},
			wantErr: "",
		},
		{
			name:    "missing subject resource id",
			modify:  func(p *ProtectionPosture) { p.SubjectResourceID = "" },
			wantErr: "protection posture subject resource id is required",
		},
		{
			name:    "invalid state",
			modify:  func(p *ProtectionPosture) { p.State = ProtectionState("bogus") },
			wantErr: "protection posture state",
		},
		{
			name:    "invalid freshness",
			modify:  func(p *ProtectionPosture) { p.Freshness = ProtectionFreshness("bogus") },
			wantErr: "protection posture freshness",
		},
		{
			name:    "invalid verification",
			modify:  func(p *ProtectionPosture) { p.Verification = ProtectionVerification("bogus") },
			wantErr: "protection posture verification",
		},
		{
			name:    "invalid coverage",
			modify:  func(p *ProtectionPosture) { p.Coverage = ProtectionCoverage("bogus") },
			wantErr: "protection posture coverage",
		},
		{
			name:    "zero evaluated at",
			modify:  func(p *ProtectionPosture) { p.EvaluatedAt = time.Time{} },
			wantErr: "protection posture evaluation time is required",
		},
		{
			name:    "missing explanation",
			modify:  func(p *ProtectionPosture) { p.Explanation = "" },
			wantErr: "protection posture explanation is required",
		},
		{
			name:    "unsorted repository resource ids",
			modify:  func(p *ProtectionPosture) { p.RepositoryResourceIDs = []string{"z", "a"} },
			wantErr: "protection repository resource ids must be sorted and unique",
		},
		{
			name:    "unsorted evidence ids",
			modify:  func(p *ProtectionPosture) { p.EvidenceIDs = []string{"z", "a"} },
			wantErr: "protection evidence ids must be sorted and unique",
		},
		{
			name: "provider state validation error is wrapped with index",
			modify: func(p *ProtectionPosture) {
				p.ProviderStates = []ProtectionProviderState{
					{
						Provider:            "",
						Source:              "source-1",
						Scope:               "scope-1",
						JobState:            OutcomeSuccess,
						HistoryCompleteness: ProtectionHistoryComplete,
						Permissions:         operationaltrust.EvidencePermissionsSufficient,
						EvidenceIDs:         []string{"evidence-1"},
					},
				}
			},
			wantErr: "protection provider state 0:",
		},
		{
			name: "provider states must be sorted and unique",
			modify: func(p *ProtectionPosture) {
				p.ProviderStates = []ProtectionProviderState{
					{
						Provider:            ProviderProxmoxPVE,
						Source:              "source-1",
						Scope:               "scope-1",
						JobState:            OutcomeSuccess,
						HistoryCompleteness: ProtectionHistoryComplete,
						Permissions:         operationaltrust.EvidencePermissionsSufficient,
						EvidenceIDs:         []string{"evidence-1"},
					},
					{
						Provider:            ProviderProxmoxPBS,
						Source:              "source-1",
						Scope:               "scope-1",
						JobState:            OutcomeSuccess,
						HistoryCompleteness: ProtectionHistoryComplete,
						Permissions:         operationaltrust.EvidencePermissionsSufficient,
						EvidenceIDs:         []string{"evidence-2"},
					},
				}
			},
			wantErr: "protection provider states must be sorted and unique",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posture := validPosture()
			tc.modify(&posture)
			err := posture.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}
