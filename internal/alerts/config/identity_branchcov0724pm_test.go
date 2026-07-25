package config

import (
	"reflect"
	"testing"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0724pm"`) for CanonicalResourceTypeKeys (identity.go:9),
// which had 34.3% coverage. The existing alerts_config_identity_branchcov0716
// test only reaches a handful of switch arms (vm, node, agent-disk, storage,
// k8s-cluster + the default/legacy-nil paths); the majority of the resource-
// type enumeration in the switch was never executed.
//
// These tests drive EVERY reachable case arm of the switch directly and assert
// the exact, ordered key slice each produces, including the multi-key ancestry
// chains (e.g. oci-container -> system-container -> guest). They also pin two
// observable invariants of the addUnique closure: ordering is append-order
// (not sorted), and each call returns a fresh, independent allocation.
//
// Conventions match sibling in-package tests in this directory (see
// validsignal_branchcov0724pm_test.go): stdlib `testing` only, table-driven
// subtests with `tc := tc`, reflect.DeepEqual assertions, no testify.
//
// Purity: CanonicalResourceTypeKeys is a pure function over its string argument;
// no network, daemon, database, or filesystem is touched.

// TestBranchcov0724pmCanonicalResourceTypeKeys exercises every reachable case
// arm of the CanonicalResourceTypeKeys switch. Inputs are the canonical
// (post-normalization) type keys, since CanonicalAlertResourceType folds any
// spaced display alias ("kubernetes cluster", "truenas disk", ...) into its
// hyphenated form before this switch is consulted.
func TestBranchcov0724pmCanonicalResourceTypeKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		// Single-key leaf types and their ancestry chains. Order is append-order,
		// which these assertions pin against accidental reordering.
		{"guest", "guest", []string{"guest"}},
		{"vm", "vm", []string{"vm", "guest"}},
		{"system-container", "system-container", []string{"system-container", "guest"}},
		{"oci-container", "oci-container", []string{"oci-container", "system-container", "guest"}},
		{"app-container", "app-container", []string{"app-container", "guest"}},
		{"docker-host", "docker-host", []string{"docker-host", "node"}},
		{"docker-service", "docker-service", []string{"docker-service", "app-container", "guest"}},
		{"node", "node", []string{"node"}},
		{"agent", "agent", []string{"agent", "node"}},
		{"agent-disk", "agent-disk", []string{"agent-disk", "agent", "storage"}},
		{"pbs", "pbs", []string{"pbs", "node"}},
		{"pmg", "pmg", []string{"pmg", "node"}},
		{"k8s-cluster", "k8s-cluster", []string{"k8s-cluster", "node"}},
		{"k8s-node", "k8s-node", []string{"k8s-node", "node"}},
		{"k8s-deployment", "k8s-deployment", []string{"k8s-deployment", "guest"}},
		{"k8s-namespace", "k8s-namespace", []string{"k8s-namespace"}},
		{"pod", "pod", []string{"pod", "guest"}},

		// TrueNAS family.
		{"truenas-system", "truenas-system", []string{"truenas-system", "agent", "node"}},
		{"truenas-pool", "truenas-pool", []string{"truenas-pool", "storage"}},
		{"truenas-dataset", "truenas-dataset", []string{"truenas-dataset", "storage"}},
		{"truenas-disk", "truenas-disk", []string{"truenas-disk", "physical_disk", "disk", "storage"}},

		// VMware family.
		{"vmware-host", "vmware-host", []string{"vmware-host", "agent", "node"}},
		{"vmware-vm", "vmware-vm", []string{"vmware-vm", "vm", "guest"}},
		{"vmware-datastore", "vmware-datastore", []string{"vmware-datastore", "storage"}},
		{"vmware-network", "vmware-network", []string{"vmware-network", "network"}},

		// Generic storage-shaped types.
		{"storage", "storage", []string{"storage"}},
		{"disk", "disk", []string{"disk", "storage"}},
		{"datastore", "datastore", []string{"datastore", "storage", "pbs"}},
		{"pool", "pool", []string{"pool", "storage"}},
		{"dataset", "dataset", []string{"dataset", "storage"}},
		{"ceph", "ceph", []string{"ceph", "storage"}},
		{"physical_disk", "physical_disk", []string{"physical_disk", "disk", "storage"}},

		// Unknown type -> default arm: the type key is the sole element.
		{"unknown custom type default passthrough", "widget", []string{"widget"}},
		{"unknown hyphenated type default passthrough", "foo-bar-baz", []string{"foo-bar-baz"}},

		// Empty / blank -> early nil return (typeKey == "" branch). Asserts a
		// true nil rather than an empty non-nil slice.
		{"empty string returns nil", "", nil},
		{"whitespace only returns nil", "   ", nil},

		// Legacy unsupported types that survive CanonicalAlertResourceType
		// unchanged but are rejected by isUnsupportedLegacyAlertResourceType ->
		// early nil return. These reach the second operand of the early-return
		// conjunction (the local switch arms).
		{"legacy host returns nil", "host", nil},
		{"legacy qemu returns nil", "qemu", nil},
		{"legacy lxc returns nil", "lxc", nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CanonicalResourceTypeKeys(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("CanonicalResourceTypeKeys(%q) = %v, want %v", tc.in, got, tc.want)
			}
			// A nil want must be a TRUE nil slice, not a non-nil empty one: the
			// function's contract is `return nil`.
			if tc.want == nil && got != nil {
				t.Fatalf("CanonicalResourceTypeKeys(%q) = %#v, want true nil", tc.in, got)
			}
		})
	}
}

// TestBranchcov0724pmCanonicalResourceTypeKeysIndependentCopy proves each call
// returns a fresh, independent allocation: mutating one result must not affect a
// subsequent call's result (no shared backing array).
func TestBranchcov0724pmCanonicalResourceTypeKeysIndependentCopy(t *testing.T) {
	first := CanonicalResourceTypeKeys("vm")
	if len(first) != 2 {
		t.Fatalf("expected two keys for vm, got %v", first)
	}
	// Mutate the returned slice in place and via append.
	first[0] = "MUTATED"
	first = append(first, "EXTRA")

	second := CanonicalResourceTypeKeys("vm")
	want := []string{"vm", "guest"}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("second call returned %v, want %v (result must be an independent copy)", second, want)
	}
}
