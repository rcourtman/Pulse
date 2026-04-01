package mock

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

type demoWorkloadProfile struct {
	Name       string
	Tags       []string
	BackupAge  time.Duration
	ForceState string
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

var demoVMProfiles = []demoWorkloadProfile{
	{Name: "checkout-web-01", Tags: []string{"production", "web", "customer-facing"}, BackupAge: 6 * time.Hour},
	{Name: "checkout-web-02", Tags: []string{"production", "web", "customer-facing"}, BackupAge: 7 * time.Hour},
	{Name: "orders-api-01", Tags: []string{"production", "api", "critical"}, BackupAge: 3 * time.Hour},
	{Name: "orders-api-02", Tags: []string{"production", "api", "critical"}, BackupAge: 4 * time.Hour},
	{Name: "postgres-primary-01", Tags: []string{"production", "database", "critical"}, BackupAge: 90 * time.Minute},
	{Name: "postgres-replica-01", Tags: []string{"production", "database", "replica"}, BackupAge: 2 * time.Hour},
	{Name: "redis-cache-01", Tags: []string{"production", "cache", "latency-sensitive"}, BackupAge: 12 * time.Hour},
	{Name: "observability-core-01", Tags: []string{"monitoring", "platform", "production"}, BackupAge: 10 * time.Hour},
	{Name: "ci-runner-01", Tags: []string{"ci", "platform", "shared-services"}, BackupAge: 18 * time.Hour},
	{Name: "analytics-batch-01", Tags: []string{"analytics", "batch", "internal"}, BackupAge: 26 * time.Hour},
	{Name: "erp-app-01", Tags: []string{"business", "api", "internal"}, BackupAge: 8 * time.Hour},
	{Name: "docs-portal-01", Tags: []string{"web", "internal", "docs"}, BackupAge: 14 * time.Hour},
}

var demoContainerProfiles = []demoWorkloadProfile{
	{Name: "edge-proxy-01", Tags: []string{"production", "ingress", "edge"}, BackupAge: 6 * time.Hour},
	{Name: "auth-service-01", Tags: []string{"production", "api", "security"}, BackupAge: 4 * time.Hour},
	{Name: "billing-worker-01", Tags: []string{"production", "queue", "worker"}, BackupAge: 8 * time.Hour},
	{Name: "reporting-api-01", Tags: []string{"internal", "api", "reporting"}, BackupAge: 10 * time.Hour},
	{Name: "minio-gateway-01", Tags: []string{"storage", "internal", "gateway"}, BackupAge: 12 * time.Hour},
	{Name: "wireguard-edge-01", Tags: []string{"edge", "security", "vpn"}, BackupAge: 8 * time.Hour},
	{Name: "docs-wiki-01", Tags: []string{"internal", "web", "docs"}, BackupAge: 14 * time.Hour},
	{Name: "smtp-relay-01", Tags: []string{"mail", "queue", "platform"}, BackupAge: 16 * time.Hour},
	{Name: "grafana-agent-01", Tags: []string{"monitoring", "agent", "platform"}, BackupAge: 11 * time.Hour},
	{Name: "artifact-cache-01", Tags: []string{"ci", "cache", "platform"}, BackupAge: 19 * time.Hour},
	{Name: "backup-orchestrator-01", Tags: []string{"backup", "platform", "worker"}, BackupAge: 45 * time.Minute},
	{Name: "dev-portal-01", Tags: []string{"development", "web", "internal"}, BackupAge: 20 * time.Hour},
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
}

var demoKubernetesNodeProfiles = []demoKubernetesNodeProfile{
	{Name: "prod-euw1-k8s-01", Roles: []string{"control-plane"}},
	{Name: "prod-euw1-k8s-02", Roles: []string{"worker"}},
	{Name: "prod-euw1-k8s-03", Roles: []string{"worker"}},
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

const demoProxmoxClusterName = "Core Fabric"

func applyDemoScenarioGraph(graph *FixtureGraph, now time.Time) {
	if graph == nil {
		return
	}

	applyDemoPlatformScenario(&graph.PlatformFixtures)
	applyDemoNodeScenario(&graph.State)
	vmNames := applyDemoWorkloadScenario(graph.State.VMs, demoVMProfiles, now)
	containerNames := applyDemoContainerScenario(graph.State.Containers, demoContainerProfiles, now)
	applyDemoDockerScenario(&graph.State, now)
	applyDemoKubernetesScenario(&graph.State, now)
	applyDemoStorageScenario(&graph.State)
	applyDemoBackupScenario(&graph.State, vmNames, containerNames, now)
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

	nodeDisplayNames := []string{
		"West Production A",
		"West Production B",
		"West Production C",
		"Disaster Recovery A",
		"Disaster Recovery B",
	}
	for i := range state.Nodes {
		state.Nodes[i].Instance = scenarioClusterAlias(state.Nodes[i].Instance)
		if state.Nodes[i].IsClusterMember || strings.TrimSpace(state.Nodes[i].ClusterName) != "" {
			state.Nodes[i].ClusterName = scenarioClusterAlias(state.Nodes[i].ClusterName)
		}
		if i < len(nodeDisplayNames) {
			state.Nodes[i].DisplayName = nodeDisplayNames[i]
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

func applyDemoWorkloadScenario(workloads []models.VM, profiles []demoWorkloadProfile, now time.Time) map[int]string {
	guestNames := make(map[int]string, len(workloads))
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
		if !now.IsZero() && profile.BackupAge > 0 {
			workloads[i].LastBackup = now.Add(-profile.BackupAge)
		}
		guestNames[workloads[i].VMID] = workloads[i].Name
	}
	return guestNames
}

func applyDemoContainerScenario(workloads []models.Container, profiles []demoWorkloadProfile, now time.Time) map[int]string {
	guestNames := make(map[int]string, len(workloads))
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
		if !now.IsZero() && profile.BackupAge > 0 {
			workloads[i].LastBackup = now.Add(-profile.BackupAge)
		}
		guestNames[workloads[i].VMID] = workloads[i].Name
	}
	return guestNames
}

func applyDemoDockerScenario(state *models.StateSnapshot, now time.Time) {
	if state == nil {
		return
	}

	sort.Slice(state.DockerHosts, func(i, j int) bool {
		return state.DockerHosts[i].ID < state.DockerHosts[j].ID
	})
	for i := range state.DockerHosts {
		hostProfile := demoDockerHostProfiles[i%len(demoDockerHostProfiles)]
		host := &state.DockerHosts[i]
		host.Hostname = hostProfile.Hostname
		host.DisplayName = hostProfile.DisplayName
		host.Status = "online"
		host.Containers = applyDemoDockerContainerProfiles(host.Containers, hostProfile.Containers, now)
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
		cluster.Name = "Production"
		cluster.DisplayName = "Production"
		cluster.Status = "online"

		nodeNameMap := make(map[string]string, len(cluster.Nodes))
		for i := range cluster.Nodes {
			profile := demoKubernetesNodeProfiles[i%len(demoKubernetesNodeProfiles)]
			oldName := cluster.Nodes[i].Name
			cluster.Nodes[i].Name = profile.Name
			cluster.Nodes[i].Ready = true
			cluster.Nodes[i].Unschedulable = false
			cluster.Nodes[i].Roles = append([]string(nil), profile.Roles...)
			nodeNameMap[oldName] = profile.Name
		}

		for i := range state.Hosts {
			host := &state.Hosts[i]
			if renamed, ok := nodeNameMap[strings.TrimSpace(host.Hostname)]; ok {
				host.Hostname = renamed
				host.DisplayName = humanizeHostDisplayName(renamed)
			}
		}

		for i := range cluster.Deployments {
			profile := demoKubernetesDeploymentProfiles[i%len(demoKubernetesDeploymentProfiles)]
			cluster.Deployments[i].Name = profile.Name
			cluster.Deployments[i].Namespace = profile.Namespace
			cluster.Deployments[i].DesiredReplicas = profile.DesiredReplicas
			cluster.Deployments[i].UpdatedReplicas = profile.ReadyReplicas
			cluster.Deployments[i].ReadyReplicas = profile.ReadyReplicas
			cluster.Deployments[i].AvailableReplicas = profile.ReadyReplicas
			cluster.Deployments[i].Labels = map[string]string{
				"app.kubernetes.io/name": profile.Name,
				"cluster":                cluster.ID,
				"environment":            "production",
			}
		}

		for i := range cluster.Pods {
			profile := demoKubernetesPodProfiles[i%len(demoKubernetesPodProfiles)]
			pod := &cluster.Pods[i]
			pod.Name = profile.Name
			pod.Namespace = profile.Namespace
			if profile.NodeIndex >= 0 && profile.NodeIndex < len(cluster.Nodes) {
				pod.NodeName = cluster.Nodes[profile.NodeIndex].Name
			} else if renamed, ok := nodeNameMap[pod.NodeName]; ok {
				pod.NodeName = renamed
			}
			pod.OwnerKind = profile.OwnerKind
			pod.OwnerName = profile.OwnerName
			pod.Phase = profile.Phase
			pod.Reason = profile.Reason
			pod.Message = profile.Message
			pod.Restarts = int(profile.Restarts)
			if len(pod.Containers) == 0 {
				pod.Containers = []models.KubernetesPodContainer{{}}
			}
			for j := range pod.Containers {
				pod.Containers[j].Name = profile.Container
				pod.Containers[j].Image = profile.Image
				pod.Containers[j].RestartCount = profile.Restarts
				if profile.ContainerOK {
					pod.Containers[j].Ready = true
					pod.Containers[j].State = "running"
					pod.Containers[j].Reason = ""
					pod.Containers[j].Message = ""
				} else {
					pod.Containers[j].Ready = false
					switch strings.ToLower(strings.TrimSpace(profile.Phase)) {
					case "pending":
						pod.Containers[j].State = "waiting"
						pod.Containers[j].Reason = "PodInitializing"
					default:
						pod.Containers[j].State = "waiting"
						pod.Containers[j].Reason = "CrashLoopBackOff"
					}
					pod.Containers[j].Message = profile.Message
				}
			}
			pod.Labels = map[string]string{
				"app.kubernetes.io/name":     profile.OwnerName,
				"app.kubernetes.io/part-of":  "pulse-demo-estate",
				"app.kubernetes.io/instance": profile.OwnerName,
			}
		}

		initializeMockKubernetesClusterUsage(cluster, now, false)
	}

	syncMockKubernetesNodeHosts(state)
}

func applyDemoStorageScenario(state *models.StateSnapshot) {
	if state == nil {
		return
	}

	for i := range state.Storage {
		storage := &state.Storage[i]
		if name := strings.TrimSpace(storageScenarioAlias(*storage)); name != "" {
			storage.Name = name
		}
		storage.Instance = scenarioClusterAlias(storage.Instance)
	}

	for i := range state.PBSInstances {
		state.PBSInstances[i].Name = []string{"backup-vault", "dr-vault"}[minInt(i, 1)]
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
		} else {
			state.PMGInstances[i].Name = "mail-gateway-us"
		}
	}
}

func applyDemoBackupScenario(
	state *models.StateSnapshot,
	vmNames map[int]string,
	containerNames map[int]string,
	now time.Time,
) {
	if state == nil {
		return
	}

	nameForBackup := func(backupType string, vmid int) string {
		switch strings.ToLower(strings.TrimSpace(backupType)) {
		case "qemu", "vm":
			return vmNames[vmid]
		case "lxc", "ct":
			return containerNames[vmid]
		default:
			return ""
		}
	}

	for i := range state.PVEBackups.StorageBackups {
		backup := &state.PVEBackups.StorageBackups[i]
		if guestName := nameForBackup(backup.Type, backup.VMID); guestName != "" {
			backup.Notes = fmt.Sprintf("Backup of %s", guestName)
		}
		if backup.Storage == "local" {
			backup.Storage = "backup-vault"
		}
	}

	for i := range state.PBSBackups {
		backup := &state.PBSBackups[i]
		if backup.Instance == "pbs-main" {
			backup.Instance = "backup-vault"
		} else if backup.Instance == "pbs-secondary" {
			backup.Instance = "dr-vault"
		}
		if backup.Datastore == "backup-store" {
			backup.Datastore = "primary-vault"
		} else if backup.Datastore == "offsite-backup" {
			backup.Datastore = "offsite-vault"
		} else if backup.Datastore == "replica-store" {
			backup.Datastore = "replica-vault"
		}
		if guestName := nameForBackup(backup.BackupType, atoiSafe(backup.VMID)); guestName != "" {
			backup.Comment = fmt.Sprintf("Protected recovery point for %s", guestName)
		}
	}

	for i := range state.PVEBackups.GuestSnapshots {
		snapshot := &state.PVEBackups.GuestSnapshots[i]
		if guestName := nameForBackup(snapshot.Type, snapshot.VMID); guestName != "" {
			snapshot.Description = fmt.Sprintf("%s snapshot", guestName)
		}
	}

	for i := range state.ReplicationJobs {
		job := &state.ReplicationJobs[i]
		if guestName := vmNames[job.GuestID]; guestName != "" {
			job.GuestName = guestName
		}
		job.SourceStorage = scenarioStorageAliasForNode(job.SourceStorage, job.SourceNode)
		job.TargetStorage = scenarioStorageAliasForNode(job.TargetStorage, job.TargetNode)
	}

	for i := range state.VMs {
		if state.VMs[i].LastBackup.IsZero() {
			state.VMs[i].LastBackup = now.Add(-12 * time.Hour)
		}
	}
	for i := range state.Containers {
		if state.Containers[i].LastBackup.IsZero() {
			state.Containers[i].LastBackup = now.Add(-18 * time.Hour)
		}
	}

	state.PMGBackups = extractPMGBackups(state.PVEBackups.StorageBackups)
	state.Backups.PVE = state.PVEBackups
	state.Backups.PBS = append([]models.PBSBackup(nil), state.PBSBackups...)
	state.Backups.PMG = append([]models.PMGBackup(nil), state.PMGBackups...)
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
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "", "standalone":
		return strings.TrimSpace(name)
	case "mock-cluster":
		return demoProxmoxClusterName
	default:
		return name
	}
}

func scenarioStorageAlias(name string) string {
	return scenarioStorageAliasForNode(name, "")
}

func storageScenarioAlias(storage models.Storage) string {
	return scenarioStorageAliasForNode(storage.Name, storage.Node)
}

func scenarioStorageAliasForNode(name, node string) string {
	trimmedName := strings.TrimSpace(name)
	normalizedName := strings.ToLower(trimmedName)
	normalizedNode := strings.ToLower(strings.TrimSpace(node))
	switch normalizedName {
	case "shared-storage":
		return "shared-backup-fabric"
	case "pbs-pve1":
		return "backup-vault-a"
	case "pbs-pve2":
		return "backup-vault-b"
	case "pbs-pve3":
		return "backup-vault-c"
	case "local":
		switch normalizedNode {
		case "pve1":
			return "west-a-iso-library"
		case "pve2":
			return "west-b-iso-library"
		case "pve3":
			return "west-c-iso-library"
		case "pve4":
			return "dr-a-iso-library"
		case "pve5":
			return "dr-b-iso-library"
		default:
			return "iso-library"
		}
	case "local-zfs":
		switch normalizedNode {
		case "pve1":
			return "west-a-service-pool"
		case "pve2":
			return "west-b-service-pool"
		case "pve3":
			return "west-c-service-pool"
		case "pve4":
			return "dr-a-service-pool"
		case "pve5":
			return "dr-b-service-pool"
		default:
			return "service-pool"
		}
	default:
		return trimmedName
	}
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
