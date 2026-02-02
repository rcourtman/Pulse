package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDiscoverySource mocks the DiscoverySource interface
type MockDiscoverySource struct {
	mock.Mock
}

func (m *MockDiscoverySource) GetDiscovery(id string) (DiscoverySourceData, error) {
	args := m.Called(id)
	return args.Get(0).(DiscoverySourceData), args.Error(1)
}

func (m *MockDiscoverySource) GetDiscoveryByResource(resourceType, hostID, resourceID string) (DiscoverySourceData, error) {
	args := m.Called(resourceType, hostID, resourceID)
	return args.Get(0).(DiscoverySourceData), args.Error(1)
}

func (m *MockDiscoverySource) ListDiscoveries() ([]DiscoverySourceData, error) {
	args := m.Called()
	return args.Get(0).([]DiscoverySourceData), args.Error(1)
}

func (m *MockDiscoverySource) ListDiscoveriesByType(resourceType string) ([]DiscoverySourceData, error) {
	args := m.Called(resourceType)
	return args.Get(0).([]DiscoverySourceData), args.Error(1)
}

func (m *MockDiscoverySource) ListDiscoveriesByHost(hostID string) ([]DiscoverySourceData, error) {
	args := m.Called(hostID)
	return args.Get(0).([]DiscoverySourceData), args.Error(1)
}

func (m *MockDiscoverySource) FormatForAIContext(discoveries []DiscoverySourceData) string {
	args := m.Called(discoveries)
	return args.String(0)
}

func (m *MockDiscoverySource) TriggerDiscovery(ctx context.Context, resourceType, hostID, resourceID string) (DiscoverySourceData, error) {
	args := m.Called(ctx, resourceType, hostID, resourceID)
	return args.Get(0).(DiscoverySourceData), args.Error(1)
}

func TestNewDiscoveryMCPAdapter(t *testing.T) {
	assert.Nil(t, NewDiscoveryMCPAdapter(nil))
	assert.NotNil(t, NewDiscoveryMCPAdapter(&MockDiscoverySource{}))
}

func TestDiscoveryMCPAdapter_GetDiscovery(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	expectedData := DiscoverySourceData{
		ID:           "test-id",
		ServiceName:  "test-service",
		DiscoveredAt: time.Now(),
		Facts: []DiscoverySourceFact{
			{Key: "version", Value: "1.0"},
		},
	}

	mockSource.On("GetDiscovery", "test-id").Return(expectedData, nil)

	result, err := adapter.GetDiscovery("test-id")
	assert.NoError(t, err)
	assert.Equal(t, "test-id", result.ID)
	assert.Equal(t, "test-service", result.ServiceName)
	assert.Len(t, result.Facts, 1)
	assert.Equal(t, "version", result.Facts[0].Key)
	assert.Equal(t, "1.0", result.Facts[0].Value)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_GetDiscovery_Error(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	mockSource.On("GetDiscovery", "invalid").Return(DiscoverySourceData{}, errors.New("not found"))

	result, err := adapter.GetDiscovery("invalid")
	assert.Error(t, err)
	assert.Nil(t, result)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_GetDiscoveryByResource(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	expectedData := DiscoverySourceData{
		ID:         "res-id",
		ResourceID: "100",
	}

	mockSource.On("GetDiscoveryByResource", "vm", "host1", "100").Return(expectedData, nil)

	result, err := adapter.GetDiscoveryByResource("vm", "host1", "100")
	assert.NoError(t, err)
	assert.Equal(t, "res-id", result.ID)
	assert.Equal(t, "100", result.ResourceID)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_ListDiscoveries(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	list := []DiscoverySourceData{
		{ID: "d1", ServiceName: "s1"},
		{ID: "d2", ServiceName: "s2"},
	}

	mockSource.On("ListDiscoveries").Return(list, nil)

	results, err := adapter.ListDiscoveries()
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "s1", results[0].ServiceName)
	assert.Equal(t, "s2", results[1].ServiceName)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_ListDiscoveriesByType(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	list := []DiscoverySourceData{{ID: "d1", ResourceType: "vm"}}
	mockSource.On("ListDiscoveriesByType", "vm").Return(list, nil)

	results, err := adapter.ListDiscoveriesByType("vm")
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_ListDiscoveriesByHost(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	list := []DiscoverySourceData{{ID: "d1", HostID: "h1"}}
	mockSource.On("ListDiscoveriesByHost", "h1").Return(list, nil)

	results, err := adapter.ListDiscoveriesByHost("h1")
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_TriggerDiscovery(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)
	ctx := context.Background()

	expectedData := DiscoverySourceData{ID: "new1"}
	mockSource.On("TriggerDiscovery", ctx, "vm", "h1", "100").Return(expectedData, nil)

	result, err := adapter.TriggerDiscovery(ctx, "vm", "h1", "100")
	assert.NoError(t, err)
	assert.Equal(t, "new1", result.ID)

	mockSource.AssertExpectations(t)
}

func TestDiscoveryMCPAdapter_FormatForAIContext(t *testing.T) {
	mockSource := &MockDiscoverySource{}
	adapter := NewDiscoveryMCPAdapter(mockSource)

	inputs := []*ResourceDiscoveryInfo{
		{
			ID: "d1",
			Facts: []DiscoveryFact{
				{Key: "k", Value: "v"},
			},
			Ports: []DiscoveryPortInfo{
				{Port: 80},
			},
			BindMounts: []DiscoveryMount{
				{Source: "/src"},
			},
		},
	}

	// We expect the adapter to convert ResourceDiscoveryInfo back to DiscoverySourceData
	// The mock should match roughly what's passed
	mockSource.On("FormatForAIContext", mock.MatchedBy(func(ds []DiscoverySourceData) bool {
		if len(ds) != 1 {
			return false
		}
		d := ds[0]
		return d.ID == "d1" &&
			len(d.Facts) == 1 && d.Facts[0].Key == "k" &&
			len(d.Ports) == 1 && d.Ports[0].Port == 80 &&
			len(d.DockerMounts) == 1 && d.DockerMounts[0].Source == "/src"
	})).Return("formatted context")

	result := adapter.FormatForAIContext(inputs)
	assert.Equal(t, "formatted context", result)

	mockSource.AssertExpectations(t)
}
