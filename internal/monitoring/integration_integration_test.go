//go:build integration

package monitoring

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var soakFlag = flag.Bool("soak", false, "run adaptive polling soak test")

func TestAdaptiveSchedulerIntegration(t *testing.T) {
	scenario := HarnessScenario{
		Duration:       45 * time.Second,
		WarmupDuration: 10 * time.Second,
	}

	for i := 0; i < 10; i++ {
		scenario.Instances = append(scenario.Instances, InstanceConfig{
			Type:        "pve",
			Name:        fmt.Sprintf("pve-%02d", i),
			SuccessRate: 1.0,
			BaseLatency: 150 * time.Millisecond,
		})
	}

	scenario.Instances = append(scenario.Instances, InstanceConfig{
		Type:        "pve",
		Name:        "pve-transient",
		SuccessRate: 1.0,
		FailureSeq: []FailureType{
			FailureTransient,
			FailureTransient,
			FailureTransient,
			FailureNone,
			FailureNone,
			FailureNone,
		},
		BaseLatency: 120 * time.Millisecond,
	})

	scenario.Instances = append(scenario.Instances, InstanceConfig{
		Type:        "pve",
		Name:        "pve-permanent",
		SuccessRate: 1.0,
		FailureSeq: []FailureType{
			FailurePermanent,
		},
		BaseLatency: 160 * time.Millisecond,
	})

	harness := NewHarness(scenario)

	ctx, cancel := context.WithTimeout(context.Background(), scenario.Duration+scenario.WarmupDuration+10*time.Second)
	defer cancel()

	report := harness.Run(ctx)

	instanceCount := len(scenario.Instances)
	if len(report.PerInstanceStats) != instanceCount {
		t.Fatalf("expected stats for %d instances, got %d", instanceCount, len(report.PerInstanceStats))
	}

	for key, stats := range report.PerInstanceStats {
		if stats.Total == 0 {
			t.Fatalf("instance %s executed zero tasks", key)
		}
		if stats.AverageLatency <= 0 {
			t.Fatalf("instance %s reported invalid latency %v", key, stats.AverageLatency)
		}
		if stats.Successes > 0 && stats.LastSuccessAt.IsZero() {
			t.Fatalf("instance %s recorded successes but missing last success timestamp", key)
		}
		if stats.PermanentFailures == 0 && stats.TransientFailures == 0 && stats.Successes < 3 {
			t.Fatalf("instance %s expected to execute at least 3 successful polls, got %d", key, stats.Successes)
		}
	}

	maxAllowedDepth := int(math.Ceil(float64(instanceCount) * 1.5))
	if report.QueueStats.MaxDepth > maxAllowedDepth {
		t.Fatalf("queue depth exceeded threshold: max %d, allowed %d", report.QueueStats.MaxDepth, maxAllowedDepth)
	}
	if report.QueueStats.FinalDepth > instanceCount {
		t.Fatalf("final queue depth %d exceeds instance count %d", report.QueueStats.FinalDepth, instanceCount)
	}
	if report.QueueStats.AverageDepth <= 0 {
		t.Fatalf("expected average queue depth > 0, got %f", report.QueueStats.AverageDepth)
	}
	if report.QueueStats.MaxDepth == 0 {
		t.Fatal("expected queue depth to grow beyond zero")
	}

	if report.Health.Queue.Depth != report.QueueStats.FinalDepth {
		t.Fatalf("health queue depth %d does not match final depth %d", report.Health.Queue.Depth, report.QueueStats.FinalDepth)
	}
	if report.Health.Queue.Depth > report.QueueStats.MaxDepth {
		t.Fatalf("health queue depth %d exceeds observed max %d", report.Health.Queue.Depth, report.QueueStats.MaxDepth)
	}

	if len(report.RuntimeSamples) >= 2 {
		startSample := report.RuntimeSamples[0]
		finalSample := report.RuntimeSamples[len(report.RuntimeSamples)-1]
		if startSample.HeapAlloc > 0 {
			growthRatio := float64(finalSample.HeapAlloc) / float64(startSample.HeapAlloc)
			if growthRatio > 1.25 && finalSample.HeapAlloc > startSample.HeapAlloc+5*1024*1024 {
				t.Fatalf("heap allocation grew too much: start=%d final=%d ratio=%.2f", startSample.HeapAlloc, finalSample.HeapAlloc, growthRatio)
			}
		}
		if finalSample.Goroutines > startSample.Goroutines+20 {
			t.Fatalf("goroutine count grew too much: start=%d final=%d", startSample.Goroutines, finalSample.Goroutines)
		}
	}

	maxStaleness := report.MaxStaleness
	if maxStaleness <= 0 {
		t.Fatalf("invalid max staleness value: %v", maxStaleness)
	}
	for _, snap := range report.Health.Staleness {
		key := instanceKey(snap.Type, snap.Instance)
		stats, ok := report.PerInstanceStats[key]
		if !ok {
			t.Fatalf("missing stats for staleness snapshot %s", key)
		}
		if stats.Successes == 0 || stats.PermanentFailures > 0 {
			continue
		}
		if stats.LastSuccessAt.IsZero() {
			t.Fatalf("missing last success timestamp for %s", key)
		}
		age := time.Since(stats.LastSuccessAt)
		maxHealthyAge := 20 * time.Second
		if maxHealthyAge > scenario.Duration {
			maxHealthyAge = scenario.Duration
		}
		if age > maxHealthyAge {
			t.Fatalf("instance %s staleness age %v exceeds healthy threshold %v", key, age, maxHealthyAge)
		}
		observedScore := age.Seconds() / maxStaleness.Seconds()
		if snap.Score < 0 || snap.Score > 1.01 {
			t.Fatalf("invalid staleness score %.2f for %s", snap.Score, key)
		}
		if math.Abs(snap.Score-observedScore) > 0.5 {
			t.Fatalf("staleness score %.2f for %s diverges from observed %.2f", snap.Score, key, observedScore)
		}
	}

	transientKey := instanceKey("pve", "pve-transient")
	transientStats, ok := report.PerInstanceStats[transientKey]
	if !ok {
		t.Fatalf("missing transient instance stats for %s", transientKey)
	}
	if transientStats.TransientFailures < 3 {
		t.Fatalf("expected at least 3 transient failures for %s, got %d", transientKey, transientStats.TransientFailures)
	}
	if transientStats.Successes == 0 {
		t.Fatalf("expected transient instance to recover with successes, got 0")
	}

	dlqKeys := map[string]struct{}{}
	for _, task := range report.Health.DeadLetter.Tasks {
		dlqKeys[instanceKey(task.Type, task.Instance)] = struct{}{}
	}
	if len(report.Health.Breakers) > len(dlqKeys) {
		t.Fatalf("unexpected number of circuit breaker entries: got %d want <= %d", len(report.Health.Breakers), len(dlqKeys))
	}
	for _, breaker := range report.Health.Breakers {
		key := instanceKey(breaker.Type, breaker.Instance)
		if _, ok := dlqKeys[key]; !ok {
			t.Fatalf("unexpected circuit breaker entry: %+v", breaker)
		}
		if breaker.Failures <= 0 {
			t.Fatalf("expected breaker %s to record failures, got %d", key, breaker.Failures)
		}
	}

	expectedDLQ := map[string]struct{}{}
	for _, inst := range scenario.Instances {
		for _, ft := range inst.FailureSeq {
			if ft == FailurePermanent {
				expectedDLQ[instanceKey(inst.Type, inst.Name)] = struct{}{}
				break
			}
		}
	}

	if report.Health.DeadLetter.Count != len(expectedDLQ) {
		t.Fatalf("expected %d dead-letter tasks, got %d", len(expectedDLQ), report.Health.DeadLetter.Count)
	}
	if len(report.Health.DeadLetter.Tasks) != len(expectedDLQ) {
		t.Fatalf("dead-letter task list mismatch: expected %d, got %d", len(expectedDLQ), len(report.Health.DeadLetter.Tasks))
	}
	for _, task := range report.Health.DeadLetter.Tasks {
		key := instanceKey(task.Type, task.Instance)
		if _, ok := expectedDLQ[key]; !ok {
			t.Fatalf("unexpected dead-letter task: %s", key)
		}
		delete(expectedDLQ, key)
	}
	if len(expectedDLQ) != 0 {
		t.Fatalf("missing dead-letter entries for: %v", expectedDLQ)
	}

	if !report.Health.Enabled {
		t.Fatal("expected adaptive polling to be enabled in scheduler health response")
	}
}

func TestAdaptiveSchedulerSoak(t *testing.T) {
	minutesEnv := os.Getenv("HARNESS_SOAK_MINUTES")
	if !*soakFlag && minutesEnv == "" {
		t.Skip("skipping soak test (enable with -soak or HARNESS_SOAK_MINUTES)")
	}

	minutes := 15
	if minutesEnv != "" {
		if parsed, err := strconv.Atoi(minutesEnv); err == nil && parsed > 0 {
			minutes = parsed
		}
	}

	duration := time.Duration(minutes) * time.Minute
	warmup := 2 * time.Minute
	scenario := HarnessScenario{Duration: duration, WarmupDuration: warmup}

	for i := 0; i < 60; i++ {
		scenario.Instances = append(scenario.Instances, InstanceConfig{
			Type:        "pve",
			Name:        fmt.Sprintf("soak-healthy-%02d", i),
			SuccessRate: 0.98,
			BaseLatency: 200 * time.Millisecond,
		})
	}

	for i := 0; i < 15; i++ {
		scenario.Instances = append(scenario.Instances, InstanceConfig{
			Type:        "pve",
			Name:        fmt.Sprintf("soak-transient-%02d", i),
			SuccessRate: 0.85,
			FailureSeq: []FailureType{
				FailureTransient,
				FailureTransient,
				FailureNone,
				FailureTransient,
				FailureNone,
			},
			BaseLatency: 220 * time.Millisecond,
		})
	}

	permanentCount := 5
	for i := 0; i < permanentCount; i++ {
		scenario.Instances = append(scenario.Instances, InstanceConfig{
			Type:        "pve",
			Name:        fmt.Sprintf("soak-permanent-%02d", i),
			SuccessRate: 0,
			FailureSeq: []FailureType{
				FailureTransient,
				FailureTransient,
				FailurePermanent,
			},
			BaseLatency: 250 * time.Millisecond,
		})
	}

	harness := NewHarness(scenario)
	ctx, cancel := context.WithTimeout(context.Background(), duration+warmup+5*time.Minute)
	defer cancel()

	report := harness.Run(ctx)

	if len(report.RuntimeSamples) < 2 {
		t.Fatalf("expected runtime samples, got %d", len(report.RuntimeSamples))
	}

	startSample := report.RuntimeSamples[0]
	warmupEnd := startSample.Timestamp.Add(report.Scenario.WarmupDuration)
	baseline := startSample
	for _, sample := range report.RuntimeSamples {
		if !sample.Timestamp.Before(warmupEnd) {
			baseline = sample
			break
		}
	}
	finalSample := report.RuntimeSamples[len(report.RuntimeSamples)-1]

	if baseline.HeapAlloc > 0 {
		allowed := float64(baseline.HeapAlloc)*1.10 + 10*1024*1024
		if float64(finalSample.HeapAlloc) > allowed {
			t.Fatalf("heap allocation grew too much: baseline=%d final=%d", baseline.HeapAlloc, finalSample.HeapAlloc)
		}
	}
	if finalSample.Goroutines > baseline.Goroutines+20 {
		t.Fatalf("goroutine count grew too much: baseline=%d final=%d", baseline.Goroutines, finalSample.Goroutines)
	}

	if report.QueueStats.FinalDepth > len(scenario.Instances) {
		t.Fatalf("final queue depth %d exceeds instance count %d", report.QueueStats.FinalDepth, len(scenario.Instances))
	}

	healthyThreshold := 45 * time.Second
	for key, stats := range report.PerInstanceStats {
		if stats.Successes == 0 || stats.PermanentFailures > 0 {
			continue
		}
		if stats.LastSuccessAt.IsZero() {
			t.Fatalf("missing last success timestamp for %s", key)
		}
		age := time.Since(stats.LastSuccessAt)
		if age > healthyThreshold {
			t.Fatalf("instance %s staleness age %v exceeds threshold %v", key, age, healthyThreshold)
		}
	}

	if len(report.Health.DeadLetter.Tasks) != permanentCount {
		t.Fatalf("expected %d dead-letter tasks, got %d", permanentCount, len(report.Health.DeadLetter.Tasks))
	}

	for name, stats := range report.PerInstanceStats {
		if strings.Contains(name, "transient") {
			if stats.TransientFailures == 0 {
				t.Fatalf("expected transient failures for %s", name)
			}
			if stats.Successes == 0 {
				t.Fatalf("expected recoveries for %s", name)
			}
		}
	}

	if !report.Health.Enabled {
		t.Fatal("expected adaptive polling to be enabled during soak run")
	}

	t.Logf("soak run complete: instances=%d duration=%v samples=%d heap(start=%d end=%d) goroutines(start=%d end=%d)", len(scenario.Instances), duration, len(report.RuntimeSamples), baseline.HeapAlloc, finalSample.HeapAlloc, baseline.Goroutines, finalSample.Goroutines)
}
