package config

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests in this file use the TestBranchcov0724pm prefix so the scoped run
//
//	go test ./internal/config/ -run '^TestBranchcov0724pm' -count=1
//
// selects only them. They raise branch/line coverage for five pure
// value-in/value-out helpers in pve_node_identity.go:
//   - identityIDExists                     (pve_node_identity.go:209)
//   - deterministicPVEClusterNodeIdentity  (pve_node_identity.go:218)
//   - samePVEClusterNode                   (pve_node_identity.go:232)
//   - PVEClusterNodeIdentityByID           (pve_node_identity.go:289)
//   - PVEClusterNodeNativeAliases          (pve_node_identity.go:304)
//
// All targets are pure value-in/value-out functions (no network, SSH, daemon
// or live database), so none is skipped on purity grounds.

// ---------------------------------------------------------------------------
// identityIDExists
// ---------------------------------------------------------------------------

func TestBranchcov0724pmIdentityIDExists(t *testing.T) {
	identities := []PVEClusterNodeIdentity{
		{ID: "first", NativeName: "n0"},
		{ID: "middle", NativeName: "n1"},
		{ID: "last", NativeName: "n2"},
	}

	tests := []struct {
		name       string
		identities []PVEClusterNodeIdentity
		id         string
		want       bool
	}{
		{
			name:       "nil slice returns false",
			identities: nil,
			id:         "anything",
			want:       false,
		},
		{
			name:       "empty slice returns false",
			identities: []PVEClusterNodeIdentity{},
			id:         "anything",
			want:       false,
		},
		{
			name:       "id absent returns false",
			identities: identities,
			id:         "nope",
			want:       false,
		},
		{
			name:       "empty id with no empty-id identity returns false",
			identities: identities,
			id:         "",
			want:       false,
		},
		{
			name:       "match at first position returns true",
			identities: identities,
			id:         "first",
			want:       true,
		},
		{
			name:       "match at middle position returns true",
			identities: identities,
			id:         "middle",
			want:       true,
		},
		{
			name:       "match at last position returns true",
			identities: identities,
			id:         "last",
			want:       true,
		},
		{
			name: "empty id matching an empty-id identity returns true",
			identities: []PVEClusterNodeIdentity{
				{ID: "real", NativeName: "n0"},
				{ID: "", NativeName: "n1"},
			},
			id:   "",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, identityIDExists(tt.identities, tt.id))
		})
	}
}

// ---------------------------------------------------------------------------
// deterministicPVEClusterNodeIdentity
// ---------------------------------------------------------------------------

func TestBranchcov0724pmDeterministicPVEClusterNodeIdentity(t *testing.T) {
	baseEndpoint := ClusterEndpoint{
		NodeID:       "qemu/1",
		NodeName:     "node1",
		Host:         "https://10.0.0.1:8006",
		IP:           "10.0.0.1",
		NativeNodeID: 1,
	}

	t.Run("returns hardcoded expected value for known inputs", func(t *testing.T) {
		// Pre-computed via Python hashlib.sha256 over the exact \x00-joined seed:
		//   "prod\x00qemu/1\x00node1\x00https://10.0.0.1:8006\x0010.0.0.1\x001\x000"
		// First 6 bytes hex-encoded and prefixed with "prod-node-".
		// This is NOT a tautology: the expected string was computed independently.
		const want = "prod-node-3d45ba619571"
		assert.Equal(t, want,
			deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0))
	})

	t.Run("same inputs always produce the same id", func(t *testing.T) {
		first := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0)
		second := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0)
		assert.NotEmpty(t, first)
		assert.Equal(t, first, second, "deterministic id must be stable across calls")
	})

	t.Run("output has correct format: scope prefix, 12 hex chars after -node-", func(t *testing.T) {
		scope := "my-cluster"
		got := deterministicPVEClusterNodeIdentity(scope, baseEndpoint, 5)
		// The id must be "<scope>-node-" followed by exactly 12 lowercase hex chars.
		assert.True(t, strings.HasPrefix(got, scope+"-node-"),
			"id must start with scope+\"-node-\", got %q", got)
		hexPart := strings.TrimPrefix(got, scope+"-node-")
		assert.Len(t, hexPart, 12, "hex suffix must be exactly 12 characters (6 bytes), got %q", hexPart)
		assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{12}$`), hexPart,
			"hex suffix must be lowercase hex only, got %q", hexPart)
	})

	t.Run("different ordinal produces different id", func(t *testing.T) {
		ordinal0 := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0)
		ordinal1 := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 1)
		// Pre-computed expected for ordinal 1.
		assert.Equal(t, "prod-node-9bc5e09ca228", ordinal1)
		assert.NotEqual(t, ordinal0, ordinal1,
			"different ordinals must yield different deterministic ids")
	})

	t.Run("different scope produces different id", func(t *testing.T) {
		prod := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0)
		staging := deterministicPVEClusterNodeIdentity("staging", baseEndpoint, 0)
		assert.NotEqual(t, prod, staging,
			"different scopes must yield different deterministic ids")
		assert.True(t, strings.HasPrefix(staging, "staging-node-"),
			"id must carry the new scope prefix")
	})

	t.Run("different endpoint fields each produce different id", func(t *testing.T) {
		base := deterministicPVEClusterNodeIdentity("prod", baseEndpoint, 0)

		// Vary one field at a time.
		for _, mod := range []struct {
			desc string
			ep   ClusterEndpoint
		}{
			{"NodeID", func(e ClusterEndpoint) ClusterEndpoint { e.NodeID = "qemu/2"; return e }(baseEndpoint)},
			{"NodeName", func(e ClusterEndpoint) ClusterEndpoint { e.NodeName = "node2"; return e }(baseEndpoint)},
			{"Host", func(e ClusterEndpoint) ClusterEndpoint { e.Host = "https://10.0.0.2:8006"; return e }(baseEndpoint)},
			{"IP", func(e ClusterEndpoint) ClusterEndpoint { e.IP = "10.0.0.2"; return e }(baseEndpoint)},
			{"NativeNodeID", func(e ClusterEndpoint) ClusterEndpoint { e.NativeNodeID = 2; return e }(baseEndpoint)},
		} {
			t.Run(mod.desc, func(t *testing.T) {
				got := deterministicPVEClusterNodeIdentity("prod", mod.ep, 0)
				assert.NotEqual(t, base, got,
					"changing %s must yield a different id", mod.desc)
			})
		}
	})

	t.Run("zero-value endpoint and scope produce valid prefixed id", func(t *testing.T) {
		got := deterministicPVEClusterNodeIdentity("", ClusterEndpoint{}, 0)
		// Pre-computed via Python hashlib.sha256 over the exact \x00-joined
		// seed of all-empty fields + strconv.Itoa(0) + strconv.Itoa(0).
		assert.Equal(t, "-node-871ec6e10d82", got)
	})

	t.Run("hash uses exactly the first 6 bytes of sha256", func(t *testing.T) {
		// Cross-check that the function's hex suffix matches a manual
		// sha256 of the documented seed, proving the seed assembly
		// (field order, separator, strconv.Itoa) is what the docstring claims.
		scope := "edge"
		ep := ClusterEndpoint{NodeID: "n/9", NodeName: "z", Host: "h", IP: "i", NativeNodeID: 42}
		ordinal := 7
		seed := strings.Join([]string{
			scope, ep.NodeID, ep.NodeName, ep.Host, ep.IP,
			itoa(ep.NativeNodeID), itoa(ordinal),
		}, "\x00")
		sum := sha256.Sum256([]byte(seed))
		wantHex := hex.EncodeToString(sum[:6])
		got := deterministicPVEClusterNodeIdentity(scope, ep, ordinal)
		assert.True(t, strings.HasSuffix(got, wantHex),
			"hex suffix %q must equal first-6-bytes of sha256(seed) %q, got %q",
			wantHex, wantHex, got)
	})
}

// itoa is a tiny local wrapper to avoid importing strconv just for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// samePVEClusterNode
// ---------------------------------------------------------------------------

func TestBranchcov0724pmSamePVEClusterNode(t *testing.T) {
	tests := []struct {
		name  string
		left  ClusterEndpoint
		right ClusterEndpoint
		want  bool
	}{
		// --- Branch: both NativeNodeID non-zero ---
		{
			name:  "both nonzero and equal returns true",
			left:  ClusterEndpoint{NativeNodeID: 5, NodeName: "a", Host: "h1"},
			right: ClusterEndpoint{NativeNodeID: 5, NodeName: "b", Host: "h2"},
			want:  true,
		},
		{
			name:  "both nonzero and different returns false",
			left:  ClusterEndpoint{NativeNodeID: 5, NodeName: "same", Host: "h1"},
			right: ClusterEndpoint{NativeNodeID: 6, NodeName: "same", Host: "h1"},
			want:  false,
		},
		// --- Branch: at least one NativeNodeID zero, name comparison ---
		{
			name:  "left zero right nonzero same name returns true",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			right: ClusterEndpoint{NativeNodeID: 9, NodeName: "pve1"},
			want:  true,
		},
		{
			name:  "left nonzero right zero same name returns true",
			left:  ClusterEndpoint{NativeNodeID: 9, NodeName: "pve1"},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			want:  true,
		},
		{
			name:  "both zero same name returns true",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			want:  true,
		},
		{
			name:  "both zero different name returns false",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "pve2"},
			want:  false,
		},
		{
			name:  "both zero empty names returns true",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: ""},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: ""},
			want:  true,
		},
		// --- Case-insensitivity and whitespace trimming in name comparison ---
		{
			name:  "case-insensitive name match returns true",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "PVE1"},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			want:  true,
		},
		{
			name:  "leading trailing whitespace trimmed before comparison",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "  pve1  "},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "pve1"},
			want:  true,
		},
		// --- Host and port differences are ignored (only ID/name matter) ---
		{
			name:  "different host same native id returns true",
			left:  ClusterEndpoint{NativeNodeID: 3, NodeName: "x", Host: "https://10.0.0.1:8006"},
			right: ClusterEndpoint{NativeNodeID: 3, NodeName: "x", Host: "https://10.0.0.99:8006"},
			want:  true,
		},
		{
			name:  "different port same native id returns true",
			left:  ClusterEndpoint{NativeNodeID: 3, NodeName: "x", Host: "https://10.0.0.1:8006"},
			right: ClusterEndpoint{NativeNodeID: 3, NodeName: "x", Host: "https://10.0.0.1:8007"},
			want:  true,
		},
		{
			name:  "identical endpoints return true",
			left:  ClusterEndpoint{NativeNodeID: 1, NodeName: "n", Host: "h", IP: "i"},
			right: ClusterEndpoint{NativeNodeID: 1, NodeName: "n", Host: "h", IP: "i"},
			want:  true,
		},
		{
			name:  "different host no native id different name returns false",
			left:  ClusterEndpoint{NativeNodeID: 0, NodeName: "alpha", Host: "h1"},
			right: ClusterEndpoint{NativeNodeID: 0, NodeName: "beta", Host: "h2"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, samePVEClusterNode(tt.left, tt.right))
		})
	}
}

// ---------------------------------------------------------------------------
// PVEClusterNodeIdentityByID
// ---------------------------------------------------------------------------

func TestBranchcov0724pmPVEClusterNodeIdentityByID(t *testing.T) {
	t.Run("nil instance returns zero value and false", func(t *testing.T) {
		identity, ok := PVEClusterNodeIdentityByID(nil, "any-id")
		assert.False(t, ok, "nil instance must report not-found")
		assert.Equal(t, PVEClusterNodeIdentity{}, identity,
			"nil instance must return zero-valued identity")
	})

	t.Run("empty identities returns zero value and false", func(t *testing.T) {
		instance := &PVEInstance{}
		identity, ok := PVEClusterNodeIdentityByID(instance, "missing")
		assert.False(t, ok)
		assert.Equal(t, PVEClusterNodeIdentity{}, identity)
	})

	t.Run("id not found returns zero value and false", func(t *testing.T) {
		instance := &PVEInstance{
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "alpha", NativeName: "n0"},
				{ID: "beta", NativeName: "n1"},
			},
		}
		identity, ok := PVEClusterNodeIdentityByID(instance, "gamma")
		assert.False(t, ok)
		assert.Equal(t, PVEClusterNodeIdentity{}, identity)
	})

	t.Run("found returns identity with all fields preserved", func(t *testing.T) {
		instance := &PVEInstance{
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "alpha", NativeNodeID: 1, NativeName: "n0"},
				{ID: "beta", NativeNodeID: 2, NativeName: "n1",
					NativeAliases: []string{"old1", "old2"}, DisplayName: "Display"},
			},
		}
		identity, ok := PVEClusterNodeIdentityByID(instance, "beta")
		require.True(t, ok, "existing identity must be found")
		assert.Equal(t, "beta", identity.ID)
		assert.Equal(t, 2, identity.NativeNodeID)
		assert.Equal(t, "n1", identity.NativeName)
		assert.Equal(t, []string{"old1", "old2"}, identity.NativeAliases)
		assert.Equal(t, "Display", identity.DisplayName)
	})

	t.Run("found with nil aliases returns nil aliases but ok true", func(t *testing.T) {
		instance := &PVEInstance{
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "no-aliases", NativeName: "n0"},
			},
		}
		identity, ok := PVEClusterNodeIdentityByID(instance, "no-aliases")
		require.True(t, ok)
		assert.Nil(t, identity.NativeAliases,
			"identity with nil aliases must return nil (clone of nil is nil)")
	})

	t.Run("returned NativeAliases is an independent copy", func(t *testing.T) {
		instance := &PVEInstance{
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "src", NativeName: "n0", NativeAliases: []string{"a", "b"}},
			},
		}
		identity, ok := PVEClusterNodeIdentityByID(instance, "src")
		require.True(t, ok)

		// Mutate the returned copy.
		identity.NativeAliases[0] = "MUTATED"
		identity.NativeAliases = append(identity.NativeAliases, "c")

		// Source must be untouched.
		assert.Equal(t, []string{"a", "b"},
			instance.ClusterNodeIdentities[0].NativeAliases,
			"mutating returned aliases must not affect the source instance")
	})

	t.Run("only first matching identity is returned", func(t *testing.T) {
		instance := &PVEInstance{
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "dup", NativeName: "first"},
				{ID: "dup", NativeName: "second"},
			},
		}
		identity, ok := PVEClusterNodeIdentityByID(instance, "dup")
		require.True(t, ok)
		assert.Equal(t, "first", identity.NativeName,
			"first match in iteration order must be returned")
	})
}

// ---------------------------------------------------------------------------
// PVEClusterNodeNativeAliases
// ---------------------------------------------------------------------------

// pveNodeIdentityTestInstance builds a PVEInstance with one endpoint whose
// NodeIdentity maps to a stored identity that carries the given aliases.
func pveNodeIdentityTestInstance(aliases []string) *PVEInstance {
	return &PVEInstance{
		ClusterEndpoints: []ClusterEndpoint{
			{NodeName: "pve1", NodeIdentity: "id-1"},
		},
		ClusterNodeIdentities: []PVEClusterNodeIdentity{
			{ID: "id-1", NativeName: "pve1", NativeAliases: aliases},
		},
	}
}

func TestBranchcov0724pmPVEClusterNodeNativeAliases(t *testing.T) {
	t.Run("nil instance returns nil", func(t *testing.T) {
		assert.Nil(t, PVEClusterNodeNativeAliases(nil, "pve1"))
	})

	t.Run("native name not matching any endpoint returns nil", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance([]string{"old"})
		assert.Nil(t, PVEClusterNodeNativeAliases(instance, "no-such-node"))
	})

	t.Run("endpoint match but identity not stored returns nil", func(t *testing.T) {
		// Endpoint references an identity ID that does not exist in the
		// identities slice, so the lookup chain resolves to not-found.
		instance := &PVEInstance{
			ClusterEndpoints: []ClusterEndpoint{
				{NodeName: "orphan", NodeIdentity: "missing-id"},
			},
			ClusterNodeIdentities: []PVEClusterNodeIdentity{
				{ID: "other-id", NativeName: "other"},
			},
		}
		assert.Nil(t, PVEClusterNodeNativeAliases(instance, "orphan"))
	})

	t.Run("matching native name returns stored aliases", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance([]string{"old1", "old2"})
		assert.Equal(t, []string{"old1", "old2"},
			PVEClusterNodeNativeAliases(instance, "pve1"))
	})

	t.Run("identity with nil aliases returns nil", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance(nil)
		assert.Nil(t, PVEClusterNodeNativeAliases(instance, "pve1"))
	})

	t.Run("identity with empty aliases returns nil", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance([]string{})
		got := PVEClusterNodeNativeAliases(instance, "pve1")
		// append([]string(nil), <empty>...) yields nil.
		assert.Nil(t, got, "empty alias slice must be returned as nil after clone")
	})

	t.Run("duplicate aliases are returned as stored without dedup", func(t *testing.T) {
		// PVEClusterNodeNativeAliases does not deduplicate; it returns whatever
		// the identity slice holds.
		instance := pveNodeIdentityTestInstance([]string{"dup", "dup", "other"})
		assert.Equal(t, []string{"dup", "dup", "other"},
			PVEClusterNodeNativeAliases(instance, "pve1"))
	})

	t.Run("case-insensitive native name match resolves aliases", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance([]string{"a"})
		// PVEClusterNodeIdentityForName falls back to case-insensitive matching.
		assert.Equal(t, []string{"a"},
			PVEClusterNodeNativeAliases(instance, "PVE1"))
	})

	t.Run("returned slice is an independent copy", func(t *testing.T) {
		instance := pveNodeIdentityTestInstance([]string{"orig1", "orig2"})
		got := PVEClusterNodeNativeAliases(instance, "pve1")
		require.Len(t, got, 2)

		got[0] = "tampered"

		assert.Equal(t, []string{"orig1", "orig2"},
			instance.ClusterNodeIdentities[0].NativeAliases,
			"mutating the returned alias slice must not affect the source instance")
	})
}
