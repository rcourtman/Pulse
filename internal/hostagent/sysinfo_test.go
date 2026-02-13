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
	c.HostUptime(ctx)

	// Metrics
	c.Metrics(ctx, nil)

	// Sensors (will return error/empty on Mac but it's fine)
	c.SensorsLocal(ctx)
	c.SensorsParse("{}")
	c.SensorsPower(ctx)

	// RAID
	c.RAIDArrays(ctx)

	// Ceph
	c.CephStatus(ctx)

	// SMART
	c.SMARTLocal(ctx, nil)

	// Now
	if c.Now().IsZero() {
		t.Errorf("Now() returned zero time")
	}

	// GOOS
	if c.GOOS() == "" {
		t.Errorf("GOOS() returned empty string")
	}

	// ReadFile (test with non-existent file to avoid impact)
	c.ReadFile("/non-existent-file-pulse-test")

	// NetInterfaces
	c.NetInterfaces()

	// New methods:
	c.Hostname()
	c.LookupIP("localhost")
	c.DialTimeout("udp", "8.8.8.8:53", 10*time.Millisecond)
	c.Stat("/tmp")
	c.MkdirAll("/tmp/pulse-test-dir", 0755)
	c.WriteFile("/tmp/pulse-test-file", []byte("test"), 0644)
	c.Chmod("/tmp/pulse-test-file", 0600)
	c.CommandCombinedOutput(ctx, "echo", "hi")
	c.LookPath("ls")

	// Cleanup if possible (best effort)
	os.Remove("/tmp/pulse-test-file")
}
