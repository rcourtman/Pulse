package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestPMGPollLoggerKeepsProcessFallback(t *testing.T) {
	if pmgPollLogger(context.Background()) != &log.Logger {
		t.Fatal("ordinary PMG polling lost the process logger")
	}
}

func TestPMGPollUsesCallerLogger(t *testing.T) {
	for _, scenario := range []string{"cluster-unread", "all-nodes-unread", "partial", "complete"} {
		t.Run(scenario, func(t *testing.T) {
			var output bytes.Buffer
			logger := zerolog.New(&output)
			ctx := logger.WithContext(context.Background())
			if pmgPollLogger(ctx) != log.Ctx(ctx) {
				t.Fatal("PMG polling lost the caller's logger")
			}
			m, f := pmgPollingReadFixture(0, scenario)
			m.pollPMGInstance(ctx, "gateway", f)

			completed, preserved := 0, 0
			decoder := json.NewDecoder(&output)
			for {
				var event map[string]any
				err := decoder.Decode(&event)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("invalid PMG diagnostic: %v", err)
				}
				if event["instance"] != "gateway" {
					t.Fatalf("PMG diagnostic lost its instance: %v", event)
				}
				switch event["message"] {
				case "PMG instance updated in state":
					completed++
					if event["level"] != "info" || event["status"] != "online" {
						t.Fatalf("PMG completion diagnostic changed: %v", event)
					}
				case "PMG backup list unavailable for some nodes; keeping their previous backups":
					preserved++
					if event["level"] != "warn" || event["nodes"] == nil || event["clusterReadFailed"] != (scenario == "cluster-unread") {
						t.Fatalf("PMG carry-forward diagnostic changed: %v", event)
					}
				}
			}
			wantPreserved := 1
			if scenario == "complete" {
				wantPreserved = 0
			}
			if completed != 1 || preserved != wantPreserved {
				t.Fatalf("caller did not receive PMG diagnostics: completed=%d preserved=%d", completed, preserved)
			}
		})
	}
}

func TestPMGPollAllocationGuardDoesNotReplaceProcessLogger(t *testing.T) {
	// Reproduce the shared resource in the containing-suite failure without
	// timers, persistence or background allocations contaminating AllocsPerRun.
	// Reading a level is allocation-free but races with replacing log.Logger,
	// just as the alert manager's background log reads did in the original job.
	var observedLevel atomic.Int32
	var reads atomic.Uint64
	started, stop, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		observedLevel.Store(int32(log.Logger.GetLevel()))
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
				observedLevel.Store(int32(log.Logger.GetLevel()))
				reads.Add(1)
				runtime.Gosched()
			}
		}
	}()
	<-started
	defer func() {
		close(stop)
		<-done
	}()
	t.Run("real-allocation-guard", TestPMGBackupReadsAvoidUnrelatedFleetCopy)
	if reads.Load() == 0 {
		t.Fatal("concurrent process logger reader did not run")
	}
}
