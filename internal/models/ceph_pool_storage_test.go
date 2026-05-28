package models

import "testing"

func TestStorageFromCephPool(t *testing.T) {
	cluster := CephCluster{
		Instance: "Main",
		Health:   "HEALTH_OK",
	}
	pool := CephPool{
		ID:             7,
		Name:           "data_replication",
		StoredBytes:    70,
		AvailableBytes: 30,
		Objects:        12,
		PercentUsed:    70,
	}

	storage := StorageFromCephPool(cluster, pool)

	if storage.ID != "Main-ceph-pool-data_replication" {
		t.Fatalf("ID = %q, want Main-ceph-pool-data_replication", storage.ID)
	}
	if len(storage.AliasIDs) != 1 || storage.AliasIDs[0] != "agent:Main-ceph-pool-data_replication" {
		t.Fatalf("AliasIDs = %#v, want agent-prefixed Ceph pool alias", storage.AliasIDs)
	}
	if storage.Name != "data_replication" || storage.Pool != "data_replication" {
		t.Fatalf("name/pool = %q/%q, want data_replication", storage.Name, storage.Pool)
	}
	if storage.Type != "ceph" || !storage.Shared || !storage.Enabled || !storage.Active {
		t.Fatalf("unexpected storage flags: %+v", storage)
	}
	if storage.Total != 100 || storage.Used != 70 || storage.Free != 30 || storage.Usage != 70 {
		t.Fatalf("unexpected capacity projection: %+v", storage)
	}
}
