package dockeragent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"math/big"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	systemtypes "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/rcourtman/pulse-go-rewrite/internal/agenthelper"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestNewLogsDirectAdmissionRefusalBeforeTypedHelperFallback(t *testing.T) {
	originalConnect := connectCollectorRuntimeFn
	t.Cleanup(func() { connectCollectorRuntimeFn = originalConnect })
	connectCollectorRuntimeFn = func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return nil, systemtypes.Info{}, RuntimeAuto, errors.New("ambiguous collector-owned rootless runtime endpoints")
	}
	helper := &helperInventoryStub{result: agenthelper.ContainerInventoryResult{
		Runtimes: []agenthelper.ContainerRuntimeSnapshot{{Runtime: "docker", Available: true}},
	}}
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	agent, err := New(Config{
		PulseURL: "http://127.0.0.1:7655", APIToken: "token", Runtime: "auto",
		HelperInventory: helper, Logger: &logger,
	})
	if err != nil {
		t.Fatalf("New helper fallback: %v", err)
	}
	defer agent.Close()
	for _, expected := range []string{
		"Direct collector-owned rootless runtime unavailable; using typed helper inventory",
		"ambiguous collector-owned rootless runtime endpoints",
	} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("helper fallback log omitted %q: %s", expected, logs.String())
		}
	}
}

func TestDockerReportSequenceIDsAreProcessScopedAndMonotonic(t *testing.T) {
	agent := &Agent{reportStreamID: "docker-process-stream"}
	first := agent.nextReportSequenceID()
	second := agent.nextReportSequenceID()

	firstStream, firstSequence, firstOK := agentshost.ParseReportSequenceID(first)
	secondStream, secondSequence, secondOK := agentshost.ParseReportSequenceID(second)
	if !firstOK || !secondOK || firstStream != "docker-process-stream" || secondStream != firstStream {
		t.Fatalf("report streams = (%q, %q), parsed = (%v, %v)", firstStream, secondStream, firstOK, secondOK)
	}
	if firstSequence != 1 || secondSequence != 2 {
		t.Fatalf("report sequences = (%d, %d), want (1, 2)", firstSequence, secondSequence)
	}
}

func TestNormalizeTargets(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{
		{URL: " https://pulse.example.com/ ", Token: "tokenA", InsecureSkipVerify: false, CACertPath: " /etc/pulse/ca.pem ", ServerFingerprint: " AB:CD "},
		{URL: "https://pulse.example.com", Token: "tokenA", InsecureSkipVerify: false, CACertPath: "/etc/pulse/ca.pem", ServerFingerprint: "AB:CD"}, // duplicate
		{URL: "https://pulse-dr.example.com", Token: "tokenB", InsecureSkipVerify: true},
	})
	if err != nil {
		t.Fatalf("normalizeTargets returned error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].URL != "https://pulse.example.com" || targets[0].Token != "tokenA" || targets[0].InsecureSkipVerify || targets[0].CACertPath != "/etc/pulse/ca.pem" || targets[0].ServerFingerprint != "AB:CD" {
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
	if _, err := normalizeTargets([]TargetConfig{{URL: "pulse.example.com", Token: "token"}}); err == nil {
		t.Fatalf("expected error for missing scheme")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "ftp://pulse.example.com", Token: "token"}}); err == nil {
		t.Fatalf("expected error for unsupported scheme")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "https://pulse.example.com:70000", Token: "token"}}); err == nil {
		t.Fatalf("expected error for out-of-range port")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "https://pulse.example.com/path?env=prod", Token: "token"}}); err == nil {
		t.Fatalf("expected error for URL query parameters")
	}
	if _, err := normalizeTargets([]TargetConfig{{URL: "https://user:pass@pulse.example.com", Token: "token"}}); err == nil {
		t.Fatalf("expected error for URL userinfo")
	}
}

func TestNormalizeTargetURL(t *testing.T) {
	normalized, err := normalizeTargetURL(" HTTPS://Pulse.EXAMPLE.com:443/api/ ")
	if err != nil {
		t.Fatalf("normalizeTargetURL returned error: %v", err)
	}
	if normalized != "https://pulse.example.com:443/api" {
		t.Fatalf("unexpected normalized URL %q", normalized)
	}
}

func TestNormalizeTargetURLAllowsLocalNetworkHTTP(t *testing.T) {
	normalized, err := normalizeTargetURL(" http://10.0.0.5:7655/api/ ")
	if err != nil {
		t.Fatalf("normalizeTargetURL returned error: %v", err)
	}
	if normalized != "http://10.0.0.5:7655/api" {
		t.Fatalf("unexpected normalized URL %q", normalized)
	}
}

func TestNormalizeTargetURLRejectsPublicHTTP(t *testing.T) {
	if _, err := normalizeTargetURL("http://pulse.example.com:7655/api"); err == nil {
		t.Fatal("expected public HTTP Pulse target to be rejected")
	}
}

func TestNormalizeTargetsAllowsLocalNetworkHTTP(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{{URL: "http://10.0.0.5:7655", Token: "token"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].URL != "http://10.0.0.5:7655" {
		t.Fatalf("unexpected target URL %q", targets[0].URL)
	}
}

func TestNormalizeTargetsRejectsInsecureOrInvalidURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "public http", url: "http://pulse.example.com"},
		{name: "unsupported scheme", url: "ftp://pulse.example.com"},
		{name: "missing scheme", url: "pulse.example.com"},
		{name: "query string", url: "https://pulse.example.com?x=1"},
		{name: "fragment", url: "https://pulse.example.com#frag"},
		{name: "userinfo", url: "https://user:pass@pulse.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeTargets([]TargetConfig{{URL: tt.url, Token: "token"}}); err == nil {
				t.Fatalf("expected error for URL %q", tt.url)
			}
		})
	}
}

func TestNormalizeTargetsAllowsLoopbackHTTP(t *testing.T) {
	targets, err := normalizeTargets([]TargetConfig{
		{URL: "http://localhost:7655/", Token: "token"},
		{URL: "http://127.0.0.1:7655", Token: "token2"},
		{URL: "http://[::1]:7655", Token: "token3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	if targets[0].URL != "http://localhost:7655" {
		t.Fatalf("unexpected localhost URL: %q", targets[0].URL)
	}
	if targets[1].URL != "http://127.0.0.1:7655" {
		t.Fatalf("unexpected IPv4 loopback URL: %q", targets[1].URL)
	}
	if targets[2].URL != "http://[::1]:7655" {
		t.Fatalf("unexpected IPv6 loopback URL: %q", targets[2].URL)
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
			name: "cache larger than usage keeps raw usage",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 4000000,
					Stats: map[string]uint64{"cache": 2000000}, // more than usage
				},
			},
			wantUsage:   1000000, // keeps raw usage when cache > usage
			wantLimit:   4000000,
			wantPercent: 25.0,
		},
		{
			name: "cgroup v2 uses inactive_file when no cache",
			stats: containertypes.StatsResponse{
				MemoryStats: containertypes.MemoryStats{
					Usage: 1000000,
					Limit: 4000000,
					Stats: map[string]uint64{"inactive_file": 300000}, // cgroup v2 style
				},
			},
			wantUsage:   700000, // 1000000 - 300000
			wantLimit:   4000000,
			wantPercent: 17.5,
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

func TestSummarizeNetworkIO(t *testing.T) {
	t.Run("aggregates all interfaces", func(t *testing.T) {
		stats := containertypes.StatsResponse{
			Networks: map[string]containertypes.NetworkStats{
				"eth0": {RxBytes: 1200, TxBytes: 3400},
				"eth1": {RxBytes: 800, TxBytes: 600},
			},
		}
		rx, tx := summarizeNetworkIO(stats)
		if rx != 2000 {
			t.Fatalf("rx bytes = %d, want 2000", rx)
		}
		if tx != 4000 {
			t.Fatalf("tx bytes = %d, want 4000", tx)
		}
	})

	t.Run("empty stats returns zero", func(t *testing.T) {
		rx, tx := summarizeNetworkIO(containertypes.StatsResponse{})
		if rx != 0 || tx != 0 {
			t.Fatalf("expected zero rx/tx, got %d/%d", rx, tx)
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
			name:       "preference podman with unlabeled endpoint returns podman",
			info:       systemtypes.Info{},
			endpoint:   "unix:///custom/container.sock",
			preference: RuntimePodman,
			want:       RuntimePodman,
		},
		{
			// Issue #1647: a podman preference must not mislabel a connection
			// that actually landed on the Docker socket.
			name:       "preference podman with docker socket endpoint returns docker",
			info:       systemtypes.Info{},
			endpoint:   "unix:///var/run/docker.sock",
			preference: RuntimePodman,
			want:       RuntimeDocker,
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
			wantMin:    4, // at least podman rootless + podman system + podman (var/run) + env defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := buildRuntimeCandidates(tt.preference)
			if len(candidates) < tt.wantMin {
				t.Errorf("buildRuntimeCandidates(%v) returned %d candidates, want at least %d", tt.preference, len(candidates), tt.wantMin)
			}

			// When podman is explicitly requested, podman sockets should be tried first.
			// For auto/docker modes, environment defaults should be first.
			if len(candidates) > 0 {
				if tt.preference == RuntimePodman {
					if candidates[0].label != "podman rootless socket" {
						t.Errorf("podman preference: first candidate should be 'podman rootless socket', got %q", candidates[0].label)
					}
				} else if candidates[0].label != "environment defaults" {
					t.Errorf("first candidate should be 'environment defaults', got %q", candidates[0].label)
				}
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

func TestBuildRuntimeCandidatesDeduplicates(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/shared.sock")
	t.Setenv("CONTAINER_HOST", "unix:///tmp/shared.sock")

	candidates := buildRuntimeCandidates(RuntimeAuto)
	count := 0
	for _, c := range candidates {
		if c.host == "unix:///tmp/shared.sock" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected duplicate host to be deduplicated, got %d", count)
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
		{
			name: "monitoring stopped error is detected",
			body: []byte(`{"error": "docker host \"host-1\" had monitoring stopped at 2025-11-02T13:45:15Z and cannot report again", "code": "invalid_report"}`),
			want: `docker host "host-1" had monitoring stopped at 2025-11-02T13:45:15Z and cannot report again`,
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

// TestAgentHookSeamsArePerAgentFields pins the shape of the two indirection
// seams the Docker agent exposes for tests: the timer constructor read by
// waitForAsyncDelay and the JSON marshaller read by acknowledgement delivery
// and update-payload decoding. Both used to be package-level vars in deps.go,
// which raced with async goroutines leaked from earlier tests. They are now
// fields on Agent, injected at construction and never reassigned afterwards.
func TestAgentHookSeamsArePerAgentFields(t *testing.T) {
	agentType := reflect.TypeOf(Agent{})

	for _, tc := range []struct {
		field string
		want  reflect.Type
	}{
		{field: "newTimerFn", want: reflect.TypeOf((func(time.Duration) *time.Timer)(nil))},
		{field: "jsonMarshalFn", want: reflect.TypeOf((func(any) ([]byte, error))(nil))},
	} {
		field, ok := agentType.FieldByName(tc.field)
		if !ok {
			t.Fatalf("Agent is missing the %s test seam field; the hook must stay per-Agent, not a package global", tc.field)
		}
		if field.Type != tc.want {
			t.Fatalf("Agent.%s has type %s, want %s", tc.field, field.Type, tc.want)
		}
		if field.PkgPath == "" {
			t.Fatalf("Agent.%s must stay unexported; it is an internal test seam, not a configuration surface", tc.field)
		}
	}
}

// TestAgentHookSeamsHaveNoPackageGlobals fails if either seam is reintroduced
// as a package-level var in the non-test sources of internal/dockeragent. A
// package global is reachable from every leaked goroutine in the package, which
// is exactly the race the per-Agent fields removed.
func TestAgentHookSeamsHaveNoPackageGlobals(t *testing.T) {
	forbidden := map[string]struct{}{
		"newTimerFn":    {},
		"jsonMarshalFn": {},
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package sources: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no package sources found; the global scan would vacuously pass")
	}

	scanned := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		scanned++

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if _, bad := forbidden[name.Name]; bad {
						t.Errorf(
							"%s declares package-level var %s; the timer and JSON-marshal seams must stay Agent fields so async goroutines never read shared state",
							source, name.Name,
						)
					}
				}
			}
		}
	}

	if scanned == 0 {
		t.Fatal("scanned no non-test sources; the global scan would vacuously pass")
	}
}

// TestAgentHookSeamsDefaultToStdlib proves a zero-value Agent — the production
// shape, where nothing injects a hook — runs the standard library behind both
// seams.
func TestAgentHookSeamsDefaultToStdlib(t *testing.T) {
	agent := &Agent{}

	body, err := agent.jsonMarshal(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("nil jsonMarshalFn should fall back to json.Marshal: %v", err)
	}
	if string(body) != `{"k":"v"}` {
		t.Fatalf("nil jsonMarshalFn produced %q, want json.Marshal output", body)
	}

	timer := agent.newTimer(time.Millisecond)
	if timer == nil {
		t.Fatal("nil newTimerFn should fall back to time.NewTimer")
	}
	defer stopTimer(timer)
	select {
	case <-timer.C:
	case <-time.After(5 * time.Second):
		t.Fatal("fallback timer never fired")
	}
}

// TestAgentHookSeamsAreIsolatedPerAgent is the regression proof for the race
// the per-Agent seams fixed. Two Agents constructed with different hooks are
// exercised concurrently; with a package global, one Agent's injection would be
// visible to the other and the swap would race the reads. Run under -race.
func TestAgentHookSeamsAreIsolatedPerAgent(t *testing.T) {
	marshalErr := errors.New("injected marshal failure")

	failing := &Agent{
		hostID: "failing",
		jsonMarshalFn: func(any) ([]byte, error) {
			return nil, marshalErr
		},
		newTimerFn: func(time.Duration) *time.Timer {
			return time.NewTimer(0)
		},
	}
	plain := &Agent{hostID: "plain"}

	var wg sync.WaitGroup
	errs := make(chan error, 64)

	for range 16 {
		wg.Add(2)

		go func() {
			defer wg.Done()
			if _, err := failing.jsonMarshal(struct{}{}); !errors.Is(err, marshalErr) {
				errs <- fmt.Errorf("injected agent should use its own marshaller, got %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			body, err := plain.jsonMarshal(map[string]int{"n": 1})
			if err != nil {
				errs <- fmt.Errorf("sibling agent must not observe the injected marshaller: %v", err)
				return
			}
			if string(body) != `{"n":1}` {
				errs <- fmt.Errorf("sibling agent produced %q, want json.Marshal output", body)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// The injected timer fires immediately; the sibling's nil field must still
	// resolve to a real time.NewTimer rather than the injected constructor.
	injected := failing.newTimer(time.Hour)
	defer stopTimer(injected)
	select {
	case <-injected.C:
	case <-time.After(5 * time.Second):
		t.Fatal("injected newTimerFn was not used by its own Agent")
	}

	sibling := plain.newTimer(time.Hour)
	defer stopTimer(sibling)
	select {
	case <-sibling.C:
		t.Fatal("sibling agent observed the injected immediate timer; the seam is not per-Agent")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestSendCommandAckUsesReceiverMarshaller proves the acknowledgement transport
// path in agent.go marshals through the receiver's own seam, so an injected
// failure stays scoped to the Agent under test.
func TestSendCommandAckUsesReceiverMarshaller(t *testing.T) {
	marshalErr := errors.New("injected ack marshal failure")
	agent := &Agent{
		hostID: "host1",
		jsonMarshalFn: func(any) ([]byte, error) {
			return nil, marshalErr
		},
	}

	err := agent.sendCommandAck(context.Background(), TargetConfig{URL: "http://example"}, "cmd", "status", "msg")
	if err == nil {
		t.Fatal("expected the injected marshaller to fail the acknowledgement")
	}
	if !errors.Is(err, marshalErr) {
		t.Fatalf("acknowledgement error %v does not wrap the injected failure; the ack path is not using a.jsonMarshal", err)
	}
	if !strings.Contains(err.Error(), "marshal command acknowledgement") {
		t.Fatalf("acknowledgement error %v lost its marshal context", err)
	}
}

// TestIssue1647ConnectRuntimePodmanPreferenceFallsBackToDockerSocket pins the
// runtime-labeling half of issue #1647: when the pinned rootless podman socket
// is gone (it is socket-activated and dies with the login session) and the
// connection falls through to the system Docker socket, the agent must report
// docker — not the preference — so Swarm collection stays available.
func TestIssue1647ConnectRuntimePodmanPreferenceFallsBackToDockerSocket(t *testing.T) {
	docker := &fakeDockerClient{daemonHost: "unix:///var/run/docker.sock"}

	swap(t, &buildRuntimeCandidatesFn, func(preference RuntimeKind) []runtimeCandidate {
		if preference != RuntimePodman {
			t.Fatalf("expected podman preference, got %v", preference)
		}
		return []runtimeCandidate{
			{label: "podman rootless socket", host: "unix:///run/user/0/podman/podman.sock"},
			{label: "environment defaults", applyDockerEnv: true},
		}
	})

	calls := 0
	swap(t, &tryRuntimeCandidateFn, func(_ []client.Opt) (dockerClient, systemtypes.Info, error) {
		calls++
		if calls == 1 {
			return nil, systemtypes.Info{}, errors.New("dial unix /run/user/0/podman/podman.sock: connect: no such file or directory")
		}
		return docker, systemtypes.Info{ServerVersion: "24.0.5", InitBinary: "docker-init"}, nil
	})

	cli, info, runtimeKind, err := connectRuntime(RuntimePodman, nil)
	if err != nil {
		t.Fatalf("expected fallback connection to succeed, got %v", err)
	}
	if cli != docker {
		t.Fatalf("expected the docker fallback client to be returned")
	}
	if runtimeKind != RuntimeDocker {
		t.Fatalf("runtime = %v, want %v (podman preference must not mislabel a Docker connection)", runtimeKind, RuntimeDocker)
	}
	if info.ServerVersion != "24.0.5" {
		t.Fatalf("expected docker info to be returned, got %+v", info)
	}
}

// TestIssue1647ReconnectAfterPersistentDaemonGone pins the reconnect half of
// issue #1647: when the bound socket disappears mid-run, the agent re-runs
// runtime discovery after persistent daemon-unavailable collects instead of
// erroring forever until a process restart.
func TestIssue1647ReconnectAfterPersistentDaemonGone(t *testing.T) {
	failing := &fakeDockerClient{
		daemonHost: "unix:///run/user/0/podman/podman.sock",
		infoFunc: func(context.Context) (systemtypes.Info, error) {
			return systemtypes.Info{}, errors.New("Cannot connect to the Docker daemon at unix:///run/user/0/podman/podman.sock. Is the docker daemon running?")
		},
	}
	healthy := &fakeDockerClient{
		daemonHost: "unix:///var/run/docker.sock",
		infoFunc: func(context.Context) (systemtypes.Info, error) {
			return systemtypes.Info{ServerVersion: "24.0.5", InitBinary: "docker-init"}, nil
		},
		containerListFunc: func(context.Context, dockerContainerListOptions) ([]containertypes.Summary, error) {
			return nil, nil
		},
		closeFn: func() error { return nil },
	}

	reconnects := 0
	swap(t, &connectRuntimeFn, func(preference RuntimeKind, _ *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		reconnects++
		if preference != RuntimePodman {
			t.Errorf("reconnect preference = %v, want %v", preference, RuntimePodman)
		}
		return healthy, systemtypes.Info{ServerVersion: "24.0.5", InitBinary: "docker-init"}, RuntimeDocker, nil
	})
	swap(t, &hostmetricsCollect, func(context.Context, []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	})

	agent := &Agent{
		cfg:         Config{IncludeContainers: true},
		logger:      zerolog.Nop(),
		docker:      newSwappableDockerClient(failing),
		runtime:     RuntimePodman,
		runtimePref: RuntimePodman,
	}

	ctx := context.Background()
	for i := 0; i < runtimeReconnectFailureThreshold-1; i++ {
		if err := agent.collectOnce(ctx); err == nil {
			t.Fatalf("collect %d: expected daemon-unavailable error", i+1)
		}
		if reconnects != 0 {
			t.Fatalf("collect %d: reconnect attempted before failure threshold", i+1)
		}
	}

	if err := agent.collectOnce(ctx); err != nil {
		t.Fatalf("collect at threshold should reconnect and succeed, got %v", err)
	}
	if reconnects != 1 {
		t.Fatalf("reconnects = %d, want 1", reconnects)
	}
	if agent.runtime != RuntimeDocker {
		t.Fatalf("runtime after reconnect = %v, want %v", agent.runtime, RuntimeDocker)
	}
	if !agent.supportsSwarm {
		t.Fatalf("expected Swarm support after reconnecting to Docker")
	}
	if agent.daemonHost != "unix:///var/run/docker.sock" {
		t.Fatalf("daemonHost after reconnect = %q, want the docker socket", agent.daemonHost)
	}
	if agent.runtimeGoneStreak != 0 {
		t.Fatalf("failure streak not reset after successful reconnect: %d", agent.runtimeGoneStreak)
	}

	if err := agent.collectOnce(ctx); err != nil {
		t.Fatalf("follow-up collect on reconnected client failed: %v", err)
	}
	if reconnects != 1 {
		t.Fatalf("healthy collects must not trigger further reconnects, got %d", reconnects)
	}
}

func TestTypedHelperProfileRejectsRootfulReconnect(t *testing.T) {
	closed := false
	rootful := &fakeDockerClient{
		daemonHost: "unix:///var/run/docker.sock",
		closeFn:    func() error { closed = true; return nil },
	}
	swap(t, &connectCollectorRuntimeFn, func(RuntimeKind, *zerolog.Logger) (dockerClient, systemtypes.Info, RuntimeKind, error) {
		return rootful, systemtypes.Info{}, RuntimeDocker, nil
	})

	agent := &Agent{
		cfg:               Config{HelperInventory: &helperInventoryStub{}},
		logger:            zerolog.Nop(),
		runtimePref:       RuntimeAuto,
		runtimeGoneStreak: runtimeReconnectFailureThreshold - 1,
	}
	if agent.maybeReconnectRuntime(errors.New("cannot connect to the Docker daemon")) {
		t.Fatal("typed-helper profile adopted a rootful reconnect endpoint")
	}
	if !closed {
		t.Fatal("rejected rootful reconnect client was not closed")
	}
	if agent.docker != nil {
		t.Fatal("rejected rootful reconnect was installed")
	}
}

func TestBuildReportForwardsExplicitDiskIncludesAndExcludes(t *testing.T) {
	var gotExclude, gotInclude []string
	swap(t, &hostmetricsCollectWithDiskFilters, func(_ context.Context, exclude, include []string) (hostmetrics.Snapshot, error) {
		gotExclude = append([]string(nil), exclude...)
		gotInclude = append([]string(nil), include...)
		return hostmetrics.Snapshot{}, nil
	})

	agent := &Agent{
		cfg: Config{
			AgentID:     "docker-agent-1",
			Interval:    30 * time.Second,
			DiskExclude: []string{"/mnt/private"},
			DiskInclude: []string{"/mnt/containers"},
		},
		docker: &fakeDockerClient{
			infoFunc: func(context.Context) (systemtypes.Info, error) {
				return systemtypes.Info{ID: "daemon", ServerVersion: "24.0.0"}, nil
			},
		},
		logger: zerolog.Nop(),
	}

	if _, err := agent.buildReport(context.Background()); err != nil {
		t.Fatalf("buildReport() failed: %v", err)
	}
	if got := strings.Join(gotExclude, ","); got != "/mnt/private" {
		t.Fatalf("disk excludes = %q, want /mnt/private", got)
	}
	if got := strings.Join(gotInclude, ","); got != "/mnt/containers" {
		t.Fatalf("disk includes = %q, want /mnt/containers", got)
	}
}

func TestDockerStorageUsageCadenceIsScopedPerAgentInstance(t *testing.T) {
	firstCalls, secondCalls := 0, 0
	newAgent := func(calls *int, total int64) *Agent {
		return &Agent{
			logger: zerolog.Nop(),
			docker: &fakeDockerClient{
				diskUsageFn: func(context.Context, dockerDiskUsageOptions) (client.DiskUsageResult, error) {
					*calls++
					return client.DiskUsageResult{Containers: client.ContainersDiskUsage{TotalCount: total}}, nil
				},
			},
		}
	}
	first := newAgent(&firstCalls, 41)
	second := newAgent(&secondCalls, 7)

	for _, agent := range []*Agent{first, first, second, second} {
		if _, _, err := agent.collectStorageUsage(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("per-agent Docker storage scans = %d/%d, want 1/1", firstCalls, secondCalls)
	}
}

func TestRegistryCredentialSourceForConfig(t *testing.T) {
	logger := zerolog.Nop()

	source := registryCredentialSourceForConfig(Config{}, logger)
	if _, ok := source.(*dockerConfigCredentials); !ok {
		t.Fatalf("expected host Docker credential source by default, got %T", source)
	}

	if disabled := registryCredentialSourceForConfig(Config{DisableRegistryCredentials: true}, logger); disabled != nil {
		t.Fatalf("expected nil credential source when disabled, got %T", disabled)
	}
}

func TestCollectorRuntimeBoundaryLossIsDaemonUnavailable(t *testing.T) {
	for _, err := range []error{
		errCollectorRuntimeBoundaryChanged,
		fmt.Errorf("cycle validation: %w", errCollectorRuntimeBoundaryChanged),
		syscall.EACCES,
		syscall.EPERM,
		errors.New("dial unix /run/user/991/docker.sock: permission denied"),
	} {
		if !isDockerDaemonUnavailable(err) {
			t.Fatalf("collector boundary loss %q was not classified unavailable", err)
		}
	}
}
