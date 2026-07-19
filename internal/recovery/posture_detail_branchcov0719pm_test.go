package recovery

import (
	"testing"
	"time"
)

func TestRecoveryDetailStringBranch0719pm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		point RecoveryPoint
		key   string
		want  string
	}{
		{
			name:  "nil details returns empty",
			point: RecoveryPoint{},
			key:   "instance",
			want:  "",
		},
		{
			name: "populated details but key absent returns empty",
			point: RecoveryPoint{
				Details: map[string]any{
					"instance": "pve-main",
				},
			},
			key:  "missing-key",
			want: "",
		},
		{
			name: "key present but value is an int returns empty",
			point: RecoveryPoint{
				Details: map[string]any{
					"vmid": 100,
				},
			},
			key:  "vmid",
			want: "",
		},
		{
			name: "key present but value is nil returns empty",
			point: RecoveryPoint{
				Details: map[string]any{
					"connectionId": nil,
				},
			},
			key:  "connectionId",
			want: "",
		},
		{
			name: "key present with bare string value returns value",
			point: RecoveryPoint{
				Details: map[string]any{
					"instance": "pve-main",
				},
			},
			key:  "instance",
			want: "pve-main",
		},
		{
			name: "key present with surrounding whitespace returns trimmed value",
			point: RecoveryPoint{
				Details: map[string]any{
					"k8sClusterId": "  prod-eks  ",
				},
			},
			key:  "k8sClusterId",
			want: "prod-eks",
		},
		{
			name: "key present with whitespace-only string returns empty",
			point: RecoveryPoint{
				Details: map[string]any{
					"instance": "   ",
				},
			},
			key:  "instance",
			want: "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := recoveryDetailString(test.point, test.key)
			if got != test.want {
				t.Fatalf("recoveryDetailString(...) = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRecoveryPointObservedAtBranch0719pm(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	// Construct a non-UTC time so we can assert the result is normalised to UTC.
	completedNonUTC := time.Date(
		2026, 7, 19, 8, 0, 0, 0, time.FixedZone("PVE", -2*3600),
	)
	zeroTime := time.Time{}

	tests := []struct {
		name  string
		point RecoveryPoint
		want  time.Time
	}{
		{
			name:  "nil completed and nil started returns zero",
			point: RecoveryPoint{},
			want:  time.Time{},
		},
		{
			name: "completed present returns completed in UTC",
			point: RecoveryPoint{
				CompletedAt: &completedAt,
			},
			want: completedAt.UTC(),
		},
		{
			name: "completed nil but started present returns started in UTC",
			point: RecoveryPoint{
				StartedAt: &startedAt,
			},
			want: startedAt.UTC(),
		},
		{
			name: "both completed and started present prefers completed",
			point: RecoveryPoint{
				CompletedAt: &completedAt,
				StartedAt:   &startedAt,
			},
			want: completedAt.UTC(),
		},
		{
			name: "non-nil completed pointer holding zero value falls through to started",
			point: RecoveryPoint{
				CompletedAt: &zeroTime,
				StartedAt:   &startedAt,
			},
			want: startedAt.UTC(),
		},
		{
			name: "non-nil started pointer holding zero value returns zero",
			point: RecoveryPoint{
				StartedAt: &zeroTime,
			},
			want: time.Time{},
		},
		{
			name: "non-nil completed and started both zero returns zero",
			point: RecoveryPoint{
				CompletedAt: &zeroTime,
				StartedAt:   &zeroTime,
			},
			want: time.Time{},
		},
		{
			name: "non-UTC completed is normalised to UTC equivalent",
			point: RecoveryPoint{
				CompletedAt: &completedNonUTC,
			},
			want: completedNonUTC.UTC(),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := recoveryPointObservedAt(test.point)
			if !got.Equal(test.want) {
				t.Fatalf(
					"recoveryPointObservedAt(...) = %v (loc %s), want %v (loc %s)",
					got, got.Location(), test.want, test.want.Location(),
				)
			}
		})
	}
}
