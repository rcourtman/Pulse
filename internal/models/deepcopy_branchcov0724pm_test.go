package models

import (
	"reflect"
	"testing"
	"time"
)

// bc0724pmT is a stable timestamp reused across these branch-coverage tests;
// it is deliberately distinct from w0716dcT so the independence assertions
// (which compare clone values against an immutable reference) stay legible.
var bc0724pmT = time.Date(2025, 7, 24, 13, 45, 0, 0, time.UTC)

// This file exercises the previously-uncovered ARMS of the deep-copy helpers
// in deepcopy.go / converters.go and the cephClusterSourceRank ranking
// function in ceph_cluster_identity.go. For every clone helper it covers:
//   - the nil-input guard (returns nil),
//   - the empty-collection path (collapsed/normalized as the source dictates),
//   - the fully-populated path,
//   - INDEPENDENCE: mutating every nested slice, map and pointer field of the
//     returned CLONE must leave the original input completely untouched (a
//     shared backing array/map would alias and is a real clone-correctness bug).
//
// cephClusterSourceRank is a plain ranking switch; the provably-unreachable
// default arm is documented inline rather than faked.

// ---------------------------------------------------------------------------
// deepcopy.go: cloneHostUnraidStorage (was 75.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneHostUnraidStorage(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneHostUnraidStorage(nil); got != nil {
			t.Fatalf("cloneHostUnraidStorage(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty disks normalized to non-nil empty slice", func(t *testing.T) {
		src := &HostUnraidStorage{ArrayStarted: true, ArrayState: "Started", Disks: nil}
		got := cloneHostUnraidStorage(src)
		if got == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if !got.ArrayStarted || got.ArrayState != "Started" {
			t.Fatalf("scalar fields not preserved: %#v", got)
		}
		// The len==0 branch skips the copy; NormalizeCollections must still
		// materialise a non-nil empty slice so the clone never carries a nil.
		if got.Disks == nil {
			t.Fatal("Disks must normalize to a non-nil empty slice, got nil")
		}
		if len(got.Disks) != 0 {
			t.Fatalf("len(Disks) = %d, want 0", len(got.Disks))
		}
	})

	t.Run("populated disks are deep-copied and isolated", func(t *testing.T) {
		src := &HostUnraidStorage{
			ArrayStarted: true,
			SyncProgress: 42.5,
			Disks: []HostUnraidDisk{
				{Name: "parity", Device: "/dev/sda", Status: "DISK_OK"},
				{Name: "disk1", Device: "/dev/sdb", Status: "DISK_OK"},
			},
		}
		clone := cloneHostUnraidStorage(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if len(clone.Disks) != 2 || clone.Disks[0].Name != "parity" || clone.Disks[1].Device != "/dev/sdb" {
			t.Fatalf("disks not preserved: %#v", clone.Disks)
		}
		if clone.SyncProgress != 42.5 {
			t.Fatalf("SyncProgress = %v, want 42.5", clone.SyncProgress)
		}

		// Mutate the clone's nested Disks slice; the original must be untouched.
		clone.Disks[0].Name = "MUTATED"
		clone.Disks[0].Status = "MUTATED"
		if src.Disks[0].Name != "parity" || src.Disks[0].Status != "DISK_OK" {
			t.Errorf("Disks slice aliases source backing array: src[0]=%#v", src.Disks[0])
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneHostCephHealth (was 62.5%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneHostCephHealth(t *testing.T) {
	t.Run("empty checks collapses to nil then re-normalized", func(t *testing.T) {
		src := HostCephHealth{Status: "HEALTH_OK"}
		got := cloneHostCephHealth(src)
		if got.Status != "HEALTH_OK" {
			t.Fatalf("Status = %q, want HEALTH_OK", got.Status)
		}
		// The len==0 branch sets Checks=nil; NormalizeCollections must give back
		// a non-nil empty map so downstream code never sees a nil map.
		if got.Checks == nil {
			t.Fatal("Checks must normalize to non-nil empty map, got nil")
		}
		if len(got.Checks) != 0 {
			t.Fatalf("len(Checks) = %d, want 0", len(got.Checks))
		}
		if got.Summary == nil {
			t.Fatal("Summary must normalize to non-nil empty slice, got nil")
		}
	})

	t.Run("populated checks and summary are deep-copied and isolated", func(t *testing.T) {
		src := HostCephHealth{
			Status: "HEALTH_WARN",
			Checks: map[string]HostCephCheck{
				"OSD_DOWN": {Severity: "warning", Message: "1 osd down", Detail: []string{"osd.3 is down", "host node2"}},
			},
			Summary: []HostCephHealthSummary{
				{Severity: "warning", Message: "1 osds down"},
			},
		}
		clone := cloneHostCephHealth(src)
		if clone.Status != "HEALTH_WARN" {
			t.Fatalf("Status = %q, want HEALTH_WARN", clone.Status)
		}
		if len(clone.Checks) != 1 {
			t.Fatalf("len(Checks) = %d, want 1", len(clone.Checks))
		}
		gotCheck := clone.Checks["OSD_DOWN"]
		if gotCheck.Severity != "warning" || gotCheck.Message != "1 osd down" {
			t.Fatalf("check scalar fields not preserved: %#v", gotCheck)
		}
		if len(gotCheck.Detail) != 2 || gotCheck.Detail[0] != "osd.3 is down" {
			t.Fatalf("check Detail not deep-copied: %#v", gotCheck.Detail)
		}

		// Mutate the CLONE's nested check (Detail slice + scalar) and add a key;
		// the source Checks map and its Detail slice must stay untouched.
		mut := clone.Checks["OSD_DOWN"]
		mut.Detail[0] = "MUTATED"
		mut.Severity = "MUTATED"
		clone.Checks["OSD_DOWN"] = mut
		clone.Checks["leak"] = HostCephCheck{Severity: "x"}
		if src.Checks["OSD_DOWN"].Detail[0] != "osd.3 is down" {
			t.Errorf("Checks Detail slice aliases source: %q", src.Checks["OSD_DOWN"].Detail[0])
		}
		if src.Checks["OSD_DOWN"].Severity != "warning" {
			t.Errorf("Checks severity not isolated: %q", src.Checks["OSD_DOWN"].Severity)
		}
		if _, leak := src.Checks["leak"]; leak {
			t.Error("adding a key to clone Checks leaked into source map")
		}

		// Mutate the CLONE's Summary slice; the source Summary must stay untouched.
		if len(clone.Summary) != 1 || clone.Summary[0].Message != "1 osds down" {
			t.Fatalf("Summary not deep-copied: %#v", clone.Summary)
		}
		clone.Summary[0].Message = "MUTATED"
		clone.Summary[0].Severity = "MUTATED"
		if src.Summary[0].Message != "1 osds down" || src.Summary[0].Severity != "warning" {
			t.Errorf("Summary slice aliases source: %#v", src.Summary[0])
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneHostCephCluster (was 80.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneHostCephCluster(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneHostCephCluster(nil); got != nil {
			t.Fatalf("cloneHostCephCluster(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty services normalized to non-nil empty slice", func(t *testing.T) {
		src := &HostCephCluster{FSID: "fsid-1", Services: nil}
		got := cloneHostCephCluster(src)
		if got == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if got.FSID != "fsid-1" {
			t.Fatalf("FSID = %q, want fsid-1", got.FSID)
		}
		// The len==0 branch sets Services=nil; NormalizeCollections re-materialises
		// a non-nil empty slice.
		if got.Services == nil {
			t.Fatal("Services must normalize to non-nil empty slice, got nil")
		}
		if len(got.Services) != 0 {
			t.Fatalf("len(Services) = %d, want 0", len(got.Services))
		}
	})

	t.Run("populated monitors pools services health are deep-copied and isolated", func(t *testing.T) {
		src := &HostCephCluster{
			FSID: "fsid-1",
			Health: HostCephHealth{
				Status: "HEALTH_OK",
				Checks: map[string]HostCephCheck{
					"POOL_NEAR_FULL": {Severity: "warning", Detail: []string{"pool rbd near full"}},
				},
			},
			MonMap: HostCephMonitorMap{
				Epoch:   3,
				NumMons: 1,
				Monitors: []HostCephMonitor{
					{Name: "mon-a", Rank: 0, Addr: "10.0.0.1:6789", Status: "ok"},
				},
			},
			Pools: []HostCephPool{{ID: 1, Name: "rbd", PercentUsed: 85.0}},
			Services: []HostCephService{
				{Type: "mon", Running: 1, Total: 1, Daemons: []string{"mon.a", "mon.b"}},
			},
		}
		clone := cloneHostCephCluster(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone.FSID != "fsid-1" || len(clone.Pools) != 1 || clone.Pools[0].Name != "rbd" {
			t.Fatalf("scalar/pool fields not preserved: %#v", clone)
		}
		if len(clone.MonMap.Monitors) != 1 || clone.MonMap.Monitors[0].Name != "mon-a" {
			t.Fatalf("monitors not preserved: %#v", clone.MonMap.Monitors)
		}
		if len(clone.Services) != 1 || len(clone.Services[0].Daemons) != 2 {
			t.Fatalf("services not preserved: %#v", clone.Services)
		}

		// Mutate every nested collection of the CLONE; the source must be untouched.
		clone.MonMap.Monitors[0].Name = "MUTATED"
		clone.MonMap.Monitors[0].Status = "MUTATED"
		clone.Pools[0].Name = "MUTATED"
		clone.Services[0].Daemons[0] = "MUTATED"
		mutCheck := clone.Health.Checks["POOL_NEAR_FULL"]
		mutCheck.Detail[0] = "MUTATED"
		clone.Health.Checks["POOL_NEAR_FULL"] = mutCheck
		clone.Health.Checks["leak"] = HostCephCheck{Severity: "x"}

		if src.MonMap.Monitors[0].Name != "mon-a" || src.MonMap.Monitors[0].Status != "ok" {
			t.Errorf("MonMap.Monitors aliases source: %#v", src.MonMap.Monitors[0])
		}
		if src.Pools[0].Name != "rbd" {
			t.Errorf("Pools slice aliases source: %q", src.Pools[0].Name)
		}
		if src.Services[0].Daemons[0] != "mon.a" {
			t.Errorf("Services Daemons slice aliases source: %q", src.Services[0].Daemons[0])
		}
		if src.Health.Checks["POOL_NEAR_FULL"].Detail[0] != "pool rbd near full" {
			t.Errorf("Health Checks Detail aliases source: %q", src.Health.Checks["POOL_NEAR_FULL"].Detail[0])
		}
		if _, leak := src.Health.Checks["leak"]; leak {
			t.Error("adding a key to clone Health Checks leaked into source map")
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerPodmanContainer (was 50.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerPodmanContainer(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerPodmanContainer(nil); got != nil {
			t.Fatalf("cloneDockerPodmanContainer(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated returns distinct pointer with preserved values", func(t *testing.T) {
		src := &DockerPodmanContainer{
			PodName:        "web-pod",
			PodID:          "pod-123",
			Infra:          true,
			ComposeProject: "app",
		}
		clone := cloneDockerPodmanContainer(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if clone.PodName != "web-pod" || clone.PodID != "pod-123" || !clone.Infra || clone.ComposeProject != "app" {
			t.Fatalf("fields not preserved: %#v", clone)
		}
		// Mutate clone scalar fields; a value copy must not propagate back to src.
		clone.PodName = "MUTATED"
		clone.Infra = false
		if src.PodName != "web-pod" || !src.Infra {
			t.Errorf("source struct mutated by clone write: %#v", src)
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerContainerUpdateStatus (was 50.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerContainerUpdateStatus(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerContainerUpdateStatus(nil); got != nil {
			t.Fatalf("cloneDockerContainerUpdateStatus(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated returns distinct pointer with preserved values", func(t *testing.T) {
		src := &DockerContainerUpdateStatus{
			UpdateAvailable: true,
			CurrentDigest:   "sha256:aaa",
			LatestDigest:    "sha256:bbb",
			LastChecked:     bc0724pmT,
			Error:           "rate limited",
		}
		clone := cloneDockerContainerUpdateStatus(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if !clone.UpdateAvailable || clone.CurrentDigest != "sha256:aaa" ||
			clone.LatestDigest != "sha256:bbb" || clone.Error != "rate limited" ||
			!clone.LastChecked.Equal(bc0724pmT) {
			t.Fatalf("fields not preserved: %#v", clone)
		}
		// Mutate clone scalar fields; a value copy must not propagate back to src.
		clone.UpdateAvailable = false
		clone.Error = "MUTATED"
		if !src.UpdateAvailable || src.Error != "rate limited" {
			t.Errorf("source struct mutated by clone write: %#v", src)
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerStorageUsage (was 50.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerStorageUsage(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerStorageUsage(nil); got != nil {
			t.Fatalf("cloneDockerStorageUsage(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated returns distinct pointer with preserved values", func(t *testing.T) {
		src := &DockerStorageUsage{
			Images:     DockerStorageUsageBucket{TotalCount: 5, TotalSizeBytes: 1024},
			Containers: DockerStorageUsageBucket{TotalCount: 10, ReclaimableBytes: 512},
			Volumes:    DockerStorageUsageBucket{TotalCount: 3, ActiveCount: 2},
			BuildCache: DockerStorageUsageBucket{TotalCount: 1, ReclaimableBytes: 256},
		}
		clone := cloneDockerStorageUsage(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if clone.Images.TotalCount != 5 || clone.Containers.ReclaimableBytes != 512 ||
			clone.Volumes.ActiveCount != 2 || clone.BuildCache.TotalSizeBytes != 0 {
			t.Fatalf("fields not preserved: %#v", clone)
		}
		// Mutate clone nested value fields; a value copy must not propagate to src.
		clone.Images.TotalCount = 999
		clone.Containers.ReclaimableBytes = 999
		if src.Images.TotalCount != 5 || src.Containers.ReclaimableBytes != 512 {
			t.Errorf("source struct mutated by clone write: %#v", src)
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerServiceUpdate (was 40.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerServiceUpdate(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerServiceUpdate(nil); got != nil {
			t.Fatalf("cloneDockerServiceUpdate(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated with nil CompletedAt preserved", func(t *testing.T) {
		src := &DockerServiceUpdate{State: "completed", Message: "done"}
		clone := cloneDockerServiceUpdate(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone.CompletedAt != nil {
			t.Fatalf("CompletedAt = %#v, want nil when source has none", clone.CompletedAt)
		}
		if clone.State != "completed" || clone.Message != "done" {
			t.Fatalf("scalar fields not preserved: %#v", clone)
		}
	})

	t.Run("populated CompletedAt pointer deep-copied and isolated", func(t *testing.T) {
		completed := bc0724pmT
		src := &DockerServiceUpdate{State: "completed", CompletedAt: &completed}
		clone := cloneDockerServiceUpdate(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone.CompletedAt == nil {
			t.Fatal("CompletedAt dropped during clone")
		}
		if clone.CompletedAt == src.CompletedAt {
			t.Fatal("CompletedAt aliases the source pointer instead of allocating a new *time.Time")
		}
		if !clone.CompletedAt.Equal(bc0724pmT) {
			t.Fatalf("CompletedAt = %v, want %v", clone.CompletedAt, bc0724pmT)
		}
		// Mutate the pointed-to time through the CLONE; the source must be untouched.
		*clone.CompletedAt = time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
		if !src.CompletedAt.Equal(bc0724pmT) {
			t.Errorf("CompletedAt pointer not isolated: src = %v", src.CompletedAt)
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerSwarmInfo (was 50.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerSwarmInfo(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerSwarmInfo(nil); got != nil {
			t.Fatalf("cloneDockerSwarmInfo(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated returns distinct pointer with preserved values", func(t *testing.T) {
		src := &DockerSwarmInfo{
			NodeID:           "node-1",
			NodeRole:         "manager",
			LocalState:       "active",
			ControlAvailable: true,
			ClusterID:        "cluster-1",
			ClusterName:      "prod",
			Scope:            "swarm",
			Error:            "",
		}
		clone := cloneDockerSwarmInfo(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if clone.NodeID != "node-1" || clone.NodeRole != "manager" || !clone.ControlAvailable ||
			clone.ClusterName != "prod" {
			t.Fatalf("fields not preserved: %#v", clone)
		}
		// Mutate clone scalar fields; a value copy must not propagate to src.
		clone.NodeRole = "MUTATED"
		clone.ControlAvailable = false
		if src.NodeRole != "manager" || !src.ControlAvailable {
			t.Errorf("source struct mutated by clone write: %#v", src)
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneDockerHostSecurity (was 40.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDockerHostSecurity(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDockerHostSecurity(nil); got != nil {
			t.Fatalf("cloneDockerHostSecurity(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated with empty plugins preserved", func(t *testing.T) {
		src := &DockerHostSecurity{MutatingCommandsBlocked: true, MutatingCommandsBlockedReason: "rootful"}
		got := cloneDockerHostSecurity(src)
		if got == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if got.AuthorizationPlugins != nil {
			t.Fatalf("AuthorizationPlugins = %#v, want nil for empty source", got.AuthorizationPlugins)
		}
		if !got.MutatingCommandsBlocked || got.MutatingCommandsBlockedReason != "rootful" {
			t.Fatalf("scalar fields not preserved: %#v", got)
		}
	})

	t.Run("populated plugins slice deep-copied and isolated", func(t *testing.T) {
		src := &DockerHostSecurity{
			AuthorizationPlugins:    []string{"opa", "opa-authz"},
			MutatingCommandsBlocked: true,
		}
		clone := cloneDockerHostSecurity(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if len(clone.AuthorizationPlugins) != 2 || clone.AuthorizationPlugins[0] != "opa" {
			t.Fatalf("AuthorizationPlugins not preserved: %#v", clone.AuthorizationPlugins)
		}
		// Mutate the CLONE's plugins slice; the source slice must be untouched.
		clone.AuthorizationPlugins[0] = "MUTATED"
		if src.AuthorizationPlugins[0] != "opa" {
			t.Errorf("AuthorizationPlugins slice aliases source: %q", src.AuthorizationPlugins[0])
		}
	})
}

// ---------------------------------------------------------------------------
// deepcopy.go: cloneKubernetesNamespaces (was 25.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneKubernetesNamespaces(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneKubernetesNamespaces(nil); got != nil {
			t.Fatalf("cloneKubernetesNamespaces(nil) = %#v, want nil", got)
		}
	})

	t.Run("empty returns nil", func(t *testing.T) {
		if got := cloneKubernetesNamespaces([]KubernetesNamespace{}); got != nil {
			t.Fatalf("cloneKubernetesNamespaces(empty) = %#v, want nil", got)
		}
	})

	t.Run("populated labels are deep-copied and isolated", func(t *testing.T) {
		src := []KubernetesNamespace{
			{
				UID:   "ns-1",
				Name:  "default",
				Phase: "Active",
				Labels: map[string]string{
					"kubernetes.io/metadata.name": "default",
				},
			},
			{
				UID:    "ns-2",
				Name:   "kube-system",
				Phase:  "Active",
				Labels: map[string]string{"app": "coredns"},
			},
		}
		clone := cloneKubernetesNamespaces(src)
		if !reflect.DeepEqual(src, clone) {
			t.Fatalf("value mismatch: src=%#v clone=%#v", src, clone)
		}

		// Mutate the CLONE's nested Labels maps; the source maps must be untouched.
		clone[0].Labels["kubernetes.io/metadata.name"] = "MUTATED"
		clone[0].Labels["leak"] = "x"
		clone[1].Labels["app"] = "MUTATED"
		if src[0].Labels["kubernetes.io/metadata.name"] != "default" {
			t.Errorf("Labels map aliases source: %q", src[0].Labels["kubernetes.io/metadata.name"])
		}
		if _, leak := src[0].Labels["leak"]; leak {
			t.Error("adding a key to clone Labels leaked into source map")
		}
		if src[1].Labels["app"] != "coredns" {
			t.Errorf("Labels map aliases source: %q", src[1].Labels["app"])
		}
	})

	t.Run("nil labels normalized to non-nil empty map", func(t *testing.T) {
		src := []KubernetesNamespace{{UID: "ns-3", Name: "blank"}}
		clone := cloneKubernetesNamespaces(src)
		if clone == nil || len(clone) != 1 {
			t.Fatalf("clone = %#v, want one element", clone)
		}
		// cloneStringMap(nil) returns nil; NormalizeCollections must materialise a
		// non-nil empty map so consumers never observe a nil Labels.
		if clone[0].Labels == nil {
			t.Fatal("Labels must normalize to non-nil empty map, got nil")
		}
		if len(clone[0].Labels) != 0 {
			t.Fatalf("len(Labels) = %d, want 0", len(clone[0].Labels))
		}
	})
}

// ---------------------------------------------------------------------------
// converters.go: cloneDiskIOForFrontend (was 50.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCloneDiskIOForFrontend(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneDiskIOForFrontend(nil); got != nil {
			t.Fatalf("cloneDiskIOForFrontend(nil) = %#v, want nil", got)
		}
	})

	t.Run("populated returns distinct pointer with preserved values", func(t *testing.T) {
		src := &DiskIO{
			Device:     "/dev/sda",
			ReadBytes:  1024,
			WriteBytes: 2048,
			ReadOps:    10,
			WriteOps:   20,
			ReadTime:   100,
			WriteTime:  200,
			IOTime:     500,
		}
		clone := cloneDiskIOForFrontend(src)
		if clone == nil {
			t.Fatal("non-nil input returned nil clone")
		}
		if clone == src {
			t.Fatal("clone aliases the source pointer instead of allocating a new struct")
		}
		if clone.Device != "/dev/sda" || clone.ReadBytes != 1024 || clone.WriteOps != 20 ||
			clone.ReadTime != 100 || clone.IOTime != 500 {
			t.Fatalf("fields not preserved: %#v", clone)
		}
		// Mutate clone scalar fields; a value copy must not propagate to src.
		clone.ReadBytes = 999999
		clone.IOTime = 999999
		if src.ReadBytes != 1024 || src.IOTime != 500 {
			t.Errorf("source struct mutated by clone write: %#v", src)
		}
	})
}

// ---------------------------------------------------------------------------
// ceph_cluster_identity.go: cephClusterSourceRank (was 75.0%)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCephClusterSourceRank(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cluster  CephCluster
		wantRank int
	}{
		{"proxmox-api canonical source ranks 2", CephCluster{Source: CephClusterSourceProxmoxAPI}, 2},
		{"proxmox alias source ranks 2", CephCluster{Source: "pve"}, 2},
		{"api alias source ranks 2", CephCluster{Source: "api"}, 2},
		{"host-agent canonical source ranks 1", CephCluster{Source: CephClusterSourceHostAgent}, 1},
		{"agent alias source ranks 1", CephCluster{Source: "agent"}, 1},
		{"host alias source ranks 1", CephCluster{Source: "host"}, 1},
		{"agent: instance prefix ranks 1", CephCluster{Instance: "agent:node1"}, 1},
		// An "unknown" source does NOT rank 0: normalizeCephClusterSource's final
		// fallback coerces anything unrecognized to proxmox-api, so it ranks 2.
		{"unknown source coerces to proxmox-api rank 2", CephCluster{Source: "who-knows"}, 2},
		{"empty source and instance coerces to proxmox-api rank 2", CephCluster{}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cephClusterSourceRank(tc.cluster); got != tc.wantRank {
				t.Fatalf("cephClusterSourceRank(cluster{Source:%q,Instance:%q}) = %d, want %d",
					tc.cluster.Source, tc.cluster.Instance, got, tc.wantRank)
			}
		})
	}

	// The `default: return 0` arm of cephClusterSourceRank is PROVABLY
	// UNREACHABLE: it switches on normalizeCephClusterSource(...), which only
	// ever returns CephClusterSourceProxmoxAPI or CephClusterSourceHostAgent
	// (its only two return values). No input can produce any other value, so
	// the default case is dead code and cannot be covered without faking it.
	// See the report for details.
}
