package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/stretchr/testify/assert"
)

func TestLoadAIConfig_Branches(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	aiFile := filepath.Join(tempDir, "ai.enc")

	// 1. Decrupt error
	cm, _ := crypto.NewCryptoManagerAt(tempDir)
	cp.crypto = cm

	// Write too short data for AES-GCM
	_ = os.WriteFile(aiFile, []byte("too short"), 0600)

	_, err := cp.LoadAIConfig()
	assert.Error(t, err)

	// 2. Unmarshal error (valid crypto but invalid JSON)
	validCipher, _ := cm.Encrypt([]byte("not json"))
	_ = os.WriteFile(aiFile, validCipher, 0600)

	_, err = cp.LoadAIConfig()
	assert.Error(t, err)

	// 3. Migration branch (PatrolIntervalMinutes <= 0)
	// We use map to avoid omitempty
	validConfig := map[string]interface{}{
		"enabled":                 true,
		"patrol_interval_minutes": 0,
	}
	configData, _ := json.Marshal(validConfig)
	encryptedConfig, _ := cm.Encrypt(configData)
	_ = os.WriteFile(aiFile, encryptedConfig, 0600)

	settings, err := cp.LoadAIConfig()
	assert.NoError(t, err)
	assert.Equal(t, 360, settings.PatrolIntervalMinutes)
}

func TestLoadAIFindings_Branches(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	findingsFile := filepath.Join(tempDir, "ai_findings.json")

	// 1. Not Exists
	os.Remove(findingsFile)
	data, err := cp.LoadAIFindings()
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Empty(t, data.Findings)

	// 2. Unmarshal Error
	_ = os.WriteFile(findingsFile, []byte("not json"), 0600)
	data, err = cp.LoadAIFindings()
	assert.NoError(t, err)
	assert.Empty(t, data.Findings)

	// 3. Read Error (not IsNotExist)
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err = cp.LoadAIFindings()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestLoadAIUsageHistory_Branches(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	usageFile := filepath.Join(tempDir, "ai_usage_history.json")

	// 1. Not Exists
	os.Remove(usageFile)
	data, err := cp.LoadAIUsageHistory()
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Empty(t, data.Events)

	// 2. Unmarshal Error
	_ = os.WriteFile(usageFile, []byte("not json"), 0600)
	data, err = cp.LoadAIUsageHistory()
	assert.NoError(t, err)
	assert.Empty(t, data.Events)

	// 3. Read Error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err = cp.LoadAIUsageHistory()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestLoadPatrolRunHistory_Branches(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	patrolFile := filepath.Join(tempDir, "ai_patrol_runs.json")

	// 1. Not Exists
	os.Remove(patrolFile)
	data, err := cp.LoadPatrolRunHistory()
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Empty(t, data.Runs)

	// 2. Unmarshal Error
	_ = os.WriteFile(patrolFile, []byte("not json"), 0600)
	data, err = cp.LoadPatrolRunHistory()
	assert.NoError(t, err)
	assert.Empty(t, data.Runs)

	// 3. Read Error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err = cp.LoadPatrolRunHistory()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}
