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

func TestHistoryManagerOnAlertConcurrentRegistrationAndDispatch(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(origLevel)

	hm := newTestHistoryManager(t)

	const iterations = 200
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			hm.OnAlert(func(alert Alert) {})
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			hm.AddAlert(Alert{
				ID:        fmt.Sprintf("callback-alert-%d", i),
				Type:      "cpu",
				Level:     AlertLevelWarning,
				Message:   "callback test",
				StartTime: time.Now(),
			})
		}
	}()

	close(start)
	wg.Wait()
}

func TestHistoryManagerStopIdempotent(t *testing.T) {
	hm := newTestHistoryManager(t)
	hm.saveTicker = time.NewTicker(time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hm.Stop()
		}()
	}
	wg.Wait()

	select {
	case <-hm.stopChan:
	default:
		t.Fatal("stopChan should be closed after Stop")
	}
}
