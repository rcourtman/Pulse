package unifiedresources

import (
	"reflect"
	"testing"
	"time"
)

// TestK8sDeploymentViewBranchcov0719late_NilReceiver exercises the nil-receiver
// arm of every K8sDeploymentView accessor. A zero-value view (no backing
// *Resource) must return zero values without panicking.
func TestK8sDeploymentViewBranchcov0719late_NilReceiver(t *testing.T) {
	var v K8sDeploymentView

	if got := v.ID(); got != "" {
		t.Fatalf("nil-receiver ID: expected %q, got %q", "", got)
	}
	if got := v.Status(); got != "" {
		t.Fatalf("nil-receiver Status: expected %q, got %q", "", got)
	}
	if got := v.ClusterName(); got != "" {
		t.Fatalf("nil-receiver ClusterName: expected %q, got %q", "", got)
	}
	if got := v.DeploymentUID(); got != "" {
		t.Fatalf("nil-receiver DeploymentUID: expected %q, got %q", "", got)
	}
	if got := v.DesiredReplicas(); got != 0 {
		t.Fatalf("nil-receiver DesiredReplicas: expected 0, got %d", got)
	}
	if got := v.UpdatedReplicas(); got != 0 {
		t.Fatalf("nil-receiver UpdatedReplicas: expected 0, got %d", got)
	}
	if got := v.ReadyReplicas(); got != 0 {
		t.Fatalf("nil-receiver ReadyReplicas: expected 0, got %d", got)
	}
	if got := v.AvailableReplicas(); got != 0 {
		t.Fatalf("nil-receiver AvailableReplicas: expected 0, got %d", got)
	}
	if got := v.Labels(); got != nil {
		t.Fatalf("nil-receiver Labels: expected nil, got %+v", got)
	}
	if got := v.Tags(); got != nil {
		t.Fatalf("nil-receiver Tags: expected nil, got %+v", got)
	}
	if got := v.LastSeen(); !got.IsZero() {
		t.Fatalf("nil-receiver LastSeen: expected zero time, got %v", got)
	}
	if got, want := v.String(), `K8sDeploymentView(, "")`; got != want {
		t.Fatalf("nil-receiver String: expected %q, got %q", want, got)
	}
}

// TestK8sDeploymentViewBranchcov0719late_NilKubernetesNested exercises the
// nil-nested arm of accessors that dereference v.r.Kubernetes. With a populated
// *Resource but nil Kubernetes payload, Kubernetes-backed accessors must return
// their zero values, while the resource-level accessors (ID/Status/Tags/LastSeen)
// still project the underlying value.
func TestK8sDeploymentViewBranchcov0719late_NilKubernetesNested(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 30, 0, 0, time.UTC)
	r := &Resource{
		ID:       "k8sdep-no-k8s",
		Type:     ResourceTypeK8sDeployment,
		Name:     "orphan-deploy",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"k8s", "app:orphan"},
		// Kubernetes deliberately nil.
	}
	v := NewK8sDeploymentView(r)

	// Kubernetes-backed accessors -> zero values (nil-nested arm).
	if got := v.ClusterName(); got != "" {
		t.Fatalf("nil-nested ClusterName: expected %q, got %q", "", got)
	}
	if got := v.DeploymentUID(); got != "" {
		t.Fatalf("nil-nested DeploymentUID: expected %q, got %q", "", got)
	}
	if got := v.DesiredReplicas(); got != 0 {
		t.Fatalf("nil-nested DesiredReplicas: expected 0, got %d", got)
	}
	if got := v.UpdatedReplicas(); got != 0 {
		t.Fatalf("nil-nested UpdatedReplicas: expected 0, got %d", got)
	}
	if got := v.ReadyReplicas(); got != 0 {
		t.Fatalf("nil-nested ReadyReplicas: expected 0, got %d", got)
	}
	if got := v.AvailableReplicas(); got != 0 {
		t.Fatalf("nil-nested AvailableReplicas: expected 0, got %d", got)
	}
	if got := v.Labels(); got != nil {
		t.Fatalf("nil-nested Labels: expected nil, got %+v", got)
	}

	// Resource-backed accessors still project the underlying value (only nil-r
	// guard, which is non-nil here).
	if got := v.ID(); got != "k8sdep-no-k8s" {
		t.Fatalf("nil-nested ID: expected %q, got %q", "k8sdep-no-k8s", got)
	}
	if got := v.Status(); got != StatusOnline {
		t.Fatalf("nil-nested Status: expected %q, got %q", StatusOnline, got)
	}
	assertStringSlice(t, v.Tags(), []string{"k8s", "app:orphan"})
	if got := v.LastSeen(); !got.Equal(now) {
		t.Fatalf("nil-nested LastSeen: expected %v, got %v", now, got)
	}
}

// TestK8sDeploymentViewBranchcov0719late_Populated exercises the populated arm
// of every accessor with a fully-populated Kubernetes payload, including
// clone-independence for Labels and Tags (mutating the returned value must not
// affect the backing resource).
func TestK8sDeploymentViewBranchcov0719late_Populated(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	r := &Resource{
		ID:       "k8sdep-1",
		Type:     ResourceTypeK8sDeployment,
		Name:     "frontend",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"k8s", "app:frontend"},
		Kubernetes: &K8sData{
			ClusterName:       "prod-k8s",
			Namespace:         "web",
			DeploymentUID:     "deploy-uid-123",
			DesiredReplicas:   3,
			UpdatedReplicas:   2,
			ReadyReplicas:     2,
			AvailableReplicas: 1,
			Labels:            map[string]string{"app": "nginx", "tier": "web"},
		},
	}
	v := NewK8sDeploymentView(r)

	if got := v.ID(); got != "k8sdep-1" {
		t.Fatalf("populated ID: expected %q, got %q", "k8sdep-1", got)
	}
	if got := v.Status(); got != StatusOnline {
		t.Fatalf("populated Status: expected %q, got %q", StatusOnline, got)
	}
	if got := v.ClusterName(); got != "prod-k8s" {
		t.Fatalf("populated ClusterName: expected %q, got %q", "prod-k8s", got)
	}
	if got := v.DeploymentUID(); got != "deploy-uid-123" {
		t.Fatalf("populated DeploymentUID: expected %q, got %q", "deploy-uid-123", got)
	}
	if got := v.DesiredReplicas(); got != 3 {
		t.Fatalf("populated DesiredReplicas: expected 3, got %d", got)
	}
	if got := v.UpdatedReplicas(); got != 2 {
		t.Fatalf("populated UpdatedReplicas: expected 2, got %d", got)
	}
	if got := v.ReadyReplicas(); got != 2 {
		t.Fatalf("populated ReadyReplicas: expected 2, got %d", got)
	}
	if got := v.AvailableReplicas(); got != 1 {
		t.Fatalf("populated AvailableReplicas: expected 1, got %d", got)
	}

	wantLabels := map[string]string{"app": "nginx", "tier": "web"}
	if got := v.Labels(); !reflect.DeepEqual(got, wantLabels) {
		t.Fatalf("populated Labels: expected %+v, got %+v", wantLabels, got)
	}
	// Clone independence: mutating the returned map must not affect the source.
	labelsClone := v.Labels()
	labelsClone["app"] = "mutated"
	delete(labelsClone, "tier")
	labelsClone["new"] = "value"
	if got := v.Labels(); !reflect.DeepEqual(got, wantLabels) {
		t.Fatalf("populated Labels independence broken: source mutated to %+v", got)
	}

	assertStringSlice(t, v.Tags(), []string{"k8s", "app:frontend"})
	// Clone independence: mutating the returned slice must not affect the source.
	tagsClone := v.Tags()
	if len(tagsClone) > 0 {
		tagsClone[0] = "mutated"
	}
	if got, want := v.Tags(), ([]string{"k8s", "app:frontend"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("populated Tags independence broken: source mutated to %+v", got)
	}

	if got := v.LastSeen(); !got.Equal(now) {
		t.Fatalf("populated LastSeen: expected %v, got %v", now, got)
	}
	if got, want := v.String(), `K8sDeploymentView(k8sdep-1, "frontend")`; got != want {
		t.Fatalf("populated String: expected %q, got %q", want, got)
	}
}

// TestK8sClusterViewBranchcov0719late_SourceStatusAgentVersionInterval
// exercises the nil-receiver, nil-Kubernetes-nested, and populated arms of the
// three K8sClusterView accessors delegated to the prompt.
func TestK8sClusterViewBranchcov0719late_SourceStatusAgentVersionInterval(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		var v K8sClusterView
		if got := v.SourceStatus(); got != "" {
			t.Fatalf("nil-receiver SourceStatus: expected %q, got %q", "", got)
		}
		if got := v.AgentVersion(); got != "" {
			t.Fatalf("nil-receiver AgentVersion: expected %q, got %q", "", got)
		}
		if got := v.IntervalSeconds(); got != 0 {
			t.Fatalf("nil-receiver IntervalSeconds: expected 0, got %d", got)
		}
	})

	t.Run("NilKubernetesNested", func(t *testing.T) {
		r := &Resource{
			ID:       "k8s-no-payload",
			Type:     ResourceTypeK8sCluster,
			Name:     "orphan-cluster",
			Status:   StatusOnline,
			LastSeen: time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC),
			// Kubernetes deliberately nil.
		}
		v := NewK8sClusterView(r)
		if got := v.SourceStatus(); got != "" {
			t.Fatalf("nil-nested SourceStatus: expected %q, got %q", "", got)
		}
		if got := v.AgentVersion(); got != "" {
			t.Fatalf("nil-nested AgentVersion: expected %q, got %q", "", got)
		}
		if got := v.IntervalSeconds(); got != 0 {
			t.Fatalf("nil-nested IntervalSeconds: expected 0, got %d", got)
		}
	})

	t.Run("Populated", func(t *testing.T) {
		r := &Resource{
			ID:       "k8s-1",
			Type:     ResourceTypeK8sCluster,
			Name:     "prod-k8s",
			Status:   StatusOnline,
			LastSeen: time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC),
			Kubernetes: &K8sData{
				ClusterName:     "prod-k8s",
				SourceStatus:    "online",
				AgentVersion:    "1.5.0",
				IntervalSeconds: 30,
			},
		}
		v := NewK8sClusterView(r)
		if got := v.SourceStatus(); got != "online" {
			t.Fatalf("populated SourceStatus: expected %q, got %q", "online", got)
		}
		if got := v.AgentVersion(); got != "1.5.0" {
			t.Fatalf("populated AgentVersion: expected %q, got %q", "1.5.0", got)
		}
		if got := v.IntervalSeconds(); got != 30 {
			t.Fatalf("populated IntervalSeconds: expected 30, got %d", got)
		}
	})
}
