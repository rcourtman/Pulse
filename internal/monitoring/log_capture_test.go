package monitoring

import (
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
)

type monitoringLogCapture struct {
	id    string
	lines chan string
	mu    sync.Mutex
	buf   strings.Builder
}

func newMonitoringLogCapture(t *testing.T) *monitoringLogCapture {
	t.Helper()

	id, lines, _ := logging.GetBroadcaster().Subscribe()
	capture := &monitoringLogCapture{
		id:    id,
		lines: lines,
	}
	t.Cleanup(func() {
		logging.GetBroadcaster().Unsubscribe(id)
	})
	return capture
}

func (c *monitoringLogCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				return c.buf.String()
			}
			c.buf.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				c.buf.WriteByte('\n')
			}
		default:
			return c.buf.String()
		}
	}
}
