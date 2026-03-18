package monitoring

import (
	"os"
	"testing"
)

func TestZFSMonitoringEnabledFromEnv(t *testing.T) {
	const envKey = "PULSE_DISABLE_ZFS_MONITORING"

	t.Run("unset env keeps zfs monitoring enabled", func(t *testing.T) {
		original, hadOriginal := os.LookupEnv(envKey)
		if err := os.Unsetenv(envKey); err != nil {
			t.Fatalf("unset env: %v", err)
		}
		t.Cleanup(func() {
			if hadOriginal {
				_ = os.Setenv(envKey, original)
				return
			}
			_ = os.Unsetenv(envKey)
		})

		if !zfsMonitoringEnabledFromEnv() {
			t.Fatal("expected zfs monitoring enabled when env is unset")
		}
	})

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "explicit true disables monitoring", value: "true", want: false},
		{name: "whitespace and uppercase true disables monitoring", value: " TRUE ", want: false},
		{name: "numeric true disables monitoring", value: "1", want: false},
		{name: "explicit false keeps monitoring enabled", value: "false", want: true},
		{name: "numeric false keeps monitoring enabled", value: "0", want: true},
		{name: "blank value falls back to enabled", value: "   ", want: true},
		{name: "invalid value falls back to enabled", value: "definitely-not-a-bool", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKey, tt.value)

			if got := zfsMonitoringEnabledFromEnv(); got != tt.want {
				t.Fatalf("zfsMonitoringEnabledFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
