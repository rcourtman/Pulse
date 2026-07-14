package tools

import "testing"

// These table tests close the previously uncovered branches of the three pure
// canonicalization helpers in current_resource.go:
//   - canonicalQueryTypeForResolvedResource
//   - canonicalQueryIDForResolvedResource
//   - resolvedResourceKindMatchesLocation
//
// They reuse the existing `mockResource` fake defined in strict_resolution_test.go,
// which implements the full ResolvedResourceInfo interface with settable fields.

func TestCanonicalQueryTypeForResolvedResource(t *testing.T) {
	tests := []struct {
		name     string
		resource ResolvedResourceInfo
		want     string
	}{
		// nil guard.
		{"nil resource returns empty", nil, ""},

		// case "lxc", "container", "system-container" -> "system-container"
		{"kind lxc maps to system-container", &mockResource{kind: "lxc"}, "system-container"},
		{"kind container maps to system-container", &mockResource{kind: "container"}, "system-container"},
		{"kind system-container maps to itself", &mockResource{kind: "system-container"}, "system-container"},

		// case "vm", "agent", "app-container", "storage" -> lower(trim(kind))
		{"kind vm maps to vm", &mockResource{kind: "vm"}, "vm"},
		{"kind agent maps to agent", &mockResource{kind: "agent"}, "agent"},
		{"kind app-container maps to itself", &mockResource{kind: "app-container"}, "app-container"},
		{"kind storage maps to itself", &mockResource{kind: "storage"}, "storage"},

		// case "node", "docker-host" -> "agent"
		{"kind node maps to agent", &mockResource{kind: "node"}, "agent"},
		{"kind docker-host maps to agent", &mockResource{kind: "docker-host"}, "agent"},

		// default -> lower(trim(GetResourceType()))
		{"unknown kind falls back to resource type", &mockResource{kind: "widget", resourceType: "CustomType"}, "customtype"},
		{"unknown kind with spaced resource type trims and lowercases", &mockResource{kind: "mystery", resourceType: "  ObjectStore  "}, "objectstore"},
		{"unknown kind with empty resource type yields empty", &mockResource{kind: "mystery", resourceType: ""}, ""},

		// normalization: trim + lower applied to kind before switch.
		{"padded upper LXC still maps to system-container", &mockResource{kind: "  LXC  "}, "system-container"},
		{"upper VM maps to vm", &mockResource{kind: "VM"}, "vm"},
		{"upper Node maps to agent", &mockResource{kind: "Node"}, "agent"},
		{"upper App-Container maps to app-container", &mockResource{kind: "App-Container"}, "app-container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalQueryTypeForResolvedResource(tt.resource)
			if got != tt.want {
				t.Fatalf("canonicalQueryTypeForResolvedResource(%+v) = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}

func TestCanonicalQueryIDForResolvedResource(t *testing.T) {
	tests := []struct {
		name     string
		resource ResolvedResourceInfo
		want     string
	}{
		// nil guard.
		{"nil resource returns empty", nil, ""},

		// provider-uid wins.
		{"provider uid wins over resource id", &mockResource{
			providerUID: "pve1/lxc/141",
			resourceID:  "res-141",
			aliases:     []string{"homepage"},
		}, "pve1/lxc/141"},

		// falls through to resource-id.
		{"falls through to resource id when provider uid blank", &mockResource{
			providerUID: "   ",
			resourceID:  "res-141",
			aliases:     []string{"homepage"},
		}, "res-141"},

		// falls through to alias.
		{"falls through to alias when ids blank", &mockResource{
			providerUID: "",
			resourceID:  "",
			aliases:     []string{"homepage", "ignored"},
		}, "homepage"},

		// skips blank aliases and returns first non-blank alias.
		{"skips blank aliases to first non-blank", &mockResource{
			providerUID: "",
			resourceID:  "",
			aliases:     []string{"", "  ", "first-real", "second"},
		}, "first-real"},

		// all blank -> "".
		{"all blank ids and aliases returns empty", &mockResource{
			providerUID: "  ",
			resourceID:  "",
			aliases:     []string{"", "   "},
		}, ""},

		// no aliases at all and blank ids -> "".
		{"blank ids and nil aliases returns empty", &mockResource{
			providerUID: "",
			resourceID:  "",
		}, ""},

		// trim applied to returned provider uid.
		{"trims whitespace from provider uid", &mockResource{
			providerUID: "  uid-with-space  ",
			resourceID:  "ignored",
		}, "uid-with-space"},

		// trim applied to returned resource id.
		{"trims whitespace from resource id", &mockResource{
			providerUID: "",
			resourceID:  "  rid-with-space  ",
		}, "rid-with-space"},

		// trim applied to returned alias.
		{"trims whitespace from alias", &mockResource{
			providerUID: "",
			resourceID:  "",
			aliases:     []string{"  alias-with-space  "},
		}, "alias-with-space"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalQueryIDForResolvedResource(tt.resource)
			if got != tt.want {
				t.Fatalf("canonicalQueryIDForResolvedResource(%+v) = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}

func TestResolvedResourceKindMatchesLocation(t *testing.T) {
	tests := []struct {
		name     string
		resource ResolvedResourceInfo
		locType  string
		want     bool
	}{
		// nil guard.
		{"nil resource returns false", nil, "vm", false},

		// system-container (kind lxc/container/system-container).
		{"system-container matches system-container loc", &mockResource{kind: "lxc"}, "system-container", true},
		{"system-container matches lxc loc", &mockResource{kind: "container"}, "lxc", true},
		{"system-container mismatches vm loc", &mockResource{kind: "system-container"}, "vm", false},
		{"system-container mismatches agent loc", &mockResource{kind: "lxc"}, "agent", false},

		// vm.
		{"vm matches vm loc", &mockResource{kind: "vm"}, "vm", true},
		{"vm mismatches system-container loc", &mockResource{kind: "vm"}, "system-container", false},
		{"vm mismatches agent loc", &mockResource{kind: "vm"}, "agent", false},

		// agent (kind node/docker-host/agent).
		{"agent matches agent loc", &mockResource{kind: "agent"}, "agent", true},
		{"node matches agent loc", &mockResource{kind: "node"}, "agent", true},
		{"node matches node loc", &mockResource{kind: "node"}, "node", true},
		{"node matches docker-host loc", &mockResource{kind: "node"}, "docker-host", true},
		{"docker-host matches docker-host loc", &mockResource{kind: "docker-host"}, "docker-host", true},
		{"docker-host matches agent loc", &mockResource{kind: "docker-host"}, "agent", true},
		{"agent mismatches vm loc", &mockResource{kind: "agent"}, "vm", false},
		{"agent mismatches app-container loc", &mockResource{kind: "agent"}, "app-container", false},

		// app-container.
		{"app-container matches app-container loc", &mockResource{kind: "app-container"}, "app-container", true},
		{"app-container matches docker-host loc", &mockResource{kind: "app-container"}, "docker-host", true},
		{"app-container mismatches system-container loc", &mockResource{kind: "app-container"}, "system-container", false},
		{"app-container mismatches agent loc", &mockResource{kind: "app-container"}, "agent", false},

		// default branch (kind not in switch; canonical = resource type).
		{"default kind matches equal loc", &mockResource{kind: "widget", resourceType: "customkind"}, "customkind", true},
		{"default kind mismatches different loc", &mockResource{kind: "widget", resourceType: "customkind"}, "other", false},
		{"default empty canonical never matches", &mockResource{kind: "widget", resourceType: ""}, "", false},
		{"default empty canonical mismatches nonempty loc", &mockResource{kind: "widget", resourceType: ""}, "vm", false},

		// storage passes through default branch (canonical = "storage").
		{"storage matches storage loc via default", &mockResource{kind: "storage"}, "storage", true},
		{"storage mismatches vm loc via default", &mockResource{kind: "storage"}, "vm", false},

		// locType normalization (trim + lower).
		{"loc type is trimmed and lowercased before compare", &mockResource{kind: "vm"}, "  VM  ", true},
		{"system-container loc trims and lowercases", &mockResource{kind: "lxc"}, "  System-Container ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvedResourceKindMatchesLocation(tt.resource, tt.locType)
			if got != tt.want {
				t.Fatalf("resolvedResourceKindMatchesLocation(%+v, %q) = %v, want %v", tt.resource, tt.locType, got, tt.want)
			}
		})
	}
}
