// Package ceph provides functionality for collecting Ceph cluster status
// directly from the local system using the ceph CLI.
package ceph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

var commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// ClusterStatus represents the complete Ceph cluster status as collected by the agent.
type ClusterStatus struct {
	FSID        string        `json:"fsid"`
	Health      HealthStatus  `json:"health"`
	MonMap      MonitorMap    `json:"monMap,omitempty"`
	MgrMap      ManagerMap    `json:"mgrMap,omitempty"`
	OSDMap      OSDMap        `json:"osdMap"`
	PGMap       PGMap         `json:"pgMap"`
	Pools       []Pool        `json:"pools,omitempty"`
	Services    []ServiceInfo `json:"services,omitempty"`
	CollectedAt time.Time     `json:"collectedAt"`
}

// HealthStatus represents Ceph cluster health.
type HealthStatus struct {
	Status  string           `json:"status"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	Checks  map[string]Check `json:"checks,omitempty"`
	Summary []HealthSummary  `json:"summary,omitempty"`
}

// Check represents a health check detail.
type Check struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message,omitempty"`
	Detail   []string `json:"detail,omitempty"`
}

// HealthSummary represents a health summary message.
type HealthSummary struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// MonitorMap represents Ceph monitor information.
type MonitorMap struct {
	Epoch    int       `json:"epoch"`
	NumMons  int       `json:"numMons"`
	Monitors []Monitor `json:"monitors,omitempty"`
}

// Monitor represents a single Ceph monitor.
type Monitor struct {
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Addr   string `json:"addr,omitempty"`
	Status string `json:"status,omitempty"`
}

// ManagerMap represents Ceph manager information.
type ManagerMap struct {
	Available bool   `json:"available"`
	NumMgrs   int    `json:"numMgrs"`
	ActiveMgr string `json:"activeMgr,omitempty"`
	Standbys  int    `json:"standbys"`
}

// OSDMap represents OSD status summary.
type OSDMap struct {
	Epoch   int `json:"epoch"`
	NumOSDs int `json:"numOsds"`
	NumUp   int `json:"numUp"`
	NumIn   int `json:"numIn"`
	NumDown int `json:"numDown,omitempty"`
	NumOut  int `json:"numOut,omitempty"`
}

// PGMap represents placement group statistics.
type PGMap struct {
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

// Pool represents a Ceph pool.
type Pool struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	BytesUsed      uint64  `json:"bytesUsed"`
	BytesAvailable uint64  `json:"bytesAvailable"`
	Objects        uint64  `json:"objects"`
	PercentUsed    float64 `json:"percentUsed"`
}

// ServiceInfo represents a Ceph service summary.
type ServiceInfo struct {
	Type    string   `json:"type"` // mon, mgr, osd, mds, rgw
	Running int      `json:"running"`
	Total   int      `json:"total"`
	Daemons []string `json:"daemons,omitempty"`
}

// IsAvailable checks if the ceph CLI is available on the system.
func IsAvailable(ctx context.Context) bool {
	_, _, err := commandRunner(ctx, "which", "ceph")
	return err == nil
}

// Collect gathers Ceph cluster status using the ceph CLI.
// Returns nil if Ceph is not available or not configured on this host.
func Collect(ctx context.Context) (*ClusterStatus, error) {
	// Check if ceph CLI is available
	if !IsAvailable(ctx) {
		return nil, nil
	}

	// Try to get ceph status - this will fail if not a ceph node
	statusJSON, err := runCephCommand(ctx, "status", "--format", "json")
	if err != nil {
		// Not an error - just means this isn't a Ceph node
		return nil, nil
	}

	status, err := parseStatus(statusJSON)
	if err != nil {
		return nil, fmt.Errorf("parse ceph status: %w", err)
	}

	// Get pool usage from ceph df
	dfJSON, err := runCephCommand(ctx, "df", "--format", "json")
	if err == nil {
		pools, usagePercent, err := parseDF(dfJSON)
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
	// Use a reasonable timeout for each command
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stdout, stderr, err := commandRunner(cmdCtx, "ceph", args...)
	if err != nil {
		return nil, fmt.Errorf("ceph %s failed: %w (stderr: %s)",
			strings.Join(args, " "), err, string(stderr))
	}

	return stdout, nil
}

// parseStatus parses the output of `ceph status --format json`.
func parseStatus(data []byte) (*ClusterStatus, error) {
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
			Epoch       int      `json:"epoch"`
			NumMons     int      `json:"num_mons"`
			QuorumNames []string `json:"quorum_names"`
			Mons        []struct {
				Name string `json:"name"`
				Rank int    `json:"rank"`
				Addr string `json:"addr"`
			} `json:"mons"`
		} `json:"monmap"`
		MgrMap struct {
			Available   bool   `json:"available"`
			ActiveName  string `json:"active_name"`
			NumStandbys int    `json:"num_standbys"`
			NumStandby  int    `json:"num_standby"`
			Standbys    []struct {
				Name string `json:"name"`
			} `json:"standbys"`
		} `json:"mgrmap"`
		ServiceMap struct {
			Services map[string]struct {
				Daemons map[string]struct {
					Status   string `json:"status"`
					Hostname string `json:"hostname"`
					Metadata struct {
						Hostname string `json:"hostname"`
					} `json:"metadata"`
				} `json:"daemons"`
			} `json:"services"`
		} `json:"servicemap"`
		OSDMap struct {
			Epoch  int `json:"epoch"`
			NumOSD int `json:"num_osds"`
			NumUp  int `json:"num_up_osds"`
			NumIn  int `json:"num_in_osds"`
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
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	monitors := make([]Monitor, 0, len(raw.MonMap.Mons))
	for _, mon := range raw.MonMap.Mons {
		monitors = append(monitors, Monitor{
			Name: mon.Name,
			Rank: mon.Rank,
			Addr: mon.Addr,
		})
	}
	if len(monitors) == 0 && len(raw.MonMap.QuorumNames) > 0 {
		for i, name := range raw.MonMap.QuorumNames {
			monitors = append(monitors, Monitor{
				Name:   name,
				Rank:   i,
				Status: "up",
			})
		}
	}

	monCount := len(monitors)
	if raw.MonMap.NumMons > monCount {
		monCount = raw.MonMap.NumMons
	}

	standbyCount := len(raw.MgrMap.Standbys)
	if raw.MgrMap.NumStandbys > standbyCount {
		standbyCount = raw.MgrMap.NumStandbys
	}
	if raw.MgrMap.NumStandby > standbyCount {
		standbyCount = raw.MgrMap.NumStandby
	}

	managerCount := standbyCount
	if raw.MgrMap.ActiveName != "" || raw.MgrMap.Available {
		managerCount++
	}

	services := buildServiceSummary(raw.ServiceMap.Services)
	if monCount == 0 {
		if monService, ok := services["mon"]; ok && monService.Total > 0 {
			monCount = monService.Total
			if len(monitors) == 0 {
				for i, daemon := range monService.Daemons {
					monitors = append(monitors, Monitor{
						Name:   daemon,
						Rank:   i,
						Status: "unknown",
					})
				}
			}
		}
	} else if monService, ok := services["mon"]; ok && monService.Total > monCount {
		monCount = monService.Total
	}
	if mgrService, ok := services["mgr"]; ok && mgrService.Total > managerCount {
		managerCount = mgrService.Total
	}

	status := &ClusterStatus{
		FSID: raw.FSID,
		Health: HealthStatus{
			Status: raw.Health.Status,
			Checks: make(map[string]Check),
		},
		MonMap: MonitorMap{
			Epoch:    raw.MonMap.Epoch,
			NumMons:  monCount,
			Monitors: monitors,
		},
		MgrMap: ManagerMap{
			Available: raw.MgrMap.Available,
			NumMgrs:   managerCount,
			ActiveMgr: raw.MgrMap.ActiveName,
			Standbys:  standbyCount,
		},
		OSDMap: OSDMap{
			Epoch:   raw.OSDMap.Epoch,
			NumOSDs: raw.OSDMap.NumOSD,
			NumUp:   raw.OSDMap.NumUp,
			NumIn:   raw.OSDMap.NumIn,
			NumDown: raw.OSDMap.NumOSD - raw.OSDMap.NumUp,
			NumOut:  raw.OSDMap.NumOSD - raw.OSDMap.NumIn,
		},
		PGMap: PGMap{
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

	// Parse health checks
	for name, check := range raw.Health.Checks {
		details := make([]string, 0, len(check.Detail))
		for _, d := range check.Detail {
			details = append(details, d.Message)
		}
		status.Health.Checks[name] = Check{
			Severity: check.Severity,
			Message:  check.Summary.Message,
			Detail:   details,
		}
	}

	if len(services) > 0 {
		keys := make([]string, 0, len(services))
		for serviceType := range services {
			keys = append(keys, serviceType)
		}
		sort.Strings(keys)
		status.Services = make([]ServiceInfo, 0, len(keys)+1)
		for _, serviceType := range keys {
			status.Services = append(status.Services, services[serviceType])
		}
	} else {
		status.Services = []ServiceInfo{
			{Type: "mon", Running: status.MonMap.NumMons, Total: status.MonMap.NumMons, Daemons: monitorNames(status.MonMap.Monitors)},
			{Type: "mgr", Running: boolToInt(raw.MgrMap.Available), Total: status.MgrMap.NumMgrs, Daemons: managerNames(raw.MgrMap.ActiveName, raw.MgrMap.Standbys)},
		}
	}
	if !serviceInfoExists(status.Services, "osd") {
		status.Services = append(status.Services, ServiceInfo{Type: "osd", Running: raw.OSDMap.NumUp, Total: raw.OSDMap.NumOSD})
	}

	return status, nil
}

func buildServiceSummary(services map[string]struct {
	Daemons map[string]struct {
		Status   string `json:"status"`
		Hostname string `json:"hostname"`
		Metadata struct {
			Hostname string `json:"hostname"`
		} `json:"metadata"`
	} `json:"daemons"`
}) map[string]ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	result := make(map[string]ServiceInfo, len(services))
	for serviceType, definition := range services {
		summary := ServiceInfo{Type: serviceType}
		if len(definition.Daemons) > 0 {
			daemonNames := make([]string, 0, len(definition.Daemons))
			for daemonName, daemon := range definition.Daemons {
				summary.Total++
				if isServiceRunning(daemon.Status) {
					summary.Running++
				}
				label := daemonName
				host := strings.TrimSpace(daemon.Hostname)
				if host == "" {
					host = strings.TrimSpace(daemon.Metadata.Hostname)
				}
				if host != "" {
					label = fmt.Sprintf("%s@%s", daemonName, host)
				}
				daemonNames = append(daemonNames, label)
			}
			sort.Strings(daemonNames)
			summary.Daemons = daemonNames
		}
		result[serviceType] = summary
	}

	return result
}

func isServiceRunning(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "active", "up":
		return true
	default:
		return false
	}
}

func monitorNames(monitors []Monitor) []string {
	if len(monitors) == 0 {
		return nil
	}

	names := make([]string, 0, len(monitors))
	for _, mon := range monitors {
		if strings.TrimSpace(mon.Name) != "" {
			names = append(names, mon.Name)
		}
	}
	return names
}

func managerNames(activeName string, standbys []struct {
	Name string `json:"name"`
}) []string {
	names := make([]string, 0, 1+len(standbys))
	if strings.TrimSpace(activeName) != "" {
		names = append(names, activeName)
	}
	for _, standby := range standbys {
		if strings.TrimSpace(standby.Name) != "" {
			names = append(names, standby.Name)
		}
	}
	return names
}

func serviceInfoExists(services []ServiceInfo, serviceType string) bool {
	for _, service := range services {
		if service.Type == serviceType {
			return true
		}
	}
	return false
}

// parseDF parses the output of `ceph df --format json`.
func parseDF(data []byte) ([]Pool, float64, error) {
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
		return nil, 0, fmt.Errorf("unmarshal df: %w", err)
	}

	pools := make([]Pool, 0, len(raw.Pools))
	for _, p := range raw.Pools {
		pools = append(pools, Pool{
			ID:             p.ID,
			Name:           p.Name,
			BytesUsed:      p.Stats.BytesUsed,
			BytesAvailable: p.Stats.MaxAvail,
			Objects:        p.Stats.Objects,
			PercentUsed:    p.Stats.PercentUsed * 100, // Convert from 0-1 to 0-100
		})
	}

	return pools, raw.Stats.PercentUsed * 100, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
