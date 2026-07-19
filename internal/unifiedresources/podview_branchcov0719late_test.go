package unifiedresources

import (
	"testing"
	"time"
)

// branchcov0719latePodResource builds a fully-populated Pod resource used to
// exercise the populated arm of every PodView accessor. The nil-receiver and
// nil-Kubernetes arms are covered separately in each test below.
func branchcov0719latePodResource(now time.Time) *Resource {
	return &Resource{
		ID:       "pod-id-1",
		Type:     ResourceTypePod,
		Name:     "pod-name-1",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"app:web", "tier:frontend"},
		Kubernetes: &K8sData{
			ClusterID:   "cluster-id-1",
			ClusterName: "prod-cluster",
			NodeName:    "node-a",
			Namespace:   "checkout",
			PodUID:      "uid-1234",
			PodPhase:    "Running",
			PodReason:   "OutOfcpu",
			PodMessage:  "Pod was terminated due to cpu limit.",
			Restarts:    3,
			OwnerKind:   "ReplicaSet",
			OwnerName:   "checkout-abc",
			Image:       "ghcr.io/example/checkout:v1.2.3",
			PodContainers: []K8sPodContainer{
				{Name: "app", Image: "ghcr.io/example/checkout:v1.2.3", Ready: true, RestartCount: 1, State: "running"},
				{Name: "sidecar", Image: "busybox:1.36", Ready: false, RestartCount: 2, State: "waiting", Reason: "CrashLoopBackOff", Message: "Back-off pulling image"},
			},
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 42.5},
			Memory: &MetricValue{Percent: 55.0},
			Disk:   &MetricValue{Percent: 12.0},
			NetIn:  &MetricValue{Value: 1024.5},
			NetOut: &MetricValue{Value: 2048.0},
		},
	}
}

// TestPodViewBranchcov0719late_StringIDStatus covers String, ID and Status
// across the nil-receiver and populated arms.
func TestPodViewBranchcov0719late_StringIDStatus(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	// Nil-receiver arm: every accessor must return its zero value without panic.
	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if got := zero.String(); got != `PodView(, "")` {
			t.Fatalf("nil String(): got %q want %q", got, `PodView(, "")`)
		}
		if zero.ID() != "" {
			t.Fatalf("nil ID(): got %q want empty", zero.ID())
		}
		if zero.Status() != ResourceStatus("") {
			t.Fatalf("nil Status(): got %q want empty", zero.Status())
		}
	})

	// Populated arm.
	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		if got, want := v.String(), `PodView(pod-id-1, "pod-name-1")`; got != want {
			t.Fatalf("String(): got %q want %q", got, want)
		}
		if v.ID() != "pod-id-1" {
			t.Fatalf("ID(): got %q want %q", v.ID(), "pod-id-1")
		}
		if v.Status() != StatusOnline {
			t.Fatalf("Status(): got %q want %q", v.Status(), StatusOnline)
		}
	})
}

// TestPodViewBranchcov0719late_KubernetesFields covers the Kubernetes-backed
// accessors: ClusterName, ClusterID, NodeName, PodUID, PodPhase, Restarts,
// OwnerKind, OwnerName, Image, PodReason, PodMessage. Each has both a
// nil-receiver arm and a nil-Kubernetes-nested arm, plus the populated arm.
func TestPodViewBranchcov0719late_KubernetesFields(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if zero.ClusterName() != "" {
			t.Fatalf("nil ClusterName(): got %q want empty", zero.ClusterName())
		}
		if zero.ClusterID() != "" {
			t.Fatalf("nil ClusterID(): got %q want empty", zero.ClusterID())
		}
		if zero.NodeName() != "" {
			t.Fatalf("nil NodeName(): got %q want empty", zero.NodeName())
		}
		if zero.PodUID() != "" {
			t.Fatalf("nil PodUID(): got %q want empty", zero.PodUID())
		}
		if zero.PodPhase() != "" {
			t.Fatalf("nil PodPhase(): got %q want empty", zero.PodPhase())
		}
		if zero.Restarts() != 0 {
			t.Fatalf("nil Restarts(): got %d want 0", zero.Restarts())
		}
		if zero.OwnerKind() != "" {
			t.Fatalf("nil OwnerKind(): got %q want empty", zero.OwnerKind())
		}
		if zero.OwnerName() != "" {
			t.Fatalf("nil OwnerName(): got %q want empty", zero.OwnerName())
		}
		if zero.Image() != "" {
			t.Fatalf("nil Image(): got %q want empty", zero.Image())
		}
		if zero.PodReason() != "" {
			t.Fatalf("nil PodReason(): got %q want empty", zero.PodReason())
		}
		if zero.PodMessage() != "" {
			t.Fatalf("nil PodMessage(): got %q want empty", zero.PodMessage())
		}
	})

	// Nested-nil arm: Resource is present but Kubernetes is nil.
	t.Run("NilKubernetes", func(t *testing.T) {
		r := testResource(ResourceTypePod)
		r.Kubernetes = nil
		v := NewPodView(r)

		if v.ClusterName() != "" {
			t.Fatalf("ClusterName() with nil Kubernetes: got %q want empty", v.ClusterName())
		}
		if v.ClusterID() != "" {
			t.Fatalf("ClusterID() with nil Kubernetes: got %q want empty", v.ClusterID())
		}
		if v.NodeName() != "" {
			t.Fatalf("NodeName() with nil Kubernetes: got %q want empty", v.NodeName())
		}
		if v.PodUID() != "" {
			t.Fatalf("PodUID() with nil Kubernetes: got %q want empty", v.PodUID())
		}
		if v.PodPhase() != "" {
			t.Fatalf("PodPhase() with nil Kubernetes: got %q want empty", v.PodPhase())
		}
		if v.Restarts() != 0 {
			t.Fatalf("Restarts() with nil Kubernetes: got %d want 0", v.Restarts())
		}
		if v.OwnerKind() != "" {
			t.Fatalf("OwnerKind() with nil Kubernetes: got %q want empty", v.OwnerKind())
		}
		if v.OwnerName() != "" {
			t.Fatalf("OwnerName() with nil Kubernetes: got %q want empty", v.OwnerName())
		}
		if v.Image() != "" {
			t.Fatalf("Image() with nil Kubernetes: got %q want empty", v.Image())
		}
		if v.PodReason() != "" {
			t.Fatalf("PodReason() with nil Kubernetes: got %q want empty", v.PodReason())
		}
		if v.PodMessage() != "" {
			t.Fatalf("PodMessage() with nil Kubernetes: got %q want empty", v.PodMessage())
		}
	})

	// Populated arm: each accessor projects its backing field verbatim.
	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		if v.ClusterName() != "prod-cluster" {
			t.Fatalf("ClusterName(): got %q want %q", v.ClusterName(), "prod-cluster")
		}
		if v.ClusterID() != "cluster-id-1" {
			t.Fatalf("ClusterID(): got %q want %q", v.ClusterID(), "cluster-id-1")
		}
		if v.NodeName() != "node-a" {
			t.Fatalf("NodeName(): got %q want %q", v.NodeName(), "node-a")
		}
		if v.PodUID() != "uid-1234" {
			t.Fatalf("PodUID(): got %q want %q", v.PodUID(), "uid-1234")
		}
		if v.PodPhase() != "Running" {
			t.Fatalf("PodPhase(): got %q want %q", v.PodPhase(), "Running")
		}
		if v.Restarts() != 3 {
			t.Fatalf("Restarts(): got %d want 3", v.Restarts())
		}
		if v.OwnerKind() != "ReplicaSet" {
			t.Fatalf("OwnerKind(): got %q want %q", v.OwnerKind(), "ReplicaSet")
		}
		if v.OwnerName() != "checkout-abc" {
			t.Fatalf("OwnerName(): got %q want %q", v.OwnerName(), "checkout-abc")
		}
		if v.Image() != "ghcr.io/example/checkout:v1.2.3" {
			t.Fatalf("Image(): got %q want %q", v.Image(), "ghcr.io/example/checkout:v1.2.3")
		}
		if v.PodReason() != "OutOfcpu" {
			t.Fatalf("PodReason(): got %q want %q", v.PodReason(), "OutOfcpu")
		}
		if v.PodMessage() != "Pod was terminated due to cpu limit." {
			t.Fatalf("PodMessage(): got %q want %q", v.PodMessage(), "Pod was terminated due to cpu limit.")
		}
	})
}

// TestPodViewBranchcov0719late_Metrics covers CPUPercent, MemoryPercent,
// DiskPercent, NetInRate and NetOutRate. The defensive nil-receiver arm
// returns 0; the nil-Metrics nested arm also routes through
// viewMetricPercent/viewMetricValue and returns 0.
func TestPodViewBranchcov0719late_Metrics(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if zero.CPUPercent() != 0 {
			t.Fatalf("nil CPUPercent(): got %v want 0", zero.CPUPercent())
		}
		if zero.MemoryPercent() != 0 {
			t.Fatalf("nil MemoryPercent(): got %v want 0", zero.MemoryPercent())
		}
		if zero.DiskPercent() != 0 {
			t.Fatalf("nil DiskPercent(): got %v want 0", zero.DiskPercent())
		}
		if zero.NetInRate() != 0 {
			t.Fatalf("nil NetInRate(): got %v want 0", zero.NetInRate())
		}
		if zero.NetOutRate() != 0 {
			t.Fatalf("nil NetOutRate(): got %v want 0", zero.NetOutRate())
		}
	})

	t.Run("NilMetrics", func(t *testing.T) {
		r := testResource(ResourceTypePod)
		r.Metrics = nil
		v := NewPodView(r)
		if v.CPUPercent() != 0 || v.MemoryPercent() != 0 || v.DiskPercent() != 0 || v.NetInRate() != 0 || v.NetOutRate() != 0 {
			t.Fatalf("metric accessors with nil Metrics: got cpu=%v mem=%v disk=%v netIn=%v netOut=%v, all want 0",
				v.CPUPercent(), v.MemoryPercent(), v.DiskPercent(), v.NetInRate(), v.NetOutRate())
		}
	})

	t.Run("NilNestedMetricValues", func(t *testing.T) {
		// ResourceMetrics present but the individual MetricValue pointers nil.
		r := testResource(ResourceTypePod)
		r.Metrics = &ResourceMetrics{}
		v := NewPodView(r)
		if v.CPUPercent() != 0 || v.MemoryPercent() != 0 || v.DiskPercent() != 0 || v.NetInRate() != 0 || v.NetOutRate() != 0 {
			t.Fatalf("metric accessors with nil nested MetricValue: got cpu=%v mem=%v disk=%v netIn=%v netOut=%v",
				v.CPUPercent(), v.MemoryPercent(), v.DiskPercent(), v.NetInRate(), v.NetOutRate())
		}
	})

	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		if v.CPUPercent() != 42.5 {
			t.Fatalf("CPUPercent(): got %v want 42.5", v.CPUPercent())
		}
		if v.MemoryPercent() != 55.0 {
			t.Fatalf("MemoryPercent(): got %v want 55", v.MemoryPercent())
		}
		if v.DiskPercent() != 12.0 {
			t.Fatalf("DiskPercent(): got %v want 12", v.DiskPercent())
		}
		if v.NetInRate() != 1024.5 {
			t.Fatalf("NetInRate(): got %v want 1024.5", v.NetInRate())
		}
		if v.NetOutRate() != 2048.0 {
			t.Fatalf("NetOutRate(): got %v want 2048", v.NetOutRate())
		}
	})
}

// TestPodViewBranchcov0719late_Tags covers Tags: nil-receiver, nil field, and
// populated arms, plus clone independence.
func TestPodViewBranchcov0719late_Tags(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if got := zero.Tags(); got != nil {
			t.Fatalf("nil Tags(): got %v want nil", got)
		}
	})

	t.Run("NilField", func(t *testing.T) {
		r := testResource(ResourceTypePod)
		r.Tags = nil
		v := NewPodView(r)
		if got := v.Tags(); got != nil {
			t.Fatalf("Tags() with nil Tags field: got %v want nil", got)
		}
	})

	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		assertStringSlice(t, v.Tags(), []string{"app:web", "tier:frontend"})
	})

	t.Run("CloneIsIndependent", func(t *testing.T) {
		src := branchcov0719latePodResource(now)
		v := NewPodView(src)
		got := v.Tags()
		got[0] = "mutated"
		if src.Tags[0] != "app:web" {
			t.Fatalf("Tags() did not return an independent copy; underlying slice mutated to %q", src.Tags[0])
		}
	})
}

// TestPodViewBranchcov0719late_LastSeen covers LastSeen nil-receiver and
// populated arms.
func TestPodViewBranchcov0719late_LastSeen(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if !zero.LastSeen().IsZero() {
			t.Fatalf("nil LastSeen(): got %v want zero time", zero.LastSeen())
		}
	})

	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		if !v.LastSeen().Equal(now) {
			t.Fatalf("LastSeen(): got %v want %v", v.LastSeen(), now)
		}
	})
}

// TestPodViewBranchcov0719late_PodContainers covers PodContainers: nil
// receiver, nil Kubernetes facet, empty backing slice, populated, and clone
// independence.
func TestPodViewBranchcov0719late_PodContainers(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("NilReceiver", func(t *testing.T) {
		var zero PodView
		if got := zero.PodContainers(); got != nil {
			t.Fatalf("nil PodContainers(): got %v want nil", got)
		}
	})

	t.Run("NilKubernetes", func(t *testing.T) {
		r := testResource(ResourceTypePod)
		r.Kubernetes = nil
		v := NewPodView(r)
		if got := v.PodContainers(); got != nil {
			t.Fatalf("PodContainers() with nil Kubernetes: got %v want nil", got)
		}
	})

	t.Run("EmptySliceReturnsNil", func(t *testing.T) {
		// Backing slice is empty/nil — the len==0 guard must return nil.
		r := testResource(ResourceTypePod)
		r.Kubernetes = &K8sData{}
		v := NewPodView(r)
		if got := v.PodContainers(); got != nil {
			t.Fatalf("PodContainers() with empty backing slice: got %v want nil", got)
		}
	})

	t.Run("Populated", func(t *testing.T) {
		v := NewPodView(branchcov0719latePodResource(now))
		got := v.PodContainers()
		if len(got) != 2 {
			t.Fatalf("PodContainers(): got %d items want 2: %+v", len(got), got)
		}
		if got[0].Name != "app" || got[0].Image != "ghcr.io/example/checkout:v1.2.3" || got[0].Ready != true || got[0].RestartCount != 1 || got[0].State != "running" {
			t.Fatalf("PodContainers()[0] mismatch: got %+v", got[0])
		}
		if got[1].Name != "sidecar" || got[1].Reason != "CrashLoopBackOff" || got[1].State != "waiting" || got[1].Message != "Back-off pulling image" {
			t.Fatalf("PodContainers()[1] mismatch: got %+v", got[1])
		}
	})

	t.Run("CloneIsIndependent", func(t *testing.T) {
		src := branchcov0719latePodResource(now)
		v := NewPodView(src)
		got := v.PodContainers()
		got[0].Name = "mutated"
		got[1].RestartCount = 99
		if src.Kubernetes.PodContainers[0].Name != "app" {
			t.Fatalf("PodContainers() did not return an independent copy; element[0].Name mutated to %q", src.Kubernetes.PodContainers[0].Name)
		}
		if src.Kubernetes.PodContainers[1].RestartCount != 2 {
			t.Fatalf("PodContainers() did not return an independent copy; element[1].RestartCount mutated to %d", src.Kubernetes.PodContainers[1].RestartCount)
		}
	})
}
