package api

import (
	"testing"
)

func TestFirstValueForKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		m        map[string]interface{}
		keys     []string
		wantVal  interface{}
		wantBool bool
	}{
		// Basic functionality
		{
			name:     "single key found",
			m:        map[string]interface{}{"foo": "bar"},
			keys:     []string{"foo"},
			wantVal:  "bar",
			wantBool: true,
		},
		{
			name:     "single key not found",
			m:        map[string]interface{}{"foo": "bar"},
			keys:     []string{"baz"},
			wantVal:  nil,
			wantBool: false,
		},
		{
			name:     "first of multiple keys found",
			m:        map[string]interface{}{"camelCase": "value1", "snake_case": "value2"},
			keys:     []string{"camelCase", "snake_case"},
			wantVal:  "value1",
			wantBool: true,
		},
		{
			name:     "second key found when first missing",
			m:        map[string]interface{}{"snake_case": "value2"},
			keys:     []string{"camelCase", "snake_case"},
			wantVal:  "value2",
			wantBool: true,
		},
		{
			name:     "third key found when first two missing",
			m:        map[string]interface{}{"third": "value3"},
			keys:     []string{"first", "second", "third"},
			wantVal:  "value3",
			wantBool: true,
		},
		// Edge cases
		{
			name:     "nil map",
			m:        nil,
			keys:     []string{"foo"},
			wantVal:  nil,
			wantBool: false,
		},
		{
			name:     "empty map",
			m:        map[string]interface{}{},
			keys:     []string{"foo"},
			wantVal:  nil,
			wantBool: false,
		},
		{
			name:     "empty keys slice",
			m:        map[string]interface{}{"foo": "bar"},
			keys:     []string{},
			wantVal:  nil,
			wantBool: false,
		},
		{
			name:     "nil keys slice",
			m:        map[string]interface{}{"foo": "bar"},
			keys:     nil,
			wantVal:  nil,
			wantBool: false,
		},
		{
			name:     "empty string key",
			m:        map[string]interface{}{"": "empty-key-value"},
			keys:     []string{""},
			wantVal:  "empty-key-value",
			wantBool: true,
		},
		// Value types
		{
			name:     "nil value is found",
			m:        map[string]interface{}{"foo": nil},
			keys:     []string{"foo"},
			wantVal:  nil,
			wantBool: true,
		},
		{
			name:     "integer value",
			m:        map[string]interface{}{"count": 42},
			keys:     []string{"count"},
			wantVal:  42,
			wantBool: true,
		},
		{
			name:     "float value",
			m:        map[string]interface{}{"ratio": 3.14},
			keys:     []string{"ratio"},
			wantVal:  3.14,
			wantBool: true,
		},
		{
			name:     "bool value",
			m:        map[string]interface{}{"enabled": true},
			keys:     []string{"enabled"},
			wantVal:  true,
			wantBool: true,
		},
		{
			name:     "nested map value",
			m:        map[string]interface{}{"nested": map[string]interface{}{"inner": "data"}},
			keys:     []string{"nested"},
			wantVal:  map[string]interface{}{"inner": "data"},
			wantBool: true,
		},
		{
			name:     "slice value",
			m:        map[string]interface{}{"items": []string{"a", "b", "c"}},
			keys:     []string{"items"},
			wantVal:  []string{"a", "b", "c"},
			wantBool: true,
		},
		// Priority order matters
		{
			name:     "priority prefers first key even if second also exists",
			m:        map[string]interface{}{"a": "first", "b": "second"},
			keys:     []string{"a", "b"},
			wantVal:  "first",
			wantBool: true,
		},
		{
			name:     "priority skips nil value keys - nil value is still returned",
			m:        map[string]interface{}{"a": nil, "b": "second"},
			keys:     []string{"a", "b"},
			wantVal:  nil,
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotBool := firstValueForKeys(tt.m, tt.keys...)
			if gotBool != tt.wantBool {
				t.Errorf("firstValueForKeys() bool = %v, want %v", gotBool, tt.wantBool)
			}
			// For comparing interface{} values
			if !equalInterface(gotVal, tt.wantVal) {
				t.Errorf("firstValueForKeys() val = %v (%T), want %v (%T)", gotVal, gotVal, tt.wantVal, tt.wantVal)
			}
		})
	}
}

func TestHasAnyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]interface{}
		keys []string
		want bool
	}{
		// Basic functionality
		{
			name: "single key exists",
			m:    map[string]interface{}{"foo": "bar"},
			keys: []string{"foo"},
			want: true,
		},
		{
			name: "single key does not exist",
			m:    map[string]interface{}{"foo": "bar"},
			keys: []string{"baz"},
			want: false,
		},
		{
			name: "first of multiple keys exists",
			m:    map[string]interface{}{"first": "value"},
			keys: []string{"first", "second"},
			want: true,
		},
		{
			name: "second of multiple keys exists",
			m:    map[string]interface{}{"second": "value"},
			keys: []string{"first", "second"},
			want: true,
		},
		{
			name: "last of multiple keys exists",
			m:    map[string]interface{}{"third": "value"},
			keys: []string{"first", "second", "third"},
			want: true,
		},
		{
			name: "none of multiple keys exist",
			m:    map[string]interface{}{"other": "value"},
			keys: []string{"first", "second", "third"},
			want: false,
		},
		// Edge cases
		{
			name: "nil map",
			m:    nil,
			keys: []string{"foo"},
			want: false,
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			keys: []string{"foo"},
			want: false,
		},
		{
			name: "empty keys slice",
			m:    map[string]interface{}{"foo": "bar"},
			keys: []string{},
			want: false,
		},
		{
			name: "nil keys slice",
			m:    map[string]interface{}{"foo": "bar"},
			keys: nil,
			want: false,
		},
		{
			name: "empty string key exists",
			m:    map[string]interface{}{"": "empty"},
			keys: []string{""},
			want: true,
		},
		// Key exists with nil value
		{
			name: "key with nil value counts as existing",
			m:    map[string]interface{}{"foo": nil},
			keys: []string{"foo"},
			want: true,
		},
		// Real-world usage: camelCase vs snake_case
		{
			name: "discoveryConfig camelCase exists",
			m:    map[string]interface{}{"discoveryConfig": map[string]interface{}{}},
			keys: []string{"discoveryConfig", "discovery_config"},
			want: true,
		},
		{
			name: "discovery_config snake_case exists",
			m:    map[string]interface{}{"discovery_config": map[string]interface{}{}},
			keys: []string{"discoveryConfig", "discovery_config"},
			want: true,
		},
		{
			name: "neither naming convention exists",
			m:    map[string]interface{}{"otherConfig": map[string]interface{}{}},
			keys: []string{"discoveryConfig", "discovery_config"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAnyKey(tt.m, tt.keys...); got != tt.want {
				t.Errorf("hasAnyKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoveryConfigMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      map[string]interface{}
		wantMap  map[string]interface{}
		wantBool bool
	}{
		// CamelCase variant
		{
			name: "discoveryConfig with valid map",
			raw: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{
					"enabled": true,
					"subnet":  "192.168.1.0/24",
				},
			},
			wantMap:  map[string]interface{}{"enabled": true, "subnet": "192.168.1.0/24"},
			wantBool: true,
		},
		{
			name: "discoveryConfig with empty map",
			raw: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{},
			},
			wantMap:  map[string]interface{}{},
			wantBool: true,
		},
		// Snake_case variant
		{
			name: "discovery_config with valid map",
			raw: map[string]interface{}{
				"discovery_config": map[string]interface{}{
					"enabled": false,
					"timeout": 30,
				},
			},
			wantMap:  map[string]interface{}{"enabled": false, "timeout": 30},
			wantBool: true,
		},
		{
			name: "discovery_config with empty map",
			raw: map[string]interface{}{
				"discovery_config": map[string]interface{}{},
			},
			wantMap:  map[string]interface{}{},
			wantBool: true,
		},
		// Priority: camelCase over snake_case
		{
			name: "camelCase takes priority over snake_case",
			raw: map[string]interface{}{
				"discoveryConfig":  map[string]interface{}{"from": "camel"},
				"discovery_config": map[string]interface{}{"from": "snake"},
			},
			wantMap:  map[string]interface{}{"from": "camel"},
			wantBool: true,
		},
		// Not found
		{
			name:     "neither key exists",
			raw:      map[string]interface{}{"otherKey": "value"},
			wantMap:  nil,
			wantBool: false,
		},
		{
			name:     "empty raw map",
			raw:      map[string]interface{}{},
			wantMap:  nil,
			wantBool: false,
		},
		{
			name:     "nil raw map",
			raw:      nil,
			wantMap:  nil,
			wantBool: false,
		},
		// Type assertion failures - key exists but not a map
		{
			name: "discoveryConfig is string not map",
			raw: map[string]interface{}{
				"discoveryConfig": "not-a-map",
			},
			wantMap:  nil,
			wantBool: true, // key found, but type assertion failed
		},
		{
			name: "discoveryConfig is int not map",
			raw: map[string]interface{}{
				"discoveryConfig": 42,
			},
			wantMap:  nil,
			wantBool: true,
		},
		{
			name: "discoveryConfig is slice not map",
			raw: map[string]interface{}{
				"discoveryConfig": []string{"a", "b"},
			},
			wantMap:  nil,
			wantBool: true,
		},
		{
			name: "discoveryConfig is nil",
			raw: map[string]interface{}{
				"discoveryConfig": nil,
			},
			wantMap:  nil,
			wantBool: true,
		},
		{
			name: "discovery_config is string not map",
			raw: map[string]interface{}{
				"discovery_config": "also-not-a-map",
			},
			wantMap:  nil,
			wantBool: true,
		},
		{
			name: "discovery_config is bool not map",
			raw: map[string]interface{}{
				"discovery_config": true,
			},
			wantMap:  nil,
			wantBool: true,
		},
		// Nested map structure
		{
			name: "deeply nested config values",
			raw: map[string]interface{}{
				"discoveryConfig": map[string]interface{}{
					"nested": map[string]interface{}{
						"deep": "value",
					},
					"array": []int{1, 2, 3},
				},
			},
			wantMap: map[string]interface{}{
				"nested": map[string]interface{}{"deep": "value"},
				"array":  []int{1, 2, 3},
			},
			wantBool: true,
		},
		// Edge case: wrong type map (not map[string]interface{})
		{
			name: "discoveryConfig is map[string]string not map[string]interface{}",
			raw: map[string]interface{}{
				"discoveryConfig": map[string]string{"key": "value"},
			},
			wantMap:  nil,
			wantBool: true, // key found, type assertion to map[string]interface{} fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMap, gotBool := discoveryConfigMap(tt.raw)
			if gotBool != tt.wantBool {
				t.Errorf("discoveryConfigMap() bool = %v, want %v", gotBool, tt.wantBool)
			}
			if !equalMapInterface(gotMap, tt.wantMap) {
				t.Errorf("discoveryConfigMap() map = %v, want %v", gotMap, tt.wantMap)
			}
		})
	}
}

// equalInterface compares two interface{} values for equality
func equalInterface(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle map[string]interface{} specially
	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		return equalMapInterface(aMap, bMap)
	}

	// Handle []string specially
	aSlice, aIsSlice := a.([]string)
	bSlice, bIsSlice := b.([]string)
	if aIsSlice && bIsSlice {
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i := range aSlice {
			if aSlice[i] != bSlice[i] {
				return false
			}
		}
		return true
	}

	// Handle []int specially
	aIntSlice, aIsIntSlice := a.([]int)
	bIntSlice, bIsIntSlice := b.([]int)
	if aIsIntSlice && bIsIntSlice {
		if len(aIntSlice) != len(bIntSlice) {
			return false
		}
		for i := range aIntSlice {
			if aIntSlice[i] != bIntSlice[i] {
				return false
			}
		}
		return true
	}

	// Default comparison (may panic for uncomparable types, but those should be handled above)
	return a == b
}

// equalMapInterface compares two map[string]interface{} values
func equalMapInterface(a, b map[string]interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !equalInterface(v, bv) {
			return false
		}
	}
	return true
}
