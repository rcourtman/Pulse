package dockeragent

import (
	"math"
	"reflect"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
)

func TestNormalizeTargets(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{
		{URL: " https://pulse.example.com/ ", Token: "tokenA", InsecureSkipVerify: false},
		{URL: "https://pulse.example.com", Token: "tokenA", InsecureSkipVerify: false}, // duplicate
		{URL: "https://pulse-dr.example.com", Token: "tokenB", InsecureSkipVerify: true},
	})
	if err != nil {
		t.Fatalf("normalizeTargets returned error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].URL != "https://pulse.example.com" || targets[0].Token != "tokenA" || targets[0].InsecureSkipVerify {
		t.Fatalf("unexpected first target: %+v", targets[0])
	}

	if targets[1].URL != "https://pulse-dr.example.com" || targets[1].Token != "tokenB" || !targets[1].InsecureSkipVerify {
		t.Fatalf("unexpected second target: %+v", targets[1])
	}
}

func TestNormalizeTargetsInvalid(t *testing.T) {
	if _, err := normalizeTargets([]TargetConfig{{URL: "", Token: "token"}}); err == nil {
		t.Fatalf("expected error for missing URL")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "https://pulse.example.com", Token: ""}}); err == nil {
		t.Fatalf("expected error for missing token")
	}
}

func TestNormalizeContainerStates(t *testing.T) {
	states, err := normalizeContainerStates([]string{"running", "Exited", "running", "stopped"})
	if err != nil {
		t.Fatalf("normalizeContainerStates returned error: %v", err)
	}

	expected := []string{"running", "exited"}
	if !reflect.DeepEqual(states, expected) {
		t.Fatalf("expected %v, got %v", expected, states)
	}
}

func TestNormalizeContainerStatesInvalid(t *testing.T) {
	if _, err := normalizeContainerStates([]string{"unknown"}); err == nil {
		t.Fatalf("expected error for invalid container state")
	}
}

func TestNormalizeSwarmScope(t *testing.T) {
	tests := map[string]string{
		"":        "node",
		"node":    "node",
		"NODE":    "node",
		"cluster": "cluster",
		"AUTO":    "auto",
	}

	for input, expected := range tests {
		scope, err := normalizeSwarmScope(input)
		if err != nil {
			t.Fatalf("normalizeSwarmScope(%q) returned error: %v", input, err)
		}
		if scope != expected {
			t.Fatalf("normalizeSwarmScope(%q)=%q, expected %q", input, scope, expected)
		}
	}
}

func TestNormalizeSwarmScopeInvalid(t *testing.T) {
	if _, err := normalizeSwarmScope("invalid"); err == nil {
		t.Fatalf("expected error for invalid swarm scope")
	}
}

func TestNormalizeRuntime(t *testing.T) {
	tests := map[string]RuntimeKind{
		"":       RuntimeAuto,
		"auto":   RuntimeAuto,
		"docker": RuntimeDocker,
		"podman": RuntimePodman,
		" Auto ": RuntimeAuto,
		"DOCKER": RuntimeDocker,
		"PODMAN": RuntimePodman,
	}

	for input, expected := range tests {
		runtime, err := normalizeRuntime(input)
		if err != nil {
			t.Fatalf("normalizeRuntime(%q) returned error: %v", input, err)
		}
		if runtime != expected {
			t.Fatalf("normalizeRuntime(%q) = %q, expected %q", input, runtime, expected)
		}
	}
}

func TestNormalizeRuntimeInvalid(t *testing.T) {
	if _, err := normalizeRuntime("containerd"); err == nil {
		t.Fatalf("expected error for unsupported runtime")
	}
}

func TestCalculateCPUPercent(t *testing.T) {
	tests := []struct {
		name     string
		stats    containertypes.StatsResponse
		hostCPUs int
		want     float64
	}{
		{
			name: "normal calculation",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 200000000, // 200ms
					},
					SystemUsage: 2000000000, // 2s
					OnlineCPUs:  4,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000, // 100ms
					},
					SystemUsage: 1000000000, // 1s
				},
			},
			hostCPUs: 4,
			want:     40.0, // (100ms / 1s) * 4 CPUs * 100
		},
		{
			name: "zero system delta returns zero",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 200000000,
					},
					SystemUsage: 1000000000,
					OnlineCPUs:  4,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 1000000000, // same as current = zero delta
				},
			},
			hostCPUs: 4,
			want:     0,
		},
		{
			name: "zero total delta returns zero",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 2000000000,
					OnlineCPUs:  4,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000, // same as current
					},
					SystemUsage: 1000000000,
				},
			},
			hostCPUs: 4,
			want:     0,
		},
		{
			name: "uses PercpuUsage length when OnlineCPUs is zero",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage:   200000000,
						PercpuUsage:  []uint64{1, 2, 3, 4}, // 4 CPUs
					},
					SystemUsage: 2000000000,
					OnlineCPUs:  0,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 1000000000,
				},
			},
			hostCPUs: 8,
			want:     40.0, // uses PercpuUsage length (4), not hostCPUs
		},
		{
			name: "uses hostCPUs when OnlineCPUs and PercpuUsage both zero",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 200000000,
					},
					SystemUsage: 2000000000,
					OnlineCPUs:  0,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 1000000000,
				},
			},
			hostCPUs: 2,
			want:     20.0, // uses hostCPUs (2)
		},
		{
			name: "returns zero when no CPU count available",
			stats: containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 200000000,
					},
					SystemUsage: 2000000000,
					OnlineCPUs:  0,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage: 100000000,
					},
					SystemUsage: 1000000000,
				},
			},
			hostCPUs: 0,
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCPUPercent(tt.stats, tt.hostCPUs)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("calculateCPUPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMemoryUsage(t *testing.T) {
	tests := []struct {
		name        string
		stats       containertypes.StatsResponse
		wantUsage   int64
		wantLimit   int64
		wantPercent float64
	}{
		{
			name: "normal calculation with cache",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 4000000,
					Stats: map[string]uint64{"cache": 200000},
				},
			},
			wantUsage:   800000,
			wantLimit:   4000000,
			wantPercent: 20.0,
		},
		{
			name: "no cache in stats",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 4000000,
					Stats: map[string]uint64{},
				},
			},
			wantUsage:   1000000,
			wantLimit:   4000000,
			wantPercent: 25.0,
		},
		{
			name: "cache larger than usage falls back to raw usage",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 4000000,
					Stats: map[string]uint64{"cache": 2000000}, // more than usage
				},
			},
			wantUsage:   1000000, // falls back to raw usage
			wantLimit:   4000000,
			wantPercent: 25.0,
		},
		{
			name: "zero limit returns zero percent",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 0,
					Stats: map[string]uint64{},
				},
			},
			wantUsage:   1000000,
			wantLimit:   0,
			wantPercent: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, limit, percent := calculateMemoryUsage(tt.stats)
			if usage != tt.wantUsage {
				t.Errorf("usage = %v, want %v", usage, tt.wantUsage)
			}
			if limit != tt.wantLimit {
				t.Errorf("limit = %v, want %v", limit, tt.wantLimit)
			}
			if math.Abs(percent-tt.wantPercent) > 0.001 {
				t.Errorf("percent = %v, want %v", percent, tt.wantPercent)
			}
		})
	}
}

func TestSafeFloat(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"normal positive", 42.5, 42.5},
		{"normal negative", -42.5, -42.5},
		{"zero", 0, 0},
		{"NaN returns zero", math.NaN(), 0},
		{"positive infinity returns zero", math.Inf(1), 0},
		{"negative infinity returns zero", math.Inf(-1), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeFloat(tt.input)
			if got != tt.want {
				t.Errorf("safeFloat(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"empty string", "", time.Time{}},
		{"zero time string", "0001-01-01T00:00:00Z", time.Time{}},
		{"RFC3339Nano", "2024-01-15T10:30:00.123456789Z", time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)},
		{"RFC3339", "2024-01-15T10:30:00Z", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		{"RFC3339 with offset", "2024-01-15T10:30:00+05:00", time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", 5*3600))},
		{"invalid format", "not-a-time", time.Time{}},
		{"partial date", "2024-01-15", time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTime(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("parseTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimLeadingSlash(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  string
	}{
		{"empty slice", []string{}, ""},
		{"single name with slash", []string{"/mycontainer"}, "mycontainer"},
		{"single name without slash", []string{"mycontainer"}, "mycontainer"},
		{"multiple names uses first", []string{"/first", "/second"}, "first"},
		{"name with multiple slashes only trims first", []string{"//double"}, "/double"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimLeadingSlash(tt.names)
			if got != tt.want {
				t.Errorf("trimLeadingSlash(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestSummarizeBlockIO(t *testing.T) {
	tests := []struct {
		name  string
		stats containertypes.StatsResponse
		want  *containertypes.BlkioStatEntry
	}{
		{
			name: "read and write ops",
			stats: containertypes.StatsResponse{
				BlkioStats: containertypes.BlkioStats{
					IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
						{Op: "Read", Value: 1000},
						{Op: "Write", Value: 2000},
						{Op: "Read", Value: 500},  // multiple read entries
						{Op: "Write", Value: 300}, // multiple write entries
					},
				},
			},
			want: &containertypes.BlkioStatEntry{}, // will check values separately
		},
		{
			name: "case insensitive ops",
			stats: containertypes.StatsResponse{
				BlkioStats: containertypes.BlkioStats{
					IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
						{Op: "READ", Value: 100},
						{Op: "write", Value: 200},
					},
				},
			},
			want: &containertypes.BlkioStatEntry{},
		},
		{
			name: "zero values returns nil",
			stats: containertypes.StatsResponse{
				BlkioStats: containertypes.BlkioStats{
					IoServiceBytesRecursive: []containertypes.BlkioStatEntry{},
				},
			},
			want: nil,
		},
		{
			name: "only non-read-write ops returns nil",
			stats: containertypes.StatsResponse{
				BlkioStats: containertypes.BlkioStats{
					IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
						{Op: "Sync", Value: 1000},
						{Op: "Async", Value: 2000},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeBlockIO(tt.stats)
			if tt.want == nil {
				if got != nil {
					t.Errorf("summarizeBlockIO() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("summarizeBlockIO() = nil, want non-nil")
				return
			}
		})
	}

	// Additional test for exact values
	t.Run("exact read/write values", func(t *testing.T) {
		stats := containertypes.StatsResponse{
			BlkioStats: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "Read", Value: 1000},
					{Op: "Write", Value: 2000},
					{Op: "Read", Value: 500},
				},
			},
		}
		got := summarizeBlockIO(stats)
		if got == nil {
			t.Fatal("expected non-nil result")
		}
		if got.ReadBytes != 1500 {
			t.Errorf("ReadBytes = %d, want 1500", got.ReadBytes)
		}
		if got.WriteBytes != 2000 {
			t.Errorf("WriteBytes = %d, want 2000", got.WriteBytes)
		}
	})
}
