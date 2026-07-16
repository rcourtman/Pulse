package models

import (
	"testing"
	"time"
)

// TestBranchCovAddRemovedHostAgent exercises both arms of the replace-vs-append
// decision and the descending RemovedAt sort performed by AddRemovedHostAgent:
//   - append branch (replaced == false): empty slice and no-match among entries
//   - replace branch (replaced == true, break): existing ID is overwritten in place
//   - sort.Slice comparator: entries end up newest-first by RemovedAt
//   - replace does not grow the slice
func TestBranchCovAddRemovedHostAgent(t *testing.T) {
	t.Run("append branch on empty slice", func(t *testing.T) {
		state := NewState()
		now := time.Now()
		entry := RemovedHostAgent{ID: "a-1", Hostname: "h1", RemovedAt: now}

		state.AddRemovedHostAgent(entry)

		got := state.GetRemovedHostAgents()
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != "a-1" || got[0].Hostname != "h1" {
			t.Fatalf("stored entry = %+v, want ID a-1 / Hostname h1", got[0])
		}
		if !got[0].RemovedAt.Equal(now) {
			t.Fatalf("RemovedAt = %v, want %v", got[0].RemovedAt, now)
		}
		if state.LastUpdate.IsZero() {
			t.Fatalf("AddRemovedHostAgent did not set LastUpdate")
		}
	})

	t.Run("append branch when no existing ID matches", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-2", RemovedAt: t0.Add(time.Hour)})

		// Third add with an ID that matches neither existing entry must take
		// the append branch; the loop runs but never sets replaced=true.
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-3", RemovedAt: t0.Add(2 * time.Hour)})

		got := state.GetRemovedHostAgents()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
	})

	t.Run("replace branch overwrites existing ID in place", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", Hostname: "old", RemovedAt: t0})

		// Same ID -> replace branch: slice must not grow, fields updated.
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", Hostname: "new", RemovedAt: t0.Add(time.Hour)})

		got := state.GetRemovedHostAgents()
		if len(got) != 1 {
			t.Fatalf("replace grew slice: len = %d, want 1", len(got))
		}
		if got[0].Hostname != "new" {
			t.Fatalf("entry not replaced: Hostname = %q, want %q", got[0].Hostname, "new")
		}
		if !got[0].RemovedAt.Equal(t0.Add(time.Hour)) {
			t.Fatalf("RemovedAt not replaced: got %v", got[0].RemovedAt)
		}
	})

	t.Run("sort keeps entries newest-first by RemovedAt", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		// Insert in ascending (oldest first) order; sort must reverse to descending.
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "old", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "mid", RemovedAt: t0.Add(time.Hour)})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "new", RemovedAt: t0.Add(2 * time.Hour)})

		got := state.GetRemovedHostAgents()
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		wantOrder := []string{"new", "mid", "old"}
		for i, w := range wantOrder {
			if got[i].ID != w {
				t.Fatalf("pos %d ID = %q, want %q (order=%v)", i, got[i].ID, w, ids(got))
			}
		}
	})

	t.Run("replace re-sorts using the new RemovedAt", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "b", RemovedAt: t0.Add(time.Hour)})

		// Replace "a" with a newer timestamp than "b"; after re-sort it must lead.
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a", RemovedAt: t0.Add(2 * time.Hour)})

		got := state.GetRemovedHostAgents()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].ID != "a" || got[1].ID != "b" {
			t.Fatalf("post-replace order = %v, want [a b]", ids(got))
		}
	})
}

// TestBranchCovRemoveRemovedHostAgent covers:
//   - match branch: existing ID removed, slice shrinks, LastUpdate refreshed
//   - no-match branch: unknown ID on non-empty list is a no-op (LastUpdate untouched)
//   - empty list: no iteration, no panic
//   - first-match-only: break removes exactly one matching entry
func TestBranchCovRemoveRemovedHostAgent(t *testing.T) {
	t.Run("match branch removes entry and refreshes LastUpdate", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-2", RemovedAt: t0.Add(time.Hour)})

		before := state.LastUpdate
		// Ensure time advances so a new LastUpdate would be distinguishable.
		time.Sleep(2 * time.Millisecond)
		state.RemoveRemovedHostAgent("a-1")

		got := state.GetRemovedHostAgents()
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].ID != "a-2" {
			t.Fatalf("remaining ID = %q, want %q", got[0].ID, "a-2")
		}
		if !state.LastUpdate.After(before) {
			t.Fatalf("LastUpdate not refreshed on remove: before=%v after=%v", before, state.LastUpdate)
		}
	})

	t.Run("no-match branch is a no-op on non-empty list", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", RemovedAt: t0})

		before := state.LastUpdate
		state.RemoveRemovedHostAgent("does-not-exist")

		got := state.GetRemovedHostAgents()
		if len(got) != 1 {
			t.Fatalf("no-match remove changed length: len = %d, want 1", len(got))
		}
		if got[0].ID != "a-1" {
			t.Fatalf("entry changed unexpectedly: ID = %q", got[0].ID)
		}
		if state.LastUpdate != before {
			t.Fatalf("no-match remove touched LastUpdate: before=%v after=%v", before, state.LastUpdate)
		}
	})

	t.Run("empty list does not panic and leaves nothing to remove", func(t *testing.T) {
		state := NewState()

		// Must not panic on a zero-length loop.
		state.RemoveRemovedHostAgent("anything")

		got := state.GetRemovedHostAgents()
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("removing down to empty across successive calls", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-2", RemovedAt: t0.Add(time.Hour)})

		state.RemoveRemovedHostAgent("a-1")
		state.RemoveRemovedHostAgent("a-2")

		got := state.GetRemovedHostAgents()
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0 after removing all", len(got))
		}
	})
}

// TestBranchCovGetRemovedHostAgents covers:
//   - empty state returns a usable (non-nil) empty slice
//   - non-empty state returns a content- and length-equal copy
//   - the returned slice is a defensive copy: mutating it must not affect state
func TestBranchCovGetRemovedHostAgents(t *testing.T) {
	t.Run("empty state returns non-nil empty slice", func(t *testing.T) {
		state := NewState()

		got := state.GetRemovedHostAgents()
		if got == nil {
			t.Fatalf("got == nil, want non-nil empty slice (make([]T, 0))")
		}
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("returns content- and order-equal copy", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "old", RemovedAt: t0})
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "new", RemovedAt: t0.Add(time.Hour)})

		got := state.GetRemovedHostAgents()
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		wantOrder := []string{"new", "old"}
		for i, w := range wantOrder {
			if got[i].ID != w {
				t.Fatalf("pos %d ID = %q, want %q", i, got[i].ID, w)
			}
		}
	})

	t.Run("returned slice is isolated from internal state", func(t *testing.T) {
		state := NewState()
		t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		state.AddRemovedHostAgent(RemovedHostAgent{ID: "a-1", Hostname: "h1", RemovedAt: t0})

		first := state.GetRemovedHostAgents()
		// Mutate the caller-owned copy in every way available.
		first[0].ID = "mutated"
		first[0].Hostname = "mutated"
		first[0].RemovedAt = t0.Add(999 * time.Hour)
		first[0].MachineID = "mutated-mid"

		again := state.GetRemovedHostAgents()
		if len(again) != 1 {
			t.Fatalf("len = %d, want 1", len(again))
		}
		if again[0].ID != "a-1" || again[0].Hostname != "h1" || again[0].MachineID != "" {
			t.Fatalf("internal state mutated via returned slice: %+v", again[0])
		}
		if !again[0].RemovedAt.Equal(t0) {
			t.Fatalf("internal RemovedAt mutated via returned slice: %v", again[0].RemovedAt)
		}
	})
}

// TestBranchCovUpdatePollStats covers the unconditional assignment path of
// UpdatePollStats, which has no validation branch, plus the accumulation vs.
// overwrite semantics of its fields and the fact that it deliberately does
// NOT refresh LastUpdate (unlike Add/Remove).
func TestBranchCovUpdatePollStats(t *testing.T) {
	t.Run("writes all four fields from zero baseline", func(t *testing.T) {
		state := NewState()

		state.UpdatePollStats(12.5, int64(3600), 3)

		if got := state.Stats.PollingCycles; got != 1 {
			t.Fatalf("PollingCycles = %d, want 1 (first cycle)", got)
		}
		if got := state.Performance.LastPollDuration; got != 12.5 {
			t.Fatalf("LastPollDuration = %v, want 12.5", got)
		}
		if got := state.Stats.Uptime; got != int64(3600) {
			t.Fatalf("Uptime = %d, want 3600", got)
		}
		if got := state.Stats.WebSocketClients; got != 3 {
			t.Fatalf("WebSocketClients = %d, want 3", got)
		}
	})

	t.Run("zero values are stored as-is (no guard branch)", func(t *testing.T) {
		state := NewState()

		state.UpdatePollStats(0, int64(0), 0)

		if state.Performance.LastPollDuration != 0 {
			t.Fatalf("LastPollDuration = %v, want 0", state.Performance.LastPollDuration)
		}
		if state.Stats.Uptime != 0 {
			t.Fatalf("Uptime = %d, want 0", state.Stats.Uptime)
		}
		if state.Stats.WebSocketClients != 0 {
			t.Fatalf("WebSocketClients = %d, want 0", state.Stats.WebSocketClients)
		}
		if state.Stats.PollingCycles != 1 {
			t.Fatalf("PollingCycles = %d, want 1", state.Stats.PollingCycles)
		}
	})

	t.Run("negative values are stored as-is (no validation branch)", func(t *testing.T) {
		state := NewState()

		// There is no guard against nonsensical inputs; the function assigns them.
		state.UpdatePollStats(-7.25, int64(-100), -2)

		if state.Performance.LastPollDuration != -7.25 {
			t.Fatalf("LastPollDuration = %v, want -7.25", state.Performance.LastPollDuration)
		}
		if state.Stats.Uptime != -100 {
			t.Fatalf("Uptime = %d, want -100", state.Stats.Uptime)
		}
		if state.Stats.WebSocketClients != -2 {
			t.Fatalf("WebSocketClients = %d, want -2", state.Stats.WebSocketClients)
		}
	})

	t.Run("PollingCycles accumulates while other fields overwrite", func(t *testing.T) {
		state := NewState()

		state.UpdatePollStats(1.0, int64(10), 1)
		state.UpdatePollStats(2.0, int64(20), 2)
		state.UpdatePollStats(3.0, int64(30), 3)

		if state.Stats.PollingCycles != 3 {
			t.Fatalf("PollingCycles = %d, want 3 (accumulates)", state.Stats.PollingCycles)
		}
		// These three are overwritten each call, not accumulated.
		if state.Performance.LastPollDuration != 3.0 {
			t.Fatalf("LastPollDuration = %v, want 3.0 (last value)", state.Performance.LastPollDuration)
		}
		if state.Stats.Uptime != int64(30) {
			t.Fatalf("Uptime = %d, want 30 (last value)", state.Stats.Uptime)
		}
		if state.Stats.WebSocketClients != 3 {
			t.Fatalf("WebSocketClients = %d, want 3 (last value)", state.Stats.WebSocketClients)
		}
	})

	t.Run("does not refresh LastUpdate", func(t *testing.T) {
		state := NewState()
		before := state.LastUpdate

		state.UpdatePollStats(5.0, int64(5), 5)

		if state.LastUpdate != before {
			t.Fatalf("UpdatePollStats modified LastUpdate: before=%v after=%v", before, state.LastUpdate)
		}
	})
}

// ids is a small helper that flattens a slice of RemovedHostAgent to its IDs,
// used only to produce readable failure messages.
func ids(entries []RemovedHostAgent) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}
