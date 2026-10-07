package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestProxmoxRuntimeStatusJSONContract(t *testing.T) {
	payload := ProxmoxData{
		SourceID:      "lab:node-a:101",
		RuntimeStatus: "running",
		NodeName:      "node-a",
		VMID:          101,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal ProxmoxData: %v", err)
	}
	if !strings.Contains(string(data), `"runtimeStatus":"running"`) {
		t.Fatalf("ProxmoxData JSON did not carry runtimeStatus: %s", data)
	}
}

func TestHostSMARTMetaCarriesSizeBytesJSONContract(t *testing.T) {
	payload := HostSMARTMeta{
		Device:    "/dev/sda",
		Model:     "CT240BX500SSD1",
		Serial:    "SATA-SERIAL-1",
		Type:      "sata",
		SizeBytes: 240_057_409_536,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal HostSMARTMeta: %v", err)
	}
	if !strings.Contains(string(data), `"sizeBytes":240057409536`) {
		t.Fatalf("HostSMARTMeta JSON did not carry sizeBytes: %s", data)
	}

	var decoded HostSMARTMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal HostSMARTMeta: %v", err)
	}
	if decoded.SizeBytes != payload.SizeBytes {
		t.Fatalf("decoded sizeBytes = %d, want %d", decoded.SizeBytes, payload.SizeBytes)
	}
}

func TestHostCustomSensorMetaJSONContract(t *testing.T) {
	value := 12.5
	observedAt := "2026-07-30T19:00:00Z"
	eventAt := "2026-07-30T18:00:00Z"
	payload := HostSensorMeta{
		Custom: []HostCustomSensorMetric{{
			ID:         "queue_depth",
			Name:       "Queue depth",
			Group:      "Main server",
			Subgroup:   "Backup",
			Kind:       "timestamp",
			Unit:       "items",
			Value:      &value,
			Status:     "warning",
			ObservedAt: time.Date(2026, 7, 30, 19, 0, 0, 0, time.UTC),
			EventAt:    timePointer(time.Date(2026, 7, 30, 18, 0, 0, 0, time.UTC)),
		}},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal HostSensorMeta: %v", err)
	}
	for _, expected := range []string{
		`"custom":[`,
		`"id":"queue_depth"`,
		`"group":"Main server"`,
		`"subgroup":"Backup"`,
		`"kind":"timestamp"`,
		`"value":12.5`,
		`"status":"warning"`,
		`"observedAt":"` + observedAt + `"`,
		`"eventAt":"` + eventAt + `"`,
	} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("custom sensor JSON missing %s: %s", expected, data)
		}
	}
}

func TestSMARTMetaJSONPreservesReportedZeroAndOmitsAbsentFields(t *testing.T) {
	zeroInt := 0
	zeroInt64 := int64(0)
	payload := SMARTMeta{
		PercentageUsed:     &zeroInt,
		AvailableSpare:     &zeroInt,
		ReallocatedSectors: &zeroInt64,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal SMARTMeta: %v", err)
	}
	for _, expected := range []string{
		`"percentageUsed":0`,
		`"availableSpare":0`,
		`"reallocatedSectors":0`,
	} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("reported zero field %s was omitted: %s", expected, data)
		}
	}
	if strings.Contains(string(data), `"powerOnHours"`) {
		t.Fatalf("absent SMART field was serialized: %s", data)
	}

	var decoded SMARTMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal SMARTMeta: %v", err)
	}
	if decoded.PercentageUsed == nil || *decoded.PercentageUsed != 0 ||
		decoded.AvailableSpare == nil || *decoded.AvailableSpare != 0 ||
		decoded.ReallocatedSectors == nil || *decoded.ReallocatedSectors != 0 {
		t.Fatalf("reported zero fields did not survive round trip: %+v", decoded)
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
			name: "docker app container metadata keeps app-container contract type",
			resource: Resource{
				Type: ResourceTypeAppContainer,
				Docker: &DockerData{
					ContainerID: "container-1",
					BlockIO:     &DockerContainerBlockIOMeta{ReadBytes: 1024, WriteBytes: 2048},
					Podman:      &DockerPodmanContainerMeta{PodName: "edge-pod", ComposeProject: "orion"},
				},
			},
			want: ResourceTypeAppContainer,
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
			name:     "docker secret passthrough remains canonical",
			resource: Resource{Type: ResourceTypeDockerSecret, Docker: &DockerData{SecretID: "secret1", SecretName: "api-token"}},
			want:     ResourceTypeDockerSecret,
		},
		{
			name:     "docker config passthrough remains canonical",
			resource: Resource{Type: ResourceTypeDockerConfig, Docker: &DockerData{ConfigID: "config1", ConfigName: "nginx-conf"}},
			want:     ResourceTypeDockerConfig,
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
			name:     "kubernetes metadata-only configmap passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sConfigMap, Kubernetes: &K8sData{MetadataOnly: true}},
			want:     ResourceTypeK8sConfigMap,
		},
		{
			name:     "kubernetes metadata-only secret passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sSecret, Kubernetes: &K8sData{MetadataOnly: true}},
			want:     ResourceTypeK8sSecret,
		},
		{
			name:     "kubernetes serviceaccount passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sServiceAccount, Kubernetes: &K8sData{SecretCount: 1}},
			want:     ResourceTypeK8sServiceAccount,
		},
		{
			name:     "kubernetes resource quota passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sResourceQuota, Kubernetes: &K8sData{Hard: map[string]string{"pods": "10"}}},
			want:     ResourceTypeK8sResourceQuota,
		},
		{
			name:     "kubernetes limit range passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sLimitRange, Kubernetes: &K8sData{LimitTypes: []string{"Container"}}},
			want:     ResourceTypeK8sLimitRange,
		},
		{
			name:     "kubernetes pod disruption budget passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sPDB, Kubernetes: &K8sData{ExpectedPods: 2}},
			want:     ResourceTypeK8sPDB,
		},
		{
			name:     "kubernetes horizontal pod autoscaler passthrough remains canonical",
			resource: Resource{Type: ResourceTypeK8sHPA, Kubernetes: &K8sData{TargetName: "checkout"}},
			want:     ResourceTypeK8sHPA,
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
			name: "serial wins, scoped to its host",
			disk: models.HostUnraidDisk{Device: "/dev/sdg", Serial: "SERIAL-DATA"},
			want: "host-tower/physical-disk:SERIAL-DATA",
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

// An Unraid inventory row's history key is the one the metrics target of the
// disk resource built from it reads: its usable serial, unscoped like every
// physical-disk history key, or else its host/device source ID. A row without
// a device is not ingested as a disk and has no key.
func TestHostUnraidDiskMetricIDMatchesItsDiskMetricsTarget(t *testing.T) {
	host := models.Host{ID: "host-tower", Hostname: "tower"}
	tests := []struct {
		name string
		disk models.HostUnraidDisk
		want string
	}{
		{"serial", models.HostUnraidDisk{Device: "/dev/sdb", Serial: "SERIAL-DATA"}, "SERIAL-DATA"},
		{"placeholder serial", models.HostUnraidDisk{Device: "sdc", Serial: "N/A"}, "host-tower:sdc"},
		{"no serial", models.HostUnraidDisk{Device: "sdd [sat]"}, "host-tower:sdd"},
		{"no device", models.HostUnraidDisk{Name: "disk9", Serial: "SERIAL-MISSING"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HostUnraidDiskMetricID(host, tt.disk)
			if got != tt.want {
				t.Fatalf("HostUnraidDiskMetricID(%+v) = %q, want %q", tt.disk, got, tt.want)
			}
			if tt.want == "" {
				return
			}
			disk := tt.disk
			disk.Name, disk.Role, disk.Status = "disk1", "data", "online"
			registry := NewRegistry(nil)
			registry.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{{
				ID: host.ID, Hostname: host.Hostname, Status: "online",
				Unraid: &models.HostUnraidStorage{ArrayStarted: true, Disks: []models.HostUnraidDisk{disk}},
			}}})
			disks := registry.ListByType(ResourceTypePhysicalDisk)
			if len(disks) != 1 {
				t.Fatalf("disks = %d, want 1", len(disks))
			}
			target := registry.MetricsTarget(disks[0].ID)
			if target == nil || target.ResourceType != "disk" || target.ResourceID != got {
				t.Fatalf("metrics target = %+v, want disk %q", target, got)
			}
		})
	}
}

// An agent disk's source ID carries its host, because a usable serial or WWN
// names the drive but not the machine; its metrics key is the hardware ID
// alone. Without hardware identity the two keep one host/device/topology key.
// The source-specific ID an operator split derives from still hashes the bare
// hardware ID it was derived from before the host joined the source ID.
func TestHostSMARTDiskSourceIDScopesHardwareIdentityToItsHost(t *testing.T) {
	host := models.Host{ID: "host-tower"}
	tests := []struct {
		name       string
		disk       models.HostDiskSMART
		wantSource string
		wantMetric string
	}{
		{
			name:       "serial",
			disk:       models.HostDiskSMART{Device: "/dev/sda", Serial: "SERIAL-A", WWN: "5000c500a1b2c3d4"},
			wantSource: "host-tower/physical-disk:SERIAL-A",
			wantMetric: "SERIAL-A",
		},
		{
			name:       "wwn when the serial is a placeholder",
			disk:       models.HostDiskSMART{Device: "sdb", Serial: "To Be Filled By O.E.M.", WWN: "5000c500a1b2c3d4"},
			wantSource: "host-tower/physical-disk:5000c500a1b2c3d4",
			wantMetric: "5000c500a1b2c3d4",
		},
		{
			name:       "no hardware identity",
			disk:       models.HostDiskSMART{Device: "sdc [sat]", Serial: "0000000000"},
			wantSource: "host-tower:sdc",
			wantMetric: "host-tower:sdc",
		},
		{
			name:       "identity-less controller member",
			disk:       models.HostDiskSMART{Device: "sdd", Controller: "ctrl0", Target: "megaraid,3"},
			wantSource: "host-tower:sdd@ctrl0/megaraid,3",
			wantMetric: "host-tower:sdd@ctrl0/megaraid,3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostSMARTDiskSourceID(host, tt.disk); got != tt.wantSource {
				t.Fatalf("HostSMARTDiskSourceID = %q, want %q", got, tt.wantSource)
			}
			if got := HostSMARTDiskMetricID(host, tt.disk); got != tt.wantMetric {
				t.Fatalf("HostSMARTDiskMetricID = %q, want %q", got, tt.wantMetric)
			}
		})
	}

	sourceID := HostSMARTDiskSourceID(host, tests[0].disk)
	if got, want := SourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, sourceID), SourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, "SERIAL-A"); got != want {
		t.Fatalf("agent disk source-specific ID = %q, want the bare serial's %q", got, want)
	}
	rr := NewRegistry(nil)
	if got, want := rr.sourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, sourceID), SourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, sourceID); got != want {
		t.Fatalf("registry source-specific ID = %q, want SourceSpecificID's %q", got, want)
	}
	if got, want := SourceSpecificID(ResourceTypeStorage, SourceAgent, "host-tower/physical-disk:x"), SourceSpecificID(ResourceTypeStorage, SourceAgent, "x"); got == want {
		t.Fatalf("only agent physical disks drop the host from their source-specific ID, got %q for storage", got)
	}
}

// A SMART row without a serial takes the one the host's Unraid inventory
// reports for the disk, and its metrics key follows, because the disk resource
// carries that serial and its metrics target reads it. The source ID stays the
// row's own. A row's own serial, even a placeholder, is kept, as the adapter
// keeps it, and the Unraid row then stays a disk of its own. So does a row on a
// device path several rows share: controller members behind one block device
// keep their own identity. A disk with no SMART row is keyed by its Unraid
// serial alone, unless several Unraid rows name its device.
func TestHostSMARTDiskMetricIDTakesTheSerialItsUnraidRowReports(t *testing.T) {
	host := models.Host{ID: "host-tower", Hostname: "tower", MachineID: "machine-tower", Status: "online", LastSeen: time.Now().UTC()}
	host.Unraid = &models.HostUnraidStorage{ArrayStarted: true, Disks: []models.HostUnraidDisk{
		{Name: "disk1", Device: "sdb", Role: "data", Serial: "UNRAID-B"},
		{Name: "disk2", Device: "sdc", Role: "data", Serial: "UNRAID-C"},
		{Name: "disk3", Device: "sdd", Role: "data", Serial: "UNRAID-D"},
		{Name: "disk4", Device: "sde", Role: "data", Serial: "UNRAID-E"},
		{Name: "disk5", Device: "sdf", Role: "data", Serial: "UNRAID-F"},
		{Name: "disk6", Device: "sdg", Role: "data"},
		{Name: "disk7", Device: "sdh", Role: "data", Serial: "UNRAID-H"},
		{Name: "disk8", Device: "sdi", Role: "data", Serial: "UNRAID-I1"},
		{Name: "disk9", Device: "sdi", Role: "data", Serial: "UNRAID-I2"},
	}}
	tests := []struct {
		name       string
		disk       models.HostDiskSMART
		wantSource string
		wantMetric string
	}{
		{
			name:       "no hardware identity",
			disk:       models.HostDiskSMART{Device: "/dev/sdb"},
			wantSource: "host-tower:sdb",
			wantMetric: "UNRAID-B",
		},
		{
			name:       "standby row",
			disk:       models.HostDiskSMART{Device: "sdc", Standby: true},
			wantSource: "host-tower:sdc",
			wantMetric: "UNRAID-C",
		},
		{
			name:       "wwn only",
			disk:       models.HostDiskSMART{Device: "sdd", WWN: "5000c500a1b2c3d4"},
			wantSource: "host-tower/physical-disk:5000c500a1b2c3d4",
			wantMetric: "UNRAID-D",
		},
		{
			name:       "own serial",
			disk:       models.HostDiskSMART{Device: "sde", Serial: "SMART-E"},
			wantSource: "host-tower/physical-disk:SMART-E",
			wantMetric: "SMART-E",
		},
		{
			name:       "own placeholder serial",
			disk:       models.HostDiskSMART{Device: "sdf", Serial: "0000000000"},
			wantSource: "host-tower:sdf",
			wantMetric: "host-tower:sdf",
		},
		{
			name:       "unraid row without serial",
			disk:       models.HostDiskSMART{Device: "sdg"},
			wantSource: "host-tower:sdg",
			wantMetric: "host-tower:sdg",
		},
		{
			name:       "controller member sharing a path",
			disk:       models.HostDiskSMART{Device: "sdh", WWN: "5000c500a1b2c3e0", Controller: "ctrl0", Target: "megaraid,0"},
			wantSource: "host-tower/physical-disk:5000c500a1b2c3e0",
			wantMetric: "5000c500a1b2c3e0",
		},
		{
			name:       "second controller member on that path",
			disk:       models.HostDiskSMART{Device: "/dev/sdh", WWN: "5000c500a1b2c3e1", Controller: "ctrl0", Target: "megaraid,1"},
			wantSource: "host-tower/physical-disk:5000c500a1b2c3e1",
			wantMetric: "5000c500a1b2c3e1",
		},
	}
	host.Sensors.SMART = make([]models.HostDiskSMART, 0, len(tests))
	for _, tt := range tests {
		host.Sensors.SMART = append(host.Sensors.SMART, tt.disk)
	}
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{Hosts: []models.Host{host}})
	targetBySourceID := make(map[string]string)
	for _, disk := range rr.ListByType(ResourceTypePhysicalDisk) {
		target := rr.MetricsTarget(disk.ID)
		if target == nil {
			continue
		}
		for _, sourceTarget := range rr.SourceTargets(disk.ID) {
			if sourceTarget.Source == SourceAgent {
				targetBySourceID[sourceTarget.SourceID] = target.ResourceID
			}
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostSMARTDiskSourceID(host, tt.disk); got != tt.wantSource {
				t.Fatalf("HostSMARTDiskSourceID = %q, want %q", got, tt.wantSource)
			}
			if got := HostSMARTDiskMetricID(host, tt.disk); got != tt.wantMetric {
				t.Fatalf("HostSMARTDiskMetricID = %q, want %q", got, tt.wantMetric)
			}
			if got := targetBySourceID[tt.wantSource]; got != tt.wantMetric {
				t.Fatalf("disk metrics target = %q, want the writer's %q", got, tt.wantMetric)
			}
		})
	}

	for device, want := range map[string]string{"/dev/sde": "UNRAID-E", "sdg": "", "sdi": "", "sdz": ""} {
		if got := HostUnraidDeviceMetricID(host, device); got != want {
			t.Fatalf("HostUnraidDeviceMetricID(%q) = %q, want %q", device, got, want)
		}
	}
}

// A registry seeded from unified resources rebuilds each agent disk's source
// mapping from the resource alone, after a JSON round trip drops per-source
// parents. It must reproduce the key the agent's observation ingests under,
// for a disk on the host, behind a controller, in the Unraid array or a cache
// pool, or merged with a linked node's Proxmox row, with or without hardware
// identity, so a rehydrated disk keeps its agent source and metrics targets.
func TestRehydratedAgentDisksKeepTheirLiveSourceIDs(t *testing.T) {
	now := time.Now().UTC()
	tower := models.Host{
		ID: "host-tower", Hostname: "tower", MachineID: "machine-tower", Status: "online", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{
			{Device: "sda", Serial: "BOOT-SERIAL", Model: "Boot SSD", Health: "PASSED"},
			{Device: "sdb", Serial: "ARRAY-SERIAL", Model: "Array Disk", Health: "PASSED"},
			{Device: "sdc", Model: "Array Disk", Health: "PASSED"},
			{Device: "nvme0n1", Serial: "CACHE-SERIAL", Model: "Cache NVMe", Health: "PASSED"},
			{Device: "sdd", Model: "RAID Member", Controller: "ctrl0", Target: "megaraid,3", Health: "PASSED"},
		}},
		Unraid: &models.HostUnraidStorage{ArrayStarted: true, ArrayState: "STARTED", Disks: []models.HostUnraidDisk{
			{Name: "disk1", Device: "sdb", Role: "data", Status: "DISK_OK", Serial: "ARRAY-SERIAL"},
			{Name: "disk2", Device: "sdc", Role: "data", Status: "DISK_OK"},
			{Name: "cache", Device: "nvme0n1", Role: "cache", Status: "DISK_OK", Serial: "CACHE-SERIAL"},
		}},
	}
	for _, tc := range []struct {
		name          string
		snapshot      models.StateSnapshot
		wantSourceIDs map[string]bool
	}{
		{
			name:     "unraid host",
			snapshot: models.StateSnapshot{Hosts: []models.Host{tower}},
			wantSourceIDs: map[string]bool{
				"host-tower/physical-disk:BOOT-SERIAL":  true,
				"host-tower/physical-disk:ARRAY-SERIAL": true,
				"host-tower:sdc":                        true,
				"host-tower/physical-disk:CACHE-SERIAL": true,
				"host-tower:sdd@ctrl0/megaraid,3":       true,
			},
		},
		{
			name:          "linked proxmox node",
			snapshot:      sharedSerialDiskSnapshot(now, true, "pve1"),
			wantSourceIDs: map[string]bool{"host-pve1/physical-disk:" + sharedDiskSerial: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			live := NewRegistry(nil)
			live.IngestSnapshot(tc.snapshot)
			payload, err := json.Marshal(live.List())
			if err != nil {
				t.Fatal(err)
			}
			var persisted []Resource
			if err := json.Unmarshal(payload, &persisted); err != nil {
				t.Fatal(err)
			}
			rehydrated := NewRegistry(nil)
			rehydrated.IngestResources(persisted)

			agentSourceID := func(rr *ResourceRegistry, diskID string) string {
				for _, target := range rr.SourceTargets(diskID) {
					if target.Source == SourceAgent {
						return target.SourceID
					}
				}
				return ""
			}
			seen := make(map[string]bool, len(tc.wantSourceIDs))
			for _, disk := range live.ListByType(ResourceTypePhysicalDisk) {
				liveID := agentSourceID(live, disk.ID)
				if liveID == "" {
					continue
				}
				seen[liveID] = true
				if rehydratedID := agentSourceID(rehydrated, disk.ID); rehydratedID != liveID {
					t.Fatalf("disk %s rehydrated agent source ID = %q, want the live %q", disk.Name, rehydratedID, liveID)
				}
				if live, seeded := live.MetricsTarget(disk.ID), rehydrated.MetricsTarget(disk.ID); live == nil || seeded == nil || *live != *seeded {
					t.Fatalf("disk %s metrics target live %+v, rehydrated %+v, want one non-nil target", disk.Name, live, seeded)
				}
			}
			if len(seen) != len(tc.wantSourceIDs) {
				t.Fatalf("live agent source IDs = %v, want %v", seen, tc.wantSourceIDs)
			}
			for id := range tc.wantSourceIDs {
				if !seen[id] {
					t.Fatalf("live agent source IDs = %v, want %v", seen, tc.wantSourceIDs)
				}
			}
		})
	}
}

// Two agent IDs can report from one machine (a re-enrolled agent whose old
// record still reports), so their disks no longer share a source key, and an
// operator split sends the second onto the source-specific ID, which hashes
// the bare serial. Another machine's split copy must then take that ID keyed
// to its own machine, never overwrite the first machine's disk.
func TestSplitAgentDisksOnTwoMachinesNeverOverwriteEachOther(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	host := func(id, machine string) models.Host {
		return models.Host{
			ID: id, Hostname: machine, MachineID: "machine-" + machine, Status: "online", LastSeen: now,
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{
				Device: "/dev/sda", Model: "SEAGATE ST4000NM0023", Serial: sharedDiskSerial, Type: "sas", Health: "PASSED",
			}}},
		}
	}
	snapshot := models.StateSnapshot{Hosts: []models.Host{host("c1", "c"), host("c2", "c"), host("a1", "a"), host("a2", "a")}}

	unsplit := NewRegistry(nil)
	unsplit.IngestSnapshot(snapshot)
	machineADisk := unsplit.sourceResourceID(SourceAgent, HostSMARTDiskSourceID(snapshot.Hosts[2], snapshot.Hosts[2].Sensors.SMART[0]))
	if machineADisk == "" {
		t.Fatal("machine a's disk has no agent source mapping")
	}
	store := NewMemoryStore()
	agentCandidate := SourceSpecificID(ResourceTypePhysicalDisk, SourceAgent, sharedDiskSerial)
	for _, splitFrom := range []string{MachineIdentityCanonicalID(ResourceTypePhysicalDisk, sharedDiskSerial), machineADisk} {
		if err := store.AddExclusion(ResourceExclusion{ResourceA: splitFrom, ResourceB: agentCandidate}); err != nil {
			t.Fatal(err)
		}
	}

	split := NewRegistry(store)
	split.IngestSnapshot(snapshot)
	diskIDs := make(map[string]bool, len(snapshot.Hosts))
	for _, h := range snapshot.Hosts {
		hostID := split.sourceResourceID(SourceAgent, h.ID)
		diskID := split.sourceResourceID(SourceAgent, HostSMARTDiskSourceID(h, h.Sensors.SMART[0]))
		disk, ok := split.Get(diskID)
		if hostID == "" || !ok {
			t.Fatalf("%s: host %q disk %q, want both mapped", h.ID, hostID, diskID)
		}
		if disk.ParentID == nil || *disk.ParentID != hostID {
			t.Fatalf("%s's disk %s sits under %v, want its own machine %s", h.ID, diskID, disk.ParentID, hostID)
		}
		diskIDs[diskID] = true
	}
	if !diskIDs[agentCandidate] {
		t.Fatalf("disk IDs = %v, want the first split copy on its source-specific ID %s", diskIDs, agentCandidate)
	}
}

// The metrics reader scopes a controller member's fallback key to the member
// once, as the writers do. A source ID that already names the member is the
// writer's key and stays unchanged. A Proxmox source ID from before members
// were scoped, and the canonical resource ID a view falls back to, get the
// member topology appended. A disk that is no controller member keeps its
// fallback.
func TestPhysicalDiskMetaMetricIDScopesAControllerMemberOnce(t *testing.T) {
	agent := models.Host{ID: "host-pve"}
	agentMember := models.HostDiskSMART{Device: "sdd", Controller: "ctrl0", Target: "megaraid,3"}
	labelledMember := models.HostDiskSMART{Device: "sdc [megaraid,1]", Controller: "sdc", Target: "megaraid,1"}
	pveMember := models.PhysicalDisk{
		ID:       ProxmoxPhysicalDiskSourceID("pve", "node1", "/dev/sdx", "", "megaraid,0"),
		Instance: "pve", Node: "node1", DevPath: "/dev/sdx", Target: "megaraid,0",
	}
	legacyPVEMember := pveMember
	legacyPVEMember.ID = "pve-node1--dev-sdx"
	tests := []struct {
		name     string
		meta     PhysicalDiskMeta
		fallback string
		want     string
	}{
		{
			name:     "agent member source ID",
			meta:     PhysicalDiskMeta{DevPath: "sdd", Controller: "ctrl0", Target: "megaraid,3"},
			fallback: HostSMARTDiskSourceID(agent, agentMember),
			want:     HostSMARTDiskMetricID(agent, agentMember),
		},
		{
			name:     "agent member reported under its smartctl label, merged with its Proxmox path",
			meta:     PhysicalDiskMeta{DevPath: "/dev/sdc", Controller: "sdc", Target: "megaraid,1"},
			fallback: HostSMARTDiskSourceID(agent, labelledMember),
			want:     HostSMARTDiskMetricID(agent, labelledMember),
		},
		{
			name:     "proxmox member source ID",
			meta:     PhysicalDiskMeta{DevPath: "/dev/sdx", Target: "megaraid,0"},
			fallback: pveMember.ID,
			want:     PhysicalDiskMetricID(pveMember),
		},
		{
			name:     "proxmox member source ID from before members were scoped",
			meta:     PhysicalDiskMeta{DevPath: "/dev/sdx", Target: "megaraid,0"},
			fallback: legacyPVEMember.ID,
			want:     PhysicalDiskMetricID(legacyPVEMember),
		},
		{
			name:     "canonical resource ID",
			meta:     PhysicalDiskMeta{DevPath: "/dev/sdx", Target: "megaraid,0"},
			fallback: "physical_disk-0123456789abcdef",
			want:     "physical_disk-0123456789abcdef:sdx@/megaraid,0",
		},
		{
			name:     "no controller member",
			meta:     PhysicalDiskMeta{DevPath: "/dev/sda"},
			fallback: "truenas-system:disk:sda",
			want:     "truenas-system:disk:sda",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PhysicalDiskMetaMetricID(&tt.meta, tt.fallback); got != tt.want {
				t.Fatalf("PhysicalDiskMetaMetricID(%q) = %q, want %q", tt.fallback, got, tt.want)
			}
		})
	}
	for name, key := range map[string]string{
		"agent member":          HostSMARTDiskMetricID(agent, agentMember),
		"proxmox member":        PhysicalDiskMetricID(pveMember),
		"legacy proxmox member": PhysicalDiskMetricID(legacyPVEMember),
		"labelled agent member": HostSMARTDiskMetricID(agent, labelledMember),
	} {
		if strings.Count(key, "@") != 1 {
			t.Fatalf("%s writer key %q, want the member topology once", name, key)
		}
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

func TestAgentMemoryMetaSerializesReclaimableCache(t *testing.T) {
	meta := AgentMemoryMeta{Total: 16, Used: 6, Free: 4, Cache: 6}
	payload, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal AgentMemoryMeta: %v", err)
	}
	if !strings.Contains(string(payload), `"cache":6`) {
		t.Fatalf("expected cache in agent memory payload, got %s", payload)
	}

	empty, err := json.Marshal(AgentMemoryMeta{Total: 16, Used: 6, Free: 10})
	if err != nil {
		t.Fatalf("marshal AgentMemoryMeta without cache: %v", err)
	}
	if strings.Contains(string(empty), "cache") {
		t.Fatalf("cache should be omitted when unreported, got %s", empty)
	}
}

// WearoutUnreported is the one sentinel every layer agrees on. models.PhysicalDisk
// and PhysicalDiskMeta both document wearout as "0-100, -1 unavailable", so the
// constant must stay pinned to -1 rather than drifting onto a real percentage.
func TestWearoutUnreportedSentinelIsNegativeOne(t *testing.T) {
	if WearoutUnreported != -1 {
		t.Fatalf("WearoutUnreported = %d, want -1", WearoutUnreported)
	}
	if WearoutUnreported >= 0 {
		t.Fatal("the unreported sentinel must not fall inside the real 0-100 reporting range")
	}
}

func TestUnraidDiskCountJSONAndIdentity(t *testing.T) {
	host := models.Host{ID: "pool-only", Hostname: "pool-only", Unraid: &models.HostUnraidStorage{}}
	_, unknownIdentity := resourceFromHostUnraidStorage(host)
	zero := 0
	host.Unraid.NumDisks = &zero
	resource, zeroIdentity := resourceFromHostUnraidStorage(host)
	if unknownIdentity.MachineID != zeroIdentity.MachineID {
		t.Fatal("disk count changed storage identity")
	}
	data, err := json.Marshal(resource.Storage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"numDisks":0`) {
		t.Fatalf("explicit zero lost: %s", data)
	}
	var restored StorageMeta
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.NumDisks == nil || *restored.NumDisks != 0 {
		t.Fatal("zero lost in round trip")
	}
	host.Unraid.NumDisks = nil
	resource, _ = resourceFromHostUnraidStorage(host)
	data, err = json.Marshal(resource.Storage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"numDisks"`) {
		t.Fatalf("unknown count became known: %s", data)
	}
}

func TestResourceIncidentNativeSeverityJSONContract(t *testing.T) {
	for _, native := range []string{"", "INFO", "NOTICE"} {
		incident := ResourceIncident{Provider: "truenas", NativeID: "condition-1", Code: "provider_condition", NativeSeverity: native}
		data, err := json.Marshal(incident)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "\"nativeSeverity\"") != (native != "") {
			t.Fatalf("optional native severity encoding: %s", data)
		}
		var decoded ResourceIncident
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded != incident {
			t.Fatalf("incident evidence or identity changed: %+v", decoded)
		}
	}
}
