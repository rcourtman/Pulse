package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// 4. Plaintext migration rewrite
	plaintextConfig := map[string]interface{}{
		"enabled":                 true,
		"model":                   "openai/gpt-4o-mini",
		"patrol_interval_minutes": 360,
		"openai_api_key":          "secret-key",
	}
	raw, _ := json.Marshal(plaintextConfig)
	_ = os.WriteFile(aiFile, raw, 0o600)

	settings, err = cp.LoadAIConfig()
	assert.NoError(t, err)
	assert.Equal(t, "openai/gpt-4o-mini", settings.Model)

	rewritten, err := os.ReadFile(aiFile)
	assert.NoError(t, err)
	assert.False(t, bytes.Equal(rewritten, raw))
	assert.NotContains(t, string(rewritten), "secret-key")
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

func TestLoadAIFindings_MigratesPlaintextFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	findingsFile := filepath.Join(tempDir, "ai_findings.json")

	plaintext := AIFindingsData{
		Version:   3,
		Findings:  map[string]*AIFindingRecord{},
		LastSaved: time.Now().UTC(),
	}
	raw, err := json.Marshal(plaintext)
	if err != nil {
		t.Fatalf("marshal plaintext findings: %v", err)
	}
	if err := os.WriteFile(findingsFile, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := cp.LoadAIFindings()
	if err != nil {
		t.Fatalf("LoadAIFindings: %v", err)
	}
	assert.NotNil(t, data)

	rewritten, err := os.ReadFile(findingsFile)
	if err != nil {
		t.Fatalf("ReadFile rewritten findings: %v", err)
	}
	if bytes.Equal(rewritten, raw) {
		t.Fatalf("expected plaintext findings file to be rewritten encrypted")
	}
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

func TestLoadAIUsageHistory_MigratesPlaintextFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	usageFile := filepath.Join(tempDir, "ai_usage_history.json")

	plaintext := AIUsageHistoryData{
		Version:   1,
		Events:    []AIUsageEventRecord{},
		LastSaved: time.Now().UTC(),
	}
	raw, err := json.Marshal(plaintext)
	if err != nil {
		t.Fatalf("marshal plaintext usage history: %v", err)
	}
	if err := os.WriteFile(usageFile, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := cp.LoadAIUsageHistory()
	if err != nil {
		t.Fatalf("LoadAIUsageHistory: %v", err)
	}
	assert.NotNil(t, data)

	rewritten, err := os.ReadFile(usageFile)
	if err != nil {
		t.Fatalf("ReadFile rewritten usage history: %v", err)
	}
	if bytes.Equal(rewritten, raw) {
		t.Fatalf("expected plaintext usage history file to be rewritten encrypted")
	}
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

func TestLoadPatrolRunHistory_MigratesPlaintextFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	patrolFile := filepath.Join(tempDir, "ai_patrol_runs.json")

	plaintext := PatrolRunHistoryData{
		Version:   1,
		Runs:      []PatrolRunRecord{},
		LastSaved: time.Now().UTC(),
	}
	raw, err := json.Marshal(plaintext)
	if err != nil {
		t.Fatalf("marshal plaintext patrol history: %v", err)
	}
	if err := os.WriteFile(patrolFile, raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, err := cp.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory: %v", err)
	}
	assert.NotNil(t, data)

	rewritten, err := os.ReadFile(patrolFile)
	if err != nil {
		t.Fatalf("ReadFile rewritten patrol history: %v", err)
	}
	if bytes.Equal(rewritten, raw) {
		t.Fatalf("expected plaintext patrol history file to be rewritten encrypted")
	}
}
