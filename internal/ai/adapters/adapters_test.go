package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// mockStateProvider provides test state data
type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

func TestForecastDataAdapter_NilHistory(t *testing.T) {
	adapter := NewForecastDataAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil history")
	}
}

func TestMetricsAdapter_GetCurrentMetrics_VM(t *testing.T) {
	state := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:   "qemu/100",
				VMID: 100,
				Name: "webserver",
				CPU:  45.5,
				Memory: models.Memory{
					Usage: 72.3,
				},
				Disk: models.Disk{
					Usage: 55.0,
				},
				NetworkIn:  1024000,
				NetworkOut: 512000,
				DiskRead:   2048000,
				DiskWrite:  1024000,
			},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	metrics, err := adapter.GetCurrentMetrics("qemu/100")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 45.5 {
		t.Errorf("Expected CPU 45.5, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 72.3 {
		t.Errorf("Expected memory 72.3, got %f", metrics["memory"])
	}
	if metrics["disk"] != 55.0 {
		t.Errorf("Expected disk 55.0, got %f", metrics["disk"])
	}
	if metrics["netin"] != 1024000 {
		t.Errorf("Expected netin 1024000, got %f", metrics["netin"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Container(t *testing.T) {
	state := models.StateSnapshot{
		Containers: []models.Container{
			{
				ID:   "lxc/101",
				VMID: 101,
				Name: "container1",
				CPU:  25.0,
				Memory: models.Memory{
					Usage: 45.0,
				},
				Disk: models.Disk{
					Usage: 30.0,
				},
				NetworkIn:  500000,
				NetworkOut: 250000,
				DiskRead:   1000000,
				DiskWrite:  500000,
			},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	metrics, err := adapter.GetCurrentMetrics("lxc/101")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 25.0 {
		t.Errorf("Expected CPU 25.0, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 45.0 {
		t.Errorf("Expected memory 45.0, got %f", metrics["memory"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Node(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:   "node/pve1",
				Name: "pve1",
				CPU:  25.5,
				Memory: models.Memory{
					Usage: 65.0,
				},
				Disk: models.Disk{
					Usage: 40.0,
				},
			},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	metrics, err := adapter.GetCurrentMetrics("node/pve1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 25.5 {
		t.Errorf("Expected CPU 25.5, got %f", metrics["cpu"])
	}
	if metrics["memory"] != 65.0 {
		t.Errorf("Expected memory 65.0, got %f", metrics["memory"])
	}
	if metrics["disk"] != 40.0 {
		t.Errorf("Expected disk 40.0, got %f", metrics["disk"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_Storage(t *testing.T) {
	state := models.StateSnapshot{
		Storage: []models.Storage{
			{
				ID:    "storage/local-zfs",
				Name:  "local-zfs",
				Used:  50000000000,
				Total: 100000000000,
				Usage: 50.0,
			},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	metrics, err := adapter.GetCurrentMetrics("storage/local-zfs")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["disk"] != 50.0 {
		t.Errorf("Expected disk 50.0, got %f", metrics["disk"])
	}
	if metrics["used"] != 50000000000 {
		t.Errorf("Expected used 50000000000, got %f", metrics["used"])
	}
	if metrics["total"] != 100000000000 {
		t.Errorf("Expected total 100000000000, got %f", metrics["total"])
	}
}

func TestMetricsAdapter_GetCurrentMetrics_NotFound(t *testing.T) {
	state := models.StateSnapshot{}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	metrics, err := adapter.GetCurrentMetrics("nonexistent")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("Expected empty metrics, got %d entries", len(metrics))
	}
}

func TestCommandExecutorAdapter_Disabled(t *testing.T) {
	adapter := NewCommandExecutorAdapter()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := adapter.Execute(ctx, "pve1", "echo test")
	if err == nil {
		t.Error("Expected error for disabled command execution")
	}
	if output != "" {
		t.Errorf("Expected empty output, got '%s'", output)
	}

	// Verify error type
	_, ok := err.(*CommandExecutionDisabledError)
	if !ok {
		t.Errorf("Expected CommandExecutionDisabledError, got %T", err)
	}
}

func TestCommandExecutionDisabledError_Message(t *testing.T) {
	err := &CommandExecutionDisabledError{
		Target:  "pve1",
		Command: "test command",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
	if msg != "command execution is disabled - commands must be run manually" {
		t.Errorf("Unexpected error message: %s", msg)
	}
}

func TestMetricsAdapter_NilStateProvider(t *testing.T) {
	adapter := NewMetricsAdapter(nil)

	metrics, err := adapter.GetCurrentMetrics("anything")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if metrics != nil {
		t.Error("Expected nil metrics for nil state provider")
	}
}

func TestMetricsAdapter_VMIDMatch(t *testing.T) {
	state := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:   "qemu/100",
				VMID: 100,
				Name: "webserver",
				CPU:  45.5,
				Memory: models.Memory{
					Usage: 72.3,
				},
				Disk: models.Disk{
					Usage: 55.0,
				},
			},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	adapter := NewMetricsAdapter(stateProvider)

	// Test lookup by VMID string
	metrics, err := adapter.GetCurrentMetrics("100")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if metrics["cpu"] != 45.5 {
		t.Errorf("Expected CPU 45.5 when matching by VMID, got %f", metrics["cpu"])
	}
}
