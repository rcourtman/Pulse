package unifiedresources

import "testing"

// Branch-coverage tests for four currently-uncovered UNEXPORTED helpers in
// registry.go. Each target function is pure (no registry state, no I/O) so
// the tests exercise them directly with hand-computed expected values that
// pin the exact composition/format produced by the source.
//
//   - seededVMwareSourceID — nil-guards + trimmed-ManagedObjectID empty guard
//     + EntityType fallback to CanonicalResourceType(resource.Type) + optional
//     ConnectionID prefix; final shape is ":"-joined.
//   - proxmoxGuestFallbackSourceID — builds a non-blank-trimmed slice in
//     (kind, instance, node) order, ALWAYS appends fmt.Sprintf("%d", vmid),
//     then ":"-joins.
//   - isDockerNetworkAttachmentRelationship — Type==RelAttachedTo &&
//     TrimSpace(Discoverer)==dockerAdapterRelationshipDiscoverer.
//   - physicalDiskTopologyCompatible — nil-guards + EqualFold checks for
//     Controller and Target (each check skipped when either side is blank).
//
// Expected values are derived by hand from the source of each function in
// registry.go (see line references inline). No assertion is a "no panic"
// assertion: each subtest pins a specific string or boolean computed from
// the documented composition rules.

func TestBranchcov0721amRegistrySourceID(t *testing.T) {
	t.Run("seededVMwareSourceID", func(t *testing.T) {
		t.Parallel()

		// Each row drives one branch of seededVMwareSourceID
		// (registry.go:1122). The ":"-joined composition is:
		//   [connectionID?] + [entityType?] + managedObjectID
		// where connectionID is included only when the trimmed
		// VMware.ConnectionID is non-empty, and entityType is either the
		// trimmed VMware.EntityType or — when that is blank — the
		// CanonicalResourceType(resource.Type) (i.e.
		// ToLower(TrimSpace(string(resource.Type)))).
		cases := []struct {
			name           string
			resource       *Resource
			want           string
			skipEntityType bool // documents that entityType is intentionally absent
		}{
			{
				name:     "nil_resource_returns_empty",
				resource: nil,
				want:     "",
			},
			{
				name:     "nil_VMware_payload_returns_empty",
				resource: &Resource{Type: ResourceTypeVM},
				want:     "",
			},
			{
				name: "empty_managed_object_id_returns_empty",
				resource: &Resource{
					Type:   ResourceTypeVM,
					VMware: &VMwareData{ManagedObjectID: "", EntityType: "host"},
				},
				want: "",
			},
			{
				name: "whitespace_only_managed_object_id_returns_empty",
				resource: &Resource{
					Type:   ResourceTypeVM,
					VMware: &VMwareData{ManagedObjectID: "   \t  ", EntityType: "host"},
				},
				want: "",
			},
			{
				name: "entity_type_present_no_connection_id",
				resource: &Resource{
					Type:   ResourceTypeVM,
					VMware: &VMwareData{ManagedObjectID: "mo-1", EntityType: "host"},
				},
				want: "host:mo-1",
			},
			{
				name: "entity_type_present_with_connection_id_prefix",
				resource: &Resource{
					Type:   ResourceTypeVM,
					VMware: &VMwareData{ConnectionID: "conn-1", ManagedObjectID: "mo-1", EntityType: "host"},
				},
				want: "conn-1:host:mo-1",
			},
			{
				// EntityType blank → CanonicalResourceType("vm") = "vm".
				name: "blank_entity_type_falls_back_to_canonical_resource_type_vm",
				resource: &Resource{
					Type:   ResourceTypeVM,
					VMware: &VMwareData{ManagedObjectID: "mo-1", EntityType: ""},
				},
				want: "vm:mo-1",
			},
			{
				// CanonicalResourceType = ToLower(TrimSpace("  VM  ")) = "vm".
				name: "blank_entity_type_with_uppercase_whitespace_type_normalizes_to_vm",
				resource: &Resource{
					Type:   ResourceType("  VM  "),
					VMware: &VMwareData{ManagedObjectID: "mo-1", EntityType: "   "},
				},
				want: "vm:mo-1",
			},
			{
				// CanonicalResourceType("VirtualMachine") = "virtualmachine".
				name: "blank_entity_type_with_long_type_name_normalizes_to_virtualmachine",
				resource: &Resource{
					Type:   ResourceType("VirtualMachine"),
					VMware: &VMwareData{ManagedObjectID: "mo-9", EntityType: ""},
				},
				want: "virtualmachine:mo-9",
			},
			{
				// EntityType blank AND resource.Type empty → CanonicalResourceType
				// returns "" → the entityType slot is omitted entirely.
				name: "blank_entity_type_and_blank_type_omits_entity_type_slot",
				resource: &Resource{
					Type:   ResourceType(""),
					VMware: &VMwareData{ManagedObjectID: "mo-2", EntityType: ""},
				},
				want:           "mo-2",
				skipEntityType: true,
			},
			{
				// TrimSpace applied to all three composed fields:
				//   ConnectionID "  conn-1  " → "conn-1"
				//   EntityType   "  host  "   → "host"
				//   ManagedObjectID "  mo-1  " → "mo-1"
				name: "all_composed_fields_are_trimmed",
				resource: &Resource{
					Type: ResourceTypeVM,
					VMware: &VMwareData{
						ConnectionID:    "  conn-1  ",
						ManagedObjectID: "  mo-1  ",
						EntityType:      "  host  ",
					},
				},
				want: "conn-1:host:mo-1",
			},
			{
				// ConnectionID whitespace-only is dropped → no prefix.
				name: "whitespace_only_connection_id_is_dropped",
				resource: &Resource{
					Type: ResourceTypeVM,
					VMware: &VMwareData{
						ConnectionID:    "   ",
						ManagedObjectID: "mo-1",
						EntityType:      "host",
					},
				},
				want: "host:mo-1",
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := seededVMwareSourceID(tc.resource)
				if got != tc.want {
					t.Fatalf("seededVMwareSourceID(%+v) = %q, want %q", tc.resource, got, tc.want)
				}
			})
		}
	})

	t.Run("proxmoxGuestFallbackSourceID", func(t *testing.T) {
		t.Parallel()

		// Each row drives one of the three conditional appends in
		// proxmoxGuestFallbackSourceID (registry.go:4177). The final
		// element (Sprintf("%d", vmid)) is ALWAYS appended.
		cases := []struct {
			name     string
			kind     string
			instance string
			node     string
			vmid     int
			want     string
		}{
			{
				name:     "all_components_present",
				kind:     "qemu",
				instance: "inst-1",
				node:     "node-1",
				vmid:     100,
				want:     "qemu:inst-1:node-1:100",
			},
			{
				name:     "kind_blank_dropped",
				kind:     "",
				instance: "inst-1",
				node:     "node-1",
				vmid:     101,
				want:     "inst-1:node-1:101",
			},
			{
				name:     "instance_blank_dropped",
				kind:     "qemu",
				instance: "",
				node:     "node-1",
				vmid:     102,
				want:     "qemu:node-1:102",
			},
			{
				name:     "node_blank_dropped",
				kind:     "qemu",
				instance: "inst-1",
				node:     "",
				vmid:     103,
				want:     "qemu:inst-1:103",
			},
			{
				name:     "all_three_blank_yields_just_vmid",
				kind:     "",
				instance: "",
				node:     "",
				vmid:     104,
				want:     "104",
			},
			{
				name:     "whitespace_only_components_are_dropped",
				kind:     "   ",
				instance: "\t",
				node:     "  ",
				vmid:     105,
				want:     "105",
			},
			{
				name:     "components_are_trimmed_before_joining",
				kind:     "  qemu  ",
				instance: "  inst-1  ",
				node:     "  node-1  ",
				vmid:     200,
				want:     "qemu:inst-1:node-1:200",
			},
			{
				// vmid is rendered with %d, so a zero value still appears.
				name: "zero_vmid_is_rendered",
				kind: "qemu", instance: "i", node: "n", vmid: 0,
				want: "qemu:i:n:0",
			},
			{
				// Negative vmid keeps its sign under %d formatting.
				name: "negative_vmid_keeps_sign",
				kind: "qemu", instance: "i", node: "n", vmid: -7,
				want: "qemu:i:n:-7",
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := proxmoxGuestFallbackSourceID(tc.kind, tc.instance, tc.node, tc.vmid)
				if got != tc.want {
					t.Fatalf("proxmoxGuestFallbackSourceID(%q, %q, %q, %d) = %q, want %q",
						tc.kind, tc.instance, tc.node, tc.vmid, got, tc.want)
				}
			})
		}
	})

	t.Run("isDockerNetworkAttachmentRelationship", func(t *testing.T) {
		t.Parallel()

		// Each row drives one arm of the && predicate in
		// isDockerNetworkAttachmentRelationship (registry.go:1948):
		//   Type == RelAttachedTo && TrimSpace(Discoverer) == dockerAdapterRelationshipDiscoverer
		cases := []struct {
			name         string
			relationship ResourceRelationship
			want         bool
		}{
			{
				name:         "both_type_and_discoverer_match",
				relationship: ResourceRelationship{Type: RelAttachedTo, Discoverer: dockerAdapterRelationshipDiscoverer},
				want:         true,
			},
			{
				// TrimSpace is applied to the Discoverer side.
				name:         "discoverer_with_surrounding_whitespace_matches",
				relationship: ResourceRelationship{Type: RelAttachedTo, Discoverer: "  " + dockerAdapterRelationshipDiscoverer + "  "},
				want:         true,
			},
			{
				name:         "wrong_relationship_type_returns_false",
				relationship: ResourceRelationship{Type: RelRunsOn, Discoverer: dockerAdapterRelationshipDiscoverer},
				want:         false,
			},
			{
				name:         "empty_relationship_type_returns_false",
				relationship: ResourceRelationship{Type: "", Discoverer: dockerAdapterRelationshipDiscoverer},
				want:         false,
			},
			{
				name:         "wrong_discoverer_returns_false",
				relationship: ResourceRelationship{Type: RelAttachedTo, Discoverer: "pulse_inference_engine"},
				want:         false,
			},
			{
				name:         "empty_discoverer_returns_false",
				relationship: ResourceRelationship{Type: RelAttachedTo, Discoverer: ""},
				want:         false,
			},
			{
				// Whitespace-only Discoverer trims to "" which != constant.
				name:         "whitespace_only_discoverer_returns_false",
				relationship: ResourceRelationship{Type: RelAttachedTo, Discoverer: "    "},
				want:         false,
			},
			{
				name:         "both_wrong_returns_false",
				relationship: ResourceRelationship{Type: RelOwnedBy, Discoverer: "k8s_adapter"},
				want:         false,
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := isDockerNetworkAttachmentRelationship(tc.relationship)
				if got != tc.want {
					t.Fatalf("isDockerNetworkAttachmentRelationship(type=%s, discoverer=%q) = %v, want %v",
						tc.relationship.Type, tc.relationship.Discoverer, got, tc.want)
				}
			})
		}
	})

	t.Run("physicalDiskTopologyCompatible", func(t *testing.T) {
		t.Parallel()

		// Each row drives one branch of physicalDiskTopologyCompatible
		// (registry.go:2658):
		//   nil-guard OR controller-mismatch OR target-mismatch OR compatible.
		// Each comparison is skipped when EITHER side's field is blank,
		// and uses EqualFold(TrimSpace(left), TrimSpace(right)).
		sda := &PhysicalDiskMeta{Controller: "sda"}
		sdb := &PhysicalDiskMeta{Controller: "sdb"}
		sdaUpper := &PhysicalDiskMeta{Controller: "SDA"}
		sdaWithSpaces := &PhysicalDiskMeta{Controller: "  sda  "}
		controllerBlank := &PhysicalDiskMeta{Controller: ""}
		targetOne := &PhysicalDiskMeta{Target: "t1"}
		targetTwo := &PhysicalDiskMeta{Target: "t2"}
		targetOneUpper := &PhysicalDiskMeta{Target: "T1"}
		targetOneSpaces := &PhysicalDiskMeta{Target: "  t1  "}
		bothFieldsMatch := &PhysicalDiskMeta{Controller: "sda", Target: "t1"}
		bothFieldsMatchAlt := &PhysicalDiskMeta{Controller: "SDA", Target: "T1"}
		empty := &PhysicalDiskMeta{}

		cases := []struct {
			name  string
			left  *PhysicalDiskMeta
			right *PhysicalDiskMeta
			want  bool
		}{
			{name: "nil_left_returns_false", left: nil, right: sda, want: false},
			{name: "nil_right_returns_false", left: sda, right: nil, want: false},
			{name: "both_nil_returns_false", left: nil, right: nil, want: false},

			{name: "controller_mismatch_returns_false", left: sda, right: sdb, want: false},
			{name: "controller_case_insensitive_match_is_compatible", left: sda, right: sdaUpper, want: true},
			{name: "controller_match_with_inner_whitespace_is_compatible", left: sda, right: sdaWithSpaces, want: true},

			{name: "one_side_blank_controller_skips_controller_check", left: controllerBlank, right: sda, want: true},
			{name: "both_sides_blank_controller_skips_controller_check", left: controllerBlank, right: &PhysicalDiskMeta{Controller: " "}, want: true},

			{name: "target_mismatch_returns_false", left: targetOne, right: targetTwo, want: false},
			{name: "target_case_insensitive_match_is_compatible", left: targetOne, right: targetOneUpper, want: true},
			{name: "target_match_with_inner_whitespace_is_compatible", left: targetOne, right: targetOneSpaces, want: true},

			{name: "one_side_blank_target_skips_target_check", left: &PhysicalDiskMeta{Target: ""}, right: targetOne, want: true},

			{name: "controller_match_target_mismatch_returns_false", left: bothFieldsMatch, right: &PhysicalDiskMeta{Controller: "sda", Target: "t2"}, want: false},
			{name: "controller_mismatch_target_match_returns_false", left: bothFieldsMatch, right: &PhysicalDiskMeta{Controller: "sdb", Target: "t1"}, want: false},

			{name: "both_controller_and_target_match_case_insensitively", left: bothFieldsMatch, right: bothFieldsMatchAlt, want: true},
			{name: "both_sides_empty_compatible", left: empty, right: empty, want: true},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := physicalDiskTopologyCompatible(tc.left, tc.right)
				if got != tc.want {
					t.Fatalf("physicalDiskTopologyCompatible(left=%+v, right=%+v) = %v, want %v",
						tc.left, tc.right, got, tc.want)
				}
			})
		}
	})
}
