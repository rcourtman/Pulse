package unifiedresources

// ReadState provides typed read access to current infrastructure state.
//
// This is the interface that internal consumers (AI, Patrol, API handlers)
// should depend on instead of models.StateSnapshot. It exposes only typed
// view accessors — generic methods (ListByType, List, Get) remain on
// ResourceRegistry only.
//
// All accessor methods return cached, pre-sorted slices that are O(1) to
// read. The cache is invalidated and lazily rebuilt after each
// IngestSnapshot or IngestRecords call.
type ReadState interface {
	// Proxmox workloads
	VMs() []*VMView
	Containers() []*ContainerView // LXC containers

	// Infrastructure hosts
	Nodes() []*NodeView             // Proxmox nodes (host resources with Proxmox data)
	Hosts() []*HostView             // Host agent resources (host resources with Agent data)
	DockerHosts() []*DockerHostView // Docker host resources (host resources with Docker data)

	// Storage
	StoragePools() []*StoragePoolView

	// Backup & mail
	PBSInstances() []*PBSInstanceView
	PMGInstances() []*PMGInstanceView

	// Kubernetes
	K8sClusters() []*K8sClusterView

	// Polymorphic accessors for mixed-type iteration
	Workloads() []*WorkloadView            // VMs + LXC containers
	Infrastructure() []*InfrastructureView // All host-type resources
}
