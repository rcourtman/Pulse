package api

import (
	"context"
	"testing"

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
	h := newTestAIHandler(nil, mockPersist, nil)
	// We need h.legacyService to be non-nil for the Restart check to proceed past first nil check
	// But it must return IsRunning() = false
	h.legacyService = mockSvc

	// Config allows enabling
	aiCfg := &config.AIConfig{Enabled: true}
	mockPersist.On("LoadAIConfig").Return(aiCfg, nil)

	// Service is NOT running
	mockSvc.On("IsRunning").Return(false)

	// Start should be called because Enabled=true
	mockSvc.On("Start", mock.Anything).Return(nil)

	err := h.Restart(context.Background())
	assert.NoError(t, err)

	mockSvc.AssertExpectations(t)
	mockPersist.AssertExpectations(t)
}
