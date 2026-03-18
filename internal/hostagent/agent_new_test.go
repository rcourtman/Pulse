package hostagent

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

const testPEMCertificate = `-----BEGIN CERTIFICATE-----
MIICuDCCAaACCQDptFpSdDdFNjANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNw
dWxzZS10ZXN0LWNhLmxvY2FsMB4XDTI2MDMxNDA4NTgyN1oXDTI3MDMxNDA4NTgy
N1owHjEcMBoGA1UEAwwTcHVsc2UtdGVzdC1jYS5sb2NhbDCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBANWmj5xXF1pDWKqbScN6VtU1PX3e9DuyDnegnAuR
UA7QIqgyQ7gfPZtAABr0kaV993mZZw92XkdXeF+9eClRBnVoJmISdwiBpB6oE8w/
H6tfnG34JUjvXN39/B66mAeuBd/erAxj4fXuH+ohA3AWZcotCYS2anOAbyRPo8BU
DGm79VBp5/s/uZ8bGe5LiSPxFXOp7kBk2sDWI77Y0UNwuc/wzO+GrE0GGXnbxcRW
9ICRPq7pked0BO2oBaeMRmvo7npAn9+w+0EDVi1qqw5xoYposYgsR76uLSYhQgaL
5ZgUYlCW7Vvp5ve/tmxPXuae8y3OIrOT7WFWfm8GAa9ZneMCAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAdpFuEiVPhYcJe/kkfPuHwv68Dx+/5jFXMkLQFIZnnC5Umkph
ubtFPrce9BLqLQBGdhQ4IkaEA9QDSZDTUbzZLtw3G6tHgl63H4kuB5ZbXgEVPmNT
07i8Obt4uUgIhfx/EzyCaZpfoQnXHmHm2xxg6QiP4v2TUQdBkLpD5mzVTwYOw9GF
w8AuCKd92UTs4/0ikTMdK0M4zwhF0JAhibyMNBRXfg1c96KyCFYSSNeERQFy5Fqo
TREsx8ScXgne7V+lLwLa8CTjUAcvCVq6SIqKbjSEZ1V5UpzvwBh52/cWCa6Rafd5
ARKc3gwyVxyCX3h21kFcEU2rt7C7/RcXBCyWzQ==
-----END CERTIFICATE-----
`

func TestNew_AllowsMissingAPITokenWhenEnrollmentDisabled(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{APIToken: "  ", LogLevel: zerolog.InfoLevel, Collector: mc})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.cfg.APIToken != "  " {
		t.Fatalf("agent token = %q, want original empty token input", agent.cfg.APIToken)
	}
}

func TestNew_RequiresAPITokenWhenEnrollmentEnabled(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	_, err := New(Config{APIToken: "  ", Enroll: true, LogLevel: zerolog.InfoLevel, Collector: mc})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "api token is required when enrollment is enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_NormalizesConfigAndTags(t *testing.T) {
	mc := &mockCollector{
		goos: "darwin",
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        " host-from-info ",
				HostID:          " gopsutil-id ",
				Platform:        "Darwin",
				PlatformFamily:  "",
				PlatformVersion: "14.0",
				KernelVersion:   "6.6.0",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		readFileFn: func(name string) ([]byte, error) {
			if name == "/etc/machine-id" {
				return []byte("0123456789abcdef0123456789abcdef\n"), nil
			}
			return nil, os.ErrNotExist
		},
	}

	originalTags := []string{"  tag-a ", "tag-a", "", " tag-b", "tag-b "}
	cfg := Config{
		PulseURL:           "https://example.com///",
		APIToken:           "token",
		Interval:           0,
		Tags:               originalTags,
		InsecureSkipVerify: true,
		LogLevel:           zerolog.InfoLevel,
		Collector:          mc,
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if agent.interval != defaultInterval {
		t.Fatalf("interval = %v, want %v", agent.interval, defaultInterval)
	}
	if agent.trimmedPulseURL != "https://example.com" {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, "https://example.com")
	}
	if agent.hostname != "host-from-info" {
		t.Fatalf("hostname = %q, want %q", agent.hostname, "host-from-info")
	}
	if agent.displayName != "host-from-info" {
		t.Fatalf("displayName = %q, want %q", agent.displayName, "host-from-info")
	}
	if agent.platform != "macos" {
		t.Fatalf("platform = %q, want %q", agent.platform, "macos")
	}
	if got, want := agent.cfg.Tags, []string{"tag-a", "tag-b"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}

	// Ensure we don't retain aliasing to the caller-provided tags slice.
	originalTags[0] = "mutated"
	if agent.cfg.Tags[0] != "tag-a" {
		t.Fatalf("agent tags aliased caller slice: %#v", agent.cfg.Tags)
	}

	httpTransport, ok := agent.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", agent.httpClient.Transport)
	}
	if httpTransport.TLSClientConfig == nil || httpTransport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLSClientConfig MinVersion = %#v, want TLS1.2", httpTransport.TLSClientConfig)
	}
	if !httpTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected InsecureSkipVerify=true")
	}

	if mc.GOOS() == "linux" {
		const want = "01234567-89ab-cdef-0123-456789abcdef"
		if agent.machineID != want {
			t.Fatalf("machineID = %q, want %q", agent.machineID, want)
		}
	} else {
		if agent.machineID != "gopsutil-id" {
			t.Fatalf("machineID = %q, want %q", agent.machineID, "gopsutil-id")
		}
	}
}

func TestNew_DefaultPulseURL(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{PulseURL: "", APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.trimmedPulseURL != "http://localhost:7655" {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, "http://localhost:7655")
	}
}

func TestNew_UsesCustomCABundleForHTTPTransport(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	certPath := filepath.Join(t.TempDir(), "pulse-ca.pem")
	if err := os.WriteFile(certPath, []byte(testPEMCertificate), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	agent, err := New(Config{
		APIToken:   "token",
		CACertPath: certPath,
		LogLevel:   zerolog.InfoLevel,
		Collector:  mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	httpTransport, ok := agent.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", agent.httpClient.Transport)
	}
	if httpTransport.TLSClientConfig == nil || httpTransport.TLSClientConfig.RootCAs == nil {
		t.Fatalf("expected RootCAs to be populated for a custom CA bundle")
	}
}

func TestNew_CarriesUpdatedFromIntoFirstV6Report(t *testing.T) {
	origGetUpdatedFromVersion := getUpdatedFromVersion
	t.Cleanup(func() { getUpdatedFromVersion = origGetUpdatedFromVersion })

	getUpdatedFromVersion = func() string { return "5.1.14" }

	fixedTime := time.Date(2026, time.March, 12, 9, 10, 11, 0, time.UTC)
	mc := &mockCollector{
		nowFn: func() time.Time { return fixedTime },
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "upgraded-host",
				HostID:          "machine-id-1",
				Platform:        "linux",
				PlatformVersion: "6.8",
				KernelVersion:   "6.8.12",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		hostUptimeFn: func(context.Context) (uint64, error) { return 7200, nil },
		metricsFn: func(context.Context, []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{
				Memory: agentshost.MemoryMetric{
					TotalBytes: 1024,
					UsedBytes:  512,
					Usage:      50,
				},
			}, nil
		},
	}

	agent, err := New(Config{
		APIToken:     "token",
		AgentID:      "agent-1",
		AgentType:    "unified",
		AgentVersion: "6.0.0-rc.1",
		LogLevel:     zerolog.InfoLevel,
		Collector:    mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if agent.updatedFrom != "5.1.14" {
		t.Fatalf("updatedFrom = %q, want %q", agent.updatedFrom, "5.1.14")
	}

	report, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("buildReport: %v", err)
	}

	if report.Agent.Version != "6.0.0-rc.1" {
		t.Fatalf("report agent version = %q, want %q", report.Agent.Version, "6.0.0-rc.1")
	}
	if report.Agent.UpdatedFrom != "5.1.14" {
		t.Fatalf("report updated_from = %q, want %q", report.Agent.UpdatedFrom, "5.1.14")
	}
	if agent.updatedFrom != "" {
		t.Fatalf("agent.updatedFrom after first build = %q, want empty string", agent.updatedFrom)
	}

	secondReport, err := agent.buildReport(context.Background())
	if err != nil {
		t.Fatalf("second buildReport: %v", err)
	}
	if secondReport.Agent.UpdatedFrom != "" {
		t.Fatalf("second report updated_from = %q, want empty string", secondReport.Agent.UpdatedFrom)
	}
}

func TestNew_RejectsInvalidPulseURL(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "non-loopback http rejected",
			url:  "http://example.com",
			want: "must use https unless host is loopback or private network",
		},
		{
			name: "query rejected",
			url:  "https://example.com?token=abc",
			want: "must not include query or fragment",
		},
		{
			name: "userinfo rejected",
			url:  "https://user:pass@example.com",
			want: "must not include user credentials",
		},
		{
			name: "unsupported scheme rejected",
			url:  "ftp://example.com",
			want: "unsupported scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(Config{
				PulseURL:  tt.url,
				APIToken:  "token",
				LogLevel:  zerolog.InfoLevel,
				Collector: mc,
			})
			if err == nil {
				t.Fatalf("expected error for URL %q", tt.url)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNew_NormalizesAndValidatesProxmoxType(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{
		APIToken:    "token",
		ProxmoxType: "  Auto ",
		LogLevel:    zerolog.InfoLevel,
		Collector:   mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.cfg.ProxmoxType != "" {
		t.Fatalf("cfg.ProxmoxType = %q, want empty auto value", agent.cfg.ProxmoxType)
	}

	_, err = New(Config{
		APIToken:    "token",
		ProxmoxType: "invalid",
		LogLevel:    zerolog.InfoLevel,
		Collector:   mc,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid proxmox type") {
		t.Fatalf("expected proxmox type validation error, got %v", err)
	}
}

func TestNew_NormalizesAndValidatesReportIP(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		ReportIP:  " 2001:0DB8::0001 ",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := agent.cfg.ReportIP; got != "2001:db8::1" {
		t.Fatalf("cfg.ReportIP = %q, want %q", got, "2001:db8::1")
	}

	_, err = New(Config{
		APIToken:  "token",
		ReportIP:  "not-an-ip",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid report IP") {
		t.Fatalf("expected report IP validation error, got %v", err)
	}

	_, err = New(Config{
		APIToken:  "token",
		ReportIP:  "0.0.0.0",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err == nil || !strings.Contains(err.Error(), "unspecified addresses are not allowed") {
		t.Fatalf("expected unspecified report IP validation error, got %v", err)
	}
}

func TestNew_FallsBackToHostnameWhenMachineIDAndMACEmpty(t *testing.T) {
	mc := &mockCollector{
		goos: "linux",
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:   "host-from-info",
				HostID:     "",
				KernelArch: runtime.GOARCH,
			}, nil
		},
		readFileFn:      func(string) ([]byte, error) { return nil, os.ErrNotExist },
		netInterfacesFn: func() ([]net.Interface, error) { return nil, errors.New("no interfaces") },
	}

	agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.machineID != "" {
		t.Fatalf("expected empty machineID, got %q", agent.machineID)
	}
	if agent.agentID != "host-from-info" {
		t.Fatalf("agentID = %q, want %q", agent.agentID, "host-from-info")
	}
}

func TestNew_FallsBackToMACWhenMachineIDEmpty(t *testing.T) {
	mc := &mockCollector{
		goos: "linux",
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:   "host-from-info",
				HostID:     "",
				KernelArch: runtime.GOARCH,
			}, nil
		},
		readFileFn: func(string) ([]byte, error) { return nil, os.ErrNotExist },
		netInterfacesFn: func() ([]net.Interface, error) {
			return []net.Interface{
				{Name: "eth0", HardwareAddr: net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}},
			}, nil
		},
	}

	agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if agent.machineID != "mac-020000000001" {
		t.Fatalf("machineID = %q, want %q", agent.machineID, "mac-020000000001")
	}
}
