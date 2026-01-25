package chat

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPatrolService_ParseFindings(t *testing.T) {
	service := NewPatrolService(nil)

	tests := []struct {
		name     string
		response string
		want     int
		check    func(t *testing.T, f *PatrolFinding)
	}{
		{
			name: "Single Valid Finding",
			response: `
Some introductory text...

[FINDING]
KEY: high-cpu-load
SEVERITY: warning
CATEGORY: performance
RESOURCE: node-01
RESOURCE_TYPE: node
TITLE: High CPU Load
DESCRIPTION: CPU usage is consistently above 80%
RECOMMENDATION: Resize node or investigate process
EVIDENCE: metrics show 85% avg
[/FINDING]

Closing remarks...
`,
			want: 1,
			check: func(t *testing.T, f *PatrolFinding) {
				assert.Equal(t, "high-cpu-load", f.Key)
				assert.Equal(t, "warning", f.Severity)
				assert.Equal(t, "performance", f.Category)
				assert.Equal(t, "node-01", f.ResourceID)
				assert.Equal(t, "node", f.ResourceType)
				assert.Equal(t, "High CPU Load", f.Title)
			},
		},
		{
			name: "Multiple Findings",
			response: `
[FINDING]
KEY: disk-full
SEVERITY: critical
CATEGORY: capacity
RESOURCE: local-lvm
RESOURCE_TYPE: storage
TITLE: Disk Full
DESCRIPTION: Storage is 95% full
RECOMMENDATION: Clean up
EVIDENCE: df output
[/FINDING]

[FINDING]
KEY: vm-down
SEVERITY: critical
CATEGORY: reliability
RESOURCE: 100
RESOURCE_TYPE: vm
TITLE: VM Down
DESCRIPTION: VM 100 is stopped
RECOMMENDATION: Start VM
EVIDENCE: status check
[/FINDING]
`,
			want: 2,
		},
		{
			name: "Malformed Blocks",
			response: `
[FINDING]
Incomplete block...
[/FINDING]

[FINDING]
KEY: valid-one
SEVERITY: info
CATEGORY: configuration
RESOURCE: test
RESOURCE_TYPE: vm
TITLE: Test
DESCRIPTION: A test
RECOMMENDATION: None
EVIDENCE: None
[/FINDING]
`,
			want: 1,
		},
		{
			name:     "No Findings",
			response: "Everything looks good.",
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := service.parseFindings(tt.response)
			assert.Len(t, findings, tt.want)
			if tt.want > 0 && tt.check != nil {
				tt.check(t, findings[0])
			}
		})
	}
}

func TestPatrolService_ParseFindings_BackupCategory(t *testing.T) {
	service := NewPatrolService(nil)

	response := `
[FINDING]
KEY: backup-stale
SEVERITY: warning
CATEGORY: backup
RESOURCE: vm-101
RESOURCE_TYPE: vm
TITLE: Backup stale
DESCRIPTION: No backup in 48 hours
RECOMMENDATION: Check backup jobs
EVIDENCE: Last backup: 2 days ago
[/FINDING]
`

	findings := service.parseFindings(response)
	require.Len(t, findings, 1)
	assert.Equal(t, "backup", findings[0].Category)
	assert.Equal(t, "Backup stale", findings[0].Title)
}

// MockFindingsStore
type MockFindingsStore struct {
	mock.Mock
}

func (m *MockFindingsStore) Add(finding *PatrolFinding) bool {
	args := m.Called(finding)
	return args.Bool(0)
}

func (m *MockFindingsStore) GetActive() []*PatrolFinding {
	args := m.Called()
	return args.Get(0).([]*PatrolFinding)
}

func (m *MockFindingsStore) GetDismissed() []*PatrolFinding {
	args := m.Called()
	return args.Get(0).([]*PatrolFinding)
}

func TestPatrolService_RunPatrol(t *testing.T) {
	// Setup dependencies
	mockStore := new(MockFindingsStore)
	mockProvider := new(MockProvider)

	// Create Service manually
	cfg := Config{
		AIConfig: &config.AIConfig{
			Enabled:      true,
			OpenAIAPIKey: "sk-test",
			ChatModel:    "openai:gpt-4",
		},
		StateProvider: &mockStateProvider{},
		DataDir:       t.TempDir(),
	}

	// Use NewService to initialize internal structures (like executor)
	chatService := NewService(cfg)

	// Manually inject missing pieces since we are bypassing Start()
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)
	chatService.sessions = store

	// Create AgenticLoop with our mock provider
	// We need a non-nil executor, which NewService created.
	chatService.agenticLoop = NewAgenticLoop(mockProvider, chatService.executor, "test-prompt")

	// Mark as started
	chatService.started = true

	// Create PatrolService
	patrolService := NewPatrolService(chatService)
	patrolService.SetFindingsStore(mockStore)

	// Mock Provider ChatStream
	responseContent := `
[FINDING]
KEY: test-issue
SEVERITY: warning
CATEGORY: performance
RESOURCE: db-01
RESOURCE_TYPE: container
TITLE: High Latency
DESCRIPTION: DB latency > 100ms
RECOMMENDATION: Check indexes
EVIDENCE: logs
[/FINDING]
`
	mockProvider.On("ChatStream", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		return true
	}), mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		callback := args.Get(2).(providers.StreamCallback)

		// Simulate content stream
		callback(providers.StreamEvent{
			Type: "content",
			Data: providers.ContentEvent{Text: responseContent},
		})

		// Simulate done
		callback(providers.StreamEvent{
			Type: "done",
			Data: providers.DoneEvent{StopReason: "end_turn"},
		})
	})

	// Expect finding to be stored
	mockStore.On("Add", mock.MatchedBy(func(f *PatrolFinding) bool {
		return f.Key == "test-issue" && f.ResourceID == "db-01"
	})).Return(true)

	// Run Patrol
	result, err := patrolService.RunPatrolWithResult(context.Background())
	require.NoError(t, err)

	// Verify
	assert.Equal(t, 1, result.NewFindings)
	assert.Len(t, result.Findings, 1)
	assert.Equal(t, "test-issue", result.Findings[0].Key)
	assert.Equal(t, result, patrolService.GetLastResult())

	mockProvider.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestPatrolService_SubscribeUnsubscribe(t *testing.T) {
	patrolService := NewPatrolService(nil)
	ch := patrolService.Subscribe()

	patrolService.broadcast(PatrolStreamEvent{Type: "content", Content: "hello"})
	select {
	case event := <-ch:
		assert.Equal(t, "content", event.Type)
		assert.Equal(t, "hello", event.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected to receive patrol stream event")
	}

	patrolService.Unsubscribe(ch)
	_, ok := <-ch
	assert.False(t, ok)
}

func TestPatrolService_RunPatrol_Error(t *testing.T) {
	patrolService := NewPatrolService(nil)
	err := patrolService.RunPatrol(context.Background())
	require.Error(t, err)
}
