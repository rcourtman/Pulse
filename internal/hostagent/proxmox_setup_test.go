package hostagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

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
			expected:   "192.168.1.100", // LAN (100) > Cluster (50 + 40 = 90)
		},
		{
			name:       "hostname breaks tie between equal subnets (e.g. two 10.x.x.x)",
			ips:        []string{"10.0.0.1", "10.0.0.2"},
			hostnameIP: "10.0.0.2",
			expected:   "10.0.0.2", // (90 + 40) > 90
		},
		{
			name:       "falls back to score heuristic if no hostname IP match",
			ips:        []string{"10.0.0.1", "192.168.1.100"},
			hostnameIP: "10.0.0.2", // Different IP
			expected:   "192.168.1.100",
		},
		{
			name:       "prefers 192.168.x.x over corosync 172.20.x.x (original behavior)",
			ips:        []string{"172.20.0.80", "192.168.1.100"},
			hostnameIP: "",
			expected:   "192.168.1.100",
		},
		{
			name:       "prefers 192.168.x.x even when listed second",
			ips:        []string{"10.0.0.1", "192.168.0.1"},
			hostnameIP: "",
			expected:   "192.168.0.1",
		},
		{
			name:       "prefers 10.x.x.x over 172.16-31.x.x",
			ips:        []string{"172.20.0.1", "10.1.10.5"},
			hostnameIP: "",
			expected:   "10.1.10.5",
		},
		{
			name:       "handles single IP",
			ips:        []string{"192.168.1.1"},
			hostnameIP: "",
			expected:   "192.168.1.1",
		},
		{
			name:       "skips loopback",
			ips:        []string{"127.0.0.1", "192.168.1.1"},
			hostnameIP: "",
			expected:   "192.168.1.1",
		},
		{
			name:       "skips IPv6 loopback",
			ips:        []string{"::1", "10.0.0.1"},
			hostnameIP: "",
			expected:   "10.0.0.1",
		},
		{
			name:       "skips link-local IPv6",
			ips:        []string{"fe80::1", "192.168.1.1"},
			hostnameIP: "",
			expected:   "192.168.1.1",
		},
		{
			name:       "skips link-local IPv4",
			ips:        []string{"169.254.1.1", "10.0.0.1"},
			hostnameIP: "",
			expected:   "10.0.0.1",
		},
		{
			name:       "returns corosync IP if only option",
			ips:        []string{"127.0.0.1", "172.20.0.80"},
			hostnameIP: "",
			expected:   "172.20.0.80",
		},
		{
			name:       "empty list returns empty",
			ips:        []string{},
			hostnameIP: "",
			expected:   "",
		},
		{
			name:       "only loopback returns empty",
			ips:        []string{"127.0.0.1", "::1"},
			hostnameIP: "",
			expected:   "",
		},
		{
			name:       "common 10.1.x.x LAN preferred over 172.x.x",
			ips:        []string{"172.16.0.1", "10.1.10.50"},
			hostnameIP: "",
			expected:   "10.1.10.50",
		},
		{
			name:       "prefers 10.0.x.x to 10.100.x.x (common ranges first)",
			ips:        []string{"10.100.0.1", "10.0.0.1"},
			hostnameIP: "",
			expected:   "10.0.0.1",
		},
		{
			name:       "prefers LAN over Tailscale if hostname matches both",
			ips:        []string{"192.168.1.5", "100.64.0.1"},
			hostnameIP: "192.168.1.5",
			expected:   "192.168.1.5",
		},
		{
			name:     "prefers Tailscale over cluster network",
			ips:      []string{"172.20.0.1", "100.64.0.1"},
			expected: "100.64.0.1",
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
		// 192.168.x.x - highest priority (100)
		{"192.168.1.1", 100},
		{"192.168.0.100", 100},
		{"192.168.255.255", 100},

		// 10.0-31.x.x - common corporate (90)
		{"10.0.0.1", 90},
		{"10.1.10.5", 90},
		{"10.31.255.255", 90},

		// 10.32+.x.x - less common (70)
		{"10.32.0.1", 70},
		{"10.100.0.1", 70},
		{"10.255.255.255", 70},

		// 172.16-31.x.x - private but often cluster (50)
		{"172.16.0.1", 50},
		{"172.20.0.80", 50}, // Corosync typical
		{"172.31.255.255", 50},

		// 100.64.x.x - Tailscale / CGNAT (85)
		{"100.64.0.1", 85},
		{"100.100.100.100", 85},
		{"100.127.255.255", 85},
		{"100.63.255.255", 30}, // Just outside range (below)
		{"100.128.0.1", 30},    // Just outside range (above)

		// 169.254.x.x - link-local (0)
		{"169.254.1.1", 0},

		// Other/public (30)
		{"8.8.8.8", 30},
		{"1.1.1.1", 30},
		{"203.0.113.1", 30},

		// Invalid
		{"not-an-ip", 0},
		{"", 0},
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

	tests := []struct {
		ptype    string
		expected string
	}{
		{"pve", stateFilePVE},
		{"pbs", stateFilePBS},
		{"unknown", stateFilePath}, // fallback to legacy
	}

	for _, tt := range tests {
		t.Run(tt.ptype, func(t *testing.T) {
			result := setup.stateFileForType(tt.ptype)
			if result != tt.expected {
				t.Errorf("stateFileForType(%q) = %q, want %q", tt.ptype, result, tt.expected)
			}
		})
	}
}
func TestGetHostURL(t *testing.T) {
	tests := []struct {
		name     string
		ptype    string
		reportIP string
		expected string
	}{
		{
			name:     "uses reportIP override for PVE",
			ptype:    "pve",
			reportIP: "10.0.0.50",
			expected: "https://10.0.0.50:8006",
		},
		{
			name:     "uses reportIP override for PBS",
			ptype:    "pbs",
			reportIP: "192.168.1.100",
			expected: "https://192.168.1.100:8007",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := &ProxmoxSetup{
				reportIP: tt.reportIP,
				logger:   zerolog.Nop(),
			}
			result := setup.getHostURL(tt.ptype)
			if result != tt.expected {
				t.Errorf("getHostURL(%q) = %q, want %q", tt.ptype, result, tt.expected)
			}
		})
	}
}

func TestParseTokenValue(t *testing.T) {
	setup := &ProxmoxSetup{
		logger: zerolog.Nop(),
	}

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name: "parses standard table output",
			output: `
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pulse-monitor@pam!pulse-monitor      │
├──────────────┼──────────────────────────────────────┤
│ info         │ {"privsep":1}                        │
├──────────────┼──────────────────────────────────────┤
│ value        │ 7c5709fb-6aee-4c32-8b9f-5c2656912345 │
└──────────────┴──────────────────────────────────────┘
`,
			expected: "7c5709fb-6aee-4c32-8b9f-5c2656912345",
		},
		{
			name: "parses output with extra whitespace",
			output: `
│ value        │   7c5709fb-6aee-4c32-8b9f-5c2656912345   │
`,
			expected: "7c5709fb-6aee-4c32-8b9f-5c2656912345",
		},
		{
			name:     "returns empty on missing value",
			output:   `│ other │ something │`,
			expected: "",
		},
		{
			name:     "empty input",
			output:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setup.parseTokenValue(tt.output)
			if result != tt.expected {
				t.Errorf("parseTokenValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParsePBSTokenValue(t *testing.T) {
	setup := &ProxmoxSetup{
		logger: zerolog.Nop(),
	}

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "parses standard JSON output",
			output:   `{"tokenid":"pulse-monitor@pbs!pulse-monitor","value":"pbs-api-token-value-12345"}`,
			expected: "pbs-api-token-value-12345",
		},
		{
			name:     "parses JSON with extra fields",
			output:   `{"other":"stuff","value":"my-secret-token","more":"stuff"}`,
			expected: "my-secret-token",
		},
		{
			name:     "returns empty on invalid JSON",
			output:   `not-json`,
			expected: "",
		},
		{
			name:     "returns empty when value missing",
			output:   `{"tokenid":"foo"}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setup.parsePBSTokenValue(tt.output)
			if result != tt.expected {
				t.Errorf("parsePBSTokenValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSetupPVEToken(t *testing.T) {
	// Backup and restore
	origRunCommandOutput := runCommandOutput
	origRunCommand := runCommand
	defer func() {
		runCommandOutput = origRunCommandOutput
		runCommand = origRunCommand
	}()

	mockOutput := `
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pulse-monitor@pam!pulse-monitor      │
├──────────────┼──────────────────────────────────────┤
│ value        │ 7c5709fb-6aee-4c32-8b9f-5c2656912345 │
└──────────────┴──────────────────────────────────────┘
`
	var capturedCmd string
	var capturedArgs []string

	// Mock runCommand to do nothing (success)
	runCommand = func(ctx context.Context, name string, args ...string) error {
		return nil
	}

	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		capturedCmd = name
		capturedArgs = args
		return mockOutput, nil
	}

	setup := NewProxmoxSetup(zerolog.Nop(), nil, "", "", "pve", "", "", false)
	id, value, err := setup.setupPVEToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("setupPVEToken failed: %v", err)
	}

	if id != "pulse-monitor@pam!test-token" {
		t.Errorf("expected token ID pulse-monitor@pam!test-token, got %s", id)
	}
	if value != "7c5709fb-6aee-4c32-8b9f-5c2656912345" {
		t.Errorf("expected token value 7c5709fb-6aee-4c32-8b9f-5c2656912345, got %s", value)
	}

	if capturedCmd != "pveum" {
		t.Errorf("expected command pveum, got %s", capturedCmd)
	}
	// Verify critical args
	foundAdd := false
	foundTokenName := false
	for _, arg := range capturedArgs {
		if arg == "add" {
			foundAdd = true
		}
		if arg == "test-token" {
			foundTokenName = true
		}
	}
	if !foundAdd || !foundTokenName {
		t.Errorf("missing critical args in %v", capturedArgs)
	}
}

func TestSetupPBSToken(t *testing.T) {
	// Backup and restore
	origRunCommandOutput := runCommandOutput
	origRunCommand := runCommand
	defer func() {
		runCommandOutput = origRunCommandOutput
		runCommand = origRunCommand
	}()

	mockOutput := `{"tokenid":"pulse-monitor@pbs!test-token","value":"pbs-api-token-value-12345"}`

	// Mock runCommand to do nothing
	runCommand = func(ctx context.Context, name string, args ...string) error {
		return nil
	}

	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		return mockOutput, nil
	}

	setup := NewProxmoxSetup(zerolog.Nop(), nil, "", "", "pbs", "", "", false)
	id, value, err := setup.setupPBSToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("setupPBSToken failed: %v", err)
	}

	if id != "pulse-monitor@pbs!test-token" {
		t.Errorf("expected token ID pulse-monitor@pbs!test-token, got %s", id)
	}
	if value != "pbs-api-token-value-12345" {
		t.Errorf("expected token value pbs-api-token-value-12345, got %s", value)
	}
}

func TestDetectProxmoxTypes(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	tests := []struct {
		name      string
		mockPaths map[string]bool // map[exe]exists
		expected  []string
	}{
		{
			name: "detects pve only",
			mockPaths: map[string]bool{
				"pvesh":                  true,
				"proxmox-backup-manager": false,
			},
			expected: []string{"pve"},
		},
		{
			name: "detects pbs only",
			mockPaths: map[string]bool{
				"pvesh":                  false,
				"proxmox-backup-manager": true,
			},
			expected: []string{"pbs"},
		},
		{
			name: "detects both",
			mockPaths: map[string]bool{
				"pvesh":                  true,
				"proxmox-backup-manager": true,
			},
			expected: []string{"pve", "pbs"},
		},
		{
			name: "detects none",
			mockPaths: map[string]bool{
				"pvesh":                  false,
				"proxmox-backup-manager": false,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookPath = func(file string) (string, error) {
				if exists := tt.mockPaths[file]; exists {
					return "/usr/bin/" + file, nil
				}
				return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
			}

			setup := &ProxmoxSetup{}
			result := setup.detectProxmoxTypes()

			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			} else {
				for i, v := range result {
					if v != tt.expected[i] {
						t.Errorf("expected %v, got %v", tt.expected, result)
						break
					}
				}
			}
		})
	}
}

func TestRunForType(t *testing.T) {
	// Setup temporary state dir
	tmpDir := t.TempDir()

	// Backup and restore state file paths
	origStateFileDir := stateFileDir
	origStateFilePath := stateFilePath
	origStateFilePVE := stateFilePVE
	origStateFilePBS := stateFilePBS

	stateFileDir = tmpDir
	stateFilePath = filepath.Join(tmpDir, "proxmox-registered")
	stateFilePVE = filepath.Join(tmpDir, "proxmox-pve-registered")
	stateFilePBS = filepath.Join(tmpDir, "proxmox-pbs-registered")

	defer func() {
		stateFileDir = origStateFileDir
		stateFilePath = origStateFilePath
		stateFilePVE = origStateFilePVE
		stateFilePBS = origStateFilePBS
	}()

	// Backup and restore runCommand functions
	origRunCommand := runCommand
	origRunCommandOutput := runCommandOutput
	defer func() {
		runCommand = origRunCommand
		runCommandOutput = origRunCommandOutput
	}()

	// Mock Proxmox commands
	mockTokenOutput := `
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pulse-monitor@pam!test-token         │
├──────────────┼──────────────────────────────────────┤
│ value        │ 7c5709fb-6aee-4c32-8b9f-5c2656912345 │
└──────────────┴──────────────────────────────────────┘
`
	runCommand = func(ctx context.Context, name string, args ...string) error {
		return nil
	}
	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		return mockTokenOutput, nil
	}

	// Mock HTTP Client to capture registration
	var capturedReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	setup := NewProxmoxSetup(zerolog.Nop(), server.Client(), server.URL, "api-token", "pve", "test-host", "", false)

	// Test case 1: Not registered yet
	result, err := setup.runForType(context.Background(), "pve")
	if err != nil {
		t.Fatalf("runForType failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.Registered {
		t.Error("expected Registered=true")
	}
	if !strings.HasPrefix(result.TokenID, "pulse-monitor@pam!pulse-") {
		t.Errorf("expected token ID starting with pulse-monitor@pam!pulse-, got %s", result.TokenID)
	}

	// Verify HTTP registration call
	if capturedReq == nil {
		t.Error("expected HTTP registration request")
	} else {
		if capturedReq.URL.Path != "/api/auto-register" {
			t.Errorf("expected path /api/auto-register, got %s", capturedReq.URL.Path)
		}
	}

	// Verify state file created
	if _, err := os.Stat(stateFilePVE); os.IsNotExist(err) {
		t.Error("expected state file to be created")
	}

	// Test case 2: Already registered (should skip)
	capturedReq = nil // Reset capture
	result, err = setup.runForType(context.Background(), "pve")
	if err != nil {
		t.Fatalf("runForType (2nd call) failed: %v", err)
	}

	if result != nil {
		t.Error("expected nil result (skipped), got something")
	}
	if capturedReq != nil {
		t.Error("did not expect HTTP call on 2nd run")
	}
}

func TestRunAll(t *testing.T) {
	// Setup temporary state dir
	tmpDir := t.TempDir()

	// Backup and restore state variables
	origStateFileDir := stateFileDir
	origStateFilePath := stateFilePath
	origStateFilePVE := stateFilePVE
	origStateFilePBS := stateFilePBS

	stateFileDir = tmpDir
	stateFilePath = filepath.Join(tmpDir, "proxmox-registered")
	stateFilePVE = filepath.Join(tmpDir, "proxmox-pve-registered")
	stateFilePBS = filepath.Join(tmpDir, "proxmox-pbs-registered")

	defer func() {
		stateFileDir = origStateFileDir
		stateFilePath = origStateFilePath
		stateFilePVE = origStateFilePVE
		stateFilePBS = origStateFilePBS
	}()

	// Backup and restore lookPath
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	// Backup runCommand
	origRunCommand := runCommand
	origRunCommandOutput := runCommandOutput
	defer func() {
		runCommand = origRunCommand
		runCommandOutput = origRunCommandOutput
	}()

	// Mock LookPath to find BOTH PVE and PBS
	lookPath = func(file string) (string, error) {
		if file == "pvesh" || file == "proxmox-backup-manager" {
			return "/usr/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	// Mock Command execution
	runCommand = func(ctx context.Context, name string, args ...string) error {
		return nil
	}
	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		// Return valid tokens for both types
		if name == "pveum" {
			return `
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pulse-monitor@pam!pulse-pve-token    │
├──────────────┼──────────────────────────────────────┤
│ value        │ 7c5709fb-6aee-4c32-8b9f-5c2656912345 │
└──────────────┴──────────────────────────────────────┘
`, nil
		}
		if name == "proxmox-backup-manager" {
			return `{"tokenid":"pulse-monitor@pbs!pulse-pbs-token","value":"pbs-value"}`, nil
		}
		return "", nil
	}

	// Mock HTTP Server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	setup := NewProxmoxSetup(zerolog.Nop(), server.Client(), server.URL, "api-token", "", "test-host", "", false)

	// RunAll
	results, err := setup.RunAll(context.Background())
	if err != nil {
		t.Fatalf("RunAll failed: %v", err)
	}

	// Should have 2 results
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Should have made 2 HTTP calls
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}

	// Check state files
	if _, err := os.Stat(stateFilePVE); os.IsNotExist(err) {
		t.Error("expected PVE state file")
	}
	if _, err := os.Stat(stateFilePBS); os.IsNotExist(err) {
		t.Error("expected PBS state file")
	}
}

func TestRun_Legacy(t *testing.T) {
	// Setup temporary state dir
	tmpDir := t.TempDir()

	// Backup
	origStateFilePath := stateFilePath
	stateFilePath = filepath.Join(tmpDir, "proxmox-registered")
	defer func() { stateFilePath = origStateFilePath }()

	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	origRunCommand := runCommand
	origRunCommandOutput := runCommandOutput
	defer func() {
		runCommand = origRunCommand
		runCommandOutput = origRunCommandOutput
	}()

	// Mock LookPath - find PVE
	lookPath = func(file string) (string, error) {
		if file == "pvesh" {
			return "/usr/bin/pvesh", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	// Mock Token
	runCommand = func(ctx context.Context, name string, args ...string) error { return nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "pveum" {
			return `
┌──────────────┬──────────────────────────────────────┐
│ key          │ value                                │
╞══════════════╪══════════════════════════════════════╡
│ full-tokenid │ pulse-monitor@pam!pulse-pve-token    │
├──────────────┼──────────────────────────────────────┤
│ value        │ 7c5709fb-6aee-4c32-8b9f-5c2656912345 │
└──────────────┴──────────────────────────────────────┘
`, nil
		}
		return "", nil
	}

	// Mock HTTP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Test Run()
	setup := NewProxmoxSetup(zerolog.Nop(), server.Client(), server.URL, "api-token", "", "test-host", "", false) // empty ptype -> auto-detect
	result, err := setup.Run(context.Background())
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result == nil || !result.Registered {
		t.Error("expected successful registration")
	}

	// Verify legacy state file
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		t.Error("expected legacy state file")
	}

	// Run again - checks isAlreadyRegistered (idempotency)
	result2, err := setup.Run(context.Background())
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}
	if result2 != nil {
		t.Error("expected nil result (skipped) on 2nd run")
	}
}

func TestIsTypeRegistered_Legacy(t *testing.T) {
	// Setup temporary state dir
	tmpDir := t.TempDir()

	// Backup
	origStateFilePath := stateFilePath
	origStateFilePVE := stateFilePVE // Ensure new files don't exist
	stateFilePath = filepath.Join(tmpDir, "proxmox-registered")
	stateFilePVE = filepath.Join(tmpDir, "proxmox-pve-registered")

	defer func() {
		stateFilePath = origStateFilePath
		stateFilePVE = origStateFilePVE
	}()

	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	// Create legacy state file
	if err := os.WriteFile(stateFilePath, []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}

	setup := &ProxmoxSetup{}

	// Scenario 1: PVE installed. Requesting PVE check. Should be true.
	lookPath = func(file string) (string, error) {
		if file == "pvesh" {
			return "/bin/pvesh", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}
	if !setup.isTypeRegistered("pve") {
		t.Error("Legacy: PVE installed + legacy file should => registered")
	}

	// Scenario 2: PVE installed. Requesting PBS check. Should be false (PVE assumed primary).
	if setup.isTypeRegistered("pbs") {
		t.Error("Legacy: PVE installed + legacy file should => PBS NOT registered")
	}

	// Scenario 3: Only PBS installed. Requesting PBS check. Should be true.
	lookPath = func(file string) (string, error) {
		if file == "proxmox-backup-manager" {
			return "/bin/proxmox-backup-manager", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}
	if !setup.isTypeRegistered("pbs") {
		t.Error("Legacy: Only PBS installed + legacy file should => PBS registered")
	}
}
