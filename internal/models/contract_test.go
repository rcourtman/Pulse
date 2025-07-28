package models_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestAPIContract ensures the API response structure remains stable
func TestAPIContract(t *testing.T) {
	// This test documents the expected API response structure
	// If this test fails, it means the API contract has changed
	// and frontend code may need updates

	tests := []struct {
		name     string
		generate func() interface{}
		expected map[string][]string // expected field names for each type
	}{
		{
			name: "StateFrontend structure",
			generate: func() interface{} {
				state := models.State{
					Nodes:      []models.Node{{}},
					VMs:        []models.VM{{}},
					Containers: []models.Container{{}},
					Storage:    []models.Storage{{}},
				}
				return state.ToFrontend()
			},
			expected: map[string][]string{
				"root": {"nodes", "vms", "containers", "storage", "pbs", "metrics", 
					"pveBackups", "performance", "connectionHealth", "stats", "alerts", "lastUpdate"},
				"nodes": {"id", "node", "name", "instance", "status", "type", "cpu", 
					"mem", "maxmem", "disk", "maxdisk", "uptime", "loadAverage", 
					"kernelVersion", "pveVersion", "cpuInfo", "lastSeen", "connectionHealth"},
				"vms": {"id", "vmid", "name", "node", "instance", "status", "type", 
					"cpu", "cpus", "mem", "maxmem", "disk", "maxdisk", "netin", "netout", 
					"diskread", "diskwrite", "uptime", "template", "lastSeen"},
				"containers": {"id", "vmid", "name", "node", "instance", "status", "type", 
					"cpu", "cpus", "mem", "maxmem", "disk", "maxdisk", "netin", "netout", 
					"diskread", "diskwrite", "uptime", "template", "lastSeen"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate the response
			response := tt.generate()
			
			// Marshal to JSON and back to get field names
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}
			
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			
			// Check root fields
			checkFields(t, "root", result, tt.expected["root"])
			
			// Check nested structures
			for key, expectedFields := range tt.expected {
				if key == "root" {
					continue
				}
				
				if arr, ok := result[key].([]interface{}); ok && len(arr) > 0 {
					if obj, ok := arr[0].(map[string]interface{}); ok {
						checkFields(t, key, obj, expectedFields)
					}
				}
			}
		})
	}
}

func checkFields(t *testing.T, name string, obj map[string]interface{}, expected []string) {
	// Get actual fields
	actual := make([]string, 0, len(obj))
	for k := range obj {
		actual = append(actual, k)
	}
	
	// Check for missing expected fields
	for _, exp := range expected {
		found := false
		for _, act := range actual {
			if act == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: missing expected field '%s'", name, exp)
		}
	}
	
	// Warn about unexpected fields (don't fail, as we might add new fields)
	for _, act := range actual {
		found := false
		for _, exp := range expected {
			if act == exp {
				found = true
				break
			}
		}
		if !found {
			t.Logf("%s: unexpected field '%s' (this might be a new addition)", name, act)
		}
	}
}

// TestFieldTypeConsistency ensures numeric fields stay numeric
func TestFieldTypeConsistency(t *testing.T) {
	state := models.State{
		VMs: []models.VM{{
			NetworkIn:  -1, // Should become 0, not null
			NetworkOut: -1,
			DiskRead:   -1,
			DiskWrite:  -1,
		}},
	}
	
	frontend := state.ToFrontend()
	data, _ := json.Marshal(frontend)
	
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	
	vms := result["vms"].([]interface{})
	if len(vms) > 0 {
		vm := vms[0].(map[string]interface{})
		
		// These should be numbers, not null
		ioFields := []string{"netin", "netout", "diskread", "diskwrite"}
		for _, field := range ioFields {
			val := vm[field]
			if val == nil {
				t.Errorf("Field %s is null, should be 0", field)
			}
			if _, ok := val.(float64); !ok {
				t.Errorf("Field %s is not a number: %v (%T)", field, val, val)
			}
		}
	}
}

// TestBackwardCompatibility ensures we maintain backward compatibility
func TestBackwardCompatibility(t *testing.T) {
	// This test ensures that old field names still work
	// For example, if frontend expects both "node" and "name" fields
	
	node := models.Node{
		Name: "test-node",
	}
	
	frontend := node.ToFrontend()
	data, _ := json.Marshal(frontend)
	
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	
	// Both "node" and "name" should have the same value
	if result["node"] != result["name"] {
		t.Errorf("Backward compatibility broken: node=%v, name=%v", 
			result["node"], result["name"])
	}
}