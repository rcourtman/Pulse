package models

import "testing"

func TestStateFrontendStripLegacyArrays(t *testing.T) {
	state := &StateFrontend{
		Nodes:                     []NodeFrontend{{ID: "n1"}},
		VMs:                       []VMFrontend{{ID: "vm1"}},
		Containers:                []ContainerFrontend{{ID: "ct1"}},
		DockerHosts:               []DockerHostFrontend{{ID: "dh1"}},
		RemovedDockerHosts:        []RemovedDockerHostFrontend{{ID: "rdh1"}},
		KubernetesClusters:        []KubernetesClusterFrontend{{ID: "kc1"}},
		RemovedKubernetesClusters: []RemovedKubernetesClusterFrontend{{ID: "rkc1"}},
		Hosts:                     []HostFrontend{{ID: "h1"}},
		Storage:                   []StorageFrontend{{ID: "s1"}},
		PBS:                       []PBSInstance{{Name: "pbs-1"}},
		PMG:                       []PMGInstance{{Name: "pmg-1"}},
		ReplicationJobs:           []ReplicationJobFrontend{{ID: "job-1"}},
	}

	state.StripLegacyArrays()

	if state.Nodes != nil {
		t.Fatalf("Nodes should be nil after strip")
	}
	if state.VMs != nil {
		t.Fatalf("VMs should be nil after strip")
	}
	if state.Containers != nil {
		t.Fatalf("Containers should be nil after strip")
	}
	if state.DockerHosts != nil {
		t.Fatalf("DockerHosts should be nil after strip")
	}
	if len(state.RemovedDockerHosts) != 1 || state.RemovedDockerHosts[0].ID != "rdh1" {
		t.Fatalf("RemovedDockerHosts should be preserved, got %#v", state.RemovedDockerHosts)
	}
	if len(state.KubernetesClusters) != 1 || state.KubernetesClusters[0].ID != "kc1" {
		t.Fatalf("KubernetesClusters should be preserved, got %#v", state.KubernetesClusters)
	}
	if len(state.RemovedKubernetesClusters) != 1 || state.RemovedKubernetesClusters[0].ID != "rkc1" {
		t.Fatalf(
			"RemovedKubernetesClusters should be preserved, got %#v",
			state.RemovedKubernetesClusters,
		)
	}
	if state.Hosts != nil {
		t.Fatalf("Hosts should be nil after strip")
	}
	if state.Storage != nil {
		t.Fatalf("Storage should be nil after strip")
	}
	if len(state.PBS) != 1 || state.PBS[0].Name != "pbs-1" {
		t.Fatalf("PBS should be preserved, got %#v", state.PBS)
	}
	if len(state.PMG) != 1 || state.PMG[0].Name != "pmg-1" {
		t.Fatalf("PMG should be preserved, got %#v", state.PMG)
	}
	if len(state.ReplicationJobs) != 1 || state.ReplicationJobs[0].ID != "job-1" {
		t.Fatalf("ReplicationJobs should be preserved, got %#v", state.ReplicationJobs)
	}
}

func TestStateFrontendStripLegacyArraysNilReceiver(t *testing.T) {
	var state *StateFrontend
	state.StripLegacyArrays()
}
