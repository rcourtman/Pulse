package models

import "testing"

func TestStateFrontendStripLegacyArrays(t *testing.T) {
	state := &StateFrontend{
		Nodes:              []NodeFrontend{{ID: "n1"}},
		VMs:                []VMFrontend{{ID: "vm1"}},
		Containers:         []ContainerFrontend{{ID: "ct1"}},
		DockerHosts:        []DockerHostFrontend{{ID: "dh1"}},
		RemovedDockerHosts: []RemovedDockerHostFrontend{{ID: "rdh1"}},
		Hosts:              []HostFrontend{{ID: "h1"}},
		Storage:            []StorageFrontend{{ID: "s1"}},
		PBS:                []PBSInstance{{Name: "pbs-1"}},
		PMG:                []PMGInstance{{Name: "pmg-1"}},
		Backups:            Backups{PVE: PVEBackups{}},
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
	if state.RemovedDockerHosts != nil {
		t.Fatalf("RemovedDockerHosts should be nil after strip")
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
}

func TestStateFrontendStripLegacyArraysNilReceiver(t *testing.T) {
	var state *StateFrontend
	state.StripLegacyArrays()
}
