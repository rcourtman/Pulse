package logging

import (
	"bytes"
	"container/ring"
	"context"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
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

// Run the writer in a child whose stderr is an unread pipe. The configured
// sink remains writable. Any per-drop stderr diagnostic fills that pipe and
// stalls the logging caller, even though the viewer's channel is nonblocking.
// The timeout kills and reaps only this test child, not a service or a guest.
func TestLogBroadcasterSlowViewerDoesNotWriteStderr(t *testing.T) {
	const childEnv = "PULSE_TEST_SLOW_LOG_VIEWER_CHILD"
	if os.Getenv(childEnv) == "1" {
		b := newTestBroadcaster(4)
		fast := make(chan string, 1)
		slow := make(chan string, 1)
		slow <- "queued"
		b.subscribers["fast"] = fast
		b.subscribers["slow"] = slow

		var sink bytes.Buffer
		logger := zerolog.New(io.MultiWriter(&sink, b))
		originalLevel := zerolog.GlobalLevel()
		defer zerolog.SetGlobalLevel(originalLevel)
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		const line = "{\"level\":\"debug\",\"message\":\"normal polling fixture\"}\n"
		const records = 32768
		for i := 0; i < records; i++ {
			logger.Debug().Msg("normal polling fixture")
			select {
			case got := <-fast:
				if got != line {
					t.Fatalf("fast viewer received %q, want %q", got, line)
				}
			default:
				t.Fatal("slow viewer prevented delivery to fast viewer")
			}
		}
		if got, want := sink.String(), strings.Repeat(line, records); got != want {
			t.Fatal("slow viewer changed the configured log sink")
		}
		if got := <-slow; got != "queued" {
			t.Fatalf("slow viewer's queue was changed: %q", got)
		}
		if got, want := b.GetHistory(), []string{line, line, line, line}; !slices.Equal(got, want) {
			t.Fatalf("slow viewer changed bounded history: %#v", got)
		}
		b.Shutdown()
		return
	}

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = writePipe.Close()
		_ = readPipe.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLogBroadcasterSlowViewerDoesNotWriteStderr$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = writePipe
	cmd.WaitDelay = 2 * time.Second
	runErr := cmd.Run()
	_ = writePipe.Close()
	stderr, readErr := io.ReadAll(readPipe)
	if runErr != nil {
		t.Fatalf("logging caller stalled or child failed: %v (deadline: %v, stderr bytes: %d)\n%s", runErr, ctx.Err(), len(stderr), output.String())
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(stderr) != 0 {
		t.Fatalf("dropping viewer deliveries wrote %d bytes to stderr", len(stderr))
	}
}

func TestLogBroadcasterWriteBoundsAndCopiesMessages(t *testing.T) {
	for _, size := range []int{0, 1, maxBroadcastMessageBytes, maxBroadcastMessageBytes + 1, 1024 * 1024} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			b := newTestBroadcaster(2)
			ch := make(chan string, 1)
			b.subscribers["viewer"] = ch
			payload := bytes.Repeat([]byte{'x'}, size)
			want := string(payload)
			if size > maxBroadcastMessageBytes {
				want = want[:maxBroadcastMessageBytes-len(broadcastTruncationTag)] + broadcastTruncationTag
			}
			n, err := b.Write(payload)
			if err != nil || n != size {
				t.Fatalf("Write = %d, %v; want %d, nil", n, err, size)
			}
			for i := range payload {
				payload[i] = 'y'
			}
			if got := <-ch; got != want {
				t.Fatalf("viewer message size/content changed: length %d, want %d", len(got), len(want))
			}
			if got := b.GetHistory(); !slices.Equal(got, []string{want}) {
				t.Fatal("history did not retain the bounded independent copy")
			}
		})
	}
}

func TestLogBroadcasterSlowViewerDoesNotBlockConcurrentLifecycle(t *testing.T) {
	b := newTestBroadcaster(4)
	_, slow, _ := b.Subscribe()
	for i := 0; i < cap(slow); i++ {
		slow <- "queued"
	}

	var writers sync.WaitGroup
	for i := 0; i < 4; i++ {
		writers.Go(func() {
			for i := 0; i < 1000; i++ {
				n, err := b.Write([]byte("poll"))
				if err != nil || n != len("poll") {
					t.Errorf("Write = %d, %v", n, err)
				}
			}
		})
	}
	for i := 0; i < 2; i++ {
		writers.Go(func() {
			for i := 0; i < 100; i++ {
				id, _, _ := b.Subscribe()
				b.GetHistory()
				b.Unsubscribe(id)
				b.Shutdown()
			}
		})
	}
	writers.Wait()
	b.Shutdown()
	if len(b.subscribers) != 0 {
		t.Fatal("subscriber registrations survived shutdown")
	}
	for range slow {
	}
	if _, err := b.Write([]byte("after shutdown")); err != nil {
		t.Fatal(err)
	}
	if history := b.GetHistory(); history[len(history)-1] != "after shutdown" {
		t.Fatal("shutdown lost subsequent history")
	}
}

func BenchmarkLogBroadcasterOversizedMessage(b *testing.B) {
	writer := newTestBroadcaster(4)
	payload := bytes.Repeat([]byte{'x'}, 1024*1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = writer.Write(payload)
	}
}

func TestLogBroadcasterOversizedMessageAllocationBound(t *testing.T) {
	measured := testing.Benchmark(BenchmarkLogBroadcasterOversizedMessage)
	// Leave room for bookkeeping while rejecting a copy of the 1 MiB input.
	// This is an allocation bound, not a timing or process-RSS assertion.
	if got, limit := measured.AllocedBytesPerOp(), int64(2*maxBroadcastMessageBytes); got > limit {
		t.Fatalf("broadcaster allocated %d bytes per capped record, want <= %d", got, limit)
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
