package api

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func assertAgentConfigFetchAuditIndex(t *testing.T, tracker *agentConfigFetchAuditTracker) {
	t.Helper()
	if len(tracker.last) > maxAgentConfigFetchAudits || len(tracker.oldest) != len(tracker.last) {
		t.Fatalf("audit state: map=%d index=%d limit=%d", len(tracker.last), len(tracker.oldest), maxAgentConfigFetchAudits)
	}
	for i, entry := range tracker.oldest {
		if entry.index != i || tracker.last[entry.key] != entry {
			t.Fatalf("expiry index lost entry %d", i)
		}
		if i > 0 && entry.auditedAt.Before(tracker.oldest[(i-1)/2].auditedAt) {
			t.Fatalf("expiry index out of order at %d", i)
		}
	}
}

func fillAgentConfigFetchAuditTracker(t *testing.T, tracker *agentConfigFetchAuditTracker, start time.Time) {
	t.Helper()
	for i := 0; i < maxAgentConfigFetchAudits; i++ {
		if reason, audit := tracker.observe("org", fmt.Sprint(i), "token", "hash", start.Add(time.Duration(i)*time.Second)); !audit || reason != "first_since_start" {
			t.Fatalf("first delivery %d: audit=%v reason=%q", i, audit, reason)
		}
	}
	assertAgentConfigFetchAuditIndex(t, tracker)
}

func TestAgentConfigFetchAuditTrackerBoundsRecentKeys(t *testing.T) {
	tracker := newAgentConfigFetchAuditTracker()
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	fillAgentConfigFetchAuditTracker(t, tracker, start)
	now := start.Add(2 * time.Hour)
	for i := 0; i < 2*maxAgentConfigFetchAudits; i++ {
		if reason, audit := tracker.observe("overflow-org", fmt.Sprint(i), "token", "hash", now); !audit || reason != "capacity" {
			t.Fatalf("overflow delivery %d: audit=%v reason=%q, want capacity audit", i, audit, reason)
		}
	}
	// Overflow is fail-open for audit, not for state growth or cache churn.
	// Repeat, token and config changes for an unremembered agent all remain
	// visible; none displaces a remembered agent's useful daily suppression.
	for _, step := range []struct{ token, hash string }{{"token", "hash"}, {"rotated", "hash"}, {"rotated", "changed"}} {
		if reason, audit := tracker.observe("overflow-org", "0", step.token, step.hash, now); !audit || reason != "capacity" {
			t.Fatalf("unremembered delivery: audit=%v reason=%q", audit, reason)
		}
	}
	for minute := 0; minute < 1440; minute++ {
		if reason, audit := tracker.observe("org", "0", "token", "hash", start.Add(time.Duration(minute)*time.Minute)); audit || reason != "" {
			t.Fatalf("remembered minute poll %d was re-audited: %q", minute, reason)
		}
	}
	assertAgentConfigFetchAuditIndex(t, tracker)
	if len(tracker.last) != maxAgentConfigFetchAudits {
		t.Fatal("recent agents were evicted by new-key churn")
	}
}

func TestAgentConfigFetchAuditTrackerReusesOnlyExpiredSlots(t *testing.T) {
	tracker := newAgentConfigFetchAuditTracker()
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	fillAgentConfigFetchAuditTracker(t, tracker, start)
	before := start.Add(agentConfigFetchAuditInterval - time.Nanosecond)
	if reason, audit := tracker.observe("new-org", "0", "token", "hash", before); !audit || reason != "capacity" {
		t.Fatal("a recent entry was replaced before its daily interval")
	}
	atExpiry := start.Add(agentConfigFetchAuditInterval)
	if reason, audit := tracker.observe("new-org", "0", "token", "hash", atExpiry); !audit || reason != "first_since_start" {
		t.Fatalf("expired slot was not reused: audit=%v reason=%q", audit, reason)
	}
	if tracker.last[agentConfigFetchAuditKey{"org", "0"}] != nil || tracker.last[agentConfigFetchAuditKey{"org", "1"}] == nil {
		t.Fatal("replacement removed a recent entry instead of the oldest expired entry")
	}
	if _, audit := tracker.observe("new-org", "0", "token", "hash", atExpiry.Add(time.Minute)); audit {
		t.Fatal("admitted overflow agent did not gain repeat-poll suppression")
	}
	// Forgetting a stale slot never suppresses that agent's next fetch.
	if reason, audit := tracker.observe("org", "0", "token", "hash", atExpiry); !audit || reason != "capacity" {
		t.Fatal("an expired, displaced agent lost its next audit")
	}
	assertAgentConfigFetchAuditIndex(t, tracker)
}

func TestAgentConfigFetchAuditTrackerReordersChangedEntries(t *testing.T) {
	for _, change := range []string{"token", "config", "daily", "backward-clock"} {
		t.Run(change, func(t *testing.T) {
			tracker := newAgentConfigFetchAuditTracker()
			start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			fillAgentConfigFetchAuditTracker(t, tracker, start)
			agent, token, hash, reason := "0", "token", "hash", "token_changed"
			changedAt := start.Add(2 * time.Hour)
			oldest := "1"
			switch change {
			case "token":
				token = "rotated"
			case "config":
				hash, reason = "changed", "config_changed"
			case "daily":
				changedAt, reason = start.Add(agentConfigFetchAuditInterval), "daily"
			case "backward-clock":
				agent, token, changedAt, oldest = "10", "rotated", start.Add(-time.Hour), "10"
			}
			if got, audit := tracker.observe("org", agent, token, hash, changedAt); !audit || got != reason {
				t.Fatalf("changed delivery: audit=%v reason=%q, want %q", audit, got, reason)
			}
			assertAgentConfigFetchAuditIndex(t, tracker)
			if tracker.oldest[0].key.agentID != oldest {
				t.Fatalf("oldest agent=%s, want %s", tracker.oldest[0].key.agentID, oldest)
			}
			if _, audit := tracker.observe("other", "new", "token", "hash", start.Add(agentConfigFetchAuditInterval+time.Second)); !audit {
				t.Fatal("replacement lost a delivery")
			}
			if tracker.last[agentConfigFetchAuditKey{"org", oldest}] != nil {
				t.Fatal("replacement ignored reordered expiry")
			}
			assertAgentConfigFetchAuditIndex(t, tracker)
		})
	}
}

func TestAgentConfigFetchAuditTrackerConcurrentCapacityAndRestart(t *testing.T) {
	tracker := newAgentConfigFetchAuditTracker()
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	var audits atomic.Int32
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < maxAgentConfigFetchAudits/8; i++ {
				if _, audit := tracker.observe(fmt.Sprint(worker), fmt.Sprint(i), "token", "hash", start); audit {
					audits.Add(1)
				}
			}
		}(worker)
	}
	wg.Wait()
	if got := audits.Load(); got != 2*maxAgentConfigFetchAudits {
		t.Fatalf("concurrent first deliveries audited=%d, want %d", got, 2*maxAgentConfigFetchAudits)
	}
	assertAgentConfigFetchAuditIndex(t, tracker)
	if len(tracker.last) != maxAgentConfigFetchAudits {
		t.Fatal("concurrent delivery did not fill the bounded state")
	}
	// A restart forgets suppression, never the persisted audit trail.
	restarted := newAgentConfigFetchAuditTracker()
	for _, org := range []string{"org", "other-org"} {
		if reason, audit := restarted.observe(org, "0", "token", "hash", start); !audit || reason != "first_since_start" {
			t.Fatal("restart or organisation isolation suppressed a first delivery")
		}
	}
	var zero agentConfigFetchAuditTracker
	if _, audit := zero.observe("org", "0", "token", "hash", start); !audit {
		t.Fatal("zero-value tracker suppressed a first delivery")
	}
}

// Run with -benchtime=8192x for a fixed recent-key-churn comparison. This is
// an in-memory tracker measurement, not an installed write/CPU claim.
func BenchmarkAgentConfigFetchAuditTrackerRecentKeyChurn(b *testing.B) {
	tracker := newAgentConfigFetchAuditTracker()
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < maxAgentConfigFetchAudits; i++ {
		tracker.observe("org", fmt.Sprint(i), "token", "hash", start)
	}
	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = fmt.Sprint(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for _, key := range keys {
		tracker.observe("overflow-org", key, "token", "hash", start.Add(time.Hour))
	}
	b.StopTimer()
	b.ReportMetric(float64(len(tracker.last)), "remembered-keys")
}
