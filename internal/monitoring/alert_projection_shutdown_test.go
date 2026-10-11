package monitoring

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Hold a real replay at the exported canonical writer boundary. No database
// implementation, workload or timing assumption chooses the overlap.
type projectionShutdownRecorder struct {
	*unifiedresources.MonitorAdapter
	resourceID string
	entered    chan struct{}
	release    chan struct{}
	enterOnce  sync.Once
	writes     atomic.Int64
}

func (s *projectionShutdownRecorder) RecordChange(change unifiedresources.ResourceChange) error {
	if change.ResourceID == s.resourceID {
		s.enterOnce.Do(func() {
			close(s.entered)
			<-s.release
		})
		s.writes.Add(1)
	}
	return s.MonitorAdapter.RecordChange(change)
}

func blockedProjectionRecorder(t *testing.T) (*projectionShutdownRecorder, func()) {
	t.Helper()
	store := unifiedresources.NewMemoryStore()
	s := &projectionShutdownRecorder{
		MonitorAdapter: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)),
		resourceID:     "shutdown-history-resource",
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	var once sync.Once
	unblock := func() { once.Do(func() { close(s.release) }) }
	t.Cleanup(unblock)
	return s, unblock
}

func shutdownHistorySnapshots() []alerts.Alert {
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	ack := start.Add(time.Second)
	return []alerts.Alert{{
		ID: "shutdown-history-alert", ResourceID: "shutdown-history-resource",
		Type: "cpu", Level: alerts.AlertLevelWarning,
		StartTime: start, LastSeen: start.Add(2 * time.Second),
		Acknowledged: true, AckTime: &ack, AckUser: "synthetic-operator",
	}}
}

func TestMonitorStartJoinsAlertProjectionBeforeReturning(t *testing.T) {
	configureAlertShutdownFixture(t)
	monitor, err := New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	recorder, unblock := blockedProjectionRecorder(t)
	importReplaySnapshots(t, monitor.GetAlertManager(), shutdownHistorySnapshots())
	monitor.SetResourceStore(recorder)
	select {
	case <-recorder.entered:
	case <-time.After(5 * time.Second):
		unblock()
		monitor.alertProjectionWG.Wait()
		monitor.Stop()
		t.Fatal("history replay did not reach the canonical writer")
	}

	// Replay starts during router attachment, before Start. Even a runtime
	// cancelled before startup must join that already-admitted writer.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitor.Start(ctx, nil)
	}()
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
			monitor.alertProjectionWG.Wait()
			monitor.Stop()
		case <-time.After(5 * time.Second):
			t.Error("monitor did not finish after releasing its history writer")
		}
	})
	select {
	case <-done:
		t.Error("monitor reported completion while alert-history replay could still write")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor failed to join its completed history writer")
	}
	monitor.alertProjectionWG.Wait()
	if got := recorder.writes.Load(); got != 3 {
		t.Fatalf("admitted snapshot wrote %d changes, want its complete fired/acknowledged/resolved occurrence", got)
	}
}
