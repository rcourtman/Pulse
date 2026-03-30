package unifiedresources

import "testing"

func TestRefreshCanonicalIdentityPrefersTargetsAndCanonicalHostData(t *testing.T) {
	resource := Resource{
		ID:   "agent-1",
		Type: ResourceTypeAgent,
		Name: "Tower",
		Identity: ResourceIdentity{
			Hostnames: []string{"tower.local"},
			MachineID: "machine-1",
		},
		DiscoveryTarget: &DiscoveryTarget{
			ResourceType: "agent",
			AgentID:      "host-1",
			ResourceID:   "host-1",
		},
		MetricsTarget: &MetricsTarget{
			ResourceType: "docker-host",
			ResourceID:   "docker-runtime-1",
		},
		Proxmox: &ProxmoxData{
			NodeName: "pve1",
		},
		Agent: &AgentData{
			AgentID:  "host-1",
			Hostname: "tower-agent.local",
		},
		Docker: &DockerData{
			HostSourceID: "docker-runtime-1",
			Hostname:     "tower-docker.local",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.DisplayName; got != "Tower" {
		t.Fatalf("displayName = %q, want Tower", got)
	}
	if got := resource.Canonical.Hostname; got != "tower.local" {
		t.Fatalf("hostname = %q, want tower.local", got)
	}
	if got := resource.Canonical.PlatformID; got != "pve1" {
		t.Fatalf("platformId = %q, want pve1", got)
	}
	if got := resource.Canonical.PrimaryID; got != "docker-host:docker-runtime-1" {
		t.Fatalf("primaryId = %q, want docker-host:docker-runtime-1", got)
	}

	wantAliases := []string{
		"docker-host:docker-runtime-1",
		"docker-runtime-1",
		"host-1",
		"pve1",
		"tower.local",
		"machine-1",
		"agent-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityFallsBackWithoutTargets(t *testing.T) {
	resource := Resource{
		ID:   "pbs-1",
		Type: ResourceTypePBS,
		Name: "",
		PBS: &PBSData{
			InstanceID: "pbs-main",
			Hostname:   "pbs.example",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.DisplayName; got != "pbs.example" {
		t.Fatalf("displayName = %q, want pbs.example", got)
	}
	if got := resource.Canonical.Hostname; got != "pbs.example" {
		t.Fatalf("hostname = %q, want pbs.example", got)
	}
	if got := resource.Canonical.PlatformID; got != "pbs.example" {
		t.Fatalf("platformId = %q, want pbs.example", got)
	}
	if got := resource.Canonical.PrimaryID; got != "pbs:pbs-main" {
		t.Fatalf("primaryId = %q, want pbs:pbs-main", got)
	}
}

func TestRefreshCanonicalIdentityPrefersProxmoxNodePrimaryIDForAgentResources(t *testing.T) {
	resource := Resource{
		ID:   "agent-1",
		Type: ResourceTypeAgent,
		Name: "pve1",
		Proxmox: &ProxmoxData{
			SourceID: "instance-pve1",
			NodeName: "pve1",
		},
		DiscoveryTarget: &DiscoveryTarget{
			ResourceType: "agent",
			ResourceID:   "host-1",
			AgentID:      "host-1",
		},
		Agent: &AgentData{
			AgentID:  "host-1",
			Hostname: "pve1",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PrimaryID; got != "node:instance-pve1" {
		t.Fatalf("primaryId = %q, want node:instance-pve1", got)
	}
}

func TestRefreshCanonicalIdentityCanonicalizesTargetResourceTypeAliases(t *testing.T) {
	resource := Resource{
		ID:   "agent-1",
		Type: ResourceTypeAgent,
		Name: "Tower",
		MetricsTarget: &MetricsTarget{
			ResourceType: " HOST ",
			ResourceID:   " host-1 ",
		},
		DiscoveryTarget: &DiscoveryTarget{
			ResourceType: "docker_host",
			ResourceID:   "docker-runtime-1",
			AgentID:      "host-1",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PrimaryID; got != "host:host-1" {
		t.Fatalf("primaryId = %q, want host:host-1", got)
	}

	wantAliases := []string{
		"host:host-1",
		"host-1",
		"docker-runtime-1",
		"Tower",
		"agent-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityFeedsPolicyMetadata(t *testing.T) {
	resource := Resource{
		ID:   "pbs-1",
		Type: ResourceTypePBS,
		Name: "primary-backup",
		PBS: &PBSData{
			InstanceID: "pbs-main",
			Hostname:   "backup.internal",
		},
		Identity: ResourceIdentity{
			IPAddresses: []string{"10.0.0.20"},
		},
	}

	RefreshCanonicalIdentity(&resource)
	RefreshPolicyMetadata(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if resource.Policy == nil {
		t.Fatalf("expected policy metadata")
	}
	if got := resource.Policy.Sensitivity; got != ResourceSensitivitySensitive {
		t.Fatalf("policy sensitivity = %q, want %q", got, ResourceSensitivitySensitive)
	}
	wantRedactions := []ResourceRedactionHint{
		ResourceRedactionHostname,
		ResourceRedactionIPAddress,
		ResourceRedactionPlatformID,
		ResourceRedactionAlias,
	}
	if len(resource.Policy.Routing.Redact) != len(wantRedactions) {
		t.Fatalf("redaction hint len = %d, want %d", len(resource.Policy.Routing.Redact), len(wantRedactions))
	}
	for i, want := range wantRedactions {
		if got := resource.Policy.Routing.Redact[i]; got != want {
			t.Fatalf("redaction[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityIgnoresProxmoxPoolAsPlatformIdentity(t *testing.T) {
	resource := Resource{
		ID:   "vm-101",
		Type: ResourceTypeVM,
		Name: "app-vm",
		Proxmox: &ProxmoxData{
			NodeName: "pve-a",
			Pool:     "prod-vms",
			VMID:     101,
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PlatformID; got != "pve-a" {
		t.Fatalf("platformId = %q, want pve-a", got)
	}
	for _, alias := range resource.Canonical.Aliases {
		if alias == "prod-vms" {
			t.Fatalf("expected pool name not to become canonical identity alias: %+v", resource.Canonical.Aliases)
		}
	}
}

func TestRefreshCanonicalIdentityScopesVMwarePrimaryIDToConnection(t *testing.T) {
	resource := Resource{
		ID:   "vm-1",
		Type: ResourceTypeVM,
		Name: "db-vm",
		VMware: &VMwareData{
			ConnectionID:    "vc-1",
			ManagedObjectID: "vm-101",
			EntityType:      "vm",
			HostUUID:        "uuid-host-1",
		},
		Identity: ResourceIdentity{
			Hostnames: []string{"db-vm.lab.local"},
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PrimaryID; got != "vmware:vc-1:vm:vm-101" {
		t.Fatalf("primaryId = %q, want vmware:vc-1:vm:vm-101", got)
	}
	wantAliases := []string{
		"vmware:vc-1:vm:vm-101",
		"vm-101",
		"uuid-host-1",
		"db-vm",
		"db-vm.lab.local",
		"vm-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityScopesVMwareManagedObjectsByConnection(t *testing.T) {
	resource := Resource{
		ID:   "vmware-host-1",
		Type: ResourceTypeAgent,
		Name: "esxi-01.lab.local",
		Identity: ResourceIdentity{
			Hostnames: []string{"esxi-01.lab.local"},
		},
		VMware: &VMwareData{
			ConnectionID:    "vc-1",
			ConnectionName:  "Lab VC",
			VCenterHost:     "vc.lab.local",
			ManagedObjectID: "host-101",
			EntityType:      "host",
			HostUUID:        "uuid-host-1",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.DisplayName; got != "esxi-01.lab.local" {
		t.Fatalf("displayName = %q, want esxi-01.lab.local", got)
	}
	if got := resource.Canonical.Hostname; got != "esxi-01.lab.local" {
		t.Fatalf("hostname = %q, want esxi-01.lab.local", got)
	}
	if got := resource.Canonical.PlatformID; got != "esxi-01.lab.local" {
		t.Fatalf("platformId = %q, want esxi-01.lab.local", got)
	}
	if got := resource.Canonical.PrimaryID; got != "vmware:vc-1:host:host-101" {
		t.Fatalf("primaryId = %q, want vmware:vc-1:host:host-101", got)
	}

	wantAliases := []string{
		"vmware:vc-1:host:host-101",
		"host-101",
		"uuid-host-1",
		"esxi-01.lab.local",
		"vmware-host-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestRefreshCanonicalIdentityIgnoresVMwarePlacementDetailAliases(t *testing.T) {
	resource := Resource{
		ID:   "vmware-vm-1",
		Type: ResourceTypeVM,
		Name: "db-vm",
		Identity: ResourceIdentity{
			Hostnames: []string{"db-vm.guest.lab.local"},
			MachineID: "vm-instance-uuid-1",
		},
		VMware: &VMwareData{
			ConnectionID:        "vc-1",
			ManagedObjectID:     "vm-101",
			EntityType:          "vm",
			DatacenterName:      "DC-1",
			ComputeResourceName: "Compute-A",
			ClusterName:         "Cluster-A",
			FolderName:          "Production",
			ResourcePoolName:    "Prod-VMs",
			RuntimeHostName:     "esxi-01.lab.local",
			DatastoreNames:      []string{"vmfs-prod-01", "vmfs-prod-02"},
			GuestHostname:       "db-vm.guest.lab.local",
			GuestIPAddresses:    []string{"10.0.0.10", "10.0.0.11"},
			InstanceUUID:        "vm-instance-uuid-1",
			BIOSUUID:            "vm-bios-uuid-1",
		},
	}

	RefreshCanonicalIdentity(&resource)

	if resource.Canonical == nil {
		t.Fatalf("expected canonical identity")
	}
	if got := resource.Canonical.PrimaryID; got != "vmware:vc-1:vm:vm-101" {
		t.Fatalf("primaryId = %q, want vmware:vc-1:vm:vm-101", got)
	}

	wantAliases := []string{
		"vmware:vc-1:vm:vm-101",
		"vm-101",
		"db-vm",
		"db-vm.guest.lab.local",
		"vm-instance-uuid-1",
		"vmware-vm-1",
	}
	if len(resource.Canonical.Aliases) != len(wantAliases) {
		t.Fatalf("aliases len = %d, want %d (%v)", len(resource.Canonical.Aliases), len(wantAliases), resource.Canonical.Aliases)
	}
	for i, want := range wantAliases {
		if got := resource.Canonical.Aliases[i]; got != want {
			t.Fatalf("alias[%d] = %q, want %q", i, got, want)
		}
	}

	disallowedAliases := []string{
		"DC-1",
		"Compute-A",
		"Cluster-A",
		"Production",
		"Prod-VMs",
		"esxi-01.lab.local",
		"vmfs-prod-01",
		"vmfs-prod-02",
		"10.0.0.10",
		"10.0.0.11",
		"vm-bios-uuid-1",
	}
	for _, disallowed := range disallowedAliases {
		for _, alias := range resource.Canonical.Aliases {
			if alias == disallowed {
				t.Fatalf("expected VMware topology/detail value %q not to become canonical alias: %+v", disallowed, resource.Canonical.Aliases)
			}
		}
	}
}
