package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestDerivePollTimeout(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want time.Duration
	}{
		{
			name: "nil config falls back to default",
			cfg:  nil,
			want: defaultTaskTimeout,
		},
		{
			name: "scales with connection timeout",
			cfg: &config.Config{
				ConnectionTimeout: 45 * time.Second,
			},
			want: 90 * time.Second,
		},
		{
			name: "enforces minimum",
			cfg: &config.Config{
				ConnectionTimeout: 5 * time.Second,
			},
			want: minTaskTimeout,
		},
		{
			name: "enforces maximum",
			cfg: &config.Config{
				ConnectionTimeout: 2 * time.Minute,
			},
			want: maxTaskTimeout,
		},
		{
			name: "respects custom MaxPollTimeout",
			cfg: &config.Config{
				ConnectionTimeout: 3 * time.Minute,
				MaxPollTimeout:    10 * time.Minute,
			},
			want: 6 * time.Minute, // 2 * ConnectionTimeout, still under MaxPollTimeout
		},
		{
			name: "custom MaxPollTimeout caps at configured value",
			cfg: &config.Config{
				ConnectionTimeout: 10 * time.Minute,
				MaxPollTimeout:    5 * time.Minute,
			},
			want: 5 * time.Minute, // Capped at MaxPollTimeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derivePollTimeout(tt.cfg)
			if got != tt.want {
				t.Fatalf("derivePollTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskExecutionTimeout(t *testing.T) {
	t.Run("uses configured timeout when set", func(t *testing.T) {
		monitor := &Monitor{pollTimeout: 42 * time.Second}
		if got := monitor.taskExecutionTimeout(InstanceTypePVE); got != 42*time.Second {
			t.Fatalf("taskExecutionTimeout() = %v, want %v", got, 42*time.Second)
		}
	})

	t.Run("falls back to default when unset", func(t *testing.T) {
		var monitor Monitor
		if got := monitor.taskExecutionTimeout(InstanceTypePVE); got != defaultTaskTimeout {
			t.Fatalf("taskExecutionTimeout() = %v, want %v", got, defaultTaskTimeout)
		}
	})

	t.Run("nil Monitor returns defaultTaskTimeout", func(t *testing.T) {
		var m *Monitor
		got := m.taskExecutionTimeout(InstanceTypePVE)
		if got != defaultTaskTimeout {
			t.Fatalf("taskExecutionTimeout() = %v, want %v", got, defaultTaskTimeout)
		}
	})

	t.Run("zero pollTimeout returns defaultTaskTimeout", func(t *testing.T) {
		m := &Monitor{pollTimeout: 0}
		got := m.taskExecutionTimeout(InstanceTypePVE)
		if got != defaultTaskTimeout {
			t.Fatalf("taskExecutionTimeout() = %v, want %v", got, defaultTaskTimeout)
		}
	})

	t.Run("negative pollTimeout returns defaultTaskTimeout", func(t *testing.T) {
		m := &Monitor{pollTimeout: -5 * time.Second}
		got := m.taskExecutionTimeout(InstanceTypePVE)
		if got != defaultTaskTimeout {
			t.Fatalf("taskExecutionTimeout() = %v, want %v", got, defaultTaskTimeout)
		}
	})

	t.Run("InstanceType parameter is ignored", func(t *testing.T) {
		m := &Monitor{pollTimeout: 60 * time.Second}
		// All instance types should return the same value
		gotPVE := m.taskExecutionTimeout(InstanceTypePVE)
		gotPBS := m.taskExecutionTimeout(InstanceTypePBS)
		gotPMG := m.taskExecutionTimeout(InstanceTypePMG)
		gotUnknown := m.taskExecutionTimeout(InstanceType("unknown"))

		expected := 60 * time.Second
		if gotPVE != expected {
			t.Errorf("taskExecutionTimeout(PVE) = %v, want %v", gotPVE, expected)
		}
		if gotPBS != expected {
			t.Errorf("taskExecutionTimeout(PBS) = %v, want %v", gotPBS, expected)
		}
		if gotPMG != expected {
			t.Errorf("taskExecutionTimeout(PMG) = %v, want %v", gotPMG, expected)
		}
		if gotUnknown != expected {
			t.Errorf("taskExecutionTimeout(unknown) = %v, want %v", gotUnknown, expected)
		}
	})
}
