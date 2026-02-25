package hostagent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestParseClusterStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPeers  int
		wantLocal  string
		wantOnline int
		wantErr    bool
	}{
		{
			name: "three node cluster",
			input: `[
				{"type":"cluster","name":"pve-cluster","version":3,"nodes":3,"quorate":1},
				{"type":"node","name":"pve1","ip":"10.0.0.1","local":1,"online":1,"nodeid":1},
				{"type":"node","name":"pve2","ip":"10.0.0.2","local":0,"online":1,"nodeid":2},
				{"type":"node","name":"pve3","ip":"10.0.0.3","local":0,"online":1,"nodeid":3}
			]`,
			wantPeers:  3,
			wantLocal:  "pve1",
			wantOnline: 3,
		},
		{
			name: "node offline",
			input: `[
				{"type":"cluster","name":"cluster1","version":2,"nodes":3,"quorate":1},
				{"type":"node","name":"node1","ip":"192.168.1.1","local":1,"online":1,"nodeid":1},
				{"type":"node","name":"node2","ip":"192.168.1.2","local":0,"online":1,"nodeid":2},
				{"type":"node","name":"node3","ip":"192.168.1.3","local":0,"online":0,"nodeid":3}
			]`,
			wantPeers:  3,
			wantLocal:  "node1",
			wantOnline: 2,
		},
		{
			name: "single node (no cluster)",
			input: `[
				{"type":"node","name":"standalone","ip":"10.0.0.1","local":1,"online":1,"nodeid":1}
			]`,
			wantPeers:  1,
			wantLocal:  "standalone",
			wantOnline: 1,
		},
		{
			name:    "invalid JSON",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:      "empty array",
			input:     `[]`,
			wantPeers: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peers, err := parseClusterStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseClusterStatus() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if len(peers) != tt.wantPeers {
				t.Errorf("got %d peers, want %d", len(peers), tt.wantPeers)
			}

			onlineCount := 0
			for _, p := range peers {
				if p.Online {
					onlineCount++
				}
				if p.Local && p.Name != tt.wantLocal {
					t.Errorf("local node = %q, want %q", p.Name, tt.wantLocal)
				}
			}
			if onlineCount != tt.wantOnline {
				t.Errorf("online count = %d, want %d", onlineCount, tt.wantOnline)
			}
		})
	}
}

func TestFindClusterSSHKey(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	tests := []struct {
		name     string
		statFn   func(string) (os.FileInfo, error)
		wantPath string
	}{
		{
			name: "ed25519 key exists",
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_ed25519" {
					return nil, nil // exists
				}
				return nil, os.ErrNotExist
			},
			wantPath: "/root/.ssh/id_ed25519",
		},
		{
			name: "only RSA key exists",
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_rsa" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			wantPath: "/root/.ssh/id_rsa",
		},
		{
			name: "no keys",
			statFn: func(name string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			wantPath: "",
		},
		{
			name: "prefers ed25519 over RSA",
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_ed25519" || name == "/root/.ssh/id_rsa" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			wantPath: "/root/.ssh/id_ed25519",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Agent{
				logger: logger,
				collector: &mockCollector{
					statFn: tt.statFn,
				},
			}

			got := a.findClusterSSHKey()
			if got != tt.wantPath {
				t.Errorf("findClusterSSHKey() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestCollectClusterSensors_NotProxmox(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	a := &Agent{
		logger: logger,
		cfg:    Config{EnableProxmox: false},
		collector: &mockCollector{
			goos: "linux",
		},
	}

	result := a.collectClusterSensors(context.Background())
	if result != nil {
		t.Errorf("expected nil when Proxmox disabled, got %v", result)
	}
}

func TestCollectClusterSensors_NotLinux(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	a := &Agent{
		logger: logger,
		cfg:    Config{EnableProxmox: true},
		collector: &mockCollector{
			goos: "darwin",
		},
	}

	result := a.collectClusterSensors(context.Background())
	if result != nil {
		t.Errorf("expected nil on non-Linux, got %v", result)
	}
}

func TestCollectClusterSensors_FullFlow(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	sensorsJSON := `{
		"coretemp-isa-0000":{
			"Adapter": "ISA adapter",
			"Package id 0":{"temp1_input": 55.000},
			"Core 0":{"temp2_input": 52.000},
			"Core 1":{"temp3_input": 53.000}
		}
	}`

	a := &Agent{
		logger: logger,
		cfg:    Config{EnableProxmox: true},
		collector: &mockCollector{
			goos: "linux",
			commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
				args := strings.Join(arg, " ")
				// pvesh call
				if name == "pvesh" {
					return `[
						{"type":"cluster","name":"test","version":1},
						{"type":"node","name":"pve1","ip":"10.0.0.1","local":1,"online":1},
						{"type":"node","name":"pve2","ip":"10.0.0.2","local":0,"online":1}
					]`, nil
				}
				// ssh call to pve2
				if name == "ssh" && strings.Contains(args, "10.0.0.2") {
					return sensorsJSON, nil
				}
				return "", fmt.Errorf("unexpected command: %s %s", name, args)
			},
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_rsa" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			sensorsParseFn: func(jsonStr string) (*sensors.TemperatureData, error) {
				return sensors.Parse(jsonStr)
			},
		},
	}

	result := a.collectClusterSensors(context.Background())
	if len(result) != 1 {
		t.Fatalf("expected 1 cluster sensor entry, got %d", len(result))
	}

	entry := result[0]
	if entry.NodeName != "pve2" {
		t.Errorf("node name = %q, want %q", entry.NodeName, "pve2")
	}
	if len(entry.Sensors.TemperatureCelsius) == 0 {
		t.Error("expected temperature data, got none")
	}
	if _, ok := entry.Sensors.TemperatureCelsius["cpu_package"]; !ok {
		t.Error("expected cpu_package in temperature data")
	}
}

func TestCollectClusterSensors_PeerFailure(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	sensorsJSON := `{
		"coretemp-isa-0000":{
			"Adapter": "ISA adapter",
			"Package id 0":{"temp1_input": 45.000}
		}
	}`

	a := &Agent{
		logger: logger,
		cfg:    Config{EnableProxmox: true},
		collector: &mockCollector{
			goos: "linux",
			commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
				args := strings.Join(arg, " ")
				if name == "pvesh" {
					return `[
						{"type":"node","name":"local","ip":"10.0.0.1","local":1,"online":1},
						{"type":"node","name":"good","ip":"10.0.0.2","local":0,"online":1},
						{"type":"node","name":"bad","ip":"10.0.0.3","local":0,"online":1}
					]`, nil
				}
				if name == "ssh" && strings.Contains(args, "10.0.0.2") {
					return sensorsJSON, nil
				}
				if name == "ssh" && strings.Contains(args, "10.0.0.3") {
					return "", fmt.Errorf("ssh: connect to host 10.0.0.3: Connection refused")
				}
				return "", fmt.Errorf("unexpected: %s", name)
			},
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_ed25519" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			sensorsParseFn: func(jsonStr string) (*sensors.TemperatureData, error) {
				return sensors.Parse(jsonStr)
			},
		},
	}

	result := a.collectClusterSensors(context.Background())
	if len(result) != 1 {
		t.Fatalf("expected 1 successful entry (bad peer should be skipped), got %d", len(result))
	}
	if result[0].NodeName != "good" {
		t.Errorf("expected 'good' node, got %q", result[0].NodeName)
	}
}

func TestConvertTemperatureDataToSensors(t *testing.T) {
	tempData := &sensors.TemperatureData{
		CPUPackage: 65.0,
		Cores: map[string]float64{
			"Core 0": 62.0,
			"Core 1": 63.0,
		},
		NVMe: map[string]float64{
			"nvme0": 42.0,
		},
		GPU: map[string]float64{
			"gpu_edge": 55.0,
		},
		Fans: map[string]float64{
			"cpu_fan": 1200.0,
		},
		Other: map[string]float64{
			"pch_temp": 48.0,
		},
		Available: true,
	}

	result := convertTemperatureDataToSensors(tempData)

	if result.TemperatureCelsius["cpu_package"] != 65.0 {
		t.Errorf("cpu_package = %v, want 65.0", result.TemperatureCelsius["cpu_package"])
	}
	if result.TemperatureCelsius["cpu_core_0"] != 62.0 {
		t.Errorf("cpu_core_0 = %v, want 62.0", result.TemperatureCelsius["cpu_core_0"])
	}
	if result.TemperatureCelsius["nvme0"] != 42.0 {
		t.Errorf("nvme0 = %v, want 42.0", result.TemperatureCelsius["nvme0"])
	}
	if result.TemperatureCelsius["gpu_edge"] != 55.0 {
		t.Errorf("gpu_edge = %v, want 55.0", result.TemperatureCelsius["gpu_edge"])
	}
	if result.FanRPM["cpu_fan"] != 1200.0 {
		t.Errorf("cpu_fan RPM = %v, want 1200.0", result.FanRPM["cpu_fan"])
	}
	if result.Additional["pch_temp"] != 48.0 {
		t.Errorf("pch_temp additional = %v, want 48.0", result.Additional["pch_temp"])
	}
}

func TestCollectClusterSensors_SkipsOfflinePeers(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	sshCalled := false
	a := &Agent{
		logger: logger,
		cfg:    Config{EnableProxmox: true},
		collector: &mockCollector{
			goos: "linux",
			commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
				if name == "pvesh" {
					return `[
						{"type":"node","name":"local","ip":"10.0.0.1","local":1,"online":1},
						{"type":"node","name":"offline","ip":"10.0.0.2","local":0,"online":0}
					]`, nil
				}
				if name == "ssh" {
					sshCalled = true
				}
				return "", fmt.Errorf("unexpected")
			},
			statFn: func(name string) (os.FileInfo, error) {
				if name == "/root/.ssh/id_rsa" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
		},
	}

	result := a.collectClusterSensors(context.Background())
	if result != nil {
		t.Errorf("expected nil for no online remote peers, got %v", result)
	}
	if sshCalled {
		t.Error("SSH should not be called for offline peers")
	}
}

func TestCollectPeerSensors_EmptyOutput(t *testing.T) {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
	})).Level(zerolog.DebugLevel)

	a := &Agent{
		logger: logger,
		collector: &mockCollector{
			commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
				return "", nil // empty output (sensors not installed on peer)
			},
		},
	}

	result, err := a.collectPeerSensors(context.Background(), clusterPeer{
		Name: "peer1",
		IP:   "10.0.0.2",
	}, "/root/.ssh/id_rsa")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.TemperatureCelsius) != 0 {
		t.Errorf("expected empty sensors for empty output, got %v", result)
	}
}

// Verify the ClusterNodeSensors type is correctly structured
func TestClusterNodeSensorsType(t *testing.T) {
	entry := agentshost.ClusterNodeSensors{
		NodeName: "pve2",
		Sensors: agentshost.Sensors{
			TemperatureCelsius: map[string]float64{
				"cpu_package": 55.0,
			},
		},
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if entry.NodeName != "pve2" {
		t.Errorf("NodeName = %q, want %q", entry.NodeName, "pve2")
	}
	if entry.Sensors.TemperatureCelsius["cpu_package"] != 55.0 {
		t.Error("expected cpu_package temperature")
	}
}
