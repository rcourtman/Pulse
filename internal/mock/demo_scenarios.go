package mock

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

type demoWorkloadProfile struct {
	Name            string
	Tags            []string
	BackupAge       time.Duration
	ProtectionStory demoProtectionStory
	ForceState      string
}

type demoProtectionStory string

const (
	demoProtectionProtected       demoProtectionStory = "protected"
	demoProtectionAttentionStale  demoProtectionStory = "attention-stale"
	demoProtectionAttentionFailed demoProtectionStory = "attention-failed"
	demoProtectionSnapshotOnly    demoProtectionStory = "snapshot-only"
	demoProtectionTaskOnly        demoProtectionStory = "task-only"
	demoProtectionUnknown         demoProtectionStory = "unknown"
)

type demoBackupGuest struct {
	Profile  demoWorkloadProfile
	Node     string
	Instance string
	Type     string
	PBSKind  string
	VMID     int
	DiskSize int64
}

type demoProxmoxClusterProfile struct {
	SourceInstance    string
	Name              string
	NodeDisplayPrefix string
	StoragePrefix     string
	SharedStorageName string
}

type demoDockerHostProfile struct {
	Hostname    string
	DisplayName string
	Containers  []demoDockerContainerProfile
}

type demoDockerContainerProfile struct {
	Name        string
	Image       string
	Tags        []string
	Health      string
	Description string
}

type demoKubernetesNodeProfile struct {
	Name  string
	Roles []string
}

type demoKubernetesDeploymentProfile struct {
	Name            string
	Namespace       string
	DesiredReplicas int32
	ReadyReplicas   int32
}

type demoKubernetesPodProfile struct {
	Name        string
	Namespace   string
	NodeIndex   int
	OwnerKind   string
	OwnerName   string
	Phase       string
	Reason      string
	Message     string
	Container   string
	Image       string
	ContainerOK bool
	Restarts    int32
}

type demoKubernetesClusterProfile struct {
	Name            string
	DisplayName     string
	Context         string
	ServerSlug      string
	Version         string
	NodeProfiles    []demoKubernetesNodeProfile
	OfflineNodeName string
	// Scenario selects which degraded story this cluster carries.
	// "" = all-healthy. Each value flips a specific pod profile's
	// container reason inside applyDemoKubernetesScenario without
	// polluting the global pod profile arrays.
	Scenario demoKubernetesScenario
}

type demoKubernetesScenario string

const (
	demoK8sScenarioHealthy          demoKubernetesScenario = ""
	demoK8sScenarioNotReadyWorker   demoKubernetesScenario = "not-ready-worker"
	demoK8sScenarioCrashLoopBackOff demoKubernetesScenario = "crashloopbackoff"
	demoK8sScenarioImagePullBackOff demoKubernetesScenario = "imagepullbackoff"
)

var demoVMProfiles = []demoWorkloadProfile{
	{Name: "checkout-web-01", Tags: []string{"production", "web", "customer-facing"}, BackupAge: 6 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "checkout-web-02", Tags: []string{"production", "web", "customer-facing"}, BackupAge: 36 * time.Hour, ProtectionStory: demoProtectionAttentionFailed},
	{Name: "orders-api-01", Tags: []string{"production", "api", "critical"}, BackupAge: 3 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "orders-api-02", Tags: []string{"production", "api", "critical"}, BackupAge: 12 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "postgres-primary-01", Tags: []string{"production", "database", "critical"}, BackupAge: 90 * time.Minute, ProtectionStory: demoProtectionProtected},
	{Name: "postgres-replica-01", Tags: []string{"production", "database", "replica"}, BackupAge: 2 * time.Hour, ProtectionStory: demoProtectionProtected, ForceState: "stopped"},
	{Name: "redis-cache-01", Tags: []string{"production", "cache", "latency-sensitive"}, BackupAge: 12 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "observability-core-01", Tags: []string{"monitoring", "platform", "production"}, BackupAge: 10 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "ci-runner-01", Tags: []string{"ci", "platform", "shared-services"}, BackupAge: 10 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "analytics-batch-01", Tags: []string{"analytics", "batch", "internal"}, BackupAge: 14 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "erp-app-01", Tags: []string{"business", "api", "internal"}, BackupAge: 8 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "docs-portal-01", Tags: []string{"web", "internal", "docs"}, ProtectionStory: demoProtectionSnapshotOnly},
}

var demoContainerProfiles = []demoWorkloadProfile{
	{Name: "edge-proxy-01", Tags: []string{"production", "ingress", "edge"}, BackupAge: 6 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "auth-service-01", Tags: []string{"production", "api", "security"}, BackupAge: 4 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "billing-worker-01", Tags: []string{"production", "queue", "worker"}, BackupAge: 8 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "reporting-api-01", Tags: []string{"internal", "api", "reporting"}, BackupAge: 10 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "minio-gateway-01", Tags: []string{"storage", "internal", "gateway"}, BackupAge: 9 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "wireguard-edge-01", Tags: []string{"edge", "security", "vpn"}, BackupAge: 8 * time.Hour, ProtectionStory: demoProtectionProtected},
	{Name: "docs-wiki-01", Tags: []string{"internal", "web", "docs"}, ProtectionStory: demoProtectionSnapshotOnly},
	{Name: "smtp-relay-01", Tags: []string{"mail", "queue", "platform"}, ProtectionStory: demoProtectionTaskOnly},
	{Name: "grafana-agent-01", Tags: []string{"monitoring", "agent", "platform"}, BackupAge: 15 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "artifact-cache-01", Tags: []string{"ci", "cache", "platform"}, BackupAge: 21 * 24 * time.Hour, ProtectionStory: demoProtectionAttentionStale},
	{Name: "backup-orchestrator-01", Tags: []string{"backup", "platform", "worker"}, ProtectionStory: demoProtectionUnknown},
	{Name: "dev-portal-01", Tags: []string{"development", "web", "internal"}, ProtectionStory: demoProtectionTaskOnly, ForceState: "stopped"},
}

var demoDockerHostProfiles = []demoDockerHostProfile{
	{
		Hostname:    "edge-apps-01",
		DisplayName: "Edge Apps 01",
		Containers: []demoDockerContainerProfile{
			{Name: "customer-portal", Image: "ghcr.io/pulse-demo/customer-portal:2026.04", Tags: []string{"web", "customer-facing", "production"}, Health: "healthy"},
			{Name: "inventory-api", Image: "ghcr.io/pulse-demo/inventory-api:2026.04", Tags: []string{"api", "production", "platform"}, Health: "healthy"},
			{Name: "edge-proxy", Image: "traefik:v3.1", Tags: []string{"ingress", "edge", "production"}, Health: "healthy"},
			{Name: "vaultwarden", Image: "vaultwarden/server:1.32.7", Tags: []string{"security", "internal"}, Health: "healthy"},
			{Name: "uptime-kuma", Image: "louislam/uptime-kuma:1.23.16", Tags: []string{"monitoring", "internal"}, Health: "healthy"},
		},
	},
	{
		Hostname:    "ops-services-01",
		DisplayName: "Ops Services 01",
		Containers: []demoDockerContainerProfile{
			{Name: "postgres-archive", Image: "postgres:16.4", Tags: []string{"database", "archive", "internal"}, Health: "healthy"},
			{Name: "backup-coordinator", Image: "ghcr.io/pulse-demo/backup-coordinator:2026.04", Tags: []string{"backup", "platform", "worker"}, Health: "healthy"},
			{Name: "sftp-ingest", Image: "atmoz/sftp:latest", Tags: []string{"transfer", "integration", "internal"}, Health: "healthy"},
			{Name: "prometheus", Image: "prom/prometheus:v2.54.1", Tags: []string{"monitoring", "platform"}, Health: "healthy"},
			{Name: "grafana-agent", Image: "grafana/agent:v0.42.0", Tags: []string{"monitoring", "agent", "platform"}, Health: "healthy"},
		},
	},
	{
		Hostname:    "app-platform-01",
		DisplayName: "App Platform 01",
		Containers: []demoDockerContainerProfile{
			{Name: "orders-api", Image: "ghcr.io/pulse-demo/orders-api:2026.04", Tags: []string{"api", "production", "customer-facing"}, Health: "healthy"},
			{Name: "notifications-worker", Image: "ghcr.io/pulse-demo/notifications-worker:2026.04", Tags: []string{"queue", "worker", "production"}, Health: "healthy"},
			{Name: "redis-cache", Image: "redis:7.4.1", Tags: []string{"cache", "production", "latency-sensitive"}, Health: "healthy"},
			{Name: "nginx-edge", Image: "nginx:1.27.2", Tags: []string{"web", "ingress", "production"}, Health: "healthy"},
			{Name: "loki", Image: "grafana/loki:3.2.0", Tags: []string{"logging", "platform"}, Health: "healthy"},
		},
	},
	{
		Hostname:    "data-services-01",
		DisplayName: "Data Services 01",
		Containers: []demoDockerContainerProfile{
			{Name: "kafka-broker", Image: "confluentinc/cp-kafka:7.7.1", Tags: []string{"streaming", "platform", "internal"}, Health: "healthy"},
			{Name: "clickhouse", Image: "clickhouse/clickhouse-server:24.8", Tags: []string{"database", "analytics", "internal"}, Health: "healthy"},
			{Name: "etl-runner", Image: "ghcr.io/pulse-demo/etl-runner:2026.04", Tags: []string{"batch", "analytics", "worker"}, Health: "healthy"},
			{Name: "minio", Image: "minio/minio:RELEASE.2026-01-20T00-00-00Z", Tags: []string{"storage", "gateway", "internal"}, Health: "healthy"},
			{Name: "schema-registry", Image: "confluentinc/cp-schema-registry:7.7.1", Tags: []string{"streaming", "platform", "internal"}, Health: "healthy"},
		},
	},
}

// Per-cluster demo identities. Each cluster carries its own node naming
// scheme, kubelet version, and a single degraded scenario so the four
// clusters tell distinct stories rather than each rendering an identical
// CrashLoopBackOff payments-worker. Production EU stays at "real outage"
// (NotReady worker), Staging EU stays at "deploy that crashed before
// promotion" (CrashLoopBackOff), Development EU stays at "forgot to push
// the image" (ImagePullBackOff), Edge stays healthy.
var demoKubernetesClusterProfiles = []demoKubernetesClusterProfile{
	{
		Name:        "production-eu",
		DisplayName: "Production EU",
		Context:     "production-eu-context",
		ServerSlug:  "production-eu",
		Version:     "v1.30.4",
		NodeProfiles: []demoKubernetesNodeProfile{
			{Name: "prod-euw1-k8s-01", Roles: []string{"control-plane"}},
			{Name: "prod-euw1-k8s-02", Roles: []string{"worker"}},
			{Name: "prod-euw1-k8s-03", Roles: []string{"worker"}},
			{Name: "prod-euw1-k8s-04", Roles: []string{"worker"}},
			{Name: "prod-euw1-k8s-05", Roles: []string{"worker"}},
		},
		OfflineNodeName: "prod-euw1-k8s-03",
		Scenario:        demoK8sScenarioNotReadyWorker,
	},
	{
		Name:        "staging-eu",
		DisplayName: "Staging EU",
		Context:     "staging-eu-context",
		ServerSlug:  "staging-eu",
		Version:     "v1.31.2",
		NodeProfiles: []demoKubernetesNodeProfile{
			{Name: "stage-euw1-k8s-01", Roles: []string{"control-plane"}},
			{Name: "stage-euw1-k8s-02", Roles: []string{"worker"}},
			{Name: "stage-euw1-k8s-03", Roles: []string{"worker"}},
			{Name: "stage-euw1-k8s-04", Roles: []string{"worker"}},
			{Name: "stage-euw1-k8s-05", Roles: []string{"worker"}},
		},
		Scenario: demoK8sScenarioCrashLoopBackOff,
	},
	{
		Name:        "development-eu",
		DisplayName: "Development EU",
		Context:     "development-eu-context",
		ServerSlug:  "development-eu",
		Version:     "v1.32.0-rc.1",
		NodeProfiles: []demoKubernetesNodeProfile{
			{Name: "dev-euw1-01", Roles: []string{"control-plane"}},
			{Name: "dev-euw1-02", Roles: []string{"worker"}},
			{Name: "dev-euw1-03", Roles: []string{"worker"}},
			{Name: "dev-euw1-04", Roles: []string{"worker"}},
			{Name: "dev-euw1-05", Roles: []string{"worker"}},
		},
		Scenario: demoK8sScenarioImagePullBackOff,
	},
	{
		Name:        "edge",
		DisplayName: "Edge",
		Context:     "edge-context",
		ServerSlug:  "edge",
		Version:     "v1.29.6+k3s1",
		NodeProfiles: []demoKubernetesNodeProfile{
			{Name: "edge-pop-lax-01", Roles: []string{"control-plane"}},
			{Name: "edge-pop-nrt-01", Roles: []string{"worker"}},
			{Name: "edge-pop-fra-01", Roles: []string{"worker"}},
			{Name: "edge-pop-iad-01", Roles: []string{"worker"}},
			{Name: "edge-pop-sin-01", Roles: []string{"worker"}},
		},
		Scenario: demoK8sScenarioHealthy,
	},
}

var demoKubernetesDeploymentProfiles = []demoKubernetesDeploymentProfile{
	{Name: "checkout-web", Namespace: "apps", DesiredReplicas: 3, ReadyReplicas: 3},
	{Name: "checkout-api", Namespace: "services", DesiredReplicas: 3, ReadyReplicas: 3},
	{Name: "payments-worker", Namespace: "services", DesiredReplicas: 2, ReadyReplicas: 1},
	{Name: "platform-observability", Namespace: "monitoring", DesiredReplicas: 2, ReadyReplicas: 2},
}

var demoKubernetesPodProfiles = []demoKubernetesPodProfile{
	{Name: "checkout-web-7fd84d8f76-2xp6n", Namespace: "apps", NodeIndex: 1, OwnerKind: "Deployment", OwnerName: "checkout-web", Phase: "Running", Container: "checkout-web", Image: "ghcr.io/pulse-demo/checkout-web:2026.04", ContainerOK: true},
	{Name: "checkout-web-7fd84d8f76-kjpl2", Namespace: "apps", NodeIndex: 2, OwnerKind: "Deployment", OwnerName: "checkout-web", Phase: "Running", Container: "checkout-web", Image: "ghcr.io/pulse-demo/checkout-web:2026.04", ContainerOK: true},
	{Name: "checkout-api-6c746d5bcf-c7z2p", Namespace: "services", NodeIndex: 1, OwnerKind: "Deployment", OwnerName: "checkout-api", Phase: "Running", Container: "checkout-api", Image: "ghcr.io/pulse-demo/checkout-api:2026.04", ContainerOK: true},
	{Name: "checkout-api-6c746d5bcf-s48vh", Namespace: "services", NodeIndex: 2, OwnerKind: "Deployment", OwnerName: "checkout-api", Phase: "Running", Container: "checkout-api", Image: "ghcr.io/pulse-demo/checkout-api:2026.04", ContainerOK: true},
	{Name: "payments-worker-58484d44f7-f6gzh", Namespace: "services", NodeIndex: 2, OwnerKind: "Deployment", OwnerName: "payments-worker", Phase: "Running", Container: "payments-worker", Image: "ghcr.io/pulse-demo/payments-worker:2026.04", ContainerOK: false, Reason: "CrashLoopBackOff", Message: "Back-off restarting failed container", Restarts: 7},
	{Name: "payments-worker-58484d44f7-s9m5r", Namespace: "services", NodeIndex: 1, OwnerKind: "Deployment", OwnerName: "payments-worker", Phase: "Running", Container: "payments-worker", Image: "ghcr.io/pulse-demo/payments-worker:2026.04", ContainerOK: true},
	{Name: "platform-observability-0", Namespace: "monitoring", NodeIndex: 0, OwnerKind: "StatefulSet", OwnerName: "platform-observability", Phase: "Running", Container: "platform-observability", Image: "grafana/loki:3.1.0", ContainerOK: true},
	{Name: "platform-observability-1", Namespace: "monitoring", NodeIndex: 2, OwnerKind: "StatefulSet", OwnerName: "platform-observability", Phase: "Running", Container: "platform-observability", Image: "prom/prometheus:v2.54.1", ContainerOK: true},
	{Name: "ingress-nginx-controller-86d96647c9-jms2p", Namespace: "ingress-nginx", NodeIndex: 0, OwnerKind: "Deployment", OwnerName: "ingress-nginx-controller", Phase: "Running", Container: "controller", Image: "registry.k8s.io/ingress-nginx/controller:v1.11.2", ContainerOK: true},
	{Name: "cron-nightly-backfill-28918234", Namespace: "services", NodeIndex: 2, OwnerKind: "Job", OwnerName: "cron-nightly-backfill", Phase: "Pending", Reason: "PodInitializing", Message: "Waiting for scheduled execution window", Container: "cron-nightly-backfill", Image: "ghcr.io/pulse-demo/cron-nightly-backfill:2026.04", ContainerOK: false},
}

var demoProxmoxClusterProfiles = []demoProxmoxClusterProfile{
	{SourceInstance: "mock-cluster-1", Name: "Production West", NodeDisplayPrefix: "West Production", StoragePrefix: "west", SharedStorageName: "shared-backup-fabric"},
	{SourceInstance: "mock-cluster-2", Name: "Production East", NodeDisplayPrefix: "East Production", StoragePrefix: "east", SharedStorageName: "east-shared-fabric"},
	{SourceInstance: "mock-cluster-3", Name: "Core Services", NodeDisplayPrefix: "Core Services", StoragePrefix: "core", SharedStorageName: "core-shared-fabric"},
	{SourceInstance: "mock-cluster-4", Name: "Disaster Recovery", NodeDisplayPrefix: "Disaster Recovery", StoragePrefix: "dr", SharedStorageName: "dr-shared-fabric"},
	{SourceInstance: "mock-cluster-5", Name: "Edge Sites", NodeDisplayPrefix: "Edge Sites", StoragePrefix: "edge", SharedStorageName: "edge-shared-fabric"},
}

const demoOfflineProxmoxNode = "pve22"

func applyDemoScenarioGraph(graph *FixtureGraph, cfg MockConfig, now time.Time) {
	if graph == nil {
		return
	}

	applyDemoPlatformScenario(&graph.PlatformFixtures)
	applyDemoNodeScenario(&graph.State)
	vmProfiles := applyDemoWorkloadScenario(graph.State.VMs, demoVMProfiles, now)
	containerProfiles := applyDemoContainerScenario(graph.State.Containers, demoContainerProfiles, now)
	applyDemoDockerScenario(&graph.State, now)
	// Docker-in-LXC nested hosts bind to the guests' final (scenario-shaped)
	// names, instances, and states, so the fixture runs after the workload
	// and Docker scenarios and before connection health is synced.
	ensureMockDockerInLXCFixture(&graph.State, cfg)
	applyDemoKubernetesScenario(&graph.State, now)
	applyDemoHostScenario(&graph.State, now)
	applyDemoStorageScenario(&graph.State, now)
	applyDemoBackupScenario(&graph.State, vmProfiles, containerProfiles, now)
	// In-flight backups attach to the guests' final curated backup stories,
	// so the fixture runs after the backup scenario rebuilds that history
	// (which also sweeps away the previous tick's fixture artifacts).
	ensureMockBackupRunningFixture(&graph.State, now)
	syncDemoConnectionHealth(&graph.State)
}

func applyDemoPlatformScenario(fixtures *PlatformFixtures) {
	if fixtures == nil {
		return
	}

	fixtures.TrueNAS = applyDemoTrueNASPlatformScenario(fixtures.TrueNAS)
	fixtures.VMware = applyDemoVMwarePlatformScenario(fixtures.VMware)
}

func applyDemoTrueNASPlatformScenario(snapshot truenas.FixtureSnapshot) truenas.FixtureSnapshot {
	out := cloneTrueNASFixtureSnapshot(snapshot)
	nameByID := map[string]string{
		"nextcloud":     "client-files",
		"immich":        "photo-archive",
		"paperless-ngx": "document-hub",
		"grafana":       "ops-dashboards",
		"adguard-home":  "edge-dns",
	}
	notesByID := map[string]string{
		"nextcloud":     "Client file portal and secure sync",
		"immich":        "Shared photo archive and mobile uploads",
		"paperless-ngx": "Operations document inbox and OCR workflows",
		"grafana":       "Operations dashboards and service overviews",
		"adguard-home":  "Edge DNS and network filtering",
	}
	for i := range out.Apps {
		app := &out.Apps[i]
		if name := strings.TrimSpace(nameByID[strings.TrimSpace(app.ID)]); name != "" {
			app.Name = name
		}
		if note := strings.TrimSpace(notesByID[strings.TrimSpace(app.ID)]); note != "" {
			app.Notes = note
		}
		app.State = "RUNNING"
		for j := range app.Containers {
			app.Containers[j].State = "running"
		}
	}
	return out
}

func applyDemoVMwarePlatformScenario(snapshot vmware.InventorySnapshot) vmware.InventorySnapshot {
	out := cloneVMwareInventorySnapshot(snapshot)

	vmNameByID := map[string]string{
		"vm-201": "warehouse-api-01",
		"vm-202": "warehouse-db-01",
		"vm-203": "client-portal-01",
		"vm-204": "ops-observability-01",
		"vm-205": "finance-jump-01",
		"vm-206": "etl-batch-01",
	}
	vmIPByID := map[string][]string{
		"vm-201": {"10.42.10.121"},
		"vm-202": {"10.42.10.132"},
		"vm-203": {"10.42.10.144"},
		"vm-204": {"10.42.20.115"},
		"vm-205": {"10.42.30.118"},
		"vm-206": {"10.42.40.206"},
	}

	for i := range out.Hosts {
		host := &out.Hosts[i]
		if strings.TrimSpace(host.Host) == "host-102" {
			host.OverallStatus = "green"
			host.TriggeredAlarms = nil
			for j := range host.RecentTasks {
				host.RecentTasks[j].State = "success"
			}
		}
	}

	for i := range out.VMs {
		vm := &out.VMs[i]
		if name := strings.TrimSpace(vmNameByID[strings.TrimSpace(vm.VM)]); name != "" {
			vm.Name = name
			vm.GuestHostname = name + ".internal"
		}
		if ips := vmIPByID[strings.TrimSpace(vm.VM)]; len(ips) > 0 {
			vm.GuestIPAddresses = append([]string(nil), ips...)
		}
		vm.PowerState = "POWERED_ON"
		vm.OverallStatus = "green"
		vm.TriggeredAlarms = nil
		for j := range vm.RecentTasks {
			if strings.EqualFold(strings.TrimSpace(vm.RecentTasks[j].State), "queued") {
				vm.RecentTasks[j].State = "success"
			}
		}
		if vm.Metrics == nil {
			vm.Metrics = defaultDemoVMwareMetrics()
		}
	}

	for i := range out.Datastores {
		datastore := &out.Datastores[i]
		datastore.VMNames = make([]string, 0, len(datastore.VMIDs))
		for _, vmID := range datastore.VMIDs {
			if name := strings.TrimSpace(vmNameByID[strings.TrimSpace(vmID)]); name != "" {
				datastore.VMNames = append(datastore.VMNames, name)
			}
		}
	}

	return out
}

func applyDemoNodeScenario(state *models.StateSnapshot) {
	if state == nil {
		return
	}

	for i := range state.Nodes {
		node := &state.Nodes[i]
		sourceInstance := node.Instance
		node.Instance = scenarioClusterAlias(node.Instance)
		if node.IsClusterMember || strings.TrimSpace(node.ClusterName) != "" {
			node.ClusterName = scenarioClusterAlias(node.ClusterName)
		}
		if displayName := demoProxmoxNodeDisplayName(*node, sourceInstance); displayName != "" {
			node.DisplayName = displayName
		}
		if isDemoOfflineProxmoxNode(node.Name) {
			node.Status = "offline"
			node.CPU = 0
			node.Memory.Used = 0
			node.Memory.Usage = 0
			node.Memory.Free = node.Memory.Total
			node.Memory.SwapUsed = 0
			node.Disk.Used = 0
			node.Disk.Free = node.Disk.Total
			node.Disk.Usage = -1
			node.Uptime = 0
			node.LoadAverage = []float64{0.0, 0.0, 0.0}
			node.ConnectionHealth = "offline"
		}
	}

	hostsByID := make(map[string]*models.Host, len(state.Hosts))
	hostsByHostname := make(map[string]*models.Host, len(state.Hosts))
	for i := range state.Hosts {
		host := &state.Hosts[i]
		if id := strings.TrimSpace(host.ID); id != "" {
			hostsByID[id] = host
		}
		if name := strings.ToLower(strings.TrimSpace(host.Hostname)); name != "" {
			hostsByHostname[name] = host
		}
	}
	for _, node := range state.Nodes {
		if host := hostsByID[strings.TrimSpace(node.LinkedAgentID)]; host != nil {
			host.DisplayName = node.DisplayName
			continue
		}
		if host := hostsByHostname[strings.ToLower(strings.TrimSpace(node.Name))]; host != nil {
			host.DisplayName = node.DisplayName
		}
	}
}

func applyDemoWorkloadScenario(workloads []models.VM, profiles []demoWorkloadProfile, now time.Time) map[int]demoWorkloadProfile {
	guestProfiles := make(map[int]demoWorkloadProfile, len(workloads))
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].VMID == workloads[j].VMID {
			return workloads[i].ID < workloads[j].ID
		}
		return workloads[i].VMID < workloads[j].VMID
	})
	for i := range workloads {
		profile := namedWorkloadProfile(profiles, i)
		workloads[i].Name = profile.Name
		workloads[i].Instance = scenarioClusterAlias(workloads[i].Instance)
		workloads[i].Tags = mergeScenarioTags(workloads[i].Tags, profile.Tags)
		workloads[i].Status = normalizeWorkloadState(profile.ForceState, "running")
		if isDemoOfflineProxmoxNode(workloads[i].Node) {
			workloads[i].Status = "stopped"
		}
		if workloads[i].Status == "stopped" {
			workloads[i].CPU = 0
			workloads[i].Memory.Used = 0
			workloads[i].Memory.Usage = 0
			workloads[i].Memory.Free = workloads[i].Memory.Total
			workloads[i].Memory.SwapUsed = 0
			workloads[i].Disk.Used = 0
			workloads[i].Disk.Usage = -1
			workloads[i].Disk.Free = workloads[i].Disk.Total
			workloads[i].NetworkIn = 0
			workloads[i].NetworkOut = 0
			workloads[i].DiskRead = 0
			workloads[i].DiskWrite = 0
			workloads[i].Uptime = 0
			workloads[i].IPAddresses = nil
			workloads[i].NetworkInterfaces = nil
		}
		workloads[i].LastBackup = demoProfileLastBackup(profile, now)
		guestProfiles[workloads[i].VMID] = profile
	}
	return guestProfiles
}

func applyDemoContainerScenario(workloads []models.Container, profiles []demoWorkloadProfile, now time.Time) map[int]demoWorkloadProfile {
	guestProfiles := make(map[int]demoWorkloadProfile, len(workloads))
	sort.Slice(workloads, func(i, j int) bool {
		if workloads[i].VMID == workloads[j].VMID {
			return workloads[i].ID < workloads[j].ID
		}
		return workloads[i].VMID < workloads[j].VMID
	})
	for i := range workloads {
		profile := namedWorkloadProfile(profiles, i)
		workloads[i].Name = profile.Name
		workloads[i].Instance = scenarioClusterAlias(workloads[i].Instance)
		workloads[i].Tags = mergeScenarioTags(workloads[i].Tags, profile.Tags)
		workloads[i].Status = normalizeWorkloadState(profile.ForceState, "running")
		if isDemoOfflineProxmoxNode(workloads[i].Node) {
			workloads[i].Status = "stopped"
		}
		if workloads[i].Status == "stopped" {
			workloads[i].CPU = 0
			workloads[i].Memory.Used = 0
			workloads[i].Memory.Usage = 0
			workloads[i].Memory.Free = workloads[i].Memory.Total
			workloads[i].Memory.SwapUsed = 0
			workloads[i].Disk.Used = 0
			workloads[i].Disk.Usage = -1
			workloads[i].Disk.Free = workloads[i].Disk.Total
			workloads[i].NetworkIn = 0
			workloads[i].NetworkOut = 0
			workloads[i].DiskRead = 0
			workloads[i].DiskWrite = 0
			workloads[i].Uptime = 0
			workloads[i].IPAddresses = nil
			workloads[i].NetworkInterfaces = nil
		}
		workloads[i].LastBackup = demoProfileLastBackup(profile, now)
		guestProfiles[workloads[i].VMID] = profile
	}
	return guestProfiles
}

func demoProfileLastBackup(profile demoWorkloadProfile, now time.Time) time.Time {
	switch profile.ProtectionStory {
	case demoProtectionProtected, demoProtectionAttentionStale, demoProtectionAttentionFailed:
		if !now.IsZero() && profile.BackupAge > 0 {
			return now.UTC().Add(-profile.BackupAge)
		}
	}
	return time.Time{}
}

func applyDemoDockerScenario(state *models.StateSnapshot, now time.Time) {
	if state == nil {
		return
	}

	sort.Slice(state.DockerHosts, func(i, j int) bool {
		return state.DockerHosts[i].ID < state.DockerHosts[j].ID
	})
	// One docker host index is forced offline so the demo Docker page exposes a
	// disconnected host with its containers shown as exited rather than running.
	const offlineDockerIndex = 2
	onlineOrdinal := 0
	nativeIndex := -1
	for i := range state.DockerHosts {
		host := &state.DockerHosts[i]
		if strings.HasPrefix(host.ID, mockProxmoxLXCDockerHostSourcePrefix) {
			// Docker-in-LXC hosts are identity-bound to their parent LXC
			// guest; ensureMockDockerInLXCFixture owns their naming, status,
			// and inventory. Handing them a fleet profile would sever the
			// guest linkage and fabricate native engine inventory the
			// production pct exec path never reports.
			continue
		}
		nativeIndex++
		if nativeIndex == offlineDockerIndex {
			host.Hostname = "field-office-edge-01"
			host.DisplayName = "Field Office Edge 01"
			host.Status = "offline"
			host.Containers = applyDemoOfflineDockerContainers(host.Containers, now)
			ensureMockDockerNativeInventory(host, i, now)
			continue
		}
		hostProfile := demoDockerHostProfiles[onlineOrdinal%len(demoDockerHostProfiles)]
		// Hostnames must stay unique across the fleet: unified resources
		// treats hostname as identity, so two hosts sharing a profile name
		// collapse into one canonical resource whose status, parent, and
		// swarm services then flap on every poll as the two real hosts
		// alternate underneath it.
		round := onlineOrdinal / len(demoDockerHostProfiles)
		onlineOrdinal++
		host.Hostname = hostProfile.Hostname
		host.DisplayName = hostProfile.DisplayName
		if round > 0 {
			host.Hostname = fmt.Sprintf("%s-%d", hostProfile.Hostname, round+1)
			host.DisplayName = fmt.Sprintf("%s (%d)", hostProfile.DisplayName, round+1)
		}
		host.Status = "online"
		host.Containers = applyDemoDockerContainerProfiles(host.Containers, hostProfile.Containers, now)
		ensureMockDockerNativeInventory(host, i, now)
		flagged := 0
		for _, container := range host.Containers {
			switch strings.ToLower(strings.TrimSpace(container.Health)) {
			case "unhealthy", "starting":
				flagged++
			}
		}
		if flagged > 0 {
			host.Status = "degraded"
		}
	}
	populateMockDockerSwarmNodeInventories(state.DockerHosts, now)
}

func applyDemoOfflineDockerContainers(containers []models.DockerContainer, now time.Time) []models.DockerContainer {
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].ID < containers[j].ID
	})
	offlineProfiles := []demoDockerContainerProfile{
		{Name: "branch-portal", Image: "ghcr.io/pulse-demo/branch-portal:2026.04", Tags: []string{"edge", "web", "branch-office"}},
		{Name: "branch-syncthing", Image: "linuxserver/syncthing:1.27.10", Tags: []string{"sync", "edge", "branch-office"}},
		{Name: "branch-vpn", Image: "wireguard:1.0.20210914", Tags: []string{"vpn", "edge", "branch-office"}},
	}
	finished := now
	if finished.IsZero() {
		finished = time.Now()
	}
	finished = finished.Add(-18 * time.Minute)
	for i := range containers {
		profile := offlineProfiles[i%len(offlineProfiles)]
		containers[i].Name = profile.Name
		containers[i].Image = profile.Image
		containers[i].Labels = mergeScenarioLabelSet(containers[i].Labels, profile.Tags)
		containers[i].State = "exited"
		containers[i].Status = "Exited (137) 18 minutes ago"
		containers[i].Health = ""
		containers[i].StartedAt = nil
		finishedAt := finished
		containers[i].FinishedAt = &finishedAt
	}
	return containers
}

func applyDemoDockerContainerProfiles(
	containers []models.DockerContainer,
	profiles []demoDockerContainerProfile,
	now time.Time,
) []models.DockerContainer {
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].ID < containers[j].ID
	})
	for i := range containers {
		profile := profiles[i%len(profiles)]
		containers[i].Name = profile.Name
		containers[i].Image = profile.Image
		containers[i].Labels = mergeScenarioLabelSet(containers[i].Labels, profile.Tags)
		containers[i].State = "running"
		containers[i].Status = "Up"
		containers[i].Health = profile.Health
		if containers[i].StartedAt == nil {
			started := now
			if started.IsZero() {
				started = time.Now()
			}
			started = started.Add(-time.Duration(600+i*300) * time.Second)
			containers[i].StartedAt = &started
		}
		containers[i].FinishedAt = nil
		if containers[i].MemoryLimit <= 0 {
			containers[i].MemoryLimit = int64(2+i%3) * 1024 * 1024 * 1024
		}
		if containers[i].RootFilesystemBytes <= 0 {
			containers[i].RootFilesystemBytes = int64(20+i%4*10) * 1024 * 1024 * 1024
		}
	}
	return containers
}

func applyDemoKubernetesScenario(state *models.StateSnapshot, now time.Time) {
	if state == nil || len(state.KubernetesClusters) == 0 {
		return
	}

	for clusterIndex := range state.KubernetesClusters {
		cluster := &state.KubernetesClusters[clusterIndex]
		profile := demoKubernetesClusterProfileFor(clusterIndex, *cluster)
		cluster.Name = profile.Name
		cluster.DisplayName = profile.DisplayName
		cluster.Context = profile.Context
		cluster.Server = fmt.Sprintf("https://%s.k8s.local:6443", profile.ServerSlug)
		cluster.Status = "online"
		if strings.TrimSpace(profile.Version) != "" {
			cluster.Version = profile.Version
		}

		nodeProfiles := profile.NodeProfiles
		if len(nodeProfiles) == 0 {
			nodeProfiles = demoKubernetesClusterProfiles[0].NodeProfiles
		}
		nodeNameMap := make(map[string]string, len(cluster.Nodes))
		for i := range cluster.Nodes {
			nodeProfile := nodeProfiles[i%len(nodeProfiles)]
			oldName := cluster.Nodes[i].Name
			cluster.Nodes[i].Name = nodeProfile.Name
			cluster.Nodes[i].Ready = nodeProfile.Name != profile.OfflineNodeName
			cluster.Nodes[i].Unschedulable = false
			cluster.Nodes[i].Roles = append([]string(nil), nodeProfile.Roles...)
			nodeNameMap[oldName] = nodeProfile.Name
		}

		for i := range state.Hosts {
			host := &state.Hosts[i]
			if renamed, ok := nodeNameMap[strings.TrimSpace(host.Hostname)]; ok {
				host.Hostname = renamed
				host.DisplayName = humanizeHostDisplayName(renamed)
			}
		}

		clusterCarriesDeployCrash := profile.Scenario == demoK8sScenarioCrashLoopBackOff
		for i := range cluster.Deployments {
			deploymentProfile := demoKubernetesDeploymentProfiles[i%len(demoKubernetesDeploymentProfiles)]
			desired := deploymentProfile.DesiredReplicas
			ready := deploymentProfile.ReadyReplicas
			// payments-worker is under-replicated only in the cluster that
			// carries the crashloop story; everywhere else it runs healthy.
			if deploymentProfile.Name == "payments-worker" && !clusterCarriesDeployCrash {
				ready = desired
			}
			cluster.Deployments[i].Name = deploymentProfile.Name
			cluster.Deployments[i].Namespace = deploymentProfile.Namespace
			cluster.Deployments[i].DesiredReplicas = desired
			cluster.Deployments[i].UpdatedReplicas = ready
			cluster.Deployments[i].ReadyReplicas = ready
			cluster.Deployments[i].AvailableReplicas = ready
			cluster.Deployments[i].Labels = map[string]string{
				"app.kubernetes.io/name": deploymentProfile.Name,
				"cluster":                cluster.ID,
				"environment":            "production",
			}
		}

		for i := range cluster.Pods {
			podProfile := applyDemoKubernetesPodScenario(
				demoKubernetesPodProfiles[i%len(demoKubernetesPodProfiles)],
				profile.Scenario,
			)
			pod := &cluster.Pods[i]
			pod.Name = podProfile.Name
			pod.Namespace = podProfile.Namespace
			if podProfile.NodeIndex >= 0 && podProfile.NodeIndex < len(cluster.Nodes) {
				pod.NodeName = cluster.Nodes[podProfile.NodeIndex].Name
			} else if renamed, ok := nodeNameMap[pod.NodeName]; ok {
				pod.NodeName = renamed
			}
			pod.OwnerKind = podProfile.OwnerKind
			pod.OwnerName = podProfile.OwnerName
			pod.Phase = podProfile.Phase
			pod.Reason = podProfile.Reason
			pod.Message = podProfile.Message
			pod.Restarts = int(podProfile.Restarts)
			if len(pod.Containers) == 0 {
				pod.Containers = []models.KubernetesPodContainer{{}}
			}
			for j := range pod.Containers {
				pod.Containers[j].Name = podProfile.Container
				pod.Containers[j].Image = podProfile.Image
				pod.Containers[j].RestartCount = podProfile.Restarts
				if podProfile.ContainerOK {
					pod.Containers[j].Ready = true
					pod.Containers[j].State = "running"
					pod.Containers[j].Reason = ""
					pod.Containers[j].Message = ""
				} else {
					pod.Containers[j].Ready = false
					pod.Containers[j].State = "waiting"
					pod.Containers[j].Reason = podProfile.Reason
					if pod.Containers[j].Reason == "" {
						pod.Containers[j].Reason = "CrashLoopBackOff"
					}
					pod.Containers[j].Message = podProfile.Message
				}
			}
			pod.Labels = map[string]string{
				"app.kubernetes.io/name":     podProfile.OwnerName,
				"app.kubernetes.io/part-of":  "pulse-demo-estate",
				"app.kubernetes.io/instance": podProfile.OwnerName,
			}
		}

		applyDemoKubernetesNativeInventory(cluster, now)
		initializeMockKubernetesClusterUsage(cluster, now, false)
	}

	syncMockKubernetesNodeHosts(state)
}

// applyDemoKubernetesPodScenario rewrites a global pod profile so the
// cluster's chosen scenario is the only place its degraded state appears.
// Production EU stays healthy across the board; Staging EU keeps the
// payments-worker CrashLoopBackOff; Development EU re-labels the Pending
// cron-nightly-backfill pod as ImagePullBackOff to tell a distinct
// "forgot to push the image" story.
func applyDemoKubernetesPodScenario(
	profile demoKubernetesPodProfile,
	scenario demoKubernetesScenario,
) demoKubernetesPodProfile {
	switch scenario {
	case demoK8sScenarioCrashLoopBackOff:
		return profile
	case demoK8sScenarioImagePullBackOff:
		if profile.OwnerName == "payments-worker" {
			profile.ContainerOK = true
			profile.Reason = ""
			profile.Message = ""
			profile.Restarts = 0
		}
		if profile.OwnerName == "cron-nightly-backfill" {
			// Phase must be Running so the curated reconciler doesn't
			// "recover" the pod and wipe the container's waiting state.
			profile.Phase = "Running"
			profile.Reason = "ImagePullBackOff"
			profile.Message = "Back-off pulling image " + profile.Image
		}
		return profile
	default:
		// healthy / not-ready-worker scenarios: scrub any pod-level
		// crashloop story so only the cluster's own scenario shows.
		if profile.OwnerName == "payments-worker" {
			profile.ContainerOK = true
			profile.Reason = ""
			profile.Message = ""
			profile.Restarts = 0
		}
		return profile
	}
}

func demoKubernetesClusterProfileFor(index int, cluster models.KubernetesCluster) demoKubernetesClusterProfile {
	if index >= 0 && index < len(demoKubernetesClusterProfiles) {
		return demoKubernetesClusterProfiles[index]
	}
	name := strings.TrimSpace(cluster.Name)
	if name == "" {
		name = fmt.Sprintf("cluster-%d", index+1)
	}
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return demoKubernetesClusterProfile{
		Name:        slug,
		DisplayName: titleCase(name),
		Context:     slug + "-context",
		ServerSlug:  slug,
	}
}

func applyDemoKubernetesNativeInventory(cluster *models.KubernetesCluster, now time.Time) {
	if cluster == nil {
		return
	}
	createdAt := now.Add(-36 * time.Hour)
	labels := func(component string) map[string]string {
		return map[string]string{
			"app.kubernetes.io/part-of": "pulse-demo-estate",
			"app.kubernetes.io/name":    component,
			"environment":               "production",
		}
	}

	cluster.Namespaces = []models.KubernetesNamespace{
		{UID: cluster.ID + "-ns-apps", Name: "apps", Phase: "Active", CreatedAt: createdAt, Labels: map[string]string{"environment": "production"}},
		{UID: cluster.ID + "-ns-services", Name: "services", Phase: "Active", CreatedAt: createdAt, Labels: map[string]string{"environment": "production"}},
		{UID: cluster.ID + "-ns-monitoring", Name: "monitoring", Phase: "Active", CreatedAt: createdAt, Labels: map[string]string{"environment": "production"}},
	}
	cluster.Services = []models.KubernetesService{
		{UID: cluster.ID + "-svc-checkout-web", Name: "checkout-web", Namespace: "apps", ServiceType: "ClusterIP", ClusterIP: "10.96.12.10", Ports: []models.KubernetesServicePort{{Name: "http", Protocol: "TCP", Port: 80, TargetPort: "8080"}}, Selector: map[string]string{"app.kubernetes.io/name": "checkout-web"}, CreatedAt: createdAt, Labels: labels("checkout-web")},
		{UID: cluster.ID + "-svc-checkout-public", Name: "checkout-public", Namespace: "apps", ServiceType: "NodePort", ClusterIP: "10.96.12.20", ExternalIPs: []string{"203.0.113.24"}, Ports: []models.KubernetesServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: "8443", NodePort: 30443}}, Selector: map[string]string{"app.kubernetes.io/name": "checkout-web"}, CreatedAt: createdAt, Labels: labels("checkout-web")},
		{UID: cluster.ID + "-svc-checkout-api", Name: "checkout-api", Namespace: "services", ServiceType: "ClusterIP", ClusterIP: "10.96.18.24", Ports: []models.KubernetesServicePort{{Name: "http", Protocol: "TCP", Port: 8080, TargetPort: "8080"}}, Selector: map[string]string{"app.kubernetes.io/name": "checkout-api"}, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.EndpointSlices = []models.KubernetesEndpointSlice{
		{UID: cluster.ID + "-eps-checkout-api", Name: "checkout-api-abc12", Namespace: "services", AddressType: "IPv4", ServiceName: "checkout-api", EndpointCount: 3, ReadyEndpointCount: 3, Ports: []models.KubernetesEndpointPort{{Name: "http", Protocol: "TCP", Port: 8080, AppProtocol: "kubernetes.io/http"}}, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.Ingresses = []models.KubernetesIngress{
		{UID: cluster.ID + "-ing-checkout-web", Name: "checkout-web", Namespace: "apps", ClassName: "nginx", Hosts: []string{"checkout.demo.pulse.local"}, Addresses: []string{"198.51.100.24"}, CreatedAt: createdAt, Labels: labels("checkout-web")},
	}
	allowExpansion := true
	cluster.StorageClasses = []models.KubernetesStorageClass{
		{UID: cluster.ID + "-sc-fast-ssd", Name: "fast-ssd", Provisioner: "csi.pulse-demo.local", ReclaimPolicy: "Delete", VolumeBindingMode: "WaitForFirstConsumer", AllowVolumeExpansion: &allowExpansion, ParameterKeys: []string{"type", "iops", "encrypted"}, CreatedAt: createdAt, Labels: labels("fast-ssd")},
	}
	cluster.PersistentVolumes = []models.KubernetesPersistentVolume{
		{UID: cluster.ID + "-pv-checkout-postgres", Name: cluster.ID + "-pv-checkout-postgres", Phase: "Bound", StorageClass: "fast-ssd", CapacityBytes: int64(80) << 30, AccessModes: []string{"ReadWriteOnce"}, ReclaimPolicy: "Delete", ClaimNamespace: "services", ClaimName: "checkout-postgres-data", CreatedAt: createdAt, Labels: labels("checkout-postgres")},
	}
	cluster.PersistentVolumeClaims = []models.KubernetesPersistentVolumeClaim{
		{UID: cluster.ID + "-pvc-checkout-postgres", Name: "checkout-postgres-data", Namespace: "services", Phase: "Bound", StorageClass: "fast-ssd", RequestedBytes: int64(80) << 30, CapacityBytes: int64(80) << 30, AccessModes: []string{"ReadWriteOnce"}, VolumeName: cluster.ID + "-pv-checkout-postgres", CreatedAt: createdAt, Labels: labels("checkout-postgres")},
	}
	readyNodeCount := 0
	for _, node := range cluster.Nodes {
		if node.Ready {
			readyNodeCount++
		}
	}
	desiredNodeCount := int32(len(cluster.Nodes))
	readyDaemonCount := int32(readyNodeCount)
	unavailableDaemonCount := desiredNodeCount - readyDaemonCount
	if unavailableDaemonCount < 0 {
		unavailableDaemonCount = 0
	}
	jobStartedAt := now.Add(-42 * time.Minute)
	jobCompletedAt := now.Add(-37 * time.Minute)
	lastCronScheduleAt := now.Add(-7 * time.Minute)
	lastCronSuccessAt := now.Add(-67 * time.Minute)
	cluster.StatefulSets = []models.KubernetesStatefulSet{
		{UID: cluster.ID + "-sts-platform-observability", Name: "platform-observability", Namespace: "monitoring", DesiredReplicas: 2, CurrentReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2, ServiceName: "platform-observability", Labels: labels("platform-observability")},
	}
	cluster.DaemonSets = []models.KubernetesDaemonSet{
		{UID: cluster.ID + "-ds-node-exporter", Name: "node-exporter", Namespace: "monitoring", DesiredNumberScheduled: desiredNodeCount, CurrentNumberScheduled: desiredNodeCount, NumberReady: readyDaemonCount, UpdatedNumberScheduled: readyDaemonCount, NumberAvailable: readyDaemonCount, NumberUnavailable: unavailableDaemonCount, Labels: labels("node-exporter")},
	}
	cluster.Jobs = []models.KubernetesJob{
		{UID: cluster.ID + "-job-nightly-backfill", Name: "nightly-backfill-28918234", Namespace: "services", DesiredCompletions: 1, Succeeded: 1, Active: 0, Failed: 0, StartTime: &jobStartedAt, CompletionTime: &jobCompletedAt, Labels: labels("nightly-backfill")},
	}
	cluster.CronJobs = []models.KubernetesCronJob{
		{UID: cluster.ID + "-cron-nightly-backfill", Name: "cron-nightly-backfill", Namespace: "services", Schedule: "0 2 * * *", Active: 1, LastScheduleTime: &lastCronScheduleAt, LastSuccessfulTime: &lastCronSuccessAt, Labels: labels("nightly-backfill")},
	}
	cluster.NetworkPolicies = []models.KubernetesNetworkPolicy{
		{UID: cluster.ID + "-netpol-default-deny", Name: "default-deny", Namespace: "services", PolicyTypes: []string{"Ingress", "Egress"}, IngressRuleCount: 1, EgressRuleCount: 1, CreatedAt: createdAt, Labels: labels("default-deny")},
	}
	cluster.ConfigMaps = []models.KubernetesConfigMap{
		{UID: cluster.ID + "-cm-checkout-api", Name: "checkout-api-config", Namespace: "services", MetadataOnly: true, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.Secrets = []models.KubernetesSecret{
		{UID: cluster.ID + "-secret-checkout-api", Name: "checkout-api-runtime", Namespace: "services", MetadataOnly: true, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.ServiceAccounts = []models.KubernetesServiceAccount{
		{UID: cluster.ID + "-sa-checkout-api", Name: "checkout-api", Namespace: "services", SecretCount: 1, ImagePullSecrets: []string{"registry-pull"}, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.Roles = []models.KubernetesRole{
		{UID: cluster.ID + "-role-checkout-api", Name: "checkout-api-runtime", Namespace: "services", RuleCount: 4, CreatedAt: createdAt, Labels: labels("checkout-api")},
		{UID: cluster.ID + "-role-payments-worker", Name: "payments-worker-runtime", Namespace: "services", RuleCount: 6, CreatedAt: createdAt, Labels: labels("payments-worker")},
	}
	cluster.ClusterRoles = []models.KubernetesClusterRole{
		{UID: cluster.ID + "-clusterrole-monitoring", Name: "pulse-demo-monitoring", RuleCount: 12, AggregationLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}, CreatedAt: createdAt, Labels: labels("monitoring")},
	}
	cluster.RoleBindings = []models.KubernetesRoleBinding{
		{UID: cluster.ID + "-rb-checkout-api", Name: "checkout-api-runtime", Namespace: "services", RoleKind: "Role", RoleName: "checkout-api-runtime", SubjectCount: 1, SubjectKinds: []string{"ServiceAccount"}, CreatedAt: createdAt, Labels: labels("checkout-api")},
		{UID: cluster.ID + "-rb-payments-worker", Name: "payments-worker-runtime", Namespace: "services", RoleKind: "Role", RoleName: "payments-worker-runtime", SubjectCount: 2, SubjectKinds: []string{"Group", "ServiceAccount"}, CreatedAt: createdAt, Labels: labels("payments-worker")},
	}
	cluster.ClusterRoleBindings = []models.KubernetesClusterRoleBinding{
		{UID: cluster.ID + "-crb-monitoring", Name: "pulse-demo-monitoring", RoleKind: "ClusterRole", RoleName: "pulse-demo-monitoring", SubjectCount: 3, SubjectKinds: []string{"Group", "ServiceAccount", "User"}, CreatedAt: createdAt, Labels: labels("monitoring")},
	}
	cluster.ResourceQuotas = []models.KubernetesResourceQuota{
		{UID: cluster.ID + "-quota-services", Name: "services-quota", Namespace: "services", Hard: map[string]string{"pods": "80", "requests.cpu": "24", "requests.memory": "96Gi"}, Used: map[string]string{"pods": "34", "requests.cpu": "11", "requests.memory": "42Gi"}, CreatedAt: createdAt, Labels: labels("services-quota")},
	}
	cluster.LimitRanges = []models.KubernetesLimitRange{
		{UID: cluster.ID + "-limits-services", Name: "services-defaults", Namespace: "services", LimitTypes: []string{"Container", "Pod"}, CreatedAt: createdAt, Labels: labels("services-defaults")},
	}
	cluster.PodDisruptionBudgets = []models.KubernetesPodDisruptionBudget{
		{UID: cluster.ID + "-pdb-checkout-api", Name: "checkout-api", Namespace: "services", MinAvailable: "2", DesiredHealthy: 2, CurrentHealthy: 3, DisruptionsAllowed: 1, ExpectedPods: 3, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.HorizontalPodAutoscalers = []models.KubernetesHorizontalPodAutoscaler{
		{UID: cluster.ID + "-hpa-checkout-api", Name: "checkout-api", Namespace: "services", TargetKind: "Deployment", TargetName: "checkout-api", MinReplicas: 3, MaxReplicas: 12, CurrentReplicas: 3, DesiredReplicas: 4, MetricTypes: []string{"Resource:cpu", "Resource:memory"}, CreatedAt: createdAt, Labels: labels("checkout-api")},
	}
	cluster.Events = []models.KubernetesEvent{
		{UID: cluster.ID + "-event-payments-worker", Name: "payments-worker.1", Namespace: "services", EventType: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container", InvolvedKind: "Pod", InvolvedName: "payments-worker-58484d44f7-f6gzh", Count: 7, EventTime: &createdAt},
	}
}

func applyDemoHostScenario(state *models.StateSnapshot, now time.Time) {
	if state == nil {
		return
	}

	for i := range state.Hosts {
		host := &state.Hosts[i]
		host.Status = "online"
		if strings.TrimSpace(host.DisplayName) == "" {
			host.DisplayName = humanizeHostDisplayName(host.Hostname)
		}
		hostname := strings.ToLower(strings.TrimSpace(host.Hostname))
		if isDemoOfflineProxmoxNode(hostname) || hostname == "prod-euw1-k8s-03" {
			host.Status = "offline"
			host.CPUUsage = 0
			host.Memory.Used = 0
			host.Memory.Usage = 0
			host.Memory.Free = host.Memory.Total
			host.Memory.SwapUsed = 0
			host.LoadAverage = []float64{0.0, 0.0, 0.0}
			host.UptimeSeconds = 0
			host.NetInRate = 0
			host.NetOutRate = 0
			host.DiskReadRate = 0
			host.DiskWriteRate = 0
			for j := range host.Disks {
				host.Disks[j].Used = 0
				host.Disks[j].Free = host.Disks[j].Total
				host.Disks[j].Usage = -1
			}
			// An offline host stopped reporting, so its sighting must stay
			// stale. Refreshing it like the online hosts below made the
			// agent's LastSeen race the 2-minute source staleness threshold
			// and sawtooth the resource between online and offline every
			// couple of minutes.
			if !now.IsZero() {
				host.LastSeen = now.Add(-10 * time.Minute)
			}
			continue
		}
		if now.IsZero() {
			continue
		}
		if host.LastSeen.IsZero() || host.LastSeen.Before(now.Add(-2*time.Minute)) || host.LastSeen.After(now) {
			host.LastSeen = demoRecentSeenAt(now, "host", host.ID, host.Hostname)
		}
	}
}

func applyDemoStorageScenario(state *models.StateSnapshot, now time.Time) {
	if state == nil {
		return
	}

	offlineNodes := make(map[string]struct{})
	for _, node := range state.Nodes {
		if strings.EqualFold(strings.TrimSpace(node.Status), "offline") {
			offlineNodes[strings.ToLower(strings.TrimSpace(node.Name))] = struct{}{}
		}
	}
	for i := range state.Storage {
		storage := &state.Storage[i]
		if name := strings.TrimSpace(storageScenarioAlias(*storage)); name != "" {
			storage.Name = name
		}
		storage.Instance = scenarioClusterAlias(storage.Instance)
		if _, offline := offlineNodes[strings.ToLower(strings.TrimSpace(storage.Node))]; offline {
			storage.Status = "offline"
			storage.Enabled = false
			storage.Active = false
			storage.Used = 0
			storage.Free = storage.Total
			storage.Usage = 0
			if !now.IsZero() {
				storage.LastSeen = now.Add(-10 * time.Minute)
			}
		}
	}
	for i := range state.PhysicalDisks {
		state.PhysicalDisks[i].Instance = scenarioClusterAlias(state.PhysicalDisks[i].Instance)
	}
	for i := range state.CephClusters {
		cluster := &state.CephClusters[i]
		cluster.Instance = scenarioClusterAlias(cluster.Instance)
		cluster.Name = scenarioClusterAlias(cluster.Name)
	}

	for i := range state.PBSInstances {
		state.PBSInstances[i].Name = []string{"backup-vault", "dr-vault"}[minInt(i, 1)]
		if i == 1 {
			// dr-vault simulates a degraded secondary so the PBS page shows a
			// non-healthy instance alongside the healthy primary.
			state.PBSInstances[i].Status = "degraded"
			state.PBSInstances[i].ConnectionHealth = "degraded"
		} else {
			state.PBSInstances[i].Status = "online"
			state.PBSInstances[i].ConnectionHealth = "healthy"
		}
		state.PBSInstances[i].LastSeen = demoRecentSeenAt(now, "pbs", state.PBSInstances[i].ID, state.PBSInstances[i].Name)
		if len(state.PBSInstances[i].Datastores) > 0 {
			datastoreNames := []string{"primary-vault", "offsite-vault", "replica-vault"}
			for j := range state.PBSInstances[i].Datastores {
				if j < len(datastoreNames) {
					state.PBSInstances[i].Datastores[j].Name = datastoreNames[j]
				}
				state.PBSInstances[i].Datastores[j].Status = "online"
			}
		}
	}

	for i := range state.PMGInstances {
		if i == 0 {
			state.PMGInstances[i].Name = "mail-gateway-eu"
			state.PMGInstances[i].Status = "online"
			state.PMGInstances[i].ConnectionHealth = "healthy"
		} else {
			// mail-gateway-us simulates a degraded edge gateway so the PMG page
			// shows a non-healthy instance alongside the healthy primary.
			state.PMGInstances[i].Name = "mail-gateway-us"
			state.PMGInstances[i].Status = "degraded"
			state.PMGInstances[i].ConnectionHealth = "degraded"
		}
		state.PMGInstances[i].LastSeen = demoRecentSeenAt(now, "pmg", state.PMGInstances[i].ID, state.PMGInstances[i].Name)
	}
}

func syncDemoConnectionHealth(state *models.StateSnapshot) {
	if state == nil {
		return
	}
	if state.ConnectionHealth == nil {
		state.ConnectionHealth = make(map[string]bool)
	}

	for key := range state.ConnectionHealth {
		switch {
		case strings.HasPrefix(key, "docker-"),
			strings.HasPrefix(key, "kubernetes-"),
			strings.HasPrefix(key, "host-"),
			strings.HasPrefix(key, "pbs-"),
			strings.HasPrefix(key, "pmg-"),
			strings.HasPrefix(key, "pve-"):
			delete(state.ConnectionHealth, key)
		}
	}

	for _, node := range state.Nodes {
		if name := strings.TrimSpace(node.Name); name != "" {
			state.ConnectionHealth["pve-"+name] = !strings.EqualFold(strings.TrimSpace(node.Status), "offline")
		}
	}
	for _, host := range state.DockerHosts {
		if id := strings.TrimSpace(host.ID); id != "" {
			state.ConnectionHealth[dockerConnectionPrefix+id] = !strings.EqualFold(strings.TrimSpace(host.Status), "offline")
		}
	}
	for _, cluster := range state.KubernetesClusters {
		if id := strings.TrimSpace(cluster.ID); id != "" {
			state.ConnectionHealth[kubernetesConnectionPrefix+id] = !strings.EqualFold(strings.TrimSpace(cluster.Status), "offline")
		}
	}
	for _, host := range state.Hosts {
		if id := strings.TrimSpace(host.ID); id != "" {
			state.ConnectionHealth[hostConnectionPrefix+id] = !strings.EqualFold(strings.TrimSpace(host.Status), "offline")
		}
	}
	for _, inst := range state.PBSInstances {
		if name := strings.TrimSpace(inst.Name); name != "" {
			state.ConnectionHealth["pbs-"+name] = !strings.EqualFold(strings.TrimSpace(inst.Status), "offline")
		}
	}
	for _, inst := range state.PMGInstances {
		if name := strings.TrimSpace(inst.Name); name != "" {
			state.ConnectionHealth["pmg-"+name] = !strings.EqualFold(strings.TrimSpace(inst.Status), "offline")
		}
	}
}

func demoRecentSeenAt(now time.Time, parts ...string) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	offsetSeconds := 4 + mockStableChoice(15, parts...)
	return now.UTC().Add(-time.Duration(offsetSeconds) * time.Second)
}

func applyDemoBackupScenario(
	state *models.StateSnapshot,
	vmProfiles map[int]demoWorkloadProfile,
	containerProfiles map[int]demoWorkloadProfile,
	now time.Time,
) {
	if state == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	guests := make([]demoBackupGuest, 0, len(state.VMs)+len(state.Containers))
	curatedKeys := make(map[string]struct{}, len(state.VMs)+len(state.Containers))
	for _, vm := range state.VMs {
		profile, ok := vmProfiles[vm.VMID]
		if !ok {
			continue
		}
		guests = append(guests, demoBackupGuest{
			Profile:  profile,
			Node:     vm.Node,
			Instance: vm.Instance,
			Type:     "qemu",
			PBSKind:  "vm",
			VMID:     vm.VMID,
			DiskSize: vm.Disk.Total,
		})
		curatedKeys[demoBackupGuestKey("qemu", vm.VMID)] = struct{}{}
	}
	for _, container := range state.Containers {
		profile, ok := containerProfiles[container.VMID]
		if !ok {
			continue
		}
		guests = append(guests, demoBackupGuest{
			Profile:  profile,
			Node:     container.Node,
			Instance: container.Instance,
			Type:     "lxc",
			PBSKind:  "ct",
			VMID:     container.VMID,
			DiskSize: container.Disk.Total,
		})
		curatedKeys[demoBackupGuestKey("lxc", container.VMID)] = struct{}{}
	}

	storageBackups := make([]models.StorageBackup, 0, len(state.PVEBackups.StorageBackups)+len(guests)*3)
	for _, backup := range state.PVEBackups.StorageBackups {
		if _, curated := curatedKeys[demoBackupGuestKey(backup.Type, backup.VMID)]; curated {
			continue
		}
		if backup.Storage == "local" {
			backup.Storage = "backup-vault"
		}
		storageBackups = append(storageBackups, backup)
	}

	pbsBackups := make([]models.PBSBackup, 0, len(state.PBSBackups)+len(guests)*2)
	for _, backup := range state.PBSBackups {
		vmid := atoiSafe(backup.VMID)
		if _, curated := curatedKeys[demoBackupGuestKey(backup.BackupType, vmid)]; curated {
			continue
		}
		backup.Instance = scenarioPBSInstanceAlias(backup.Instance)
		backup.Datastore = scenarioPBSDatastoreAlias(backup.Datastore)
		pbsBackups = append(pbsBackups, backup)
	}

	guestSnapshots := make([]models.GuestSnapshot, 0, len(state.PVEBackups.GuestSnapshots)+len(guests))
	for _, snapshot := range state.PVEBackups.GuestSnapshots {
		if _, curated := curatedKeys[demoBackupGuestKey(snapshot.Type, snapshot.VMID)]; curated {
			continue
		}
		guestSnapshots = append(guestSnapshots, snapshot)
	}

	backupTasks := make([]models.BackupTask, 0, len(state.PVEBackups.BackupTasks)+len(guests))
	for _, task := range state.PVEBackups.BackupTasks {
		if _, curated := curatedKeys[demoBackupGuestKey(task.Type, task.VMID)]; curated {
			continue
		}
		backupTasks = append(backupTasks, task)
	}

	for _, guest := range guests {
		guestBackups, guestPBSBackups, snapshots, tasks := demoProtectionArtifacts(guest, now)
		storageBackups = append(storageBackups, guestBackups...)
		pbsBackups = append(pbsBackups, guestPBSBackups...)
		guestSnapshots = append(guestSnapshots, snapshots...)
		backupTasks = append(backupTasks, tasks...)
	}

	sort.SliceStable(storageBackups, func(i, j int) bool {
		if storageBackups[i].Time.Equal(storageBackups[j].Time) {
			return storageBackups[i].ID < storageBackups[j].ID
		}
		return storageBackups[i].Time.After(storageBackups[j].Time)
	})
	sort.SliceStable(pbsBackups, func(i, j int) bool {
		if pbsBackups[i].BackupTime.Equal(pbsBackups[j].BackupTime) {
			return pbsBackups[i].ID < pbsBackups[j].ID
		}
		return pbsBackups[i].BackupTime.After(pbsBackups[j].BackupTime)
	})
	sort.SliceStable(guestSnapshots, func(i, j int) bool {
		if guestSnapshots[i].Time.Equal(guestSnapshots[j].Time) {
			return guestSnapshots[i].ID < guestSnapshots[j].ID
		}
		return guestSnapshots[i].Time.After(guestSnapshots[j].Time)
	})
	sort.SliceStable(backupTasks, func(i, j int) bool {
		if backupTasks[i].StartTime.Equal(backupTasks[j].StartTime) {
			return backupTasks[i].ID < backupTasks[j].ID
		}
		return backupTasks[i].StartTime.After(backupTasks[j].StartTime)
	})

	state.PVEBackups.StorageBackups = storageBackups
	state.PVEBackups.GuestSnapshots = guestSnapshots
	state.PVEBackups.BackupTasks = backupTasks
	state.PBSBackups = pbsBackups

	for i := range state.ReplicationJobs {
		job := &state.ReplicationJobs[i]
		job.Instance = scenarioClusterAlias(job.Instance)
		if profile, ok := vmProfiles[job.GuestID]; ok {
			job.GuestName = profile.Name
		}
		job.SourceStorage = scenarioStorageAliasForNode(job.SourceStorage, job.SourceNode)
		job.TargetStorage = scenarioStorageAliasForNode(job.TargetStorage, job.TargetNode)
	}

	state.PMGBackups = extractPMGBackups(state.PVEBackups.StorageBackups)
	state.Backups.PVE = state.PVEBackups
	state.Backups.PBS = append([]models.PBSBackup(nil), state.PBSBackups...)
	state.Backups.PMG = append([]models.PMGBackup(nil), state.PMGBackups...)
}

func demoBackupGuestKey(guestType string, vmid int) string {
	switch strings.ToLower(strings.TrimSpace(guestType)) {
	case "qemu", "vm":
		return fmt.Sprintf("vm:%d", vmid)
	case "lxc", "ct":
		return fmt.Sprintf("ct:%d", vmid)
	default:
		return ""
	}
}

func scenarioPBSInstanceAlias(value string) string {
	switch strings.TrimSpace(value) {
	case "pbs-main":
		return "backup-vault"
	case "pbs-secondary":
		return "dr-vault"
	default:
		return value
	}
}

func scenarioPBSDatastoreAlias(value string) string {
	switch strings.TrimSpace(value) {
	case "backup-store":
		return "primary-vault"
	case "offsite-backup":
		return "offsite-vault"
	case "replica-store":
		return "replica-vault"
	default:
		return value
	}
}

func demoProtectionArtifacts(
	guest demoBackupGuest,
	now time.Time,
) ([]models.StorageBackup, []models.PBSBackup, []models.GuestSnapshot, []models.BackupTask) {
	profile := guest.Profile
	backupSize := guest.DiskSize / 8
	if backupSize <= 0 {
		backupSize = 1 << 30
	}
	snapshotSize := guest.DiskSize / 12
	if snapshotSize <= 0 {
		snapshotSize = 512 << 20
	}

	addTask := func(status string, completedAt time.Time, errorMessage string) models.BackupTask {
		startedAt := completedAt.Add(-12 * time.Minute)
		return models.BackupTask{
			ID:         fmt.Sprintf("demo-task-%s-%d-%s", guest.PBSKind, guest.VMID, strings.ToLower(status)),
			Node:       guest.Node,
			Instance:   guest.Instance,
			Type:       guest.Type,
			VMID:       guest.VMID,
			Status:     status,
			StartTime:  startedAt,
			EndTime:    completedAt,
			ObservedAt: completedAt,
			Size:       backupSize,
			Error:      errorMessage,
		}
	}
	addSnapshot := func(index int, age time.Duration) models.GuestSnapshot {
		snapshotAt := now.Add(-age)
		return models.GuestSnapshot{
			ID:          fmt.Sprintf("demo-snapshot-%s-%d-%d", guest.PBSKind, guest.VMID, index),
			Name:        "pre-maintenance",
			Node:        guest.Node,
			Instance:    guest.Instance,
			Type:        guest.Type,
			VMID:        guest.VMID,
			Time:        snapshotAt,
			Description: fmt.Sprintf("%s local rollback snapshot", profile.Name),
			VMState:     guest.Type == "qemu",
			SizeBytes:   snapshotSize,
		}
	}

	switch profile.ProtectionStory {
	case demoProtectionUnknown:
		return nil, nil, nil, nil
	case demoProtectionTaskOnly:
		taskAt := now.Add(-18 * time.Hour)
		return nil, nil, nil, []models.BackupTask{addTask("OK", taskAt, "")}
	case demoProtectionSnapshotOnly:
		snapshotAge := 20 * time.Hour
		taskAt := now.Add(-18 * time.Hour)
		return nil, nil, []models.GuestSnapshot{addSnapshot(0, snapshotAge)}, []models.BackupTask{addTask("OK", taskAt, "")}
	}

	baseAge := profile.BackupAge
	if baseAge <= 0 {
		baseAge = 12 * time.Hour
	}
	// Two evidence-backed recovery points per protected guest are enough to
	// demonstrate recency and retention without multiplying the public demo's
	// payload by a third historical copy across a 500+ workload estate.
	backupAges := []time.Duration{baseAge, baseAge + 4*24*time.Hour}
	storageBackups := make([]models.StorageBackup, 0, len(backupAges))
	for index, age := range backupAges {
		backupAt := now.Add(-age)
		format := "vma.zst"
		if guest.Type == "lxc" {
			format = "tar.zst"
		}
		storageBackups = append(storageBackups, models.StorageBackup{
			ID:           fmt.Sprintf("demo-backup-%s-%d-%d", guest.PBSKind, guest.VMID, index),
			Storage:      "backup-vault",
			Node:         guest.Node,
			Instance:     guest.Instance,
			Type:         guest.Type,
			VMID:         guest.VMID,
			Time:         backupAt,
			CTime:        backupAt.Unix(),
			Size:         backupSize,
			Format:       format,
			Notes:        fmt.Sprintf("Backup of %s", profile.Name),
			Protected:    index == len(backupAges)-1,
			Volid:        fmt.Sprintf("backup-vault:backup/vzdump-%s-%d-%s.%s", guest.Type, guest.VMID, backupAt.Format("2006_01_02-15_04_05"), format),
			IsPBS:        false,
			Verified:     false,
			Verification: "",
		})
	}

	pbsBackups := make([]models.PBSBackup, 0, 2)
	if guest.VMID%2 == 0 {
		for index, age := range backupAges[:2] {
			backupAt := now.Add(-age)
			pbsBackups = append(pbsBackups, models.PBSBackup{
				ID:              fmt.Sprintf("demo-pbs-%s-%d-%d", guest.PBSKind, guest.VMID, index),
				Instance:        "backup-vault",
				Datastore:       "primary-vault",
				Namespace:       "root",
				BackupType:      guest.PBSKind,
				VMID:            strconv.Itoa(guest.VMID),
				BackupTime:      backupAt,
				Size:            backupSize,
				Protected:       index == 1,
				Verified:        true,
				VerificationRaw: "verified",
				Comment:         fmt.Sprintf("Protected recovery point for %s", profile.Name),
				Files:           []string{fmt.Sprintf("%s-%d-%d.pxar", guest.PBSKind, guest.VMID, index)},
				Owner:           "automation@pbs",
			})
		}
	}

	tasks := []models.BackupTask{addTask("OK", now.Add(-baseAge), "")}
	if profile.ProtectionStory == demoProtectionAttentionFailed {
		tasks = append(tasks, addTask("failed", now.Add(-3*time.Hour), "backup write failed: repository connection timed out"))
	}
	snapshotAge := time.Duration(
		18+mockStableChoice(102, guest.PBSKind, strconv.Itoa(guest.VMID), "snapshot-age"),
	) * time.Hour
	snapshots := []models.GuestSnapshot{addSnapshot(0, snapshotAge)}
	return storageBackups, pbsBackups, snapshots, tasks
}

func namedWorkloadProfile(profiles []demoWorkloadProfile, index int) demoWorkloadProfile {
	if len(profiles) == 0 {
		return demoWorkloadProfile{Name: fmt.Sprintf("workload-%02d", index+1)}
	}
	profile := profiles[index%len(profiles)]
	if index < len(profiles) {
		return profile
	}
	baseName := strings.TrimSpace(profile.Name)
	baseName = strings.TrimRight(baseName, "0123456789-")
	baseName = strings.TrimRight(baseName, "-")
	if baseName == "" {
		baseName = "workload"
	}
	profile.Name = fmt.Sprintf("%s-%02d", baseName, index+1)
	return profile
}

func mergeScenarioTags(existing []string, scenario []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(scenario))
	out := make([]string, 0, len(existing)+len(scenario))
	for _, set := range [][]string{scenario, existing} {
		for _, tag := range set {
			tag = strings.TrimSpace(strings.ToLower(tag))
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}

func mergeScenarioLabelSet(existing map[string]string, tags []string) map[string]string {
	labels := make(map[string]string, len(existing)+len(tags)+1)
	for key, value := range existing {
		labels[key] = value
	}
	if _, ok := labels["com.pulse.demo"]; !ok {
		labels["com.pulse.demo"] = "true"
	}
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		labels["com.pulse.role."+tag] = "true"
	}
	return labels
}

func normalizeWorkloadState(preferred, fallback string) string {
	state := strings.ToLower(strings.TrimSpace(preferred))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(fallback))
	}
	switch state {
	case "running", "online":
		return "running"
	case "stopped", "offline":
		return "stopped"
	default:
		return "running"
	}
}

func scenarioClusterAlias(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.EqualFold(trimmed, "standalone") {
		return trimmed
	}
	if strings.EqualFold(trimmed, "mock-cluster") {
		return demoProxmoxClusterProfiles[0].Name
	}
	if profile, ok := lookupDemoProxmoxClusterProfile(trimmed); ok {
		return profile.Name
	}
	return name
}

func storageScenarioAlias(storage models.Storage) string {
	normalizedName := strings.ToLower(strings.TrimSpace(storage.Name))
	if profile, ok := lookupDemoProxmoxClusterProfile(storage.Instance); ok {
		switch normalizedName {
		case "shared-storage":
			return profile.SharedStorageName
		case "pbs-primary":
			return profile.StoragePrefix + "-backup-primary"
		case "pbs-offsite":
			return profile.StoragePrefix + "-backup-offsite"
		}
	}
	return scenarioStorageAliasForNode(storage.Name, storage.Node)
}

func scenarioStorageAliasForNode(name, node string) string {
	trimmedName := strings.TrimSpace(name)
	normalizedName := strings.ToLower(trimmedName)
	normalizedNode := strings.ToLower(strings.TrimSpace(node))
	switch normalizedName {
	case "shared-storage":
		return "shared-backup-fabric"
	case "local":
		return fmt.Sprintf("%s-iso-library", demoProxmoxNodeStoragePrefix(normalizedNode))
	case "local-zfs":
		return fmt.Sprintf("%s-service-pool", demoProxmoxNodeStoragePrefix(normalizedNode))
	default:
		return trimmedName
	}
}

func lookupDemoProxmoxClusterProfile(sourceInstance string) (demoProxmoxClusterProfile, bool) {
	for _, profile := range demoProxmoxClusterProfiles {
		if strings.EqualFold(strings.TrimSpace(sourceInstance), profile.SourceInstance) ||
			strings.EqualFold(strings.TrimSpace(sourceInstance), profile.Name) {
			return profile, true
		}
	}
	return demoProxmoxClusterProfile{}, false
}

func demoProxmoxNodeOrdinal(nodeName string) (int, bool) {
	normalized := strings.ToLower(strings.TrimSpace(nodeName))
	if !strings.HasPrefix(normalized, "pve") {
		return 0, false
	}
	ordinal, err := strconv.Atoi(strings.TrimPrefix(normalized, "pve"))
	return ordinal, err == nil && ordinal > 0
}

func demoProxmoxNodeDisplayName(node models.Node, sourceInstance string) string {
	if profile, ok := lookupDemoProxmoxClusterProfile(sourceInstance); ok {
		ordinal, valid := demoProxmoxNodeOrdinal(node.Name)
		if !valid {
			return profile.NodeDisplayPrefix
		}
		letter := rune('A' + (ordinal-1)%mockProxmoxClusterSize)
		return fmt.Sprintf("%s %c", profile.NodeDisplayPrefix, letter)
	}

	switch strings.ToLower(strings.TrimSpace(node.Name)) {
	case "standalone1":
		return "London Edge Standalone"
	case "standalone2":
		return "Automation Lab Standalone"
	default:
		return humanizeHostDisplayName(node.Name)
	}
}

func demoProxmoxNodeStoragePrefix(nodeName string) string {
	if ordinal, ok := demoProxmoxNodeOrdinal(nodeName); ok {
		clusterIndex := (ordinal - 1) / mockProxmoxClusterSize
		if clusterIndex >= 0 && clusterIndex < len(demoProxmoxClusterProfiles) {
			profile := demoProxmoxClusterProfiles[clusterIndex]
			letter := rune('a' + (ordinal-1)%mockProxmoxClusterSize)
			return fmt.Sprintf("%s-%c", profile.StoragePrefix, letter)
		}
	}
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(nodeName)), "_", "-")
}

func isDemoOfflineProxmoxNode(nodeName string) bool {
	return strings.EqualFold(strings.TrimSpace(nodeName), demoOfflineProxmoxNode)
}

func defaultDemoVMwareMetrics() *vmware.InventoryMetrics {
	return &vmware.InventoryMetrics{
		CPUPercent:              demoFloat64Ptr(17.4),
		MemoryPercent:           demoFloat64Ptr(48.8),
		MemoryUsedBytes:         demoInt64Ptr(4_192_337_920),
		MemoryTotalBytes:        demoInt64Ptr(8_589_934_592),
		NetInBytesPerSecond:     demoFloat64Ptr(260_000),
		NetOutBytesPerSecond:    demoFloat64Ptr(340_000),
		DiskReadBytesPerSecond:  demoFloat64Ptr(540_000),
		DiskWriteBytesPerSecond: demoFloat64Ptr(460_000),
	}
}

func demoFloat64Ptr(value float64) *float64 {
	return &value
}

func demoInt64Ptr(value int64) *int64 {
	return &value
}

func atoiSafe(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return value
		}
		value = value*10 + int(r-'0')
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
