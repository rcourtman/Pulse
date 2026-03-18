package agentupdate_test

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostagent"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

type integrationCollector struct {
	now time.Time
}

func (c *integrationCollector) HostInfo(context.Context) (*gohost.InfoStat, error) {
	return &gohost.InfoStat{
		Hostname:        "upgraded-host",
		HostID:          "machine-upgrade-1",
		Platform:        "linux",
		PlatformVersion: "6.8",
		KernelVersion:   "6.8.12",
		KernelArch:      runtime.GOARCH,
	}, nil
}

func (c *integrationCollector) HostUptime(context.Context) (uint64, error) {
	return 7200, nil
}

func (c *integrationCollector) Metrics(context.Context, []string) (hostmetrics.Snapshot, error) {
	return hostmetrics.Snapshot{
		Memory: agentshost.MemoryMetric{
			TotalBytes: 1024,
			UsedBytes:  512,
			Usage:      50,
		},
	}, nil
}

func (c *integrationCollector) SensorsLocal(context.Context) (string, error) {
	return "{}", nil
}

func (c *integrationCollector) SensorsParse(string) (*sensors.TemperatureData, error) {
	return &sensors.TemperatureData{}, nil
}

func (c *integrationCollector) SensorsPower(context.Context) (*sensors.PowerData, error) {
	return &sensors.PowerData{}, nil
}

func (c *integrationCollector) RAIDArrays(context.Context) ([]agentshost.RAIDArray, error) {
	return nil, nil
}

func (c *integrationCollector) UnraidStorage(context.Context) (*agentshost.UnraidStorage, error) {
	return nil, nil
}

func (c *integrationCollector) CephStatus(context.Context) (*hostagent.CephClusterStatus, error) {
	return nil, nil
}

func (c *integrationCollector) SMARTLocal(context.Context, []string) ([]hostagent.DiskSMART, error) {
	return nil, nil
}

func (c *integrationCollector) Now() time.Time {
	return c.now
}

func (c *integrationCollector) GOOS() string {
	return "linux"
}

func (c *integrationCollector) ReadFile(string) ([]byte, error) {
	return nil, os.ErrNotExist
}

func (c *integrationCollector) NetInterfaces() ([]net.Interface, error) {
	return nil, nil
}

func (c *integrationCollector) Hostname() (string, error) {
	return "upgraded-host", nil
}

func (c *integrationCollector) LookupIP(string) ([]net.IP, error) {
	return nil, nil
}

func (c *integrationCollector) DialTimeout(string, string, time.Duration) (net.Conn, error) {
	return nil, nil
}

func (c *integrationCollector) Stat(string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (c *integrationCollector) MkdirAll(string, os.FileMode) error {
	return nil
}

func (c *integrationCollector) Chmod(string, os.FileMode) error {
	return nil
}

func (c *integrationCollector) WriteFile(string, []byte, os.FileMode) error {
	return nil
}

func (c *integrationCollector) CommandCombinedOutput(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (c *integrationCollector) LookPath(string) (string, error) {
	return "", nil
}

func testBinaryPayload() []byte {
	switch runtime.GOOS {
	case "darwin":
		return []byte{0xcf, 0xfa, 0xed, 0xfe, 0x01, 0x02, 0x03, 0x04}
	case "windows":
		return []byte{'M', 'Z', 0x90, 0x00, 0x01, 0x02, 0x03, 0x04}
	default:
		return []byte{0x7f, 'E', 'L', 'F', 0x01, 0x02, 0x03, 0x04}
	}
}

func testChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func captureReportServer(t *testing.T) (*httptest.Server, func() []agentshost.Report) {
	t.Helper()

	reports := make([]agentshost.Report, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/agent/report" {
			http.NotFound(w, r)
			return
		}

		var bodyReader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			bodyReader = gz
		}

		var report agentshost.Report
		if err := json.NewDecoder(bodyReader).Decode(&report); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		reports = append(reports, report)
		w.WriteHeader(http.StatusOK)
	}))

	return server, func() []agentshost.Report {
		return append([]agentshost.Report(nil), reports...)
	}
}

func TestUpdateToFirstHostReportCarriesPreviousVersionOnce(t *testing.T) {
	binaryPayload := testBinaryPayload()
	checksum := testChecksum(binaryPayload)

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Checksum-Sha256", checksum)
		_, _ = w.Write(binaryPayload)
	}))
	defer downloadServer.Close()

	tempDir := t.TempDir()
	execPath := filepath.Join(tempDir, "pulse-agent")
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write exec path: %v", err)
	}

	updater := agentupdate.New(agentupdate.Config{
		PulseURL:       downloadServer.URL,
		AgentName:      "pulse-agent",
		CurrentVersion: "5.1.14",
		CheckInterval:  time.Hour,
	})

	if err := agentupdate.PerformUpdateWithExecPathForTest(updater, context.Background(), execPath); err != nil {
		t.Fatalf("PerformUpdateWithExecPathForTest: %v", err)
	}

	firstServer, firstReports := captureReportServer(t)
	defer firstServer.Close()
	secondServer, secondReports := captureReportServer(t)
	defer secondServer.Close()

	collector := &integrationCollector{
		now: time.Date(2026, time.March, 12, 11, 12, 13, 0, time.UTC),
	}

	agentupdate.WithExecutablePathForTest(execPath, func() {
		firstAgent, err := hostagent.New(hostagent.Config{
			PulseURL:     firstServer.URL,
			APIToken:     "token",
			AgentID:      "agent-1",
			AgentType:    "unified",
			AgentVersion: "6.0.0-rc.1",
			RunOnce:      true,
			LogLevel:     zerolog.InfoLevel,
			Collector:    collector,
		})
		if err != nil {
			t.Fatalf("first hostagent.New: %v", err)
		}
		if err := firstAgent.Run(context.Background()); err != nil {
			t.Fatalf("first Agent.Run: %v", err)
		}

		secondAgent, err := hostagent.New(hostagent.Config{
			PulseURL:     secondServer.URL,
			APIToken:     "token",
			AgentID:      "agent-1",
			AgentType:    "unified",
			AgentVersion: "6.0.0-rc.1",
			RunOnce:      true,
			LogLevel:     zerolog.InfoLevel,
			Collector:    collector,
		})
		if err != nil {
			t.Fatalf("second hostagent.New: %v", err)
		}
		if err := secondAgent.Run(context.Background()); err != nil {
			t.Fatalf("second Agent.Run: %v", err)
		}
	})

	first := firstReports()
	if len(first) != 1 {
		t.Fatalf("expected 1 first-start report, got %d", len(first))
	}
	if first[0].Agent.UpdatedFrom != "5.1.14" {
		t.Fatalf("first report updated_from = %q, want %q", first[0].Agent.UpdatedFrom, "5.1.14")
	}
	if first[0].Agent.Version != "6.0.0-rc.1" {
		t.Fatalf("first report version = %q, want %q", first[0].Agent.Version, "6.0.0-rc.1")
	}

	second := secondReports()
	if len(second) != 1 {
		t.Fatalf("expected 1 second-start report, got %d", len(second))
	}
	if second[0].Agent.UpdatedFrom != "" {
		t.Fatalf("second report updated_from = %q, want empty string", second[0].Agent.UpdatedFrom)
	}
}

func TestCheckAndUpdateToFirstHostReportCarriesPreviousVersionOnce(t *testing.T) {
	binaryPayload := testBinaryPayload()
	checksum := testChecksum(binaryPayload)

	updateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent/version":
			if err := json.NewEncoder(w).Encode(map[string]string{"version": "6.0.0-rc.1"}); err != nil {
				t.Fatalf("encode version: %v", err)
			}
		case "/download/pulse-agent":
			w.Header().Set("X-Checksum-Sha256", checksum)
			_, _ = w.Write(binaryPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer updateServer.Close()

	tempDir := t.TempDir()
	execPath := filepath.Join(tempDir, "pulse-agent")
	if err := os.WriteFile(execPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write exec path: %v", err)
	}

	updater := agentupdate.New(agentupdate.Config{
		PulseURL:       updateServer.URL,
		AgentName:      "pulse-agent",
		CurrentVersion: "5.1.14",
		CheckInterval:  time.Hour,
	})
	restoreUpdateHook := agentupdate.UseExecPathForUpdateChecksForTest(updater, execPath)
	defer restoreUpdateHook()

	updater.CheckAndUpdate(context.Background())

	firstServer, firstReports := captureReportServer(t)
	defer firstServer.Close()

	collector := &integrationCollector{
		now: time.Date(2026, time.March, 12, 12, 13, 14, 0, time.UTC),
	}

	agentupdate.WithExecutablePathForTest(execPath, func() {
		firstAgent, err := hostagent.New(hostagent.Config{
			PulseURL:     firstServer.URL,
			APIToken:     "token",
			AgentID:      "agent-1",
			AgentType:    "unified",
			AgentVersion: "6.0.0-rc.1",
			RunOnce:      true,
			LogLevel:     zerolog.InfoLevel,
			Collector:    collector,
		})
		if err != nil {
			t.Fatalf("hostagent.New after CheckAndUpdate: %v", err)
		}
		if err := firstAgent.Run(context.Background()); err != nil {
			t.Fatalf("Agent.Run after CheckAndUpdate: %v", err)
		}
	})

	first := firstReports()
	if len(first) != 1 {
		t.Fatalf("expected 1 report after CheckAndUpdate, got %d", len(first))
	}
	if first[0].Agent.UpdatedFrom != "5.1.14" {
		t.Fatalf("report updated_from = %q, want %q", first[0].Agent.UpdatedFrom, "5.1.14")
	}
	if first[0].Agent.Version != "6.0.0-rc.1" {
		t.Fatalf("report version = %q, want %q", first[0].Agent.Version, "6.0.0-rc.1")
	}
}
