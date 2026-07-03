package unifiedresources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
)

const autoMergeThreshold = 0.85
const dockerAdapterRelationshipDiscoverer = "docker_adapter"

var defaultStaleThresholds = map[DataSource]time.Duration{
	SourceProxmox:      60 * time.Second,
	SourceAgent:        60 * time.Second,
	SourceDocker:       120 * time.Second,
	SourcePBS:          120 * time.Second,
	SourcePMG:          120 * time.Second,
	SourceK8s:          120 * time.Second,
	SourceTrueNAS:      120 * time.Second,
	SourceVMware:       120 * time.Second,
	SourceAvailability: 120 * time.Second,
}

func cloneStaleThresholds(thresholds map[DataSource]time.Duration) map[DataSource]time.Duration {
	if len(thresholds) == 0 {
		return nil
	}
	cloned := make(map[DataSource]time.Duration, len(thresholds))
	for source, threshold := range thresholds {
		if strings.TrimSpace(string(source)) == "" || threshold <= 0 {
			continue
		}
		cloned[source] = threshold
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func effectiveStaleThresholds(thresholds map[DataSource]time.Duration) map[DataSource]time.Duration {
	if len(thresholds) == 0 {
		return defaultStaleThresholds
	}
	effective := make(map[DataSource]time.Duration, len(defaultStaleThresholds)+len(thresholds))
	for source, threshold := range defaultStaleThresholds {
		effective[source] = threshold
	}
	for source, threshold := range thresholds {
		if strings.TrimSpace(string(source)) == "" || threshold <= 0 {
			continue
		}
		effective[source] = threshold
	}
	return effective
}

// IngestRecord is a source-native resource entry normalized for registry ingestion.
type IngestRecord struct {
	SourceID       string
	ParentSourceID string
	Resource       Resource
	Identity       ResourceIdentity
}

// ResourceRegistry merges resources from multiple sources.
type ResourceRegistry struct {
	mu           sync.RWMutex
	resources    map[string]*Resource
	bySource     map[DataSource]map[string]string
	matcher      *IdentityMatcher
	store        ResourceStore
	links        []ResourceLink
	exclusions   map[string]struct{}
	identityPins *identityPinIndex
	pbsBackups   []models.PBSBackup

	// Cached typed view indexes. Invalidated on ingest, rebuilt lazily on
	// first access. Protected by mu — callers hold RLock to read, and the
	// rebuild path upgrades to a write lock only when needed.
	viewsDirty             bool
	cachedVMs              []*VMView
	cachedLXC              []*ContainerView
	cachedNodes            []*NodeView
	cachedHosts            []*HostView
	cachedDocker           []*DockerHostView
	cachedDockerContainers []*DockerContainerView
	cachedStorage          []*StoragePoolView
	cachedPhysicalDisks    []*PhysicalDiskView
	cachedPBS              []*PBSInstanceView
	cachedPMG              []*PMGInstanceView
	cachedK8s              []*K8sClusterView
	cachedK8sNodes         []*K8sNodeView
	cachedPods             []*PodView
	cachedK8sDeployments   []*K8sDeploymentView
	cachedWorkload         []*WorkloadView
	cachedInfra            []*InfrastructureView
}

// NewRegistry creates a new registry using the provided store for overrides.
func NewRegistry(store ResourceStore) *ResourceRegistry {
	rr := &ResourceRegistry{
		resources:  make(map[string]*Resource),
		bySource:   make(map[DataSource]map[string]string),
		matcher:    NewIdentityMatcher(),
		store:      store,
		exclusions: make(map[string]struct{}),
	}

	rr.bySource[SourceProxmox] = make(map[string]string)
	rr.bySource[SourceAgent] = make(map[string]string)
	rr.bySource[SourceDocker] = make(map[string]string)
	rr.bySource[SourcePBS] = make(map[string]string)
	rr.bySource[SourcePMG] = make(map[string]string)
	rr.bySource[SourceK8s] = make(map[string]string)
	rr.bySource[SourceTrueNAS] = make(map[string]string)
	rr.bySource[SourceVMware] = make(map[string]string)
	rr.bySource[SourceAvailability] = make(map[string]string)

	rr.loadOverrides()
	rr.loadIdentityPins()
	return rr
}

func (rr *ResourceRegistry) loadOverrides() {
	if rr.store == nil {
		return
	}
	links, err := rr.store.GetLinks()
	if err == nil {
		rr.links = links
	} else {
		log.Printf("unifiedresources: failed to load manual links from store: %v", err)
	}
	exclusions, err := rr.store.GetExclusions()
	if err == nil {
		for _, exclusion := range exclusions {
			key := exclusionKey(exclusion.ResourceA, exclusion.ResourceB)
			rr.exclusions[key] = struct{}{}
		}
	} else {
		log.Printf("unifiedresources: failed to load manual exclusions from store: %v", err)
	}
}

// IngestSnapshot ingests all resources from the current state snapshot.
func (rr *ResourceRegistry) IngestSnapshot(snapshot models.StateSnapshot) {
	rr.ingestSnapshot(snapshot, nil)
}

// IngestSnapshotWithStaleThresholds ingests all resources from the current
// state snapshot and evaluates source freshness with caller-owned thresholds.
func (rr *ResourceRegistry) IngestSnapshotWithStaleThresholds(snapshot models.StateSnapshot, thresholds map[DataSource]time.Duration) {
	rr.ingestSnapshot(snapshot, thresholds)
}

func (rr *ResourceRegistry) ingestSnapshot(snapshot models.StateSnapshot, thresholds map[DataSource]time.Duration) {
	hostByID := make(map[string]*models.Host, len(snapshot.Hosts))
	for i := range snapshot.Hosts {
		host := snapshot.Hosts[i]
		if id := strings.TrimSpace(host.ID); id != "" {
			hostByID[id] = &snapshot.Hosts[i]
		}
	}
	inferredLinkedHostByNodeID := inferLinkedHostsForProxmoxNodes(snapshot.Nodes, hostByID)

	// Build instance→clusterName lookup from nodes so we can propagate
	// cluster names to VMs/Containers (their parent-ID lookup may fail
	// when the node ID uses clusterName instead of instanceName).
	clusterByInstance := make(map[string]string)
	for _, node := range snapshot.Nodes {
		if node.ClusterName != "" && node.Instance != "" {
			clusterByInstance[node.Instance] = node.ClusterName
		}
		rr.ingestProxmoxNode(node, inferredLinkedHostByNodeID[strings.TrimSpace(node.ID)])
	}
	for _, host := range snapshot.Hosts {
		rr.ingestHost(host)
	}
	for _, host := range snapshot.Hosts {
		rr.ingestHostUnraidStorage(host)
	}
	for _, host := range snapshot.Hosts {
		rr.ingestHostUnraidPhysicalDisks(host)
	}
	for _, host := range snapshot.Hosts {
		rr.ingestHostSMARTDisks(host)
	}
	for _, dh := range snapshot.DockerHosts {
		rr.ingestDockerHost(dh)
	}
	for _, instance := range snapshot.PBSInstances {
		rr.ingestPBSInstance(instance)
	}
	for _, instance := range snapshot.PMGInstances {
		rr.ingestPMGInstance(instance)
	}
	for _, vm := range snapshot.VMs {
		rr.ingestVM(vm, clusterByInstance)
	}
	for _, ct := range snapshot.Containers {
		rr.ingestContainer(ct, clusterByInstance)
	}
	for _, storage := range snapshot.Storage {
		rr.ingestStorage(storage)
	}
	for _, disk := range snapshot.PhysicalDisks {
		rr.ingestPhysicalDisk(disk)
	}
	for _, cluster := range snapshot.CephClusters {
		rr.ingestCephCluster(cluster)
		for _, storage := range models.CephPoolStorage(cluster) {
			rr.ingestStorage(storage)
		}
	}
	for _, dh := range snapshot.DockerHosts {
		for _, dc := range dh.Containers {
			rr.ingestDockerContainer(dc, dh)
		}
		for _, image := range dh.Images {
			rr.ingestDockerImage(image, dh)
		}
		for _, volume := range dh.Volumes {
			rr.ingestDockerVolume(volume, dh)
		}
		for _, network := range dh.Networks {
			rr.ingestDockerNetwork(network, dh)
		}
		rr.refreshDockerNetworkAttachmentRelationships(dh)
	}

	type dockerSecretCandidate struct {
		host   models.DockerHost
		secret models.DockerSecret
	}
	secretByID := make(map[string]dockerSecretCandidate)
	for _, dh := range snapshot.DockerHosts {
		if dh.Swarm == nil {
			continue
		}
		for _, secret := range dh.Secrets {
			sourceID := dockerSecretSourceID(dh, secret)
			if sourceID == "" {
				continue
			}
			existing, ok := secretByID[sourceID]
			if !ok {
				secretByID[sourceID] = dockerSecretCandidate{host: dh, secret: secret}
				continue
			}
			replace := false
			if existing.secret.DriverName == "" && secret.DriverName != "" {
				replace = true
			}
			if !replace && dh.LastSeen.After(existing.host.LastSeen) {
				replace = true
			}
			if replace {
				secretByID[sourceID] = dockerSecretCandidate{host: dh, secret: secret}
			}
		}
	}
	for _, candidate := range secretByID {
		rr.ingestDockerSecret(candidate.secret, candidate.host)
	}

	type dockerConfigCandidate struct {
		host   models.DockerHost
		config models.DockerConfig
	}
	configByID := make(map[string]dockerConfigCandidate)
	for _, dh := range snapshot.DockerHosts {
		if dh.Swarm == nil {
			continue
		}
		for _, config := range dh.Configs {
			sourceID := dockerConfigSourceID(dh, config)
			if sourceID == "" {
				continue
			}
			existing, ok := configByID[sourceID]
			if !ok {
				configByID[sourceID] = dockerConfigCandidate{host: dh, config: config}
				continue
			}
			replace := false
			if existing.config.TemplatingDriver == "" && config.TemplatingDriver != "" {
				replace = true
			}
			if !replace && dh.LastSeen.After(existing.host.LastSeen) {
				replace = true
			}
			if replace {
				configByID[sourceID] = dockerConfigCandidate{host: dh, config: config}
			}
		}
	}
	for _, candidate := range configByID {
		rr.ingestDockerConfig(candidate.config, candidate.host)
	}

	// Swarm services are cluster-scoped; multiple nodes can report identical
	// service lists. Deduplicate by a stable source ID to avoid churn.
	type dockerServiceCandidate struct {
		host    models.DockerHost
		service models.DockerService
	}
	serviceByID := make(map[string]dockerServiceCandidate)
	for _, dh := range snapshot.DockerHosts {
		if dh.Swarm == nil {
			continue
		}
		for _, svc := range dh.Services {
			sourceID := dockerServiceSourceID(dh, svc)
			if sourceID == "" {
				continue
			}
			existing, ok := serviceByID[sourceID]
			if !ok {
				serviceByID[sourceID] = dockerServiceCandidate{host: dh, service: svc}
				continue
			}
			// Prefer candidates with richer fields and fresher host timestamps.
			replace := false
			if existing.service.Image == "" && svc.Image != "" {
				replace = true
			}
			if existing.service.UpdateStatus == nil && svc.UpdateStatus != nil {
				replace = true
			}
			if !replace && dh.LastSeen.After(existing.host.LastSeen) {
				replace = true
			}
			if replace {
				serviceByID[sourceID] = dockerServiceCandidate{host: dh, service: svc}
			}
		}
	}
	for _, candidate := range serviceByID {
		rr.ingestDockerService(candidate.service, candidate.host)
	}

	type dockerNodeCandidate struct {
		host models.DockerHost
		node models.DockerNode
	}
	nodeByID := make(map[string]dockerNodeCandidate)
	for _, dh := range snapshot.DockerHosts {
		if dh.Swarm == nil {
			continue
		}
		for _, node := range dh.Nodes {
			sourceID := dockerSwarmNodeSourceID(dh, node)
			if sourceID == "" {
				continue
			}
			existing, ok := nodeByID[sourceID]
			if !ok {
				nodeByID[sourceID] = dockerNodeCandidate{host: dh, node: node}
				continue
			}
			replace := false
			if existing.node.EngineVersion == "" && node.EngineVersion != "" {
				replace = true
			}
			if existing.node.ManagerReachability == "" && node.ManagerReachability != "" {
				replace = true
			}
			if !replace && dh.LastSeen.After(existing.host.LastSeen) {
				replace = true
			}
			if replace {
				nodeByID[sourceID] = dockerNodeCandidate{host: dh, node: node}
			}
		}
	}
	for _, candidate := range nodeByID {
		rr.ingestDockerSwarmNode(candidate.node, candidate.host)
	}

	for _, dh := range snapshot.DockerHosts {
		for _, task := range dh.Tasks {
			rr.ingestDockerTask(task, dh)
		}
	}

	kubernetesHostLookup := buildKubernetesNodeHostLookup(snapshot.Hosts)
	for _, cluster := range snapshot.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		linkedHosts := linkedHostsForKubernetesCluster(cluster, kubernetesHostLookup)
		capabilities := kubernetesMetricCapabilities(cluster, linkedHosts)
		clusterID := rr.ingestKubernetesCluster(cluster, linkedHosts, capabilities)
		for _, node := range cluster.Nodes {
			linkedHost := resolveKubernetesNodeHost(node, kubernetesHostLookup)
			rr.ingestKubernetesNode(cluster, node, linkedHost, clusterID, capabilities)
		}
		for _, pod := range cluster.Pods {
			rr.ingestKubernetesPod(cluster, pod, clusterID, capabilities)
		}
		for _, deployment := range cluster.Deployments {
			rr.ingestKubernetesDeployment(cluster, deployment, clusterID, capabilities)
		}
		for _, replicaSet := range cluster.ReplicaSets {
			rr.ingestKubernetesReplicaSet(cluster, replicaSet, clusterID, capabilities)
		}
		for _, namespace := range cluster.Namespaces {
			rr.ingestKubernetesNamespace(cluster, namespace, clusterID, capabilities)
		}
		for _, service := range cluster.Services {
			rr.ingestKubernetesService(cluster, service, clusterID, capabilities)
		}
		for _, statefulSet := range cluster.StatefulSets {
			rr.ingestKubernetesStatefulSet(cluster, statefulSet, clusterID, capabilities)
		}
		for _, daemonSet := range cluster.DaemonSets {
			rr.ingestKubernetesDaemonSet(cluster, daemonSet, clusterID, capabilities)
		}
		for _, job := range cluster.Jobs {
			rr.ingestKubernetesJob(cluster, job, clusterID, capabilities)
		}
		for _, cronJob := range cluster.CronJobs {
			rr.ingestKubernetesCronJob(cluster, cronJob, clusterID, capabilities)
		}
		for _, ingress := range cluster.Ingresses {
			rr.ingestKubernetesIngress(cluster, ingress, clusterID, capabilities)
		}
		for _, endpointSlice := range cluster.EndpointSlices {
			rr.ingestKubernetesEndpointSlice(cluster, endpointSlice, clusterID, capabilities)
		}
		for _, policy := range cluster.NetworkPolicies {
			rr.ingestKubernetesNetworkPolicy(cluster, policy, clusterID, capabilities)
		}
		for _, volume := range cluster.PersistentVolumes {
			rr.ingestKubernetesPersistentVolume(cluster, volume, clusterID, capabilities)
		}
		for _, claim := range cluster.PersistentVolumeClaims {
			rr.ingestKubernetesPersistentVolumeClaim(cluster, claim, clusterID, capabilities)
		}
		for _, class := range cluster.StorageClasses {
			rr.ingestKubernetesStorageClass(cluster, class, clusterID, capabilities)
		}
		for _, configMap := range cluster.ConfigMaps {
			rr.ingestKubernetesConfigMap(cluster, configMap, clusterID, capabilities)
		}
		for _, secret := range cluster.Secrets {
			rr.ingestKubernetesSecret(cluster, secret, clusterID, capabilities)
		}
		for _, account := range cluster.ServiceAccounts {
			rr.ingestKubernetesServiceAccount(cluster, account, clusterID, capabilities)
		}
		for _, role := range cluster.Roles {
			rr.ingestKubernetesRole(cluster, role, clusterID, capabilities)
		}
		for _, clusterRole := range cluster.ClusterRoles {
			rr.ingestKubernetesClusterRole(cluster, clusterRole, clusterID, capabilities)
		}
		for _, binding := range cluster.RoleBindings {
			rr.ingestKubernetesRoleBinding(cluster, binding, clusterID, capabilities)
		}
		for _, binding := range cluster.ClusterRoleBindings {
			rr.ingestKubernetesClusterRoleBinding(cluster, binding, clusterID, capabilities)
		}
		for _, quota := range cluster.ResourceQuotas {
			rr.ingestKubernetesResourceQuota(cluster, quota, clusterID, capabilities)
		}
		for _, limitRange := range cluster.LimitRanges {
			rr.ingestKubernetesLimitRange(cluster, limitRange, clusterID, capabilities)
		}
		for _, budget := range cluster.PodDisruptionBudgets {
			rr.ingestKubernetesPodDisruptionBudget(cluster, budget, clusterID, capabilities)
		}
		for _, autoscaler := range cluster.HorizontalPodAutoscalers {
			rr.ingestKubernetesHorizontalPodAutoscaler(cluster, autoscaler, clusterID, capabilities)
		}
		for _, event := range cluster.Events {
			rr.ingestKubernetesEvent(cluster, event, clusterID, capabilities)
		}
	}

	rr.mu.Lock()
	rr.pbsBackups = clonePBSBackups(snapshot.PBSBackups)
	rr.applyManualLinks()
	rr.refreshStorageConsumersLocked()
	rr.refreshPBSRollupsLocked()
	rr.refreshStoragePostureLocked()
	rr.refreshIncidentRollupsLocked()
	rr.buildChildCounts()
	rr.markStaleLocked(time.Now().UTC(), thresholds)
	rr.viewsDirty = true
	rr.mu.Unlock()
}

// IngestRecords ingests normalized records for a single source.
func (rr *ResourceRegistry) IngestRecords(source DataSource, records []IngestRecord) {
	for _, record := range records {
		sourceID := normalizeSourceID(record.SourceID)
		if sourceID == "" {
			continue
		}
		resource := record.Resource
		if parentSourceID := normalizeSourceID(record.ParentSourceID); parentSourceID != "" {
			parentID := rr.sourceResourceID(source, parentSourceID)
			if parentID != "" {
				resource.ParentID = &parentID
			}
		}
		rr.ingest(source, sourceID, resource, record.Identity)
	}

	rr.mu.Lock()
	rr.refreshStorageConsumersLocked()
	rr.refreshPBSRollupsLocked()
	rr.refreshStoragePostureLocked()
	rr.refreshIncidentRollupsLocked()
	rr.buildChildCounts()
	rr.viewsDirty = true
	rr.mu.Unlock()
}

// IngestResources seeds the registry from already-unified resources.
// This is used when a caller already has a canonical unified read model and
// only needs store-backed manual links/exclusions applied on top.
func (rr *ResourceRegistry) IngestResources(resources []Resource) {
	rr.ingestResources(resources, nil)
}

// IngestResourcesWithStaleThresholds seeds the registry from already-unified
// resources and evaluates source freshness with caller-owned thresholds.
func (rr *ResourceRegistry) IngestResourcesWithStaleThresholds(resources []Resource, thresholds map[DataSource]time.Duration) {
	rr.ingestResources(resources, thresholds)
}

func (rr *ResourceRegistry) ingestResources(resources []Resource, thresholds map[DataSource]time.Duration) {
	seededIDs := make([]string, 0, len(resources))
	for _, incoming := range resources {
		resource := cloneResourcePtr(&incoming)
		if resource == nil {
			continue
		}

		resource.ID = CanonicalResourceID(resource.ID)
		if resource.ID == "" {
			continue
		}
		resource.Type = CanonicalResourceType(resource.Type)
		if resource.SourceStatus == nil && len(resource.Sources) > 0 {
			resource.SourceStatus = make(map[DataSource]SourceStatus, len(resource.Sources))
			for _, source := range resource.Sources {
				resource.SourceStatus[source] = SourceStatus{
					Status:   sourceSightingStatus(resource.LastSeen),
					LastSeen: resource.LastSeen,
				}
			}
		}

		rr.mu.Lock()
		rr.resources[resource.ID] = resource
		rr.matcher.Add(resource.ID, resource.Identity)
		rr.viewsDirty = true
		rr.mu.Unlock()

		seededIDs = append(seededIDs, resource.ID)
	}

	rr.mu.Lock()
	for _, resourceID := range seededIDs {
		rr.seedSourceMappingsFromResourceLocked(rr.resources[resourceID])
	}
	rr.applyManualLinks()
	rr.refreshStorageConsumersLocked()
	rr.refreshPBSRollupsLocked()
	rr.refreshStoragePostureLocked()
	rr.refreshIncidentRollupsLocked()
	rr.buildChildCounts()
	rr.markStaleLocked(time.Now().UTC(), thresholds)
	rr.viewsDirty = true
	rr.mu.Unlock()
}

func (rr *ResourceRegistry) seedSourceMappingsFromResourceLocked(resource *Resource) {
	if resource == nil {
		return
	}

	sources := make([]DataSource, 0, len(resource.Sources)+len(resource.SourceStatus))
	seen := make(map[DataSource]struct{}, len(resource.Sources)+len(resource.SourceStatus))
	for _, source := range resource.Sources {
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	for source := range resource.SourceStatus {
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool {
		return string(sources[i]) < string(sources[j])
	})

	for _, source := range sources {
		sourceID := rr.seedSourceIDForResourceLocked(resource, source)
		sourceID = normalizeSourceID(sourceID)
		if sourceID == "" {
			continue
		}
		if _, ok := rr.bySource[source]; !ok {
			rr.bySource[source] = make(map[string]string)
		}
		rr.bySource[source][sourceID] = resource.ID
	}
}

func (rr *ResourceRegistry) seedSourceIDForResourceLocked(resource *Resource, source DataSource) string {
	if resource == nil {
		return ""
	}

	switch source {
	case SourceProxmox:
		if resource.Proxmox != nil {
			if sourceID := strings.TrimSpace(resource.Proxmox.SourceID); sourceID != "" {
				return sourceID
			}
			if CanonicalResourceType(resource.Type) == ResourceTypeAgent {
				return proxmoxNodeSourceID(
					strings.TrimSpace(resource.Proxmox.Instance),
					strings.TrimSpace(resource.Proxmox.NodeName),
				)
			}
		}
		if resource.Ceph != nil {
			return strings.TrimSpace(resource.Ceph.FSID)
		}
	case SourceAgent:
		if resource.Agent != nil {
			if sourceID := strings.TrimSpace(resource.Agent.AgentID); sourceID != "" {
				return sourceID
			}
		}
		switch CanonicalResourceType(resource.Type) {
		case ResourceTypeStorage:
			if resource.Storage != nil && strings.EqualFold(resource.Storage.Type, "unraid-array") {
				if parentSourceID := rr.seedParentSourceIDLocked(resource, SourceAgent); parentSourceID != "" {
					return parentSourceID + "/storage:unraid-array"
				}
			}
		case ResourceTypePhysicalDisk:
			if resource.PhysicalDisk == nil {
				return ""
			}
			fallback := ""
			if parentSourceID := rr.seedParentSourceIDLocked(resource, SourceAgent); parentSourceID != "" {
				device := normalizePhysicalDiskDeviceToken(resource.PhysicalDisk.DevPath)
				if device != "" {
					fallback = fmt.Sprintf("%s:%s", parentSourceID, device)
				}
			}
			return PreferredPhysicalDiskMetricID(resource.PhysicalDisk.Serial, resource.PhysicalDisk.WWN, fallback)
		}
	case SourceDocker:
		if resource.Docker == nil {
			return ""
		}
		switch CanonicalResourceType(resource.Type) {
		case ResourceTypeAgent:
			return strings.TrimSpace(resource.Docker.HostSourceID)
		case ResourceTypeAppContainer:
			// Match the host-scoped key shape produced by
			// ingestDockerContainer so records-path ingests resolve to
			// the same registry entries as inventory-path ingests.
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			containerID := strings.TrimSpace(resource.Docker.ContainerID)
			if hostSourceID == "" {
				return containerID
			}
			if containerID == "" {
				name := strings.TrimSpace(resource.Name)
				if name == "" {
					return ""
				}
				containerID = "name:" + name
			}
			return hostSourceID + "/container/" + containerID
		case ResourceTypeDockerService:
			clusterKey := dockerSwarmClusterKeyFromMeta(resource.Docker.Swarm)
			if clusterKey == "" {
				return ""
			}
			serviceID := strings.TrimSpace(resource.Docker.ServiceID)
			if serviceID == "" {
				serviceID = strings.TrimSpace(resource.Name)
			}
			if serviceID == "" {
				return ""
			}
			return fmt.Sprintf("%s:service:%s", clusterKey, serviceID)
		case ResourceTypeDockerImage:
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			imageID := strings.TrimSpace(resource.Docker.ImageID)
			if imageID == "" {
				imageID = strings.TrimSpace(resource.Name)
			}
			if hostSourceID == "" || imageID == "" {
				return ""
			}
			return hostSourceID + "/image/" + imageID
		case ResourceTypeDockerVolume:
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			volumeName := strings.TrimSpace(resource.Docker.VolumeName)
			if volumeName == "" {
				volumeName = strings.TrimSpace(resource.Name)
			}
			if hostSourceID == "" || volumeName == "" {
				return ""
			}
			return hostSourceID + "/volume/" + volumeName
		case ResourceTypeDockerNetwork:
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			networkID := strings.TrimSpace(resource.Docker.NetworkID)
			if networkID == "" {
				networkID = strings.TrimSpace(resource.Name)
			}
			if hostSourceID == "" || networkID == "" {
				return ""
			}
			return hostSourceID + "/network/" + networkID
		case ResourceTypeDockerTask:
			taskID := strings.TrimSpace(resource.Docker.TaskID)
			if taskID == "" {
				taskID = strings.TrimSpace(resource.Name)
			}
			if taskID == "" {
				return ""
			}
			if clusterKey := dockerSwarmClusterKeyFromMeta(resource.Docker.Swarm); clusterKey != "" {
				return fmt.Sprintf("%s:task:%s", clusterKey, taskID)
			}
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			if hostSourceID == "" {
				return ""
			}
			return hostSourceID + "/task/" + taskID
		case ResourceTypeDockerSwarmNode:
			nodeID := strings.TrimSpace(resource.Docker.NodeID)
			if nodeID == "" {
				nodeID = strings.TrimSpace(resource.Name)
			}
			if nodeID == "" {
				return ""
			}
			if clusterKey := dockerSwarmClusterKeyFromMeta(resource.Docker.Swarm); clusterKey != "" {
				return fmt.Sprintf("%s:node:%s", clusterKey, nodeID)
			}
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			if hostSourceID == "" {
				return ""
			}
			return hostSourceID + "/swarm-node/" + nodeID
		case ResourceTypeDockerSecret:
			secretID := strings.TrimSpace(resource.Docker.SecretID)
			if secretID == "" {
				secretID = strings.TrimSpace(resource.Name)
			}
			if secretID == "" {
				return ""
			}
			if clusterKey := dockerSwarmClusterKeyFromMeta(resource.Docker.Swarm); clusterKey != "" {
				return fmt.Sprintf("%s:secret:%s", clusterKey, secretID)
			}
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			if hostSourceID == "" {
				return ""
			}
			return hostSourceID + "/secret/" + secretID
		case ResourceTypeDockerConfig:
			configID := strings.TrimSpace(resource.Docker.ConfigID)
			if configID == "" {
				configID = strings.TrimSpace(resource.Name)
			}
			if configID == "" {
				return ""
			}
			if clusterKey := dockerSwarmClusterKeyFromMeta(resource.Docker.Swarm); clusterKey != "" {
				return fmt.Sprintf("%s:config:%s", clusterKey, configID)
			}
			hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID)
			if hostSourceID == "" {
				return ""
			}
			return hostSourceID + "/config/" + configID
		default:
			return strings.TrimSpace(resource.Docker.HostSourceID)
		}
	case SourcePBS:
		if resource.PBS != nil {
			if sourceID := strings.TrimSpace(resource.PBS.InstanceID); sourceID != "" {
				return sourceID
			}
		}
		if CanonicalResourceType(resource.Type) == ResourceTypeStorage && resource.Storage != nil && strings.EqualFold(resource.Storage.Platform, "pbs") {
			parentSourceID := rr.seedParentSourceIDLocked(resource, SourcePBS)
			if parentSourceID == "" {
				return ""
			}
			datastoreName := strings.TrimSpace(resource.Name)
			if datastoreName == "" {
				return parentSourceID
			}
			return parentSourceID + "/" + datastoreName
		}
	case SourcePMG:
		if resource.PMG != nil {
			return strings.TrimSpace(resource.PMG.InstanceID)
		}
	case SourceK8s:
		clusterSourceID := seededKubernetesClusterSourceID(resource.Kubernetes)
		switch CanonicalResourceType(resource.Type) {
		case ResourceTypeK8sCluster:
			return clusterSourceID
		case ResourceTypeAgent, ResourceTypeK8sNode:
			nodeID := seededKubernetesNodeIdentity(resource.Kubernetes)
			if clusterSourceID == "" || nodeID == "" {
				return ""
			}
			return fmt.Sprintf("%s:node:%s", clusterSourceID, nodeID)
		case ResourceTypePod:
			podID := seededKubernetesPodIdentity(resource)
			if clusterSourceID == "" || podID == "" {
				return ""
			}
			return fmt.Sprintf("%s:pod:%s", clusterSourceID, podID)
		case ResourceTypeK8sDeployment:
			deploymentID := seededKubernetesDeploymentIdentity(resource)
			if clusterSourceID == "" || deploymentID == "" {
				return ""
			}
			return fmt.Sprintf("%s:deployment:%s", clusterSourceID, deploymentID)
		case ResourceTypeK8sReplicaSet:
			return seededKubernetesTypedSourceID(clusterSourceID, "replicaset", resource, resource.Kubernetes.ReplicaSetUID)
		case ResourceTypeK8sNamespace:
			return seededKubernetesTypedSourceID(clusterSourceID, "namespace", resource, resource.Kubernetes.NamespaceUID)
		case ResourceTypeK8sService:
			return seededKubernetesTypedSourceID(clusterSourceID, "service", resource, resource.Kubernetes.ServiceUID)
		case ResourceTypeK8sStatefulSet:
			return seededKubernetesTypedSourceID(clusterSourceID, "statefulset", resource, resource.Kubernetes.StatefulSetUID)
		case ResourceTypeK8sDaemonSet:
			return seededKubernetesTypedSourceID(clusterSourceID, "daemonset", resource, resource.Kubernetes.DaemonSetUID)
		case ResourceTypeK8sJob:
			return seededKubernetesTypedSourceID(clusterSourceID, "job", resource, resource.Kubernetes.JobUID)
		case ResourceTypeK8sCronJob:
			return seededKubernetesTypedSourceID(clusterSourceID, "cronjob", resource, resource.Kubernetes.CronJobUID)
		case ResourceTypeK8sIngress:
			return seededKubernetesTypedSourceID(clusterSourceID, "ingress", resource, resource.Kubernetes.IngressUID)
		case ResourceTypeK8sEndpointSlice:
			return seededKubernetesTypedSourceID(clusterSourceID, "endpointslice", resource, resource.Kubernetes.EndpointSliceUID)
		case ResourceTypeK8sNetworkPolicy:
			return seededKubernetesTypedSourceID(clusterSourceID, "networkpolicy", resource, resource.Kubernetes.NetworkPolicyUID)
		case ResourceTypeK8sPV:
			return seededKubernetesTypedSourceID(clusterSourceID, "persistentvolume", resource, resource.Kubernetes.PersistentVolumeUID)
		case ResourceTypeK8sPVC:
			return seededKubernetesTypedSourceID(clusterSourceID, "persistentvolumeclaim", resource, resource.Kubernetes.PersistentVolumeClaimUID)
		case ResourceTypeK8sStorageClass:
			return seededKubernetesTypedSourceID(clusterSourceID, "storageclass", resource, resource.Kubernetes.StorageClassUID)
		case ResourceTypeK8sConfigMap:
			return seededKubernetesTypedSourceID(clusterSourceID, "configmap", resource, resource.Kubernetes.ConfigMapUID)
		case ResourceTypeK8sSecret:
			return seededKubernetesTypedSourceID(clusterSourceID, "secret", resource, resource.Kubernetes.SecretUID)
		case ResourceTypeK8sServiceAccount:
			return seededKubernetesTypedSourceID(clusterSourceID, "serviceaccount", resource, resource.Kubernetes.ServiceAccountUID)
		case ResourceTypeK8sRole:
			return seededKubernetesTypedSourceID(clusterSourceID, "role", resource, resource.Kubernetes.RoleUID)
		case ResourceTypeK8sClusterRole:
			return seededKubernetesTypedSourceID(clusterSourceID, "clusterrole", resource, resource.Kubernetes.ClusterRoleUID)
		case ResourceTypeK8sRoleBinding:
			return seededKubernetesTypedSourceID(clusterSourceID, "rolebinding", resource, resource.Kubernetes.RoleBindingUID)
		case ResourceTypeK8sClusterRoleBinding:
			return seededKubernetesTypedSourceID(clusterSourceID, "clusterrolebinding", resource, resource.Kubernetes.ClusterRoleBindingUID)
		case ResourceTypeK8sResourceQuota:
			return seededKubernetesTypedSourceID(clusterSourceID, "resourcequota", resource, resource.Kubernetes.ResourceQuotaUID)
		case ResourceTypeK8sLimitRange:
			return seededKubernetesTypedSourceID(clusterSourceID, "limitrange", resource, resource.Kubernetes.LimitRangeUID)
		case ResourceTypeK8sPDB:
			return seededKubernetesTypedSourceID(clusterSourceID, "poddisruptionbudget", resource, resource.Kubernetes.PodDisruptionBudgetUID)
		case ResourceTypeK8sHPA:
			return seededKubernetesTypedSourceID(clusterSourceID, "horizontalpodautoscaler", resource, resource.Kubernetes.HorizontalPodAutoscalerUID)
		case ResourceTypeK8sEvent:
			return seededKubernetesTypedSourceID(clusterSourceID, "event", resource, resource.Kubernetes.EventUID)
		}
	case SourceVMware:
		return seededVMwareSourceID(resource)
	case SourceAvailability:
		if resource.Availability != nil {
			return strings.TrimSpace(resource.Availability.TargetID)
		}
	}

	return ""
}

func (rr *ResourceRegistry) seedParentSourceIDLocked(resource *Resource, source DataSource) string {
	if resource == nil || resource.ParentID == nil {
		return ""
	}

	parentID := CanonicalResourceID(strings.TrimSpace(*resource.ParentID))
	if parentID == "" {
		return ""
	}

	parent := rr.resources[parentID]
	return rr.seedSourceIDForResourceLocked(parent, source)
}

func dockerSwarmClusterKeyFromMeta(swarm *DockerSwarmInfo) string {
	if swarm == nil {
		return ""
	}
	if clusterID := strings.TrimSpace(swarm.ClusterID); clusterID != "" {
		return clusterID
	}
	return strings.TrimSpace(swarm.ClusterName)
}

func seededKubernetesClusterSourceID(kubernetes *K8sData) string {
	if kubernetes == nil {
		return ""
	}
	for _, candidate := range []string{
		kubernetes.ClusterID,
		kubernetes.AgentID,
		kubernetes.SourceName,
		kubernetes.ClusterName,
		kubernetes.Context,
	} {
		if value := strings.TrimSpace(candidate); value != "" {
			return value
		}
	}
	return ""
}

func seededKubernetesNodeIdentity(kubernetes *K8sData) string {
	if kubernetes == nil {
		return ""
	}
	if nodeUID := strings.TrimSpace(kubernetes.NodeUID); nodeUID != "" {
		return nodeUID
	}
	return strings.TrimSpace(kubernetes.NodeName)
}

func seededKubernetesPodIdentity(resource *Resource) string {
	if resource == nil || resource.Kubernetes == nil {
		return ""
	}
	if podUID := strings.TrimSpace(resource.Kubernetes.PodUID); podUID != "" {
		return podUID
	}
	namespace := strings.TrimSpace(resource.Kubernetes.Namespace)
	name := strings.TrimSpace(resource.Name)
	if namespace == "" || name == "" {
		return ""
	}
	return namespace + "/" + name
}

func seededKubernetesDeploymentIdentity(resource *Resource) string {
	if resource == nil || resource.Kubernetes == nil {
		return ""
	}
	if deploymentUID := strings.TrimSpace(resource.Kubernetes.DeploymentUID); deploymentUID != "" {
		return deploymentUID
	}
	namespace := strings.TrimSpace(resource.Kubernetes.Namespace)
	name := strings.TrimSpace(resource.Name)
	if namespace == "" || name == "" {
		return ""
	}
	return namespace + "/" + name
}

func seededKubernetesTypedSourceID(clusterSourceID, kind string, resource *Resource, uid string) string {
	if resource == nil || resource.Kubernetes == nil {
		return ""
	}
	id := strings.TrimSpace(uid)
	if id == "" {
		namespace := strings.TrimSpace(resource.Kubernetes.Namespace)
		name := strings.TrimSpace(resource.Name)
		if namespace != "" && name != "" {
			id = namespace + "/" + name
		} else {
			id = name
		}
	}
	clusterSourceID = strings.TrimSpace(clusterSourceID)
	kind = strings.TrimSpace(kind)
	if clusterSourceID == "" || kind == "" || id == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", clusterSourceID, kind, id)
}

func seededVMwareSourceID(resource *Resource) string {
	if resource == nil || resource.VMware == nil {
		return ""
	}

	managedObjectID := strings.TrimSpace(resource.VMware.ManagedObjectID)
	if managedObjectID == "" {
		return ""
	}

	entityType := strings.TrimSpace(resource.VMware.EntityType)
	if entityType == "" {
		entityType = string(CanonicalResourceType(resource.Type))
	}

	parts := make([]string, 0, 3)
	if connectionID := strings.TrimSpace(resource.VMware.ConnectionID); connectionID != "" {
		parts = append(parts, connectionID)
	}
	if entityType != "" {
		parts = append(parts, entityType)
	}
	parts = append(parts, managedObjectID)
	return strings.Join(parts, ":")
}

// List returns all resources.
func (rr *ResourceRegistry) List() []Resource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	out := make([]Resource, 0, len(rr.resources))
	for _, r := range rr.resources {
		out = append(out, cloneResource(r))
	}
	sortResourcesByName(out)
	return out
}

// ListForPresentation returns resources in the canonical API/broadcast
// presentation shape, including top-level host coalescing that respects manual
// merge exclusions.
func (rr *ResourceRegistry) ListForPresentation() []Resource {
	resources := rr.List()

	rr.mu.RLock()
	exclusions := make(map[string]struct{}, len(rr.exclusions))
	for key := range rr.exclusions {
		exclusions[key] = struct{}{}
	}
	rr.mu.RUnlock()

	return CoalescePresentationHostResourcesWithExclusions(resources, func(left, right Resource) bool {
		leftID := CanonicalResourceID(left.ID)
		rightID := CanonicalResourceID(right.ID)
		if leftID == "" || rightID == "" {
			return false
		}
		_, ok := exclusions[exclusionKey(leftID, rightID)]
		return ok
	})
}

// ListByType returns all resources of the provided type.
//
// The returned slice is sorted by resource ID to provide deterministic results.
func (rr *ResourceRegistry) ListByType(t ResourceType) []Resource {
	t = CanonicalResourceType(t)

	rr.mu.RLock()
	defer rr.mu.RUnlock()

	out := make([]Resource, 0, len(rr.resources))
	for _, r := range rr.resources {
		if CanonicalResourceType(r.Type) != t {
			continue
		}
		out = append(out, cloneResource(r))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// Get returns a resource by ID.
func (rr *ResourceRegistry) Get(id string) (*Resource, bool) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	id = CanonicalResourceID(id)
	r, ok := rr.resources[id]
	if !ok || r == nil {
		return nil, false
	}
	clone := cloneResource(r)
	return &clone, true
}

// GetByReference resolves a resource by its canonical resource ID, a unique
// source-specific ID, or a canonical identity alias. It returns the registered
// resource ID alongside the cloned resource so callers can keep downstream
// store lookups on the canonical registry identity.
func (rr *ResourceRegistry) GetByReference(ref string) (*Resource, string, bool) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	ref = CanonicalResourceID(ref)
	if ref == "" {
		return nil, "", false
	}

	if r := rr.resources[ref]; r != nil {
		clone := cloneResource(r)
		return &clone, ref, true
	}

	for _, resolvedID := range []string{
		rr.uniqueSourceResourceIDLocked(ref),
		rr.uniqueCanonicalIdentityResourceIDLocked(ref),
	} {
		if resolvedID == "" {
			continue
		}
		if r := rr.resources[resolvedID]; r != nil {
			clone := cloneResource(r)
			return &clone, resolvedID, true
		}
	}

	return nil, "", false
}

func (rr *ResourceRegistry) uniqueSourceResourceIDLocked(sourceID string) string {
	sourceID = normalizeSourceID(sourceID)
	if sourceID == "" {
		return ""
	}

	matches := map[string]struct{}{}
	for _, mapping := range rr.bySource {
		if resourceID := mapping[sourceID]; resourceID != "" {
			matches[resourceID] = struct{}{}
		}
	}
	return uniqueResourceIDMatch(matches)
}

func (rr *ResourceRegistry) uniqueCanonicalIdentityResourceIDLocked(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}

	matches := map[string]struct{}{}
	for resourceID, resource := range rr.resources {
		if resourceMatchesCanonicalIdentityReference(resource, ref) {
			matches[resourceID] = struct{}{}
		}
	}
	return uniqueResourceIDMatch(matches)
}

func resourceMatchesCanonicalIdentityReference(resource *Resource, ref string) bool {
	if resource == nil || resource.Canonical == nil {
		return false
	}

	canonical := resource.Canonical
	candidates := append([]string{
		canonical.PrimaryID,
		canonical.PlatformID,
	}, canonical.Aliases...)
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate), ref) {
			return true
		}
	}
	return false
}

func uniqueResourceIDMatch(matches map[string]struct{}) string {
	if len(matches) != 1 {
		return ""
	}
	for resourceID := range matches {
		return resourceID
	}
	return ""
}

// SourceTargets returns the source-specific IDs that map to the provided resource ID.
func (rr *ResourceRegistry) SourceTargets(resourceID string) []SourceTarget {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	resourceID = CanonicalResourceID(resourceID)
	resource := rr.resources[resourceID]
	if resource == nil {
		return nil
	}

	out := make([]SourceTarget, 0)
	for source, mapping := range rr.bySource {
		for sourceID, mappedID := range mapping {
			if mappedID != resourceID {
				continue
			}
			out = append(out, SourceTarget{
				Source:      source,
				SourceID:    sourceID,
				CandidateID: rr.sourceSpecificID(resource.Type, source, sourceID),
			})
		}
	}
	return out
}

// MetricsTarget resolves the query target used by the metrics/history APIs for
// the canonical resource.
func (rr *ResourceRegistry) MetricsTarget(resourceID string) *MetricsTarget {
	return BuildMetricsTargetForRegistry(rr, resourceID)
}

// BuildMetricsTargetForRegistry resolves the metrics target for a registry
// resource ID without exposing registry internals to callers.
func BuildMetricsTargetForRegistry(rr *ResourceRegistry, resourceID string) *MetricsTarget {
	if rr == nil {
		return nil
	}

	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return rr.metricsTargetForResourceLocked(resourceID)
}

func (rr *ResourceRegistry) metricsTargetForResourceLocked(resourceID string) *MetricsTarget {
	if rr == nil {
		return nil
	}

	resourceID = CanonicalResourceID(resourceID)
	resource := rr.resources[resourceID]
	if resource == nil {
		return nil
	}

	sourceTargets := make([]SourceTarget, 0)
	for source, mapping := range rr.bySource {
		for sourceID, mappedID := range mapping {
			if mappedID != resourceID {
				continue
			}
			sourceTargets = append(sourceTargets, SourceTarget{
				Source:      source,
				SourceID:    sourceID,
				CandidateID: rr.sourceSpecificID(resource.Type, source, sourceID),
			})
		}
	}

	if target := BuildMetricsTarget(*resource, sourceTargets); target != nil {
		return target
	}

	return cloneMetricsTarget(resource.MetricsTarget)
}

// GetChildren returns child resources for a parent.
func (rr *ResourceRegistry) GetChildren(parentID string) []Resource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	parentID = CanonicalResourceID(parentID)
	var out []Resource
	for _, r := range rr.resources {
		if r.ParentID != nil && *r.ParentID == parentID {
			out = append(out, cloneResource(r))
		}
	}
	return out
}

// Stats returns aggregated stats.
func (rr *ResourceRegistry) Stats() ResourceStats {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	stats := ResourceStats{
		Total:    len(rr.resources),
		ByType:   make(map[ResourceType]int),
		ByStatus: make(map[ResourceStatus]int),
		BySource: make(map[DataSource]int),
	}
	for _, r := range rr.resources {
		stats.ByType[CanonicalResourceType(r.Type)]++
		stats.ByStatus[r.Status]++
		for _, source := range r.Sources {
			stats.BySource[source]++
		}
	}
	return stats
}

// MarkStale marks sources as stale based on last seen timestamps.
// If thresholds is nil, default thresholds are used.
func (rr *ResourceRegistry) MarkStale(now time.Time, thresholds map[DataSource]time.Duration) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.markStaleLocked(now, thresholds)
}

// sourceSightingStatus derives the per-source delivery status from the last
// time that source actually delivered the resource. A zero sighting means the
// source has never delivered it (synthesized offline placeholders, instances
// that have not completed a poll since startup); its delivery state is
// unknown, not online. Stamping it "online" would exempt it from
// stale-marking forever, because markStaleLocked skips zero LastSeen entries.
func sourceSightingStatus(lastSeen time.Time) string {
	if lastSeen.IsZero() {
		return "unknown"
	}
	return "online"
}

func (rr *ResourceRegistry) markStaleLocked(now time.Time, thresholds map[DataSource]time.Duration) {
	thresholds = effectiveStaleThresholds(thresholds)

	for _, resource := range rr.resources {
		staleFound := false
		for source, status := range resource.SourceStatus {
			threshold, ok := thresholds[source]
			if !ok {
				threshold = 120 * time.Second
			}
			if status.LastSeen.IsZero() {
				continue
			}
			if now.Sub(status.LastSeen) > threshold {
				status.Status = "stale"
				resource.SourceStatus[source] = status
				staleFound = true
			}
		}
		if staleFound {
			recomputed := aggregateStatus(resource)
			if recomputed != StatusUnknown {
				resource.Status = recomputed
			} else if resource.Status == StatusOnline {
				resource.Status = StatusWarning
			}
		}
	}
}

func (rr *ResourceRegistry) ingestProxmoxNode(node models.Node, linkedHost *models.Host) {
	resource, identity := resourceFromProxmoxNode(node, linkedHost)
	rr.ingest(SourceProxmox, node.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestHost(host models.Host) {
	resource, identity := resourceFromHost(host)
	rr.ingest(SourceAgent, host.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestHostUnraidStorage(host models.Host) {
	if host.Unraid == nil {
		return
	}

	resource, identity := resourceFromHostUnraidStorage(host)
	parentID := rr.sourceResourceID(SourceAgent, host.ID)
	if parentID != "" {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceAgent, hostUnraidStorageSourceID(host), resource, identity)

	for _, disk := range host.Unraid.Disks {
		if unraidDiskRole(&disk) != "cache" {
			continue
		}
		resource, identity := resourceFromHostUnraidCacheStorage(host, disk)
		if parentID != "" {
			resource.ParentID = &parentID
		}
		if sourceID := hostUnraidCacheStorageSourceID(host, disk); sourceID != "" {
			rr.ingest(SourceAgent, sourceID, resource, identity)
		}
	}
}

func (rr *ResourceRegistry) ingestHostUnraidPhysicalDisks(host models.Host) {
	if host.Unraid == nil || len(host.Unraid.Disks) == 0 {
		return
	}

	hostParentID := rr.sourceResourceID(SourceAgent, host.ID)
	unraidStorageID := rr.sourceResourceID(SourceAgent, hostUnraidStorageSourceID(host))
	cacheStorageIDs := make(map[string]string)
	for _, disk := range host.Unraid.Disks {
		if unraidDiskRole(&disk) != "cache" {
			continue
		}
		sourceID := hostUnraidCacheStorageSourceID(host, disk)
		if sourceID == "" {
			continue
		}
		if resourceID := rr.sourceResourceID(SourceAgent, sourceID); resourceID != "" {
			cacheStorageIDs[unraidCachePoolName(disk)] = resourceID
		}
	}

	for _, disk := range host.Unraid.Disks {
		if strings.TrimSpace(disk.Device) == "" {
			continue
		}
		resource, identity := resourceFromHostUnraidPhysicalDisk(host, disk)
		if resource.PhysicalDisk == nil {
			continue
		}
		parentID := hostParentID
		switch unraidDiskGroup(&disk) {
		case "unraid-array":
			if unraidStorageID != "" {
				parentID = unraidStorageID
			}
		default:
			if cacheID := cacheStorageIDs[unraidCachePoolName(disk)]; cacheID != "" {
				parentID = cacheID
			}
		}
		if parentID != "" {
			resource.ParentID = &parentID
		}
		sourceID := HostUnraidDiskSourceID(host, disk)
		if sourceID == "" {
			continue
		}
		rr.ingest(SourceAgent, sourceID, resource, identity)
	}
}

func (rr *ResourceRegistry) ingestHostSMARTDisks(host models.Host) {
	if len(host.Sensors.SMART) == 0 {
		return
	}

	hostParentID := rr.sourceResourceID(SourceAgent, host.ID)
	unraidStorageID := rr.sourceResourceID(SourceAgent, hostUnraidStorageSourceID(host))
	for _, disk := range host.Sensors.SMART {
		if fsfilters.IsVirtualBlockDevice(disk.Device) {
			continue
		}
		resource, identity := resourceFromHostSMARTDisk(host, disk)
		if resource.PhysicalDisk == nil {
			continue
		}
		parentID := hostParentID
		if matched := matchUnraidDisk(host.Unraid, disk); matched != nil {
			switch unraidDiskGroup(matched) {
			case "unraid-array":
				if unraidStorageID != "" {
					parentID = unraidStorageID
				}
			default:
				if cacheID := rr.sourceResourceID(SourceAgent, hostUnraidCacheStorageSourceID(host, *matched)); cacheID != "" {
					parentID = cacheID
				}
			}
		}
		if parentID != "" {
			resource.ParentID = &parentID
		}
		sourceID := HostSMARTDiskSourceID(host, disk)
		if sourceID == "" {
			continue
		}
		rr.ingest(SourceAgent, sourceID, resource, identity)
	}
}

func (rr *ResourceRegistry) ingestDockerHost(host models.DockerHost) {
	resource, identity := resourceFromDockerHost(host)
	rr.ingest(SourceDocker, host.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestPBSInstance(instance models.PBSInstance) {
	resource, identity := resourceFromPBSInstance(instance)
	sourceID := pbsInstanceSourceID(instance)
	rr.ingest(SourcePBS, sourceID, resource, identity)
	parentID := rr.sourceResourceID(SourcePBS, sourceID)
	for _, datastore := range instance.Datastores {
		resource, identity := resourceFromPBSDatastore(instance, datastore)
		if parentID != "" {
			resource.ParentID = &parentID
		}
		rr.ingest(SourcePBS, pbsDatastoreSourceID(instance, datastore), resource, identity)
	}
}

func (rr *ResourceRegistry) ingestPMGInstance(instance models.PMGInstance) {
	resource, identity := resourceFromPMGInstance(instance)
	sourceID := pmgInstanceSourceID(instance)
	rr.ingest(SourcePMG, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestVM(vm models.VM, clusterByInstance map[string]string) {
	resource, identity := resourceFromVM(vm)
	sourceID := proxmoxVMSourceID(vm)
	clusterName := clusterByInstance[vm.Instance]
	if parentID := rr.proxmoxNodeParentID(vm.Instance, clusterName, vm.Node, ""); parentID != "" {
		resource.ParentID = &parentID
		attachLinkedAgentID(resource.Proxmox, rr.linkedAgentIDForResource(parentID))
	}
	if clusterName != "" && resource.Proxmox != nil {
		resource.Proxmox.ClusterName = clusterName
	}
	rr.ingest(SourceProxmox, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestContainer(ct models.Container, clusterByInstance map[string]string) {
	resource, identity := resourceFromContainer(ct)
	sourceID := proxmoxContainerSourceID(ct)
	clusterName := clusterByInstance[ct.Instance]
	if parentID := rr.proxmoxNodeParentID(ct.Instance, clusterName, ct.Node, ""); parentID != "" {
		resource.ParentID = &parentID
		attachLinkedAgentID(resource.Proxmox, rr.linkedAgentIDForResource(parentID))
	}
	if clusterName != "" && resource.Proxmox != nil {
		resource.Proxmox.ClusterName = clusterName
	}
	rr.ingest(SourceProxmox, sourceID, resource, identity)
}

func (rr *ResourceRegistry) linkedAgentIDForResource(resourceID string) string {
	if rr == nil {
		return ""
	}
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	resourceID = CanonicalResourceID(resourceID)
	return linkedAgentIDFromResource(rr.resources[resourceID])
}

func attachLinkedAgentID(proxmox *ProxmoxData, linkedAgentID string) {
	if proxmox == nil {
		return
	}
	linkedAgentID = strings.TrimSpace(linkedAgentID)
	if linkedAgentID == "" {
		return
	}
	proxmox.LinkedAgentID = linkedAgentID
}

func (rr *ResourceRegistry) ingestStorage(storage models.Storage) {
	resource, identity := resourceFromStorage(storage)
	if pbsStorageInstance(storage) {
		// Datastore-derived entries from the PBS poller (monitor_pbs_pmg)
		// are delivered by the PBS source on the PBS cadence, not by a PVE
		// poll. Keying them SourceProxmox would judge their freshness
		// against the (faster) Proxmox stale threshold and flap healthy
		// datastores stale between PBS polls. storage.Instance carries the
		// PBS instance source ID ("pbs-<name>"), so parent them to the PBS
		// instance, which IngestSnapshot ingests before storage.
		if parentID, ok := rr.bySource[SourcePBS][storage.Instance]; ok {
			resource.ParentID = &parentID
		}
		rr.ingest(SourcePBS, storage.ID, resource, identity)
		return
	}
	parentSourceID := proxmoxNodeSourceID(storage.Instance, storage.Node)
	if parentID, ok := rr.bySource[SourceProxmox][parentSourceID]; ok {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceProxmox, storage.ID, resource, identity)
}

// pbsStorageInstance reports whether a storage entry was produced by the PBS
// poller's datastore conversion rather than a PVE storage poll. PVE itself
// reports pbs-typed storage.cfg backends too, but those carry the PVE
// instance name; only the PBS poller writes the "pbs-" instance prefix.
func pbsStorageInstance(storage models.Storage) bool {
	return strings.EqualFold(strings.TrimSpace(storage.Type), "pbs") &&
		strings.HasPrefix(strings.TrimSpace(storage.Instance), "pbs-")
}

func (rr *ResourceRegistry) ingestPhysicalDisk(disk models.PhysicalDisk) {
	resource, identity := resourceFromPhysicalDisk(disk)
	parentSourceID := proxmoxNodeSourceID(disk.Instance, disk.Node)
	if parentID, ok := rr.bySource[SourceProxmox][parentSourceID]; ok {
		resource.ParentID = &parentID
	}
	rr.ingest(SourceProxmox, disk.ID, resource, identity)
}

func (rr *ResourceRegistry) ingestCephCluster(cluster models.CephCluster) {
	resource, identity := resourceFromCephCluster(cluster)
	sourceID := cluster.FSID
	if sourceID == "" {
		sourceID = cluster.ID
	}
	rr.ingest(SourceProxmox, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerContainer(ct models.DockerContainer, host models.DockerHost) {
	resource, identity := resourceFromDockerContainer(ct, host)
	if parentID, ok := rr.bySource[SourceDocker][host.ID]; ok {
		resource.ParentID = &parentID
	}
	// Scope the source key to the docker host so that two containers
	// reported by different docker hosts can never collapse into a
	// single registry entry. Without host scoping, anything that
	// produces a colliding container source ID (a docker ps that
	// briefly returns an empty ID and falls back to the container
	// name in parseDockerInventoryContainerLine, a future short-ID
	// truncation, or a daemon-side identifier reset across recreate
	// cycles) would route both containers to the same resource ID,
	// and mergeInto would overwrite the Docker payload — including
	// HostSourceID and ParentID — with whichever ingest ran second.
	// That race surfaced as the "frigate@141" host re-attribution
	// flicker on the Docker page.
	rr.ingest(SourceDocker, dockerContainerSourceID(host, ct), resource, identity)
}

// dockerContainerSourceID produces a stable per-(host, container) key for
// the bySource[SourceDocker] mapping. The hashed resource ID derives from
// this same string via sourceSpecificID, so it must remain stable across
// projection rebuilds for a given container under a given docker host.
func dockerContainerSourceID(host models.DockerHost, ct models.DockerContainer) string {
	hostID := strings.TrimSpace(host.ID)
	ctID := strings.TrimSpace(ct.ID)
	if hostID == "" {
		return ctID
	}
	if ctID == "" {
		ctID = "name:" + strings.TrimSpace(ct.Name)
	}
	return hostID + "/container/" + ctID
}

func (rr *ResourceRegistry) ingestDockerService(service models.DockerService, host models.DockerHost) {
	resource, identity := resourceFromDockerService(service, host)
	sourceID := dockerServiceSourceID(host, service)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerImage(image models.DockerImage, host models.DockerHost) {
	resource, identity := resourceFromDockerImage(image, host)
	if parentID := rr.sourceResourceID(SourceDocker, host.ID); parentID != "" {
		resource.ParentID = &parentID
	}
	sourceID := dockerImageSourceID(host, image)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerVolume(volume models.DockerVolume, host models.DockerHost) {
	resource, identity := resourceFromDockerVolume(volume, host)
	if parentID := rr.sourceResourceID(SourceDocker, host.ID); parentID != "" {
		resource.ParentID = &parentID
	}
	sourceID := dockerVolumeSourceID(host, volume)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerNetwork(network models.DockerNetwork, host models.DockerHost) {
	resource, identity := resourceFromDockerNetwork(network, host)
	if parentID := rr.sourceResourceID(SourceDocker, host.ID); parentID != "" {
		resource.ParentID = &parentID
	}
	sourceID := dockerNetworkSourceID(host, network)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) refreshDockerNetworkAttachmentRelationships(host models.DockerHost) {
	if len(host.Containers) == 0 && len(host.Networks) == 0 {
		return
	}

	observedAt := host.LastSeen
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	dockerSource := rr.bySource[SourceDocker]
	networkIDByName := make(map[string]string, len(host.Networks))
	for _, network := range host.Networks {
		networkName := normalizeDockerNetworkName(network.Name)
		if networkName == "" {
			continue
		}
		sourceID := normalizeSourceID(dockerNetworkSourceID(host, network))
		resourceID := dockerSource[sourceID]
		if resourceID == "" {
			continue
		}
		networkIDByName[networkName] = resourceID
	}

	relationshipsByContainer := make(map[string][]ResourceRelationship, len(host.Containers))
	relationshipsByNetwork := make(map[string][]ResourceRelationship, len(networkIDByName))

	for _, container := range host.Containers {
		containerSourceID := normalizeSourceID(dockerContainerSourceID(host, container))
		containerID := dockerSource[containerSourceID]
		if containerID == "" {
			continue
		}
		for _, attachment := range container.Networks {
			networkID := networkIDByName[normalizeDockerNetworkName(attachment.Name)]
			if networkID == "" {
				continue
			}
			relationship := ResourceRelationship{
				SourceID:   containerID,
				TargetID:   networkID,
				Type:       RelAttachedTo,
				Confidence: 1,
				Active:     true,
				Discoverer: dockerAdapterRelationshipDiscoverer,
				Metadata:   dockerNetworkAttachmentMetadata(host, attachment),
			}
			relationshipsByContainer[containerID] = append(relationshipsByContainer[containerID], relationship)
			relationshipsByNetwork[networkID] = append(relationshipsByNetwork[networkID], relationship)
		}
	}

	for _, container := range host.Containers {
		containerID := dockerSource[normalizeSourceID(dockerContainerSourceID(host, container))]
		if containerID == "" {
			continue
		}
		rr.setDockerNetworkAttachmentRelationshipsLocked(
			containerID,
			relationshipsByContainer[containerID],
			observedAt,
		)
	}
	for _, networkID := range networkIDByName {
		rr.setDockerNetworkAttachmentRelationshipsLocked(
			networkID,
			relationshipsByNetwork[networkID],
			observedAt,
		)
	}
}

func (rr *ResourceRegistry) setDockerNetworkAttachmentRelationshipsLocked(
	resourceID string,
	relationships []ResourceRelationship,
	observedAt time.Time,
) {
	resource := rr.resources[resourceID]
	if resource == nil {
		return
	}

	existingByEdge := make(map[string]ResourceRelationship)
	base := make([]ResourceRelationship, 0, len(resource.Relationships)+len(relationships))
	for _, relationship := range resource.Relationships {
		if isDockerNetworkAttachmentRelationship(relationship) {
			existingByEdge[resourceRelationshipEdgeKey(relationship)] = relationship
			continue
		}
		base = append(base, relationship)
	}

	nextDockerRelationships := make([]ResourceRelationship, 0, len(relationships))
	seen := make(map[string]struct{}, len(relationships))
	for _, relationship := range relationships {
		key := resourceRelationshipEdgeKey(relationship)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if existing, ok := existingByEdge[key]; ok {
			relationship.ObservedAt = existing.ObservedAt
			relationship.LastSeenAt = existing.LastSeenAt
		}
		if relationship.ObservedAt.IsZero() {
			relationship.ObservedAt = observedAt
		}
		if relationship.LastSeenAt.IsZero() {
			relationship.LastSeenAt = observedAt
		}
		nextDockerRelationships = append(nextDockerRelationships, relationship)
	}
	sort.Slice(nextDockerRelationships, func(i, j int) bool {
		left := nextDockerRelationships[i]
		right := nextDockerRelationships[j]
		if left.SourceID != right.SourceID {
			return left.SourceID < right.SourceID
		}
		return left.TargetID < right.TargetID
	})

	resource.Relationships = append(base, nextDockerRelationships...)
}

func isDockerNetworkAttachmentRelationship(relationship ResourceRelationship) bool {
	return relationship.Type == RelAttachedTo &&
		strings.TrimSpace(relationship.Discoverer) == dockerAdapterRelationshipDiscoverer
}

func resourceRelationshipEdgeKey(relationship ResourceRelationship) string {
	sourceID := CanonicalResourceID(relationship.SourceID)
	targetID := CanonicalResourceID(relationship.TargetID)
	if sourceID == "" || targetID == "" || relationship.Type == "" {
		return ""
	}
	return strings.Join(
		[]string{sourceID, targetID, string(relationship.Type), strings.TrimSpace(relationship.Discoverer)},
		"\x00",
	)
}

func normalizeDockerNetworkName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func dockerNetworkAttachmentMetadata(
	host models.DockerHost,
	attachment models.DockerContainerNetworkLink,
) map[string]any {
	metadata := make(map[string]any, 5)
	if hostSourceID := strings.TrimSpace(host.ID); hostSourceID != "" {
		metadata["hostSourceId"] = hostSourceID
	}
	if hostname := strings.TrimSpace(host.Hostname); hostname != "" {
		metadata["host"] = hostname
	}
	if networkName := strings.TrimSpace(attachment.Name); networkName != "" {
		metadata["networkName"] = networkName
	}
	if ipv4 := strings.TrimSpace(attachment.IPv4); ipv4 != "" {
		metadata["ipv4"] = ipv4
	}
	if ipv6 := strings.TrimSpace(attachment.IPv6); ipv6 != "" {
		metadata["ipv6"] = ipv6
	}
	return metadata
}

func (rr *ResourceRegistry) ingestDockerTask(task models.DockerTask, host models.DockerHost) {
	resource, identity := resourceFromDockerTask(task, host)
	parentID := ""
	if serviceSourceID := dockerServiceSourceID(host, models.DockerService{ID: task.ServiceID, Name: task.ServiceName}); serviceSourceID != "" {
		parentID = rr.sourceResourceID(SourceDocker, serviceSourceID)
	}
	if parentID == "" {
		parentID = rr.sourceResourceID(SourceDocker, host.ID)
	}
	if parentID != "" {
		resource.ParentID = &parentID
	}
	sourceID := dockerTaskSourceID(host, task)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerSwarmNode(node models.DockerNode, host models.DockerHost) {
	resource, identity := resourceFromDockerSwarmNode(node, host)
	if parentID := rr.sourceResourceID(SourceDocker, host.ID); parentID != "" {
		resource.ParentID = &parentID
	}
	sourceID := dockerSwarmNodeSourceID(host, node)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerSecret(secret models.DockerSecret, host models.DockerHost) {
	resource, identity := resourceFromDockerSecret(secret, host)
	sourceID := dockerSecretSourceID(host, secret)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestDockerConfig(config models.DockerConfig, host models.DockerHost) {
	resource, identity := resourceFromDockerConfig(config, host)
	sourceID := dockerConfigSourceID(host, config)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceDocker, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesCluster(cluster models.KubernetesCluster, linkedHosts []*models.Host, capabilities *K8sMetricCapabilities) string {
	resource, identity := resourceFromKubernetesCluster(cluster, linkedHosts, capabilities)
	sourceID := kubernetesClusterSourceID(cluster)
	if sourceID == "" {
		return ""
	}
	return rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesNode(cluster models.KubernetesCluster, node models.KubernetesNode, linkedHost *models.Host, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesNode(cluster, node, linkedHost, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesNodeSourceID(kubernetesClusterSourceID(cluster), node)
	if sourceID == "" {
		return
	}
	if rr.mergeLinkedKubernetesNode(sourceID, resource, identity, linkedHost) {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesNamespace(cluster models.KubernetesCluster, namespace models.KubernetesNamespace, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesNamespace(cluster, namespace, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesNamespaceSourceID(kubernetesClusterSourceID(cluster), namespace)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesService(cluster models.KubernetesCluster, service models.KubernetesService, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesService(cluster, service, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesServiceSourceID(kubernetesClusterSourceID(cluster), service)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesStatefulSet(cluster models.KubernetesCluster, statefulSet models.KubernetesStatefulSet, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesStatefulSet(cluster, statefulSet, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesStatefulSetSourceID(kubernetesClusterSourceID(cluster), statefulSet)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesDaemonSet(cluster models.KubernetesCluster, daemonSet models.KubernetesDaemonSet, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesDaemonSet(cluster, daemonSet, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesDaemonSetSourceID(kubernetesClusterSourceID(cluster), daemonSet)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesJob(cluster models.KubernetesCluster, job models.KubernetesJob, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesJob(cluster, job, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesJobSourceID(kubernetesClusterSourceID(cluster), job)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesCronJob(cluster models.KubernetesCluster, cronJob models.KubernetesCronJob, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesCronJob(cluster, cronJob, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesCronJobSourceID(kubernetesClusterSourceID(cluster), cronJob)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesIngress(cluster models.KubernetesCluster, ingress models.KubernetesIngress, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesIngress(cluster, ingress, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesIngressSourceID(kubernetesClusterSourceID(cluster), ingress)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesEndpointSlice(cluster models.KubernetesCluster, slice models.KubernetesEndpointSlice, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesEndpointSlice(cluster, slice, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesEndpointSliceSourceID(kubernetesClusterSourceID(cluster), slice)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesNetworkPolicy(cluster models.KubernetesCluster, policy models.KubernetesNetworkPolicy, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesNetworkPolicy(cluster, policy, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesNetworkPolicySourceID(kubernetesClusterSourceID(cluster), policy)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesPersistentVolume(cluster models.KubernetesCluster, volume models.KubernetesPersistentVolume, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesPersistentVolume(cluster, volume, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesPersistentVolumeSourceID(kubernetesClusterSourceID(cluster), volume)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesPersistentVolumeClaim(cluster models.KubernetesCluster, claim models.KubernetesPersistentVolumeClaim, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesPersistentVolumeClaim(cluster, claim, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesPersistentVolumeClaimSourceID(kubernetesClusterSourceID(cluster), claim)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesStorageClass(cluster models.KubernetesCluster, class models.KubernetesStorageClass, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesStorageClass(cluster, class, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesStorageClassSourceID(kubernetesClusterSourceID(cluster), class)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesConfigMap(cluster models.KubernetesCluster, configMap models.KubernetesConfigMap, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesConfigMap(cluster, configMap, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesConfigMapSourceID(kubernetesClusterSourceID(cluster), configMap)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesSecret(cluster models.KubernetesCluster, secret models.KubernetesSecret, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesSecret(cluster, secret, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesSecretSourceID(kubernetesClusterSourceID(cluster), secret)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesServiceAccount(cluster models.KubernetesCluster, account models.KubernetesServiceAccount, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesServiceAccount(cluster, account, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesServiceAccountSourceID(kubernetesClusterSourceID(cluster), account)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesRole(cluster models.KubernetesCluster, role models.KubernetesRole, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesRole(cluster, role, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesRoleSourceID(kubernetesClusterSourceID(cluster), role)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesClusterRole(cluster models.KubernetesCluster, role models.KubernetesClusterRole, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesClusterRole(cluster, role, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesClusterRoleSourceID(kubernetesClusterSourceID(cluster), role)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesRoleBinding(cluster models.KubernetesCluster, binding models.KubernetesRoleBinding, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesRoleBinding(cluster, binding, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesRoleBindingSourceID(kubernetesClusterSourceID(cluster), binding)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesClusterRoleBinding(cluster models.KubernetesCluster, binding models.KubernetesClusterRoleBinding, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesClusterRoleBinding(cluster, binding, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesClusterRoleBindingSourceID(kubernetesClusterSourceID(cluster), binding)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesResourceQuota(cluster models.KubernetesCluster, quota models.KubernetesResourceQuota, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesResourceQuota(cluster, quota, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesResourceQuotaSourceID(kubernetesClusterSourceID(cluster), quota)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesLimitRange(cluster models.KubernetesCluster, limitRange models.KubernetesLimitRange, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesLimitRange(cluster, limitRange, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesLimitRangeSourceID(kubernetesClusterSourceID(cluster), limitRange)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesPodDisruptionBudget(cluster models.KubernetesCluster, budget models.KubernetesPodDisruptionBudget, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesPodDisruptionBudget(cluster, budget, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesPodDisruptionBudgetSourceID(kubernetesClusterSourceID(cluster), budget)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesHorizontalPodAutoscaler(cluster models.KubernetesCluster, autoscaler models.KubernetesHorizontalPodAutoscaler, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesHorizontalPodAutoscaler(cluster, autoscaler, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesHorizontalPodAutoscalerSourceID(kubernetesClusterSourceID(cluster), autoscaler)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesEvent(cluster models.KubernetesCluster, event models.KubernetesEvent, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesEvent(cluster, event, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesEventSourceID(kubernetesClusterSourceID(cluster), event)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesPod(cluster models.KubernetesCluster, pod models.KubernetesPod, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesPod(cluster, pod, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesPodSourceID(kubernetesClusterSourceID(cluster), pod)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesDeployment(cluster models.KubernetesCluster, deployment models.KubernetesDeployment, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesDeployment(cluster, deployment, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesDeploymentSourceID(kubernetesClusterSourceID(cluster), deployment)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingestKubernetesReplicaSet(cluster models.KubernetesCluster, replicaSet models.KubernetesReplicaSet, clusterResourceID string, capabilities *K8sMetricCapabilities) {
	resource, identity := resourceFromKubernetesReplicaSet(cluster, replicaSet, capabilities)
	if clusterResourceID != "" {
		resource.ParentID = &clusterResourceID
	}
	sourceID := kubernetesReplicaSetSourceID(kubernetesClusterSourceID(cluster), replicaSet)
	if sourceID == "" {
		return
	}
	rr.ingest(SourceK8s, sourceID, resource, identity)
}

func (rr *ResourceRegistry) ingest(source DataSource, sourceID string, resource Resource, identity ResourceIdentity) string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	sourceID = normalizeSourceID(sourceID)
	if sourceID == "" {
		return ""
	}
	if _, ok := rr.bySource[source]; !ok {
		rr.bySource[source] = make(map[string]string)
	}

	resource.Type = CanonicalResourceType(resource.Type)
	// Complete weak host identities from the durable pins before matching and
	// ID derivation, so canonical IDs do not depend on which sources happen to
	// be present in this rebuild (boot windows ingest the Proxmox node record
	// before the agent has checked in).
	identity = rr.completeIdentityFromPins(resource.Type, identity)
	resource.Identity = identity
	resource.Sources = []DataSource{source}
	resource.SourceStatus = map[DataSource]SourceStatus{
		source: {Status: sourceSightingStatus(resource.LastSeen), LastSeen: resource.LastSeen},
	}
	resource.parentBySource = make(map[DataSource]string)
	rr.setSourceParent(&resource, source, resource.ParentID)

	// Linked resources must be mutually linked to avoid one-sided/ambiguous auto-merges.
	if linked := rr.resolveLinkedResource(source, sourceID, resource); linked != "" {
		existing := rr.resources[linked]
		if existing != nil {
			rr.mergeInto(existing, resource, source)
			rr.bySource[source][sourceID] = existing.ID
			return existing.ID
		}
	}

	// Agentless availability probes attach as a facet on the known resource
	// they monitor instead of minting a parallel network-endpoint. An explicit
	// linkedResourceId wins; otherwise an exact, unique IP match may attach.
	// Lossy hostname-only correlation is intentionally avoided to prevent
	// ambiguous one-sided merges, and the link is refused when it would
	// overwrite a different target's already-attached facet or fold one probe
	// into another.
	if source == SourceAvailability && resource.Type == ResourceTypeNetworkEndpoint {
		if linked := rr.resolveAvailabilityLink(resource); linked != "" {
			if existing := rr.resources[linked]; existing != nil {
				rr.mergeInto(existing, resource, source)
				rr.bySource[source][sourceID] = existing.ID
				return existing.ID
			}
		}
	}

	candidateID := rr.sourceSpecificID(resource.Type, source, sourceID)

	if resource.Type == ResourceTypeAgent || resource.Type == ResourceTypePhysicalDisk {
		if match, excluded := rr.findMatch(identity, resource.Type, candidateID); match != nil {
			existing := rr.resources[match.ResourceB]
			if existing != nil {
				rr.mergeInto(existing, resource, source)
				rr.bySource[source][sourceID] = existing.ID
				return existing.ID
			}
		} else if excluded {
			resource.ID = candidateID
			rr.resources[resource.ID] = &resource
			rr.bySource[source][sourceID] = resource.ID
			rr.matcher.Add(resource.ID, identity)
			return resource.ID
		}
	}

	resource.ID = rr.chooseNewID(resource.Type, identity, source, sourceID)
	if existing := rr.resources[resource.ID]; existing != nil {
		rr.mergeInto(existing, resource, source)
		rr.bySource[source][sourceID] = existing.ID
		rr.matcher.Add(existing.ID, existing.Identity)
		return existing.ID
	}
	rr.resources[resource.ID] = &resource
	rr.bySource[source][sourceID] = resource.ID
	rr.matcher.Add(resource.ID, identity)

	return resource.ID
}

func (rr *ResourceRegistry) mergeLinkedKubernetesNode(
	sourceID string,
	resource Resource,
	identity ResourceIdentity,
	linkedHost *models.Host,
) bool {
	if linkedHost == nil {
		return false
	}

	linkedAgentSourceID := normalizeSourceID(strings.TrimSpace(linkedHost.ID))
	if linkedAgentSourceID == "" {
		return false
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	existingID := rr.bySource[SourceAgent][linkedAgentSourceID]
	existing := rr.resources[existingID]
	if existing == nil || existing.Agent == nil {
		return false
	}

	resource.Identity = identity
	resource.Type = CanonicalResourceType(resource.Type)

	rr.mergeInto(existing, resource, SourceK8s)
	rr.bySource[SourceK8s][sourceID] = existing.ID
	rr.matcher.Add(existing.ID, existing.Identity)
	return true
}

func (rr *ResourceRegistry) findMatch(identity ResourceIdentity, resourceType ResourceType, candidateID string) (*MatchResult, bool) {
	excludedMatch := false
	candidates := rr.matcher.FindCandidates(identity)
	for _, candidate := range candidates {
		if candidate.ID == "" {
			continue
		}
		if candidate.Confidence < autoMergeThreshold {
			continue
		}
		existing := rr.resources[candidate.ID]
		if existing == nil || existing.Type != resourceType {
			continue
		}
		if rr.isExcluded(candidate.ID, candidateID) {
			excludedMatch = true
			continue
		}
		return &MatchResult{
			ResourceA:      candidateID,
			ResourceB:      candidate.ID,
			Confidence:     candidate.Confidence,
			MatchReason:    candidate.Reason,
			RequiresReview: candidate.RequiresReview,
		}, false
	}
	return nil, excludedMatch
}

func (rr *ResourceRegistry) resolveLinkedResource(source DataSource, sourceID string, resource Resource) string {
	switch source {
	case SourceProxmox:
		if resource.Proxmox != nil && resource.Proxmox.LinkedAgentID != "" {
			if id, ok := rr.bySource[SourceAgent][resource.Proxmox.LinkedAgentID]; ok {
				existing := rr.resources[id]
				if existing == nil || existing.Agent == nil {
					return ""
				}
				linkedNodeID := strings.TrimSpace(existing.Agent.LinkedNodeID)
				if linkedNodeID == sourceID {
					return id
				}
				if linkedNodeID == "" && identitiesShareHostname(existing.Identity, resource.Identity) {
					return id
				}
			}
		}
	case SourceAgent:
		if resource.Agent != nil && resource.Agent.LinkedNodeID != "" {
			if id, ok := rr.bySource[SourceProxmox][resource.Agent.LinkedNodeID]; ok {
				existing := rr.resources[id]
				if existing == nil || existing.Proxmox == nil {
					return ""
				}
				linkedHostID := strings.TrimSpace(existing.Proxmox.LinkedAgentID)
				if linkedHostID == "" || linkedHostID != sourceID {
					return ""
				}
				return id
			}
		}
		if resource.Agent != nil {
			return rr.findCorroboratedOneSidedProxmoxLink(sourceID, resource.Identity)
		}
	}
	return ""
}

// resolveAvailabilityLink attaches an agentless availability probe to the
// known resource it monitors, instead of minting a parallel
// network-endpoint. An explicit linkedResourceId wins unambiguously;
// otherwise the probe address may attach on an exact, unique IP overlap.
// Lossy hostname-only correlation is intentionally avoided, and the link is
// refused when it would overwrite a different target's already-attached
// availability facet or fold one probe into another.
func (rr *ResourceRegistry) resolveAvailabilityLink(resource Resource) string {
	if resource.Availability == nil {
		return ""
	}

	if linkedID := strings.TrimSpace(resource.Availability.LinkedResourceID); linkedID != "" {
		if resolved := rr.resolveAvailabilityLinkedResource(linkedID, resource); resolved != "" {
			return resolved
		}
	}

	matchID := ""
	for _, candidate := range rr.matcher.FindCandidates(resource.Identity) {
		if candidate.Reason != "ip" && candidate.Reason != "hostname+ip" {
			continue
		}
		existing := rr.resources[candidate.ID]
		if existing == nil || isAvailabilityOwnedResource(*existing) {
			continue
		}
		if !availabilityFacetCompatible(existing, resource) {
			continue
		}
		if matchID != "" && matchID != candidate.ID {
			return ""
		}
		matchID = candidate.ID
	}
	return matchID
}

func (rr *ResourceRegistry) resolveAvailabilityLinkedResource(ref string, incoming Resource) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}

	exactID := CanonicalResourceID(ref)
	if existing := rr.resources[exactID]; existing != nil {
		if !isAvailabilityOwnedResource(*existing) && availabilityFacetCompatible(existing, incoming) {
			return exactID
		}
		return ""
	}

	for _, candidateID := range uniqueTrimmed(
		rr.uniqueSourceResourceIDLocked(ref),
		rr.uniqueCanonicalIdentityResourceIDLocked(ref),
	) {
		existing := rr.resources[candidateID]
		if existing != nil && !isAvailabilityOwnedResource(*existing) && availabilityFacetCompatible(existing, incoming) {
			return candidateID
		}
	}

	return ""
}

// isAvailabilityOwnedResource reports whether a resource is itself an
// agentless availability endpoint (a standalone network-endpoint, or any
// resource sourced only from availability), which probes must never fold into.
func isAvailabilityOwnedResource(r Resource) bool {
	if r.Type == ResourceTypeNetworkEndpoint {
		return true
	}
	if len(r.Sources) == 0 {
		return false
	}
	for _, s := range r.Sources {
		if s != SourceAvailability {
			return false
		}
	}
	return true
}

// availabilityFacetCompatible reports whether an existing resource can accept
// the incoming availability facet without silently overwriting a different
// target's already-attached probe.
func availabilityFacetCompatible(existing *Resource, incoming Resource) bool {
	if existing == nil || existing.Availability == nil {
		return true
	}
	current := strings.TrimSpace(existing.Availability.TargetID)
	if current == "" {
		return true
	}
	incomingID := strings.TrimSpace(incoming.Availability.TargetID)
	return incomingID == "" || incomingID == current
}

func (rr *ResourceRegistry) findCorroboratedOneSidedProxmoxLink(
	hostAgentID string,
	identity ResourceIdentity,
) string {
	if strings.TrimSpace(hostAgentID) == "" {
		return ""
	}

	matchID := ""
	for _, resourceID := range rr.bySource[SourceProxmox] {
		existing := rr.resources[resourceID]
		if existing == nil || existing.Proxmox == nil {
			continue
		}
		if strings.TrimSpace(existing.Proxmox.LinkedAgentID) != hostAgentID {
			continue
		}
		if !identitiesShareHostname(existing.Identity, identity) {
			continue
		}
		if matchID != "" && matchID != resourceID {
			return ""
		}
		matchID = resourceID
	}

	return matchID
}

func identitiesShareHostname(a, b ResourceIdentity) bool {
	if len(a.Hostnames) == 0 || len(b.Hostnames) == 0 {
		return false
	}

	seen := make(map[string]struct{}, len(a.Hostnames))
	for _, hostname := range a.Hostnames {
		normalized := NormalizeHostname(hostname)
		if normalized == "" {
			continue
		}
		seen[normalized] = struct{}{}
	}

	for _, hostname := range b.Hostnames {
		normalized := NormalizeHostname(hostname)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			return true
		}
	}

	return false
}

func (rr *ResourceRegistry) mergeInto(existing *Resource, incoming Resource, source DataSource) {
	if existing == nil {
		return
	}

	rr.setSourceParent(existing, source, incoming.ParentID)

	// Merge identity
	existing.Identity = mergeIdentity(existing.Identity, incoming.Identity)

	// Merge tags
	existing.Tags = uniqueStrings(append(existing.Tags, incoming.Tags...))
	existing.Incidents = mergeResourceIncidents(existing.Incidents, incoming.Incidents)

	mergedPhysicalDisk := incoming.PhysicalDisk != nil
	if mergedPhysicalDisk {
		previous := existing.PhysicalDisk
		existing.PhysicalDisk = mergePhysicalDiskData(existing.PhysicalDisk, incoming.PhysicalDisk)
		if source == SourceProxmox && previous != nil && hasDataSource(existing.Sources, SourceAgent) {
			if previous.Temperature > 0 {
				existing.PhysicalDisk.Temperature = previous.Temperature
			}
			if previous.Wearout >= 0 {
				existing.PhysicalDisk.Wearout = previous.Wearout
			}
			if previous.SMART != nil {
				smart := *previous.SMART
				existing.PhysicalDisk.SMART = &smart
			}
		}
	}
	if existing.PhysicalDisk != nil && (mergedPhysicalDisk || len(incoming.Incidents) > 0) {
		existing.PhysicalDisk.Risk = physicalDiskRiskFromMeta(existing.PhysicalDisk, existing.Incidents)
	}

	// Update source payload
	switch source {
	case SourceProxmox:
		if mergedPhysicalDisk {
			break
		}
		existing.Proxmox = mergeProxmoxData(existing.Proxmox, incoming.Proxmox)
	case SourceAgent:
		if mergedPhysicalDisk {
			break
		}
		existing.Agent = incoming.Agent
	case SourceDocker:
		existing.Docker = incoming.Docker
	case SourcePBS:
		existing.PBS = incoming.PBS
	case SourceK8s:
		existing.Kubernetes = incoming.Kubernetes
	case SourcePMG:
		existing.PMG = incoming.PMG
	case SourceTrueNAS:
		existing.TrueNAS = mergeTrueNASData(existing.TrueNAS, incoming.TrueNAS)
		if incoming.Agent != nil && !hasDataSource(existing.Sources, SourceAgent) {
			existing.Agent = incoming.Agent
		}
		if incoming.Docker != nil && !hasDataSource(existing.Sources, SourceDocker) {
			existing.Docker = incoming.Docker
		}
		if incoming.Storage != nil && existing.Storage == nil {
			existing.Storage = incoming.Storage
		}
	case SourceVMware:
		existing.VMware = mergeVMwareData(existing.VMware, incoming.VMware)
	case SourceAvailability:
		existing.Availability = incoming.Availability
	}

	existing.Sources = addSource(existing.Sources, source)
	if existing.SourceStatus == nil {
		existing.SourceStatus = make(map[DataSource]SourceStatus)
	}
	existing.SourceStatus[source] = SourceStatus{Status: sourceSightingStatus(incoming.LastSeen), LastSeen: incoming.LastSeen}

	if incoming.LastSeen.After(existing.LastSeen) {
		existing.LastSeen = incoming.LastSeen
	}
	now := time.Now().UTC()
	existing.UpdatedAt = now
	existing.ParentID = rr.resolveCanonicalParentID(existing)

	existing.Status = chooseStatus(existing.Status, incoming.Status, source)
	existing.Metrics = mergeMetrics(existing.Metrics, incoming.Metrics, source, now, existing.SourceStatus)

	// Prefer agent naming when available
	if incoming.Name != "" {
		if existing.Name == "" || sourcePriority(source) >= sourcePriority(SourceAgent) {
			existing.Name = incoming.Name
		}
	}
}

func mergeProxmoxData(existing *ProxmoxData, incoming *ProxmoxData) *ProxmoxData {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	merged := *existing
	if incoming.SourceID != "" {
		merged.SourceID = incoming.SourceID
	}
	if incoming.NodeName != "" {
		merged.NodeName = incoming.NodeName
	}
	if incoming.ClusterName != "" {
		merged.ClusterName = incoming.ClusterName
	}
	if incoming.IsClusterMember {
		merged.IsClusterMember = true
	}
	if incoming.Instance != "" {
		merged.Instance = incoming.Instance
	}
	if incoming.HostURL != "" {
		merged.HostURL = incoming.HostURL
	}
	if incoming.VMID != 0 {
		merged.VMID = incoming.VMID
	}
	if incoming.CPUs != 0 {
		merged.CPUs = incoming.CPUs
	}
	if incoming.Template {
		merged.Template = true
	}
	if incoming.Temperature != nil {
		temperature := *incoming.Temperature
		merged.Temperature = &temperature
	}
	if incoming.PVEVersion != "" {
		merged.PVEVersion = incoming.PVEVersion
	}
	if incoming.KernelVersion != "" {
		merged.KernelVersion = incoming.KernelVersion
	}
	if incoming.Uptime != 0 {
		merged.Uptime = incoming.Uptime
	}
	if !incoming.LastBackup.IsZero() {
		merged.LastBackup = incoming.LastBackup
	}
	if incoming.CPUInfo != nil {
		cpuInfo := *incoming.CPUInfo
		merged.CPUInfo = &cpuInfo
	}
	if len(incoming.LoadAverage) > 0 {
		merged.LoadAverage = append([]float64(nil), incoming.LoadAverage...)
	}
	if incoming.PendingUpdates != 0 {
		merged.PendingUpdates = incoming.PendingUpdates
	}
	if len(incoming.Disks) > 0 {
		merged.Disks = append([]DiskInfo(nil), incoming.Disks...)
	}
	if incoming.SwapUsed != 0 {
		merged.SwapUsed = incoming.SwapUsed
	}
	if incoming.SwapTotal != 0 {
		merged.SwapTotal = incoming.SwapTotal
	}
	if incoming.Balloon != 0 {
		merged.Balloon = incoming.Balloon
	}
	if incoming.Lock != "" {
		merged.Lock = incoming.Lock
	}
	if incoming.LinkedAgentID != "" {
		merged.LinkedAgentID = incoming.LinkedAgentID
	}

	return &merged
}

func mergeTrueNASData(existing *TrueNASData, incoming *TrueNASData) *TrueNASData {
	if existing == nil {
		return cloneTrueNASData(incoming)
	}
	if incoming == nil {
		return cloneTrueNASData(existing)
	}

	merged := *existing
	if incoming.Hostname != "" {
		merged.Hostname = incoming.Hostname
	}
	if incoming.Version != "" {
		merged.Version = incoming.Version
	}
	if incoming.UptimeSeconds != 0 {
		merged.UptimeSeconds = incoming.UptimeSeconds
	}
	if incoming.StorageRisk != nil {
		merged.StorageRisk = cloneStorageRisk(incoming.StorageRisk)
	}
	if incoming.StorageRiskSummary != "" {
		merged.StorageRiskSummary = incoming.StorageRiskSummary
	}
	if incoming.StoragePostureSummary != "" {
		merged.StoragePostureSummary = incoming.StoragePostureSummary
	}
	if incoming.ProtectionReduced {
		merged.ProtectionReduced = true
	}
	if incoming.ProtectionSummary != "" {
		merged.ProtectionSummary = incoming.ProtectionSummary
	}
	if incoming.RebuildInProgress {
		merged.RebuildInProgress = true
	}
	if incoming.RebuildSummary != "" {
		merged.RebuildSummary = incoming.RebuildSummary
	}
	if incoming.App != nil {
		merged.App = cloneTrueNASApp(incoming.App)
	}
	if incoming.VM != nil {
		merged.VM = cloneTrueNASVM(incoming.VM)
	}
	if incoming.Share != nil {
		merged.Share = cloneTrueNASShare(incoming.Share)
	}
	if len(incoming.Services) > 0 {
		merged.Services = cloneTrueNASServices(incoming.Services)
	}
	return &merged
}

func mergePhysicalDiskData(existing *PhysicalDiskMeta, incoming *PhysicalDiskMeta) *PhysicalDiskMeta {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	merged := *existing
	if shouldReplacePhysicalDiskDevPath(merged.DevPath, incoming.DevPath) {
		merged.DevPath = incoming.DevPath
	}
	if incoming.Model != "" {
		merged.Model = incoming.Model
	}
	if incoming.Serial != "" {
		merged.Serial = incoming.Serial
	}
	if incoming.WWN != "" {
		merged.WWN = incoming.WWN
	}
	if incoming.DiskType != "" {
		merged.DiskType = incoming.DiskType
	}
	if incoming.SizeBytes > 0 {
		merged.SizeBytes = incoming.SizeBytes
	}
	if shouldReplacePhysicalDiskHealth(merged.Health, incoming.Health) {
		merged.Health = incoming.Health
	}
	if incoming.Wearout >= 0 && (merged.Wearout < 0 || incoming.SMART != nil || merged.SMART == nil) {
		merged.Wearout = incoming.Wearout
	}
	if incoming.Temperature > 0 && (merged.Temperature == 0 || incoming.SMART != nil || merged.SMART == nil) {
		merged.Temperature = incoming.Temperature
	}
	if incoming.TemperatureAggregate != nil {
		merged.TemperatureAggregate = cloneTemperatureAggregateMeta(incoming.TemperatureAggregate)
	}
	if incoming.RPM > 0 {
		merged.RPM = incoming.RPM
	}
	if incoming.Used != "" {
		merged.Used = incoming.Used
	}
	if incoming.StorageRole != "" {
		merged.StorageRole = incoming.StorageRole
	}
	if incoming.StorageGroup != "" {
		merged.StorageGroup = incoming.StorageGroup
	}
	if incoming.StorageState != "" {
		merged.StorageState = incoming.StorageState
	}
	if incoming.SpunDown {
		merged.SpunDown = true
	}
	if incoming.ReadCount > 0 {
		merged.ReadCount = incoming.ReadCount
	}
	if incoming.WriteCount > 0 {
		merged.WriteCount = incoming.WriteCount
	}
	if incoming.ErrorCount > 0 {
		merged.ErrorCount = incoming.ErrorCount
	}
	if incoming.SMART != nil {
		smart := *incoming.SMART
		merged.SMART = &smart
	}
	merged.Risk = mergePhysicalDiskRisk(
		physicalDiskRiskFromAssessment(physicalDiskAssessmentFromMeta(&merged)),
		existing.Risk,
		incoming.Risk,
	)

	return &merged
}

// shouldReplacePhysicalDiskDevPath decides whether an incoming devPath should
// overwrite the existing one when two sources describe the same disk. A canonical
// /dev/<device> path (as produced by the Proxmox disks/list poll) must never be
// downgraded to a non-canonical scan label such as the host agent's
// "nvme0 [nvme]" controller display name. When both are equally canonical the
// incoming value wins, preserving the previous last-writer behaviour.
func shouldReplacePhysicalDiskDevPath(existing, incoming string) bool {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return false
	}
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return true
	}
	if isCanonicalDevPath(existing) && !isCanonicalDevPath(incoming) {
		return false
	}
	return true
}

// isCanonicalDevPath reports whether devPath is a clean /dev/<device> path with
// no whitespace or scan-type suffix (e.g. "/dev/nvme0n1", not "nvme0 [nvme]").
func isCanonicalDevPath(devPath string) bool {
	devPath = strings.TrimSpace(devPath)
	if !strings.HasPrefix(devPath, "/dev/") {
		return false
	}
	return !strings.ContainsAny(devPath, " \t[")
}

func shouldReplacePhysicalDiskHealth(existing, incoming string) bool {
	incoming = strings.ToUpper(strings.TrimSpace(incoming))
	if incoming == "" {
		return false
	}
	existing = strings.ToUpper(strings.TrimSpace(existing))
	switch incoming {
	case "UNKNOWN":
		return existing == ""
	case "FAILED":
		return true
	case "PASSED", "OK":
		return existing == "" || existing == "UNKNOWN" || existing == "PASSED" || existing == "OK"
	default:
		return true
	}
}

func mergePhysicalDiskRisk(base *PhysicalDiskRisk, risks ...*PhysicalDiskRisk) *PhysicalDiskRisk {
	var merged *PhysicalDiskRisk
	appendRisk := func(risk *PhysicalDiskRisk) {
		if risk == nil {
			return
		}
		if merged == nil {
			merged = &PhysicalDiskRisk{Level: risk.Level}
		}
		if incidentSeverityRank(risk.Level) > incidentSeverityRank(merged.Level) {
			merged.Level = risk.Level
		}
		seen := make(map[string]struct{}, len(merged.Reasons))
		for _, reason := range merged.Reasons {
			seen[reason.Code+"\x00"+reason.Summary] = struct{}{}
		}
		for _, reason := range risk.Reasons {
			key := reason.Code + "\x00" + reason.Summary
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged.Reasons = append(merged.Reasons, reason)
		}
	}
	appendRisk(base)
	for _, risk := range risks {
		appendRisk(risk)
	}
	if merged == nil || (merged.Level == storagehealth.RiskHealthy && len(merged.Reasons) == 0) {
		return nil
	}
	return merged
}

func mergeVMwareData(existing *VMwareData, incoming *VMwareData) *VMwareData {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	merged := *existing
	if incoming.ConnectionID != "" {
		merged.ConnectionID = incoming.ConnectionID
	}
	if incoming.ConnectionName != "" {
		merged.ConnectionName = incoming.ConnectionName
	}
	if incoming.VCenterHost != "" {
		merged.VCenterHost = incoming.VCenterHost
	}
	if incoming.ManagedObjectID != "" {
		merged.ManagedObjectID = incoming.ManagedObjectID
	}
	if incoming.EntityType != "" {
		merged.EntityType = incoming.EntityType
	}
	if incoming.HostUUID != "" {
		merged.HostUUID = incoming.HostUUID
	}
	if incoming.DatacenterID != "" {
		merged.DatacenterID = incoming.DatacenterID
	}
	if incoming.DatacenterName != "" {
		merged.DatacenterName = incoming.DatacenterName
	}
	if incoming.ComputeResourceID != "" {
		merged.ComputeResourceID = incoming.ComputeResourceID
	}
	if incoming.ComputeResourceName != "" {
		merged.ComputeResourceName = incoming.ComputeResourceName
	}
	if incoming.ClusterID != "" {
		merged.ClusterID = incoming.ClusterID
	}
	if incoming.ClusterName != "" {
		merged.ClusterName = incoming.ClusterName
	}
	if incoming.ClusterHAEnabled != nil {
		merged.ClusterHAEnabled = cloneBoolPtr(incoming.ClusterHAEnabled)
	}
	if incoming.ClusterDRSEnabled != nil {
		merged.ClusterDRSEnabled = cloneBoolPtr(incoming.ClusterDRSEnabled)
	}
	if incoming.FolderID != "" {
		merged.FolderID = incoming.FolderID
	}
	if incoming.FolderName != "" {
		merged.FolderName = incoming.FolderName
	}
	if incoming.ResourcePoolID != "" {
		merged.ResourcePoolID = incoming.ResourcePoolID
	}
	if incoming.ResourcePoolName != "" {
		merged.ResourcePoolName = incoming.ResourcePoolName
	}
	if incoming.RuntimeHostID != "" {
		merged.RuntimeHostID = incoming.RuntimeHostID
	}
	if incoming.RuntimeHostName != "" {
		merged.RuntimeHostName = incoming.RuntimeHostName
	}
	if incoming.ConnectionState != "" {
		merged.ConnectionState = incoming.ConnectionState
	}
	if incoming.PowerState != "" {
		merged.PowerState = incoming.PowerState
	}
	if incoming.OverallStatus != "" {
		merged.OverallStatus = incoming.OverallStatus
	}
	if incoming.CPUCount > 0 {
		merged.CPUCount = incoming.CPUCount
	}
	if incoming.MemorySizeMiB > 0 {
		merged.MemorySizeMiB = incoming.MemorySizeMiB
	}
	if incoming.DatastoreType != "" {
		merged.DatastoreType = incoming.DatastoreType
	}
	if len(incoming.DatastoreIDs) > 0 {
		merged.DatastoreIDs = uniqueStrings(append(cloneStringSlice(merged.DatastoreIDs), incoming.DatastoreIDs...))
	}
	if len(incoming.DatastoreNames) > 0 {
		merged.DatastoreNames = uniqueStrings(append(cloneStringSlice(merged.DatastoreNames), incoming.DatastoreNames...))
	}
	if incoming.DatastoreURL != "" {
		merged.DatastoreURL = incoming.DatastoreURL
	}
	if incoming.DatastoreAccessible != nil {
		merged.DatastoreAccessible = cloneBoolPtr(incoming.DatastoreAccessible)
	}
	if incoming.MultipleHostAccess != nil {
		merged.MultipleHostAccess = cloneBoolPtr(incoming.MultipleHostAccess)
	}
	if incoming.MaintenanceMode != "" {
		merged.MaintenanceMode = incoming.MaintenanceMode
	}
	if incoming.NetworkType != "" {
		merged.NetworkType = incoming.NetworkType
	}
	if len(incoming.NetworkHostIDs) > 0 {
		merged.NetworkHostIDs = uniqueStrings(append(cloneStringSlice(merged.NetworkHostIDs), incoming.NetworkHostIDs...))
	}
	if len(incoming.NetworkHostNames) > 0 {
		merged.NetworkHostNames = uniqueStrings(append(cloneStringSlice(merged.NetworkHostNames), incoming.NetworkHostNames...))
	}
	if len(incoming.NetworkVMIDs) > 0 {
		merged.NetworkVMIDs = uniqueStrings(append(cloneStringSlice(merged.NetworkVMIDs), incoming.NetworkVMIDs...))
	}
	if len(incoming.NetworkVMNames) > 0 {
		merged.NetworkVMNames = uniqueStrings(append(cloneStringSlice(merged.NetworkVMNames), incoming.NetworkVMNames...))
	}
	if incoming.InstanceUUID != "" {
		merged.InstanceUUID = incoming.InstanceUUID
	}
	if incoming.BIOSUUID != "" {
		merged.BIOSUUID = incoming.BIOSUUID
	}
	if incoming.GuestOSFamily != "" {
		merged.GuestOSFamily = incoming.GuestOSFamily
	}
	if incoming.GuestHostname != "" {
		merged.GuestHostname = incoming.GuestHostname
	}
	if len(incoming.GuestIPAddresses) > 0 {
		merged.GuestIPAddresses = uniqueStrings(append(cloneStringSlice(merged.GuestIPAddresses), incoming.GuestIPAddresses...))
	}
	if incoming.ActiveAlarmCount > 0 {
		merged.ActiveAlarmCount = incoming.ActiveAlarmCount
	}
	if incoming.ActiveAlarmSummary != "" {
		merged.ActiveAlarmSummary = incoming.ActiveAlarmSummary
	}
	if incoming.RecentTaskCount > 0 {
		merged.RecentTaskCount = incoming.RecentTaskCount
	}
	if incoming.RecentTaskSummary != "" {
		merged.RecentTaskSummary = incoming.RecentTaskSummary
	}
	if incoming.SnapshotCount > 0 {
		merged.SnapshotCount = incoming.SnapshotCount
	}
	if incoming.CurrentSnapshotID != "" {
		merged.CurrentSnapshotID = incoming.CurrentSnapshotID
	}
	if len(incoming.SnapshotTree) > 0 {
		merged.SnapshotTree = cloneVMwareSnapshotDataSlice(incoming.SnapshotTree)
	}
	if len(incoming.NetworkAdapters) > 0 {
		merged.NetworkAdapters = cloneVMwareNetworkAdapterDataSlice(incoming.NetworkAdapters)
	}
	if len(incoming.VirtualDisks) > 0 {
		merged.VirtualDisks = cloneVMwareVirtualDiskDataSlice(incoming.VirtualDisks)
	}
	if incoming.Tools != nil {
		merged.Tools = cloneVMwareToolsData(incoming.Tools)
	}
	if incoming.Hardware != nil {
		merged.Hardware = cloneVMwareVMHardwareData(incoming.Hardware)
	}

	return &merged
}

func (rr *ResourceRegistry) applyManualLinks() {
	if len(rr.links) == 0 {
		return
	}
	for _, link := range rr.links {
		primaryID := link.PrimaryID
		if primaryID == "" {
			primaryID = link.ResourceA
		}
		primary := rr.resources[primaryID]
		if primary == nil {
			primary = rr.resources[link.ResourceA]
			primaryID = link.ResourceA
		}
		if primary == nil {
			primary = rr.resources[link.ResourceB]
			primaryID = link.ResourceB
		}
		if primary == nil {
			continue
		}

		otherID := link.ResourceB
		if otherID == primaryID {
			otherID = link.ResourceA
		}
		other := rr.resources[otherID]
		if other == nil || otherID == primaryID {
			continue
		}

		rr.mergeResourceData(primary, other)
		delete(rr.resources, otherID)
		rr.updateSourceMappings(otherID, primaryID)
	}
}

func (rr *ResourceRegistry) mergeResourceData(primary *Resource, other *Resource) {
	if other == nil || primary == nil {
		return
	}
	if other.parentBySource != nil {
		if primary.parentBySource == nil {
			primary.parentBySource = make(map[DataSource]string, len(other.parentBySource))
		}
		for source, parentID := range other.parentBySource {
			primary.parentBySource[source] = parentID
		}
	}
	primary.Identity = mergeIdentity(primary.Identity, other.Identity)
	primary.Tags = uniqueStrings(append(primary.Tags, other.Tags...))
	primary.Incidents = mergeResourceIncidents(primary.Incidents, other.Incidents)
	primary.Sources = addSources(primary.Sources, other.Sources)
	if primary.SourceStatus == nil {
		primary.SourceStatus = make(map[DataSource]SourceStatus)
	}
	for source, status := range other.SourceStatus {
		primary.SourceStatus[source] = status
	}
	if other.LastSeen.After(primary.LastSeen) {
		primary.LastSeen = other.LastSeen
	}
	primary.UpdatedAt = time.Now().UTC()
	if primary.ParentID == nil && other.ParentID != nil {
		primary.ParentID = cloneStringPtr(other.ParentID)
	}
	primary.ParentID = rr.resolveCanonicalParentID(primary)

	if primary.Proxmox == nil {
		primary.Proxmox = other.Proxmox
	}
	if primary.Agent == nil {
		primary.Agent = other.Agent
	}
	if primary.Docker == nil {
		primary.Docker = other.Docker
	}
	if primary.PBS == nil {
		primary.PBS = other.PBS
	}
	if primary.PMG == nil {
		primary.PMG = other.PMG
	}
	if primary.Kubernetes == nil {
		primary.Kubernetes = other.Kubernetes
	}
	if primary.PhysicalDisk == nil {
		primary.PhysicalDisk = other.PhysicalDisk
	}
	if primary.Ceph == nil {
		primary.Ceph = other.Ceph
	}

	primary.Metrics = mergeMetrics(primary.Metrics, other.Metrics, SourceAgent, time.Now().UTC(), primary.SourceStatus)
	primary.Status = aggregateStatus(primary)
}

func (rr *ResourceRegistry) updateSourceMappings(fromID, toID string) {
	fromID = CanonicalResourceID(strings.TrimSpace(fromID))
	toID = CanonicalResourceID(strings.TrimSpace(toID))
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	for source, mapping := range rr.bySource {
		for key, value := range mapping {
			if value == fromID {
				mapping[key] = toID
			}
		}
		rr.bySource[source] = mapping
	}
	rr.updateParentReferencesLocked(fromID, toID)
}

func (rr *ResourceRegistry) updateParentReferencesLocked(fromID, toID string) {
	for _, resource := range rr.resources {
		if resource == nil {
			continue
		}
		if resource.ParentID != nil && CanonicalResourceID(strings.TrimSpace(*resource.ParentID)) == fromID {
			resource.ParentID = &toID
		}
		for source, parentID := range resource.parentBySource {
			if CanonicalResourceID(strings.TrimSpace(parentID)) == fromID {
				resource.parentBySource[source] = toID
			}
		}
	}
}

func (rr *ResourceRegistry) setSourceParent(resource *Resource, source DataSource, parentID *string) {
	if resource == nil {
		return
	}
	if resource.parentBySource == nil {
		resource.parentBySource = make(map[DataSource]string)
	}
	if parentID == nil {
		delete(resource.parentBySource, source)
		return
	}
	canonicalParentID := CanonicalResourceID(strings.TrimSpace(*parentID))
	if canonicalParentID == "" {
		delete(resource.parentBySource, source)
		return
	}
	resource.parentBySource[source] = canonicalParentID
}

func (rr *ResourceRegistry) resolveCanonicalParentID(resource *Resource) *string {
	if resource == nil {
		return nil
	}

	if resource.parentBySource == nil {
		if resource.ParentID != nil {
			canonicalParentID := CanonicalResourceID(strings.TrimSpace(*resource.ParentID))
			if canonicalParentID != "" {
				if _, ok := rr.resources[canonicalParentID]; ok {
					return &canonicalParentID
				}
			}
		}
		return rr.resolveDerivedParentIDLocked(resource)
	}

	bestPriority := -1
	bestParentID := ""
	for source, parentID := range resource.parentBySource {
		parentID = CanonicalResourceID(strings.TrimSpace(parentID))
		if parentID == "" {
			continue
		}
		if _, ok := rr.resources[parentID]; !ok {
			continue
		}
		priority := sourcePriority(source)
		if priority > bestPriority {
			bestPriority = priority
			bestParentID = parentID
		}
	}
	if bestParentID == "" {
		return rr.resolveDerivedParentIDLocked(resource)
	}
	if derivedParentID := rr.resolveDerivedParentIDLocked(resource); derivedParentID != nil {
		derivedID := CanonicalResourceID(strings.TrimSpace(*derivedParentID))
		if rr.shouldPreferDerivedProxmoxParentLocked(resource, bestParentID, derivedID) {
			return &derivedID
		}
	}
	return &bestParentID
}

func (rr *ResourceRegistry) shouldPreferDerivedProxmoxParentLocked(resource *Resource, currentParentID, derivedParentID string) bool {
	if resource == nil || resource.Proxmox == nil {
		return false
	}
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeVM, ResourceTypeSystemContainer, ResourceTypeStorage, ResourceTypePhysicalDisk:
	default:
		return false
	}

	currentParentID = CanonicalResourceID(strings.TrimSpace(currentParentID))
	derivedParentID = CanonicalResourceID(strings.TrimSpace(derivedParentID))
	if currentParentID == "" || derivedParentID == "" || currentParentID == derivedParentID {
		return false
	}

	derivedParent := rr.resources[derivedParentID]
	if derivedParent == nil {
		return false
	}
	currentParent := rr.resources[currentParentID]
	if currentParent == nil {
		return true
	}
	return proxmoxNodeParentAgentScore(derivedParent) > proxmoxNodeParentAgentScore(currentParent)
}

func (rr *ResourceRegistry) resolveDerivedParentIDLocked(resource *Resource) *string {
	if resource == nil || resource.Proxmox == nil {
		return nil
	}
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeVM, ResourceTypeSystemContainer, ResourceTypeStorage, ResourceTypePhysicalDisk:
	default:
		return nil
	}

	parentID := rr.proxmoxNodeParentIDLocked(
		resource.Proxmox.Instance,
		resource.Proxmox.ClusterName,
		resource.Proxmox.NodeName,
		resource.ID,
	)
	if parentID == "" {
		return nil
	}
	return &parentID
}

func (rr *ResourceRegistry) proxmoxNodeParentID(instance, clusterName, nodeName, excludeID string) string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return rr.proxmoxNodeParentIDLocked(instance, clusterName, nodeName, excludeID)
}

func (rr *ResourceRegistry) proxmoxNodeParentIDLocked(instance, clusterName, nodeName, excludeID string) string {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return ""
	}

	mapping := rr.bySource[SourceProxmox]
	if len(mapping) == 0 {
		return ""
	}

	excludeID = CanonicalResourceID(strings.TrimSpace(excludeID))
	fallbackParentID := ""
	for _, sourceID := range proxmoxNodeParentSourceIDCandidates(instance, clusterName, nodeName) {
		parentID := CanonicalResourceID(mapping[normalizeSourceID(sourceID)])
		if parentID == "" || parentID == excludeID {
			continue
		}
		parent := rr.resources[parentID]
		if parent == nil || CanonicalResourceType(parent.Type) != ResourceTypeAgent {
			continue
		}
		if proxmoxNodeParentAgentScore(parent) > 0 {
			return parentID
		}
		if fallbackParentID == "" {
			fallbackParentID = parentID
		}
	}
	if parentID := rr.proxmoxNodeParentIDFromResourcesLocked(instance, clusterName, nodeName, excludeID); parentID != "" {
		return parentID
	}
	return fallbackParentID
}

func proxmoxNodeParentSourceIDCandidates(instance, clusterName, nodeName string) []string {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return nil
	}

	candidates := []string{
		proxmoxNodeSourceID(strings.TrimSpace(instance), nodeName),
		proxmoxNodeSourceID(strings.TrimSpace(clusterName), nodeName),
		nodeName,
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizeSourceID(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func (rr *ResourceRegistry) proxmoxNodeParentIDFromResourcesLocked(instance, clusterName, nodeName, excludeID string) string {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return ""
	}

	excludeID = CanonicalResourceID(strings.TrimSpace(excludeID))
	bestID := ""
	bestScore := -1
	for id, resource := range rr.resources {
		if resource == nil {
			continue
		}
		candidateID := CanonicalResourceID(strings.TrimSpace(id))
		if candidateID == "" || candidateID == excludeID {
			continue
		}
		if CanonicalResourceType(resource.Type) != ResourceTypeAgent || resource.Proxmox == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(resource.Proxmox.NodeName), nodeName) {
			continue
		}
		score := proxmoxNodeParentScopeScore(instance, clusterName, resource.Proxmox)
		if score < 0 {
			continue
		}
		score += proxmoxNodeParentAgentScore(resource)
		if score > bestScore || (score == bestScore && (bestID == "" || candidateID < bestID)) {
			bestID = candidateID
			bestScore = score
		}
	}
	return bestID
}

func proxmoxNodeParentAgentScore(resource *Resource) int {
	if resource == nil {
		return 0
	}
	score := 0
	if resource.Agent != nil || hasDataSource(resource.Sources, SourceAgent) {
		score += 1000
	}
	if resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.LinkedAgentID) != "" {
		score += 500
	}
	return score
}

func proxmoxNodeParentScopeScore(instance, clusterName string, parent *ProxmoxData) int {
	if parent == nil {
		return -1
	}

	childInstance := strings.TrimSpace(instance)
	childCluster := strings.TrimSpace(clusterName)
	parentInstance := strings.TrimSpace(parent.Instance)
	parentCluster := strings.TrimSpace(parent.ClusterName)

	switch {
	case childInstance != "" && parentInstance != "" && strings.EqualFold(childInstance, parentInstance):
		return 40
	case childCluster != "" && parentCluster != "" && strings.EqualFold(childCluster, parentCluster):
		return 35
	case childInstance != "" && parentCluster != "" && strings.EqualFold(childInstance, parentCluster):
		return 30
	case childCluster != "" && parentInstance != "" && strings.EqualFold(childCluster, parentInstance):
		return 30
	case childInstance == "" && childCluster == "":
		return 10
	case parentInstance == "" && parentCluster == "":
		return 5
	default:
		return -1
	}
}

func (rr *ResourceRegistry) buildChildCounts() {
	// ChildCount and ParentName are derived fields. Clear prior values before
	// recomputing to prevent stale state after re-parenting or parent removal.
	for _, r := range rr.resources {
		r.ParentID = rr.resolveCanonicalParentID(r)
		rr.refreshLinkedAgentIDFromParentLocked(r)
		r.ChildCount = 0
		r.ParentName = ""
	}

	childCounts := make(map[string]int)
	for _, r := range rr.resources {
		if r.ParentID != nil {
			childCounts[*r.ParentID]++
		}
	}
	for id, count := range childCounts {
		if r, ok := rr.resources[id]; ok {
			r.ChildCount = count
		}
	}
	// Resolve parent names for child resources.
	for _, r := range rr.resources {
		if r.ParentID != nil {
			if parent, ok := rr.resources[*r.ParentID]; ok {
				r.ParentName = parent.Name
			}
		}
	}
}

func (rr *ResourceRegistry) refreshLinkedAgentIDFromParentLocked(resource *Resource) {
	if resource == nil || resource.Proxmox == nil || resource.ParentID == nil {
		return
	}
	switch CanonicalResourceType(resource.Type) {
	case ResourceTypeVM, ResourceTypeSystemContainer:
	default:
		return
	}
	parentID := CanonicalResourceID(strings.TrimSpace(*resource.ParentID))
	if parentID == "" {
		return
	}
	attachLinkedAgentID(resource.Proxmox, linkedAgentIDFromResource(rr.resources[parentID]))
}

func linkedAgentIDFromResource(resource *Resource) string {
	if resource == nil {
		return ""
	}
	if resource.Proxmox != nil {
		if linkedAgentID := strings.TrimSpace(resource.Proxmox.LinkedAgentID); linkedAgentID != "" {
			return linkedAgentID
		}
	}
	if resource.Agent != nil {
		return strings.TrimSpace(resource.Agent.AgentID)
	}
	return ""
}

func (rr *ResourceRegistry) chooseNewID(resourceType ResourceType, identity ResourceIdentity, source DataSource, sourceID string) string {
	switch resourceType {
	case ResourceTypeAgent:
		if identity.MachineID != "" || identity.DMIUUID != "" || identity.ClusterName != "" {
			return rr.canonicalIDFromIdentity(resourceType, identity)
		}
	case ResourceTypePhysicalDisk:
		if identity.MachineID != "" || identity.DMIUUID != "" {
			return rr.canonicalIDFromIdentity(resourceType, identity)
		}
	}
	return rr.sourceSpecificID(resourceType, source, sourceID)
}

func (rr *ResourceRegistry) sourceResourceID(source DataSource, sourceID string) string {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	mapping, ok := rr.bySource[source]
	if !ok {
		return ""
	}
	return mapping[normalizeSourceID(sourceID)]
}

func (rr *ResourceRegistry) canonicalIDFromIdentity(resourceType ResourceType, identity ResourceIdentity) string {
	var stable string
	switch {
	case identity.MachineID != "":
		stable = "machine:" + strings.TrimSpace(identity.MachineID)
	case identity.DMIUUID != "":
		stable = "dmi:" + strings.TrimSpace(identity.DMIUUID)
	case identity.ClusterName != "" && len(identity.Hostnames) > 0:
		stable = fmt.Sprintf("cluster:%s:%s", identity.ClusterName, NormalizeHostname(identity.Hostnames[0]))
	case len(identity.Hostnames) > 0:
		stable = "hostname:" + NormalizeHostname(identity.Hostnames[0])
	case len(identity.IPAddresses) > 0:
		stable = "ip:" + NormalizeIP(identity.IPAddresses[0])
	default:
		stable = fmt.Sprintf("unknown:%d", time.Now().UnixNano())
	}
	return buildHashID(resourceType, stable)
}

func (rr *ResourceRegistry) sourceSpecificID(resourceType ResourceType, source DataSource, sourceID string) string {
	stable := fmt.Sprintf("%s:%s", source, normalizeSourceID(sourceID))
	return buildHashID(resourceType, stable)
}

func buildHashID(resourceType ResourceType, stable string) string {
	resourceType = CanonicalResourceType(resourceType)
	hash := sha256.Sum256([]byte(stable))
	return fmt.Sprintf("%s-%s", resourceType, hex.EncodeToString(hash[:8]))
}

func proxmoxNodeSourceID(instance, nodeName string) string {
	if instance == "" {
		return nodeName
	}
	return fmt.Sprintf("%s-%s", instance, nodeName)
}

func proxmoxVMSourceID(vm models.VM) string {
	if sourceID := strings.TrimSpace(vm.ID); sourceID != "" {
		return sourceID
	}
	if vm.VMID > 0 {
		return proxmoxGuestFallbackSourceID("vm", vm.Instance, vm.Node, vm.VMID)
	}
	return strings.TrimSpace(vm.Name)
}

func proxmoxContainerSourceID(ct models.Container) string {
	if sourceID := strings.TrimSpace(ct.ID); sourceID != "" {
		return sourceID
	}
	if ct.VMID > 0 {
		return proxmoxGuestFallbackSourceID("ct", ct.Instance, ct.Node, ct.VMID)
	}
	return strings.TrimSpace(ct.Name)
}

func proxmoxGuestFallbackSourceID(kind, instance, node string, vmid int) string {
	parts := make([]string, 0, 4)
	if normalizedKind := strings.TrimSpace(kind); normalizedKind != "" {
		parts = append(parts, normalizedKind)
	}
	if normalizedInstance := strings.TrimSpace(instance); normalizedInstance != "" {
		parts = append(parts, normalizedInstance)
	}
	if normalizedNode := strings.TrimSpace(node); normalizedNode != "" {
		parts = append(parts, normalizedNode)
	}
	parts = append(parts, fmt.Sprintf("%d", vmid))
	return strings.Join(parts, ":")
}

func kubernetesClusterSourceID(cluster models.KubernetesCluster) string {
	return CanonicalKubernetesClusterSourceID(cluster)
}

func kubernetesNodeSourceID(clusterSourceID string, node models.KubernetesNode) string {
	return CanonicalKubernetesNodeSourceID(clusterSourceID, node)
}

func kubernetesPodSourceID(clusterSourceID string, pod models.KubernetesPod) string {
	return CanonicalKubernetesPodSourceID(clusterSourceID, pod)
}

func kubernetesDeploymentSourceID(clusterSourceID string, deployment models.KubernetesDeployment) string {
	return CanonicalKubernetesDeploymentSourceID(clusterSourceID, deployment)
}

func kubernetesReplicaSetSourceID(clusterSourceID string, replicaSet models.KubernetesReplicaSet) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "replicaset", replicaSet.UID, replicaSet.Namespace, replicaSet.Name)
}

func kubernetesNamespaceSourceID(clusterSourceID string, namespace models.KubernetesNamespace) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "namespace", namespace.UID, "", namespace.Name)
}

func kubernetesServiceSourceID(clusterSourceID string, service models.KubernetesService) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "service", service.UID, service.Namespace, service.Name)
}

func kubernetesStatefulSetSourceID(clusterSourceID string, statefulSet models.KubernetesStatefulSet) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "statefulset", statefulSet.UID, statefulSet.Namespace, statefulSet.Name)
}

func kubernetesDaemonSetSourceID(clusterSourceID string, daemonSet models.KubernetesDaemonSet) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "daemonset", daemonSet.UID, daemonSet.Namespace, daemonSet.Name)
}

func kubernetesJobSourceID(clusterSourceID string, job models.KubernetesJob) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "job", job.UID, job.Namespace, job.Name)
}

func kubernetesCronJobSourceID(clusterSourceID string, cronJob models.KubernetesCronJob) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "cronjob", cronJob.UID, cronJob.Namespace, cronJob.Name)
}

func kubernetesIngressSourceID(clusterSourceID string, ingress models.KubernetesIngress) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "ingress", ingress.UID, ingress.Namespace, ingress.Name)
}

func kubernetesEndpointSliceSourceID(clusterSourceID string, slice models.KubernetesEndpointSlice) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "endpointslice", slice.UID, slice.Namespace, slice.Name)
}

func kubernetesNetworkPolicySourceID(clusterSourceID string, policy models.KubernetesNetworkPolicy) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "networkpolicy", policy.UID, policy.Namespace, policy.Name)
}

func kubernetesPersistentVolumeSourceID(clusterSourceID string, volume models.KubernetesPersistentVolume) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "persistentvolume", volume.UID, "", volume.Name)
}

func kubernetesPersistentVolumeClaimSourceID(clusterSourceID string, claim models.KubernetesPersistentVolumeClaim) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "persistentvolumeclaim", claim.UID, claim.Namespace, claim.Name)
}

func kubernetesStorageClassSourceID(clusterSourceID string, class models.KubernetesStorageClass) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "storageclass", class.UID, "", class.Name)
}

func kubernetesConfigMapSourceID(clusterSourceID string, configMap models.KubernetesConfigMap) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "configmap", configMap.UID, configMap.Namespace, configMap.Name)
}

func kubernetesSecretSourceID(clusterSourceID string, secret models.KubernetesSecret) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "secret", secret.UID, secret.Namespace, secret.Name)
}

func kubernetesServiceAccountSourceID(clusterSourceID string, account models.KubernetesServiceAccount) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "serviceaccount", account.UID, account.Namespace, account.Name)
}

func kubernetesRoleSourceID(clusterSourceID string, role models.KubernetesRole) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "role", role.UID, role.Namespace, role.Name)
}

func kubernetesClusterRoleSourceID(clusterSourceID string, role models.KubernetesClusterRole) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "clusterrole", role.UID, "", role.Name)
}

func kubernetesRoleBindingSourceID(clusterSourceID string, binding models.KubernetesRoleBinding) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "rolebinding", binding.UID, binding.Namespace, binding.Name)
}

func kubernetesClusterRoleBindingSourceID(clusterSourceID string, binding models.KubernetesClusterRoleBinding) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "clusterrolebinding", binding.UID, "", binding.Name)
}

func kubernetesResourceQuotaSourceID(clusterSourceID string, quota models.KubernetesResourceQuota) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "resourcequota", quota.UID, quota.Namespace, quota.Name)
}

func kubernetesLimitRangeSourceID(clusterSourceID string, limitRange models.KubernetesLimitRange) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "limitrange", limitRange.UID, limitRange.Namespace, limitRange.Name)
}

func kubernetesPodDisruptionBudgetSourceID(clusterSourceID string, budget models.KubernetesPodDisruptionBudget) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "poddisruptionbudget", budget.UID, budget.Namespace, budget.Name)
}

func kubernetesHorizontalPodAutoscalerSourceID(clusterSourceID string, autoscaler models.KubernetesHorizontalPodAutoscaler) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "horizontalpodautoscaler", autoscaler.UID, autoscaler.Namespace, autoscaler.Name)
}

func kubernetesEventSourceID(clusterSourceID string, event models.KubernetesEvent) string {
	return canonicalKubernetesTypedSourceID(clusterSourceID, "event", event.UID, event.Namespace, event.Name)
}

func canonicalKubernetesTypedSourceID(clusterSourceID, kind, uid, namespace, name string) string {
	clusterKey := strings.TrimSpace(clusterSourceID)
	kind = strings.TrimSpace(kind)
	id := strings.TrimSpace(uid)
	if id == "" {
		namespace = strings.TrimSpace(namespace)
		name = strings.TrimSpace(name)
		if namespace != "" && name != "" {
			id = namespace + "/" + name
		} else {
			id = name
		}
	}
	if clusterKey == "" || kind == "" || id == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", clusterKey, kind, id)
}

func buildKubernetesNodeHostLookup(hosts []models.Host) map[string]*models.Host {
	lookup := make(map[string]*models.Host, len(hosts)*2)
	for i := range hosts {
		host := &hosts[i]
		if host == nil {
			continue
		}

		exactKey := strings.ToLower(strings.TrimSpace(host.Hostname))
		if exactKey != "" {
			lookup["agent:"+exactKey] = host
		}

		normalized := NormalizeHostname(host.Hostname)
		if normalized != "" {
			if _, exists := lookup["short:"+normalized]; !exists {
				lookup["short:"+normalized] = host
			}
		}
	}
	return lookup
}

func resolveKubernetesNodeHost(node models.KubernetesNode, lookup map[string]*models.Host) *models.Host {
	if len(lookup) == 0 {
		return nil
	}

	exactKey := strings.ToLower(strings.TrimSpace(node.Name))
	if exactKey != "" {
		if host, ok := lookup["agent:"+exactKey]; ok {
			return host
		}
	}

	normalized := NormalizeHostname(node.Name)
	if normalized != "" {
		if host, ok := lookup["short:"+normalized]; ok {
			return host
		}
	}

	return nil
}

func linkedHostsForKubernetesCluster(cluster models.KubernetesCluster, lookup map[string]*models.Host) []*models.Host {
	if len(cluster.Nodes) == 0 || len(lookup) == 0 {
		return nil
	}

	hosts := make([]*models.Host, 0, len(cluster.Nodes))
	seen := make(map[string]struct{}, len(cluster.Nodes))
	for _, node := range cluster.Nodes {
		host := resolveKubernetesNodeHost(node, lookup)
		if host == nil {
			continue
		}
		hostID := strings.TrimSpace(host.ID)
		if hostID == "" {
			hostID = strings.ToLower(strings.TrimSpace(host.Hostname))
		}
		if hostID == "" {
			continue
		}
		if _, exists := seen[hostID]; exists {
			continue
		}
		seen[hostID] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts
}

func pbsInstanceSourceID(instance models.PBSInstance) string {
	if v := strings.TrimSpace(instance.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(instance.Name); v != "" {
		return v
	}
	return extractHostname(instance.Host)
}

func pbsDatastoreSourceID(instance models.PBSInstance, datastore models.PBSDatastore) string {
	instanceID := pbsInstanceSourceID(instance)
	datastoreName := strings.TrimSpace(datastore.Name)
	if instanceID == "" {
		return datastoreName
	}
	if datastoreName == "" {
		return instanceID
	}
	return instanceID + "/" + datastoreName
}

func pmgInstanceSourceID(instance models.PMGInstance) string {
	if v := strings.TrimSpace(instance.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(instance.Name); v != "" {
		return v
	}
	return extractHostname(instance.Host)
}

func clonePBSBackups(in []models.PBSBackup) []models.PBSBackup {
	if len(in) == 0 {
		return nil
	}
	out := make([]models.PBSBackup, len(in))
	copy(out, in)
	return out
}

func dockerSwarmClusterKey(host models.DockerHost) string {
	if host.Swarm == nil {
		return ""
	}
	if v := strings.TrimSpace(host.Swarm.ClusterID); v != "" {
		return v
	}
	if v := strings.TrimSpace(host.Swarm.ClusterName); v != "" {
		return v
	}
	return ""
}

func dockerServiceSourceID(host models.DockerHost, service models.DockerService) string {
	cluster := dockerSwarmClusterKey(host)
	if cluster == "" {
		return ""
	}
	serviceID := strings.TrimSpace(service.ID)
	if serviceID == "" {
		serviceID = strings.TrimSpace(service.Name)
	}
	if serviceID == "" {
		return ""
	}
	return fmt.Sprintf("%s:service:%s", cluster, serviceID)
}

func dockerImageSourceID(host models.DockerHost, image models.DockerImage) string {
	hostID := strings.TrimSpace(host.ID)
	imageID := strings.TrimSpace(image.ID)
	if imageID == "" {
		imageID = firstDockerImageReference(image)
	}
	if hostID == "" || imageID == "" {
		return ""
	}
	return hostID + "/image/" + imageID
}

func dockerVolumeSourceID(host models.DockerHost, volume models.DockerVolume) string {
	hostID := strings.TrimSpace(host.ID)
	name := strings.TrimSpace(volume.Name)
	if hostID == "" || name == "" {
		return ""
	}
	return hostID + "/volume/" + name
}

func dockerNetworkSourceID(host models.DockerHost, network models.DockerNetwork) string {
	hostID := strings.TrimSpace(host.ID)
	networkID := strings.TrimSpace(network.ID)
	if networkID == "" {
		networkID = strings.TrimSpace(network.Name)
	}
	if hostID == "" || networkID == "" {
		return ""
	}
	return hostID + "/network/" + networkID
}

func dockerTaskSourceID(host models.DockerHost, task models.DockerTask) string {
	taskID := strings.TrimSpace(task.ID)
	if taskID == "" {
		taskID = strings.TrimSpace(task.ContainerID)
	}
	if taskID == "" {
		taskID = strings.TrimSpace(task.ServiceName)
		if task.Slot > 0 && taskID != "" {
			taskID = fmt.Sprintf("%s.%d", taskID, task.Slot)
		}
	}
	if taskID == "" {
		return ""
	}
	if cluster := dockerSwarmClusterKey(host); cluster != "" {
		return fmt.Sprintf("%s:task:%s", cluster, taskID)
	}
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return ""
	}
	return hostID + "/task/" + taskID
}

func dockerSwarmNodeSourceID(host models.DockerHost, node models.DockerNode) string {
	nodeID := strings.TrimSpace(node.ID)
	if nodeID == "" {
		nodeID = strings.TrimSpace(node.Hostname)
	}
	if nodeID == "" {
		return ""
	}
	if cluster := dockerSwarmClusterKey(host); cluster != "" {
		return fmt.Sprintf("%s:node:%s", cluster, nodeID)
	}
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return ""
	}
	return hostID + "/swarm-node/" + nodeID
}

func dockerSecretSourceID(host models.DockerHost, secret models.DockerSecret) string {
	secretID := strings.TrimSpace(secret.ID)
	if secretID == "" {
		secretID = strings.TrimSpace(secret.Name)
	}
	if secretID == "" {
		return ""
	}
	if cluster := dockerSwarmClusterKey(host); cluster != "" {
		return fmt.Sprintf("%s:secret:%s", cluster, secretID)
	}
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return ""
	}
	return hostID + "/secret/" + secretID
}

func dockerConfigSourceID(host models.DockerHost, config models.DockerConfig) string {
	configID := strings.TrimSpace(config.ID)
	if configID == "" {
		configID = strings.TrimSpace(config.Name)
	}
	if configID == "" {
		return ""
	}
	if cluster := dockerSwarmClusterKey(host); cluster != "" {
		return fmt.Sprintf("%s:config:%s", cluster, configID)
	}
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return ""
	}
	return hostID + "/config/" + configID
}

func mergeIdentity(existing ResourceIdentity, incoming ResourceIdentity) ResourceIdentity {
	if existing.MachineID == "" {
		existing.MachineID = incoming.MachineID
	}
	if existing.DMIUUID == "" {
		existing.DMIUUID = incoming.DMIUUID
	}
	if existing.ClusterName == "" {
		existing.ClusterName = incoming.ClusterName
	}
	existing.Hostnames = uniqueStrings(append(existing.Hostnames, incoming.Hostnames...))
	existing.IPAddresses = uniqueStrings(append(existing.IPAddresses, incoming.IPAddresses...))
	existing.MACAddresses = uniqueStrings(append(existing.MACAddresses, incoming.MACAddresses...))
	return existing
}

func addSource(sources []DataSource, source DataSource) []DataSource {
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func hasDataSource(sources []DataSource, source DataSource) bool {
	for _, existing := range sources {
		if existing == source {
			return true
		}
	}
	return false
}

func addSources(sources []DataSource, more []DataSource) []DataSource {
	out := sources
	for _, source := range more {
		out = addSource(out, source)
	}
	return out
}

func mergeMetrics(existing *ResourceMetrics, incoming *ResourceMetrics, source DataSource, now time.Time, status map[DataSource]SourceStatus) *ResourceMetrics {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}
	merged := *existing
	merged.CPU = mergeMetric(existing.CPU, incoming.CPU, source, now, status)
	merged.Memory = mergeMetric(existing.Memory, incoming.Memory, source, now, status)
	merged.Disk = mergeMetric(existing.Disk, incoming.Disk, source, now, status)
	merged.NetIn = mergeMetric(existing.NetIn, incoming.NetIn, source, now, status)
	merged.NetOut = mergeMetric(existing.NetOut, incoming.NetOut, source, now, status)
	merged.DiskRead = mergeMetric(existing.DiskRead, incoming.DiskRead, source, now, status)
	merged.DiskWrite = mergeMetric(existing.DiskWrite, incoming.DiskWrite, source, now, status)
	return &merged
}

// metricSourceStale reports whether a source's most recent report is older than
// its stale threshold. A zero/unknown last-seen is treated as NOT stale so the
// merge never demotes a source on missing information.
func metricSourceStale(now time.Time, status map[DataSource]SourceStatus, source DataSource) bool {
	if status == nil {
		return false
	}
	st, ok := status[source]
	if !ok || st.LastSeen.IsZero() {
		return false
	}
	threshold, ok := defaultStaleThresholds[source]
	if !ok {
		threshold = 60 * time.Second
	}
	return now.Sub(st.LastSeen) > threshold
}

func mergeMetric(existing *MetricValue, incoming *MetricValue, source DataSource, now time.Time, status map[DataSource]SourceStatus) *MetricValue {
	if incoming == nil {
		return existing
	}
	incomingCopy := *incoming
	incomingCopy.Source = source
	if existing == nil {
		return &incomingCopy
	}
	// Freshness gates static source priority: a stale source must never hold a
	// metric against a live one, and a live source overrides a stale one
	// regardless of priority. Mirrors the presentation-coalesce rule so the
	// "live source wins a metric" invariant holds in both metric-merge paths.
	existingStale := metricSourceStale(now, status, existing.Source)
	incomingStale := metricSourceStale(now, status, source)
	if existingStale != incomingStale {
		if existingStale {
			return &incomingCopy
		}
		return existing
	}
	if sourcePriority(source) >= sourcePriority(existing.Source) {
		return &incomingCopy
	}
	return existing
}

func sourcePriority(source DataSource) int {
	switch source {
	case SourceAgent:
		return 3
	case SourceProxmox:
		return 2
	case SourceDocker:
		return 1
	case SourceVMware:
		return 2
	default:
		return 0
	}
}

func chooseStatus(existing ResourceStatus, incoming ResourceStatus, source DataSource) ResourceStatus {
	if existing == "" || existing == StatusUnknown {
		return incoming
	}
	if sourcePriority(source) >= sourcePriority(SourceAgent) {
		return incoming
	}
	return existing
}

func aggregateStatus(resource *Resource) ResourceStatus {
	statusPriority := map[string]int{
		"online":  3,
		"stale":   2,
		"offline": 1,
	}
	best := StatusUnknown
	bestScore := 0
	for _, status := range resource.SourceStatus {
		score := statusPriority[strings.ToLower(status.Status)]
		if score > bestScore {
			bestScore = score
			if status.Status == "online" {
				best = StatusOnline
			} else if status.Status == "stale" {
				best = StatusWarning
			} else if status.Status == "offline" {
				best = StatusOffline
			}
		}
	}
	if best == "" {
		return StatusUnknown
	}
	return best
}

func (rr *ResourceRegistry) isExcluded(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	_, ok := rr.exclusions[exclusionKey(a, b)]
	return ok
}

func exclusionKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// Stable ordering helper for deterministic output.
func sortResourcesByName(resources []Resource) {
	sort.SliceStable(resources, func(i, j int) bool {
		return CompareResourcesByCanonicalName(resources[i], resources[j]) < 0
	})
}

type namedResourceView interface {
	ID() string
	Name() string
}

func sortNamedResourceViewsByName[T namedResourceView](views []T) {
	sort.SliceStable(views, func(i, j int) bool {
		return compareResourceNameIdentity(views[i].Name(), "", views[i].ID(), views[j].Name(), "", views[j].ID()) < 0
	})
}

// ---------------------------------------------------------------------------
// ReadState implementation — typed, cached view accessors
// ---------------------------------------------------------------------------

// ensureViewsLocked rebuilds all cached view slices if dirty.
// Caller must hold rr.mu for writing.
func (rr *ResourceRegistry) ensureViewsLocked() {
	if !rr.viewsDirty {
		return
	}
	rr.rebuildViews()
}

// withViewCache acquires a write lock to ensure views are fresh, then
// downgrades to a read lock and calls fn. This avoids TOCTOU gaps.
func withViewCache[T any](rr *ResourceRegistry, fn func() T) T {
	rr.mu.Lock()
	rr.ensureViewsLocked()
	rr.mu.Unlock()
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return fn()
}

// rebuildViews recomputes all cached view slices from the current resource map.
// Caller must hold rr.mu for writing.
func (rr *ResourceRegistry) rebuildViews() {
	rr.cachedVMs = nil
	rr.cachedLXC = nil
	rr.cachedNodes = nil
	rr.cachedHosts = nil
	rr.cachedDocker = nil
	rr.cachedDockerContainers = nil
	rr.cachedStorage = nil
	rr.cachedPhysicalDisks = nil
	rr.cachedPBS = nil
	rr.cachedPMG = nil
	rr.cachedK8s = nil
	rr.cachedK8sNodes = nil
	rr.cachedPods = nil
	rr.cachedK8sDeployments = nil
	rr.cachedWorkload = nil
	rr.cachedInfra = nil

	for _, r := range rr.resources {
		viewResource := cloneResourcePtr(r)
		viewResource.MetricsTarget = rr.metricsTargetForResourceLocked(r.ID)
		switch r.Type {
		case ResourceTypeVM:
			v := NewVMView(viewResource)
			rr.cachedVMs = append(rr.cachedVMs, &v)
			w := NewWorkloadView(viewResource)
			rr.cachedWorkload = append(rr.cachedWorkload, &w)
		case ResourceTypeSystemContainer:
			v := NewContainerView(viewResource)
			rr.cachedLXC = append(rr.cachedLXC, &v)
			w := NewWorkloadView(viewResource)
			rr.cachedWorkload = append(rr.cachedWorkload, &w)
		case ResourceTypeAppContainer:
			v := NewDockerContainerView(viewResource)
			rr.cachedDockerContainers = append(rr.cachedDockerContainers, &v)
			w := NewWorkloadView(viewResource)
			rr.cachedWorkload = append(rr.cachedWorkload, &w)
		case ResourceTypeAgent:
			inf := NewInfrastructureView(viewResource)
			rr.cachedInfra = append(rr.cachedInfra, &inf)
			if r.Proxmox != nil {
				v := NewNodeView(viewResource)
				rr.cachedNodes = append(rr.cachedNodes, &v)
			}
			if r.Agent != nil || r.VMware != nil {
				v := NewHostView(viewResource)
				rr.cachedHosts = append(rr.cachedHosts, &v)
			}
			if r.Docker != nil {
				v := NewDockerHostView(viewResource)
				rr.cachedDocker = append(rr.cachedDocker, &v)
			}
		case ResourceTypeStorage:
			v := NewStoragePoolView(viewResource)
			rr.cachedStorage = append(rr.cachedStorage, &v)
		case ResourceTypePhysicalDisk:
			v := NewPhysicalDiskView(viewResource)
			rr.cachedPhysicalDisks = append(rr.cachedPhysicalDisks, &v)
		case ResourceTypePBS:
			v := NewPBSInstanceView(viewResource)
			rr.cachedPBS = append(rr.cachedPBS, &v)
		case ResourceTypePMG:
			v := NewPMGInstanceView(viewResource)
			rr.cachedPMG = append(rr.cachedPMG, &v)
		case ResourceTypeK8sCluster:
			v := NewK8sClusterView(viewResource)
			rr.cachedK8s = append(rr.cachedK8s, &v)
		case ResourceTypeK8sNode:
			v := NewK8sNodeView(viewResource)
			rr.cachedK8sNodes = append(rr.cachedK8sNodes, &v)
		case ResourceTypePod:
			v := NewPodView(viewResource)
			rr.cachedPods = append(rr.cachedPods, &v)
		case ResourceTypeK8sDeployment:
			v := NewK8sDeploymentView(viewResource)
			rr.cachedK8sDeployments = append(rr.cachedK8sDeployments, &v)
		}
	}

	sortNamedResourceViewsByName(rr.cachedVMs)
	sortNamedResourceViewsByName(rr.cachedLXC)
	sortNamedResourceViewsByName(rr.cachedNodes)
	sortNamedResourceViewsByName(rr.cachedHosts)
	sortNamedResourceViewsByName(rr.cachedDocker)
	sortNamedResourceViewsByName(rr.cachedDockerContainers)
	sortNamedResourceViewsByName(rr.cachedStorage)
	sortNamedResourceViewsByName(rr.cachedPhysicalDisks)
	sortNamedResourceViewsByName(rr.cachedPBS)
	sortNamedResourceViewsByName(rr.cachedPMG)
	sortNamedResourceViewsByName(rr.cachedK8s)
	sortNamedResourceViewsByName(rr.cachedK8sNodes)
	sortNamedResourceViewsByName(rr.cachedPods)
	sortNamedResourceViewsByName(rr.cachedK8sDeployments)
	sortNamedResourceViewsByName(rr.cachedWorkload)
	sortNamedResourceViewsByName(rr.cachedInfra)

	rr.viewsDirty = false
}

// VMs returns cached VM views sorted by name.
func (rr *ResourceRegistry) VMs() []*VMView {
	return withViewCache(rr, func() []*VMView { return rr.cachedVMs })
}

// Containers returns cached LXC container views sorted by name.
func (rr *ResourceRegistry) Containers() []*ContainerView {
	return withViewCache(rr, func() []*ContainerView { return rr.cachedLXC })
}

// Nodes returns cached Proxmox node views sorted by name.
// Only includes host resources that have Proxmox data.
func (rr *ResourceRegistry) Nodes() []*NodeView {
	return withViewCache(rr, func() []*NodeView { return rr.cachedNodes })
}

// Hosts returns cached host agent views sorted by name.
// Only includes host resources that have Agent data.
func (rr *ResourceRegistry) Hosts() []*HostView {
	return withViewCache(rr, func() []*HostView { return rr.cachedHosts })
}

// DockerHosts returns cached Docker host views sorted by name.
// Only includes host resources that have Docker data.
func (rr *ResourceRegistry) DockerHosts() []*DockerHostView {
	return withViewCache(rr, func() []*DockerHostView { return rr.cachedDocker })
}

func (rr *ResourceRegistry) DockerContainers() []*DockerContainerView {
	return withViewCache(rr, func() []*DockerContainerView { return rr.cachedDockerContainers })
}

// StoragePools returns cached storage pool views sorted by name.
func (rr *ResourceRegistry) StoragePools() []*StoragePoolView {
	return withViewCache(rr, func() []*StoragePoolView { return rr.cachedStorage })
}

// PhysicalDisks returns cached physical disk views sorted by name.
func (rr *ResourceRegistry) PhysicalDisks() []*PhysicalDiskView {
	return withViewCache(rr, func() []*PhysicalDiskView { return rr.cachedPhysicalDisks })
}

// PBSInstances returns cached PBS instance views sorted by name.
func (rr *ResourceRegistry) PBSInstances() []*PBSInstanceView {
	return withViewCache(rr, func() []*PBSInstanceView { return rr.cachedPBS })
}

// PMGInstances returns cached PMG instance views sorted by name.
func (rr *ResourceRegistry) PMGInstances() []*PMGInstanceView {
	return withViewCache(rr, func() []*PMGInstanceView { return rr.cachedPMG })
}

// K8sClusters returns cached Kubernetes cluster views sorted by name.
func (rr *ResourceRegistry) K8sClusters() []*K8sClusterView {
	return withViewCache(rr, func() []*K8sClusterView { return rr.cachedK8s })
}

func (rr *ResourceRegistry) K8sNodes() []*K8sNodeView {
	return withViewCache(rr, func() []*K8sNodeView { return rr.cachedK8sNodes })
}

func (rr *ResourceRegistry) Pods() []*PodView {
	return withViewCache(rr, func() []*PodView { return rr.cachedPods })
}

func (rr *ResourceRegistry) K8sDeployments() []*K8sDeploymentView {
	return withViewCache(rr, func() []*K8sDeploymentView { return rr.cachedK8sDeployments })
}

// Workloads returns a unified slice of canonical workload views sorted by name.
func (rr *ResourceRegistry) Workloads() []*WorkloadView {
	return withViewCache(rr, func() []*WorkloadView { return rr.cachedWorkload })
}

// Infrastructure returns a unified slice of all infrastructure parent resource views sorted by name.
func (rr *ResourceRegistry) Infrastructure() []*InfrastructureView {
	return withViewCache(rr, func() []*InfrastructureView { return rr.cachedInfra })
}

// Compile-time check: ResourceRegistry implements ReadState.
var _ ReadState = (*ResourceRegistry)(nil)
