package config

import (
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
)

func TestSaveComplexConfigs_ErrorPaths(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write error")}
	cp.SetFileSystem(mfs)

	// Test various Save* methods for error coverage

	t.Run("SaveAIConfig_Error", func(t *testing.T) {
		err := cp.SaveAIConfig(AIConfig{})
		assert.Error(t, err)
	})

	t.Run("SaveAIFindings_Error", func(t *testing.T) {
		err := cp.SaveAIFindings(map[string]*AIFindingRecord{})
		assert.Error(t, err)
	})

	t.Run("SaveAIUsageHistory_Error", func(t *testing.T) {
		err := cp.SaveAIUsageHistory([]AIUsageEventRecord{})
		assert.Error(t, err)
	})

	t.Run("SavePatrolRunHistory_Error", func(t *testing.T) {
		err := cp.SavePatrolRunHistory([]PatrolRunRecord{})
		assert.Error(t, err)
	})

	t.Run("SaveEnvTokenSuppressions_Error", func(t *testing.T) {
		err := cp.SaveEnvTokenSuppressions([]string{})
		assert.Error(t, err)
	})

	t.Run("SaveWebhooks_Error", func(t *testing.T) {
		err := cp.SaveWebhooks([]notifications.WebhookConfig{})
		assert.Error(t, err)
	})

	t.Run("SaveAppriseConfig_Error", func(t *testing.T) {
		err := cp.SaveAppriseConfig(notifications.AppriseConfig{})
		assert.Error(t, err)
	})

	t.Run("SaveAPITokens_Error", func(t *testing.T) {
		err := cp.SaveAPITokens([]APITokenRecord{})
		assert.Error(t, err)
	})

	t.Run("SaveEmailConfig_Error", func(t *testing.T) {
		err := cp.SaveEmailConfig(notifications.EmailConfig{})
		assert.Error(t, err)
	})
}

func TestSaveComplexConfigs_MkdirErrors(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, mkdirError: errors.New("mkdir error")}
	cp.SetFileSystem(mfs)

	t.Run("SaveAIUsageHistory_MkdirError", func(t *testing.T) {
		err := cp.SaveAIUsageHistory([]AIUsageEventRecord{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mkdir error")
	})

	t.Run("SavePatrolRunHistory_MkdirError", func(t *testing.T) {
		err := cp.SavePatrolRunHistory([]PatrolRunRecord{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mkdir error")
	})

	t.Run("SaveEnvTokenSuppressions_MkdirError", func(t *testing.T) {
		err := cp.SaveEnvTokenSuppressions([]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mkdir error")
	})

	t.Run("SaveAlertConfig_MkdirError", func(t *testing.T) {
		err := cp.SaveAlertConfig(alerts.AlertConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mkdir error")
	})
}
