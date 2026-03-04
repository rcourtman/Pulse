package api

import (
	"testing"
)

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
		// snake_case key is no longer accepted
		{
			name: "camelCase key is used even when snake_case also exists",
			raw: map[string]interface{}{
				"discoveryConfig":  map[string]interface{}{"from": "camel"},
				"discovery_config": map[string]interface{}{"from": "snake"},
			},
			wantMap:  map[string]interface{}{"from": "camel"},
			wantBool: true,
		},
		{
			name: "discovery_config with valid map is ignored",
			raw: map[string]interface{}{
				"discovery_config": map[string]interface{}{
					"enabled": false,
					"timeout": 30,
				},
			},
			wantMap:  nil,
			wantBool: false,
		},
		{
			name: "discovery_config with empty map is ignored",
			raw: map[string]interface{}{
				"discovery_config": map[string]interface{}{},
			},
			wantMap:  nil,
			wantBool: false,
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
			wantBool: false,
		},
		{
			name: "discovery_config is bool not map",
			raw: map[string]interface{}{
				"discovery_config": true,
			},
			wantMap:  nil,
			wantBool: false,
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
