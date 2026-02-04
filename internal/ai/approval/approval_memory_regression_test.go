package approval

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestApprovalStoreMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	store, err := NewStore(StoreConfig{
		DataDir:            t.TempDir(),
		DefaultTimeout:     2 * time.Minute,
		MaxApprovals:       50,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	approvalsPerCycle := 120
	executionsPerCycle := 60
	warmupCycles := 5
	measureCycles := 12

	runCycle := func(offset int) {
		for i := 0; i < approvalsPerCycle; i++ {
			idx := offset + i
			req := &ApprovalRequest{
				ExecutionID: fmt.Sprintf("exec-%d", idx),
				ToolID:      "pulse_exec",
				Command:     "systemctl status nginx",
				TargetType:  "vm",
				TargetID:    fmt.Sprintf("vm-%02d", idx%20),
				Context:     "status check",
			}
			if err := store.CreateApproval(req); err != nil {
				t.Fatalf("CreateApproval: %v", err)
			}

			var updated *ApprovalRequest
			if idx%2 == 0 {
				approved, err := store.Approve(req.ID, "admin")
				if err != nil {
					t.Fatalf("Approve: %v", err)
				}
				updated = approved
			} else {
				denied, err := store.Deny(req.ID, "admin", "not needed")
				if err != nil {
					t.Fatalf("Deny: %v", err)
				}
				updated = denied
			}
			if updated != nil {
				decidedAt := time.Now().Add(-48 * time.Hour)
				updated.DecidedAt = &decidedAt
			}
		}

		for i := 0; i < executionsPerCycle; i++ {
			idx := offset + i
			state := &ExecutionState{
				ID:              fmt.Sprintf("exec-state-%d", idx),
				CreatedAt:       time.Now().Add(-2 * time.Hour),
				ExpiresAt:       time.Now().Add(-1 * time.Hour),
				Messages:        []map[string]interface{}{{"role": "user", "content": "status"}},
				OriginalRequest: map[string]interface{}{"prompt": "status"},
			}
			if err := store.StoreExecution(state); err != nil {
				t.Fatalf("StoreExecution: %v", err)
			}
		}

		store.CleanupExpired()
	}

	for i := 0; i < warmupCycles; i++ {
		runCycle(i * approvalsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		runCycle(100000 + i*approvalsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if baseline.HeapAlloc > 0 {
		allowed := baseline.HeapAlloc + 5*1024*1024
		growthRatio := float64(after.HeapAlloc) / float64(baseline.HeapAlloc)
		if after.HeapAlloc > allowed && growthRatio > 1.25 {
			t.Fatalf("heap allocation grew too much: baseline=%d final=%d ratio=%.2f", baseline.HeapAlloc, after.HeapAlloc, growthRatio)
		}
	}
}
