package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

// Exercise the actual archive, not a fabricated ExportData value: the migration
// warnings must match what a fresh destination can recover with a different key.
func TestConfigurationMigrationArchiveScope(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	sourceDir, targetDir := t.TempDir(), t.TempDir()
	source, target := NewConfigPersistence(sourceDir), NewConfigPersistence(targetDir)
	require.NoError(t, source.EnsureConfigDir())
	require.NoError(t, target.EnsureConfigDir())

	nodes := []PVEInstance{{Name: "synthetic-pve", Host: "https://pve.example:8006", TokenName: "pulse@pve!read", TokenValue: "synthetic-node-secret"}}
	require.NoError(t, source.SaveNodesConfig(nodes, nil, nil))
	expires := time.Now().Add(time.Hour).UTC()
	tokens := []APITokenRecord{{ID: "synthetic-agent-token", Name: "agent", Hash: auth.HashAPIToken("synthetic-agent-secret"), CreatedAt: time.Now().UTC(), ExpiresAt: &expires, Scopes: []string{ScopeAgentReport}, OrgID: "default"}}
	require.NoError(t, source.SaveAPITokens(tokens))
	sso := &SSOConfig{Providers: []SSOProvider{{ID: "synthetic-sso", Name: "SSO", Type: SSOProviderTypeOIDC, Enabled: true, OIDC: &OIDCProviderConfig{IssuerURL: "https://idp.example", ClientID: "synthetic", ClientSecret: "synthetic-sso-secret"}}}}
	require.NoError(t, source.SaveSSOConfig(sso))
	sourceNAS := []TrueNASInstance{{Name: "source-nas", Host: "https://nas.example", APIKey: "synthetic-nas-secret"}}
	require.NoError(t, source.SaveTrueNASConfig(sourceNAS))
	targetNAS := []TrueNASInstance{{Name: "target-nas", Host: "https://target.example", APIKey: "synthetic-target-secret"}}
	require.NoError(t, target.SaveTrueNASConfig(targetNAS))

	// Excluded state exists at both ends. Import must not pretend to transfer it
	// or overwrite the destination's excluded files. These are inert sentinels,
	// not live databases, credentials or operational agent records.
	excluded := []string{".env", "metrics.db", "audit.db", "host_agents.json", "agent_profiles.json", "rbac_roles.json", "license.enc"}
	for _, name := range excluded {
		require.NoError(t, os.WriteFile(filepath.Join(sourceDir, name), []byte("synthetic-source-only"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(targetDir, name), []byte("synthetic-destination-only"), 0600))
	}
	sourceKey, err := os.ReadFile(filepath.Join(sourceDir, ".encryption.key"))
	require.NoError(t, err)
	targetKey, err := os.ReadFile(filepath.Join(targetDir, ".encryption.key"))
	require.NoError(t, err)
	require.False(t, bytes.Equal(sourceKey, targetKey), "fresh destinations must use a different key")

	const passphrase = "synthetic-migration-passphrase"
	archive, err := source.ExportConfig(passphrase)
	require.NoError(t, err)
	ciphertext, err := base64.StdEncoding.DecodeString(archive)
	require.NoError(t, err)
	plaintext, err := decryptWithPassphrase(ciphertext, passphrase)
	require.NoError(t, err)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(plaintext, &payload))
	require.Equal(t, "4.4", stringValue(t, payload["version"]))
	// Changing archive scope requires updating the actual user-facing account,
	// not silently assuming every persisted store is in a configuration export.
	require.ElementsMatch(t, []string{"version", "exportedAt", "nodes", "alerts", "alertIntentPolicies", "email", "webhooks", "apprise", "deadMan", "system", "sso", "apiTokens"}, rawKeys(payload))
	require.NotContains(t, string(plaintext), "synthetic-nas-secret")
	require.NotContains(t, string(plaintext), "synthetic-source-only")
	require.Contains(t, string(plaintext), "synthetic-sso-secret")

	baseline := []PVEInstance{{Name: "destination-before", Host: "https://before.example:8006"}}
	require.NoError(t, target.SaveNodesConfig(baseline, nil, nil))
	require.Error(t, target.ImportConfig(archive, "wrong-synthetic-passphrase"))
	unchanged, err := target.LoadNodesConfig()
	require.NoError(t, err)
	require.Equal(t, baseline[0].Name, unchanged.PVEInstances[0].Name)

	require.NoError(t, target.ImportConfig(archive, passphrase))
	reopened := NewConfigPersistence(targetDir)
	restored, err := reopened.LoadNodesConfig()
	require.NoError(t, err)
	require.Equal(t, nodes[0].TokenValue, restored.PVEInstances[0].TokenValue)
	require.Equal(t, nodes[0].Name, restored.PVEInstances[0].Name)
	restoredSSO, err := reopened.LoadSSOConfig()
	require.NoError(t, err)
	require.Equal(t, sso.Providers[0].OIDC.ClientSecret, restoredSSO.Providers[0].OIDC.ClientSecret)
	restoredTokens, err := reopened.LoadAPITokens()
	require.NoError(t, err)
	cfg := &Config{APITokens: restoredTokens}
	record, ok := cfg.ValidateAPIToken("synthetic-agent-secret")
	require.True(t, ok, "restored records validate the existing credential without replacing it")
	require.Equal(t, tokens[0].ID, record.ID)
	require.Equal(t, tokens[0].Scopes, record.Scopes)
	require.WithinDuration(t, expires, *record.ExpiresAt, time.Millisecond)
	nas, err := reopened.LoadTrueNASConfig()
	require.NoError(t, err)
	require.Equal(t, targetNAS[0].Name, nas[0].Name)
	for _, name := range excluded {
		data, err := os.ReadFile(filepath.Join(targetDir, name))
		require.NoError(t, err)
		require.Equal(t, "synthetic-destination-only", string(data), name)
	}
	keyAfter, err := os.ReadFile(filepath.Join(targetDir, ".encryption.key"))
	require.NoError(t, err)
	require.True(t, bytes.Equal(targetKey, keyAfter), "configuration import keeps the destination key")
}

func rawKeys(payload map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	return keys
}

func stringValue(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var value string
	require.NoError(t, json.Unmarshal(raw, &value))
	return value
}
