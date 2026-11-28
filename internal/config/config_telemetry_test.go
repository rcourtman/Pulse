package config

import "testing"

func TestLoadTelemetryDoNotTrackAndUpdateChecksEnv(t *testing.T) {
	t.Run("DO_NOT_TRACK disables telemetry when PULSE_TELEMETRY is unset", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		t.Setenv("PULSE_TELEMETRY", "")
		t.Setenv("DO_NOT_TRACK", "1")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.TelemetryEnabled {
			t.Fatal("expected TelemetryEnabled=false when DO_NOT_TRACK=1")
		}
		if !cfg.EnvOverrides["DO_NOT_TRACK"] || !cfg.EnvOverrides["telemetryEnabled"] {
			t.Fatalf("expected DO_NOT_TRACK env override, got %+v", cfg.EnvOverrides)
		}
	})

	t.Run("PULSE_TELEMETRY overrides DO_NOT_TRACK", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		t.Setenv("DO_NOT_TRACK", "1")
		t.Setenv("PULSE_TELEMETRY", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if !cfg.TelemetryEnabled {
			t.Fatal("expected explicit PULSE_TELEMETRY=true to override DO_NOT_TRACK")
		}
		if !cfg.EnvOverrides["PULSE_TELEMETRY"] {
			t.Fatalf("expected PULSE_TELEMETRY env override, got %+v", cfg.EnvOverrides)
		}
	})

	t.Run("PULSE_DISABLE_UPDATE_CHECKS disables update checks", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		t.Setenv("PULSE_DISABLE_UPDATE_CHECKS", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if !cfg.DisableUpdateChecks {
			t.Fatal("expected DisableUpdateChecks=true when PULSE_DISABLE_UPDATE_CHECKS=true")
		}
		if !cfg.EnvOverrides["PULSE_DISABLE_UPDATE_CHECKS"] {
			t.Fatalf("expected PULSE_DISABLE_UPDATE_CHECKS env override, got %+v", cfg.EnvOverrides)
		}
	})
}
