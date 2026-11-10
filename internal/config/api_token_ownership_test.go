package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestUpsertAPITokenOwnsMutableRecord(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "insert"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			created := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
			used, expires := created.Add(time.Hour), time.Now().UTC().Add(time.Hour)
			input := APITokenRecord{
				ID: "snapshot-token", Name: "snapshot", Hash: auth.HashAPIToken("synthetic-upsert-token"),
				CreatedAt: created, LastUsedAt: &used, ExpiresAt: &expires,
				Scopes: []string{"host-agent:report", ScopeMonitoringRead},
				OrgID:  "fallback-org", OrgIDs: []string{"tenant-a", "tenant-b"},
				Metadata: map[string]string{"bound_agent_id": "host-a", "owner_user_id": "owner-a"},
			}
			cfg := &Config{}
			if replace {
				cfg.UpsertAPIToken(APITokenRecord{ID: input.ID, Hash: "old", CreatedAt: created.Add(-time.Hour)})
			}
			cfg.UpsertAPIToken(input)
			if len(cfg.APITokens) != 1 || cfg.PrimaryAPITokenHash() != input.Hash {
				t.Fatal("upsert changed identity, count or primary hash")
			}
			want := input.Clone()
			stored := &cfg.APITokens[0]
			if !reflect.DeepEqual(*stored, want) {
				t.Fatal("upsert changed the admitted values")
			}

			input.Scopes[0] = ScopeSettingsWrite
			input.OrgIDs[0] = "unrelated-tenant"
			input.Metadata["bound_agent_id"] = "unrelated-host"
			*input.LastUsedAt = time.Time{}
			*input.ExpiresAt = time.Now().UTC().Add(-time.Hour)
			if !reflect.DeepEqual(*stored, want) {
				t.Error("caller-owned mutable fields changed the admitted record")
			}
			if !cfg.IsValidAPIToken("synthetic-upsert-token") || !stored.CanAccessOrg("tenant-a") || stored.CanAccessOrg("unrelated-tenant") {
				t.Error("caller mutation changed expiry or organization admission")
			}

			callerAfterEdit := input.Clone()
			stored.Scopes[0] = ScopeAuditRead
			stored.OrgIDs[1] = "changed-inventory-tenant"
			stored.Metadata["owner_user_id"] = "changed-inventory-owner"
			*stored.LastUsedAt = created.Add(2 * time.Hour)
			*stored.ExpiresAt = created
			if !reflect.DeepEqual(input, callerAfterEdit) {
				t.Error("inventory mutation changed the caller's record")
			}
		})
	}
}

func TestAPITokenCloneOwnsEmptyOrganizationCapacity(t *testing.T) {
	for _, tc := range []struct {
		name string
		orgs []string
	}{
		{"nil", nil}, {"empty", []string{}}, {"empty-with-capacity", make([]string, 0, 2)},
		{"populated", []string{"tenant-a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := APITokenRecord{ID: "clone-token", OrgIDs: tc.orgs}
			clone := record.Clone()
			if !reflect.DeepEqual(record.OrgIDs, clone.OrgIDs) {
				t.Fatal("clone changed nil/empty organization representation or binding values")
			}
			if len(record.OrgIDs) == 0 {
				clone.OrgIDs = append(clone.OrgIDs, "snapshot-only")
				record.OrgIDs = append(record.OrgIDs, "inventory-only")
			} else {
				clone.OrgIDs[0] = "snapshot-only"
				record.OrgIDs[0] = "inventory-only"
			}
			if clone.OrgIDs[0] != "snapshot-only" {
				t.Fatal("clone shares organization backing capacity with the source")
			}
		})
	}
}

func TestUpsertAPITokenPreservesOptionalFieldShapes(t *testing.T) {
	for _, populated := range []bool{false, true} {
		record := APITokenRecord{ID: "optional-shapes"}
		if populated {
			record.OrgIDs = []string{}
			record.Metadata = map[string]string{}
		}
		cfg := &Config{}
		cfg.UpsertAPIToken(record)
		got := cfg.APITokens[0]
		if !reflect.DeepEqual(got.OrgIDs, record.OrgIDs) || !reflect.DeepEqual(got.Metadata, record.Metadata) ||
			got.LastUsedAt != nil || got.ExpiresAt != nil || got.ID != record.ID ||
			!reflect.DeepEqual(got.Scopes, []string{ScopeWildcard}) {
			t.Fatal("upsert changed optional field representation, ID or legacy default scopes")
		}
	}
}
