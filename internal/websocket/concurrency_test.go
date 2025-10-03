package websocket

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog"
)

func TestBroadcastAlertConcurrentMutation(t *testing.T) {
	origLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() {
		zerolog.SetGlobalLevel(origLevel)
	})

	hub := NewHub(nil)

	done := make(chan struct{})
	var drain sync.WaitGroup
	drain.Add(1)
	go func() {
		defer drain.Done()
		for {
			select {
			case <-done:
				return
			case _, ok := <-hub.broadcast:
				if !ok {
					return
				}
			}
		}
	}()

	alert := &alerts.Alert{
		ID:         "test-alert",
		Type:       "cpu",
		Level:      alerts.AlertLevelWarning,
		ResourceID: "vm/100",
		Message:    "CPU high",
		Metadata: map[string]interface{}{
			"initial": true,
		},
		StartTime: time.Now(),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	iterations := 1000
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mu.Lock()
			alert.Value = float64(i)
			if alert.Metadata != nil {
				alert.Metadata["iteration"] = i
			}
			mu.Unlock()
			time.Sleep(time.Microsecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			mu.Lock()
			alertCopy := alert.Clone()
			mu.Unlock()
			hub.BroadcastAlert(alertCopy)
		}
	}()

	wg.Wait()
	close(done)
	drain.Wait()
}
