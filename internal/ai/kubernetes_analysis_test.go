package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// ========================================
// truncateKubernetesMessage tests
// ========================================

func TestTruncateKubernetesMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short message",
			input:    "This is a short message",
			expected: "This is a short message",
		},
		{
			name:     "empty message",
			input:    "",
			expected: "",
		},
		{
			name:     "max length message",
			input:    string(make([]byte, maxKubernetesMessageLength)),
			expected: string(make([]byte, maxKubernetesMessageLength)),
		},
		{
			name:     "over max length",
			input:    string(make([]byte, maxKubernetesMessageLength+50)),
			expected: string(make([]byte, maxKubernetesMessageLength)) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateKubernetesMessage(tt.input)
			if result != tt.expected {
				if len(result) < 100 {
					t.Errorf("truncateKubernetesMessage() = %q, want %q", result, tt.expected)
				} else {
					t.Errorf("truncateKubernetesMessage() length = %d, want %d", len(result), len(tt.expected))
				}
			}
		})
	}
}

// ========================================
// formatKubernetesAge tests
// ========================================

func TestFormatKubernetesAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
			expected: "5m",
		},
		{
			name:     "59 minutes",
			duration: 59 * time.Minute,
			expected: "59m",
		},
		{
			name:     "1 hour",
			duration: time.Hour,
			expected: "1h",
		},
		{
			name:     "23 hours",
			duration: 23 * time.Hour,
			expected: "23h",
		},
		{
			name:     "1 day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: "7d",
		},
		{
			name:     "negative duration",
			duration: -5 * time.Second,
			expected: "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKubernetesAge(tt.duration)
			if result != tt.expected {
				t.Errorf("formatKubernetesAge(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

// ========================================
// isK8sPodHealthy tests
// ========================================

func TestIsK8sPodHealthy(t *testing.T) {
	tests := []struct {
		name       string
		phase      string
		containers []unifiedresources.K8sPodContainer
		expected   bool
	}{
		{
			name:     "empty phase",
			phase:    "",
			expected: false,
		},
		{
			name:     "pending phase",
			phase:    "Pending",
			expected: false,
		},
		{
			name:     "running phase, no containers",
			phase:    "Running",
			expected: true,
		},
		{
			name:  "running phase, healthy container",
			phase: "Running",
			containers: []unifiedresources.K8sPodContainer{
				{Name: "app", Ready: true, State: "running"},
			},
			expected: true,
		},
		{
			name:  "running phase, not ready container",
			phase: "Running",
			containers: []unifiedresources.K8sPodContainer{
				{Name: "app", Ready: false, State: "running"},
			},
			expected: false,
		},
		{
			name:  "running phase, container not running",
			phase: "Running",
			containers: []unifiedresources.K8sPodContainer{
				{Name: "app", Ready: true, State: "waiting"},
			},
			expected: false,
		},
		{
			name:     "failed phase",
			phase:    "Failed",
			expected: false,
		},
		{
			name:     "succeeded phase",
			phase:    "Succeeded",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := makePodView(t, tt.phase, tt.containers)
			result := isK8sPodHealthy(pod)
			if result != tt.expected {
				t.Errorf("isK8sPodHealthy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========================================
// k8sPodReason tests
// ========================================

func TestK8sPodReason(t *testing.T) {
	tests := []struct {
		name     string
		pod      models.KubernetesPod
		contains []string
	}{
		{
			name:     "empty pod",
			pod:      models.KubernetesPod{},
			contains: []string{},
		},
		{
			name:     "pod with phase",
			pod:      models.KubernetesPod{Phase: "Pending"},
			contains: []string{"phase=Pending"},
		},
		{
			name:     "pod with reason",
			pod:      models.KubernetesPod{Phase: "Failed", Reason: "OOMKilled"},
			contains: []string{"phase=Failed", "reason=OOMKilled"},
		},
		{
			name:     "pod with restarts",
			pod:      models.KubernetesPod{Phase: "Running", Restarts: 5},
			contains: []string{"restarts=5"},
		},
		{
			name: "pod with container issue",
			pod: models.KubernetesPod{
				Phase: "Running",
				Containers: []models.KubernetesPodContainer{
					{Name: "app", Ready: false, State: "waiting", Reason: "CrashLoopBackOff"},
				},
			},
			contains: []string{"containers=app"},
		},
		{
			name: "pod with message and multiple containers",
			pod: models.KubernetesPod{
				Phase:    "Failed",
				Message:  strings.Repeat("x", maxKubernetesMessageLength+10),
				Restarts: 3,
				Containers: []models.KubernetesPodContainer{
					{Name: "ok", Ready: true, State: "running"},
					{Name: "one", Ready: false, State: "waiting"},
					{Name: "two", Ready: false, State: "terminated"},
					{Name: "three", Ready: false, State: "waiting"},
					{Name: "four", Ready: false, State: "waiting"},
				},
			},
			contains: []string{"message=", "containers=", "restarts=3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podView := makePodViewFromModel(t, tt.pod)
			result := k8sPodReason(podView)
			for _, expected := range tt.contains {
				if len(expected) > 0 && !kubeTestContains(result, expected) {
					t.Errorf("k8sPodReason() = %q, want to contain %q", result, expected)
				}
			}
		})
	}
}

func kubeTestContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========================================
// isK8sDeploymentHealthy tests
// ========================================

func TestIsK8sDeploymentHealthy(t *testing.T) {
	tests := []struct {
		name                               string
		desired, ready, available, updated int32
		expected                           bool
	}{
		{"empty deployment", 0, 0, 0, 0, true},
		{"all replicas ready", 3, 3, 3, 3, true},
		{"some replicas not ready", 3, 2, 3, 3, false},
		{"no replicas ready", 3, 0, 0, 0, false},
		{"not all available", 3, 3, 2, 3, false},
		{"not all updated", 3, 3, 3, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := makeDeploymentView(t, tt.desired, tt.ready, tt.available, tt.updated)
			result := isK8sDeploymentHealthy(view)
			if result != tt.expected {
				t.Errorf("isK8sDeploymentHealthy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========================================
// formatPodIssueLine tests
// ========================================

func TestFormatPodIssueLine(t *testing.T) {
	tests := []struct {
		name     string
		issue    podIssue
		expected string
	}{
		{
			name: "with namespace and name only",
			issue: podIssue{
				namespace: "default",
				name:      "nginx-pod",
			},
			expected: "default/nginx-pod",
		},
		{
			name: "with reason",
			issue: podIssue{
				namespace: "kube-system",
				name:      "coredns-pod",
				reason:    "CrashLoopBackOff",
			},
			expected: "kube-system/coredns-pod CrashLoopBackOff",
		},
		{
			name: "empty reason",
			issue: podIssue{
				namespace: "production",
				name:      "api-server",
				reason:    "",
			},
			expected: "production/api-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPodIssueLine(tt.issue)
			if result != tt.expected {
				t.Errorf("formatPodIssueLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ========================================
// formatPodIssues tests
// ========================================

func TestFormatPodIssues(t *testing.T) {
	issues := []podIssue{
		{namespace: "ns1", name: "pod1", reason: "reason1"},
		{namespace: "ns2", name: "pod2", reason: "reason2"},
		{namespace: "ns3", name: "pod3", reason: "reason3"},
		{namespace: "ns4", name: "pod4", reason: "reason4"},
	}

	t.Run("within limit", func(t *testing.T) {
		result := formatPodIssues(issues, 10)
		if len(result) != 4 {
			t.Errorf("Expected 4 lines, got %d", len(result))
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		result := formatPodIssues(issues, 2)
		if len(result) != 3 { // 2 issues + "... and X more"
			t.Errorf("Expected 3 lines (2 issues + truncation notice), got %d", len(result))
		}
		if !kubeTestContains(result[2], "and 2 more") {
			t.Errorf("Expected truncation notice, got %q", result[2])
		}
	})

	t.Run("empty issues", func(t *testing.T) {
		result := formatPodIssues([]podIssue{}, 10)
		if len(result) != 0 {
			t.Errorf("Expected 0 lines for empty issues, got %d", len(result))
		}
	})
}

// ========================================
// formatPodRestarts tests
// ========================================

func TestFormatPodRestarts(t *testing.T) {
	t.Run("empty issues", func(t *testing.T) {
		result := formatPodRestarts(nil, 10)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("sorts by restarts descending", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 5},
			{namespace: "ns2", name: "pod2", restarts: 10},
			{namespace: "ns3", name: "pod3", restarts: 2},
		}
		result := formatPodRestarts(issues, 10)
		if len(result) != 3 {
			t.Errorf("Expected 3 lines, got %d", len(result))
		}
		// First should be pod2 (10 restarts)
		if !kubeTestContains(result[0], "restarts=10") {
			t.Errorf("Expected first to have 10 restarts, got %q", result[0])
		}
	})

	t.Run("with reason", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 5, reason: "OOMKilled"},
		}
		result := formatPodRestarts(issues, 10)
		if len(result) != 1 {
			t.Errorf("Expected 1 line, got %d", len(result))
		}
		if !kubeTestContains(result[0], "OOMKilled") {
			t.Errorf("Expected reason in output, got %q", result[0])
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns1", name: "pod1", restarts: 10},
			{namespace: "ns2", name: "pod2", restarts: 8},
			{namespace: "ns3", name: "pod3", restarts: 6},
			{namespace: "ns4", name: "pod4", restarts: 4},
		}
		result := formatPodRestarts(issues, 2)
		if len(result) != 2 {
			t.Errorf("Expected 2 lines (limit), got %d", len(result))
		}
	})

	t.Run("ties sort by name", func(t *testing.T) {
		issues := []podIssue{
			{namespace: "ns", name: "b-pod", restarts: 3},
			{namespace: "ns", name: "a-pod", restarts: 3},
		}
		result := formatPodRestarts(issues, 10)
		if len(result) != 2 {
			t.Fatalf("Expected 2 lines, got %d", len(result))
		}
		if !strings.Contains(result[0], "a-pod") {
			t.Fatalf("Expected tie-breaker to sort by name, got %q", result[0])
		}
	})
}

// ========================================
// findK8sCluster tests
// ========================================

func TestFindK8sCluster(t *testing.T) {
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "cluster-1", Name: "one"},
			{ID: "cluster-2", Name: "two"},
		},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "cluster-2")
	if cluster == nil || cluster.ClusterID() != "cluster-2" {
		t.Fatalf("expected to find cluster-2, got %v", cluster)
	}

	if missing := findK8sCluster(reg.K8sClusters(), "missing"); missing != nil {
		t.Fatal("expected missing cluster to return nil")
	}
}

// ========================================
// summarizeK8sNodes tests
// ========================================

func TestSummarizeK8sNodes(t *testing.T) {
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Nodes: []models.KubernetesNode{
				{Name: "node-1", Ready: true},
				{Name: "node-2", Ready: false, Unschedulable: true},
			},
		}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "c1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}
	var clusterNodes []*unifiedresources.K8sNodeView
	for _, node := range reg.K8sNodes() {
		if node != nil && node.ParentID() == cluster.ID() {
			clusterNodes = append(clusterNodes, node)
		}
	}

	summary, issues := summarizeK8sNodes(clusterNodes)
	if !strings.Contains(summary, "2 total, 1 ready, 1 not ready, 1 unschedulable") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "node-2") {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestSummarizeK8sNodes_Truncates(t *testing.T) {
	nodes := make([]models.KubernetesNode, maxKubernetesNodeIssues+1)
	for i := range nodes {
		nodes[i] = models.KubernetesNode{Name: "node-" + string(rune('a'+i)), Ready: false}
	}
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{ID: "c1", Name: "cluster", Nodes: nodes}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "c1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}
	var clusterNodes []*unifiedresources.K8sNodeView
	for _, node := range reg.K8sNodes() {
		if node != nil && node.ParentID() == cluster.ID() {
			clusterNodes = append(clusterNodes, node)
		}
	}

	_, issues := summarizeK8sNodes(clusterNodes)
	if len(issues) != maxKubernetesNodeIssues+1 {
		t.Fatalf("expected truncated issues list, got %d", len(issues))
	}
	if !strings.Contains(issues[len(issues)-1], "and 1 more") {
		t.Fatalf("expected truncation message, got %s", issues[len(issues)-1])
	}
}

// ========================================
// summarizeK8sPods tests
// ========================================

func TestSummarizeK8sPods(t *testing.T) {
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Pods: []models.KubernetesPod{
				{
					Name:      "ok",
					Namespace: "default",
					Phase:     "Running",
					Containers: []models.KubernetesPodContainer{
						{Name: "app", Ready: true, State: "running"},
					},
				},
				{
					Name:      "bad",
					Namespace: "default",
					Phase:     "Running",
					Restarts:  2,
					Containers: []models.KubernetesPodContainer{
						{Name: "app", Ready: false, State: "waiting", Reason: "CrashLoopBackOff"},
					},
				},
				{Name: "pending", Namespace: "default", Phase: "Pending"},
				{Name: "failed", Namespace: "default", Phase: "Failed", Reason: "Error", Message: "failed", Restarts: 1},
				{Name: "unknown", Namespace: "default", Phase: ""},
				{Name: "succeeded", Namespace: "default", Phase: "Succeeded"},
			},
		}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "c1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}
	var clusterPods []*unifiedresources.PodView
	for _, pod := range reg.Pods() {
		if pod != nil && pod.ParentID() == cluster.ID() {
			clusterPods = append(clusterPods, pod)
		}
	}

	summary, issues, restarts := summarizeK8sPods(clusterPods)
	if !strings.Contains(summary, "6 total") || !strings.Contains(summary, "2 running") || !strings.Contains(summary, "1 pending") ||
		!strings.Contains(summary, "1 failed") || !strings.Contains(summary, "1 succeeded") || !strings.Contains(summary, "1 unknown") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if len(issues) == 0 {
		t.Fatalf("expected pod issues, got none")
	}
	if len(restarts) != 2 {
		t.Fatalf("expected restart leaders, got %d", len(restarts))
	}
}

// ========================================
// summarizeK8sDeployments tests
// ========================================

func TestSummarizeK8sDeployments(t *testing.T) {
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Deployments: []models.KubernetesDeployment{
				{Namespace: "default", Name: "ok", DesiredReplicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2},
				{Namespace: "default", Name: "bad", DesiredReplicas: 2, ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
			},
		}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "c1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}
	var clusterDeploys []*unifiedresources.K8sDeploymentView
	for _, deploy := range reg.K8sDeployments() {
		if deploy != nil && deploy.ParentID() == cluster.ID() {
			clusterDeploys = append(clusterDeploys, deploy)
		}
	}

	summary, issues := summarizeK8sDeployments(clusterDeploys)
	if !strings.Contains(summary, "2 total, 1 healthy, 1 unhealthy") {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "default/bad") {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestSummarizeK8sDeployments_Truncates(t *testing.T) {
	deployments := make([]models.KubernetesDeployment, maxKubernetesDeploymentIssues+1)
	for i := range deployments {
		deployments[i] = models.KubernetesDeployment{
			Namespace:         "ns",
			Name:              "dep-" + string(rune('a'+i)),
			DesiredReplicas:   2,
			ReadyReplicas:     0,
			UpdatedReplicas:   0,
			AvailableReplicas: 0,
		}
	}

	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:          "c1",
			Name:        "cluster",
			Deployments: deployments,
		}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "c1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}
	var clusterDeploys []*unifiedresources.K8sDeploymentView
	for _, deploy := range reg.K8sDeployments() {
		if deploy != nil && deploy.ParentID() == cluster.ID() {
			clusterDeploys = append(clusterDeploys, deploy)
		}
	}

	_, issues := summarizeK8sDeployments(clusterDeploys)
	if len(issues) != maxKubernetesDeploymentIssues+1 {
		t.Fatalf("expected truncated issues list, got %d", len(issues))
	}
	if !strings.Contains(issues[len(issues)-1], "and 1 more") {
		t.Fatalf("expected truncation message, got %s", issues[len(issues)-1])
	}
}

// ========================================
// buildK8sClusterContext tests
// ========================================

func TestBuildK8sClusterContext(t *testing.T) {
	now := time.Now().Add(-5 * time.Minute)
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:               "cluster-1",
			Name:             "prod",
			Status:           "healthy",
			Version:          "1.27",
			Server:           "https://kube.local",
			Context:          "prod",
			AgentVersion:     "v1",
			IntervalSeconds:  60,
			LastSeen:         now,
			PendingUninstall: true,
			Nodes:            []models.KubernetesNode{{Name: "node-1", Ready: false, Unschedulable: true}},
			Pods: []models.KubernetesPod{
				{Name: "pod-1", Namespace: "default", Phase: "Pending"},
				{Name: "pod-2", Namespace: "default", Phase: "Running", Restarts: 2},
			},
			Deployments: []models.KubernetesDeployment{{Namespace: "default", Name: "dep", DesiredReplicas: 1}},
		}},
	})

	cluster := findK8sCluster(reg.K8sClusters(), "cluster-1")
	if cluster == nil {
		t.Fatal("cluster not found")
	}

	ctx := buildK8sClusterContext(cluster, reg)
	if !strings.Contains(ctx, "Cluster Summary") || !strings.Contains(ctx, "Pending uninstall: true") {
		t.Fatalf("unexpected context: %s", ctx)
	}
	if !strings.Contains(ctx, "Unhealthy Nodes") || !strings.Contains(ctx, "Unhealthy Pods") {
		t.Fatalf("expected unhealthy sections, got: %s", ctx)
	}
	if !strings.Contains(ctx, "Pods With Restarts") {
		t.Fatalf("expected pod restarts section, got: %s", ctx)
	}
	if !strings.Contains(ctx, "Deployments Not Fully Available") {
		t.Fatalf("expected deployment issues section, got: %s", ctx)
	}
}

// ========================================
// AnalyzeKubernetesCluster tests
// ========================================

func TestAnalyzeKubernetesCluster_Errors(t *testing.T) {
	svc := &Service{}

	if _, err := svc.AnalyzeKubernetesCluster(context.Background(), ""); err == nil {
		t.Fatal("expected error for missing cluster ID")
	}

	if _, err := svc.AnalyzeKubernetesCluster(context.Background(), "cluster-1"); !errors.Is(err, ErrKubernetesStateUnavailable) {
		t.Fatalf("expected ErrKubernetesStateUnavailable, got %v", err)
	}

	// Set up ReadState with no clusters → should return ClusterNotFound.
	reg := unifiedresources.NewRegistry(nil)
	svc.SetReadState(reg)
	if _, err := svc.AnalyzeKubernetesCluster(context.Background(), "cluster-1"); !errors.Is(err, ErrKubernetesClusterNotFound) {
		t.Fatalf("expected ErrKubernetesClusterNotFound, got %v", err)
	}
}

func TestAnalyzeKubernetesCluster_Success(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, Model: "anthropic:test-model"}
	svc.provider = &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{Content: "ok"}, nil
		},
	}

	cluster := models.KubernetesCluster{
		ID:   "cluster-1",
		Name: "prod",
	}
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{cluster},
	})
	svc.SetReadState(reg)

	resp, err := svc.AnalyzeKubernetesCluster(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("AnalyzeKubernetesCluster failed: %v", err)
	}
	if resp == nil || resp.Content != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// ========================================
// Test helpers
// ========================================

// makePodView creates a PodView from a phase and containers for testing.
func makePodView(t *testing.T, phase string, containers []unifiedresources.K8sPodContainer) *unifiedresources.PodView {
	t.Helper()
	pod := models.KubernetesPod{
		Name:      "test-pod",
		Namespace: "default",
		Phase:     phase,
	}
	for _, c := range containers {
		pod.Containers = append(pod.Containers, models.KubernetesPodContainer{
			Name:  c.Name,
			Ready: c.Ready,
			State: c.State,
		})
	}
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Pods: []models.KubernetesPod{pod},
		}},
	})
	pods := reg.Pods()
	if len(pods) == 0 {
		t.Fatal("expected pod in registry")
	}
	return pods[0]
}

// makePodViewFromModel creates a PodView from a models.KubernetesPod for testing.
func makePodViewFromModel(t *testing.T, pod models.KubernetesPod) *unifiedresources.PodView {
	t.Helper()
	if pod.Name == "" {
		pod.Name = "test-pod"
	}
	if pod.Namespace == "" {
		pod.Namespace = "default"
	}
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Pods: []models.KubernetesPod{pod},
		}},
	})
	pods := reg.Pods()
	if len(pods) == 0 {
		t.Fatal("expected pod in registry")
	}
	return pods[0]
}

// makeDeploymentView creates a K8sDeploymentView for testing.
func makeDeploymentView(t *testing.T, desired, ready, available, updated int32) *unifiedresources.K8sDeploymentView {
	t.Helper()
	reg := unifiedresources.NewRegistry(nil)
	reg.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "c1",
			Name: "cluster",
			Deployments: []models.KubernetesDeployment{{
				Name:              "test-deploy",
				Namespace:         "default",
				DesiredReplicas:   desired,
				ReadyReplicas:     ready,
				AvailableReplicas: available,
				UpdatedReplicas:   updated,
			}},
		}},
	})
	deploys := reg.K8sDeployments()
	if len(deploys) == 0 {
		t.Fatal("expected deployment in registry")
	}
	return deploys[0]
}
