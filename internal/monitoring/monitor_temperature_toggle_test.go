package monitoring

import (
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestMonitorTemperatureCollectorToggle(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{TemperatureMonitoringEnabled: false}

	service := newTemperatureService(false, "root", filepath.Join(t.TempDir(), "id_ed25519_sensors"), 22)

	m := &Monitor{
		config:      cfg,
		tempService: service,
	}

	if provider := m.temperatureService(); provider == nil {
		t.Fatalf("expected temperature service to be present")
	} else if provider.Enabled() {
		t.Fatalf("expected service to start disabled")
	}

	m.EnableTemperatureMonitoring()
	if !cfg.TemperatureMonitoringEnabled {
		t.Fatalf("expected config flag to be true after enabling")
	}
	if provider := m.temperatureService(); provider == nil || !provider.Enabled() {
		t.Fatalf("expected service to be enabled after EnableTemperatureMonitoring")
	}

	m.DisableTemperatureMonitoring()
	if cfg.TemperatureMonitoringEnabled {
		t.Fatalf("expected config flag to be false after disabling")
	}
	if provider := m.temperatureService(); provider == nil || provider.Enabled() {
		t.Fatalf("expected service to be disabled after DisableTemperatureMonitoring")
	}
}
