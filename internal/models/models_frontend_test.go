package models

import (
	"testing"
)

// --- EmptyStateFrontend ---

func TestEmptyStateFrontend_AllFieldsNonNil(t *testing.T) {
	s := EmptyStateFrontend()

	if s.ActiveAlerts == nil {
		t.Error("ActiveAlerts should be non-nil")
	}
	if s.RecentlyResolved == nil {
		t.Error("RecentlyResolved should be non-nil")
	}
	if s.Metrics == nil {
		t.Error("Metrics should be non-nil")
	}
	if s.Resources == nil {
		t.Error("Resources should be non-nil")
	}
	if s.ConnectedInfrastructure == nil {
		t.Error("ConnectedInfrastructure should be non-nil")
	}
}

// --- NodeFrontend.NormalizeCollections ---

func TestNodeFrontendNormalize_NilLoadAverage(t *testing.T) {
	n := NodeFrontend{ID: "n-1"}
	n = n.NormalizeCollections()
	if n.LoadAverage == nil {
		t.Error("LoadAverage should be non-nil after normalize")
	}
}

func TestNodeFrontendNormalize_PreservesExisting(t *testing.T) {
	n := NodeFrontend{
		ID:          "n-1",
		LoadAverage: []float64{1.0, 2.0, 3.0},
	}
	n = n.NormalizeCollections()
	if len(n.LoadAverage) != 3 {
		t.Error("existing LoadAverage should be preserved")
	}
}

// --- VMFrontend.NormalizeCollections ---

func TestVMFrontendNormalize(t *testing.T) {
	v := VMFrontend{ID: "v-1"}
	v = v.NormalizeCollections()
	if v.Disks == nil {
		t.Error("Disks should be non-nil")
	}
	if v.NetworkInterfaces == nil {
		t.Error("NetworkInterfaces should be non-nil")
	}
	if v.IPAddresses == nil {
		t.Error("IPAddresses should be non-nil")
	}
}

// --- ContainerFrontend.NormalizeCollections ---

func TestContainerFrontendNormalize(t *testing.T) {
	c := ContainerFrontend{ID: "c-1"}
	c = c.NormalizeCollections()
	if c.Disks == nil || c.NetworkInterfaces == nil || c.IPAddresses == nil {
		t.Error("all slice fields should be non-nil after normalize")
	}
}

// --- DockerHostFrontend.NormalizeCollections ---

func TestDockerHostFrontendNormalize(t *testing.T) {
	h := DockerHostFrontend{ID: "dh-1"}
	h = h.NormalizeCollections()
	if h.LoadAverage == nil || h.Disks == nil || h.NetworkInterfaces == nil ||
		h.Containers == nil || h.Services == nil || h.Tasks == nil {
		t.Error("all slice fields should be non-nil after normalize")
	}
}

// --- DockerContainerFrontend.NormalizeCollections ---

func TestDockerContainerFrontendNormalize(t *testing.T) {
	c := DockerContainerFrontend{ID: "dc-1"}
	c = c.NormalizeCollections()
	if c.Ports == nil || c.Labels == nil || c.Networks == nil || c.Mounts == nil {
		t.Error("all slice/map fields should be non-nil after normalize")
	}
}

// --- DockerServiceFrontend.NormalizeCollections ---

func TestDockerServiceFrontendNormalize(t *testing.T) {
	s := DockerServiceFrontend{ID: "ds-1"}
	s = s.NormalizeCollections()
	if s.Labels == nil || s.EndpointPorts == nil {
		t.Error("Labels and EndpointPorts should be non-nil")
	}
}

// --- HostFrontend.NormalizeCollections ---

func TestHostFrontendNormalize(t *testing.T) {
	h := HostFrontend{ID: "h-1"}
	h = h.NormalizeCollections()
	if h.LoadAverage == nil || h.Disks == nil || h.DiskIO == nil ||
		h.NetworkInterfaces == nil || h.Tags == nil {
		t.Error("all slice fields should be non-nil after normalize")
	}
}

func TestHostFrontendNormalize_SensorsNormalized(t *testing.T) {
	h := HostFrontend{
		ID:      "h-1",
		Sensors: &HostSensorSummaryFrontend{},
	}
	h = h.NormalizeCollections()
	if h.Sensors.TemperatureCelsius == nil || h.Sensors.FanRPM == nil ||
		h.Sensors.Additional == nil || h.Sensors.SMART == nil {
		t.Error("nested sensor maps should be normalized")
	}
}

// --- StorageFrontend.NormalizeCollections ---

func TestStorageFrontendNormalize(t *testing.T) {
	s := StorageFrontend{ID: "s-1"}
	s = s.NormalizeCollections()
	if s.Nodes == nil || s.NodeIDs == nil {
		t.Error("Nodes and NodeIDs should be non-nil")
	}
}

// --- KubernetesClusterFrontend.NormalizeCollections ---

func TestKubernetesClusterFrontendNormalize(t *testing.T) {
	c := KubernetesClusterFrontend{ID: "k-1"}
	c = c.NormalizeCollections()
	if c.Nodes == nil || c.Pods == nil || c.Deployments == nil {
		t.Error("all slice fields should be non-nil")
	}
}

// --- KubernetesNodeFrontend.NormalizeCollections ---

func TestKubernetesNodeFrontendNormalize(t *testing.T) {
	n := KubernetesNodeFrontend{}
	n = n.NormalizeCollections()
	if n.Roles == nil {
		t.Error("Roles should be non-nil")
	}
}

// --- KubernetesPodFrontend.NormalizeCollections ---

func TestKubernetesPodFrontendNormalize(t *testing.T) {
	p := KubernetesPodFrontend{}
	p = p.NormalizeCollections()
	if p.Labels == nil || p.Containers == nil {
		t.Error("Labels and Containers should be non-nil")
	}
}

// --- KubernetesDeploymentFrontend.NormalizeCollections ---

func TestKubernetesDeploymentFrontendNormalize(t *testing.T) {
	d := KubernetesDeploymentFrontend{}
	d = d.NormalizeCollections()
	if d.Labels == nil {
		t.Error("Labels should be non-nil")
	}
}

// --- CephClusterFrontend.NormalizeCollections ---

func TestCephClusterFrontendNormalize(t *testing.T) {
	c := CephClusterFrontend{ID: "ceph-1"}
	c = c.NormalizeCollections()
	if c.Pools == nil || c.Services == nil {
		t.Error("Pools and Services should be non-nil")
	}
}

// --- ConnectedInfrastructureItemFrontend.NormalizeCollections ---

func TestConnectedInfrastructureItemFrontendNormalize(t *testing.T) {
	i := ConnectedInfrastructureItemFrontend{ID: "ci-1"}
	i = i.NormalizeCollections()
	if i.Surfaces == nil {
		t.Error("Surfaces should be non-nil")
	}
}
