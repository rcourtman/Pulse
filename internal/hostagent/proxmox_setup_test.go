package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestParseProxmoxProductType(t *testing.T) {
	tests := []struct {
		name     string
		rawType  string
		expected proxmoxProductType
	}{
		{name: "pve", rawType: "pve", expected: proxmoxProductPVE},
		{name: "pbs uppercase", rawType: "PBS", expected: proxmoxProductPBS},
		{name: "trimmed", rawType: "  pve  ", expected: proxmoxProductPVE},
		{name: "invalid", rawType: "invalid", expected: proxmoxProductUnknown},
		{name: "empty", rawType: "", expected: proxmoxProductUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseProxmoxProductType(tt.rawType); got != tt.expected {
				t.Fatalf("parseProxmoxProductType(%q) = %q, want %q", tt.rawType, got, tt.expected)
			}
		})
	}
}

func TestRegisterWithPulse_Payload(t *testing.T) {
	var gotPayload autoRegisterRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/api/auto-register" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if gotToken := r.Header.Get("X-API-Token"); gotToken != "pulse-token" {
			t.Fatalf("unexpected X-API-Token header %q", gotToken)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &ProxmoxSetup{
		logger:     zerolog.Nop(),
		httpClient: server.Client(),
		pulseURL:   server.URL,
		apiToken:   "pulse-token",
		hostname:   "node-1",
	}

	err := p.registerWithPulse(context.Background(), proxmoxProductPVE, "https://10.0.0.4:8006", "token-id", "token-value")
	if err != nil {
		t.Fatalf("registerWithPulse failed: %v", err)
	}

	if gotPayload.Type != proxmoxProductPVE {
		t.Fatalf("unexpected type %q", gotPayload.Type)
	}
	if gotPayload.Host != "https://10.0.0.4:8006" {
		t.Fatalf("unexpected host %q", gotPayload.Host)
	}
	if gotPayload.ServerName != "node-1" {
		t.Fatalf("unexpected serverName %q", gotPayload.ServerName)
	}
	if gotPayload.TokenID != "token-id" {
		t.Fatalf("unexpected tokenId %q", gotPayload.TokenID)
	}
	if gotPayload.TokenValue != "token-value" {
		t.Fatalf("unexpected tokenValue %q", gotPayload.TokenValue)
	}
	if gotPayload.Source != autoRegisterSourceAgent {
		t.Fatalf("unexpected source %q", gotPayload.Source)
	}
}

func TestParseTokenValue_RegexFallback(t *testing.T) {
	p := &ProxmoxSetup{logger: zerolog.Nop()}
	output := "some noise then 7c5709fb-6aee-4c32-8b9f-5c2656912345 more noise"
	got := p.parseTokenValue(output)
	if got != "7c5709fb-6aee-4c32-8b9f-5c2656912345" {
		t.Errorf("got %s", got)
	}
	if p.parseTokenValue("no uuid") != "" {
		t.Error("expected empty")
	}
}

func TestParsePBSTokenValue_RegexFallback(t *testing.T) {
	p := &ProxmoxSetup{logger: zerolog.Nop()}
	output := `random "value" : "pbs-abc-123" text`
	got := p.parsePBSTokenValue(output)
	if got != "pbs-abc-123" {
		t.Errorf("got %s", got)
	}
	if p.parsePBSTokenValue("no match") != "" {
		t.Error("expected empty")
	}
}

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name       string
		ips        []string
		hostnameIP string
		expected   string
	}{
		{
			name:       "prefers hostname IP matching 10.x range over 192.x range",
			ips:        []string{"10.0.0.1", "192.168.1.100"},
			hostnameIP: "10.0.0.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "prefers 192.168.x.x LAN over 172.20.x.x hostname match (Corosync)",
			ips:        []string{"172.20.0.1", "192.168.1.100"},
			hostnameIP: "172.20.0.1",
			expected:   "192.168.1.100",
		},
		{
			name:       "hostname breaks tie between equal subnets",
			ips:        []string{"10.0.0.1", "10.0.0.2"},
			hostnameIP: "10.0.0.2",
			expected:   "10.0.0.2",
		},
		{
			name:       "falls back to score heuristic if no hostname IP match",
			ips:        []string{"10.0.0.1", "192.168.1.100"},
			hostnameIP: "10.0.0.2",
			expected:   "192.168.1.100",
		},
		{
			name:       "prefers 192.168.x.x over corosync 172.20.x.x",
			ips:        []string{"172.20.0.80", "192.168.1.100"},
			hostnameIP: "",
			expected:   "192.168.1.100",
		},
		{
			name:     "returns corosync IP if only option",
			ips:      []string{"127.0.0.1", "172.20.0.80"},
			expected: "172.20.0.80",
		},
		{
			name:     "empty list returns empty",
			ips:      []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestIP(tt.ips, tt.hostnameIP)
			if result != tt.expected {
				t.Errorf("selectBestIP(%v, %q) = %q, want %q", tt.ips, tt.hostnameIP, result, tt.expected)
			}
		})
	}
}

func TestScoreIPv4(t *testing.T) {
	tests := []struct {
		ip            string
		expectedScore int
	}{
		{"192.168.1.1", 100},
		{"10.0.0.1", 90},
		{"100.64.0.1", 85},
		{"10.32.0.1", 70},
		{"172.16.0.1", 50},
		{"169.254.1.1", 0},
		{"8.8.8.8", 30},
		{"not-an-ip", 0},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := scoreIPv4(tt.ip)
			if result != tt.expectedScore {
				t.Errorf("scoreIPv4(%q) = %d, want %d", tt.ip, result, tt.expectedScore)
			}
		})
	}
}

func TestStateFileForType(t *testing.T) {
	setup := &ProxmoxSetup{}
	if setup.stateFileForType("pve") != stateFilePVE {
		t.Error("wrong PVE state file")
	}
	if setup.stateFileForType("pbs") != stateFilePBS {
		t.Error("wrong PBS state file")
	}
	if setup.stateFileForType("unknown") != stateFilePath {
		t.Error("wrong unknown state file")
	}
}

func TestGetHostURL(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{
		logger:    zerolog.Nop(),
		collector: mc,
		hostname:  "test-host",
	}

	t.Run("uses reportIP override", func(t *testing.T) {
		p.reportIP = "1.2.3.4"
		if got := p.getHostURL(context.Background(), "pve"); got != "https://1.2.3.4:8006" {
			t.Errorf("got %s", got)
		}
	})

	t.Run("formats IPv6 reportIP override", func(t *testing.T) {
		p.reportIP = "2001:db8::1"
		if got := p.getHostURL(context.Background(), proxmoxProductPBS); got != "https://[2001:db8::1]:8007" {
			t.Errorf("got %s", got)
		}
	})

	t.Run("uses IP that reaches pulse", func(t *testing.T) {
		p.reportIP = ""
		p.pulseURL = "https://pulse:7655"
		mc.dialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
			return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("10.1.1.1")}}, nil
		}
		if got := p.getHostURL(context.Background(), "pve"); got != "https://10.1.1.1:8006" {
			t.Errorf("got %s", got)
		}
	})

	t.Run("falls back to hostname -I and best IP", func(t *testing.T) {
		p.reportIP = ""
		p.pulseURL = "" // trigger getIPThatReachesPulse returning empty
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) { return nil, fmt.Errorf("fail") }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			if name == "hostname" && len(arg) > 0 && arg[0] == "-I" {
				return "10.0.0.5 172.20.0.1 127.0.0.1", nil
			}
			return "", nil
		}
		mc.hostnameFn = func() (string, error) { return "test-host", nil }
		mc.lookupIPFn = func(h string) ([]net.IP, error) { return nil, nil }

		if got := p.getHostURL(context.Background(), "pbs"); got != "https://10.0.0.5:8007" {
			t.Errorf("got %s", got)
		}
	})

	t.Run("final fallback to hostname", func(t *testing.T) {
		p.pulseURL = ""
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) { return nil, fmt.Errorf("fail") }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			return "", fmt.Errorf("fail")
		}
		if got := p.getHostURL(context.Background(), "pve"); got != "https://test-host:8006" {
			t.Errorf("got %s", got)
		}
	})

	t.Run("hostname probe uses caller context", func(t *testing.T) {
		p.reportIP = ""
		p.pulseURL = ""
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) { return nil, fmt.Errorf("fail") }

		var sawCanceledCtx bool
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			if name == "hostname" && len(arg) > 0 && arg[0] == "-I" && ctx.Err() == context.Canceled {
				sawCanceledCtx = true
			}
			return "", context.Canceled
		}

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if got := p.getHostURL(canceledCtx, "pve"); got != "https://test-host:8006" {
			t.Errorf("got %s", got)
		}
		if !sawCanceledCtx {
			t.Fatalf("expected hostname probe to observe canceled caller context")
		}
	})
}

func TestGetIPThatReachesPulse_IPv6Target(t *testing.T) {
	mc := &mockCollector{}
	var dialAddress string
	mc.dialTimeoutFn = func(network, address string, timeout time.Duration) (net.Conn, error) {
		dialAddress = address
		return nil, fmt.Errorf("expected failure to stop flow")
	}

	p := &ProxmoxSetup{
		logger:    zerolog.Nop(),
		collector: mc,
		pulseURL:  "https://[2001:db8::1]",
	}

	_ = p.getIPThatReachesPulse()
	if dialAddress != "[2001:db8::1]:443" {
		t.Fatalf("dial address = %q, want %q", dialAddress, "[2001:db8::1]:443")
	}
}

func TestProxmoxSetup_RunForType(t *testing.T) {
	mc := &mockCollector{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewProxmoxSetup(zerolog.Nop(), server.Client(), mc, server.URL, "token", "pve", "host", "", false)

	t.Run("skips if already registered", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, nil } // exists
		res, err := p.runForType(context.Background(), "pve")
		if err != nil || res != nil {
			t.Errorf("expected skip")
		}
	})

	t.Run("performs registration successfully", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			// pveum user token add pulse-monitor@pam ... --privsep 0
			// arg[0]=user, arg[1]=token, arg[2]=add
			if name == "pveum" && len(arg) > 2 && arg[1] == "token" && arg[2] == "add" {
				return "│ value │ my-token │", nil
			}
			return "", nil
		}
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return nil }
		mc.writeFileFn = func(f string, d []byte, m os.FileMode) error { return nil }
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) {
			return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}}, nil
		}

		res, err := p.runForType(context.Background(), "pve")
		if err != nil || res == nil || !res.Registered {
			t.Fatalf("failed: %v", err)
		}
		if res.TokenValue != "my-token" {
			t.Errorf("got %s", res.TokenValue)
		}
	})
}

func TestPrivProbeRoleName_Sanitizes(t *testing.T) {
	got := privProbeRoleName("VM.GuestAgent.Audit")
	if got != "PulseTmpPrivCheck_VM_GuestAgent_Audit" {
		t.Fatalf("unexpected role name: %q", got)
	}

	got2 := privProbeRoleName("a:b/c d,e.f")
	if strings.ContainsAny(got2, ".:/ ,") {
		t.Fatalf("expected sanitized role name, got: %q", got2)
	}
}

func TestProxmoxSetup_ProbePVEPrivilege_CreatesAndDeletesRole(t *testing.T) {
	mc := &mockCollector{}
	addCalls := 0
	deleteCalls := 0

	mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
		if name != "pveum" || len(arg) < 3 {
			return "", nil
		}
		if arg[0] == "role" && arg[1] == "add" && strings.HasPrefix(arg[2], "PulseTmpPrivCheck_") {
			addCalls++
			return "", nil
		}
		if arg[0] == "role" && arg[1] == "delete" && strings.HasPrefix(arg[2], "PulseTmpPrivCheck_") {
			deleteCalls++
			return "", nil
		}
		return "", nil
	}

	p := &ProxmoxSetup{
		logger:    zerolog.Nop(),
		collector: mc,
	}

	if ok := p.probePVEPrivilege(context.Background(), "Sys.Audit"); !ok {
		t.Fatalf("expected probe to succeed")
	}
	if addCalls != 1 || deleteCalls != 1 {
		t.Fatalf("expected 1 add and 1 delete call, got add=%d delete=%d", addCalls, deleteCalls)
	}
}

func TestProxmoxSetup_ProbePVEPrivilege_ReturnsFalseOnAddError(t *testing.T) {
	mc := &mockCollector{}
	deleteCalls := 0
	mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
		if name == "pveum" && len(arg) >= 3 && arg[0] == "role" && arg[1] == "add" && strings.HasPrefix(arg[2], "PulseTmpPrivCheck_") {
			return "", fmt.Errorf("unknown privilege")
		}
		if name == "pveum" && len(arg) >= 3 && arg[0] == "role" && arg[1] == "delete" {
			deleteCalls++
		}
		return "", nil
	}

	p := &ProxmoxSetup{
		logger:    zerolog.Nop(),
		collector: mc,
	}

	if ok := p.probePVEPrivilege(context.Background(), "VM.Monitor"); ok {
		t.Fatalf("expected probe to fail")
	}
	if deleteCalls != 0 {
		t.Fatalf("expected no delete calls when add fails, got %d", deleteCalls)
	}
}

func TestProxmoxSetup_ConfigurePVEPermissions_FallsBackToGuestAgentAudit(t *testing.T) {
	mc := &mockCollector{}
	var pulseMonitorPrivs string

	mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
		if name != "pveum" {
			return "", nil
		}

		// Temp role privilege probe: pveum role add PulseTmpPrivCheck_* -privs <priv>
		if len(arg) >= 5 && arg[0] == "role" && arg[1] == "add" && strings.HasPrefix(arg[2], "PulseTmpPrivCheck_") {
			for i := 0; i < len(arg)-1; i++ {
				if arg[i] == "-privs" {
					priv := arg[i+1]
					if priv == "VM.Monitor" {
						return "", fmt.Errorf("unknown privilege")
					}
					return "", nil
				}
			}
		}

		// Capture configured privileges for PulseMonitor role.
		if len(arg) >= 5 && arg[0] == "role" && (arg[1] == "modify" || arg[1] == "add") && arg[2] == proxmoxMonitorRole {
			for i := 0; i < len(arg)-1; i++ {
				if arg[i] == "-privs" {
					pulseMonitorPrivs = arg[i+1]
				}
			}
			return "", nil
		}

		return "", nil
	}

	p := &ProxmoxSetup{
		logger:    zerolog.Nop(),
		collector: mc,
	}

	p.configurePVEPermissions(context.Background())

	if !strings.Contains(pulseMonitorPrivs, "VM.GuestAgent.Audit") {
		t.Fatalf("expected PulseMonitor privileges to include VM.GuestAgent.Audit, got %q", pulseMonitorPrivs)
	}
	if strings.Contains(pulseMonitorPrivs, "VM.Monitor") {
		t.Fatalf("did not expect PulseMonitor privileges to include VM.Monitor when it is unavailable, got %q", pulseMonitorPrivs)
	}
	if strings.Contains(pulseMonitorPrivs, " ") {
		t.Fatalf("expected comma-separated privileges, got %q", pulseMonitorPrivs)
	}
}

func TestProxmoxSetup_RunAll(t *testing.T) {
	mc := &mockCollector{}

	t.Run("detects both and runs both", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), http.DefaultClient, mc, "https://pulse", "token", "", "host", "", false)
		mc.lookPathFn = func(file string) (string, error) { return "/bin/" + file, nil }
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			// PVE: pveum user token add ...
			if name == "pveum" && len(arg) > 2 && arg[1] == "token" {
				return "│ value │ v1 │", nil
			}
			// PBS: proxmox-backup-manager user generate-token ...
			if name == "proxmox-backup-manager" && len(arg) > 1 && arg[1] == "generate-token" {
				return `{"value":"v2"}`, nil
			}
			return "", nil
		}
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return nil }
		mc.writeFileFn = func(f string, d []byte, m os.FileMode) error { return nil }
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) {
			return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}}, nil
		}

		hc := &http.Client{Transport: &mockTransport{statusCode: 200}}
		p.httpClient = hc

		results, err := p.RunAll(context.Background())
		if err != nil || len(results) != 2 {
			t.Errorf("expected 2 results, got %d (err: %v)", len(results), err)
		}
	})

	t.Run("forced type", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), http.DefaultClient, mc, "https://pulse", "token", "pbs", "host", "", false)
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			if name == "proxmox-backup-manager" && len(arg) > 1 && arg[1] == "generate-token" {
				return `{"value":"v2"}`, nil
			}
			return "", nil
		}

		results, _ := p.RunAll(context.Background())
		if len(results) != 1 || results[0].ProxmoxType != "pbs" {
			t.Errorf("expected pbs result")
		}
	})

	t.Run("invalid forced type returns error", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), http.DefaultClient, mc, "https://pulse", "token", "invalid", "host", "", false)
		if _, err := p.RunAll(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid proxmox type") {
			t.Fatalf("expected proxmox type validation error, got %v", err)
		}
	})
}

func TestIsTypeRegistered_Legacy(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{collector: mc}

	t.Run("legacy state file exists and pve installed", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) {
			if name == stateFilePath {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}
		mc.lookPathFn = func(file string) (string, error) {
			if file == "pvesh" {
				return "/bin/pvesh", nil
			}
			return "", os.ErrNotExist
		}
		if !p.isTypeRegistered("pve") {
			t.Error("expected registered via legacy pve")
		}
	})

	t.Run("legacy state file exists and pbs only", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) {
			if name == stateFilePath {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}
		mc.lookPathFn = func(file string) (string, error) {
			if file == "proxmox-backup-manager" {
				return "/bin/pbs", nil
			}
			return "", os.ErrNotExist
		}
		if !p.isTypeRegistered("pbs") {
			t.Error("expected registered via legacy pbs")
		}
	})

	t.Run("legacy exists but no products found", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) {
			if name == stateFilePath {
				return nil, nil
			}
			return nil, os.ErrNotExist
		}
		mc.lookPathFn = func(file string) (string, error) { return "", os.ErrNotExist }
		if p.isTypeRegistered("pve") {
			t.Error("expected false")
		}
	})

	t.Run("nothing exists", func(t *testing.T) {
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		if p.isTypeRegistered("pve") {
			t.Error("expected false")
		}
	})
}

func TestProxmoxSetup_MarkAsRegistered_Errors(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{collector: mc, logger: zerolog.Nop()}

	t.Run("mkdir error", func(t *testing.T) {
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return fmt.Errorf("fail") }
		p.markAsRegistered() // should just walk away
	})

	t.Run("write error", func(t *testing.T) {
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return nil }
		mc.writeFileFn = func(f string, d []byte, m os.FileMode) error { return fmt.Errorf("fail") }
		p.markAsRegistered()
	})
}

func TestProxmoxSetup_MarkAsRegistered_EnforcesPrivatePermissions(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{collector: mc, logger: zerolog.Nop()}

	var gotDirPerm os.FileMode
	var gotFilePerm os.FileMode
	chmodPerms := make(map[string]os.FileMode)

	mc.mkdirAllFn = func(path string, perm os.FileMode) error {
		gotDirPerm = perm
		return nil
	}
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error {
		gotFilePerm = perm
		return nil
	}
	mc.chmodFn = func(name string, mode os.FileMode) error {
		chmodPerms[name] = mode
		return nil
	}

	p.markAsRegistered()

	if gotDirPerm != proxmoxStateDirPerm {
		t.Fatalf("MkdirAll perm = %o, want %o", gotDirPerm, proxmoxStateDirPerm)
	}
	if gotFilePerm != proxmoxStateFilePerm {
		t.Fatalf("WriteFile perm = %o, want %o", gotFilePerm, proxmoxStateFilePerm)
	}
	if chmodPerms[stateFileDir] != proxmoxStateDirPerm {
		t.Fatalf("Chmod(%q) = %o, want %o", stateFileDir, chmodPerms[stateFileDir], proxmoxStateDirPerm)
	}
	if chmodPerms[stateFilePath] != proxmoxStateFilePerm {
		t.Fatalf("Chmod(%q) = %o, want %o", stateFilePath, chmodPerms[stateFilePath], proxmoxStateFilePerm)
	}
}

func TestProxmoxSetup_MarkTypeAsRegistered_Errors(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{collector: mc, logger: zerolog.Nop()}

	t.Run("mkdir error", func(t *testing.T) {
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return fmt.Errorf("fail") }
		p.markTypeAsRegistered("pve")
	})

	t.Run("write error", func(t *testing.T) {
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return nil }
		mc.writeFileFn = func(f string, d []byte, m os.FileMode) error { return fmt.Errorf("fail") }
		p.markTypeAsRegistered("pve")
	})
}

func TestProxmoxSetup_MarkTypeAsRegistered_EnforcesPrivatePermissions(t *testing.T) {
	mc := &mockCollector{}
	p := &ProxmoxSetup{collector: mc, logger: zerolog.Nop()}

	var gotDirPerm os.FileMode
	var gotFilePerm os.FileMode
	chmodPerms := make(map[string]os.FileMode)
	targetFile := p.stateFileForType("pve")

	mc.mkdirAllFn = func(path string, perm os.FileMode) error {
		gotDirPerm = perm
		return nil
	}
	mc.writeFileFn = func(filename string, data []byte, perm os.FileMode) error {
		if filename != targetFile {
			t.Fatalf("write target file = %q, want %q", filename, targetFile)
		}
		gotFilePerm = perm
		return nil
	}
	mc.chmodFn = func(name string, mode os.FileMode) error {
		chmodPerms[name] = mode
		return nil
	}

	p.markTypeAsRegistered("pve")

	if gotDirPerm != proxmoxStateDirPerm {
		t.Fatalf("MkdirAll perm = %o, want %o", gotDirPerm, proxmoxStateDirPerm)
	}
	if gotFilePerm != proxmoxStateFilePerm {
		t.Fatalf("WriteFile perm = %o, want %o", gotFilePerm, proxmoxStateFilePerm)
	}
	if chmodPerms[stateFileDir] != proxmoxStateDirPerm {
		t.Fatalf("Chmod(%q) = %o, want %o", stateFileDir, chmodPerms[stateFileDir], proxmoxStateDirPerm)
	}
	if chmodPerms[targetFile] != proxmoxStateFilePerm {
		t.Fatalf("Chmod(%q) = %o, want %o", targetFile, chmodPerms[targetFile], proxmoxStateFilePerm)
	}
}

func TestProxmoxSetup_Run_TopLevel(t *testing.T) {
	mc := &mockCollector{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("auto-detects and runs", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), server.Client(), mc, server.URL, "token", "", "host", "", false)
		mc.lookPathFn = func(file string) (string, error) {
			if file == "pvesh" {
				return "/bin/pvesh", nil
			}
			return "", os.ErrNotExist
		}
		mc.statFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		mc.commandCombinedOutputFn = func(ctx context.Context, name string, arg ...string) (string, error) {
			if name == "pveum" && len(arg) > 2 && arg[1] == "token" {
				return "│ value │ v1 │", nil
			}
			return "", nil
		}
		mc.mkdirAllFn = func(p string, m os.FileMode) error { return nil }
		mc.writeFileFn = func(f string, d []byte, m os.FileMode) error { return nil }
		mc.dialTimeoutFn = func(n, a string, d time.Duration) (net.Conn, error) {
			return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("10.0.0.1")}}, nil
		}

		res, err := p.Run(context.Background())
		if err != nil || res == nil || !res.Registered || res.ProxmoxType != "pve" {
			t.Fatalf("failed: %v (res: %v)", err, res)
		}
	})

	t.Run("invalid pulse URL fails fast", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), server.Client(), mc, "not-a-url", "token", "", "host", "", false)
		if _, err := p.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid pulse URL") {
			t.Fatalf("expected pulse URL validation error, got %v", err)
		}
	})

	t.Run("missing token fails fast", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), server.Client(), mc, server.URL, " ", "", "host", "", false)
		if _, err := p.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "api token is required") {
			t.Fatalf("expected missing token error, got %v", err)
		}
	})

	t.Run("invalid proxmox type fails fast", func(t *testing.T) {
		p := NewProxmoxSetup(zerolog.Nop(), server.Client(), mc, server.URL, "token", "bad", "host", "", false)
		if _, err := p.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid proxmox type") {
			t.Fatalf("expected proxmox type validation error, got %v", err)
		}
	})
}

// Helpers

type mockConn struct {
	net.Conn
	localAddr net.Addr
}

func (m *mockConn) LocalAddr() net.Addr { return m.localAddr }
func (m *mockConn) Close() error        { return nil }

type mockTransport struct {
	statusCode int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}
