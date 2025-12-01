package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestEvaluateVMCondition(t *testing.T) {
	t.Parallel()
	m := NewManager()

	testVM := models.VM{
		ID:     "test-vm-1",
		VMID:   100,
		Name:   "test-vm",
		Node:   "node1",
		Status: "running",
		CPU:    0.50, // 50%
		Memory: models.Memory{Usage: 75.0},
		Disk:   models.Disk{Usage: 60.0},
		DiskRead:  100 * 1024 * 1024, // 100 MB/s in bytes
		DiskWrite: 50 * 1024 * 1024,  // 50 MB/s in bytes
		NetworkIn: 200 * 1024 * 1024, // 200 MB/s in bytes
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
		name      string
		field     string
		operator  string
		value     float64
		vmValue   float64 // expected processed value
		want      bool
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
