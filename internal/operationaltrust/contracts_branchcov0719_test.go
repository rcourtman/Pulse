package operationaltrust

import (
	"strings"
	"testing"
	"time"
)

func TestEvidencePayloadRefValidate(t *testing.T) {
	valid := EvidencePayloadRef{Kind: "metric", ID: "payload-1"}

	tests := []struct {
		name      string
		ref       EvidencePayloadRef
		wantError string
	}{
		{
			name:      "valid",
			ref:       valid,
			wantError: "",
		},
		{
			name:      "missing kind",
			ref:       EvidencePayloadRef{ID: "payload-1"},
			wantError: "kind is required",
		},
		{
			name:      "whitespace kind",
			ref:       EvidencePayloadRef{Kind: "   ", ID: "payload-1"},
			wantError: "kind is required",
		},
		{
			name:      "missing id",
			ref:       EvidencePayloadRef{Kind: "metric"},
			wantError: "id is required",
		},
		{
			name:      "whitespace id",
			ref:       EvidencePayloadRef{Kind: "metric", ID: "\t "},
			wantError: "id is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ref.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantError)
			}
		})
	}
}

func TestAcknowledgementValidate(t *testing.T) {
	at := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	valid := Acknowledgement{At: at, By: "operator-1", Note: "acknowledged"}

	tests := []struct {
		name      string
		ack       Acknowledgement
		wantError string
	}{
		{
			name:      "valid",
			ack:       valid,
			wantError: "",
		},
		{
			name:      "zero time",
			ack:       Acknowledgement{By: "operator-1"},
			wantError: "time is required",
		},
		{
			name:      "missing actor",
			ack:       Acknowledgement{At: at, By: ""},
			wantError: "actor is required",
		},
		{
			name:      "whitespace actor",
			ack:       Acknowledgement{At: at, By: "   "},
			wantError: "actor is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ack.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantError)
			}
		})
	}
}

func TestSuppressionValidate(t *testing.T) {
	at := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	later := at.Add(time.Hour)
	earlier := at.Add(-time.Minute)
	equal := at

	tests := []struct {
		name      string
		sup       Suppression
		wantError string
	}{
		{
			name:      "valid without expiry",
			sup:       Suppression{At: at, By: "operator-1", Reason: "maintenance"},
			wantError: "",
		},
		{
			name:      "valid with expiry",
			sup:       Suppression{At: at, By: "operator-1", Reason: "maintenance", ExpiresAt: &later},
			wantError: "",
		},
		{
			name:      "zero time",
			sup:       Suppression{By: "operator-1", Reason: "maintenance"},
			wantError: "time is required",
		},
		{
			name:      "missing actor",
			sup:       Suppression{At: at, Reason: "maintenance"},
			wantError: "actor is required",
		},
		{
			name:      "whitespace reason",
			sup:       Suppression{At: at, By: "operator-1", Reason: "  "},
			wantError: "reason is required",
		},
		{
			name:      "expiry before at",
			sup:       Suppression{At: at, By: "operator-1", Reason: "maintenance", ExpiresAt: &earlier},
			wantError: "expiry must follow",
		},
		{
			name:      "expiry equal at",
			sup:       Suppression{At: at, By: "operator-1", Reason: "maintenance", ExpiresAt: &equal},
			wantError: "expiry must follow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sup.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantError)
			}
		})
	}
}

func TestLifecycleTransitionClone(t *testing.T) {
	t.Run("evidence slice is independent", func(t *testing.T) {
		transition := LifecycleTransition{
			ID:                  "transition-1",
			OperationalRecordID: "record-1",
			From:                OperationalObserving,
			To:                  OperationalOpen,
			EvidenceIDs:         []string{"evidence-a", "evidence-b"},
		}
		clone := transition.Clone()
		clone.ID = "transition-2"
		clone.EvidenceIDs[0] = "evidence-mutated"
		clone.EvidenceIDs = append(clone.EvidenceIDs, "evidence-new")

		if transition.ID != "transition-1" {
			t.Fatalf("clone mutated source ID: got %q", transition.ID)
		}
		if transition.EvidenceIDs[0] != "evidence-a" {
			t.Fatalf("clone mutated source EvidenceIDs[0]: got %q", transition.EvidenceIDs[0])
		}
		if len(transition.EvidenceIDs) != 2 {
			t.Fatalf("clone mutated source EvidenceIDs length: got %d, want 2", len(transition.EvidenceIDs))
		}
		if clone.EvidenceIDs[0] != "evidence-mutated" {
			t.Fatalf("clone did not receive mutation: got %q", clone.EvidenceIDs[0])
		}
	})

	t.Run("nil evidence slice stays nil-safe", func(t *testing.T) {
		transition := LifecycleTransition{
			ID:                  "transition-1",
			OperationalRecordID: "record-1",
			From:                OperationalObserving,
			To:                  OperationalOpen,
		}
		clone := transition.Clone()
		if clone.ID != "transition-1" {
			t.Fatalf("clone lost scalar ID: got %q", clone.ID)
		}
		if clone.From != OperationalObserving || clone.To != OperationalOpen {
			t.Fatalf("clone lost state scalars: from=%q to=%q", clone.From, clone.To)
		}
		if clone.EvidenceIDs != nil {
			t.Fatalf("clone EvidenceIDs = %v, want nil", clone.EvidenceIDs)
		}
	})
}

func TestLifecycleTransitionValidate(t *testing.T) {
	at := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	base := LifecycleTransition{
		ID:                  "transition-1",
		OperationalRecordID: "record-1",
		From:                OperationalObserving,
		To:                  OperationalOpen,
		At:                  at,
		Cause:               TransitionDetectorDecision,
		CauseKey:            "cpu-high",
		EvidenceIDs:         []string{"evidence-1"},
	}

	tests := []struct {
		name      string
		mutate    func(LifecycleTransition) LifecycleTransition
		wantError string
	}{
		{
			name:      "valid",
			mutate:    func(t LifecycleTransition) LifecycleTransition { return t },
			wantError: "",
		},
		{
			name: "missing id",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.ID = ""
				return t
			},
			wantError: "transition id is required",
		},
		{
			name: "whitespace id treated as missing",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.ID = "   "
				return t
			},
			wantError: "transition id is required",
		},
		{
			name: "delegates missing operational record id",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.OperationalRecordID = ""
				return t
			},
			wantError: "operational record id is required",
		},
		{
			name: "delegates invalid from state",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.From = OperationalState("bogus")
				return t
			},
			wantError: "from state",
		},
		{
			name: "delegates invalid to state",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.To = OperationalState("bogus")
				return t
			},
			wantError: "to state",
		},
		{
			name: "delegates no-op transition",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.To = OperationalObserving
				return t
			},
			wantError: "must change state",
		},
		{
			name: "delegates missing transition time",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.At = time.Time{}
				return t
			},
			wantError: "transition time is required",
		},
		{
			name: "delegates missing cause key",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.CauseKey = ""
				return t
			},
			wantError: "cause key is required",
		},
		{
			name: "delegates detector decision requires evidence",
			mutate: func(t LifecycleTransition) LifecycleTransition {
				t.EvidenceIDs = nil
				return t
			},
			wantError: "detector decision requires evidence",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transition := tc.mutate(base)
			err := transition.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantError)
			}
		})
	}
}

func TestNotificationLinkClone(t *testing.T) {
	t.Run("time pointers are independent", func(t *testing.T) {
		attemptedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
		completedAt := attemptedAt.Add(time.Second)
		link := NotificationLink{
			NotificationID:      "notification-1",
			OperationalRecordID: "record-1",
			TransitionID:        "transition-1",
			LifecycleState:      OperationalOpen,
			CauseKey:            "cpu-high",
			DestinationID:       "email-primary",
			DeliveryState:       NotificationQueued,
			AttemptedAt:         &attemptedAt,
			CompletedAt:         &completedAt,
		}
		originalAttempted := *link.AttemptedAt
		originalCompleted := *link.CompletedAt

		clone := link.Clone()
		*clone.AttemptedAt = attemptedAt.Add(time.Hour)
		*clone.CompletedAt = completedAt.Add(time.Hour)
		clone.NotificationID = "notification-2"

		if link.NotificationID != "notification-1" {
			t.Fatalf("clone mutated source NotificationID: got %q", link.NotificationID)
		}
		if !link.AttemptedAt.Equal(originalAttempted) {
			t.Fatalf("clone mutated source AttemptedAt: got %v, want %v", link.AttemptedAt, originalAttempted)
		}
		if !link.CompletedAt.Equal(originalCompleted) {
			t.Fatalf("clone mutated source CompletedAt: got %v, want %v", link.CompletedAt, originalCompleted)
		}
		if !clone.AttemptedAt.Equal(attemptedAt.Add(time.Hour)) {
			t.Fatalf("clone did not receive AttemptedAt mutation: got %v", clone.AttemptedAt)
		}
	})

	t.Run("nil time pointers stay nil", func(t *testing.T) {
		link := NotificationLink{
			NotificationID:      "notification-1",
			OperationalRecordID: "record-1",
			TransitionID:        "transition-1",
			LifecycleState:      OperationalOpen,
			CauseKey:            "cpu-high",
			DestinationID:       "email-primary",
			DeliveryState:       NotificationQueued,
		}
		clone := link.Clone()
		if clone.NotificationID != "notification-1" {
			t.Fatalf("clone lost scalar NotificationID: got %q", clone.NotificationID)
		}
		if clone.AttemptedAt != nil {
			t.Fatalf("clone AttemptedAt = %v, want nil", clone.AttemptedAt)
		}
		if clone.CompletedAt != nil {
			t.Fatalf("clone CompletedAt = %v, want nil", clone.CompletedAt)
		}
	})
}
