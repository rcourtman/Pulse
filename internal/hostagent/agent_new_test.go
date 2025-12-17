package hostagent

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/rs/zerolog"
	gohost "github.com/shirou/gopsutil/v4/host"
)

func TestNew_RequiresAPIToken(t *testing.T) {
	originalHostInfo := hostInfoWithContext
	t.Cleanup(func() { hostInfoWithContext = originalHostInfo })
	hostInfoWithContext = func(context.Context) (*gohost.InfoStat, error) {
		return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
	}

	_, err := New(Config{APIToken: "  ", LogLevel: zerolog.InfoLevel})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNew_NormalizesConfigAndTags(t *testing.T) {
	originalHostInfo := hostInfoWithContext
	originalReadFile := readFile
	t.Cleanup(func() {
		hostInfoWithContext = originalHostInfo
		readFile = originalReadFile
	})

	hostInfoWithContext = func(context.Context) (*gohost.InfoStat, error) {
		return &gohost.InfoStat{
			Hostname:        " host-from-info ",
			HostID:          " gopsutil-id ",
			Platform:        "Darwin",
			PlatformFamily:  "",
			PlatformVersion: "14.0",
			KernelVersion:   "6.6.0",
			KernelArch:      runtime.GOARCH,
		}, nil
	}

	readFile = func(name string) ([]byte, error) {
		if name == "/etc/machine-id" {
			return []byte("0123456789abcdef0123456789abcdef\n"), nil
		}
		return nil, os.ErrNotExist
	}

	originalTags := []string{"  tag-a ", "tag-a", "", " tag-b", "tag-b "}
	cfg := Config{
		PulseURL:           "http://example.com///",
		APIToken:           "token",
		Interval:           0,
		Tags:               originalTags,
		InsecureSkipVerify: true,
		LogLevel:           zerolog.InfoLevel,
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if agent.interval != defaultInterval {
		t.Fatalf("interval = %v, want %v", agent.interval, defaultInterval)
	}
	if agent.trimmedPulseURL != "http://example.com" {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, "http://example.com")
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

	if runtime.GOOS == "linux" {
		const want = "01234567-89ab-cdef-0123-456789abcdef"
		if agent.machineID != want {
			t.Fatalf("machineID = %q, want %q", agent.machineID, want)
		}
		if agent.agentID != want {
			t.Fatalf("agentID = %q, want %q", agent.agentID, want)
		}
	} else {
		if agent.machineID != "gopsutil-id" {
			t.Fatalf("machineID = %q, want %q", agent.machineID, "gopsutil-id")
		}
		if agent.agentID != "gopsutil-id" {
			t.Fatalf("agentID = %q, want %q", agent.agentID, "gopsutil-id")
		}
	}
}

func TestNew_DefaultPulseURL(t *testing.T) {
	originalHostInfo := hostInfoWithContext
	t.Cleanup(func() { hostInfoWithContext = originalHostInfo })
	hostInfoWithContext = func(context.Context) (*gohost.InfoStat, error) {
		return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
	}

	agent, err := New(Config{PulseURL: "", APIToken: "token", LogLevel: zerolog.InfoLevel})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.trimmedPulseURL != "http://localhost:7655" {
		t.Fatalf("trimmedPulseURL = %q, want %q", agent.trimmedPulseURL, "http://localhost:7655")
	}
}

func TestNew_FallsBackToHostnameWhenMachineIDEmpty(t *testing.T) {
	originalHostInfo := hostInfoWithContext
	originalReadFile := readFile
	t.Cleanup(func() {
		hostInfoWithContext = originalHostInfo
		readFile = originalReadFile
	})

	hostInfoWithContext = func(context.Context) (*gohost.InfoStat, error) {
		return &gohost.InfoStat{
			Hostname:   "host-from-info",
			HostID:     "",
			KernelArch: runtime.GOARCH,
		}, nil
	}
	readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }

	agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel})
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
