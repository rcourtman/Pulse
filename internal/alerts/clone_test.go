package alerts

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizePoweredOffSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    AlertLevel
		expected AlertLevel
	}{
		{
			name:     "critical lowercase",
			input:    AlertLevel("critical"),
			expected: AlertLevelCritical,
		},
		{
			name:     "critical uppercase",
			input:    AlertLevel("CRITICAL"),
			expected: AlertLevelCritical,
		},
		{
			name:     "critical mixed case",
			input:    AlertLevel("Critical"),
			expected: AlertLevelCritical,
		},
		{
			name:     "warning lowercase",
			input:    AlertLevel("warning"),
			expected: AlertLevelWarning,
		},
		{
			name:     "warning uppercase",
			input:    AlertLevel("WARNING"),
			expected: AlertLevelWarning,
		},
		{
			name:     "empty string defaults to warning",
			input:    AlertLevel(""),
			expected: AlertLevelWarning,
		},
		{
			name:     "unknown value defaults to warning",
			input:    AlertLevel("info"),
			expected: AlertLevelWarning,
		},
		{
			name:     "garbage value defaults to warning",
			input:    AlertLevel("xyz123"),
			expected: AlertLevelWarning,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizePoweredOffSeverity(tc.input)
			if result != tc.expected {
				t.Errorf("normalizePoweredOffSeverity(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCloneMetadata_Nil(t *testing.T) {
	result := cloneMetadata(nil)
	if result != nil {
		t.Errorf("cloneMetadata(nil) = %v, want nil", result)
	}
}

func TestCloneMetadata_Empty(t *testing.T) {
	src := map[string]interface{}{}
	result := cloneMetadata(src)
	if result == nil {
		t.Fatal("cloneMetadata({}) returned nil, want empty map")
	}
	if len(result) != 0 {
		t.Errorf("cloneMetadata({}) length = %d, want 0", len(result))
	}
}

func TestCloneMetadata_SimpleValues(t *testing.T) {
	src := map[string]interface{}{
		"string":  "hello",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"nil_val": nil,
	}

	result := cloneMetadata(src)

	if len(result) != len(src) {
		t.Errorf("cloneMetadata length = %d, want %d", len(result), len(src))
	}

	for k, v := range src {
		if result[k] != v {
			t.Errorf("cloneMetadata[%s] = %v, want %v", k, result[k], v)
		}
	}

	// Verify it's a copy, not the same reference
	src["new_key"] = "new_value"
	if _, exists := result["new_key"]; exists {
		t.Error("cloneMetadata should create independent copy")
	}
}

func TestCloneMetadata_NestedMap(t *testing.T) {
	src := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "value",
			"count": 10,
		},
	}

	result := cloneMetadata(src)

	// Verify structure
	outer, ok := result["outer"].(map[string]interface{})
	if !ok {
		t.Fatal("cloneMetadata nested map not preserved")
	}
	if outer["inner"] != "value" {
		t.Errorf("cloneMetadata nested value = %v, want 'value'", outer["inner"])
	}

	// Verify it's a deep copy - modify original nested map
	srcOuter := src["outer"].(map[string]interface{})
	srcOuter["new"] = "modified"

	// Result should not be affected
	if _, exists := outer["new"]; exists {
		t.Error("cloneMetadata should deep copy nested maps")
	}
}

func TestCloneMetadataValue_StringSlice(t *testing.T) {
	src := []string{"a", "b", "c"}
	result := cloneMetadataValue(src)

	resultSlice, ok := result.([]string)
	if !ok {
		t.Fatalf("cloneMetadataValue([]string) type = %T, want []string", result)
	}

	if !reflect.DeepEqual(resultSlice, src) {
		t.Errorf("cloneMetadataValue([]string) = %v, want %v", resultSlice, src)
	}

	// Verify independence
	src[0] = "modified"
	if resultSlice[0] == "modified" {
		t.Error("cloneMetadataValue should create independent copy of []string")
	}
}

func TestCloneMetadataValue_IntSlice(t *testing.T) {
	src := []int{1, 2, 3}
	result := cloneMetadataValue(src)

	resultSlice, ok := result.([]int)
	if !ok {
		t.Fatalf("cloneMetadataValue([]int) type = %T, want []int", result)
	}

	if !reflect.DeepEqual(resultSlice, src) {
		t.Errorf("cloneMetadataValue([]int) = %v, want %v", resultSlice, src)
	}

	// Verify independence
	src[0] = 999
	if resultSlice[0] == 999 {
		t.Error("cloneMetadataValue should create independent copy of []int")
	}
}

func TestCloneMetadataValue_FloatSlice(t *testing.T) {
	src := []float64{1.1, 2.2, 3.3}
	result := cloneMetadataValue(src)

	resultSlice, ok := result.([]float64)
	if !ok {
		t.Fatalf("cloneMetadataValue([]float64) type = %T, want []float64", result)
	}

	if !reflect.DeepEqual(resultSlice, src) {
		t.Errorf("cloneMetadataValue([]float64) = %v, want %v", resultSlice, src)
	}

	// Verify independence
	src[0] = 999.9
	if resultSlice[0] == 999.9 {
		t.Error("cloneMetadataValue should create independent copy of []float64")
	}
}

func TestCloneMetadataValue_InterfaceSlice(t *testing.T) {
	src := []interface{}{"a", 1, 3.14}
	result := cloneMetadataValue(src)

	resultSlice, ok := result.([]interface{})
	if !ok {
		t.Fatalf("cloneMetadataValue([]interface{}) type = %T, want []interface{}", result)
	}

	if len(resultSlice) != len(src) {
		t.Errorf("cloneMetadataValue([]interface{}) length = %d, want %d", len(resultSlice), len(src))
	}

	for i, v := range src {
		if resultSlice[i] != v {
			t.Errorf("cloneMetadataValue([]interface{})[%d] = %v, want %v", i, resultSlice[i], v)
		}
	}
}

func TestCloneMetadataValue_MapStringString(t *testing.T) {
	src := map[string]string{"key": "value", "foo": "bar"}
	result := cloneMetadataValue(src)

	// The function converts map[string]string to map[string]interface{}
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("cloneMetadataValue(map[string]string) type = %T, want map[string]interface{}", result)
	}

	if len(resultMap) != len(src) {
		t.Errorf("cloneMetadataValue length = %d, want %d", len(resultMap), len(src))
	}

	for k, v := range src {
		if resultMap[k] != v {
			t.Errorf("cloneMetadataValue[%s] = %v, want %v", k, resultMap[k], v)
		}
	}
}

func TestCloneMetadataValue_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float64", 3.14},
		{"bool", true},
		{"nil", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cloneMetadataValue(tc.input)
			if result != tc.input {
				t.Errorf("cloneMetadataValue(%v) = %v, want same value", tc.input, result)
			}
		})
	}
}

func TestAlert_Clone_Nil(t *testing.T) {
	var a *Alert
	result := a.Clone()
	if result != nil {
		t.Errorf("nil.Clone() = %v, want nil", result)
	}
}

func TestAlert_Clone_BasicFields(t *testing.T) {
	now := time.Now()
	original := &Alert{
		ID:           "test-alert-1",
		Type:         "cpu",
		Level:        AlertLevelCritical,
		ResourceID:   "vm-100",
		ResourceName: "test-vm",
		Node:         "node1",
		Instance:     "instance1",
		Message:      "CPU usage high",
		Value:        95.5,
		Threshold:    90.0,
		StartTime:    now,
		LastSeen:     now,
		Acknowledged: true,
		AckUser:      "admin",
	}

	clone := original.Clone()

	if clone == original {
		t.Error("Clone() should return a different pointer")
	}

	if clone.ID != original.ID {
		t.Errorf("Clone ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Type != original.Type {
		t.Errorf("Clone Type = %q, want %q", clone.Type, original.Type)
	}
	if clone.Level != original.Level {
		t.Errorf("Clone Level = %q, want %q", clone.Level, original.Level)
	}
	if clone.ResourceID != original.ResourceID {
		t.Errorf("Clone ResourceID = %q, want %q", clone.ResourceID, original.ResourceID)
	}
	if clone.ResourceName != original.ResourceName {
		t.Errorf("Clone ResourceName = %q, want %q", clone.ResourceName, original.ResourceName)
	}
	if clone.Node != original.Node {
		t.Errorf("Clone Node = %q, want %q", clone.Node, original.Node)
	}
	if clone.Instance != original.Instance {
		t.Errorf("Clone Instance = %q, want %q", clone.Instance, original.Instance)
	}
	if clone.Message != original.Message {
		t.Errorf("Clone Message = %q, want %q", clone.Message, original.Message)
	}
	if clone.Value != original.Value {
		t.Errorf("Clone Value = %f, want %f", clone.Value, original.Value)
	}
	if clone.Threshold != original.Threshold {
		t.Errorf("Clone Threshold = %f, want %f", clone.Threshold, original.Threshold)
	}
	if !clone.StartTime.Equal(original.StartTime) {
		t.Errorf("Clone StartTime = %v, want %v", clone.StartTime, original.StartTime)
	}
	if !clone.LastSeen.Equal(original.LastSeen) {
		t.Errorf("Clone LastSeen = %v, want %v", clone.LastSeen, original.LastSeen)
	}
	if clone.Acknowledged != original.Acknowledged {
		t.Errorf("Clone Acknowledged = %v, want %v", clone.Acknowledged, original.Acknowledged)
	}
	if clone.AckUser != original.AckUser {
		t.Errorf("Clone AckUser = %q, want %q", clone.AckUser, original.AckUser)
	}
}

func TestAlert_Clone_AckTime(t *testing.T) {
	now := time.Now()
	ackTime := now.Add(-1 * time.Hour)
	original := &Alert{
		ID:      "test-1",
		AckTime: &ackTime,
	}

	clone := original.Clone()

	if clone.AckTime == nil {
		t.Fatal("Clone AckTime should not be nil")
	}
	if clone.AckTime == original.AckTime {
		t.Error("Clone AckTime should be a different pointer")
	}
	if !clone.AckTime.Equal(*original.AckTime) {
		t.Errorf("Clone AckTime = %v, want %v", *clone.AckTime, *original.AckTime)
	}

	// Verify independence
	*original.AckTime = time.Now().Add(24 * time.Hour)
	if clone.AckTime.Equal(*original.AckTime) {
		t.Error("Clone AckTime should be independent of original")
	}
}

func TestAlert_Clone_LastNotified(t *testing.T) {
	now := time.Now()
	notified := now.Add(-30 * time.Minute)
	original := &Alert{
		ID:           "test-1",
		LastNotified: &notified,
	}

	clone := original.Clone()

	if clone.LastNotified == nil {
		t.Fatal("Clone LastNotified should not be nil")
	}
	if clone.LastNotified == original.LastNotified {
		t.Error("Clone LastNotified should be a different pointer")
	}
	if !clone.LastNotified.Equal(*original.LastNotified) {
		t.Errorf("Clone LastNotified = %v, want %v", *clone.LastNotified, *original.LastNotified)
	}
}

func TestAlert_Clone_EscalationTimes(t *testing.T) {
	now := time.Now()
	original := &Alert{
		ID:              "test-1",
		LastEscalation:  2,
		EscalationTimes: []time.Time{now.Add(-2 * time.Hour), now.Add(-1 * time.Hour)},
	}

	clone := original.Clone()

	if clone.LastEscalation != original.LastEscalation {
		t.Errorf("Clone LastEscalation = %d, want %d", clone.LastEscalation, original.LastEscalation)
	}
	if len(clone.EscalationTimes) != len(original.EscalationTimes) {
		t.Fatalf("Clone EscalationTimes length = %d, want %d", len(clone.EscalationTimes), len(original.EscalationTimes))
	}
	for i, et := range original.EscalationTimes {
		if !clone.EscalationTimes[i].Equal(et) {
			t.Errorf("Clone EscalationTimes[%d] = %v, want %v", i, clone.EscalationTimes[i], et)
		}
	}

	// Verify slice independence
	original.EscalationTimes[0] = time.Now()
	if clone.EscalationTimes[0].Equal(original.EscalationTimes[0]) {
		t.Error("Clone EscalationTimes should be independent of original")
	}
}

func TestAlert_Clone_Metadata(t *testing.T) {
	original := &Alert{
		ID: "test-1",
		Metadata: map[string]interface{}{
			"key":    "value",
			"nested": map[string]interface{}{"inner": "data"},
		},
	}

	clone := original.Clone()

	if clone.Metadata == nil {
		t.Fatal("Clone Metadata should not be nil")
	}
	if clone.Metadata["key"] != "value" {
		t.Errorf("Clone Metadata[key] = %v, want 'value'", clone.Metadata["key"])
	}

	// Verify deep copy
	original.Metadata["key"] = "modified"
	if clone.Metadata["key"] == "modified" {
		t.Error("Clone Metadata should be a deep copy")
	}

	// Verify nested map independence
	originalNested := original.Metadata["nested"].(map[string]interface{})
	originalNested["inner"] = "changed"

	cloneNested := clone.Metadata["nested"].(map[string]interface{})
	if cloneNested["inner"] == "changed" {
		t.Error("Clone Metadata nested map should be independent")
	}
}

func TestAlert_Clone_NilOptionalFields(t *testing.T) {
	original := &Alert{
		ID:              "test-1",
		AckTime:         nil,
		LastNotified:    nil,
		EscalationTimes: nil,
		Metadata:        nil,
	}

	clone := original.Clone()

	if clone.AckTime != nil {
		t.Error("Clone AckTime should be nil when original is nil")
	}
	if clone.LastNotified != nil {
		t.Error("Clone LastNotified should be nil when original is nil")
	}
	if clone.EscalationTimes != nil {
		t.Error("Clone EscalationTimes should be nil when original is nil")
	}
	if clone.Metadata != nil {
		t.Error("Clone Metadata should be nil when original is nil")
	}
}

func TestAlert_Clone_EmptyEscalationTimes(t *testing.T) {
	original := &Alert{
		ID:              "test-1",
		EscalationTimes: []time.Time{},
	}

	clone := original.Clone()

	// Empty slice should remain nil after clone (len == 0 check)
	if len(clone.EscalationTimes) != 0 {
		t.Errorf("Clone EscalationTimes length = %d, want 0", len(clone.EscalationTimes))
	}
}
