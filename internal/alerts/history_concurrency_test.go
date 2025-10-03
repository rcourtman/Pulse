package alerts

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestHistoryManagerConcurrentAccess(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(origLevel)

	tempDir := t.TempDir()

	hm := &HistoryManager{
		dataDir:      tempDir,
		historyFile:  filepath.Join(tempDir, HistoryFileName),
		backupFile:   filepath.Join(tempDir, HistoryBackupFileName),
		history:      make([]HistoryEntry, 0),
		saveInterval: 50 * time.Millisecond,
		stopChan:     make(chan struct{}),
	}

	// Start periodic save and cleanup routines
	hm.startPeriodicSave()
	go hm.cleanupRoutine()

	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hm.AddAlert(Alert{
				ID:        fmt.Sprintf("alert-%d", i),
				Type:      "test",
				Level:     AlertLevelWarning,
				Message:   "test alert",
				StartTime: time.Now(),
			})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hm.GetHistory(time.Now().Add(-1*time.Hour), 100)
			hm.GetAllHistory(100)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			hm.RemoveAlert(fmt.Sprintf("alert-%d", i))
			time.Sleep(1 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations/10; i++ {
			_ = hm.ClearAllHistory()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	close(hm.stopChan)
	if hm.saveTicker != nil {
		hm.saveTicker.Stop()
	}

	// give final save a chance
	time.Sleep(20 * time.Millisecond)

	_ = os.RemoveAll(tempDir)
}
