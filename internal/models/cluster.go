package models

// ClusterHealth represents the health status of a cluster
type ClusterHealth struct {
	Name             string              `json:"name"`
	TotalNodes       int                 `json:"totalNodes"`
	OnlineNodes      int                 `json:"onlineNodes"`
	OfflineNodes     int                 `json:"offlineNodes"`
	HealthPercentage float64             `json:"healthPercentage"`
	NodeStatuses     []ClusterNodeStatus `json:"nodeStatuses"`
}

// ClusterNodeStatus represents the status of a single node in a cluster
type ClusterNodeStatus struct {
	Endpoint string `json:"endpoint"`
	Online   bool   `json:"online"`
}
