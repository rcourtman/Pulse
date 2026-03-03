package config

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostRuntimeStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	store := NewHostRuntimeStore(dir, nil)

	lastSeen := time.Now().UTC().Truncate(time.Second)
	host := models.Host{
		ID:              "host-1",
		Hostname:        "node-1.local",
		DisplayName:     "Node 1",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        lastSeen,
		TokenID:         "token-1",
	}

	if err := store.Upsert(host); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reloaded := NewHostRuntimeStore(dir, nil)
	all := reloaded.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 host, got %d", len(all))
	}
	got := all["host-1"]
	if got.Hostname != host.Hostname {
		t.Fatalf("hostname = %q, want %q", got.Hostname, host.Hostname)
	}
	if !got.LastSeen.Equal(lastSeen) {
		t.Fatalf("lastSeen = %v, want %v", got.LastSeen, lastSeen)
	}
	if got.TokenID != host.TokenID {
		t.Fatalf("tokenID = %q, want %q", got.TokenID, host.TokenID)
	}

	if err := reloaded.Delete("host-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotCount := len(reloaded.GetAll()); gotCount != 0 {
		t.Fatalf("expected 0 hosts after delete, got %d", gotCount)
	}
}

func TestHostRuntimeStoreClear(t *testing.T) {
	dir := t.TempDir()
	store := NewHostRuntimeStore(dir, nil)

	if err := store.Upsert(models.Host{ID: "host-1", Hostname: "one"}); err != nil {
		t.Fatalf("Upsert host-1: %v", err)
	}
	if err := store.Upsert(models.Host{ID: "host-2", Hostname: "two"}); err != nil {
		t.Fatalf("Upsert host-2: %v", err)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if gotCount := len(store.GetAll()); gotCount != 0 {
		t.Fatalf("expected 0 hosts after clear, got %d", gotCount)
	}

	reloaded := NewHostRuntimeStore(dir, nil)
	if gotCount := len(reloaded.GetAll()); gotCount != 0 {
		t.Fatalf("expected reloaded store to stay empty, got %d", gotCount)
	}
}
