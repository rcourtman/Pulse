package tools

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing

type mockStateProvider struct {
	mock.Mock
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	if len(m.ExpectedCalls) == 0 {
		return m.state
	}
	args := m.Called()
	if args.Get(0) == nil {
		return models.StateSnapshot{}
	}
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
	agents []agentexec.ConnectedAgent
}

func (m *mockAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	if len(m.ExpectedCalls) == 0 {
		return m.agents
	}
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
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

func (m *mockAlertProvider) GetRecentlyResolved(minutes int) []models.ResolvedAlert {
	args := m.Called(minutes)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]models.ResolvedAlert)
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

// stubUnifiedResourceProvider is a simple mock for UnifiedResourceProvider.
type stubUnifiedResourceProvider struct {
	resources []unifiedresources.Resource
}

func (s *stubUnifiedResourceProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	var out []unifiedresources.Resource
	for _, r := range s.resources {
		if r.Type == t {
			out = append(out, r)
		}
	}
	return out
}
