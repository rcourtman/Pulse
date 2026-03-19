package memory

import (
	"strings"
	"testing"
	"time"
)

func TestChangeTypeLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind ChangeType
		want string
	}{
		{name: "created", kind: ChangeCreated, want: "Created"},
		{name: "deleted", kind: ChangeDeleted, want: "Deleted"},
		{name: "restart", kind: ChangeRestarted, want: "Restart"},
		{name: "fallback", kind: ChangeType("custom_type"), want: "Custom type"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ChangeTypeLabel(tc.kind); got != tc.want {
				t.Fatalf("ChangeTypeLabel(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestFormatRecentChangesContext(t *testing.T) {
	t.Parallel()

	now := time.Now()
	changes := []Change{
		{
			ResourceID:   "res-0",
			ResourceType: "node",
			ResourceName: "lb-01",
			ChangeType:   ChangeCreated,
			Description:  "came online",
			DetectedAt:   now.Add(-30 * time.Second),
		},
		{
			ResourceID:   "res-1",
			ResourceType: "vm",
			ResourceName: "web-01",
			ChangeType:   ChangeRestarted,
			Description:  "restarted after maintenance",
			DetectedAt:   now.Add(-90 * time.Minute),
		},
		{
			ResourceName: "cache-1",
			ResourceType: "container",
			ChangeType:   ChangeConfig,
			Description:  "adjusted memory limit",
			DetectedAt:   now.Add(-30 * time.Minute),
		},
	}

	got := FormatRecentChangesContext(changes, true, "###")
	for _, want := range []string{
		"### Recent Changes Across Infrastructure",
		"res-0 (node): **Created** came online (just now)",
		"res-1 (vm): **Restart** restarted after maintenance (1 hour ago)",
		"cache-1 (container): **Config update** adjusted memory limit (30 minutes ago)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatRecentChangesContext output %q does not contain %q", got, want)
		}
	}
}
