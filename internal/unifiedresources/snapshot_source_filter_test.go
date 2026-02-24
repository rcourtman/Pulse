package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestSnapshotWithoutSources_ExcludesSourceSlices(t *testing.T) {
	original := models.StateSnapshot{
		Nodes:              []models.Node{{}},
		VMs:                []models.VM{{}},
		Containers:         []models.Container{{}},
		Storage:            []models.Storage{{}},
		CephClusters:       []models.CephCluster{{}},
		PhysicalDisks:      []models.PhysicalDisk{{}},
		Hosts:              []models.Host{{}},
		DockerHosts:        []models.DockerHost{{}},
		KubernetesClusters: []models.KubernetesCluster{{}},
		PBSInstances:       []models.PBSInstance{{}},
		PMGInstances:       []models.PMGInstance{{}},
	}

	filtered := SnapshotWithoutSources(original, []DataSource{SourceProxmox, SourcePBS})

	if len(filtered.Nodes) != 0 || len(filtered.VMs) != 0 || len(filtered.Containers) != 0 {
		t.Fatalf("expected proxmox compute slices to be removed")
	}
	if len(filtered.Storage) != 0 || len(filtered.CephClusters) != 0 || len(filtered.PhysicalDisks) != 0 {
		t.Fatalf("expected proxmox storage slices to be removed")
	}
	if len(filtered.PBSInstances) != 0 {
		t.Fatalf("expected pbs instances to be removed")
	}

	if len(filtered.Hosts) != 1 || len(filtered.DockerHosts) != 1 || len(filtered.KubernetesClusters) != 1 || len(filtered.PMGInstances) != 1 {
		t.Fatalf("expected non-excluded source slices to remain")
	}

	if len(original.Nodes) != 1 || len(original.PBSInstances) != 1 {
		t.Fatalf("expected original snapshot to remain unchanged")
	}
}

func TestSnapshotWithoutSources_NormalizesAliasesAndWhitespace(t *testing.T) {
	original := models.StateSnapshot{
		Nodes:              []models.Node{{}},
		KubernetesClusters: []models.KubernetesCluster{{}},
		Hosts:              []models.Host{{}},
	}

	filtered := SnapshotWithoutSources(original, []DataSource{
		DataSource(" PVE "),
		DataSource("k8s"),
		DataSource(" unknown "),
		"",
	})

	if len(filtered.Nodes) != 0 {
		t.Fatalf("expected PVE alias to remove proxmox nodes")
	}
	if len(filtered.KubernetesClusters) != 0 {
		t.Fatalf("expected k8s alias to remove kubernetes clusters")
	}
	if len(filtered.Hosts) != 1 {
		t.Fatalf("expected unknown source to have no effect on hosts")
	}
}
