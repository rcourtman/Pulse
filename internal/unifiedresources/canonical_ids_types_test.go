package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCanonicalResourceTypeDoesNotAliasHost(t *testing.T) {
	if got := CanonicalResourceType(ResourceType("host")); got != ResourceType("host") {
		t.Fatalf("CanonicalResourceType(host) = %q, want %q", got, ResourceType("host"))
	}

	if got := CanonicalResourceType(ResourceType("HOST")); got != ResourceType("host") {
		t.Fatalf("CanonicalResourceType(HOST) = %q, want %q", got, ResourceType("host"))
	}

	if got := CanonicalResourceType(ResourceType("agent")); got != ResourceTypeAgent {
		t.Fatalf("CanonicalResourceType(agent) = %q, want %q", got, ResourceTypeAgent)
	}
}

func TestContractResourceType(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		want     ResourceType
	}{
		{
			name: "proxmox agent uses canonical agent contract type",
			resource: Resource{
				Type:    ResourceTypeAgent,
				Proxmox: &ProxmoxData{},
			},
			want: ResourceTypeAgent,
		},
		{
			name: "docker host uses docker-host contract type",
			resource: Resource{
				Type:   ResourceTypeAgent,
				Docker: &DockerData{},
			},
			want: ResourceType("docker-host"),
		},
		{
			name: "vmware host stays agent",
			resource: Resource{
				Type:   ResourceTypeAgent,
				VMware: &VMwareData{},
			},
			want: ResourceTypeAgent,
		},
		{
			name: "truenas host stays agent",
			resource: Resource{
				Type:    ResourceTypeAgent,
				TrueNAS: &TrueNASData{},
			},
			want: ResourceTypeAgent,
		},
		{
			name: "workload passthrough remains canonical",
			resource: Resource{
				Type: ResourceTypeVM,
			},
			want: ResourceTypeVM,
		},
		{
			name: "network share passthrough remains canonical",
			resource: Resource{
				Type: ResourceTypeNetworkShare,
				TrueNAS: &TrueNASData{
					Share: &TrueNASShare{ID: "smb-media", Protocol: "SMB"},
				},
			},
			want: ResourceTypeNetworkShare,
		},
		{
			name:     "docker image passthrough remains canonical",
			resource: Resource{Type: ResourceTypeDockerImage, Docker: &DockerData{ImageID: "sha256:image1"}},
			want:     ResourceTypeDockerImage,
		},
		{
			name:     "docker volume passthrough remains canonical",
			resource: Resource{Type: ResourceTypeDockerVolume, Docker: &DockerData{VolumeName: "app-data"}},
			want:     ResourceTypeDockerVolume,
		},
		{
			name:     "kubernetes service passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sService, Kubernetes: &K8sData{ServiceType: "ClusterIP"}},
			want:     ResourceTypeK8sService,
		},
		{
			name:     "kubernetes replicaset passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sReplicaSet, Kubernetes: &K8sData{OwnerKind: "Deployment"}},
			want:     ResourceTypeK8sReplicaSet,
		},
		{
			name:     "kubernetes endpoint slice passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sEndpointSlice, Kubernetes: &K8sData{ServiceName: "checkout"}},
			want:     ResourceTypeK8sEndpointSlice,
		},
		{
			name:     "kubernetes network policy passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sNetworkPolicy, Kubernetes: &K8sData{PolicyTypes: []string{"Ingress"}}},
			want:     ResourceTypeK8sNetworkPolicy,
		},
		{
			name:     "kubernetes storage class passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sStorageClass, Kubernetes: &K8sData{Provisioner: "csi.example.test"}},
			want:     ResourceTypeK8sStorageClass,
		},
		{
			name:     "kubernetes configmap passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sConfigMap, Kubernetes: &K8sData{DataKeys: []string{"app.yaml"}}},
			want:     ResourceTypeK8sConfigMap,
		},
		{
			name:     "kubernetes serviceaccount passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sServiceAccount, Kubernetes: &K8sData{SecretCount: 1}},
			want:     ResourceTypeK8sServiceAccount,
		},
		{
			name:     "kubernetes event passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sEvent, Kubernetes: &K8sData{Reason: "BackOff"}},
			want:     ResourceTypeK8sEvent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContractResourceType(tt.resource); got != tt.want {
				t.Fatalf("ContractResourceType(%+v) = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}

func TestTrueNASServiceInventoryStaysOnSystemResource(t *testing.T) {
	resource := Resource{
		Type: ResourceTypeAgent,
		TrueNAS: &TrueNASData{
			Hostname: "truenas-a",
			Services: []TrueNASService{
				{
					ID:      "cifs",
					Service: "smb",
					Enabled: true,
					State:   "RUNNING",
					PIDs:    []int{1234, 5678},
				},
			},
		},
	}

	if got := ContractResourceType(resource); got != ResourceTypeAgent {
		t.Fatalf("ContractResourceType(TrueNAS system with services) = %q, want %q", got, ResourceTypeAgent)
	}
	if len(resource.TrueNAS.Services) != 1 {
		t.Fatalf("TrueNAS service inventory length = %d, want 1", len(resource.TrueNAS.Services))
	}
	service := resource.TrueNAS.Services[0]
	if service.ID != "cifs" || service.Service != "smb" || !service.Enabled || service.State != "RUNNING" {
		t.Fatalf("unexpected TrueNAS service metadata: %+v", service)
	}
	if len(service.PIDs) != 2 || service.PIDs[0] != 1234 || service.PIDs[1] != 5678 {
		t.Fatalf("unexpected TrueNAS service pid metadata: %+v", service.PIDs)
	}
}

func TestCanonicalResourceIDDoesNotAliasLegacyHostPrefixes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "host colon prefix remains unchanged",
			in:   "host:alpha",
			want: "host:alpha",
		},
		{
			name: "host dash prefix remains unchanged",
			in:   "host-alpha",
			want: "host-alpha",
		},
		{
			name: "agent prefix unchanged",
			in:   "agent:alpha",
			want: "agent:alpha",
		},
		{
			name: "trims surrounding whitespace only",
			in:   "  host:trim-me  ",
			want: "host:trim-me",
		},
		{
			name: "empty becomes empty",
			in:   "   ",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanonicalResourceID(tc.in); got != tc.want {
				t.Fatalf("CanonicalResourceID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsUnsupportedLegacyResourceTypeAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host alias", in: "host", want: true},
		{name: "host mixed case alias", in: " HoSt ", want: true},
		{name: "legacy system_container alias", in: "system_container", want: true},
		{name: "legacy docker_container alias", in: "docker_container", want: true},
		{name: "legacy app_container alias", in: "app_container", want: true},
		{name: "legacy docker_host alias", in: "docker_host", want: true},
		{name: "legacy kubernetes_cluster alias", in: "kubernetes_cluster", want: true},
		{name: "legacy k8s_cluster alias", in: "k8s_cluster", want: true},
		{name: "agent type", in: "agent", want: false},
		{name: "empty", in: "  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnsupportedLegacyResourceTypeAlias(tt.in); got != tt.want {
				t.Fatalf("IsUnsupportedLegacyResourceTypeAlias(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalizeLegacyResourceTypeAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "host", in: "host", want: "agent", ok: true},
		{name: "system_container", in: "system_container", want: "system-container", ok: true},
		{name: "docker_container", in: "docker_container", want: "app-container", ok: true},
		{name: "app_container", in: "app_container", want: "app-container", ok: true},
		{name: "docker_host", in: "docker_host", want: "docker-host", ok: true},
		{name: "kubernetes_cluster", in: "kubernetes_cluster", want: "k8s-cluster", ok: true},
		{name: "k8s_cluster", in: "k8s_cluster", want: "k8s-cluster", ok: true},
		{name: "canonical_passthrough_rejected", in: "agent", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalizeLegacyResourceTypeAlias(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("CanonicalizeLegacyResourceTypeAlias(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResourceRelationshipFieldsDefaultToNil(t *testing.T) {
	r := Resource{}
	if r.Capabilities != nil {
		t.Error("Capabilities should default to nil")
	}
	if r.Relationships != nil {
		t.Error("Relationships should default to nil")
	}
	if r.RecentChanges != nil {
		t.Error("RecentChanges should default to nil")
	}
}

func TestPhysicalDiskTemperatureAggregateDefaultsToNil(t *testing.T) {
	meta := PhysicalDiskMeta{}
	if meta.TemperatureAggregate != nil {
		t.Fatalf("TemperatureAggregate should default to nil, got %+v", meta.TemperatureAggregate)
	}

	meta.TemperatureAggregate = &TemperatureAggregateMeta{
		WindowDays: 7,
		MinCelsius: 29.0,
		AvgCelsius: 32.7,
		MaxCelsius: 38.0,
	}
	if meta.TemperatureAggregate.WindowDays != 7 || meta.TemperatureAggregate.MaxCelsius != 38.0 {
		t.Fatalf("unexpected temperature aggregate assignment: %+v", meta.TemperatureAggregate)
	}
}

func TestHostUnraidDiskSourceIDNormalizesDeviceAndPrefersSerial(t *testing.T) {
	host := models.Host{ID: "host-tower"}

	tests := []struct {
		name string
		disk models.HostUnraidDisk
		want string
	}{
		{
			name: "plain dev path",
			disk: models.HostUnraidDisk{Device: "/dev/sdd"},
			want: "host-tower:sdd",
		},
		{
			name: "smartctl transport suffix",
			disk: models.HostUnraidDisk{Device: "sdf [sat]"},
			want: "host-tower:sdf",
		},
		{
			name: "serial wins",
			disk: models.HostUnraidDisk{Device: "/dev/sdg", Serial: "SERIAL-DATA"},
			want: "SERIAL-DATA",
		},
		{
			name: "slot fallback",
			disk: models.HostUnraidDisk{Name: "disk1"},
			want: "host-tower:unraid-slot:disk1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostUnraidDiskSourceID(host, tt.disk); got != tt.want {
				t.Fatalf("HostUnraidDiskSourceID(%+v) = %q, want %q", tt.disk, got, tt.want)
			}
		})
	}
}

func TestIsUnsupportedLegacyResourceIDAlias(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host prefixed id", in: "host:alpha", want: true},
		{name: "host mixed case prefixed id", in: " HoSt:alpha ", want: true},
		{name: "agent id", in: "agent:alpha", want: false},
		{name: "host without colon", in: "host-alpha", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUnsupportedLegacyResourceIDAlias(tt.in); got != tt.want {
				t.Fatalf("IsUnsupportedLegacyResourceIDAlias(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
