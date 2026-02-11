package unifiedresources

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestResourceRegistry_ReadAPIsReturnDefensiveCopies(t *testing.T) {
	rr := NewRegistry(nil)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "host-1",
				Status:   StatusOnline,
				LastSeen: time.Now().UTC(),
				Tags:     []string{"prod"},
				Agent: &AgentData{
					Hostname: "host-1.local",
					NetworkInterfaces: []NetworkInterface{
						{Name: "eth0", Addresses: []string{"10.0.0.11"}},
					},
					Sensors: &HostSensorMeta{
						TemperatureCelsius: map[string]float64{"cpu.package": 55.5},
					},
				},
			},
			Identity: ResourceIdentity{
				Hostnames:   []string{"host-1.local"},
				IPAddresses: []string{"10.0.0.11"},
			},
		},
	})

	rr.IngestRecords(SourceK8s, []IngestRecord{
		{
			SourceID: "cluster-a:pod:ns/pod-a",
			Resource: Resource{
				Type:     ResourceTypePod,
				Name:     "pod-a",
				Status:   StatusOnline,
				LastSeen: time.Now().UTC(),
				Kubernetes: &K8sData{
					ClusterName: "cluster-a",
					Namespace:   "ns",
					PodUID:      "pod-a",
					Labels:      map[string]string{"app": "api"},
				},
			},
		},
	})

	hostID := rr.sourceSpecificID(ResourceTypeHost, SourceAgent, "host-1")
	got, ok := rr.Get(hostID)
	if !ok || got == nil {
		t.Fatalf("expected Get(%q) to succeed", hostID)
	}
	got.Tags[0] = "mutated"
	got.Identity.Hostnames[0] = "changed.local"
	got.Agent.NetworkInterfaces[0].Addresses[0] = "192.0.2.1"
	got.SourceStatus[SourceAgent] = SourceStatus{Status: "offline"}

	afterGet, ok := rr.Get(hostID)
	if !ok || afterGet == nil {
		t.Fatalf("expected Get(%q) to succeed after mutation", hostID)
	}
	if afterGet.Tags[0] != "prod" {
		t.Fatalf("expected Get() tags to remain unchanged, got %v", afterGet.Tags)
	}
	if afterGet.Identity.Hostnames[0] != "host-1.local" {
		t.Fatalf("expected Get() hostnames to remain unchanged, got %v", afterGet.Identity.Hostnames)
	}
	if afterGet.Agent.NetworkInterfaces[0].Addresses[0] != "10.0.0.11" {
		t.Fatalf("expected Get() network addresses to remain unchanged, got %v", afterGet.Agent.NetworkInterfaces[0].Addresses)
	}
	if status := afterGet.SourceStatus[SourceAgent].Status; status != "online" {
		t.Fatalf("expected Get() source status to remain online, got %q", status)
	}

	list := rr.List()
	if len(list) == 0 {
		t.Fatalf("expected non-empty list")
	}
	list[0].Tags = append(list[0].Tags, "injected")

	afterList, ok := rr.Get(hostID)
	if !ok || afterList == nil {
		t.Fatalf("expected Get(%q) to succeed after list mutation", hostID)
	}
	if len(afterList.Tags) != 1 {
		t.Fatalf("expected list mutation to not affect registry tags, got %v", afterList.Tags)
	}

	hosts := rr.Hosts()
	if len(hosts) != 1 {
		t.Fatalf("expected one host view, got %d", len(hosts))
	}
	hostView := hosts[0]
	hostTags := hostView.Tags()
	hostTags[0] = "host-mutated"
	hostNics := hostView.NetworkInterfaces()
	hostNics[0].Addresses[0] = "203.0.113.1"
	hostSensors := hostView.Sensors()
	hostSensors.TemperatureCelsius["cpu.package"] = 99.9

	hostsAgain := rr.Hosts()
	if len(hostsAgain) != 1 {
		t.Fatalf("expected one host view on second read, got %d", len(hostsAgain))
	}
	if hostsAgain[0].Tags()[0] != "prod" {
		t.Fatalf("expected host tags to be detached from caller mutation, got %v", hostsAgain[0].Tags())
	}
	if hostsAgain[0].NetworkInterfaces()[0].Addresses[0] != "10.0.0.11" {
		t.Fatalf("expected host interfaces to be detached from caller mutation, got %v", hostsAgain[0].NetworkInterfaces()[0].Addresses)
	}
	if hostsAgain[0].Sensors().TemperatureCelsius["cpu.package"] != 55.5 {
		t.Fatalf("expected host sensors to be detached from caller mutation, got %v", hostsAgain[0].Sensors().TemperatureCelsius)
	}

	pods := rr.Pods()
	if len(pods) != 1 {
		t.Fatalf("expected one pod view, got %d", len(pods))
	}
	labels := pods[0].Labels()
	labels["app"] = "mutated"

	podsAgain := rr.Pods()
	if len(podsAgain) != 1 {
		t.Fatalf("expected one pod view on second read, got %d", len(podsAgain))
	}
	if podsAgain[0].Labels()["app"] != "api" {
		t.Fatalf("expected pod labels to be detached from caller mutation, got %v", podsAgain[0].Labels())
	}
}

func TestResourceRegistry_ConcurrentIngestAndRead(t *testing.T) {
	rr := NewRegistry(nil)

	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			rr.IngestRecords(SourceAgent, []IngestRecord{
				{
					SourceID: "host-1",
					Resource: Resource{
						Type:     ResourceTypeHost,
						Name:     fmt.Sprintf("host-%d", i),
						Status:   StatusOnline,
						LastSeen: time.Now().UTC(),
						Tags:     []string{"prod", fmt.Sprintf("iter:%d", i)},
						Agent: &AgentData{
							Hostname: "host-1.local",
							NetworkInterfaces: []NetworkInterface{
								{Name: "eth0", Addresses: []string{fmt.Sprintf("10.0.0.%d", (i%200)+1)}},
							},
							Sensors: &HostSensorMeta{
								TemperatureCelsius: map[string]float64{"cpu.package": float64(45 + (i % 10))},
							},
						},
					},
					Identity: ResourceIdentity{
						Hostnames:   []string{"host-1.local"},
						IPAddresses: []string{fmt.Sprintf("10.0.0.%d", (i%200)+1)},
					},
				},
			})

			rr.IngestRecords(SourceK8s, []IngestRecord{
				{
					SourceID: "cluster-a:pod:ns/pod-a",
					Resource: Resource{
						Type:     ResourceTypePod,
						Name:     "pod-a",
						Status:   StatusOnline,
						LastSeen: time.Now().UTC(),
						Kubernetes: &K8sData{
							ClusterName: "cluster-a",
							Namespace:   "ns",
							PodUID:      "pod-a",
							Labels: map[string]string{
								"app":  "api",
								"iter": fmt.Sprintf("%d", i),
							},
						},
					},
				},
			})
		}
	}()

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				resources := rr.List()
				if len(resources) > 0 {
					resources[0].Tags = append(resources[0].Tags, "caller-mutated")
				}

				hosts := rr.Hosts()
				if len(hosts) > 0 {
					tags := hosts[0].Tags()
					if len(tags) > 0 {
						tags[0] = "mutated"
					}
					nics := hosts[0].NetworkInterfaces()
					if len(nics) > 0 && len(nics[0].Addresses) > 0 {
						nics[0].Addresses[0] = "198.51.100.1"
					}
					sensors := hosts[0].Sensors()
					if sensors != nil && sensors.TemperatureCelsius != nil {
						sensors.TemperatureCelsius["cpu.package"] = 100
					}
				}

				pods := rr.Pods()
				if len(pods) > 0 {
					labels := pods[0].Labels()
					if labels != nil {
						labels["app"] = "caller-mutated"
					}
				}
			}
		}()
	}

	wg.Wait()

	if len(rr.List()) == 0 {
		t.Fatalf("expected resources after concurrent ingest/read")
	}
}
