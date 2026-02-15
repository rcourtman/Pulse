package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadNodesConfig_Recovery_RealCrypto(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	nodesFile := filepath.Join(tempDir, "nodes.enc")
	backupFile := nodesFile + ".backup"

	// Create a real crypto manager
	cm, err := crypto.NewCryptoManagerAt(tempDir)
	require.NoError(t, err)
	cp.crypto = cm

	// 1. Decryption failure (invalid data) with NO backup
	_ = os.WriteFile(nodesFile, []byte("too short"), 0600)

	nodes, err := cp.LoadNodesConfig()
	assert.NoError(t, err) // Returns empty config on critical failure
	assert.Empty(t, nodes.PVEInstances)
	// Verify corrupted file moved
	matches, _ := filepath.Glob(nodesFile + ".corrupted-*")
	assert.NotEmpty(t, matches)

	// 2. Decryption failure (invalid data) with corrupted backup
	_ = os.WriteFile(nodesFile, []byte("too short data"), 0600)
	_ = os.WriteFile(backupFile, []byte("too short backup"), 0600)

	nodes, err = cp.LoadNodesConfig()
	assert.NoError(t, err)
	assert.Empty(t, nodes.PVEInstances)

	// 3. Decryption failure with VALID backup
	validConfig := NodesConfig{PVEInstances: []PVEInstance{{Host: "valid"}}}
	validData, _ := json.Marshal(validConfig)
	encryptedValid, _ := cm.Encrypt(validData)

	_ = os.WriteFile(nodesFile, []byte("too short again"), 0600)
	_ = os.WriteFile(backupFile, encryptedValid, 0600)

	nodes, err = cp.LoadNodesConfig()
	assert.NoError(t, err)
	assert.Equal(t, "https://valid:8006", nodes.PVEInstances[0].Host)
}
