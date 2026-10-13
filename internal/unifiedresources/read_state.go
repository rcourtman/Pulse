package unifiedresources

// ReadState provides typed read access to current infrastructure state.
//
// This is the interface that internal consumers (AI, Patrol, API handlers)
// should depend on instead of models.StateSnapshot. It exposes only typed
// view accessors. Explicit selection uses the ReferenceReadState capability;
// raw enumeration methods (ListByType, List, Get) remain on ResourceRegistry.
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
	DockerContainers() []*DockerContainerView

	// Storage
	StoragePools() []*StoragePoolView
	PhysicalDisks() []*PhysicalDiskView

	// Backup & mail
	PBSInstances() []*PBSInstanceView
	PMGInstances() []*PMGInstanceView

	// Kubernetes
	K8sClusters() []*K8sClusterView
	// Kubernetes sub-resources
	K8sNodes() []*K8sNodeView
	Pods() []*PodView
	K8sDeployments() []*K8sDeploymentView

	// Polymorphic accessors for mixed-type iteration
	Workloads() []*WorkloadView            // Canonical runtime workloads (VMs, system containers, app containers)
	Infrastructure() []*InfrastructureView // All infrastructure parent resources
}

// ReferenceReadState resolves explicit resource references through the registry.
// It preserves declared successions and refuses ambiguous aliases. Production
// read-state providers implement this capability without exposing a raw list.
type ReferenceReadState interface {
	ReadState
	GetByReference(string) (*Resource, string, bool)
}

var _ ReferenceReadState = (*ResourceRegistry)(nil)
var _ ReferenceReadState = (*MonitorAdapter)(nil)
