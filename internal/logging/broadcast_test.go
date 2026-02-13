package logging

import (
	"strings"
	"sync"
	"testing"
)

func resetBroadcasterState() {
	broadcaster = nil
	broadcastMu = sync.Once{}
}

func TestLogBroadcasterWriteTruncatesOversizedMessages(t *testing.T) {
	resetBroadcasterState()
	t.Cleanup(resetBroadcasterState)

	b := GetBroadcaster()
	id, ch, _ := b.Subscribe()
	t.Cleanup(func() { b.Unsubscribe(id) })

	payload := strings.Repeat("a", maxBroadcastMessageBytes+64)
	n, err := b.Write([]byte(payload))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected write size %d, got %d", len(payload), n)
	}

	msg := <-ch
	if len(msg) > maxBroadcastMessageBytes {
		t.Fatalf("expected truncated message <= %d bytes, got %d", maxBroadcastMessageBytes, len(msg))
	}
	if !strings.HasSuffix(msg, broadcastTruncationTag) {
		t.Fatalf("expected truncation tag suffix, got %q", msg[len(msg)-len(broadcastTruncationTag):])
	}

	history := b.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected one history entry, got %d", len(history))
	}
	if history[0] != msg {
		t.Fatal("expected history to store the truncated message")
	}
}

func TestLogBroadcasterWriteDoesNotTruncateSmallMessages(t *testing.T) {
	resetBroadcasterState()
	t.Cleanup(resetBroadcasterState)

	b := GetBroadcaster()
	id, ch, _ := b.Subscribe()
	t.Cleanup(func() { b.Unsubscribe(id) })

	payload := "small-message"
	n, err := b.Write([]byte(payload))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected write size %d, got %d", len(payload), n)
	}

	msg := <-ch
	if msg != payload {
		t.Fatalf("expected %q, got %q", payload, msg)
	}

	history := b.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected one history entry, got %d", len(history))
	}
	if history[0] != payload {
		t.Fatalf("expected history payload %q, got %q", payload, history[0])
	}
}
