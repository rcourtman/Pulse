package kubernetes

import "time"

// Report represents a single heartbeat from the Kubernetes agent to Pulse.
// It is designed to be useful without requiring Metrics Server (metrics.k8s.io).
type Report struct {
	Agent                    AgentInfo                 `json:"agent"`
	Cluster                  ClusterInfo               `json:"cluster"`
	Nodes                    []Node                    `json:"nodes,omitempty"`
	Namespaces               []Namespace               `json:"namespaces,omitempty"`
	Pods                     []Pod                     `json:"pods,omitempty"`
	Deployments              []Deployment              `json:"deployments,omitempty"`
	ReplicaSets              []ReplicaSet              `json:"replicaSets,omitempty"`
	StatefulSets             []StatefulSet             `json:"statefulSets,omitempty"`
	DaemonSets               []DaemonSet               `json:"daemonSets,omitempty"`
	Services                 []Service                 `json:"services,omitempty"`
	Jobs                     []Job                     `json:"jobs,omitempty"`
	CronJobs                 []CronJob                 `json:"cronJobs,omitempty"`
	Ingresses                []Ingress                 `json:"ingresses,omitempty"`
	EndpointSlices           []EndpointSlice           `json:"endpointSlices,omitempty"`
	NetworkPolicies          []NetworkPolicy           `json:"networkPolicies,omitempty"`
	PersistentVolumes        []PersistentVolume        `json:"persistentVolumes,omitempty"`
	PersistentVolumeClaims   []PersistentVolumeClaim   `json:"persistentVolumeClaims,omitempty"`
	StorageClasses           []StorageClass            `json:"storageClasses,omitempty"`
	ConfigMaps               []ConfigMap               `json:"configMaps,omitempty"`
	Secrets                  []Secret                  `json:"secrets,omitempty"`
	ServiceAccounts          []ServiceAccount          `json:"serviceAccounts,omitempty"`
	ResourceQuotas           []ResourceQuota           `json:"resourceQuotas,omitempty"`
	LimitRanges              []LimitRange              `json:"limitRanges,omitempty"`
	PodDisruptionBudgets     []PodDisruptionBudget     `json:"podDisruptionBudgets,omitempty"`
	HorizontalPodAutoscalers []HorizontalPodAutoscaler `json:"horizontalPodAutoscalers,omitempty"`
	Events                   []Event                   `json:"events,omitempty"`
	Recovery                 *RecoveryReport           `json:"recovery,omitempty"`
	Timestamp                time.Time                 `json:"timestamp"`
}

// AgentInfo describes the reporting agent instance.
type AgentInfo struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Type            string `json:"type,omitempty"` // "unified" when running as part of pulse-agent
	IntervalSeconds int    `json:"intervalSeconds"`
}

// ClusterInfo identifies the Kubernetes cluster and provides basic metadata.
type ClusterInfo struct {
	ID        string `json:"id"`                  // Stable cluster identifier (agent-generated)
	Name      string `json:"name,omitempty"`      // Optional human-friendly name
	Server    string `json:"server,omitempty"`    // API server URL
	Context   string `json:"context,omitempty"`   // Kubeconfig context name if applicable
	Version   string `json:"version,omitempty"`   // Kubernetes version string
	Provider  string `json:"provider,omitempty"`  // Optional: gke, eks, aks, etc (best-effort)
	Namespace string `json:"namespace,omitempty"` // Optional agent namespace when running in-cluster
}

// Node represents a Kubernetes node at report time.
type Node struct {
	UID                     string        `json:"uid"`
	Name                    string        `json:"name"`
	Ready                   bool          `json:"ready"`
	Unschedulable           bool          `json:"unschedulable,omitempty"`
	KubeletVersion          string        `json:"kubeletVersion,omitempty"`
	ContainerRuntimeVersion string        `json:"containerRuntimeVersion,omitempty"`
	OSImage                 string        `json:"osImage,omitempty"`
	KernelVersion           string        `json:"kernelVersion,omitempty"`
	Architecture            string        `json:"architecture,omitempty"`
	Capacity                NodeResources `json:"capacity,omitempty"`
	Allocatable             NodeResources `json:"allocatable,omitempty"`
	Usage                   *NodeUsage    `json:"usage,omitempty"`
	Roles                   []string      `json:"roles,omitempty"`
}

type NodeResources struct {
	CPUCores    int64 `json:"cpuCores,omitempty"`
	MemoryBytes int64 `json:"memoryBytes,omitempty"`
	Pods        int64 `json:"pods,omitempty"`
}

// Pod represents a Kubernetes pod at report time.
type Pod struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	NodeName  string            `json:"nodeName,omitempty"`
	Phase     string            `json:"phase,omitempty"`  // Pending, Running, Succeeded, Failed, Unknown
	Reason    string            `json:"reason,omitempty"` // Best-effort
	Message   string            `json:"message,omitempty"`
	QoSClass  string            `json:"qosClass,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	StartTime *time.Time        `json:"startTime,omitempty"`
	Restarts  int               `json:"restarts,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	OwnerKind string    `json:"ownerKind,omitempty"`
	OwnerName string    `json:"ownerName,omitempty"`
	Usage     *PodUsage `json:"usage,omitempty"`

	Containers []PodContainer `json:"containers,omitempty"`
}

type NodeUsage struct {
	CPUMilliCores int64 `json:"cpuMilliCores,omitempty"`
	MemoryBytes   int64 `json:"memoryBytes,omitempty"`
}

type PodUsage struct {
	CPUMilliCores                 int64 `json:"cpuMilliCores,omitempty"`
	MemoryBytes                   int64 `json:"memoryBytes,omitempty"`
	NetworkRxBytes                int64 `json:"networkRxBytes,omitempty"`
	NetworkTxBytes                int64 `json:"networkTxBytes,omitempty"`
	EphemeralStorageUsedBytes     int64 `json:"ephemeralStorageUsedBytes,omitempty"`
	EphemeralStorageCapacityBytes int64 `json:"ephemeralStorageCapacityBytes,omitempty"`
}

type PodContainer struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount,omitempty"`
	State        string `json:"state,omitempty"`  // waiting, running, terminated
	Reason       string `json:"reason,omitempty"` // waiting/terminated reason
	Message      string `json:"message,omitempty"`
}

// Deployment represents a Kubernetes deployment at report time.
type Deployment struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	CreatedAt          time.Time         `json:"createdAt,omitempty"`
	DesiredReplicas    int32             `json:"desiredReplicas,omitempty"`
	UpdatedReplicas    int32             `json:"updatedReplicas,omitempty"`
	ReadyReplicas      int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas  int32             `json:"availableReplicas,omitempty"`
	ObservedGeneration int64             `json:"observedGeneration,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// ReplicaSet represents a Kubernetes replica set at report time.
type ReplicaSet struct {
	UID                  string            `json:"uid"`
	Name                 string            `json:"name"`
	Namespace            string            `json:"namespace"`
	DesiredReplicas      int32             `json:"desiredReplicas,omitempty"`
	ReadyReplicas        int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas    int32             `json:"availableReplicas,omitempty"`
	FullyLabeledReplicas int32             `json:"fullyLabeledReplicas,omitempty"`
	ObservedGeneration   int64             `json:"observedGeneration,omitempty"`
	OwnerKind            string            `json:"ownerKind,omitempty"`
	OwnerName            string            `json:"ownerName,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

// Namespace represents a Kubernetes namespace at report time.
type Namespace struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Phase     string            `json:"phase,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// StatefulSet represents a Kubernetes stateful set at report time.
type StatefulSet struct {
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas,omitempty"`
	ReadyReplicas     int32             `json:"readyReplicas,omitempty"`
	CurrentReplicas   int32             `json:"currentReplicas,omitempty"`
	UpdatedReplicas   int32             `json:"updatedReplicas,omitempty"`
	AvailableReplicas int32             `json:"availableReplicas,omitempty"`
	ServiceName       string            `json:"serviceName,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// DaemonSet represents a Kubernetes daemon set at report time.
type DaemonSet struct {
	UID                    string            `json:"uid"`
	Name                   string            `json:"name"`
	Namespace              string            `json:"namespace"`
	DesiredNumberScheduled int32             `json:"desiredNumberScheduled,omitempty"`
	CurrentNumberScheduled int32             `json:"currentNumberScheduled,omitempty"`
	NumberReady            int32             `json:"numberReady,omitempty"`
	UpdatedNumberScheduled int32             `json:"updatedNumberScheduled,omitempty"`
	NumberAvailable        int32             `json:"numberAvailable,omitempty"`
	NumberUnavailable      int32             `json:"numberUnavailable,omitempty"`
	NumberMisscheduled     int32             `json:"numberMisscheduled,omitempty"`
	Labels                 map[string]string `json:"labels,omitempty"`
}

// Service represents a Kubernetes service at report time.
type Service struct {
	UID         string            `json:"uid"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Type        string            `json:"type,omitempty"`
	ClusterIP   string            `json:"clusterIp,omitempty"`
	ExternalIPs []string          `json:"externalIps,omitempty"`
	Ports       []ServicePort     `json:"ports,omitempty"`
	Selector    map[string]string `json:"selector,omitempty"`
	CreatedAt   time.Time         `json:"createdAt,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ServicePort describes one Kubernetes service port.
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Port       int32  `json:"port,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

// Job represents a Kubernetes job at report time.
type Job struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	DesiredCompletions int32             `json:"desiredCompletions,omitempty"`
	Succeeded          int32             `json:"succeeded,omitempty"`
	Failed             int32             `json:"failed,omitempty"`
	Active             int32             `json:"active,omitempty"`
	StartTime          *time.Time        `json:"startTime,omitempty"`
	CompletionTime     *time.Time        `json:"completionTime,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// CronJob represents a Kubernetes cron job at report time.
type CronJob struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	Schedule           string            `json:"schedule,omitempty"`
	Suspend            bool              `json:"suspend,omitempty"`
	Active             int               `json:"active,omitempty"`
	LastScheduleTime   *time.Time        `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime *time.Time        `json:"lastSuccessfulTime,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// Ingress represents a Kubernetes ingress at report time.
type Ingress struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	ClassName string            `json:"className,omitempty"`
	Hosts     []string          `json:"hosts,omitempty"`
	Addresses []string          `json:"addresses,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// EndpointSlice represents a Kubernetes discovery EndpointSlice at report time.
type EndpointSlice struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	AddressType        string            `json:"addressType,omitempty"`
	ServiceName        string            `json:"serviceName,omitempty"`
	Ports              []EndpointPort    `json:"ports,omitempty"`
	EndpointCount      int               `json:"endpointCount,omitempty"`
	ReadyEndpointCount int               `json:"readyEndpointCount,omitempty"`
	CreatedAt          time.Time         `json:"createdAt,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// EndpointPort describes one EndpointSlice port.
type EndpointPort struct {
	Name        string `json:"name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Port        int32  `json:"port,omitempty"`
	AppProtocol string `json:"appProtocol,omitempty"`
}

// NetworkPolicy represents a Kubernetes network policy at report time.
type NetworkPolicy struct {
	UID              string            `json:"uid"`
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	PolicyTypes      []string          `json:"policyTypes,omitempty"`
	IngressRuleCount int               `json:"ingressRuleCount,omitempty"`
	EgressRuleCount  int               `json:"egressRuleCount,omitempty"`
	CreatedAt        time.Time         `json:"createdAt,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

// PersistentVolume represents a Kubernetes persistent volume at report time.
type PersistentVolume struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Phase          string            `json:"phase,omitempty"`
	StorageClass   string            `json:"storageClass,omitempty"`
	CapacityBytes  int64             `json:"capacityBytes,omitempty"`
	AccessModes    []string          `json:"accessModes,omitempty"`
	ReclaimPolicy  string            `json:"reclaimPolicy,omitempty"`
	ClaimNamespace string            `json:"claimNamespace,omitempty"`
	ClaimName      string            `json:"claimName,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// PersistentVolumeClaim represents a Kubernetes persistent volume claim at report time.
type PersistentVolumeClaim struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Phase          string            `json:"phase,omitempty"`
	StorageClass   string            `json:"storageClass,omitempty"`
	RequestedBytes int64             `json:"requestedBytes,omitempty"`
	CapacityBytes  int64             `json:"capacityBytes,omitempty"`
	AccessModes    []string          `json:"accessModes,omitempty"`
	VolumeName     string            `json:"volumeName,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// StorageClass represents a Kubernetes storage class at report time.
type StorageClass struct {
	UID                  string            `json:"uid"`
	Name                 string            `json:"name"`
	Provisioner          string            `json:"provisioner,omitempty"`
	ReclaimPolicy        string            `json:"reclaimPolicy,omitempty"`
	VolumeBindingMode    string            `json:"volumeBindingMode,omitempty"`
	AllowVolumeExpansion *bool             `json:"allowVolumeExpansion,omitempty"`
	ParameterKeys        []string          `json:"parameterKeys,omitempty"`
	CreatedAt            time.Time         `json:"createdAt,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}

// ConfigMap represents Kubernetes ConfigMap metadata at report time.
// ConfigMap payload values are intentionally omitted. Current agents prefer
// Kubernetes metadata-only API responses and therefore do not need to receive
// ConfigMap data values at all; older reports may include key names only.
type ConfigMap struct {
	UID            string            `json:"uid"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	DataKeys       []string          `json:"dataKeys,omitempty"`
	BinaryDataKeys []string          `json:"binaryDataKeys,omitempty"`
	Immutable      bool              `json:"immutable,omitempty"`
	MetadataOnly   bool              `json:"metadataOnly,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// Secret represents Kubernetes Secret metadata at report time.
// Secret payload values are intentionally omitted. Current agents use
// Kubernetes metadata-only API responses for Secret inventory; older reports
// may include type and key names, but never Secret values.
type Secret struct {
	UID          string            `json:"uid"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Type         string            `json:"type,omitempty"`
	DataKeys     []string          `json:"dataKeys,omitempty"`
	Immutable    bool              `json:"immutable,omitempty"`
	MetadataOnly bool              `json:"metadataOnly,omitempty"`
	CreatedAt    time.Time         `json:"createdAt,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// ServiceAccount represents Kubernetes ServiceAccount metadata at report time.
type ServiceAccount struct {
	UID                          string            `json:"uid"`
	Name                         string            `json:"name"`
	Namespace                    string            `json:"namespace"`
	AutomountServiceAccountToken *bool             `json:"automountServiceAccountToken,omitempty"`
	SecretCount                  int               `json:"secretCount,omitempty"`
	ImagePullSecrets             []string          `json:"imagePullSecrets,omitempty"`
	CreatedAt                    time.Time         `json:"createdAt,omitempty"`
	Labels                       map[string]string `json:"labels,omitempty"`
}

// ResourceQuota represents Kubernetes ResourceQuota metadata and observed usage.
type ResourceQuota struct {
	UID       string            `json:"uid"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Hard      map[string]string `json:"hard,omitempty"`
	Used      map[string]string `json:"used,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// LimitRange represents Kubernetes LimitRange metadata at report time.
type LimitRange struct {
	UID        string            `json:"uid"`
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	LimitTypes []string          `json:"limitTypes,omitempty"`
	CreatedAt  time.Time         `json:"createdAt,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// PodDisruptionBudget represents Kubernetes PodDisruptionBudget policy and status.
type PodDisruptionBudget struct {
	UID                string            `json:"uid"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	MinAvailable       string            `json:"minAvailable,omitempty"`
	MaxUnavailable     string            `json:"maxUnavailable,omitempty"`
	DesiredHealthy     int32             `json:"desiredHealthy,omitempty"`
	CurrentHealthy     int32             `json:"currentHealthy,omitempty"`
	DisruptionsAllowed int32             `json:"disruptionsAllowed,omitempty"`
	ExpectedPods       int32             `json:"expectedPods,omitempty"`
	CreatedAt          time.Time         `json:"createdAt,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// HorizontalPodAutoscaler represents Kubernetes autoscaling/v2 HPA metadata and status.
type HorizontalPodAutoscaler struct {
	UID             string            `json:"uid"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	TargetKind      string            `json:"targetKind,omitempty"`
	TargetName      string            `json:"targetName,omitempty"`
	MinReplicas     int32             `json:"minReplicas,omitempty"`
	MaxReplicas     int32             `json:"maxReplicas,omitempty"`
	CurrentReplicas int32             `json:"currentReplicas,omitempty"`
	DesiredReplicas int32             `json:"desiredReplicas,omitempty"`
	MetricTypes     []string          `json:"metricTypes,omitempty"`
	CreatedAt       time.Time         `json:"createdAt,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}

// Event represents a Kubernetes event at report time.
type Event struct {
	UID          string     `json:"uid"`
	Name         string     `json:"name"`
	Namespace    string     `json:"namespace,omitempty"`
	Type         string     `json:"type,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Message      string     `json:"message,omitempty"`
	InvolvedKind string     `json:"involvedKind,omitempty"`
	InvolvedName string     `json:"involvedName,omitempty"`
	Count        int32      `json:"count,omitempty"`
	FirstSeen    *time.Time `json:"firstSeen,omitempty"`
	LastSeen     *time.Time `json:"lastSeen,omitempty"`
	EventTime    *time.Time `json:"eventTime,omitempty"`
}

// RecoveryReport contains optional, best-effort "recovery point" artifacts discovered by the agent.
// This is intentionally capped and time-bounded by the agent to keep payloads small.
type RecoveryReport struct {
	VolumeSnapshots []VolumeSnapshot `json:"volumeSnapshots,omitempty"`
	VeleroBackups   []VeleroBackup   `json:"veleroBackups,omitempty"`
}

// VolumeSnapshot mirrors a subset of snapshot.storage.k8s.io/v1 VolumeSnapshot fields.
// The agent should populate these from the live API when permissions and APIs are available.
type VolumeSnapshot struct {
	UID           string `json:"uid,omitempty"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	SnapshotClass string `json:"snapshotClass,omitempty"`

	SourcePVC    string `json:"sourcePvc,omitempty"`
	SourcePVCUID string `json:"sourcePvcUid,omitempty"`

	ReadyToUse       *bool      `json:"readyToUse,omitempty"`
	RestoreSizeBytes *int64     `json:"restoreSizeBytes,omitempty"`
	CreationTime     *time.Time `json:"creationTime,omitempty"`
	CompletionTime   *time.Time `json:"completionTime,omitempty"`
	ContentName      string     `json:"contentName,omitempty"`
	Error            string     `json:"error,omitempty"`
}

// VeleroBackup mirrors a subset of Velero Backup CRD fields (backups.velero.io).
// The agent should only populate these when Velero is installed and RBAC allows reading it.
type VeleroBackup struct {
	UID       string `json:"uid,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Phase     string `json:"phase,omitempty"`

	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Expiration  *time.Time `json:"expiration,omitempty"`

	StorageLocation string `json:"storageLocation,omitempty"`
}
