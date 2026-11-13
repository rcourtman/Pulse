package config

import (
	"bytes"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

func unusableAPITokenInventories() []struct{ name, body string } {
	return []struct{ name, body string }{
		{"null row", `[null]`},
		{"empty record", `[{}]`},
		{"missing hash", `[{"id":"incomplete","name":"Incomplete","orgId":"client-a","scopes":["monitoring:read"]}]`},
		{"null hash", `[{"hash":null}]`},
		{"empty hash", `[{"hash":""}]`},
		{"whitespace hash", `[{"hash":" \t\r\n"}]`},
		{"valid and null", `[{"id":"valid","hash":"synthetic-existing-hash","orgId":"client-a","scopes":["monitoring:read"]},null]`},
		{"valid and incomplete", `[{"id":"valid","hash":"synthetic-existing-hash","orgId":"client-a","scopes":["monitoring:read"]},{"id":"incomplete"}]`},
	}
}

func writeAPITokenInventoryBytes(t *testing.T, p *ConfigPersistence, plain []byte, encrypted bool) []byte {
	t.Helper()
	data := append([]byte(nil), plain...)
	if encrypted {
		require.NotNil(t, p.crypto)
		var err error
		data, err = p.crypto.Encrypt(data)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(p.apiTokensFile, data, 0600))
	return data
}

func requireAPITokenFileUnchanged(t *testing.T, path string, want []byte, before os.FileInfo) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, got)
	after, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, os.SameFile(before, after))
	require.Equal(t, before.Mode(), after.Mode())
	require.Equal(t, before.ModTime(), after.ModTime())
}

func TestLoadAPITokensRejectsUnusableCredentialRecords(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		if encrypted {
			encoding = "encrypted"
		}
		for _, tc := range unusableAPITokenInventories() {
			t.Run(encoding+"/"+tc.name, func(t *testing.T) {
				p := NewConfigPersistence(t.TempDir())
				original := writeAPITokenInventoryBytes(t, p, []byte(tc.body), encrypted)
				before, err := os.Stat(p.apiTokensFile)
				require.NoError(t, err)
				tokens, fingerprint, err := p.loadAPITokenSnapshot(false)
				require.ErrorContains(t, err, "missing credential hash")
				require.Nil(t, tokens, "an unusable record is not a partial successful inventory")
				require.Empty(t, fingerprint, "failed admission advanced the consumed snapshot")
				require.NotContains(t, err.Error(), "synthetic-existing-hash")
				requireAPITokenFileUnchanged(t, p.apiTokensFile, original, before)
				_, err = os.Stat(p.apiTokensFile + ".backup")
				require.True(t, os.IsNotExist(err), "invalid admission migrated or backed up the file")
			})
		}
	}
}

func TestSaveAPITokensRejectsUnusableCredentialRecordsBeforeWriting(t *testing.T) {
	for _, hash := range []string{"", " \t\r\n"} {
		for _, mixed := range []bool{false, true} {
			name := "single"
			if mixed {
				name = "mixed"
			}
			t.Run(name+"/"+hash, func(t *testing.T) {
				p := NewConfigPersistence(t.TempDir())
				valid := APITokenRecord{ID: "saved", Hash: "synthetic-existing-hash", OrgID: "client-a", Scopes: []string{ScopeMonitoringRead}}
				require.NoError(t, p.SaveAPITokens([]APITokenRecord{valid}))
				require.NoError(t, p.SaveAPITokens([]APITokenRecord{valid}))
				original, err := os.ReadFile(p.apiTokensFile)
				require.NoError(t, err)
				backup, err := os.ReadFile(p.apiTokensFile + ".backup")
				require.NoError(t, err)
				before, err := os.Stat(p.apiTokensFile)
				require.NoError(t, err)
				backupBefore, err := os.Stat(p.apiTokensFile + ".backup")
				require.NoError(t, err)
				input := []APITokenRecord{{Hash: hash}}
				if mixed {
					input = append([]APITokenRecord{valid}, input...)
				}
				inputBefore, err := json.Marshal(input)
				require.NoError(t, err)
				err = p.SaveAPITokens(input)
				require.ErrorContains(t, err, "missing credential hash")
				require.NotContains(t, err.Error(), valid.Hash)
				requireAPITokenFileUnchanged(t, p.apiTokensFile, original, before)
				requireAPITokenFileUnchanged(t, p.apiTokensFile+".backup", backup, backupBefore)
				inputAfter, err := json.Marshal(input)
				require.NoError(t, err)
				require.True(t, bytes.Equal(inputBefore, inputAfter), "rejected save changed its caller's inventory")
			})
		}
	}
}

func TestConfigWatcherRetainsAuthorityThroughUnusableCredentialRecords(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		if encrypted {
			encoding = "encrypted"
		}
		for _, tc := range unusableAPITokenInventories() {
			t.Run(encoding+"/"+tc.name, func(t *testing.T) {
				p := NewConfigPersistence(t.TempDir())
				created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
				saved := []APITokenRecord{{ID: "saved-monitor", Hash: auth.HashAPIToken("synthetic-saved-monitor"), Name: "Saved monitoring", CreatedAt: created, ExpiresAt: &expires, Scopes: []string{ScopeMonitoringRead}, OrgID: "client-a", Metadata: map[string]string{"owner_user_id": "alice"}}}
				require.NoError(t, p.SaveAPITokens(saved))
				original, err := os.ReadFile(p.apiTokensFile)
				require.NoError(t, err)
				cfg := &Config{}
				cw := &ConfigWatcher{config: cfg, persistence: p, apiTokensPath: p.apiTokensFile, stopChan: make(chan struct{})}
				var callbacks atomic.Int32
				cw.SetAPITokenReloadCallback(func() { callbacks.Add(1) })
				cw.reloadAPITokens()
				require.Eventually(t, func() bool { return callbacks.Load() == 1 }, time.Second, time.Millisecond)
				oldFileHash, oldLiveHash := cw.apiTokensLastHash, cw.apiTokensLastInventoryHash
				broken := writeAPITokenInventoryBytes(t, p, []byte(tc.body), encrypted)
				before, err := os.Stat(p.apiTokensFile)
				require.NoError(t, err)
				cw.reloadAPITokensIfChanged()
				require.Equal(t, saved, cfg.APITokens, "failed observation replaced live authority")
				require.Equal(t, saved[0].Hash, cfg.APIToken)
				require.True(t, cfg.IsValidAPIToken("synthetic-saved-monitor"))
				require.Equal(t, oldFileHash, cw.apiTokensLastHash)
				require.Equal(t, oldLiveHash, cw.apiTokensLastInventoryHash)
				require.EqualValues(t, 1, callbacks.Load(), "failed observation rebuilt bindings")
				requireAPITokenFileUnchanged(t, p.apiTokensFile, broken, before)
				// Exact restoration is usable again; a later deliberate empty
				// inventory still withdraws the saved credential.
				require.NoError(t, os.WriteFile(p.apiTokensFile, original, 0600))
				cw.reloadAPITokens()
				require.Eventually(t, func() bool { return callbacks.Load() == 2 }, time.Second, time.Millisecond)
				require.Equal(t, saved, cfg.APITokens)
				writeAPITokenInventoryBytes(t, p, []byte("[]"), encrypted)
				cw.reloadAPITokensIfChanged()
				require.Eventually(t, func() bool { return callbacks.Load() == 3 }, time.Second, time.Millisecond)
				require.Empty(t, cfg.APITokens)
				require.Empty(t, cfg.APIToken)
				require.False(t, cfg.IsValidAPIToken("synthetic-saved-monitor"))
			})
		}
	}
}

func TestLoad_APITokensRejectsUnusableSavedCredentialRecords(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		encoding := "plaintext"
		if encrypted {
			encoding = "encrypted"
		}
		for _, tc := range unusableAPITokenInventories() {
			t.Run(encoding+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("PULSE_DATA_DIR", dir)
				t.Setenv("PULSE_AUTH_CONFIG_DIR", dir)
				p := NewConfigPersistence(dir)
				original := writeAPITokenInventoryBytes(t, p, []byte(tc.body), encrypted)
				before, err := os.Stat(p.apiTokensFile)
				require.NoError(t, err)
				cfg, err := LoadWithoutLoggingInit()
				require.ErrorContains(t, err, "load API tokens")
				require.ErrorContains(t, err, "missing credential hash")
				require.Nil(t, cfg)
				requireAPITokenFileUnchanged(t, p.apiTokensFile, original, before)
			})
		}
	}
}
