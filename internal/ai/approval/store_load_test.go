package approval

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApprovalLoadFailurePreservesBothFiles(t *testing.T) {
	for _, filename := range []string{"ai_approvals.json", "ai_executions.json"} {
		for _, tc := range []struct{ name, raw string }{
			{"truncated", `[{"id":`},
			{"empty_file", ``},
			{"wrong_shape", `{}`},
			{"null_record", `[null]`},
			{"bad_timestamp", `[{"id":"same","expiresAt":"not-a-time"}]`},
		} {
			t.Run(filename+"/"+tc.name, func(t *testing.T) {
				if filename == "ai_executions.json" && tc.name == "null_record" {
					t.Skip("legacy null execution records remain supported")
				}
				dir := t.TempDir()
				// Missing hash/padded target deliberately require the existing migration;
				// the other file must validate before a migration is allowed to publish.
				originals := map[string][]byte{
					"ai_approvals.json":  []byte(`[{"id":"kept","orgId":"tenant-a","targetType":" AGENT ","command":"printf fixture","status":"pending","expiresAt":"2099-01-01T00:00:00Z"}]`),
					"ai_executions.json": []byte(`[{"id":"execution-kept","expiresAt":"2099-01-01T00:00:00Z"}]`),
				}
				originals[filename] = []byte(tc.raw)
				for name, body := range originals {
					require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0600))
				}
				var store *Store
				var err error
				require.NotPanics(t, func() { store, err = NewStore(StoreConfig{DataDir: dir}) })
				loadErr := err
				if store != nil {
					store.Flush() // normal shutdown of an incorrectly accepted partial load
				}
				for name, body := range originals {
					got, e := os.ReadFile(filepath.Join(dir, name))
					require.NoError(t, e)
					require.Equal(t, body, got, name)
				}
				require.Error(t, loadErr, "unknown saved state must not become a fresh writable store")
				require.Nil(t, store)
				entries, e := os.ReadDir(dir)
				require.NoError(t, e)
				require.Len(t, entries, 2, "no replacement, backup, or temporary save after failed load")
			})
		}
	}
}

func TestApprovalLoadReadErrorsAreNotAbsence(t *testing.T) {
	for _, filename := range []string{"ai_approvals.json", "ai_executions.json"} {
		t.Run(filename, func(t *testing.T) {
			dir := t.TempDir()
			other := "ai_executions.json"
			if filename == other {
				other = "ai_approvals.json"
			}
			original := []byte(`[{"id":"kept","targetType":" AGENT ","expiresAt":"2099-01-01T00:00:00Z"}]`)
			require.NoError(t, os.WriteFile(filepath.Join(dir, other), original, 0600))
			require.NoError(t, os.Mkdir(filepath.Join(dir, filename), 0700))
			store, err := NewStore(StoreConfig{DataDir: dir})
			require.Error(t, err)
			require.Nil(t, store)
			info, err := os.Stat(filepath.Join(dir, filename))
			require.NoError(t, err)
			require.True(t, info.IsDir())
			got, err := os.ReadFile(filepath.Join(dir, other))
			require.NoError(t, err)
			require.Equal(t, original, got)
		})
	}
}

func TestApprovalLoadMissingEmptyAndLegacyRecovery(t *testing.T) {
	t.Run("new_install", func(t *testing.T) {
		dir := t.TempDir()
		store, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		require.Empty(t, store.GetPendingApprovals())
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})
	t.Run("legacy_null", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"ai_approvals.json", "ai_executions.json"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("null\n"), 0600))
		}
		store, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		require.Empty(t, store.GetPendingApprovals())
	})
	t.Run("legacy_sparse_identity", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), []byte(`[{"id":"","targetType":"agent"},{"id":"same","targetType":"agent","command":"old"},{"id":"same","targetType":"agent","command":"last"}]`), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_executions.json"), []byte(`[null,{"id":"exec","expiresAt":"2099-01-01T00:00:00Z"}]`), 0600))
		store, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		got, ok := store.GetApproval("same")
		require.True(t, ok)
		require.Equal(t, "last", got.Command)
		require.Len(t, store.executions, 2)
	})
	t.Run("explicit_empty", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"ai_approvals.json", "ai_executions.json"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("[]\n"), 0600))
		}
		store, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		require.Empty(t, store.GetPendingApprovals())
		for _, name := range []string{"ai_approvals.json", "ai_executions.json"} {
			got, err := os.ReadFile(filepath.Join(dir, name))
			require.NoError(t, err)
			require.Equal(t, []byte("[]\n"), got)
		}
	})
	t.Run("disabled_persistence", func(t *testing.T) {
		dir := t.TempDir()
		raw := []byte("not json")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), raw, 0600))
		store, err := NewStore(StoreConfig{DataDir: dir, DisablePersistence: true})
		require.NoError(t, err)
		store.Flush()
		got, err := os.ReadFile(filepath.Join(dir, "ai_approvals.json"))
		require.NoError(t, err)
		require.Equal(t, raw, got)
	})
	t.Run("complete_legacy_load", func(t *testing.T) {
		dir := t.TempDir()
		// Missing hashes/padded canonical target and an unsupported legacy alias
		// retain their established migration only after both files validate.
		raw := `[{"id":"kept","command":"printf fixture","targetType":" AGENT ","targetId":"fixture-host","status":"approved","consumed":true,"expiresAt":"2099-01-01T00:00:00Z"},{"id":"removed","targetType":"host","status":"pending"}]`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), []byte(raw), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_executions.json"), []byte(`[{"id":"exec","expiresAt":"2099-01-01T00:00:00Z"}]`), 0600))
		store, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		kept, ok := store.GetApproval("kept")
		require.True(t, ok)
		require.Equal(t, "agent", kept.TargetType)
		require.True(t, kept.Consumed)
		require.Equal(t, ComputeCommandHash("printf fixture", "agent", "fixture-host"), kept.CommandHash)
		_, ok = store.GetApproval("removed")
		require.False(t, ok)
		_, err = store.ConsumeApproval("kept", "printf fixture", "agent", "fixture-host")
		require.ErrorContains(t, err, "consumed")
		ex, ok := store.GetExecution("exec")
		require.True(t, ok)
		require.NotNil(t, ex.OriginalRequest)
		require.NotNil(t, ex.Messages)
		require.NotNil(t, ex.PendingToolCall)
		store.Flush()
		reloaded, err := NewStore(StoreConfig{DataDir: dir})
		require.NoError(t, err)
		kept, ok = reloaded.GetApproval("kept")
		require.True(t, ok)
		require.True(t, kept.Consumed)
		data, err := os.ReadFile(filepath.Join(dir, "ai_approvals.json"))
		require.NoError(t, err)
		require.True(t, json.Valid(data))
		require.False(t, bytes.Contains(data, []byte(`"removed"`)))
	})
}

func TestApprovalLoadFailureDoesNotPublishPartialMaps(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), []byte(`[{"id":"replacement","targetType":"agent"}]`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_executions.json"), []byte(`{`), 0600))
	store := &Store{dataDir: dir, approvals: map[string]*ApprovalRequest{"prior": {ID: "prior"}}, executions: map[string]*ExecutionState{"prior": {ID: "prior", ExpiresAt: time.Now().Add(time.Hour)}}}
	err := store.load()
	require.Error(t, err)
	require.Len(t, store.approvals, 1)
	require.NotNil(t, store.approvals["prior"])
	require.NotNil(t, store.executions["prior"])
	require.False(t, strings.Contains(err.Error(), "replacement"), "errors identify structure, not saved command or record contents")
}

func TestApprovalStoreReadinessPublication(t *testing.T) {
	prior, status := GetStoreWithStatus()
	t.Cleanup(func() {
		if status == StoreUnavailable {
			SetStoreUnavailable()
		} else {
			SetStore(prior)
		}
	})
	store, err := NewStore(StoreConfig{DataDir: t.TempDir(), DisablePersistence: true})
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			SetStoreUnavailable()
			SetStore(store)
			SetStore(nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			got, status := GetStoreWithStatus()
			switch status {
			case StoreReady:
				if got == nil {
					t.Error("ready without store")
				}
			case StoreUnavailable, StoreUninitialized:
				if got != nil {
					t.Error("unavailable/uninitialized with store")
				}
			default:
				t.Error("unknown readiness")
			}
		}
	}()
	wg.Wait()
	SetStoreUnavailable()
	got, status := GetStoreWithStatus()
	require.Nil(t, got)
	require.Equal(t, StoreUnavailable, status)
	require.Nil(t, GetStore())
	SetStore(store)
	got, status = GetStoreWithStatus()
	require.Same(t, store, got)
	require.Equal(t, StoreReady, status)
	SetStore(nil)
	got, status = GetStoreWithStatus()
	require.Nil(t, got)
	require.Equal(t, StoreUninitialized, status)
}
