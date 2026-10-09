//go:build linux

package websocket_test

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

func TestConnectedCostFixture(t *testing.T) {
	state := connectedCostMonitor().BuildBroadcastFrontendState()
	if len(state.Resources) != 1236 {
		t.Fatalf("resource fixture=%d, want1236", len(state.Resources))
	}
}

// Opt-in residency/CPU observation in a fresh test process for each declared
// scenario. VM RSS/heap and CPU here are synthetic source-native path costs,
// never a customer's process/container CPU or memory result.
func TestConnectedDashboardResidency(t *testing.T) {
	mode := os.Getenv("PULSE_CONNECTED_COST_MODE")
	if mode == "" {
		t.Skip("paired cost runner only")
	}
	if mode != "cold" && mode != "quiet" && mode != "heartbeat" && mode != "churn" {
		t.Fatal("unknown cost mode")
	}
	viewers := 17
	if mode == "cold" {
		viewers = 1
	}
	monitor := connectedCostMonitor()
	seq := int64(0)
	getter := func(string) interface{} {
		state := monitor.BuildBroadcastFrontendState()
		seq++
		if mode == "heartbeat" || mode == "churn" {
			state.LastUpdate = seq
			for i := range state.Resources {
				state.Resources[i].LastSeen = seq
			}
		}
		if mode == "churn" {
			state.Resources = state.Resources[:len(state.Resources)-int(seq%2)]
			state.Resources[0].ID = fmt.Sprintf("replaced-%d", seq)
			state.Resources[0].Labels = map[string]string{"team": fmt.Sprint(seq)}
		}
		return state
	}
	run, err := websocket.NewQuietBroadcastProjectionProbeForTest(getter, viewers)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run(); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var cpuBefore, cpuAfter syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &cpuBefore)
	start := time.Now()
	const iterations = 25
	for i := 0; i < iterations; i++ {
		if mode == "cold" {
			run, err = websocket.NewQuietBroadcastProjectionProbeForTest(getter, viewers)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err = run(); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)
	syscall.Getrusage(syscall.RUSAGE_SELF, &cpuAfter)
	runtime.ReadMemStats(&after)
	runtime.GC()
	var retained runtime.MemStats
	runtime.ReadMemStats(&retained)
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	rss, hwm := "", ""
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			rss = strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:"))
		}
		if strings.HasPrefix(line, "VmHWM:") {
			hwm = strings.TrimSpace(strings.TrimPrefix(line, "VmHWM:"))
		}
	}
	cpuUsec := func(r syscall.Rusage) int64 {
		return r.Utime.Sec*1000000 + r.Utime.Usec + r.Stime.Sec*1000000 + r.Stime.Usec
	}
	record := map[string]any{"mode": mode, "viewers": viewers, "iterations": iterations, "wall_ns": elapsed.Nanoseconds(), "cpu_usec": cpuUsec(cpuAfter) - cpuUsec(cpuBefore), "allocated_bytes": after.TotalAlloc - before.TotalAlloc, "allocations": after.Mallocs - before.Mallocs, "retained_heap_bytes": retained.HeapAlloc, "rss": rss, "high_water_rss": hwm, "gomaxprocs": runtime.GOMAXPROCS(0)}
	encoded, _ := json.Marshal(record)
	t.Logf("CONNECTED_COST %s", encoded)
	runtime.KeepAlive(run)
	runtime.KeepAlive(monitor)
}
