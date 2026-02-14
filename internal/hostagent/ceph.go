package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	cephCommandTimeout       = 10 * time.Second
	maxCephCommandOutputSize = 4 << 20 // 4 MiB
)

var errCephCommandOutputTooLarge = errors.New("ceph command output exceeded limit")

type limitedBuffer struct {
	buf      bytes.Buffer
	maxBytes int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.exceeded = true
		return 0, errCephCommandOutputTooLarge
	}
	if len(p) > remaining {
		b.exceeded = true
		written, _ := b.buf.Write(p[:remaining])
		return written, errCephCommandOutputTooLarge
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

var commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout := limitedBuffer{maxBytes: maxCephCommandOutputSize}
	stderr := limitedBuffer{maxBytes: maxCephCommandOutputSize}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return stdout.Bytes(), stderr.Bytes(), errCephCommandOutputTooLarge
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

var lookPath = exec.LookPath

// CephClusterStatus represents the complete Ceph cluster status as collected by the agent.
type CephClusterStatus struct {
	FSID        string            `json:"fsid"`
	Health      CephHealthStatus  `json:"health"`
	MonMap      CephMonitorMap    `json:"monMap,omitempty"`
	MgrMap      CephManagerMap    `json:"mgrMap,omitempty"`
	OSDMap      CephOSDMap        `json:"osdMap"`
	PGMap       CephPGMap         `json:"pgMap"`
	Pools       []CephPool        `json:"pools,omitempty"`
	Services    []CephServiceInfo `json:"services,omitempty"`
	CollectedAt time.Time         `json:"collectedAt"`
}

// CephHealthStatus represents Ceph cluster health.
type CephHealthStatus struct {
	Status  string               `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]CephCheck `json:"checks,omitempty"`
	Summary []CephHealthSummary  `json:"summary,omitempty"`
}

// CephCheck represents a health check detail.
type CephCheck struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

// CephHealthSummary represents a health summary message.
type CephHealthSummary struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// CephMonitorMap represents Ceph monitor information.
type CephMonitorMap struct {
	Epoch    int           `json:"epoch"`
	NumMons  int           `json:"numMons"`
	Monitors []CephMonitor `json:"monitors,omitempty"`
}

// CephMonitor represents a single Ceph monitor.
type CephMonitor struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// CephManagerMap represents Ceph manager information.
type CephManagerMap struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"numMgrs"`
	ActiveMgr string `json:"activeMgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// CephOSDMap represents OSD status summary.
type CephOSDMap struct {
	Epoch   int `json:"epoch"`
	NumOSDs int `json:"numOsds"`
	NumUp   int `json:"numUp"`
	NumIn   int `json:"numIn"`
	NumDown int `json:"numDown,omitempty"`
	NumOut  int `json:"numOut,omitempty"`
}

// CephPGMap represents placement group statistics.
type CephPGMap struct {
	NumPGs           int     `json:"numPgs"`
	BytesTotal       uint64  `json:"bytesTotal"`
	BytesUsed        uint64  `json:"bytesUsed"`
	BytesAvailable   uint64  `json:"bytesAvailable"`
	DataBytes        uint64  `json:"dataBytes,omitempty"`
	UsagePercent     float64 `json:"usagePercent"`
	DegradedRatio    float64 `json:"degradedRatio,omitempty"`
	MisplacedRatio   float64 `json:"misplacedRatio,omitempty"`
	ReadBytesPerSec  uint64  `json:"readBytesPerSec,omitempty"`
	WriteBytesPerSec uint64  `json:"writeBytesPerSec,omitempty"`
	ReadOpsPerSec    uint64  `json:"readOpsPerSec,omitempty"`
	WriteOpsPerSec   uint64  `json:"writeOpsPerSec,omitempty"`
}

// CephPool represents a Ceph pool.
type CephPool struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytesUsed"`
	BytesAvailable uint64  `json:"bytesAvailable"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// CephServiceInfo represents a Ceph service summary.
type CephServiceInfo struct {
	Type    string   `json:"type"` // mon, mgr, osd, mds, rgw
	Running int      `json:"running"`
	Total   int      `json:"total"`
	Daemons []string `json:"daemons,omitempty"`
}

// IsCephAvailable checks if the ceph CLI is available on the system.
func IsCephAvailable(ctx context.Context) bool {
	_ = ctx
	_, err := lookPath("ceph")
	return err == nil
}

// CollectCeph gathers Ceph cluster status using the ceph CLI.
// Returns nil if Ceph is not available or not configured on this host.
func CollectCeph(ctx context.Context) (*CephClusterStatus, error) {
	// Check if ceph CLI is available
	if !IsCephAvailable(ctx) {
		return nil, nil
	}

	// Try to get ceph status - this will fail if not a ceph node
	statusJSON, err := runCephCommand(ctx, "status", "--format", "json")
	if err != nil {
		// Not an error - just means this isn't a Ceph node
		return nil, nil
	}

	status, err := parseCephStatus(statusJSON)
	if err != nil {
		return nil, err
	}

	// Get pool usage from ceph df
	dfJSON, err := runCephCommand(ctx, "df", "--format", "json")
	if err == nil {
		pools, usagePercent, err := parseCephDF(dfJSON)
		if err == nil {
			status.Pools = pools
			if status.PGMap.UsagePercent == 0 && usagePercent > 0 {
				status.PGMap.UsagePercent = usagePercent
			}
		}
	}

	status.CollectedAt = time.Now().UTC()
	return status, nil
}

// runCephCommand executes a ceph command and returns the output.
func runCephCommand(ctx context.Context, args ...string) ([]byte, error) {
	cephPath, err := lookPath("ceph")
	if err != nil {
		return nil, fmt.Errorf("ceph binary not found: %w", err)
	}

	// Use a reasonable timeout for each command
	cmdCtx, cancel := context.WithTimeout(ctx, cephCommandTimeout)
	defer cancel()

	stdout, stderr, err := commandRunner(cmdCtx, cephPath, args...)
	if err != nil {
		if errors.Is(err, errCephCommandOutputTooLarge) {
			return nil, fmt.Errorf("ceph %s output exceeded %d bytes",
				strings.Join(args, " "), maxCephCommandOutputSize)
		}
		return nil, fmt.Errorf("ceph %s failed: %w (stderr: %s)",
			strings.Join(args, " "), err, string(stderr))
	}

	return stdout, nil
}

// parseCephStatus parses the output of `ceph status --format json`.
func parseCephStatus(data []byte) (*CephClusterStatus, error) {
	var raw struct {
		FSID   string `json:"fsid"`
		Health struct {
			Status string `json:"status"`
			Checks map[string]struct {
				Severity string `json:"severity"`
				Summary  struct {
					Message string `json:"message"`
				} `json:"summary"`
				Detail []struct {
					Message string `json:"message"`
				} `json:"detail"`
			} `json:"checks"`
		} `json:"health"`
		MonMap struct {
			Epoch int `json:"epoch"`
			Mons  []struct {
				Name string `json:"name"`
				Rank int    `json:"rank"`
				Addr string `json:"addr"`
			} `json:"mons"`
		} `json:"monmap"`
		MgrMap struct {
			Available  bool   `json:"available"`
			NumActive  int    `json:"num_active_name,omitempty"`
			ActiveName string `json:"active_name"`
			Standbys   []struct {
				Name string `json:"name"`
			} `json:"standbys"`
		} `json:"mgrmap"`
		OSDMap struct {
			Epoch   int `json:"epoch"`
			NumOSDs int `json:"num_osds"`
			NumUp   int `json:"num_up_osds"`
			NumIn   int `json:"num_in_osds"`
		} `json:"osdmap"`
		PGMap struct {
			NumPGs           int     `json:"num_pgs"`
			BytesTotal       uint64  `json:"bytes_total"`
			BytesUsed        uint64  `json:"bytes_used"`
			BytesAvail       uint64  `json:"bytes_avail"`
			DataBytes        uint64  `json:"data_bytes"`
			DegradedRatio    float64 `json:"degraded_ratio"`
			MisplacedRatio   float64 `json:"misplaced_ratio"`
			ReadBytesPerSec  uint64  `json:"read_bytes_sec"`
			WriteBytesPerSec uint64  `json:"write_bytes_sec"`
			ReadOpsPerSec    uint64  `json:"read_op_per_sec"`
			WriteOpsPerSec   uint64  `json:"write_op_per_sec"`
		} `json:"pgmap"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ceph.parseStatus: unmarshal ceph status JSON: %w", err)
	}

	status := &CephClusterStatus{
		FSID: raw.FSID,
		Health: CephHealthStatus{
			Status: raw.Health.Status,
			Checks: make(map[string]CephCheck),
		},
		MonMap: CephMonitorMap{
			Epoch:   raw.MonMap.Epoch,
			NumMons: len(raw.MonMap.Mons),
		},
		MgrMap: CephManagerMap{
			Available: raw.MgrMap.Available,
			NumMgrs:   1 + len(raw.MgrMap.Standbys),
			ActiveMgr: raw.MgrMap.ActiveName,
			Standbys:  len(raw.MgrMap.Standbys),
		},
		OSDMap: CephOSDMap{
			Epoch:   raw.OSDMap.Epoch,
			NumOSDs: raw.OSDMap.NumOSDs,
			NumUp:   raw.OSDMap.NumUp,
			NumIn:   raw.OSDMap.NumIn,
			NumDown: raw.OSDMap.NumOSDs - raw.OSDMap.NumUp,
			NumOut:  raw.OSDMap.NumOSDs - raw.OSDMap.NumIn,
		},
		PGMap: CephPGMap{
			NumPGs:           raw.PGMap.NumPGs,
			BytesTotal:       raw.PGMap.BytesTotal,
			BytesUsed:        raw.PGMap.BytesUsed,
			BytesAvailable:   raw.PGMap.BytesAvail,
			DataBytes:        raw.PGMap.DataBytes,
			DegradedRatio:    raw.PGMap.DegradedRatio,
			MisplacedRatio:   raw.PGMap.MisplacedRatio,
			ReadBytesPerSec:  raw.PGMap.ReadBytesPerSec,
			WriteBytesPerSec: raw.PGMap.WriteBytesPerSec,
			ReadOpsPerSec:    raw.PGMap.ReadOpsPerSec,
			WriteOpsPerSec:   raw.PGMap.WriteOpsPerSec,
		},
	}

	// Calculate usage percent
	if raw.PGMap.BytesTotal > 0 {
		status.PGMap.UsagePercent = float64(raw.PGMap.BytesUsed) / float64(raw.PGMap.BytesTotal) * 100
	}

	// Parse monitors
	for _, mon := range raw.MonMap.Mons {
		status.MonMap.Monitors = append(status.MonMap.Monitors, CephMonitor{
			Name: mon.Name,
			Rank: mon.Rank,
			Addr: mon.Addr,
		})
	}

	// Parse health checks
	for name, check := range raw.Health.Checks {
		details := make([]string, 0, len(check.Detail))
		for _, d := range check.Detail {
			details = append(details, d.Message)
		}
		status.Health.Checks[name] = CephCheck{
			Severity: check.Severity,
			Message:  check.Summary.Message,
			Detail:   details,
		}
	}

	// Build service summary
	status.Services = []CephServiceInfo{
		{Type: "mon", Running: len(raw.MonMap.Mons), Total: len(raw.MonMap.Mons)},
		{Type: "mgr", Running: cephBoolToInt(raw.MgrMap.Available), Total: status.MgrMap.NumMgrs},
		{Type: "osd", Running: raw.OSDMap.NumUp, Total: raw.OSDMap.NumOSDs},
	}

	return status, nil
}

// parseCephDF parses the output of `ceph df --format json`.
func parseCephDF(data []byte) ([]CephPool, float64, error) {
	var raw struct {
		Stats struct {
			TotalBytes     uint64  `json:"total_bytes"`
			TotalUsedBytes uint64  `json:"total_used_bytes"`
			PercentUsed    float64 `json:"percent_used"`
		} `json:"stats"`
		Pools []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Stats struct {
				BytesUsed   uint64  `json:"bytes_used"`
				MaxAvail    uint64  `json:"max_avail"`
				Objects     uint64  `json:"objects"`
				PercentUsed float64 `json:"percent_used"`
			} `json:"stats"`
		} `json:"pools"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, 0, fmt.Errorf("ceph.parseDF: unmarshal ceph df JSON: %w", err)
	}

	pools := make([]CephPool, 0, len(raw.Pools))
	for _, p := range raw.Pools {
		pools = append(pools, CephPool{
			ID:             p.ID,
			Name:           p.Name,
			BytesUsed:      p.Stats.BytesUsed,
			BytesAvailable: p.Stats.MaxAvail,
			Objects:        p.Stats.Objects,
			PercentUsed:    normalizePercentUsed(p.Stats.PercentUsed),
		})
	}

	return pools, normalizePercentUsed(raw.Stats.PercentUsed), nil
}

func cephBoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// normalizePercentUsed accepts Ceph values reported either as ratio (0-1) or percent (0-100).
func normalizePercentUsed(value float64) float64 {
	if value >= 0 && value <= 1 {
		return value * 100
	}
	return value
}
