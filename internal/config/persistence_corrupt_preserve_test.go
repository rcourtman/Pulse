package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readPreservedCopies returns the contents of every <path>.corrupt-* file.
func readPreservedCopies(t *testing.T, path string) [][]byte {
	t.Helper()

	matches, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)

	var contents [][]byte
	for _, match := range matches {
		data, err := os.ReadFile(match)
		require.NoError(t, err)
		contents = append(contents, data)
	}
	return contents
}

// An unparseable findings file must be moved aside before the loader reports
// empty data, so a subsequent save cannot destroy the suppression rules it
// still holds.
func TestLoadAIFindings_CorruptFilePreservedBeforeRewrite(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())
	logs := captureConfigLogs(t)

	corrupt := []byte("{this is not json")
	require.NoError(t, os.WriteFile(cp.aiFindingsFile, corrupt, 0600))

	loaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	assert.Empty(t, loaded.Findings)
	assert.Empty(t, loaded.SuppressionRules)

	_, statErr := os.Stat(cp.aiFindingsFile)
	assert.True(t, os.IsNotExist(statErr), "corrupt file must be moved aside, not left in place")
	preserved := readPreservedCopies(t, cp.aiFindingsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, corrupt, preserved[0])
	assert.Contains(t, logs.String(), "preserved for recovery")

	// The save that previously clobbered the corrupt file now writes a fresh
	// file while the preserved copy keeps the original bytes.
	require.NoError(t, cp.SaveAIFindings(map[string]*AIFindingRecord{"f1": {}}))

	reloaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	assert.Len(t, reloaded.Findings, 1)

	preserved = readPreservedCopies(t, cp.aiFindingsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, corrupt, preserved[0])
}

// Same contract for chat sessions: the corrupt file holds user conversations,
// so SaveAIChatSession must not rewrite it before it is moved aside.
func TestLoadAIChatSessions_CorruptFilePreservedBeforeRewrite(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())
	logs := captureConfigLogs(t)

	corrupt := []byte("{this is not json")
	require.NoError(t, os.WriteFile(cp.aiChatSessionsFile, corrupt, 0600))

	loaded, err := cp.LoadAIChatSessions()
	require.NoError(t, err)
	assert.Empty(t, loaded.Sessions)

	_, statErr := os.Stat(cp.aiChatSessionsFile)
	assert.True(t, os.IsNotExist(statErr), "corrupt file must be moved aside, not left in place")
	preserved := readPreservedCopies(t, cp.aiChatSessionsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, corrupt, preserved[0])
	assert.Contains(t, logs.String(), "preserved for recovery")

	require.NoError(t, cp.SaveAIChatSession(&AIChatSession{ID: "s1", Title: "hello"}))

	reloaded, err := cp.LoadAIChatSessions()
	require.NoError(t, err)
	assert.Len(t, reloaded.Sessions, 1)

	preserved = readPreservedCopies(t, cp.aiChatSessionsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, corrupt, preserved[0])
}

// A file encrypted with a different key is valid JSON underneath but
// undecryptable here: Decrypt fails, the ciphertext fails to parse as JSON,
// and the file must be preserved exactly like an unparseable one.
func TestLoadAIFindings_UndecryptableFilePreserved(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	otherCrypto, err := crypto.NewCryptoManagerAt(t.TempDir())
	require.NoError(t, err)
	ciphertext, err := otherCrypto.Encrypt([]byte(`{"version":4,"findings":{},"suppression_rules":{"r1":{"id":"r1"}}}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cp.aiFindingsFile, ciphertext, 0600))

	loaded, err := cp.LoadAIFindings()
	require.NoError(t, err)
	assert.Empty(t, loaded.Findings)

	preserved := readPreservedCopies(t, cp.aiFindingsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, ciphertext, preserved[0])

	require.NoError(t, cp.SaveAIFindings(map[string]*AIFindingRecord{"f1": {}}))
	preserved = readPreservedCopies(t, cp.aiFindingsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, ciphertext, preserved[0])
}

func TestLoadAIChatSessions_UndecryptableFilePreserved(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	otherCrypto, err := crypto.NewCryptoManagerAt(t.TempDir())
	require.NoError(t, err)
	ciphertext, err := otherCrypto.Encrypt([]byte(`{"version":1,"sessions":{"s1":{"id":"s1"}}}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cp.aiChatSessionsFile, ciphertext, 0600))

	loaded, err := cp.LoadAIChatSessions()
	require.NoError(t, err)
	assert.Empty(t, loaded.Sessions)

	preserved := readPreservedCopies(t, cp.aiChatSessionsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, ciphertext, preserved[0])

	require.NoError(t, cp.SaveAIChatSession(&AIChatSession{ID: "s2"}))
	preserved = readPreservedCopies(t, cp.aiChatSessionsFile)
	require.Len(t, preserved, 1)
	assert.Equal(t, ciphertext, preserved[0])
}

// If the corrupt file cannot be moved aside, the loader must fail rather than
// report empty data, so read-modify-write savers abort instead of clobbering.
func TestLoadAI_CorruptFilePreserveFailureAbortsSaves(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	corrupt := []byte("{this is not json")
	require.NoError(t, os.WriteFile(cp.aiFindingsFile, corrupt, 0600))
	require.NoError(t, os.WriteFile(cp.aiChatSessionsFile, corrupt, 0600))

	cp.SetFileSystem(&mockFSError{FileSystem: defaultFileSystem{}, renameError: errors.New("disk full")})

	_, err := cp.LoadAIFindings()
	assert.ErrorContains(t, err, "disk full")
	assert.ErrorContains(t, cp.SaveAIFindings(map[string]*AIFindingRecord{"f1": {}}), "disk full")

	_, err = cp.LoadAIChatSessions()
	assert.ErrorContains(t, err, "disk full")
	assert.ErrorContains(t, cp.SaveAIChatSession(&AIChatSession{ID: "s1"}), "disk full")
	assert.ErrorContains(t, cp.DeleteAIChatSession("s1"), "disk full")
	_, err = cp.CleanupOldAIChatSessions(time.Hour)
	assert.ErrorContains(t, err, "disk full")

	// Both corrupt files survive untouched.
	data, err := os.ReadFile(cp.aiFindingsFile)
	require.NoError(t, err)
	assert.Equal(t, corrupt, data)
	data, err = os.ReadFile(cp.aiChatSessionsFile)
	require.NoError(t, err)
	assert.Equal(t, corrupt, data)
}
