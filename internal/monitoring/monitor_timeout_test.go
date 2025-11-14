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
}
