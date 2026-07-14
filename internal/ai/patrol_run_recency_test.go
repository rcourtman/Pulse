package ai

import (
	"testing"
	"time"
)

// patrolrecencyRun builds a PatrolRunRecord with only the fields that
// patrolRecencyFromHistory inspects (CompletedAt and Type) populated, so table
// cases stay readable. The patrolrecency prefix avoids collisions with helpers
// defined in sibling test files in this package.
func patrolrecencyRun(runType string, completedAt time.Time) PatrolRunRecord {
	return PatrolRunRecord{
		ID:          "rec-" + runType,
		Type:        runType,
		CompletedAt: completedAt,
	}
}

// TestPatrolRecencyFromHistory exercises patrolRecencyFromHistory across every
// branch of its input space: nil/empty history, zero CompletedAt records (the
// skip branch), each arm of the isFullPatrolRun switch (via normalizePatrolRun
// Type: "", "full", "patrol", case/whitespace normalization, and the default
// fallthrough for scoped/verification/unknown types), and the latest-timestamp
// selection for both lastActivity (any run) and lastFullPatrol (full runs only).
//
// Note: patrolRecencyFromHistory uses isFullPatrolRun (not
// isSuccessfulFullPatrolRun), so a full patrol that finished with errors still
// counts toward lastFullPatrol. This is asserted below and called out in
// GLM_REPORT.md as a behavioral observation (it differs from
// shouldSkipInitialFullPatrol, which does gate on success).
func TestPatrolRecencyFromHistory(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	t3 := t1.Add(2 * time.Hour)
	zero := time.Time{}

	tests := []struct {
		name               string
		history            []PatrolRunRecord
		wantActivity       time.Time
		wantActivityZero   bool
		wantFullPatrol     time.Time
		wantFullPatrolZero bool
	}{
		{
			name:               "nil history returns zero times",
			history:            nil,
			wantActivityZero:   true,
			wantFullPatrolZero: true,
		},
		{
			name:               "empty history returns zero times",
			history:            []PatrolRunRecord{},
			wantActivityZero:   true,
			wantFullPatrolZero: true,
		},
		{
			name:               "all zero CompletedAt records are skipped",
			history:            []PatrolRunRecord{patrolrecencyRun("patrol", zero), patrolrecencyRun("scoped", zero)},
			wantActivityZero:   true,
			wantFullPatrolZero: true,
		},
		{
			name:           "single patrol-type full run populates both",
			history:        []PatrolRunRecord{patrolrecencyRun("patrol", t1)},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
		{
			name:           "empty Type falls through to full patrol arm",
			history:        []PatrolRunRecord{patrolrecencyRun("", t1)},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
		{
			name:           "full Type matches full patrol arm",
			history:        []PatrolRunRecord{patrolrecencyRun("full", t1)},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
		{
			name:           "case-insensitive and whitespace-trimmed Type normalizes to full patrol",
			history:        []PatrolRunRecord{patrolrecencyRun("  PaTrOl ", t1)},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
		{
			name:               "scoped run populates lastActivity only",
			history:            []PatrolRunRecord{patrolrecencyRun("scoped", t1)},
			wantActivity:       t1,
			wantFullPatrolZero: true,
		},
		{
			name:               "verification run populates lastActivity only",
			history:            []PatrolRunRecord{patrolrecencyRun("verification", t1)},
			wantActivity:       t1,
			wantFullPatrolZero: true,
		},
		{
			name:               "unknown run Type hits default arm and populates lastActivity only",
			history:            []PatrolRunRecord{patrolrecencyRun("custom-type", t1)},
			wantActivity:       t1,
			wantFullPatrolZero: true,
		},
		{
			name:           "mixed scoped older and full newer picks the full run for both timestamps",
			history:        []PatrolRunRecord{patrolrecencyRun("scoped", t1), patrolrecencyRun("patrol", t2)},
			wantActivity:   t2,
			wantFullPatrol: t2,
		},
		{
			name:           "mixed full older and scoped newer splits lastActivity and lastFullPatrol",
			history:        []PatrolRunRecord{patrolrecencyRun("patrol", t1), patrolrecencyRun("scoped", t2)},
			wantActivity:   t2,
			wantFullPatrol: t1,
		},
		{
			name:           "multiple full patrols selects the latest CompletedAt",
			history:        []PatrolRunRecord{patrolrecencyRun("patrol", t1), patrolrecencyRun("patrol", t3), patrolrecencyRun("patrol", t2)},
			wantActivity:   t3,
			wantFullPatrol: t3,
		},
		{
			name:           "unsorted mixed-type history selects correct latest for each timestamp",
			history:        []PatrolRunRecord{patrolrecencyRun("scoped", t3), patrolrecencyRun("patrol", t1), patrolrecencyRun("scoped", t2)},
			wantActivity:   t3,
			wantFullPatrol: t1,
		},
		{
			name:               "zero CompletedAt mixed with a valid scoped record leaves lastFullPatrol zero",
			history:            []PatrolRunRecord{patrolrecencyRun("patrol", zero), patrolrecencyRun("scoped", t1), patrolrecencyRun("patrol", zero)},
			wantActivity:       t1,
			wantFullPatrolZero: true,
		},
		{
			name:           "equal CompletedAt values resolve to that timestamp (After is strict, first-seen wins)",
			history:        []PatrolRunRecord{patrolrecencyRun("patrol", t1), patrolrecencyRun("patrol", t1)},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
		{
			name: "full patrol with errors still counts toward lastFullPatrol",
			history: []PatrolRunRecord{
				{
					ID:          "errored-full",
					Type:        "patrol",
					CompletedAt: t1,
					ErrorCount:  2,
					Status:      "error",
				},
			},
			wantActivity:   t1,
			wantFullPatrol: t1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotActivity, gotFullPatrol := patrolRecencyFromHistory(tc.history)

			if tc.wantActivityZero {
				if !gotActivity.IsZero() {
					t.Fatalf("lastActivity: expected zero time, got %v", gotActivity)
				}
			} else if !gotActivity.Equal(tc.wantActivity) {
				t.Fatalf("lastActivity: expected %v, got %v", tc.wantActivity, gotActivity)
			}

			if tc.wantFullPatrolZero {
				if !gotFullPatrol.IsZero() {
					t.Fatalf("lastFullPatrol: expected zero time, got %v", gotFullPatrol)
				}
			} else if !gotFullPatrol.Equal(tc.wantFullPatrol) {
				t.Fatalf("lastFullPatrol: expected %v, got %v", tc.wantFullPatrol, gotFullPatrol)
			}
		})
	}
}
