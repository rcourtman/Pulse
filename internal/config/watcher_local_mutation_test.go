package config

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

// The inode event stream is deliberately absent. Local writers update both
// config and encrypted storage under Mu, before storage restores exact A.
func TestConfigWatcher_PollRestoresInventoryAfterLocalMutation(t *testing.T) {
	mutations := map[string]func(*Config){
		"create": func(c *Config) {
			c.UpsertAPIToken(APITokenRecord{ID: "local-addition", Name: "Created locally", Hash: auth.HashAPIToken("synthetic-local-addition"), Scopes: []string{ScopeMonitoringRead}, OrgID: "client-c"})
		},
		"rotate":            func(c *Config) { c.APITokens[1].Hash = auth.HashAPIToken("synthetic-local-rotation") },
		"delete":            func(c *Config) { c.RemoveAPIToken("saved-agent") },
		"scope-in-place":    func(c *Config) { c.APITokens[0].Scopes[0] = ScopeAgentReport },
		"org-ids":           func(c *Config) { c.APITokens[0].OrgIDs = []string{"client-c", "client-d"} },
		"metadata-in-place": func(c *Config) { c.APITokens[0].Metadata["owner_user_id"] = "changed-owner" },
		"expiry-in-place":   func(c *Config) { *c.APITokens[0].ExpiresAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		"name":              func(c *Config) { c.APITokens[0].Name = "Renamed locally" },
	}
	for name, mutate := range mutations {
		for _, incomplete := range []bool{false, true} {
			mode := "complete"
			if incomplete {
				mode = "after-incomplete"
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				f := newTokenDiscoveryFixture(t)
				Mu.Lock()
				mutate(f.cw.config)
				f.cw.config.SortAPITokens()
				err := f.cw.persistence.SaveAPITokens(f.cw.config.APITokens)
				Mu.Unlock()
				require.NoError(t, err)
				local := f.snapshot()
				require.NotEqual(t, f.initial, local)
				if incomplete {
					f.publish(t, []byte(" \n"), f.stamp)
					f.cw.reloadAPITokensIfChanged()
					require.Equal(t, local, f.snapshot(), "incomplete read replaced locally mutated authority")
					require.Zero(t, f.callbacks.Load())
					bytes, err := os.ReadFile(f.cw.apiTokensPath)
					require.NoError(t, err)
					require.Equal(t, " \n", string(bytes), "failed observation wrote replacement authority")
				}
				// Return the original ciphertext and timestamp, not an equivalent re-encoding.
				f.publish(t, f.original, f.stamp)
				for i := 0; i < 5; i++ {
					f.cw.reloadAPITokensIfChanged()
				}
				require.ElementsMatch(t, f.initial, f.snapshot(), "cached disk bytes concealed a local inventory mutation")
				require.True(t, f.valid("synthetic-discovery-monitor"))
				require.True(t, f.valid("synthetic-discovery-agent"))
				require.False(t, f.valid("synthetic-local-addition"), "restored inventory did not withdraw local addition")
				require.False(t, f.valid("synthetic-local-rotation"), "restored inventory did not withdraw rotated credential")
				require.Eventually(t, func() bool { return f.callbacks.Load() == 1 }, time.Second, time.Millisecond)
				bytes, err := os.ReadFile(f.cw.apiTokensPath)
				require.NoError(t, err)
				require.Equal(t, f.original, bytes, "discovery rewrote restored encrypted authority")
				f.publish(t, []byte("[]"), f.stamp)
				f.cw.reloadAPITokensIfChanged()
				require.Empty(t, f.snapshot())
				require.False(t, f.valid("synthetic-discovery-monitor"))
				require.False(t, f.valid("synthetic-discovery-agent"))
			})
		}
	}
}

func TestConfigWatcher_PollLoopWithdrawsLocalAdditionOnExactRestore(t *testing.T) {
	f := newTokenDiscoveryFixture(t)
	Mu.Lock()
	f.cw.config.UpsertAPIToken(APITokenRecord{ID: "local-addition", Hash: auth.HashAPIToken("synthetic-local-addition"), Scopes: []string{ScopeMonitoringRead}, OrgID: "client-c"})
	err := f.cw.persistence.SaveAPITokens(f.cw.config.APITokens)
	Mu.Unlock()
	require.NoError(t, err)
	require.True(t, f.valid("synthetic-local-addition"))
	f.publish(t, f.original, f.stamp)
	f.poll() // The actual event-free loop, not a forced reload.
	require.Eventually(t, func() bool { return !f.valid("synthetic-local-addition") }, time.Second, time.Millisecond)
	require.ElementsMatch(t, f.initial, f.snapshot())
}

type countTokenSnapshotReads struct {
	FileSystem
	target string
	reads  atomic.Int32
}

func (fs *countTokenSnapshotReads) ReadFile(name string) ([]byte, error) {
	if name == fs.target {
		fs.reads.Add(1)
	}
	return fs.FileSystem.ReadFile(name)
}

func TestConfigWatcher_UsageOnlyPollsStayQuiet(t *testing.T) {
	f := newTokenDiscoveryFixture(t)
	fs := &countTokenSnapshotReads{FileSystem: f.cw.persistence.fs, target: f.cw.apiTokensPath}
	f.cw.persistence.fs = fs
	Mu.Lock()
	_, ok := f.cw.config.ValidateAPIToken("synthetic-discovery-monitor")
	used := *f.cw.config.APITokens[0].LastUsedAt
	Mu.Unlock()
	require.True(t, ok)
	for i := 0; i < 5; i++ {
		f.cw.reloadAPITokensIfChanged()
	}
	require.Zero(t, fs.reads.Load(), "request-local LastUsedAt triggered a decoded snapshot")
	require.Zero(t, f.callbacks.Load(), "request-local LastUsedAt rebuilt bindings")
	require.Equal(t, used, *f.snapshot()[0].LastUsedAt, "poll rolled back request-local usage")
	bytes, err := os.ReadFile(f.cw.apiTokensPath)
	require.NoError(t, err)
	require.Equal(t, f.original, bytes)
}
