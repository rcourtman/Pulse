package models

import (
	"reflect"
	"testing"
	"time"
)

// bcIntPtr returns a pointer to v; used to build metric values that exercise
// the reflect.Pointer arm of cloneDynamicValue. It is prefixed to avoid any
// future collision with other test helpers in this package.
func bcIntPtr(v int) *int { return &v }

// TestBranchCovCloneMetricValues covers every branch of cloneMetricValues:
//   - the len==0 fast path (exercised by BOTH a nil map and an allocated-but-empty
//     map, which are distinct boundaries that both collapse to nil),
//   - the iteration body that delegates each value to cloneDynamicValue.
//
// Independence is verified for nested reference values (maps, slices, pointers)
// so the test exercises the deep-copy behavior rather than being a vacuous echo
// of the input.
func TestBranchCovCloneMetricValues(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		if got := cloneMetricValues(nil); got != nil {
			t.Fatalf("cloneMetricValues(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty non-nil map also returns nil", func(t *testing.T) {
		// Distinct boundary from nil: an allocated-but-empty map still has
		// len==0 and must collapse to nil via the same fast path.
		if got := cloneMetricValues(map[string]interface{}{}); got != nil {
			t.Fatalf("cloneMetricValues(empty) = %#v, want nil", got)
		}
	})

	t.Run("non-empty map is deep-copied and isolated", func(t *testing.T) {
		src := map[string]interface{}{
			"scalar":   42,
			"nested":   map[string]int{"x": 1},
			"slice":    []int{1, 2, 3},
			"ptr":      bcIntPtr(7),
			"nilValue": nil,
		}
		got := cloneMetricValues(src)
		if got == nil {
			t.Fatal("non-empty input must return a non-nil map")
		}
		if len(got) != len(src) {
			t.Fatalf("len = %d, want %d", len(got), len(src))
		}

		// Scalar values are preserved through the default arm of cloneDynamicValue.
		if got["scalar"] != 42 {
			t.Errorf("scalar = %v, want 42", got["scalar"])
		}
		// A nil-valued entry survives cloning (cloneDynamicValue(nil) -> nil,
		// but the key must remain present in the destination map).
		if _, ok := got["nilValue"]; !ok {
			t.Error("nil-valued key was dropped during clone")
		}

		// Nested map independence: mutating the cloned inner map must not leak.
		nestedGot := got["nested"].(map[string]int)
		nestedGot["x"] = 999
		if src["nested"].(map[string]int)["x"] != 1 {
			t.Error("mutating cloned nested map affected source")
		}

		// Slice independence.
		sliceGot := got["slice"].([]int)
		sliceGot[0] = 999
		if src["slice"].([]int)[0] != 1 {
			t.Error("mutating cloned nested slice affected source")
		}

		// Pointer independence.
		*got["ptr"].(*int) = 999
		if *src["ptr"].(*int) != 7 {
			t.Error("mutating cloned nested pointer affected source")
		}

		// Top-level map independence: adding a key to the clone must not leak.
		got["new"] = "added"
		if _, ok := src["new"]; ok {
			t.Error("mutating cloned top-level map affected source")
		}
	})
}

// TestBranchCovCloneDynamicValue covers every arm and sub-branch of
// cloneDynamicValue:
//   - the top-level `value == nil` guard (untyped nil interface),
//   - reflect.Map arm: typed-nil map (returns original) AND a populated map
//     (deep-copied with independence),
//   - reflect.Slice arm: typed-nil slice (returns original) AND a populated
//     slice (deep-copied with independence, including a nil element),
//   - reflect.Pointer arm: typed-nil pointer (returns original) AND a populated
//     pointer (deep-copied with independence),
//   - the default arm for primitive/scalar values.
//
// The typed-nil cases are distinct from the untyped-nil guard because a typed
// nil wrapped in an interface is itself non-nil; only the inner IsNil() check
// catches it.
func TestBranchCovCloneDynamicValue(t *testing.T) {
	// Branch: untyped nil interface -> nil.
	if got := cloneDynamicValue(nil); got != nil {
		t.Fatalf("cloneDynamicValue(nil) = %#v, want nil", got)
	}

	// --- Default arm: scalars are returned as-is. ---
	for _, tc := range []struct {
		name  string
		value interface{}
	}{
		{"int", 42},
		{"string", "hello"},
		{"bool", true},
		{"float64", 3.5},
	} {
		t.Run("default/"+tc.name, func(t *testing.T) {
			got := cloneDynamicValue(tc.value)
			if got != tc.value {
				t.Fatalf("cloneDynamicValue(%v) = %#v, want %#v", tc.value, got, tc.value)
			}
		})
	}

	// --- Map arm. ---
	t.Run("map/typed-nil map returns original", func(t *testing.T) {
		var nilMap map[string]int // typed nil, non-nil interface
		got := cloneDynamicValue(nilMap)
		rv := reflect.ValueOf(got)
		if rv.Kind() != reflect.Map {
			t.Fatalf("kind = %v, want Map", rv.Kind())
		}
		if !rv.IsNil() {
			t.Fatal("typed-nil map must be returned as a nil map, not deep-copied")
		}
	})

	t.Run("map/populated map is deep-copied and isolated", func(t *testing.T) {
		src := map[string]interface{}{
			"k": map[string]int{"x": 1},
		}
		got := cloneDynamicValue(src)
		gotMap, ok := got.(map[string]interface{})
		if !ok {
			t.Fatalf("result type = %T, want map[string]interface{}", got)
		}
		inner := gotMap["k"].(map[string]int)
		inner["x"] = 999
		if src["k"].(map[string]int)["x"] != 1 {
			t.Error("mutating cloned nested map value affected source map")
		}
		// Mutating the top-level cloned map must not affect the source.
		gotMap["extra"] = 1
		if _, leak := src["extra"]; leak {
			t.Error("mutating cloned top-level map affected source map")
		}
	})

	t.Run("map/element-type conversion via valueForType", func(t *testing.T) {
		// A map whose element type is a concrete numeric type exercises the
		// valueForType call site inside the Map arm: the cloned dynamic value
		// (an int) is assignable to the element type (int) and is set directly.
		src := map[string]int{"a": 1, "b": 2}
		got := cloneDynamicValue(src)
		gotMap, ok := got.(map[string]int)
		if !ok {
			t.Fatalf("result type = %T, want map[string]int", got)
		}
		if len(gotMap) != 2 || gotMap["a"] != 1 || gotMap["b"] != 2 {
			t.Errorf("cloned map = %#v, want {a:1,b:2}", gotMap)
		}
		gotMap["a"] = 99
		if src["a"] != 1 {
			t.Error("mutating cloned int-typed map affected source")
		}
	})

	// --- Slice arm. ---
	t.Run("slice/typed-nil slice returns original", func(t *testing.T) {
		var nilSlice []int // typed nil, non-nil interface
		got := cloneDynamicValue(nilSlice)
		rv := reflect.ValueOf(got)
		if rv.Kind() != reflect.Slice {
			t.Fatalf("kind = %v, want Slice", rv.Kind())
		}
		if !rv.IsNil() {
			t.Fatal("typed-nil slice must be returned as a nil slice, not deep-copied")
		}
	})

	t.Run("slice/populated slice is deep-copied and isolated", func(t *testing.T) {
		src := []interface{}{
			map[string]int{"x": 1},
			[]int{2, 3},
			nil, // nil element exercises cloneDynamicValue(nil) within recursion
			4,
		}
		got := cloneDynamicValue(src)
		gotSlice, ok := got.([]interface{})
		if !ok {
			t.Fatalf("result type = %T, want []interface{}", got)
		}
		if len(gotSlice) != len(src) {
			t.Fatalf("len = %d, want %d", len(gotSlice), len(src))
		}

		// Element 0: nested map independence.
		gotSlice[0].(map[string]int)["x"] = 999
		if src[0].(map[string]int)["x"] != 1 {
			t.Error("mutating cloned nested map inside slice affected source")
		}
		// Element 1: nested slice independence.
		gotSlice[1].([]int)[0] = 999
		if src[1].([]int)[0] != 2 {
			t.Error("mutating cloned nested slice inside slice affected source")
		}
		// Element 2: nil element preserved.
		if gotSlice[2] != nil {
			t.Errorf("nil element = %#v, want nil", gotSlice[2])
		}
		// Element 3: scalar element preserved.
		if gotSlice[3] != 4 {
			t.Errorf("scalar element = %v, want 4", gotSlice[3])
		}
	})

	t.Run("slice/concrete element type", func(t *testing.T) {
		// A []int exercises valueForType's AssignableTo path inside the Slice
		// arm (element type int, cloned dynamic value is int).
		src := []int{10, 20, 30}
		got := cloneDynamicValue(src)
		gotSlice, ok := got.([]int)
		if !ok {
			t.Fatalf("result type = %T, want []int", got)
		}
		gotSlice[0] = 999
		if src[0] != 10 {
			t.Error("mutating cloned []int affected source")
		}
	})

	// --- Pointer arm. ---
	t.Run("pointer/typed-nil pointer returns original", func(t *testing.T) {
		var nilPtr *int // typed nil, non-nil interface
		got := cloneDynamicValue(nilPtr)
		rv := reflect.ValueOf(got)
		if rv.Kind() != reflect.Ptr {
			t.Fatalf("kind = %v, want Ptr", rv.Kind())
		}
		if !rv.IsNil() {
			t.Fatal("typed-nil pointer must be returned as a nil pointer, not deep-copied")
		}
	})

	t.Run("pointer/populated pointer is deep-copied and isolated", func(t *testing.T) {
		v := 5
		src := &v
		got := cloneDynamicValue(src)
		gotPtr, ok := got.(*int)
		if !ok {
			t.Fatalf("result type = %T, want *int", got)
		}
		if gotPtr == src {
			t.Fatal("cloned pointer aliases the source pointer; expected a distinct allocation")
		}
		if *gotPtr != 5 {
			t.Errorf("cloned pointed-to value = %d, want 5", *gotPtr)
		}
		*gotPtr = 999
		if *src != 5 {
			t.Error("mutating cloned pointer's target affected source pointer")
		}
	})
}

// TestBranchCovValueForType covers every branch of valueForType in isolation:
//   - value == nil -> reflect.Zero(targetType) (for both a value type and a
//     pointer type, whose zero is a nil pointer),
//   - AssignableTo -> the value returned unchanged,
//   - ConvertibleTo-but-not-AssignableTo -> result.Convert(targetType),
//   - neither assignable nor convertible -> reflect.Zero(targetType) fallback.
func TestBranchCovValueForType(t *testing.T) {
	intType := reflect.TypeOf(int(0))
	float64Type := reflect.TypeOf(float64(0))
	ptrIntType := reflect.TypeOf((*int)(nil))

	for _, tc := range []struct {
		name       string
		targetType reflect.Type
		value      interface{}
		validate   func(t *testing.T, rv reflect.Value)
	}{
		{
			name:       "nil value yields zero int",
			targetType: intType,
			value:      nil,
			validate: func(t *testing.T, rv reflect.Value) {
				if rv.Kind() != reflect.Int || rv.Int() != 0 {
					t.Fatalf("got %v (kind %v), want zero int", rv, rv.Kind())
				}
			},
		},
		{
			name:       "nil value yields nil pointer of target type",
			targetType: ptrIntType,
			value:      nil,
			validate: func(t *testing.T, rv reflect.Value) {
				if rv.Kind() != reflect.Ptr || !rv.IsNil() {
					t.Fatalf("got %v (kind %v), want nil *int", rv, rv.Kind())
				}
			},
		},
		{
			name:       "assignable: int value to int target",
			targetType: intType,
			value:      7,
			validate: func(t *testing.T, rv reflect.Value) {
				if rv.Kind() != reflect.Int || rv.Int() != 7 {
					t.Fatalf("got %v, want int 7", rv)
				}
			},
		},
		{
			name:       "convertible: int value to float64 target via Convert",
			targetType: float64Type,
			value:      3, // int not assignable to float64, but convertible
			validate: func(t *testing.T, rv reflect.Value) {
				if rv.Kind() != reflect.Float64 || rv.Float() != 3.0 {
					t.Fatalf("got %v, want float64 3.0", rv)
				}
			},
		},
		{
			name:       "neither assignable nor convertible: string to int yields zero",
			targetType: intType,
			value:      "hello", // string not assignable nor convertible to int
			validate: func(t *testing.T, rv reflect.Value) {
				if rv.Kind() != reflect.Int || rv.Int() != 0 {
					t.Fatalf("got %v, want zero int fallback", rv)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.validate(t, valueForType(tc.targetType, tc.value))
		})
	}
}

// TestBranchCovCloneMetric covers cloneMetric end to end:
//   - the nil/empty Values path, which must surface NormalizeCollections'
//     guarantee that a cloned Metric never has a nil Values map,
//   - deep-copy independence of nested reference values within Values,
//   - preservation of the scalar (non-collection) fields Timestamp/Type/ID.
func TestBranchCovCloneMetric(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("nil Values normalized to non-nil empty map", func(t *testing.T) {
		src := Metric{Timestamp: ts, Type: "cpu", ID: "m1", Values: nil}
		got := cloneMetric(src)
		if got.Values == nil {
			t.Fatal("cloneMetric must normalize nil Values to a non-nil map")
		}
		if len(got.Values) != 0 {
			t.Fatalf("len(Values) = %d, want 0", len(got.Values))
		}
	})

	t.Run("empty Values map normalized to non-nil empty map", func(t *testing.T) {
		// cloneMetricValues collapses the empty map to nil; NormalizeCollections
		// must then re-materialize an empty (non-nil) map.
		src := Metric{Timestamp: ts, Type: "cpu", ID: "m1", Values: map[string]interface{}{}}
		got := cloneMetric(src)
		if got.Values == nil {
			t.Fatal("cloneMetric must normalize empty Values to a non-nil map")
		}
		if len(got.Values) != 0 {
			t.Fatalf("len(Values) = %d, want 0", len(got.Values))
		}
	})

	t.Run("scalar fields preserved", func(t *testing.T) {
		src := Metric{Timestamp: ts, Type: "mem", ID: "m2", Values: map[string]interface{}{"u": 1.0}}
		got := cloneMetric(src)
		if !got.Timestamp.Equal(ts) {
			t.Errorf("Timestamp = %v, want %v", got.Timestamp, ts)
		}
		if got.Type != "mem" {
			t.Errorf("Type = %q, want %q", got.Type, "mem")
		}
		if got.ID != "m2" {
			t.Errorf("ID = %q, want %q", got.ID, "m2")
		}
	})

	t.Run("nested values are deep-copied and isolated", func(t *testing.T) {
		src := Metric{
			Timestamp: ts,
			Type:      "disk",
			ID:        "m3",
			Values: map[string]interface{}{
				"nested": map[string]int{"x": 1},
				"slice":  []int{1, 2},
				"ptr":    bcIntPtr(7),
			},
		}
		got := cloneMetric(src)

		// Nested map independence.
		got.Values["nested"].(map[string]int)["x"] = 999
		if src.Values["nested"].(map[string]int)["x"] != 1 {
			t.Error("mutating cloned Metric nested map affected source Metric")
		}

		// Nested slice independence.
		got.Values["slice"].([]int)[0] = 999
		if src.Values["slice"].([]int)[0] != 1 {
			t.Error("mutating cloned Metric nested slice affected source Metric")
		}

		// Nested pointer independence.
		*got.Values["ptr"].(*int) = 999
		if *src.Values["ptr"].(*int) != 7 {
			t.Error("mutating cloned Metric nested pointer affected source Metric")
		}

		// Top-level Values map independence.
		got.Values["added"] = true
		if _, leak := src.Values["added"]; leak {
			t.Error("mutating cloned Metric Values map affected source Metric")
		}
	})
}
