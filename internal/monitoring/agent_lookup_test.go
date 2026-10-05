package monitoring

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These collections cannot contribute a host facet. Workloads, Kubernetes
// nodes and all non-Kubernetes sources deliberately remain in the lookup.
var agentLookupMetadataCollections = []string{
	"Namespaces", "Services", "Ingresses", "EndpointSlices", "NetworkPolicies",
	"PersistentVolumes", "PersistentVolumeClaims", "StorageClasses", "ConfigMaps",
	"Secrets", "ServiceAccounts", "Roles", "ClusterRoles", "RoleBindings",
	"ClusterRoleBindings", "ResourceQuotas", "LimitRanges", "PodDisruptionBudgets",
	"HorizontalPodAutoscalers", "Events",
}

func agentLookupFixture(metadataCount int) *Monitor {
	now := time.Now().UTC()
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID: "host", Hostname: "worker", MachineID: "machine", Status: "online", LastSeen: now,
		TokenID: "host-token", Tags: []string{"host-tag"},
		NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.1"}}},
	})
	state.UpsertDockerHost(models.DockerHost{
		ID: "docker-host", AgentID: "host", Hostname: "worker", MachineID: "machine",
		Status: "online", LastSeen: now, TokenID: "docker-token", CPUs: 4,
		Containers: []models.DockerContainer{{ID: "app", Name: "app", State: "running", Labels: map[string]string{"app": "web"}}},
	})
	state.UpdateNodes([]models.Node{{ID: "pve:worker", Name: "worker", Instance: "pve", LinkedAgentID: "host", Status: "online", LastSeen: now}})
	state.UpdateVMs([]models.VM{{ID: "pve:worker:100", Name: "vm", VMID: 100, Node: "worker", Instance: "pve", Status: "running", LastSeen: now}})
	state.UpdateContainers([]models.Container{{ID: "pve:worker:101", Name: "lxc", VMID: 101, Node: "worker", Instance: "pve", Status: "running", LastSeen: now}})
	cluster := models.KubernetesCluster{
		ID: "cluster", AgentID: "host", Name: "cluster", Status: "online", LastSeen: now,
		Nodes:       []models.KubernetesNode{{UID: "node", Name: "worker", MachineID: "machine", Ready: true}},
		Pods:        []models.KubernetesPod{{UID: "pod", Name: "pod", NodeName: "worker", Namespace: "ns", Phase: "Running"}},
		Deployments: []models.KubernetesDeployment{{UID: "deployment", Name: "deployment", Namespace: "ns"}},
	}
	value := reflect.ValueOf(&cluster).Elem()
	for i := 0; i < metadataCount; i++ {
		field := value.FieldByName(agentLookupMetadataCollections[i%len(agentLookupMetadataCollections)])
		entry := reflect.New(field.Type().Elem()).Elem()
		// Deliberately overlap metadata names/UIDs with host identities. Type,
		// not a friendly naming convention, must keep these out of admission.
		for key, text := range map[string]string{"UID": fmt.Sprintf("host-%d", i), "Name": "worker", "Namespace": "ns"} {
			if f := entry.FieldByName(key); f.IsValid() && f.Kind() == reflect.String {
				f.SetString(text)
			}
		}
		field.Set(reflect.Append(field, entry))
	}
	state.UpsertKubernetesCluster(cluster)
	return &Monitor{state: state, config: &config.Config{}}
}

func fullAgentLookupReference(m *Monitor) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	thresholds := m.resourceStaleThresholds()
	registry.IngestSnapshotWithStaleThresholds(m.state.GetSnapshot(), thresholds)
	return unifiedresources.NewMonitorAdapterWithStaleThresholds(registry, thresholds)
}

func TestAgentLookupMatchesWholeSnapshotHostViews(t *testing.T) {
	for _, count := range []int{0, 20, 1000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := agentLookupFixture(count)
			before := m.state.GetSnapshot()
			want := fullAgentLookupReference(m)
			got := m.snapshotBackedUnifiedReadState()
			if len(want.Hosts()) == 0 || len(want.DockerHosts()) == 0 {
				t.Fatal("fixture must exercise both host facets")
			}
			if !reflect.DeepEqual(got.Hosts(), want.Hosts()) || !reflect.DeepEqual(got.DockerHosts(), want.DockerHosts()) {
				t.Fatal("agent admission changed a canonical host or Docker host view")
			}
			if !reflect.DeepEqual(before, m.state.GetSnapshot()) {
				t.Fatal("agent lookup mutated the full inventory")
			}
			// A second report must see current source data without a cache or
			// freshness grace period, including explicit clears and removal.
			m.state.UpsertHost(models.Host{ID: "host", Hostname: "renamed", MachineID: "machine", Status: "online", LastSeen: time.Now().UTC()})
			m.state.RemoveDockerHost("docker-host")
			want = fullAgentLookupReference(m)
			got = m.snapshotBackedUnifiedReadState()
			if !reflect.DeepEqual(got.Hosts(), want.Hosts()) || !reflect.DeepEqual(got.DockerHosts(), want.DockerHosts()) {
				t.Fatal("agent lookup retained a removed host or old metadata")
			}
		})
	}
}

func TestAgentLookupAllocationIgnoresUnrelatedKubernetesMetadata(t *testing.T) {
	measure := func(count int) float64 {
		m := agentLookupFixture(count)
		return testing.AllocsPerRun(3, func() {
			read := m.snapshotBackedUnifiedReadState()
			if len(read.Hosts()) == 0 || len(read.DockerHosts()) == 0 {
				panic("missing fixture host")
			}
		})
	}
	small, large := measure(0), measure(1000)
	t.Logf("agent identity lookup allocations: no metadata=%.0f, 1000 metadata objects=%.0f", small, large)
	if large > small+10 {
		t.Fatalf("unrelated Kubernetes metadata still adds identity lookup allocations: %.0f -> %.0f", small, large)
	}
}

func TestAgentPointLookupAllocationIgnoresOtherHosts(t *testing.T) {
	measure := func(count int) float64 {
		m := &Monitor{state: models.NewState()}
		for i := 0; i < count; i++ {
			m.state.UpsertHost(models.Host{ID: fmt.Sprint(i), Hostname: fmt.Sprint(i), Tags: []string{"tag"}, Sensors: models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 40}}})
		}
		return testing.AllocsPerRun(10, func() {
			if _, ok := m.hostByID("0"); !ok {
				panic("missing fixture host")
			}
		})
	}
	small, large := measure(1), measure(17)
	t.Logf("native host point lookup allocations: one host=%.0f, 17 hosts=%.0f", small, large)
	if large > small+1 {
		t.Fatalf("point lookup still clones unrelated hosts: %.0f -> %.0f", small, large)
	}
}

func BenchmarkAgentIdentityLookup(b *testing.B) {
	for _, count := range []int{0, 1000} {
		m := agentLookupFixture(count)
		for _, reference := range []bool{true, false} {
			name := "scoped"
			if reference {
				name = "whole-snapshot-reference"
			}
			b.Run(fmt.Sprintf("%s/metadata-%d", name, count), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var read unifiedresources.ReadState
					if reference {
						read = fullAgentLookupReference(m)
					} else {
						read = m.snapshotBackedUnifiedReadState()
					}
					if len(read.Hosts()) == 0 || len(read.DockerHosts()) == 0 {
						b.Fatal("missing fixture host")
					}
				}
			})
		}
	}
}
