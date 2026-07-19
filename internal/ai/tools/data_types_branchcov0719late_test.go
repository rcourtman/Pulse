package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDataTypesBranchcov0719lateDockerHostSummaryNormalize covers
// DockerHostSummary.NormalizeCollections. The method only nil-normalizes
// the Containers slice (it does not sort or dedup); tests assert the actual
// behavior: nil -> empty slice, populated -> unchanged passthrough, empty -> stable.
func TestDataTypesBranchcov0719lateDockerHostSummaryNormalize(t *testing.T) {
	t.Run("NilContainersBecomesEmptySlice", func(t *testing.T) {
		s := DockerHostSummary{ID: "host-1", Hostname: "dock-1"}
		assert.Nil(t, s.Containers, "precondition: input Containers must be nil")

		out := s.NormalizeCollections()

		require.NotNil(t, out.Containers, "nil Containers must become a non-nil empty slice")
		assert.Len(t, out.Containers, 0)
		assert.Equal(t, "host-1", out.ID, "scalar fields must be preserved")
		assert.Equal(t, "dock-1", out.Hostname)
	})

	t.Run("ZeroValueContainersBecomesEmptySlice", func(t *testing.T) {
		var s DockerHostSummary
		assert.Nil(t, s.Containers)

		out := s.NormalizeCollections()

		require.NotNil(t, out.Containers)
		assert.Len(t, out.Containers, 0)
	})

	t.Run("PopulatedContainersPreservedUnchanged", func(t *testing.T) {
		// Unsorted, duplicated, multi-element input — NormalizeCollections does
		// not sort/dedup, so contents must pass through in original order.
		original := []DockerContainerSummary{
			{ID: "c2", Name: "zeta", State: "running"},
			{ID: "c1", Name: "alpha", State: "stopped"},
			{ID: "c1", Name: "alpha", State: "stopped"}, // intentional duplicate
		}
		s := DockerHostSummary{
			ID:         "host-1",
			Hostname:   "dock-1",
			Containers: original,
		}

		out := s.NormalizeCollections()

		require.Len(t, out.Containers, 3, "populated slice length must be unchanged")
		assert.Equal(t, []DockerContainerSummary(original), out.Containers,
			"contents and order must be preserved exactly when not nil")
		// Same backing array (no copy expected): identity check on first element addr.
		assert.True(t, &out.Containers[0] == &original[0],
			"non-nil slice should keep the same backing array (no defensive copy in source)")
	})

	t.Run("EmptyContainersStable", func(t *testing.T) {
		s := DockerHostSummary{Containers: []DockerContainerSummary{}}

		out := s.NormalizeCollections()

		require.NotNil(t, out.Containers)
		assert.Len(t, out.Containers, 0)
	})
}

// TestDataTypesBranchcov0719lateK8sNodeSummaryNormalize covers
// K8sNodeSummary.NormalizeCollections. Same shape as Docker: nil Roles
// becomes an empty slice; populated Roles passes through unchanged.
func TestDataTypesBranchcov0719lateK8sNodeSummaryNormalize(t *testing.T) {
	t.Run("NilRolesBecomesEmptySlice", func(t *testing.T) {
		s := K8sNodeSummary{Name: "n1", Cluster: "c1", Status: "Ready", Ready: true}
		assert.Nil(t, s.Roles)

		out := s.NormalizeCollections()

		require.NotNil(t, out.Roles)
		assert.Len(t, out.Roles, 0)
		assert.Equal(t, "n1", out.Name)
		assert.Equal(t, "c1", out.Cluster)
		assert.True(t, out.Ready)
	})

	t.Run("ZeroValueRolesBecomesEmptySlice", func(t *testing.T) {
		var s K8sNodeSummary
		assert.Nil(t, s.Roles)

		out := s.NormalizeCollections()

		require.NotNil(t, out.Roles)
		assert.Len(t, out.Roles, 0)
	})

	t.Run("PopulatedRolesPreservedUnchanged", func(t *testing.T) {
		// Unsorted + duplicated input; method does not sort/dedup.
		original := []string{"worker", "master", "worker", "etcd"}
		s := K8sNodeSummary{Name: "n1", Roles: original}

		out := s.NormalizeCollections()

		require.Len(t, out.Roles, 4)
		assert.Equal(t, original, out.Roles, "roles must pass through in original order")
	})

	t.Run("EmptyRolesStable", func(t *testing.T) {
		s := K8sNodeSummary{Roles: []string{}}

		out := s.NormalizeCollections()

		require.NotNil(t, out.Roles)
		assert.Len(t, out.Roles, 0)
	})
}

// TestDataTypesBranchcov0719latePVEClusterStatusNormalize covers
// PVEClusterStatus.NormalizeCollections. Nil Nodes becomes empty slice;
// populated Nodes passes through unchanged.
func TestDataTypesBranchcov0719latePVEClusterStatusNormalize(t *testing.T) {
	t.Run("NilNodesBecomesEmptySlice", func(t *testing.T) {
		s := PVEClusterStatus{Instance: "inst-1", ClusterName: "clus-1", QuorumOK: true, TotalNodes: 3}
		assert.Nil(t, s.Nodes)

		out := s.NormalizeCollections()

		require.NotNil(t, out.Nodes)
		assert.Len(t, out.Nodes, 0)
		assert.Equal(t, "inst-1", out.Instance)
		assert.Equal(t, "clus-1", out.ClusterName)
		assert.True(t, out.QuorumOK)
		assert.Equal(t, 3, out.TotalNodes)
	})

	t.Run("ZeroValueNodesBecomesEmptySlice", func(t *testing.T) {
		var s PVEClusterStatus
		assert.Nil(t, s.Nodes)

		out := s.NormalizeCollections()

		require.NotNil(t, out.Nodes)
		assert.Len(t, out.Nodes, 0)
	})

	t.Run("PopulatedNodesPreservedUnchanged", func(t *testing.T) {
		// Multi-element, duplicated input — method does not sort/dedup.
		original := []PVEClusterNodeStatus{
			{Name: "node-b", Status: "online", IsClusterMember: true},
			{Name: "node-a", Status: "online", IsClusterMember: true},
			{Name: "node-b", Status: "online", IsClusterMember: true}, // duplicate
		}
		s := PVEClusterStatus{Instance: "inst-1", Nodes: original}

		out := s.NormalizeCollections()

		require.Len(t, out.Nodes, 3)
		assert.Equal(t, original, out.Nodes, "nodes must pass through in original order")
	})

	t.Run("EmptyNodesStable", func(t *testing.T) {
		s := PVEClusterStatus{Nodes: []PVEClusterNodeStatus{}}

		out := s.NormalizeCollections()

		require.NotNil(t, out.Nodes)
		assert.Len(t, out.Nodes, 0)
	})
}

// TestDataTypesBranchcov0719lateErrExecutionContextUnavailableError covers
// ErrExecutionContextUnavailable.Error(). It returns Message verbatim.
func TestDataTypesBranchcov0719lateErrExecutionContextUnavailableError(t *testing.T) {
	t.Run("ReturnsMessageVerbatim", func(t *testing.T) {
		const msg = "write would execute on the host node instead of inside the system-container"
		err := &ErrExecutionContextUnavailable{
			TargetHost:   "homepage-docker",
			ResolvedKind: "system-container",
			ResolvedNode: "pve-node",
			Transport:    "direct",
			Message:      msg,
		}
		assert.Equal(t, msg, err.Error())
	})

	t.Run("EmptyMessageYieldsEmptyString", func(t *testing.T) {
		err := &ErrExecutionContextUnavailable{}
		assert.Equal(t, "", err.Error())
	})

	t.Run("SatisfiesErrorInterface", func(t *testing.T) {
		var err error = &ErrExecutionContextUnavailable{Message: "boom"}
		assert.Equal(t, "boom", err.Error())
	})
}

// TestDataTypesBranchcov0719lateValidateCurrentResourceAvailable covers
// PulseToolExecutor.ValidateCurrentResourceAvailable across its nil/empty
// error arms and its single-resource OK arm.
func TestDataTypesBranchcov0719lateValidateCurrentResourceAvailable(t *testing.T) {
	t.Run("NilResolvedContextReturnsContextError", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		require.Nil(t, exec.GetResolvedContext(), "precondition: no context set")

		err := exec.ValidateCurrentResourceAvailable()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "current_resource")
		assert.Contains(t, err.Error(), "no Pulse resource context is attached")
	})

	t.Run("EmptyResolvedContextReturnsNoSelectionError", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetResolvedContext(&mockResolvedContext{})

		err := exec.ValidateCurrentResourceAvailable()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "current_resource")
		assert.Contains(t, err.Error(), "no single attached resource is selected")
	})

	t.Run("SingleResourceReturnsNil", func(t *testing.T) {
		res := &mockResource{
			resourceID:  "vm:100",
			kind:        "vm",
			targetHost:  "vm100",
			providerUID: "100",
			aliases:     []string{"vm100", "100"},
		}
		ctx := &mockResolvedContext{
			resources: map[string]ResolvedResourceInfo{
				"vm:100": res,
			},
			lastAccessed: map[string]time.Time{
				"vm:100": time.Now(),
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetResolvedContext(ctx)

		err := exec.ValidateCurrentResourceAvailable()

		assert.NoError(t, err, "a single attached resource must satisfy validation")
	})

	t.Run("MultipleResourcesReturnsAmbiguousError", func(t *testing.T) {
		res1 := &mockResource{resourceID: "vm:100", kind: "vm"}
		res2 := &mockResource{resourceID: "vm:101", kind: "vm"}
		ctx := &mockResolvedContext{
			resources: map[string]ResolvedResourceInfo{
				"vm:100": res1,
				"vm:101": res2,
			},
			lastAccessed: map[string]time.Time{
				"vm:100": time.Now(),
				"vm:101": time.Now(),
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{})
		exec.SetResolvedContext(ctx)

		err := exec.ValidateCurrentResourceAvailable()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "current_resource")
		assert.Contains(t, err.Error(), "ambiguous")
	})
}
