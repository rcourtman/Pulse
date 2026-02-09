package ai

import (
	"context"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// mockProvider implements providers.Provider for testing
type mockProvider struct {
	chatFunc           func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error)
	testConnectionFunc func(ctx context.Context) error
	nameFunc           func() string
	listModelsFunc     func(ctx context.Context) ([]providers.ModelInfo, error)
}

func (m *mockProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return &providers.ChatResponse{Content: "Default mock response"}, nil
}

func (m *mockProvider) TestConnection(ctx context.Context) error {
	if m.testConnectionFunc != nil {
		return m.testConnectionFunc(ctx)
	}
	return nil
}

func (m *mockProvider) Name() string {
	if m.nameFunc != nil {
		return m.nameFunc()
	}
	return "mock"
}

func (m *mockProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	if m.listModelsFunc != nil {
		return m.listModelsFunc(ctx)
	}
	return nil, nil
}

// mockThresholdProvider implements patrol.ThresholdProvider for testing
type mockThresholdProvider struct {
	nodeCPU    float64
	nodeMemory float64
	guestMem   float64
	guestDisk  float64
	storage    float64
}

func (m *mockThresholdProvider) GetNodeCPUThreshold() float64     { return m.nodeCPU }
func (m *mockThresholdProvider) GetNodeMemoryThreshold() float64  { return m.nodeMemory }
func (m *mockThresholdProvider) GetGuestCPUThreshold() float64    { return 0 }
func (m *mockThresholdProvider) GetGuestMemoryThreshold() float64 { return m.guestMem }
func (m *mockThresholdProvider) GetGuestDiskThreshold() float64   { return m.guestDisk }
func (m *mockThresholdProvider) GetStorageThreshold() float64     { return m.storage }

type mockUnifiedResourceProvider struct {
	UnifiedResourceProvider
	getAllFunc            func() []unifiedresources.Resource
	getStatsFunc          func() unifiedresources.ResourceStats
	getInfrastructureFunc func() []unifiedresources.Resource
	getWorkloadsFunc      func() []unifiedresources.Resource
	getByTypeFunc         func(t unifiedresources.ResourceType) []unifiedresources.Resource
	getTopCPUFunc         func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	getTopMemoryFunc      func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	getTopDiskFunc        func(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource
	getRelatedFunc        func(resourceID string) map[string][]unifiedresources.Resource
	findContainerHostFunc func(containerNameOrID string) string
}

func (m *mockUnifiedResourceProvider) GetAll() []unifiedresources.Resource {
	if m.getAllFunc != nil {
		return m.getAllFunc()
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetStats() unifiedresources.ResourceStats {
	if m.getStatsFunc != nil {
		return m.getStatsFunc()
	}
	return unifiedresources.ResourceStats{}
}
func (m *mockUnifiedResourceProvider) GetInfrastructure() []unifiedresources.Resource {
	if m.getInfrastructureFunc != nil {
		return m.getInfrastructureFunc()
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetWorkloads() []unifiedresources.Resource {
	if m.getWorkloadsFunc != nil {
		return m.getWorkloadsFunc()
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	if m.getByTypeFunc != nil {
		return m.getByTypeFunc(t)
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetTopByCPU(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
	if m.getTopCPUFunc != nil {
		return m.getTopCPUFunc(limit, types)
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetTopByMemory(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
	if m.getTopMemoryFunc != nil {
		return m.getTopMemoryFunc(limit, types)
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetTopByDisk(limit int, types []unifiedresources.ResourceType) []unifiedresources.Resource {
	if m.getTopDiskFunc != nil {
		return m.getTopDiskFunc(limit, types)
	}
	return nil
}
func (m *mockUnifiedResourceProvider) GetRelated(resourceID string) map[string][]unifiedresources.Resource {
	if m.getRelatedFunc != nil {
		return m.getRelatedFunc(resourceID)
	}
	return nil
}
func (m *mockUnifiedResourceProvider) FindContainerHost(containerNameOrID string) string {
	if m.findContainerHostFunc != nil {
		return m.findContainerHostFunc(containerNameOrID)
	}
	return ""
}

type mockAgentServer struct {
	agents      []agentexec.ConnectedAgent
	executeFunc func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

func (m *mockAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return m.agents
}

func (m *mockAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, agentID, cmd)
	}
	return &agentexec.CommandResultPayload{Success: true, Stdout: "Mock output"}, nil
}

type mockPolicy struct {
	decision agentexec.PolicyDecision
}

func (m *mockPolicy) Evaluate(command string) agentexec.PolicyDecision {
	return m.decision
}

type mockMetadataProvider struct {
	lastGuestID   string
	lastGuestURL  string
	lastDockerID  string
	lastDockerURL string
	lastHostID    string
	lastHostURL   string
}

func (m *mockMetadataProvider) SetGuestURL(id, url string) error {
	m.lastGuestID = id
	m.lastGuestURL = url
	return nil
}

func (m *mockMetadataProvider) SetDockerURL(id, url string) error {
	m.lastDockerID = id
	m.lastDockerURL = url
	return nil
}

func (m *mockMetadataProvider) SetHostURL(id, url string) error {
	m.lastHostID = id
	m.lastHostURL = url
	return nil
}

type mockLicenseStore struct {
	features map[string]bool
	state    string
	valid    bool
}

func (m *mockLicenseStore) HasFeature(feature string) bool {
	return m.features[feature]
}

func (m *mockLicenseStore) GetLicenseStateString() (string, bool) {
	return m.state, m.valid
}
