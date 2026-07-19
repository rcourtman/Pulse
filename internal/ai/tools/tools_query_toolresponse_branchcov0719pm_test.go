package tools

import (
	"testing"
)

// This test file raises branch/function coverage on the two pure
// ToToolResponse() methods declared in tools_query.go:
//   - (*ErrStrictResolution).ToToolResponse()  (tools_query.go:48)
//   - (*ErrRoutingMismatch).ToToolResponse()   (tools_query.go:93)
//
// It deliberately exercises both arms of the conditional inside
// ErrRoutingMismatch.ToToolResponse() (MoreSpecificIDs populated vs empty),
// since the existing strict_resolution_test.go only hits the empty branch.

// TestErrStrictResolutionToToolResponse_BranchCov0719pm drives the single
// return path of (*ErrStrictResolution).ToToolResponse() and asserts the
// returned ToolResponse carries the STRICT_RESOLUTION code, the original
// human-readable message, the blocked flag, and the exact metadata map the
// policy contract publishes (resource_id / action / policy_boundary).
func TestErrStrictResolutionToToolResponse_BranchCov0719pm(t *testing.T) {
	const (
		wantResourceID = "vm:101"
		wantAction     = "restart"
		wantMessage    = "resource 'vm:101' has not been discovered; discovery is required before restart"
		wantPolicy     = "Resource discovery is required before resource-specific actions."
	)

	err := &ErrStrictResolution{
		ResourceID: wantResourceID,
		Action:     wantAction,
		Message:    wantMessage,
	}

	resp := err.ToToolResponse()

	if resp.OK {
		t.Fatalf("ToToolResponse().OK = true; blocked operations must report OK=false")
	}
	if resp.Error == nil {
		t.Fatalf("ToToolResponse().Error = nil; STRICT_RESOLUTION must populate Error envelope")
	}
	if resp.Error.Code != ErrCodeStrictResolution {
		t.Fatalf("Error.Code = %q, want %q", resp.Error.Code, ErrCodeStrictResolution)
	}
	if !resp.Error.Blocked {
		t.Errorf("Error.Blocked = false, want true (STRICT_RESOLUTION is a policy block)")
	}
	if resp.Error.Failed {
		t.Errorf("Error.Failed = true, want false (block, not execution failure)")
	}
	if resp.Error.Message != wantMessage {
		t.Errorf("Error.Message = %q, want %q", resp.Error.Message, wantMessage)
	}

	details := resp.Error.Details
	if details == nil {
		t.Fatalf("Error.Details = nil; expected populated policy metadata")
	}

	if got, ok := details["resource_id"].(string); !ok || got != wantResourceID {
		t.Errorf("Details[resource_id] = %v, want %q", details["resource_id"], wantResourceID)
	}
	if got, ok := details["action"].(string); !ok || got != wantAction {
		t.Errorf("Details[action] = %v, want %q", details["action"], wantAction)
	}
	if got, ok := details["policy_boundary"].(string); !ok || got != wantPolicy {
		t.Errorf("Details[policy_boundary] = %v, want %q", details["policy_boundary"], wantPolicy)
	}

	if _, present := details["more_specific_resources"]; present {
		t.Errorf("Details unexpectedly carries more_specific_resources (strict-resolution payload)")
	}
}

// TestErrRoutingMismatchToToolResponse_BranchCov0719pm is table-driven over
// both branches of the `len(e.MoreSpecificIDs) > 0` conditional in
// (*ErrRoutingMismatch).ToToolResponse(): the canonical-ID arm (which adds
// the more_specific_resource_ids fact) and the empty arm (which omits it).
// Each case asserts the ROUTING_MISMATCH code, the blocked flag, the echoed
// message, and the precise set of metadata keys/values the function emits.
func TestErrRoutingMismatchToToolResponse_BranchCov0719pm(t *testing.T) {
	const wantPolicy = "A more specific child resource exists on the requested host; choose the intended resource before retrying a scoped action."

	type expect struct {
		name                       string
		targetHost                 string
		moreSpecificResources      []string
		moreSpecificIDs            []string
		childKinds                 []string
		message                    string
		wantIDsKey                 bool // true → expects more_specific_resource_ids
		wantIDsValue               []string
		wantResourcesAssertionType bool // true → assert Details[more_specific_resources] type-asserts as []string
	}

	cases := []expect{
		{
			name:                       "with_canonical_ids_populates_more_specific_resource_ids",
			targetHost:                 "pve-node",
			moreSpecificResources:      []string{"homepage-docker", "jellyfin"},
			moreSpecificIDs:            []string{"system-container:proxmox:141", "vm:proxmox:100"},
			childKinds:                 []string{"system-container", "vm"},
			message:                    "target_host 'pve-node' has more specific children: [homepage-docker jellyfin]",
			wantIDsKey:                 true,
			wantIDsValue:               []string{"system-container:proxmox:141", "vm:proxmox:100"},
			wantResourcesAssertionType: true,
		},
		{
			name:                       "without_canonical_ids_omits_more_specific_resource_ids",
			targetHost:                 "pve-node",
			moreSpecificResources:      []string{"homepage-docker"},
			moreSpecificIDs:            nil, // forces the else branch
			childKinds:                 nil,
			message:                    "target_host 'pve-node' has more specific children: [homepage-docker]",
			wantIDsKey:                 false,
			wantResourcesAssertionType: true,
		},
		{
			name:                       "empty_canonical_ids_slice_also_takes_else_branch",
			targetHost:                 "prox97",
			moreSpecificResources:      []string{},
			moreSpecificIDs:            []string{},
			childKinds:                 []string{},
			message:                    "target_host 'prox97' has more specific children: []",
			wantIDsKey:                 false,
			wantResourcesAssertionType: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := &ErrRoutingMismatch{
				TargetHost:            tc.targetHost,
				MoreSpecificResources: tc.moreSpecificResources,
				MoreSpecificIDs:       tc.moreSpecificIDs,
				ChildKinds:            tc.childKinds,
				Message:               tc.message,
			}

			resp := err.ToToolResponse()

			if resp.OK {
				t.Fatalf("ToToolResponse().OK = true; routing mismatch must report OK=false")
			}
			if resp.Error == nil {
				t.Fatalf("ToToolResponse().Error = nil; ROUTING_MISMATCH must populate Error envelope")
			}
			if resp.Error.Code != ErrCodeRoutingMismatch {
				t.Fatalf("Error.Code = %q, want %q", resp.Error.Code, ErrCodeRoutingMismatch)
			}
			if !resp.Error.Blocked {
				t.Errorf("Error.Blocked = false, want true (ROUTING_MISMATCH is a policy block)")
			}
			if resp.Error.Failed {
				t.Errorf("Error.Failed = true, want false (block, not execution failure)")
			}
			if resp.Error.Message != tc.message {
				t.Errorf("Error.Message = %q, want %q", resp.Error.Message, tc.message)
			}

			details := resp.Error.Details
			if details == nil {
				t.Fatalf("Error.Details = nil; expected populated routing metadata")
			}

			if got, ok := details["target_host"].(string); !ok || got != tc.targetHost {
				t.Errorf("Details[target_host] = %v, want %q", details["target_host"], tc.targetHost)
			}

			if tc.wantResourcesAssertionType {
				resources, ok := details["more_specific_resources"].([]string)
				if !ok {
					t.Fatalf("Details[more_specific_resources] type = %T, want []string", details["more_specific_resources"])
				}
				if len(resources) != len(tc.moreSpecificResources) {
					t.Fatalf("Details[more_specific_resources] = %v, want %v", resources, tc.moreSpecificResources)
				}
				for i := range resources {
					if resources[i] != tc.moreSpecificResources[i] {
						t.Errorf("Details[more_specific_resources][%d] = %q, want %q", i, resources[i], tc.moreSpecificResources[i])
					}
				}
			}

			idsValue, idsPresent := details["more_specific_resource_ids"]
			if tc.wantIDsKey {
				if !idsPresent {
					t.Fatalf("Details[more_specific_resource_ids] missing; expected to be present when MoreSpecificIDs is non-empty")
				}
				idsSlice, ok := idsValue.([]string)
				if !ok {
					t.Fatalf("Details[more_specific_resource_ids] type = %T, want []string", idsValue)
				}
				if len(idsSlice) != len(tc.wantIDsValue) {
					t.Fatalf("Details[more_specific_resource_ids] = %v, want %v", idsSlice, tc.wantIDsValue)
				}
				for i := range idsSlice {
					if idsSlice[i] != tc.wantIDsValue[i] {
						t.Errorf("Details[more_specific_resource_ids][%d] = %q, want %q", i, idsSlice[i], tc.wantIDsValue[i])
					}
				}
			} else {
				if idsPresent {
					t.Errorf("Details[more_specific_resource_ids] = %v; expected to be absent when MoreSpecificIDs is empty", idsValue)
				}
			}

			if got, ok := details["policy_boundary"].(string); !ok || got != wantPolicy {
				t.Errorf("Details[policy_boundary] = %v, want %q", details["policy_boundary"], wantPolicy)
			}

			if _, present := details["auto_recoverable"]; present {
				t.Errorf("Details[auto_recoverable] present; routing mismatch must not surface auto-recovery hint")
			}
		})
	}
}
