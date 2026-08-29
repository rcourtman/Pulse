package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeLegacyEncryptedAPITokens(t *testing.T, persistence *ConfigPersistence, tokens []APITokenRecord) []byte {
	t.Helper()

	data, err := json.Marshal(tokens)
	require.NoError(t, err)
	if persistence.crypto != nil {
		data, err = persistence.crypto.Encrypt(data)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(persistence.apiTokensFile, data, 0o600))
	return data
}

func TestLoadAPITokensPersistsCanonicalAuthorizationStateBeforeReturning(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	createdAt := time.Now().UTC().Add(-time.Hour)
	writeLegacyEncryptedAPITokens(t, persistence, []APITokenRecord{{
		Name:      "legacy agent",
		Hash:      "legacy-hash",
		CreatedAt: createdAt,
		Scopes:    []string{"host-agent:report"},
	}})

	loaded, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.NotEmpty(t, loaded[0].ID)
	require.Equal(t, []string{ScopeAgentReport}, loaded[0].Scopes)
	require.Equal(t, "default", loaded[0].OrgID)

	reloaded, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, reloaded, 1)
	require.Equal(t, loaded[0].ID, reloaded[0].ID, "the persisted revocation handle must be stable")
	require.Equal(t, loaded[0].Scopes, reloaded[0].Scopes)
	require.Equal(t, loaded[0].OrgID, reloaded[0].OrgID)
}

func TestLoadAPITokensRejectsUnpersistedCanonicalAuthorizationState(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	original := writeLegacyEncryptedAPITokens(t, persistence, []APITokenRecord{{
		Name:      "legacy agent",
		Hash:      "legacy-hash",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		Scopes:    []string{"host-agent:report"},
	}})
	persistence.SetFileSystem(&mockFSError{
		FileSystem: defaultFileSystem{},
		writeError: errors.New("forced migration write failure"),
	})

	loaded, err := persistence.LoadAPITokens()
	require.Error(t, err)
	require.Nil(t, loaded, "unpersisted authorization state must not be admitted")
	require.Contains(t, err.Error(), "persist canonical api tokens")

	after, readErr := os.ReadFile(persistence.apiTokensFile)
	require.NoError(t, readErr)
	require.True(t, bytes.Equal(original, after), "failed migration must leave the durable inventory unchanged")
}

func TestConfigWatcherPreservesLiveTokensWhenCanonicalMigrationCannotPersist(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())
	writeLegacyEncryptedAPITokens(t, persistence, []APITokenRecord{{
		Name:      "disk legacy token",
		Hash:      "disk-hash",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		Scopes:    []string{"host-agent:report"},
	}})
	persistence.SetFileSystem(&mockFSError{
		FileSystem: defaultFileSystem{},
		writeError: errors.New("forced migration write failure"),
	})

	live := APITokenRecord{
		ID:        "live-token-id",
		Name:      "live token",
		Hash:      "live-hash",
		CreatedAt: time.Now().UTC(),
		Scopes:    []string{ScopeWildcard},
		OrgID:     "default",
	}
	cfg := &Config{APITokens: []APITokenRecord{live}}
	cfg.SortAPITokens()
	cw := &ConfigWatcher{
		config:        cfg,
		apiTokensPath: persistence.apiTokensFile,
		persistence:   persistence,
		stopChan:      make(chan struct{}),
	}
	var callbackCalled atomic.Bool
	cw.SetAPITokenReloadCallback(func() { callbackCalled.Store(true) })

	cw.reloadAPITokens()

	require.Equal(t, []APITokenRecord{live}, cfg.APITokens)
	require.Equal(t, live.Hash, cfg.APIToken)
	require.False(t, callbackCalled.Load(), "failed reload must not advertise a credential change")
}
