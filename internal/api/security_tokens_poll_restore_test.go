package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

type localMutationPollingFixture struct {
	router    *Router
	watcher   *config.ConfigWatcher
	file      string
	original  []byte
	stamp     time.Time
	callbacks atomic.Int32
}

func newLocalMutationPollingFixture(t *testing.T) *localMutationPollingFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_AUTH_CONFIG_DIR", dir)
	p := config.NewConfigPersistence(dir)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	initial := []config.APITokenRecord{{ID: "saved-monitor", Name: "Saved monitoring", Hash: auth.HashAPIToken("synthetic-saved-monitor"), Prefix: "synthetic", Suffix: "monitor", CreatedAt: created, ExpiresAt: &expires, Scopes: []string{config.ScopeMonitoringRead}, OrgID: "default", Metadata: map[string]string{"owner_user_id": "alice"}}}
	require.NoError(t, p.SaveAPITokens(initial))
	f := &localMutationPollingFixture{file: filepath.Join(dir, "api_tokens.json"), stamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	require.NoError(t, os.Chtimes(f.file, f.stamp, f.stamp))
	var err error
	f.original, err = os.ReadFile(f.file)
	require.NoError(t, err)
	loaded, err := p.LoadAPITokens()
	require.NoError(t, err)
	f.router = &Router{config: &config.Config{APITokens: loaded}, persistence: p}
	f.watcher, err = config.NewConfigWatcher(f.router.config)
	require.NoError(t, err)
	t.Cleanup(f.watcher.Stop)
	f.watcher.SetAPITokenReloadCallback(func() { f.callbacks.Add(1) })
	// Start while the inventory is absent: no inode event can deliver either
	// the subsequent local save or the restore. Use the production five-second poll.
	require.NoError(t, os.Rename(f.file, f.file+".parked"))
	require.NoError(t, f.watcher.Start())
	f.publish(t, f.original)
	require.Eventually(t, func() bool { return f.callbacks.Load() == 1 }, 8*time.Second, 10*time.Millisecond, "initial production poll never consumed A")
	return f
}

func (f *localMutationPollingFixture) publish(t *testing.T, data []byte) {
	t.Helper()
	tmp := f.file + ".restore"
	require.NoError(t, os.WriteFile(tmp, data, 0600))
	require.NoError(t, os.Chtimes(tmp, f.stamp, f.stamp))
	require.NoError(t, os.Rename(tmp, f.file))
}
func (f *localMutationPollingFixture) valid(raw string) bool {
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	return f.router.config.IsValidAPIToken(raw)
}

func TestSecurityTokens_PollWithdrawsLocallyCreatedTokenAfterRestore(t *testing.T) {
	f := newLocalMutationPollingFixture(t)
	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", nil)
	raw, record, err := f.router.createAPITokenRecord(req, "Local addition", []string{config.ScopeMonitoringRead}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.True(t, f.valid(raw))
	require.True(t, f.valid("synthetic-saved-monitor"))
	f.publish(t, f.original) // Exact A, before any poll can consume local B.
	require.Eventually(t, func() bool { return !f.valid(raw) }, 8*time.Second, 10*time.Millisecond, "poll left locally issued bearer valid after exact restoration")
	require.True(t, f.valid("synthetic-saved-monitor"))
	config.Mu.RLock()
	count := len(f.router.config.APITokens)
	id := f.router.config.APITokens[0].ID
	config.Mu.RUnlock()
	require.Equal(t, 1, count)
	require.Equal(t, "saved-monitor", id)
	require.Eventually(t, func() bool { return f.callbacks.Load() == 2 }, time.Second, time.Millisecond)
	// Follow the restore with real empty saved authority, not a display-only assertion.
	f.publish(t, []byte("[]"))
	require.Eventually(t, func() bool { return !f.valid("synthetic-saved-monitor") }, 8*time.Second, 10*time.Millisecond)
	require.False(t, f.valid(raw))
}
