package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func newKubernetesTestMonitor() *Monitor {
	return &Monitor{
		state:                     models.NewState(),
		config:                    &config.Config{},
		removedKubernetesClusters: make(map[string]time.Time),
		kubernetesTokenBindings:   make(map[string]string),
	}
}

func TestNormalizeKubernetesClusterIdentifier(t *testing.T) {
	report := agentsk8s.Report{
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1"},
		Agent:   agentsk8s.AgentInfo{ID: "agent-1"},
	}
	if got := normalizeKubernetesClusterIdentifier(report); got != "cluster-1" {
		t.Fatalf("unexpected identifier: %s", got)
	}

	report.Cluster.ID = ""
	if got := normalizeKubernetesClusterIdentifier(report); got != "agent-1" {
		t.Fatalf("unexpected identifier: %s", got)
	}

	report.Agent.ID = ""
	report.Cluster.Server = "https://server"
	report.Cluster.Context = "ctx"
	report.Cluster.Name = "name"
	stableKey := "https://server|ctx|name"
	sum := sha256.Sum256([]byte(stableKey))
	expected := hex.EncodeToString(sum[:])
	if got := normalizeKubernetesClusterIdentifier(report); got != expected {
		t.Fatalf("unexpected hashed identifier: %s", got)
	}

	report.Cluster.Server = ""
	report.Cluster.Context = ""
	report.Cluster.Name = ""
	if got := normalizeKubernetesClusterIdentifier(report); got != "" {
		t.Fatalf("expected empty identifier, got %s", got)
	}
}

func TestApplyKubernetesReport(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "agent-1", IntervalSeconds: 10},
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1", Name: "cluster"},
	}

	cluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.ID != "cluster-1" || cluster.DisplayName != "cluster" {
		t.Fatalf("unexpected cluster: %+v", cluster)
	}
	if !monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"] {
		t.Fatal("expected connection health to be true")
	}

	monitor.removedKubernetesClusters["cluster-2"] = time.Now()
	report.Cluster.ID = "cluster-2"
	if _, err := monitor.ApplyKubernetesReport(report, nil); err == nil {
		t.Fatal("expected error for removed cluster")
	}

	token := &config.APITokenRecord{ID: "token-1", Name: "Token"}
	monitor.kubernetesTokenBindings["token-1"] = "other-agent"
	report.Cluster.ID = "cluster-3"
	report.Agent.ID = "agent-1"
	if _, err := monitor.ApplyKubernetesReport(report, token); err == nil {
		t.Fatal("expected error for token bound to different agent")
	}
}

func TestApplyKubernetesReportPreservesNativeAPIInventory(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	allowExpansion := true
	automountToken := true
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "agent-1", IntervalSeconds: 10},
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1", Name: "cluster"},
		Namespaces: []agentsk8s.Namespace{{
			UID:       "ns-1",
			Name:      " services ",
			Phase:     " Active ",
			CreatedAt: now,
			Labels:    map[string]string{"team": "checkout"},
		}},
		ReplicaSets: []agentsk8s.ReplicaSet{{UID: "rs-1", Name: "checkout-6d4", Namespace: "services", DesiredReplicas: 3, ReadyReplicas: 2, FullyLabeledReplicas: 3, ObservedGeneration: 7, OwnerKind: "Deployment", OwnerName: "checkout"}},
		Services: []agentsk8s.Service{{
			UID:         "svc-1",
			Name:        " checkout ",
			Namespace:   " services ",
			Type:        " ClusterIP ",
			ClusterIP:   "10.96.0.10",
			ExternalIPs: []string{"192.0.2.10"},
			Ports:       []agentsk8s.ServicePort{{Name: " http ", Protocol: " TCP ", Port: 80, TargetPort: "8080"}},
			Selector:    map[string]string{"app": "checkout"},
			CreatedAt:   now,
		}},
		Deployments: []agentsk8s.Deployment{{
			UID:                "deploy-1",
			Name:               "checkout-api",
			Namespace:          "services",
			CreatedAt:          now,
			DesiredReplicas:    4,
			UpdatedReplicas:    3,
			ReadyReplicas:      2,
			AvailableReplicas:  2,
			ObservedGeneration: 12,
			Labels:             map[string]string{"app": "checkout"},
		}},
		StatefulSets: []agentsk8s.StatefulSet{{UID: "sts-1", Name: "db", Namespace: "services", DesiredReplicas: 3, ReadyReplicas: 2, ServiceName: "db-headless"}},
		DaemonSets:   []agentsk8s.DaemonSet{{UID: "ds-1", Name: "node-agent", Namespace: "kube-system", DesiredNumberScheduled: 3, NumberReady: 3}},
		Jobs:         []agentsk8s.Job{{UID: "job-1", Name: "backup", Namespace: "services", DesiredCompletions: 1, Succeeded: 1}},
		CronJobs:     []agentsk8s.CronJob{{UID: "cron-1", Name: "nightly", Namespace: "services", Schedule: "0 1 * * *"}},
		Ingresses:    []agentsk8s.Ingress{{UID: "ing-1", Name: "checkout", Namespace: "services", ClassName: "nginx", Hosts: []string{"checkout.example.test"}, Addresses: []string{"192.0.2.20"}, CreatedAt: now}},
		EndpointSlices: []agentsk8s.EndpointSlice{{
			UID: "eps-1", Name: "checkout-abc", Namespace: "services", AddressType: "IPv4", ServiceName: "checkout", EndpointCount: 3, ReadyEndpointCount: 2, Ports: []agentsk8s.EndpointPort{{Name: "http", Protocol: "TCP", Port: 80, AppProtocol: "kubernetes.io/http"}},
		}},
		NetworkPolicies: []agentsk8s.NetworkPolicy{{UID: "netpol-1", Name: "checkout-deny", Namespace: "services", PolicyTypes: []string{"Ingress", "Egress"}, IngressRuleCount: 1, EgressRuleCount: 2}},
		PersistentVolumes: []agentsk8s.PersistentVolume{{
			UID: "pv-1", Name: "pv-checkout", Phase: " Bound ", StorageClass: "fast", CapacityBytes: 10_000, AccessModes: []string{"ReadWriteOnce"}, ReclaimPolicy: "Retain", ClaimNamespace: "services", ClaimName: "checkout-data", CreatedAt: now,
		}},
		PersistentVolumeClaims: []agentsk8s.PersistentVolumeClaim{{
			UID: "pvc-1", Name: "checkout-data", Namespace: "services", Phase: " Bound ", StorageClass: "fast", RequestedBytes: 10_000, CapacityBytes: 10_000, AccessModes: []string{"ReadWriteOnce"}, VolumeName: "pv-checkout", CreatedAt: now,
		}},
		StorageClasses:      []agentsk8s.StorageClass{{UID: "sc-1", Name: "fast", Provisioner: "csi.example.test", ReclaimPolicy: "Delete", VolumeBindingMode: "WaitForFirstConsumer", AllowVolumeExpansion: &allowExpansion, ParameterKeys: []string{"type"}}},
		ConfigMaps:          []agentsk8s.ConfigMap{{UID: "cm-1", Name: "checkout-config", Namespace: "services", DataKeys: []string{"app.yaml"}, BinaryDataKeys: []string{"logo.png"}, Immutable: true, MetadataOnly: true}},
		Secrets:             []agentsk8s.Secret{{UID: "sec-1", Name: "checkout-secret", Namespace: "services", Type: "Opaque", DataKeys: []string{"token"}, Immutable: true, MetadataOnly: true}},
		ServiceAccounts:     []agentsk8s.ServiceAccount{{UID: "sa-1", Name: "checkout", Namespace: "services", AutomountServiceAccountToken: &automountToken, SecretCount: 1, ImagePullSecrets: []string{"pull-secret"}}},
		Roles:               []agentsk8s.Role{{UID: "role-1", Name: "checkout-runtime", Namespace: "services", RuleCount: 4}},
		ClusterRoles:        []agentsk8s.ClusterRole{{UID: "crole-1", Name: "platform-monitoring", RuleCount: 12, AggregationLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"}}},
		RoleBindings:        []agentsk8s.RoleBinding{{UID: "rb-1", Name: "checkout-runtime", Namespace: "services", RoleKind: "Role", RoleName: "checkout-runtime", SubjectCount: 2, SubjectKinds: []string{"Group", "ServiceAccount"}}},
		ClusterRoleBindings: []agentsk8s.ClusterRoleBinding{{UID: "crb-1", Name: "platform-monitoring", RoleKind: "ClusterRole", RoleName: "platform-monitoring", SubjectCount: 3, SubjectKinds: []string{"Group", "ServiceAccount", "User"}}},
		ResourceQuotas:      []agentsk8s.ResourceQuota{{UID: "rq-1", Name: "services-quota", Namespace: "services", Hard: map[string]string{"pods": "10"}, Used: map[string]string{"pods": "3"}}},
		LimitRanges:         []agentsk8s.LimitRange{{UID: "lr-1", Name: "services-limits", Namespace: "services", LimitTypes: []string{"Container"}}},
		PodDisruptionBudgets: []agentsk8s.PodDisruptionBudget{{
			UID: "pdb-1", Name: "checkout-pdb", Namespace: "services", MinAvailable: "1", DesiredHealthy: 1, CurrentHealthy: 1, DisruptionsAllowed: 1, ExpectedPods: 2,
		}},
		HorizontalPodAutoscalers: []agentsk8s.HorizontalPodAutoscaler{{
			UID: "hpa-1", Name: "checkout-hpa", Namespace: "services", TargetKind: "Deployment", TargetName: "checkout", MinReplicas: 2, MaxReplicas: 10, CurrentReplicas: 2, DesiredReplicas: 3, MetricTypes: []string{"Resource:cpu"},
		}},
		Events:    []agentsk8s.Event{{UID: "evt-1", Name: "checkout.1", Namespace: "services", Type: "Warning", Reason: "BackOff", Message: "retrying", InvolvedKind: "Pod", InvolvedName: "checkout-0", Count: 2}},
		Timestamp: now,
	}

	cluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport: %v", err)
	}
	if len(cluster.Namespaces) != 1 || cluster.Namespaces[0].Name != "services" {
		t.Fatalf("expected namespace inventory, got %+v", cluster.Namespaces)
	}
	if len(cluster.Services) != 1 || cluster.Services[0].ServiceType != "ClusterIP" || cluster.Services[0].Ports[0].TargetPort != "8080" {
		t.Fatalf("expected service inventory, got %+v", cluster.Services)
	}
	if len(cluster.ReplicaSets) != 1 || cluster.ReplicaSets[0].OwnerName != "checkout" || cluster.ReplicaSets[0].FullyLabeledReplicas != 3 {
		t.Fatalf("expected replicaset inventory, got %+v", cluster.ReplicaSets)
	}
	if len(cluster.Deployments) != 1 || cluster.Deployments[0].CreatedAt.IsZero() || cluster.Deployments[0].ObservedGeneration != 12 {
		t.Fatalf("expected deployment API metadata inventory, got %+v", cluster.Deployments)
	}
	if len(cluster.StatefulSets) != 1 || cluster.StatefulSets[0].ReadyReplicas != 2 {
		t.Fatalf("expected statefulset inventory, got %+v", cluster.StatefulSets)
	}
	if len(cluster.DaemonSets) != 1 || cluster.DaemonSets[0].NumberReady != 3 {
		t.Fatalf("expected daemonset inventory, got %+v", cluster.DaemonSets)
	}
	if len(cluster.Jobs) != 1 || cluster.Jobs[0].Succeeded != 1 {
		t.Fatalf("expected job inventory, got %+v", cluster.Jobs)
	}
	if len(cluster.CronJobs) != 1 || cluster.CronJobs[0].Schedule != "0 1 * * *" {
		t.Fatalf("expected cronjob inventory, got %+v", cluster.CronJobs)
	}
	if len(cluster.Ingresses) != 1 || cluster.Ingresses[0].Hosts[0] != "checkout.example.test" {
		t.Fatalf("expected ingress inventory, got %+v", cluster.Ingresses)
	}
	if len(cluster.EndpointSlices) != 1 || cluster.EndpointSlices[0].ServiceName != "checkout" || cluster.EndpointSlices[0].ReadyEndpointCount != 2 {
		t.Fatalf("expected endpoint slice inventory, got %+v", cluster.EndpointSlices)
	}
	if len(cluster.NetworkPolicies) != 1 || cluster.NetworkPolicies[0].EgressRuleCount != 2 {
		t.Fatalf("expected network policy inventory, got %+v", cluster.NetworkPolicies)
	}
	if len(cluster.PersistentVolumes) != 1 || cluster.PersistentVolumes[0].ClaimName != "checkout-data" {
		t.Fatalf("expected PV inventory, got %+v", cluster.PersistentVolumes)
	}
	if len(cluster.PersistentVolumeClaims) != 1 || cluster.PersistentVolumeClaims[0].VolumeName != "pv-checkout" {
		t.Fatalf("expected PVC inventory, got %+v", cluster.PersistentVolumeClaims)
	}
	if len(cluster.StorageClasses) != 1 || cluster.StorageClasses[0].Provisioner != "csi.example.test" || cluster.StorageClasses[0].AllowVolumeExpansion == nil || !*cluster.StorageClasses[0].AllowVolumeExpansion {
		t.Fatalf("expected storage class inventory, got %+v", cluster.StorageClasses)
	}
	if len(cluster.ConfigMaps) != 1 || cluster.ConfigMaps[0].DataKeys[0] != "app.yaml" || !cluster.ConfigMaps[0].Immutable || !cluster.ConfigMaps[0].MetadataOnly {
		t.Fatalf("expected configmap inventory, got %+v", cluster.ConfigMaps)
	}
	if len(cluster.Secrets) != 1 || cluster.Secrets[0].Type != "Opaque" || cluster.Secrets[0].DataKeys[0] != "token" || !cluster.Secrets[0].Immutable || !cluster.Secrets[0].MetadataOnly {
		t.Fatalf("expected secret metadata inventory, got %+v", cluster.Secrets)
	}
	if len(cluster.ServiceAccounts) != 1 || cluster.ServiceAccounts[0].SecretCount != 1 || cluster.ServiceAccounts[0].AutomountServiceAccountToken == nil || !*cluster.ServiceAccounts[0].AutomountServiceAccountToken {
		t.Fatalf("expected serviceaccount inventory, got %+v", cluster.ServiceAccounts)
	}
	if len(cluster.Roles) != 1 || cluster.Roles[0].Name != "checkout-runtime" || cluster.Roles[0].RuleCount != 4 {
		t.Fatalf("expected role inventory, got %+v", cluster.Roles)
	}
	if len(cluster.ClusterRoles) != 1 || cluster.ClusterRoles[0].RuleCount != 12 || cluster.ClusterRoles[0].AggregationLabels["rbac.authorization.k8s.io/aggregate-to-admin"] != "true" {
		t.Fatalf("expected clusterrole inventory, got %+v", cluster.ClusterRoles)
	}
	if len(cluster.RoleBindings) != 1 || cluster.RoleBindings[0].RoleKind != "Role" || cluster.RoleBindings[0].SubjectCount != 2 || len(cluster.RoleBindings[0].SubjectKinds) != 2 {
		t.Fatalf("expected rolebinding inventory, got %+v", cluster.RoleBindings)
	}
	if len(cluster.ClusterRoleBindings) != 1 || cluster.ClusterRoleBindings[0].RoleKind != "ClusterRole" || cluster.ClusterRoleBindings[0].SubjectCount != 3 {
		t.Fatalf("expected clusterrolebinding inventory, got %+v", cluster.ClusterRoleBindings)
	}
	if len(cluster.ResourceQuotas) != 1 || cluster.ResourceQuotas[0].Hard["pods"] != "10" || cluster.ResourceQuotas[0].Used["pods"] != "3" {
		t.Fatalf("expected resourcequota inventory, got %+v", cluster.ResourceQuotas)
	}
	if len(cluster.LimitRanges) != 1 || cluster.LimitRanges[0].LimitTypes[0] != "Container" {
		t.Fatalf("expected limitrange inventory, got %+v", cluster.LimitRanges)
	}
	if len(cluster.PodDisruptionBudgets) != 1 || cluster.PodDisruptionBudgets[0].ExpectedPods != 2 || cluster.PodDisruptionBudgets[0].MinAvailable != "1" {
		t.Fatalf("expected poddisruptionbudget inventory, got %+v", cluster.PodDisruptionBudgets)
	}
	if len(cluster.HorizontalPodAutoscalers) != 1 || cluster.HorizontalPodAutoscalers[0].TargetName != "checkout" || cluster.HorizontalPodAutoscalers[0].MetricTypes[0] != "Resource:cpu" {
		t.Fatalf("expected horizontalpodautoscaler inventory, got %+v", cluster.HorizontalPodAutoscalers)
	}
	if len(cluster.Events) != 1 || cluster.Events[0].Reason != "BackOff" || cluster.Events[0].Count != 2 {
		t.Fatalf("expected event inventory, got %+v", cluster.Events)
	}
}

func TestApplyKubernetesReport_PodNetworkAndEphemeralMetrics(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.rateTracker = NewRateTracker()
	monitor.metricsHistory = NewMetricsHistory(10, time.Hour)

	baseTime := time.Now().Add(-20 * time.Second).UTC()
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "agent-1", IntervalSeconds: 10},
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1", Name: "cluster"},
		Nodes: []agentsk8s.Node{
			{
				Name: "node-a",
				Capacity: agentsk8s.NodeResources{
					CPUCores:    4,
					MemoryBytes: 16 * 1024 * 1024 * 1024,
				},
				Allocatable: agentsk8s.NodeResources{
					CPUCores:    4,
					MemoryBytes: 16 * 1024 * 1024 * 1024,
				},
			},
		},
		Pods: []agentsk8s.Pod{
			{
				UID:       "pod-1",
				Name:      "api-0",
				Namespace: "default",
				NodeName:  "node-a",
				Phase:     "Running",
				Usage: &agentsk8s.PodUsage{
					CPUMilliCores:                 400,
					MemoryBytes:                   1024 * 1024 * 1024,
					NetworkRxBytes:                1000,
					NetworkTxBytes:                2000,
					EphemeralStorageUsedBytes:     500 * 1024 * 1024,
					EphemeralStorageCapacityBytes: 2 * 1024 * 1024 * 1024,
				},
			},
		},
		Timestamp: baseTime,
	}

	firstCluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport(first): %v", err)
	}
	if len(firstCluster.Pods) != 1 {
		t.Fatalf("expected one pod, got %d", len(firstCluster.Pods))
	}

	firstPod := firstCluster.Pods[0]
	if firstPod.DiskUsagePercent <= 0 {
		t.Fatalf("expected non-zero disk usage percent from ephemeral storage, got %f", firstPod.DiskUsagePercent)
	}
	if firstPod.NetInRate != 0 || firstPod.NetOutRate != 0 {
		t.Fatalf("expected first sample network rates to be zero, got in=%f out=%f", firstPod.NetInRate, firstPod.NetOutRate)
	}

	report.Timestamp = baseTime.Add(10 * time.Second)
	report.Pods[0].Usage.NetworkRxBytes = 61000
	report.Pods[0].Usage.NetworkTxBytes = 42000

	secondCluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport(second): %v", err)
	}
	secondPod := secondCluster.Pods[0]
	if secondPod.NetInRate <= 0 || secondPod.NetOutRate <= 0 {
		t.Fatalf("expected positive network rates, got in=%f out=%f", secondPod.NetInRate, secondPod.NetOutRate)
	}

	metricID := kubernetesPodMetricID(secondCluster, secondPod)
	if metricID == "" {
		t.Fatal("expected kubernetes pod metric id")
	}
	netInPoints := monitor.metricsHistory.GetGuestMetrics(metricID, "netin", time.Hour)
	if len(netInPoints) == 0 {
		t.Fatal("expected netin points to be recorded")
	}
	diskPoints := monitor.metricsHistory.GetGuestMetrics(metricID, "disk", time.Hour)
	if len(diskPoints) == 0 {
		t.Fatal("expected disk points to be recorded")
	}
}

func TestRemoveAndReenrollKubernetesCluster(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.kubernetesTokenBindings["token-1"] = "agent-1"
	monitor.config.APITokens = []config.APITokenRecord{{ID: "token-1"}}
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:          "cluster-1",
		Name:        "cluster",
		DisplayName: "cluster",
		TokenID:     "token-1",
		TokenName:   "Token",
	})
	monitor.state.SetConnectionHealth(kubernetesConnectionPrefix+"cluster-1", true)

	_, err := monitor.RemoveKubernetesCluster("cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(monitor.state.GetKubernetesClusters()) != 0 {
		t.Fatal("expected cluster removed")
	}
	if _, exists := monitor.kubernetesTokenBindings["token-1"]; exists {
		t.Fatal("expected token binding removed")
	}
	if _, exists := monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"]; exists {
		t.Fatal("expected connection health removed")
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 1 {
		t.Fatal("expected removed cluster entry")
	}

	if err := monitor.AllowKubernetesClusterReenroll("cluster-1"); err != nil {
		t.Fatalf("unexpected reenroll error: %v", err)
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 0 {
		t.Fatal("expected removed entry cleared")
	}
}

func TestKubernetesClusterUpdates(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:              "cluster-1",
		Name:            "cluster",
		LastSeen:        time.Now().Add(-10 * time.Second),
		Status:          "online",
		IntervalSeconds: 5,
	})
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:       "cluster-2",
		Name:     "cluster2",
		LastSeen: time.Now().Add(-10 * time.Hour),
		Status:   "online",
	})

	now := time.Now()
	monitor.evaluateKubernetesAgents(now)
	if monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"] != true {
		t.Fatal("expected cluster-1 healthy")
	}
	if monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-2"] != false {
		t.Fatal("expected cluster-2 unhealthy")
	}

	_, err := monitor.UnhideKubernetesCluster("cluster-1")
	if err != nil {
		t.Fatalf("unexpected unhide error: %v", err)
	}
	if _, err := monitor.MarkKubernetesClusterPendingUninstall("cluster-1"); err != nil {
		t.Fatalf("unexpected pending uninstall error: %v", err)
	}
	if _, err := monitor.SetKubernetesClusterCustomDisplayName("cluster-1", "custom"); err != nil {
		t.Fatalf("unexpected set display name error: %v", err)
	}
}

func TestCleanupRemovedKubernetesClusters(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.removedKubernetesClusters["cluster-1"] = time.Now().Add(-2 * removedKubernetesClustersTTL)
	monitor.state.AddRemovedKubernetesCluster(models.RemovedKubernetesCluster{
		ID:        "cluster-1",
		Name:      "cluster",
		RemovedAt: time.Now().Add(-2 * removedKubernetesClustersTTL),
	})

	monitor.cleanupRemovedKubernetesClusters(time.Now())
	if len(monitor.removedKubernetesClusters) != 0 {
		t.Fatal("expected removed clusters cleaned up")
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 0 {
		t.Fatal("expected state cleanup")
	}
}
