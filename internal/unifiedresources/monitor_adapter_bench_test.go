package unifiedresources

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// benchmarkVMState is a synthetic 1,000-guest snapshot for comparing the
// accepted-ingest rebuild with its constituent registry operations. It does
// not model an installed fleet, provider polling, or durable-store latency.
func benchmarkVMState(count int) models.StateSnapshot {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{VMs: make([]models.VM, count), LastUpdate: now}
	for i := range snapshot.VMs {
		snapshot.VMs[i] = models.VM{
			ID:       fmt.Sprintf("lab:node-1:%d", i+100),
			VMID:     i + 100,
			Name:     fmt.Sprintf("vm-%d", i),
			Node:     "node-1",
			Instance: "lab",
			Status:   "running",
			CPUs:     4,
			CPU:      0.25,
			LastSeen: now,
		}
	}
	return snapshot
}

func BenchmarkMonitorAdapterReplaceRegistry1000VMs(b *testing.B) {
	snapshot := benchmarkVMState(1000)
	// A memory-backed store exercises change comparison and identity-pin
	// handling without measuring disk I/O or relying on host services.
	adapter := NewMonitorAdapter(NewRegistry(NewMemoryStore()))
	adapter.replaceRegistry(snapshot, nil)
	if got := len(adapter.GetAll()); got != len(snapshot.VMs) {
		b.Fatalf("warm registry length = %d, want %d", got, len(snapshot.VMs))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.replaceRegistry(snapshot, nil)
	}
	b.StopTimer()
	if got := len(adapter.GetAll()); got != len(snapshot.VMs) {
		b.Fatalf("final registry length = %d, want %d", got, len(snapshot.VMs))
	}
}

func BenchmarkRegistryIngestSnapshot1000VMs(b *testing.B) {
	snapshot := benchmarkVMState(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry := NewRegistry(nil)
		registry.IngestSnapshot(snapshot)
	}
}

func BenchmarkRegistryList1000VMs(b *testing.B) {
	registry := NewRegistry(nil)
	registry.IngestSnapshot(benchmarkVMState(1000))
	if got := len(registry.List()); got != 1000 {
		b.Fatalf("registry length = %d, want 1000", got)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.List()
	}
}

func BenchmarkRegistryChangeEmission1000VMs(b *testing.B) {
	snapshot := benchmarkVMState(1000)
	store := NewMemoryStore()
	before := NewRegistry(store)
	after := NewRegistry(store)
	before.IngestSnapshot(snapshot)
	after.IngestSnapshot(snapshot)
	observedAt := snapshot.LastUpdate
	b.Run("list-clones", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			recordRegistryChanges(store, before.List(), after.List(), observedAt, nil, SourcePulseDiff, "")
		}
	})
	b.Run("locked-generations", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			recordRegistryChangesBetweenGenerations(before, after, observedAt, nil, SourcePulseDiff, "")
		}
	})
}
