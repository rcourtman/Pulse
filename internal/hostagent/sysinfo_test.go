package hostagent

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewDefaultCollector(t *testing.T) {
	c := NewDefaultCollector()
	if c == nil {
		t.Fatal("NewDefaultCollector returned nil")
	}
	if _, ok := c.(*defaultCollector); !ok {
		t.Fatalf("NewDefaultCollector returned %T, want *defaultCollector", c)
	}
}

func TestDefaultCollector_Smoke(t *testing.T) {
	// These tests just ensure the wrappers don't crash and call the expected libraries.
	// We don't need to verify the actual system data here, just that the plumbing works.
	c := &defaultCollector{}

	ctx := context.Background()

	// HostInfo
	if info, _ := c.HostInfo(ctx); info == nil {
		// info might be nil on some weird systems but usually gopsutil returns something
	}

	// HostUptime
	_, _ = c.HostUptime(ctx)

	// Metrics
	_, _ = c.Metrics(ctx, nil)

	// Sensors (will return error/empty on Mac but it's fine)
	_, _ = c.SensorsLocal(ctx)
	_, _ = c.SensorsParse("{}")
	_, _ = c.SensorsPower(ctx)

	// RAID
	_, _ = c.RAIDArrays(ctx)

	// Ceph
	_, _ = c.CephStatus(ctx)

	// SMART
	_, _ = c.SMARTLocal(ctx, nil, nil)

	// Now
	if c.Now().IsZero() {
		t.Errorf("Now() returned zero time")
	}

	// GOOS
	if c.GOOS() == "" {
		t.Errorf("GOOS() returned empty string")
	}

	// ReadFile (test with non-existent file to avoid impact)
	_, _ = c.ReadFile("/non-existent-file-pulse-test")

	// NetInterfaces
	_, _ = c.NetInterfaces()

	// New methods:
	_, _ = c.Hostname()
	_, _ = c.LookupIP("localhost")
	_, _ = c.DialTimeout("udp", "8.8.8.8:53", 10*time.Millisecond)
	_, _ = c.Stat("/tmp")
	_ = c.MkdirAll("/tmp/pulse-test-dir", 0755)
	_ = c.WriteFile("/tmp/pulse-test-file", []byte("test"), 0644)
	_ = c.Chmod("/tmp/pulse-test-file", 0600)
	_, _ = c.CommandCombinedOutput(ctx, "echo", "hi")
	_, _ = c.LookPath("ls")

	// Cleanup if possible (best effort)
	os.Remove("/tmp/pulse-test-file")
}

func TestLimitedCombinedBufferCapsCapturedOutput(t *testing.T) {
	buffer := &limitedCombinedBuffer{limit: 5}
	if written, err := buffer.Write([]byte("123")); err != nil || written != 3 {
		t.Fatalf("first write = (%d, %v)", written, err)
	}
	if written, err := buffer.Write([]byte("4567")); err != nil || written != 4 {
		t.Fatalf("second write = (%d, %v)", written, err)
	}
	if got := buffer.String(); got != "12345" {
		t.Fatalf("captured output = %q, want %q", got, "12345")
	}
	if !buffer.Exceeded() {
		t.Fatal("expected buffer to record the output limit was exceeded")
	}
}
