package unifiedresources

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestProxmoxUpdateEvidenceJSONCarriesAuthoritativeClears(t *testing.T) {
	at := time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		data *ProxmoxData
	}{
		{"checked zero", &ProxmoxData{PendingUpdatesStatus: "checked", PendingUpdatesCheckedAt: &at}},
		{"stale zero", &ProxmoxData{PendingUpdatesStatus: "stale", PendingUpdatesCheckedAt: &at, PendingUpdatesReason: "permission_denied"}},
		{"unavailable", &ProxmoxData{PendingUpdatesStatus: "unavailable", PendingUpdatesReason: "permission_denied"}},
		{"offline", &ProxmoxData{PendingUpdatesStatus: "not_checked", PendingUpdatesReason: "node_offline"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.Marshal(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(wire, &fields); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"pendingUpdates", "pendingUpdatesCheckedAt", "pendingUpdatesStatus", "pendingUpdatesReason"} {
				if _, present := fields[field]; !present {
					t.Errorf("authoritative observation omitted %s: %s", field, wire)
				}
			}
			// The consuming snapshot merger retains omitted keys. Exercise that
			// real wire behaviour rather than merely round-tripping a fresh zero.
			merged := map[string]any{"pendingUpdates": float64(7), "pendingUpdatesReason": "permission_denied", "pendingUpdatesCheckedAt": "old"}
			if err := json.Unmarshal(wire, &merged); err != nil {
				t.Fatal(err)
			}
			if merged["pendingUpdates"] != float64(0) || merged["pendingUpdatesReason"] != tc.data.PendingUpdatesReason ||
				(tc.data.PendingUpdatesCheckedAt == nil && merged["pendingUpdatesCheckedAt"] != nil) {
				t.Fatalf("JSON retained an obsolete count/reason/origin: %+v", merged)
			}
			// The same metadata is exposed in API resources and realtime
			// projections. Both must invoke the authoritative serializer.
			resourceWire, err := json.Marshal(&Resource{Proxmox: tc.data})
			if err != nil {
				t.Fatal(err)
			}
			var resource struct{ Proxmox json.RawMessage }
			if err := json.Unmarshal(resourceWire, &resource); err != nil || string(resource.Proxmox) != string(wire) {
				t.Fatalf("resource API changed update metadata: %s / %v", resourceWire, err)
			}
			front := models.ConvertResourceToFrontend(models.ResourceConvertInput{Proxmox: wire})
			if string(front.Proxmox) != string(wire) {
				t.Fatal("realtime projection dropped authoritative clears")
			}
		})
	}
}

func TestProxmoxUpdateEvidenceJSONPreservesGuestsLegacyAndNil(t *testing.T) {
	for _, p := range []*ProxmoxData{nil, {}, {VMID: 105, OSName: "Linux"}, {PendingUpdates: 7}} {
		wire, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		type plain ProxmoxData
		original, err := json.Marshal((*plain)(p))
		if err != nil || string(wire) != string(original) {
			t.Fatalf("unrelated payload changed: %s, want %s / %v", wire, original, err)
		}
	}
}

func TestMergeProxmoxDataTreatsUpdateEvidenceAsAuthoritative(t *testing.T) {
	oldCheckedAt := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	newCheckedAt := oldCheckedAt.Add(time.Hour)
	existing := &ProxmoxData{
		PendingUpdates:          9,
		PendingUpdatesCheckedAt: &oldCheckedAt,
		PendingUpdatesStatus:    "checked",
	}
	incoming := &ProxmoxData{
		PendingUpdates:          0,
		PendingUpdatesCheckedAt: &newCheckedAt,
		PendingUpdatesStatus:    "checked",
	}

	merged := mergeProxmoxData(existing, incoming)
	if merged.PendingUpdates != 0 || merged.PendingUpdatesStatus != "checked" || merged.PendingUpdatesCheckedAt == nil || !merged.PendingUpdatesCheckedAt.Equal(newCheckedAt) {
		t.Fatalf("expected confirmed zero to replace old positive evidence, got %+v", merged)
	}

	incoming.PendingUpdatesStatus = "unavailable"
	incoming.PendingUpdatesReason = "permission_denied"
	incoming.PendingUpdatesCheckedAt = nil
	merged = mergeProxmoxData(merged, incoming)
	if merged.PendingUpdatesStatus != "unavailable" || merged.PendingUpdatesReason != "permission_denied" || merged.PendingUpdatesCheckedAt != nil {
		t.Fatalf("expected unavailable evidence to clear inherited check metadata, got %+v", merged)
	}
}
