package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveAIConfig_NoCrypto(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	cp.crypto = nil

	err := cp.SaveAIConfig(AIConfig{Enabled: true})
	assert.NoError(t, err)
}

func TestSaveWebhooks_NoCrypto(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	cp.crypto = nil

	err := cp.SaveWebhooks(nil)
	assert.NoError(t, err)
}
