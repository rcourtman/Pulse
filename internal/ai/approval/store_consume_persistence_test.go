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
