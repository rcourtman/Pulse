package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func retainedHistoryEntry(id string, at time.Time) HistoryEntry {
	return HistoryEntry{
		Alert: Alert{
			ID: id, CanonicalState: id, Type: "cpu", ResourceID: "guest-1",
			Level: AlertLevelWarning, StartTime: at, LastSeen: at,
			Metadata: map[string]interface{}{"sample": "unchanged"},
		},
		Timestamp: at,
	}
}

func TestHistoryCleanupReleasesRemovedRowStorage(t *testing.T) {
	for _, survivors := range []int{0, 3} {
		t.Run(fmt.Sprint(survivors), func(t *testing.T) {
			hm := newTestHistoryManager(t)
			now := time.Now()
			const rows = 8192
			hm.history = make([]HistoryEntry, rows)
			for i := range hm.history {
				hm.history[i] = retainedHistoryEntry(fmt.Sprint(i), now.AddDate(0, 0, -31))
			}
			var want []HistoryEntry
			// Survivors are interleaved with expired rows, not a sorted tail.
			for i := 0; i < survivors; i++ {
				entry := retainedHistoryEntry(fmt.Sprintf("live-%d", i), now.Add(-time.Duration(i+1)*time.Hour))
				hm.history[i*100] = entry
				want = append(want, entry)
			}
			hm.cleanOldEntries()
			if len(hm.history) != survivors || cap(hm.history) != survivors {
				t.Fatalf("retained rows/storage = %d/%d, want %d/%d", len(hm.history), cap(hm.history), survivors, survivors)
			}
			if hm.history == nil {
				t.Fatal("empty history must retain its non-nil slice shape")
			}
			if survivors > 0 && !reflect.DeepEqual(hm.history, want) {
				t.Fatal("cleanup changed survivor order, timestamps or alert evidence")
			}
			t.Logf("row_bytes=%d original_backing_bytes=%d retained_backing_bytes=%d", unsafe.Sizeof(HistoryEntry{}), rows*unsafe.Sizeof(HistoryEntry{}), cap(hm.history)*int(unsafe.Sizeof(HistoryEntry{})))
			hm.AddAlert(Alert{ID: "after-cleanup", Type: "cpu"})
			if len(hm.GetAllHistory(0)) != survivors+1 {
				t.Fatal("history could not accept a new occurrence after cleanup")
			}
		})
	}
}

func TestHistoryDedupReleasesRemovedRowStorage(t *testing.T) {
	hm := newTestHistoryManager(t)
	base := time.Now().Add(-time.Hour)
	const rows = 8192
	for i := 0; i < rows; i++ {
		hm.history = append(hm.history, retainedHistoryEntry("one-chain", base.Add(time.Duration(i)*time.Millisecond)))
	}
	lastSeen := hm.history[rows-1].Alert.LastSeen
	separate := retainedHistoryEntry("one-chain", lastSeen.Add(historyDedupWindow+time.Second))
	neighbour := retainedHistoryEntry("other-identity", base)
	hm.history = append(hm.history, separate, neighbour)
	hm.deduplicateHistory()
	if len(hm.history) != 3 || cap(hm.history) != 3 {
		t.Fatalf("retained rows/storage = %d/%d, want 3/3", len(hm.history), cap(hm.history))
	}
	if !hm.history[0].Timestamp.Equal(base) || !hm.history[0].Alert.StartTime.Equal(base) || !hm.history[0].Alert.LastSeen.Equal(lastSeen) {
		t.Fatal("dedup changed earliest occurrence or latest observation")
	}
	if !reflect.DeepEqual(hm.history[1], separate) || !reflect.DeepEqual(hm.history[2], neighbour) {
		t.Fatal("dedup merged a separate recurrence or another identity")
	}
	// The migration snapshot still owns its nested evidence independently.
	snapshot := hm.SnapshotEntries()
	snapshot[0].Alert.Metadata["sample"] = "caller-change"
	if hm.history[0].Alert.Metadata["sample"] != "unchanged" {
		t.Fatal("migration snapshot aliases retained evidence")
	}
}

func TestHistoryRemovalReleasesRemovedRowStorage(t *testing.T) {
	for _, survivors := range []int{0, 2} {
		t.Run(fmt.Sprint(survivors), func(t *testing.T) {
			hm := newTestHistoryManager(t)
			now := time.Now()
			hm.history = make([]HistoryEntry, 8192)
			for i := range hm.history {
				hm.history[i] = retainedHistoryEntry("remove", now)
			}
			want := make([]HistoryEntry, survivors)
			for i := range want {
				want[i] = retainedHistoryEntry(fmt.Sprintf("keep-%d", i), now.Add(-time.Duration(i)*time.Minute))
				hm.history[i*100] = want[i]
			}
			hm.RemoveAlert("remove")
			if cap(hm.history) != survivors || !reflect.DeepEqual(hm.history, want) {
				t.Fatalf("removal retained %d rows with capacity %d, want %d unchanged survivors", len(hm.history), cap(hm.history), survivors)
			}
		})
	}
}

func TestHistoryCompactionNoOpKeepsStorage(t *testing.T) {
	hm := newTestHistoryManager(t)
	hm.history = []HistoryEntry{retainedHistoryEntry("current", time.Now())}
	before := &hm.history[0]
	hm.cleanOldEntries()
	hm.RemoveAlert("absent")
	hm.deduplicateHistory()
	if &hm.history[0] != before {
		t.Fatal("unchanged cleanup replaced the current array")
	}
}

func TestHistoryLoadCompactsRecoverySourcesWithoutWriting(t *testing.T) {
	for _, leaf := range []string{"primary", "backup", "retired"} {
		t.Run(leaf, func(t *testing.T) {
			hm := newTestHistoryManager(t)
			path := hm.historyFile
			if leaf == "backup" {
				path = hm.backupFile
			} else if leaf == "retired" {
				path += ".imported"
			}
			now := time.Now()
			entries := make([]HistoryEntry, 512)
			for i := range entries {
				entries[i] = retainedHistoryEntry("expired", now.AddDate(0, 0, -31))
			}
			first := now.Add(-time.Hour)
			for i := 0; i < 128; i++ {
				entries = append(entries, retainedHistoryEntry("live", first.Add(time.Duration(i)*time.Second)))
			}
			encoded, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			if err := hm.loadHistory(); err != nil {
				t.Fatal(err)
			}
			if len(hm.history) != 1 || cap(hm.history) != 1 {
				t.Fatalf("startup retained rows/storage = %d/%d, want 1/1", len(hm.history), cap(hm.history))
			}
			if !hm.history[0].Timestamp.Equal(first) || !hm.history[0].Alert.LastSeen.Equal(entries[len(entries)-1].Alert.LastSeen) {
				t.Fatal("startup changed occurrence or latest observation")
			}
			if hm.storageRetired != (leaf == "retired") {
				t.Fatal("startup changed recovery-source retirement state")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(encoded, after) {
				t.Fatal("compaction rewrote the recovery source")
			}
			if len(hm.SnapshotEntries()) != 1 || len(hm.GetHistory(first.Add(-time.Second), 0)) != 1 {
				t.Fatal("migration or fallback reads lost the surviving occurrence")
			}
		})
	}
}

func TestHistoryCompactionConcurrentReadersAndWriters(t *testing.T) {
	hm := newTestHistoryManager(t)
	hm.history = []HistoryEntry{retainedHistoryEntry("survivor", time.Now())}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 64; i++ {
			hm.AddAlert(Alert{ID: "temporary", Type: "cpu"})
			hm.RemoveAlert("temporary")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 64; i++ {
			hm.cleanOldEntries()
			hm.deduplicateHistory()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 64; i++ {
			hm.GetAllHistory(0)
			hm.SnapshotEntries()
		}
	}()
	wg.Wait()
	if len(hm.history) != 1 || hm.history[0].Alert.ID != "survivor" {
		t.Fatal("concurrent compaction lost the surviving occurrence")
	}
}
