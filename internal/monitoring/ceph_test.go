package monitoring

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestIsCephStorageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		storageType string
		want        bool
	}{
		// Valid Ceph types
		{"rbd lowercase", "rbd", true},
		{"cephfs lowercase", "cephfs", true},
		{"ceph lowercase", "ceph", true},

		// Case variations
		{"RBD uppercase", "RBD", true},
		{"CephFS mixed case", "CephFS", true},
		{"CEPH uppercase", "CEPH", true},

		// Whitespace handling
		{"rbd with leading space", " rbd", true},
		{"rbd with trailing space", "rbd ", true},
		{"rbd with surrounding spaces", "  rbd  ", true},

		// Non-Ceph storage types
		{"local storage", "local", false},
		{"dir storage", "dir", false},
		{"nfs storage", "nfs", false},
		{"lvm storage", "lvm", false},
		{"zfs storage", "zfs", false},
		{"zfspool storage", "zfspool", false},
		{"iscsi storage", "iscsi", false},

		// Edge cases
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"partial match rbd", "rbd-pool", false},
		{"partial match ceph", "ceph-pool", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCephStorageType(tc.storageType); got != tc.want {
				t.Fatalf("isCephStorageType(%q) = %v, want %v", tc.storageType, got, tc.want)
			}
		})
	}
}

func TestCountServiceDaemons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		services    map[string]proxmox.CephServiceDefinition
		serviceType string
		want        int
	}{
		{
			name:        "nil services map",
			services:    nil,
			serviceType: "mon",
			want:        0,
		},
		{
			name:        "empty services map",
			services:    map[string]proxmox.CephServiceDefinition{},
			serviceType: "mon",
			want:        0,
		},
		{
			name: "service type not found",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
					},
				},
			},
			serviceType: "osd",
			want:        0,
		},
		{
			name: "single daemon",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
					},
				},
			},
			serviceType: "mon",
			want:        1,
		},
		{
			name: "multiple daemons",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
						"b": {Host: "node2", Status: "running"},
						"c": {Host: "node3", Status: "running"},
					},
				},
			},
			serviceType: "mon",
			want:        3,
		},
		{
			name: "multiple service types",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"a": {Host: "node1", Status: "running"},
						"b": {Host: "node2", Status: "running"},
					},
				},
				"mgr": {
					Daemons: map[string]proxmox.CephServiceDaemon{
						"node1": {Host: "node1", Status: "active"},
					},
				},
			},
			serviceType: "mgr",
			want:        1,
		},
		{
			name: "empty daemons map",
			services: map[string]proxmox.CephServiceDefinition{
				"mon": {
					Daemons: map[string]proxmox.CephServiceDaemon{},
				},
			},
			serviceType: "mon",
			want:        0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := countServiceDaemons(tc.services, tc.serviceType); got != tc.want {
				t.Fatalf("countServiceDaemons(%v, %q) = %d, want %d", tc.services, tc.serviceType, got, tc.want)
			}
		})
	}
}

func TestExtractCephCheckSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		// Empty/nil cases
		{
			name: "nil raw message",
			raw:  nil,
			want: "",
		},
		{
			name: "empty raw message",
			raw:  json.RawMessage{},
			want: "",
		},

		// Object format with message field
		{
			name: "object with message",
			raw:  json.RawMessage(`{"message": "1 slow ops"}`),
			want: "1 slow ops",
		},
		{
			name: "object with summary",
			raw:  json.RawMessage(`{"summary": "health warning"}`),
			want: "health warning",
		},
		{
			name: "object with both message and summary prefers message",
			raw:  json.RawMessage(`{"message": "from message", "summary": "from summary"}`),
			want: "from message",
		},
		{
			name: "object with empty message falls back to summary",
			raw:  json.RawMessage(`{"message": "", "summary": "fallback summary"}`),
			want: "fallback summary",
		},

		// Array format
		{
			name: "array with single item message",
			raw:  json.RawMessage(`[{"message": "array message"}]`),
			want: "array message",
		},
		{
			name: "array with single item summary",
			raw:  json.RawMessage(`[{"summary": "array summary"}]`),
			want: "array summary",
		},
		{
			name: "array with multiple items returns first",
			raw:  json.RawMessage(`[{"message": "first"}, {"message": "second"}]`),
			want: "first",
		},
		{
			name: "array skips empty to find message",
			raw:  json.RawMessage(`[{"message": ""}, {"message": "second"}]`),
			want: "second",
		},
		{
			name: "empty array",
			raw:  json.RawMessage(`[]`),
			want: "",
		},

		// Plain string format
		{
			name: "plain string",
			raw:  json.RawMessage(`"simple string message"`),
			want: "simple string message",
		},
		{
			name: "empty string",
			raw:  json.RawMessage(`""`),
			want: "",
		},

		// Invalid JSON
		{
			name: "invalid JSON",
			raw:  json.RawMessage(`{invalid`),
			want: "",
		},
		{
			name: "number value",
			raw:  json.RawMessage(`123`),
			want: "",
		},
		{
			name: "boolean value",
			raw:  json.RawMessage(`true`),
			want: "",
		},
		{
			name: "null value",
			raw:  json.RawMessage(`null`),
			want: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractCephCheckSummary(tc.raw); got != tc.want {
				t.Fatalf("extractCephCheckSummary(%s) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSummarizeCephHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *proxmox.CephStatus
		want   string
	}{
		{
			name:   "nil status",
			status: nil,
			want:   "",
		},
		{
			name:   "empty status",
			status: &proxmox.CephStatus{},
			want:   "",
		},
		{
			name: "single summary message",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "HEALTH_WARN"},
					},
				},
			},
			want: "HEALTH_WARN",
		},
		{
			name: "summary with summary field instead of message",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Summary: "1 pool(s) have no replicas configured"},
					},
				},
			},
			want: "1 pool(s) have no replicas configured",
		},
		{
			name: "multiple summary messages",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "first warning"},
						{Message: "second warning"},
					},
				},
			},
			want: "first warning; second warning",
		},
		{
			name: "health check with summary object",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"SLOW_OPS": {
							Summary: json.RawMessage(`{"message": "1 slow ops"}`),
						},
					},
				},
			},
			want: "SLOW_OPS: 1 slow ops",
		},
		{
			name: "health check with detail fallback",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"OSD_DOWN": {
							Summary: json.RawMessage(`{}`),
							Detail: []proxmox.CephCheckDetail{
								{Message: "osd.0 is down"},
							},
						},
					},
				},
			},
			want: "OSD_DOWN: osd.0 is down",
		},
		{
			name: "combined summary and checks",
			status: &proxmox.CephStatus{
				Health: proxmox.CephHealth{
					Summary: []proxmox.CephHealthSummary{
						{Message: "HEALTH_WARN"},
					},
					Checks: map[string]proxmox.CephHealthCheckRaw{
						"PG_DEGRADED": {
							Summary: json.RawMessage(`{"message": "Degraded data redundancy"}`),
						},
					},
				},
			},
			want: "HEALTH_WARN; PG_DEGRADED: Degraded data redundancy",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := summarizeCephHealth(tc.status); got != tc.want {
				t.Fatalf("summarizeCephHealth() = %q, want %q", got, tc.want)
			}
		})
	}
}
