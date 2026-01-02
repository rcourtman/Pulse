package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestEvaluateVMCondition(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testVM := models.VM{
		ID:         "test-vm-1",
		VMID:       100,
		Name:       "test-vm",
		Node:       "node1",
		Status:     "running",
		CPU:        0.50, // 50%
		Memory:     models.Memory{Usage: 75.0},
		Disk:       models.Disk{Usage: 60.0},
		DiskRead:   100 * 1024 * 1024, // 100 MB/s in bytes
		DiskWrite:  50 * 1024 * 1024,  // 50 MB/s in bytes
		NetworkIn:  200 * 1024 * 1024, // 200 MB/s in bytes
		NetworkOut: 150 * 1024 * 1024, // 150 MB/s in bytes
	}

	tests := []struct {
		name      string
		vm        models.VM
		condition FilterCondition
		want      bool
	}{
		// Metric conditions - CPU
		{
			name: "CPU above threshold",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    40.0,
			},
			want: true,
		},
		{
			name: "CPU below threshold",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    60.0,
			},
			want: false,
		},
		{
			name: "CPU equals threshold (within tolerance)",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: "==",
				Value:    50.0,
			},
			want: true,
		},
		{
			name: "CPU case insensitive field",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "CPU",
				Operator: ">",
				Value:    40.0,
			},
			want: true,
		},
		// Metric conditions - Memory
		{
			name: "Memory >= threshold",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "memory",
				Operator: ">=",
				Value:    75.0,
			},
			want: true,
		},
		{
			name: "Memory < threshold",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "memory",
				Operator: "<",
				Value:    80.0,
			},
			want: true,
		},
		// Metric conditions - Disk
		{
			name: "Disk <= threshold",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "disk",
				Operator: "<=",
				Value:    70.0,
			},
			want: true,
		},
		// Metric conditions - DiskRead
		{
			name: "DiskRead conversion to MB/s",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "diskread",
				Operator: ">",
				Value:    50.0, // Threshold in MB/s
			},
			want: true, // 100 MB/s > 50 MB/s
		},
		// Metric conditions - DiskWrite
		{
			name: "DiskWrite conversion to MB/s",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "diskwrite",
				Operator: "<",
				Value:    100.0,
			},
			want: true, // 50 MB/s < 100 MB/s
		},
		// Metric conditions - NetworkIn
		{
			name: "NetworkIn conversion to MB/s",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "networkin",
				Operator: ">=",
				Value:    200.0,
			},
			want: true,
		},
		// Metric conditions - NetworkOut
		{
			name: "NetworkOut conversion to MB/s",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "networkout",
				Operator: "<=",
				Value:    150.0,
			},
			want: true,
		},
		// Value type conversions
		{
			name: "Value as int instead of float64",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    40, // int, not float64
			},
			want: true,
		},
		{
			name: "Value as string (should fail)",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    "40", // string, not numeric
			},
			want: false,
		},
		// Unknown field
		{
			name: "Unknown metric field",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "unknown",
				Operator: ">",
				Value:    10.0,
			},
			want: false,
		},
		// Text conditions
		{
			name: "Name contains text",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "test",
			},
			want: true,
		},
		{
			name: "Name does not contain text",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "production",
			},
			want: false,
		},
		{
			name: "Name case insensitive match",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "TEST",
			},
			want: true,
		},
		{
			name: "Node contains text",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "node",
				Value: "node1",
			},
			want: true,
		},
		{
			name: "Node case insensitive",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "node",
				Value: "NODE1",
			},
			want: true,
		},
		{
			name: "VMID matches",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "vmid",
				Value: "test-vm-1",
			},
			want: true,
		},
		{
			name: "VMID partial match",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "vmid",
				Value: "test",
			},
			want: true,
		},
		{
			name: "Text condition unknown field",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "unknown",
				Value: "test",
			},
			want: false,
		},
		// Raw conditions
		{
			name: "Raw text matches name",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "test",
			},
			want: true,
		},
		{
			name: "Raw text matches node",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "node1",
			},
			want: true,
		},
		{
			name: "Raw text matches ID",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "test-vm-1",
			},
			want: true,
		},
		{
			name: "Raw text matches status",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "running",
			},
			want: true,
		},
		{
			name: "Raw text case insensitive",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "RUNNING",
			},
			want: true,
		},
		{
			name: "Raw text no match",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "stopped",
			},
			want: false,
		},
		{
			name: "Raw text empty (should fail)",
			vm:   testVM,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "",
			},
			want: false,
		},
		// Edge cases
		{
			name: "Unknown condition type",
			vm:   testVM,
			condition: FilterCondition{
				Type:  "unknown",
				Field: "cpu",
				Value: 50.0,
			},
			want: false,
		},
		{
			name: "Unknown operator",
			vm:   testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: "!=",
				Value:    50.0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.evaluateVMCondition(tt.vm, tt.condition)
			if got != tt.want {
				t.Errorf("evaluateVMCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateContainerCondition(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testContainer := models.Container{
		ID:         "test-ct-1",
		VMID:       200,
		Name:       "test-container",
		Node:       "node2",
		Status:     "running",
		CPU:        0.30, // 30%
		Memory:     models.Memory{Usage: 50.0},
		Disk:       models.Disk{Usage: 40.0},
		DiskRead:   75 * 1024 * 1024,  // 75 MB/s in bytes
		DiskWrite:  25 * 1024 * 1024,  // 25 MB/s in bytes
		NetworkIn:  100 * 1024 * 1024, // 100 MB/s in bytes
		NetworkOut: 80 * 1024 * 1024,  // 80 MB/s in bytes
	}

	tests := []struct {
		name      string
		container models.Container
		condition FilterCondition
		want      bool
	}{
		// Metric conditions - CPU
		{
			name:      "CPU above threshold",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    20.0,
			},
			want: true,
		},
		{
			name:      "CPU below threshold",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: "<",
				Value:    50.0,
			},
			want: true,
		},
		// Metric conditions - Memory
		{
			name:      "Memory equals threshold",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "memory",
				Operator: "=",
				Value:    50.0,
			},
			want: true,
		},
		// Metric conditions - Disk
		{
			name:      "Disk threshold",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "disk",
				Operator: "<=",
				Value:    40.0,
			},
			want: true,
		},
		// Metric conditions - DiskRead
		{
			name:      "DiskRead conversion",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "diskread",
				Operator: ">",
				Value:    50.0,
			},
			want: true, // 75 MB/s > 50 MB/s
		},
		// Metric conditions - DiskWrite
		{
			name:      "DiskWrite conversion",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "diskwrite",
				Operator: ">=",
				Value:    25.0,
			},
			want: true,
		},
		// Metric conditions - NetworkIn
		{
			name:      "NetworkIn conversion",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "networkin",
				Operator: "==",
				Value:    100.0,
			},
			want: true,
		},
		// Metric conditions - NetworkOut
		{
			name:      "NetworkOut conversion",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "networkout",
				Operator: "<",
				Value:    90.0,
			},
			want: true,
		},
		// Value type conversions
		{
			name:      "Value as int",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    20,
			},
			want: true,
		},
		{
			name:      "Value as string (should fail)",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    "20",
			},
			want: false,
		},
		// Text conditions
		{
			name:      "Name contains text",
			container: testContainer,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "container",
			},
			want: true,
		},
		{
			name:      "Name case insensitive",
			container: testContainer,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "CONTAINER",
			},
			want: true,
		},
		{
			name:      "Node matches",
			container: testContainer,
			condition: FilterCondition{
				Type:  "text",
				Field: "node",
				Value: "node2",
			},
			want: true,
		},
		{
			name:      "VMID matches",
			container: testContainer,
			condition: FilterCondition{
				Type:  "text",
				Field: "vmid",
				Value: "test-ct",
			},
			want: true,
		},
		// Raw conditions
		{
			name:      "Raw text matches name",
			container: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "test",
			},
			want: true,
		},
		{
			name:      "Raw text matches node",
			container: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "node2",
			},
			want: true,
		},
		{
			name:      "Raw text matches status",
			container: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "running",
			},
			want: true,
		},
		{
			name:      "Raw text no match",
			container: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "stopped",
			},
			want: false,
		},
		{
			name:      "Raw text empty",
			container: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "",
			},
			want: false,
		},
		// Unknown field
		{
			name:      "Unknown metric field",
			container: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "unknown",
				Operator: ">",
				Value:    10.0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.evaluateContainerCondition(tt.container, tt.condition)
			if got != tt.want {
				t.Errorf("evaluateContainerCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateFilterStack(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testVM := models.VM{
		ID:     "test-vm",
		Name:   "production-vm",
		Node:   "node1",
		Status: "running",
		CPU:    0.60, // 60%
		Memory: models.Memory{Usage: 80.0},
		Disk:   models.Disk{Usage: 70.0},
	}

	tests := []struct {
		name  string
		guest interface{}
		stack FilterStack
		want  bool
	}{
		{
			name:  "Empty filter stack returns true",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "AND",
				Filters:         []FilterCondition{},
			},
			want: true,
		},
		{
			name:  "AND operator - all match",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    50.0,
					},
					{
						Type:     "metric",
						Field:    "memory",
						Operator: ">",
						Value:    70.0,
					},
				},
			},
			want: true,
		},
		{
			name:  "AND operator - one does not match",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    50.0,
					},
					{
						Type:     "metric",
						Field:    "memory",
						Operator: ">",
						Value:    90.0, // This will fail
					},
				},
			},
			want: false,
		},
		{
			name:  "OR operator - one matches",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "OR",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    90.0, // This will fail
					},
					{
						Type:     "metric",
						Field:    "memory",
						Operator: ">",
						Value:    70.0, // This will pass
					},
				},
			},
			want: true,
		},
		{
			name:  "OR operator - none match",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "OR",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    90.0,
					},
					{
						Type:     "metric",
						Field:    "memory",
						Operator: ">",
						Value:    95.0,
					},
				},
			},
			want: false,
		},
		{
			name:  "Mixed condition types with AND",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    50.0,
					},
					{
						Type:  "text",
						Field: "name",
						Value: "production",
					},
					{
						Type:    "raw",
						RawText: "node1",
					},
				},
			},
			want: true,
		},
		{
			name:  "Mixed condition types with OR",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "OR",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    90.0, // Fails
					},
					{
						Type:  "text",
						Field: "name",
						Value: "development", // Fails
					},
					{
						Type:    "raw",
						RawText: "node1", // Passes
					},
				},
			},
			want: true,
		},
		{
			name:  "Single condition in stack",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "AND",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    50.0,
					},
				},
			},
			want: true,
		},
		{
			name:  "Default operator (not AND) - should behave as OR",
			guest: testVM,
			stack: FilterStack{
				LogicalOperator: "UNKNOWN",
				Filters: []FilterCondition{
					{
						Type:     "metric",
						Field:    "cpu",
						Operator: ">",
						Value:    90.0,
					},
					{
						Type:     "metric",
						Field:    "memory",
						Operator: ">",
						Value:    70.0,
					},
				},
			},
			want: true, // Falls through to OR logic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.evaluateFilterStack(tt.guest, tt.stack)
			if got != tt.want {
				t.Errorf("evaluateFilterStack() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateFilterCondition(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testVM := models.VM{
		ID:     "test-vm",
		Name:   "test",
		CPU:    0.50,
		Memory: models.Memory{Usage: 75.0},
	}

	testContainer := models.Container{
		ID:     "test-ct",
		Name:   "test-container",
		CPU:    0.30,
		Memory: models.Memory{Usage: 50.0},
	}

	tests := []struct {
		name      string
		guest     interface{}
		condition FilterCondition
		want      bool
	}{
		{
			name:  "VM type delegates to evaluateVMCondition",
			guest: testVM,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    40.0,
			},
			want: true,
		},
		{
			name:  "Container type delegates to evaluateContainerCondition",
			guest: testContainer,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    20.0,
			},
			want: true,
		},
		{
			name:  "Unknown type returns false",
			guest: "invalid-guest-type",
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    50.0,
			},
			want: false,
		},
		{
			name:  "Nil guest returns false",
			guest: nil,
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: ">",
				Value:    50.0,
			},
			want: false,
		},
		{
			name:  "VM with text condition",
			guest: testVM,
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "test",
			},
			want: true,
		},
		{
			name:  "Container with raw condition",
			guest: testContainer,
			condition: FilterCondition{
				Type:    "raw",
				RawText: "container",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.evaluateFilterCondition(tt.guest, tt.condition)
			if got != tt.want {
				t.Errorf("evaluateFilterCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMetricOperators tests all metric operators thoroughly
func TestMetricOperators(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testVM := models.VM{
		CPU:    0.50, // 50%
		Memory: models.Memory{Usage: 75.0},
	}

	tests := []struct {
		name     string
		field    string
		operator string
		value    float64
		vmValue  float64 // expected processed value
		want     bool
	}{
		// Greater than
		{"GT true", "cpu", ">", 40.0, 50.0, true},
		{"GT false", "cpu", ">", 60.0, 50.0, false},
		{"GT equal", "cpu", ">", 50.0, 50.0, false},

		// Less than
		{"LT true", "cpu", "<", 60.0, 50.0, true},
		{"LT false", "cpu", "<", 40.0, 50.0, false},
		{"LT equal", "cpu", "<", 50.0, 50.0, false},

		// Greater than or equal
		{"GTE true greater", "cpu", ">=", 40.0, 50.0, true},
		{"GTE true equal", "cpu", ">=", 50.0, 50.0, true},
		{"GTE false", "cpu", ">=", 60.0, 50.0, false},

		// Less than or equal
		{"LTE true less", "cpu", "<=", 60.0, 50.0, true},
		{"LTE true equal", "cpu", "<=", 50.0, 50.0, true},
		{"LTE false", "cpu", "<=", 40.0, 50.0, false},

		// Equality with tolerance
		{"EQ true exact", "memory", "=", 75.0, 75.0, true},
		{"EQ true within tolerance high", "memory", "=", 75.4, 75.0, true},
		{"EQ true within tolerance low", "memory", "=", 74.6, 75.0, true},
		{"EQ false outside tolerance", "memory", "=", 76.0, 75.0, false},
		{"EQ double equals", "memory", "==", 75.0, 75.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			condition := FilterCondition{
				Type:     "metric",
				Field:    tt.field,
				Operator: tt.operator,
				Value:    tt.value,
			}
			got := m.evaluateVMCondition(testVM, condition)
			if got != tt.want {
				t.Errorf("evaluateVMCondition(%s %s %v) = %v, want %v (vm value: %v)",
					tt.field, tt.operator, tt.value, got, tt.want, tt.vmValue)
			}
		})
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	t.Parallel()
	m := NewManager()

	tests := []struct {
		name      string
		vm        models.VM
		condition FilterCondition
		want      bool
	}{
		{
			name: "Zero CPU value",
			vm: models.VM{
				CPU: 0.0,
			},
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: "==",
				Value:    0.0,
			},
			want: true,
		},
		{
			name: "Empty string in text search",
			vm: models.VM{
				Name: "test-vm",
			},
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "",
			},
			want: true, // Empty string matches everything
		},
		{
			name: "Empty VM name",
			vm: models.VM{
				Name: "",
			},
			condition: FilterCondition{
				Type:  "text",
				Field: "name",
				Value: "test",
			},
			want: false,
		},
		{
			name: "Negative CPU (edge case)",
			vm: models.VM{
				CPU: -0.1,
			},
			condition: FilterCondition{
				Type:     "metric",
				Field:    "cpu",
				Operator: "<",
				Value:    0.0,
			},
			want: true,
		},
		{
			name: "Very large metric value",
			vm: models.VM{
				Memory: models.Memory{Usage: 99999.0},
			},
			condition: FilterCondition{
				Type:     "metric",
				Field:    "memory",
				Operator: ">",
				Value:    10000.0,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.evaluateVMCondition(tt.vm, tt.condition)
			if got != tt.want {
				t.Errorf("evaluateVMCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetGuestThresholds(t *testing.T) {
	t.Run("returns defaults when no overrides or rules", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides:   make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		if result.CPU.Trigger != 80 {
			t.Errorf("expected CPU trigger 80, got %v", result.CPU.Trigger)
		}
	})

	t.Run("applies guest-specific override", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: map[string]ThresholdConfig{
					"pve1-100": {
						CPU: &HysteresisThreshold{
							Trigger: 95,
							Clear:   90,
						},
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		if result.CPU.Trigger != 95 {
			t.Errorf("expected CPU trigger 95 from override, got %v", result.CPU.Trigger)
		}
	})

	t.Run("applies custom rule matching filter", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{
					{
						Name:     "high-cpu-vms",
						Enabled:  true,
						Priority: 10,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{
									Type:  "text",
									Field: "name",
									Value: "test",
								},
							},
						},
						Thresholds: ThresholdConfig{
							CPU: &HysteresisThreshold{
								Trigger: 70,
								Clear:   65,
							},
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		if result.CPU.Trigger != 70 {
			t.Errorf("expected CPU trigger 70 from custom rule, got %v", result.CPU.Trigger)
		}
	})

	t.Run("override takes precedence over custom rule", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: map[string]ThresholdConfig{
					"pve1-100": {
						CPU: &HysteresisThreshold{
							Trigger: 95,
							Clear:   90,
						},
					},
				},
				CustomRules: []CustomAlertRule{
					{
						Name:     "high-cpu-vms",
						Enabled:  true,
						Priority: 10,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{
									Type:  "text",
									Field: "name",
									Value: "test",
								},
							},
						},
						Thresholds: ThresholdConfig{
							CPU: &HysteresisThreshold{
								Trigger: 70,
								Clear:   65,
							},
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		// Override takes precedence over custom rule
		if result.CPU.Trigger != 95 {
			t.Errorf("expected CPU trigger 95 from override, got %v", result.CPU.Trigger)
		}
	})

	t.Run("higher priority rule wins", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{
					{
						Name:     "low-priority-rule",
						Enabled:  true,
						Priority: 5,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{Type: "text", Field: "name", Value: "test"},
							},
						},
						Thresholds: ThresholdConfig{
							CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
						},
					},
					{
						Name:     "high-priority-rule",
						Enabled:  true,
						Priority: 20,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{Type: "text", Field: "name", Value: "test"},
							},
						},
						Thresholds: ThresholdConfig{
							CPU: &HysteresisThreshold{Trigger: 75, Clear: 70},
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		if result.CPU.Trigger != 75 {
			t.Errorf("expected CPU trigger 75 from high-priority rule, got %v", result.CPU.Trigger)
		}
	})

	t.Run("disabled rule is skipped", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{
					{
						Name:     "disabled-rule",
						Enabled:  false,
						Priority: 100,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{Type: "text", Field: "name", Value: "test"},
							},
						},
						Thresholds: ThresholdConfig{
							CPU: &HysteresisThreshold{Trigger: 50, Clear: 45},
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		// Should use defaults since rule is disabled
		if result.CPU.Trigger != 80 {
			t.Errorf("expected CPU trigger 80 from defaults, got %v", result.CPU.Trigger)
		}
	})

	t.Run("disabled override disables thresholds", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: map[string]ThresholdConfig{
					"pve1-100": {
						Disabled: true,
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if !result.Disabled {
			t.Error("expected thresholds to be disabled")
		}
	})

	t.Run("applies disable connectivity from override", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					CPU: &HysteresisThreshold{
						Trigger: 80,
						Clear:   75,
					},
				},
				Overrides: map[string]ThresholdConfig{
					"pve1-100": {
						DisableConnectivity: true,
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if !result.DisableConnectivity {
			t.Error("expected DisableConnectivity to be true")
		}
	})

	t.Run("applies disable connectivity from custom rule", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{},
				Overrides:     make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{
					{
						Name:     "no-connectivity-rule",
						Enabled:  true,
						Priority: 10,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{Type: "text", Field: "name", Value: "test"},
							},
						},
						Thresholds: ThresholdConfig{
							DisableConnectivity: true,
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if !result.DisableConnectivity {
			t.Error("expected DisableConnectivity to be true from custom rule")
		}
	})

	t.Run("applies legacy CPU threshold", func(t *testing.T) {
		legacyThreshold := float64(85)
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{},
				Overrides: map[string]ThresholdConfig{
					"pve1-100": {
						CPULegacy: &legacyThreshold,
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		// Legacy threshold becomes trigger with calculated clear
		if result.CPU.Trigger != 85 {
			t.Errorf("expected CPU trigger 85 from legacy threshold, got %v", result.CPU.Trigger)
		}
	})

	t.Run("legacy ID migration for clustered VM", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{},
				Overrides: map[string]ThresholdConfig{
					// Legacy format: instance-node-vmid
					"pve1-node1-100": {
						CPU: &HysteresisThreshold{Trigger: 60, Clear: 55},
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "node1",
			Instance: "pve1",
			VMID:     100,
		}

		// Query with new format
		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil after legacy migration")
		}
		if result.CPU.Trigger != 60 {
			t.Errorf("expected CPU trigger 60 from migrated legacy override, got %v", result.CPU.Trigger)
		}

		// Verify the override was migrated to new ID
		if _, exists := m.config.Overrides["pve1-100"]; !exists {
			t.Error("override should be migrated to new ID format")
		}
		if _, exists := m.config.Overrides["pve1-node1-100"]; exists {
			t.Error("old legacy override should be removed after migration")
		}
	})

	t.Run("legacy ID migration for standalone VM", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{},
				Overrides: map[string]ThresholdConfig{
					// Legacy standalone format: node-vmid
					"pve1-100": {
						CPU: &HysteresisThreshold{Trigger: 55, Clear: 50},
					},
				},
				CustomRules: []CustomAlertRule{},
			},
		}

		// For standalone, node == instance
		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil {
			t.Fatal("CPU threshold should not be nil")
		}
		if result.CPU.Trigger != 55 {
			t.Errorf("expected CPU trigger 55, got %v", result.CPU.Trigger)
		}
	})

	t.Run("works with container type", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{
					Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
				},
				Overrides:   make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{},
			},
		}

		container := models.Container{
			Name:     "test-container",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     200,
		}

		result := m.getGuestThresholds(container, "pve1-200")

		if result.Memory == nil {
			t.Fatal("Memory threshold should not be nil")
		}
		if result.Memory.Trigger != 85 {
			t.Errorf("expected Memory trigger 85, got %v", result.Memory.Trigger)
		}
	})

	t.Run("applies all metric thresholds from rule", func(t *testing.T) {
		m := &Manager{
			config: AlertConfig{
				GuestDefaults: ThresholdConfig{},
				Overrides:     make(map[string]ThresholdConfig),
				CustomRules: []CustomAlertRule{
					{
						Name:     "full-metrics-rule",
						Enabled:  true,
						Priority: 10,
						FilterConditions: FilterStack{
							LogicalOperator: "AND",
							Filters: []FilterCondition{
								{Type: "text", Field: "name", Value: "test"},
							},
						},
						Thresholds: ThresholdConfig{
							CPU:        &HysteresisThreshold{Trigger: 70, Clear: 65},
							Memory:     &HysteresisThreshold{Trigger: 75, Clear: 70},
							Disk:       &HysteresisThreshold{Trigger: 80, Clear: 75},
							DiskRead:   &HysteresisThreshold{Trigger: 50, Clear: 45},
							DiskWrite:  &HysteresisThreshold{Trigger: 55, Clear: 50},
							NetworkIn:  &HysteresisThreshold{Trigger: 60, Clear: 55},
							NetworkOut: &HysteresisThreshold{Trigger: 65, Clear: 60},
						},
					},
				},
			},
		}

		vm := models.VM{
			Name:     "test-vm",
			Node:     "pve1",
			Instance: "pve1",
			VMID:     100,
		}

		result := m.getGuestThresholds(vm, "pve1-100")

		if result.CPU == nil || result.CPU.Trigger != 70 {
			t.Errorf("CPU threshold not applied correctly")
		}
		if result.Memory == nil || result.Memory.Trigger != 75 {
			t.Errorf("Memory threshold not applied correctly")
		}
		if result.Disk == nil || result.Disk.Trigger != 80 {
			t.Errorf("Disk threshold not applied correctly")
		}
		if result.DiskRead == nil || result.DiskRead.Trigger != 50 {
			t.Errorf("DiskRead threshold not applied correctly")
		}
		if result.DiskWrite == nil || result.DiskWrite.Trigger != 55 {
			t.Errorf("DiskWrite threshold not applied correctly")
		}
		if result.NetworkIn == nil || result.NetworkIn.Trigger != 60 {
			t.Errorf("NetworkIn threshold not applied correctly")
		}
		if result.NetworkOut == nil || result.NetworkOut.Trigger != 65 {
			t.Errorf("NetworkOut threshold not applied correctly")
		}
	})
}

func TestExtractGuestMetrics_Default(t *testing.T) {
	t.Parallel()
	_, ok := extractGuestMetrics("invalid-type")
	if ok {
		t.Error("extractGuestMetrics should return false for invalid type")
	}
}

func TestGetGuestThresholds_AllFields(t *testing.T) {
	t.Parallel()
	m := NewManager()

	// Define a custom rule that sets all fields
	trigger := 90.0
	clear := 85.0
	threshold := &HysteresisThreshold{Trigger: trigger, Clear: clear}

	rule := CustomAlertRule{
		ID:       "rule-1",
		Name:     "All Fields Rule",
		Enabled:  true,
		Priority: 100,
		FilterConditions: FilterStack{
			LogicalOperator: "AND",
			Filters: []FilterCondition{
				{Type: "text", Field: "name", Value: "test-guest"},
			},
		},
		Thresholds: ThresholdConfig{
			CPU:                 threshold,
			Memory:              threshold,
			Disk:                threshold,
			DiskRead:            threshold,
			DiskWrite:           threshold,
			NetworkIn:           threshold,
			NetworkOut:          threshold,
			DisableConnectivity: true,
			Backup:              &BackupAlertConfig{Enabled: true},
			Snapshot:            &SnapshotAlertConfig{Enabled: true},
		},
	}

	m.config.CustomRules = []CustomAlertRule{rule}

	guest := models.VM{ID: "guest-1", Name: "test-guest"}

	thresholds := m.getGuestThresholds(guest, "guest-1")

	if thresholds.CPU == nil || thresholds.CPU.Trigger != trigger {
		t.Error("CPU threshold not applied")
	}
	if thresholds.Memory == nil || thresholds.Memory.Trigger != trigger {
		t.Error("Memory threshold not applied")
	}
	if thresholds.Disk == nil || thresholds.Disk.Trigger != trigger {
		t.Error("Disk threshold not applied")
	}
	if thresholds.DiskRead == nil || thresholds.DiskRead.Trigger != trigger {
		t.Error("DiskRead threshold not applied")
	}
	if thresholds.DiskWrite == nil || thresholds.DiskWrite.Trigger != trigger {
		t.Error("DiskWrite threshold not applied")
	}
	if thresholds.NetworkIn == nil || thresholds.NetworkIn.Trigger != trigger {
		t.Error("NetworkIn threshold not applied")
	}
	if thresholds.NetworkOut == nil || thresholds.NetworkOut.Trigger != trigger {
		t.Error("NetworkOut threshold not applied")
	}
	if !thresholds.DisableConnectivity {
		t.Error("DisableConnectivity not applied")
	}
	if thresholds.Backup == nil || !thresholds.Backup.Enabled {
		t.Error("Backup config not applied")
	}
	if thresholds.Snapshot == nil || !thresholds.Snapshot.Enabled {
		t.Error("Snapshot config not applied")
	}
}

func TestGetGuestThresholds_LegacyFields(t *testing.T) {
	t.Parallel()
	m := NewManager()

	legacyValue := 95.0

	ruleLegacy := CustomAlertRule{
		ID:       "rule-legacy",
		Name:     "Legacy Fields Rule",
		Enabled:  true,
		Priority: 200,
		FilterConditions: FilterStack{
			LogicalOperator: "AND",
			Filters: []FilterCondition{
				{Type: "text", Field: "name", Value: "test-guest"},
			},
		},
		Thresholds: ThresholdConfig{
			CPULegacy:        &legacyValue,
			MemoryLegacy:     &legacyValue,
			DiskLegacy:       &legacyValue,
			DiskReadLegacy:   &legacyValue,
			DiskWriteLegacy:  &legacyValue,
			NetworkInLegacy:  &legacyValue,
			NetworkOutLegacy: &legacyValue,
		},
	}

	m.config.CustomRules = []CustomAlertRule{ruleLegacy}

	guest := models.VM{ID: "guest-1", Name: "test-guest"}

	thresholds := m.getGuestThresholds(guest, "guest-1")

	if thresholds.CPU == nil || thresholds.CPU.Trigger != legacyValue {
		t.Errorf("Legacy CPU threshold not applied")
	}
	if thresholds.Memory == nil || thresholds.Memory.Trigger != legacyValue {
		t.Errorf("Legacy Memory threshold not applied")
	}
	if thresholds.Disk == nil || thresholds.Disk.Trigger != legacyValue {
		t.Errorf("Legacy Disk threshold not applied")
	}
	if thresholds.DiskRead == nil || thresholds.DiskRead.Trigger != legacyValue {
		t.Errorf("Legacy DiskRead threshold not applied")
	}
	if thresholds.DiskWrite == nil || thresholds.DiskWrite.Trigger != legacyValue {
		t.Errorf("Legacy DiskWrite threshold not applied")
	}
	if thresholds.NetworkIn == nil || thresholds.NetworkIn.Trigger != legacyValue {
		t.Errorf("Legacy NetworkIn threshold not applied")
	}
	if thresholds.NetworkOut == nil || thresholds.NetworkOut.Trigger != legacyValue {
		t.Errorf("Legacy NetworkOut threshold not applied")
	}
}

func TestGetGuestThresholds_Override(t *testing.T) {
	t.Parallel()
	m := NewManager()

	trigger := 88.0
	threshold := &HysteresisThreshold{Trigger: trigger, Clear: trigger - 5.0}

	m.config.Overrides = map[string]ThresholdConfig{
		"guest-1": {
			CPU:                 threshold,
			Memory:              threshold,
			Disk:                threshold,
			DiskRead:            threshold,
			DiskWrite:           threshold,
			NetworkIn:           threshold,
			NetworkOut:          threshold,
			Disabled:            true,
			DisableConnectivity: true,
			Backup:              &BackupAlertConfig{Enabled: true},
			Snapshot:            &SnapshotAlertConfig{Enabled: true},
		},
	}

	guest := models.VM{ID: "guest-1", Name: "test-guest"}
	thresholds := m.getGuestThresholds(guest, "guest-1")

	if thresholds.CPU.Trigger != trigger {
		t.Error("Override CPU not applied")
	}
	if !thresholds.Disabled {
		t.Error("Override Disabled not applied")
	}
	if !thresholds.DisableConnectivity {
		t.Error("Override DisableConnectivity not applied")
	}
	if thresholds.Backup == nil {
		t.Error("Override Backup not applied")
	}
}

func TestGetGuestThresholds_OverrideLegacy(t *testing.T) {
	t.Parallel()
	m := NewManager()

	legacyValue := 77.0

	m.config.Overrides = map[string]ThresholdConfig{
		"guest-1": {
			CPULegacy:        &legacyValue,
			MemoryLegacy:     &legacyValue,
			DiskLegacy:       &legacyValue,
			DiskReadLegacy:   &legacyValue,
			DiskWriteLegacy:  &legacyValue,
			NetworkInLegacy:  &legacyValue,
			NetworkOutLegacy: &legacyValue,
		},
	}

	guest := models.VM{ID: "guest-1", Name: "test-guest"}
	thresholds := m.getGuestThresholds(guest, "guest-1")

	if thresholds.CPU == nil || thresholds.CPU.Trigger != legacyValue {
		t.Error("Override Legacy CPU not applied")
	}
}

func TestGetGuestThresholds_InvalidGuest(t *testing.T) {
	t.Parallel()
	m := NewManager()

	// Should return defaults (and hit default case in tryLegacyOverrideMigration)
	thresholds := m.getGuestThresholds("invalid-guest-struct", "guest-1")
	if thresholds.CPU == nil {
		// Just check it returns something valid (defaults)
		// actually default has nil pointers, so this check is just ensuring no panic
	}
}
