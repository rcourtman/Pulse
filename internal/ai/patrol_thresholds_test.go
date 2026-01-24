package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockThresholdProvider implements ThresholdProvider for testing
type MockThresholdProvider struct {
	NodeCPU   float64
	NodeMem   float64
	GuestMem  float64
	GuestDisk float64
	Storage   float64
}

func (m MockThresholdProvider) GetNodeCPUThreshold() float64     { return m.NodeCPU }
func (m MockThresholdProvider) GetNodeMemoryThreshold() float64  { return m.NodeMem }
func (m MockThresholdProvider) GetGuestMemoryThreshold() float64 { return m.GuestMem }
func (m MockThresholdProvider) GetGuestDiskThreshold() float64   { return m.GuestDisk }
func (m MockThresholdProvider) GetStorageThreshold() float64     { return m.Storage }

func TestCalculatePatrolThresholds_Default(t *testing.T) {
	// Test default behavior (exact mode)
	provider := MockThresholdProvider{
		NodeCPU:   80,
		NodeMem:   90,
		GuestMem:  85,
		GuestDisk: 90,
		Storage:   95,
	}

	thresholds := CalculatePatrolThresholds(provider)

	// In exact mode (default):
	// Warning level should match the alert threshold exactly
	assert.Equal(t, 80.0, thresholds.NodeCPUWarning, "NodeCPUWarning should match alert threshold")
	assert.Equal(t, 90.0, thresholds.NodeMemWarning, "NodeMemWarning should match alert threshold")

	// Watch level should be slightly below (threshold - 5)
	assert.Equal(t, 75.0, thresholds.NodeCPUWatch, "NodeCPUWatch should be 5% below alert")
	assert.Equal(t, 85.0, thresholds.NodeMemWatch, "NodeMemWatch should be 5% below alert")

	// Critical levels (where defined) should be slightly above
	assert.Equal(t, 95.0, thresholds.GuestDiskCrit, "GuestDiskCrit should be 5% above alert")
}

func TestCalculatePatrolThresholds_Proactive(t *testing.T) {
	// Test proactive mode (warn BEFORE alert)
	provider := MockThresholdProvider{
		NodeCPU:   80,
		NodeMem:   90,
		GuestMem:  85,
		GuestDisk: 90,
		Storage:   95,
	}

	thresholds := CalculatePatrolThresholdsWithMode(provider, true)

	// In proactive mode:
	// Warning level should be 5% BELOW alert threshold
	assert.Equal(t, 75.0, thresholds.NodeCPUWarning, "NodeCPUWarning should be 5% below alert")
	assert.Equal(t, 85.0, thresholds.NodeMemWarning, "NodeMemWarning should be 5% below alert")

	// Watch level should be 15% BELOW alert threshold
	assert.Equal(t, 65.0, thresholds.NodeCPUWatch, "NodeCPUWatch should be 15% below alert")
	assert.Equal(t, 75.0, thresholds.NodeMemWatch, "NodeMemWatch should be 15% below alert")
}

func TestDefaultPatrolThresholds(t *testing.T) {
	defaults := DefaultPatrolThresholds()

	assert.Greater(t, defaults.NodeCPUWarning, defaults.NodeCPUWatch)
	assert.Greater(t, defaults.StorageCritical, defaults.StorageWarning)
	assert.Greater(t, defaults.StorageWarning, defaults.StorageWatch)
}

func TestClampThreshold(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{50.0, 50.0},
		{5.0, 10.0},   // Min cap
		{105.0, 99.0}, // Max cap
		{10.0, 10.0},
		{99.0, 99.0},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, clampThreshold(tt.input))
	}
}
