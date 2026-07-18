package servicediscovery

import (
	"sort"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestDiscoveryReadinessForResource_Branches exercises the pure early-exit
// branches of (*Service).DiscoveryReadinessForResource: nil target, nil
// receiver, nil store, and unsupported resource type. The supported-type
// branch falls through into Store file I/O via GetDiscoveryByResource and is
// covered instead through the dedicated DiscoveryReadinessForTarget sibling
// tests.
func TestDiscoveryReadinessForResource_Branches(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	supportedTarget := &unified.DiscoveryTarget{
		ResourceType: "system-container",
		AgentID:      "node-a",
		ResourceID:   "101",
	}
	unsupportedTarget := &unified.DiscoveryTarget{
		ResourceType: "ceph",
		AgentID:      "cluster",
		ResourceID:   "fsid",
	}

	t.Run("nil-target-returns-unsupported", func(t *testing.T) {
		var s *Service // nil receiver is fine because target short-circuits first
		got := s.DiscoveryReadinessForResource(
			unified.Resource{DiscoveryTarget: nil}, now,
		)
		if got.State != unified.ResourceDiscoveryReadinessUnsupported {
			t.Fatalf("state = %q, want %q", got.State, unified.ResourceDiscoveryReadinessUnsupported)
		}
		if got.Source != discoveryReadinessSource {
			t.Fatalf("Source = %q, want %q", got.Source, discoveryReadinessSource)
		}
		if !got.GeneratedAt.Equal(now) {
			t.Fatalf("GeneratedAt = %v, want %v", got.GeneratedAt, now)
		}
		if got.ResourceType != "" || got.TargetID != "" || got.ResourceID != "" {
			t.Fatalf("base fields should be empty for nil target, got %+v", got)
		}
		// DiscoveryReadinessForTarget returns the Unsupported state before
		// populating StaleAfterSeconds; confirm the staleness slot is left at
		// its zero value (which is the actual observable behaviour here).
		if got.StaleAfterSeconds != 0 {
			t.Fatalf("StaleAfterSeconds = %d, want 0 (nil-target path returns before staleness is set)",
				got.StaleAfterSeconds)
		}
		// defaultDiscoveryMaxAge is the value passed into the inner call; it
		// cannot be observed because the unsupported branch short-circuits, so
		// assert the call still completes without populating a discovery id.
		if got.DiscoveryID != "" {
			t.Fatalf("DiscoveryID should be empty for nil target, got %q", got.DiscoveryID)
		}
	})

	t.Run("nil-receiver-returns-unavailable", func(t *testing.T) {
		var s *Service
		got := s.DiscoveryReadinessForResource(
			unified.Resource{DiscoveryTarget: supportedTarget}, now,
		)
		if got.State != unified.ResourceDiscoveryReadinessUnavailable {
			t.Fatalf("state = %q, want %q", got.State, unified.ResourceDiscoveryReadinessUnavailable)
		}
		if got.Reason != "Discovery service is not configured." {
			t.Fatalf("Reason = %q, want not-configured reason", got.Reason)
		}
		if got.ResourceType != "system-container" || got.TargetID != "node-a" || got.ResourceID != "101" {
			t.Fatalf("target fields not projected: %+v", got)
		}
	})

	t.Run("nil-store-returns-unavailable", func(t *testing.T) {
		s := &Service{} // non-nil receiver but nil store
		got := s.DiscoveryReadinessForResource(
			unified.Resource{DiscoveryTarget: supportedTarget}, now,
		)
		if got.State != unified.ResourceDiscoveryReadinessUnavailable {
			t.Fatalf("state = %q, want %q", got.State, unified.ResourceDiscoveryReadinessUnavailable)
		}
		if got.Reason != "Discovery service is not configured." {
			t.Fatalf("Reason = %q, want not-configured reason", got.Reason)
		}
	})

	t.Run("zero-now-is-normalized-to-utc", func(t *testing.T) {
		var s *Service
		got := s.DiscoveryReadinessForResource(
			unified.Resource{DiscoveryTarget: nil}, time.Time{},
		)
		if got.GeneratedAt.IsZero() {
			t.Fatal("zero now should be replaced with time.Now().UTC()")
		}
		if got.GeneratedAt.Location() != time.UTC {
			t.Fatalf("GeneratedAt location = %v, want UTC", got.GeneratedAt.Location())
		}
	})

	t.Run("unsupported-resource-type-returns-unsupported", func(t *testing.T) {
		// Non-nil store with configured maxDiscoveryAge exercises the
		// DiscoveryResourceTypeForTarget !ok branch without descending into
		// Store file I/O. maxDiscoveryAge is consumed by the inner call but
		// not observable on the projection (the unsupported branch returns
		// before StaleAfterSeconds is set).
		s := &Service{
			store:           &Store{},
			maxDiscoveryAge: 7 * 24 * time.Hour,
		}
		got := s.DiscoveryReadinessForResource(
			unified.Resource{DiscoveryTarget: unsupportedTarget}, now,
		)
		if got.State != unified.ResourceDiscoveryReadinessUnsupported {
			t.Fatalf("state = %q, want %q", got.State, unified.ResourceDiscoveryReadinessUnsupported)
		}
		if got.Reason != "Service discovery does not support this resource type." {
			t.Fatalf("Reason = %q, want unsupported-type reason", got.Reason)
		}
		// The unsupported-type branch returns from DiscoveryReadinessForTarget
		// before StaleAfterSeconds is populated, so the configured maxDiscoveryAge
		// is consumed by the call but not observable on the projection.
		if got.StaleAfterSeconds != 0 {
			t.Fatalf("StaleAfterSeconds = %d, want 0 (unsupported-type path returns before staleness is set)",
				got.StaleAfterSeconds)
		}
		if got.DiscoveryID != "" {
			t.Fatalf("DiscoveryID should be empty for unsupported target, got %q", got.DiscoveryID)
		}
	})
}

// TestDiscoveryReadinessUnavailableForTarget_Branches covers the unavailable
// projection: nil vs non-nil target, custom vs empty reason, and the default
// reason substitution when an empty reason is supplied.
func TestDiscoveryReadinessUnavailableForTarget_Branches(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	t.Run("nil-target-minimal-base", func(t *testing.T) {
		got := DiscoveryReadinessUnavailableForTarget(nil, now, "custom reason")
		if got.State != unified.ResourceDiscoveryReadinessUnavailable {
			t.Fatalf("state = %q, want unavailable", got.State)
		}
		if got.Reason != "custom reason" {
			t.Fatalf("Reason = %q, want custom reason", got.Reason)
		}
		if got.ResourceType != "" || got.TargetID != "" || got.ResourceID != "" {
			t.Fatalf("base fields should be empty for nil target, got %+v", got)
		}
		if !got.GeneratedAt.Equal(now) {
			t.Fatalf("GeneratedAt = %v, want %v", got.GeneratedAt, now)
		}
	})

	t.Run("non-nil-target-with-custom-reason", func(t *testing.T) {
		target := &unified.DiscoveryTarget{
			ResourceType: "vm",
			AgentID:      "node-b",
			ResourceID:   "200",
		}
		got := DiscoveryReadinessUnavailableForTarget(target, now, "  scanner offline  ")
		if got.State != unified.ResourceDiscoveryReadinessUnavailable {
			t.Fatalf("state = %q, want unavailable", got.State)
		}
		// Reason is trimmed before assignment.
		if got.Reason != "scanner offline" {
			t.Fatalf("Reason = %q, want trimmed 'scanner offline'", got.Reason)
		}
		if got.ResourceType != "vm" || got.TargetID != "node-b" || got.ResourceID != "200" {
			t.Fatalf("target fields not projected: %+v", got)
		}
		// Canonical vm target → discoveryID is derived.
		if got.DiscoveryID != MakeResourceID(ResourceTypeVM, "node-b", "200") {
			t.Fatalf("DiscoveryID = %q, want derived id", got.DiscoveryID)
		}
	})

	t.Run("empty-reason-falls-back-to-default", func(t *testing.T) {
		target := &unified.DiscoveryTarget{
			ResourceType: "system-container",
			AgentID:      "node-c",
			ResourceID:   "303",
		}
		got := DiscoveryReadinessUnavailableForTarget(target, now, "   ")
		if got.Reason != "Discovery status is not available." {
			t.Fatalf("Reason = %q, want default fallback", got.Reason)
		}
	})

	t.Run("zero-now-normalized", func(t *testing.T) {
		got := DiscoveryReadinessUnavailableForTarget(nil, time.Time{}, "")
		if got.GeneratedAt.IsZero() || got.GeneratedAt.Location() != time.UTC {
			t.Fatalf("GeneratedAt = %v, want non-zero UTC", got.GeneratedAt)
		}
	})
}

// TestDiscoveryReadinessReadFailureForTarget_Branches covers the read-failure
// projection for both nil and non-nil targets. The state must always be
// "failed" with the fixed read-failure reason.
func TestDiscoveryReadinessReadFailureForTarget_Branches(t *testing.T) {
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	t.Run("nil-target", func(t *testing.T) {
		got := DiscoveryReadinessReadFailureForTarget(nil, now)
		if got.State != unified.ResourceDiscoveryReadinessFailed {
			t.Fatalf("state = %q, want failed", got.State)
		}
		if got.Reason != "Discovery status could not be read." {
			t.Fatalf("Reason = %q, want fixed read-failure reason", got.Reason)
		}
		if got.Source != discoveryReadinessSource {
			t.Fatalf("Source = %q, want %q", got.Source, discoveryReadinessSource)
		}
		if got.ResourceType != "" || got.TargetID != "" || got.ResourceID != "" {
			t.Fatalf("base fields should be empty for nil target, got %+v", got)
		}
	})

	t.Run("non-nil-target-projects-base-fields", func(t *testing.T) {
		target := &unified.DiscoveryTarget{
			ResourceType: "pod",
			AgentID:      "cluster-x",
			ResourceID:   "default/web",
		}
		got := DiscoveryReadinessReadFailureForTarget(target, now)
		if got.State != unified.ResourceDiscoveryReadinessFailed {
			t.Fatalf("state = %q, want failed", got.State)
		}
		if got.Reason != "Discovery status could not be read." {
			t.Fatalf("Reason = %q, want fixed read-failure reason", got.Reason)
		}
		// pod maps to ResourceTypeK8s, so DiscoveryIDForTarget derives an id.
		if got.DiscoveryID != MakeResourceID(ResourceTypeK8s, "cluster-x", "default/web") {
			t.Fatalf("DiscoveryID = %q, want derived k8s id", got.DiscoveryID)
		}
		if got.ResourceType != "pod" || got.TargetID != "cluster-x" || got.ResourceID != "default/web" {
			t.Fatalf("target fields not projected: %+v", got)
		}
	})
}

// TestGetCommandCategories_Branches verifies GetCommandCategories returns a
// unique, sorted category set per resource type, including the empty-set case
// for an unknown type.
func TestGetCommandCategories_Branches(t *testing.T) {
	// Pre-compute the expected category set per resource type by walking the
	// same command list with a de-duping set + sort, independent of the
	// production sort order.
	expectedFor := func(rt ResourceType) []string {
		cmds := GetCommandsForResource(rt)
		set := make(map[string]struct{})
		for _, c := range cmds {
			for _, cat := range c.Categories {
				set[cat] = struct{}{}
			}
		}
		out := make([]string, 0, len(set))
		for k := range set {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}

	cases := []struct {
		name string
		rt   ResourceType
	}{
		{"system-container", ResourceTypeSystemContainer},
		{"vm", ResourceTypeVM},
		{"docker", ResourceTypeDocker},
		{"docker-vm", ResourceTypeDockerVM},
		{"docker-system-container", ResourceTypeDockerSystemContainer},
		{"k8s", ResourceTypeK8s},
		{"agent", ResourceTypeAgent},
		{"unknown", ResourceType("does-not-exist")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GetCommandCategories(tc.rt)
			want := expectedFor(tc.rt)

			if len(got) != len(want) {
				t.Fatalf("category count = %d, want %d (got=%v)", len(got), len(want), got)
			}
			// Assert each element matches (sorted order) — catches both
			// membership and ordering bugs.
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("category[%d] = %q, want %q (got=%v want=%v)",
						i, got[i], want[i], got, want)
				}
			}
			// Result must be sorted ascending.
			if !sort.StringsAreSorted(got) {
				t.Fatalf("categories not sorted: %v", got)
			}
			// Result must be unique.
			seen := make(map[string]bool, len(got))
			for _, c := range got {
				if seen[c] {
					t.Fatalf("duplicate category %q in %v", c, got)
				}
				seen[c] = true
			}
		})
	}

	// Explicit invariant: unknown type yields a non-nil empty slice so callers
	// can safely iterate.
	if cats := GetCommandCategories(ResourceType("nope")); len(cats) != 0 {
		t.Fatalf("unknown resource type should yield no categories, got %v", cats)
	}

	// Cross-check a concrete known category against the agent (host) set so
	// the test does not purely mirror production code — "version" must appear
	// because getHostCommands includes os_release/proxmox_version commands.
	agentCats := GetCommandCategories(ResourceTypeAgent)
	if !containsString(agentCats, "version") {
		t.Fatalf("agent categories missing expected 'version': %v", agentCats)
	}
	if !containsString(agentCats, "hardware") {
		t.Fatalf("agent categories missing expected 'hardware': %v", agentCats)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// TestIsSchemaOutdated_Branches covers the three meaningful arms of the
// schema-version comparison: older (true), current (false), and future
// (false — forward-compatible).
func TestIsSchemaOutdated_Branches(t *testing.T) {
	t.Run("zero-schema-outdated", func(t *testing.T) {
		fp := &ContainerFingerprint{SchemaVersion: 0}
		if !fp.IsSchemaOutdated() {
			t.Fatal("SchemaVersion=0 should be outdated")
		}
	})

	t.Run("older-schema-outdated", func(t *testing.T) {
		fp := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion - 1}
		if !fp.IsSchemaOutdated() {
			t.Fatalf("SchemaVersion=%d (current-1) should be outdated", FingerprintSchemaVersion-1)
		}
	})

	t.Run("current-schema-not-outdated", func(t *testing.T) {
		fp := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion}
		if fp.IsSchemaOutdated() {
			t.Fatalf("SchemaVersion=%d == current should not be outdated", FingerprintSchemaVersion)
		}
	})

	t.Run("future-schema-not-outdated", func(t *testing.T) {
		fp := &ContainerFingerprint{SchemaVersion: FingerprintSchemaVersion + 1}
		if fp.IsSchemaOutdated() {
			t.Fatalf("SchemaVersion=%d (future) should not be outdated", FingerprintSchemaVersion+1)
		}
	})
}

// TestNormalizeDeepScanTimeout_Branches verifies the clamp/return behavior for
// positive, zero, and negative inputs.
func TestNormalizeDeepScanTimeout_Branches(t *testing.T) {
	t.Run("positive-returned-as-is", func(t *testing.T) {
		in := 90 * time.Second
		if got := normalizeDeepScanTimeout(in); got != in {
			t.Fatalf("normalizeDeepScanTimeout(%v) = %v, want %v", in, got, in)
		}
	})

	t.Run("positive-small-value-returned-as-is", func(t *testing.T) {
		// Positive but below the default is still honored — only non-positive
		// triggers the default.
		in := 1 * time.Millisecond
		if got := normalizeDeepScanTimeout(in); got != in {
			t.Fatalf("normalizeDeepScanTimeout(%v) = %v, want %v", in, got, in)
		}
	})

	t.Run("zero-falls-back-to-default", func(t *testing.T) {
		if got := normalizeDeepScanTimeout(0); got != defaultDiscoveryScanTimeout {
			t.Fatalf("normalizeDeepScanTimeout(0) = %v, want default %v",
				got, defaultDiscoveryScanTimeout)
		}
	})

	t.Run("negative-falls-back-to-default", func(t *testing.T) {
		in := -5 * time.Second
		if got := normalizeDeepScanTimeout(in); got != defaultDiscoveryScanTimeout {
			t.Fatalf("normalizeDeepScanTimeout(%v) = %v, want default %v",
				in, got, defaultDiscoveryScanTimeout)
		}
	})
}

// TestFormatURLSuggestionDiagnostic_Branches covers each combination of
// primary/fallback code/detail, including the all-empty sentinel return.
func TestFormatURLSuggestionDiagnostic_Branches(t *testing.T) {
	cases := []struct {
		name           string
		primaryCode    string
		primaryDetail  string
		fallbackCode   string
		fallbackDetail string
		want           string
		wantSubstrings []string
	}{
		{
			name: "all-empty-sentinel",
			want: "no suggestion diagnostics available",
		},
		{
			name:          "primary-only-with-detail",
			primaryCode:   "port_match",
			primaryDetail: "matched 8123/tcp",
			want:          "primary=port_match (matched 8123/tcp)",
		},
		{
			name:        "primary-only-no-detail",
			primaryCode: "port_match",
			want:        "primary=port_match",
		},
		{
			name:           "fallback-only-with-detail",
			fallbackCode:   "service_type_default",
			fallbackDetail: "default for homeassistant",
			want:           "fallback=service_type_default (default for homeassistant)",
		},
		{
			name:         "fallback-only-no-detail",
			fallbackCode: "service_type_default",
			want:         "fallback=service_type_default",
		},
		{
			name:           "both-with-details-joined-by-semicolon",
			primaryCode:    "port_match",
			primaryDetail:  "8123/tcp",
			fallbackCode:   "service_type_default",
			fallbackDetail: "homeassistant",
			want:           "primary=port_match (8123/tcp); fallback=service_type_default (homeassistant)",
		},
		{
			name:         "both-without-details",
			primaryCode:  "port_match",
			fallbackCode: "service_type_default",
			want:         "primary=port_match; fallback=service_type_default",
		},
		{
			name:         "primary-empty-fallback-present",
			fallbackCode: "service_type_default",
			want:         "fallback=service_type_default",
		},
		{
			name:           "primary-empty-fallback-with-detail",
			fallbackCode:   "service_type_default",
			fallbackDetail: "x",
			want:           "fallback=service_type_default (x)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatURLSuggestionDiagnostic(
				tc.primaryCode, tc.primaryDetail,
				tc.fallbackCode, tc.fallbackDetail,
			)
			if tc.want != "" && got != tc.want {
				t.Fatalf("formatURLSuggestionDiagnostic(...) = %q, want %q",
					got, tc.want)
			}
			for _, sub := range tc.wantSubstrings {
				if !strings.Contains(got, sub) {
					t.Fatalf("expected substring %q in %q", sub, got)
				}
			}
		})
	}

	// Explicit invariant: primary must always precede fallback when both set.
	both := formatURLSuggestionDiagnostic("p_code", "p_detail", "f_code", "f_detail")
	if !strings.HasPrefix(both, "primary=") {
		t.Fatalf("expected primary= prefix, got %q", both)
	}
	if !strings.Contains(both, "; fallback=") {
		t.Fatalf("expected '; fallback=' separator, got %q", both)
	}
}
