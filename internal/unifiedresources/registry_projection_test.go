package unifiedresources

import (
	"fmt"
	"sync"
	"testing"
)

// Exercise the capture while a writer changes the mapping and row together.
// Separate List/point reads can legitimately observe different generations;
// this paired bulk read cannot. The test never touches persistence or drivers.
func TestRegistryProjectionSnapshotConcurrentMappingEdits(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(benchmarkVMState(1))
	id := rr.List()[0].ID
	set := func(version int) {
		rr.mu.Lock()
		defer rr.mu.Unlock()
		name := fmt.Sprintf("coordinate-%d", version)
		rr.resources[id].Name = name
		rr.bySource[SourceProxmox] = map[string]string{name: id}
		rr.invalidateViewsLocked()
	}
	set(0)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			set(i % 2)
		}
	}()
	defer wg.Wait()
	for i := 0; i < 200; i++ {
		rows, targets := rr.ListWithMetricsTargets()
		if len(rows) != 1 || rows[0].Name != targets[id].ResourceID {
			t.Fatalf("mixed row/target generation: row=%q target=%q", rows[0].Name, targets[id].ResourceID)
		}
	}
}

var projectionResourceSink []Resource
var projectionTargetSink map[string]MetricsTarget

// Include the entire owned List and every history coordinate. These are not
// point-lookup timings, nor a replacement for the old clone-allocation envelope.
func BenchmarkRegistryProjectionSnapshot(b *testing.B) {
	for _, count := range []int{0, 1, 1000} {
		for _, mode := range []string{"cold", "list-clean", "views-clean", "dirty"} {
			for _, bulk := range []bool{false, true} {
				name := "point-targets"
				if bulk {
					name = "bulk-targets"
				}
				b.Run(fmt.Sprintf("%d/%s/%s", count, mode, name), func(b *testing.B) {
					state := benchmarkVMState(count)
					rr := NewRegistry(nil)
					rr.IngestSnapshot(state)
					if mode != "cold" {
						rr.List()
					}
					if mode == "views-clean" {
						rr.VMs()
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if mode == "cold" {
							// Return to the state IngestSnapshot leaves: canonical
							// metadata, typed views and the source-target index all
							// pending. The refresh never reads its own prior output,
							// so this repeats a fresh ingest's first read exactly.
							// Rebuilding the registry behind StopTimer instead let a
							// nanosecond empty-registry read push b.N into the
							// hundreds of thousands, each paying an untimed ingest
							// and two stop-the-world timer reads.
							rr.mu.Lock()
							rr.invalidateSourceTargetsLocked()
							rr.mu.Unlock()
						}
						if mode == "dirty" {
							rr.mu.Lock()
							for _, r := range rr.resources {
								r.Tags = []string{fmt.Sprintf("version-%d", i)}
								break
							}
							rr.invalidateViewsLocked()
							rr.mu.Unlock()
						}
						if bulk {
							projectionResourceSink, projectionTargetSink = rr.ListWithMetricsTargets()
						} else {
							projectionResourceSink = rr.List()
							projectionTargetSink = make(map[string]MetricsTarget, len(projectionResourceSink))
							for _, row := range projectionResourceSink {
								if target := rr.MetricsTarget(row.ID); target != nil {
									projectionTargetSink[row.ID] = *target
								}
							}
						}
					}
				})
			}
		}
	}
}
