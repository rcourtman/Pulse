package api

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIsFallbackMemorySource(t *testing.T) {
	tests := []struct {
		source   string
		expected bool
	}{
		// Fallback sources
		{"", true},
		{"unknown", true},
		{"Unknown", true},
		{"UNKNOWN", true},
		{"nodes-endpoint", true},
		{"Nodes-Endpoint", true},
		{"node-status-used", true},
		{"previous-snapshot", true},

		// Non-fallback sources
		{"cgroup", false},
		{"qemu-agent", false},
		{"proxmox-api", false},
		{"rrddata", false},
		{"some-other-source", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := isFallbackMemorySource(tt.source)
			if result != tt.expected {
				t.Errorf("isFallbackMemorySource(%q) = %v, want %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestCopyStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"one"},
			expected: []string{"one"},
		},
		{
			name:     "multiple elements",
			input:    []string{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := copyStringSlice(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("copyStringSlice() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("copyStringSlice()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}

			// Verify it's a copy, not the same slice
			if len(tt.input) > 0 && len(result) > 0 {
				result[0] = "modified"
				if tt.input[0] == "modified" {
					t.Error("copyStringSlice() returned reference to original, not a copy")
				}
			}
		})
	}
}

func TestNormalizeHostForComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Empty/whitespace
		{"", ""},
		{"   ", ""},

		// Basic hostnames
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"  example.com  ", "example.com"},

		// With protocol (lowercase only - uppercase protocols not stripped)
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		// Note: uppercase protocols are not stripped, result is lowercase of input
		{"HTTPS://Example.COM", "https"},

		// With port
		{"example.com:8006", "example.com"},
		{"https://example.com:8006", "example.com"},
		{"192.168.1.1:8006", "192.168.1.1"},

		// With path
		{"example.com/api/v1", "example.com"},
		{"https://example.com/api/v1", "example.com"},
		{"https://example.com:8006/api/v1", "example.com"},

		// IP addresses
		{"192.168.1.1", "192.168.1.1"},
		{"https://192.168.1.1:8006", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeHostForComparison(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeHostForComparison(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeVersionLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Empty/whitespace
		{"", ""},
		{"   ", ""},

		// Already has v prefix
		{"v1.0.0", "v1.0.0"},
		{"v4.35.0", "v4.35.0"},
		{"  v1.0.0  ", "v1.0.0"},

		// Needs v prefix
		{"1.0.0", "v1.0.0"},
		{"4.35.0", "v4.35.0"},
		{"  4.35.0  ", "v4.35.0"},

		// Non-numeric prefix (no v added)
		{"dev", "dev"},
		{"alpha-1.0", "alpha-1.0"},
		{"beta", "beta"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeVersionLabel(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVersionLabel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		str      string
		expected bool
	}{
		{"empty slice", []string{}, "test", false},
		{"nil slice", nil, "test", false},
		{"found at start", []string{"test", "foo", "bar"}, "test", true},
		{"found at end", []string{"foo", "bar", "test"}, "test", true},
		{"found in middle", []string{"foo", "test", "bar"}, "test", true},
		{"not found", []string{"foo", "bar", "baz"}, "test", false},
		{"case sensitive", []string{"Test", "TEST"}, "test", false},
		{"empty string search", []string{"foo", "", "bar"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.str)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.str, result, tt.expected)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	tests := []struct {
		name      string
		slice     []string
		candidate string
		expected  bool
	}{
		{"empty slice", []string{}, "test", false},
		{"nil slice", nil, "test", false},
		{"empty candidate", []string{"foo", "bar"}, "", false},
		{"whitespace candidate", []string{"foo", "bar"}, "   ", false},
		{"exact match", []string{"test", "foo"}, "test", true},
		{"case insensitive match", []string{"Test", "foo"}, "test", true},
		{"uppercase match", []string{"test", "foo"}, "TEST", true},
		{"mixed case match", []string{"TeSt", "foo"}, "tEsT", true},
		{"with whitespace in slice", []string{"  test  ", "foo"}, "test", true},
		{"with whitespace in candidate", []string{"test", "foo"}, "  test  ", true},
		{"not found", []string{"foo", "bar"}, "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsFold(tt.slice, tt.candidate)
			if result != tt.expected {
				t.Errorf("containsFold(%v, %q) = %v, want %v", tt.slice, tt.candidate, result, tt.expected)
			}
		})
	}
}

func TestInterfaceToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "string slice",
			input:    []string{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "empty string slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "interface slice with strings",
			input:    []interface{}{"one", "two", "three"},
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "empty interface slice",
			input:    []interface{}{},
			expected: []string{},
		},
		{
			name:     "interface slice with mixed types",
			input:    []interface{}{"one", 2, "three", true},
			expected: []string{"one", "three"},
		},
		{
			name:     "interface slice with only non-strings",
			input:    []interface{}{1, 2, 3},
			expected: []string{},
		},
		{
			name:     "unsupported type",
			input:    "not a slice",
			expected: nil,
		},
		{
			name:     "int slice (unsupported)",
			input:    []int{1, 2, 3},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interfaceToStringSlice(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("interfaceToStringSlice() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("interfaceToStringSlice() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("interfaceToStringSlice()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestPreferredDockerHostName(t *testing.T) {
	tests := []struct {
		name     string
		host     models.DockerHost
		expected string
	}{
		{
			name: "display name preferred",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "My Docker Host",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "My Docker Host",
		},
		{
			name: "hostname when no display name",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "docker-host.local",
		},
		{
			name: "agent ID when no display name or hostname",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "",
				AgentID:     "agent-456",
			},
			expected: "agent-456",
		},
		{
			name: "ID as fallback",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "",
				Hostname:    "",
				AgentID:     "",
			},
			expected: "id-123",
		},
		{
			name: "whitespace-only display name ignored",
			host: models.DockerHost{
				ID:          "id-123",
				DisplayName: "   ",
				Hostname:    "docker-host.local",
				AgentID:     "agent-456",
			},
			expected: "docker-host.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preferredDockerHostName(tt.host)
			if result != tt.expected {
				t.Errorf("preferredDockerHostName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTimeMaybe(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "specific time",
			input:    time.Date(2025, 11, 30, 12, 30, 45, 0, time.UTC),
			expected: "2025-11-30T12:30:45Z",
		},
		{
			name:     "non-UTC time converted to UTC",
			input:    time.Date(2025, 11, 30, 12, 30, 45, 0, time.FixedZone("EST", -5*60*60)),
			expected: "2025-11-30T17:30:45Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeMaybe(tt.input)
			if result != tt.expected {
				t.Errorf("formatTimeMaybe() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMatchInstanceNameByHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.Config
		host     string
		expected string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			host:     "example.com",
			expected: "",
		},
		{
			name: "empty host",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local"},
				},
			},
			host:     "",
			expected: "",
		},
		{
			name: "whitespace-only host",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local"},
				},
			},
			host:     "   ",
			expected: "",
		},
		{
			name: "exact match",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local"},
					{Name: "pve2", Host: "pve2.local"},
				},
			},
			host:     "pve1.local",
			expected: "pve1",
		},
		{
			name: "case insensitive match",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "Production PVE", Host: "PVE.EXAMPLE.COM"},
				},
			},
			host:     "pve.example.com",
			expected: "Production PVE",
		},
		{
			name: "match with port in config",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local:8006"},
				},
			},
			host:     "pve1.local",
			expected: "pve1",
		},
		{
			name: "match with protocol in config",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "https://pve1.local:8006"},
				},
			},
			host:     "pve1.local",
			expected: "pve1",
		},
		{
			name: "match with protocol in search host",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local"},
				},
			},
			host:     "https://pve1.local:8006",
			expected: "pve1",
		},
		{
			name: "no match",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "pve1.local"},
					{Name: "pve2", Host: "pve2.local"},
				},
			},
			host:     "pve3.local",
			expected: "",
		},
		{
			name: "empty instances",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{},
			},
			host:     "pve1.local",
			expected: "",
		},
		{
			name: "instance with empty host skipped",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "empty", Host: ""},
					{Name: "pve1", Host: "pve1.local"},
				},
			},
			host:     "pve1.local",
			expected: "pve1",
		},
		{
			name: "IP address match",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "pve1", Host: "192.168.1.100"},
				},
			},
			host:     "192.168.1.100",
			expected: "pve1",
		},
		{
			name: "name has leading and trailing whitespace",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "  pve1  ", Host: "pve1.local"},
				},
			},
			host:     "pve1.local",
			expected: "pve1",
		},
		{
			name: "returns first match when duplicates exist",
			cfg: &config.Config{
				PVEInstances: []config.PVEInstance{
					{Name: "first", Host: "pve.local"},
					{Name: "second", Host: "pve.local"},
				},
			},
			host:     "pve.local",
			expected: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := matchInstanceNameByHost(tt.cfg, tt.host)
			if result != tt.expected {
				t.Errorf("matchInstanceNameByHost() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHasLegacyThresholds(t *testing.T) {
	ptrFloat := func(v float64) *float64 { return &v }

	tests := []struct {
		name     string
		config   alerts.ThresholdConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   alerts.ThresholdConfig{},
			expected: false,
		},
		{
			name: "modern thresholds only",
			config: alerts.ThresholdConfig{
				CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 70},
				Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 75},
			},
			expected: false,
		},
		{
			name: "CPU legacy set",
			config: alerts.ThresholdConfig{
				CPULegacy: ptrFloat(80.0),
			},
			expected: true,
		},
		{
			name: "Memory legacy set",
			config: alerts.ThresholdConfig{
				MemoryLegacy: ptrFloat(85.0),
			},
			expected: true,
		},
		{
			name: "Disk legacy set",
			config: alerts.ThresholdConfig{
				DiskLegacy: ptrFloat(90.0),
			},
			expected: true,
		},
		{
			name: "DiskRead legacy set",
			config: alerts.ThresholdConfig{
				DiskReadLegacy: ptrFloat(50.0),
			},
			expected: true,
		},
		{
			name: "DiskWrite legacy set",
			config: alerts.ThresholdConfig{
				DiskWriteLegacy: ptrFloat(60.0),
			},
			expected: true,
		},
		{
			name: "NetworkIn legacy set",
			config: alerts.ThresholdConfig{
				NetworkInLegacy: ptrFloat(70.0),
			},
			expected: true,
		},
		{
			name: "NetworkOut legacy set",
			config: alerts.ThresholdConfig{
				NetworkOutLegacy: ptrFloat(75.0),
			},
			expected: true,
		},
		{
			name: "multiple legacy fields set",
			config: alerts.ThresholdConfig{
				CPULegacy:    ptrFloat(80.0),
				MemoryLegacy: ptrFloat(85.0),
				DiskLegacy:   ptrFloat(90.0),
			},
			expected: true,
		},
		{
			name: "mixed modern and legacy",
			config: alerts.ThresholdConfig{
				CPU:          &alerts.HysteresisThreshold{Trigger: 80, Clear: 70},
				MemoryLegacy: ptrFloat(85.0),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasLegacyThresholds(tt.config)
			if result != tt.expected {
				t.Errorf("hasLegacyThresholds() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFingerprintPublicKey(t *testing.T) {
	t.Parallel()

	// Valid SSH keys for testing (these are public keys, safe to include)
	// ED25519 key from openssh-portable test suite
	validED25519Key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example.com"

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
	}{
		// Error cases - empty/whitespace
		{
			name:        "empty string returns error",
			input:       "",
			wantErr:     true,
			errContains: "empty public key",
		},
		{
			name:        "whitespace only returns error",
			input:       "   ",
			wantErr:     true,
			errContains: "empty public key",
		},
		{
			name:        "tab only returns error",
			input:       "\t",
			wantErr:     true,
			errContains: "empty public key",
		},
		{
			name:        "newline only returns error",
			input:       "\n",
			wantErr:     true,
			errContains: "empty public key",
		},

		// Error cases - invalid key format
		{
			name:        "random text returns error",
			input:       "this is not an ssh key",
			wantErr:     true,
			errContains: "", // ssh library error message varies
		},
		{
			name:        "partial key returns error",
			input:       "ssh-ed25519",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "wrong key type prefix returns error",
			input:       "ssh-fake AAAAC3NzaC1lZDI1NTE5AAAAIOMQ==",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "base64 with wrong algorithm returns error",
			input:       "ssh-ed25519 notvalidbase64!!!",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "truncated base64 returns error",
			input:       "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "malformed key returns error",
			input:       "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJ",
			wantErr:     true,
			errContains: "",
		},

		// Success cases
		{
			name:    "valid ED25519 key returns fingerprint",
			input:   validED25519Key,
			wantErr: false,
		},
		{
			name:    "valid ED25519 key with leading whitespace",
			input:   "  " + validED25519Key,
			wantErr: false,
		},
		{
			name:    "valid ED25519 key with trailing whitespace",
			input:   validED25519Key + "  ",
			wantErr: false,
		},
		{
			name:    "valid ED25519 key without comment",
			input:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := fingerprintPublicKey(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("fingerprintPublicKey() expected error, got nil with result %q", result)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("fingerprintPublicKey() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("fingerprintPublicKey() unexpected error: %v", err)
				return
			}

			// Verify fingerprint format (SHA256:base64)
			if !strings.HasPrefix(result, "SHA256:") {
				t.Errorf("fingerprintPublicKey() = %q, expected SHA256: prefix", result)
			}
		})
	}
}

func TestCountLegacySSHKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T) string // returns directory path
		wantCount int
		wantErr   bool
	}{
		{
			name: "non-existent directory returns 0 with no error",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/to/ssh/keys"
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "empty directory returns 0",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "directory with no matching files returns 0",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(dir+"/known_hosts", []byte("test"), 0600)
				os.WriteFile(dir+"/authorized_keys", []byte("test"), 0600)
				os.WriteFile(dir+"/config", []byte("test"), 0600)
				return dir
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "directory with id_rsa counts as 1",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(dir+"/id_rsa", []byte("test"), 0600)
				return dir
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "directory with id_rsa and id_rsa.pub counts as 2",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(dir+"/id_rsa", []byte("test"), 0600)
				os.WriteFile(dir+"/id_rsa.pub", []byte("test"), 0600)
				return dir
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "directory with multiple key types",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(dir+"/id_rsa", []byte("test"), 0600)
				os.WriteFile(dir+"/id_rsa.pub", []byte("test"), 0600)
				os.WriteFile(dir+"/id_ed25519", []byte("test"), 0600)
				os.WriteFile(dir+"/id_ed25519.pub", []byte("test"), 0600)
				os.WriteFile(dir+"/id_ecdsa", []byte("test"), 0600)
				return dir
			},
			wantCount: 5,
			wantErr:   false,
		},
		{
			name: "subdirectories named id_* are not counted",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.Mkdir(dir+"/id_subdirectory", 0755)
				os.WriteFile(dir+"/id_rsa", []byte("test"), 0600)
				return dir
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "mixed files and directories",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				os.Mkdir(dir+"/id_subdir", 0755)
				os.WriteFile(dir+"/id_rsa", []byte("test"), 0600)
				os.WriteFile(dir+"/id_ed25519", []byte("test"), 0600)
				os.WriteFile(dir+"/known_hosts", []byte("test"), 0600)
				os.WriteFile(dir+"/authorized_keys", []byte("test"), 0600)
				return dir
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := tt.setup(t)
			count, err := countLegacySSHKeys(dir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("countLegacySSHKeys() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("countLegacySSHKeys() unexpected error: %v", err)
				return
			}

			if count != tt.wantCount {
				t.Errorf("countLegacySSHKeys() = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestResolveUserName(t *testing.T) {
	t.Parallel()

	// Get current user to have a valid UID to test
	currentUID := uint32(os.Getuid())

	tests := []struct {
		name     string
		uid      uint32
		validate func(t *testing.T, result string)
	}{
		{
			name: "UID 0 returns root",
			uid:  0,
			validate: func(t *testing.T, result string) {
				// On most systems, UID 0 is "root"
				if result != "root" && !strings.HasPrefix(result, "uid:") {
					t.Errorf("resolveUserName(0) = %q, want 'root' or 'uid:0'", result)
				}
			},
		},
		{
			name: "current user UID returns valid username",
			uid:  currentUID,
			validate: func(t *testing.T, result string) {
				// Should return a username or uid:X fallback
				if result == "" {
					t.Error("resolveUserName() returned empty string")
				}
				if strings.HasPrefix(result, "uid:") {
					// Fallback is acceptable
					expected := "uid:" + strings.TrimPrefix(result, "uid:")
					if result != expected {
						t.Errorf("resolveUserName() fallback format invalid: %q", result)
					}
				}
			},
		},
		{
			name: "non-existent UID returns uid:X format",
			uid:  999999999,
			validate: func(t *testing.T, result string) {
				expected := "uid:999999999"
				if result != expected {
					t.Errorf("resolveUserName(999999999) = %q, want %q", result, expected)
				}
			},
		},
		{
			name: "max uint32 UID returns uid:X format",
			uid:  ^uint32(0), // 4294967295
			validate: func(t *testing.T, result string) {
				expected := "uid:4294967295"
				if result != expected {
					t.Errorf("resolveUserName(max) = %q, want %q", result, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolveUserName(tt.uid)
			tt.validate(t, result)
		})
	}
}

func TestResolveGroupName(t *testing.T) {
	t.Parallel()

	// Get current group to have a valid GID to test
	currentGID := uint32(os.Getgid())

	tests := []struct {
		name     string
		gid      uint32
		validate func(t *testing.T, result string)
	}{
		{
			name: "GID 0 returns root or wheel",
			gid:  0,
			validate: func(t *testing.T, result string) {
				// On most systems, GID 0 is "root" or "wheel"
				if result != "root" && result != "wheel" && !strings.HasPrefix(result, "gid:") {
					t.Errorf("resolveGroupName(0) = %q, want 'root', 'wheel', or 'gid:0'", result)
				}
			},
		},
		{
			name: "current group GID returns valid group name",
			gid:  currentGID,
			validate: func(t *testing.T, result string) {
				// Should return a group name or gid:X fallback
				if result == "" {
					t.Error("resolveGroupName() returned empty string")
				}
				if strings.HasPrefix(result, "gid:") {
					// Fallback is acceptable
					expected := "gid:" + strings.TrimPrefix(result, "gid:")
					if result != expected {
						t.Errorf("resolveGroupName() fallback format invalid: %q", result)
					}
				}
			},
		},
		{
			name: "non-existent GID returns gid:X format",
			gid:  999999999,
			validate: func(t *testing.T, result string) {
				expected := "gid:999999999"
				if result != expected {
					t.Errorf("resolveGroupName(999999999) = %q, want %q", result, expected)
				}
			},
		},
		{
			name: "max uint32 GID returns gid:X format",
			gid:  ^uint32(0), // 4294967295
			validate: func(t *testing.T, result string) {
				expected := "gid:4294967295"
				if result != expected {
					t.Errorf("resolveGroupName(max) = %q, want %q", result, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolveGroupName(tt.gid)
			tt.validate(t, result)
		})
	}
}
