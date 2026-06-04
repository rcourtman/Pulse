package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRestart_StartIfStopped(t *testing.T) {
	// Mock newChatService factory
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	mockSvc := new(MockAIService)
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
	}

	mockPersist := new(MockAIPersistence)
	mockPersist.dataDir = t.TempDir()
	h := newTestAIHandler(nil, mockPersist, nil)
	// We need h.defaultService to be non-nil for the Restart check to proceed past first nil check
	// But it must return IsRunning() = false
	h.defaultService = mockSvc
	prevStore := approval.GetStore()
	t.Cleanup(func() {
		h.clearApprovalStore()
		approval.SetStore(prevStore)
	})

	// Config allows enabling
	aiCfg := &config.AIConfig{Enabled: true}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)

	// Service is NOT running
	mockSvc.On("IsRunning").Return(false)

	// Start should be called because Enabled=true
	mockSvc.On("Start", mock.Anything).Return(nil)

	err := h.Restart(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, approval.GetStore())

	mockSvc.AssertExpectations(t)
	mockPersist.AssertExpectations(t)
}

func TestRestart_StartIfServiceMissing(t *testing.T) {
	oldNewService := newChatService
	defer func() { newChatService = oldNewService }()

	mockSvc := new(MockAIService)
	newChatService = func(cfg chat.Config) AIService {
		return mockSvc
	}

	mockPersist := new(MockAIPersistence)
	mockPersist.dataDir = t.TempDir()
	h := newTestAIHandler(nil, mockPersist, nil)
	prevStore := approval.GetStore()
	t.Cleanup(func() {
		h.clearApprovalStore()
		approval.SetStore(prevStore)
	})

	aiCfg := &config.AIConfig{Enabled: true}
	// Restart loads the config once and passes it to startWithConfig, so the
	// start path no longer re-loads it (the point of the fix). One load total.
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil).Once()
	mockSvc.On("Start", mock.Anything).Return(nil)

	err := h.Restart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, mockSvc, h.defaultService)
	assert.NotNil(t, approval.GetStore())

	mockSvc.AssertExpectations(t)
	mockPersist.AssertExpectations(t)
}
