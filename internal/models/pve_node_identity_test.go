package models

import "testing"

func TestUpdateNodesForInstanceKeepsAgentLinkAcrossNativeRename(t *testing.T) {
	state := NewState()
	state.Nodes = []Node{{
		ID: "production-pve-old", NodeIdentity: "production-pve-old",
		Name: "pve-old", Instance: "cluster-api", ClusterName: "production",
		IsClusterMember: true, LinkedAgentID: "agent-1", Status: "online",
	}}
	state.Hosts = []Host{{ID: "agent-1", Hostname: "pve-new", Status: "online"}}

	state.UpdateNodesForInstance("cluster-api", []Node{{
		ID: "production-pve-old", NodeIdentity: "production-pve-old",
		Name: "pve-new", Instance: "cluster-api", ClusterName: "production",
		IsClusterMember: true, Status: "online",
	}})

	if len(state.Nodes) != 1 {
		t.Fatalf("native rename duplicated node state: %+v", state.Nodes)
	}
	if state.Nodes[0].Name != "pve-new" || state.Nodes[0].LinkedAgentID != "agent-1" {
		t.Fatalf("native rename lost immutable identity/link: %+v", state.Nodes[0])
	}
}

func TestCloneNodeIsolatesNativeNameAliases(t *testing.T) {
	original := Node{
		ID:                "production-pve1",
		NodeIdentity:      "production-pve1",
		Name:              "pve1",
		NativeNameAliases: []string{"pve-old"},
		DisplayName:       "Render East",
	}

	cloned := cloneNode(original)
	cloned.NativeNameAliases[0] = "changed"

	if original.NativeNameAliases[0] != "pve-old" {
		t.Fatalf("node snapshot clone shared native alias storage: %+v", original)
	}
}
