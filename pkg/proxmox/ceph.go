package proxmox

import (
	"context"
	"encoding/json"
)

// CephStatus represents the Ceph cluster status information returned by /cluster/ceph/status
// Only the fields required for monitoring are included here.
type CephStatus struct {
	FSID       string         `json:"fsid"`
	Health     CephHealth     `json:"health"`
	ServiceMap CephServiceMap `json:"servicemap"`
	OSDMap     CephOSDMap     `json:"osdmap"`
	PGMap      CephPGMap      `json:"pgmap"`
}

// CephHealth captures cluster health status and summaries.
type CephHealth struct {
	Status  string                        `json:"status"`
	Summary []CephHealthSummary           `json:"summary"`
	Checks  map[string]CephHealthCheckRaw `json:"checks"`
}

// CephHealthSummary holds high-level messages describing the health state.
type CephHealthSummary struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Message  string `json:"message,omitempty"`
}

// CephHealthCheckRaw captures the raw JSON blocks describing specific health checks.
type CephHealthCheckRaw struct {
	Severity string            `json:"severity"`
	Summary  json.RawMessage   `json:"summary"`
	Detail   []CephCheckDetail `json:"detail"`
}

// CephCheckDetail represents individual health detail messages.
type CephCheckDetail struct {
	Message string `json:"message"`
}

// CephServiceMap contains information about Ceph services and daemon states.
type CephServiceMap struct {
	Services map[string]CephServiceDefinition `json:"services"`
}

// CephServiceDefinition represents a Ceph service (e.g., mon, mgr) and its daemons.
type CephServiceDefinition struct {
	Daemons map[string]CephServiceDaemon `json:"daemons"`
}

// CephServiceDaemon represents the status of an individual Ceph daemon.
type CephServiceDaemon struct {
	Host   string `json:"hostname"`
	Status string `json:"status"`
}

// CephOSDMap captures summary statistics about OSDs.
type CephOSDMap struct {
	NumOSDs   int `json:"num_osds"`
	NumUpOSDs int `json:"num_up_osds"`
	NumInOSDs int `json:"num_in_osds"`
}

// CephPGMap captures placement group statistics.
type CephPGMap struct {
	NumPGs        int     `json:"num_pgs"`
	BytesTotal    uint64  `json:"bytes_total"`
	BytesUsed     uint64  `json:"bytes_used"`
	BytesAvail    uint64  `json:"bytes_avail"`
	DataBytes     uint64  `json:"data_bytes"`
	Dirty         uint64  `json:"dirty"`
	Unfound       uint64  `json:"unfound"`
	DegradedRatio float64 `json:"degraded_ratio"`
}

// CephDF represents the data returned by /cluster/ceph/df.
type CephDF struct {
	Data CephDFData `json:"data"`
}

// CephDFData wraps high-level capacity stats and per-pool information.
type CephDFData struct {
	Stats CephDFStats  `json:"stats"`
	Pools []CephDFPool `json:"pools"`
}

// CephDFStats captures total cluster capacity usage metrics.
type CephDFStats struct {
	TotalBytes      uint64  `json:"total_bytes"`
	TotalUsedBytes  uint64  `json:"total_used_bytes"`
	TotalAvailBytes uint64  `json:"total_avail_bytes"`
	RawUsedBytes    uint64  `json:"total_used_raw_bytes"`
	PercentUsed     float64 `json:"percent_used"`
}

// CephDFPool represents usage metrics for an individual Ceph pool.
type CephDFPool struct {
	ID    int            `json:"id"`
	Name  string         `json:"name"`
	Stats CephDFPoolStat `json:"stats"`
}

// CephDFPoolStat represents statistics for a Ceph pool returned by /cluster/ceph/df.
type CephDFPoolStat struct {
	BytesUsed    uint64  `json:"bytes_used"`
	KBUsed       uint64  `json:"kb_used"`
	MaxAvail     uint64  `json:"max_avail"`
	Objects      uint64  `json:"objects"`
	PercentUsed  float64 `json:"percent_used"`
	Dirty        uint64  `json:"dirty"`
	RawBytesUsed uint64  `json:"stored_raw"`
}

// GetCephStatus fetches Ceph status information for the cluster.
func (c *Client) GetCephStatus(ctx context.Context) (*CephStatus, error) {
	resp, err := c.get(ctx, "/cluster/ceph/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data CephStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// GetCephDF fetches Ceph capacity information (/cluster/ceph/df).
func (c *Client) GetCephDF(ctx context.Context) (*CephDF, error) {
	resp, err := c.get(ctx, "/cluster/ceph/df")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data CephDF `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}
