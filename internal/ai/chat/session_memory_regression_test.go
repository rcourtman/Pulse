package chat

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestSessionStoreMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}

	sessionsPerCycle := 40
	messagesPerSession := 6
	warmupCycles := 4
	measureCycles := 10

	runCycle := func(offset int) {
		for i := 0; i < sessionsPerCycle; i++ {
			session, err := store.Create()
			if err != nil {
				t.Fatalf("Create: %v", err)
			}

			for j := 0; j < messagesPerSession; j++ {
				msg := Message{
					Role:      "user",
					Content:   fmt.Sprintf("message %d", offset+j),
					Timestamp: time.Now(),
				}
				if err := store.AddMessage(session.ID, msg); err != nil {
					t.Fatalf("AddMessage: %v", err)
				}
			}

			store.SetToolSet(session.ID, map[string]bool{"pulse_query": true})

			res := &ResolvedResource{
				Kind:         "vm",
				ProviderUID:  fmt.Sprintf("vm-%02d", i),
				ResourceID:   fmt.Sprintf("vm:%02d", i),
				Name:         fmt.Sprintf("vm-%02d", i),
				ResourceType: "vm",
			}
			store.AddResolvedResource(session.ID, res.Name, res)

			ka := store.GetKnowledgeAccumulator(session.ID)
			ka.SetTurn(1)
			ka.AddFact(FactCategoryResource, fmt.Sprintf("vm:%02d:status", i), "running")

			store.ClearSessionState(session.ID, false)

			if err := store.Delete(session.ID); err != nil {
				t.Fatalf("Delete: %v", err)
			}
		}
	}

	for i := 0; i < warmupCycles; i++ {
		runCycle(i * sessionsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		runCycle(100000 + i*sessionsPerCycle)
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
