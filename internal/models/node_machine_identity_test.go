package models

import "testing"

// NodeObservationsSameMachine must agree with the fold UpdateNodesForInstance
// applies to two views of a node from different connections, so callers that
// see a polled node before it is merged treat the same views as one node.
func TestNodeObservationsSameMachineMatchesStateFold(t *testing.T) {
	clusterNode := func(instance, cluster, host, fingerprint string) Node {
		return Node{
			ID: instance + "-pve01", NodeIdentity: instance + "-pve01", Name: "pve01", Instance: instance,
			ClusterName: cluster, IsClusterMember: cluster != "", Host: host, TLSFingerprint: fingerprint, Status: "online",
		}
	}
	for _, tc := range []struct {
		name                 string
		existing, candidate  Node
		wantSameMachineFolds bool
	}{
		{
			name:                 "same cluster added twice with matching fingerprints",
			existing:             clusterNode("enacon-a", "enacon", "https://192.168.1.11:8006", "AA:AA:AA:AA"),
			candidate:            clusterNode("enacon-b", "enacon", "https://192.168.1.11:8006", "aaaaaaaa"),
			wantSameMachineFolds: true,
		},
		{
			name:      "same-named clusters without fingerprints",
			existing:  clusterNode("site-a", "cluster", "https://192.168.1.11:8006", ""),
			candidate: clusterNode("site-b", "cluster", "https://192.168.1.11:8006", ""),
		},
		{
			name:      "same-named clusters with different fingerprints",
			existing:  clusterNode("site-a", "cluster", "https://192.168.1.11:8006", "AA:AA"),
			candidate: clusterNode("site-b", "cluster", "https://192.168.1.11:8006", "BB:BB"),
		},
		{
			name:      "different clusters reusing a node name and address",
			existing:  clusterNode("site-a", "a", "https://192.168.1.11:8006", "AA:AA"),
			candidate: clusterNode("site-b", "b", "https://192.168.1.11:8006", "AA:AA"),
		},
		{
			name:      "standalone nodes of one name at different addresses",
			existing:  clusterNode("site-a", "", "https://10.0.0.5:8006", ""),
			candidate: clusterNode("site-b", "", "https://10.0.1.5:8006", ""),
		},
		{
			name:                 "standalone connections to one address",
			existing:             clusterNode("by-ip", "", "https://10.0.0.5:8006", ""),
			candidate:            clusterNode("by-ip-again", "", "https://10.0.0.5:8006", ""),
			wantSameMachineFolds: true,
		},
		{
			name:                 "standalone view of a cluster member's address",
			existing:             clusterNode("cluster-view", "enacon", "https://192.168.1.11:8006", ""),
			candidate:            clusterNode("standalone-view", "", "https://192.168.1.11:8006", ""),
			wantSameMachineFolds: true,
		},
		{
			name:                 "same cluster by hostname with matching fingerprints",
			existing:             clusterNode("enacon-a", "enacon", "https://pve01.lan:8006", "AA:AA"),
			candidate:            clusterNode("enacon-b", "enacon", "https://pve01.lan:8006", "AA:AA"),
			wantSameMachineFolds: true,
		},
		{
			name:      "different clusters by hostname",
			existing:  clusterNode("site-a", "a", "https://pve01.lan:8006", ""),
			candidate: clusterNode("site-b", "b", "https://pve01.lan:8006", ""),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &State{Nodes: []Node{tc.existing}}
			state.UpdateNodesForInstance(tc.candidate.Instance, []Node{tc.candidate})
			if folded := len(state.Nodes) == 1; folded != tc.wantSameMachineFolds {
				t.Fatalf("state fold = %v (nodes %#v), want %v", folded, state.Nodes, tc.wantSameMachineFolds)
			}
			if got := NodeObservationsSameMachine(tc.existing, tc.candidate); got != tc.wantSameMachineFolds {
				t.Fatalf("NodeObservationsSameMachine = %v, want %v (the state's fold)", got, tc.wantSameMachineFolds)
			}
			if got := NodeObservationsSameMachine(tc.candidate, tc.existing); got != tc.wantSameMachineFolds {
				t.Fatalf("NodeObservationsSameMachine reversed = %v, want %v", got, tc.wantSameMachineFolds)
			}
		})
	}
}

// HostAgentBridgesNodeViews must agree with the fold UpdateNodesForInstance lets
// a linked agent make between two standalone views of one machine.
func TestHostAgentBridgesNodeViewsMatchesStateFold(t *testing.T) {
	standalone := func(instance, host string) Node {
		return Node{ID: instance + "-minipc", NodeIdentity: instance + "-minipc", Name: "minipc", Instance: instance, Host: host, Status: "online"}
	}
	for _, tc := range []struct {
		name                    string
		addresses               []string
		existingFP, candidateFP string
		wantBridge              bool
	}{
		{name: "agent reports both connection addresses", addresses: []string{"10.0.0.5/24", "10.0.1.5/24"}, wantBridge: true},
		{name: "agent reports only the linked view's address", addresses: []string{"10.0.0.5/24"}},
		{name: "matching fingerprints", addresses: []string{"10.0.0.5/24", "10.0.1.5/24"}, existingFP: "AA:AA", candidateFP: "aaaa", wantBridge: true},
		{name: "contradicting fingerprints", addresses: []string{"10.0.0.5/24", "10.0.1.5/24"}, existingFP: "AA:AA", candidateFP: "BB:BB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := standalone("lan-a", "https://10.0.0.5:8006")
			existing.LinkedAgentID = "agent-minipc"
			existing.TLSFingerprint = tc.existingFP
			host := Host{
				ID: "agent-minipc", Hostname: "minipc", LinkedNodeID: existing.ID, ReportIP: "10.0.0.5",
				NetworkInterfaces: []HostNetworkInterface{{Name: "eth0", Addresses: tc.addresses}},
			}
			candidate := standalone("lan-b", "https://10.0.1.5:8006")
			candidate.TLSFingerprint = tc.candidateFP

			state := &State{Nodes: []Node{existing}, Hosts: []Host{host}}
			state.UpdateNodesForInstance(candidate.Instance, []Node{candidate})
			if folded := len(state.Nodes) == 1; folded != tc.wantBridge {
				t.Fatalf("state fold = %v (nodes %#v), want %v", folded, state.Nodes, tc.wantBridge)
			}
			if got := HostAgentBridgesNodeViews(host, existing, candidate); got != tc.wantBridge {
				t.Fatalf("HostAgentBridgesNodeViews = %v, want %v (the state's fold)", got, tc.wantBridge)
			}
		})
	}
}
