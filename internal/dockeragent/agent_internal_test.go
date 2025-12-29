package dockeragent

import (
	"errors"
	"io"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	systemtypes "github.com/docker/docker/api/types/system"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
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

func TestNormalizeTargetsSkipsEmpty(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{
		{URL: "", Token: ""},
		{URL: "https://pulse.example.com", Token: "token"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
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

func TestNormalizeContainerStatesEmpty(t *testing.T) {
	states, err := normalizeContainerStates(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if states != nil {
		t.Fatalf("expected nil, got %v", states)
	}

	states, err = normalizeContainerStates([]string{"", "  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected empty slice, got %v", states)
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
						TotalUsage:  200000000,
						PercpuUsage: []uint64{1, 2, 3, 4}, // 4 CPUs
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

func TestExtractPodmanMetadata(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   func(*testing.T, *agentsdocker.PodmanContainer)
	}{
		{
			name:   "nil labels returns nil",
			labels: nil,
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
			},
		},
		{
			name:   "empty labels returns nil",
			labels: map[string]string{},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
			},
		},
		{
			name: "unrelated labels returns nil",
			labels: map[string]string{
				"com.docker.compose.project": "myproject",
				"maintainer":                 "test@example.com",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
			},
		},
		{
			name: "pod metadata",
			labels: map[string]string{
				"io.podman.annotations.pod.name": "mypod",
				"io.podman.annotations.pod.id":   "abc123",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.PodName != "mypod" {
					t.Errorf("PodName = %q, want %q", got.PodName, "mypod")
				}
				if got.PodID != "abc123" {
					t.Errorf("PodID = %q, want %q", got.PodID, "abc123")
				}
			},
		},
		{
			name: "infra container true",
			labels: map[string]string{
				"io.podman.annotations.pod.infra": "true",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if !got.Infra {
					t.Error("Infra should be true")
				}
			},
		},
		{
			name: "infra container yes",
			labels: map[string]string{
				"io.podman.annotations.pod.infra": "yes",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if !got.Infra {
					t.Error("Infra should be true for 'yes' value")
				}
			},
		},
		{
			name: "infra container false",
			labels: map[string]string{
				"io.podman.annotations.pod.name":  "mypod",
				"io.podman.annotations.pod.infra": "false",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.Infra {
					t.Error("Infra should be false")
				}
			},
		},
		{
			name: "compose metadata",
			labels: map[string]string{
				"io.podman.compose.project":     "myproject",
				"io.podman.compose.service":     "web",
				"io.podman.compose.working_dir": "/app",
				"io.podman.compose.config-hash": "sha256:abc123",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.ComposeProject != "myproject" {
					t.Errorf("ComposeProject = %q, want %q", got.ComposeProject, "myproject")
				}
				if got.ComposeService != "web" {
					t.Errorf("ComposeService = %q, want %q", got.ComposeService, "web")
				}
				if got.ComposeWorkdir != "/app" {
					t.Errorf("ComposeWorkdir = %q, want %q", got.ComposeWorkdir, "/app")
				}
				if got.ComposeConfig != "sha256:abc123" {
					t.Errorf("ComposeConfig = %q, want %q", got.ComposeConfig, "sha256:abc123")
				}
			},
		},
		{
			name: "auto-update metadata",
			labels: map[string]string{
				"io.containers.autoupdate":         "registry",
				"io.containers.autoupdate.restart": "true",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.AutoUpdatePolicy != "registry" {
					t.Errorf("AutoUpdatePolicy = %q, want %q", got.AutoUpdatePolicy, "registry")
				}
				if got.AutoUpdateRestart != "true" {
					t.Errorf("AutoUpdateRestart = %q, want %q", got.AutoUpdateRestart, "true")
				}
			},
		},
		{
			name: "user namespace from podman annotations",
			labels: map[string]string{
				"io.podman.annotations.userns": "keep-id",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.UserNS != "keep-id" {
					t.Errorf("UserNS = %q, want %q", got.UserNS, "keep-id")
				}
			},
		},
		{
			name: "user namespace from io.containers",
			labels: map[string]string{
				"io.containers.userns": "auto",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.UserNS != "auto" {
					t.Errorf("UserNS = %q, want %q", got.UserNS, "auto")
				}
			},
		},
		{
			name: "podman annotation takes precedence over io.containers for userns",
			labels: map[string]string{
				"io.podman.annotations.userns": "keep-id",
				"io.containers.userns":         "auto",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.UserNS != "keep-id" {
					t.Errorf("UserNS = %q, want %q (podman annotation should take precedence)", got.UserNS, "keep-id")
				}
			},
		},
		{
			name: "whitespace is trimmed",
			labels: map[string]string{
				"io.podman.annotations.pod.name": "  mypod  ",
				"io.podman.compose.project":      "\tproject\t",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.PodName != "mypod" {
					t.Errorf("PodName = %q, want %q", got.PodName, "mypod")
				}
				if got.ComposeProject != "project" {
					t.Errorf("ComposeProject = %q, want %q", got.ComposeProject, "project")
				}
			},
		},
		{
			name: "full realistic example",
			labels: map[string]string{
				"io.podman.annotations.pod.name":   "webapp-pod",
				"io.podman.annotations.pod.id":     "deadbeef1234",
				"io.podman.annotations.pod.infra":  "false",
				"io.podman.compose.project":        "myapp",
				"io.podman.compose.service":        "api",
				"io.podman.compose.working_dir":    "/home/user/myapp",
				"io.podman.compose.config-hash":    "sha256:abc",
				"io.containers.autoupdate":         "local",
				"io.containers.autoupdate.restart": "always",
				"io.podman.annotations.userns":     "keep-id:uid=1000,gid=1000",
			},
			want: func(t *testing.T, got *agentsdocker.PodmanContainer) {
				if got == nil {
					t.Fatal("expected non-nil")
				}
				if got.PodName != "webapp-pod" {
					t.Errorf("PodName = %q, want %q", got.PodName, "webapp-pod")
				}
				if got.PodID != "deadbeef1234" {
					t.Errorf("PodID = %q, want %q", got.PodID, "deadbeef1234")
				}
				if got.Infra {
					t.Error("Infra should be false")
				}
				if got.ComposeProject != "myapp" {
					t.Errorf("ComposeProject = %q, want %q", got.ComposeProject, "myapp")
				}
				if got.ComposeService != "api" {
					t.Errorf("ComposeService = %q, want %q", got.ComposeService, "api")
				}
				if got.AutoUpdatePolicy != "local" {
					t.Errorf("AutoUpdatePolicy = %q, want %q", got.AutoUpdatePolicy, "local")
				}
				if got.UserNS != "keep-id:uid=1000,gid=1000" {
					t.Errorf("UserNS = %q, want %q", got.UserNS, "keep-id:uid=1000,gid=1000")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPodmanMetadata(tt.labels)
			tt.want(t, got)
		})
	}
}

func TestDetectRuntime(t *testing.T) {
	tests := []struct {
		name       string
		info       systemtypes.Info
		endpoint   string
		preference RuntimeKind
		want       RuntimeKind
	}{
		{
			name:       "preference podman returns podman",
			info:       systemtypes.Info{},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimePodman,
			want:       RuntimePodman,
		},
		{
			name:       "podman in endpoint lowercase",
			info:       systemtypes.Info{},
			endpoint:   "unix:///run/podman/podman.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name:       "libpod in endpoint",
			info:       systemtypes.Info{},
			endpoint:   "unix:///run/user/1000/libpod/libpod.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name:       "podman in endpoint uppercase",
			info:       systemtypes.Info{},
			endpoint:   "unix:///run/PODMAN/podman.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name: "podman in InitBinary",
			info: systemtypes.Info{
				InitBinary: "/usr/bin/podman",
			},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name: "podman in ServerVersion",
			info: systemtypes.Info{
				ServerVersion: "4.5.0-podman",
			},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name: "podman in DriverStatus",
			info: systemtypes.Info{
				DriverStatus: [][2]string{
					{"Driver", "overlay"},
					{"Backing Filesystem", "extfs"},
					{"Supports d_type", "true"},
					{"Using metacopy", "podman-default"},
				},
			},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name: "podman in SecurityOptions",
			info: systemtypes.Info{
				SecurityOptions: []string{
					"name=seccomp,profile=default",
					"name=rootless,podman",
				},
			},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimePodman,
		},
		{
			name:       "preference docker returns docker",
			info:       systemtypes.Info{},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeDocker,
			want:       RuntimeDocker,
		},
		{
			name:       "auto with docker socket returns docker",
			info:       systemtypes.Info{},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimeDocker,
		},
		{
			name: "docker info with no podman indicators returns docker",
			info: systemtypes.Info{
				InitBinary:      "docker-init",
				ServerVersion:   "24.0.5",
				DriverStatus:    [][2]string{{"Driver", "overlay2"}},
				SecurityOptions: []string{"name=seccomp,profile=default"},
			},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimeAuto,
			want:       RuntimeDocker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectRuntime(tt.info, tt.endpoint, tt.preference)
			if got != tt.want {
				t.Errorf("detectRuntime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRuntimeCandidates(t *testing.T) {
	tests := []struct {
		name       string
		preference RuntimeKind
		wantMin    int // minimum expected candidates
	}{
		{
			name:       "auto includes both podman and docker sockets",
			preference: RuntimeAuto,
			wantMin:    3, // at least env defaults + podman rootless + docker default
		},
		{
			name:       "docker preference includes docker socket",
			preference: RuntimeDocker,
			wantMin:    2, // at least env defaults + docker default
		},
		{
			name:       "podman preference includes podman sockets",
			preference: RuntimePodman,
			wantMin:    3, // at least env defaults + podman rootless + podman system
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := buildRuntimeCandidates(tt.preference)
			if len(candidates) < tt.wantMin {
				t.Errorf("buildRuntimeCandidates(%v) returned %d candidates, want at least %d", tt.preference, len(candidates), tt.wantMin)
			}

			// Verify first candidate is always "environment defaults"
			if len(candidates) > 0 && candidates[0].label != "environment defaults" {
				t.Errorf("first candidate should be 'environment defaults', got %q", candidates[0].label)
			}

			// Verify no duplicates
			seen := make(map[string]bool)
			for _, c := range candidates {
				key := c.host
				if key == "" {
					key = "__default__"
				}
				if seen[key] {
					t.Errorf("duplicate candidate with host %q", c.host)
				}
				seen[key] = true
			}
		})
	}
}

func TestBuildRuntimeCandidatesContent(t *testing.T) {
	// Test that docker preference doesn't include podman sockets
	t.Run("docker excludes podman sockets", func(t *testing.T) {
		candidates := buildRuntimeCandidates(RuntimeDocker)
		for _, c := range candidates {
			if c.label == "podman rootless socket" || c.label == "podman system socket" {
				t.Errorf("docker preference should not include %q", c.label)
			}
		}
	})

	// Test that podman preference doesn't include default docker socket
	t.Run("podman excludes default docker socket", func(t *testing.T) {
		candidates := buildRuntimeCandidates(RuntimePodman)
		for _, c := range candidates {
			if c.label == "default docker socket" {
				t.Errorf("podman preference should not include %q", c.label)
			}
		}
	})

	// Test that auto includes both
	t.Run("auto includes both docker and podman", func(t *testing.T) {
		candidates := buildRuntimeCandidates(RuntimeAuto)
		hasDocker := false
		hasPodman := false
		for _, c := range candidates {
			if c.label == "default docker socket" {
				hasDocker = true
			}
			if c.label == "podman rootless socket" || c.label == "podman system socket" {
				hasPodman = true
			}
		}
		if !hasDocker {
			t.Error("auto preference should include docker socket")
		}
		if !hasPodman {
			t.Error("auto preference should include podman sockets")
		}
	})
}

func TestBuildRuntimeCandidatesEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/docker.sock")
	t.Setenv("CONTAINER_HOST", "unix:///tmp/container.sock")
	t.Setenv("PODMAN_HOST", "unix:///tmp/podman.sock")

	candidates := buildRuntimeCandidates(RuntimeAuto)
	labels := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		labels[c.label] = struct{}{}
	}

	for _, label := range []string{"DOCKER_HOST", "CONTAINER_HOST", "PODMAN_HOST"} {
		if _, ok := labels[label]; !ok {
			t.Fatalf("expected candidate %q", label)
		}
	}
}

func TestRandomDuration(t *testing.T) {
	tests := []struct {
		name string
		max  time.Duration
	}{
		{"zero max returns zero", 0},
		{"negative max returns zero", -time.Second},
		{"positive max returns value in range", 5 * time.Second},
		{"large max works", time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.max <= 0 {
				got := randomDuration(tt.max)
				if got != 0 {
					t.Errorf("randomDuration(%v) = %v, want 0", tt.max, got)
				}
				return
			}

			// For positive max, verify the result is in range
			// Run multiple times to check randomness
			for i := 0; i < 10; i++ {
				got := randomDuration(tt.max)
				if got < 0 || got >= tt.max {
					t.Errorf("randomDuration(%v) = %v, want [0, %v)", tt.max, got, tt.max)
				}
			}
		})
	}

	// Test that results are actually random (not all the same)
	t.Run("results vary", func(t *testing.T) {
		max := 10 * time.Second
		results := make(map[time.Duration]bool)
		for i := 0; i < 100; i++ {
			results[randomDuration(max)] = true
		}
		// With 100 iterations, we should get more than 1 unique result
		if len(results) < 2 {
			t.Error("randomDuration appears to not be random")
		}
	})
}

func TestRandomDurationRandError(t *testing.T) {
	swap(t, &randIntFn, func(io.Reader, *big.Int) (*big.Int, error) {
		return nil, errors.New("boom")
	})

	if got := randomDuration(10 * time.Second); got != 0 {
		t.Fatalf("expected 0 on rand error, got %v", got)
	}
}

func TestDetermineSelfUpdateArch(t *testing.T) {
	// This test validates the function returns a valid result
	// The actual result depends on the runtime environment
	got := determineSelfUpdateArch()

	// Should be one of the known values or empty (if architecture unknown)
	validArches := map[string]bool{
		"":            true,
		"linux-amd64": true,
		"linux-arm64": true,
		"linux-armv7": true,
	}

	if !validArches[got] {
		t.Errorf("determineSelfUpdateArch() = %q, want one of %v", got, validArches)
	}

	// On most test environments, we should get a non-empty result
	// But we can't assert this as it depends on the machine
}

func TestDetectHostRemovedError(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "empty body returns empty",
			body: []byte{},
			want: "",
		},
		{
			name: "nil body returns empty",
			body: nil,
			want: "",
		},
		{
			name: "invalid JSON returns empty",
			body: []byte("not json"),
			want: "",
		},
		{
			name: "wrong error code returns empty",
			body: []byte(`{"error": "host was removed", "code": "invalid_token"}`),
			want: "",
		},
		{
			name: "correct code but wrong error message returns empty",
			body: []byte(`{"error": "invalid request", "code": "invalid_report"}`),
			want: "",
		},
		{
			name: "host removed error returns message",
			body: []byte(`{"error": "Docker host xyz was removed from this Pulse instance", "code": "invalid_report"}`),
			want: "Docker host xyz was removed from this Pulse instance",
		},
		{
			name: "case insensitive code matching",
			body: []byte(`{"error": "host was removed", "code": "INVALID_REPORT"}`),
			want: "host was removed",
		},
		{
			name: "case insensitive error matching",
			body: []byte(`{"error": "Host WAS REMOVED from server", "code": "invalid_report"}`),
			want: "Host WAS REMOVED from server",
		},
		{
			name: "extra fields in JSON are ignored",
			body: []byte(`{"error": "host was removed", "code": "invalid_report", "timestamp": "2024-01-01T00:00:00Z"}`),
			want: "host was removed",
		},
		{
			name: "missing error field returns empty",
			body: []byte(`{"code": "invalid_report"}`),
			want: "",
		},
		{
			name: "missing code field returns empty",
			body: []byte(`{"error": "host was removed"}`),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectHostRemovedError(tt.body)
			if got != tt.want {
				t.Errorf("detectHostRemovedError() = %q, want %q", got, tt.want)
			}
		})
	}
}
