package unifiedresources

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkIngestRecords_NewResources measures the ingest path when every
// record creates a new resource (no dedup). This is the baseline cost of
// ingestion: ID generation, map insertion, identity indexing, and child
// count rebuild.
func BenchmarkIngestRecords_NewResources(b *testing.B) {
	for _, count := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("records=%d", count), func(b *testing.B) {
			records := make([]IngestRecord, count)
			now := time.Now().UTC()
			for i := range records {
				records[i] = IngestRecord{
					SourceID: fmt.Sprintf("vm-%d", i),
					Resource: Resource{
						Type:     ResourceTypeVM,
						Name:     fmt.Sprintf("vm-%d", i),
						Status:   StatusOnline,
						LastSeen: now,
						Proxmox: &ProxmoxData{
							NodeName: "node-1",
							VMID:     100 + i,
							CPUs:     4,
						},
						Metrics: &ResourceMetrics{
							CPU:    &MetricValue{Percent: 45.0, Source: SourceProxmox},
							Memory: &MetricValue{Percent: 62.0, Source: SourceProxmox},
						},
					},
					Identity: ResourceIdentity{
						Hostnames:   []string{fmt.Sprintf("vm-%d.local", i)},
						IPAddresses: []string{fmt.Sprintf("10.0.%d.%d", i/256, i%256)},
					},
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rr := NewRegistry(nil)
				rr.IngestRecords(SourceProxmox, records)
			}
		})
	}
}

// BenchmarkIngestRecords_MergeDedup measures ingestion when every record
// matches an existing resource via machine ID (confidence 1.0) and triggers
// the mergeInto path. This exercises identity lookup + merge overhead per
// resource — the hot path during periodic state refreshes.
func BenchmarkIngestRecords_MergeDedup(b *testing.B) {
	for _, count := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("hosts=%d", count), func(b *testing.B) {
			// Pre-populate the registry with hosts from Proxmox source.
			initial := make([]IngestRecord, count)
			now := time.Now().UTC()
			for i := range initial {
				machineID := fmt.Sprintf("machine-id-%d", i)
				initial[i] = IngestRecord{
					SourceID: fmt.Sprintf("node/pve-%d", i),
					Resource: Resource{
						Type:     ResourceTypeHost,
						Name:     fmt.Sprintf("pve-%d", i),
						Status:   StatusOnline,
						LastSeen: now,
						Proxmox:  &ProxmoxData{NodeName: fmt.Sprintf("pve-%d", i)},
						Metrics: &ResourceMetrics{
							CPU:    &MetricValue{Percent: 30.0, Source: SourceProxmox},
							Memory: &MetricValue{Percent: 50.0, Source: SourceProxmox},
						},
					},
					Identity: ResourceIdentity{
						MachineID:   machineID,
						Hostnames:   []string{fmt.Sprintf("pve-%d.local", i)},
						IPAddresses: []string{fmt.Sprintf("10.1.%d.%d", i/256, i%256)},
						MACAddresses: []string{
							fmt.Sprintf("aa:bb:cc:dd:%02x:%02x", i/256, i%256),
						},
					},
				}
			}

			// Build the incoming records from agent source — same machine IDs
			// trigger dedup merge.
			incoming := make([]IngestRecord, count)
			for i := range incoming {
				machineID := fmt.Sprintf("machine-id-%d", i)
				incoming[i] = IngestRecord{
					SourceID: fmt.Sprintf("agent-%d", i),
					Resource: Resource{
						Type:     ResourceTypeHost,
						Name:     fmt.Sprintf("pve-%d", i),
						Status:   StatusOnline,
						LastSeen: now.Add(time.Second),
						Agent: &AgentData{
							AgentID:  fmt.Sprintf("agent-%d", i),
							Hostname: fmt.Sprintf("pve-%d.local", i),
							Platform: "linux",
						},
						Tags: []string{"agent-managed"},
						Metrics: &ResourceMetrics{
							CPU:    &MetricValue{Percent: 35.0, Source: SourceAgent},
							Memory: &MetricValue{Percent: 55.0, Source: SourceAgent},
						},
					},
					Identity: ResourceIdentity{
						MachineID:   machineID,
						Hostnames:   []string{fmt.Sprintf("pve-%d.local", i)},
						IPAddresses: []string{fmt.Sprintf("10.1.%d.%d", i/256, i%256)},
						MACAddresses: []string{
							fmt.Sprintf("aa:bb:cc:dd:%02x:%02x", i/256, i%256),
						},
					},
				}
			}

			// Verify dedup actually merges (guard against silent regressions).
			{
				rr := NewRegistry(nil)
				rr.IngestRecords(SourceProxmox, initial)
				before := len(rr.List())
				rr.IngestRecords(SourceAgent, incoming)
				after := len(rr.List())
				if after != before {
					b.Fatalf("expected dedup merge to keep resource count at %d, got %d", before, after)
				}
				// Verify all hosts have both sources and payloads after merge.
				mergedCount := 0
				for _, r := range rr.List() {
					if r.Type != ResourceTypeHost {
						continue
					}
					if r.Proxmox == nil || r.Agent == nil {
						b.Fatalf("expected merged host %q to have both Proxmox and Agent payloads", r.ID)
					}
					hasProxmox, hasAgent := false, false
					for _, s := range r.Sources {
						if s == SourceProxmox {
							hasProxmox = true
						}
						if s == SourceAgent {
							hasAgent = true
						}
					}
					if !hasProxmox || !hasAgent {
						b.Fatalf("expected merged host %q to have both sources, got %v", r.ID, r.Sources)
					}
					mergedCount++
				}
				if mergedCount != count {
					b.Fatalf("expected %d merged hosts, got %d", count, mergedCount)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				rr := NewRegistry(nil)
				rr.IngestRecords(SourceProxmox, initial)
				b.StartTimer()

				rr.IngestRecords(SourceAgent, incoming)

				b.StopTimer()
			}
		})
	}
}

// BenchmarkFindCandidates measures the identity matcher's candidate lookup
// performance with a populated index. This is the core dedup decision point
// — called once per host ingest to determine if a merge is needed.
func BenchmarkFindCandidates(b *testing.B) {
	for _, indexed := range []int{50, 200, 500} {
		b.Run(fmt.Sprintf("indexed=%d", indexed), func(b *testing.B) {
			matcher := NewIdentityMatcher()
			for i := 0; i < indexed; i++ {
				matcher.Add(fmt.Sprintf("host-%d", i), ResourceIdentity{
					MachineID: fmt.Sprintf("mid-%d", i),
					DMIUUID:   fmt.Sprintf("dmi-%d", i),
					Hostnames: []string{
						fmt.Sprintf("host-%d.local", i),
						fmt.Sprintf("host-%d.example.com", i),
					},
					IPAddresses: []string{
						fmt.Sprintf("10.0.%d.%d", i/256, i%256),
						fmt.Sprintf("192.168.%d.%d", i/256, i%256),
					},
					MACAddresses: []string{
						fmt.Sprintf("aa:bb:cc:dd:%02x:%02x", i/256, i%256),
					},
				})
			}

			lookupHit := ResourceIdentity{
				MachineID:   "mid-42",
				Hostnames:   []string{"host-42.local"},
				IPAddresses: []string{"10.0.0.42"},
			}
			lookupMiss := ResourceIdentity{
				MachineID:   "nonexistent-machine-id",
				Hostnames:   []string{"unknown.local"},
				IPAddresses: []string{"203.0.113.1"},
			}

			// Verify hit/miss semantics before benchmarking.
			if hits := matcher.FindCandidates(lookupHit); len(hits) == 0 {
				b.Fatalf("expected FindCandidates(hit) to return candidates, got 0")
			}
			if misses := matcher.FindCandidates(lookupMiss); len(misses) != 0 {
				b.Fatalf("expected FindCandidates(miss) to return 0 candidates, got %d", len(misses))
			}

			b.Run("hit", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					matcher.FindCandidates(lookupHit)
				}
			})
			b.Run("miss", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					matcher.FindCandidates(lookupMiss)
				}
			})
		})
	}
}

// BenchmarkIngestMixed simulates a realistic state consolidation cycle:
// a mix of hosts, VMs, and containers ingested via IngestRecords, with
// some hosts triggering identity-based dedup merges from agent data.
func BenchmarkIngestMixed(b *testing.B) {
	const (
		numHosts   = 20
		vmsPerHost = 10
		lxcPerHost = 5
	)

	now := time.Now().UTC()

	// Build host records from Proxmox.
	hostRecords := make([]IngestRecord, numHosts)
	for i := range hostRecords {
		hostRecords[i] = IngestRecord{
			SourceID: fmt.Sprintf("node/pve-%d", i),
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     fmt.Sprintf("pve-%d", i),
				Status:   StatusOnline,
				LastSeen: now,
				Proxmox:  &ProxmoxData{NodeName: fmt.Sprintf("pve-%d", i), CPUs: 16},
				Metrics: &ResourceMetrics{
					CPU:    &MetricValue{Percent: float64(20 + i%60), Source: SourceProxmox},
					Memory: &MetricValue{Percent: float64(40 + i%40), Source: SourceProxmox},
				},
			},
			Identity: ResourceIdentity{
				MachineID:    fmt.Sprintf("mid-host-%d", i),
				Hostnames:    []string{fmt.Sprintf("pve-%d.local", i)},
				IPAddresses:  []string{fmt.Sprintf("10.0.0.%d", i+1)},
				MACAddresses: []string{fmt.Sprintf("aa:bb:cc:00:%02x:%02x", i/256, i%256)},
			},
		}
	}

	// Build VM records.
	vmRecords := make([]IngestRecord, numHosts*vmsPerHost)
	for h := 0; h < numHosts; h++ {
		for v := 0; v < vmsPerHost; v++ {
			idx := h*vmsPerHost + v
			vmRecords[idx] = IngestRecord{
				SourceID:       fmt.Sprintf("vm-%d", 100+idx),
				ParentSourceID: fmt.Sprintf("node/pve-%d", h),
				Resource: Resource{
					Type:     ResourceTypeVM,
					Name:     fmt.Sprintf("vm-%d", 100+idx),
					Status:   StatusOnline,
					LastSeen: now,
					Proxmox:  &ProxmoxData{NodeName: fmt.Sprintf("pve-%d", h), VMID: 100 + idx, CPUs: 2},
					Metrics: &ResourceMetrics{
						CPU:    &MetricValue{Percent: float64(idx % 100), Source: SourceProxmox},
						Memory: &MetricValue{Percent: float64(30 + idx%50), Source: SourceProxmox},
					},
				},
				Identity: ResourceIdentity{
					Hostnames:   []string{fmt.Sprintf("vm-%d.local", 100+idx)},
					IPAddresses: []string{fmt.Sprintf("10.1.%d.%d", idx/256, idx%256)},
				},
			}
		}
	}

	// Build LXC records.
	lxcRecords := make([]IngestRecord, numHosts*lxcPerHost)
	for h := 0; h < numHosts; h++ {
		for c := 0; c < lxcPerHost; c++ {
			idx := h*lxcPerHost + c
			lxcRecords[idx] = IngestRecord{
				SourceID:       fmt.Sprintf("lxc-%d", 500+idx),
				ParentSourceID: fmt.Sprintf("node/pve-%d", h),
				Resource: Resource{
					Type:     ResourceTypeSystemContainer,
					Name:     fmt.Sprintf("lxc-%d", 500+idx),
					Status:   StatusOnline,
					LastSeen: now,
					Proxmox:  &ProxmoxData{NodeName: fmt.Sprintf("pve-%d", h), VMID: 500 + idx, CPUs: 1},
					Metrics: &ResourceMetrics{
						CPU:    &MetricValue{Percent: float64(idx % 80), Source: SourceProxmox},
						Memory: &MetricValue{Percent: float64(20 + idx%60), Source: SourceProxmox},
					},
				},
				Identity: ResourceIdentity{
					Hostnames: []string{fmt.Sprintf("lxc-%d.local", 500+idx)},
				},
			}
		}
	}

	// Agent records for half the hosts — these will trigger identity-based
	// dedup merges via matching machine IDs.
	agentRecords := make([]IngestRecord, numHosts/2)
	for i := range agentRecords {
		agentRecords[i] = IngestRecord{
			SourceID: fmt.Sprintf("agent-host-%d", i),
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     fmt.Sprintf("pve-%d", i),
				Status:   StatusOnline,
				LastSeen: now.Add(time.Second),
				Agent: &AgentData{
					AgentID:  fmt.Sprintf("agent-host-%d", i),
					Hostname: fmt.Sprintf("pve-%d.local", i),
					Platform: "linux",
				},
				Metrics: &ResourceMetrics{
					CPU:    &MetricValue{Percent: float64(25 + i%50), Source: SourceAgent},
					Memory: &MetricValue{Percent: float64(45 + i%35), Source: SourceAgent},
				},
			},
			Identity: ResourceIdentity{
				MachineID:    fmt.Sprintf("mid-host-%d", i),
				Hostnames:    []string{fmt.Sprintf("pve-%d.local", i)},
				IPAddresses:  []string{fmt.Sprintf("10.0.0.%d", i+1)},
				MACAddresses: []string{fmt.Sprintf("aa:bb:cc:00:%02x:%02x", i/256, i%256)},
			},
		}
	}

	// Verify mixed ingest produces expected resource count (dedup merges half the hosts).
	{
		rr := NewRegistry(nil)
		rr.IngestRecords(SourceProxmox, hostRecords)
		rr.IngestRecords(SourceProxmox, vmRecords)
		rr.IngestRecords(SourceProxmox, lxcRecords)
		rr.IngestRecords(SourceAgent, agentRecords)
		total := len(rr.List())
		expected := numHosts + numHosts*vmsPerHost + numHosts*lxcPerHost // agents merge into existing hosts
		if total != expected {
			b.Fatalf("expected %d resources after mixed ingest, got %d", expected, total)
		}
		// Verify all agent hosts were merged into existing Proxmox hosts.
		mergedHostCount := 0
		for _, r := range rr.List() {
			if r.Type != ResourceTypeHost || r.Agent == nil {
				continue
			}
			if r.Proxmox == nil {
				b.Fatalf("expected merged host %q to have Proxmox payload after mixed ingest", r.ID)
			}
			mergedHostCount++
		}
		if mergedHostCount != numHosts/2 {
			b.Fatalf("expected %d merged hosts with Agent payload, got %d", numHosts/2, mergedHostCount)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rr := NewRegistry(nil)
		rr.IngestRecords(SourceProxmox, hostRecords)
		rr.IngestRecords(SourceProxmox, vmRecords)
		rr.IngestRecords(SourceProxmox, lxcRecords)
		rr.IngestRecords(SourceAgent, agentRecords)
	}
}

// BenchmarkIdentityMatcher_Add measures the cost of indexing a new identity
// into the matcher — called once per new resource during ingest.
func BenchmarkIdentityMatcher_Add(b *testing.B) {
	const numIdentities = 200

	identities := make([]ResourceIdentity, numIdentities)
	resourceIDs := make([]string, numIdentities)
	for i := range identities {
		resourceIDs[i] = fmt.Sprintf("host-%d", i)
		identities[i] = ResourceIdentity{
			MachineID: fmt.Sprintf("mid-%d", i),
			DMIUUID:   fmt.Sprintf("dmi-%d", i),
			Hostnames: []string{
				fmt.Sprintf("host-%d.local", i),
				fmt.Sprintf("host-%d.example.com", i),
			},
			IPAddresses: []string{
				fmt.Sprintf("10.0.%d.%d", i/256, i%256),
			},
			MACAddresses: []string{
				fmt.Sprintf("aa:bb:cc:dd:%02x:%02x", i/256, i%256),
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		matcher := NewIdentityMatcher()
		for j, id := range identities {
			matcher.Add(resourceIDs[j], id)
		}
	}
}
