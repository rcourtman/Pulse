package alerts

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPerMetricDelayConfiguration demonstrates the per-metric delay feature
// This test shows how to configure different alert delays for different metrics
func TestPerMetricDelayConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		config        AlertConfig
		resourceType  string
		metricType    string
		expectedDelay int
		description   string
	}{
		{
			name: "CPU alert with longer delay than memory",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60, // 60 second default for guests
				},
				MetricTimeThresholds: map[string]map[string]int{
					"guest": {
						"cpu":    300, // 5 minutes for CPU (transient spikes)
						"memory": 30,  // 30 seconds for memory (persistent issues)
					},
				},
			},
			resourceType:  "guest",
			metricType:    "cpu",
			expectedDelay: 300,
			description:   "CPU alerts should wait 5 minutes due to transient spikes",
		},
		{
			name: "Memory alert with shorter delay",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60,
				},
				MetricTimeThresholds: map[string]map[string]int{
					"guest": {
						"cpu":    300,
						"memory": 30,
					},
				},
			},
			resourceType:  "guest",
			metricType:    "memory",
			expectedDelay: 30,
			description:   "Memory alerts should trigger quickly as they're persistent",
		},
		{
			name: "Disk alert falls back to resource type default",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60,
				},
				MetricTimeThresholds: map[string]map[string]int{
					"guest": {
						"cpu":    300,
						"memory": 30,
					},
				},
			},
			resourceType:  "guest",
			metricType:    "disk",
			expectedDelay: 60,
			description:   "Disk has no specific override, uses guest default (60s)",
		},
		{
			name: "Global metric delay when no type-specific base exists",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60,
					// Note: no "storage" entry, so storage uses global metric delays
				},
				MetricTimeThresholds: map[string]map[string]int{
					"all": {
						"usage": 120, // 2 minutes for storage usage alerts globally
					},
				},
			},
			resourceType:  "storage",
			metricType:    "usage",
			expectedDelay: 120,
			description:   "Storage usage uses global metric override (120s) when no type-specific base delay exists",
		},
		{
			name: "Specific override beats global override",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60,
				},
				MetricTimeThresholds: map[string]map[string]int{
					"all": {
						"cpu": 180, // 3 minutes globally
					},
					"guest": {
						"cpu": 300, // 5 minutes for guests specifically
					},
				},
			},
			resourceType:  "guest",
			metricType:    "cpu",
			expectedDelay: 300,
			description:   "Guest-specific CPU delay (300s) overrides global CPU delay (180s)",
		},
		{
			name: "Docker container with restart count alert",
			config: AlertConfig{
				TimeThresholds: map[string]int{
					"guest": 60,
				},
				MetricTimeThresholds: map[string]map[string]int{
					"docker": {
						"restartcount": 10,  // Quick notification for container restarts
						"cpu":          120, // Longer for CPU
					},
				},
			},
			resourceType:  "docker",
			metricType:    "restartcount",
			expectedDelay: 10,
			description:   "Restart count alerts trigger quickly (10s) for immediate attention",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a manager with the test config
			manager := &Manager{
				config:        tt.config,
				pendingAlerts: make(map[string]time.Time),
			}

			// Get the time threshold for this resource/metric combination
			delay := manager.getTimeThreshold("test-resource-id", tt.resourceType, tt.metricType)

			if delay != tt.expectedDelay {
				t.Errorf("Expected delay %d seconds, got %d seconds\n%s",
					tt.expectedDelay, delay, tt.description)
			}

			t.Logf("✓ %s: %d seconds", tt.description, delay)
		})
	}
}

// TestPerMetricDelayJSON demonstrates JSON serialization/deserialization
func TestPerMetricDelayJSON(t *testing.T) {
	config := AlertConfig{
		TimeThresholds: map[string]int{
			"guest": 60,
			"node":  30,
		},
		MetricTimeThresholds: map[string]map[string]int{
			"guest": {
				"cpu":    300,
				"memory": 30,
				"disk":   90,
			},
			"node": {
				"cpu":         120,
				"temperature": 180,
			},
			"all": {
				"networkout": 60,
			},
		},
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	t.Logf("Configuration JSON:\n%s", string(data))

	// Deserialize back
	var decoded AlertConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify the data round-trips correctly
	if decoded.TimeThresholds["guest"] != 60 {
		t.Errorf("TimeThresholds not preserved")
	}
	if decoded.MetricTimeThresholds["guest"]["cpu"] != 300 {
		t.Errorf("MetricTimeThresholds not preserved")
	}

	t.Log("✓ JSON serialization/deserialization works correctly")
}

// TestPerMetricDelayUseCases documents common use cases
func TestPerMetricDelayUseCases(t *testing.T) {
	t.Log("=== Per-Metric Delay Configuration Use Cases ===\n")

	useCases := []struct {
		scenario string
		config   string
		reason   string
	}{
		{
			scenario: "Production VMs: Different delays for different metrics",
			config: `{
  "timeThresholds": {
    "guest": 60
  },
  "metricTimeThresholds": {
    "guest": {
      "cpu": 300,
      "memory": 60,
      "disk": 120,
      "diskread": 180,
      "diskwrite": 180
    }
  }
}`,
			reason: `
- CPU: 5 minutes (transient spikes are normal)
- Memory: 1 minute (persistent issues need attention)
- Disk: 2 minutes (filling up gradually)
- I/O: 3 minutes (workload-dependent, can spike)`,
		},
		{
			scenario: "Proxmox Nodes: Temperature needs patience",
			config: `{
  "timeThresholds": {
    "node": 30
  },
  "metricTimeThresholds": {
    "node": {
      "cpu": 120,
      "memory": 60,
      "temperature": 300
    }
  }
}`,
			reason: `
- CPU: 2 minutes (load balancing can spike briefly)
- Memory: 1 minute (host memory issues are serious)
- Temperature: 5 minutes (fans need time to respond)`,
		},
		{
			scenario: "Docker Containers: Fast alerts for restart issues",
			config: `{
  "timeThresholds": {
    "guest": 60
  },
  "metricTimeThresholds": {
    "docker": {
      "restartcount": 10,
      "cpu": 120,
      "memory": 60
    }
  }
}`,
			reason: `
- Restart count: 10 seconds (immediate notification needed)
- CPU: 2 minutes (containers can spike during startup)
- Memory: 1 minute (OOM is a critical issue)`,
		},
		{
			scenario: "Global overrides for specific metrics across all types",
			config: `{
  "metricTimeThresholds": {
    "all": {
      "networkout": 300
    }
  }
}`,
			reason: `
- Network Out: 5 minutes globally (backups/migrations cause spikes)
- Applies to VMs, containers, nodes, hosts unless overridden`,
		},
	}

	for _, uc := range useCases {
		t.Logf("\n📋 Scenario: %s\n", uc.scenario)
		t.Logf("Configuration:\n%s\n", uc.config)
		t.Logf("Reasoning:%s\n", uc.reason)
	}
}
