package licensing

import (
	"testing"
)

type mockSource struct {
	capabilities  []string
	limits        map[string]int64
	metersEnabled []string
	planVersion   string
	subState      SubscriptionState
	trialStarted  *int64
	trialEnds     *int64
}

func (m mockSource) Capabilities() []string               { return m.capabilities }
func (m mockSource) Limits() map[string]int64             { return m.limits }
func (m mockSource) MetersEnabled() []string              { return m.metersEnabled }
func (m mockSource) PlanVersion() string                  { return m.planVersion }
func (m mockSource) SubscriptionState() SubscriptionState { return m.subState }
func (m mockSource) TrialStartedAt() *int64               { return m.trialStarted }
func (m mockSource) TrialEndsAt() *int64                  { return m.trialEnds }
func (m mockSource) OverflowGrantedAt() *int64            { return nil }

func TestEvaluator_HasCapability(t *testing.T) {
	tests := []struct {
		name   string
		source mockSource
		key    string
		want   bool
	}{
		{
			name:   "nil_evaluator_returns_false",
			source: mockSource{},
			key:    "feature",
			want:   false,
		},
		{
			name:   "nil_source_returns_false",
			source: mockSource{},
			key:    "feature",
			want:   false,
		},
		{
			name:   "finds_capability",
			source: mockSource{capabilities: []string{"feature_a", "feature_b"}},
			key:    "feature_a",
			want:   true,
		},
		{
			name:   "missing_capability_returns_false",
			source: mockSource{capabilities: []string{"feature_a"}},
			key:    "feature_b",
			want:   false,
		},
		{
			name:   "empty_capabilities_returns_false",
			source: mockSource{capabilities: []string{}},
			key:    "feature_a",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.HasCapability(tt.key)
			if got != tt.want {
				t.Errorf("HasCapability(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvaluator_GetLimit(t *testing.T) {
	tests := []struct {
		name       string
		source     mockSource
		key        string
		wantValue  int64
		wantExists bool
	}{
		{
			name:       "nil_evaluator_returns_false",
			source:     mockSource{},
			key:        "max_agents",
			wantValue:  0,
			wantExists: false,
		},
		{
			name:       "finds_limit",
			source:     mockSource{limits: map[string]int64{"max_agents": 100}},
			key:        "max_agents",
			wantValue:  100,
			wantExists: true,
		},
		{
			name:       "missing_limit_returns_false",
			source:     mockSource{limits: map[string]int64{"max_agents": 100}},
			key:        "max_guests",
			wantValue:  0,
			wantExists: false,
		},
		{
			name:       "zero_limit_exists",
			source:     mockSource{limits: map[string]int64{"max_agents": 0}},
			key:        "max_agents",
			wantValue:  0,
			wantExists: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			gotVal, gotExists := e.GetLimit(tt.key)
			if gotVal != tt.wantValue {
				t.Errorf("GetLimit(%q).value = %v, want %v", tt.key, gotVal, tt.wantValue)
			}
			if gotExists != tt.wantExists {
				t.Errorf("GetLimit(%q).exists = %v, want %v", tt.key, gotExists, tt.wantExists)
			}
		})
	}
}

func TestEvaluator_CheckLimit(t *testing.T) {
	source := mockSource{limits: map[string]int64{"max_agents": 100}}
	e := NewEvaluator(source)

	tests := []struct {
		name     string
		key      string
		observed int64
		want     LimitCheckResult
	}{
		{
			name:     "well_under_limit",
			key:      "max_agents",
			observed: 50,
			want:     LimitAllowed,
		},
		{
			name:     "at_90_percent",
			key:      "max_agents",
			observed: 90,
			want:     LimitSoftBlock,
		},
		{
			name:     "at_limit",
			key:      "max_agents",
			observed: 100,
			want:     LimitHardBlock,
		},
		{
			name:     "over_limit",
			key:      "max_agents",
			observed: 150,
			want:     LimitHardBlock,
		},
		{
			name:     "undefined_limit_allows",
			key:      "max_guests",
			observed: 1000,
			want:     LimitAllowed,
		},
		{
			name:     "zero_limit_allows",
			key:      "max_agents",
			observed: 0,
			want:     LimitAllowed,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := e.CheckLimit(tt.key, tt.observed)
			if got != tt.want {
				t.Errorf("CheckLimit(%q, %d) = %v, want %v", tt.key, tt.observed, got, tt.want)
			}
		})
	}
}

func TestEvaluator_MeterEnabled(t *testing.T) {
	tests := []struct {
		name   string
		source mockSource
		key    string
		want   bool
	}{
		{
			name:   "nil_source_returns_false",
			source: mockSource{},
			key:    "meter_a",
			want:   false,
		},
		{
			name:   "finds_enabled_meter",
			source: mockSource{metersEnabled: []string{"meter_a", "meter_b"}},
			key:    "meter_a",
			want:   true,
		},
		{
			name:   "missing_meter_returns_false",
			source: mockSource{metersEnabled: []string{"meter_a"}},
			key:    "meter_b",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.MeterEnabled(tt.key)
			if got != tt.want {
				t.Errorf("MeterEnabled(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvaluator_PlanVersion(t *testing.T) {
	tests := []struct {
		name   string
		source mockSource
		want   string
	}{
		{
			name:   "nil_source_returns_empty",
			source: mockSource{},
			want:   "",
		},
		{
			name:   "returns_plan_version",
			source: mockSource{planVersion: "v2"},
			want:   "v2",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.PlanVersion()
			if got != tt.want {
				t.Errorf("PlanVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_SubscriptionState(t *testing.T) {
	tests := []struct {
		name   string
		source mockSource
		want   SubscriptionState
	}{
		{
			name:   "empty_source_returns_empty",
			source: mockSource{subState: ""},
			want:   "",
		},
		{
			name:   "returns_subscription_state",
			source: mockSource{subState: SubStateGrace},
			want:   SubStateGrace,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.SubscriptionState()
			if got != tt.want {
				t.Errorf("SubscriptionState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_NilSource_ReturnsDefaults(t *testing.T) {
	var e *Evaluator
	if got := e.HasCapability("feature"); got != false {
		t.Errorf("HasCapability() on nil = %v, want false", got)
	}
	if got, _ := e.GetLimit("max_agents"); got != 0 {
		t.Errorf("GetLimit() on nil = %v, want 0", got)
	}
	if got := e.CheckLimit("max_agents", 50); got != LimitAllowed {
		t.Errorf("CheckLimit() on nil = %v, want LimitAllowed", got)
	}
	if got := e.MeterEnabled("meter"); got != false {
		t.Errorf("MeterEnabled() on nil = %v, want false", got)
	}
	if got := e.PlanVersion(); got != "" {
		t.Errorf("PlanVersion() on nil = %v, want empty", got)
	}
	if got := e.SubscriptionState(); got != SubStateActive {
		t.Errorf("SubscriptionState() on nil = %v, want SubStateActive", got)
	}
	if got := e.TrialStartedAt(); got != nil {
		t.Errorf("TrialStartedAt() on nil = %v, want nil", got)
	}
	if got := e.TrialEndsAt(); got != nil {
		t.Errorf("TrialEndsAt() on nil = %v, want nil", got)
	}
}

func TestEvaluator_TrialStartedAt(t *testing.T) {
	ts := int64(1700000000)
	tests := []struct {
		name   string
		source mockSource
		want   *int64
	}{
		{
			name:   "nil_source_returns_nil",
			source: mockSource{},
			want:   nil,
		},
		{
			name:   "returns_trial_started",
			source: mockSource{trialStarted: &ts},
			want:   &ts,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.TrialStartedAt()
			if (got == nil) != (tt.want == nil) {
				t.Errorf("TrialStartedAt() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && *got != *tt.want {
				t.Errorf("TrialStartedAt() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestEvaluator_TrialEndsAt(t *testing.T) {
	ts := int64(1700000000)
	tests := []struct {
		name   string
		source mockSource
		want   *int64
	}{
		{
			name:   "nil_source_returns_nil",
			source: mockSource{},
			want:   nil,
		},
		{
			name:   "returns_trial_ends",
			source: mockSource{trialEnds: &ts},
			want:   &ts,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(tt.source)
			got := e.TrialEndsAt()
			if (got == nil) != (tt.want == nil) {
				t.Errorf("TrialEndsAt() = %v, want %v", got, tt.want)
				return
			}
			if got != nil && *got != *tt.want {
				t.Errorf("TrialEndsAt() = %v, want %v", *got, *tt.want)
			}
		})
	}
}
