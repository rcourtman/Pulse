package alerts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func awaitCheckpointSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("checkpoint worker did not reach its barrier")
	}
}

func TestAsyncActiveCheckpointCoalescesInFlightRequests(t *testing.T) {
	m := &Manager{}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	if !m.queueActiveAlertsSave("initial") {
		t.Fatal("first request did not start a worker")
	}
	go m.runActiveAlertsSaveWorker(func() error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	})
	// Always release the worker before test cleanup, even on an assertion failure.
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); m.workerWG.Wait() })
	awaitCheckpointSignal(t, started)
	const requests = 10_000
	for range requests {
		if m.queueActiveAlertsSave("during checkpoint") {
			t.Fatal("request during a write admitted another worker")
		}
	}
	releaseOnce.Do(func() { close(release) })
	m.workerWG.Wait()
	if got := calls.Load(); got != 2 {
		t.Fatalf("%d in-flight requests executed %d checkpoints, want exactly two", requests, got)
	}
	t.Logf("%d requests during one blocked write produced one follow-up checkpoint", requests)
	if !m.queueActiveAlertsSave("after idle") {
		t.Fatal("worker remained stuck after becoming idle")
	}
	go m.runActiveAlertsSaveWorker(func() error { calls.Add(1); return nil })
	m.workerWG.Wait()
	if got := calls.Load(); got != 3 {
		t.Fatalf("new request after idle executed %d total checkpoints, want three", got)
	}
}

func TestAsyncActiveCheckpointFailureDoesNotStrandRequests(t *testing.T) {
	for _, failure := range []string{"error", "panic"} {
		t.Run(failure, func(t *testing.T) {
			m := &Manager{}
			started, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			if !m.queueActiveAlertsSave("initial failure") {
				t.Fatal("first request did not start a worker")
			}
			go m.runActiveAlertsSaveWorker(func() error {
				if calls.Add(1) == 1 {
					close(started)
					<-release
					if failure == "panic" {
						panic("test checkpoint panic")
					}
					return errors.New("test checkpoint failure")
				}
				return nil
			})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); m.workerWG.Wait() })
			awaitCheckpointSignal(t, started)
			if m.queueActiveAlertsSave("after failure") {
				t.Fatal("request during a failing write admitted another worker")
			}
			releaseOnce.Do(func() { close(release) })
			m.workerWG.Wait()
			if got := calls.Load(); got != 2 {
				t.Fatalf("failure stranded the follow-up request: %d checkpoints", got)
			}
			if !m.queueActiveAlertsSave("later retry") {
				t.Fatal("failed worker did not release admission")
			}
			go m.runActiveAlertsSaveWorker(func() error { calls.Add(1); return nil })
			m.workerWG.Wait()
			if got := calls.Load(); got != 3 {
				t.Fatalf("later retry did not execute: %d checkpoints", got)
			}
		})
	}
}

func TestAsyncActiveCheckpointStopOwnsFinalState(t *testing.T) {
	m := &Manager{
		alertsDir: t.TempDir(), activeAlerts: make(map[string]*Alert),
		escalationStop: make(chan struct{}), cleanupStop: make(chan struct{}),
	}
	started, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	if !m.queueActiveAlertsSave("initial") {
		t.Fatal("first request did not start a worker")
	}
	go m.runActiveAlertsSaveWorker(func() error {
		calls.Add(1)
		close(started)
		<-release
		return nil
	})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); m.Stop() })
	awaitCheckpointSignal(t, started)
	if m.queueActiveAlertsSave("superseded at shutdown") {
		t.Fatal("follow-up request admitted another worker")
	}
	go func() { m.Stop(); close(stopped) }()
	awaitCheckpointSignal(t, m.escalationStop)
	if m.queueActiveAlertsSave("too late") {
		t.Fatal("shutdown admitted a new worker")
	}
	select {
	case <-stopped:
		t.Fatal("Stop returned before the in-flight checkpoint finished")
	default:
	}
	m.mu.Lock()
	m.activeAlerts["shutdown-final"] = &Alert{ID: "shutdown-final", Acknowledged: true, AckUser: "final-operator"}
	m.mu.Unlock()
	releaseOnce.Do(func() { close(release) })
	awaitCheckpointSignal(t, stopped)
	if got := calls.Load(); got != 1 {
		t.Fatalf("shutdown drained %d redundant async checkpoints, want one in-flight", got)
	}
	data, err := os.ReadFile(filepath.Join(m.alertsDir, "active-alerts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "final-operator") {
		t.Fatalf("Stop did not checkpoint the latest state: %s", data)
	}
}

func TestAsyncActiveCheckpointConcurrentRequestsAndStop(t *testing.T) {
	m := NewManagerWithDataDir(t.TempDir())
	m.saveMu.Lock()
	var unlockOnce sync.Once
	t.Cleanup(func() { unlockOnce.Do(m.saveMu.Unlock); m.Stop() })
	m.saveActiveAlertsAsync("initial blocked checkpoint")
	start := make(chan struct{})
	entered := make(chan struct{}, 16)
	var requesters sync.WaitGroup
	for range 16 {
		requesters.Add(1)
		go func() {
			defer requesters.Done()
			<-start
			m.saveActiveAlertsAsync("before shutdown")
			entered <- struct{}{}
			for range 256 {
				m.saveActiveAlertsAsync("concurrent request")
			}
		}()
	}
	close(start)
	for range 16 {
		<-entered
	}
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	awaitCheckpointSignal(t, m.escalationStop)
	requesters.Wait()
	unlockOnce.Do(m.saveMu.Unlock)
	awaitCheckpointSignal(t, stopped)
	m.stopMu.RLock()
	defer m.stopMu.RUnlock()
	if m.activeSaveRunning {
		t.Fatal("checkpoint worker survived shutdown")
	}
}

func TestAsyncActiveCheckpointWriteFailureCanRetry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "alerts")
	if err := os.WriteFile(dir, []byte("block checkpoint directory creation"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &Manager{alertsDir: dir, activeAlerts: make(map[string]*Alert)}
	m.saveActiveAlertsAsync("unavailable directory")
	m.workerWG.Wait()
	data, err := os.ReadFile(dir)
	if err != nil || string(data) != "block checkpoint directory creation" {
		t.Fatalf("failed save changed the source: %q, %v", data, err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	m.saveActiveAlertsAsync("directory restored")
	m.workerWG.Wait()
	data, err = os.ReadFile(filepath.Join(dir, "active-alerts.json"))
	if err != nil || string(data) != "[]" {
		t.Fatalf("retry after a failed checkpoint did not persist: %q, %v", data, err)
	}
}
