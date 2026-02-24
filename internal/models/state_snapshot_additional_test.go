package models

import (
	"testing"
	"time"
)

func TestStateUpdateRecentlyResolvedSnapshotIsolation(t *testing.T) {
	state := NewState()
	resolvedAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	input := []ResolvedAlert{
		{
			Alert: Alert{
				ID:           "alert-1",
				ResourceName: "node-a",
			},
			ResolvedTime: resolvedAt,
		},
	}

	state.UpdateRecentlyResolved(input)
	snapshot := state.GetSnapshot()
	if len(snapshot.RecentlyResolved) != 1 {
		t.Fatalf("RecentlyResolved length = %d, want 1", len(snapshot.RecentlyResolved))
	}
	if snapshot.RecentlyResolved[0].ID != "alert-1" {
		t.Fatalf("snapshot alert id = %q, want alert-1", snapshot.RecentlyResolved[0].ID)
	}

	snapshot.RecentlyResolved[0].ID = "mutated-snapshot"
	latest := state.GetSnapshot()
	if latest.RecentlyResolved[0].ID != "alert-1" {
		t.Fatalf("state changed with snapshot mutation: %q", latest.RecentlyResolved[0].ID)
	}
}

func TestStateSnapshotResolveResourceDockerAndKubernetesRouting(t *testing.T) {
	snapshot := StateSnapshot{
		Nodes: []Node{
			{Name: "pve-node"},
		},
		VMs: []VM{
			{Name: "vm-app", VMID: 101, Node: "pve-node"},
			{Name: "docker-vm", VMID: 102, Node: "pve-node"},
		},
		Containers: []Container{
			{Name: "lxc-app", VMID: 201, Node: "pve-node"},
			{Name: "docker-lxc", VMID: 202, Node: "pve-node"},
		},
		DockerHosts: []DockerHost{
			{
				ID:       "dh-lxc-id",
				Hostname: "docker-lxc",
				Containers: []DockerContainer{
					{Name: "ctr-on-lxc"},
				},
			},
			{
				ID:       "dh-vm-id",
				Hostname: "docker-vm",
				Containers: []DockerContainer{
					{Name: "ctr-on-vm"},
				},
			},
			{
				ID:       "dh-standalone-id",
				Hostname: "docker-standalone",
				Containers: []DockerContainer{
					{Name: "ctr-on-standalone"},
				},
			},
		},
		Hosts: []Host{
			{ID: "host-1", Hostname: "linux-1", Platform: "linux"},
		},
		KubernetesClusters: []KubernetesCluster{
			{
				ID:          "k8s-id",
				AgentID:     "agent-1",
				Name:        "k8s-main",
				DisplayName: "K8S Main",
				Pods: []KubernetesPod{
					{Name: "pod-a", Namespace: "ns-a"},
				},
				Deployments: []KubernetesDeployment{
					{Name: "deploy-a", Namespace: "ns-b"},
				},
			},
		},
	}

	testCases := []struct {
		name        string
		query       string
		wantType    string
		wantTarget  string
		wantName    string
		wantAgentID string
	}{
		{
			name:       "node",
			query:      "pve-node",
			wantType:   "node",
			wantTarget: "pve-node",
			wantName:   "pve-node",
		},
		{
			name:       "vm",
			query:      "vm-app",
			wantType:   "vm",
			wantTarget: "vm-app",
			wantName:   "vm-app",
		},
		{
			name:       "lxc",
			query:      "lxc-app",
			wantType:   "system-container",
			wantTarget: "lxc-app",
			wantName:   "lxc-app",
		},
		{
			name:       "docker host by id",
			query:      "dh-vm-id",
			wantType:   "dockerhost",
			wantTarget: "docker-vm",
			wantName:   "docker-vm",
		},
		{
			name:       "docker container on lxc routes to lxc host",
			query:      "ctr-on-lxc",
			wantType:   "docker",
			wantTarget: "docker-lxc",
			wantName:   "ctr-on-lxc",
		},
		{
			name:       "docker container on vm routes to vm host",
			query:      "ctr-on-vm",
			wantType:   "docker",
			wantTarget: "docker-vm",
			wantName:   "ctr-on-vm",
		},
		{
			name:       "docker container on standalone routes to docker host",
			query:      "ctr-on-standalone",
			wantType:   "docker",
			wantTarget: "docker-standalone",
			wantName:   "ctr-on-standalone",
		},
		{
			name:       "host by id",
			query:      "host-1",
			wantType:   "host",
			wantTarget: "linux-1",
			wantName:   "linux-1",
		},
		{
			name:        "kubernetes cluster by display name",
			query:       "K8S Main",
			wantType:    "k8s_cluster",
			wantTarget:  "k8s-main",
			wantName:    "k8s-main",
			wantAgentID: "agent-1",
		},
		{
			name:        "kubernetes pod",
			query:       "pod-a",
			wantType:    "k8s_pod",
			wantTarget:  "k8s-main",
			wantName:    "pod-a",
			wantAgentID: "agent-1",
		},
		{
			name:        "kubernetes deployment",
			query:       "deploy-a",
			wantType:    "k8s_deployment",
			wantTarget:  "k8s-main",
			wantName:    "deploy-a",
			wantAgentID: "agent-1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loc := snapshot.ResolveResource(tc.query)
			if !loc.Found {
				t.Fatalf("ResolveResource(%q) not found", tc.query)
			}
			if loc.ResourceType != tc.wantType {
				t.Fatalf("resource type = %q, want %q", loc.ResourceType, tc.wantType)
			}
			if loc.TargetHost != tc.wantTarget {
				t.Fatalf("target host = %q, want %q", loc.TargetHost, tc.wantTarget)
			}
			if loc.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", loc.Name, tc.wantName)
			}
			if tc.wantAgentID != "" && loc.AgentID != tc.wantAgentID {
				t.Fatalf("agent id = %q, want %q", loc.AgentID, tc.wantAgentID)
			}
		})
	}

	notFound := snapshot.ResolveResource("does-not-exist")
	if notFound.Found {
		t.Fatalf("ResolveResource should report not found for missing resource")
	}
	if notFound.Name != "does-not-exist" {
		t.Fatalf("missing resource name = %q, want does-not-exist", notFound.Name)
	}
}
