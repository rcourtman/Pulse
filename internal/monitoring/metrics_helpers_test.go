package monitoring

import (
	"testing"
	"time"
)

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal string",
			input: "proxmox",
			want:  "proxmox",
		},
		{
			name:  "empty string",
			input: "",
			want:  "unknown",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "unknown",
		},
		{
			name:  "leading whitespace",
			input: "  docker",
			want:  "docker",
		},
		{
			name:  "trailing whitespace",
			input: "docker  ",
			want:  "docker",
		},
		{
			name:  "both sides whitespace",
			input: "  docker  ",
			want:  "docker",
		},
		{
			name:  "tabs",
			input: "\ttab\t",
			want:  "tab",
		},
		{
			name:  "mixed whitespace",
			input: " \t mixed \t ",
			want:  "mixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLabel(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeNodeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal node name",
			input: "node1",
			want:  "node1",
		},
		{
			name:  "empty becomes unknown-node",
			input: "",
			want:  "unknown-node",
		},
		{
			name:  "whitespace becomes unknown-node",
			input: "   ",
			want:  "unknown-node",
		},
		{
			name:  "with whitespace trimmed",
			input: "  pve-node2  ",
			want:  "pve-node2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeNodeLabel(tt.input)
			if got != tt.want {
				t.Errorf("normalizeNodeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitInstanceKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantType     string
		wantInstance string
	}{
		{
			name:         "normal key",
			key:          "proxmox::server1",
			wantType:     "proxmox",
			wantInstance: "server1",
		},
		{
			name:         "docker key",
			key:          "docker::prod-docker",
			wantType:     "docker",
			wantInstance: "prod-docker",
		},
		{
			name:         "empty key",
			key:          "",
			wantType:     "unknown",
			wantInstance: "unknown",
		},
		{
			name:         "no separator",
			key:          "standalone",
			wantType:     "unknown",
			wantInstance: "standalone",
		},
		{
			name:         "multiple separators",
			key:          "type::instance::extra",
			wantType:     "type",
			wantInstance: "instance::extra",
		},
		{
			name:         "empty type",
			key:          "::instance",
			wantType:     "unknown",
			wantInstance: "instance",
		},
		{
			name:         "empty instance",
			key:          "type::",
			wantType:     "type",
			wantInstance: "unknown",
		},
		{
			name:         "whitespace in parts",
			key:          " proxmox :: server1 ",
			wantType:     "proxmox",
			wantInstance: "server1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotInstance := splitInstanceKey(tt.key)
			if gotType != tt.wantType || gotInstance != tt.wantInstance {
				t.Errorf("splitInstanceKey(%q) = (%q, %q), want (%q, %q)",
					tt.key, gotType, gotInstance, tt.wantType, tt.wantInstance)
			}
		})
	}
}

func TestBreakerStateToValue(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  float64
	}{
		{
			name:  "closed",
			state: "closed",
			want:  0,
		},
		{
			name:  "closed uppercase",
			state: "CLOSED",
			want:  0,
		},
		{
			name:  "closed mixed case",
			state: "Closed",
			want:  0,
		},
		{
			name:  "half_open underscore",
			state: "half_open",
			want:  1,
		},
		{
			name:  "half-open hyphen",
			state: "half-open",
			want:  1,
		},
		{
			name:  "half_open uppercase",
			state: "HALF_OPEN",
			want:  1,
		},
		{
			name:  "open",
			state: "open",
			want:  2,
		},
		{
			name:  "open uppercase",
			state: "OPEN",
			want:  2,
		},
		{
			name:  "unknown state",
			state: "invalid",
			want:  -1,
		},
		{
			name:  "empty string",
			state: "",
			want:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := breakerStateToValue(tt.state)
			if got != tt.want {
				t.Errorf("breakerStateToValue(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestSanitizeInstanceLabels(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		instance     string
		wantType     string
		wantInstance string
	}{
		{
			name:         "normal values",
			instanceType: "proxmox",
			instance:     "server1",
			wantType:     "proxmox",
			wantInstance: "server1",
		},
		{
			name:         "empty type",
			instanceType: "",
			instance:     "server1",
			wantType:     "unknown",
			wantInstance: "server1",
		},
		{
			name:         "empty instance",
			instanceType: "docker",
			instance:     "",
			wantType:     "docker",
			wantInstance: "unknown",
		},
		{
			name:         "both empty",
			instanceType: "",
			instance:     "",
			wantType:     "unknown",
			wantInstance: "unknown",
		},
		{
			name:         "whitespace trimmed",
			instanceType: "  docker  ",
			instance:     "  prod  ",
			wantType:     "docker",
			wantInstance: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotInstance := sanitizeInstanceLabels(tt.instanceType, tt.instance)
			if gotType != tt.wantType || gotInstance != tt.wantInstance {
				t.Errorf("sanitizeInstanceLabels(%q, %q) = (%q, %q), want (%q, %q)",
					tt.instanceType, tt.instance, gotType, gotInstance, tt.wantType, tt.wantInstance)
			}
		})
	}
}

func TestMakeMetricKey(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		instance     string
		wantKey      metricKey
	}{
		{
			name:         "normal values",
			instanceType: "proxmox",
			instance:     "server1",
			wantKey:      metricKey{instanceType: "proxmox", instance: "server1"},
		},
		{
			name:         "empty values become unknown",
			instanceType: "",
			instance:     "",
			wantKey:      metricKey{instanceType: "unknown", instance: "unknown"},
		},
		{
			name:         "whitespace trimmed",
			instanceType: "  docker  ",
			instance:     "  prod  ",
			wantKey:      metricKey{instanceType: "docker", instance: "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeMetricKey(tt.instanceType, tt.instance)
			if got != tt.wantKey {
				t.Errorf("makeMetricKey(%q, %q) = %+v, want %+v",
					tt.instanceType, tt.instance, got, tt.wantKey)
			}
		})
	}
}

func TestMakeNodeMetricKey(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		instance     string
		node         string
		wantKey      nodeMetricKey
	}{
		{
			name:         "normal values",
			instanceType: "proxmox",
			instance:     "server1",
			node:         "node1",
			wantKey:      nodeMetricKey{instanceType: "proxmox", instance: "server1", node: "node1"},
		},
		{
			name:         "empty node becomes unknown",
			instanceType: "proxmox",
			instance:     "server1",
			node:         "",
			wantKey:      nodeMetricKey{instanceType: "proxmox", instance: "server1", node: "unknown"},
		},
		{
			name:         "all empty",
			instanceType: "",
			instance:     "",
			node:         "",
			wantKey:      nodeMetricKey{instanceType: "unknown", instance: "unknown", node: "unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeNodeMetricKey(tt.instanceType, tt.instance, tt.node)
			if got != tt.wantKey {
				t.Errorf("makeNodeMetricKey(%q, %q, %q) = %+v, want %+v",
					tt.instanceType, tt.instance, tt.node, got, tt.wantKey)
			}
		})
	}
}

func TestStoreNodeLastSuccess(t *testing.T) {
	t.Run("stores timestamp correctly", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}
		ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

		pm.storeNodeLastSuccess("proxmox", "server1", "node1", ts)

		key := makeNodeMetricKey("proxmox", "server1", "node1")
		got, ok := pm.nodeLastSuccessByKey[key]
		if !ok {
			t.Fatal("expected key to exist in map")
		}
		if !got.Equal(ts) {
			t.Errorf("stored timestamp = %v, want %v", got, ts)
		}
	})

	t.Run("overwrites existing value", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}
		ts1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		ts2 := time.Date(2025, 1, 15, 11, 45, 0, 0, time.UTC)

		pm.storeNodeLastSuccess("proxmox", "server1", "node1", ts1)
		pm.storeNodeLastSuccess("proxmox", "server1", "node1", ts2)

		key := makeNodeMetricKey("proxmox", "server1", "node1")
		got := pm.nodeLastSuccessByKey[key]
		if !got.Equal(ts2) {
			t.Errorf("stored timestamp = %v, want %v", got, ts2)
		}
	})

	t.Run("multiple distinct keys stored independently", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}
		ts1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
		ts2 := time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC)
		ts3 := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

		pm.storeNodeLastSuccess("proxmox", "server1", "node1", ts1)
		pm.storeNodeLastSuccess("proxmox", "server1", "node2", ts2)
		pm.storeNodeLastSuccess("docker", "prod", "host1", ts3)

		key1 := makeNodeMetricKey("proxmox", "server1", "node1")
		key2 := makeNodeMetricKey("proxmox", "server1", "node2")
		key3 := makeNodeMetricKey("docker", "prod", "host1")

		if got := pm.nodeLastSuccessByKey[key1]; !got.Equal(ts1) {
			t.Errorf("key1 timestamp = %v, want %v", got, ts1)
		}
		if got := pm.nodeLastSuccessByKey[key2]; !got.Equal(ts2) {
			t.Errorf("key2 timestamp = %v, want %v", got, ts2)
		}
		if got := pm.nodeLastSuccessByKey[key3]; !got.Equal(ts3) {
			t.Errorf("key3 timestamp = %v, want %v", got, ts3)
		}
	})
}

func TestLastNodeSuccessFor(t *testing.T) {
	t.Run("returns time and true for existing key", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}
		ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		key := makeNodeMetricKey("proxmox", "server1", "node1")
		pm.nodeLastSuccessByKey[key] = ts

		got, ok := pm.lastNodeSuccessFor("proxmox", "server1", "node1")
		if !ok {
			t.Error("expected ok to be true")
		}
		if !got.Equal(ts) {
			t.Errorf("returned timestamp = %v, want %v", got, ts)
		}
	})

	t.Run("returns zero time and false for non-existent key", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}

		got, ok := pm.lastNodeSuccessFor("proxmox", "server1", "nonexistent")
		if ok {
			t.Error("expected ok to be false")
		}
		if !got.IsZero() {
			t.Errorf("expected zero time, got %v", got)
		}
	})

	t.Run("retrieves correct value after store", func(t *testing.T) {
		pm := &PollMetrics{
			nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		}
		ts := time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)

		pm.storeNodeLastSuccess("docker", "prod", "worker1", ts)

		got, ok := pm.lastNodeSuccessFor("docker", "prod", "worker1")
		if !ok {
			t.Error("expected ok to be true")
		}
		if !got.Equal(ts) {
			t.Errorf("returned timestamp = %v, want %v", got, ts)
		}
	})
}
