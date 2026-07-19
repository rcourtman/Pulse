package unifiedresources

import (
	"testing"
	"time"
)

// This file is a mechanical branch-coverage test for the K8sNodeView accessors
// in views.go. It exercises the nil-receiver arm, the nil-nested-Kubernetes
// arm (where present), and the fully-populated arm of every listed method.
// It deliberately does not touch any source file or pre-existing test.

// k8sNodeBranchcov0719lateResource builds a fully-populated K8sNode *Resource
// exercising every field surfaced by K8sNodeView's accessors.
func k8sNodeBranchcov0719lateResource(now time.Time, parentID string) *Resource {
	return &Resource{
		ID:       "k8snode-1",
		Type:     ResourceTypeK8sNode,
		Name:     "worker-1",
		Status:   StatusOnline,
		LastSeen: now,
		Tags:     []string{"k8s", "role:worker"},
		ParentID: &parentID,
		Kubernetes: &K8sData{
			ClusterName:             "prod-cluster",
			NodeUID:                 "uid-node-1",
			NodeName:                "worker-1",
			Ready:                   true,
			Unschedulable:           true,
			Roles:                   []string{"control-plane", "worker"},
			KubeletVersion:          "v1.31.0",
			ContainerRuntimeVersion: "containerd://1.7.20",
			OSImage:                 "Ubuntu 24.04 LTS",
			KernelVersion:           "6.8.0-45-generic",
			Architecture:            "amd64",
			CapacityCPU:             16,
			CapacityMemoryBytes:     32 * 1024 * 1024 * 1024,
			CapacityPods:            110,
			AllocCPU:                15,
			AllocMemoryBytes:        30 * 1024 * 1024 * 1024,
			AllocPods:               100,
		},
		Metrics: &ResourceMetrics{
			CPU:    &MetricValue{Percent: 42.5},
			Memory: &MetricValue{Percent: 67.25},
		},
	}
}

// TestK8sNodeViewBranchcov0719late_NilReceiver covers the v.r == nil arm of
// every accessor plus the String() formatter and the NewK8sNodeView ctor
// (called with a nil *Resource).
func TestK8sNodeViewBranchcov0719late_NilReceiver(t *testing.T) {
	v := NewK8sNodeView(nil)

	if got := v.ID(); got != "" {
		t.Fatalf("ID() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.Name(); got != "" {
		t.Fatalf("Name() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.Status(); got != ResourceStatus("") {
		t.Fatalf("Status() nil-receiver: want empty, got %q", got)
	}
	if got := v.ClusterName(); got != "" {
		t.Fatalf("ClusterName() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.NodeUID(); got != "" {
		t.Fatalf("NodeUID() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.NodeName(); got != "" {
		t.Fatalf("NodeName() nil-receiver: want %q, got %q", "", got)
	}
	if v.Ready() {
		t.Fatalf("Ready() nil-receiver: want false, got true")
	}
	if v.Unschedulable() {
		t.Fatalf("Unschedulable() nil-receiver: want false, got true")
	}
	if got := v.Roles(); got != nil {
		t.Fatalf("Roles() nil-receiver: want nil, got %v", got)
	}
	if got := v.KubeletVersion(); got != "" {
		t.Fatalf("KubeletVersion() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.ContainerRuntimeVersion(); got != "" {
		t.Fatalf("ContainerRuntimeVersion() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.OSImage(); got != "" {
		t.Fatalf("OSImage() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.KernelVersion(); got != "" {
		t.Fatalf("KernelVersion() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.Architecture(); got != "" {
		t.Fatalf("Architecture() nil-receiver: want %q, got %q", "", got)
	}
	if got := v.CapacityCPU(); got != 0 {
		t.Fatalf("CapacityCPU() nil-receiver: want 0, got %d", got)
	}
	if got := v.CapacityMemoryBytes(); got != 0 {
		t.Fatalf("CapacityMemoryBytes() nil-receiver: want 0, got %d", got)
	}
	if got := v.CapacityPods(); got != 0 {
		t.Fatalf("CapacityPods() nil-receiver: want 0, got %d", got)
	}
	if got := v.AllocCPU(); got != 0 {
		t.Fatalf("AllocCPU() nil-receiver: want 0, got %d", got)
	}
	if got := v.AllocMemoryBytes(); got != 0 {
		t.Fatalf("AllocMemoryBytes() nil-receiver: want 0, got %d", got)
	}
	if got := v.AllocPods(); got != 0 {
		t.Fatalf("AllocPods() nil-receiver: want 0, got %d", got)
	}
	if got := v.CPUPercent(); got != 0 {
		t.Fatalf("CPUPercent() nil-receiver: want 0, got %v", got)
	}
	if got := v.MemoryPercent(); got != 0 {
		t.Fatalf("MemoryPercent() nil-receiver: want 0, got %v", got)
	}
	if got := v.Tags(); got != nil {
		t.Fatalf("Tags() nil-receiver: want nil, got %v", got)
	}
	if got := v.LastSeen(); !got.IsZero() {
		t.Fatalf("LastSeen() nil-receiver: want zero time, got %v", got)
	}
	if got := v.ParentID(); got != "" {
		t.Fatalf("ParentID() nil-receiver: want %q, got %q", "", got)
	}

	// String() must not panic on a nil-receiver and must report empty id/name.
	if got := v.String(); got != `K8sNodeView(, "")` {
		t.Fatalf("String() nil-receiver: want %q, got %q", `K8sNodeView(, "")`, got)
	}
}

// TestK8sNodeViewBranchcov0719late_NilKubernetesNested covers the
// v.r.Kubernetes == nil arm of every Kubernetes-backed accessor. The
// non-Kubernetes accessors (ID/Name/Status/Tags/LastSeen/CPUPercent/
// MemoryPercent/ParentID) are also exercised to confirm they still project
// from the outer Resource when Kubernetes is nil.
func TestK8sNodeViewBranchcov0719late_NilKubernetesNested(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 30, 0, 0, time.UTC)
	parentID := "cluster-parent-1"
	r := &Resource{
		ID:       "k8snode-no-k8s",
		Type:     ResourceTypeK8sNode,
		Name:     "worker-bare",
		Status:   StatusOffline,
		LastSeen: now,
		Tags:     []string{"bare"},
		ParentID: &parentID,
		// Kubernetes intentionally nil.
		// Metrics intentionally nil so CPUPercent/MemoryPercent go through
		// their nil-receiver-equivalent path via viewMetricPercent(nil, ...).
	}
	v := NewK8sNodeView(r)

	// Kubernetes-backed accessors: all should return their zero value.
	if got := v.ClusterName(); got != "" {
		t.Fatalf("ClusterName() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.NodeUID(); got != "" {
		t.Fatalf("NodeUID() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.NodeName(); got != "" {
		t.Fatalf("NodeName() Kubernetes==nil: want %q, got %q", "", got)
	}
	if v.Ready() {
		t.Fatalf("Ready() Kubernetes==nil: want false, got true")
	}
	if v.Unschedulable() {
		t.Fatalf("Unschedulable() Kubernetes==nil: want false, got true")
	}
	if got := v.Roles(); got != nil {
		t.Fatalf("Roles() Kubernetes==nil: want nil, got %v", got)
	}
	if got := v.KubeletVersion(); got != "" {
		t.Fatalf("KubeletVersion() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.ContainerRuntimeVersion(); got != "" {
		t.Fatalf("ContainerRuntimeVersion() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.OSImage(); got != "" {
		t.Fatalf("OSImage() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.KernelVersion(); got != "" {
		t.Fatalf("KernelVersion() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.Architecture(); got != "" {
		t.Fatalf("Architecture() Kubernetes==nil: want %q, got %q", "", got)
	}
	if got := v.CapacityCPU(); got != 0 {
		t.Fatalf("CapacityCPU() Kubernetes==nil: want 0, got %d", got)
	}
	if got := v.CapacityMemoryBytes(); got != 0 {
		t.Fatalf("CapacityMemoryBytes() Kubernetes==nil: want 0, got %d", got)
	}
	if got := v.CapacityPods(); got != 0 {
		t.Fatalf("CapacityPods() Kubernetes==nil: want 0, got %d", got)
	}
	if got := v.AllocCPU(); got != 0 {
		t.Fatalf("AllocCPU() Kubernetes==nil: want 0, got %d", got)
	}
	if got := v.AllocMemoryBytes(); got != 0 {
		t.Fatalf("AllocMemoryBytes() Kubernetes==nil: want 0, got %d", got)
	}
	if got := v.AllocPods(); got != 0 {
		t.Fatalf("AllocPods() Kubernetes==nil: want 0, got %d", got)
	}

	// Non-Kubernetes accessors must still project the outer Resource values.
	if got := v.ID(); got != "k8snode-no-k8s" {
		t.Fatalf("ID() Kubernetes==nil: want %q, got %q", "k8snode-no-k8s", got)
	}
	if got := v.Name(); got != "worker-bare" {
		t.Fatalf("Name() Kubernetes==nil: want %q, got %q", "worker-bare", got)
	}
	if got := v.Status(); got != StatusOffline {
		t.Fatalf("Status() Kubernetes==nil: want %q, got %q", StatusOffline, got)
	}
	if got := v.CPUPercent(); got != 0 {
		t.Fatalf("CPUPercent() Metrics==nil: want 0, got %v", got)
	}
	if got := v.MemoryPercent(); got != 0 {
		t.Fatalf("MemoryPercent() Metrics==nil: want 0, got %v", got)
	}
	assertStringSlice(t, v.Tags(), []string{"bare"})
	if got := v.LastSeen(); !got.Equal(now) {
		t.Fatalf("LastSeen() Kubernetes==nil: want %v, got %v", now, got)
	}
	if got := v.ParentID(); got != parentID {
		t.Fatalf("ParentID() Kubernetes==nil: want %q, got %q", parentID, got)
	}
}

// TestK8sNodeViewBranchcov0719late_Populated covers the fully-populated arm of
// every accessor and verifies that slice-returning accessors (Roles, Tags)
// produce independent copies (defensive clone contract).
func TestK8sNodeViewBranchcov0719late_Populated(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	parentID := "cluster-parent-1"
	r := k8sNodeBranchcov0719lateResource(now, parentID)
	v := NewK8sNodeView(r)

	if got := v.ID(); got != "k8snode-1" {
		t.Fatalf("ID(): want %q, got %q", "k8snode-1", got)
	}
	if got := v.Name(); got != "worker-1" {
		t.Fatalf("Name(): want %q, got %q", "worker-1", got)
	}
	if got := v.Status(); got != StatusOnline {
		t.Fatalf("Status(): want %q, got %q", StatusOnline, got)
	}
	if got := v.ClusterName(); got != "prod-cluster" {
		t.Fatalf("ClusterName(): want %q, got %q", "prod-cluster", got)
	}
	if got := v.NodeUID(); got != "uid-node-1" {
		t.Fatalf("NodeUID(): want %q, got %q", "uid-node-1", got)
	}
	if got := v.NodeName(); got != "worker-1" {
		t.Fatalf("NodeName(): want %q, got %q", "worker-1", got)
	}
	if !v.Ready() {
		t.Fatalf("Ready(): want true, got false")
	}
	if !v.Unschedulable() {
		t.Fatalf("Unschedulable(): want true, got false")
	}
	assertStringSlice(t, v.Roles(), []string{"control-plane", "worker"})
	if got := v.KubeletVersion(); got != "v1.31.0" {
		t.Fatalf("KubeletVersion(): want %q, got %q", "v1.31.0", got)
	}
	if got := v.ContainerRuntimeVersion(); got != "containerd://1.7.20" {
		t.Fatalf("ContainerRuntimeVersion(): want %q, got %q", "containerd://1.7.20", got)
	}
	if got := v.OSImage(); got != "Ubuntu 24.04 LTS" {
		t.Fatalf("OSImage(): want %q, got %q", "Ubuntu 24.04 LTS", got)
	}
	if got := v.KernelVersion(); got != "6.8.0-45-generic" {
		t.Fatalf("KernelVersion(): want %q, got %q", "6.8.0-45-generic", got)
	}
	if got := v.Architecture(); got != "amd64" {
		t.Fatalf("Architecture(): want %q, got %q", "amd64", got)
	}
	if got := v.CapacityCPU(); got != 16 {
		t.Fatalf("CapacityCPU(): want 16, got %d", got)
	}
	const capMem = int64(32) * 1024 * 1024 * 1024
	if got := v.CapacityMemoryBytes(); got != capMem {
		t.Fatalf("CapacityMemoryBytes(): want %d, got %d", capMem, got)
	}
	if got := v.CapacityPods(); got != 110 {
		t.Fatalf("CapacityPods(): want 110, got %d", got)
	}
	if got := v.AllocCPU(); got != 15 {
		t.Fatalf("AllocCPU(): want 15, got %d", got)
	}
	const allocMem = int64(30) * 1024 * 1024 * 1024
	if got := v.AllocMemoryBytes(); got != allocMem {
		t.Fatalf("AllocMemoryBytes(): want %d, got %d", allocMem, got)
	}
	if got := v.AllocPods(); got != 100 {
		t.Fatalf("AllocPods(): want 100, got %d", got)
	}
	if got := v.CPUPercent(); got != 42.5 {
		t.Fatalf("CPUPercent(): want %v, got %v", 42.5, got)
	}
	if got := v.MemoryPercent(); got != 67.25 {
		t.Fatalf("MemoryPercent(): want %v, got %v", 67.25, got)
	}
	assertStringSlice(t, v.Tags(), []string{"k8s", "role:worker"})
	if got := v.LastSeen(); !got.Equal(now) {
		t.Fatalf("LastSeen(): want %v, got %v", now, got)
	}
	if got := v.ParentID(); got != parentID {
		t.Fatalf("ParentID(): want %q, got %q", parentID, got)
	}

	// Defensive-clone contract: mutating returned slices must not leak back
	// into the backing *Resource.
	roles := v.Roles()
	if len(roles) != 2 {
		t.Fatalf("Roles(): want len 2, got %d", len(roles))
	}
	roles[0] = "MUTATED"
	roles = append(roles, "extra")
	if got := v.Roles(); len(got) != 2 || got[0] != "control-plane" || got[2-1] != "worker" {
		t.Fatalf("Roles() independence: want [control-plane worker], got %v", got)
	}
	if r.Kubernetes.Roles[0] != "control-plane" {
		t.Fatalf("Roles() mutation leaked into backing Resource: got %v", r.Kubernetes.Roles)
	}

	tags := v.Tags()
	if len(tags) != 2 {
		t.Fatalf("Tags(): want len 2, got %d", len(tags))
	}
	tags[0] = "MUTATED"
	tags = append(tags, "extra")
	if got := v.Tags(); len(got) != 2 || got[0] != "k8s" || got[1] != "role:worker" {
		t.Fatalf("Tags() independence: want [k8s role:worker], got %v", got)
	}
	if r.Tags[0] != "k8s" {
		t.Fatalf("Tags() mutation leaked into backing Resource: got %v", r.Tags)
	}
}

// TestK8sNodeViewBranchcov0719late_NilParentID covers the v.r.ParentID == nil
// arm of ParentID() specifically (r is non-nil, Kubernetes is populated, but
// ParentID is nil).
func TestK8sNodeViewBranchcov0719late_NilParentID(t *testing.T) {
	now := time.Date(2026, 7, 19, 11, 0, 0, 0, time.UTC)
	r := &Resource{
		ID:       "k8snode-no-parent",
		Type:     ResourceTypeK8sNode,
		Name:     "orphan-node",
		Status:   StatusOnline,
		LastSeen: now,
		Kubernetes: &K8sData{
			NodeName: "orphan-node",
			Ready:    true,
		},
	}
	v := NewK8sNodeView(r)
	if got := v.ParentID(); got != "" {
		t.Fatalf("ParentID() ParentID==nil: want %q, got %q", "", got)
	}
	if got := v.NodeName(); got != "orphan-node" {
		t.Fatalf("NodeName() populated arm: want %q, got %q", "orphan-node", got)
	}
}

// TestK8sNodeViewBranchcov0719late_StringAndCtor covers the String() formatter
// and the NewK8sNodeView constructor wrapping behavior on a populated
// resource.
func TestK8sNodeViewBranchcov0719late_StringAndCtor(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	parentID := "cluster-parent-1"
	r := k8sNodeBranchcov0719lateResource(now, parentID)
	v := NewK8sNodeView(r)

	want := `K8sNodeView(k8snode-1, "worker-1")`
	if got := v.String(); got != want {
		t.Fatalf("String(): want %q, got %q", want, got)
	}

	// NewK8sNodeView must wrap the provided *Resource (ID() round-trips the
	// same value the Resource carries).
	if v.ID() != r.ID {
		t.Fatalf("NewK8sNodeView wrap: want ID %q, got %q", r.ID, v.ID())
	}
}

// TestK8sNodeViewBranchcov0719late_EmptyRolesAndTags covers the edge case
// where Kubernetes is non-nil but Roles is empty/nil, and Tags is empty/nil.
// cloneStringSlice(nil) returns nil; cloneStringSlice([]string{}) returns an
// empty (non-nil) slice — both must be handled without panic.
func TestK8sNodeViewBranchcov0719late_EmptyRolesAndTags(t *testing.T) {
	r := &Resource{
		ID:       "k8snode-empty-slices",
		Type:     ResourceTypeK8sNode,
		Name:     "empty-node",
		Status:   StatusOnline,
		LastSeen: time.Date(2026, 7, 19, 13, 0, 0, 0, time.UTC),
		Kubernetes: &K8sData{
			NodeName: "empty-node",
			// Roles intentionally nil.
		},
		// Tags intentionally nil.
	}
	v := NewK8sNodeView(r)
	if got := v.Roles(); got != nil {
		t.Fatalf("Roles() nil-slice: want nil, got %v", got)
	}
	if got := v.Tags(); got != nil {
		t.Fatalf("Tags() nil-slice: want nil, got %v", got)
	}

	// And the empty (non-nil) slice variant for both.
	r.Kubernetes.Roles = []string{}
	r.Tags = []string{}
	if got := v.Roles(); len(got) != 0 {
		t.Fatalf("Roles() empty-slice: want len 0, got %d (%v)", len(got), got)
	}
	if got := v.Tags(); len(got) != 0 {
		t.Fatalf("Tags() empty-slice: want len 0, got %d (%v)", len(got), got)
	}
}
