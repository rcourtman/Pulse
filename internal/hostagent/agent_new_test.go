package hostagent

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
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

func TestNew_AllowsLocalNetworkHTTPPulseURL(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{
		PulseURL:  "http://192.168.0.98:7655",
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.trimmedPulseURL != "http://192.168.0.98:7655" {
		t.Fatalf("trimmedPulseURL = %q", agent.trimmedPulseURL)
	}
}

func TestNew_ResolvesProxmoxVEHostIdentity(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "pve-host",
				HostID:          "hid",
				Platform:        "debian",
				PlatformFamily:  "debian",
				PlatformVersion: "13.4",
				KernelVersion:   "7.0.0-3-pve",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		statFn: func(name string) (os.FileInfo, error) {
			if name == "/etc/pve" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		lookPathFn: func(file string) (string, error) {
			if file == "pveversion" {
				return "/usr/bin/pveversion", nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
			if name != "/usr/bin/pveversion" {
				t.Fatalf("command name = %q, want /usr/bin/pveversion", name)
			}
			return "pve-manager/9.1.9/ee7bad0a3d1546c9 (running kernel: 7.0.0-3-pve)", nil
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.platform != "linux" {
		t.Fatalf("platform = %q, want Linux runtime platform", agent.platform)
	}
	if agent.osName != proxmoxPVEOSName {
		t.Fatalf("osName = %q, want %q", agent.osName, proxmoxPVEOSName)
	}
	if agent.osVersion != "9.1.9" {
		t.Fatalf("osVersion = %q, want 9.1.9", agent.osVersion)
	}
}

func TestNew_ResolvesProxmoxVEVersionFromPackageMetadata(t *testing.T) {
	pveVersionCalled := false
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "pve-host",
				HostID:          "hid",
				Platform:        "raspbian",
				PlatformFamily:  "debian",
				PlatformVersion: "12.12",
				KernelVersion:   "6.12.47+rpt-rpi-v8",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		statFn: func(name string) (os.FileInfo, error) {
			if name == "/etc/pve" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "dpkg-query":
				return "/usr/bin/dpkg-query", nil
			case "pveversion":
				return "/usr/bin/pveversion", nil
			default:
				return "", os.ErrNotExist
			}
		},
		commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
			switch name {
			case "/usr/bin/dpkg-query":
				wantArgs := []string{"-W", "-f=${Version}", "pve-manager"}
				if !reflect.DeepEqual(arg, wantArgs) {
					t.Fatalf("dpkg-query args = %#v, want %#v", arg, wantArgs)
				}
				return "8.3.3", nil
			case "/usr/bin/pveversion":
				pveVersionCalled = true
				return "pve-manager/8.3.3/bbba1c53a1a65b24", nil
			default:
				t.Fatalf("unexpected command %q", name)
				return "", nil
			}
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.osName != proxmoxPVEOSName {
		t.Fatalf("osName = %q, want %q", agent.osName, proxmoxPVEOSName)
	}
	if agent.osVersion != "8.3.3" {
		t.Fatalf("osVersion = %q, want 8.3.3", agent.osVersion)
	}
	if pveVersionCalled {
		t.Fatal("expected package metadata to avoid slow pveversion fallback")
	}
}

func TestNew_UsesLongerProxmoxVEVersionFallbackBudget(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "pve-host",
				HostID:          "hid",
				Platform:        "raspbian",
				PlatformFamily:  "debian",
				PlatformVersion: "12.12",
				KernelVersion:   "6.12.47+rpt-rpi-v8",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		statFn: func(name string) (os.FileInfo, error) {
			if name == "/etc/pve" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		lookPathFn: func(file string) (string, error) {
			if file == "pveversion" {
				return "/usr/bin/pveversion", nil
			}
			return "", os.ErrNotExist
		},
		commandCombinedOutputFn: func(ctx context.Context, name string, arg ...string) (string, error) {
			if name != "/usr/bin/pveversion" {
				t.Fatalf("command name = %q, want /usr/bin/pveversion", name)
			}
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected pveversion context to carry a deadline")
			}
			if remaining := time.Until(deadline); remaining < 7*time.Second {
				t.Fatalf("pveversion timeout remaining = %v, want at least 7s", remaining)
			}
			return "pve-manager/8.3.3/bbba1c53a1a65b24 (running kernel: 6.12.47+rpt-rpi-v8)", nil
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.osVersion != "8.3.3" {
		t.Fatalf("osVersion = %q, want 8.3.3", agent.osVersion)
	}
}

func TestNew_DoesNotUseDebianVersionForProxmoxVE(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{
				Hostname:        "pve-host",
				HostID:          "hid",
				Platform:        "debian",
				PlatformFamily:  "debian",
				PlatformVersion: "13.4",
				KernelVersion:   "7.0.0-3-pve",
				KernelArch:      runtime.GOARCH,
			}, nil
		},
		statFn: func(name string) (os.FileInfo, error) {
			if name == "/etc/pve" {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		lookPathFn: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	agent, err := New(Config{
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agent.osName != proxmoxPVEOSName {
		t.Fatalf("osName = %q, want %q", agent.osName, proxmoxPVEOSName)
	}
	if agent.osVersion != "" {
		t.Fatalf("osVersion = %q, want empty Proxmox version when pveversion is unavailable", agent.osVersion)
	}
}

func TestCleanProxmoxPVEVersion(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "short output",
			raw:  "pve-manager/9.1.9/ee7bad0a3d1546c9 (running kernel: 7.0.0-3-pve)",
			want: "9.1.9",
		},
		{
			name: "verbose output",
			raw:  "proxmox-ve: 9.1-1\npve-manager/9.1.9/ee7bad0a3d1546c9\n",
			want: "9.1.9",
		},
		{
			name: "unrelated",
			raw:  "Debian GNU/Linux 13",
			want: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanProxmoxPVEVersion(tt.raw); got != tt.want {
				t.Fatalf("cleanProxmoxPVEVersion() = %q, want %q", got, tt.want)
			}
		})
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

func TestCommandApprovalGrantRejectionMetricClassifiesMissingGrant(t *testing.T) {
	reason := agentexec.ApprovalGrantRejectionMissing
	before := testutil.ToFloat64(commandApprovalGrantRejectionsTotal.WithLabelValues(reason))

	client := &CommandClient{
		apiToken: "runtime-token",
		agentID:  "agent-1",
	}
	result := client.executeCommand(context.Background(), executeCommandPayload{
		RequestID:  "req-1",
		Command:    "echo hello",
		ApprovalID: "approval-1",
		TargetType: "agent",
		Timeout:    5,
	})
	if result.Success {
		t.Fatalf("expected approval-grant failure, got %#v", result)
	}
	if !strings.Contains(result.Error, "approval grant is required") {
		t.Fatalf("error = %q, want missing approval grant", result.Error)
	}

	after := testutil.ToFloat64(commandApprovalGrantRejectionsTotal.WithLabelValues(reason))
	if after != before+1 {
		t.Fatalf("approval grant rejection metric = %v, want %v", after, before+1)
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

func TestNewCommandClient_SetsSecureCommandDefaults(t *testing.T) {
	logger := zerolog.Nop()
	client := NewCommandClient(Config{
		PulseURL:          "https://pulse.example",
		APIToken:          "runtime-token",
		ServerFingerprint: "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		DeploySSHUser:     "pulse-deploy",
		Logger:            &logger,
	}, "agent-1", "node-1", "linux", "1.0.0")

	if client.commandPolicy == nil {
		t.Fatalf("expected default command policy to be configured")
	}
	if client.stateDir != defaultStateDir {
		t.Fatalf("stateDir = %q, want %q", client.stateDir, defaultStateDir)
	}
	if client.serverFingerprint != "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd" {
		t.Fatalf("serverFingerprint = %q, want configured SHA-256 pin", client.serverFingerprint)
	}
	if client.deploySSHUser != "pulse-deploy" {
		t.Fatalf("deploySSHUser = %q, want %q", client.deploySSHUser, "pulse-deploy")
	}

	if err := client.authorizeCommand(executeCommandPayload{Command: "systemctl restart nginx"}); err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected approval-required command to fail closed, got %v", err)
	}
	approvedPayload := testApprovedCommandPayload(t, client, executeCommandPayload{RequestID: "r1", Command: "systemctl restart nginx"})
	if err := client.authorizeCommand(approvedPayload); err != nil {
		t.Fatalf("expected approved command to pass, got %v", err)
	}
	if err := client.authorizeCommand(executeCommandPayload{Command: "rm -rf /"}); err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("expected blocked command to be rejected, got %v", err)
	}
}

func TestAgentApplyRemoteConfigUpdatesRuntimeSnapshot(t *testing.T) {
	logger := zerolog.Nop()
	commandsEnabled := false
	agent := &Agent{
		cfg: Config{
			AgentType:      "unified",
			EnableCommands: true,
			Interval:       30 * time.Second,
			DiskExclude:    []string{"/old"},
			Tags:           []string{"edge"},
		},
		interval:            30 * time.Second,
		reportIP:            "192.0.2.10",
		remoteConfigChanged: make(chan struct{}, 1),
		logger:              logger,
	}

	agent.ApplyRemoteConfig(map[string]interface{}{
		"interval":     "45s",
		"report_ip":    "192.0.2.20",
		"disable_ceph": true,
	}, &commandsEnabled)

	snapshot := agent.runtimeConfigSnapshot()
	if snapshot.interval != 45*time.Second {
		t.Fatalf("interval = %s, want 45s", snapshot.interval)
	}
	if snapshot.reportIP != "192.0.2.20" {
		t.Fatalf("reportIP = %q, want %q", snapshot.reportIP, "192.0.2.20")
	}
	if snapshot.commandsEnabled {
		t.Fatal("commandsEnabled = true, want false")
	}
	if !snapshot.disableCeph {
		t.Fatal("disableCeph = false, want true")
	}
	expectedMetadata, err := remoteconfig.BuildDesiredConfigMetadata(&commandsEnabled, map[string]interface{}{
		"interval":     "45s",
		"report_ip":    "192.0.2.20",
		"disable_ceph": true,
	})
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata: %v", err)
	}
	if snapshot.appliedConfig == nil || snapshot.appliedConfig.Version != expectedMetadata.Version || snapshot.appliedConfig.Hash != expectedMetadata.Hash {
		t.Fatalf("applied config = %+v, want %+v", snapshot.appliedConfig, expectedMetadata)
	}
	select {
	case <-agent.remoteConfigChanged:
	default:
		t.Fatal("expected interval change signal")
	}
}

func TestAgentApplyRemoteConfigKeepsPriorFingerprintWhenRestartIsRequired(t *testing.T) {
	prior := &agentshost.ConfigFingerprint{Version: "host-agent-config/v1", Hash: "sha256:prior"}
	agent := &Agent{
		cfg:                 Config{AppliedConfig: prior},
		remoteConfigChanged: make(chan struct{}, 1),
		logger:              zerolog.Nop(),
	}

	agent.ApplyRemoteConfig(map[string]interface{}{"enable_docker": true}, nil)

	snapshot := agent.runtimeConfigSnapshot()
	if snapshot.appliedConfig == nil || snapshot.appliedConfig.Hash != prior.Hash {
		t.Fatalf("restart-required config should retain prior applied fingerprint, got %+v", snapshot.appliedConfig)
	}
}

func TestNewRejectsInvalidDeploySSHUser(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	_, err := New(Config{
		APIToken:      "token",
		DeploySSHUser: "bad user",
		LogLevel:      zerolog.InfoLevel,
		Collector:     mc,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid deploy SSH user") {
		t.Fatalf("expected invalid deploy SSH user error, got %v", err)
	}
}

func TestNewCommandClient_BuildWebSocketOriginFollowsCanonicalPulseURL(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name     string
		pulseURL string
		want     string
		wantErr  bool
	}{
		{
			name:     "hosted https origin",
			pulseURL: "https://pulse.example/base/",
			want:     "https://pulse.example",
		},
		{
			name:     "loopback http origin",
			pulseURL: "http://127.0.0.1:7655/pulse",
			want:     "http://127.0.0.1:7655",
		},
		{
			name:     "rejects non loopback plaintext",
			pulseURL: "http://pulse.example",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCommandClient(Config{
				PulseURL: tt.pulseURL,
				Logger:   &logger,
			}, "agent-1", "node-1", "linux", "1.0.0")

			got, err := client.buildWebSocketOrigin()
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildWebSocketOrigin() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("buildWebSocketOrigin() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew_UsesPinnedServerFingerprintForHTTPTransport(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{
		PulseURL:          "https://pulse.example",
		APIToken:          "token",
		ServerFingerprint: "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		LogLevel:          zerolog.InfoLevel,
		Collector:         mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	httpTransport, ok := agent.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", agent.httpClient.Transport)
	}
	if httpTransport.TLSClientConfig == nil {
		t.Fatal("expected TLS config to be configured")
	}
	if !httpTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected fingerprint-pinned transport to bypass CA verification in favor of explicit pinning")
	}
	if httpTransport.TLSClientConfig.VerifyPeerCertificate == nil {
		t.Fatal("expected fingerprint-pinned transport to verify peer certificates explicitly")
	}
}

func TestNew_ResolvesVendorNASIdentityFromPlatformFiles(t *testing.T) {
	t.Run("synology dsm from version file", func(t *testing.T) {
		mc := &mockCollector{
			goos: "linux",
			hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				return &gohost.InfoStat{
					Hostname:        "nas",
					HostID:          "hid",
					Platform:        "linux",
					PlatformFamily:  "linux",
					PlatformVersion: "",
					KernelArch:      runtime.GOARCH,
				}, nil
			},
			readFileFn: func(name string) ([]byte, error) {
				switch name {
				case "/etc.defaults/VERSION":
					return []byte(`majorversion="7"
minorversion="2"
productversion="7.2.2"
buildnumber="72806"
smallfixnumber="3"
`), nil
				default:
					return nil, os.ErrNotExist
				}
			},
		}

		agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if agent.osName != "Synology DSM" {
			t.Fatalf("osName = %q, want %q", agent.osName, "Synology DSM")
		}
		if agent.osVersion != "7.2.2-72806 Update 3" {
			t.Fatalf("osVersion = %q, want %q", agent.osVersion, "7.2.2-72806 Update 3")
		}
	})

	t.Run("qnap quts from config file", func(t *testing.T) {
		mc := &mockCollector{
			goos: "linux",
			hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				return &gohost.InfoStat{
					Hostname:        "qnap",
					HostID:          "hid",
					Platform:        "linux",
					PlatformFamily:  "linux",
					PlatformVersion: "",
					KernelArch:      runtime.GOARCH,
				}, nil
			},
			readFileFn: func(name string) ([]byte, error) {
				switch name {
				case "/etc/config/uLinux.conf":
					return []byte(`Version = 5.2.0
Display_Name = QuTS hero
Platform = QNAP
`), nil
				default:
					return nil, os.ErrNotExist
				}
			},
		}

		agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if agent.osName != "QNAP QuTS" {
			t.Fatalf("osName = %q, want %q", agent.osName, "QNAP QuTS")
		}
		if agent.osVersion != "5.2.0" {
			t.Fatalf("osVersion = %q, want %q", agent.osVersion, "5.2.0")
		}
	})

	t.Run("unraid from version file", func(t *testing.T) {
		mc := &mockCollector{
			goos: "linux",
			hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				return &gohost.InfoStat{
					Hostname:        "unraid",
					HostID:          "hid",
					Platform:        "linux",
					PlatformFamily:  "linux",
					PlatformVersion: "",
					KernelArch:      runtime.GOARCH,
				}, nil
			},
			readFileFn: func(name string) ([]byte, error) {
				switch name {
				case "/etc/unraid-version":
					return []byte("Unraid OS 7.1.0\n"), nil
				default:
					return nil, os.ErrNotExist
				}
			},
		}

		agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if agent.osName != "Unraid" {
			t.Fatalf("osName = %q, want %q", agent.osName, "Unraid")
		}
		if agent.osVersion != "7.1.0" {
			t.Fatalf("osVersion = %q, want %q", agent.osVersion, "7.1.0")
		}
		if got := normalisePlatform("unraid"); got != "linux" {
			t.Fatalf("normalisePlatform(unraid) = %q, want linux", got)
		}
	})

	t.Run("unraid normalizes slackware base platform", func(t *testing.T) {
		mc := &mockCollector{
			goos: "linux",
			hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				return &gohost.InfoStat{
					Hostname:        "tower",
					HostID:          "hid",
					Platform:        "slackware",
					PlatformFamily:  "slackware",
					PlatformVersion: "15.0+",
					KernelVersion:   "6.12.54-Unraid",
					KernelArch:      runtime.GOARCH,
				}, nil
			},
			readFileFn: func(name string) ([]byte, error) {
				switch name {
				case "/etc/unraid-version":
					return []byte(`version="7.2.2"`), nil
				default:
					return nil, os.ErrNotExist
				}
			},
		}

		agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if agent.platform != "linux" {
			t.Fatalf("platform = %q, want linux", agent.platform)
		}
		if agent.osName != "Unraid" {
			t.Fatalf("osName = %q, want Unraid", agent.osName)
		}
		if agent.osVersion != "7.2.2" {
			t.Fatalf("osVersion = %q, want 7.2.2", agent.osVersion)
		}
	})

	t.Run("unraid from os release fallback", func(t *testing.T) {
		mc := &mockCollector{
			goos: "linux",
			hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
				return &gohost.InfoStat{
					Hostname:        "tower",
					HostID:          "hid",
					Platform:        "slackware",
					PlatformFamily:  "slackware",
					PlatformVersion: "15.0+",
					KernelArch:      runtime.GOARCH,
				}, nil
			},
			readFileFn: func(name string) ([]byte, error) {
				switch name {
				case "/etc/os-release":
					return []byte(`NAME="Unraid OS"
ID="unraid-os"
VERSION_ID="7.2.2"
PRETTY_NAME="Unraid OS 7.2 x86_64"
`), nil
				default:
					return nil, os.ErrNotExist
				}
			},
		}

		agent, err := New(Config{APIToken: "token", LogLevel: zerolog.InfoLevel, Collector: mc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if agent.platform != "linux" {
			t.Fatalf("platform = %q, want linux", agent.platform)
		}
		if agent.osName != "Unraid" {
			t.Fatalf("osName = %q, want Unraid", agent.osName)
		}
		if agent.osVersion != "7.2.2" {
			t.Fatalf("osVersion = %q, want 7.2.2", agent.osVersion)
		}
	})
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
		APIToken:             "token",
		AgentID:              "agent-1",
		AgentType:            "unified",
		AgentVersion:         "6.0.0-rc.1",
		LogLevel:             zerolog.InfoLevel,
		Collector:            mc,
		updatedFromVersionFn: func() string { return "5.1.14" },
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
			name: "public http rejected",
			url:  "http://example.com",
			want: "must use https unless host is loopback or local/private",
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

func TestNew_AllowsSelfHostedLocalHTTPPulseURL(t *testing.T) {
	mc := &mockCollector{
		hostInfoFn: func(context.Context) (*gohost.InfoStat, error) {
			return &gohost.InfoStat{Hostname: "host", HostID: "hid", KernelArch: runtime.GOARCH}, nil
		},
	}

	agent, err := New(Config{
		PulseURL:  "http://ct-pulse.home:7655/",
		APIToken:  "token",
		LogLevel:  zerolog.InfoLevel,
		Collector: mc,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if agent.cfg.PulseURL != "http://ct-pulse.home:7655" {
		t.Fatalf("cfg.PulseURL = %q, want http://ct-pulse.home:7655", agent.cfg.PulseURL)
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

func TestExecuteCommandPayload_TrustedBypassesAgentApprovalGate(t *testing.T) {
	// Discovery deep scans wrap probes in `docker exec ...` and dispatch them
	// over the command WebSocket with Trusted=true. The agent-side authorize
	// gate must honor that flag (matching the agentexec server-side behavior)
	// or every scan returns empty stdout because the agent refuses to run the
	// command — which is the bug this regression test guards against.
	c := &CommandClient{commandPolicy: agentexec.DefaultPolicy()}

	requireApproval := executeCommandPayload{
		RequestID: "r1",
		Command:   "docker exec pbs sh -c 'cat /etc/os-release'",
		Timeout:   1,
	}
	if err := c.authorizeCommand(requireApproval); err == nil {
		t.Fatalf("baseline: agent must reject untrusted docker exec without ApprovalID")
	}

	requireApproval.Trusted = true
	if err := c.authorizeCommand(requireApproval); err != nil {
		t.Fatalf("Trusted=true must bypass the agent approval gate, got %v", err)
	}

	// PolicyBlock is still enforced even when Trusted.
	blocked := executeCommandPayload{
		RequestID: "r2",
		Command:   "rm -rf /",
		Trusted:   true,
		Timeout:   1,
	}
	if err := c.authorizeCommand(blocked); err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("Trusted must not bypass PolicyBlock; got %v", err)
	}

	// toAgentExecPayload must propagate Trusted so the agentexec server side
	// can also bypass its mirrored approval mint step.
	round := requireApproval.toAgentExecPayload()
	if !round.Trusted {
		t.Fatalf("toAgentExecPayload dropped Trusted field; round-trip must preserve it")
	}
}

func TestNormalisePlatformCanonicalisesReportedCaptions(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		want     string
	}{
		// gopsutil reports the descriptive OS caption on Windows, not a
		// canonical token (refs #1555).
		{"windows caption", "Microsoft Windows 11 Pro", "windows"},
		{"darwin", "darwin", "macos"},
		{"freebsd caption", "FreeBSD 14.1-RELEASE", "freebsd"},
		{"linux distro preserved", "ubuntu", "ubuntu"},
		{"unraid maps to linux runtime", "unraid", "linux"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalisePlatform(tc.platform); got != tc.want {
				t.Fatalf("normalisePlatform(%q) = %q, want %q", tc.platform, got, tc.want)
			}
		})
	}
}

func TestNormaliseRuntimePlatformUsesBuildTarget(t *testing.T) {
	cases := []struct {
		name     string
		goos     string
		reported string
		want     string
	}{
		{"linux distro", "linux", "mageia", "linux"},
		{"macOS caption", "darwin", "darwin", "macos"},
		{"FreeBSD caption", "freebsd", "FreeBSD 14.1-RELEASE", "freebsd"},
		{"Windows caption", "windows", "Microsoft Windows 11 Pro", "windows"},
		{"unknown build target preserves report", "plan9", "plan9", "plan9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normaliseRuntimePlatform(tc.goos, tc.reported); got != tc.want {
				t.Fatalf("normaliseRuntimePlatform(%q, %q) = %q, want %q", tc.goos, tc.reported, got, tc.want)
			}
		})
	}
}
