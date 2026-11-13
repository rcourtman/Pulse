package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImportConfigSerializesIndependentPersistenceSaves(t *testing.T) {
	for _, fails := range []bool{false, true} {
		name := "commit"
		if fails {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			cp := seedImportValidationState(t)
			// Router, watcher and tenant consumers can hold different store
			// objects for the same normalized directory. An instance-only lock
			// would leave these ordinary saved-token writes unprotected.
			peer := NewConfigPersistence(filepath.Join(cp.configDir, "."))
			if fails {
				require.NoError(t, os.Remove(cp.systemFile))
				require.NoError(t, os.Mkdir(cp.systemFile, 0700))
			}
			archive := metadataImportArchive(t, map[string]*GuestMetadata{"imported:202": {Description: "Imported metadata"}})
			attempt := pauseConfigImport(t, cp, archive)
			token, err := NewAPITokenRecord("synthetic-independent-reader", "Independent reader", []string{ScopeMonitoringRead})
			require.NoError(t, err)
			token.OrgID = "client-b"
			done := make(chan struct{})
			var saveErr error
			go func() {
				saveErr = peer.SaveAPITokens([]APITokenRecord{*token})
				close(done)
			}()
			t.Cleanup(func() {
				attempt.release()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("independent saver did not join")
				}
			})
			select {
			case <-done:
				t.Error("independent store saved through an uncommitted import")
			case <-time.After(100 * time.Millisecond):
			}
			attempt.finish(t, fails)
			select {
			case <-done:
				require.NoError(t, saveErr)
			case <-time.After(10 * time.Second):
				t.Fatal("independent save stayed blocked after import completion")
			}
			for _, reader := range []*ConfigPersistence{cp, peer} {
				loaded, err := reader.LoadAPITokens()
				require.NoError(t, err)
				require.Equal(t, []APITokenRecord{*token}, loaded)
				cfg := &Config{APITokens: loaded}
				cfg.SortAPITokens()
				require.True(t, cfg.IsValidAPIToken("synthetic-independent-reader"))
				require.True(t, loaded[0].CanAccessOrg("client-b"))
				require.False(t, loaded[0].CanAccessOrg("client-a"))
				require.False(t, loaded[0].HasScope(ScopeSettingsWrite))
			}
			requireNoConfigImportArtifacts(t, cp)
		})
	}
}

func TestPersistenceDirectoryLockRetainsWaitersThenRetires(t *testing.T) {
	directory := t.TempDir()
	writer := persistenceMutex{directory: directory}
	reader := persistenceMutex{directory: directory}
	writer.Lock()
	var writerOnce, readerOnce sync.Once
	unlockWriter := func() { writerOnce.Do(writer.Unlock) }
	releaseReader := make(chan struct{})
	unlockReader := func() { readerOnce.Do(func() { close(releaseReader) }) }
	acquired, done := make(chan struct{}), make(chan struct{})
	go func() {
		reader.RLock()
		close(acquired)
		<-releaseReader
		reader.RUnlock()
		close(done)
	}()
	t.Cleanup(func() {
		unlockWriter()
		unlockReader()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("scope-lock waiter did not join")
		}
	})
	require.Eventually(t, func() bool {
		persistenceDirectoryLocks.Lock()
		defer persistenceDirectoryLocks.Unlock()
		entry := persistenceDirectoryLocks.directories[directory]
		return entry != nil && entry.refs == 2
	}, time.Second, time.Millisecond, "waiter did not retain the active directory lock")
	select {
	case <-acquired:
		t.Fatal("reader entered during an active directory writer")
	default:
	}
	unlockWriter()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("reader did not inherit the retained directory lock")
	}
	persistenceDirectoryLocks.Lock()
	refs := persistenceDirectoryLocks.directories[directory].refs
	persistenceDirectoryLocks.Unlock()
	require.Equal(t, 1, refs)
	unlockReader()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader did not finish")
	}
	persistenceDirectoryLocks.Lock()
	_, retained := persistenceDirectoryLocks.directories[directory]
	persistenceDirectoryLocks.Unlock()
	require.False(t, retained, "quiescent scopes must not accumulate in the lock registry")
}

func TestPersistenceDirectoryReadersRemainConcurrent(t *testing.T) {
	directory := t.TempDir()
	first := persistenceMutex{directory: directory}
	second := persistenceMutex{directory: directory}
	first.RLock()
	defer first.RUnlock()
	done := make(chan struct{})
	go func() {
		second.RLock()
		second.RUnlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("independent readers were unnecessarily serialized")
	}
}
