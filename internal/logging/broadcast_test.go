package logging

import (
	"container/ring"
	"testing"
)

func TestRingHistorySnapshot_SkipsUnexpectedTypes(t *testing.T) {
	buffer := ring.New(3)
	buffer.Value = "first"
	buffer.Next().Value = 42
	buffer.Next().Next().Value = "third"

	history := ringHistorySnapshot(buffer)
	if len(history) != 2 {
		t.Fatalf("len(history) = %d, want 2 (history=%v)", len(history), history)
	}
	if history[0] != "first" || history[1] != "third" {
		t.Fatalf("unexpected history order/content: %v", history)
	}
}

func TestLogBroadcasterGetHistory_IgnoresCorruptRingValues(t *testing.T) {
	buffer := ring.New(2)
	buffer.Value = "ok"
	buffer = buffer.Next()
	buffer.Value = struct{}{}

	b := &LogBroadcaster{
		buffer:      buffer,
		subscribers: make(map[string]chan string),
	}

	history := b.GetHistory()
	if len(history) != 1 || history[0] != "ok" {
		t.Fatalf("unexpected history: %v", history)
	}
}
