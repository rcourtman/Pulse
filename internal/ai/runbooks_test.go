package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
)

func TestMatchRunbooksForFinding_DockerRestartLoop(t *testing.T) {
	finding := &Finding{
		Key:          "restart-loop",
		ResourceType: "docker_container",
	}

	runbooks := matchRunbooksForFinding(finding)
	if len(runbooks) == 0 {
		t.Fatal("expected at least one runbook")
	}

	found := false
	for _, rb := range runbooks {
		if rb.ID == "docker-restart-loop" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected docker-restart-loop runbook to match")
	}
}

func TestMatchRunbooksForFinding_DockerHighMemory(t *testing.T) {
	finding := &Finding{
		Key:          "high-memory",
		ResourceType: "docker_container",
	}

	runbooks := matchRunbooksForFinding(finding)
	found := false
	for _, rb := range runbooks {
		if rb.ID == "docker-high-memory-restart" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected docker-high-memory-restart runbook to match")
	}
}

func TestVerifyDiskUsage(t *testing.T) {
	finding := &Finding{
		ResourceType: "container",
		Evidence:     "Disk: 95.0%",
	}
	thresholds := PatrolThresholds{
		GuestDiskWatch: 85,
	}

	output := `Filesystem 1024-blocks Used Available Capacity Mounted on
/dev/sda1 20511356 1000000 19511356 80% /`

	outcome, note := verifyDiskUsage(output, finding, thresholds)
	if outcome != memory.OutcomeResolved {
		t.Fatalf("expected resolved, got %s (%s)", outcome, note)
	}

	output = `Filesystem 1024-blocks Used Available Capacity Mounted on
/dev/sda1 20511356 1000000 19511356 93% /`

	outcome, _ = verifyDiskUsage(output, finding, thresholds)
	if outcome != memory.OutcomePartial {
		t.Fatalf("expected partial, got %s", outcome)
	}
}

// ========================================
// escapeShellArg tests
// ========================================

func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with space",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "string with single quote",
			input:    "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "string with tab",
			input:    "hello\tworld",
			expected: "'hello\tworld'",
		},
		{
			name:     "alphanumeric only",
			input:    "test123",
			expected: "test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeShellArg(tt.input)
			if result != tt.expected {
				t.Errorf("escapeShellArg(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ========================================
// parseVMID tests
// ========================================

func TestParseVMID(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		expected   string
	}{
		{
			name:       "empty string",
			resourceID: "",
			expected:   "",
		},
		{
			name:       "simple vmid",
			resourceID: "node1-100",
			expected:   "100",
		},
		{
			name:       "complex id",
			resourceID: "pve-cluster-node-200",
			expected:   "200",
		},
		{
			name:       "non-numeric ending",
			resourceID: "node1-abc",
			expected:   "",
		},
		{
			name:       "single number",
			resourceID: "100",
			expected:   "100",
		},
		{
			name:       "no dash prefix",
			resourceID: "node",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVMID(tt.resourceID)
			if result != tt.expected {
				t.Errorf("parseVMID(%q) = %q, want %q", tt.resourceID, result, tt.expected)
			}
		})
	}
}

// ========================================
// truncateRunbookOutput tests
// ========================================

func TestTruncateRunbookOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		limit    int
		expected string
	}{
		{
			name:     "empty string",
			output:   "",
			limit:    10,
			expected: "",
		},
		{
			name:     "under limit",
			output:   "hello",
			limit:    10,
			expected: "hello",
		},
		{
			name:     "at limit",
			output:   "helloworld",
			limit:    10,
			expected: "helloworld",
		},
		{
			name:     "over limit",
			output:   "hello world example",
			limit:    10,
			expected: "hello worl...",
		},
		{
			name:     "with whitespace",
			output:   "  hello  ",
			limit:    10,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateRunbookOutput(tt.output, tt.limit)
			if result != tt.expected {
				t.Errorf("truncateRunbookOutput(%q, %d) = %q, want %q", tt.output, tt.limit, result, tt.expected)
			}
		})
	}
}

// ========================================
// getRunbookByID tests
// ========================================

func TestGetRunbookByID(t *testing.T) {
	// Test finding a real runbook from the catalog
	rb, found := getRunbookByID("docker-restart-loop")
	if !found {
		t.Fatal("expected to find docker-restart-loop runbook")
	}
	if rb.ID != "docker-restart-loop" {
		t.Errorf("expected ID 'docker-restart-loop', got %q", rb.ID)
	}

	// Test not finding a non-existent runbook
	_, found = getRunbookByID("non-existent-runbook")
	if found {
		t.Error("expected not to find non-existent-runbook")
	}
}

// ========================================
// parseDFUsagePercent tests
// ========================================

func TestParseDFUsagePercent(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
		ok       bool
	}{
		{
			name:     "standard df output",
			output:   "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 20511356 1000000 19511356 80% /",
			expected: 80,
			ok:       true,
		},
		{
			name:     "no percentage",
			output:   "No disk info here",
			expected: 0,
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parseDFUsagePercent(tt.output)
			if ok != tt.ok {
				t.Errorf("parseDFUsagePercent(%q) ok = %v, want %v", tt.output, ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("parseDFUsagePercent(%q) = %v, want %v", tt.output, result, tt.expected)
			}
		})
	}
}

// ========================================
// parsePercentFromFinding tests
// ========================================

func TestParsePercentFromFinding(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
		expected float64
	}{
		{
			name:     "disk percentage",
			evidence: "Disk: 95.0%",
			expected: 95.0,
		},
		{
			name:     "cpu percentage",
			evidence: "CPU usage is at 87.5% utilization",
			expected: 87.5,
		},
		{
			name:     "no percentage",
			evidence: "No percentage here",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePercentFromFinding(tt.evidence)
			if result != tt.expected {
				t.Errorf("parsePercentFromFinding(%q) = %v, want %v", tt.evidence, result, tt.expected)
			}
		})
	}
}

// ========================================
// runbookApplies tests
// ========================================

func TestRunbookApplies(t *testing.T) {
	// Test with matching resource type and exact key
	rb := Runbook{
		ResourceTypes: []string{"docker_container"},
		FindingKeys:   []string{"restart-loop"},
	}
	finding := &Finding{
		Key:          "restart-loop", // exact match required
		ResourceType: "docker_container",
	}
	if !runbookApplies(rb, finding) {
		t.Error("expected runbook to apply")
	}

	// Test with non-matching resource type
	finding2 := &Finding{
		Key:          "restart-loop",
		ResourceType: "vm",
	}
	if runbookApplies(rb, finding2) {
		t.Error("expected runbook NOT to apply (wrong resource type)")
	}

	// Test with non-matching key
	finding3 := &Finding{
		Key:          "high-memory",
		ResourceType: "docker_container",
	}
	if runbookApplies(rb, finding3) {
		t.Error("expected runbook NOT to apply (wrong key)")
	}

	// Test with empty FindingKeys (should match any key)
	rb2 := Runbook{
		ResourceTypes: []string{"vm"},
		FindingKeys:   []string{}, // empty means match any key
	}
	finding4 := &Finding{
		Key:          "any-key",
		ResourceType: "vm",
	}
	if !runbookApplies(rb2, finding4) {
		t.Error("expected runbook with empty FindingKeys to apply to any key")
	}
}

// ========================================
// parsePBSResourceParts tests
// ========================================

func TestParsePBSResourceParts(t *testing.T) {
	tests := []struct {
		name          string
		resourceID    string
		expectedPBS   string
		expectedDS    string
		expectedJobID string
	}{
		{
			name:          "empty string",
			resourceID:    "",
			expectedPBS:   "",
			expectedDS:    "",
			expectedJobID: "",
		},
		{
			name:          "simple pbs id only",
			resourceID:    "pbs1",
			expectedPBS:   "pbs1",
			expectedDS:    "",
			expectedJobID: "",
		},
		{
			name:          "pbs with datastore",
			resourceID:    "pbs1:datastore1",
			expectedPBS:   "pbs1",
			expectedDS:    "datastore1",
			expectedJobID: "",
		},
		{
			name:          "pbs with job",
			resourceID:    "pbs1:job:backup-job-1",
			expectedPBS:   "pbs1",
			expectedDS:    "",
			expectedJobID: "backup-job-1",
		},
		{
			name:          "pbs with verify job",
			resourceID:    "pbs1:verify:verify-job-1",
			expectedPBS:   "pbs1",
			expectedDS:    "",
			expectedJobID: "verify-job-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pbsID, ds, jobID := parsePBSResourceParts(tt.resourceID)
			if pbsID != tt.expectedPBS {
				t.Errorf("pbsID = %q, want %q", pbsID, tt.expectedPBS)
			}
			if ds != tt.expectedDS {
				t.Errorf("ds = %q, want %q", ds, tt.expectedDS)
			}
			if jobID != tt.expectedJobID {
				t.Errorf("jobID = %q, want %q", jobID, tt.expectedJobID)
			}
		})
	}
}

// ========================================
// runbookTargetType tests
// ========================================

func TestRunbookTargetType(t *testing.T) {
	tests := []struct {
		name       string
		finding    *Finding
		runOnHost  bool
		expected   string
	}{
		{
			name:      "run on host override",
			finding:   &Finding{ResourceType: "vm"},
			runOnHost: true,
			expected:  "host",
		},
		{
			name:      "vm resource",
			finding:   &Finding{ResourceType: "vm"},
			runOnHost: false,
			expected:  "vm",
		},
		{
			name:      "container resource",
			finding:   &Finding{ResourceType: "container"},
			runOnHost: false,
			expected:  "container",
		},
		{
			name:      "other resource type",
			finding:   &Finding{ResourceType: "storage"},
			runOnHost: false,
			expected:  "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runbookTargetType(tt.finding, tt.runOnHost)
			if result != tt.expected {
				t.Errorf("runbookTargetType() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ========================================
// renderRunbookCommand tests
// ========================================

func TestRenderRunbookCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		ctx         runbookContext
		expected    string
		expectError bool
	}{
		{
			name:    "simple command no placeholders",
			command: "echo hello",
			ctx:     runbookContext{},
			expected: "echo hello",
		},
		{
			name:    "command with resource_id",
			command: "restart {{resource_id}}",
			ctx:     runbookContext{ResourceID: "node1-100"},
			expected: "restart node1-100",
		},
		{
			name:    "command with vmid",
			command: "qm reboot {{vmid}}",
			ctx:     runbookContext{VMID: "100"},
			expected: "qm reboot 100",
		},
		{
			name:    "command with node",
			command: "ssh {{node}} hostname",
			ctx:     runbookContext{Node: "node1"},
			expected: "ssh node1 hostname",
		},
		{
			name:        "missing required value",
			command:     "restart {{vmid}}",
			ctx:         runbookContext{VMID: ""},
			expectError: true,
		},
		{
			name:    "command with value needing escaping",
			command: "echo {{resource_name}}",
			ctx:     runbookContext{ResourceName: "my vm"},
			expected: "echo 'my vm'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderRunbookCommand(tt.command, tt.ctx)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("renderRunbookCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

