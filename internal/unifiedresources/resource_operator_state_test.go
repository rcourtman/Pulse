package unifiedresources

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResourceOperatorState_IsEmpty(t *testing.T) {
	if !(ResourceOperatorState{}).IsEmpty() {
		t.Error("zero value must report IsEmpty=true; the default no-state posture has no operator intent")
	}
	cases := []struct {
		name  string
		state ResourceOperatorState
	}{
		{"intentionally offline", ResourceOperatorState{IntentionallyOffline: true}},
		{"never auto remediate", ResourceOperatorState{NeverAutoRemediate: true}},
		{"criticality", ResourceOperatorState{Criticality: CriticalityHigh}},
		{"note", ResourceOperatorState{Note: "intentionally archived"}},
		{"maintenance reason only", ResourceOperatorState{MaintenanceReason: "scheduled"}},
		{"maintenance window", ResourceOperatorState{
			MaintenanceStartAt: timePointer(time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)),
			MaintenanceEndAt:   timePointer(time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC)),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.state.IsEmpty() {
				t.Errorf("state %+v must not report IsEmpty=true; it carries operator intent", tc.state)
			}
		})
	}
}

func TestResourceOperatorState_IsInMaintenanceAt(t *testing.T) {
	start := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	state := ResourceOperatorState{
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
	}

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before window", time.Date(2026, 5, 9, 11, 59, 59, 0, time.UTC), false},
		{"exactly start is in", start, true},
		{"midway is in", time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC), true},
		{"exactly end is out (half-open interval)", end, false},
		{"after end is out", time.Date(2026, 5, 9, 14, 0, 1, 0, time.UTC), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := state.IsInMaintenanceAt(tc.now); got != tc.want {
				t.Errorf("IsInMaintenanceAt(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}

	t.Run("no window configured", func(t *testing.T) {
		empty := ResourceOperatorState{}
		if empty.IsInMaintenanceAt(start) {
			t.Error("no-window state must always report IsInMaintenanceAt=false")
		}
	})

	t.Run("only start set is treated as no window", func(t *testing.T) {
		half := ResourceOperatorState{MaintenanceStartAt: &start}
		if half.IsInMaintenanceAt(time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)) {
			t.Error("partial maintenance window must not gate suppression")
		}
	})

	t.Run("end before start is treated as no window", func(t *testing.T) {
		bad := ResourceOperatorState{MaintenanceStartAt: &end, MaintenanceEndAt: &start}
		if bad.IsInMaintenanceAt(time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)) {
			t.Error("inverted maintenance window must not gate suppression — validation should also reject this on Set")
		}
	})
}

func TestValidateResourceOperatorState(t *testing.T) {
	t.Run("rejects empty canonical id", func(t *testing.T) {
		err := ValidateResourceOperatorState(ResourceOperatorState{CanonicalID: "   "})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid, got %v", err)
		}
	})

	t.Run("rejects unknown criticality", func(t *testing.T) {
		err := ValidateResourceOperatorState(ResourceOperatorState{
			CanonicalID: "vm:101",
			Criticality: "very-high", // not one of the canonical levels
		})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid for unknown criticality, got %v", err)
		}
		if !strings.Contains(err.Error(), "criticality") {
			t.Errorf("error should name the criticality field; got %v", err)
		}
	})

	t.Run("rejects half-set maintenance window", func(t *testing.T) {
		start := time.Now()
		err := ValidateResourceOperatorState(ResourceOperatorState{
			CanonicalID:        "vm:101",
			MaintenanceStartAt: &start,
		})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid for half-set window, got %v", err)
		}
	})

	t.Run("rejects end equal to start (zero-length window has no semantics)", func(t *testing.T) {
		t0 := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
		err := ValidateResourceOperatorState(ResourceOperatorState{
			CanonicalID:        "vm:101",
			MaintenanceStartAt: &t0,
			MaintenanceEndAt:   &t0,
		})
		if !errors.Is(err, ErrResourceOperatorStateInvalid) {
			t.Fatalf("expected ErrResourceOperatorStateInvalid for zero-length window, got %v", err)
		}
	})

	t.Run("accepts canonical empty (default) criticality", func(t *testing.T) {
		err := ValidateResourceOperatorState(ResourceOperatorState{
			CanonicalID: "vm:101",
		})
		if err != nil {
			t.Errorf("default state must validate clean; got %v", err)
		}
	})

	t.Run("accepts canonical levels", func(t *testing.T) {
		for _, level := range []ResourceCriticality{CriticalityHigh, CriticalityMedium, CriticalityLow} {
			err := ValidateResourceOperatorState(ResourceOperatorState{
				CanonicalID: "vm:101",
				Criticality: level,
			})
			if err != nil {
				t.Errorf("criticality %q must validate; got %v", level, err)
			}
		}
	})
}

func TestNormalizeResourceOperatorState_TrimsAndLowersCriticality(t *testing.T) {
	got := NormalizeResourceOperatorState(ResourceOperatorState{
		CanonicalID:       "  vm:101  ",
		MaintenanceReason: "  weekend window  ",
		Note:              "\tarchived\n",
		SetBy:             "  agent:ops  ",
		Criticality:       "  HIGH  ",
	})
	if got.CanonicalID != "vm:101" {
		t.Errorf("canonical id should be trimmed; got %q", got.CanonicalID)
	}
	if got.MaintenanceReason != "weekend window" {
		t.Errorf("maintenance reason should be trimmed; got %q", got.MaintenanceReason)
	}
	if got.Note != "archived" {
		t.Errorf("note should be trimmed; got %q", got.Note)
	}
	if got.SetBy != "agent:ops" {
		t.Errorf("set_by should be trimmed; got %q", got.SetBy)
	}
	if got.Criticality != CriticalityHigh {
		t.Errorf("criticality should be trimmed and lowered; got %q", got.Criticality)
	}
}

func TestMemoryStore_ResourceOperatorState_SetGetClearRoundTrip(t *testing.T) {
	store := NewMemoryStore()

	// Initial GET on a fresh store returns no entry.
	state, found, err := store.GetResourceOperatorState("vm:101")
	if err != nil {
		t.Fatalf("unexpected error on initial get: %v", err)
	}
	if found {
		t.Errorf("expected no entry on fresh store; got %+v", state)
	}

	start := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	setAt := time.Date(2026, 5, 9, 11, 59, 0, 0, time.UTC)
	original := ResourceOperatorState{
		CanonicalID:          "vm:101",
		IntentionallyOffline: true,
		NeverAutoRemediate:   true,
		MaintenanceStartAt:   &start,
		MaintenanceEndAt:     &end,
		MaintenanceReason:    "Q3 storage upgrade",
		Criticality:          CriticalityHigh,
		Note:                 "do not auto-fix; restoration in progress",
		SetAt:                setAt,
		SetBy:                "operator:richard",
	}
	if err := store.SetResourceOperatorState(original); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	state, found, err = store.GetResourceOperatorState("vm:101")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !found {
		t.Fatal("expected entry after Set")
	}
	if !state.IntentionallyOffline {
		t.Error("intentionally_offline must round-trip")
	}
	if !state.NeverAutoRemediate {
		t.Error("never_auto_remediate must round-trip")
	}
	if state.MaintenanceStartAt == nil || !state.MaintenanceStartAt.Equal(start) {
		t.Errorf("maintenance start must round-trip; got %v", state.MaintenanceStartAt)
	}
	if state.MaintenanceEndAt == nil || !state.MaintenanceEndAt.Equal(end) {
		t.Errorf("maintenance end must round-trip; got %v", state.MaintenanceEndAt)
	}
	if state.Criticality != CriticalityHigh {
		t.Errorf("criticality must round-trip; got %q", state.Criticality)
	}
	if state.Note != "do not auto-fix; restoration in progress" {
		t.Errorf("note must round-trip; got %q", state.Note)
	}

	// Clear is idempotent and removes the entry.
	if err := store.ClearResourceOperatorState("vm:101"); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, found, _ := store.GetResourceOperatorState("vm:101"); found {
		t.Error("entry must be gone after Clear")
	}
	if err := store.ClearResourceOperatorState("vm:101"); err != nil {
		t.Errorf("Clear must be idempotent; got %v", err)
	}
}

func TestMemoryStore_SetResourceOperatorState_RejectsInvalid(t *testing.T) {
	store := NewMemoryStore()
	err := store.SetResourceOperatorState(ResourceOperatorState{Criticality: "bogus"})
	if !errors.Is(err, ErrResourceOperatorStateInvalid) {
		t.Fatalf("invalid state must be rejected with ErrResourceOperatorStateInvalid; got %v", err)
	}
}

func timePointer(t time.Time) *time.Time {
	return &t
}
