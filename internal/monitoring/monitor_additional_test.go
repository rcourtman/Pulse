package monitoring

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type fakeDockerChecker struct{}

func (f *fakeDockerChecker) CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error) {
	return false, nil
}

func TestMonitorGetConfig(t *testing.T) {
	cfg := &config.Config{DataPath: "/tmp/pulse-test"}
	monitor := &Monitor{config: cfg}

	if got := monitor.GetConfig(); got != cfg {
		t.Fatalf("GetConfig = %v, want %v", got, cfg)
	}
}

func TestMonitorSetGetDockerChecker(t *testing.T) {
	monitor := &Monitor{}
	checker := &fakeDockerChecker{}

	monitor.SetDockerChecker(checker)
	if got := monitor.GetDockerChecker(); got != checker {
		t.Fatalf("GetDockerChecker = %v, want %v", got, checker)
	}

	monitor.SetDockerChecker(nil)
	if got := monitor.GetDockerChecker(); got != nil {
		t.Fatalf("GetDockerChecker = %v, want nil", got)
	}
}

func TestMonitorGetDockerHosts(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}
	monitor.state.UpsertDockerHost(models.DockerHost{ID: "host-1", Hostname: "host-1"})

	hosts := monitor.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("GetDockerHosts length = %d, want 1", len(hosts))
	}
	if hosts[0].ID != "host-1" {
		t.Fatalf("GetDockerHosts[0].ID = %q, want %q", hosts[0].ID, "host-1")
	}
}

func TestMonitorGetDockerHostsNilReceiver(t *testing.T) {
	var monitor *Monitor
	if got := monitor.GetDockerHosts(); got != nil {
		t.Fatalf("GetDockerHosts = %v, want nil", got)
	}
}

func TestMonitorLinkHostAgent(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}

	if err := monitor.LinkHostAgent("", "node-1"); err == nil {
		t.Fatalf("expected error on empty host ID")
	}
	if err := monitor.LinkHostAgent("host-1", ""); err == nil {
		t.Fatalf("expected error on empty node ID")
	}

	monitor.state.UpsertHost(models.Host{ID: "host-1", Hostname: "host-1"})
	monitor.state.UpdateNodes([]models.Node{{ID: "node-1", Name: "node-1"}})

	if err := monitor.LinkHostAgent("host-1", "node-1"); err != nil {
		t.Fatalf("LinkHostAgent error: %v", err)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 || hosts[0].LinkedNodeID != "node-1" {
		t.Fatalf("LinkedNodeID = %q, want %q", hosts[0].LinkedNodeID, "node-1")
	}
	if len(monitor.state.Nodes) != 1 || monitor.state.Nodes[0].LinkedHostAgentID != "host-1" {
		t.Fatalf("LinkedHostAgentID = %q, want %q", monitor.state.Nodes[0].LinkedHostAgentID, "host-1")
	}
}

func TestMonitorInvalidateAgentProfileCache(t *testing.T) {
	monitor := &Monitor{
		agentProfileCache: &agentProfileCacheEntry{
			profiles: []models.AgentProfile{{ID: "profile-1"}},
			loadedAt: time.Now(),
		},
	}

	monitor.InvalidateAgentProfileCache()
	if monitor.agentProfileCache != nil {
		t.Fatalf("expected cache to be cleared")
	}
}

func TestMonitorMarkDockerHostPendingUninstall(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}

	if _, err := monitor.MarkDockerHostPendingUninstall(""); err == nil {
		t.Fatalf("expected error on empty host ID")
	}
	if _, err := monitor.MarkDockerHostPendingUninstall("missing"); err == nil {
		t.Fatalf("expected error on missing host")
	}

	monitor.state.UpsertDockerHost(models.DockerHost{ID: "host-1", Hostname: "host-1"})
	host, err := monitor.MarkDockerHostPendingUninstall("host-1")
	if err != nil {
		t.Fatalf("MarkDockerHostPendingUninstall error: %v", err)
	}
	if !host.PendingUninstall {
		t.Fatalf("expected PendingUninstall to be true")
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 || !hosts[0].PendingUninstall {
		t.Fatalf("state PendingUninstall = %v, want true", hosts[0].PendingUninstall)
	}
}

func TestEnsureClusterEndpointURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"https://node.example:8006", "https://node.example:8006"},
		{"node.example", "https://node.example:8006"},
		{"node.example:9006", "https://node.example:9006"},
		{"  node.example  ", "https://node.example:8006"},
	}

	for _, tt := range tests {
		if got := ensureClusterEndpointURL(tt.input); got != tt.expected {
			t.Fatalf("ensureClusterEndpointURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestClusterEndpointEffectiveURL(t *testing.T) {
	endpoint := config.ClusterEndpoint{
		Host: "node.local",
		IP:   "10.0.0.1",
	}

	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://node.local:8006" {
		t.Fatalf("verifySSL host preference = %q, want %q", got, "https://node.local:8006")
	}

	endpoint.Host = ""
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://10.0.0.1:8006" {
		t.Fatalf("verifySSL fallback to IP = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.Host = "node.local"
	if got := clusterEndpointEffectiveURL(endpoint, false, ""); got != "https://10.0.0.1:8006" {
		t.Fatalf("non-SSL IP preference = %q, want %q", got, "https://10.0.0.1:8006")
	}

	endpoint.IPOverride = "192.168.1.10"
	if got := clusterEndpointEffectiveURL(endpoint, false, ""); got != "https://192.168.1.10:8006" {
		t.Fatalf("override IP preference = %q, want %q", got, "https://192.168.1.10:8006")
	}

	endpoint.Fingerprint = "ep-fingerprint"
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "https://192.168.1.10:8006" {
		t.Fatalf("per-endpoint fingerprint should allow IP override, got %q", got)
	}

	endpoint.Fingerprint = ""
	if got := clusterEndpointEffectiveURL(endpoint, true, "cluster-base-fingerprint"); got != "https://node.local:8006" {
		t.Fatalf("base fingerprint must not force IP routing for other cluster nodes, got %q", got)
	}

	endpoint = config.ClusterEndpoint{}
	if got := clusterEndpointEffectiveURL(endpoint, true, ""); got != "" {
		t.Fatalf("empty endpoint = %q, want empty", got)
	}
}

func TestBuildClusterClientEndpoints_PrefersOverrideWhenEndpointFingerprintPresent(t *testing.T) {
	pve := config.PVEInstance{
		Name:        "cluster-a",
		Host:        "https://cluster-a.local:8006",
		VerifySSL:   true,
		IsCluster:   true,
		ClusterName: "cluster-a",
		ClusterEndpoints: []config.ClusterEndpoint{
			{
				NodeName:    "node1",
				Host:        "https://node1.local:8006",
				IP:          "10.15.5.11",
				IPOverride:  "10.15.2.11",
				Fingerprint: "node1-fp",
			},
		},
	}

	endpoints, fingerprints := buildClusterClientEndpoints(pve, config.DiscoveryConfig{})

	if len(endpoints) != 2 {
		t.Fatalf("expected endpoint plus main host fallback, got %d", len(endpoints))
	}
	if endpoints[0] != "https://10.15.2.11:8006" {
		t.Fatalf("expected endpoint override URL first, got %q", endpoints[0])
	}
	if fingerprints["https://10.15.2.11:8006"] != "node1-fp" {
		t.Fatalf("expected fingerprint to follow effective endpoint URL, got %q", fingerprints["https://10.15.2.11:8006"])
	}
}

func TestProxmoxDiskMatchesExclude(t *testing.T) {
	tests := []struct {
		name     string
		disk     proxmox.Disk
		patterns []string
		want     bool
	}{
		{
			name:     "matches devpath directly",
			disk:     proxmox.Disk{DevPath: "/dev/sda"},
			patterns: []string{"/dev/sda"},
			want:     true,
		},
		{
			name:     "matches basename from devpath",
			disk:     proxmox.Disk{DevPath: "/dev/sda"},
			patterns: []string{"sda"},
			want:     true,
		},
		{
			name:     "does not match different device",
			disk:     proxmox.Disk{DevPath: "/dev/sdb"},
			patterns: []string{"sda"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := proxmoxDiskMatchesExclude(tt.disk, tt.patterns); got != tt.want {
				t.Fatalf("proxmoxDiskMatchesExclude(%+v, %v) = %t, want %t", tt.disk, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestBuildPhysicalDisksForNodeOmitsExcludedDisksAndClearsAlerts(t *testing.T) {
	monitor := &Monitor{alertManager: alerts.NewManager()}
	t.Cleanup(func() {
		monitor.alertManager.Stop()
	})

	failingDisk := proxmox.Disk{
		DevPath: "/dev/sda",
		Model:   "Samsung SSD",
		Health:  "FAILED",
		Wearout: 1,
	}
	healthyDisk := proxmox.Disk{
		DevPath: "/dev/sdb",
		Model:   "WD Blue",
		Health:  "PASSED",
		Wearout: 100,
	}

	monitor.alertManager.CheckDiskHealth("inst", "node1", failingDisk)
	if got := len(monitor.alertManager.GetActiveAlerts()); got == 0 {
		t.Fatalf("expected a failing disk alert before exclusion handling")
	}

	disks := monitor.buildPhysicalDisksForNode(
		"inst",
		"node1",
		[]proxmox.Disk{failingDisk, healthyDisk},
		[]string{"sda"},
		time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	)

	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk after exclusion, got %d", len(disks))
	}
	if disks[0].DevPath != "/dev/sdb" {
		t.Fatalf("expected only /dev/sdb to remain, got %q", disks[0].DevPath)
	}

	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		if alert.ID == "disk-health-inst-node1-/dev/sda" || alert.ID == "disk-wearout-inst-node1-/dev/sda" {
			t.Fatalf("expected excluded disk alerts to be cleared, still found %+v", alert)
		}
	}
}

func TestBuildPhysicalDisksForNodeUsesStableIdentityBeforeDevPath(t *testing.T) {
	monitor := &Monitor{}
	collectedAt := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)

	first := monitor.buildPhysicalDisksForNode(
		"inst",
		"node1",
		[]proxmox.Disk{{
			DevPath: "/dev/sda",
			Model:   "Samsung SSD",
			Serial:  "S3XYZ123",
			WWN:     "0x5002538f12345678",
			Type:    "sata",
			Health:  "PASSED",
			Wearout: 95,
		}},
		nil,
		collectedAt,
	)
	second := monitor.buildPhysicalDisksForNode(
		"inst",
		"node1",
		[]proxmox.Disk{{
			DevPath: "/dev/sdb",
			Model:   "Samsung SSD",
			Serial:  "S3XYZ123",
			WWN:     "0x5002538f12345678",
			Type:    "sata",
			Health:  "PASSED",
			Wearout: 95,
		}},
		nil,
		collectedAt.Add(time.Minute),
	)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected one disk from each poll, got %d and %d", len(first), len(second))
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("expected stable ID across devpath change, got %q then %q", first[0].ID, second[0].ID)
	}
	if first[0].ID != "inst-node1-wwn-0x5002538f12345678" {
		t.Fatalf("expected WWN-backed disk ID, got %q", first[0].ID)
	}
}

func TestPhysicalDiskIDFallsBackWhenControllerIdentityIsPlaceholder(t *testing.T) {
	got := physicalDiskID("inst", "node1", proxmox.Disk{
		DevPath: "/dev/sdc",
		Serial:  "N/A",
		WWN:     "unknown",
	})
	if got != "inst-node1-dev-sdc" {
		t.Fatalf("expected devpath fallback for placeholder serial/wwn, got %q", got)
	}
}

func TestPhysicalDiskIDCanonicalizesWWNRepresentation(t *testing.T) {
	plain := physicalDiskID("inst", "node1", proxmox.Disk{
		DevPath: "/dev/sda",
		WWN:     "5002538f12345678",
	})
	prefixed := physicalDiskID("inst", "node1", proxmox.Disk{
		DevPath: "/dev/sdb",
		WWN:     "wwn-0x5002538f12345678",
	})

	if plain != "inst-node1-wwn-0x5002538f12345678" {
		t.Fatalf("expected canonical WWN-backed ID, got %q", plain)
	}
	if prefixed != plain {
		t.Fatalf("expected equivalent WWN forms to produce the same ID, got %q and %q", plain, prefixed)
	}
}

func TestPhysicalDiskWWNMatchesCanonicalFormsAndSkipsPlaceholders(t *testing.T) {
	if !physicalDiskWWNMatches("0x5002538f12345678", "wwn-0x5002538f12345678") {
		t.Fatalf("expected equivalent WWN forms to match")
	}
	if !physicalDiskWWNMatches("00005002538f12345678", "5002538f12345678") {
		t.Fatalf("expected leading-zero WWN forms to match")
	}
	if !physicalDiskWWNMatches("eui.0025388211b67f9f", "0x0025388211b67f9f") {
		t.Fatalf("expected NVMe EUI64 and hex WWN forms to match")
	}
	if physicalDiskWWNMatches("unknown", "unknown") {
		t.Fatalf("placeholder WWNs must not match")
	}
}

func TestPhysicalDiskMetricResourceIDSkipsPlaceholderIdentity(t *testing.T) {
	got := physicalDiskMetricResourceID(models.PhysicalDisk{
		ID:       "inst-node1-dev-sdc",
		Instance: "inst",
		Node:     "node1",
		DevPath:  "/dev/sdc",
		Serial:   "N/A",
		WWN:      "unknown",
	})
	if got != "inst-node1-dev-sdc" {
		t.Fatalf("expected metric resource ID to fall back to disk ID, got %q", got)
	}
}

func TestPhysicalDiskMetricResourceIDSkipsPlaceholderDiskID(t *testing.T) {
	got := physicalDiskMetricResourceID(models.PhysicalDisk{
		ID:     "unknown",
		Serial: "S3XYZ123",
		WWN:    "0x5002538f12345678",
	})
	if got != "S3XYZ123" {
		t.Fatalf("expected placeholder disk ID to fall back to serial, got %q", got)
	}
}

func TestPhysicalDiskMetricResourceIDPrefersCanonicalDiskID(t *testing.T) {
	got := physicalDiskMetricResourceID(models.PhysicalDisk{
		ID:     "inst-node1-wwn-0x5002538f12345678",
		Serial: "S3XYZ123",
		WWN:    "0x5002538f12345678",
	})
	if got != "inst-node1-wwn-0x5002538f12345678" {
		t.Fatalf("expected canonical disk ID for metrics, got %q", got)
	}
}

func TestMergeNVMeTempsIntoDisksSkipsPlaceholderIdentityMatches(t *testing.T) {
	disks := []models.PhysicalDisk{{
		Node:    "node1",
		DevPath: "/dev/sda",
		Serial:  "N/A",
		WWN:     "unknown",
	}}
	nodes := []models.Node{{
		Name: "node1",
		Temperature: &models.Temperature{
			Available: true,
			SMART: []models.DiskTemp{{
				Device:      "/dev/sdb",
				Serial:      "n/a",
				WWN:         "UNKNOWN",
				Temperature: 44,
			}},
		},
	}}

	merged := mergeNVMeTempsIntoDisks(disks, nodes)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged disk, got %#v", merged)
	}
	if merged[0].Temperature != 0 {
		t.Fatalf("placeholder serial/WWN must not merge SMART temperature from another device, got %#v", merged[0])
	}
}

func TestMergeHostAgentSMARTIntoDisksSkipsPlaceholderIdentityMatches(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:       "inst-node1-dev-sda",
		Node:     "node1",
		Instance: "inst",
		DevPath:  "/dev/sda",
		Serial:   "N/A",
		WWN:      "unknown",
		Health:   "UNKNOWN",
		Wearout:  -1,
	}}
	nodes := []models.Node{{
		Name:              "node1",
		Instance:          "inst",
		LinkedHostAgentID: "host-1",
	}}
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:      "/dev/sdb",
				Serial:      "n/a",
				WWN:         "UNKNOWN",
				Temperature: 44,
				Health:      "FAILED",
			}},
		},
	}}

	merged := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged disk, got %#v", merged)
	}
	if merged[0].Temperature != 0 || merged[0].Health != "UNKNOWN" {
		t.Fatalf("placeholder serial/WWN must not merge host SMART evidence from another device, got %#v", merged[0])
	}
}

func TestMergeHostAgentSMARTIntoDisksBackfillsIdentityFromDeviceMatch(t *testing.T) {
	disks := []models.PhysicalDisk{{
		ID:       "inst-node1-dev-sda",
		Node:     "node1",
		Instance: "inst",
		DevPath:  "/dev/sda",
		Serial:   "N/A",
		WWN:      "unknown",
		Health:   "UNKNOWN",
		Wearout:  -1,
	}}
	nodes := []models.Node{{
		Name:              "node1",
		Instance:          "inst",
		LinkedHostAgentID: "host-1",
	}}
	used := 12
	hosts := []models.Host{{
		ID: "host-1",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{{
				Device:      "/dev/sda",
				Model:       "Samsung SSD",
				Serial:      "S3XYZ123",
				WWN:         "0x5002538f12345678",
				Type:        "sata",
				Temperature: 39,
				Health:      "PASSED",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: &used,
				},
			}},
		},
	}}

	merged := mergeHostAgentSMARTIntoDisks(disks, nodes, hosts)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged disk, got %#v", merged)
	}
	disk := merged[0]
	if disk.ID != "inst-node1-wwn-0x5002538f12345678" {
		t.Fatalf("expected SMART-backed stable disk ID, got %q", disk.ID)
	}
	if disk.Serial != "S3XYZ123" || disk.WWN != "0x5002538f12345678" {
		t.Fatalf("expected SMART identity to backfill missing Proxmox identity, got %#v", disk)
	}
	if disk.Model != "Samsung SSD" || disk.Type != "sata" {
		t.Fatalf("expected SMART model/type to fill missing disk fields, got %#v", disk)
	}
	if disk.Temperature != 39 || disk.Health != "PASSED" || disk.Wearout != 88 {
		t.Fatalf("expected SMART health evidence to merge, got %#v", disk)
	}
}

func TestBuildClusterClientEndpoints_FallsBackToMainHostWhenOnlyBaseFingerprintExists(t *testing.T) {
	pve := config.PVEInstance{
		Name:        "cluster-a",
		Host:        "https://cluster-a.example.com:8006",
		Fingerprint: "cluster-base-fp",
		VerifySSL:   true,
		IsCluster:   true,
		ClusterName: "cluster-a",
		ClusterEndpoints: []config.ClusterEndpoint{
			{
				NodeName: "node1",
				Host:     "node1",
				IP:       "10.15.5.11",
			},
			{
				NodeName: "node2",
				Host:     "node2",
				IP:       "10.15.5.12",
			},
		},
	}

	endpoints, fingerprints := buildClusterClientEndpoints(pve, config.DiscoveryConfig{})

	if len(endpoints) != 1 {
		t.Fatalf("expected only the main host fallback endpoint, got %d", len(endpoints))
	}
	if endpoints[0] != "https://cluster-a.example.com:8006" {
		t.Fatalf("expected main host fallback, got %q", endpoints[0])
	}
	if len(fingerprints) != 0 {
		t.Fatalf("expected no per-endpoint fingerprints, got %v", fingerprints)
	}
}

func TestBuildClusterClientEndpoints_SkipsEndpointsOutsideDiscoverySubnetPolicy(t *testing.T) {
	pve := config.PVEInstance{
		Name:        "cluster-a",
		Host:        "https://10.15.2.10:8006",
		VerifySSL:   false,
		IsCluster:   true,
		ClusterName: "cluster-a",
		ClusterEndpoints: []config.ClusterEndpoint{
			{
				NodeName: "node1",
				IP:       "10.15.5.11",
			},
			{
				NodeName: "node2",
				IP:       "10.15.2.12",
			},
		},
	}

	discoveryCfg := config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.15.2.0/24"},
		SubnetBlocklist: []string{"10.15.5.0/24"},
	}

	endpoints, fingerprints := buildClusterClientEndpoints(pve, discoveryCfg)

	if len(endpoints) != 2 {
		t.Fatalf("expected allowed endpoint plus main host fallback, got %d", len(endpoints))
	}
	if endpoints[0] != "https://10.15.2.12:8006" {
		t.Fatalf("expected allowed management endpoint first, got %q", endpoints[0])
	}
	if endpoints[1] != "https://10.15.2.10:8006" {
		t.Fatalf("expected main host fallback, got %q", endpoints[1])
	}
	if len(fingerprints) != 0 {
		t.Fatalf("expected no fingerprints, got %v", fingerprints)
	}
}

func TestBuildClusterClientEndpoints_SkipsHostnameResolvingOutsideDiscoverySubnetPolicy(t *testing.T) {
	originalLookupIP := lookupIPFunc
	lookupIPFunc = func(host string) ([]net.IP, error) {
		switch host {
		case "node1.local":
			return []net.IP{net.ParseIP("10.15.5.11")}, nil
		case "node2.local":
			return []net.IP{net.ParseIP("10.15.2.12")}, nil
		default:
			return nil, fmt.Errorf("unexpected host %q", host)
		}
	}
	defer func() {
		lookupIPFunc = originalLookupIP
	}()

	pve := config.PVEInstance{
		Name:        "cluster-a",
		Host:        "https://10.15.2.10:8006",
		Fingerprint: "cluster-base-fp",
		VerifySSL:   true,
		IsCluster:   true,
		ClusterName: "cluster-a",
		ClusterEndpoints: []config.ClusterEndpoint{
			{
				NodeName: "node1",
				Host:     "https://node1.local:8006",
				IP:       "10.15.5.11",
			},
			{
				NodeName: "node2",
				Host:     "https://node2.local:8006",
				IP:       "10.15.2.12",
			},
		},
	}

	discoveryCfg := config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.15.2.0/24"},
		SubnetBlocklist: []string{"10.15.5.0/24"},
	}

	endpoints, _ := buildClusterClientEndpoints(pve, discoveryCfg)

	if len(endpoints) != 2 {
		t.Fatalf("expected allowed hostname endpoint plus main host fallback, got %d", len(endpoints))
	}
	if endpoints[0] != "https://node2.local:8006" {
		t.Fatalf("expected allowed hostname endpoint first, got %q", endpoints[0])
	}
	if endpoints[1] != "https://10.15.2.10:8006" {
		t.Fatalf("expected main host fallback, got %q", endpoints[1])
	}
}
