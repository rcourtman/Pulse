package config

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

type tokenDiscoveryFixture struct {
	cw          *ConfigWatcher
	initial     []APITokenRecord
	restored    []APITokenRecord
	original    []byte
	replacement []byte
	stamp       time.Time
	callbacks   atomic.Int32
}

// This is a correctness observation budget, not a polling-latency SLO. A
// complete encrypted read/migration can finish after one second under race
// instrumentation; permanent missed restoration/withdrawal must still fail.
const tokenDiscoveryObservationBudget = 3 * time.Second

func newTokenDiscoveryFixture(t *testing.T) *tokenDiscoveryFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PULSE_AUTH_CONFIG_DIR", dir)
	p := NewConfigPersistence(dir)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	f := &tokenDiscoveryFixture{
		stamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		initial: []APITokenRecord{
			{ID: "saved-monitor", Hash: auth.HashAPIToken("synthetic-discovery-monitor"), Name: "Before", Scopes: []string{ScopeMonitoringRead}, OrgID: "client-a", CreatedAt: created, ExpiresAt: &expires, Metadata: map[string]string{"owner_user_id": "alice"}},
			{ID: "saved-agent", Hash: auth.HashAPIToken("synthetic-discovery-agent"), Name: "Retire", Scopes: []string{ScopeAgentReport}, OrgID: "client-b", CreatedAt: created, Metadata: map[string]string{"owner_user_id": "bob"}},
		},
	}
	f.restored = append([]APITokenRecord(nil), f.initial...)
	f.restored[0].Name = "After!" // Same length, so size/mtime alone cannot find this edit.
	require.NoError(t, p.SaveAPITokens(f.restored))
	var err error
	f.replacement, err = os.ReadFile(p.apiTokensFile)
	require.NoError(t, err)
	require.NoError(t, p.SaveAPITokens(f.initial))
	f.original, err = os.ReadFile(p.apiTokensFile)
	require.NoError(t, err)
	require.Equal(t, len(f.original), len(f.replacement))
	require.NoError(t, os.Chtimes(p.apiTokensFile, f.stamp, f.stamp))
	f.cw, err = NewConfigWatcher(&Config{APITokens: f.initial})
	require.NoError(t, err)
	t.Cleanup(f.cw.Stop)
	f.cw.pollInterval = 10 * time.Millisecond
	// Establish a successful read before the discovery under test. No recovery
	// below is driven by a direct reload; only the real event loop or poll runs.
	f.cw.reloadAPITokens()
	f.cw.SetAPITokenReloadCallback(func() { f.callbacks.Add(1) })
	return f
}

func (f *tokenDiscoveryFixture) poll() {
	f.cw.wg.Add(1)
	go func() {
		defer f.cw.wg.Done()
		f.cw.pollForChanges()
	}()
}

func (f *tokenDiscoveryFixture) publish(t *testing.T, contents []byte, stamp time.Time) {
	t.Helper()
	tmp := filepath.Join(filepath.Dir(f.cw.apiTokensPath), "restored-inventory.tmp")
	require.NoError(t, os.WriteFile(tmp, contents, 0600))
	require.NoError(t, os.Chtimes(tmp, stamp, stamp))
	require.NoError(t, os.Rename(tmp, f.cw.apiTokensPath))
}

func (f *tokenDiscoveryFixture) snapshot() []APITokenRecord {
	Mu.RLock()
	defer Mu.RUnlock()
	return append([]APITokenRecord(nil), f.cw.config.APITokens...)
}

func (f *tokenDiscoveryFixture) valid(raw string) bool {
	Mu.RLock()
	defer Mu.RUnlock()
	return f.cw.config.IsValidAPIToken(raw)
}

func (f *tokenDiscoveryFixture) awaitRestored(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, record := range f.snapshot() {
			if record.ID == "saved-monitor" {
				return record.Name == "After!"
			}
		}
		return false
	}, tokenDiscoveryObservationBudget, 10*time.Millisecond, "automatic discovery missed complete inventory with preserved/older timestamp")
	require.ElementsMatch(t, f.restored, f.snapshot(), "discovery changed saved authority")
	require.True(t, f.valid("synthetic-discovery-monitor"))
	require.True(t, f.valid("synthetic-discovery-agent"))
	require.False(t, f.valid("synthetic-unissued"))
	require.Eventually(t, func() bool { return f.callbacks.Load() > 0 }, tokenDiscoveryObservationBudget, time.Millisecond)
}

func (f *tokenDiscoveryFixture) awaitRevoked(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool { return len(f.snapshot()) == 0 }, tokenDiscoveryObservationBudget, 10*time.Millisecond,
		"automatic discovery missed explicit withdrawal after restoration")
	require.False(t, f.valid("synthetic-discovery-monitor"))
	require.False(t, f.valid("synthetic-discovery-agent"))
}

func TestConfigWatcher_PollDiscoversPreservedAPITokens(t *testing.T) {
	for _, mode := range []string{"newer", "same", "older", "same-inode-and-size"} {
		t.Run(mode, func(t *testing.T) {
			f := newTokenDiscoveryFixture(t)
			stamp := f.stamp
			if mode == "older" {
				stamp = stamp.Add(-time.Hour)
			}
			if mode == "newer" {
				stamp = stamp.Add(time.Hour)
			}
			if mode == "same-inode-and-size" {
				before, err := os.Stat(f.cw.apiTokensPath)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(f.cw.apiTokensPath, f.replacement, 0600))
				require.NoError(t, os.Chtimes(f.cw.apiTokensPath, stamp, stamp))
				after, err := os.Stat(f.cw.apiTokensPath)
				require.NoError(t, err)
				require.True(t, os.SameFile(before, after))
				require.Equal(t, before.Size(), after.Size())
				require.Equal(t, before.ModTime(), after.ModTime())
			} else {
				require.NoError(t, os.Remove(f.cw.apiTokensPath))
				f.publish(t, f.replacement, stamp)
			}
			f.poll() // Deliberately no fsnotify events: the fallback must be sufficient.
			f.awaitRestored(t)
			for _, empty := range []string{"[]", "null"} {
				f.publish(t, []byte(empty), stamp)
				f.awaitRevoked(t)
				f.publish(t, f.replacement, stamp)
				f.awaitRestored(t)
			}
		})
	}
}

type missingDiscoveryFileSystem struct {
	FileSystem
	target  string
	missing atomic.Int32
}

func (fs *missingDiscoveryFileSystem) ReadFile(name string) ([]byte, error) {
	data, err := fs.FileSystem.ReadFile(name)
	if name == fs.target && os.IsNotExist(err) {
		fs.missing.Add(1)
	}
	return data, err
}

func TestConfigWatcher_LiveDiscoversRestoredAPITokens(t *testing.T) {
	for _, mode := range []string{"same", "older"} {
		t.Run(mode, func(t *testing.T) {
			f := newTokenDiscoveryFixture(t)
			fs := &missingDiscoveryFileSystem{FileSystem: f.cw.persistence.fs, target: f.cw.apiTokensPath}
			f.cw.persistence.fs = fs
			require.NoError(t, f.cw.Start())
			require.NoError(t, os.Remove(f.cw.apiTokensPath))
			require.Eventually(t, func() bool {
				return len(f.cw.watcher.WatchList()) == 0 && fs.missing.Load() >= 3
			}, 2*time.Second, 10*time.Millisecond, "real removal event was not consumed")
			f.cw.mu.Lock() // Wait for the incomplete read's retries to finish.
			f.cw.mu.Unlock()
			require.ElementsMatch(t, f.initial, f.snapshot())
			require.Zero(t, f.callbacks.Load(), "missing storage rebuilt bindings")
			stamp := f.stamp
			if mode == "older" {
				stamp = stamp.Add(-time.Hour)
			}
			f.publish(t, f.replacement, stamp)
			f.awaitRestored(t)
			f.publish(t, []byte("[]"), stamp)
			f.awaitRevoked(t)
		})
	}
}

func TestConfigWatcher_PollRetriesIncompleteAPITokens(t *testing.T) {
	f := newTokenDiscoveryFixture(t)
	f.publish(t, []byte(" \n"), f.stamp)
	f.poll()
	time.Sleep(250 * time.Millisecond) // More than one poll and a complete bounded retry.
	require.ElementsMatch(t, f.initial, f.snapshot())
	require.Zero(t, f.callbacks.Load())
	bytes, err := os.ReadFile(f.cw.apiTokensPath)
	require.NoError(t, err)
	require.Equal(t, " \n", string(bytes), "incomplete polling wrote replacement authority")
	f.publish(t, f.replacement, f.stamp)
	f.awaitRestored(t)
}

func TestConfigWatcher_PollUsesLoadedAPITokenSnapshot(t *testing.T) {
	f := newTokenDiscoveryFixture(t)
	fs := &blockingTokenReadFileSystem{FileSystem: f.cw.persistence.fs, target: f.cw.apiTokensPath, readStarted: make(chan struct{}), releaseRead: make(chan struct{})}
	f.cw.persistence.fs = fs
	done := make(chan struct{})
	go func() { f.cw.reloadAPITokens(); close(done) }()
	<-fs.readStarted
	// A foreign writer replaces the file after the loader has read its bytes.
	// A later stat/read must not claim those replacement bytes were loaded.
	f.publish(t, f.replacement, f.stamp)
	close(fs.releaseRead)
	<-done
	require.ElementsMatch(t, f.initial, f.snapshot())
	f.poll()
	f.awaitRestored(t)
}

func TestConfigWatcher_UnchangedAPITokenPollsStayQuiet(t *testing.T) {
	f := newTokenDiscoveryFixture(t)
	before, err := os.Stat(f.cw.apiTokensPath)
	require.NoError(t, err)
	f.poll()
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, f.callbacks.Load(), "unchanged polls rebuilt bindings")
	after, err := os.Stat(f.cw.apiTokensPath)
	require.NoError(t, err)
	bytes, err := os.ReadFile(f.cw.apiTokensPath)
	require.NoError(t, err)
	require.Equal(t, f.original, bytes)
	require.True(t, os.SameFile(before, after))
	require.Equal(t, before.ModTime(), after.ModTime())
}
