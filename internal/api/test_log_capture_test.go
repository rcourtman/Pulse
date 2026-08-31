package api

import (
	"bytes"
	"sync"
	"testing"
)

// lockedLogBuffer permits assertions while unrelated package goroutines are
// still emitting log records.
type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// synchronizedTestLogSink remains installed for the lifetime of the test
// process. Tests change only its protected destination, never zerolog's
// process-global logger, so background monitor goroutines can log safely.
type synchronizedTestLogSink struct {
	mu     sync.Mutex
	target *lockedLogBuffer
}

func (s *synchronizedTestLogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.target == nil {
		return len(p), nil
	}
	return s.target.Write(p)
}

var (
	testLogSink        = &synchronizedTestLogSink{}
	testLogCaptureGate = func() chan struct{} {
		gate := make(chan struct{}, 1)
		gate <- struct{}{}
		return gate
	}()
)

// captureTestLogs serializes the small number of tests that need exact log
// assertions, while all other test logging continues through the stable sink.
func captureTestLogs(t testing.TB) *lockedLogBuffer {
	t.Helper()
	<-testLogCaptureGate

	buf := &lockedLogBuffer{}
	testLogSink.mu.Lock()
	testLogSink.target = buf
	testLogSink.mu.Unlock()

	t.Cleanup(func() {
		testLogSink.mu.Lock()
		if testLogSink.target == buf {
			testLogSink.target = nil
		}
		testLogSink.mu.Unlock()
		testLogCaptureGate <- struct{}{}
	})
	return buf
}
