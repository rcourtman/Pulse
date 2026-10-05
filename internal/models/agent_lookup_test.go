package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

var agentLookupOmittedCollections = []string{
	"Namespaces", "Services", "Ingresses", "EndpointSlices", "NetworkPolicies",
	"PersistentVolumes", "PersistentVolumeClaims", "StorageClasses", "ConfigMaps",
	"Secrets", "ServiceAccounts", "Roles", "ClusterRoles", "RoleBindings",
	"ClusterRoleBindings", "ResourceQuotas", "LimitRanges", "PodDisruptionBudgets",
	"HorizontalPodAutoscalers", "Events",
}

func TestAgentLookupSnapshotPreservesEveryOtherField(t *testing.T) {
	cluster := KubernetesCluster{ID: "cluster", AgentID: "agent", Name: "cluster", TokenID: "token", LastSeen: time.Now().UTC()}
	// Populate every collection, not just the fields currently omitted. New
	// fields must stay on the ordinary clone path unless explicitly reviewed.
	value := reflect.ValueOf(&cluster).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() == reflect.Slice {
			field.Set(reflect.MakeSlice(field.Type(), 1, 1))
			if uid := field.Index(0).FieldByName("UID"); uid.IsValid() {
				uid.SetString(value.Type().Field(i).Name)
			}
		}
	}
	s := NewState()
	s.UpsertKubernetesCluster(cluster)
	s.UpsertHost(Host{ID: "host", Tags: []string{"tag"}, LastSeen: cluster.LastSeen})
	s.UpsertDockerHost(DockerHost{ID: "docker", Containers: []DockerContainer{{ID: "app", Labels: map[string]string{"app": "web"}}}})
	s.PVEBackups = PVEBackups{GuestSnapshots: []GuestSnapshot{{VMID: 100}}}
	s.ConnectionHealth["cluster"] = true
	before := s.GetSnapshot()
	want := before.Clone()
	wantCluster := reflect.ValueOf(&want.KubernetesClusters[0]).Elem()
	for _, name := range agentLookupOmittedCollections {
		field := wantCluster.FieldByName(name)
		field.Set(reflect.MakeSlice(field.Type(), 0, 0))
	}
	got := s.GetAgentLookupSnapshot()
	if !reflect.DeepEqual(got, want) {
		t.Fatal("agent lookup projection changed more than the explicitly omitted metadata collections")
	}
	got.KubernetesClusters[0].Nodes[0].Roles = append(got.KubernetesClusters[0].Nodes[0].Roles, "changed")
	got.Hosts[0].Tags[0] = "changed"
	got.DockerHosts[0].Containers[0].Labels["app"] = "changed"
	got.ConnectionHealth["cluster"] = false
	if !reflect.DeepEqual(before, s.GetSnapshot()) {
		t.Fatal("lookup projection aliases the full source inventory")
	}
}

func TestStatePointLookupsMatchWholeLists(t *testing.T) {
	now := time.Now().UTC()
	s := NewState()
	s.UpsertHost(Host{
		ID: "host", Tags: []string{"tag"}, NetworkInterfaces: []HostNetworkInterface{{Addresses: []string{"192.0.2.1"}}},
		Sensors:       HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 40}},
		AppliedConfig: &AgentConfigFingerprint{Version: "1", Hash: "hash"},
		AgentModules:  []AgentModuleStatus{{Name: "host", State: "running"}},
		AgentUpdate:   &AgentUpdateStatus{State: "idle", LastCheckedAt: &now, LastAttemptAt: &now, LastSuccessAt: &now},
	})
	s.UpsertDockerHost(DockerHost{
		ID: "docker", AgentID: "collector", Containers: []DockerContainer{{ID: "app", Labels: map[string]string{"app": "web"}}},
		NetworkInterfaces: []HostNetworkInterface{{Addresses: []string{"192.0.2.1"}}},
		AgentModules:      []AgentModuleStatus{{Name: "docker", State: "running"}},
	})
	for _, id := range []string{"", "missing", " host ", "host", "docker", "collector"} {
		wantHost, wantDocker := Host{}, DockerHost{}
		hostFound, dockerFound := false, false
		for _, host := range s.GetHosts() {
			if host.ID == id {
				wantHost, hostFound = host, true
				break
			}
		}
		for _, host := range s.GetDockerHosts() {
			if host.ID == id {
				wantDocker, dockerFound = host, true
				break
			}
		}
		gotHost, gotHostFound := s.GetHost(id)
		gotDocker, gotDockerFound := s.GetDockerHost(id)
		if gotHostFound != hostFound || gotDockerFound != dockerFound || !reflect.DeepEqual(gotHost, wantHost) || !reflect.DeepEqual(gotDocker, wantDocker) {
			t.Fatalf("exact source lookup %q differs from the existing whole-list lookup", id)
		}
	}
	// Freeze the expectation independently of the clone under test. A
	// defective clone must not mutate both the source and its reference.
	before, err := json.Marshal(s.GetSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	for _, fromList := range []bool{false, true} {
		host, _ := s.GetHost("host")
		docker, _ := s.GetDockerHost("docker")
		if fromList {
			host, docker = s.GetHosts()[0], s.GetDockerHosts()[0]
		}
		host.Tags[0] = "changed"
		host.NetworkInterfaces[0].Addresses[0] = "changed"
		host.Sensors.TemperatureCelsius["cpu"] = 99
		host.AppliedConfig.Hash = "changed"
		host.AgentModules[0].State = "changed"
		host.AgentUpdate.State = "changed"
		*host.AgentUpdate.LastCheckedAt = time.Time{}
		*host.AgentUpdate.LastAttemptAt = time.Time{}
		*host.AgentUpdate.LastSuccessAt = time.Time{}
		docker.Containers[0].Labels["app"] = "changed"
		docker.NetworkInterfaces[0].Addresses[0] = "changed"
		docker.AgentModules[0].State = "changed"
		after, err := json.Marshal(s.GetSnapshot())
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("point or list result aliases mutable source fields")
		}
	}
}

func TestStatePointLookupAllocationsAndConcurrentOwnership(t *testing.T) {
	s := NewState()
	for i := 0; i < 17; i++ {
		id := fmt.Sprint(i)
		s.UpsertHost(Host{ID: id, Tags: []string{"tag"}})
		s.UpsertDockerHost(DockerHost{ID: id, Containers: []DockerContainer{{ID: "app", Labels: map[string]string{"key": "value"}}}})
	}
	for _, lookup := range []func(string){
		func(id string) { _, _ = s.GetHost(id) },
		func(id string) { _, _ = s.GetDockerHost(id) },
	} {
		if allocations := testing.AllocsPerRun(10, func() { lookup("missing") }); allocations != 0 {
			t.Fatalf("missing exact record allocated %.0f times", allocations)
		}
	}
	var workers sync.WaitGroup
	for i := 0; i < 3; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for j := 0; j < 20; j++ {
				id := fmt.Sprint(i)
				if i == 0 {
					s.UpsertHost(Host{ID: id, Tags: []string{"tag"}})
					s.UpsertDockerHost(DockerHost{ID: id, Containers: []DockerContainer{{ID: "app", Labels: map[string]string{"key": "value"}}}})
				} else {
					host, _ := s.GetHost("0")
					docker, _ := s.GetDockerHost("0")
					host.Tags[0] = "caller-owned"
					docker.Containers[0].Labels["key"] = "caller-owned"
					_ = s.GetAgentLookupSnapshot()
				}
			}
		}(i)
	}
	workers.Wait()
}
