package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestUniqueNonEmptyStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all empty strings",
			input:    []string{"", "", ""},
			expected: []string{},
		},
		{
			name:     "all whitespace strings",
			input:    []string{"  ", "\t", "\n"},
			expected: []string{},
		},
		{
			name:     "duplicates removed",
			input:    []string{"foo", "bar", "foo", "baz"},
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "order preserved",
			input:    []string{"first", "second", "third"},
			expected: []string{"first", "second", "third"},
		},
		{
			name:     "whitespace trimmed",
			input:    []string{"  foo  ", "bar", "  foo"},
			expected: []string{"foo", "bar"},
		},
		{
			name:     "mixed empty and non-empty",
			input:    []string{"", "foo", "", "bar", "foo"},
			expected: []string{"foo", "bar"},
		},
		{
			name:     "single value",
			input:    []string{"value"},
			expected: []string{"value"},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := uniqueNonEmptyStrings(tt.input...)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSanitizeDockerHostSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "lowercase conversion",
			input:    "UPPERCASE",
			expected: "uppercase",
		},
		{
			name:     "special chars to hyphens",
			input:    "foo@bar!baz",
			expected: "foo-bar-baz",
		},
		{
			name:     "consecutive special chars become single hyphen",
			input:    "foo@@@bar",
			expected: "foo-bar",
		},
		{
			name:     "leading hyphen trimmed",
			input:    "@@@foo",
			expected: "foo",
		},
		{
			name:     "trailing hyphen trimmed",
			input:    "foo@@@",
			expected: "foo",
		},
		{
			name:     "leading and trailing hyphens trimmed",
			input:    "@@foo@@",
			expected: "foo",
		},
		{
			name:     "truncation at 48 runes",
			input:    "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
			expected: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijkl",
		},
		{
			name:     "unicode letters preserved",
			input:    "café",
			expected: "café",
		},
		{
			name:     "digits preserved",
			input:    "host123",
			expected: "host123",
		},
		{
			name:     "all special chars",
			input:    "@@@@",
			expected: "",
		},
		{
			name:     "mixed case with spaces",
			input:    "My Docker Host",
			expected: "my-docker-host",
		},
		{
			name:     "truncation with trailing special char",
			input:    "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij@@@",
			expected: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeDockerHostSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTokenHintFromRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		record   *config.APITokenRecord
		expected string
	}{
		{
			name:     "nil record",
			record:   nil,
			expected: "",
		},
		{
			name: "both prefix and suffix",
			record: &config.APITokenRecord{
				Prefix: "ps_abc",
				Suffix: "xyz",
			},
			expected: "ps_abc…xyz",
		},
		{
			name: "only prefix",
			record: &config.APITokenRecord{
				Prefix: "ps_abc",
			},
			expected: "ps_abc…",
		},
		{
			name: "only suffix",
			record: &config.APITokenRecord{
				Suffix: "xyz",
			},
			expected: "…xyz",
		},
		{
			name:     "neither prefix nor suffix",
			record:   &config.APITokenRecord{},
			expected: "",
		},
		{
			name: "empty strings",
			record: &config.APITokenRecord{
				Prefix: "",
				Suffix: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tokenHintFromRecord(tt.record)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDockerHostIDExists(t *testing.T) {
	t.Parallel()

	hosts := []models.DockerHost{
		{ID: "host1"},
		{ID: "host2"},
		{ID: "host3"},
	}

	tests := []struct {
		name     string
		id       string
		hosts    []models.DockerHost
		expected bool
	}{
		{
			name:     "empty id",
			id:       "",
			hosts:    hosts,
			expected: false,
		},
		{
			name:     "whitespace id",
			id:       "   ",
			hosts:    hosts,
			expected: false,
		},
		{
			name:     "id found",
			id:       "host2",
			hosts:    hosts,
			expected: true,
		},
		{
			name:     "id not found",
			id:       "host4",
			hosts:    hosts,
			expected: false,
		},
		{
			name:     "empty hosts list",
			id:       "host1",
			hosts:    []models.DockerHost{},
			expected: false,
		},
		{
			name:     "nil hosts list",
			id:       "host1",
			hosts:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := dockerHostIDExists(tt.id, tt.hosts)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDockerHostSuffixCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		report      agentsdocker.Report
		tokenRecord *config.APITokenRecord
		expected    []string
	}{
		{
			name:        "nil tokenRecord, empty report",
			report:      agentsdocker.Report{},
			tokenRecord: nil,
			expected:    []string{},
		},
		{
			name: "tokenRecord with ID",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-123"},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-456"},
			expected:    []string{"token-token-456", "agent-agent-123"},
		},
		{
			name: "agent ID only",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-abc"},
			},
			tokenRecord: nil,
			expected:    []string{"agent-agent-abc"},
		},
		{
			name: "machine ID only",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{MachineID: "machine-xyz"},
			},
			tokenRecord: nil,
			expected:    []string{"machine-machine-xyz"},
		},
		{
			name: "hostname only",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{Hostname: "my-host"},
			},
			tokenRecord: nil,
			expected:    []string{"host-my-host"},
		},
		{
			name: "display name different from hostname",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					Hostname: "host1",
					Name:     "Custom Name",
				},
			},
			tokenRecord: nil,
			expected:    []string{"host-host1", "name-custom-name"},
		},
		{
			name: "display name same as hostname",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					Hostname: "my-host",
					Name:     "my-host",
				},
			},
			tokenRecord: nil,
			expected:    []string{"host-my-host"},
		},
		{
			name: "all fields present",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
				Host: agentsdocker.HostInfo{
					MachineID: "machine-1",
					Hostname:  "host-1",
					Name:      "Display-1",
				},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-1"},
			expected:    []string{"token-token-1", "agent-agent-1", "machine-machine-1", "host-host-1", "name-display-1"},
		},
		{
			name: "sanitization applies",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "Agent@123"},
			},
			tokenRecord: nil,
			expected:    []string{"agent-agent-123"},
		},
		{
			name: "duplicates removed after sanitization",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					Hostname: "my-host",
					Name:     "MY-HOST",
				},
			},
			tokenRecord: nil,
			expected:    []string{"host-my-host"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := dockerHostSuffixCandidates(tt.report, tt.tokenRecord)
			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d\ngot: %v\nwant: %v", len(result), len(tt.expected), result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFallbackDockerHostID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		report      agentsdocker.Report
		tokenRecord *config.APITokenRecord
		expectEmpty bool
		expectHash  bool
	}{
		{
			name:        "empty candidates",
			report:      agentsdocker.Report{},
			tokenRecord: nil,
			expectEmpty: true,
		},
		{
			name: "has candidates",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-123"},
			},
			tokenRecord: nil,
			expectHash:  true,
		},
		{
			name: "multiple candidates",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-123"},
				Host: agentsdocker.HostInfo{
					MachineID: "machine-456",
					Hostname:  "my-host",
				},
			},
			tokenRecord: nil,
			expectHash:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := fallbackDockerHostID(tt.report, tt.tokenRecord)
			if tt.expectEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			}
			if tt.expectHash {
				if result == "" {
					t.Errorf("expected non-empty hash, got empty string")
				}
				if len(result) < len("docker-host-") {
					t.Errorf("expected docker-host- prefix, got %q", result)
				}
				if result[:12] != "docker-host-" {
					t.Errorf("expected docker-host- prefix, got %q", result[:12])
				}
				// Should be 12 hex chars after prefix (6 bytes)
				if len(result) != 12+12 {
					t.Errorf("expected length %d, got %d: %q", 12+12, len(result), result)
				}
			}
		})
	}
}

func TestFindMatchingDockerHost(t *testing.T) {
	t.Parallel()

	hosts := []models.DockerHost{
		{
			ID:        "host1",
			AgentID:   "agent-1",
			TokenID:   "token-1",
			MachineID: "machine-1",
			Hostname:  "hostname-1",
		},
		{
			ID:        "host2",
			AgentID:   "agent-2",
			TokenID:   "",
			MachineID: "machine-2",
			Hostname:  "hostname-2",
		},
		{
			ID:        "host3",
			AgentID:   "",
			TokenID:   "",
			MachineID: "machine-3",
			Hostname:  "hostname-3",
		},
		{
			ID:        "host4",
			AgentID:   "agent-4",
			TokenID:   "token-4",
			MachineID: "machine-4",
			Hostname:  "hostname-4",
		},
	}

	tests := []struct {
		name        string
		hosts       []models.DockerHost
		report      agentsdocker.Report
		tokenRecord *config.APITokenRecord
		expectedID  string
		expectMatch bool
	}{
		{
			name:        "empty hosts",
			hosts:       []models.DockerHost{},
			report:      agentsdocker.Report{},
			tokenRecord: nil,
			expectMatch: false,
		},
		{
			name:  "match by agentID with matching token",
			hosts: hosts,
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-1"},
			expectedID:  "host1",
			expectMatch: true,
		},
		{
			name:  "match by agentID with no token",
			hosts: hosts,
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-2"},
			},
			tokenRecord: nil,
			expectedID:  "host2",
			expectMatch: true,
		},
		{
			name:  "match by machineID and hostname with matching token",
			hosts: hosts,
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					MachineID: "machine-4",
					Hostname:  "hostname-4",
				},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-4"},
			expectedID:  "host4",
			expectMatch: true,
		},
		{
			name:  "match by machineID and hostname with no token",
			hosts: hosts,
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					MachineID: "machine-3",
					Hostname:  "hostname-3",
				},
			},
			tokenRecord: nil,
			expectedID:  "host3",
			expectMatch: true,
		},
		{
			name:  "match by machineID only (no token)",
			hosts: hosts,
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					MachineID: "machine-3",
				},
			},
			tokenRecord: nil,
			expectedID:  "host3",
			expectMatch: true,
		},
		{
			name:  "match by hostname only (no token)",
			hosts: hosts,
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					Hostname: "hostname-3",
				},
			},
			tokenRecord: nil,
			expectedID:  "host3",
			expectMatch: true,
		},
		{
			name:  "no match - token mismatch on agentID",
			hosts: hosts,
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
			},
			tokenRecord: &config.APITokenRecord{ID: "wrong-token"},
			expectMatch: false,
		},
		{
			name:  "no match - token mismatch on machineID+hostname",
			hosts: hosts,
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{
					MachineID: "machine-1",
					Hostname:  "hostname-1",
				},
			},
			tokenRecord: &config.APITokenRecord{ID: "wrong-token"},
			expectMatch: false,
		},
		{
			name:  "no match - unknown agent",
			hosts: hosts,
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-unknown"},
			},
			tokenRecord: nil,
			expectMatch: false,
		},
		{
			name:  "whitespace trimming on agentID",
			hosts: hosts,
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "  agent-1  "},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-1"},
			expectedID:  "host1",
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, found := findMatchingDockerHost(tt.hosts, tt.report, tt.tokenRecord)
			if found != tt.expectMatch {
				t.Errorf("found mismatch: got %v, want %v", found, tt.expectMatch)
			}
			if tt.expectMatch && result.ID != tt.expectedID {
				t.Errorf("ID mismatch: got %q, want %q", result.ID, tt.expectedID)
			}
		})
	}
}

func TestGenerateDockerHostIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		report   agentsdocker.Report
		token    *config.APITokenRecord
		hosts    []models.DockerHost
		expected string
	}{
		{
			name:     "empty base with fallback from report",
			base:     "",
			report:   agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "agent-1"}},
			token:    nil,
			hosts:    []models.DockerHost{},
			expected: "docker-host-546dbe5233c6::agent-agent-1",
		},
		{
			name:     "empty base with fallback from token",
			base:     "",
			report:   agentsdocker.Report{},
			token:    &config.APITokenRecord{ID: "token-abc"},
			hosts:    []models.DockerHost{},
			expected: "docker-host-a9bb97f6f774::token-token-abc",
		},
		{
			name:     "empty base no fallback uses docker-host",
			base:     "",
			report:   agentsdocker.Report{},
			token:    nil,
			hosts:    []models.DockerHost{},
			expected: "docker-host::hash-bd318191f5b8",
		},
		{
			name:     "whitespace base no fallback uses docker-host",
			base:     "   ",
			report:   agentsdocker.Report{},
			token:    nil,
			hosts:    []models.DockerHost{},
			expected: "docker-host::hash-bd318191f5b8",
		},
		{
			name:   "first suffix candidate works",
			base:   "my-host",
			report: agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "agent-1"}},
			token:  nil,
			hosts: []models.DockerHost{
				{ID: "other-host"},
			},
			expected: "my-host::agent-agent-1",
		},
		{
			name: "second suffix candidate when first taken",
			base: "my-host",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
				Host:  agentsdocker.HostInfo{MachineID: "machine-1"},
			},
			token: nil,
			hosts: []models.DockerHost{
				{ID: "my-host::agent-agent-1"},
			},
			expected: "my-host::machine-machine-1",
		},
		{
			name:   "hash suffix when all candidates taken",
			base:   "my-host",
			report: agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "agent-1"}},
			token:  nil,
			hosts: []models.DockerHost{
				{ID: "my-host::agent-agent-1"},
			},
			expected: "my-host::hash-546dbe5233c6",
		},
		{
			name:   "numeric suffix when hash also taken",
			base:   "my-host",
			report: agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "agent-1"}},
			token:  nil,
			hosts: []models.DockerHost{
				{ID: "my-host::agent-agent-1"},
				{ID: "my-host::hash-546dbe5233c6"},
			},
			expected: "my-host::2",
		},
		{
			name:   "numeric suffix increments",
			base:   "my-host",
			report: agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "agent-1"}},
			token:  nil,
			hosts: []models.DockerHost{
				{ID: "my-host::agent-agent-1"},
				{ID: "my-host::hash-546dbe5233c6"},
				{ID: "my-host::2"},
				{ID: "my-host::3"},
			},
			expected: "my-host::4",
		},
		{
			name:     "empty suffixes list seed becomes base",
			base:     "my-host",
			report:   agentsdocker.Report{},
			token:    nil,
			hosts:    []models.DockerHost{},
			expected: "my-host::hash-6dd480c53846",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := generateDockerHostIdentifier(tt.base, tt.report, tt.token, tt.hosts)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResolveDockerHostIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		report          agentsdocker.Report
		tokenRecord     *config.APITokenRecord
		hosts           []models.DockerHost
		expectMatch     bool
		expectFallbacks int
		expectedID      string
		checkIDFormat   bool
	}{
		{
			name: "existing host match",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
			},
			tokenRecord: &config.APITokenRecord{ID: "token-1"},
			hosts: []models.DockerHost{
				{ID: "existing-host", AgentID: "agent-1", TokenID: "token-1"},
			},
			expectMatch:     true,
			expectedID:      "existing-host",
			expectFallbacks: 1,
		},
		{
			name: "no match, base available",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "new-agent"},
			},
			tokenRecord:     nil,
			hosts:           []models.DockerHost{{ID: "other-host"}},
			expectMatch:     false,
			expectedID:      "new-agent",
			expectFallbacks: 1,
		},
		{
			name: "no match, base taken, generates unique",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
				Host:  agentsdocker.HostInfo{MachineID: "machine-1"},
			},
			tokenRecord: nil,
			hosts: []models.DockerHost{
				{ID: "agent-1"},
			},
			expectMatch:     false,
			checkIDFormat:   true,
			expectFallbacks: 2,
		},
		{
			name: "empty report uses fallback",
			report: agentsdocker.Report{
				Host: agentsdocker.HostInfo{Hostname: "my-host"},
			},
			tokenRecord:     nil,
			hosts:           []models.DockerHost{},
			expectMatch:     false,
			expectedID:      "my-host",
			expectFallbacks: 1,
		},
		{
			name:            "completely empty report",
			report:          agentsdocker.Report{},
			tokenRecord:     nil,
			hosts:           []models.DockerHost{},
			expectMatch:     false,
			expectedID:      "docker-host",
			expectFallbacks: 0,
		},
		{
			name: "multiple fallback candidates",
			report: agentsdocker.Report{
				Agent: agentsdocker.AgentInfo{ID: "agent-1"},
				Host: agentsdocker.HostInfo{
					MachineID: "machine-1",
					Hostname:  "host-1",
				},
			},
			tokenRecord:     nil,
			hosts:           []models.DockerHost{},
			expectMatch:     false,
			expectedID:      "agent-1",
			expectFallbacks: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, fallbacks, existing, found := resolveDockerHostIdentifier(tt.report, tt.tokenRecord, tt.hosts)
			if found != tt.expectMatch {
				t.Errorf("found mismatch: got %v, want %v", found, tt.expectMatch)
			}
			if tt.expectMatch && existing.ID != tt.expectedID {
				t.Errorf("existing ID mismatch: got %q, want %q", existing.ID, tt.expectedID)
			}
			if !tt.expectMatch && existing.ID != "" {
				t.Errorf("expected empty existing host, got %q", existing.ID)
			}
			if tt.expectedID != "" && !tt.checkIDFormat && id != tt.expectedID {
				t.Errorf("ID mismatch: got %q, want %q", id, tt.expectedID)
			}
			if tt.checkIDFormat && id == "" {
				t.Errorf("expected non-empty ID")
			}
			if len(fallbacks) != tt.expectFallbacks {
				t.Errorf("fallback count mismatch: got %d, want %d", len(fallbacks), tt.expectFallbacks)
			}
		})
	}
}

func TestSanitizeDockerHostSuffix_UnicodeRunes(t *testing.T) {
	t.Parallel()

	// Test that rune counting works correctly (not byte counting)
	input := "🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀🚀" // 12 emoji (48+ bytes but 12 runes)
	result := sanitizeDockerHostSuffix(input)

	// All emojis should become single hyphen since they're not letters/digits
	expected := ""
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}

	// Test with mix of emoji and letters
	input = "abc🚀def"
	result = sanitizeDockerHostSuffix(input)
	expected = "abc-def"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
