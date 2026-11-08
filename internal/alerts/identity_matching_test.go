package alerts

import (
	"strings"
	"testing"
)

func TestGuestAlertIdentityParsing(t *testing.T) {
	cases := []struct {
		id   string
		want guestAlertIdentity
	}{
		{"site:node:101", guestAlertIdentity{"site", "node", 101, ""}},
		{"site:new-node:000101", guestAlertIdentity{"site", "new-node", 101, ""}},
		{"region:site:node:101-disk-root", guestAlertIdentity{"region:site", "node", 101, "-disk-root"}},
		{"a::b:node:101", guestAlertIdentity{"a::b", "node", 101, ""}},
		{"\u2003site : node : 101-disk-root \u2003", guestAlertIdentity{"site", "node", 101, "-disk-root"}},
		{":node:101", guestAlertIdentity{"node", "node", 101, ""}},
		{" : node : 101", guestAlertIdentity{"node", "node", 101, ""}},
		{"site:node:101-", guestAlertIdentity{"site", "node", 101, "-"}},
		{"site:node:101-disk-", guestAlertIdentity{"site", "node", 101, "-disk-"}},
		{"site:node:101-other-suffix", guestAlertIdentity{"site", "node", 101, "-other-suffix"}},
		{"", guestAlertIdentity{}},
		{"  ", guestAlertIdentity{}},
		{"101", guestAlertIdentity{}},
		{"node:101", guestAlertIdentity{}},
		{"site::101", guestAlertIdentity{}},
		{"site: :101", guestAlertIdentity{}},
		{"::101", guestAlertIdentity{}},
		{"site:node:", guestAlertIdentity{}},
		{"site:node:0", guestAlertIdentity{}},
		{"site:node:000", guestAlertIdentity{}},
		{"site:node:-101", guestAlertIdentity{}},
		{"site:node:+101", guestAlertIdentity{}},
		{"site:node:101disk", guestAlertIdentity{}},
		{"site:node:101_disk", guestAlertIdentity{}},
		{"site:node:101 -disk-root", guestAlertIdentity{}},
		{"site:node:101-disk:root", guestAlertIdentity{}},
		{"site:node:１０１", guestAlertIdentity{}},
		{"site:node:999999999999999999999999999999999999999", guestAlertIdentity{}},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got, ok := parseGuestAlertIdentity(tc.id)
			if got != tc.want || ok != (tc.want.vmid != 0) {
				t.Fatalf("parse(%q) = %+v, %v; want %+v", tc.id, got, ok, tc.want)
			}
		})
	}
}

func TestGuestAlertIdentityMembership(t *testing.T) {
	cases := []struct {
		name, alert, guest string
		belongs, disk      bool
	}{
		{"same guest", "site:a:101", "site:a:101", true, false},
		{"moved guest", "site:a:101", "site:b:101", true, false},
		{"moved filesystem", "site:a:101-disk-root", "site:b:101", true, true},
		{"nested instance", "region:site:a:101-disk-root", "region:site:b:101", true, true},
		{"different instance", "site-a:a:101-disk-root", "site-b:a:101", false, false},
		{"instance boundary", "a:b:node:101-disk-root", "b:node:101", false, false},
		{"different guest", "site:a:101-disk-root", "site:a:102", false, false},
		{"guest cannot be child", "site:a:101-disk-root", "site:b:101-disk-root", false, false},
		{"leading zero", "site:a:00101-disk-root", "site:b:101", true, true},
		{"legacy empty instance", ":site:101-disk-root", "site:b:101", true, true},
		{"other child", "site:a:101-other", "site:b:101", true, false},
		{"disk without separator", "site:a:101-disk", "site:b:101", true, false},
		{"disk empty label", "site:a:101-disk-", "site:b:101", true, true},
		{"invalid alert", "site::101-disk-root", "site:b:101", false, false},
		{"invalid guest", "site:a:101-disk-root", "site:101", false, false},
		{"empty alert", "", "site:b:101", false, false},
		{"empty guest", "site:a:101-disk-root", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := guestAlertBelongsToGuest(tc.alert, tc.guest); got != tc.belongs {
				t.Fatalf("guest match = %v, want %v", got, tc.belongs)
			}
			if got := guestDiskAlertBelongsToGuest(tc.alert, tc.guest); got != tc.disk {
				t.Fatalf("filesystem match = %v, want %v", got, tc.disk)
			}
		})
	}
}

func TestGuestSpecIdentityAcrossNodes(t *testing.T) {
	cases := []struct {
		name, existingSpec, existingResource, spec, resource string
		want                                                 bool
	}{
		{"metric", "metric-threshold:cpu", "site:a:101", "metric-threshold:cpu", "site:b:101", true},
		{"equal without resources", "metric-threshold:cpu", "", "metric-threshold:cpu", "", true},
		{"powered state", "site:a:101-powered-state", "site:a:101", "site:b:101-powered-state", "site:b:101", true},
		{"filesystem", "site:a:101-disk-root-custom", "site:a:101-disk-root", "site:b:101-disk-root-custom", "site:b:101-disk-root", true},
		{"different metric", "metric-threshold:disk", "site:a:101", "metric-threshold:cpu", "site:b:101", false},
		{"different suffix", "old-other", "old", "new-powered-state", "new", false},
		{"partial existing resource", "old-extra-powered-state", "old", "new-powered-state", "new", false},
		{"existing prefix missing", "other-powered-state", "old", "new-powered-state", "new", false},
		{"current prefix missing", "old-powered-state", "old", "other-powered-state", "new", false},
		{"empty suffix", "old", "old", "new", "new", false},
		{"empty old resource", "-powered-state", "", "new-powered-state", "new", false},
		{"empty spec", "", "old", "", "new", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := guestSpecIDMatchesAcrossNodes(tc.existingSpec, tc.existingResource, tc.spec, tc.resource); got != tc.want {
				t.Fatalf("spec match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanonicalStateIdentitySplitHints(t *testing.T) {
	cases := []struct {
		name, id, resourceHint, specHint, resource, spec string
		ok                                               bool
	}{
		{"spec takes precedence", "docker:a::b::metric-threshold:cpu", "docker:a", "metric-threshold:cpu", "docker:a::b", "metric-threshold:cpu", true},
		{"spec contains separator", "resource::custom::state", "resource::custom", "custom::state", "resource", "custom::state", true},
		{"resource hint", "docker:a::b::custom", "docker:a::b", "", "docker:a::b", "custom", true},
		{"trimmed hints", "resource::custom", " resource ", " custom ", "resource", "custom", true},
		{"spec substring not delimited", "resource::metric", "", "etric", "resource", "metric", true},
		{"resource substring not delimited", "resource-long::spec", "resource", "", "resource-long", "spec", true},
		{"empty hinted resource falls back", "::spec", "", "spec", "", "spec", true},
		{"empty hinted spec falls back", "docker:a::b::", "docker:a::b", "", "docker:a", "b::", true},
		{"missing hints", "resource::spec::extra", "", "", "resource", "spec::extra", true},
		{"wrong hints", "resource::spec", "other", "other", "resource", "spec", true},
		{"single colon", "resource:spec", "resource", "spec", "resource:spec", "", false},
		{"empty spec", "resource::", "resource", "", "resource", "", true},
		{"empty resource", "::spec", "", "", "", "spec", true},
		{"separator only", "::", "", "", "", "", true},
		{"empty id", "", "resource", "spec", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource, spec, ok := splitCanonicalStateID(tc.id, tc.resourceHint, tc.specHint)
			if resource != tc.resource || spec != tc.spec || ok != tc.ok {
				t.Fatalf("split = %q, %q, %v; want %q, %q, %v", resource, spec, ok, tc.resource, tc.spec, tc.ok)
			}
		})
	}
}

var identityMatchResult struct {
	identity       guestAlertIdentity
	resource, spec string
	ok             bool
}

func TestAlertIdentityMatchingDoesNotAllocate(t *testing.T) {
	// Long IDs force the old concatenations past the compiler's small stack
	// buffer. Return values escape to a sink so the reads cannot disappear.
	instance := strings.Repeat("region:", 16) + "site"
	oldResource, resource := instance+":old-node:101", instance+":new-node:101"
	oldSpec, spec := oldResource+"-powered-state", resource+"-powered-state"
	diskResource := oldResource + "-disk-root"
	state := resource + "::" + spec
	cases := []struct {
		name string
		run  func()
	}{
		{"guest parser", func() { identityMatchResult.identity, identityMatchResult.ok = parseGuestAlertIdentity(resource) }},
		{"guest match", func() { identityMatchResult.ok = guestAlertBelongsToGuest(oldResource, resource) }},
		{"filesystem match", func() { identityMatchResult.ok = guestDiskAlertBelongsToGuest(diskResource, resource) }},
		{"spec match", func() { identityMatchResult.ok = guestSpecIDMatchesAcrossNodes(oldSpec, oldResource, spec, resource) }},
		{"state spec hint", func() {
			identityMatchResult.resource, identityMatchResult.spec, identityMatchResult.ok = splitCanonicalStateID(state, "old", spec)
		}},
		{"state resource hint", func() {
			identityMatchResult.resource, identityMatchResult.spec, identityMatchResult.ok = splitCanonicalStateID(state, resource, "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := testing.AllocsPerRun(100, tc.run); got != 0 {
				t.Fatalf("identity matching allocations = %g, want zero", got)
			}
		})
	}
}
