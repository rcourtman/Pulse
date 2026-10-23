package configapi

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Several tests here assert on what the handlers log. Assigning log.Logger in
// a test races with goroutines an earlier test left running, such as a
// websocket writer that logs as it exits. The package installs one logger
// before any test runs, and tests redirect its output under a lock instead.
type testLogSink struct {
	mu  sync.Mutex
	out io.Writer
	min zerolog.Level
}

func (s *testLogSink) Write(p []byte) (int, error) { return s.WriteLevel(zerolog.NoLevel, p) }

func (s *testLogSink) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if level != zerolog.NoLevel && level < s.min {
		return len(p), nil
	}
	return s.out.Write(p)
}

var packageLogs = &testLogSink{out: os.Stderr, min: zerolog.TraceLevel}

func TestMain(m *testing.M) {
	log.Logger = zerolog.New(packageLogs).With().Timestamp().Logger()
	os.Exit(m.Run())
}

// capturedLogs is safe to read while a stray goroutine may still be writing.
type capturedLogs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *capturedLogs) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *capturedLogs) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func (c *capturedLogs) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Len()
}

// Reader returns what has been captured so far.
func (c *capturedLogs) Reader() io.Reader {
	return bytes.NewReader([]byte(c.String()))
}

// captureLogs sends the package logger's output at min and above to a buffer
// until the test ends.
func captureLogs(t *testing.T, min zerolog.Level) *capturedLogs {
	t.Helper()
	captured := &capturedLogs{}
	packageLogs.mu.Lock()
	priorOut, priorMin := packageLogs.out, packageLogs.min
	packageLogs.out, packageLogs.min = captured, min
	packageLogs.mu.Unlock()
	t.Cleanup(func() {
		packageLogs.mu.Lock()
		packageLogs.out, packageLogs.min = priorOut, priorMin
		packageLogs.mu.Unlock()
	})
	return captured
}
