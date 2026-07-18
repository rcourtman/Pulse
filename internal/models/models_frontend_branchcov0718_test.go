package models

import (
	"testing"
)

// This file raises BRANCH coverage for the NormalizeCollections methods on
// the Frontend types defined in models_frontend.go.
//
// For every target type we exercise BOTH arms of each nil-collection
// conditional:
//
//   - nil arm:    the collection field is left nil; after NormalizeCollections
//                 it MUST be a non-nil empty slice/map of the right type.
//   - populated:  the collection field is pre-populated; NormalizeCollections
//                 MUST preserve it (these methods do NOT sort or deduplicate).
//                 Where the method recurses into nested elements (DockerHost,
//                 KubernetesCluster, State, Storage.ZFSPool, Resource.Identity,
//                 Host.Sensors), the populated arm supplies a nested element
//                 with a nil sub-collection so the observable side-effect of
//                 the recursion (the sub-collection becoming non-nil) can be
//                 asserted, exercising the for-loop bodies and the `!= nil`
//                 guard branches.
//
// No source file or sibling test was modified.

func TestFrontendNormalizeCollections_BranchCov0718(t *testing.T) {
	// ---------------- NodeFrontend ----------------
	t.Run("NodeFrontend_nil_becomes_empty", func(t *testing.T) {
		n := NodeFrontend{ID: "n-1"} // LoadAverage is nil
		out := n.NormalizeCollections()
		if out.LoadAverage == nil {
			t.Fatalf("LoadAverage should be non-nil after normalize, got nil")
		}
		if len(out.LoadAverage) != 0 {
			t.Fatalf("LoadAverage should be empty, got len=%d", len(out.LoadAverage))
		}
	})
	t.Run("NodeFrontend_populated_preserved_no_dedup", func(t *testing.T) {
		// Duplicates included on purpose: the method does NOT dedup/sort.
		orig := []float64{3.0, 1.0, 3.0, 2.0}
		n := NodeFrontend{ID: "n-1", LoadAverage: orig}
		out := n.NormalizeCollections()
		if len(out.LoadAverage) != len(orig) {
			t.Fatalf("LoadAverage length changed: got %d want %d", len(out.LoadAverage), len(orig))
		}
		for i := range orig {
			if out.LoadAverage[i] != orig[i] {
				t.Fatalf("LoadAverage[%d] = %v, want %v (no sort/dedup expected)", i, out.LoadAverage[i], orig[i])
			}
		}
	})

	// ---------------- VMFrontend ----------------
	t.Run("VMFrontend_nil_becomes_empty", func(t *testing.T) {
		v := VMFrontend{ID: "v-1"}
		out := v.NormalizeCollections()
		if v.Disks != nil || v.NetworkInterfaces != nil || v.IPAddresses != nil {
			t.Fatalf("precondition: input collections must be nil")
		}
		if out.Disks == nil || out.NetworkInterfaces == nil || out.IPAddresses == nil {
			t.Fatalf("Disks/NetworkInterfaces/IPAddresses must all be non-nil after normalize")
		}
		if len(out.Disks) != 0 || len(out.NetworkInterfaces) != 0 || len(out.IPAddresses) != 0 {
			t.Fatalf("normalized empty collections must have len 0")
		}
	})
	t.Run("VMFrontend_populated_preserved", func(t *testing.T) {
		v := VMFrontend{
			ID:                "v-1",
			Disks:             []Disk{{Device: "/sda"}},
			NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}},
			IPAddresses:       []string{"10.0.0.1", "10.0.0.1"}, // duplicate on purpose
		}
		out := v.NormalizeCollections()
		if len(out.Disks) != 1 || out.Disks[0].Device != "/sda" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eth0" {
			t.Fatalf("NetworkInterfaces not preserved: %+v", out.NetworkInterfaces)
		}
		if len(out.IPAddresses) != 2 || out.IPAddresses[0] != "10.0.0.1" || out.IPAddresses[1] != "10.0.0.1" {
			t.Fatalf("IPAddresses not preserved (no dedup expected): %+v", out.IPAddresses)
		}
	})

	// ---------------- ContainerFrontend ----------------
	t.Run("ContainerFrontend_nil_becomes_empty", func(t *testing.T) {
		c := ContainerFrontend{ID: "c-1"}
		out := c.NormalizeCollections()
		if out.Disks == nil || out.NetworkInterfaces == nil || out.IPAddresses == nil {
			t.Fatalf("Disks/NetworkInterfaces/IPAddresses must all be non-nil after normalize")
		}
	})
	t.Run("ContainerFrontend_populated_preserved", func(t *testing.T) {
		c := ContainerFrontend{
			ID:                "c-1",
			Disks:             []Disk{{Device: "/data"}},
			NetworkInterfaces: []GuestNetworkInterface{{Name: "eth0"}},
			IPAddresses:       []string{"10.0.0.2"},
		}
		out := c.NormalizeCollections()
		if len(out.Disks) != 1 || out.Disks[0].Device != "/data" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
		if len(out.NetworkInterfaces) != 1 {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if len(out.IPAddresses) != 1 || out.IPAddresses[0] != "10.0.0.2" {
			t.Fatalf("IPAddresses not preserved: %+v", out.IPAddresses)
		}
	})

	// ---------------- DockerHostFrontend ----------------
	t.Run("DockerHostFrontend_nil_all_collections", func(t *testing.T) {
		h := DockerHostFrontend{ID: "dh-1"}
		out := h.NormalizeCollections()
		if out.LoadAverage == nil || out.Disks == nil || out.NetworkInterfaces == nil ||
			out.Containers == nil || out.Services == nil || out.Tasks == nil ||
			out.Nodes == nil || out.Secrets == nil || out.Configs == nil {
			t.Fatalf("all nine collection fields must be non-nil after normalize")
		}
		// Security is nil on input → must remain nil (no fabrication).
		if out.Security != nil {
			t.Fatalf("Security must remain nil when nil on input")
		}
	})
	t.Run("DockerHostFrontend_populated_recurses_into_nested", func(t *testing.T) {
		// Every nested element starts with nil sub-collections so we can
		// observe the parent's for-loop recursion turning them non-nil.
		h := DockerHostFrontend{
			ID:                "dh-1",
			LoadAverage:       []float64{1.5},
			Disks:             []Disk{{Device: "/dev/sda"}},
			NetworkInterfaces: []HostNetworkInterface{{Name: "eno1"}},
			Containers:        []DockerContainerFrontend{{ID: "ctr-1"}}, // Ports/Labels/Networks/Mounts nil
			Services:          []DockerServiceFrontend{{ID: "svc-1"}},   // Labels/EndpointPorts nil
			Tasks:             []DockerTaskFrontend{{ID: "task-1"}},
			Nodes:             []DockerNodeFrontend{{ID: "node-1"}},
			Secrets:           []DockerSecretFrontend{{ID: "sec-1"}}, // Labels nil
			Configs:           []DockerConfigFrontend{{ID: "cfg-1"}}, // Labels nil
			Security:          &DockerHostSecurityFrontend{},         // AuthorizationPlugins nil
		}
		out := h.NormalizeCollections()

		// Top-level populated collections preserved.
		if len(out.LoadAverage) != 1 || out.LoadAverage[0] != 1.5 {
			t.Fatalf("LoadAverage not preserved: %+v", out.LoadAverage)
		}
		if len(out.Disks) != 1 || out.Disks[0].Device != "/dev/sda" {
			t.Fatalf("Disks not preserved: %+v", out.Disks)
		}
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eno1" {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if len(out.Tasks) != 1 || out.Tasks[0].ID != "task-1" {
			t.Fatalf("Tasks not preserved")
		}
		if len(out.Nodes) != 1 || out.Nodes[0].ID != "node-1" {
			t.Fatalf("Nodes not preserved")
		}

		// Recursion into containers/services/secrets/configs observable.
		if len(out.Containers) != 1 {
			t.Fatalf("Containers length changed")
		}
		c := out.Containers[0]
		if c.Ports == nil || c.Labels == nil || c.Networks == nil || c.Mounts == nil {
			t.Fatalf("nested DockerContainerFrontend collections not normalized: %+v", c)
		}
		if len(out.Services) != 1 || out.Services[0].Labels == nil || out.Services[0].EndpointPorts == nil {
			t.Fatalf("nested DockerServiceFrontend collections not normalized")
		}
		if len(out.Secrets) != 1 || out.Secrets[0].Labels == nil {
			t.Fatalf("nested DockerSecretFrontend.Labels not normalized")
		}
		if len(out.Configs) != 1 || out.Configs[0].Labels == nil {
			t.Fatalf("nested DockerConfigFrontend.Labels not normalized")
		}

		// The `if h.Security != nil` branch must have run and recursed.
		if out.Security == nil {
			t.Fatalf("Security must be preserved when non-nil on input")
		}
		if out.Security.AuthorizationPlugins == nil {
			t.Fatalf("Security.AuthorizationPlugins should be normalized to non-nil")
		}
	})

	// ---------------- DockerHostSecurityFrontend (was 0%) ----------------
	t.Run("DockerHostSecurityFrontend_nil", func(t *testing.T) {
		s := DockerHostSecurityFrontend{MutatingCommandsBlocked: true}
		out := s.NormalizeCollections()
		if out.AuthorizationPlugins == nil {
			t.Fatalf("AuthorizationPlugins should be non-nil after normalize")
		}
		if len(out.AuthorizationPlugins) != 0 {
			t.Fatalf("AuthorizationPlugins should be empty, got len=%d", len(out.AuthorizationPlugins))
		}
		if !out.MutatingCommandsBlocked {
			t.Fatalf("scalar MutatingCommandsBlocked must be preserved")
		}
	})
	t.Run("DockerHostSecurityFrontend_populated_preserved", func(t *testing.T) {
		s := DockerHostSecurityFrontend{
			AuthorizationPlugins: []string{"opa", "opa"}, // duplicate on purpose
		}
		out := s.NormalizeCollections()
		if len(out.AuthorizationPlugins) != 2 || out.AuthorizationPlugins[0] != "opa" || out.AuthorizationPlugins[1] != "opa" {
			t.Fatalf("AuthorizationPlugins not preserved (no dedup expected): %+v", out.AuthorizationPlugins)
		}
	})

	// ---------------- ConnectedInfrastructureItemFrontend ----------------
	t.Run("ConnectedInfrastructureItemFrontend_nil", func(t *testing.T) {
		i := ConnectedInfrastructureItemFrontend{ID: "ci-1"}
		out := i.NormalizeCollections()
		if out.Surfaces == nil {
			t.Fatalf("Surfaces should be non-nil after normalize")
		}
	})
	t.Run("ConnectedInfrastructureItemFrontend_populated_preserved", func(t *testing.T) {
		i := ConnectedInfrastructureItemFrontend{
			ID:       "ci-1",
			Surfaces: []ConnectedInfrastructureSurfaceFrontend{{ID: "sf-1", Kind: "agent"}},
		}
		out := i.NormalizeCollections()
		if len(out.Surfaces) != 1 || out.Surfaces[0].ID != "sf-1" || out.Surfaces[0].Kind != "agent" {
			t.Fatalf("Surfaces not preserved: %+v", out.Surfaces)
		}
	})

	// ---------------- KubernetesClusterFrontend ----------------
	t.Run("KubernetesClusterFrontend_nil", func(t *testing.T) {
		c := KubernetesClusterFrontend{ID: "k-1"}
		out := c.NormalizeCollections()
		if out.Nodes == nil || out.Pods == nil || out.Deployments == nil {
			t.Fatalf("Nodes/Pods/Deployments must all be non-nil after normalize")
		}
	})
	t.Run("KubernetesClusterFrontend_populated_recurses_into_nested", func(t *testing.T) {
		c := KubernetesClusterFrontend{
			ID:          "k-1",
			Nodes:       []KubernetesNodeFrontend{{UID: "n-1"}},       // Roles nil
			Pods:        []KubernetesPodFrontend{{UID: "p-1"}},        // Labels/Containers nil
			Deployments: []KubernetesDeploymentFrontend{{UID: "d-1"}}, // Labels nil
		}
		out := c.NormalizeCollections()

		if len(out.Nodes) != 1 || out.Nodes[0].UID != "n-1" {
			t.Fatalf("Nodes not preserved")
		}
		if out.Nodes[0].Roles == nil {
			t.Fatalf("nested KubernetesNodeFrontend.Roles not normalized")
		}
		if len(out.Pods) != 1 || out.Pods[0].UID != "p-1" {
			t.Fatalf("Pods not preserved")
		}
		if out.Pods[0].Labels == nil || out.Pods[0].Containers == nil {
			t.Fatalf("nested KubernetesPodFrontend collections not normalized")
		}
		if len(out.Deployments) != 1 || out.Deployments[0].UID != "d-1" {
			t.Fatalf("Deployments not preserved")
		}
		if out.Deployments[0].Labels == nil {
			t.Fatalf("nested KubernetesDeploymentFrontend.Labels not normalized")
		}
	})

	// ---------------- KubernetesNodeFrontend ----------------
	t.Run("KubernetesNodeFrontend_nil", func(t *testing.T) {
		n := KubernetesNodeFrontend{UID: "n-1"}
		out := n.NormalizeCollections()
		if out.Roles == nil {
			t.Fatalf("Roles should be non-nil after normalize")
		}
	})
	t.Run("KubernetesNodeFrontend_populated_preserved", func(t *testing.T) {
		n := KubernetesNodeFrontend{UID: "n-1", Roles: []string{"control-plane", "worker", "control-plane"}}
		out := n.NormalizeCollections()
		if len(out.Roles) != 3 || out.Roles[0] != "control-plane" || out.Roles[2] != "control-plane" {
			t.Fatalf("Roles not preserved (no dedup expected): %+v", out.Roles)
		}
	})

	// ---------------- KubernetesPodFrontend ----------------
	t.Run("KubernetesPodFrontend_nil", func(t *testing.T) {
		p := KubernetesPodFrontend{UID: "p-1"}
		out := p.NormalizeCollections()
		if out.Labels == nil || out.Containers == nil {
			t.Fatalf("Labels/Containers should be non-nil after normalize")
		}
	})
	t.Run("KubernetesPodFrontend_populated_preserved", func(t *testing.T) {
		p := KubernetesPodFrontend{
			UID:        "p-1",
			Labels:     map[string]string{"app": "web"},
			Containers: []KubernetesPodContainerFrontend{{Name: "c-1"}},
		}
		out := p.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Containers) != 1 || out.Containers[0].Name != "c-1" {
			t.Fatalf("Containers not preserved: %+v", out.Containers)
		}
	})

	// ---------------- KubernetesDeploymentFrontend ----------------
	t.Run("KubernetesDeploymentFrontend_nil", func(t *testing.T) {
		d := KubernetesDeploymentFrontend{UID: "d-1"}
		out := d.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
	})
	t.Run("KubernetesDeploymentFrontend_populated_preserved", func(t *testing.T) {
		d := KubernetesDeploymentFrontend{UID: "d-1", Labels: map[string]string{"app": "api"}}
		out := d.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["app"] != "api" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- DockerContainerFrontend ----------------
	t.Run("DockerContainerFrontend_nil", func(t *testing.T) {
		c := DockerContainerFrontend{ID: "ctr-1"}
		out := c.NormalizeCollections()
		if out.Ports == nil || out.Labels == nil || out.Networks == nil || out.Mounts == nil {
			t.Fatalf("Ports/Labels/Networks/Mounts must all be non-nil after normalize")
		}
	})
	t.Run("DockerContainerFrontend_populated_preserved", func(t *testing.T) {
		c := DockerContainerFrontend{
			ID:       "ctr-1",
			Ports:    []DockerContainerPortFrontend{{PrivatePort: 80, Protocol: "tcp"}},
			Labels:   map[string]string{"io.docker.compose.service": "web"},
			Networks: []DockerContainerNetworkFrontend{{Name: "bridge", IPv4: "172.17.0.2"}},
			Mounts:   []DockerContainerMountFrontend{{Type: "bind", Source: "/host"}},
		}
		out := c.NormalizeCollections()
		if len(out.Ports) != 1 || out.Ports[0].PrivatePort != 80 || out.Ports[0].Protocol != "tcp" {
			t.Fatalf("Ports not preserved: %+v", out.Ports)
		}
		if len(out.Labels) != 1 || out.Labels["io.docker.compose.service"] != "web" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Networks) != 1 || out.Networks[0].IPv4 != "172.17.0.2" {
			t.Fatalf("Networks not preserved: %+v", out.Networks)
		}
		if len(out.Mounts) != 1 || out.Mounts[0].Source != "/host" {
			t.Fatalf("Mounts not preserved: %+v", out.Mounts)
		}
	})

	// ---------------- DockerServiceFrontend ----------------
	t.Run("DockerServiceFrontend_nil", func(t *testing.T) {
		s := DockerServiceFrontend{ID: "svc-1"}
		out := s.NormalizeCollections()
		if out.Labels == nil || out.EndpointPorts == nil {
			t.Fatalf("Labels/EndpointPorts should be non-nil after normalize")
		}
	})
	t.Run("DockerServiceFrontend_populated_preserved", func(t *testing.T) {
		s := DockerServiceFrontend{
			ID:            "svc-1",
			Labels:        map[string]string{"swarm": "true"},
			EndpointPorts: []DockerServicePortFrontend{{PublishedPort: 8080, TargetPort: 80}},
		}
		out := s.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["swarm"] != "true" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.EndpointPorts) != 1 || out.EndpointPorts[0].TargetPort != 80 || out.EndpointPorts[0].PublishedPort != 8080 {
			t.Fatalf("EndpointPorts not preserved: %+v", out.EndpointPorts)
		}
	})

	// ---------------- DockerSecretFrontend (was 0%) ----------------
	t.Run("DockerSecretFrontend_nil", func(t *testing.T) {
		s := DockerSecretFrontend{ID: "sec-1", Name: "tls"}
		out := s.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
		if len(out.Labels) != 0 {
			t.Fatalf("Labels should be empty, got len=%d", len(out.Labels))
		}
		if out.Name != "tls" {
			t.Fatalf("Name scalar must be preserved")
		}
	})
	t.Run("DockerSecretFrontend_populated_preserved", func(t *testing.T) {
		s := DockerSecretFrontend{
			ID:     "sec-1",
			Name:   "tls",
			Labels: map[string]string{"managed": "true"},
		}
		out := s.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["managed"] != "true" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- DockerConfigFrontend (was 0%) ----------------
	t.Run("DockerConfigFrontend_nil", func(t *testing.T) {
		c := DockerConfigFrontend{ID: "cfg-1", Name: "conf"}
		out := c.NormalizeCollections()
		if out.Labels == nil {
			t.Fatalf("Labels should be non-nil after normalize")
		}
		if len(out.Labels) != 0 {
			t.Fatalf("Labels should be empty, got len=%d", len(out.Labels))
		}
		if out.Name != "conf" {
			t.Fatalf("Name scalar must be preserved")
		}
	})
	t.Run("DockerConfigFrontend_populated_preserved", func(t *testing.T) {
		c := DockerConfigFrontend{
			ID:     "cfg-1",
			Name:   "conf",
			Labels: map[string]string{"env": "prod"},
		}
		out := c.NormalizeCollections()
		if len(out.Labels) != 1 || out.Labels["env"] != "prod" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
	})

	// ---------------- HostFrontend ----------------
	t.Run("HostFrontend_nil_all_collections_and_nil_sensors", func(t *testing.T) {
		h := HostFrontend{ID: "h-1"}
		out := h.NormalizeCollections()
		if out.LoadAverage == nil || out.Disks == nil || out.DiskIO == nil ||
			out.NetworkInterfaces == nil || out.Tags == nil {
			t.Fatalf("LoadAverage/Disks/DiskIO/NetworkInterfaces/Tags must all be non-nil after normalize")
		}
		if out.Sensors != nil {
			t.Fatalf("Sensors must remain nil when nil on input")
		}
	})
	t.Run("HostFrontend_populated_with_sensors_recurses", func(t *testing.T) {
		h := HostFrontend{
			ID:                "h-1",
			LoadAverage:       []float64{0.5, 0.4, 0.3},
			Disks:             []Disk{{Device: "/"}},
			DiskIO:            []DiskIO{{Device: "sda"}},
			NetworkInterfaces: []HostNetworkInterface{{Name: "eth0"}},
			Tags:              []string{"prod", "prod"},     // duplicate on purpose
			Sensors:           &HostSensorSummaryFrontend{}, // all sub-collections nil
		}
		out := h.NormalizeCollections()

		if len(out.LoadAverage) != 3 || out.LoadAverage[2] != 0.3 {
			t.Fatalf("LoadAverage not preserved: %+v", out.LoadAverage)
		}
		if len(out.Disks) != 1 || out.Disks[0].Device != "/" {
			t.Fatalf("Disks not preserved")
		}
		if len(out.DiskIO) != 1 || out.DiskIO[0].Device != "sda" {
			t.Fatalf("DiskIO not preserved")
		}
		if len(out.NetworkInterfaces) != 1 || out.NetworkInterfaces[0].Name != "eth0" {
			t.Fatalf("NetworkInterfaces not preserved")
		}
		if len(out.Tags) != 2 || out.Tags[0] != "prod" || out.Tags[1] != "prod" {
			t.Fatalf("Tags not preserved (no dedup expected): %+v", out.Tags)
		}

		// The `if h.Sensors != nil` branch must have run and recursed.
		if out.Sensors == nil {
			t.Fatalf("Sensors must be preserved when non-nil on input")
		}
		s := out.Sensors
		if s.TemperatureCelsius == nil || s.FanRPM == nil || s.PowerWatts == nil ||
			s.Additional == nil || s.GPU == nil || s.SMART == nil {
			t.Fatalf("nested HostSensorSummaryFrontend collections not normalized: %+v", s)
		}
	})

	// ---------------- HostSensorSummaryFrontend ----------------
	t.Run("HostSensorSummaryFrontend_nil_all_six", func(t *testing.T) {
		s := HostSensorSummaryFrontend{}
		out := s.NormalizeCollections()
		if out.TemperatureCelsius == nil || out.FanRPM == nil || out.PowerWatts == nil ||
			out.Additional == nil || out.GPU == nil || out.SMART == nil {
			t.Fatalf("all six collection fields must be non-nil after normalize")
		}
	})
	t.Run("HostSensorSummaryFrontend_populated_preserved", func(t *testing.T) {
		temp := 65.0
		s := HostSensorSummaryFrontend{
			TemperatureCelsius: map[string]float64{"cpu": temp},
			FanRPM:             map[string]float64{"fan1": 1500},
			PowerWatts:         map[string]float64{"psu1": 220.5},
			Additional:         map[string]float64{"voltage": 12.0},
			GPU:                []HostGPUSensorFrontend{{ID: "gpu0", Name: "nvidia"}},
			SMART:              []HostDiskSMARTFrontend{{Device: "sda", Temperature: 40}},
		}
		out := s.NormalizeCollections()
		if len(out.TemperatureCelsius) != 1 || out.TemperatureCelsius["cpu"] != temp {
			t.Fatalf("TemperatureCelsius not preserved: %+v", out.TemperatureCelsius)
		}
		if len(out.FanRPM) != 1 || out.FanRPM["fan1"] != 1500 {
			t.Fatalf("FanRPM not preserved: %+v", out.FanRPM)
		}
		if len(out.PowerWatts) != 1 || out.PowerWatts["psu1"] != 220.5 {
			t.Fatalf("PowerWatts not preserved: %+v", out.PowerWatts)
		}
		if len(out.Additional) != 1 || out.Additional["voltage"] != 12.0 {
			t.Fatalf("Additional not preserved: %+v", out.Additional)
		}
		if len(out.GPU) != 1 || out.GPU[0].Name != "nvidia" {
			t.Fatalf("GPU not preserved: %+v", out.GPU)
		}
		if len(out.SMART) != 1 || out.SMART[0].Device != "sda" || out.SMART[0].Temperature != 40 {
			t.Fatalf("SMART not preserved: %+v", out.SMART)
		}
	})

	// ---------------- StorageFrontend ----------------
	t.Run("StorageFrontend_nil_and_nil_zfs", func(t *testing.T) {
		s := StorageFrontend{ID: "s-1"}
		out := s.NormalizeCollections()
		if out.Nodes == nil || out.NodeIDs == nil {
			t.Fatalf("Nodes/NodeIDs should be non-nil after normalize")
		}
		if out.ZFSPool != nil {
			t.Fatalf("ZFSPool must remain nil when nil on input")
		}
	})
	t.Run("StorageFrontend_populated_with_zfs_recurses", func(t *testing.T) {
		s := StorageFrontend{
			ID:      "s-1",
			Nodes:   []string{"node-a", "node-a"}, // duplicate on purpose
			NodeIDs: []string{"nid-1"},
			ZFSPool: &ZFSPool{Name: "tank"}, // Devices nil → recursion observable
		}
		out := s.NormalizeCollections()
		if len(out.Nodes) != 2 || out.Nodes[0] != "node-a" || out.Nodes[1] != "node-a" {
			t.Fatalf("Nodes not preserved (no dedup expected): %+v", out.Nodes)
		}
		if len(out.NodeIDs) != 1 || out.NodeIDs[0] != "nid-1" {
			t.Fatalf("NodeIDs not preserved: %+v", out.NodeIDs)
		}
		// The `if s.ZFSPool != nil` branch must have run and recursed.
		if out.ZFSPool == nil {
			t.Fatalf("ZFSPool must be preserved when non-nil on input")
		}
		if out.ZFSPool.Name != "tank" {
			t.Fatalf("ZFSPool.Name not preserved: %q", out.ZFSPool.Name)
		}
		if out.ZFSPool.Devices == nil {
			t.Fatalf("nested ZFSPool.Devices not normalized (recursion did not run)")
		}
	})

	// ---------------- CephClusterFrontend ----------------
	t.Run("CephClusterFrontend_nil", func(t *testing.T) {
		c := CephClusterFrontend{ID: "ceph-1"}
		out := c.NormalizeCollections()
		if out.Pools == nil || out.Services == nil {
			t.Fatalf("Pools/Services should be non-nil after normalize")
		}
	})
	t.Run("CephClusterFrontend_populated_preserved", func(t *testing.T) {
		c := CephClusterFrontend{
			ID:       "ceph-1",
			Pools:    []CephPool{{Name: "replicapool"}},
			Services: []CephServiceStatus{{Type: "mon", Running: 1}},
		}
		out := c.NormalizeCollections()
		if len(out.Pools) != 1 || out.Pools[0].Name != "replicapool" {
			t.Fatalf("Pools not preserved: %+v", out.Pools)
		}
		if len(out.Services) != 1 || out.Services[0].Type != "mon" || out.Services[0].Running != 1 {
			t.Fatalf("Services not preserved: %+v", out.Services)
		}
	})

	// ---------------- StateFrontend ----------------
	t.Run("StateFrontend_nil_all_collections", func(t *testing.T) {
		s := StateFrontend{}
		out := s.NormalizeCollections()
		if out.ActiveAlerts == nil || out.RecentlyResolved == nil || out.Metrics == nil ||
			out.ConnectionHealth == nil || out.PVETagColors == nil || out.PVETagStyles == nil ||
			out.Resources == nil || out.ConnectedInfrastructure == nil {
			t.Fatalf("all top-level collection fields must be non-nil after normalize")
		}
		// Performance.APICallDuration is a nested nil-map branch.
		if out.Performance.APICallDuration == nil {
			t.Fatalf("Performance.APICallDuration should be non-nil after normalize")
		}
	})
	t.Run("StateFrontend_populated_recurses_into_nested", func(t *testing.T) {
		// Provide a non-nil Performance.APICallDuration to exercise the
		// "already populated" arm of that specific branch.
		s := StateFrontend{
			ActiveAlerts:     []Alert{{ID: "a-1"}},
			RecentlyResolved: []ResolvedAlert{{Alert: Alert{ID: "r-1"}}},
			Metrics:          []Metric{{Type: "cpu"}},
			ConnectionHealth: map[string]bool{"api": true},
			PVETagColors:     map[string]string{"prod": "#ff0000"},
			PVETagStyles:     map[string]PVETagStyle{"pve1": {}},
			Performance:      Performance{APICallDuration: map[string]float64{"nodes": 12.3}},
			// Each nested element has nil sub-collections so recursion is observable.
			ConnectedInfrastructure: []ConnectedInfrastructureItemFrontend{{ID: "ci-1"}}, // Surfaces nil
			Resources:               []ResourceFrontend{{ID: "res-1"}},                   // Tags/Labels/Alerts nil
		}
		out := s.NormalizeCollections()

		if len(out.ActiveAlerts) != 1 || out.ActiveAlerts[0].ID != "a-1" {
			t.Fatalf("ActiveAlerts not preserved")
		}
		if len(out.RecentlyResolved) != 1 || out.RecentlyResolved[0].ID != "r-1" {
			t.Fatalf("RecentlyResolved not preserved")
		}
		if len(out.Metrics) != 1 || out.Metrics[0].Type != "cpu" {
			t.Fatalf("Metrics not preserved")
		}
		if !out.ConnectionHealth["api"] {
			t.Fatalf("ConnectionHealth not preserved: %+v", out.ConnectionHealth)
		}
		if out.PVETagColors["prod"] != "#ff0000" {
			t.Fatalf("PVETagColors not preserved: %+v", out.PVETagColors)
		}
		// Performance.APICallDuration must be preserved as-is (non-nil arm).
		if got := out.Performance.APICallDuration["nodes"]; got != 12.3 {
			t.Fatalf("Performance.APICallDuration[\"nodes\"] = %v, want 12.3", got)
		}

		// Recursion over ConnectedInfrastructure.
		if len(out.ConnectedInfrastructure) != 1 || out.ConnectedInfrastructure[0].ID != "ci-1" {
			t.Fatalf("ConnectedInfrastructure not preserved")
		}
		if out.ConnectedInfrastructure[0].Surfaces == nil {
			t.Fatalf("nested ConnectedInfrastructureItemFrontend.Surfaces not normalized")
		}
		// Recursion over Resources.
		if len(out.Resources) != 1 || out.Resources[0].ID != "res-1" {
			t.Fatalf("Resources not preserved")
		}
		r := out.Resources[0]
		if r.Tags == nil || r.Labels == nil || r.Alerts == nil {
			t.Fatalf("nested ResourceFrontend collections not normalized: %+v", r)
		}
	})

	// ---------------- ResourceFrontend ----------------
	t.Run("ResourceFrontend_nil_and_nil_identity", func(t *testing.T) {
		r := ResourceFrontend{ID: "res-1"}
		out := r.NormalizeCollections()
		if out.Tags == nil || out.Labels == nil || out.Alerts == nil {
			t.Fatalf("Tags/Labels/Alerts should be non-nil after normalize")
		}
		if out.Identity != nil {
			t.Fatalf("Identity must remain nil when nil on input")
		}
	})
	t.Run("ResourceFrontend_populated_with_identity_recurses", func(t *testing.T) {
		r := ResourceFrontend{
			ID:       "res-1",
			Tags:     []string{"env:prod", "env:prod"}, // duplicate on purpose
			Labels:   map[string]string{"team": "infra"},
			Alerts:   []ResourceAlertFrontend{{ID: "al-1", Level: "warn"}},
			Identity: &ResourceIdentityFrontend{Hostname: "host-1"}, // IPs nil → recursion observable
		}
		out := r.NormalizeCollections()
		if len(out.Tags) != 2 || out.Tags[0] != "env:prod" || out.Tags[1] != "env:prod" {
			t.Fatalf("Tags not preserved (no dedup expected): %+v", out.Tags)
		}
		if len(out.Labels) != 1 || out.Labels["team"] != "infra" {
			t.Fatalf("Labels not preserved: %+v", out.Labels)
		}
		if len(out.Alerts) != 1 || out.Alerts[0].ID != "al-1" || out.Alerts[0].Level != "warn" {
			t.Fatalf("Alerts not preserved: %+v", out.Alerts)
		}
		// The `if r.Identity != nil` branch must have run and recursed.
		if out.Identity == nil {
			t.Fatalf("Identity must be preserved when non-nil on input")
		}
		if out.Identity.Hostname != "host-1" {
			t.Fatalf("Identity.Hostname not preserved: %q", out.Identity.Hostname)
		}
		if out.Identity.IPs == nil {
			t.Fatalf("nested Identity.IPs not normalized (recursion did not run)")
		}
	})
}
