package monitoring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMonitorHasSocketTemperatureProxyDetectsEnvSocket(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "pulse-sensor-proxy.sock")

	if err := os.WriteFile(socketPath, []byte("socket-placeholder"), 0o600); err != nil {
		t.Fatalf("failed to create fake socket: %v", err)
	}

	t.Setenv("PULSE_SENSOR_PROXY_SOCKET", socketPath)

	monitor := &Monitor{}
	if !monitor.HasSocketTemperatureProxy() {
		t.Fatalf("expected monitor to detect proxy socket at %s", socketPath)
	}
}
