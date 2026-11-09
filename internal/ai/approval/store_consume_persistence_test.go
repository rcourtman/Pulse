package approval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A fresh reader, without Flush or waiting for a debounce, is the important
// boundary: the server may issue a command as soon as consumption succeeds.
func TestApprovalConsumptionIsPublishedBeforeSuccess(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "canonical", true: "legacy_hash"}[legacy], func(t *testing.T) {
			dir := t.TempDir()
			req := approvalSnapshotFixture()
			req.Status = StatusApproved
			req.ExpiresAt = time.Now().Add(time.Hour)
			if !legacy {
				req.CommandHash = ComputeCommandHash(req.Command, req.TargetType, req.TargetID)
			}
			raw, err := json.Marshal([]*ApprovalRequest{req})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_approvals.json"), raw, 0600))
			store, err := NewStore(StoreConfig{DataDir: dir})
			require.NoError(t, err)
			t.Cleanup(func() {
				store.mu.Lock()
				defer store.mu.Unlock()
				if store.saveTimer != nil {
					store.saveTimer.Stop()
				}
			})
			consumed, err := store.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
			require.NoError(t, err)
			require.True(t, consumed.Consumed)
			// Deliberately no Flush: it concealed the original restart gap.
			reloaded, err := NewStore(StoreConfig{DataDir: dir})
			require.NoError(t, err)
			saved, found := reloaded.GetApproval(req.ID)
			require.True(t, found)
			require.True(t, saved.Consumed, "successful consumption must already be saved before command dispatch")
			_, err = reloaded.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
			require.ErrorContains(t, err, "already been consumed")
		})
	}
}

func TestApprovalConsumptionPersistenceFailureReturnsNoAuthority(t *testing.T) {
	for _, failure := range []string{"write", "encode"} {
		t.Run(failure, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "store")
			require.NoError(t, os.Mkdir(dir, 0700))
			store, err := NewStore(StoreConfig{DataDir: dir})
			require.NoError(t, err)
			req := approvalSnapshotFixture()
			require.NoError(t, store.CreateApproval(req))
			_, err = store.Approve(req.ID, "fixture-reviewer")
			require.NoError(t, err)
			store.Flush()
			originals := map[string][]byte{}
			for _, name := range []string{"ai_approvals.json", "ai_executions.json"} {
				originals[name], err = os.ReadFile(filepath.Join(dir, name))
				require.NoError(t, err)
			}
			savedDir := dir
			if failure == "write" {
				// A deterministic ENOTDIR works even in root-run test environments.
				// All previously saved files stay intact in this owned fixture.
				savedDir = filepath.Join(parent, "original-store")
				require.NoError(t, os.Rename(dir, savedDir))
				require.NoError(t, os.WriteFile(dir, []byte("fixture storage unavailable"), 0600))
				t.Cleanup(func() {
					require.NoError(t, os.Remove(dir))
					require.NoError(t, os.Rename(savedDir, dir))
				})
			} else {
				// A non-encodable context elsewhere in the complete snapshot must
				// not turn a failed save into a successful command authorization.
				store.mu.Lock()
				store.approvals[req.ID].Plan.PlannedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
				store.mu.Unlock()
			}
			consumed, err := store.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
			require.ErrorContains(t, err, "approval consumption could not be saved")
			require.NotContains(t, err.Error(), parent)
			require.NotContains(t, err.Error(), req.Command)
			require.Nil(t, consumed, "a storage failure must return no dispatch authority")
			local, found := store.GetApproval(req.ID)
			require.True(t, found)
			require.True(t, local.Consumed, "uncertain publication must not revive local authority")
			_, err = store.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
			require.ErrorContains(t, err, "already been consumed")
			for name, original := range originals {
				got, err := os.ReadFile(filepath.Join(savedDir, name))
				require.NoError(t, err)
				require.Equal(t, original, got, name)
			}
			entries, err := os.ReadDir(savedDir)
			require.NoError(t, err)
			require.Len(t, entries, 2, "failure must not leave a partial or temporary save")
		})
	}
}

func TestApprovalConsumptionWithoutPersistenceNeedsNoWritableDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(dir, []byte("fixture"), 0600))
	store, err := NewStore(StoreConfig{DataDir: dir, DisablePersistence: true})
	require.NoError(t, err)
	req := approvalSnapshotFixture()
	require.NoError(t, store.CreateApproval(req))
	_, err = store.Approve(req.ID, "fixture-reviewer")
	require.NoError(t, err)
	consumed, err := store.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
	require.NoError(t, err)
	require.True(t, consumed.Consumed)
	_, err = store.ConsumeApproval(req.ID, req.Command, req.TargetType, req.TargetID)
	require.ErrorContains(t, err, "already been consumed")
}
