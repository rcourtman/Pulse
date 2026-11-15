package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAPITokenInventoryRetainsAuthorityAfterBindingReads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record config.APITokenRecord
		owned  bool
	}{
		{"unbound", config.APITokenRecord{}, false},
		{"single", config.APITokenRecord{OrgID: "tenant-a"}, true},
		{"empty-list-fallback", config.APITokenRecord{OrgID: "tenant-a", OrgIDs: []string{}}, true},
		{"list-single", config.APITokenRecord{OrgID: "fallback", OrgIDs: []string{"tenant-a"}}, true},
		{"foreign-list", config.APITokenRecord{OrgID: "tenant-a", OrgIDs: []string{"tenant-b"}}, false},
		{"shared", config.APITokenRecord{OrgIDs: []string{"tenant-a", "tenant-b"}}, false},
		{"duplicate-list", config.APITokenRecord{OrgIDs: []string{"tenant-a", "tenant-a"}}, false},
		{"literal-space", config.APITokenRecord{OrgIDs: []string{" tenant-a "}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.record.ID = "fixture-inventory-read"
			cfg := &config.Config{}
			cfg.UpsertAPIToken(tc.record)
			record := &cfg.APITokens[0]
			before := record.Clone()
			inventory := APITokenInventory{Config: cfg, OrgID: "tenant-a"}
			config.Mu.RLock()
			bound := record.GetBoundOrgs()
			config.Mu.RUnlock()
			// A read consumer may edit its own returned list outside the
			// inventory lock, without changing any monitor's token authority.
			if len(bound) > 0 {
				bound[0] = "tenant-a"
			}
			bound = append(bound, "snapshot-only")
			config.Mu.RLock()
			found, _, ok := FindAPITokenLocked([]APITokenInventory{inventory}, record.ID)
			config.Mu.RUnlock()
			if inventory.Owns(record) != tc.owned || ok != tc.owned || (found == record) != tc.owned {
				t.Fatal("binding read changed the inventory's exact single-organization ownership")
			}
			if (APITokenInventory{}).Owns(record) != true || inventory.Owns(nil) {
				t.Fatal("instance-wide or missing-record ownership changed")
			}
			if !reflect.DeepEqual(*record, before) {
				t.Fatal("binding read changed saved token metadata or authority")
			}
		})
	}
}
