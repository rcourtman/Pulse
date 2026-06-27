package alerts

import (
	"testing"
	"time"
)

func TestDeduplicateHistory(t *testing.T) {
	t.Run("collapses consecutive same-alert entries within dedup window", func(t *testing.T) {
		hm := &HistoryManager{
			history:  make([]HistoryEntry, 0),
			dataDir:  t.TempDir(),
			stopChan: make(chan struct{}),
		}

		base := time.Now()
		alert := Alert{
			ID:             "zfs-1::zfs-1-health",
			CanonicalState: "zfs-1::zfs-1-health",
			Type:           "zfs-pool-errors",
			Level:          AlertLevelWarning,
			ResourceID:     "zfs-1",
			StartTime:      base,
			LastSeen:       base.Add(1 * time.Minute),
		}

		for i := 0; i < 10; i++ {
			dup := *alert.Clone()
			dup.StartTime = base.Add(time.Duration(i*2) * time.Minute)
			dup.LastSeen = base.Add(time.Duration(i*2+1) * time.Minute)
			hm.history = append(hm.history, HistoryEntry{
				Alert:     dup,
				Timestamp: base.Add(time.Duration(i*2) * time.Minute),
			})
		}

		hm.deduplicateHistory()

		if len(hm.history) != 1 {
			t.Fatalf("expected 1 entry after dedup, got %d", len(hm.history))
		}
		if !hm.history[0].Alert.LastSeen.Equal(base.Add(19 * time.Minute)) {
			t.Errorf("expected LastSeen to be latest (%v), got %v", base.Add(19*time.Minute), hm.history[0].Alert.LastSeen)
		}
	})

	t.Run("preserves entries beyond dedup window", func(t *testing.T) {
		hm := &HistoryManager{
			history:  make([]HistoryEntry, 0),
			dataDir:  t.TempDir(),
			stopChan: make(chan struct{}),
		}

		base := time.Now()
		key := "storage-1::storage-1-health"

		for _, offset := range []time.Duration{0, 2 * time.Minute, 10 * time.Minute, 12 * time.Minute} {
			hm.history = append(hm.history, HistoryEntry{
				Alert: Alert{
					ID:             key,
					CanonicalState: key,
					StartTime:      base.Add(offset),
					LastSeen:       base.Add(offset + 1*time.Minute),
				},
				Timestamp: base.Add(offset),
			})
		}

		hm.deduplicateHistory()

		if len(hm.history) != 2 {
			t.Fatalf("expected 2 entries (two chains separated by 8min gap), got %d", len(hm.history))
		}
	})

	t.Run("collapses interleaved same-alert entries within dedup window", func(t *testing.T) {
		hm := &HistoryManager{
			history:  make([]HistoryEntry, 0),
			dataDir:  t.TempDir(),
			stopChan: make(chan struct{}),
		}

		base := time.Now()
		// alert-a and alert-b alternate every 1 minute
		// Each alert appears twice within the 5-min window, so each collapses to 1
		for i, key := range []string{"alert-a", "alert-b", "alert-a", "alert-b"} {
			hm.history = append(hm.history, HistoryEntry{
				Alert: Alert{
					ID:             key,
					CanonicalState: key,
					StartTime:      base.Add(time.Duration(i) * time.Minute),
					LastSeen:       base.Add(time.Duration(i) * time.Minute),
				},
				Timestamp: base.Add(time.Duration(i) * time.Minute),
			})
		}

		hm.deduplicateHistory()

		if len(hm.history) != 2 {
			t.Fatalf("expected 2 entries (interleaved same-alert pairs collapsed), got %d", len(hm.history))
		}
	})

	t.Run("no-op on single entry", func(t *testing.T) {
		hm := &HistoryManager{
			history:  make([]HistoryEntry, 0),
			stopChan: make(chan struct{}),
		}
		hm.history = append(hm.history, HistoryEntry{
			Alert:     Alert{ID: "x"},
			Timestamp: time.Now(),
		})

		hm.deduplicateHistory()

		if len(hm.history) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(hm.history))
		}
	})
}
