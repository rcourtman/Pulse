package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeDuplicateAllowedNodesBlocks_RemovesExtraBlocks(t *testing.T) {
	raw := `
allowed_nodes:
  - delly
  - minipc

# Cluster nodes (auto-discovered during installation)
# These nodes are allowed to request temperature data when cluster IPC validation is unavailable
allowed_nodes:
  - delly
  - minipc
  - extra
`

	sanitized, out := sanitizeDuplicateAllowedNodesBlocks("", []byte(raw))
	if !sanitized {
		t.Fatalf("expected sanitization to occur")
	}

	result := string(out)
	if strings.Count(result, "allowed_nodes:") != 1 {
		t.Fatalf("expected only one allowed_nodes block, got %q", result)
	}
	if strings.Contains(result, "extra") {
		t.Fatalf("duplicate entries should be removed, got %q", result)
	}
	if strings.Contains(result, "Cluster nodes (auto-discovered during installation)") {
		t.Fatalf("duplicate comment block should be removed")
	}
}

func TestSanitizeDuplicateAllowedNodesBlocks_NoChangeWhenUnique(t *testing.T) {
	raw := `
metrics_address: 127.0.0.1:9127
allowed_nodes:
  - delly
`
	sanitized, out := sanitizeDuplicateAllowedNodesBlocks("", []byte(raw))
	if sanitized {
		t.Fatalf("unexpected sanitization for unique config")
	}
	if string(out) != raw {
		t.Fatalf("expected config to remain unchanged")
	}
}

func TestSanitizeDuplicateAllowedNodesBlocks_WithCommentBlocks(t *testing.T) {
	raw := `
allowed_source_subnets:
  - 192.168.1.0/24

# Cluster nodes (auto-discovered during installation)
# These nodes are allowed to request temperature data when cluster IPC validation is unavailable
allowed_nodes:
  - delly
  - minipc

# Cluster nodes (auto-discovered during installation)
# These nodes are allowed to request temperature data when cluster IPC validation is unavailable
allowed_nodes:
  - delly
  - minipc
`

	sanitized, out := sanitizeDuplicateAllowedNodesBlocks("", []byte(raw))
	if !sanitized {
		t.Fatalf("expected sanitizer to run for duplicate comment blocks")
	}

	result := string(out)
	if strings.Count(result, "allowed_nodes:") != 1 {
		t.Fatalf("expected a single allowed_nodes block, got %q", result)
	}
	if strings.Count(result, "# Cluster nodes") != 1 {
		t.Fatalf("expected duplicate comments to collapse, got %q", result)
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		// Truthy values
		{"true lowercase", "true", true, false},
		{"TRUE uppercase", "TRUE", true, false},
		{"True mixed", "True", true, false},
		{"1", "1", true, false},
		{"yes", "yes", true, false},
		{"YES", "YES", true, false},
		{"on", "on", true, false},
		{"ON", "ON", true, false},

		// Falsy values
		{"false lowercase", "false", false, false},
		{"FALSE uppercase", "FALSE", false, false},
		{"False mixed", "False", false, false},
		{"0", "0", false, false},
		{"no", "no", false, false},
		{"NO", "NO", false, false},
		{"off", "off", false, false},
		{"OFF", "OFF", false, false},

		// Whitespace handling
		{"true with leading space", "  true", true, false},
		{"false with trailing space", "false  ", false, false},
		{"yes with surrounding spaces", "  yes  ", true, false},

		// Invalid values
		{"invalid string", "maybe", false, true},
		{"empty string", "", false, true},
		{"numeric 2", "2", false, true},
		{"random word", "enabled", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseBool(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseBool(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseBool(%q) unexpected error: %v", tc.input, err)
				return
			}
			if result != tc.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseUint32List(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []uint32
		wantErr  bool
	}{
		{"single value", "1000", []uint32{1000}, false},
		{"multiple values", "1000,2000,3000", []uint32{1000, 2000, 3000}, false},
		{"with spaces", "1000, 2000, 3000", []uint32{1000, 2000, 3000}, false},
		{"empty string", "", nil, false},
		{"only spaces", "   ", nil, false},
		{"zero value", "0", []uint32{0}, false},
		{"max uint32", "4294967295", []uint32{4294967295}, false},
		{"with empty parts", "1000,,2000", []uint32{1000, 2000}, false},
		{"negative value", "-1", nil, true},
		{"overflow uint32", "4294967296", nil, true},
		{"non-numeric", "abc", nil, true},
		{"mixed valid invalid", "1000,abc", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseUint32List(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseUint32List(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseUint32List(%q) unexpected error: %v", tc.input, err)
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("parseUint32List(%q) length = %d, want %d", tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("parseUint32List(%q)[%d] = %d, want %d", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple comma separated", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", "a, b, c", []string{"a", "b", "c"}},
		{"with leading/trailing spaces", "  a  ,  b  ,  c  ", []string{"a", "b", "c"}},
		{"empty string", "", nil},
		{"single value", "single", []string{"single"}},
		{"empty parts filtered", "a,,b", []string{"a", "b"}},
		{"only spaces between commas", "a,   ,b", []string{"a", "b"}},
		{"all empty parts", ",,", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := splitAndTrim(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("splitAndTrim(%q) length = %d, want %d", tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestNormalizeNodes(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"simple list", []string{"node1", "node2"}, []string{"node1", "node2"}},
		{"with whitespace", []string{"  node1  ", "node2"}, []string{"node1", "node2"}},
		{"deduplicates case-insensitive", []string{"Node1", "NODE1", "node1"}, []string{"Node1"}},
		{"preserves original case", []string{"MyNode", "mynode"}, []string{"MyNode"}},
		{"filters empty strings", []string{"node1", "", "node2"}, []string{"node1", "node2"}},
		{"filters whitespace-only", []string{"node1", "   ", "node2"}, []string{"node1", "node2"}},
		{"empty input", []string{}, nil},
		{"nil input", nil, nil},
		{"mixed duplicates", []string{"a", "B", "a", "b", "C"}, []string{"a", "B", "C"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeNodes(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("normalizeNodes(%v) length = %d, want %d", tc.input, len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("normalizeNodes(%v)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestParseAllowedSubnets(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		wantErr  bool
	}{
		// Valid CIDRs
		{"single IPv4 CIDR", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}, false},
		{"multiple IPv4 CIDRs", []string{"192.168.1.0/24", "10.0.0.0/8"}, []string{"192.168.1.0/24", "10.0.0.0/8"}, false},
		{"IPv6 CIDR", []string{"2001:db8::/32"}, []string{"2001:db8::/32"}, false},

		// Single IPs converted to CIDRs
		{"single IPv4 converted", []string{"192.168.1.100"}, []string{"192.168.1.100/32"}, false},
		{"single IPv6 converted", []string{"2001:db8::1"}, []string{"2001:db8::1/128"}, false},

		// Whitespace handling
		{"with whitespace", []string{"  192.168.1.0/24  "}, []string{"192.168.1.0/24"}, false},
		{"empty entries filtered", []string{"192.168.1.0/24", "", "10.0.0.0/8"}, []string{"192.168.1.0/24", "10.0.0.0/8"}, false},

		// Deduplication
		{"deduplicates exact", []string{"192.168.1.0/24", "192.168.1.0/24"}, []string{"192.168.1.0/24"}, false},
		{"deduplicates converted", []string{"192.168.1.100", "192.168.1.100"}, []string{"192.168.1.100/32"}, false},

		// Invalid inputs
		{"invalid format", []string{"not-a-subnet"}, nil, true},
		{"invalid CIDR", []string{"192.168.1.0/33"}, nil, true},
		{"partial valid", []string{"192.168.1.0/24", "invalid"}, nil, true},

		// Empty input
		{"empty input", []string{}, nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAllowedSubnets(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseAllowedSubnets(%v) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseAllowedSubnets(%v) unexpected error: %v", tc.input, err)
				return
			}
			if len(result) != len(tc.expected) {
				t.Errorf("parseAllowedSubnets(%v) length = %d, want %d; result = %v", tc.input, len(result), len(tc.expected), result)
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("parseAllowedSubnets(%v)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestLoadConfig_Basic(t *testing.T) {
	// Non-existent file should just use defaults
	cfg, err := loadConfig("/non/existent/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("PULSE_SENSOR_PROXY_LOG_LEVEL", "debug")
	t.Setenv("PULSE_SENSOR_PROXY_READ_TIMEOUT", "10s")
	t.Setenv("PULSE_SENSOR_PROXY_WRITE_TIMEOUT", "20s")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOWED_SUBNETS", "10.0.0.0/24,192.168.1.1")
	t.Setenv("PULSE_SENSOR_PROXY_MAX_SSH_OUTPUT_BYTES", "2097152")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOW_IDMAPPED_ROOT", "false")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOWED_IDMAP_USERS", "root,admin")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOWED_PEER_UIDS", "1000,1001")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOWED_PEER_GIDS", "1000,1001")
	t.Setenv("PULSE_SENSOR_PROXY_ALLOWED_NODES", "node1,node2")
	t.Setenv("PULSE_SENSOR_PROXY_STRICT_NODE_VALIDATION", "true")
	t.Setenv("PULSE_SENSOR_PROXY_REQUIRE_PROXMOX_HOSTKEYS", "true")
	t.Setenv("PULSE_SENSOR_PROXY_METRICS_ADDR", "127.0.0.1:9999")

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug log level, got %s", cfg.LogLevel)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("expected 10s read timeout, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 20*time.Second {
		t.Errorf("expected 20s write timeout, got %v", cfg.WriteTimeout)
	}
	if cfg.MaxSSHOutputBytes != 2097152 {
		t.Errorf("expected 2MB max SSH output, got %d", cfg.MaxSSHOutputBytes)
	}
	if cfg.AllowIDMappedRoot {
		t.Error("expected allow_idmapped_root false")
	}
	if len(cfg.AllowedPeerUIDs) != 2 {
		t.Errorf("expected 2 UIDs, got %d", len(cfg.AllowedPeerUIDs))
	}
	if cfg.MetricsAddress != "127.0.0.1:9999" {
		t.Errorf("expected metrics addr 127.0.0.1:9999, got %s", cfg.MetricsAddress)
	}
}

func TestLoadAllowedNodesFile(t *testing.T) {
	tmpDir := t.TempDir()

	// YAML list format
	yamlList := filepath.Join(tmpDir, "list.yaml")
	os.WriteFile(yamlList, []byte("- node1\n- node2\n"), 0644)
	nodes, err := loadAllowedNodesFile(yamlList)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	// YAML map format
	yamlMap := filepath.Join(tmpDir, "map.yaml")
	os.WriteFile(yamlMap, []byte("allowed_nodes:\n  - node3\n"), 0644)
	nodes, err = loadAllowedNodesFile(yamlMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0] != "node3" {
		t.Errorf("unexpected nodes: %v", nodes)
	}

	// Plain text format
	plainText := filepath.Join(tmpDir, "plain.txt")
	os.WriteFile(plainText, []byte("node4\n# comment\n- node5\n"), 0644)
	nodes, err = loadAllowedNodesFile(plainText)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[1] != "node5" {
		t.Errorf("unexpected nodes: %v", nodes)
	}
}

func TestLoadConfig_HTTP_Validation(t *testing.T) {
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_ENABLED", "true")
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_ADDR", ":8443")
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_TLS_CERT", "/tmp/cert")
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_TLS_KEY", "/tmp/key")
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_AUTH_TOKEN", "token")

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPListenAddr != ":8443" {
		t.Errorf("expected addr :8443, got %s", cfg.HTTPListenAddr)
	}

	// Test missing token
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_AUTH_TOKEN", "")
	_, err = loadConfig("")
	if err == nil {
		t.Error("expected error for missing HTTP token")
	}

	// Test missing cert
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_AUTH_TOKEN", "token")
	t.Setenv("PULSE_SENSOR_PROXY_HTTP_TLS_CERT", "")
	_, err = loadConfig("")
	if err == nil {
		t.Error("expected error for missing TLS cert")
	}
}

func TestDetectHostCIDRs(t *testing.T) {
	cidrs := detectHostCIDRs()
	// Should at least have some IPs if running in a container with network
	if len(cidrs) == 0 {
		t.Log("detectHostCIDRs returned no CIDRs (might be expected if no non-loopback IPs)")
	}
	for _, cidr := range cidrs {
		if !strings.Contains(cidr, "/") {
			t.Errorf("invalid CIDR: %s", cidr)
		}
	}
}

func TestLoadConfig_TimeoutValidation(t *testing.T) {
	t.Setenv("PULSE_SENSOR_PROXY_READ_TIMEOUT", "0s")
	t.Setenv("PULSE_SENSOR_PROXY_WRITE_TIMEOUT", "0s")
	t.Setenv("PULSE_SENSOR_PROXY_MAX_SSH_OUTPUT_BYTES", "0")

	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReadTimeout <= 0 {
		t.Error("expected positive read timeout default")
	}
	if cfg.WriteTimeout <= 0 {
		t.Error("expected positive write timeout default")
	}
}
