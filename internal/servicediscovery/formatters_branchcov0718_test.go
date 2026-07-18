package servicediscovery

import (
	"sort"
	"strings"
	"testing"
)

// tokenSetFrom runs addResourceIDTokens against a single resource ID and
// returns the resulting set as a sorted slice, so table cases can assert the
// exact token population produced by each branch.
func tokenSetFrom(resourceID string) []string {
	tokens := make(map[string]struct{})
	addResourceIDTokens(tokens, resourceID)
	out := make([]string, 0, len(tokens))
	for k := range tokens {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAddResourceIDTokens(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		expected   []string
	}{
		{
			name:       "empty-input-skipped",
			resourceID: "",
			expected:   []string{},
		},
		{
			name:       "whitespace-only-skipped",
			resourceID: "   ",
			expected:   []string{},
		},
		{
			name:       "plain-id",
			resourceID: "abc",
			expected:   []string{"abc"},
		},
		{
			name:       "slash-last-segment",
			resourceID: "abc/def",
			expected:   []string{"abc/def", "def"},
		},
		{
			name:       "colon-last-segment",
			resourceID: "abc:def",
			expected:   []string{"abc:def", "def"},
		},
		{
			name:       "vm-prefix-and-trailing-digits",
			resourceID: "VM-101",
			expected:   []string{"101", "vm-101"},
		},
		{
			name:       "ct-prefix-and-trailing-digits",
			resourceID: "ct-202",
			expected:   []string{"202", "ct-202"},
		},
		{
			name:       "lxc-prefix-no-trailing-digits-branch",
			resourceID: "LXC-303",
			expected:   []string{"303", "lxc-303"},
		},
		{
			name:       "qemu-slash-with-trailing-digits",
			resourceID: "qemu/404",
			expected:   []string{"404", "qemu/404"},
		},
		{
			name:       "lxc-slash-with-trailing-digits",
			resourceID: "lxc/505",
			expected:   []string{"505", "lxc/505"},
		},
		{
			name:       "docker-host-container-split",
			resourceID: "docker:host1/container1",
			expected:   []string{"container1", "docker:host1/container1", "host1", "host1/container1"},
		},
		{
			name:       "colon-without-slash-no-host-container-split",
			resourceID: "abc:def",
			expected:   []string{"abc:def", "def"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenSetFrom(tt.resourceID)
			if len(got) != len(tt.expected) {
				t.Fatalf("token count mismatch: got %v, want %v", got, tt.expected)
			}
			for i, v := range tt.expected {
				if got[i] != v {
					t.Fatalf("token[%d]: got %q, want %q (full got=%v)", i, got[i], v, got)
				}
			}
		})
	}
}

func TestBuildResourceIDTokenSet(t *testing.T) {
	t.Run("empty-input-yields-empty-set", func(t *testing.T) {
		if got := buildResourceIDTokenSet(nil); len(got) != 0 {
			t.Fatalf("expected empty token set, got %v", got)
		}
	})

	t.Run("all-whitespace-yields-empty-set", func(t *testing.T) {
		got := buildResourceIDTokenSet([]string{"  ", ""})
		if len(got) != 0 {
			t.Fatalf("expected empty token set for whitespace-only IDs, got %v", got)
		}
	})

	t.Run("multiple-ids-aggregated", func(t *testing.T) {
		got := buildResourceIDTokenSet([]string{"vm-101", "ct-202"})
		for _, want := range []string{"vm-101", "101", "ct-202", "202"} {
			if _, ok := got[want]; !ok {
				t.Fatalf("expected token %q in set, got %v", want, got)
			}
		}
	})
}

func TestDiscoveryMatchesTokens(t *testing.T) {
	tokens := map[string]struct{}{"abc": {}, "101": {}}

	t.Run("nil-discovery-never-matches", func(t *testing.T) {
		if discoveryMatchesTokens(nil, tokens) {
			t.Fatalf("nil discovery must not match any token set")
		}
	})

	t.Run("matching-token-returns-true", func(t *testing.T) {
		d := &ResourceDiscovery{ResourceID: "ABC"} // lowercased to "abc" by discoveryTokens
		if !discoveryMatchesTokens(d, tokens) {
			t.Fatalf("expected match for ResourceID ABC against token abc")
		}
	})

	t.Run("no-matching-token-returns-false", func(t *testing.T) {
		d := &ResourceDiscovery{ResourceID: "xyz"}
		if discoveryMatchesTokens(d, tokens) {
			t.Fatalf("expected no match for ResourceID xyz")
		}
	})

	t.Run("empty-token-set-returns-false", func(t *testing.T) {
		d := &ResourceDiscovery{ResourceID: "abc"}
		if discoveryMatchesTokens(d, map[string]struct{}{}) {
			t.Fatalf("empty token set must not match anything")
		}
	})
}

func TestDiscoveryTokens(t *testing.T) {
	tests := []struct {
		name     string
		disc     *ResourceDiscovery
		mustHave []string // tokens that MUST be present (lowercased)
	}{
		{
			name:     "vm-type",
			disc:     &ResourceDiscovery{ResourceID: "101", TargetID: "node1", ID: "vm:node1:101", ResourceType: ResourceTypeVM},
			mustHave: []string{"101", "vm:node1:101", "node1", "qemu/101", "vm/101", "vm-101", "agent:node1"},
		},
		{
			name:     "system-container-type",
			disc:     &ResourceDiscovery{ResourceID: "202", TargetID: "node1", ID: "lxc:node1:202", ResourceType: ResourceTypeSystemContainer},
			mustHave: []string{"202", "lxc/202", "ct/202", "ct-202", "system-container/202"},
		},
		{
			name:     "docker-type-with-target",
			disc:     &ResourceDiscovery{ResourceID: "app", TargetID: "host1", ID: "docker:host1:app", ResourceType: ResourceTypeDocker},
			mustHave: []string{"app", "host1", "docker:host1", "docker:host1/app"},
		},
		{
			name:     "docker-type-without-target",
			disc:     &ResourceDiscovery{ResourceID: "app", ID: "docker::app", ResourceType: ResourceTypeDocker},
			mustHave: []string{"app"},
		},
		{
			name:     "agent-type",
			disc:     &ResourceDiscovery{ResourceID: "ag1", TargetID: "host1", ID: "agent:host1:ag1", ResourceType: ResourceTypeAgent},
			mustHave: []string{"agent:ag1", "agent:host1", "ag1"},
		},
		{
			name:     "k8s-type",
			disc:     &ResourceDiscovery{ResourceID: "pod1", TargetID: "cluster1", ID: "k8s:cluster1:pod1", ResourceType: ResourceTypeK8s},
			mustHave: []string{"pod1", "k8s/pod1", "kubernetes/pod1"},
		},
		{
			name:     "unknown-type-falls-through-switch",
			disc:     &ResourceDiscovery{ResourceID: "rid", TargetID: "tid", ID: "weird:tid:rid", ResourceType: ResourceType("unknown")},
			mustHave: []string{"rid", "tid", "agent:tid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoveryTokens(tt.disc)
			set := make(map[string]struct{}, len(got))
			for _, g := range got {
				set[g] = struct{}{}
			}
			for _, want := range tt.mustHave {
				if _, ok := set[want]; !ok {
					t.Fatalf("expected token %q in discoveryTokens output, got %v", want, got)
				}
			}
			// Every returned token must be lowercase — matching relies on it.
			for _, g := range got {
				if g != strings.ToLower(g) {
					t.Fatalf("discoveryTokens returned non-lowercased token %q", g)
				}
			}
		})
	}
}

func TestFilterDiscoveriesByResourceIDs(t *testing.T) {
	discoveries := []*ResourceDiscovery{
		{ID: "vm:node1:101", ResourceType: ResourceTypeVM, ResourceID: "101", TargetID: "node1", ServiceName: "VM One"},
		{ID: "docker:host1:app", ResourceType: ResourceTypeDocker, ResourceID: "app", TargetID: "host1", ServiceName: "App"},
		{ID: "lxc:node2:202", ResourceType: ResourceTypeSystemContainer, ResourceID: "202", TargetID: "node2", ServiceName: "LXC"},
	}

	t.Run("empty-discoveries-returns-nil", func(t *testing.T) {
		if got := FilterDiscoveriesByResourceIDs(nil, []string{"101"}); got != nil {
			t.Fatalf("expected nil for empty discoveries, got %v", got)
		}
	})

	t.Run("empty-resource-ids-returns-all", func(t *testing.T) {
		got := FilterDiscoveriesByResourceIDs(discoveries, nil)
		if len(got) != len(discoveries) {
			t.Fatalf("expected all %d discoveries, got %d (%v)", len(discoveries), len(got), got)
		}
	})

	t.Run("whitespace-only-ids-returns-nil", func(t *testing.T) {
		// tokens set ends up empty -> filter returns nil (distinct from the
		// empty-resourceIDs arm which returns all).
		if got := FilterDiscoveriesByResourceIDs(discoveries, []string{"  ", ""}); got != nil {
			t.Fatalf("expected nil when token set is empty, got %v", got)
		}
	})

	t.Run("keeps-matching-drops-rest", func(t *testing.T) {
		got := FilterDiscoveriesByResourceIDs(discoveries, []string{"101"})
		if len(got) != 1 || got[0].ResourceID != "101" {
			t.Fatalf("expected only the VM (101), got %v", got)
		}
	})

	t.Run("no-match-returns-empty", func(t *testing.T) {
		got := FilterDiscoveriesByResourceIDs(discoveries, []string{"does-not-exist"})
		if len(got) != 0 {
			t.Fatalf("expected empty result for non-matching ID, got %v", got)
		}
	})

	t.Run("multiple-resource-ids", func(t *testing.T) {
		got := FilterDiscoveriesByResourceIDs(discoveries, []string{"101", "app"})
		if len(got) != 2 {
			t.Fatalf("expected 2 matches, got %d (%v)", len(got), got)
		}
		seen := map[string]bool{}
		for _, d := range got {
			seen[d.ResourceID] = true
		}
		if !seen["101"] || !seen["app"] {
			t.Fatalf("expected to keep 101 and app, got %v", seen)
		}
	})
}

func TestContainerFingerprint_HasChanged(t *testing.T) {
	fp := &ContainerFingerprint{Hash: "abc123", SchemaVersion: FingerprintSchemaVersion}

	t.Run("nil-other-is-change", func(t *testing.T) {
		if !fp.HasChanged(nil) {
			t.Fatalf("HasChanged(nil) must be true")
		}
	})

	t.Run("same-hash-no-change", func(t *testing.T) {
		other := &ContainerFingerprint{Hash: "abc123", SchemaVersion: FingerprintSchemaVersion}
		if fp.HasChanged(other) {
			t.Fatalf("HasChanged with identical hash must be false")
		}
	})

	t.Run("different-hash-is-change", func(t *testing.T) {
		other := &ContainerFingerprint{Hash: "different", SchemaVersion: FingerprintSchemaVersion}
		if !fp.HasChanged(other) {
			t.Fatalf("HasChanged with different hash must be true")
		}
	})
}

func TestContainerFingerprint_String(t *testing.T) {
	fp := &ContainerFingerprint{
		ResourceID: "rid-1",
		TargetID:   "tid-1",
		Hash:       "deadbeef",
		ImageName:  "nginx:1.2.3",
		Ports:      []string{"80/tcp", "443/tcp"},
	}

	got := fp.String()
	want := "Fingerprint{id=rid-1, target=tid-1, hash=deadbeef, image=nginx:1.2.3, ports=[80/tcp 443/tcp]}"
	if got != want {
		t.Fatalf("String() mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestGenerateK8sPodFingerprint(t *testing.T) {
	basePod := &KubernetesPod{
		UID:       "uid-1",
		Name:      "web",
		Namespace: "prod",
		NodeName:  "node-a",
		OwnerKind: "Deployment",
		OwnerName: "web-deploy",
		Containers: []KubernetesPodContainer{
			{Name: "c1", Image: "img1:v1"},
			{Name: "c2", Image: "img2:v2"},
		},
		Labels: map[string]string{"app": "web", "team": "platform"},
	}

	t.Run("deterministic-for-same-input", func(t *testing.T) {
		fp1 := GenerateK8sPodFingerprint("cluster-1", basePod)
		fp2 := GenerateK8sPodFingerprint("cluster-1", basePod)
		if fp1.Hash != fp2.Hash {
			t.Fatalf("expected deterministic hash; got %q then %q", fp1.Hash, fp2.Hash)
		}
		if fp1.Hash == "" {
			t.Fatalf("expected non-empty hash")
		}
	})

	t.Run("identity-fields-populated", func(t *testing.T) {
		fp := GenerateK8sPodFingerprint("cluster-1", basePod)
		if fp.ResourceID != "uid-1" {
			t.Fatalf("ResourceID: got %q, want uid-1", fp.ResourceID)
		}
		if fp.TargetID != "cluster-1" {
			t.Fatalf("TargetID: got %q, want cluster-1", fp.TargetID)
		}
		if fp.SchemaVersion != FingerprintSchemaVersion {
			t.Fatalf("SchemaVersion: got %d, want %d", fp.SchemaVersion, FingerprintSchemaVersion)
		}
		// First sorted image is "c1:img1:v1" (c1 < c2).
		if fp.ImageName != "c1:img1:v1" {
			t.Fatalf("ImageName: got %q, want c1:img1:v1", fp.ImageName)
		}
	})

	t.Run("empty-containers-leaves-image-empty", func(t *testing.T) {
		empty := &KubernetesPod{UID: "u", Name: "n", Namespace: "ns"}
		fp := GenerateK8sPodFingerprint("c", empty)
		if fp.ImageName != "" {
			t.Fatalf("ImageName: got %q, want empty", fp.ImageName)
		}
		if fp.Hash == "" {
			t.Fatalf("expected non-empty hash even with no containers")
		}
	})

	// Each mutation below must produce a different hash from the base.
	changeCases := []struct {
		name   string
		mutate func(p KubernetesPod) *KubernetesPod
	}{
		{
			name: "changed-uid",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.UID = "uid-2"
				return &p
			},
		},
		{
			name: "changed-name",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Name = "worker"
				return &p
			},
		},
		{
			name: "changed-namespace",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Namespace = "staging"
				return &p
			},
		},
		{
			name: "changed-node",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.NodeName = "node-b"
				return &p
			},
		},
		{
			name: "changed-owner-kind",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.OwnerKind = "StatefulSet"
				return &p
			},
		},
		{
			name: "cleared-owner",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.OwnerKind = ""
				p.OwnerName = ""
				return &p
			},
		},
		{
			name: "changed-container-image",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Containers = []KubernetesPodContainer{{Name: "c1", Image: "img1:v9"}}
				return &p
			},
		},
		{
			name: "cleared-containers",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Containers = nil
				return &p
			},
		},
		{
			name: "changed-label-value",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Labels = map[string]string{"app": "web", "team": "sre"}
				return &p
			},
		},
		{
			name: "cleared-labels",
			mutate: func(p KubernetesPod) *KubernetesPod {
				p.Labels = nil
				return &p
			},
		},
	}

	base := GenerateK8sPodFingerprint("cluster-1", basePod).Hash
	for _, tc := range changeCases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(*basePod)
			fp := GenerateK8sPodFingerprint("cluster-1", mutated)
			if fp.Hash == base {
				t.Fatalf("expected different hash after %s; both = %q", tc.name, base)
			}
		})
	}
}
