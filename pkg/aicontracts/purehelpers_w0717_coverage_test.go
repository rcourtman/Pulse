package aicontracts

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// alertPayloadStub is a minimal AlertPayload implementation used only to
// exercise IsNilAlertPayload. Receivers are value receivers on purpose so that
// both alertPayloadStub (value, reflect Kind==Struct) and *alertPayloadStub
// (pointer, reflect Kind==Ptr) satisfy the interface, letting one type reach
// the nil-interface, typed-nil-pointer, populated-pointer, and value-kind arms.
// IsNilAlertPayload never invokes these methods (it inspects interface nilness
// via reflect only), so the bodies exist solely to satisfy the interface.
type alertPayloadStub struct {
	id           string
	typ          string
	resourceID   string
	resourceName string
	node         string
	instance     string
	message      string
	value        float64
	threshold    float64
	metadata     map[string]interface{}
}

func (a alertPayloadStub) GetID() string                       { return a.id }
func (a alertPayloadStub) GetType() string                     { return a.typ }
func (a alertPayloadStub) GetResourceID() string               { return a.resourceID }
func (a alertPayloadStub) GetResourceName() string             { return a.resourceName }
func (a alertPayloadStub) GetNode() string                     { return a.node }
func (a alertPayloadStub) GetInstance() string                 { return a.instance }
func (a alertPayloadStub) GetMessage() string                  { return a.message }
func (a alertPayloadStub) GetValue() float64                   { return a.value }
func (a alertPayloadStub) GetThreshold() float64               { return a.threshold }
func (a alertPayloadStub) GetMetadata() map[string]interface{} { return a.metadata }

// TestCloneActionReference covers every arm of CloneActionReference: the nil
// input arm (returns nil), the zero-value reference path, the populated happy
// path with a Preflight (asserting every scalar field is copied and every
// deep-copied slice is content-equal yet independently backed), the Preflight
// nil branch, and bidirectional independence of the three re-allocated slices.
func TestCloneActionReference(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		if got := CloneActionReference(nil); got != nil {
			t.Fatalf("CloneActionReference(nil) = %#v, want nil", got)
		}
	})

	t.Run("zero value reference returns a distinct equal clone", func(t *testing.T) {
		ref := &ActionReference{}
		clone := CloneActionReference(ref)
		if clone == nil {
			t.Fatal("CloneActionReference(zero ref) = nil, want non-nil")
		}
		if clone == ref {
			t.Fatal("clone aliases the input pointer; want a distinct allocation")
		}
		if clone.Plan.PredictedBlastRadius != nil {
			t.Fatalf("clone.Plan.PredictedBlastRadius = %#v, want nil for empty source", clone.Plan.PredictedBlastRadius)
		}
		if clone.Plan.Preflight != nil {
			t.Fatalf("clone.Plan.Preflight = %#v, want nil when source preflight is nil", clone.Plan.Preflight)
		}
		if !reflect.DeepEqual(*clone, *ref) {
			t.Fatalf("zero-value clone diverges from source: clone=%#v ref=%#v", *clone, *ref)
		}
	})

	t.Run("preflight nil branch leaves clone preflight nil", func(t *testing.T) {
		ref := &ActionReference{
			ActionID: "act-no-preflight",
			Plan: ActionPlanInfo{
				ActionID:             "act-no-preflight",
				PredictedBlastRadius: []string{"zone-a"},
			},
		}
		clone := CloneActionReference(ref)
		if clone == nil {
			t.Fatal("clone = nil, want non-nil")
		}
		if clone.Plan.Preflight != nil {
			t.Fatalf("clone.Plan.Preflight = %#v, want nil when source preflight is nil", clone.Plan.Preflight)
		}
		if !reflect.DeepEqual(clone.Plan.PredictedBlastRadius, ref.Plan.PredictedBlastRadius) {
			t.Fatalf("PredictedBlastRadius content mismatch: clone=%#v ref=%#v", clone.Plan.PredictedBlastRadius, ref.Plan.PredictedBlastRadius)
		}
	})

	t.Run("populated reference copies all fields and deep copies slices", func(t *testing.T) {
		plannedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
		expiresAt := plannedAt.Add(24 * time.Hour)
		generatedAt := plannedAt.Add(-1 * time.Minute)
		ref := &ActionReference{
			ActionID:       "act_123",
			ProposalID:     "prop_456",
			ResourceID:     "res_789",
			CapabilityName: "restart-service",
			State:          "approved",
			Plan: ActionPlanInfo{
				ActionID:             "act_plan_123",
				RequestID:            "req_001",
				Allowed:              true,
				RequiresApproval:     true,
				ApprovalPolicy:       "operator-approve",
				PredictedBlastRadius: []string{"svc-a", "svc-b"},
				RollbackAvailable:    true,
				Message:              "rolling restart",
				PlannedAt:            plannedAt,
				ExpiresAt:            expiresAt,
				ResourceVersion:      "v42",
				PolicyVersion:        "pv3",
				PolicyDecision:       json.RawMessage(`{"allow":true}`),
				PlanHash:             "hash-abc",
				Preflight: &ActionPreflightInfo{
					Target:            "nginx",
					CurrentState:      "running",
					IntendedChange:    "restart",
					DryRunAvailable:   true,
					DryRunSummary:     "no-op in dry run",
					SafetyChecks:      []string{"check-1", "check-2"},
					VerificationSteps: []string{"step-1", "step-2", "step-3"},
					GeneratedAt:       generatedAt,
				},
			},
		}

		clone := CloneActionReference(ref)
		if clone == nil {
			t.Fatal("CloneActionReference(populated ref) = nil, want non-nil")
		}
		if clone == ref {
			t.Fatal("clone aliases the input pointer; want a distinct allocation")
		}
		if clone.Plan.Preflight == nil {
			t.Fatal("clone.Plan.Preflight = nil, want a deep copy")
		}
		if clone.Plan.Preflight == ref.Plan.Preflight {
			t.Fatal("clone.Plan.Preflight aliases the source preflight pointer; want a distinct allocation")
		}

		// All fields copied exactly: one rigorous content-equality check before
		// any in-place mutation.
		if !reflect.DeepEqual(*clone, *ref) {
			t.Fatalf("clone content diverges from source:\nclone=%#v\nref  =%#v", *clone, *ref)
		}

		// Independence direction A: mutating the clone's re-allocated slices in
		// place must not leak back into the source.
		clone.Plan.PredictedBlastRadius[0] = "MUTATED-BY-CLONE-BR"
		clone.Plan.Preflight.SafetyChecks[0] = "MUTATED-BY-CLONE-SC"
		clone.Plan.Preflight.VerificationSteps[0] = "MUTATED-BY-CLONE-VS"
		if ref.Plan.PredictedBlastRadius[0] != "svc-a" {
			t.Fatalf("mutating clone PredictedBlastRadius leaked to source: source[0]=%q", ref.Plan.PredictedBlastRadius[0])
		}
		if ref.Plan.Preflight.SafetyChecks[0] != "check-1" {
			t.Fatalf("mutating clone Preflight.SafetyChecks leaked to source: source[0]=%q", ref.Plan.Preflight.SafetyChecks[0])
		}
		if ref.Plan.Preflight.VerificationSteps[0] != "step-1" {
			t.Fatalf("mutating clone Preflight.VerificationSteps leaked to source: source[0]=%q", ref.Plan.Preflight.VerificationSteps[0])
		}

		// Independence direction B: mutating the source's slices in place must
		// not leak into a fresh clone taken before the mutation.
		clone2 := CloneActionReference(ref)
		ref.Plan.PredictedBlastRadius[0] = "MUTATED-BY-SOURCE-BR"
		ref.Plan.Preflight.SafetyChecks[0] = "MUTATED-BY-SOURCE-SC"
		ref.Plan.Preflight.VerificationSteps[0] = "MUTATED-BY-SOURCE-VS"
		if clone2.Plan.PredictedBlastRadius[0] != "svc-a" {
			t.Fatalf("mutating source PredictedBlastRadius leaked to clone2: clone2[0]=%q", clone2.Plan.PredictedBlastRadius[0])
		}
		if clone2.Plan.Preflight.SafetyChecks[0] != "check-1" {
			t.Fatalf("mutating source Preflight.SafetyChecks leaked to clone2: clone2[0]=%q", clone2.Plan.Preflight.SafetyChecks[0])
		}
		if clone2.Plan.Preflight.VerificationSteps[0] != "step-1" {
			t.Fatalf("mutating source Preflight.VerificationSteps leaked to clone2: clone2[0]=%q", clone2.Plan.Preflight.VerificationSteps[0])
		}
	})
}

// TestIsNilAlertPayload covers all branches of IsNilAlertPayload: the bare nil
// interface (p == nil arm), the typed-nil pointer boxed in a non-nil interface
// (reflect Kind==Ptr && IsNil arm, which is the whole reason the helper exists),
// and the two false arms — a populated pointer (Ptr but not nil) and a value
// receiver (Kind != Ptr, short-circuiting the && to false).
func TestIsNilAlertPayload(t *testing.T) {
	populated := alertPayloadStub{
		id:           "alert-1",
		typ:          "cpu_high",
		resourceID:   "res-1",
		resourceName: "node-1",
		node:         "node-1",
		instance:     "inst-1",
		message:      "cpu saturated",
		value:        95.5,
		threshold:    90.0,
		metadata:     map[string]interface{}{"region": "us-east-1"},
	}
	var typedNilPtr *alertPayloadStub // deliberately nil pointer

	tests := []struct {
		name    string
		payload AlertPayload
		want    bool
	}{
		{name: "nil interface is nil", payload: AlertPayload(nil), want: true},
		{name: "typed nil pointer boxed in interface is nil", payload: typedNilPtr, want: true},
		{name: "populated pointer payload is not nil", payload: &populated, want: false},
		{name: "value receiver payload is not nil", payload: populated, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNilAlertPayload(tt.payload)
			if got != tt.want {
				t.Fatalf("IsNilAlertPayload(%v) = %v, want %v", tt.payload, got, tt.want)
			}
		})
	}
}

// TestDefaultEngineConfig pins every concrete default field value returned by
// DefaultEngineConfig, including the deliberately-unset DataDir, so silent drift
// in the engine's safety budgets is caught.
func TestDefaultEngineConfig(t *testing.T) {
	cfg := DefaultEngineConfig()

	if cfg.DataDir != "" {
		t.Fatalf("DataDir = %q, want empty (unset by default)", cfg.DataDir)
	}
	if cfg.MaxExecutions != 100 {
		t.Fatalf("MaxExecutions = %d, want 100", cfg.MaxExecutions)
	}
	if cfg.PlanExpiry != 24*time.Hour {
		t.Fatalf("PlanExpiry = %v, want 24h", cfg.PlanExpiry)
	}
	if cfg.ExecutionTimeout != 5*time.Minute {
		t.Fatalf("ExecutionTimeout = %v, want 5m", cfg.ExecutionTimeout)
	}
}
