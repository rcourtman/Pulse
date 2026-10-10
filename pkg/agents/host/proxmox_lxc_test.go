package host

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestProxmoxLXCCollectionCompletenessWire(t *testing.T) {
	tests := []struct {
		name      string
		inventory ProxmoxLXCInventory
		valid     bool
	}{
		{"legacy", ProxmoxLXCInventory{}, true},
		{"complete empty", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionComplete}, true},
		{"partial", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial, Containers: []ProxmoxLXCContainer{{VMID: 100}}, OmittedVMIDs: []int{102}}, true},
		{"all failed", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial, OmittedVMIDs: []int{100, 102}}, true},
		{"partial without omissions", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial}, false},
		{"complete with omission", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionComplete, OmittedVMIDs: []int{100}}, false},
		{"legacy with omission", ProxmoxLXCInventory{OmittedVMIDs: []int{100}}, false},
		{"unknown status", ProxmoxLXCInventory{Status: "unavailable"}, false},
		{"contradictory VMID", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial, Containers: []ProxmoxLXCContainer{{VMID: 100}}, OmittedVMIDs: []int{100}}, false},
		{"duplicate omission", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial, OmittedVMIDs: []int{100, 100}}, false},
		{"invalid omission", ProxmoxLXCInventory{Status: ProxmoxLXCCollectionPartial, OmittedVMIDs: []int{99}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.inventory.CollectedAt = time.Unix(1700000000, 0).UTC()
			if got := tt.inventory.ValidateCollection() == nil; got != tt.valid {
				t.Fatalf("valid=%v, want %v", got, tt.valid)
			}
			b, err := json.Marshal(tt.inventory)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip ProxmoxLXCInventory
			if err := json.Unmarshal(b, &roundTrip); err != nil || !reflect.DeepEqual(tt.inventory, roundTrip) {
				t.Fatalf("round-trip=%+v, err=%v", roundTrip, err)
			}
			if tt.inventory.Status == "" && len(tt.inventory.OmittedVMIDs) == 0 {
				var old struct {
					Containers  []ProxmoxLXCContainer `json:"containers"`
					CollectedAt time.Time             `json:"collectedAt"`
				}
				if err := json.Unmarshal(b, &old); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestProxmoxLXCCollectionBounds(t *testing.T) {
	i := ProxmoxLXCInventory{Status: ProxmoxLXCCollectionComplete}
	for n := 0; n < ProxmoxLXCMaxContainers; n++ {
		i.Containers = append(i.Containers, ProxmoxLXCContainer{VMID: 100 + n})
	}
	if err := i.ValidateCollection(); err != nil {
		t.Fatal(err)
	}
	i.Containers = append(i.Containers, ProxmoxLXCContainer{VMID: 100 + ProxmoxLXCMaxContainers})
	if i.ValidateCollection() == nil {
		t.Fatal("over-bound inventory accepted")
	}
}
