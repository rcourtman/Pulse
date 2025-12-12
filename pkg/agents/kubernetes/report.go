package kubernetes

import "time"

// Report represents a single heartbeat from the Kubernetes agent to Pulse.
// It is designed to be useful without requiring Metrics Server (metrics.k8s.io).
type Report struct {
	Agent       AgentInfo    `json:"agent"`
	Cluster     ClusterInfo  `json:"cluster"`
	Nodes       []Node       `json:"nodes,omitempty"`
	Pods        []Pod        `json:"pods,omitempty"`
	Deployments []Deployment `json:"deployments,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
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

	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`

	Containers []PodContainer `json:"containers,omitempty"`
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
	UID               string            `json:"uid"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	DesiredReplicas   int32             `json:"desiredReplicas,omitempty"`
	UpdatedReplicas   int32             `json:"updatedReplicas,omitempty"`
	ReadyReplicas     int32             `json:"readyReplicas,omitempty"`
	AvailableReplicas int32             `json:"availableReplicas,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}
