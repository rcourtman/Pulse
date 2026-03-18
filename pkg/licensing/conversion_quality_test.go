package licensing

import (
	"sync"
	"testing"
	"time"
)

func TestPipelineHealthHealthy(t *testing.T) {
	health := NewPipelineHealth()
	health.RecordEvent(EventPaywallViewed)
	health.RecordEvent(EventTrialStarted)
	health.RecordEvent(EventUpgradeClicked)
	health.RecordEvent(EventCheckoutCompleted)

	status := health.CheckHealth()

	if status.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", status.Status)
	}
	if status.EventsTotal != 4 {
		t.Fatalf("events_total = %d, want 4", status.EventsTotal)
	}
	if len(status.EventsByType) != 4 {
		t.Fatalf("len(events_by_type) = %d, want 4", len(status.EventsByType))
	}
	if status.StartedAt <= 0 {
		t.Fatalf("started_at = %d, want > 0", status.StartedAt)
	}
}

func TestPipelineHealthStaleNoEvents(t *testing.T) {
	baseNow := time.Now()
	currentNow := baseNow
	health := NewPipelineHealth(
		WithPipelineHealthNow(func() time.Time { return currentNow }),
		WithPipelineHealthStaleThreshold(5*time.Minute),
	)
	currentNow = baseNow.Add(6 * time.Minute)

	status := health.CheckHealth()

	if status.Status != "stale" {
		t.Fatalf("status = %q, want stale", status.Status)
	}
	if status.EventsTotal != 0 {
		t.Fatalf("events_total = %d, want 0", status.EventsTotal)
	}
}

func TestPipelineHealthDegradedCoverage(t *testing.T) {
	health := NewPipelineHealth()
	for i := 0; i < 5; i++ {
		health.RecordEvent(EventPaywallViewed)
	}

	status := health.CheckHealth()

	if status.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", status.Status)
	}
	if status.EventsByType[EventPaywallViewed] != 5 {
		t.Fatalf("events_by_type[%q] = %d, want 5", EventPaywallViewed, status.EventsByType[EventPaywallViewed])
	}
}

func TestPipelineHealthColdStartHealthy(t *testing.T) {
	health := NewPipelineHealth()

	status := health.CheckHealth()

	if status.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", status.Status)
	}
	if status.EventsTotal != 0 {
		t.Fatalf("events_total = %d, want 0", status.EventsTotal)
	}
}

func TestPipelineHealthConcurrentAccess(t *testing.T) {
	health := NewPipelineHealth()
	eventTypes := KnownConversionEventTypes()

	const goroutines = 16
	const recordsPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				health.RecordEvent(eventTypes[(id+i)%len(eventTypes)])
				_ = health.CheckHealth()
			}
		}(g)
	}
	wg.Wait()

	status := health.CheckHealth()
	wantTotal := int64(goroutines * recordsPerGoroutine)
	if status.EventsTotal != wantTotal {
		t.Fatalf("events_total = %d, want %d", status.EventsTotal, wantTotal)
	}

	var summed int64
	for _, count := range status.EventsByType {
		summed += count
	}
	if summed != wantTotal {
		t.Fatalf("sum(events_by_type) = %d, want %d", summed, wantTotal)
	}
	if status.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", status.Status)
	}
}
