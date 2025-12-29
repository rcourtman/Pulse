package updatedetection

import "testing"

func TestStore_DeleteUpdateNotFound(t *testing.T) {
	store := NewStore()
	store.DeleteUpdate("missing")
	if store.Count() != 0 {
		t.Fatalf("expected empty store")
	}
}

func TestStore_DeleteUpdatesForResourceMissing(t *testing.T) {
	store := NewStore()
	store.DeleteUpdatesForResource("missing")
	if store.Count() != 0 {
		t.Fatalf("expected empty store")
	}
}

func TestStore_DeleteUpdatesForResourceNilUpdate(t *testing.T) {
	store := NewStore()
	store.byResource["res-1"] = "update-1"
	store.updates["update-1"] = nil

	store.DeleteUpdatesForResource("res-1")
	if _, ok := store.byResource["res-1"]; ok {
		t.Fatalf("expected byResource entry to be removed")
	}
}

func TestStore_CountForHost(t *testing.T) {
	store := NewStore()
	store.UpsertUpdate(&UpdateInfo{ID: "update-1", ResourceID: "res-1", HostID: "host-1"})
	store.UpsertUpdate(&UpdateInfo{ID: "update-2", ResourceID: "res-2", HostID: "host-1"})

	if store.CountForHost("host-1") != 2 {
		t.Fatalf("expected count 2 for host-1")
	}
	if store.CountForHost("missing") != 0 {
		t.Fatalf("expected count 0 for missing host")
	}
}
