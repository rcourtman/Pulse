package models

import (
	"strings"
	"testing"
	"time"
)

func TestStateClearAllDockerHosts(t *testing.T) {
	state := &State{
		DockerHosts: []DockerHost{
			{ID: "h1"},
			{ID: "h2"},
		},
	}

	count := state.ClearAllDockerHosts()
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if len(state.DockerHosts) != 0 {
		t.Fatalf("DockerHosts = %#v, want empty", state.DockerHosts)
	}
}

func TestStateKubernetesClusterLifecycle(t *testing.T) {
	state := &State{}

	initial := KubernetesCluster{
		ID:                "c1",
		Name:              "alpha",
		CustomDisplayName: "keep",
		Hidden:            true,
		PendingUninstall:  true,
		Status:            "init",
	}
	state.UpsertKubernetesCluster(initial)

	update := KubernetesCluster{
		ID:                "c1",
		Name:              "alpha",
		CustomDisplayName: "",
		Hidden:            false,
		PendingUninstall:  false,
		Status:            "ready",
	}
	state.UpsertKubernetesCluster(update)

	clusters := state.GetKubernetesClusters()
	if len(clusters) != 1 {
		t.Fatalf("clusters = %#v, want 1", clusters)
	}
	if clusters[0].CustomDisplayName != "keep" {
		t.Fatalf("CustomDisplayName = %q, want keep", clusters[0].CustomDisplayName)
	}
	if !clusters[0].Hidden || !clusters[0].PendingUninstall {
		t.Fatalf("expected Hidden and PendingUninstall preserved")
	}

	if ok := state.SetKubernetesClusterStatus("c1", "ok"); !ok {
		t.Fatalf("SetKubernetesClusterStatus returned false")
	}
	if _, ok := state.SetKubernetesClusterHidden("c1", false); !ok {
		t.Fatalf("SetKubernetesClusterHidden returned false")
	}
	if _, ok := state.SetKubernetesClusterPendingUninstall("c1", false); !ok {
		t.Fatalf("SetKubernetesClusterPendingUninstall returned false")
	}
	if _, ok := state.SetKubernetesClusterCustomDisplayName("c1", "custom"); !ok {
		t.Fatalf("SetKubernetesClusterCustomDisplayName returned false")
	}

	removed, ok := state.RemoveKubernetesCluster("c1")
	if !ok || removed.ID != "c1" {
		t.Fatalf("RemoveKubernetesCluster = (%v, %v), want c1", removed, ok)
	}
	if _, ok := state.RemoveKubernetesCluster("missing"); ok {
		t.Fatalf("expected RemoveKubernetesCluster to fail for missing")
	}
}

func TestStateRemovedKubernetesClusters(t *testing.T) {
	state := &State{}
	t1 := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)

	state.AddRemovedKubernetesCluster(RemovedKubernetesCluster{ID: "c1", RemovedAt: t1})
	state.AddRemovedKubernetesCluster(RemovedKubernetesCluster{ID: "c2", RemovedAt: t2})
	state.AddRemovedKubernetesCluster(RemovedKubernetesCluster{ID: "c1", DisplayName: "updated", RemovedAt: t1})

	entries := state.GetRemovedKubernetesClusters()
	if len(entries) != 2 {
		t.Fatalf("entries = %#v, want 2", entries)
	}
	if entries[0].ID != "c2" {
		t.Fatalf("entries[0].ID = %q, want c2", entries[0].ID)
	}

	state.RemoveRemovedKubernetesCluster("c1")
	entries = state.GetRemovedKubernetesClusters()
	if len(entries) != 1 || entries[0].ID != "c2" {
		t.Fatalf("entries = %#v, want c2 only", entries)
	}
}

func TestStateClearAllHosts(t *testing.T) {
	state := &State{
		Hosts: []Host{{ID: "h1"}, {ID: "h2"}},
	}

	count := state.ClearAllHosts()
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if len(state.Hosts) != 0 {
		t.Fatalf("Hosts = %#v, want empty", state.Hosts)
	}
}

func TestStateLinkNodeToHostAgent(t *testing.T) {
	state := &State{
		Nodes: []Node{{ID: "n1"}},
	}

	if ok := state.LinkNodeToHostAgent("n1", "h1"); !ok {
		t.Fatalf("LinkNodeToHostAgent returned false")
	}
	if state.Nodes[0].LinkedHostAgentID != "h1" {
		t.Fatalf("LinkedHostAgentID = %q, want h1", state.Nodes[0].LinkedHostAgentID)
	}
	if ok := state.LinkNodeToHostAgent("missing", "h1"); ok {
		t.Fatalf("expected false for missing node")
	}
}

func TestStateUnlinkNodesFromHostAgent(t *testing.T) {
	state := &State{
		Nodes: []Node{
			{ID: "n1", LinkedHostAgentID: "h1"},
			{ID: "n2", LinkedHostAgentID: "h1"},
			{ID: "n3", LinkedHostAgentID: "h2"},
		},
	}

	count := state.UnlinkNodesFromHostAgent("h1")
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	for _, node := range state.Nodes[:2] {
		if node.LinkedHostAgentID != "" {
			t.Fatalf("expected LinkedHostAgentID cleared, got %q", node.LinkedHostAgentID)
		}
	}
}

func TestStateLinkHostAgentToNode(t *testing.T) {
	state := &State{
		Hosts: []Host{
			{ID: "h1", LinkedNodeID: "n1"},
			{ID: "h2", LinkedVMID: "vm1", LinkedContainerID: "ct1"},
		},
		Nodes: []Node{
			{ID: "n1", LinkedHostAgentID: "h1"},
			{ID: "n2"},
		},
	}

	if err := state.LinkHostAgentToNode("h2", "n2"); err != nil {
		t.Fatalf("LinkHostAgentToNode error: %v", err)
	}
	if state.Hosts[1].LinkedNodeID != "n2" {
		t.Fatalf("LinkedNodeID = %q, want n2", state.Hosts[1].LinkedNodeID)
	}
	if state.Nodes[1].LinkedHostAgentID != "h2" {
		t.Fatalf("LinkedHostAgentID = %q, want h2", state.Nodes[1].LinkedHostAgentID)
	}
	if state.Hosts[1].LinkedVMID != "" || state.Hosts[1].LinkedContainerID != "" {
		t.Fatalf("expected VM/container links cleared")
	}

	if err := state.LinkHostAgentToNode("missing", "n2"); err == nil || !strings.Contains(err.Error(), "host agent not found") {
		t.Fatalf("expected host not found error, got %v", err)
	}
	if err := state.LinkHostAgentToNode("h2", "missing"); err == nil || !strings.Contains(err.Error(), "node not found") {
		t.Fatalf("expected node not found error, got %v", err)
	}
}

func TestStateUnlinkHostAgent(t *testing.T) {
	state := &State{
		Hosts: []Host{{ID: "h1", LinkedNodeID: "n1", LinkedVMID: "vm", LinkedContainerID: "ct"}},
		Nodes: []Node{{ID: "n1", LinkedHostAgentID: "h1"}},
	}

	if ok := state.UnlinkHostAgent("h1"); !ok {
		t.Fatalf("UnlinkHostAgent returned false")
	}
	if state.Hosts[0].LinkedNodeID != "" || state.Hosts[0].LinkedVMID != "" || state.Hosts[0].LinkedContainerID != "" {
		t.Fatalf("expected host links cleared")
	}
	if state.Nodes[0].LinkedHostAgentID != "" {
		t.Fatalf("expected node link cleared")
	}
	if ok := state.UnlinkHostAgent("missing"); ok {
		t.Fatalf("expected false for missing host")
	}
}

func TestStateUpsertCephCluster(t *testing.T) {
	state := &State{}
	state.UpsertCephCluster(CephCluster{ID: "c1", Name: "b"})
	state.UpsertCephCluster(CephCluster{ID: "c2", Name: "a"})
	state.UpsertCephCluster(CephCluster{ID: "c1", Name: "c"})

	if len(state.CephClusters) != 2 {
		t.Fatalf("clusters = %#v, want 2", state.CephClusters)
	}
	if state.CephClusters[0].Name != "a" || state.CephClusters[1].Name != "c" {
		t.Fatalf("clusters order = %#v, want a then c", state.CephClusters)
	}
}

func TestStateSetHostCommandsEnabled(t *testing.T) {
	state := &State{
		Hosts: []Host{{ID: "h1", CommandsEnabled: false}},
	}

	if ok := state.SetHostCommandsEnabled("h1", true); !ok {
		t.Fatalf("SetHostCommandsEnabled returned false")
	}
	if !state.Hosts[0].CommandsEnabled {
		t.Fatalf("CommandsEnabled not updated")
	}
	if ok := state.SetHostCommandsEnabled("missing", true); ok {
		t.Fatalf("expected false for missing host")
	}
}

func TestStateContainers(t *testing.T) {
	now := time.Now()
	state := &State{
		Containers: []Container{{ID: "ct1"}},
	}

	containers := state.GetContainers()
	if len(containers) != 1 || containers[0].ID != "ct1" {
		t.Fatalf("containers = %#v, want ct1", containers)
	}
	containers[0].ID = "changed"
	if state.Containers[0].ID != "ct1" {
		t.Fatalf("state containers should not be modified by copy")
	}

	if ok := state.UpdateContainerDockerStatus("ct1", true, now); !ok {
		t.Fatalf("UpdateContainerDockerStatus returned false")
	}
	if !state.Containers[0].HasDocker {
		t.Fatalf("HasDocker not updated")
	}
	if ok := state.UpdateContainerDockerStatus("missing", true, now); ok {
		t.Fatalf("expected false for missing container")
	}
}
