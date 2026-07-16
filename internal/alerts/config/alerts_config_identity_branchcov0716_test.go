package config_test

import (
	"fmt"
	"reflect"
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// This file adds branch-coverage tests for the identity-normalization helpers
// in internal/alerts/config/identity.go:
//   - CanonicalAlertResourceType (previously only exercised indirectly via the
//     alerts facade wrapper; its switch arms and trim/lowercase normalization
//     are covered directly here).
//   - CanonicalResourceTypeKeys (legacy-alias rejection paths returning a true
//     nil, case-insensitivity, and equivalence between display aliases and
//     their canonical hyphenated forms).

// TestBranchCovCanonicalAlertResourceType exercises every switch arm of
// CanonicalAlertResourceType directly, including the whitespace trimming and
// case-folding performed before the switch, each multi-token display alias,
// every vsphere/virtual-machine variant, and the default passthrough.
func TestBranchCovCanonicalAlertResourceType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Normalization: leading/trailing whitespace is trimmed and the value
		// is lower-cased before the switch is consulted.
		{name: "trim and lowercase", in: "  Kubernetes Pod  ", want: "pod"},
		{name: "all uppercased input", in: "VMWARE VM", want: "vmware-vm"},
		{name: "mixed case input", in: "TruENas SYStem", want: "truenas-system"},

		// Multi-token display aliases -> hyphenated canonical forms.
		{name: "kubernetes cluster", in: "kubernetes cluster", want: "k8s-cluster"},
		{name: "kubernetes node", in: "kubernetes node", want: "k8s-node"},
		{name: "kubernetes deployment", in: "kubernetes deployment", want: "k8s-deployment"},
		{name: "kubernetes namespace", in: "kubernetes namespace", want: "k8s-namespace"},
		{name: "kubernetes pod", in: "kubernetes pod", want: "pod"},
		{name: "truenas system", in: "truenas system", want: "truenas-system"},
		{name: "truenas pool", in: "truenas pool", want: "truenas-pool"},
		{name: "truenas dataset", in: "truenas dataset", want: "truenas-dataset"},
		{name: "truenas disk", in: "truenas disk", want: "truenas-disk"},

		// vmware/vsphere host variants collapse to vmware-host.
		{name: "vmware host", in: "vmware host", want: "vmware-host"},
		{name: "vsphere host", in: "vsphere host", want: "vmware-host"},

		// vmware/vsphere vm + "virtual machine" variants collapse to vmware-vm.
		{name: "vmware vm", in: "vmware vm", want: "vmware-vm"},
		{name: "vsphere vm", in: "vsphere vm", want: "vmware-vm"},
		{name: "vmware virtual machine", in: "vmware virtual machine", want: "vmware-vm"},
		{name: "vsphere virtual machine", in: "vsphere virtual machine", want: "vmware-vm"},

		// datastore / network variants.
		{name: "vmware datastore", in: "vmware datastore", want: "vmware-datastore"},
		{name: "vsphere datastore", in: "vsphere datastore", want: "vmware-datastore"},
		{name: "vmware network", in: "vmware network", want: "vmware-network"},
		{name: "vsphere network", in: "vsphere network", want: "vmware-network"},

		// Default arm: unknown tokens and already-canonical tokens pass through
		// unchanged (after lowercasing/trimming).
		{name: "unknown type passthrough", in: "custom-thing", want: "custom-thing"},
		{name: "already canonical k8s-cluster", in: "k8s-cluster", want: "k8s-cluster"},
		{name: "already canonical vmware-vm", in: "vmware-vm", want: "vmware-vm"},
		{name: "single token node", in: "node", want: "node"},

		// Empty / whitespace-only input: TrimSpace yields "" which hits default.
		{name: "empty string", in: "", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		{name: "tabs only", in: "\t\t", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.CanonicalAlertResourceType(tc.in)
			if got != tc.want {
				t.Errorf("CanonicalAlertResourceType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestBranchCovCanonicalResourceTypeKeysLegacyNil verifies that inputs which
// are rejected as unsupported legacy aliases (or empty) return a true nil
// slice rather than an empty non-nil slice. This exercises both the
// `typeKey == ""` branch and the `isUnsupportedLegacyAlertResourceType` branch
// of the early return, covering legacy tokens sourced from the
// unifiedresources alias map as well as the local switch in
// isUnsupportedLegacyAlertResourceType that are not asserted by the existing
// alerts-package test.
func TestBranchCovCanonicalResourceTypeKeysLegacyNil(t *testing.T) {
	// Each of these must yield a nil slice (the `return nil` path).
	legacyCases := []string{
		// Empty / blank -> typeKey == "" branch.
		"",
		"   ",
		// Local switch arms in isUnsupportedLegacyAlertResourceType not
		// previously asserted.
		"qemu", "lxc", "docker container", "dockercontainer", "docker service",
		"dockerservice", "k8s pod", "kubernetes", "kubernetes-cluster",
		"agent disk", "agentdisk", "pbs server", "pbsserver", "pmg server",
		"proxmox mail gateway",
		// unifiedresources-backed aliases.
		"system_container", "docker_container", "app_container",
		"docker_host", "kubernetes_cluster", "k8s_cluster",
		// Sanity: a couple already covered by the alerts-package test still
		// return nil here when invoked through the config package directly.
		"host", "docker", "dockerhost", "k8s",
	}

	for idx, in := range legacyCases {
		// Include the index so subtest names stay unique even when two tokens
		// differ only by separator (e.g. "docker container" vs "docker_container"),
		// which the test runner would otherwise collapse via space->underscore.
		name := fmt.Sprintf("legacy_%02d_%q", idx, in)
		t.Run(name, func(t *testing.T) {
			got := alertconfig.CanonicalResourceTypeKeys(in)
			if got != nil {
				t.Errorf("CanonicalResourceTypeKeys(%q) = %v, want nil", in, got)
			}
		})
	}
}

// TestBranchCovCanonicalResourceTypeKeysCaseInsensitive verifies that the
// case-folding performed inside CanonicalAlertResourceType makes
// CanonicalResourceTypeKeys case-insensitive on its input, exercising the
// integration between the two functions for representative type families.
func TestBranchCovCanonicalResourceTypeKeysCaseInsensitive(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "upper VM", in: "VM", want: []string{"vm", "guest"}},
		{name: "mixed Node", in: "Node", want: []string{"node"}},
		{name: "upper AGENT-DISK", in: "AGENT-DISK", want: []string{"agent-disk", "agent", "storage"}},
		{name: "upper STORAGE", in: "STORAGE", want: []string{"storage"}},
		{name: "title Kubernetes Cluster", in: "Kubernetes Cluster", want: []string{"k8s-cluster", "node"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.CanonicalResourceTypeKeys(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CanonicalResourceTypeKeys(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestBranchCovCanonicalResourceTypeKeysDisplayAliasEquivalence documents and
// pins the fact that a spaced display alias (e.g. "kubernetes cluster") and its
// hyphenated canonical form ("k8s-cluster") produce identical key sets. This is
// because CanonicalAlertResourceType normalizes the spaced form to the
// hyphenated form before CanonicalResourceTypeKeys' switch is evaluated. See
// the report for the related dead-code observation.
func TestBranchCovCanonicalResourceTypeKeysDisplayAliasEquivalence(t *testing.T) {
	pairs := []struct {
		display   string
		canonical string
	}{
		{"kubernetes cluster", "k8s-cluster"},
		{"kubernetes node", "k8s-node"},
		{"kubernetes deployment", "k8s-deployment"},
		{"kubernetes namespace", "k8s-namespace"},
		{"truenas system", "truenas-system"},
		{"truenas pool", "truenas-pool"},
		{"truenas disk", "truenas-disk"},
	}

	for _, p := range pairs {
		t.Run(p.display, func(t *testing.T) {
			fromDisplay := alertconfig.CanonicalResourceTypeKeys(p.display)
			fromCanonical := alertconfig.CanonicalResourceTypeKeys(p.canonical)
			if !reflect.DeepEqual(fromDisplay, fromCanonical) {
				t.Errorf("display %q -> %v, canonical %q -> %v; expected identical key sets",
					p.display, fromDisplay, p.canonical, fromCanonical)
			}
			// And both must be non-nil / non-empty for these valid types.
			if len(fromDisplay) == 0 {
				t.Errorf("CanonicalResourceTypeKeys(%q) returned no keys", p.display)
			}
		})
	}
}

// TestBranchCovCanonicalResourceTypeKeysDefaultPassthrough covers the default
// switch arm (unknown type returned as the sole key) directly through the
// config package, including a type that contains characters that would be a
// display alias only if it matched a known arm.
func TestBranchCovCanonicalResourceTypeKeysDefaultPassthrough(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "unknown custom type", in: "widget", want: []string{"widget"}},
		{name: "unknown with dashes", in: "foo-bar-baz", want: []string{"foo-bar-baz"}},
		{name: "empty after normalize default", in: "   ", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.CanonicalResourceTypeKeys(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CanonicalResourceTypeKeys(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
