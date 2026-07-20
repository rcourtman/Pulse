package unifiedresources

import (
	"strings"
	"testing"
)

// Branch-coverage tests for CanonicalGovernanceMetadata in policy_metadata.go.
//
// The function has two top-level branches:
//
//   - nil-resource guard: returns (nil, "")
//   - non-nil projection: shallow-copies the input, refreshes policy metadata
//     on the copy (which always (re)assigns a non-nil derived Policy and a
//     rebuilt AISafeSummary), and returns a deep-cloned policy plus the
//     TrimSpace'd summary.
//
// Within the non-nil arm the input may arrive with Policy present or absent;
// RefreshPolicyMetadata overwrites either way, so the returned policy must
// always reflect the DERIVED classification (not any pre-existing Policy on
// the input), and the caller's Resource must not be mutated by the projection.
//
// Cases drive each branch and assert BOTH return values, the input-immunity
// contract, and the clone-independence contract.

func TestBranchcov0720am_CanonicalGovernanceMetadata(t *testing.T) {
	t.Parallel()

	// preExistingPolicy is intentionally misaligned with what classification
	// will derive, so we can prove the present-policy arm overwrites rather
	// than trusts the caller's Policy.
	preExistingPolicy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityPublic,
		Routing: ResourceRoutingPolicy{
			Scope:  ResourceRoutingScopeCloudSummary,
			Redact: []ResourceRedactionHint{ResourceRedactionHostname},
		},
	}

	type expect struct {
		nilPolicy         bool
		emptySummary      bool
		sensitivity       ResourceSensitivity
		routingScope      ResourceRoutingScope
		summaryContains   string
		summaryTrimmed    bool
		summaryNotContain string
	}

	cases := []struct {
		name     string
		resource *Resource
		want     expect
	}{
		{
			// nil-resource arm: returns (nil, "").
			name:     "nil resource returns nil policy and empty summary",
			resource: nil,
			want: expect{
				nilPolicy:    true,
				emptySummary: true,
			},
		},
		{
			// Non-nil arm, ABSENT policy: a plain compute workload (VM) has no
			// Policy set, so Refresh must derive Internal / cloud-summary.
			name: "absent policy derives internal cloud-summary posture",
			resource: &Resource{
				ID:     "vm-100",
				Name:   "web-01",
				Type:   ResourceTypeVM,
				Status: StatusOnline,
			},
			want: expect{
				sensitivity:     ResourceSensitivityInternal,
				routingScope:    ResourceRoutingScopeCloudSummary,
				summaryContains: "virtual machine resource",
				summaryTrimmed:  true,
			},
		},
		{
			// Non-nil arm, ABSENT policy but a restricted tag drives a
			// different derived branch (LocalOnly / restricted).
			name: "absent policy with pii tag derives restricted local-only",
			resource: &Resource{
				ID:     "vm-200",
				Name:   "payments-db",
				Type:   ResourceTypeVM,
				Status: StatusOnline,
				Tags:   []string{"pii"},
			},
			want: expect{
				sensitivity:     ResourceSensitivityRestricted,
				routingScope:    ResourceRoutingScopeLocalOnly,
				summaryContains: "local-only context",
				summaryTrimmed:  true,
			},
		},
		{
			// Non-nil arm, PRESENT policy: the caller has already set a Policy
			// (Public). Refresh overwrites it, so the returned policy must
			// reflect the derived Internal classification for a plain VM,
			// NOT the pre-existing Public.
			name: "present policy is overwritten by derived classification",
			resource: &Resource{
				ID:     "agent-9",
				Name:   "pve-node",
				Type:   ResourceTypeAgent,
				Status: StatusOnline,
				Policy: preExistingPolicy,
			},
			want: expect{
				sensitivity:     ResourceSensitivityInternal,
				routingScope:    ResourceRoutingScopeCloudSummary,
				summaryContains: "agent resource",
				summaryTrimmed:  true,
				// The misaligned pre-existing sensitivity must not leak into
				// the summary text.
				summaryNotContain: "public",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			policy, summary := CanonicalGovernanceMetadata(tc.resource)

			// --- Both return values, per branch. ---
			if tc.want.nilPolicy {
				if policy != nil {
					t.Fatalf("nil-resource arm: policy = %#v, want nil", policy)
				}
				if !tc.want.emptySummary {
					t.Fatalf("test setup error: nilPolicy implies emptySummary")
				}
				if summary != "" {
					t.Fatalf("nil-resource arm: summary = %q, want empty", summary)
				}
				return
			}

			if policy == nil {
				t.Fatal("non-nil arm: expected non-nil policy")
			}
			if tc.want.emptySummary && summary != "" {
				t.Fatalf("summary = %q, want empty", summary)
			}

			// Returned policy reflects the DERIVED classification.
			if got := policy.Sensitivity; got != tc.want.sensitivity {
				t.Errorf("Sensitivity = %q, want %q", got, tc.want.sensitivity)
			}
			if got := policy.Routing.Scope; got != tc.want.routingScope {
				t.Errorf("Routing.Scope = %q, want %q", got, tc.want.routingScope)
			}

			// Summary content + trim contract.
			if tc.want.summaryContains != "" && !strings.Contains(summary, tc.want.summaryContains) {
				t.Errorf("summary = %q, want substring %q", summary, tc.want.summaryContains)
			}
			if tc.want.summaryNotContain != "" && strings.Contains(strings.ToLower(summary), tc.want.summaryNotContain) {
				t.Errorf("summary = %q, must not contain %q", summary, tc.want.summaryNotContain)
			}
			if tc.want.summaryTrimmed {
				if summary != strings.TrimSpace(summary) {
					t.Errorf("summary = %q, must have no surrounding whitespace", summary)
				}
			}
		})
	}

	// Verify the table's present-policy case actually used a non-nil Policy
	// input (guards against accidental test-setup regressions that would turn
	// it into a duplicate of the absent-policy case).
	if preExistingPolicy == nil {
		t.Fatal("test setup error: preExistingPolicy must be non-nil")
	}
}

// TestBranchcov0720am_CanonicalGovernanceMetadata_InputNotMutated asserts the
// projection never writes back to the caller's Resource: neither the Policy
// pointer, nor the AISafeSummary, nor any pre-existing Policy Redact slice is
// touched. This is the central behavioral contract of a "canonical read".
func TestBranchcov0720am_CanonicalGovernanceMetadata_InputNotMutated(t *testing.T) {
	t.Parallel()

	originalPolicy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityPublic,
		Routing: ResourceRoutingPolicy{
			Scope: ResourceRoutingScopeCloudSummary,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionIPAddress,
			},
		},
	}
	originalRedactLen := len(originalPolicy.Routing.Redact)
	originalSummary := "  pre-existing summary  "

	resource := &Resource{
		ID:            "vm-300",
		Name:          "cache-01",
		Type:          ResourceTypeVM,
		Status:        StatusOnline,
		Policy:        originalPolicy,
		AISafeSummary: originalSummary,
	}

	policy, summary := CanonicalGovernanceMetadata(resource)

	// The caller's Resource.Policy pointer is unchanged.
	if resource.Policy != originalPolicy {
		t.Fatalf("input Resource.Policy pointer was mutated: got %p, want %p", resource.Policy, originalPolicy)
	}
	// The caller's pre-existing Policy content is unchanged.
	if resource.Policy.Sensitivity != ResourceSensitivityPublic {
		t.Errorf("input Policy.Sensitivity mutated: got %q, want public", resource.Policy.Sensitivity)
	}
	if resource.Policy.Routing.Scope != ResourceRoutingScopeCloudSummary {
		t.Errorf("input Policy.Routing.Scope mutated: got %q, want cloud-summary", resource.Policy.Routing.Scope)
	}
	if len(resource.Policy.Routing.Redact) != originalRedactLen {
		t.Errorf("input Policy.Routing.Redact len mutated: got %d, want %d", len(resource.Policy.Routing.Redact), originalRedactLen)
	}
	// The caller's AISafeSummary is unchanged (Refresh writes only to the copy).
	if resource.AISafeSummary != originalSummary {
		t.Errorf("input AISafeSummary mutated: got %q, want %q", resource.AISafeSummary, originalSummary)
	}

	// And the returned policy is NOT the same object as the input's policy
	// (it must be a freshly derived + cloned value), while the returned
	// summary differs from the raw input summary (it was rebuilt + trimmed).
	if policy == originalPolicy {
		t.Fatal("returned policy is the same pointer as input Policy; expected a derived clone")
	}
	if summary == originalSummary {
		t.Fatalf("returned summary equals raw input %q; expected a rebuilt trimmed summary", summary)
	}
}

// TestBranchcov0720am_CanonicalGovernanceMetadata_CloneIndependence asserts the
// returned policy's Redact slice is an independent allocation: mutating it must
// not affect the input resource's pre-existing Policy (deep-copy guarantee).
func TestBranchcov0720am_CanonicalGovernanceMetadata_CloneIndependence(t *testing.T) {
	t.Parallel()

	originalPolicy := &ResourcePolicy{
		Sensitivity: ResourceSensitivitySensitive,
		Routing: ResourceRoutingPolicy{
			Scope: ResourceRoutingScopeLocalFirst,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionIPAddress,
			},
		},
	}
	// A storage-typed resource with a filesystem path derives a Sensitive
	// posture that includes a Path redaction hint, so the returned Redact
	// slice is guaranteed non-empty and safe to mutate.
	resource := &Resource{
		ID:     "storage-zfs-1",
		Name:   "tank",
		Type:   ResourceTypeStorage,
		Status: StatusOnline,
		Storage: &StorageMeta{
			Path: "/mnt/tank",
		},
		Policy: originalPolicy,
	}

	policy, _ := CanonicalGovernanceMetadata(resource)

	if policy == nil {
		t.Fatal("expected non-nil policy")
	}
	if policy.Sensitivity != ResourceSensitivitySensitive {
		t.Fatalf("Sensitivity = %q, want sensitive", policy.Sensitivity)
	}
	if len(policy.Routing.Redact) == 0 {
		t.Fatalf("expected non-empty Redact for sensitive storage resource, got %#v", policy.Routing.Redact)
	}

	// Mutate every element of the returned Redact slice. The input resource's
	// pre-existing Policy Redact must remain pristine.
	for i := range policy.Routing.Redact {
		policy.Routing.Redact[i] = ResourceRedactionHint("MUTATED")
	}
	for _, got := range resource.Policy.Routing.Redact {
		if strings.Contains(string(got), "MUTATED") {
			t.Errorf("input Policy Redact mutated via returned clone: resource slice now contains %q", got)
		}
	}

	// Also confirm the input's original Redact content is intact.
	if len(resource.Policy.Routing.Redact) != 2 {
		t.Errorf("input Policy Redact len changed: got %d, want 2", len(resource.Policy.Routing.Redact))
	}
	if resource.Policy.Routing.Redact[0] != ResourceRedactionHostname ||
		resource.Policy.Routing.Redact[1] != ResourceRedactionIPAddress {
		t.Errorf("input Policy Redact content changed: got %#v, want [hostname ip-address]", resource.Policy.Routing.Redact)
	}
}
