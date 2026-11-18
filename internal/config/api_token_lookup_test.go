package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Both entry points must keep the ordered, first-hash-match decision. An
// expired or role-invalid matching record cannot fall through to a later grant.
func TestAPITokenInventoryLookupPreservesFirstMatch(t *testing.T) {
	const raw = "synthetic-inventory-lookup-bearer"
	past := time.Now().Add(-time.Hour)
	for _, scenario := range []string{"accepted", "missing", "empty", "malformed_hash", "uppercase_hash", "expired_first", "invalid_role_first", "valid_duplicate", "exact_whitespace", "untrimmed_bearer"} {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/readOnly=%t", scenario, readOnly), func(t *testing.T) {
				first := APITokenRecord{ID: "first", Hash: auth.HashAPIToken(raw), OrgID: "client-a", Scopes: []string{ScopeMonitoringRead}}
				second := APITokenRecord{ID: "second", Hash: auth.HashAPIToken("different-synthetic-bearer"), OrgID: "client-b", Scopes: []string{ScopeWildcard}}
				presented, wantID := raw, "first"
				switch scenario {
				case "missing":
					presented, wantID = "not-in-inventory", ""
				case "empty":
					first.Hash = auth.HashAPIToken("")
					presented, wantID = "", ""
				case "malformed_hash":
					first.Hash = "malformed"
					second.Hash, wantID = auth.HashAPIToken(raw), "second"
				case "uppercase_hash":
					first.Hash = strings.ToUpper(first.Hash)
					wantID = ""
				case "expired_first":
					first.ExpiresAt = &past
					second.Hash, wantID = first.Hash, ""
				case "invalid_role_first":
					first.Metadata = map[string]string{auth.RuntimeRoleMetadataKey: "unknown-role"}
					second.Hash, wantID = first.Hash, ""
				case "valid_duplicate":
					second.Hash = first.Hash
				case "exact_whitespace":
					presented = " " + raw + " "
					first.Hash = auth.HashAPIToken(presented)
				case "untrimmed_bearer":
					presented, wantID = " "+raw+" ", ""
				}
				cfg := &Config{APITokens: []APITokenRecord{first, second}}
				before := []APITokenRecord{first.Clone(), second.Clone()}
				if readOnly {
					if got := cfg.IsValidAPIToken(presented); got != (wantID != "") {
						t.Fatalf("read-only lookup = %t, want ID %q", got, wantID)
					}
					if !reflect.DeepEqual(cfg.APITokens, before) {
						t.Fatal("read-only lookup changed saved token metadata or authority")
					}
					return
				}
				got, ok := cfg.ValidateAPIToken(presented)
				if ok != (wantID != "") || (got != nil && got.ID != wantID) || (got == nil && ok) {
					t.Fatalf("validating lookup = (%v, %t), want ID %q", got, ok, wantID)
				}
				for i := range cfg.APITokens {
					if cfg.APITokens[i].ID == wantID {
						if got != &cfg.APITokens[i] || got.LastUsedAt == nil {
							t.Fatal("accepted lookup did not return/update only its live inventory record")
						}
						before[i].LastUsedAt = got.LastUsedAt
					}
				}
				if !reflect.DeepEqual(cfg.APITokens, before) {
					t.Fatal("lookup changed a different token or accepted authority")
				}
			})
		}
	}
}

func TestAPITokenInventoryReadOnlyLeavesLegacyMetadataUnchanged(t *testing.T) {
	const raw = "synthetic-read-only-bearer"
	cfg := &Config{APITokens: []APITokenRecord{{Hash: auth.HashAPIToken(raw), OrgID: "client-a", Scopes: []string{"host-agent:report"}}}}
	before := cfg.APITokens[0]
	before.Scopes = append([]string(nil), before.Scopes...)
	if !cfg.IsValidAPIToken(raw) {
		t.Fatal("legacy-compatible read-only lookup was denied")
	}
	if !reflect.DeepEqual(cfg.APITokens[0], before) {
		t.Fatal("read-only lookup assigned an ID, canonical scopes or last-used timestamp")
	}
}

func TestAPITokenInventoryScopeNormalizationAgreesAcrossOperations(t *testing.T) {
	const raw = "synthetic-token-scope-normalization"
	collector := auth.RuntimeRoleMonitoringCollector
	paddedReport := "\u2003host-agent:report\t"
	configRead := " host-agent:config:read "
	wide := append(slices.Repeat([]string{paddedReport}, 8), configRead)
	wideUnknown := append(slices.Clone(wide), " unknown:scope ")

	for _, tc := range []struct {
		name, role string
		scopes     []string
		canonical  []string
		allow      bool
	}{
		{"absent", "", nil, []string{ScopeWildcard}, true},
		{"empty", "", []string{}, []string{ScopeWildcard}, true},
		{"explicit-blank", "", []string{"\t", ""}, []string{""}, true},
		{"alias-order", " " + collector + " ", []string{configRead, paddedReport, ScopeAgentConfigRead},
			[]string{ScopeAgentConfigRead, ScopeAgentReport}, true},
		{"collector-missing-required", collector, []string{paddedReport, ScopeAgentReport}, []string{ScopeAgentReport}, false},
		{"collector-no-scopes", collector, nil, []string{ScopeWildcard}, false},
		{"collector-wide", collector, wide, []string{ScopeAgentReport, ScopeAgentConfigRead}, true},
		{"collector-wide-extra", collector, wideUnknown, []string{ScopeAgentReport, ScopeAgentConfigRead, "unknown:scope"}, false},
		{"runner", auth.RuntimeRoleActionRunner, []string{" " + ScopeAgentExec, ScopeAgentExec}, []string{ScopeAgentExec}, true},
		{"runner-wildcard", auth.RuntimeRoleActionRunner, []string{ScopeAgentExec, ScopeWildcard}, []string{ScopeAgentExec, ScopeWildcard}, false},
		{"legacy-full-trust", auth.RuntimeRoleLegacyFullTrust, []string{ScopeAgentExec, ScopeWildcard}, []string{ScopeAgentExec, ScopeWildcard}, true},
		{"unknown-role", "unknown", []string{ScopeAgentExec}, []string{ScopeAgentExec}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{APITokens: []APITokenRecord{{
				ID: "synthetic-id", Hash: auth.HashAPIToken(raw), OrgID: "client-a",
				Scopes: slices.Clone(tc.scopes), Metadata: map[string]string{auth.RuntimeRoleMetadataKey: tc.role},
			}}}
			live := &cfg.APITokens[0]
			original := *live
			original.Scopes = slices.Clone(live.Scopes)

			if got := cfg.IsValidAPIToken(raw); got != tc.allow {
				t.Fatalf("read-only decision = %t, want %t", got, tc.allow)
			}
			snapshot := live.Clone()
			if !slices.Equal(snapshot.Scopes, tc.canonical) {
				t.Fatalf("snapshot scopes = %q, want %q", snapshot.Scopes, tc.canonical)
			}
			snapshot.Scopes[0] = "fixture-only"
			if !reflect.DeepEqual(*live, original) {
				t.Fatal("read-only check or editing the snapshot changed live scopes or metadata")
			}

			record, got := cfg.ValidateAPIToken(raw)
			if got != tc.allow || (record == live) != tc.allow || (live.LastUsedAt != nil) != tc.allow {
				t.Fatalf("write decision = %t, want %t with metadata updated only on acceptance", got, tc.allow)
			}
			if !slices.Equal(live.Scopes, tc.canonical) {
				t.Fatalf("saved runtime scopes = %q, want %q", live.Scopes, tc.canonical)
			}
		})
	}
}

func TestAPITokenInventorySmallScopePrecheckNeedsNoExtraAllocation(t *testing.T) {
	const raw = "synthetic-auth-precheck-present"
	const missing = "synthetic-auth-precheck-missing"
	for _, tc := range []struct {
		name, role string
		scopes     []string
	}{
		{"wildcard-default", "", nil},
		{"scoped-api", "", []string{ScopeMonitoringRead}},
		{"collector", auth.RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport}},
		{"eight-legacy-inputs", auth.RuntimeRoleMonitoringCollector, append(slices.Repeat([]string{"host-agent:report"}, 7), " host-agent:config:read ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{APITokens: []APITokenRecord{{
				ID: "synthetic-id", Hash: auth.HashAPIToken(raw), Scopes: tc.scopes,
				Metadata: map[string]string{auth.RuntimeRoleMetadataKey: tc.role},
			}}}
			measure := func(bearer string, want bool) float64 {
				return testing.AllocsPerRun(100, func() {
					if cfg.IsValidAPIToken(bearer) != want {
						panic("authentication decision changed")
					}
				})
			}
			// Both inputs hash one bearer and check one record. An accepted
			// authorisation with a small scope list needs no extra allocation.
			miss, hit := measure(missing, false), measure(raw, true)
			if hit > miss {
				t.Fatalf("accepted precheck allocated %.0f times vs %.0f for a miss", hit, miss)
			}
		})
	}
}

// Measure the authentication work itself. Inventory construction and credential
// hashing for saved records are outside the timed loop; no secrets or providers
// are involved. Include cheap first matches as well as late/missing lookups.
func BenchmarkAPITokenInventory(b *testing.B) {
	for _, size := range []int{1, 100, 1000} {
		for _, position := range []string{"first", "last", "missing"} {
			for _, readOnly := range []bool{false, true} {
				b.Run(fmt.Sprintf("records=%d/%s/readOnly=%t", size, position, readOnly), func(b *testing.B) {
					cfg := &Config{APITokens: make([]APITokenRecord, size)}
					for i := range cfg.APITokens {
						cfg.APITokens[i] = APITokenRecord{ID: fmt.Sprintf("synthetic-%d", i), Hash: auth.HashAPIToken(fmt.Sprintf("synthetic-bearer-%d", i)), OrgID: "client-a", Scopes: []string{ScopeMonitoringRead}}
					}
					raw := "not-in-inventory"
					if position == "first" {
						raw = "synthetic-bearer-0"
					} else if position == "last" {
						raw = fmt.Sprintf("synthetic-bearer-%d", size-1)
					}
					want := position != "missing"
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						var ok bool
						if readOnly {
							ok = cfg.IsValidAPIToken(raw)
						} else {
							_, ok = cfg.ValidateAPIToken(raw)
						}
						if ok != want {
							b.Fatal("authentication decision changed")
						}
					}
				})
			}
		}
	}
}
