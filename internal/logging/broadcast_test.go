package logging

import (
	"container/ring"
	"testing"
)

func newTestBroadcaster(bufferSize int) *LogBroadcaster {
	return &LogBroadcaster{
		buffer:      ring.New(bufferSize),
		subscribers: make(map[string]chan string),
	}
}

func historyContains(history []string, want string) bool {
	for _, entry := range history {
		if entry == want {
			return true
		}
	}
	return false
}

func TestLogBroadcasterWriteBroadcastsAndHandlesBlockedSubscribers(t *testing.T) {
	b := newTestBroadcaster(4)
	fast := make(chan string, 1)
	blocked := make(chan string, 1)
	blocked <- "already-full"
	b.subscribers["fast"] = fast
	b.subscribers["blocked"] = blocked

	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len("hello") {
		t.Fatalf("Write returned %d bytes, want %d", n, len("hello"))
	}

	select {
	case got := <-fast:
		if got != "hello" {
			t.Fatalf("subscriber received %q, want %q", got, "hello")
		}
	default:
		t.Fatal("expected fast subscriber to receive message")
	}

	// Blocked channel should remain unchanged when Write hits the default branch.
	select {
	case got := <-blocked:
		if got != "already-full" {
			t.Fatalf("blocked subscriber payload changed: got %q", got)
		}
	default:
		t.Fatal("expected blocked channel to still contain original message")
	}

	history := b.GetHistory()
	if !historyContains(history, "hello") {
		t.Fatalf("expected history to contain %q, got %#v", "hello", history)
	}
}

func TestLogBroadcasterSubscribeReturnsHistoryAndRegistersSubscriber(t *testing.T) {
	b := newTestBroadcaster(6)
	_, _ = b.Write([]byte("one"))
	_, _ = b.Write([]byte("two"))

	id, ch, history := b.Subscribe()
	if id == "" {
		t.Fatal("Subscribe returned empty subscriber id")
	}
	if ch == nil {
		t.Fatal("Subscribe returned nil channel")
	}

	b.mu.RLock()
	registered, ok := b.subscribers[id]
	b.mu.RUnlock()
	if !ok {
		t.Fatal("subscriber was not registered")
	}
	if registered != ch {
		t.Fatal("registered channel does not match returned channel")
	}
	if !historyContains(history, "one") || !historyContains(history, "two") {
		t.Fatalf("expected history snapshot to contain existing messages, got %#v", history)
	}

	b.Unsubscribe(id)
}

func TestLogBroadcasterUnsubscribeRemovesAndClosesChannel(t *testing.T) {
	b := newTestBroadcaster(4)
	id, ch, _ := b.Subscribe()

	b.Unsubscribe(id)

	b.mu.RLock()
	_, ok := b.subscribers[id]
	b.mu.RUnlock()
	if ok {
		t.Fatal("expected subscriber to be removed")
	}

	select {
	case _, open := <-ch:
		if open {
			t.Fatal("expected channel to be closed")
		}
	default:
		t.Fatal("expected closed channel to be readable immediately")
	}

	// Missing subscriber should be a no-op.
	b.Unsubscribe("missing-subscriber")
}

func TestLogBroadcasterGetHistoryAfterWrap(t *testing.T) {
	b := newTestBroadcaster(3)
	_, _ = b.Write([]byte("a"))
	_, _ = b.Write([]byte("b"))
	_, _ = b.Write([]byte("c"))
	_, _ = b.Write([]byte("d"))

	history := b.GetHistory()
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3 (%#v)", len(history), history)
	}
	if historyContains(history, "a") {
		t.Fatalf("oldest entry should have been rotated out, got %#v", history)
	}
	if !historyContains(history, "b") || !historyContains(history, "c") || !historyContains(history, "d") {
		t.Fatalf("history missing expected rotated entries, got %#v", history)
	}
}

func TestGlobalLevelSetAndGet(t *testing.T) {
	t.Cleanup(resetLoggingState)

	SetGlobalLevel("warn")
	if got := GetGlobalLevel(); got != "warn" {
		t.Fatalf("GetGlobalLevel() = %q, want %q", got, "warn")
	}

	// Unknown levels fall back to info via parseLevel.
	SetGlobalLevel("not-a-level")
	if got := GetGlobalLevel(); got != "info" {
		t.Fatalf("GetGlobalLevel() with invalid level = %q, want %q", got, "info")
	}
}
