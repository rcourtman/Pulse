package ai

import "testing"

func TestInferFindingResourceType(t *testing.T) {
	tests := []struct {
		name         string
		resourceID   string
		resourceName string
		expected     string
	}{
		{
			name:         "pbs keyword in resource id",
			resourceID:   "pbs/datastore-1",
			resourceName: "Datastore",
			expected:     "pbs",
		},
		{
			name:         "backup keyword in resource name",
			resourceID:   "job-1",
			resourceName: "Nightly Backup",
			expected:     "pbs",
		},
		{
			name:         "storage keyword maps to storage",
			resourceID:   "zfs-pool-1",
			resourceName: "Primary Pool",
			expected:     "storage",
		},
		{
			name:         "docker wins over generic container keyword",
			resourceID:   "docker://app",
			resourceName: "Docker Container",
			expected:     "app-container",
		},
		{
			name:         "lxc keyword maps to system-container",
			resourceID:   "lxc/200",
			resourceName: "App CT",
			expected:     "system-container",
		},
		{
			name:         "vm keyword maps to vm",
			resourceID:   "cluster-vm-alpha",
			resourceName: "Compute VM",
			expected:     "vm",
		},
		{
			name:         "host keyword maps to agent",
			resourceID:   "host-01",
			resourceName: "Primary Host",
			expected:     "agent",
		},
		{
			name:         "numeric suffix fallback maps to vm",
			resourceID:   "qemu/101",
			resourceName: "guest",
			expected:     "vm",
		},
		{
			name:         "default fallback maps to node",
			resourceID:   "resource-x",
			resourceName: "unknown",
			expected:     "node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferFindingResourceType(tt.resourceID, tt.resourceName)
			if got != tt.expected {
				t.Fatalf("inferFindingResourceType(%q, %q) = %q, want %q", tt.resourceID, tt.resourceName, got, tt.expected)
			}
		})
	}
}

func TestHasFindingNumericSuffix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "empty", value: "", expected: false},
		{name: "whitespace", value: "   ", expected: false},
		{name: "plain number", value: "101", expected: true},
		{name: "colon suffix", value: "vm:101", expected: true},
		{name: "slash suffix", value: "qemu/202", expected: true},
		{name: "missing suffix after separator", value: "qemu/", expected: false},
		{name: "non numeric suffix", value: "vm:10a", expected: false},
		{name: "non numeric token", value: "vm-101", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFindingNumericSuffix(tt.value)
			if got != tt.expected {
				t.Fatalf("hasFindingNumericSuffix(%q) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestCanonicalFindingResourceType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "guest", want: ""},
		{in: "vm", want: "vm"},
		{in: "system_container", want: ""},
		{in: "docker_container", want: ""},
		{in: "docker_host", want: ""},
		{in: "kubernetes_cluster", want: ""},
		{in: "host", want: ""},
		{in: "storage", want: "storage"},
	}

	for _, tc := range tests {
		if got := canonicalFindingResourceType(tc.in); got != tc.want {
			t.Fatalf("canonicalFindingResourceType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeFindingResourceTypes_RejectsLegacyAndInfers(t *testing.T) {
	findings := []*Finding{
		{ID: "f1", ResourceType: "docker_container", ResourceID: "docker://abc", ResourceName: "Docker Container"},
		{ID: "f2", ResourceType: "", ResourceID: "lxc/101", ResourceName: "LXC Guest"},
	}

	normalizeFindingResourceTypes(findings)

	if findings[0].ResourceType != "app-container" {
		t.Fatalf("expected inferred app-container, got %q", findings[0].ResourceType)
	}
	if findings[1].ResourceType != "system-container" {
		t.Fatalf("expected inferred system-container, got %q", findings[1].ResourceType)
	}
}

func TestFindingsStoreAdd_InferResourceTypeWhenMissing(t *testing.T) {
	store := NewFindingsStore()

	added := store.Add(&Finding{
		ID:         "inferred",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "lxc/123",
		Title:      "LXC pressure",
	})
	if !added {
		t.Fatalf("expected new finding to be added")
	}

	inferred := store.Get("inferred")
	if inferred == nil {
		t.Fatalf("expected finding to exist")
	}
	if inferred.ResourceType != "system-container" {
		t.Fatalf("expected inferred resource type %q, got %q", "system-container", inferred.ResourceType)
	}

	added = store.Add(&Finding{
		ID:           "explicit",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "host-1",
		ResourceType: "vm",
		Title:        "Explicit type",
	})
	if !added {
		t.Fatalf("expected second finding to be added")
	}

	explicit := store.Get("explicit")
	if explicit == nil {
		t.Fatalf("expected explicit finding to exist")
	}
	if explicit.ResourceType != "vm" {
		t.Fatalf("expected explicit resource type to be preserved, got %q", explicit.ResourceType)
	}
}
