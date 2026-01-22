package tools

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing

type mockStateProvider struct {
	mock.Mock
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	args := m.Called()
	return args.Get(0).(models.StateSnapshot)
}

type mockCommandPolicy struct {
	mock.Mock
}

func (m *mockCommandPolicy) Evaluate(command string) agentexec.PolicyDecision {
	args := m.Called(command)
	return args.Get(0).(agentexec.PolicyDecision)
}

type mockAgentServer struct {
	mock.Mock
}

func (m *mockAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	args := m.Called()
	return args.Get(0).([]agentexec.ConnectedAgent)
}

func (m *mockAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	args := m.Called(ctx, agentID, cmd)
	return args.Get(0).(*agentexec.CommandResultPayload), args.Error(1)
}

type mockMetricsHistoryProvider struct {
	mock.Mock
}

func (m *mockMetricsHistoryProvider) GetResourceMetrics(resourceID string, period time.Duration) ([]MetricPoint, error) {
	args := m.Called(resourceID, period)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]MetricPoint), args.Error(1)
}

func (m *mockMetricsHistoryProvider) GetAllMetricsSummary(period time.Duration) (map[string]ResourceMetricsSummary, error) {
	args := m.Called(period)
	return args.Get(0).(map[string]ResourceMetricsSummary), args.Error(1)
}

type mockAlertProvider struct {
	mock.Mock
}

func (m *mockAlertProvider) GetActiveAlerts() []ActiveAlert {
	args := m.Called()
	return args.Get(0).([]ActiveAlert)
}

type mockFindingsProvider struct {
	mock.Mock
}

func (m *mockFindingsProvider) GetActiveFindings() []Finding {
	args := m.Called()
	return args.Get(0).([]Finding)
}

func (m *mockFindingsProvider) GetDismissedFindings() []Finding {
	args := m.Called()
	return args.Get(0).([]Finding)
}

type mockDiskHealthProvider struct {
	mock.Mock
}

func (m *mockDiskHealthProvider) GetHosts() []models.Host {
	args := m.Called()
	return args.Get(0).([]models.Host)
}

type mockUpdatesProvider struct {
	mock.Mock
}

func (m *mockUpdatesProvider) GetPendingUpdates(hostID string) []ContainerUpdateInfo {
	args := m.Called(hostID)
	return args.Get(0).([]ContainerUpdateInfo)
}

func (m *mockUpdatesProvider) TriggerUpdateCheck(hostID string) (DockerCommandStatus, error) {
	args := m.Called(hostID)
	return args.Get(0).(DockerCommandStatus), args.Error(1)
}

func (m *mockUpdatesProvider) UpdateContainer(hostID, containerID, containerName string) (DockerCommandStatus, error) {
	args := m.Called(hostID, containerID, containerName)
	return args.Get(0).(DockerCommandStatus), args.Error(1)
}

func (m *mockUpdatesProvider) IsUpdateActionsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

type mockBackupProvider struct {
	mock.Mock
}

func (m *mockBackupProvider) GetBackups() models.Backups {
	args := m.Called()
	return args.Get(0).(models.Backups)
}

func (m *mockBackupProvider) GetPBSInstances() []models.PBSInstance {
	args := m.Called()
	return args.Get(0).([]models.PBSInstance)
}

type mockStorageProvider struct {
	mock.Mock
}

func (m *mockStorageProvider) GetStorage() []models.Storage {
	args := m.Called()
	return args.Get(0).([]models.Storage)
}

func (m *mockStorageProvider) GetCephClusters() []models.CephCluster {
	args := m.Called()
	return args.Get(0).([]models.CephCluster)
}
